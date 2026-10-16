package photokeep

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func writeTestPNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 60), G: uint8(y * 60), B: 100, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func newRefreshTestService(t *testing.T, src, dst string) *Service {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	svc, err := NewService(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { svc.Close() })
	if _, err := svc.SaveSettings(src, dst); err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestRefreshDoesNotCopy(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeTestPNG(t, filepath.Join(src, "a.png"))

	svc := newRefreshTestService(t, src, dst)
	got, err := svc.Refresh()
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != 1 || got.New != 1 || got.Known != 0 {
		t.Fatalf("unexpected refresh result: %+v", got)
	}
	entries, _ := os.ReadDir(dst)
	if len(entries) != 0 {
		t.Fatalf("refresh copied files to dest: %v", entries)
	}

	got, err = svc.Refresh()
	if err != nil {
		t.Fatal(err)
	}
	_ = got
}

func TestRefreshSeesExistingBackup(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeTestPNG(t, filepath.Join(src, "a.png"))

	svc := newRefreshTestService(t, src, dst)
	if _, err := svc.Backup(nil); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Refresh()
	if err != nil {
		t.Fatal(err)
	}
	if got.New != 0 || got.Known != 1 {
		t.Fatalf("expected known=1 new=0, got %+v", got)
	}
}

func TestBackupDestLayoutFlat(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeTestPNG(t, filepath.Join(src, "a.png"))

	svc := newRefreshTestService(t, src, dst)
	res, err := svc.Backup(nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Added != 1 {
		t.Fatalf("expected added=1, got %+v", res)
	}
	if _, err := os.Stat(filepath.Join(dst, "a.png")); err != nil {
		t.Fatalf("expected file directly in dest: %v", err)
	}
}
