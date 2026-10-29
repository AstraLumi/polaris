// The fonts library (backend/fontlib.go; docs/EDITOR.md, "Fonts library"):
// one library for every story, so its calls are global (/api/fonts), not
// under the story's prefix. Loading it also puts an @font-face for every
// file on the page, so a library family draws in the editor, the page view
// and the library page itself (the browser only downloads a face when text
// uses it). The rules are pure, in fontRules.js.
import { ref } from 'vue'
import { t, tr } from './i18n'
import { STORY_API } from './stories'
import { fontFaceRules } from './fontRules'

// Messages the library's calls can give (backend fontlib.go, fontparse.go);
// listed so they get translated.
void [
  tr('font not found'), tr('choose a font file'), tr('choose a file'), tr('invalid form data'), tr('invalid request'),
  tr('this is not a font file the library can use (TTF, OTF, WOFF or WOFF2)'),
  tr("font collections (.ttc) can't be used; upload each font on its own"),
  tr("this font file is damaged and can't be read"), tr('this font file is already in the library'),
  tr('that font file is too large (over 40 MB)'), tr('those font files are too large'), tr('upload at most 20 font files at once'),
  tr('failed to save the font'), tr('failed to load the fonts'),
  tr('a font needs a name (200 characters at most, without quotes or brackets)'), tr('another font in the library has that name'),
  tr('unknown license kind'), tr('that text is too long'), tr('the purchase date is not valid'), tr('a weight is one of 100, 200 … 900'),
  tr("that file is not one of this font's"), tr('that file is too large (over 20 MB)'),
  tr('a license or receipt must be a PDF, a picture or a text file'),
  tr('failed to change the font'), tr('that is the same font'),
]

export const fontLibrary = ref([])
let loading = null

function install(list) {
  if (typeof document === 'undefined') return
  let el = document.getElementById('polaris-font-library')
  if (!el) {
    el = document.createElement('style')
    el.id = 'polaris-font-library'
    document.head.appendChild(el)
  }
  el.textContent = fontFaceRules(list)
}

function use(list) {
  fontLibrary.value = Array.isArray(list) ? list : []
  install(fontLibrary.value)
  return fontLibrary.value
}

async function call(url, options, fallback) {
  let res
  try {
    res = await fetch(url, options)
  } catch (e) {
    throw new Error(t("can't reach the server"))
  }
  if (!res.ok) {
    let msg = ''
    try {
      msg = (await res.text()).trim()
    } catch (e) {
      /* no body */
    }
    throw new Error(t(msg || fallback))
  }
  return res.json()
}

// The library, loaded once (force: again). Never fails: without the server
// it answers what it had.
export function loadFontLibrary(force = false) {
  if (!loading || force) {
    loading = call('/api/fonts', undefined, 'failed to load the fonts')
      .then(use)
      .catch(() => {
        loading = null
        return fontLibrary.value
      })
  }
  return loading
}

// The library as an offline book kept it (docs/EDITOR.md, "Offline
// writing"), used while the server can't be reached and nothing was loaded.
export function adoptFontLibrary(list) {
  if (!fontLibrary.value.length && Array.isArray(list) && list.length) use(list)
}

// Several files at once; familyId puts them all in that family. Answers
// { families, added, problems: [{ file, error }] }.
export async function uploadFonts(files, familyId = null) {
  const fd = new FormData()
  for (const f of files) fd.append('file', f)
  if (familyId) fd.append('family_id', String(familyId))
  const out = await call('/api/fonts', { method: 'POST', body: fd }, 'failed to save the font')
  use(out.families)
  loading = Promise.resolve(fontLibrary.value)
  return out
}

function replace(fam) {
  const list = fontLibrary.value.map((f) => (f.id === fam.id ? fam : f))
  use(list.sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' })))
  return fam
}

export const saveFontFamily = async (id, body) =>
  replace(await call(`/api/fonts/${id}`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }, 'failed to save the font'))

export async function deleteFontFamily(id) {
  await call(`/api/fonts/${id}`, { method: 'DELETE' }, 'failed to save the font')
  use(fontLibrary.value.filter((f) => f.id !== id))
}

export const deleteFontFile = async (fileId) => replace(await call(`/api/font-files/${fileId}`, { method: 'DELETE' }, 'failed to save the font'))

export async function uploadLicenseFile(id, file) {
  const fd = new FormData()
  fd.append('file', file)
  return replace(await call(`/api/fonts/${id}/license-file`, { method: 'POST', body: fd }, 'failed to save the font'))
}

export const removeLicenseFile = async (id) => replace(await call(`/api/fonts/${id}/license-file`, { method: 'DELETE' }, 'failed to save the font'))
export const licenseFileUrl = (id) => `/api/fonts/${id}/license-file`

// Which stories, books and styles use each family: { "<family id>": [{
// story_id, story, volume, book, family, styles, bands, chapters }] }.
export const fetchFontsUsed = () => call('/api/fonts/used', undefined, 'failed to load the fonts')

// The families one book of the open story names (styles, bands, text).
export const fetchBookFonts = (volume) => call(`${STORY_API}/books/${volume}/fonts`, undefined, 'failed to load the book')

// Changes a font in the book's text (backend fontswap.go): { family, what:
// 'spaces' | 'chars' | 'all', chars, to ('' = the body font), label } →
// { chapters, pieces }. Each chapter changed keeps a version first.
export const swapBookFont = (volume, body) =>
  call(`${STORY_API}/books/${volume}/fonts/swap`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }, 'failed to change the font')

// The words for a warning from fontRules.js.
export function fontWarningText(w) {
  const p = { family: w.family, styles: (w.styles || []).join(', ') }
  switch (w.reason) {
    case 'personal':
      return w.styles ? t('{family} is licensed for personal use only; used by: {styles}.', p) : t('{family} is licensed for personal use only.', p)
    case 'no-commercial':
      return w.styles ? t("{family}'s license doesn't allow commercial use; used by: {styles}.", p) : t("{family}'s license doesn't allow commercial use.", p)
    case 'no-print':
      return t("{family}: its license doesn't allow print.", p)
    case 'no-ebook':
      return t("{family}: its license doesn't allow e-books.", p)
    case 'no-embedding':
      return t('{family}: the font file forbids putting it in documents.', p)
    case 'unrecorded':
      return t("{family}: its license isn't recorded yet.", p)
  }
  return w.family
}

export const LICENSE_LABELS = {
  '': tr('Not recorded'),
  ofl: tr('SIL Open Font License'),
  apache: tr('Apache License'),
  commercial: tr('Commercial'),
  personal: tr('Personal use only'),
  other: tr('Other'),
}
export const USE_LABELS = { print: tr('Print'), ebook: tr('E-books'), web: tr('Web'), commercial: tr('Commercial use') }
export const WEIGHT_NAMES = {
  100: tr('Thin'), 200: tr('Extra light'), 300: tr('Light'), 400: tr('Regular'), 500: tr('Medium'),
  600: tr('Semibold'), 700: tr('Bold'), 800: tr('Extra bold'), 900: tr('Black'),
}
export const EMBEDDING_LABELS = {
  installable: tr('Can be embedded'),
  editable: tr('Can be embedded (editable)'),
  preview: tr('Can be embedded (preview and print)'),
  restricted: tr("Can't be embedded (restricted)"),
  unknown: tr('Embedding permissions not read'),
}
