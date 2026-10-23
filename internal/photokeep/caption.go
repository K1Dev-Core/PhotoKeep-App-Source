package photokeep

import (
	"strings"

	"gorm.io/gorm"
)

var imageCaptionOpeners = []string{
	"ในภาพแสดงให้เห็นว่ามี",
	"ในภาพแสดงให้เห็นว่า",
	"ในภาพแสดงให้เห็น",
	"ภาพถ่ายแสดงให้เห็นว่ามี",
	"ภาพถ่ายแสดงให้เห็นว่า",
	"ภาพถ่ายแสดงให้เห็น",
	"ภาพนี้แสดงให้เห็นว่ามี",
	"ภาพนี้แสดงให้เห็นว่า",
	"ภาพนี้แสดงให้เห็น",
	"ภาพแสดงให้เห็นว่ามี",
	"ภาพแสดงให้เห็นว่า",
	"ภาพแสดงให้เห็น",
	"ภาพถ่ายแสดงว่า",
	"ภาพนี้แสดงว่า",
	"ภาพแสดงว่า",
	"ภาพถ่ายแสดง",
	"ภาพนี้แสดง",
	"ภาพแสดง",
}

func normalizeAICaption(value string) string {
	caption := strings.TrimSpace(value)
	for _, opener := range imageCaptionOpeners {
		if !strings.HasPrefix(caption, opener) {
			continue
		}
		direct := strings.TrimLeft(strings.TrimSpace(strings.TrimPrefix(caption, opener)), " :：-–—")
		if direct != "" {
			return direct
		}
	}
	return caption
}

func (s *Service) normalizeExistingAICaptions() error {
	var photos []Photo
	if err := s.db.Select("id", "description").Where("ai_status = ? AND (description LIKE ? OR description LIKE ? OR description LIKE ? OR description LIKE ?)", "completed", "ภาพแสดง%", "ภาพนี้แสดง%", "ภาพถ่ายแสดง%", "ในภาพแสดง%").Find(&photos).Error; err != nil {
		return err
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		for _, photo := range photos {
			caption := normalizeAICaption(photo.Description)
			if caption != photo.Description {
				if err := tx.Model(&Photo{}).Where("id = ?", photo.ID).Update("description", caption).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (s *Service) requeueLegacyTagLanguageFailures() error {
	return s.db.Model(&Photo{}).
		Where("ai_status = ? AND ai_error = ?", "failed", "แท็กบางคำไม่มีข้อความภาษาไทย").
		Updates(map[string]interface{}{"ai_status": "pending", "ai_error": ""}).Error
}
