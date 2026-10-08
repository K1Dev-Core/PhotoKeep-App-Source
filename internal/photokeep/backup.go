package photokeep

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
	"gorm.io/gorm"
	"image"
	_ "image/gif"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var imageExtensions = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".bmp": true, ".tif": true, ".tiff": true}

func (a *Service) Backup(onProgress func(BackupProgress)) (BackupResult, error) {
	start := time.Now()
	state, err := a.GetState()
	if err != nil {
		return BackupResult{}, err
	}
	if state.Source == "" || state.Dest == "" {
		return BackupResult{}, errors.New("ตั้งค่าโฟลเดอร์ก่อนสำรองรูปภาพ")
	}
	if nested(state.Source, state.Dest) || nested(state.Dest, state.Source) {
		return BackupResult{}, errors.New("โฟลเดอร์ต้นทางกับปลายทางซ้อนกัน")
	}
	if info, e := os.Stat(state.Source); e != nil || !info.IsDir() {
		return BackupResult{}, errors.New("ไม่พบโฟลเดอร์ต้นทาง กรุณาเชื่อมต่ออุปกรณ์")
	}
	result := BackupResult{Errors: []string{}}
	root := filepath.Join(state.Dest, "PhotoKeep", filepath.Base(state.Source))
	type backupFile struct {
		path string
		rel  string
	}
	files := []backupFile{}
	emit := func(stage, current string, completed int) {
		if onProgress != nil {
			onProgress(BackupProgress{
				Stage: stage, Total: len(files), Completed: completed,
				Added: result.Added, Skipped: result.Skipped, Failed: result.Failed,
				CurrentFile: current,
			})
		}
	}
	emit("scanning", "", 0)
	lastScanUpdate := time.Now()
	err = filepath.WalkDir(state.Source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			result.Failed++
			result.Errors = append(result.Errors, walkErr.Error())
			return nil
		}
		if entry.IsDir() || !entry.Type().IsRegular() || !imageExtensions[strings.ToLower(filepath.Ext(entry.Name()))] {
			return nil
		}
		rel, e := filepath.Rel(state.Source, path)
		if e != nil {
			return e
		}
		files = append(files, backupFile{path: path, rel: rel})
		if time.Since(lastScanUpdate) >= 120*time.Millisecond {
			emit("scanning", rel, 0)
			lastScanUpdate = time.Now()
		}
		return nil
	})
	if err != nil {
		result.DurationMS = time.Since(start).Milliseconds()
		return result, err
	}
	emit("copying", "", 0)
	for index, file := range files {
		emit("copying", file.rel, index)
		if e := a.backupOne(file.path, filepath.Join(root, file.rel), file.rel, &result); e != nil {
			result.Failed++
			if len(result.Errors) < 8 {
				result.Errors = append(result.Errors, file.rel+": "+e.Error())
			}
		}
		emit("copying", file.rel, index+1)
	}
	result.DurationMS = time.Since(start).Milliseconds()
	emit("done", "", len(files))
	return result, nil
}

func (a *Service) backupOne(src, dst, rel string, result *BackupResult) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	var existing Photo
	if err := a.db.Where("source_path = ? AND size = ?", src, info.Size()).Order("id desc").First(&existing).Error; err == nil {
		if _, err := os.Stat(existing.BackupPath); err == nil {
			currentHash, err := fileSHA256(src)
			if err != nil {
				return err
			}
			if currentHash == existing.SHA256 {
				result.Skipped++
				return nil
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	if _, err := os.Stat(dst); err == nil {
		ext := filepath.Ext(dst)
		base := strings.TrimSuffix(dst, ext)
		for i := 1; ; i++ {
			candidate := fmt.Sprintf("%s (%d)%s", base, i, ext)
			if _, e := os.Stat(candidate); os.IsNotExist(e) {
				dst = candidate
				break
			}
		}
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	config, _, decodeErr := image.DecodeConfig(in)
	if decodeErr != nil {
		return fmt.Errorf("อ่านรูปภาพไม่ได้: %w", decodeErr)
	}
	var colorTags []string
	if int64(config.Width)*int64(config.Height) <= 20_000_000 {
		if _, err := in.Seek(0, io.SeekStart); err != nil {
			return err
		}
		if decoded, _, err := image.Decode(in); err == nil {
			colorTags = analyzeColors(decoded)
		}
	}
	if _, err := in.Seek(0, io.SeekStart); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".photokeep-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	hash := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(tmp, hash), in)
	if syncErr := tmp.Sync(); copyErr == nil {
		copyErr = syncErr
	}
	if closeErr := tmp.Close(); copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		return copyErr
	}
	if n != info.Size() {
		return errors.New("ขนาดไฟล์ที่คัดลอกไม่ครบ")
	}
	if err := os.Rename(tmpName, dst); err != nil {
		return err
	}
	photo := Photo{SourcePath: src, BackupPath: dst, Filename: filepath.Base(src), RelativePath: rel, SHA256: hex.EncodeToString(hash.Sum(nil)), Size: n, Width: config.Width, Height: config.Height, TakenAt: info.ModTime(), BackedUpAt: time.Now()}
	photo.Tags = strings.Join(append(strings.Split(initialTags(photo), ","), colorTags...), ",")
	photo.Description = fmt.Sprintf("%s • %d × %d พิกเซล", strings.TrimSuffix(photo.Filename, filepath.Ext(photo.Filename)), photo.Width, photo.Height)
	if err := a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&photo).Error; err != nil {
			return err
		}
		return tx.Create(&FileRecord{Dest: filepath.Dir(dst), Filename: photo.Filename, Status: "backed_up"}).Error
	}); err != nil {
		_ = os.Remove(dst)
		return err
	}
	result.Added++
	return nil
}
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
