package photokeep

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type Service struct {
	db                *gorm.DB
	thumbnailCacheDir string
	backupInsertMu    sync.Mutex
	tagCacheMu        sync.RWMutex
	tagCache          []TagCount
	tagCacheVersion   uint64
	tagCacheValid     bool
}

func NewService(dbPath string) (*Service, error) {
	if dbPath == "" {
		config, err := os.UserConfigDir()
		if err != nil {
			return nil, err
		}
		dbPath = filepath.Join(config, "PhotoKeep", "crossplatform.db")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.Exec("PRAGMA busy_timeout = 5000").Error; err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&FileRecord{}, &Setting{}, &Photo{}, &ImageAIAnalysisJob{}); err != nil {
		return nil, err
	}
	if err := ensurePhotoSearchIndex(db); err != nil {
		return nil, err
	}
	service := &Service{db: db, thumbnailCacheDir: filepath.Join(filepath.Dir(dbPath), "thumbnails")}
	if err := service.normalizeExistingAICaptions(); err != nil {
		return nil, err
	}
	if err := service.requeueLegacyTagLanguageFailures(); err != nil {
		return nil, err
	}
	return service, nil
}

func ensurePhotoSearchIndex(db *gorm.DB) error {
	statements := []string{
		`CREATE VIRTUAL TABLE IF NOT EXISTS photo_search USING fts5(filename, description, tags, content='photos', content_rowid='id', tokenize='trigram')`,
		`CREATE TRIGGER IF NOT EXISTS photos_search_ai AFTER INSERT ON photos BEGIN
			INSERT INTO photo_search(rowid, filename, description, tags) VALUES (new.id, new.filename, new.description, new.tags);
		END`,
		`CREATE TRIGGER IF NOT EXISTS photos_search_ad AFTER DELETE ON photos BEGIN
			INSERT INTO photo_search(photo_search, rowid, filename, description, tags) VALUES ('delete', old.id, old.filename, old.description, old.tags);
		END`,
		`CREATE TRIGGER IF NOT EXISTS photos_search_au AFTER UPDATE OF filename, description, tags ON photos BEGIN
			INSERT INTO photo_search(photo_search, rowid, filename, description, tags) VALUES ('delete', old.id, old.filename, old.description, old.tags);
			INSERT INTO photo_search(rowid, filename, description, tags) VALUES (new.id, new.filename, new.description, new.tags);
		END`,
		`INSERT INTO photo_search(photo_search) VALUES ('rebuild')`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return fmt.Errorf("initialize local photo search index: %w", err)
		}
	}
	return nil
}

func (a *Service) Close() {
	if sqlDB, err := a.db.DB(); err == nil {
		_ = sqlDB.Close()
	}
}

func (a *Service) ClearDatabase() (State, error) {
	state, err := a.GetState()
	if err != nil {
		return State{}, err
	}
	var movedPaths []string
	if err := a.db.Model(&Photo{}).Where("moved_original = ?", true).Pluck("backup_path", &movedPaths).Error; err != nil {
		return State{}, err
	}
	protected := make(map[string]bool, len(movedPaths))
	for _, path := range movedPaths {
		protected[path] = true
	}
	if state.Dest != "" {
		if err := a.removeDestinationImagesExcept(state.Source, state.Dest, protected); err != nil {
			return State{}, err
		}
	}
	if err := a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&Photo{}).Error; err != nil {
			return err
		}
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&ImageAIAnalysisJob{}).Error; err != nil {
			return err
		}
		return tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&FileRecord{}).Error
	}); err != nil {
		return State{}, err
	}
	a.invalidateTagCloud()
	if err := os.RemoveAll(a.thumbnailCacheDir); err != nil {
		return State{}, err
	}
	return a.GetState()
}

func (a *Service) removeDestinationImages(source, dest string) error {
	return a.removeDestinationImagesExcept(source, dest, nil)
}

func (a *Service) removeDestinationImagesExcept(source, dest string, protected map[string]bool) error {
	dest, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	dest, err = filepath.EvalSymlinks(dest)
	if err != nil {
		return errors.New("เปิดโฟลเดอร์ปลายทางไม่ได้ จึงยังไม่ลบข้อมูลในคลัง")
	}
	info, err := os.Stat(dest)
	if err != nil || !info.IsDir() {
		return errors.New("โฟลเดอร์ปลายทางไม่พร้อมใช้งาน จึงยังไม่ลบข้อมูลในคลัง")
	}
	if source != "" {
		source, err = filepath.Abs(source)
		if err != nil {
			return err
		}
		if resolved, resolveErr := filepath.EvalSymlinks(source); resolveErr == nil {
			source = resolved
		}
		if nested(source, dest) || nested(dest, source) {
			return errors.New("ยกเลิกการลบเพื่อป้องกันรูปต้นฉบับ: โฟลเดอร์ต้นทางกับปลายทางซ้อนกัน")
		}
	}
	var directories []string
	var images []string
	if err := filepath.WalkDir(dest, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != dest {
				directories = append(directories, path)
			}
			return nil
		}
		isImage := imageExtensions[strings.ToLower(filepath.Ext(entry.Name()))]
		if isImage && !protected[path] && (entry.Type().IsRegular() || entry.Type()&os.ModeSymlink != 0) {
			images = append(images, path)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("สำรวจโฟลเดอร์ปลายทางไม่ได้: %w", err)
	}
	for _, path := range images {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("ลบรูปในโฟลเดอร์ปลายทางไม่ได้ (%s): %w", filepath.Base(path), err)
		}
	}
	for index := len(directories) - 1; index >= 0; index-- {
		_ = os.Remove(directories[index])
	}
	return nil
}

func (a *Service) GetState() (State, error) {
	var s State
	var setting Setting
	if err := a.db.Order("idx desc").First(&setting).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return s, err
	}
	s.Source, s.Dest = setting.Source, setting.Dest
	s.KeepOriginal = setting.KeepOriginal == nil || *setting.KeepOriginal
	if err := a.db.Model(&Photo{}).Count(&s.Total).Error; err != nil {
		return s, err
	}
	s.Tags, _ = a.tagCloud()
	return s, nil
}
