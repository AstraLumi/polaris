// Interface translation. The English text itself is the key: write
// $t('Add class') in a template or t('Add class') in a script. English is
// what you get when a language has no entry for a string, so a missing
// translation can never break a page. Only the UI is translated; whatever
// the user writes into their story stays as written.
//
// {name} placeholders are filled from the params object:
//   t('Delete {name}?', { name })
// Plurals pick between two keys: tn('{n} event', '{n} events', n, { n }).

import { ref } from 'vue'
import ptBR from './locales/pt-BR'
import esES from './locales/es-ES'
import es419 from './locales/es-419'

export const LOCALES = [
  { id: 'en-US', name: 'English (US)' },
  { id: 'pt-BR', name: 'Português (Brasil)' },
  { id: 'es-ES', name: 'Español (España)' },
  { id: 'es-419', name: 'Español (Latinoamérica)' },
]

export const DEFAULT_LOCALE = 'en-US'
const KEY = 'polaris-locale'
const catalogs = { 'pt-BR': ptBR, 'es-ES': esES, 'es-419': es419 }

export const locale = ref(DEFAULT_LOCALE)

export function t(key, params) {
  const catalog = catalogs[locale.value] // reading locale.value is what makes templates update
  let text = (catalog && catalog[key]) || key
  if (params) text = text.replace(/\{(\w+)\}/g, (m, name) => (name in params ? params[name] : m))
  return text
}

export function tn(one, other, n, params) {
  return t(n === 1 ? one : other, { n, ...params })
}

// Marks an English string as translatable where it is defined (a table of
// labels, say) without translating it yet: render it later with $t(label).
// The translation checker finds these the same way it finds t() calls.
export const tr = (s) => s

export function applyLocale(id) {
  const next = LOCALES.some((l) => l.id === id) ? id : DEFAULT_LOCALE
  locale.value = next
  document.documentElement.lang = next
  return next
}

// The browser's own language, mapped onto the ones we have.
function guessLocale() {
  const langs = (typeof navigator !== 'undefined' && (navigator.languages || [navigator.language])) || []
  for (const raw of langs) {
    const l = String(raw || '').toLowerCase()
    if (l.startsWith('pt')) return 'pt-BR'
    if (l === 'es-es') return 'es-ES'
    if (l.startsWith('es')) return 'es-419'
    if (l.startsWith('en')) return 'en-US'
  }
  return DEFAULT_LOCALE
}

export function loadSavedLocale() {
  let id = ''
  try {
    id = localStorage.getItem(KEY) || ''
  } catch (e) {
    /* storage blocked: fall back to the browser's language */
  }
  return applyLocale(id || guessLocale())
}

export function saveLocale(id) {
  const next = applyLocale(id)
  try {
    localStorage.setItem(KEY, next)
  } catch (e) {
    /* applied for this visit even if it can't be remembered */
  }
}

export const i18nPlugin = {
  install(app) {
    app.config.globalProperties.$t = t
    app.config.globalProperties.$tn = tn
  },
}
