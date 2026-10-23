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
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gen2brain/jpegn"
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
	root := state.Dest
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
		if path != state.Source && strings.HasPrefix(entry.Name(), ".") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
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
	workers := minInt(4, maxInt(1, runtime.GOMAXPROCS(0)))
	type backupJob struct {
		index int
		file  backupFile
	}
	type backupOutcome struct {
		index  int
		file   backupFile
		result BackupResult
		err    error
	}
	jobs := make(chan backupJob)
	outcomes := make(chan backupOutcome, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				fileResult := BackupResult{}
				err := a.backupOne(job.file.path, filepath.Join(root, job.file.rel), job.file.rel, state.KeepOriginal, &fileResult)
				outcomes <- backupOutcome{index: job.index, file: job.file, result: fileResult, err: err}
			}
		}()
	}
	go func() {
		for index, file := range files {
			jobs <- backupJob{index: index, file: file}
		}
		close(jobs)
		wg.Wait()
		close(outcomes)
	}()
	completed := 0
	for outcome := range outcomes {
		completed++
		result.Added += outcome.result.Added
		result.Skipped += outcome.result.Skipped
		result.Failed += outcome.result.Failed
		if outcome.err != nil {
			result.Failed++
			if len(result.Errors) < 8 {
				result.Errors = append(result.Errors, outcome.file.rel+": "+outcome.err.Error())
			}
		}
		emit("copying", outcome.file.rel, completed)
	}
	result.DurationMS = time.Since(start).Milliseconds()
	emit("done", "", len(files))
	return result, nil
}

func (a *Service) Refresh() (RefreshResult, error) {
	state, err := a.GetState()
	if err != nil {
		return RefreshResult{}, err
	}
	if state.Source == "" || state.Dest == "" {
		return RefreshResult{}, errors.New("ตั้งค่าโฟลเดอร์ก่อนรีเฟรช")
	}
	if nested(state.Source, state.Dest) || nested(state.Dest, state.Source) {
		return RefreshResult{}, errors.New("โฟลเดอร์ต้นทางกับปลายทางซ้อนกัน")
	}
	if info, e := os.Stat(state.Source); e != nil || !info.IsDir() {
		return RefreshResult{}, errors.New("ไม่พบโฟลเดอร์ต้นทาง กรุณาเชื่อมต่ออุปกรณ์")
	}
	result := RefreshResult{Errors: []string{}}
	err = filepath.WalkDir(state.Source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			result.Failed++
			return nil
		}
		if path != state.Source && strings.HasPrefix(entry.Name(), ".") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() || !entry.Type().IsRegular() || !imageExtensions[strings.ToLower(filepath.Ext(entry.Name()))] {
			return nil
		}
		info, err := os.Stat(path)
		if err != nil {
			result.Failed++
			return nil
		}
		result.Total++
		var existing Photo
		known := false
		if err := a.db.Where("source_path = ? AND size = ?", path, info.Size()).Order("id desc").First(&existing).Error; err == nil {
			if _, err := os.Stat(existing.BackupPath); err == nil {
				if currentHash, err := fileSHA256(path); err == nil && currentHash == existing.SHA256 {
					known = true
				}
			}
		}
		if known {
			result.Known++
		} else {
			result.New++
		}
		return nil
	})
	if err != nil {
		return result, err
	}
	return result, nil
}

func (a *Service) backupOne(src, dst, rel string, keepOriginal bool, result *BackupResult) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	var existing Photo
	if err := a.db.Where("source_path = ? AND size = ?", src, info.Size()).Order("id desc").Find(&existing).Error; err != nil {
		return err
	} else if existing.ID != 0 && filepath.Dir(existing.BackupPath) == filepath.Dir(dst) {
		if _, err := os.Stat(existing.BackupPath); err == nil {
			currentHash, err := fileSHA256(src)
			if err != nil {
				return err
			}
			if currentHash == existing.SHA256 {
				if !keepOriginal {
					if err := a.markAndRemoveOriginal(&existing, src); err != nil {
						return err
					}
				}
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
	config, format, decodeErr := image.DecodeConfig(in)
	if decodeErr != nil {
		return fmt.Errorf("อ่านรูปภาพไม่ได้: %w", decodeErr)
	}
	var colorTags []string
	var decodedForThumbnail image.Image
	if format == "jpeg" {
		if _, err := in.Seek(0, io.SeekStart); err != nil {
			return err
		}
		denom := jpegThumbnailScale(config.Width, config.Height, 256)
		if decoded, err := jpegn.Decode(in, &jpegn.Options{ScaleDenom: denom, AutoRotate: true}); err == nil {
			decodedForThumbnail = decoded
			colorTags = analyzeColors(decoded)
		}
	} else if int64(config.Width)*int64(config.Height) <= 20_000_000 {
		if _, err := in.Seek(0, io.SeekStart); err != nil {
			return err
		}
		if decoded, _, err := image.Decode(in); err == nil {
			decodedForThumbnail = decoded
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
	photo := Photo{SourcePath: src, BackupPath: dst, Filename: filepath.Base(src), RelativePath: rel, SHA256: hex.EncodeToString(hash.Sum(nil)), Size: n, Width: config.Width, Height: config.Height, TakenAt: info.ModTime(), BackedUpAt: time.Now(), AIStatus: "pending"}
	photo.Tags = strings.Join(append(strings.Split(initialTags(photo), ","), colorTags...), ",")
	photo.Description = fmt.Sprintf("%s • %d × %d พิกเซล", strings.TrimSuffix(photo.Filename, filepath.Ext(photo.Filename)), photo.Width, photo.Height)
	a.backupInsertMu.Lock()
	err = a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&photo).Error; err != nil {
			return err
		}
		return tx.Create(&FileRecord{Dest: filepath.Dir(dst), Filename: photo.Filename, Status: "backed_up"}).Error
	})
	a.backupInsertMu.Unlock()
	if err != nil {
		_ = os.Remove(dst)
		return err
	}
	a.invalidateTagCloud()
	if decodedForThumbnail != nil {
		_, _ = a.encodeThumbnail(photo, decodedForThumbnail)
	}
	if !keepOriginal {
		if err := a.markAndRemoveOriginal(&photo, src); err != nil {
			return err
		}
	}
	result.Added++
	return nil
}

func (a *Service) markAndRemoveOriginal(photo *Photo, src string) error {
	if err := a.db.Model(photo).Update("moved_original", true).Error; err != nil {
		return fmt.Errorf("บันทึกสถานะการย้ายภาพไม่ได้: %w", err)
	}
	if err := os.Remove(src); err != nil {
		_ = a.db.Model(photo).Update("moved_original", false).Error
		return fmt.Errorf("ลบต้นฉบับหลังสำรองไม่สำเร็จ: %w", err)
	}
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
