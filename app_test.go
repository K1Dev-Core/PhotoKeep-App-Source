package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackupSearchAndMetadata(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "camera")
	dst := filepath.Join(root, "archive")
	if err := os.MkdirAll(filepath.Join(src, "trip"), 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(src, "trip", "beach.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 4, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	app, err := NewApp(filepath.Join(root, "crossplatform.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer app.close()
	if _, err := app.SaveSettings(src, dst); err != nil {
		t.Fatal(err)
	}
	first, err := app.Backup()
	if err != nil {
		t.Fatal(err)
	}
	if first.Added != 1 || first.Failed != 0 {
		t.Fatalf("first backup: %+v", first)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("original removed:", err)
	}
	second, err := app.Backup()
	if err != nil {
		t.Fatal(err)
	}
	if second.Skipped != 1 || second.Added != 0 {
		t.Fatalf("second backup: %+v", second)
	}
	newFile, err := os.Create(filepath.Join(src, "trip", "new.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(newFile, img); err != nil {
		t.Fatal(err)
	}
	if err := newFile.Close(); err != nil {
		t.Fatal(err)
	}
	third, err := app.Backup()
	if err != nil {
		t.Fatal(err)
	}
	if third.Added != 1 || third.Skipped != 1 || third.Failed != 0 {
		t.Fatalf("backup after new source file: %+v", third)
	}
	newPhotos, err := app.Search("new", "")
	if err != nil || newPhotos.Total != 1 {
		t.Fatalf("new photo not searchable: result=%+v err=%v", newPhotos, err)
	}
	all, err := app.Search("beach", "")
	if err != nil {
		t.Fatal(err)
	}
	if all.Total != 1 || all.Photos[0].Width != 4 || all.Photos[0].Height != 2 {
		t.Fatalf("search: %+v", all)
	}
	photo, err := app.UpdatePhoto(all.Photos[0].ID, "ทะเลตอนเช้า", "ทะเล,เที่ยว,ทะเล")
	if err != nil {
		t.Fatal(err)
	}
	if photo.Tags != "ทะเล,เที่ยว" {
		t.Fatal(photo.Tags)
	}
	matches, err := app.Search("ทะเล", "เที่ยว")
	if err != nil {
		t.Fatal(err)
	}
	if matches.Total != 1 {
		t.Fatal("description/tag search failed")
	}
	preview, err := app.Preview(photo.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(preview, "data:image/jpeg;base64,") {
		t.Fatal("preview missing")
	}
	if _, err := app.SaveSettings(src, filepath.Join(src, "inside")); err == nil {
		t.Fatal("nested folders accepted")
	}
}
