export type Photo = {
  id: number
  sourcePath: string
  backupPath: string
  filename: string
  description: string
  tags: string
  aiStatus: 'pending' | 'analyzing' | 'completed' | 'failed' | ''
  aiError: string
  width: number
  height: number
  size: number
  takenAt: string
  backedUpAt: string
}

export type TagItem = { name: string; count: number }
export type BackupResult = { added: number; skipped: number; failed: number; errors: string[]; durationMs: number }
export type RefreshResult = { total: number; new: number; known: number; failed: number; errors: string[] }
export type BackupProgress = {
  stage: 'scanning' | 'copying' | 'done' | 'error'
  total: number
  completed: number
  added: number
  skipped: number
  failed: number
  currentFile: string
}
export type AppState = { source: string; dest: string; keepOriginal: boolean; total: number; tags: TagItem[] }
export type ImageAIAnalysisJob = { id: number; status: string; total: number; completed: number; failed: number; finished: boolean; error: string; durationSeconds: number; createdAt: string }
export type ImageAIQueueProgress = { total: number; completed: number; pending: number; analyzing: number; failed: number; elapsedSeconds: number; paused?: boolean; error?: string }
export type ImageAIErrorLog = { photoId: number; filename: string; kind: string; error: string; loggedAt: string }
export type ImageAISettings = { endpoint: string; searchEndpoint: string; provider: 'managed'; model: string; keyCount: number; concurrency: number; configured: boolean }
export type View = 'gallery' | 'logs' | 'settings'
