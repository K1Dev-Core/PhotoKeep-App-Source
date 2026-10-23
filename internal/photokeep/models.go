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
	Idx          uint   `gorm:"column:idx;primaryKey;autoIncrement"`
	Source       string `gorm:"column:source;not null"`
	Dest         string `gorm:"column:dest;not null"`
	KeepOriginal *bool  `gorm:"column:keep_original;not null;default:true"`
}

func (Setting) TableName() string { return "settings" }

type Photo struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	SourcePath      string    `json:"sourcePath"`
	BackupPath      string    `json:"backupPath" gorm:"uniqueIndex"`
	MovedOriginal   bool      `json:"movedOriginal" gorm:"not null;default:false"`
	Filename        string    `json:"filename"`
	RelativePath    string    `json:"relativePath"`
	Description     string    `json:"description"`
	Tags            string    `json:"tags"`
	Embedding       []byte    `json:"-" gorm:"type:blob"`
	EmbeddingModel  string    `json:"-"`
	EmbeddingStatus string    `json:"embeddingStatus,omitempty"`
	AIStatus        string    `json:"aiStatus"`
	AIError         string    `json:"aiError"`
	AIUpdatedAt     time.Time `json:"aiUpdatedAt"`
	SHA256          string    `json:"sha256"`
	Size            int64     `json:"size"`
	Width           int       `json:"width"`
	Height          int       `json:"height"`
	TakenAt         time.Time `json:"takenAt"`
	BackedUpAt      time.Time `json:"backedUpAt"`
}

func (Photo) TableName() string { return "photos" }

type ImageAIAnalysisJob struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	PhotoIDs        string    `json:"-" gorm:"type:text;not null"`
	Status          string    `json:"status" gorm:"not null"`
	Total           int       `json:"total"`
	Completed       int       `json:"completed"`
	Failed          int       `json:"failed"`
	Finished        bool      `json:"finished"`
	Error           string    `json:"error"`
	DurationSeconds int64     `json:"durationSeconds"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func (ImageAIAnalysisJob) TableName() string { return "image_ai_analysis_jobs" }

type ImageAIQueueProgress struct {
	Total          int64  `json:"total"`
	Completed      int64  `json:"completed"`
	Pending        int64  `json:"pending"`
	Analyzing      int64  `json:"analyzing"`
	Failed         int64  `json:"failed"`
	ElapsedSeconds int64  `json:"elapsedSeconds"`
	Paused         bool   `json:"paused"`
	Error          string `json:"error,omitempty"`
}

type ImageAIErrorLog struct {
	PhotoID  uint      `json:"photoId"`
	Filename string    `json:"filename"`
	Kind     string    `json:"kind"`
	Error    string    `json:"error"`
	LoggedAt time.Time `json:"loggedAt"`
}

type State struct {
	Source       string     `json:"source"`
	Dest         string     `json:"dest"`
	KeepOriginal bool       `json:"keepOriginal"`
	Total        int64      `json:"total"`
	Tags         []TagCount `json:"tags"`
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
type RefreshResult struct {
	Total  int      `json:"total"`
	New    int      `json:"new"`
	Known  int      `json:"known"`
	Failed int      `json:"failed"`
	Errors []string `json:"errors"`
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

type ThumbnailPreview struct {
	PhotoID uint   `json:"photoId"`
	DataURL string `json:"dataUrl"`
	Error   string `json:"error,omitempty"`
}
