import { parse, type ParseError } from 'jsonc-parser'
import { z } from 'zod'

const colorSchema = z.string().regex(/^#[0-9a-fA-F]{6}$/)
export const paletteSchema = z.object({
  ink: colorSchema, muted: colorSchema, accent: colorSchema, accentText: colorSchema,
  accentSoft: colorSchema, border: colorSchema, surface: colorSchema, page: colorSchema,
  input: colorSchema, sidebar: colorSchema, danger: colorSchema.optional(),
  hover: colorSchema.optional(), selected: colorSchema.optional(), selectedText: colorSchema.optional(),
  buttonHover: colorSchema.optional(), secondaryButton: colorSchema.optional(),
  secondaryButtonText: colorSchema.optional(), secondaryButtonHover: colorSchema.optional(),
  focus: colorSchema.optional(), sidebarText: colorSchema.optional(),
}).strict()
export const importedThemeSchema = z.object({
  id: z.string().min(3).max(300), label: z.string().min(1).max(120),
  publisher: z.string().min(1).max(100), extension: z.string().min(1).max(100),
  version: z.string().min(1).max(100), sha256: z.string().regex(/^[a-f0-9]{64}$/),
  dark: z.boolean(), palette: paletteSchema,
}).strict()
export type ImportedTheme = z.infer<typeof importedThemeSchema>

export const searchResponseSchema = z.object({
  extensions: z.array(z.object({
    namespace: z.string(), name: z.string(), displayName: z.string(),
    description: z.string().optional().nullable(), version: z.string(),
  })).max(24), total: z.number().int().nonnegative(), offset: z.number().int().nonnegative(),
  nextOffset: z.number().int().nonnegative(), hasMore: z.boolean(),
})
export const packageResponseSchema = z.object({
  namespace: z.string(), name: z.string(), version: z.string(),
  sha256: z.string().regex(/^[a-f0-9]{64}$/), files: z.record(z.string(), z.string()),
})
export type OpenVSXResult = z.infer<typeof searchResponseSchema>['extensions'][number]

const contributionSchema = z.object({
  label: z.string().min(1).max(120), path: z.string().min(1).max(240),
  uiTheme: z.enum(['vs', 'vs-dark', 'hc-black', 'hc-light']),
})
const manifestSchema = z.object({
  publisher: z.string(), name: z.string(),
  contributes: z.object({ themes: z.array(contributionSchema).min(1).max(512) }),
})
const themeFileSchema = z.object({
  colors: z.record(z.string(), z.string()).optional(), include: z.string().optional(),
})

function parseJSONC(text: string, filename: string): unknown {
  const errors: ParseError[] = []
  const value: unknown = parse(text, errors, { allowTrailingComma: true })
  if (errors.length || !value || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error(`${filename} ไม่ใช่ JSON/JSONC ที่ถูกต้อง`)
  }
  return value
}

function resolvePackagePath(from: string, relative: string): string {
  if (!relative || relative.startsWith('/') || relative.includes('\\') || relative.includes('\0')) throw new Error('ตำแหน่งไฟล์ธีมไม่ปลอดภัย')
  const parts = from.split('/').slice(0, -1)
  for (const segment of relative.split('/')) {
    if (!segment || segment === '.') continue
    if (segment === '..') {
      if (parts.length <= 1) throw new Error('ตำแหน่งไฟล์ธีมออกนอกแพ็กเกจ')
      parts.pop()
    } else parts.push(segment)
  }
  const result = parts.join('/')
  if (!result.startsWith('extension/')) throw new Error('ตำแหน่งไฟล์ธีมออกนอกแพ็กเกจ')
  return result
}

function resolveColors(files: Record<string, string>, filename: string, visited = new Set<string>()): Record<string, string> {
  if (visited.size > 8 || visited.has(filename)) throw new Error('ไฟล์ธีมอ้างอิงวนหรือซ้อนมากเกินไป')
  const text = files[filename]
  if (!text || text.length > 8 * 1024 * 1024) throw new Error(`ไม่พบไฟล์สี ${filename}`)
  visited.add(filename)
  const theme = themeFileSchema.parse(parseJSONC(text, filename))
  const base = theme.include ? resolveColors(files, resolvePackagePath(filename, theme.include), visited) : {}
  return { ...base, ...(theme.colors || {}) }
}

function opaqueHex(value: unknown, background: string): string | undefined {
  if (typeof value !== 'string') return undefined
  if (/^#[\da-fA-F]{3}$/.test(value)) return '#' + [...value.slice(1)].map(char => char + char).join('').toLowerCase()
  if (/^#[\da-fA-F]{6}$/.test(value)) return value.toLowerCase()
  if (/^#[\da-fA-F]{4}$/.test(value)) {
    const expanded = '#' + [...value.slice(1)].map(char => char + char).join('').toLowerCase()
    return blend(background, expanded.slice(0, 7), parseInt(expanded.slice(7), 16) / 255)
  }
  if (/^#[\da-fA-F]{8}$/.test(value)) return blend(background, value.slice(0, 7).toLowerCase(), parseInt(value.slice(7), 16) / 255)
  return undefined
}

function solidHex(value: unknown): string | undefined {
  if (typeof value !== 'string') return undefined
  if (/^#[\da-fA-F]{3}$/.test(value)) return '#' + [...value.slice(1)].map(char => char + char).join('').toLowerCase()
  if (/^#[\da-fA-F]{6}$/.test(value)) return value.toLowerCase()
  return undefined
}

function luminance(hex: string): number {
  const rgb = [1, 3, 5].map(start => parseInt(hex.slice(start, start + 2), 16) / 255)
  return rgb.map(value => value <= .04045 ? value / 12.92 : ((value + .055) / 1.055) ** 2.4)
    .reduce((sum, value, index) => sum + value * ([.2126, .7152, .0722][index] ?? 0), 0)
}

function blend(background: string, foreground: string, amount: number): string {
  const channel = (start: number) => Math.round(parseInt(background.slice(start, start + 2), 16) * (1 - amount) + parseInt(foreground.slice(start, start + 2), 16) * amount).toString(16).padStart(2, '0')
  return `#${channel(1)}${channel(3)}${channel(5)}`
}

export function parseOpenVSXThemes(input: unknown): ImportedTheme[] {
  const pkg = packageResponseSchema.parse(input)
  if (Object.keys(pkg.files).length > 512 || !pkg.files['extension/package.json']) throw new Error('ข้อมูลแพ็กเกจไม่ครบ')
  const manifest = manifestSchema.parse(parseJSONC(pkg.files['extension/package.json'], 'package.json'))
  if (manifest.publisher !== pkg.namespace || manifest.name !== pkg.name) throw new Error('ข้อมูลใน package.json ไม่ตรงกับแพ็กเกจที่เลือก')
  return manifest.contributes.themes.map((entry, index) => {
    const filename = resolvePackagePath('extension/package.json', entry.path)
    const colors = resolveColors(pkg.files, filename)
    const dark = entry.uiTheme === 'vs-dark' || entry.uiTheme === 'hc-black'
    const initialBaseColor = solidHex(colors['editor.background']) || solidHex(colors['sideBar.background']) || solidHex(colors['activityBar.background'])
    if (!initialBaseColor) throw new Error(`ธีม ${entry.label} ต้องมีสีพื้นหลังแบบทึบ`)
    let baseColor: string = initialBaseColor
    const get = (...keys: string[]) => {
      for (const key of keys) {
        if (colors[key] === undefined) continue
        const color = opaqueHex(colors[key], baseColor)
        if (!color) throw new Error(`ค่าสี ${key} ไม่ใช่เลขฐานสิบหกแบบทึบ`)
        return color
      }
      return undefined
    }
    const page = get('editor.background', 'sideBar.background', 'activityBar.background')
    if (page) baseColor = page
    const ink = get('editor.foreground', 'foreground')
    if (!page || !ink || Object.keys(colors).length < 4) throw new Error(`ธีม ${entry.label} มีสี UI ไม่ครบ`)
    const accent = get('button.background', 'focusBorder', 'list.activeSelectionBackground') || ink
    const surface = get('sideBar.background', 'editorWidget.background', 'panel.background') || blend(page, ink, .08)
    const contrast = (color: string) => {
      const lighter = Math.max(luminance(color), luminance(accent))
      const darker = Math.min(luminance(color), luminance(accent))
      return (lighter + .05) / (darker + .05)
    }
    const palette = paletteSchema.parse({
      ink, muted: get('descriptionForeground', 'sideBar.foreground') || blend(ink, page, .32),
      accent, accentText: get('button.foreground') || (contrast(page) > contrast(ink) ? page : ink),
      accentSoft: blend(surface, accent, dark ? .2 : .13),
      border: get('panel.border', 'sideBar.border', 'contrastBorder') || blend(surface, ink, dark ? .22 : .17),
      surface, page, input: get('input.background') || surface, sidebar: get('sideBar.background') || surface,
      danger: get('errorForeground', 'inputValidation.errorForeground') || accent,
      hover: get('list.hoverBackground', 'menu.selectionBackground', 'toolbar.hoverBackground') || blend(surface, accent, dark ? .22 : .13),
      selected: get('list.activeSelectionBackground', 'sideBarSectionHeader.background') || blend(surface, accent, dark ? .32 : .2),
      selectedText: get('list.activeSelectionForeground', 'sideBar.foreground') || ink,
      buttonHover: get('button.hoverBackground') || accent,
      secondaryButton: get('button.secondaryBackground') || surface,
      secondaryButtonText: get('button.secondaryForeground') || ink,
      secondaryButtonHover: get('button.secondaryHoverBackground') || blend(surface, accent, dark ? .22 : .13),
      focus: get('focusBorder') || accent,
      sidebarText: get('sideBar.foreground') || ink,
    })
    return importedThemeSchema.parse({
      id: `${pkg.namespace}.${pkg.name}@${pkg.version}#${index}`, label: entry.label,
      publisher: pkg.namespace, extension: pkg.name, version: pkg.version, sha256: pkg.sha256,
      dark, palette,
    })
  })
}
