package photokeep

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
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
	if err := db.AutoMigrate(&FileRecord{}, &Setting{}, &Photo{}); err != nil {
		return nil, err
	}
	return &Service{db: db}, nil
}

func (a *Service) Close() {
	if sqlDB, err := a.db.DB(); err == nil {
		_ = sqlDB.Close()
	}
}

func (a *Service) ClearDatabase() (State, error) {
	if err := a.db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&Photo{}).Error; err != nil {
		return State{}, err
	}
	if err := a.db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&FileRecord{}).Error; err != nil {
		return State{}, err
	}
	return a.GetState()
}

func (a *Service) GetState() (State, error) {
	var s State
	var setting Setting
	if err := a.db.Order("idx desc").First(&setting).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return s, err
	}
	s.Source, s.Dest = setting.Source, setting.Dest
	if err := a.db.Model(&Photo{}).Count(&s.Total).Error; err != nil {
		return s, err
	}
	s.Tags, _ = a.tagCloud()
	return s, nil
}
