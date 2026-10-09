package photokeep

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"golang.org/x/image/draw"
	"image"
	"image/jpeg"
	"os"
	"sort"
	"strings"
)

func (a *Service) Search(query, tag string) (SearchResult, error) {
	var photos []Photo
	db := a.db.Model(&Photo{})
	if q := strings.TrimSpace(query); q != "" {
		db = db.Where("LOWER(filename) LIKE ? OR LOWER(description) LIKE ?", "%"+strings.ToLower(q)+"%", "%"+strings.ToLower(q)+"%")
	}
	if tag = strings.TrimSpace(tag); tag != "" {
		db = db.Where("(',' || LOWER(tags) || ',') LIKE ?", "%,"+strings.ToLower(tag)+",%")
	}
	if err := db.Order("backed_up_at desc, id desc").Find(&photos).Error; err != nil {
		return SearchResult{}, err
	}
	tags, err := a.tagCloud()
	if err != nil {
		return SearchResult{}, err
	}
	return SearchResult{Photos: photos, Total: int64(len(photos)), Tags: tags}, nil
}

func (a *Service) tagCloud() ([]TagCount, error) {
	var rows []Photo
	if err := a.db.Select("tags").Find(&rows).Error; err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, row := range rows {
		for _, t := range strings.Split(row.Tags, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				counts[t]++
			}
		}
	}
	out := make([]TagCount, 0, len(counts))
	for name, count := range counts {
		out = append(out, TagCount{name, count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Name < out[j].Name
		}
		return out[i].Count > out[j].Count
	})
	return out, nil
}

func (a *Service) UpdatePhoto(id uint, description, tags string) (Photo, error) {
	var p Photo
	if err := a.db.First(&p, id).Error; err != nil {
		return p, err
	}
	description = strings.TrimSpace(description)
	if len([]rune(description)) > 500 {
		return p, errors.New("คำอธิบายยาวเกิน 500 ตัวอักษร")
	}
	unique := map[string]bool{}
	clean := []string{}
	for _, tag := range strings.Split(tags, ",") {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if len([]rune(tag)) > 40 {
			return p, errors.New("แท็กหนึ่งคำยาวเกิน 40 ตัวอักษร")
		}
		key := strings.ToLower(tag)
		if !unique[key] {
			unique[key] = true
			clean = append(clean, tag)
		}
	}
	if len(clean) > 20 {
		return p, errors.New("ใส่แท็กได้สูงสุด 20 คำ")
	}
	p.Description = description
	p.Tags = strings.Join(clean, ",")
	if err := a.db.Model(&p).Updates(map[string]interface{}{"description": p.Description, "tags": p.Tags}).Error; err != nil {
		return p, err
	}
	return p, nil
}

func (a *Service) Preview(id uint, thumbnail bool) (string, error) {
	var p Photo
	if err := a.db.First(&p, id).Error; err != nil {
		return "", err
	}
	f, err := os.Open(p.BackupPath)
	if err != nil {
		return "", fmt.Errorf("เปิดรูปจากไฟล์สำรองไม่ได้: %w", err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return "", err
	}
	max := 1600
	if thumbnail {
		max = 420
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w > max || h > max {
		scale := float64(max) / float64(maxInt(w, h))
		nw, nh := maxInt(1, int(float64(w)*scale)), maxInt(1, int(float64(h)*scale))
		dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
		draw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)
		img = dst
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 82}); err != nil {
		return "", err
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
