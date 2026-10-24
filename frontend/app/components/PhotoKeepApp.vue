<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { AlertCircle, Sparkles } from '@lucide/vue'
import AppNavigation from './shared/AppNavigation.vue'
import AppToast from './shared/AppToast.vue'
import BackupProgressDialog from './backup/BackupProgressDialog.vue'
import GalleryPanel from './gallery/GalleryPanel.vue'
import ImageAILogsPanel from './ai/ImageAILogsPanel.vue'
import LoadingState from './shared/LoadingState.vue'
import PhotoDetailDialog from './gallery/PhotoDetailDialog.vue'
import SettingsPanel from './settings/SettingsPanel.vue'
import SetupDialog from './backup/SetupDialog.vue'
import { usePhotoKeepTheme } from '../composables/usePhotoKeepTheme'
import type { AppState, BackupProgress, BackupResult, ImageAIAnalysisJob, ImageAIErrorLog, ImageAIQueueProgress, ImageAISettings, Photo, RefreshResult, View } from '../types/photo'
import '../assets/css/main.css'
import '../assets/css/theme.css'
import '../assets/css/appearance.css'
import '../assets/css/imported-theme.css'

const state = reactive<AppState>({ source: '', dest: '', keepOriginal: true, total: 0, tags: [] })
const source = ref(''), dest = ref(''), keepOriginal = ref(true), query = ref('')
const photos = ref<Photo[]>([]), thumbnails = reactive<Record<number, string>>({})
const searchTotal = ref(0), loadingMore = ref(false), loadingPrevious = ref(false), photosOffset = ref(0)
const thumbnailFailures = reactive<Record<number, boolean>>({})
const selected = ref<Photo | null>(null), fullImage = ref('')
const selectionMode = ref(false), selectedPhotoIds = ref<number[]>([]), aiSettings = ref<ImageAISettings>({ endpoint: '', searchEndpoint: '', provider: 'managed', model: '', keyCount: 0, concurrency: 8, configured: true })
const gatewayUrl = ref('https://vision.k1god.com/v1/analyze'), gatewaySaving = ref(false)
const analysisBusy = ref(false), currentImageAIJob = ref<ImageAIAnalysisJob | null>(null)
const analysisControlBusy = ref(false), imageAILogsLoading = ref(false), imageAIRetrying = ref(false)
const imageAIErrorLogs = ref<ImageAIErrorLog[]>([]), imageAIJobs = ref<ImageAIAnalysisJob[]>([])
const aiQueueProgress = ref<ImageAIQueueProgress>({ total: 0, completed: 0, pending: 0, analyzing: 0, failed: 0, elapsedSeconds: 0 })
const IMAGE_AI_ELAPSED_STORAGE_KEY = 'photokeep.image-ai.elapsed-seconds'
const imageAIConfirmOpen = ref(false)
let imageAIPollTimer: ReturnType<typeof setInterval> | undefined
let imageAIClockTimer: ReturnType<typeof setInterval> | undefined
let imageAIRefreshRunning = false
let imageAIQueueGeneration = 0
let lastAILogRefresh = 0
let imageAIEventOff: (() => void) | undefined
const view = ref<View>('gallery')
const { themePreference, importedThemes, selectedImportedThemeId, importTheme, selectTheme, removeTheme } = usePhotoKeepTheme(message => showToast('success', message))
const setupOpen = ref(false), setupDismissed = ref(false), setupMode = ref<'setup' | 'folder'>('setup')
const busy = ref(false), busyAction = ref<'pick' | 'settings' | 'ai' | 'backup' | 'photo' | 'clear' | null>(null)
const clearArmed = ref(false)
let clearTimer: ReturnType<typeof setTimeout> | undefined
const backupMode = ref<'backup' | 'refresh'>('backup')
const initializing = ref(true), searching = ref(false), previewLoading = ref(false), previewFailed = ref(false)
const hasBridge = ref(true)
const toast = ref<{ id: number; type: 'success' | 'error'; message: string; duration: number } | null>(null)
let toastTimer: ReturnType<typeof setTimeout> | undefined
let toastId = 0
function dismissToast() { clearTimeout(toastTimer); toast.value = null }
function showToast(type: 'success' | 'error', message: string) {
  clearTimeout(toastTimer)
  const duration = type === 'error' ? 5200 : 3600
  const id = ++toastId
  toast.value = { id, type, message, duration }
  toastTimer = setTimeout(() => { if (toast.value?.id === id) toast.value = null }, duration)
}
const editDescription = ref(''), editTags = ref('')
const backupProgress = ref<BackupProgress>({ stage: 'scanning', total: 0, completed: 0, added: 0, skipped: 0, failed: 0, currentFile: '' })
const backupElapsedMs = ref(0)
const backupDialogOpen = ref(false), backupDialogError = ref('')
const configured = computed(() => !!state.source && !!state.dest)
const canSave = computed(() => !!source.value.trim() && !!dest.value.trim() && !busy.value && hasBridge.value)
const folderSettingsBusy = computed(() => busy.value || analysisBusy.value || analysisControlBusy.value || (!aiQueueProgress.value.paused && (aiQueueProgress.value.pending > 0 || aiQueueProgress.value.analyzing > 0)))
const api = () => (window as any).go?.main?.App
const PHOTO_PAGE_SIZE = 96
const MAX_VISIBLE_PHOTOS = PHOTO_PAGE_SIZE * 3
const THUMBNAIL_BATCH_SIZE = 8
const THUMBNAIL_BATCH_CONCURRENCY = 3
const thumbnailQueue: Array<{ id: number; distance: number }> = []
const queuedThumbnailIds = new Set<number>()
const activeThumbnailIds = new Set<number>()
const thumbnailRetryCount = new Map<number, number>()
let activeThumbnailBatches = 0
let thumbnailPumpTimer: ReturnType<typeof setTimeout> | undefined
let backupGalleryRefreshTimer: ReturnType<typeof setTimeout> | undefined
let backupGalleryRefreshRunning = false
let backupGalleryRefreshPending = false
let lastBackupGalleryAdded = 0

function pumpThumbnailQueue(force = false) {
  clearTimeout(thumbnailPumpTimer)
  thumbnailPumpTimer = undefined
  thumbnailQueue.sort((a, b) => b.distance - a.distance)
  while (activeThumbnailBatches < THUMBNAIL_BATCH_CONCURRENCY && thumbnailQueue.length) {
    const batch: number[] = []
    while (batch.length < THUMBNAIL_BATCH_SIZE && thumbnailQueue.length) {
      const photoId = thumbnailQueue.pop()!.id
      queuedThumbnailIds.delete(photoId)
      if (thumbnails[photoId] || thumbnailFailures[photoId] || activeThumbnailIds.has(photoId)) continue
      activeThumbnailIds.add(photoId)
      batch.push(photoId)
    }
    if (batch.length < THUMBNAIL_BATCH_SIZE && !force) {
      batch.forEach(photoId => activeThumbnailIds.delete(photoId))
      thumbnailQueue.push(...batch.reverse().map(id => ({ id, distance: Number.POSITIVE_INFINITY })))
      batch.forEach(photoId => queuedThumbnailIds.add(photoId))
      thumbnailPumpTimer = setTimeout(() => pumpThumbnailQueue(true), 12)
      return
    }
    if (!batch.length) return
    activeThumbnailBatches++
    const retryIds: number[] = []
    void api().PreviewThumbnails(batch)
      .then((results: Array<{ photoId: number; dataUrl: string; error?: string }>) => {
        const completedIds = new Set<number>()
        for (const result of results || []) {
          completedIds.add(result.photoId)
          if (result.dataUrl) {
            thumbnails[result.photoId] = result.dataUrl
            thumbnailRetryCount.delete(result.photoId)
          } else retryIds.push(result.photoId)
        }
        batch.filter(photoId => !completedIds.has(photoId)).forEach(photoId => retryIds.push(photoId))
      })
      .catch(() => { retryIds.push(...batch) })
      .finally(() => {
        batch.forEach(photoId => activeThumbnailIds.delete(photoId))
        activeThumbnailBatches--
        if (retryIds.length) {
          setTimeout(() => {
            const visibleIds = new Set(photos.value.map(photo => photo.id))
            for (const photoId of retryIds) {
              const attempts = (thumbnailRetryCount.get(photoId) || 0) + 1
              thumbnailRetryCount.set(photoId, attempts)
              if (attempts >= 3) thumbnailFailures[photoId] = true
              else if (visibleIds.has(photoId)) requestThumbnail(photoId)
            }
          }, 400)
        }
        pumpThumbnailQueue(true)
      })
  }
}

function requestThumbnail(photoId: number, distance = Number.POSITIVE_INFINITY) {
  if (!photoId || thumbnails[photoId] || thumbnailFailures[photoId] || activeThumbnailIds.has(photoId)) return
  if (queuedThumbnailIds.has(photoId)) {
    const queued = thumbnailQueue.find(item => item.id === photoId)
    if (queued) queued.distance = Math.min(queued.distance, distance)
    return
  }
  thumbnailQueue.push({ id: photoId, distance })
  queuedThumbnailIds.add(photoId)
  if (!thumbnailPumpTimer) thumbnailPumpTimer = setTimeout(() => pumpThumbnailQueue(true), 8)
}

function clearQueuedThumbnails() {
  clearTimeout(thumbnailPumpTimer)
  thumbnailPumpTimer = undefined
  thumbnailQueue.length = 0
  queuedThumbnailIds.clear()
}

async function run<T>(task: () => Promise<T>, action: NonNullable<typeof busyAction.value>, message = ''): Promise<T | undefined> {
  busy.value = true; busyAction.value = action; dismissToast()
  try {
    const result = await task()
    if (message) showToast('success', message)
    return result
  } catch (e: any) {
    showToast('error', String(e?.message || e))
    return undefined
  } finally { busy.value = false; busyAction.value = null }
}

async function refresh() {
  if (!api()) { hasBridge.value = false; return }
  const current = await api().GetState() as AppState
  Object.assign(state, current)
  source.value = current.source || ''
  dest.value = current.dest || ''
  keepOriginal.value = current.keepOriginal ?? true
  if (!current.source || !current.dest) {
    setupMode.value = 'setup'
    if (!setupDismissed.value) setupOpen.value = true
  }
  aiSettings.value = await api().GetImageAISettings() as ImageAISettings
  gatewayUrl.value = aiSettings.value.endpoint || 'https://vision.k1god.com/v1/analyze'
  await updateImageAIProgress(false)
  await search()
}

let searchSeq = 0
async function search() {
  if (!api()) return
  const seq = ++searchSeq
  clearQueuedThumbnails()
  const searchQuery = query.value
  searching.value = true
  loadingMore.value = false
  loadingPrevious.value = false
  photosOffset.value = 0
  photos.value = []
  searchTotal.value = 0
  try {
    const result = await api().SearchPage(searchQuery, '', photosOffset.value, PHOTO_PAGE_SIZE)
    if (seq !== searchSeq) return
    photos.value = result.photos || []
    searchTotal.value = result.total || 0
  } catch (e: any) {
    if (seq === searchSeq) showToast('error', String(e?.message || e))
  } finally {
    if (seq === searchSeq) searching.value = false
    if (seq === searchSeq && backupGalleryRefreshPending) scheduleBackupGalleryRefresh()
  }
}

function scheduleBackupGalleryRefresh() {
  backupGalleryRefreshPending = true
  if (backupGalleryRefreshTimer || backupGalleryRefreshRunning) return
  backupGalleryRefreshTimer = setTimeout(async () => {
    backupGalleryRefreshTimer = undefined
    if (!backupGalleryRefreshPending || view.value !== 'gallery' || searching.value || !api()) return
    backupGalleryRefreshPending = false
    backupGalleryRefreshRunning = true
    const seq = searchSeq
    const searchQuery = query.value
    try {
      const result = await api().SearchPage(searchQuery, '', photosOffset.value, PHOTO_PAGE_SIZE)
      if (seq !== searchSeq || searchQuery !== query.value) return
      const firstPage = result.photos || []
      if (searchQuery) {
        photos.value = firstPage
      } else if (photosOffset.value === 0) {
        const firstPageIds = new Set(firstPage.map((photo: Photo) => photo.id))
        photos.value = [...firstPage, ...photos.value.filter(photo => !firstPageIds.has(photo.id))].slice(0, MAX_VISIBLE_PHOTOS)
      }
      searchTotal.value = result.total || 0
    } catch (e: any) {
      showToast('error', String(e?.message || e))
    } finally {
      backupGalleryRefreshRunning = false
      if (backupGalleryRefreshPending) scheduleBackupGalleryRefresh()
    }
  }, 350)
}

async function loadMorePhotos() {
  if (loadingMore.value || loadingPrevious.value || searching.value || photosOffset.value + photos.value.length >= searchTotal.value || !api()) return
  const seq = searchSeq
  loadingMore.value = true
  try {
    const result = await api().SearchPage(query.value, '', photosOffset.value + photos.value.length, PHOTO_PAGE_SIZE)
    if (seq !== searchSeq) return
    const existing = new Set(photos.value.map(photo => photo.id))
    const nextPhotos = (result.photos || []).filter((photo: Photo) => !existing.has(photo.id))
    const scroll = document.querySelector<HTMLElement>('.gallery-scroll-area')
    const viewport = scroll?.getBoundingClientRect()
    const anchor = scroll && viewport
      ? Array.from(scroll.querySelectorAll<HTMLElement>('.photo-card[data-photo-id]')).find(card => {
          const rect = card.getBoundingClientRect()
          return rect.bottom > viewport.top + 12 && rect.top < viewport.bottom
        })
      : undefined
    const anchorTop = anchor?.getBoundingClientRect().top
    const combined = [...photos.value, ...nextPhotos]
    const removed = Math.max(0, combined.length - MAX_VISIBLE_PHOTOS)
    photos.value = removed ? combined.slice(removed) : combined
    photosOffset.value += removed
    if (removed && scroll && anchor && anchorTop !== undefined) {
      await nextTick()
      if (seq === searchSeq && anchor.isConnected) scroll.scrollTop += anchor.getBoundingClientRect().top - anchorTop
    }
  } catch (e: any) {
    if (seq === searchSeq) showToast('error', String(e?.message || e))
  } finally {
    if (seq === searchSeq) loadingMore.value = false
  }
}

async function loadPreviousPhotos() {
  if (loadingPrevious.value || loadingMore.value || searching.value || photosOffset.value === 0 || !api()) return
  const seq = searchSeq
  const oldOffset = photosOffset.value
  const nextOffset = Math.max(0, oldOffset - PHOTO_PAGE_SIZE)
  const scroll = document.querySelector<HTMLElement>('.gallery-scroll-area')
  const viewport = scroll?.getBoundingClientRect()
  const anchor = scroll && viewport
    ? Array.from(scroll.querySelectorAll<HTMLElement>('.photo-card[data-photo-id]')).find(card => {
        const rect = card.getBoundingClientRect()
        return rect.bottom > viewport.top + 12 && rect.top < viewport.bottom
      })
    : undefined
  const anchorTop = anchor?.getBoundingClientRect().top
  const oldScrollHeight = scroll?.scrollHeight || 0
  loadingPrevious.value = true
  try {
    const result = await api().SearchPage(query.value, '', nextOffset, oldOffset - nextOffset)
    if (seq !== searchSeq || oldOffset !== photosOffset.value) return
    const existing = new Set(photos.value.map(photo => photo.id))
    const previousPhotos = (result.photos || []).filter((photo: Photo) => !existing.has(photo.id))
    photos.value = [...previousPhotos, ...photos.value].slice(0, MAX_VISIBLE_PHOTOS)
    photosOffset.value = nextOffset
    searchTotal.value = result.total || 0
    await nextTick()
    const keepAnchor = () => {
      if (!scroll || seq !== searchSeq) return
      if (anchor && anchorTop !== undefined && anchor.isConnected) {
        scroll.scrollTop += anchor.getBoundingClientRect().top - anchorTop
      } else {
        scroll.scrollTop += scroll.scrollHeight - oldScrollHeight
      }
    }
    keepAnchor()
    await new Promise<void>(resolve => requestAnimationFrame(() => resolve()))
    keepAnchor()
  } catch (e: any) {
    if (seq === searchSeq) showToast('error', String(e?.message || e))
  } finally {
    if (seq === searchSeq) loadingPrevious.value = false
  }
}

async function seekPhotos(offset: number) {
  if (!api() || searching.value || !searchTotal.value) return
  clearQueuedThumbnails()
  const target = Math.min(searchTotal.value - 1, Math.max(0, Math.floor(offset)))
  const seq = ++searchSeq
  searching.value = true
  loadingMore.value = false
  loadingPrevious.value = false
  try {
    const result = await api().SearchPage(query.value, '', target, PHOTO_PAGE_SIZE)
    if (seq !== searchSeq) return
    photosOffset.value = target
    photos.value = result.photos || []
    searchTotal.value = result.total || 0
    await nextTick()
    document.querySelector<HTMLElement>('.gallery-scroll-area')?.scrollTo({ top: 0, behavior: 'auto' })
  } catch (e: any) {
    if (seq === searchSeq) showToast('error', String(e?.message || e))
  } finally {
    if (seq === searchSeq) searching.value = false
  }
}

let timer: ReturnType<typeof setTimeout>
watch(query, () => { clearTimeout(timer); searchSeq++; photos.value = []; searchTotal.value = 0; loadingMore.value = false; loadingPrevious.value = false; searching.value = true; timer = setTimeout(search, 220) })

async function pick(which: 'source' | 'dest') {
  const path = await run<string>(() => api().PickDirectory(), 'pick')
  if (path) { if (which === 'source') source.value = path; else dest.value = path }
}
async function saveSettings() {
  if (setupOpen.value && setupMode.value === 'folder' && folderSettingsBusy.value) return
  const saved = await run<AppState>(() => api().SaveBackupSettings(source.value, dest.value, keepOriginal.value), 'settings', 'บันทึกโฟลเดอร์แล้ว')
  if (saved) { Object.assign(state, saved); query.value = ''; setupOpen.value = false; view.value = 'gallery'; await search() }
}
async function saveGatewayUrl() {
  if (!api() || gatewaySaving.value) return
  gatewaySaving.value = true
  try {
    aiSettings.value = await api().SaveImageAIGatewayURL(gatewayUrl.value) as ImageAISettings
    gatewayUrl.value = aiSettings.value.endpoint
    showToast('success', 'บันทึกที่อยู่ API วิเคราะห์ภาพแล้ว')
  } catch (e: any) {
    showToast('error', String(e?.message || e))
  } finally {
    gatewaySaving.value = false
  }
}
async function updateImageAIProgress(refreshPhotos = true) {
  if (!api() || imageAIRefreshRunning) return
  imageAIRefreshRunning = true
  const generation = imageAIQueueGeneration
  try {
    const next = await api().GetImageAIQueueProgress() as ImageAIQueueProgress
    if (generation === imageAIQueueGeneration) applyImageAIQueueProgress(next)
    if (generation === imageAIQueueGeneration && refreshPhotos && view.value === 'gallery' && !searching.value && photos.value.length) {
      const seq = searchSeq
      const offset = photosOffset.value
      const searchQuery = query.value
      const result = await api().SearchPage(searchQuery, '', offset, photos.value.length)
      if (generation !== imageAIQueueGeneration || seq !== searchSeq || offset !== photosOffset.value || searchQuery !== query.value) return
      const updates = new Map<number, Photo>((result.photos || []).map((photo: Photo) => [photo.id, photo]))
      let changed = false
      const merged = photos.value.map(photo => {
        const update = updates.get(photo.id)
        if (!update || (photo.description === update.description && photo.tags === update.tags && photo.aiStatus === update.aiStatus && photo.aiError === update.aiError)) return photo
        changed = true
        return update
      })
      if (changed) photos.value = merged
      searchTotal.value = result.total || 0
    }
  } finally { imageAIRefreshRunning = false }
}
function applyImageAIQueueProgress(next: ImageAIQueueProgress) {
  const previous = aiQueueProgress.value
  const previousActive = previous.pending > 0 || previous.analyzing > 0
  const nextActive = next.pending > 0 || next.analyzing > 0
  const backendElapsed = Math.max(0, next.elapsedSeconds || 0)
  if (!nextActive) {
    aiQueueProgress.value = { ...next, elapsedSeconds: backendElapsed }
    try { localStorage.removeItem(IMAGE_AI_ELAPSED_STORAGE_KEY) } catch {}
    return
  }

  let savedElapsed = 0
  try { savedElapsed = Math.max(0, Number(localStorage.getItem(IMAGE_AI_ELAPSED_STORAGE_KEY)) || 0) } catch {}
  const elapsedSeconds = Math.max(backendElapsed, previousActive ? previous.elapsedSeconds : savedElapsed)
  aiQueueProgress.value = { ...next, elapsedSeconds }
  try { localStorage.setItem(IMAGE_AI_ELAPSED_STORAGE_KEY, String(elapsedSeconds)) } catch {}
}
async function analyzeSelected() {
  if (!selectedPhotoIds.value.length || analysisBusy.value) return
  imageAIConfirmOpen.value = true
}
async function confirmImageAIAnalysis() {
  if (!selectedPhotoIds.value.length || analysisBusy.value) return
  imageAIConfirmOpen.value = false
  aiQueueProgress.value = { ...aiQueueProgress.value, elapsedSeconds: 0 }
  try { localStorage.setItem(IMAGE_AI_ELAPSED_STORAGE_KEY, '0') } catch {}
  analysisBusy.value = true; dismissToast()
  try {
    currentImageAIJob.value = await api().StartImageAIAnalysis(selectedPhotoIds.value) as ImageAIAnalysisJob
    selectionMode.value = false; selectedPhotoIds.value = []
    showToast('success', `เริ่มวิเคราะห์ ${currentImageAIJob.value.total} รูปแล้ว`)
    await updateImageAIProgress(true)
  } catch (e: any) { showToast('error', String(e?.message || e)) }
  finally { analysisBusy.value = false }
}
async function refreshImageAIQueue() {
  if (!api()) return
  const generation = imageAIQueueGeneration
  try {
    const next = await api().GetImageAIQueueProgress() as ImageAIQueueProgress
    if (generation !== imageAIQueueGeneration) return
    applyImageAIQueueProgress(next)
    if (aiQueueProgress.value.analyzing > 0) await updateImageAIProgress(true)
    if (view.value === 'logs' && Date.now() - lastAILogRefresh > 10000) await loadImageAILogs()
  } catch (e: any) { showToast('error', String(e?.message || e)) }
}
async function loadImageAILogs() {
  if (!api() || imageAILogsLoading.value) return
  imageAILogsLoading.value = true
  lastAILogRefresh = Date.now()
  try {
    const [logs, jobs] = await Promise.all([api().GetImageAIErrorLogs(), api().ListImageAIAnalysisJobs()])
    imageAIErrorLogs.value = logs || []
    imageAIJobs.value = jobs || []
  } catch (e: any) { showToast('error', String(e?.message || e)) }
  finally { imageAILogsLoading.value = false }
}
async function retryFailedImageAI(photoIds: number[]) {
  if (!api() || imageAIRetrying.value || !photoIds.length) return
  imageAIRetrying.value = true
  try {
    const count = await api().RetryFailedImageAIAnalysis(photoIds) as number
    showToast('success', `ส่ง ${count} รูปกลับเข้าคิววิเคราะห์แล้ว`)
    await Promise.all([loadImageAILogs(), refreshImageAIQueue()])
  } catch (e: any) { showToast('error', String(e?.message || e)) }
  finally { imageAIRetrying.value = false }
}
async function stopImageAIAnalysis() {
  if (analysisControlBusy.value) return
  analysisControlBusy.value = true
  try {
    await api().StopImageAIAnalysis()
    showToast('success', 'กำลังหยุดวิเคราะห์ งานที่เหลือยังอยู่ในคิว')
    await updateImageAIProgress(false)
  } catch (e: any) { showToast('error', String(e?.message || e)) }
  finally { analysisControlBusy.value = false }
}
async function continueImageAIAnalysis() {
  if (analysisControlBusy.value) return
  analysisControlBusy.value = true
  try {
    currentImageAIJob.value = await api().ContinueImageAIAnalysis() as ImageAIAnalysisJob
    showToast('success', 'ทำงานวิเคราะห์ต่อแล้ว')
    await updateImageAIProgress(true)
  } catch (e: any) { showToast('error', String(e?.message || e)) }
  finally { analysisControlBusy.value = false }
}
function togglePhotoSelection(photo: Photo) {
  const ids = new Set(selectedPhotoIds.value)
  if (ids.has(photo.id)) ids.delete(photo.id); else ids.add(photo.id)
  selectedPhotoIds.value = [...ids]
}
function selectVisiblePhotos(visible: Photo[]) {
  selectedPhotoIds.value = [...new Set([...selectedPhotoIds.value, ...visible.map(photo => photo.id)])]
}
function toggleSelectionMode() {
  selectionMode.value = !selectionMode.value
  selectedPhotoIds.value = []
}
async function clearDatabase() {
  if (!clearArmed.value) {
    clearArmed.value = true
    showToast('error', state.dest
      ? `กดอีกครั้งเพื่อยืนยัน: ลบไฟล์รูปภาพใน ${state.dest} และล้างข้อมูลในคลัง`
      : 'กดอีกครั้งเพื่อยืนยันการล้างข้อมูลในคลัง')
    clearTimeout(clearTimer)
    clearTimer = setTimeout(() => { clearArmed.value = false }, 5000)
    return
  }
  clearTimeout(clearTimer)
  clearArmed.value = false
  const cleared = await run<AppState>(() => api().ClearDatabase(), 'clear', 'ล้างข้อมูลทั้งหมดแล้ว')
  if (cleared) {
    imageAIQueueGeneration++
    Object.assign(state, cleared)
    currentImageAIJob.value = null
    aiQueueProgress.value = { total: 0, completed: 0, pending: 0, analyzing: 0, failed: 0, elapsedSeconds: 0, paused: false }
    try { localStorage.removeItem(IMAGE_AI_ELAPSED_STORAGE_KEY) } catch {}
    await search()
  }
}
async function refreshLibrary() {
  busy.value = true; busyAction.value = 'backup'; dismissToast()
  try {
    const result = await api().Refresh() as RefreshResult
    if (result.failed) showToast('error', `รีเฟรชเสร็จ แต่มี ${result.failed} ไฟล์ไม่สำเร็จ`)
    else if (result.new > 0) showToast('success', `พบรูปใหม่ ${result.new} รูป กด${state.keepOriginal ? 'สำรอง' : 'ย้าย'}รูปเพื่อเพิ่มเข้าคลัง`)
    else showToast('success', 'คลังภาพเป็นปัจจุบันแล้ว ไม่มีรูปใหม่')
    query.value = ''
    await refresh()
  } catch (e: any) {
    showToast('error', String(e?.message || e))
  } finally { busy.value = false; busyAction.value = null }
}
async function backup(mode: 'backup' | 'refresh' = 'backup') {
  if (!configured.value) { openInitialSetup(); return }
  backupMode.value = mode
  if (mode === 'refresh') return refreshLibrary()
  backupDialogOpen.value = mode === 'backup'; backupDialogError.value = ''
  busy.value = true; busyAction.value = 'backup'; dismissToast()
  backupProgress.value = { stage: 'scanning', total: 0, completed: 0, added: 0, skipped: 0, failed: 0, currentFile: '' }
  lastBackupGalleryAdded = 0
  backupGalleryRefreshPending = false
  backupElapsedMs.value = 0
  const started = performance.now()
  let completed = false
  const clock = setInterval(() => { if (!completed) backupElapsedMs.value = Math.round(performance.now() - started) }, 100)
  const runtimeBridge = (window as any).runtime as { EventsOn?: (event: string, callback: (progress: BackupProgress) => void) => () => void } | undefined
  const stopProgress = runtimeBridge?.EventsOn?.('backup:progress', (progress: BackupProgress) => {
    if (!completed && progress && typeof progress.completed === 'number') {
      backupProgress.value = progress
      if (progress.stage === 'copying' && progress.added > lastBackupGalleryAdded) {
        lastBackupGalleryAdded = progress.added
        scheduleBackupGalleryRefresh()
      }
    }
  })
  try {
    const result = await api().Backup() as BackupResult
    completed = true
    backupElapsedMs.value = result.durationMs || Math.round(performance.now() - started)
    const total = Math.max(backupProgress.value.total, result.added + result.skipped + result.failed)
    backupProgress.value = { stage: 'done', total, completed: total, added: result.added, skipped: result.skipped, failed: result.failed, currentFile: '' }
    backupDialogError.value = result.errors?.join(' · ') || ''
    query.value = ''
    await refresh()
  } catch (e: any) {
    completed = true
    backupElapsedMs.value = Math.round(performance.now() - started)
    backupDialogError.value = String(e?.message || e)
    backupProgress.value = { ...backupProgress.value, stage: 'error', currentFile: '' }
  }
  finally { stopProgress?.(); clearInterval(clock); busy.value = false; busyAction.value = null }
}
let previewSeq = 0
async function openPhoto(photo: Photo) {
  const seq = ++previewSeq
  selected.value = photo; editDescription.value = photo.description; editTags.value = photo.tags; fullImage.value = thumbnails[photo.id] || ''
  previewLoading.value = true; previewFailed.value = false
  try {
    const image = await api().Preview(photo.id, false)
    if (seq === previewSeq && selected.value?.id === photo.id) fullImage.value = image
  } catch (e: any) {
    if (seq === previewSeq) { previewFailed.value = true; showToast('error', String(e?.message || e)) }
  } finally { if (seq === previewSeq) previewLoading.value = false }
}
async function savePhoto() {
  if (!selected.value) return
  const updated = await run<Photo>(() => api().UpdatePhoto(selected.value!.id, editDescription.value, editTags.value), 'photo', 'บันทึกแล้ว')
  if (updated) { selected.value = updated; await search() }
}
function navigate(next: View) { view.value = next; selected.value = null; if (next === 'logs') void loadImageAILogs() }
function openFolderSettings() {
  if (folderSettingsBusy.value || !hasBridge.value) return
  source.value = state.source
  dest.value = state.dest
  keepOriginal.value = state.keepOriginal
  setupMode.value = 'folder'
  setupOpen.value = true
}
async function openBackupFolder() {
  if (folderSettingsBusy.value || !state.dest || !hasBridge.value) return
  try { await api().OpenBackupFolder() }
  catch (e: any) { showToast('error', String(e?.message || e)) }
}
function openInitialSetup() { setupMode.value = 'setup'; setupOpen.value = true }
function closeSetup() { setupOpen.value = false; setupDismissed.value = true; source.value = state.source; dest.value = state.dest; keepOriginal.value = state.keepOriginal; dismissToast() }
function closeBackupDialog() { if (backupProgress.value.stage === 'done' || backupProgress.value.stage === 'error') backupDialogOpen.value = false }
function onKeydown(e: KeyboardEvent) {
  if (e.key !== 'Escape') return
  if (imageAIConfirmOpen.value) imageAIConfirmOpen.value = false
  else if (backupDialogOpen.value) closeBackupDialog()
  else if (selected.value) selected.value = null
  else if (setupOpen.value) closeSetup()
}
watch([setupOpen, initializing], async ([open, loading]) => { if (open && !loading) { await nextTick(); document.getElementById('setup-source')?.focus() } })
onMounted(() => {
  document.addEventListener('keydown', onKeydown)
  const runtimeBridge = (window as any).runtime as { EventsOn?: (event: string, callback: () => void) => () => void } | undefined
  imageAIEventOff = runtimeBridge?.EventsOn?.('image-ai:progress', () => { void updateImageAIProgress(true) })
  imageAIPollTimer = setInterval(() => { void refreshImageAIQueue() }, 2200)
  imageAIClockTimer = setInterval(() => {
    if (aiSettings.value.configured && !aiQueueProgress.value.paused && (aiQueueProgress.value.pending > 0 || aiQueueProgress.value.analyzing > 0)) {
      const elapsedSeconds = aiQueueProgress.value.elapsedSeconds + 1
      aiQueueProgress.value = { ...aiQueueProgress.value, elapsedSeconds }
      try { localStorage.setItem(IMAGE_AI_ELAPSED_STORAGE_KEY, String(elapsedSeconds)) } catch {}
    }
  }, 1000)
  refresh().catch(e => { showToast('error', String(e)) }).finally(() => { initializing.value = false })
})
onBeforeUnmount(() => { document.removeEventListener('keydown', onKeydown); clearTimeout(timer); clearTimeout(toastTimer); clearTimeout(backupGalleryRefreshTimer); clearInterval(imageAIPollTimer); clearInterval(imageAIClockTimer); imageAIEventOff?.() })
</script>

<template>
  <div class="shell" :class="{ 'detail-open': !!selected }">
    <AppNavigation :view="view" :configured="configured" :folder-settings-busy="folderSettingsBusy" @navigate="navigate" @open-folder-settings="openFolderSettings" />
    <main class="main">
      <div class="main-body" :class="{ 'main-body-settings': view === 'settings' && !initializing, 'gallery-body': view === 'gallery' && configured && !initializing }">
        <div v-if="!hasBridge" class="message error" role="alert"><AlertCircle :size="18" />เปิดด้วยแอป PhotoKeep เพื่อใช้งานไฟล์ในเครื่อง</div>
        <LoadingState v-if="initializing" />
        <Transition name="page-switch"><SettingsPanel v-if="!initializing && view === 'settings'" v-model:theme-preference="themePreference" v-model:source="source" v-model:dest="dest" v-model:keep-original="keepOriginal" v-model:gateway-url="gatewayUrl" :can-save="canSave && !folderSettingsBusy" :saving="busyAction === 'settings'" :gateway-saving="gatewaySaving" :clear-armed="clearArmed" :clearing="busyAction === 'clear'" :imported-themes="importedThemes" :selected-imported-theme-id="selectedImportedThemeId" @import-theme="importTheme" @select-theme="selectTheme" @remove-theme="removeTheme" @pick="pick" @save="saveSettings" @save-gateway="saveGatewayUrl" @clear="clearDatabase" /></Transition>
        <Transition name="page-switch"><ImageAILogsPanel v-if="!initializing && view === 'logs'" :logs="imageAIErrorLogs" :jobs="imageAIJobs" :loading="imageAILogsLoading" :retrying="imageAIRetrying" @refresh="loadImageAILogs" @retry="retryFailedImageAI" /></Transition>
        <GalleryPanel v-if="!initializing" v-show="view === 'gallery'" v-model:query="query" :configured="configured" :photos="photos" :result-total="searchTotal" :start-offset="photosOffset" :loading-more="loadingMore" :loading-previous="loadingPrevious" :thumbnails="thumbnails" :thumbnail-failures="thumbnailFailures" :searching="searching" :busy="busy || initializing || !hasBridge" :backup-active="busyAction === 'backup'" :keep-original="state.keepOriginal" :backup-mode="busyAction === 'backup' ? backupMode : null" :selection-mode="selectionMode" :selected-photo-ids="selectedPhotoIds" :analysis-busy="analysisBusy" :analysis-control-busy="analysisControlBusy" :ai-configured="aiSettings.configured" :ai-progress="aiQueueProgress" @open-setup="openInitialSetup" @backup="backup" @open-photo="openPhoto" @selection-mode="toggleSelectionMode" @toggle-selection="togglePhotoSelection" @select-visible="selectVisiblePhotos" @load-more="loadMorePhotos" @load-previous="loadPreviousPhotos" @seek-photos="seekPhotos" @request-thumbnail="requestThumbnail" @analyze-selected="analyzeSelected" @stop-analysis="stopImageAIAnalysis" @continue-analysis="continueImageAIAnalysis" />
      </div>
    </main>
    <SetupDialog v-if="setupOpen && !initializing" v-model:source="source" v-model:dest="dest" v-model:keep-original="keepOriginal" :mode="setupMode" :can-save="canSave && !folderSettingsBusy" :saving="busyAction === 'settings'" @pick="pick" @close="closeSetup" @save="saveSettings" />
    <div v-if="imageAIConfirmOpen" class="modal-scrim image-ai-confirm-scrim" @click.self="imageAIConfirmOpen = false">
      <section class="image-ai-confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="image-ai-confirm-title" aria-describedby="image-ai-confirm-description">
        <div class="image-ai-confirm-mark"><Sparkles :size="21" /></div>
        <span class="image-ai-confirm-kicker">PHOTOKEEP AI</span>
        <h2 id="image-ai-confirm-title">วิเคราะห์ซ้ำ {{ selectedPhotoIds.length }} รูป</h2>
        <p id="image-ai-confirm-description">ภาพที่เลือกจะถูกส่งไปยัง AI server เมื่อวิเคราะห์สำเร็จ คำอธิบายและแท็กใหม่จะแทนข้อมูลเดิม</p>
        <div class="image-ai-confirm-actions">
          <button class="secondary" type="button" @click="imageAIConfirmOpen = false">ยกเลิก</button>
          <button class="primary" type="button" :disabled="analysisBusy" @click="confirmImageAIAnalysis"><Sparkles :size="16" />เริ่มวิเคราะห์</button>
        </div>
      </section>
    </div>
    <PhotoDetailDialog v-if="selected" v-model:description="editDescription" v-model:tags="editTags" :photo="selected" :full-image="fullImage" :preview-loading="previewLoading" :preview-failed="previewFailed" :busy="busy" :saving="busyAction === 'photo'" @close="selected = null" @save="savePhoto" />
    <Transition name="backup-strip"><BackupProgressDialog v-if="backupDialogOpen && !setupOpen" :progress="backupProgress" :elapsed-ms="backupElapsedMs" :error-message="backupDialogError" :mode="backupMode" :keep-original="state.keepOriginal" :source-path="state.source" :destination-path="state.dest" :folder-settings-busy="folderSettingsBusy" @close="closeBackupDialog" @open-backup-folder="openBackupFolder" /></Transition>
    <AppToast :toast="toast" @close="dismissToast" />
  </div>
</template>
