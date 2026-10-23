package photokeep

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

const (
	maxAnalysisSource   = 100 << 20
	maxVisionImageBytes = 12 << 20
)

func prepareVisionImageWithMaxSide(path string, maxSide int) (uploadPath, filename, contentType string, cleanup func(), err error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", "", "", func() {}, fmt.Errorf("เปิดรูปสำหรับวิเคราะห์ไม่ได้: %w", err)
	}
	if info.Size() > maxAnalysisSource {
		return "", "", "", func() {}, errors.New("ไฟล์ภาพใหญ่เกิน 100 MB สำหรับเตรียมส่ง AI")
	}
	data, err := prepareImageJPEGBytes(path, maxSide)
	if err != nil {
		return "", "", "", func() {}, err
	}
	file, err := os.CreateTemp("", "photokeep-ai-*.jpg")
	if err != nil {
		return "", "", "", func() {}, fmt.Errorf("สร้างไฟล์ภาพชั่วคราวไม่ได้: %w", err)
	}
	name := file.Name()
	cleanup = func() { _ = os.Remove(name) }
	if _, err = file.Write(data); err != nil {
		_ = file.Close()
		cleanup()
		return "", "", "", func() {}, fmt.Errorf("บันทึกภาพที่ย่อแล้วไม่ได้: %w", err)
	}
	if err = file.Close(); err != nil {
		cleanup()
		return "", "", "", func() {}, err
	}
	filename = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)) + ".jpg"
	return name, filename, "image/jpeg", cleanup, nil
}

func prepareImageJPEGBytes(path string, maxSide int) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("เปิดรูปไม่ได้: %w", err)
	}
	defer file.Close()
	img, _, err := image.Decode(io.LimitReader(file, maxAnalysisSource+1))
	if err != nil {
		return nil, fmt.Errorf("อ่านรูปเพื่อเตรียมส่ง AI ไม่ได้: %w", err)
	}
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width < 1 || height < 1 {
		return nil, errors.New("รูปภาพไม่มีขนาดที่ถูกต้อง")
	}
	if maxSide < 1 {
		maxSide = 768
	}
	if width > maxSide || height > maxSide {
		scale := float64(maxSide) / float64(max(width, height))
		newWidth := max(1, int(float64(width)*scale))
		newHeight := max(1, int(float64(height)*scale))
		dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
		draw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)
		img = dst
	}
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, img, &jpeg.Options{Quality: 82}); err != nil {
		return nil, fmt.Errorf("แปลงภาพเป็น JPEG ไม่ได้: %w", err)
	}
	return buffer.Bytes(), nil
}
