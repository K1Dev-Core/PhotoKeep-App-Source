<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { Check, Download, Search, Trash2, X } from '@lucide/vue'
import { ZodError } from 'zod'
import { parseOpenVSXThemes, searchResponseSchema, type ImportedTheme, type OpenVSXResult } from '../../utils/openvsx-theme'

const props = defineProps<{ themes: ImportedTheme[]; selectedId: string }>()
const emit = defineEmits<{ import: [theme: ImportedTheme]; select: [id: string]; remove: [id: string] }>()
const query = ref('')
const results = ref<OpenVSXResult[]>([])
const offset = ref(0)
const nextOffset = ref(0)
const previousOffsets = ref<number[]>([])
const loading = ref(false)
const importing = ref('')
const choices = ref<ImportedTheme[]>([])
const previewOpen = ref(false)
const previewItem = ref<OpenVSXResult | null>(null)
const previewCloseButton = ref<HTMLButtonElement | null>(null)
let previewTrigger: HTMLButtonElement | null = null
let inspectRequest = 0
const error = ref('')
const searched = ref(false)
const hasMore = ref(false)
const activeTab = ref<'discover' | 'installed'>('discover')
const api = () => (window as any).go?.main?.App

watch(query, () => {
  inspectRequest++
  closePreview(false)
  searched.value = false
  results.value = []
  choices.value = []
  hasMore.value = false
  previousOffsets.value = []
  error.value = ''
})

function installTheme(theme: ImportedTheme) {
  emit('import', theme)
  closePreview()
  activeTab.value = 'installed'
}

function closePreview(restoreFocus = true) {
  previewOpen.value = false
  choices.value = []
  previewItem.value = null
  if (restoreFocus) nextTick(() => previewTrigger?.focus())
}

function onPreviewKeydown(event: KeyboardEvent) {
  if (!previewOpen.value) return
  if (event.key === 'Escape') {
    event.preventDefault()
    closePreview()
  }
  if (event.key === 'Tab') {
    const dialog = document.querySelector<HTMLElement>('.openvsx-preview-dialog')
    const buttons = Array.from(dialog?.querySelectorAll<HTMLButtonElement>('button:not(:disabled)') ?? [])
    if (!buttons.length) return
    const first = buttons[0]!
    const last = buttons[buttons.length - 1]!
    if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
    else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
  }
}

watch(previewOpen, (open) => {
  if (open) {
    window.addEventListener('keydown', onPreviewKeydown)
    nextTick(() => previewCloseButton.value?.focus())
  } else window.removeEventListener('keydown', onPreviewKeydown)
})
onBeforeUnmount(() => window.removeEventListener('keydown', onPreviewKeydown))

async function searchThemes(nextPageOffset = 0, direction: 'new' | 'next' | 'previous' = 'new') {
  const term = query.value.trim()
  if (term.length < 2 || loading.value) return
  loading.value = true
  error.value = ''
  inspectRequest++
  closePreview(false)
  choices.value = []
  try {
    const response = searchResponseSchema.parse(await api().SearchOpenVSXThemes(term, nextPageOffset))
    if (term !== query.value.trim()) return
    if (direction === 'new') previousOffsets.value = []
    if (direction === 'next') previousOffsets.value = [...previousOffsets.value, offset.value]
    if (direction === 'previous') previousOffsets.value = previousOffsets.value.slice(0, -1)
    results.value = response.extensions
    offset.value = response.offset
    nextOffset.value = response.nextOffset
    hasMore.value = response.hasMore
    searched.value = true
  } catch (cause) {
    error.value = cause instanceof ZodError ? 'ข้อมูลค้นหาจาก Open VSX ไม่ถูกต้อง' : cause instanceof Error ? cause.message : String(cause)
  } finally { loading.value = false }
}

async function inspectTheme(item: OpenVSXResult, event: MouseEvent) {
  const request = ++inspectRequest
  previewTrigger = event.currentTarget as HTMLButtonElement
  importing.value = `${item.namespace}.${item.name}`
  error.value = ''
  closePreview(false)
  try {
    const response: unknown = await api().FetchOpenVSXThemePackage(item.namespace, item.name, item.version)
    if (request !== inspectRequest) return
    choices.value = parseOpenVSXThemes(response)
    previewItem.value = item
    previewOpen.value = true
  } catch (cause) {
    error.value = cause instanceof ZodError ? 'แพ็กเกจนี้ไม่มีชุดสีที่ PhotoKeep รองรับ (อาจเป็น Icon Theme)' : cause instanceof Error ? cause.message : String(cause)
  } finally { importing.value = '' }
}
</script>

<template>
  <section class="openvsx-settings" aria-labelledby="openvsx-title">
    <div class="openvsx-heading"><h2 id="openvsx-title">ธีม</h2><span>Open VSX</span></div>
    <div class="openvsx-tabs" role="tablist" aria-label="จัดการธีม">
      <button id="openvsx-discover-tab" type="button" role="tab" :aria-selected="activeTab === 'discover'" aria-controls="openvsx-discover-panel" @click="activeTab = 'discover'">ค้นหาและติดตั้ง</button>
      <button id="openvsx-installed-tab" type="button" role="tab" :aria-selected="activeTab === 'installed'" aria-controls="openvsx-installed-panel" @click="activeTab = 'installed'">ธีมของฉัน <span class="openvsx-count">{{ themes.length }}</span></button>
    </div>
    <div v-show="activeTab === 'discover'" id="openvsx-discover-panel" class="openvsx-tab-panel" role="tabpanel" aria-labelledby="openvsx-discover-tab">
      <form class="openvsx-search" @submit.prevent="searchThemes(0)"><label for="openvsx-search-input">ค้นหาธีมจาก Open VSX</label><div><input id="openvsx-search-input" v-model="query" type="search" maxlength="100" placeholder="ชื่อธีม เช่น Catppuccin" /><button type="submit" :disabled="loading || query.trim().length < 2"><Search :size="15" />{{ loading ? 'กำลังค้นหา…' : 'ค้นหา' }}</button></div></form>
      <p v-if="error" class="openvsx-error" role="alert">{{ error }}</p>
      <p v-if="searched && !results.length && !loading && !error" class="openvsx-note">{{ hasMore ? 'ไม่พบธีมในหน้านี้ ลองหน้าถัดไป' : 'ไม่พบธีมที่ตรงกับคำค้น' }}</p>
      <div v-if="searched && (results.length || hasMore || previousOffsets.length)" class="openvsx-results">
        <div v-for="item in results" :key="`${item.namespace}.${item.name}`" class="openvsx-result"><span><strong>{{ item.displayName || item.name }}</strong><small>{{ item.namespace }} · {{ item.version }}</small></span><button type="button" :disabled="!!importing" @click="inspectTheme(item, $event)">{{ importing === `${item.namespace}.${item.name}` ? 'กำลังโหลด…' : 'ดูชุดสี' }}</button></div>
        <div class="openvsx-pages"><button type="button" :disabled="loading || !previousOffsets.length" @click="searchThemes(previousOffsets[previousOffsets.length - 1] ?? 0, 'previous')">ก่อนหน้า</button><span>{{ results.length }} รายการ</span><button type="button" :disabled="loading || !hasMore" @click="searchThemes(nextOffset, 'next')">ถัดไป</button></div>
      </div>
    </div>
    <div v-show="activeTab === 'installed'" id="openvsx-installed-panel" class="openvsx-tab-panel" role="tabpanel" aria-labelledby="openvsx-installed-tab">
      <div v-if="themes.length" class="openvsx-saved" aria-label="ธีมที่ติดตั้งแล้ว">
        <div v-for="theme in themes" :key="theme.id" class="openvsx-saved-row">
          <span class="openvsx-swatch" :style="{ background: theme.palette.page, borderColor: theme.palette.border }"><i :style="{ background: theme.palette.accent }"></i></span>
          <span class="openvsx-saved-label"><strong>{{ theme.label }}</strong><small>{{ theme.publisher }} · {{ theme.dark ? 'มืด' : 'สว่าง' }}</small></span>
          <button type="button" class="openvsx-use" :aria-pressed="selectedId === theme.id" @click="emit('select', theme.id)"><Check v-if="selectedId === theme.id" :size="14" />{{ selectedId === theme.id ? 'ใช้อยู่' : 'ใช้ธีม' }}</button>
          <button type="button" class="openvsx-remove" :aria-label="`ลบธีม ${theme.label}`" :title="`ลบธีม ${theme.label}`" @click="emit('remove', theme.id)"><Trash2 :size="15" /></button>
        </div>
      </div>
      <div v-else class="openvsx-empty"><strong>ยังไม่มีธีมที่ติดตั้ง</strong><button type="button" @click="activeTab = 'discover'">ค้นหาธีม</button></div>
    </div>
  </section>
  <Teleport to="body">
    <div v-if="previewOpen" class="modal-scrim openvsx-preview-scrim" @click.self="closePreview()">
      <section class="openvsx-preview-dialog" role="dialog" aria-modal="true" aria-labelledby="openvsx-preview-title">
        <header class="openvsx-preview-header">
          <div><small>Open VSX · {{ previewItem?.namespace }}</small><h2 id="openvsx-preview-title">{{ previewItem?.displayName || previewItem?.name }}</h2></div>
          <button ref="previewCloseButton" type="button" class="openvsx-preview-close" aria-label="ปิดหน้าชุดสี" @click="closePreview()"><X :size="18" /></button>
        </header>
        <div class="openvsx-preview-list">
          <div v-for="theme in choices" :key="theme.id" class="openvsx-choice openvsx-preview-choice">
            <span class="openvsx-swatch" :style="{ background: theme.palette.page, borderColor: theme.palette.border }"><i :style="{ background: theme.palette.accent }"></i></span>
            <span><strong>{{ theme.label }}</strong><small>{{ theme.dark ? 'มืด' : 'สว่าง' }}</small></span>
            <div class="openvsx-preview-colors" aria-hidden="true"><i v-for="color in [theme.palette.page, theme.palette.surface, theme.palette.ink, theme.palette.accent]" :key="color" :style="{ background: color }"></i></div>
            <button type="button" @click="installTheme(theme)"><Download :size="14" />{{ themes.some(saved => saved.id === theme.id) ? 'ติดตั้งอีกครั้ง' : 'ติดตั้ง' }}</button>
          </div>
        </div>
      </section>
    </div>
  </Teleport>
</template>
