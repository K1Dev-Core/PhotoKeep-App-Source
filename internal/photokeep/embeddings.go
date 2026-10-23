package photokeep

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"strings"
	"time"
)

type EmbeddingVector struct {
	Values []float32
	Model  string
}

type ManagedEmbeddingClient struct {
	base string
	http *http.Client
}

func NewManagedEmbeddingClient(analyzeEndpoint string) (*ManagedEmbeddingClient, error) {
	analyzeEndpoint = strings.TrimSpace(analyzeEndpoint)
	if !strings.HasSuffix(analyzeEndpoint, "/v1/analyze") {
		return nil, errors.New("URL API กลางต้องลงท้ายด้วย /v1/analyze")
	}
	base := strings.TrimSuffix(analyzeEndpoint, "/v1/analyze")
	return &ManagedEmbeddingClient{base: base, http: &http.Client{Timeout: 90 * time.Second}}, nil
}

func (c *ManagedEmbeddingClient) Concurrency(ctx context.Context) (int, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/health", nil)
	if err != nil {
		return 0, err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	var status struct {
		Success bool `json:"success"`
		Embed   struct {
			Ready       bool `json:"ready"`
			Concurrency int  `json:"concurrency"`
		} `json:"embed"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&status); err != nil {
		return 0, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || !status.Success || !status.Embed.Ready {
		return 0, errors.New("embedding API ยังไม่พร้อม")
	}
	if status.Embed.Concurrency < 1 {
		return 1, nil
	}
	return status.Embed.Concurrency, nil
}

func (c *ManagedEmbeddingClient) EmbedText(ctx context.Context, text string) (EmbeddingVector, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return EmbeddingVector{}, errors.New("คำค้นว่าง")
	}
	body, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return EmbeddingVector{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/v1/embed/text", bytes.NewReader(body))
	if err != nil {
		return EmbeddingVector{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	return c.send(request)
}

func (c *ManagedEmbeddingClient) EmbedImage(ctx context.Context, path string) (EmbeddingVector, error) {
	uploadPath, filename, contentType, cleanup, err := prepareVisionImageWithMaxSide(path, imageAIAnalysisMaxSide)
	if err != nil {
		return EmbeddingVector{}, err
	}
	defer cleanup()
	file, err := os.Open(uploadPath)
	if err != nil {
		return EmbeddingVector{}, err
	}
	defer file.Close()
	reader, writer := io.Pipe()
	form := multipart.NewWriter(writer)
	writeErr := make(chan error, 1)
	go func() {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": filename}))
		header.Set("Content-Type", contentType)
		part, createErr := form.CreatePart(header)
		if createErr == nil {
			_, createErr = io.Copy(part, file)
		}
		if closeErr := form.Close(); createErr == nil {
			createErr = closeErr
		}
		_ = writer.CloseWithError(createErr)
		writeErr <- createErr
	}()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/v1/embed/image", reader)
	if err != nil {
		_ = reader.CloseWithError(err)
		_ = writer.CloseWithError(err)
		return EmbeddingVector{}, err
	}
	request.Header.Set("Content-Type", form.FormDataContentType())
	result, requestErr := c.send(request)
	if requestErr != nil {
		_ = reader.CloseWithError(requestErr)
	}
	streamErr := <-writeErr
	if requestErr != nil {
		return EmbeddingVector{}, requestErr
	}
	if streamErr != nil {
		return EmbeddingVector{}, fmt.Errorf("ส่งภาพไปสร้าง embedding ไม่สำเร็จ: %w", streamErr)
	}
	return result, nil
}

func (c *ManagedEmbeddingClient) send(request *http.Request) (EmbeddingVector, error) {
	response, err := c.http.Do(request)
	if err != nil {
		return EmbeddingVector{}, fmt.Errorf("เชื่อมต่อ embedding API ไม่สำเร็จ: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if err != nil {
		return EmbeddingVector{}, err
	}
	var result struct {
		Success   bool      `json:"success"`
		Embedding []float32 `json:"embedding"`
		Dim       int       `json:"dim"`
		Model     string    `json:"model"`
		Error     string    `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return EmbeddingVector{}, fmt.Errorf("embedding API ส่ง JSON ไม่ถูกต้อง (HTTP %d): %w", response.StatusCode, err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || !result.Success {
		message := strings.TrimSpace(result.Error)
		if message == "" {
			message = response.Status
		}
		return EmbeddingVector{}, fmt.Errorf("embedding API HTTP %d: %s", response.StatusCode, message)
	}
	if len(result.Embedding) == 0 || result.Dim != len(result.Embedding) || strings.TrimSpace(result.Model) == "" {
		return EmbeddingVector{}, errors.New("embedding API ส่งมิติหรือชื่อโมเดลไม่ถูกต้อง")
	}
	return EmbeddingVector{Values: result.Embedding, Model: strings.TrimSpace(result.Model)}, nil
}

func encodeEmbedding(values []float32) []byte {
	encoded := make([]byte, len(values)*4)
	for i, value := range values {
		binary.LittleEndian.PutUint32(encoded[i*4:], math.Float32bits(value))
	}
	return encoded
}

func decodeEmbedding(encoded []byte) []float32 {
	if len(encoded) == 0 || len(encoded)%4 != 0 {
		return nil
	}
	values := make([]float32, len(encoded)/4)
	for i := range values {
		values[i] = math.Float32frombits(binary.LittleEndian.Uint32(encoded[i*4:]))
	}
	return values
}

type PhotoEmbeddingInput struct {
	ID   uint
	Path string
}

func (s *Service) PendingPhotoEmbeddings(limit int) ([]PhotoEmbeddingInput, error) {
	if limit < 1 || limit > 1000 {
		limit = 1000
	}
	var photos []Photo
	err := s.db.Select("id", "backup_path").Where("backup_path <> '' AND (embedding IS NULL OR length(embedding) = 0) AND embedding_status IN ?", []string{"", "pending", "indexing", "failed"}).Order("backed_up_at DESC, id DESC").Limit(limit).Find(&photos).Error
	if err != nil {
		return nil, err
	}
	inputs := make([]PhotoEmbeddingInput, 0, len(photos))
	for _, photo := range photos {
		inputs = append(inputs, PhotoEmbeddingInput{ID: photo.ID, Path: photo.BackupPath})
	}
	return inputs, nil
}

func (s *Service) SetPhotoEmbeddingStatus(id uint, status string) error {
	return s.db.Model(&Photo{}).Where("id = ?", id).Update("embedding_status", status).Error
}

func (s *Service) HasPhotoEmbeddings() (bool, error) {
	var count int64
	err := s.db.Model(&Photo{}).Where("embedding_status = ? AND embedding IS NOT NULL AND length(embedding) > 0", "ready").Count(&count).Error
	return count > 0, err
}

func (s *Service) SavePhotoEmbedding(id uint, vector EmbeddingVector) error {
	return s.db.Model(&Photo{}).Where("id = ?", id).Updates(map[string]interface{}{
		"embedding":        encodeEmbedding(vector.Values),
		"embedding_model":  vector.Model,
		"embedding_status": "ready",
	}).Error
}

func (s *Service) SemanticPhotoScores(vector []float32, model, tag string) ([]Photo, []float64, error) {
	var photos []Photo
	db := s.db.Select("id", "source_path", "backup_path", "moved_original", "filename", "relative_path", "description", "tags", "embedding", "embedding_model", "embedding_status", "ai_status", "ai_error", "ai_updated_at", "sha256", "size", "width", "height", "taken_at", "backed_up_at").Where("embedding_status = ? AND embedding_model = ?", "ready", model)
	if tag = strings.TrimSpace(tag); tag != "" {
		db = db.Where("(',' || LOWER(tags) || ',') LIKE ?", "%,"+strings.ToLower(tag)+",%")
	}
	if err := db.Find(&photos).Error; err != nil {
		return nil, nil, err
	}
	matched := make([]Photo, 0, len(photos))
	scores := make([]float64, 0, len(photos))
	for _, photo := range photos {
		imageVector := decodeEmbedding(photo.Embedding)
		if len(imageVector) != len(vector) {
			continue
		}
		score := cosineSimilarity(vector, imageVector)
		photo.Embedding = nil
		matched = append(matched, photo)
		scores = append(scores, score)
	}
	return matched, scores, nil
}

func cosineSimilarity(left, right []float32) float64 {
	if len(left) == 0 || len(left) != len(right) {
		return -1
	}
	var dot, leftNorm, rightNorm float64
	for i := range left {
		a, b := float64(left[i]), float64(right[i])
		dot += a * b
		leftNorm += a * a
		rightNorm += b * b
	}
	if leftNorm == 0 || rightNorm == 0 {
		return -1
	}
	return dot / math.Sqrt(leftNorm*rightNorm)
}
