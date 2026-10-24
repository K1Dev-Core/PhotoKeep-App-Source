package gateway

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"golang.org/x/image/draw"
	"photokeep/internal/photokeep"
)

const (
	xkiroChatURL      = "https://api.xkiro.com/v1/chat/completions"
	xkiroModelsURL    = "https://api.xkiro.com/v1/models"
	xkiroUsageURL     = "https://api.xkiro.com/v1/usage"
	xkiroDefaultModel = "mistralai/mistral-medium-3.5"
)

type XKiroAnalyzer struct {
	key, model, chatURL, modelsURL, usageURL string
	http                                     *http.Client
	paceMu                                   sync.Mutex
	nextRequest, pauseUntil                  time.Time
	requestSpacing                           time.Duration
}

type XKiroKeyPool struct {
	keys []*XKiroAnalyzer
	next atomic.Uint64
}

func NewXKiroKeyPool(keys, model string) (*XKiroKeyPool, error) {
	secrets := splitSecrets(keys)
	if len(secrets) == 0 {
		return nil, errors.New("ตั้งค่า XKIRO_API_KEYS ใน gateway.env ก่อน")
	}
	pool := &XKiroKeyPool{keys: make([]*XKiroAnalyzer, 0, len(secrets))}
	for _, secret := range secrets {
		analyzer, err := NewXKiroAnalyzer(secret, model)
		if err != nil {
			return nil, err
		}
		pool.keys = append(pool.keys, analyzer)
	}
	return pool, nil
}

func (p *XKiroKeyPool) KeyCount() int { return len(p.keys) }

func (p *XKiroKeyPool) Health(ctx context.Context) error {
	var last error
	for _, key := range p.keys {
		if err := key.Health(ctx); err == nil {
			return nil
		} else {
			last = err
		}
	}
	return fmt.Errorf("xKiro ทุกคีย์ยังใช้ไม่ได้: %w", last)
}

func (p *XKiroKeyPool) Analyze(ctx context.Context, path string) (photokeep.ImageAIResult, error) {
	start := int(p.next.Add(1)-1) % len(p.keys)
	var last error
	var longestRetry time.Duration
	for offset := range len(p.keys) {
		result, err := p.keys[(start+offset)%len(p.keys)].Analyze(ctx, path)
		if err == nil {
			return result, nil
		}
		last = err
		var httpErr *photokeep.ImageAIHTTPError
		if errors.As(err, &httpErr) {
			if httpErr.RetryAfter > longestRetry {
				longestRetry = httpErr.RetryAfter
			}
			if httpErr.StatusCode == http.StatusTooManyRequests && xkiroIPScopedRateLimit(httpErr.Message) {
				return photokeep.ImageAIResult{}, err
			}
			if httpErr.StatusCode != http.StatusTooManyRequests && httpErr.StatusCode != http.StatusUnauthorized && httpErr.StatusCode != http.StatusForbidden && httpErr.StatusCode < 500 {
				return photokeep.ImageAIResult{}, err
			}
		}
		if !photokeep.IsRetryableImageAIError(err) {
			var httpErr *photokeep.ImageAIHTTPError
			if !errors.As(err, &httpErr) || (httpErr.StatusCode != http.StatusUnauthorized && httpErr.StatusCode != http.StatusForbidden) {
				return photokeep.ImageAIResult{}, err
			}
		}
	}
	var httpErr *photokeep.ImageAIHTTPError
	if errors.As(last, &httpErr) && longestRetry > httpErr.RetryAfter {
		copyErr := *httpErr
		copyErr.RetryAfter = longestRetry
		last = &copyErr
	}
	return photokeep.ImageAIResult{}, last
}

func xkiroIPScopedRateLimit(message string) bool {
	message = strings.ToLower(message)
	return strings.Contains(message, "ip, min") || strings.Contains(message, "ip/min") ||
		strings.Contains(message, "per ip") || strings.Contains(message, "ip limit")
}

func NewXKiroAnalyzer(key, model string) (*XKiroAnalyzer, error) {
	key, model = strings.TrimSpace(key), strings.TrimSpace(model)
	if key == "" {
		return nil, errors.New("ตั้งค่า XKIRO_API_KEYS ที่ Gateway ก่อน")
	}
	if model == "" {
		model = xkiroDefaultModel
	}
	spacing := 600 * time.Millisecond
	if value, err := strconv.Atoi(strings.TrimSpace(os.Getenv("XKIRO_MIN_INTERVAL_MS"))); err == nil && value >= 0 && value <= 5000 {
		spacing = time.Duration(value) * time.Millisecond
	}
	return &XKiroAnalyzer{key: key, model: model, chatURL: xkiroChatURL, modelsURL: xkiroModelsURL, usageURL: xkiroUsageURL, http: &http.Client{Timeout: 95 * time.Second}, requestSpacing: spacing}, nil
}

func (c *XKiroAnalyzer) Health(ctx context.Context) error {
	usageReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.usageURL, nil)
	if err != nil {
		return err
	}
	usageReq.Header.Set("Authorization", "Bearer "+c.key)
	usageResp, err := c.http.Do(usageReq)
	if err != nil {
		return err
	}
	usageResp.Body.Close()
	if usageResp.StatusCode != http.StatusOK {
		return fmt.Errorf("xKiro key/usage HTTP %d", usageResp.StatusCode)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.modelsURL, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("xKiro models HTTP %d", resp.StatusCode)
	}
	var models struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&models); err != nil {
		return err
	}
	for _, model := range models.Data {
		if model.ID == c.model {
			return nil
		}
	}
	return fmt.Errorf("xKiro ไม่มีโมเดล %s", c.model)
}

func (c *XKiroAnalyzer) Analyze(ctx context.Context, path string) (photokeep.ImageAIResult, error) {
	jpegBytes, err := prepareXKiroJPEG(path)
	if err != nil {
		return photokeep.ImageAIResult{}, err
	}
	prompt := "อธิบายสิ่งที่เห็นจริงในภาพเป็นภาษาไทยธรรมชาติ 1-2 ประโยค บอกวัตถุหลัก การกระทำ ลักษณะ และฉากหลังที่เห็นชัด ห้ามเดาความสัมพันธ์หรือสถานที่ ห้ามขึ้นต้นว่า ภาพแสดงหรือในภาพ ตอบ JSON เท่านั้น รูปแบบ {\"caption\":\"...\",\"tags\":[\"...\"]} โดย tags เป็นคำไทย 2-5 คำที่เฉพาะเจาะจงและไม่ซ้ำ"
	payload, err := json.Marshal(map[string]any{
		"model": c.model, "temperature": 0, "max_tokens": 320, "stream": false,
		"messages": []any{map[string]any{"role": "user", "content": []any{
			map[string]string{"type": "text", "text": prompt},
			map[string]any{"type": "image_url", "image_url": map[string]string{"url": "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(jpegBytes)}},
		}}},
	})
	if err != nil {
		return photokeep.ImageAIResult{}, err
	}
	if err := c.waitForRequestSlot(ctx); err != nil {
		return photokeep.ImageAIResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.chatURL, bytes.NewReader(payload))
	if err != nil {
		return photokeep.ImageAIResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return photokeep.ImageAIResult{}, &photokeep.ImageAITransportError{Cause: err}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20+1))
	if err != nil {
		return photokeep.ImageAIResult{}, err
	}
	if len(body) > 2<<20 {
		return photokeep.ImageAIResult{}, errors.New("ผลจาก xKiro ใหญ่เกินไป")
	}
	if resp.StatusCode != http.StatusOK {
		var detail struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(body, &detail)
		message := strings.TrimSpace(detail.Error.Message)
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}
		retryAfter := xkiroRetryAfter(resp.Header.Get("Retry-After"))
		if resp.StatusCode == http.StatusTooManyRequests {
			c.pauseAfterRateLimit(retryAfter)
		}
		return photokeep.ImageAIResult{}, &photokeep.ImageAIHTTPError{StatusCode: resp.StatusCode, Message: message, RetryAfter: retryAfter}
	}
	if rateErr := xkiroRateLimitInBody(body, resp.Header.Get("Retry-After")); rateErr != nil {
		var httpErr *photokeep.ImageAIHTTPError
		if errors.As(rateErr, &httpErr) {
			c.pauseAfterRateLimit(httpErr.RetryAfter)
		}
		return photokeep.ImageAIResult{}, rateErr
	}
	var response struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return photokeep.ImageAIResult{}, fmt.Errorf("xKiro ส่ง response ไม่ถูกต้อง: %w", err)
	}
	if len(response.Choices) == 0 {
		return photokeep.ImageAIResult{}, errors.New("xKiro ไม่ส่งคำตอบ")
	}
	if response.Choices[0].FinishReason == "length" {
		return photokeep.ImageAIResult{}, errors.New("xKiro ตอบไม่ครบเพราะถึง max_tokens")
	}
	var content string
	if err := json.Unmarshal(response.Choices[0].Message.Content, &content); err != nil {
		return photokeep.ImageAIResult{}, errors.New("xKiro ไม่ส่งข้อความคำตอบ")
	}
	result, err := parseXKiroAnalysis(content)
	if err != nil {
		return photokeep.ImageAIResult{}, err
	}
	result.Filename, result.Model = filepath.Base(path), c.model
	if response.Model != "" {
		result.Model = response.Model
	}
	return result, nil
}

func xkiroRateLimitInBody(body []byte, retryAfterHeader string) error {
	text := string(body)
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "rate limit") && !strings.Contains(lower, "too many requests") {
		return nil
	}
	message := strings.TrimSpace(text)
	if len(message) > 280 {
		message = message[:280]
	}
	delay := xkiroRetryAfter(retryAfterHeader)
	if delay == 0 {
		lowerIndex := strings.Index(lower, "retry after")
		if lowerIndex >= 0 {
			remaining := strings.Fields(strings.TrimLeft(strings.TrimSpace(text[lowerIndex+len("retry after"):]), ": ="))
			if len(remaining) > 0 {
				if parsed, err := time.ParseDuration(strings.TrimRight(remaining[0], ".,;")); err == nil && parsed > 0 {
					delay = parsed
				} else if seconds, parseErr := strconv.Atoi(strings.TrimRight(remaining[0], ".,;sS")); parseErr == nil && seconds > 0 {
					delay = time.Duration(seconds) * time.Second
				}
			}
		}
	}
	return &photokeep.ImageAIHTTPError{StatusCode: http.StatusTooManyRequests, Message: message, RetryAfter: delay}
}

func (c *XKiroAnalyzer) waitForRequestSlot(ctx context.Context) error {
	c.paceMu.Lock()
	start := time.Now()
	if c.nextRequest.After(start) {
		start = c.nextRequest
	}
	if c.pauseUntil.After(start) {
		start = c.pauseUntil
	}
	c.nextRequest = start.Add(c.requestSpacing)
	c.paceMu.Unlock()
	for {
		c.paceMu.Lock()
		if c.pauseUntil.After(start) {
			start = c.pauseUntil
		}
		c.paceMu.Unlock()
		if wait := time.Until(start); wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			}
			continue
		}
		return ctx.Err()
	}
}

func (c *XKiroAnalyzer) pauseAfterRateLimit(delay time.Duration) {
	if delay < 5*time.Second {
		delay = 5 * time.Second
	}
	c.paceMu.Lock()
	if until := time.Now().Add(delay); until.After(c.pauseUntil) {
		c.pauseUntil = until
	}
	if c.requestSpacing < 700*time.Millisecond {
		c.requestSpacing += 25 * time.Millisecond
	}
	c.paceMu.Unlock()
}

func prepareXKiroJPEG(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	config, _, err := image.DecodeConfig(f)
	if err != nil {
		return nil, err
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 50_000_000 {
		return nil, errors.New("ภาพมีขนาดพิกเซลเกินที่รองรับ")
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	source, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	w, h := source.Bounds().Dx(), source.Bounds().Dy()
	if w > 512 || h > 512 {
		scale := 512.0 / float64(max(w, h))
		w, h = max(1, int(float64(w)*scale)), max(1, int(float64(h)*scale))
	}
	resized := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.ApproxBiLinear.Scale(resized, resized.Bounds(), source, source.Bounds(), draw.Over, nil)
	var output bytes.Buffer
	if err := jpeg.Encode(&output, resized, &jpeg.Options{Quality: 74}); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func parseXKiroAnalysis(content string) (photokeep.ImageAIResult, error) {
	start := strings.Index(content, "{")
	if start < 0 {
		return photokeep.ImageAIResult{}, errors.New("xKiro ไม่ส่ง JSON object")
	}
	var result photokeep.ImageAIResult
	if err := json.NewDecoder(strings.NewReader(content[start:])).Decode(&result); err != nil {
		return result, fmt.Errorf("xKiro ส่ง JSON ไม่ถูกต้อง: %w", err)
	}
	result.Caption = strings.TrimSpace(result.Caption)
	if result.Caption == "" {
		result.Caption = strings.TrimSpace(result.Description)
	}
	result.Description = result.Caption

	tags := make([]string, 0, 5)
	seen := make(map[string]bool)
	for _, tag := range result.Tags {
		tag = strings.TrimSpace(strings.Trim(tag, "#, "))
		if tag == "" || seen[strings.ToLower(tag)] || !xkiroThaiTag(tag) {
			continue
		}
		seen[strings.ToLower(tag)] = true
		tags = append(tags, tag)
		if len(tags) == 5 {
			break
		}
	}
	result.Tags = tags
	if len(tags) == 0 {
		return result, errors.New("xKiro ไม่ส่งแท็ก")
	}
	if err := photokeep.ValidateThaiAnalysis(result); err != nil {
		return result, err
	}
	return result, nil
}

func xkiroThaiTag(tag string) bool {
	hasThai := false
	for _, r := range tag {
		if unicode.In(r, unicode.Thai) && unicode.IsLetter(r) {
			hasThai = true
		}
		if unicode.IsLetter(r) && !unicode.In(r, unicode.Thai) {
			return false
		}
	}
	return hasThai
}

func xkiroRetryAfter(value string) time.Duration {
	if seconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil && seconds > 0 {
		return time.Duration(seconds * float64(time.Second))
	}
	if when, err := http.ParseTime(value); err == nil {
		return max(0, time.Until(when))
	}
	return 0
}
