export type Photo = {
  id: number
  sourcePath: string
  backupPath: string
  filename: string
  description: string
  tags: string
  width: number
  height: number
  size: number
  takenAt: string
  backedUpAt: string
}

export type TagItem = { name: string; count: number }
export type BackupResult = { added: number; skipped: number; failed: number; errors: string[]; durationMs: number }
export type BackupProgress = {
  stage: 'scanning' | 'copying' | 'done' | 'error'
  total: number
  completed: number
  added: number
  skipped: number
  failed: number
  currentFile: string
}
export type AppState = { source: string; dest: string; total: number; tags: TagItem[] }
export type View = 'gallery' | 'settings'
