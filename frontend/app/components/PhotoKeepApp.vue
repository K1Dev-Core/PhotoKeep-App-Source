<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { AlertCircle } from '@lucide/vue'
import AppNavigation from './AppNavigation.vue'
import AppToast from './AppToast.vue'
import BackupProgressDialog from './BackupProgressDialog.vue'
import GalleryPanel from './GalleryPanel.vue'
import LoadingState from './LoadingState.vue'
import PhotoDetailDialog from './PhotoDetailDialog.vue'
import SettingsPanel from './SettingsPanel.vue'
import SetupDialog from './SetupDialog.vue'
import type { AppState, BackupProgress, BackupResult, Photo, RefreshResult, TagItem, View } from '../types/photo'
import '../assets/css/main.css'
import '../assets/css/theme.css'

const state = reactive<AppState>({ source: '', dest: '', total: 0, tags: [] })
const source = ref(''), dest = ref(''), query = ref(''), activeTag = ref('')
const photos = ref<Photo[]>([]), thumbnails = reactive<Record<number, string>>({})
const allPhotos = ref<Photo[]>([])
const thumbnailFailures = reactive<Record<number, boolean>>({})
const selected = ref<Photo | null>(null), fullImage = ref('')
const view = ref<View>('gallery')
const setupOpen = ref(false), setupDismissed = ref(false)
const busy = ref(false), busyAction = ref<'pick' | 'settings' | 'backup' | 'photo' | 'clear' | null>(null)
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
const tags = computed<TagItem[]>(() => {
  const counts = new Map<string, number>()
  for (const photo of allPhotos.value) {
    for (const tag of photo.tags.split(',')) {
      const value = tag.trim()
      if (value) counts.set(value, (counts.get(value) || 0) + 1)
    }
  }
  return [...counts].map(([name, count]) => ({ name, count })).sort((a, b) => b.count - a.count || a.name.localeCompare(b.name))
})
const api = () => (window as any).go?.main?.App

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
  if (!current.source || !current.dest) {
    if (!setupDismissed.value) setupOpen.value = true
  }
  await search()
}

let searchSeq = 0
async function search() {
  if (!api()) return
  const seq = ++searchSeq
  searching.value = true
  try {
    const result = await api().Search(query.value, activeTag.value)
    if (seq !== searchSeq) return
    photos.value = result.photos || []
    if (!query.value && !activeTag.value) allPhotos.value = photos.value
    const pending = photos.value.filter(photo => !thumbnails[photo.id] && !thumbnailFailures[photo.id])
    async function loadNext() {
      while (pending.length) {
        const photo = pending.shift()!
        try { thumbnails[photo.id] = await api().Preview(photo.id, true) }
        catch { thumbnailFailures[photo.id] = true }
      }
    }
    void Promise.all(Array.from({ length: Math.min(4, pending.length) }, loadNext))
  } catch (e: any) {
    if (seq === searchSeq) showToast('error', String(e?.message || e))
  } finally {
    if (seq === searchSeq) searching.value = false
  }
}

let timer: ReturnType<typeof setTimeout>
watch(query, () => { clearTimeout(timer); searchSeq++; searching.value = true; timer = setTimeout(search, 250) })
watch(activeTag, search)

async function pick(which: 'source' | 'dest') {
  const path = await run<string>(() => api().PickDirectory(), 'pick')
  if (path) { if (which === 'source') source.value = path; else dest.value = path }
}
async function saveSettings() {
  const saved = await run<AppState>(() => api().SaveSettings(source.value, dest.value), 'settings', 'บันทึกโฟลเดอร์แล้ว')
  if (saved) { Object.assign(state, saved); query.value = ''; activeTag.value = ''; setupOpen.value = false; view.value = 'gallery'; await search() }
}
async function clearDatabase() {
  if (!clearArmed.value) {
    clearArmed.value = true
    showToast('error', 'กดปุ่มถังขยะอีกครั้งเพื่อยืนยันล้างข้อมูลทั้งหมด')
    clearTimeout(clearTimer)
    clearTimer = setTimeout(() => { clearArmed.value = false }, 5000)
    return
  }
  clearTimeout(clearTimer)
  clearArmed.value = false
  const cleared = await run<AppState>(() => api().ClearDatabase(), 'clear', 'ล้างข้อมูลทั้งหมดแล้ว')
  if (cleared) { Object.assign(state, cleared); await search() }
}
async function refreshLibrary() {
  busy.value = true; busyAction.value = 'backup'; dismissToast()
  try {
    const result = await api().Refresh() as RefreshResult
    if (result.failed) showToast('error', `รีเฟรชเสร็จ แต่มี ${result.failed} ไฟล์ไม่สำเร็จ`)
    else if (result.new > 0) showToast('success', `พบรูปใหม่ ${result.new} รูป กดสำรองเพื่อคัดลอก`)
    else showToast('success', 'คลังภาพเป็นปัจจุบันแล้ว ไม่มีรูปใหม่')
    query.value = ''; activeTag.value = ''
    await refresh()
  } catch (e: any) {
    showToast('error', String(e?.message || e))
  } finally { busy.value = false; busyAction.value = null }
}
async function backup(mode: 'backup' | 'refresh' = 'backup') {
  if (!configured.value) { setupOpen.value = true; return }
  backupMode.value = mode
  if (mode === 'refresh') return refreshLibrary()
  backupDialogOpen.value = mode === 'backup'; backupDialogError.value = ''
  busy.value = true; busyAction.value = 'backup'; dismissToast()
  backupProgress.value = { stage: 'scanning', total: 0, completed: 0, added: 0, skipped: 0, failed: 0, currentFile: '' }
  backupElapsedMs.value = 0
  const started = performance.now()
  let completed = false
  const clock = setInterval(() => { if (!completed) backupElapsedMs.value = Math.round(performance.now() - started) }, 100)
  const runtimeBridge = (window as any).runtime as { EventsOn?: (event: string, callback: (progress: BackupProgress) => void) => () => void } | undefined
  const stopProgress = runtimeBridge?.EventsOn?.('backup:progress', (progress: BackupProgress) => {
    if (!completed && progress && typeof progress.completed === 'number') backupProgress.value = progress
  })
  try {
    const result = await api().Backup() as BackupResult
    completed = true
    backupElapsedMs.value = result.durationMs || Math.round(performance.now() - started)
    const total = Math.max(backupProgress.value.total, result.added + result.skipped + result.failed)
    backupProgress.value = { stage: 'done', total, completed: total, added: result.added, skipped: result.skipped, failed: result.failed, currentFile: '' }
    backupDialogError.value = result.errors?.join(' · ') || ''
    query.value = ''; activeTag.value = ''
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
  selected.value = photo; editDescription.value = photo.description; editTags.value = photo.tags; fullImage.value = ''
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
  if (updated) { selected.value = updated; allPhotos.value = allPhotos.value.map(photo => photo.id === updated.id ? updated : photo); await search() }
}
function navigate(next: View) { view.value = next; selected.value = null }
function closeSetup() { setupOpen.value = false; setupDismissed.value = true; dismissToast() }
function closeBackupDialog() { if (backupProgress.value.stage === 'done' || backupProgress.value.stage === 'error') backupDialogOpen.value = false }
function onKeydown(e: KeyboardEvent) {
  if (e.key !== 'Escape') return
  if (backupDialogOpen.value) closeBackupDialog()
  else if (selected.value) selected.value = null
  else if (setupOpen.value) closeSetup()
}
watch([setupOpen, initializing], async ([open, loading]) => { if (open && !loading) { await nextTick(); document.getElementById('setup-source')?.focus() } })
watch(backupDialogOpen, async open => { if (open) { await nextTick(); document.getElementById('backup-progress-title')?.focus() } })
watch(() => backupProgress.value.stage, async stage => { if (backupDialogOpen.value && (stage === 'done' || stage === 'error')) { await nextTick(); document.getElementById('backup-finish-button')?.focus() } })
onMounted(() => { document.addEventListener('keydown', onKeydown); refresh().catch(e => { showToast('error', String(e)) }).finally(() => { initializing.value = false }) })
onBeforeUnmount(() => { document.removeEventListener('keydown', onKeydown); clearTimeout(timer); clearTimeout(toastTimer) })
</script>

<template>
  <div class="shell">
    <AppNavigation :view="view" :configured="configured" @navigate="navigate" />
    <main class="main">
      <div class="main-body" :class="{ 'main-body-settings': view === 'settings' && !initializing }">
        <div v-if="!hasBridge" class="message error" role="alert"><AlertCircle :size="18" />เปิดด้วยแอป PhotoKeep เพื่อใช้งานไฟล์ในเครื่อง</div>
        <LoadingState v-if="initializing" />
        <SettingsPanel v-else-if="view === 'settings'" v-model:source="source" v-model:dest="dest" :can-save="canSave" :saving="busyAction === 'settings'" :clear-armed="clearArmed" :clearing="busyAction === 'clear'" @pick="pick" @save="saveSettings" @clear="clearDatabase" />
        <GalleryPanel v-else v-model:query="query" v-model:active-tag="activeTag" :configured="configured" :photos="photos" :total="allPhotos.length" :tags="tags" :thumbnails="thumbnails" :thumbnail-failures="thumbnailFailures" :searching="searching" :busy="busy || initializing || !hasBridge" :backup-mode="busyAction === 'backup' ? backupMode : null" @open-setup="setupOpen = true" @backup="backup" @open-photo="openPhoto" />
      </div>
    </main>
    <SetupDialog v-if="setupOpen && !initializing" v-model:source="source" v-model:dest="dest" :can-save="canSave" :saving="busyAction === 'settings'" @pick="pick" @close="closeSetup" @save="saveSettings" />
    <PhotoDetailDialog v-if="selected" v-model:description="editDescription" v-model:tags="editTags" :photo="selected" :full-image="fullImage" :preview-loading="previewLoading" :preview-failed="previewFailed" :busy="busy" :saving="busyAction === 'photo'" @close="selected = null" @save="savePhoto" />
    <BackupProgressDialog v-if="backupDialogOpen" :progress="backupProgress" :elapsed-ms="backupElapsedMs" :error-message="backupDialogError" :mode="backupMode" @close="closeBackupDialog" />
    <AppToast :toast="toast" @close="dismissToast" />
  </div>
</template>
