package gateway

import (
	"context"
	"encoding/json"
	"errors"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"photokeep/internal/photokeep"
)

const maxUploadBytes = 12 << 20

type Analyzer interface {
	Health(context.Context) error
	Analyze(context.Context, string) (photokeep.ImageAIResult, error)
}

type Server struct {
	analyzer    Analyzer
	token       string
	concurrency int
	slots       chan struct{}
}

func New(analyzer Analyzer, token string, concurrency int) *Server {
	if concurrency < 1 {
		concurrency = 1
	}
	return &Server{analyzer: analyzer, token: strings.TrimSpace(token), concurrency: concurrency, slots: make(chan struct{}, concurrency)}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("POST /v1/analyze", s.analyze)
	return mux
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := s.analyzer.Health(ctx); err != nil {
		respond(w, http.StatusServiceUnavailable, map[string]any{"success": false, "error": "AI ยังไม่พร้อม"})
		return
	}
	respond(w, http.StatusOK, map[string]any{"success": true, "status": "ok", "concurrency": s.concurrency})
}

func (s *Server) analyze(w http.ResponseWriter, r *http.Request) {

	if s.token == "" && r.Header.Get("Origin") != "" {
		respond(w, http.StatusForbidden, map[string]any{"success": false, "error": "ไม่รับคำขอจากเว็บไซต์"})
		return
	}
	if s.token != "" && r.Header.Get("x-api-key") != s.token {
		respond(w, http.StatusUnauthorized, map[string]any{"success": false, "error": "ไม่ได้รับอนุญาต"})
		return
	}
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-r.Context().Done():
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		respond(w, http.StatusBadRequest, map[string]any{"success": false, "error": "อ่านไฟล์ภาพไม่ได้หรือไฟล์ใหญ่เกิน 12 MB"})
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		respond(w, http.StatusBadRequest, map[string]any{"success": false, "error": "ไม่พบไฟล์ภาพในช่อง file"})
		return
	}
	defer file.Close()
	if header.Size > maxUploadBytes {
		respond(w, http.StatusRequestEntityTooLarge, map[string]any{"success": false, "error": "ไฟล์ใหญ่เกิน 12 MB"})
		return
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".gif" && ext != ".webp" && ext != ".bmp" {
		respond(w, http.StatusUnsupportedMediaType, map[string]any{"success": false, "error": "รองรับเฉพาะไฟล์ภาพ"})
		return
	}
	tmp, err := os.CreateTemp("", "photokeep-ai-*"+ext)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]any{"success": false, "error": "เตรียมไฟล์ภาพไม่ได้"})
		return
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	n, copyErr := io.Copy(tmp, io.LimitReader(file, maxUploadBytes+1))
	if copyErr != nil {
		respond(w, http.StatusBadRequest, map[string]any{"success": false, "error": "อ่านไฟล์ภาพไม่ได้"})
		return
	}
	if n > maxUploadBytes {
		respond(w, http.StatusRequestEntityTooLarge, map[string]any{"success": false, "error": "ไฟล์ใหญ่เกิน 12 MB"})
		return
	}
	if _, err = tmp.Seek(0, io.SeekStart); err != nil {
		respond(w, http.StatusInternalServerError, map[string]any{"success": false, "error": "ตรวจไฟล์ภาพไม่ได้"})
		return
	}
	if _, _, err = image.DecodeConfig(tmp); err != nil {
		respond(w, http.StatusUnsupportedMediaType, map[string]any{"success": false, "error": "ไฟล์ไม่ใช่ภาพที่อ่านได้"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 100*time.Second)
	defer cancel()
	result, err := s.analyzer.Analyze(ctx, tmp.Name())
	if err != nil {
		log.Printf("image analysis failed: %v", err)
		status := http.StatusBadGateway
		var providerErr *photokeep.ImageAIHTTPError
		if errors.As(err, &providerErr) && providerErr.StatusCode == http.StatusTooManyRequests {
			status = http.StatusTooManyRequests
			if providerErr.RetryAfter > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(int((providerErr.RetryAfter+time.Second-1)/time.Second)))
			}
		}
		if photokeep.IsRetryableImageAIError(err) || errors.Is(err, context.DeadlineExceeded) {
			if status != http.StatusTooManyRequests {
				status = http.StatusServiceUnavailable
			}
		}
		respond(w, status, map[string]any{"success": false, "error": "วิเคราะห์ภาพไม่สำเร็จ กรุณาลองใหม่"})
		return
	}
	result.Filename = filepath.Base(header.Filename)
	success := true
	result.Success = &success
	respond(w, http.StatusOK, result)
}

func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
