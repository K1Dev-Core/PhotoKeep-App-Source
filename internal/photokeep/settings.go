package photokeep

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (a *Service) SaveSettings(source, dest string) (State, error) {
	source, dest = strings.TrimSpace(source), strings.TrimSpace(dest)
	if source == "" || dest == "" {
		return State{}, errors.New("กรุณาเลือกโฟลเดอร์ต้นทางและปลายทาง")
	}
	var err error
	source, err = filepath.Abs(source)
	if err != nil {
		return State{}, err
	}
	dest, err = filepath.Abs(dest)
	if err != nil {
		return State{}, err
	}
	info, err := os.Stat(source)
	if err != nil || !info.IsDir() {
		return State{}, errors.New("โฟลเดอร์ต้นทางไม่มีอยู่หรือเปิดไม่ได้")
	}
	if err := os.MkdirAll(dest, 0755); err != nil {
		return State{}, fmt.Errorf("สร้างโฟลเดอร์ปลายทาง: %w", err)
	}
	source, err = filepath.EvalSymlinks(source)
	if err != nil {
		return State{}, err
	}
	dest, err = filepath.EvalSymlinks(dest)
	if err != nil {
		return State{}, err
	}
	if nested(source, dest) || nested(dest, source) {
		return State{}, errors.New("ต้นทางและปลายทางต้องเป็นคนละโฟลเดอร์และไม่ซ้อนกัน")
	}
	if err := a.db.Create(&Setting{Source: source, Dest: dest}).Error; err != nil {
		return State{}, err
	}
	return a.GetState()
}
func nested(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))))
}
