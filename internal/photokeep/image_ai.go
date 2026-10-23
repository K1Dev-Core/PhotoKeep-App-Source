package photokeep

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const imageAIAnalysisMaxSide = 768

type ManagedImageAIClient struct {
	endpoint string
	health   string
	http     *http.Client
}

type ImageAIResult struct {
	Filename     string   `json:"filename"`
	Caption      string   `json:"caption"`
	Description  string   `json:"description"`
	Tags         []string `json:"tags"`
	Model        string   `json:"model"`
	ModelVersion string   `json:"model_version"`
	Status       string   `json:"status"`
	ProcessingMS float64  `json:"processing_ms"`
	Success      *bool    `json:"success"`
	Error        string   `json:"error"`
}

func NewManagedImageAIClient(endpoint string) (*ManagedImageAIClient, error) {
	endpoint = strings.TrimSpace(endpoint)
	if !strings.HasPrefix(endpoint, "https://") && !strings.HasPrefix(endpoint, "http://127.0.0.1:") && !strings.HasPrefix(endpoint, "http://localhost:") {
		return nil, errors.New("API กลางต้องใช้ HTTPS (หรือ localhost สำหรับทดสอบ)")
	}
	if !strings.HasSuffix(endpoint, "/v1/analyze") {
		return nil, errors.New("URL API กลางต้องลงท้ายด้วย /v1/analyze")
	}
	return &ManagedImageAIClient{endpoint: endpoint, health: strings.TrimSuffix(endpoint, "/v1/analyze") + "/health", http: &http.Client{Timeout: 110 * time.Second}}, nil
}

func (c *ManagedImageAIClient) Health(ctx context.Context) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.health, nil)
	if err != nil {
		return err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("เชื่อมต่อ AI server ไม่สำเร็จ: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if err != nil {
		return fmt.Errorf("อ่านสถานะ AI server ไม่สำเร็จ: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("AI health ตอบกลับ HTTP %d: %s", response.StatusCode, imageAIErrorMessage(body))
	}
	var status struct {
		OK              bool   `json:"ok"`
		Status          string `json:"status"`
		ModelLoaded     *bool  `json:"model_loaded"`
		ModelAvailable  *bool  `json:"model_available"`
		OllamaReachable *bool  `json:"ollama_reachable"`
		Success         *bool  `json:"success"`
		Error           string `json:"error"`
	}
	if err := json.Unmarshal(body, &status); err != nil {
		return fmt.Errorf("รูปแบบสถานะ AI server ไม่ถูกต้อง: %w", err)
	}
	if status.Success != nil && !*status.Success {
		return fmt.Errorf("AI server ยังไม่พร้อม: %s", strings.TrimSpace(status.Error))
	}
	if !status.OK && status.Status != "ok" && status.Status != "ready" {
		return errors.New("AI server ยังไม่พร้อม กรุณารอสักครู่แล้วลองใหม่")
	}

	modelReady := status.ModelLoaded != nil && *status.ModelLoaded
	modelCanLoad := status.ModelAvailable != nil && *status.ModelAvailable
	if (status.ModelLoaded != nil || status.ModelAvailable != nil) && !modelReady && !modelCanLoad {
		if status.OllamaReachable != nil && !*status.OllamaReachable {
			return errors.New("เชื่อมต่อ Ollama ไม่ได้ กรุณาตรวจสอบเครื่อง AI แล้วลองใหม่")
		}
		return errors.New("AI model ยังไม่พร้อม กรุณารอสักครู่แล้วลองใหม่")
	}
	return nil
}

func (c *ManagedImageAIClient) Concurrency(ctx context.Context) (int, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.health, nil)
	if err != nil {
		return 0, err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return 0, fmt.Errorf("อ่านจำนวน worker จาก Gateway ไม่สำเร็จ: %w", err)
	}
	defer response.Body.Close()
	var status struct {
		Success     bool   `json:"success"`
		Status      string `json:"status"`
		Concurrency int    `json:"concurrency"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&status); err != nil {
		return 0, fmt.Errorf("อ่าน concurrency จาก Gateway ไม่สำเร็จ: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || (!status.Success && status.Status != "ok" && status.Status != "ready") {
		return 0, fmt.Errorf("Gateway ยังไม่พร้อมสำหรับวิเคราะห์ภาพ (HTTP %d)", response.StatusCode)
	}
	if status.Concurrency < 1 {
		return 0, errors.New("Gateway ไม่ได้แจ้งค่า concurrency สำหรับวิเคราะห์ภาพ")
	}
	return status.Concurrency, nil
}

type ImageAIHTTPError struct {
	StatusCode int
	Code       int
	Message    string
	RetryAfter time.Duration
}

func (e *ImageAIHTTPError) Error() string {
	return fmt.Sprintf("AI server ตอบกลับ HTTP %d: %s", e.StatusCode, e.Message)
}

func (e *ImageAIHTTPError) ShouldRetry() bool {
	return retryableAIStatus(e.StatusCode)
}

func retryAfterHeader(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if at, err := http.ParseTime(value); err == nil && at.After(now) {
		return at.Sub(now)
	}
	return 0
}

func parseRetryAfter(value string) time.Duration {
	return retryAfterHeader(value, time.Now())
}

func imageAIRequestRetryDelay(err error, attempt int) time.Duration {
	var httpErr *ImageAIHTTPError
	if errors.As(err, &httpErr) && httpErr.RetryAfter > 0 {
		return httpErr.RetryAfter
	}
	return time.Duration(attempt+1) * 2 * time.Second
}

type ImageAITransportError struct{ Cause error }

func (e *ImageAITransportError) Error() string {
	return fmt.Sprintf("เชื่อมต่อ AI server ไม่สำเร็จ: %v", e.Cause)
}

func (e *ImageAITransportError) Unwrap() error { return e.Cause }

func (e *ImageAITransportError) ShouldRetry() bool { return true }

func IsRetryableImageAIError(err error) bool {
	if err == nil {
		return false
	}
	var httpErr *ImageAIHTTPError
	if errors.As(err, &httpErr) {
		return httpErr.ShouldRetry()
	}
	var transportErr *ImageAITransportError
	if errors.As(err, &transportErr) {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary())
}

func retryableAIStatus(status int) bool {

	return status == http.StatusRequestTimeout || status == http.StatusTooManyRequests ||
		status == http.StatusInternalServerError || status == http.StatusBadGateway ||
		status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout ||
		(status >= 520 && status <= 527) || status == 530
}

func (c *ManagedImageAIClient) Analyze(ctx context.Context, path string) (ImageAIResult, error) {
	return c.analyze(ctx, path)
}

func (c *ManagedImageAIClient) analyze(ctx context.Context, path string) (ImageAIResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return ImageAIResult{}, fmt.Errorf("เปิดรูปสำหรับวิเคราะห์ไม่ได้: %w", err)
	}
	if info.Size() > maxAnalysisSource {
		return ImageAIResult{}, errors.New("ไฟล์ภาพใหญ่เกิน 100 MB สำหรับเตรียมส่ง AI")
	}
	ext := strings.ToLower(filepath.Ext(path))
	if imageAIContentType(ext) == "" {
		return ImageAIResult{}, fmt.Errorf("ชนิดไฟล์ %s ไม่รองรับสำหรับวิเคราะห์ภาพ", ext)
	}
	uploadPath, filename, contentType, cleanup, err := prepareVisionImageWithMaxSide(path, imageAIAnalysisMaxSide)
	if err != nil {
		return ImageAIResult{}, err
	}
	defer cleanup()
	uploadInfo, err := os.Stat(uploadPath)
	if err != nil {
		return ImageAIResult{}, fmt.Errorf("เตรียมรูปสำหรับวิเคราะห์ไม่ได้: %w", err)
	}
	if uploadInfo.Size() > maxVisionImageBytes {
		return ImageAIResult{}, errors.New("รูปที่เตรียมส่งยังใหญ่เกิน 12 MB")
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		result, retry, err := c.analyzeOnce(ctx, uploadPath, filename, contentType)
		if err == nil || !retry || attempt == 2 {
			return result, err
		}
		lastErr = err
		timer := time.NewTimer(imageAIRequestRetryDelay(err, attempt))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ImageAIResult{}, ctx.Err()
		case <-timer.C:
		}
	}
	return ImageAIResult{}, lastErr
}

func (c *ManagedImageAIClient) analyzeOnce(ctx context.Context, path, filename, contentType string) (ImageAIResult, bool, error) {
	reader, writer := io.Pipe()
	form := multipart.NewWriter(writer)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, reader)
	if err != nil {
		reader.Close()
		writer.Close()
		return ImageAIResult{}, false, err
	}
	request.Header.Set("Content-Type", form.FormDataContentType())
	writeDone := make(chan error, 1)
	go func() {
		file, err := os.Open(path)
		if err != nil {
			_ = writer.CloseWithError(err)
			writeDone <- err
			return
		}
		defer file.Close()
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": filename}))
		header.Set("Content-Type", contentType)
		part, err := form.CreatePart(header)
		if err == nil {
			_, err = io.Copy(part, file)
		}
		if closeErr := form.Close(); err == nil {
			err = closeErr
		}
		_ = writer.CloseWithError(err)
		writeDone <- err
	}()
	response, err := c.http.Do(request)
	if err != nil {
		_ = reader.CloseWithError(err)
		<-writeDone
		return ImageAIResult{}, true, &ImageAITransportError{Cause: err}
	}
	defer response.Body.Close()
	_ = reader.Close()
	if err := <-writeDone; err != nil {
		return ImageAIResult{}, false, fmt.Errorf("ส่งรูปไปยัง AI server ไม่สำเร็จ: %w", err)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20+1))
	if err != nil {
		return ImageAIResult{}, false, fmt.Errorf("อ่านผลวิเคราะห์จาก AI server ไม่สำเร็จ: %w", err)
	}
	if len(body) > 1<<20 {
		return ImageAIResult{}, false, errors.New("ผลตอบกลับจาก AI server มีขนาดใหญ่เกินไป")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message := imageAIErrorMessage(body)
		apiErr := &ImageAIHTTPError{StatusCode: response.StatusCode, Message: message, RetryAfter: retryAfterHeader(response.Header.Get("Retry-After"), time.Now())}
		return ImageAIResult{}, apiErr.ShouldRetry(), apiErr
	}
	var result ImageAIResult
	if err := json.Unmarshal(body, &result); err != nil {
		return result, false, fmt.Errorf("รูปแบบผลตอบกลับจาก AI server ไม่ถูกต้อง: %w", err)
	}
	if (result.Success != nil && !*result.Success) || (result.Success == nil && result.Status != "" && result.Status != "success" && result.Status != "ok") {
		if strings.TrimSpace(result.Error) != "" {
			return result, false, fmt.Errorf("AI server แจ้งข้อผิดพลาด: %s", strings.TrimSpace(result.Error))
		}
		return result, false, errors.New("AI server แจ้งว่าวิเคราะห์ภาพไม่สำเร็จ")
	}
	if strings.TrimSpace(result.Caption) == "" {
		result.Caption = strings.TrimSpace(result.Description)
	}
	if result.Model == "" {
		result.Model = result.ModelVersion
	}
	if strings.TrimSpace(result.Caption) == "" {
		return result, false, errors.New("AI server ไม่ได้ส่งคำอธิบายภาพกลับมา")
	}
	return result, false, nil
}

func ValidateThaiAnalysis(result ImageAIResult) error {
	caption := strings.TrimSpace(result.Caption)
	if caption == "" {
		caption = strings.TrimSpace(result.Description)
	}
	if !containsThaiLetter(caption) {
		return errors.New("คำอธิบายไม่มีข้อความภาษาไทยที่อ่านได้")
	}
	texts := []string{caption}
	if description := strings.TrimSpace(result.Description); description != "" && description != caption {
		texts = append(texts, description)
	}
	for _, text := range texts {
		for _, r := range text {
			if unicode.IsLetter(r) && !unicode.In(r, unicode.Thai) && !unicode.In(r, unicode.Latin) {
				return errors.New("ผลวิเคราะห์มีอักขระจากภาษาอื่นปะปน")
			}
		}
	}
	for _, tag := range result.Tags {
		if !isThaiOnlyTag(strings.TrimSpace(tag)) {
			return errors.New("แท็กต้องเป็นภาษาไทย")
		}
	}
	return nil
}

func containsThaiLetter(value string) bool {
	for _, r := range value {
		if unicode.In(r, unicode.Thai) && unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

func isThaiOnlyTag(value string) bool {
	if !containsThaiLetter(value) {
		return false
	}
	for _, r := range value {
		if unicode.IsLetter(r) && !unicode.In(r, unicode.Thai) {
			return false
		}
	}
	return true
}

func imageAIContentType(ext string) string {
	if contentType := mime.TypeByExtension(ext); strings.HasPrefix(contentType, "image/") {
		return contentType
	}
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".bmp":
		return "image/bmp"
	default:
		return ""
	}
}

func imageAIErrorMessage(body []byte) string {
	var payload map[string]any
	if json.Unmarshal(body, &payload) == nil {
		for _, key := range []string{"detail", "error", "message"} {
			if message, ok := payload[key].(string); ok && strings.TrimSpace(message) != "" {
				return strings.TrimSpace(message)
			}
		}
	}
	message := strings.TrimSpace(string(body))
	if len(message) > 300 {
		message = message[:300]
	}
	if message == "" {
		return http.StatusText(http.StatusBadGateway)
	}
	return message
}
