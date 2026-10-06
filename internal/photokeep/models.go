package photokeep

import "time"

type FileRecord struct {
	Idx      uint   `gorm:"column:idx;primaryKey;autoIncrement"`
	Dest     string `gorm:"column:dest;not null"`
	Filename string `gorm:"column:filename;not null"`
	Status   string `gorm:"column:status;not null;default:backed_up"`
}

func (FileRecord) TableName() string { return "files" }

type Setting struct {
	Idx    uint   `gorm:"column:idx;primaryKey;autoIncrement"`
	Source string `gorm:"column:source;not null"`
	Dest   string `gorm:"column:dest;not null"`
}

func (Setting) TableName() string { return "settings" }

type Photo struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	SourcePath   string    `json:"sourcePath"`
	BackupPath   string    `json:"backupPath" gorm:"uniqueIndex"`
	Filename     string    `json:"filename"`
	RelativePath string    `json:"relativePath"`
	Description  string    `json:"description"`
	Tags         string    `json:"tags"`
	SHA256       string    `json:"sha256"`
	Size         int64     `json:"size"`
	Width        int       `json:"width"`
	Height       int       `json:"height"`
	TakenAt      time.Time `json:"takenAt"`
	BackedUpAt   time.Time `json:"backedUpAt"`
}

type State struct {
	Source string     `json:"source"`
	Dest   string     `json:"dest"`
	Total  int64      `json:"total"`
	Tags   []TagCount `json:"tags"`
}
type TagCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}
type BackupResult struct {
	Added      int      `json:"added"`
	Skipped    int      `json:"skipped"`
	Failed     int      `json:"failed"`
	Errors     []string `json:"errors"`
	DurationMS int64    `json:"durationMs"`
}
type BackupProgress struct {
	Stage       string `json:"stage"`
	Total       int    `json:"total"`
	Completed   int    `json:"completed"`
	Added       int    `json:"added"`
	Skipped     int    `json:"skipped"`
	Failed      int    `json:"failed"`
	CurrentFile string `json:"currentFile"`
}
type SearchResult struct {
	Photos []Photo    `json:"photos"`
	Total  int64      `json:"total"`
	Tags   []TagCount `json:"tags"`
}
