package main

import (
	"context"
	"errors"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"photokeep/internal/photokeep"
)

type App struct {
	service *photokeep.Service
	ctx     context.Context
}

func NewApp(dbPath string) (*App, error) {
	service, err := photokeep.NewService(dbPath)
	if err != nil {
		return nil, err
	}
	return &App{service: service}, nil
}

func (a *App) close() { a.service.Close() }

func (a *App) Startup(ctx context.Context) { a.ctx = ctx }

func (a *App) GetState() (photokeep.State, error) {
	return a.service.GetState()
}

func (a *App) SaveSettings(source, dest string) (photokeep.State, error) {
	return a.service.SaveSettings(source, dest)
}

func (a *App) Refresh() (photokeep.RefreshResult, error) {
	return a.service.Refresh()
}

func (a *App) ClearDatabase() (photokeep.State, error) {
	return a.service.ClearDatabase()
}

func (a *App) Backup() (photokeep.BackupResult, error) {
	if a.ctx == nil {
		return a.service.Backup(nil)
	}
	progressEvents := make(chan photokeep.BackupProgress, 1)
	go func() {
		for progress := range progressEvents {
			runtime.EventsEmit(a.ctx, "backup:progress", progress)
		}
	}()
	result, err := a.service.Backup(func(progress photokeep.BackupProgress) {
		select {
		case progressEvents <- progress:
		default:
			select {
			case <-progressEvents:
			default:
			}
			progressEvents <- progress
		}
	})
	close(progressEvents)
	return result, err
}

func (a *App) Search(query, tag string) (photokeep.SearchResult, error) {
	return a.service.Search(query, tag)
}

func (a *App) UpdatePhoto(id uint, description, tags string) (photokeep.Photo, error) {
	return a.service.UpdatePhoto(id, description, tags)
}

func (a *App) Preview(id uint, thumbnail bool) (string, error) {
	return a.service.Preview(id, thumbnail)
}

func (a *App) PickDirectory() (string, error) {
	if a.ctx == nil {
		return "", errors.New("ตัวเลือกโฟลเดอร์ใช้ได้ในแอปเดสก์ท็อป")
	}
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "เลือกโฟลเดอร์"})
}
