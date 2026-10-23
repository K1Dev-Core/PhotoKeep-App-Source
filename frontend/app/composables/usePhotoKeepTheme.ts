import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { z } from 'zod'
import { importedThemeSchema, type ImportedTheme } from '../utils/openvsx-theme'

export function usePhotoKeepTheme(notify: (message: string) => void) {
type ThemePreference = 'system' | 'imported'
const THEME_STORAGE_KEY = 'photokeep.theme'
const IMPORTED_THEMES_STORAGE_KEY = 'photokeep.imported-themes.v1'
const SELECTED_IMPORTED_THEME_STORAGE_KEY = 'photokeep.imported-theme.selected'
const themePreference = ref<ThemePreference>('system')
const importedThemes = ref<ImportedTheme[]>([])
const selectedImportedThemeId = ref('')
let systemThemeQuery: MediaQueryList | undefined
function applyTheme() {
  if (typeof document === 'undefined') return
  const selected = themePreference.value === 'imported' ? importedThemes.value.find(theme => theme.id === selectedImportedThemeId.value) : undefined
  const dark = selected?.dark ?? !!systemThemeQuery?.matches
  document.documentElement.dataset.theme = dark ? 'dark' : 'light'
  if (selected) {
    document.documentElement.dataset.importedTheme = 'true'
    const palette = selected.palette
    const vars: Record<string, string> = {
      '--ink': palette.ink, '--muted': palette.muted, '--green': palette.accent,
      '--green-dark': palette.ink, '--green-soft': palette.accentSoft,
      '--border': palette.border, '--surface': palette.surface, '--page': palette.page,
      '--imported-input': palette.input, '--imported-sidebar': palette.sidebar,
      '--imported-accent-text': palette.accentText, '--danger': palette.danger || palette.accent,
      '--theme-hover': palette.hover || palette.accentSoft,
      '--theme-selection': palette.selected || palette.accentSoft,
      '--theme-selection-text': palette.selectedText || palette.ink,
      '--theme-button-hover': palette.buttonHover || palette.accent,
      '--theme-secondary': palette.secondaryButton || palette.surface,
      '--theme-secondary-text': palette.secondaryButtonText || palette.ink,
      '--theme-secondary-hover': palette.secondaryButtonHover || palette.accentSoft,
      '--theme-focus': palette.focus || palette.accent,
      '--theme-sidebar-ink': palette.sidebarText || palette.ink,
    }
    for (const [key, value] of Object.entries(vars)) document.documentElement.style.setProperty(key, value)
  } else {
    delete document.documentElement.dataset.importedTheme
    for (const key of ['--ink', '--muted', '--green', '--green-dark', '--green-soft', '--border', '--surface', '--page', '--imported-input', '--imported-sidebar', '--imported-accent-text', '--danger', '--theme-hover', '--theme-selection', '--theme-selection-text', '--theme-button-hover', '--theme-secondary', '--theme-secondary-text', '--theme-secondary-hover', '--theme-focus', '--theme-sidebar-ink']) document.documentElement.style.removeProperty(key)
  }
  document.documentElement.style.colorScheme = dark ? 'dark' : 'light'
}
watch(themePreference, value => {
  try { localStorage.setItem(THEME_STORAGE_KEY, value) } catch {}
  applyTheme()
})
watch([importedThemes, selectedImportedThemeId], () => {
  try {
    localStorage.setItem(IMPORTED_THEMES_STORAGE_KEY, JSON.stringify(importedThemes.value))
    localStorage.setItem(SELECTED_IMPORTED_THEME_STORAGE_KEY, selectedImportedThemeId.value)
  } catch {}
  applyTheme()
})
function importTheme(theme: ImportedTheme) {
  const validated = importedThemeSchema.parse(theme)
  const existing = importedThemes.value.filter(saved => saved.id !== validated.id)
  importedThemes.value = [validated, ...existing]
  selectedImportedThemeId.value = validated.id
  themePreference.value = 'imported'
  applyTheme()
  notify(`นำเข้าธีม ${validated.label} แล้ว`)
}
function selectTheme(id: string) {
  if (!importedThemes.value.some(theme => theme.id === id)) return
  selectedImportedThemeId.value = id
  themePreference.value = 'imported'
  applyTheme()
}
function removeTheme(id: string) {
  importedThemes.value = importedThemes.value.filter(theme => theme.id !== id)
  if (selectedImportedThemeId.value === id) {
    selectedImportedThemeId.value = ''
    themePreference.value = 'system'
  }
  applyTheme()
}
onMounted(() => {
  try {
    const storedThemes = z.array(importedThemeSchema).safeParse(JSON.parse(localStorage.getItem(IMPORTED_THEMES_STORAGE_KEY) || '[]'))
    if (storedThemes.success) importedThemes.value = storedThemes.data
    selectedImportedThemeId.value = localStorage.getItem(SELECTED_IMPORTED_THEME_STORAGE_KEY) || ''
    const saved = localStorage.getItem(THEME_STORAGE_KEY)
    if (saved === 'system' || (saved === 'imported' && importedThemes.value.some(theme => theme.id === selectedImportedThemeId.value))) themePreference.value = saved
  } catch {}
  systemThemeQuery = window.matchMedia('(prefers-color-scheme: dark)')
  systemThemeQuery.addEventListener('change', applyTheme)
  applyTheme()
})
onBeforeUnmount(() => systemThemeQuery?.removeEventListener('change', applyTheme))
return { themePreference, importedThemes, selectedImportedThemeId, importTheme, selectTheme, removeTheme }
}
