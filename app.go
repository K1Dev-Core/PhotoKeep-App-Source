package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"photokeep/internal/photokeep"
)

type App struct {
	service             *photokeep.Service
	ctx                 context.Context
	imageAIStartMu      sync.Mutex
	imageAIResumeMu     sync.Mutex
	imageAIResumeTimer  *time.Timer
	imageAIResumeAt     time.Time
	imageAIControlMu    sync.Mutex
	imageAICancel       context.CancelFunc
	imageAIJobID        uint
	imageAIStopped      bool
	imageAIErrorMu      sync.RWMutex
	imageAIQueueError   string
	embeddingIndexMu    sync.Mutex
	embeddingIndexing   bool
	embeddingCancel     context.CancelFunc
	queryEmbeddingMu    sync.Mutex
	queryEmbeddings     map[string]photokeep.EmbeddingVector
	searchRequestMu     sync.Mutex
	searchRequestCancel context.CancelFunc
	searchRequestSeq    atomic.Uint64
}

const minSemanticSearchScore = 0.68

const maxSemanticSearchResults = 120

const lexicalSearchFusionWeight = 2

const semanticSearchScoreMargin = 0.05

func NewApp(dbPath string) (*App, error) {
	service, err := photokeep.NewService(dbPath)
	if err != nil {
		return nil, err
	}
	return &App{service: service}, nil
}

func (a *App) close() {
	a.embeddingIndexMu.Lock()
	if a.embeddingCancel != nil {
		a.embeddingCancel()
		a.embeddingCancel = nil
	}
	a.embeddingIndexMu.Unlock()
	a.service.Close()
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	_ = a.service.MarkRunningImageAIJobsInterrupted()
	a.startPhotoEmbeddingIndex()
}

func (a *App) GetState() (photokeep.State, error) {
	return a.service.GetState()
}

func (a *App) SaveSettings(source, dest string) (photokeep.State, error) {
	return a.service.SaveSettings(source, dest)
}

func (a *App) SaveBackupSettings(source, dest string, keepOriginal bool) (photokeep.State, error) {
	return a.service.SaveBackupSettings(source, dest, keepOriginal)
}

func (a *App) Refresh() (photokeep.RefreshResult, error) {
	return a.service.Refresh()
}

func (a *App) ClearDatabase() (photokeep.State, error) {
	a.embeddingIndexMu.Lock()
	if a.embeddingCancel != nil {
		a.embeddingCancel()
		a.embeddingCancel = nil
	}
	a.embeddingIndexMu.Unlock()
	a.imageAIResumeMu.Lock()
	if a.imageAIResumeTimer != nil {
		a.imageAIResumeTimer.Stop()
		a.imageAIResumeTimer = nil
		a.imageAIResumeAt = time.Time{}
	}
	a.imageAIResumeMu.Unlock()
	a.imageAIStartMu.Lock()
	defer a.imageAIStartMu.Unlock()
	a.imageAIControlMu.Lock()
	a.imageAIStopped = true
	cancel := a.imageAICancel
	a.imageAICancel = nil
	a.imageAIControlMu.Unlock()
	if cancel != nil {
		cancel()
	}
	state, err := a.service.ClearDatabase()
	a.imageAIControlMu.Lock()
	a.imageAIStopped = false
	a.imageAIControlMu.Unlock()
	return state, err
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
	if err == nil && result.Failed == 0 && result.Added+result.Skipped > 0 {

		a.scheduleImageAIResume(0)
		a.startPhotoEmbeddingIndex()
	}
	return result, err
}

func (a *App) Search(query, tag string) (photokeep.SearchResult, error) {
	return a.searchPage(query, tag, 0, 0)
}

func (a *App) SearchPage(query, tag string, offset, limit int) (photokeep.SearchResult, error) {
	if limit < 1 || limit > 200 {
		limit = 80
	}
	return a.searchPage(query, tag, offset, limit)
}

func (a *App) searchPage(query, tag string, offset, limit int) (photokeep.SearchResult, error) {
	query = strings.TrimSpace(query)
	requestCtx, finishSearch := a.beginSearchRequest()
	defer finishSearch()
	if query == "" {
		return a.service.SearchPage(query, tag, offset, limit)
	}
	lexical, err := a.service.SearchPage(query, tag, 0, 0)
	if err != nil {
		return photokeep.SearchResult{}, err
	}
	hasEmbeddings, err := a.service.HasPhotoEmbeddings()
	if err != nil || !hasEmbeddings {
		return pageSearchResult(lexical, offset, limit), nil
	}
	client, err := photokeep.NewManagedEmbeddingClient(a.managedAIURL())
	if err != nil {
		return pageSearchResult(lexical, offset, limit), nil
	}
	vector, err := a.queryEmbedding(requestCtx, query, client)
	if err != nil {
		return pageSearchResult(lexical, offset, limit), nil
	}
	semanticPhotos, semanticScores, err := a.service.SemanticPhotoScores(vector.Values, vector.Model, tag)
	if err != nil || len(semanticPhotos) == 0 {
		return pageSearchResult(lexical, offset, limit), nil
	}
	return combineSearchResults(query, lexical, semanticPhotos, semanticScores, offset, limit), nil
}

func (a *App) beginSearchRequest() (context.Context, func()) {
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	requestCtx, cancel := context.WithTimeout(parent, 10*time.Second)
	requestID := a.searchRequestSeq.Add(1)
	a.searchRequestMu.Lock()
	if a.searchRequestCancel != nil {
		a.searchRequestCancel()
	}
	a.searchRequestCancel = cancel
	a.searchRequestMu.Unlock()
	finish := func() {
		cancel()
		a.searchRequestMu.Lock()
		if a.searchRequestSeq.Load() == requestID {
			a.searchRequestCancel = nil
		}
		a.searchRequestMu.Unlock()
	}
	return requestCtx, finish
}

func (a *App) queryEmbedding(ctx context.Context, query string, client *photokeep.ManagedEmbeddingClient) (photokeep.EmbeddingVector, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	a.queryEmbeddingMu.Lock()
	if cached, exists := a.queryEmbeddings[query]; exists {
		a.queryEmbeddingMu.Unlock()
		return cached, nil
	}
	a.queryEmbeddingMu.Unlock()
	var vector photokeep.EmbeddingVector
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		vector, err = client.EmbedText(ctx, query)
		if err == nil {
			break
		}
		if attempt < 1 && !waitForSearchEmbeddingRetry(ctx) {
			return photokeep.EmbeddingVector{}, ctx.Err()
		}
	}
	if err != nil {
		return photokeep.EmbeddingVector{}, err
	}
	a.queryEmbeddingMu.Lock()
	if a.queryEmbeddings == nil {
		a.queryEmbeddings = map[string]photokeep.EmbeddingVector{}
	}
	if len(a.queryEmbeddings) >= 128 {
		for key := range a.queryEmbeddings {
			delete(a.queryEmbeddings, key)
			break
		}
	}
	a.queryEmbeddings[query] = vector
	a.queryEmbeddingMu.Unlock()
	return vector, nil
}

func waitForSearchEmbeddingRetry(ctx context.Context) bool {
	timer := time.NewTimer(300 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func waitForEmbeddingRetry(ctx context.Context, attempt int) bool {
	timer := time.NewTimer(time.Duration(attempt+1) * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

type combinedSearchHit struct {
	photo photokeep.Photo
	score float64
}

func combineSearchResults(query string, lexical photokeep.SearchResult, semantic []photokeep.Photo, scores []float64, offset, limit int) photokeep.SearchResult {
	hits := make(map[uint]combinedSearchHit, len(lexical.Photos)+len(semantic))
	type semanticHit struct {
		photo photokeep.Photo
		score float64
	}
	semanticHits := make([]semanticHit, 0, len(semantic))
	for i, photo := range semantic {
		if i < len(scores) {
			semanticHits = append(semanticHits, semanticHit{photo: photo, score: scores[i]})
		}
	}
	sort.Slice(semanticHits, func(i, j int) bool { return semanticHits[i].score > semanticHits[j].score })
	if len(semanticHits) > 0 {
		threshold := max(minSemanticSearchScore, semanticHits[0].score-semanticSearchScoreMargin)
		filtered := semanticHits[:0]
		for _, hit := range semanticHits {
			if hit.score < threshold || len(filtered) >= maxSemanticSearchResults {
				break
			}
			filtered = append(filtered, hit)
		}
		semanticHits = filtered
	}
	for rank, semanticHit := range semanticHits {
		photo := semanticHit.photo
		score := 1 / float64(60+rank+1)
		hits[photo.ID] = combinedSearchHit{photo: photo, score: score}
	}
	query = strings.ToLower(strings.TrimSpace(query))
	for rank, photo := range lexical.Photos {
		current, exists := hits[photo.ID]
		if !exists {
			if len(semanticHits) > 0 && !strings.Contains(strings.ToLower(photo.Filename), query) {
				continue
			}
			current.photo = photo
			current.score = lexicalSearchFusionWeight / float64(60+rank+1)
			hits[photo.ID] = current
			continue
		}
		current.score += lexicalSearchFusionWeight / float64(60+rank+1)
		hits[photo.ID] = current
	}
	ordered := make([]combinedSearchHit, 0, len(hits))
	for _, hit := range hits {
		ordered = append(ordered, hit)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].score != ordered[j].score {
			return ordered[i].score > ordered[j].score
		}
		return ordered[i].photo.BackedUpAt.After(ordered[j].photo.BackedUpAt)
	})
	start := min(max(0, offset), len(ordered))
	end := len(ordered)
	if limit > 0 {
		end = min(end, start+limit)
	}
	photos := make([]photokeep.Photo, 0, end-start)
	for _, hit := range ordered[start:end] {
		photos = append(photos, hit.photo)
	}
	lexical.Photos = photos
	lexical.Total = int64(len(ordered))
	return lexical
}

func pageSearchResult(result photokeep.SearchResult, offset, limit int) photokeep.SearchResult {
	start := min(max(0, offset), len(result.Photos))
	end := len(result.Photos)
	if limit > 0 {
		end = min(end, start+limit)
	}
	result.Photos = result.Photos[start:end]
	return result
}

func (a *App) startPhotoEmbeddingIndex() {
	a.embeddingIndexMu.Lock()
	if a.embeddingIndexing || a.ctx == nil {
		a.embeddingIndexMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.embeddingIndexing = true
	a.embeddingCancel = cancel
	a.embeddingIndexMu.Unlock()
	go func() {
		defer func() {
			a.embeddingIndexMu.Lock()
			a.embeddingIndexing = false
			a.embeddingCancel = nil
			a.embeddingIndexMu.Unlock()
		}()
		client, err := photokeep.NewManagedEmbeddingClient(a.managedAIURL())
		if err != nil {
			return
		}
		inputs, err := a.service.PendingPhotoEmbeddings(1000)
		if err != nil {
			return
		}
		if len(inputs) == 0 {
			return
		}
		concurrency, err := client.Concurrency(ctx)
		if err != nil {
			concurrency = 1
		}
		workers := min(max(1, concurrency), len(inputs))
		work := make(chan photokeep.PhotoEmbeddingInput)
		var workersDone sync.WaitGroup
		for range workers {
			workersDone.Add(1)
			go func() {
				defer workersDone.Done()
				for input := range work {
					if ctx.Err() != nil {
						return
					}
					if err := a.service.SetPhotoEmbeddingStatus(input.ID, "indexing"); err != nil {
						continue
					}
					requestCtx, requestCancel := context.WithTimeout(ctx, 90*time.Second)
					var vector photokeep.EmbeddingVector
					var embedErr error
					for attempt := 0; attempt < 3; attempt++ {
						vector, embedErr = client.EmbedImage(requestCtx, input.Path)
						if embedErr == nil || requestCtx.Err() != nil {
							break
						}
						if attempt < 2 && !waitForEmbeddingRetry(requestCtx, attempt) {
							break
						}
					}
					requestCancel()
					if ctx.Err() != nil {
						return
					}
					if embedErr != nil {
						_ = a.service.SetPhotoEmbeddingStatus(input.ID, "failed")
						continue
					}
					_ = a.service.SavePhotoEmbedding(input.ID, vector)
				}
			}()
		}
	feed:
		for _, input := range inputs {
			select {
			case work <- input:
			case <-ctx.Done():
				break feed
			}
		}
		close(work)
		workersDone.Wait()
	}()
}

func (a *App) UpdatePhoto(id uint, description, tags string) (photokeep.Photo, error) {
	return a.service.UpdatePhoto(id, description, tags)
}

func (a *App) Preview(id uint, thumbnail bool) (string, error) {
	return a.service.Preview(id, thumbnail)
}

func (a *App) PreviewThumbnails(ids []uint) ([]photokeep.ThumbnailPreview, error) {
	return a.service.PreviewThumbnails(ids)
}

func (a *App) PickDirectory() (string, error) {
	if a.ctx == nil {
		return "", errors.New("ตัวเลือกโฟลเดอร์ใช้ได้ในแอปเดสก์ท็อป")
	}
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "เลือกโฟลเดอร์"})
}

func (a *App) OpenBackupFolder() error {
	state, err := a.service.GetState()
	if err != nil {
		return err
	}
	if state.Dest == "" {
		return errors.New("ยังไม่ได้ตั้งค่าโฟลเดอร์ปลายทาง")
	}
	info, err := os.Stat(state.Dest)
	if err != nil {
		return fmt.Errorf("เปิดโฟลเดอร์ปลายทางไม่ได้: %w", err)
	}
	if !info.IsDir() {
		return errors.New("ปลายทางที่ตั้งไว้ไม่ใช่โฟลเดอร์")
	}
	return exec.Command("open", state.Dest).Run()
}

type OpenVSXThemeResult = photokeep.OpenVSXThemeResult
type OpenVSXThemeSearch = photokeep.OpenVSXThemeSearch
type OpenVSXThemePackage = photokeep.OpenVSXThemePackage

func (a *App) SearchOpenVSXThemes(query string, offset int) (OpenVSXThemeSearch, error) {
	return photokeep.SearchOpenVSXThemes(query, offset)
}

func (a *App) FetchOpenVSXThemePackage(namespace, name, version string) (OpenVSXThemePackage, error) {
	return photokeep.FetchOpenVSXThemePackage(namespace, name, version)
}

var managedAIEndpoint string

const defaultManagedAIEndpoint = "https://vision.k1god.com/v1/analyze"

func (a *App) managedAIURL() string {
	if a != nil && a.service != nil {
		if value, err := a.service.GetAppSetting("ai_gateway_url"); err == nil && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	if value := strings.TrimSpace(os.Getenv("PHOTOKEEP_AI_GATEWAY_URL")); value != "" {
		return value
	}
	if value := strings.TrimSpace(managedAIEndpoint); value != "" {
		return value
	}
	return defaultManagedAIEndpoint
}

type ImageAISettings struct {
	Endpoint       string `json:"endpoint"`
	SearchEndpoint string `json:"searchEndpoint"`
	Provider       string `json:"provider"`
	Model          string `json:"model"`
	KeyCount       int    `json:"keyCount"`
	Concurrency    int    `json:"concurrency"`
	Configured     bool   `json:"configured"`
}

func (a *App) GetImageAISettings() (ImageAISettings, error) {
	concurrency := 0
	client, err := photokeep.NewManagedImageAIClient(a.managedAIURL())
	if err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		concurrency, _ = client.Concurrency(ctx)
		cancel()
	}
	return ImageAISettings{
		Endpoint:       a.managedAIURL(),
		SearchEndpoint: "ค้นหาในเครื่อง · SQLite FTS5",
		Provider:       "managed",
		Model:          "กำหนดจาก Gateway",
		Concurrency:    concurrency,
		Configured:     true,
	}, nil
}

func (a *App) SaveImageAIGatewayURL(endpoint string) (ImageAISettings, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = defaultManagedAIEndpoint
	}
	if _, err := photokeep.NewManagedImageAIClient(endpoint); err != nil {
		return ImageAISettings{}, err
	}
	if err := a.service.SaveAppSetting("ai_gateway_url", endpoint); err != nil {
		return ImageAISettings{}, err
	}
	return a.GetImageAISettings()
}

func (a *App) StartImageAIAnalysis(photoIDs []uint) (photokeep.ImageAIAnalysisJob, error) {
	a.imageAIStartMu.Lock()
	defer a.imageAIStartMu.Unlock()
	if a.ctx == nil {
		return photokeep.ImageAIAnalysisJob{}, errors.New("การวิเคราะห์ AI ใช้ได้ในแอปเดสก์ท็อป")
	}
	client, err := photokeep.NewManagedImageAIClient(a.managedAIURL())
	if err != nil {
		a.setImageAIQueueError(err.Error())
		return photokeep.ImageAIAnalysisJob{}, err
	}
	healthCtx, healthCancel := context.WithTimeout(a.ctx, 12*time.Second)
	err = client.Health(healthCtx)
	healthCancel()
	if err != nil {
		a.setImageAIQueueError(err.Error())
		return photokeep.ImageAIAnalysisJob{}, err
	}
	a.setImageAIQueueError("")
	inputs, err := a.service.PrepareImageAIInputs(photoIDs)
	if err != nil {
		return photokeep.ImageAIAnalysisJob{}, err
	}
	job, err := a.service.CreateImageAIAnalysisJob(inputs)
	if err != nil {
		return job, err
	}
	ids := make([]uint, len(inputs))
	for index, input := range inputs {
		ids[index] = input.PhotoID
	}
	if err := a.service.SetImageAIStatus(ids, "pending", ""); err != nil {
		_ = a.service.FailImageAIAnalysisJob(job.ID, err.Error())
		return job, err
	}
	jobCtx, cancel := context.WithCancel(a.ctx)
	a.imageAIControlMu.Lock()
	a.imageAIStopped = false
	a.imageAICancel = cancel
	a.imageAIJobID = job.ID
	a.imageAIControlMu.Unlock()
	go a.runImageAIAnalysisJob(jobCtx, job.ID, inputs)
	return job, nil
}

func (a *App) ResumePendingImageAIAnalysis() (photokeep.ImageAIAnalysisJob, error) {
	inputs, err := a.service.PendingImageAIInputs()
	if err != nil {
		return photokeep.ImageAIAnalysisJob{}, err
	}
	if len(inputs) == 0 {
		return photokeep.ImageAIAnalysisJob{}, nil
	}
	ids := make([]uint, len(inputs))
	for index, input := range inputs {
		ids[index] = input.PhotoID
	}
	return a.StartImageAIAnalysis(ids)
}

func (a *App) GetImageAIQueueProgress() (photokeep.ImageAIQueueProgress, error) {
	progress, err := a.service.ImageAIQueueProgress()
	if err != nil {
		return progress, err
	}
	a.imageAIErrorMu.RLock()
	progress.Error = a.imageAIQueueError
	a.imageAIErrorMu.RUnlock()

	a.imageAIControlMu.Lock()
	if progress.Pending > 0 && progress.Analyzing == 0 && a.imageAICancel == nil {
		progress.Paused = true
	}
	a.imageAIControlMu.Unlock()
	return progress, nil
}

func (a *App) setImageAIQueueError(message string) {
	a.imageAIErrorMu.Lock()
	a.imageAIQueueError = strings.TrimSpace(message)
	a.imageAIErrorMu.Unlock()
}

func (a *App) StopImageAIAnalysis() error {
	a.imageAIControlMu.Lock()
	cancel := a.imageAICancel
	if cancel != nil {
		a.imageAIStopped = true
	}
	a.imageAIControlMu.Unlock()
	if cancel == nil {
		return errors.New("ไม่มีงานวิเคราะห์ที่กำลังทำอยู่")
	}
	cancel()
	return nil
}

func (a *App) ContinueImageAIAnalysis() (photokeep.ImageAIAnalysisJob, error) {
	a.imageAIControlMu.Lock()
	a.imageAIStopped = false
	a.imageAIControlMu.Unlock()
	return a.ResumePendingImageAIAnalysis()
}

func (a *App) GetImageAIErrorLogs() ([]photokeep.ImageAIErrorLog, error) {
	return a.service.ListImageAIErrorLogs(200)
}

func (a *App) RetryFailedImageAIAnalysis(photoIDs []uint) (int64, error) {
	count, err := a.service.RequeueFailedImageAIAnalysis(photoIDs)
	if err == nil && count > 0 {
		a.scheduleImageAIResume(0)
	}
	return count, err
}

func (a *App) resumePendingImageAIAnalysis() {
	a.imageAIResumeMu.Lock()
	if a.ctx == nil || a.ctx.Err() != nil {
		a.imageAIResumeMu.Unlock()
		return
	}
	a.imageAIControlMu.Lock()
	stopped := a.imageAIStopped
	active := a.imageAICancel != nil
	a.imageAIControlMu.Unlock()
	if stopped || active {
		a.imageAIResumeMu.Unlock()
		return
	}
	progress, err := a.service.ImageAIQueueProgress()
	if err != nil {
		a.imageAIResumeMu.Unlock()
		return
	}
	if progress.Pending == 0 {
		a.setImageAIQueueError("")
		a.imageAIResumeMu.Unlock()
		return
	}
	if a.ctx == nil {
		a.imageAIResumeMu.Unlock()
		return
	}
	job, err := a.ResumePendingImageAIAnalysis()
	if err != nil {
		a.setImageAIQueueError(err.Error())
		a.imageAIResumeMu.Unlock()
		a.scheduleImageAIResume(20 * time.Second)
		return
	}
	if job.ID == 0 {
		a.imageAIResumeMu.Unlock()
		return
	}
	a.setImageAIQueueError("")
	runtime.EventsEmit(a.ctx, "image-ai:progress", job)
	a.imageAIResumeMu.Unlock()
}

func (a *App) scheduleImageAIResume(delay time.Duration) {
	a.imageAIResumeMu.Lock()
	runAt := time.Now().Add(delay)
	if a.imageAIResumeTimer != nil {

		if !a.imageAIResumeAt.IsZero() && !runAt.Before(a.imageAIResumeAt) {
			a.imageAIResumeMu.Unlock()
			return
		}
		a.imageAIResumeTimer.Stop()
	}
	a.imageAIResumeAt = runAt
	a.imageAIResumeTimer = time.AfterFunc(delay, func() {
		a.imageAIResumeMu.Lock()
		if !a.imageAIResumeAt.Equal(runAt) {
			a.imageAIResumeMu.Unlock()
			return
		}
		a.imageAIResumeTimer = nil
		a.imageAIResumeAt = time.Time{}
		a.imageAIResumeMu.Unlock()
		a.resumePendingImageAIAnalysis()
	})
	a.imageAIResumeMu.Unlock()
}

func (a *App) RefreshImageAIAnalysisJob(id uint) (photokeep.ImageAIAnalysisJob, error) {
	return a.service.ImageAIAnalysisJob(id)
}

func (a *App) ListImageAIAnalysisJobs() ([]photokeep.ImageAIAnalysisJob, error) {
	return a.service.ListImageAIAnalysisJobs()
}

func (a *App) runImageAIAnalysisJob(ctx context.Context, jobID uint, inputs []photokeep.ImageAIInput) {
	resumeAfter := false
	resumeDelay := time.Duration(0)
	defer func() {
		a.imageAIControlMu.Lock()
		if a.imageAIJobID == jobID {
			a.imageAICancel = nil
			a.imageAIJobID = 0
		}
		a.imageAIControlMu.Unlock()
		if resumeAfter {
			a.scheduleImageAIResume(resumeDelay)
		}
	}()
	a.runManagedImageAIAnalysisJob(ctx, jobID, inputs, &resumeAfter, &resumeDelay)
}

func (a *App) runManagedImageAIAnalysisJob(ctx context.Context, jobID uint, inputs []photokeep.ImageAIInput, resumeAfter *bool, resumeDelay *time.Duration) {
	client, err := photokeep.NewManagedImageAIClient(a.managedAIURL())
	if err != nil {
		_ = a.service.FailImageAIAnalysisJob(jobID, err.Error())
		return
	}
	type outcome struct {
		input  photokeep.ImageAIInput
		result photokeep.ImageAIResult
		err    error
	}
	work := make(chan photokeep.ImageAIInput)
	results := make(chan outcome, 16)
	var wg sync.WaitGroup
	var rateLimited atomic.Bool
	concurrencyCtx, concurrencyCancel := context.WithTimeout(ctx, 12*time.Second)
	concurrency, concurrencyErr := client.Concurrency(concurrencyCtx)
	concurrencyCancel()
	if concurrencyErr != nil {
		concurrency = 1
	}
	workers := min(max(1, concurrency), len(inputs))
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for input := range work {
				if ctx.Err() != nil || rateLimited.Load() {
					continue
				}
				if err := a.service.SetImageAIStatus([]uint{input.PhotoID}, "analyzing", ""); err != nil {
					results <- outcome{input: input, err: err}
					continue
				}
				result, err := client.Analyze(ctx, input.Path)
				if ctx.Err() != nil {
					continue
				}
				if err == nil {
					err = photokeep.ValidateThaiAnalysis(result)
				}
				var httpErr *photokeep.ImageAIHTTPError
				if errors.As(err, &httpErr) && httpErr.StatusCode == 429 {
					rateLimited.Store(true)
				}
				results <- outcome{input: input, result: result, err: err}
			}
		}()
	}
	go func() {
	feed:
		for _, input := range inputs {
			if ctx.Err() != nil || rateLimited.Load() {
				break
			}
			select {
			case work <- input:
			case <-ctx.Done():
				break feed
			}
		}
		close(work)
		wg.Wait()
		close(results)
	}()
	var retryDelay time.Duration
	for item := range results {
		if ctx.Err() != nil {
			continue
		}
		if item.err != nil && photokeep.IsRetryableImageAIError(item.err) {
			if delay := imageAIRetryDelay(item.err); delay > retryDelay {
				retryDelay = delay
			}
		}
		job, err := a.service.RecordImageAIResult(jobID, item.input, item.result, item.err)
		if err != nil {
			_ = a.service.FailImageAIAnalysisJob(jobID, err.Error())
			continue
		}
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "image-ai:progress", job)
		}
	}
	if ctx.Err() != nil {
		pauseImageAIJob(a, jobID)
		return
	}
	if rateLimited.Load() {
		if err := a.service.DeferImageAIAnalysisJob(jobID, "พักคิวชั่วคราวเพราะ AI จำกัดคำขอ ระบบจะลองต่ออัตโนมัติ"); err == nil && a.ctx != nil {
			if job, getErr := a.service.ImageAIAnalysisJob(jobID); getErr == nil {
				runtime.EventsEmit(a.ctx, "image-ai:progress", job)
			}
		}
	}
	*resumeAfter = true
	*resumeDelay = retryDelay
}

func imageAIRetryDelay(err error) time.Duration {
	var httpErr *photokeep.ImageAIHTTPError
	if errors.As(err, &httpErr) {
		if httpErr.RetryAfter > 0 {
			return httpErr.RetryAfter
		}
	}
	return 20 * time.Second
}

func pauseImageAIJob(a *App, jobID uint) {
	a.imageAIControlMu.Lock()
	stoppedByUser := a.imageAIStopped
	a.imageAIControlMu.Unlock()
	if stoppedByUser {
		if err := a.service.PauseImageAIAnalysisJob(jobID); err == nil {
			if job, getErr := a.service.ImageAIAnalysisJob(jobID); getErr == nil && a.ctx != nil {
				runtime.EventsEmit(a.ctx, "image-ai:progress", job)
			}
		}
	}
}
