package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"photokeep/internal/photokeep"
)

const defaultVisionAnalyzeURL = "https://vision.k1god.com/v1/analyze"

type RateLimitFailoverAnalyzer struct {
	primary   Analyzer
	fallback  Analyzer
	mu        sync.Mutex
	openUntil time.Time
	probing   bool
}

func NewRateLimitFailoverAnalyzer(primary, fallback Analyzer) *RateLimitFailoverAnalyzer {
	return &RateLimitFailoverAnalyzer{primary: primary, fallback: fallback}
}

func (a *RateLimitFailoverAnalyzer) Health(ctx context.Context) error {
	if a.primary != nil {
		if err := a.primary.Health(ctx); err == nil {
			return nil
		}
	}
	if a.fallback != nil {
		return a.fallback.Health(ctx)
	}
	return errors.New("ไม่มี AI provider ที่พร้อมใช้งาน")
}

func (a *RateLimitFailoverAnalyzer) Analyze(ctx context.Context, path string) (photokeep.ImageAIResult, error) {
	if a.fallback == nil {
		return a.primary.Analyze(ctx, path)
	}
	usePrimary, probe := a.acquirePrimary()
	if !usePrimary {
		return a.fallback.Analyze(ctx, path)
	}
	result, err := a.primary.Analyze(ctx, path)
	if err == nil {
		if probe {
			a.closeCircuit()
		}
		return result, nil
	}
	var providerErr *photokeep.ImageAIHTTPError
	if errors.As(err, &providerErr) && providerErr.StatusCode == http.StatusTooManyRequests {
		a.openCircuit(providerErr.RetryAfter)
		return a.fallback.Analyze(ctx, path)
	}
	if probe {
		a.releaseProbe()
	}
	return photokeep.ImageAIResult{}, err
}

func (a *RateLimitFailoverAnalyzer) acquirePrimary() (bool, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.openUntil.IsZero() {
		return true, false
	}
	now := time.Now()
	if now.Before(a.openUntil) || a.probing {
		return false, false
	}
	a.probing = true
	return true, true
}

func (a *RateLimitFailoverAnalyzer) releaseProbe() {
	a.mu.Lock()
	a.probing = false
	a.mu.Unlock()
}

func (a *RateLimitFailoverAnalyzer) closeCircuit() {
	a.mu.Lock()
	a.openUntil = time.Time{}
	a.probing = false
	a.mu.Unlock()
}

func (a *RateLimitFailoverAnalyzer) openCircuit(retryAfter time.Duration) {
	if retryAfter <= 0 {
		retryAfter = 40 * time.Second
	}
	if retryAfter > 30*time.Minute {
		retryAfter = 30 * time.Minute
	}
	a.mu.Lock()
	a.openUntil = time.Now().Add(retryAfter)
	a.probing = false
	a.mu.Unlock()
}

type RemoteVisionAnalyzer struct {
	endpoint string
	health   string
	apiKey   string
	header   string
	http     *http.Client
}

func NewRemoteVisionAnalyzer(endpoint, apiKey, apiKeyHeader string) (*RemoteVisionAnalyzer, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = defaultVisionAnalyzeURL
	}
	if !strings.HasPrefix(endpoint, "https://") && !strings.HasPrefix(endpoint, "http://127.0.0.1:") && !strings.HasPrefix(endpoint, "http://localhost:") {
		return nil, errors.New("Fallback AI ต้องใช้ HTTPS (หรือ localhost สำหรับทดสอบ)")
	}
	if !strings.HasSuffix(endpoint, "/v1/analyze") {
		return nil, errors.New("Fallback URL ต้องลงท้ายด้วย /v1/analyze")
	}
	healthURL := strings.TrimSuffix(endpoint, "/v1/analyze") + "/health"
	apiKeyHeader = strings.TrimSpace(apiKeyHeader)
	if apiKeyHeader == "" {
		apiKeyHeader = "x-api-key"
	}
	return &RemoteVisionAnalyzer{
		endpoint: endpoint,
		health:   healthURL,
		apiKey:   strings.TrimSpace(apiKey),
		header:   apiKeyHeader,
		http:     &http.Client{Timeout: 105 * time.Second},
	}, nil
}

func (a *RemoteVisionAnalyzer) Health(ctx context.Context) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, a.health, nil)
	if err != nil {
		return err
	}
	if a.apiKey != "" {
		request.Header.Set(a.header, a.apiKey)
	}
	response, err := a.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Fallback AI health HTTP %d", response.StatusCode)
	}
	var health struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&health); err != nil {
		return fmt.Errorf("อ่านสถานะ Fallback AI ไม่ได้: %w", err)
	}
	if !health.Success {
		return errors.New("Fallback AI ยังไม่พร้อม")
	}
	return nil
}

func (a *RemoteVisionAnalyzer) Analyze(ctx context.Context, path string) (photokeep.ImageAIResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return photokeep.ImageAIResult{}, err
	}
	defer file.Close()
	reader, writer := io.Pipe()
	form := multipart.NewWriter(writer)
	go func() {
		part, createErr := form.CreateFormFile("file", filepath.Base(path))
		if createErr == nil {
			_, createErr = io.Copy(part, file)
		}
		if closeErr := form.Close(); createErr == nil {
			createErr = closeErr
		}
		_ = writer.CloseWithError(createErr)
	}()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint, reader)
	if err != nil {
		_ = reader.CloseWithError(err)
		return photokeep.ImageAIResult{}, err
	}
	request.Header.Set("Content-Type", form.FormDataContentType())
	request.Header.Set("Accept", "application/json")
	if a.apiKey != "" {
		request.Header.Set(a.header, a.apiKey)
	}
	response, err := a.http.Do(request)
	if err != nil {
		_ = reader.CloseWithError(err)
		return photokeep.ImageAIResult{}, &photokeep.ImageAITransportError{Cause: err}
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20+1))
	if err != nil {
		return photokeep.ImageAIResult{}, err
	}
	if len(body) > 2<<20 {
		return photokeep.ImageAIResult{}, errors.New("ผลจาก Fallback AI ใหญ่เกินไป")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return photokeep.ImageAIResult{}, cloudProviderError(response.StatusCode, response.Header, body, "Fallback AI")
	}
	result, err := parseRemoteVisionResult(body)
	if err != nil {
		return photokeep.ImageAIResult{}, err
	}
	if result.Filename == "" {
		result.Filename = filepath.Base(path)
	}
	return result, nil
}

func parseRemoteVisionResult(body []byte) (photokeep.ImageAIResult, error) {
	var result photokeep.ImageAIResult
	if err := json.Unmarshal(body, &result); err != nil {
		return result, fmt.Errorf("Fallback AI ส่ง response ไม่ถูกต้อง: %w", err)
	}
	if result.Success != nil && !*result.Success {
		message := strings.TrimSpace(result.Error)
		if message == "" {
			message = "API ระบุว่าไม่สำเร็จ"
		}
		return result, errors.New("Fallback AI: " + message)
	}
	if strings.TrimSpace(result.Caption) == "" && strings.TrimSpace(result.Description) == "" {
		return result, errors.New("Fallback AI ไม่ส่งคำอธิบายภาพ")
	}
	return result, nil
}
