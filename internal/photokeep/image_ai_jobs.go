package photokeep

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

const maxImageAIAnalysisPhotos = 1000

func cleanAITags(incoming []string) string {
	tags := make([]string, 0, len(incoming))
	seen := make(map[string]struct{}, len(incoming))
	add := func(value string) {
		value = strings.TrimSpace(strings.Trim(value, ","))
		if value == "" {
			return
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		tags = append(tags, value)
	}
	for _, tag := range incoming {
		add(tag)
	}
	return strings.Join(tags, ",")
}

type ImageAIInput struct {
	PhotoID  uint
	Filename string
	Path     string
}

func (s *Service) PendingImageAIInputs() ([]ImageAIInput, error) {
	var photos []Photo
	if err := s.db.Select("id", "filename", "backup_path").Where("ai_status = '' OR ai_status = ?", "pending").Order("backed_up_at desc, id desc").Find(&photos).Error; err != nil {
		return nil, err
	}
	inputs := make([]ImageAIInput, 0, len(photos))
	for _, photo := range photos {
		inputs = append(inputs, ImageAIInput{PhotoID: photo.ID, Filename: photo.Filename, Path: photo.BackupPath})
	}
	return inputs, nil
}

func (s *Service) SetImageAIStatus(photoIDs []uint, status, message string) error {
	if len(photoIDs) == 0 {
		return nil
	}
	return s.db.Model(&Photo{}).Where("id IN ?", photoIDs).Updates(map[string]interface{}{"ai_status": status, "ai_error": message, "ai_updated_at": time.Now()}).Error
}

func (s *Service) RequeueFailedImageAIAnalysis(photoIDs []uint) (int64, error) {
	if len(photoIDs) == 0 {
		return 0, nil
	}
	result := s.db.Model(&Photo{}).Where("id IN ? AND ai_status = ?", photoIDs, "failed").Updates(map[string]interface{}{
		"ai_status": "pending", "ai_error": "", "ai_updated_at": time.Now(),
	})
	return result.RowsAffected, result.Error
}

func (s *Service) ImageAIQueueProgress() (ImageAIQueueProgress, error) {
	var progress ImageAIQueueProgress
	progress.Total = 0
	if err := s.db.Model(&Photo{}).Count(&progress.Total).Error; err != nil {
		return progress, err
	}
	if err := s.db.Model(&Photo{}).Where("ai_status = ?", "completed").Count(&progress.Completed).Error; err != nil {
		return progress, err
	}
	if err := s.db.Model(&Photo{}).Where("ai_status = ?", "pending").Count(&progress.Pending).Error; err != nil {
		return progress, err
	}
	var unanalyzed int64
	if err := s.db.Model(&Photo{}).Where("ai_status = ''").Count(&unanalyzed).Error; err != nil {
		return progress, err
	}
	progress.Pending += unanalyzed
	if err := s.db.Model(&Photo{}).Where("ai_status = ?", "analyzing").Count(&progress.Analyzing).Error; err != nil {
		return progress, err
	}
	if err := s.db.Model(&Photo{}).Where("ai_status = ?", "failed").Count(&progress.Failed).Error; err != nil {
		return progress, err
	}
	var latestJob ImageAIAnalysisJob
	latestResult := s.db.Order("id desc").Limit(1).Find(&latestJob)
	if err := latestResult.Error; err == nil && latestResult.RowsAffected > 0 {
		progress.Paused = latestJob.Status == "paused"
		end := latestJob.UpdatedAt
		if !latestJob.Finished {
			end = time.Now()
		}
		if end.After(latestJob.CreatedAt) {
			progress.ElapsedSeconds = int64(end.Sub(latestJob.CreatedAt).Seconds())
		}
	} else if err != nil {
		return progress, err
	}
	return progress, nil
}

func (s *Service) PauseImageAIAnalysisJob(id uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var job ImageAIAnalysisJob
		if err := tx.First(&job, id).Error; err != nil {
			return err
		}
		if job.Finished {
			return nil
		}
		var photoIDs []uint
		if err := json.Unmarshal([]byte(job.PhotoIDs), &photoIDs); err != nil {
			return err
		}
		if len(photoIDs) > 0 {
			if err := tx.Model(&Photo{}).Where("id IN ? AND ai_status IN ?", photoIDs, []string{"pending", "analyzing"}).Updates(map[string]interface{}{"ai_status": "pending", "ai_error": "", "ai_updated_at": time.Now()}).Error; err != nil {
				return err
			}
		}
		job.Status = "paused"
		job.Finished = true
		job.Error = "หยุดการวิเคราะห์โดยผู้ใช้ สามารถกดทำต่อได้"
		job.DurationSeconds = durationSecondsSince(job.CreatedAt)
		return tx.Save(&job).Error
	})
}

func (s *Service) ListImageAIErrorLogs(limit int) ([]ImageAIErrorLog, error) {
	if limit < 1 || limit > 500 {
		limit = 200
	}
	var photos []Photo
	if err := s.db.Select("id", "filename", "ai_status", "ai_error", "ai_updated_at").Where("ai_status = ? AND ai_error <> ''", "failed").Order("ai_updated_at desc, id desc").Limit(limit).Find(&photos).Error; err != nil {
		return nil, err
	}
	logs := make([]ImageAIErrorLog, 0, len(photos))
	for _, photo := range photos {
		if photo.AIStatus == "failed" && photo.AIError != "" {
			logs = append(logs, ImageAIErrorLog{PhotoID: photo.ID, Filename: photo.Filename, Kind: "คำอธิบายภาพ", Error: photo.AIError, LoggedAt: photo.AIUpdatedAt})
		}
	}
	sort.Slice(logs, func(i, j int) bool { return logs[i].LoggedAt.After(logs[j].LoggedAt) })
	if len(logs) > limit {
		logs = logs[:limit]
	}
	return logs, nil
}

func (s *Service) PrepareImageAIInputs(photoIDs []uint) ([]ImageAIInput, error) {
	ids := make([]uint, 0, len(photoIDs))
	seen := make(map[uint]struct{}, len(photoIDs))
	for _, id := range photoIDs {
		if id == 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, errors.New("กรุณาเลือกรูปภาพก่อนวิเคราะห์")
	}
	if len(ids) > maxImageAIAnalysisPhotos {
		return nil, fmt.Errorf("เลือกได้สูงสุด %d รูปต่อหนึ่งงาน", maxImageAIAnalysisPhotos)
	}
	var photos []Photo
	if err := s.db.Select("id", "filename", "backup_path").Where("id IN ?", ids).Find(&photos).Error; err != nil {
		return nil, err
	}
	byID := make(map[uint]Photo, len(photos))
	for _, photo := range photos {
		byID[photo.ID] = photo
	}
	inputs := make([]ImageAIInput, 0, len(ids))
	for _, id := range ids {
		photo, exists := byID[id]
		if !exists {
			return nil, fmt.Errorf("ไม่พบรูปภาพ ID %d", id)
		}
		inputs = append(inputs, ImageAIInput{PhotoID: photo.ID, Filename: photo.Filename, Path: photo.BackupPath})
	}
	return inputs, nil
}

func (s *Service) CreateImageAIAnalysisJob(inputs []ImageAIInput) (ImageAIAnalysisJob, error) {
	if len(inputs) == 0 {
		return ImageAIAnalysisJob{}, errors.New("ไม่มีรูปภาพสำหรับวิเคราะห์")
	}
	var active int64
	if err := s.db.Model(&ImageAIAnalysisJob{}).Where("finished = ?", false).Count(&active).Error; err != nil {
		return ImageAIAnalysisJob{}, err
	}
	if active > 0 {
		return ImageAIAnalysisJob{}, errors.New("รอให้งานวิเคราะห์ภาพชุดปัจจุบันเสร็จก่อน")
	}
	ids := make([]uint, len(inputs))
	for index, input := range inputs {
		ids[index] = input.PhotoID
	}
	encoded, err := json.Marshal(ids)
	if err != nil {
		return ImageAIAnalysisJob{}, err
	}
	job := ImageAIAnalysisJob{PhotoIDs: string(encoded), Status: "running", Total: len(inputs)}
	if err := s.db.Create(&job).Error; err != nil {
		return job, err
	}
	return job, nil
}

func (s *Service) ImageAIAnalysisJob(id uint) (ImageAIAnalysisJob, error) {
	var job ImageAIAnalysisJob
	err := s.db.First(&job, id).Error
	return job, err
}

func (s *Service) ListImageAIAnalysisJobs() ([]ImageAIAnalysisJob, error) {
	var jobs []ImageAIAnalysisJob
	err := s.db.Order("id desc").Limit(10).Find(&jobs).Error
	return jobs, err
}

func (s *Service) RecordImageAIResult(jobID uint, input ImageAIInput, result ImageAIResult, analysisErr error) (ImageAIAnalysisJob, error) {
	var job ImageAIAnalysisJob
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&job, jobID).Error; err != nil {
			return err
		}
		if job.Finished {
			return nil
		}
		var photo Photo
		if err := tx.First(&photo, input.PhotoID).Error; err != nil {
			analysisErr = err
		}
		if analysisErr == nil {
			caption := strings.TrimSpace(result.Description)
			if caption == "" {
				caption = strings.TrimSpace(result.Caption)
			}
			if caption == "" {
				analysisErr = errors.New("AI ไม่ได้ส่งคำอธิบายภาพกลับมา")
			} else {
				caption = normalizeAICaption(caption)
				if len([]rune(caption)) > 500 {
					caption = string([]rune(caption)[:500])
				}
				tags := cleanAITags(result.Tags)
				updates := map[string]interface{}{"description": caption, "tags": tags, "ai_status": "completed", "ai_error": "", "ai_updated_at": time.Now()}
				if err := tx.Model(&photo).Updates(updates).Error; err != nil {
					analysisErr = err
				}
			}
		}
		job.Completed++
		if analysisErr != nil {
			message := strings.TrimSpace(analysisErr.Error())
			if len([]rune(message)) > 240 {
				message = string([]rune(message)[:240])
			}
			if IsRetryableImageAIError(analysisErr) {

				_ = tx.Model(&Photo{}).Where("id = ?", input.PhotoID).Updates(map[string]interface{}{"ai_status": "pending", "ai_error": message, "ai_updated_at": time.Now()}).Error
			} else {
				_ = tx.Model(&Photo{}).Where("id = ?", input.PhotoID).Updates(map[string]interface{}{"ai_status": "failed", "ai_error": message, "ai_updated_at": time.Now()}).Error
				job.Failed++
			}
			if job.Failed <= 8 {
				job.Error = strings.TrimSpace(strings.Join([]string{job.Error, input.Filename + ": " + message}, "\n"))
			}
		}
		job.Status = "running"
		if job.Completed >= job.Total {
			job.Status = "completed"
			job.Finished = true
			job.DurationSeconds = durationSecondsSince(job.CreatedAt)
		}
		return tx.Save(&job).Error
	})
	if err == nil {
		s.invalidateTagCloud()
	}
	return job, err
}

func (s *Service) MarkRunningImageAIJobsInterrupted() error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Photo{}).Where("ai_status = ?", "analyzing").Updates(map[string]interface{}{"ai_status": "pending", "ai_error": ""}).Error; err != nil {
			return err
		}
		var jobs []ImageAIAnalysisJob
		if err := tx.Where("finished = ?", false).Find(&jobs).Error; err != nil {
			return err
		}
		for index := range jobs {
			jobs[index].Status = "interrupted"
			jobs[index].Finished = true
			jobs[index].Error = "แอปปิดก่อนงานเสร็จ ระบบจะทำรายการที่ค้างต่อเมื่อเปิดแอป"
			jobs[index].DurationSeconds = durationSecondsSince(jobs[index].CreatedAt)
			if err := tx.Save(&jobs[index]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) FailImageAIAnalysisJob(id uint, message string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var job ImageAIAnalysisJob
		if err := tx.Where("id = ? AND finished = ?", id, false).First(&job).Error; err != nil {
			return err
		}
		job.Status = "failed"
		job.Finished = true
		job.Error = strings.TrimSpace(message)
		job.DurationSeconds = durationSecondsSince(job.CreatedAt)
		return tx.Save(&job).Error
	})
}

func (s *Service) DeferImageAIAnalysisJob(id uint, message string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var job ImageAIAnalysisJob
		if err := tx.Where("id = ? AND finished = ?", id, false).First(&job).Error; err != nil {
			return err
		}
		job.Status = "waiting"
		job.Finished = true
		job.Error = strings.TrimSpace(message)
		job.DurationSeconds = durationSecondsSince(job.CreatedAt)
		return tx.Save(&job).Error
	})
}

func durationSecondsSince(start time.Time) int64 {
	seconds := int64(time.Since(start).Seconds())
	if seconds < 0 {
		return 0
	}
	return seconds
}
