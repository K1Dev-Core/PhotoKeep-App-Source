package gateway

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"photokeep/internal/photokeep"
)

const (
	cloudflareVisionModel = "@cf/meta/llama-3.2-11b-vision-instruct"
	cloudflareAPIBase     = "https://api.cloudflare.com/client/v4"
	thaiLLMEndpoint       = "https://thaillm.or.th/api/v1/chat/completions"
	thaiLLMDefaultModel   = "openthaigpt-thaillm-8b-instruct-v7.2"
)

type CloudflareAccount struct {
	AccountID string
	Token     string
}

type CloudflareThaiLLMAnalyzer struct {
	accounts []CloudflareAccount
	thaiKeys []*thailandKey
	model    string
	http     *http.Client
	vision   atomic.Uint64
	thai     atomic.Uint64
}

type thailandKey struct {
	value      string
	mu         sync.Mutex
	next       time.Time
	blockedTil time.Time
}

func NewCloudflareThaiLLMAnalyzer(accounts []CloudflareAccount, keys []string, model string) (*CloudflareThaiLLMAnalyzer, error) {
	if len(accounts) == 0 {
		return nil, errors.New("ตั้งค่าบัญชี CLOUDFLARE_ACCOUNTS ใน gateway.env")
	}
	if len(keys) == 0 {
		return nil, errors.New("ตั้งค่า THAILLM_API_KEYS ใน gateway.env")
	}
	if strings.TrimSpace(model) == "" {
		model = thaiLLMDefaultModel
	}
	analyzer := &CloudflareThaiLLMAnalyzer{
		accounts: append([]CloudflareAccount(nil), accounts...),
		model:    strings.TrimSpace(model),
		http:     &http.Client{Timeout: 65 * time.Second},
	}
	for _, key := range keys {
		if key = strings.TrimSpace(key); key != "" {
			analyzer.thaiKeys = append(analyzer.thaiKeys, &thailandKey{value: key})
		}
	}
	if len(analyzer.thaiKeys) == 0 {
		return nil, errors.New("ไม่พบ ThaiLLM API key ที่ใช้งานได้")
	}
	return analyzer, nil
}

func (a *CloudflareThaiLLMAnalyzer) DefaultConcurrency() int {
	count := min(len(a.accounts), len(a.thaiKeys))
	if count < 1 {
		return 1
	}
	return min(count*2, 128)
}

func (a *CloudflareThaiLLMAnalyzer) Health(ctx context.Context) error {
	if len(a.accounts) == 0 || len(a.thaiKeys) == 0 {
		return errors.New("Cloudflare/ThaiLLM ยังตั้งค่าไม่ครบ")
	}
	var last error
	cloudflareReady := false
	for _, account := range a.accounts {
		endpoint := cloudflareAPIBase + "/accounts/" + account.AccountID + "/ai/models/search"
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return err
		}
		request.Header.Set("Authorization", "Bearer "+account.Token)
		response, err := a.http.Do(request)
		if err != nil {
			last = err
			continue
		}
		var result struct {
			Success bool `json:"success"`
		}
		decodeErr := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result)
		response.Body.Close()
		if decodeErr == nil && response.StatusCode >= 200 && response.StatusCode < 300 && result.Success {
			cloudflareReady = true
			break
		}
		last = fmt.Errorf("Cloudflare model health ตอบกลับ HTTP %d", response.StatusCode)
	}
	if !cloudflareReady {
		return fmt.Errorf("Cloudflare ทุกบัญชียังไม่พร้อม: %w", last)
	}
	last = nil
	for _, key := range a.thaiKeys {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://thaillm.or.th/api/v1/models", nil)
		if err != nil {
			return err
		}
		request.Header.Set("Authorization", "Bearer "+key.value)
		response, err := a.http.Do(request)
		if err != nil {
			last = err
			continue
		}
		response.Body.Close()
		if response.StatusCode >= 200 && response.StatusCode < 300 {
			return nil
		}
		last = fmt.Errorf("ThaiLLM health ตอบกลับ HTTP %d", response.StatusCode)
	}
	return fmt.Errorf("ThaiLLM ทุกคีย์ health check ไม่ผ่าน: %w", last)
}

func (a *CloudflareThaiLLMAnalyzer) Analyze(ctx context.Context, path string) (photokeep.ImageAIResult, error) {
	imageBytes, err := prepareXKiroJPEG(path)
	if err != nil {
		return photokeep.ImageAIResult{}, fmt.Errorf("เตรียมภาพสำหรับวิเคราะห์ไม่ได้: %w", err)
	}
	started := time.Now()
	english, err := a.describe(ctx, imageBytes)
	if err != nil {
		return photokeep.ImageAIResult{}, err
	}
	result, err := a.translate(ctx, english)
	if err != nil {
		return photokeep.ImageAIResult{}, err
	}
	if err := photokeep.ValidateThaiAnalysis(result); err != nil {
		return photokeep.ImageAIResult{}, fmt.Errorf("ผล ThaiLLM ไม่ผ่านการตรวจสอบ: %w", err)
	}
	result.Filename = filepath.Base(path)
	result.Model = cloudflareVisionModel + " + " + a.model
	result.ProcessingMS = float64(time.Since(started).Milliseconds())
	return result, nil
}

func (a *CloudflareThaiLLMAnalyzer) describe(ctx context.Context, imageBytes []byte) (string, error) {
	requestBody, err := json.Marshal(map[string]any{
		"image":      "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(imageBytes),
		"prompt":     "Describe this photo in one factual English sentence, 12 to 22 words. Name only visible objects, actions, and surroundings. Do not guess location, identity, or relationships.",
		"max_tokens": 64, "temperature": 0,
	})
	if err != nil {
		return "", err
	}
	start := int(a.vision.Add(1)-1) % len(a.accounts)
	var last error
	for offset := range len(a.accounts) {
		account := a.accounts[(start+offset)%len(a.accounts)]
		endpoint := cloudflareAPIBase + "/accounts/" + account.AccountID + "/ai/run/" + cloudflareVisionModel
		request, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestBody))
		if reqErr != nil {
			return "", reqErr
		}
		request.Header.Set("Authorization", "Bearer "+account.Token)
		request.Header.Set("Content-Type", "application/json")
		response, doErr := a.http.Do(request)
		if doErr != nil {
			last = &photokeep.ImageAITransportError{Cause: doErr}
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 2<<20))
		response.Body.Close()
		if readErr != nil {
			last = readErr
			continue
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			last = cloudProviderError(response.StatusCode, response.Header, body, "Cloudflare Vision")
			if !shouldTryNextProviderKey(last) {
				return "", last
			}
			continue
		}
		var payload struct {
			Success bool            `json:"success"`
			Errors  []providerError `json:"errors"`
			Result  json.RawMessage `json:"result"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return "", fmt.Errorf("Cloudflare Vision ส่ง JSON ไม่ถูกต้อง: %w", err)
		}
		if !payload.Success {
			message := "Cloudflare Vision วิเคราะห์ไม่สำเร็จ"
			code := 0
			if len(payload.Errors) > 0 && payload.Errors[0].Message != "" {
				message = payload.Errors[0].Message
				code = payload.Errors[0].Code
			}
			if code == 3036 {
				last = &photokeep.ImageAIHTTPError{StatusCode: http.StatusTooManyRequests, Code: code, Message: "Cloudflare Vision โควตาบัญชีนี้หมด", RetryAfter: cloudflareQuotaRetryAfter()}
				continue
			}
			return "", errors.New(message)
		}
		caption, err := cloudflareCaption(payload.Result)
		if err != nil {
			return "", err
		}
		return caption, nil
	}
	return "", last
}

func cloudflareCaption(raw json.RawMessage) (string, error) {
	var plain string
	if json.Unmarshal(raw, &plain) == nil {
		if strings.TrimSpace(plain) != "" {
			return strings.TrimSpace(plain), nil
		}
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil {
		return "", errors.New("Cloudflare Vision ไม่ได้ส่ง result ที่อ่านได้")
	}
	for _, name := range []string{"response", "caption", "description", "text"} {
		if value, ok := object[name]; ok && json.Unmarshal(value, &plain) == nil && strings.TrimSpace(plain) != "" {
			return strings.TrimSpace(plain), nil
		}
	}
	return "", errors.New("Cloudflare Vision ไม่ได้ส่งคำบรรยายภาพ")
}

func (a *CloudflareThaiLLMAnalyzer) translate(ctx context.Context, english string) (photokeep.ImageAIResult, error) {
	prompt := "แปลงคำบรรยายภาพภาษาอังกฤษต่อไปนี้เป็นภาษาไทยที่เป็นธรรมชาติและเข้าใจง่าย สร้างคำบรรยาย 2-3 ประโยค บอกสิ่งที่เห็นจริง ห้ามเดาสถานที่ บุคคล หรือความสัมพันธ์ ตอบ JSON object เท่านั้น รูปแบบ {\"caption\":\"...\",\"tags\":[\"...\",\"...\"]} ใส่แท็กภาษาไทย 2-5 คำ ไม่ซ้ำกัน ห้ามใส่ markdown หรือข้อความอื่น: " + english
	requestBody, err := json.Marshal(map[string]any{
		"model":      a.model,
		"messages":   []map[string]string{{"role": "user", "content": prompt}},
		"max_tokens": 420, "temperature": 0,
	})
	if err != nil {
		return photokeep.ImageAIResult{}, err
	}
	start := int(a.thai.Add(1)-1) % len(a.thaiKeys)
	var last error
	for offset := range len(a.thaiKeys) {
		key := a.thaiKeys[(start+offset)%len(a.thaiKeys)]
		if key.isCoolingDown() {
			continue
		}
		if err := key.wait(ctx); err != nil {
			return photokeep.ImageAIResult{}, err
		}
		request, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, thaiLLMEndpoint, bytes.NewReader(requestBody))
		if reqErr != nil {
			return photokeep.ImageAIResult{}, reqErr
		}
		request.Header.Set("Authorization", "Bearer "+key.value)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept", "application/json")
		response, doErr := a.http.Do(request)
		if doErr != nil {
			last = &photokeep.ImageAITransportError{Cause: doErr}
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 2<<20))
		retryAfter := response.Header.Get("Retry-After")
		response.Body.Close()
		if readErr != nil {
			last = readErr
			continue
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			last = cloudProviderError(response.StatusCode, response.Header, body, "ThaiLLM")
			if response.StatusCode == http.StatusTooManyRequests {
				key.block(retryAfter)
			}
			if !shouldTryNextProviderKey(last) {
				return photokeep.ImageAIResult{}, last
			}
			continue
		}
		content, err := thaiLLMContent(body)
		if err != nil {
			last = err
			continue
		}
		result, err := parseThaiCaption(content)
		if err == nil {
			return result, nil
		}

		return photokeep.ImageAIResult{}, err
	}
	if last == nil {
		return photokeep.ImageAIResult{}, &photokeep.ImageAIHTTPError{StatusCode: http.StatusTooManyRequests, Message: "ThaiLLM ทุกคีย์ติด rate limit ชั่วคราว", RetryAfter: a.shortestThaiCooldown()}
	}
	return photokeep.ImageAIResult{}, fmt.Errorf("ThaiLLM ทุกคีย์ส่งผลลัพธ์ไม่สำเร็จ: %w", last)
}

func (k *thailandKey) isCoolingDown() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.blockedTil.After(time.Now())
}

func (a *CloudflareThaiLLMAnalyzer) shortestThaiCooldown() time.Duration {
	shortest := 24 * time.Hour
	for _, key := range a.thaiKeys {
		key.mu.Lock()
		wait := time.Until(key.blockedTil)
		key.mu.Unlock()
		if wait <= 0 {
			return 0
		}
		if wait < shortest {
			shortest = wait
		}
	}
	if shortest > 24*time.Hour || shortest <= 0 {
		return 0
	}
	return shortest
}

func cloudflareQuotaRetryAfter() time.Duration {
	location, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		location = time.FixedZone("ICT", 7*60*60)
	}
	now := time.Now().In(location)
	reset := time.Date(now.Year(), now.Month(), now.Day(), 7, 0, 0, 0, location)
	if !reset.After(now) {
		reset = reset.Add(24 * time.Hour)
	}
	return time.Until(reset)
}

func (k *thailandKey) wait(ctx context.Context) error {
	k.mu.Lock()
	start := time.Now()
	if k.next.After(start) {
		start = k.next
	}
	if k.blockedTil.After(start) {
		start = k.blockedTil
	}
	k.next = start.Add(310 * time.Millisecond)
	k.mu.Unlock()
	if delay := time.Until(start); delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return ctx.Err()
}

func (k *thailandKey) block(retryAfter string) {
	delay := parseProviderRetryAfter(retryAfter)
	if delay < 5*time.Second {
		delay = 5 * time.Second
	}
	k.mu.Lock()
	if until := time.Now().Add(delay); until.After(k.blockedTil) {
		k.blockedTil = until
	}
	k.mu.Unlock()
}

func parseProviderRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if seconds, err := time.ParseDuration(value); err == nil && seconds > 0 {
		return seconds
	}
	if seconds, err := time.ParseDuration(value + "s"); err == nil && seconds > 0 {
		return seconds
	}
	if at, err := http.ParseTime(value); err == nil && at.After(time.Now()) {
		return time.Until(at)
	}
	return 0
}

type providerError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func cloudProviderError(status int, header http.Header, body []byte, provider string) error {
	message := strings.TrimSpace(string(body))
	var payload struct {
		Error   string          `json:"error"`
		Message string          `json:"message"`
		Errors  []providerError `json:"errors"`
	}
	if json.Unmarshal(body, &payload) == nil {
		if payload.Error != "" {
			message = payload.Error
		} else if payload.Message != "" {
			message = payload.Message
		} else if len(payload.Errors) > 0 && payload.Errors[0].Message != "" {
			message = payload.Errors[0].Message
		}
	}
	if len(message) > 280 {
		message = message[:280]
	}
	if message == "" {
		message = http.StatusText(status)
	}
	return &photokeep.ImageAIHTTPError{StatusCode: status, Code: providerErrorCode(body), Message: provider + ": " + message, RetryAfter: parseProviderRetryAfter(header.Get("Retry-After"))}
}

func providerErrorCode(body []byte) int {
	var response struct {
		Error struct {
			Code int `json:"code"`
		} `json:"error"`
		Errors []providerError `json:"errors"`
	}
	if json.Unmarshal(body, &response) == nil {
		if response.Error.Code != 0 {
			return response.Error.Code
		}
		if len(response.Errors) > 0 {
			return response.Errors[0].Code
		}
	}
	return 0
}

func shouldTryNextProviderKey(err error) bool {
	var providerErr *photokeep.ImageAIHTTPError
	if !errors.As(err, &providerErr) {
		return errors.As(err, new(*photokeep.ImageAITransportError))
	}
	return providerErr.StatusCode == http.StatusTooManyRequests || providerErr.StatusCode == http.StatusUnauthorized || providerErr.StatusCode == http.StatusForbidden || providerErr.StatusCode >= 500
}

func thaiLLMContent(body []byte) (string, error) {
	var response struct {
		Choices []struct {
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("ThaiLLM ส่ง JSON response ไม่ถูกต้อง: %w", err)
	}
	if len(response.Choices) == 0 {
		return "", errors.New("ThaiLLM ไม่ส่ง choices")
	}
	var content string
	if err := json.Unmarshal(response.Choices[0].Message.Content, &content); err != nil {
		return "", errors.New("ThaiLLM ไม่ส่งข้อความคำตอบ")
	}
	return content, nil
}

func parseThaiCaption(content string) (photokeep.ImageAIResult, error) {
	content = strings.TrimSpace(content)
	if opening := strings.Index(content, "<think>"); opening >= 0 {
		if closing := strings.Index(content[opening:], "</think>"); closing >= 0 {
			content = strings.TrimSpace(content[:opening] + content[opening+closing+len("</think>"):])
		}
	}
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```JSON")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(strings.TrimSpace(content), "```")
	content = strings.TrimSpace(content)
	start, end := strings.Index(content, "{"), strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return photokeep.ImageAIResult{}, errors.New("ThaiLLM ไม่ได้ส่ง JSON object")
	}
	var parsed struct {
		Caption     string   `json:"caption"`
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
	}
	if err := json.Unmarshal([]byte(content[start:end+1]), &parsed); err != nil {
		return photokeep.ImageAIResult{}, fmt.Errorf("ThaiLLM ส่ง JSON ไม่ถูกต้อง: %w", err)
	}
	caption := strings.TrimSpace(parsed.Caption)
	if caption == "" {
		caption = strings.TrimSpace(parsed.Description)
	}
	tags := make([]string, 0, len(parsed.Tags))
	seen := make(map[string]struct{}, len(parsed.Tags))
	for _, tag := range parsed.Tags {
		tag = strings.TrimSpace(strings.Trim(tag, "#，,。.!?;；:：\"'"))
		if tag == "" || !thaiOnlyTag(tag) {
			continue
		}
		key := strings.ToLower(tag)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		tags = append(tags, tag)
	}
	if caption == "" {
		return photokeep.ImageAIResult{}, errors.New("ThaiLLM ส่ง caption ว่าง")
	}
	if len(tags) == 0 {
		return photokeep.ImageAIResult{}, errors.New("ThaiLLM ไม่ส่งแท็กภาษาไทย")
	}
	result := photokeep.ImageAIResult{Caption: caption, Description: caption, Tags: tags}
	if err := photokeep.ValidateThaiAnalysis(result); err != nil {
		return photokeep.ImageAIResult{}, err
	}
	return result, nil
}

func thaiOnlyTag(value string) bool {
	hasThaiLetter := false
	for _, char := range value {
		if unicode.IsLetter(char) && unicode.In(char, unicode.Thai) {
			hasThaiLetter = true
		} else if unicode.IsLetter(char) {
			return false
		}
	}
	return hasThaiLetter
}
