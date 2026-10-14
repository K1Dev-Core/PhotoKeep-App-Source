package photokeep

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestBackupProgressReportsEachFile(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	dest := filepath.Join(root, "dest")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"first.png", "second.png"} {
		file, err := os.Create(filepath.Join(source, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(file, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
			file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
	service, err := NewService(filepath.Join(root, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if _, err := service.SaveSettings(source, dest); err != nil {
		t.Fatal(err)
	}
	var events []BackupProgress
	result, err := service.Backup(func(progress BackupProgress) {
		events = append(events, progress)
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Added != 2 || len(events) < 5 {
		t.Fatalf("backup=%+v events=%+v", result, events)
	}
	last := events[len(events)-1]
	if last.Stage != "done" || last.Total != 2 || last.Completed != 2 || last.Added != 2 {
		t.Fatalf("final progress: %+v", last)
	}
	seen := map[string]bool{}
	for _, event := range events {
		if event.Stage == "copying" && event.CurrentFile != "" {
			seen[event.CurrentFile] = true
		}
	}
	if !seen["first.png"] || !seen["second.png"] {
		t.Fatalf("missing filenames in progress: %+v", events)
	}
}
