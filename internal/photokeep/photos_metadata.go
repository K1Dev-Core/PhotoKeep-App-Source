package photokeep

import (
	"errors"
	"strings"
)

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
	a.invalidateTagCloud()
	return p, nil
}
