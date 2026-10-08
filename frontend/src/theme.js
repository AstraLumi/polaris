// Colour themes. The colours themselves live in style.css as blocks keyed by
// `:root[data-theme='<id>']`; this file only lists them for the picker and
// applies the saved choice. The choice is kept per device (localStorage):
// a phone and a desktop can reasonably want different looks.

import { ref } from 'vue'
import { tr } from './i18n'

export const THEMES = [
  { id: 'polaris', name: 'Polaris', blurb: tr('Night-sky navy and star gold. The default.') },
  { id: 'obsidian', name: 'Obsidian', blurb: tr('The original violet-black with an ember accent.') },
  { id: 'verdant', name: 'Verdant', blurb: tr('Deep forest with a mint accent.') },
  { id: 'crimson', name: 'Crimson', blurb: tr('Wine-dark with a rose accent.') },
  { id: 'ashen', name: 'Ashen', blurb: tr('Neutral graphite with a teal accent.') },
]

export const DEFAULT_THEME = 'polaris'
const KEY = 'polaris-theme'

export const currentTheme = ref(DEFAULT_THEME)

let colorCache = new Map()

export function applyTheme(id) {
  const theme = THEMES.some((t) => t.id === id) ? id : DEFAULT_THEME
  currentTheme.value = theme
  document.documentElement.dataset.theme = theme
  colorCache = new Map()
  return theme
}

export function loadSavedTheme() {
  let id = DEFAULT_THEME
  try {
    id = localStorage.getItem(KEY) || DEFAULT_THEME
  } catch (e) {
    /* storage blocked: fall back to the default */
  }
  return applyTheme(id)
}

export function saveTheme(id) {
  const theme = applyTheme(id)
  try {
    localStorage.setItem(KEY, theme)
  } catch (e) {
    /* applied for this visit even if it can't be remembered */
  }
}

// For canvas drawing, which can't take var(...) or color-mix(...): read a
// theme colour variable (hex) and return it as rgba(...) with an alpha.
export function themeRgba(name, alpha = 1) {
  const key = name + '|' + alpha
  const hit = colorCache.get(key)
  if (hit) return hit
  const raw = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  let out = raw
  const m = /^#([0-9a-f]{3}|[0-9a-f]{6})$/i.exec(raw)
  if (m) {
    let h = m[1]
    if (h.length === 3) h = h.split('').map((c) => c + c).join('')
    const n = parseInt(h, 16)
    out = `rgba(${(n >> 16) & 255}, ${(n >> 8) & 255}, ${n & 255}, ${alpha})`
  }
  colorCache.set(key, out)
  return out
}
