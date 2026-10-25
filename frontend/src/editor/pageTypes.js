// A book's page type (docs/EDITOR.md, "Books and page types"): the page
// sizes and margins, the text block they leave, and the settings a new or
// switched book starts from. Pure.
import { BUILTIN_IDS, builtinStyles, styleById } from './styles.js'
import { normaliseLayout, normaliseMeta, normaliseChapterOptions } from './bookLayout.js'

export const PAGE_TYPE_KEYS = ['a4', 'a5', 'a6', 'custom', 'online']

// size: the body text in pt; margins: top, bottom, inner, outer (mm).
// These mirror pageTypeDefs in backend/editor.go; the server's own
// defaults (GET /api/books' `defaults`) win when it sends them.
export const PAGE_TYPES = {
  a4: { width_mm: 210, height_mm: 297, size: 12, margins: [25, 25, 25, 25], gutter_mm: 5 },
  a5: { width_mm: 148, height_mm: 210, size: 10.5, margins: [18, 18, 15, 15], gutter_mm: 5 },
  a6: { width_mm: 105, height_mm: 148, size: 8, margins: [12, 12, 12, 12], gutter_mm: 5 },
  // 5 × 8 in until the writer says otherwise.
  custom: { width_mm: 127, height_mm: 203.2, size: 10.5, margins: [15, 15, 15, 15], gutter_mm: 5 },
  // The page is ignored.
  online: { width_mm: 148, height_mm: 210, size: 12, margins: [18, 18, 15, 15], gutter_mm: 5 },
}

export const PAGE_TYPE_LABELS = { a4: 'A4', a5: 'A5', a6: 'A6', custom: 'Custom size', online: 'Online Post' }

// Pages between 50 mm and 1000 mm a side (as the server allows).
export const clampMm = (v, fallback) => {
  const n = +v
  return Number.isFinite(n) && n >= 50 && n <= 1000 ? Math.round(n * 10) / 10 : fallback
}

// The page for a type. custom: { width_mm, height_mm }.
export function defaultPage(type, custom = {}) {
  const t = PAGE_TYPES[type] || PAGE_TYPES.a5
  const [top, bottom, inner, outer] = t.margins
  return {
    width_mm: type === 'custom' ? clampMm(custom.width_mm, t.width_mm) : t.width_mm,
    height_mm: type === 'custom' ? clampMm(custom.height_mm, t.height_mm) : t.height_mm,
    margin_top_mm: top,
    margin_bottom_mm: bottom,
    margin_inner_mm: inner,
    margin_outer_mm: outer,
    gutter_mm: t.gutter_mm,
    mirror: true,
  }
}

export function defaultStyles(type) {
  const t = PAGE_TYPES[type] || PAGE_TYPES.a5
  return builtinStyles(t.size, { print: type !== 'online' })
}

// A page type's settings. serverDefaults: the `defaults` of GET
// /api/books (each type's settings), used when given; a custom size is
// applied on top.
export function defaultSettings(type = 'a5', custom = {}, serverDefaults = null) {
  const key = PAGE_TYPES[type] ? type : 'a5'
  const fromServer = serverDefaults?.[key]
  const base = fromServer && typeof fromServer === 'object' ? JSON.parse(JSON.stringify(fromServer)) : { page_type: key, page: defaultPage(key), styles: defaultStyles(key) }
  base.page_type = key
  base.page = { ...defaultPage(key), ...(base.page || {}) }
  if (key === 'custom') {
    base.page.width_mm = clampMm(custom.width_mm, base.page.width_mm)
    base.page.height_mm = clampMm(custom.height_mm, base.page.height_mm)
  }
  if (!Array.isArray(base.styles) || !base.styles.length) base.styles = defaultStyles(key)
  return base
}

// A book's settings as the server sent them, made whole: a known page type,
// a page, and every built-in style.
export function normaliseSettings(raw) {
  const s = raw && typeof raw === 'object' ? raw : {}
  const type = PAGE_TYPES[s.page_type] ? s.page_type : 'a5'
  const page = { ...defaultPage(type, s.page || {}), ...(s.page || {}) }
  const styles = Array.isArray(s.styles) ? s.styles.filter((x) => x && x.id) : []
  for (const b of defaultStyles(type)) if (!styleById(styles, b.id)) styles.splice(BUILTIN_IDS.indexOf(b.id), 0, b)
  // Stage 3: the book's details, layout and options per chapter (bookLayout.js).
  return { ...s, page_type: type, page, styles, meta: normaliseMeta(s.meta), layout: normaliseLayout(s.layout), chapter_options: normaliseChapterOptions(s.chapter_options) }
}

// Switching page type: the new type's page and built-in styles (keeping
// the names a writer gave them); the book's own styles stay.
export function switchPageType(settings, type, custom = {}, serverDefaults = null) {
  const fresh = defaultSettings(type, custom, serverDefaults)
  const styles = (settings.styles || []).map((s) => {
    if (!s.builtin) return s
    const b = styleById(fresh.styles, s.id)
    return b ? { ...b, name: s.name || b.name } : s
  })
  for (const b of fresh.styles) if (!styleById(styles, b.id)) styles.push(b)
  return { ...settings, page_type: fresh.page_type, page: fresh.page, styles }
}

// The width of the text block in mm (the page less its margins and gutter);
// null for online posts, which have no page.
export function textBlockMm(settings) {
  if (!settings || settings.page_type === 'online') return null
  const p = settings.page || {}
  const w = (+p.width_mm || 0) - (+p.margin_inner_mm || 0) - (+p.margin_outer_mm || 0) - (+p.gutter_mm || 0)
  return Math.max(20, Math.round(w * 10) / 10)
}

// The label for a page type, for "Before switching from A5 to A6"
// (English; the caller translates the words).
export function pageTypeName(settings) {
  if (!settings) return ''
  if (settings.page_type === 'custom') {
    const p = settings.page || {}
    return `${+p.width_mm || 0} × ${+p.height_mm || 0} mm`
  }
  return PAGE_TYPE_LABELS[settings.page_type] || settings.page_type
}
