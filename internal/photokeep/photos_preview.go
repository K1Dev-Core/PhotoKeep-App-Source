package photokeep

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/gen2brain/jpegn"
	"golang.org/x/image/draw"
	"image"
	"image/jpeg"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

func (a *Service) Preview(id uint, thumbnail bool) (string, error) {
	var p Photo
	if err := a.db.First(&p, id).Error; err != nil {
		return "", err
	}
	return a.previewPhoto(p, thumbnail)
}

func (a *Service) PreviewThumbnails(ids []uint) ([]ThumbnailPreview, error) {
	if len(ids) > 32 {
		return nil, errors.New("ขอ thumbnail ได้ครั้งละไม่เกิน 32 รูป")
	}
	results := make([]ThumbnailPreview, len(ids))
	if len(ids) == 0 {
		return results, nil
	}
	uniqueIDs := make([]uint, 0, len(ids))
	seen := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			uniqueIDs = append(uniqueIDs, id)
		}
	}
	var photos []Photo
	if err := a.db.Select("id", "backup_path", "sha256").Where("id IN ?", uniqueIDs).Find(&photos).Error; err != nil {
		return nil, err
	}
	byID := make(map[uint]Photo, len(photos))
	for _, photo := range photos {
		byID[photo.ID] = photo
	}
	type workItem struct {
		index int
		photo Photo
	}
	jobs := make(chan workItem)
	workers := minInt(len(ids), maxInt(1, minInt(8, runtime.GOMAXPROCS(0))))
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range jobs {
				result := ThumbnailPreview{PhotoID: ids[item.index]}
				preview, err := a.previewPhoto(item.photo, true)
				if err != nil {
					result.Error = err.Error()
				} else {
					result.DataURL = preview
				}
				results[item.index] = result
			}
		}()
	}
	for index, id := range ids {
		photo, exists := byID[id]
		if !exists {
			results[index] = ThumbnailPreview{PhotoID: id, Error: "ไม่พบรูปภาพ"}
			continue
		}
		jobs <- workItem{index: index, photo: photo}
	}
	close(jobs)
	wg.Wait()
	return results, nil
}

func (a *Service) previewPhoto(p Photo, thumbnail bool) (string, error) {
	if !thumbnail {
		data, err := os.ReadFile(p.BackupPath)
		if err != nil {
			return "", fmt.Errorf("เปิดรูปจากไฟล์สำรองไม่ได้: %w", err)
		}
		mimeType := http.DetectContentType(data)
		return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
	}
	cachePath := ""
	if thumbnail && a.thumbnailCacheDir != "" {
		cachePath = a.thumbnailPath(p)
		if cached, err := os.ReadFile(cachePath); err == nil {
			return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(cached), nil
		}
	}
	f, err := os.Open(p.BackupPath)
	if err != nil {
		return "", fmt.Errorf("เปิดรูปจากไฟล์สำรองไม่ได้: %w", err)
	}
	defer f.Close()
	max := 256
	var img image.Image
	if thumbnail {
		config, format, configErr := image.DecodeConfig(f)
		if configErr == nil && format == "jpeg" {
			denom := jpegThumbnailScale(config.Width, config.Height, max)
			if _, seekErr := f.Seek(0, 0); seekErr == nil {
				decoded, decodeErr := jpegn.Decode(f, &jpegn.Options{ScaleDenom: denom, AutoRotate: true})
				if decodeErr == nil {
					img = decoded
				}
			}
		}
	}
	if img == nil {
		if _, seekErr := f.Seek(0, 0); seekErr != nil {
			return "", seekErr
		}
		img, _, err = image.Decode(f)
	}
	if err != nil {
		return "", err
	}
	data, err := a.encodeThumbnail(p, img)
	if err != nil {
		return "", err
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data), nil
}

func (a *Service) encodeThumbnail(photo Photo, img image.Image) ([]byte, error) {
	const maxSize = 256
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w > maxSize || h > maxSize {
		scale := float64(maxSize) / float64(maxInt(w, h))
		nw, nh := maxInt(1, int(float64(w)*scale)), maxInt(1, int(float64(h)*scale))
		dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
		draw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)
		img = dst
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 68}); err != nil {
		return nil, err
	}
	data := buf.Bytes()
	if a.thumbnailCacheDir != "" {
		if err := os.MkdirAll(a.thumbnailCacheDir, 0755); err == nil {
			if cache, err := os.CreateTemp(a.thumbnailCacheDir, "thumbnail-*.tmp"); err == nil {
				name := cache.Name()
				if _, writeErr := cache.Write(data); writeErr == nil {
					if closeErr := cache.Close(); closeErr == nil {
						if err := os.Rename(name, a.thumbnailPath(photo)); err != nil {
							_ = os.Remove(name)
						}
					} else {
						_ = os.Remove(name)
					}
				} else {
					_ = cache.Close()
					_ = os.Remove(name)
				}
			}
		}
	}
	return data, nil
}

func (a *Service) thumbnailPath(photo Photo) string {
	key := fmt.Sprintf("thumb-256-q68-v3:%d:%s:%s", photo.ID, photo.SHA256, photo.BackupPath)
	digest := sha256.Sum256([]byte(key))
	return filepath.Join(a.thumbnailCacheDir, hex.EncodeToString(digest[:])+".jpg")
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func jpegThumbnailScale(width, height, maxSize int) int {
	longest := maxInt(width, height)
	switch {
	case longest >= maxSize*8:
		return 8
	case longest >= maxSize*4:
		return 4
	case longest >= maxSize*2:
		return 2
	default:
		return 1
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
