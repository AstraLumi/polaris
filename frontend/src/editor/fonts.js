// Fonts for the writing page: the writer's fonts library (fontLibrary.js,
// served by the app with @font-face), the families the app already loads, a
// short list of common book fonts, and any family a document names, loaded
// from Google Fonts when first used (a family Google doesn't have is left to
// the computer's own fonts). A library family wins over Google's of the same
// name and is never asked of Google.
import { fontLibrary, loadFontLibrary } from '../fontLibrary.js'
import { findFamily, mergeChoices, cssFamily } from '../fontRules.js'

// Loaded by index.html for the interface.
export const APP_FONTS = ['Fraunces', 'Manrope']

export const BOOK_FONTS = [
  'Merriweather',
  'EB Garamond',
  'Crimson Pro',
  'Libre Baskerville',
  'Lora',
  'Literata',
  'Source Serif 4',
  'Cormorant Garamond',
  'Alegreya',
  'Spectral',
  'Playfair Display',
  'Cardo',
  'Old Standard TT',
  'Inter',
  'Roboto',
  'Open Sans',
  'Lato',
  'Montserrat',
  'Turret Road',
  'Cinzel',
  'IM Fell English',
  'Special Elite',
  'Courier Prime',
  'Noto Sans Symbols 2',
]

// Fonts most computers have; never asked of Google.
const SYSTEM_FONTS = new Set(
  [
    'Times New Roman', 'Times', 'Arial', 'Helvetica', 'Calibri', 'Cambria', 'Georgia', 'Garamond', 'Book Antiqua',
    'Palatino', 'Palatino Linotype', 'Verdana', 'Tahoma', 'Trebuchet MS', 'Courier New', 'Courier', 'Segoe UI',
    'Aptos', 'Constantia', 'Candara', 'Consolas', 'Century', 'Century Schoolbook', 'Baskerville', 'Didot',
    'Hoefler Text', 'Optima', 'Gill Sans', 'Symbol', 'Wingdings', 'Comic Sans MS', 'Impact',
  ].map((f) => f.toLowerCase()),
)

// The pickers' list: the library's families first. Reads the library, so a
// computed list follows it.
export const fontChoices = (extra = []) =>
  mergeChoices(
    fontLibrary.value.map((f) => f.name),
    BOOK_FONTS,
    extra,
    APP_FONTS,
  )

const requested = new Map() // lower-case family → promise

const cssUrl = (family, full) =>
  `https://fonts.googleapis.com/css2?family=${encodeURIComponent(family).replace(/%20/g, '+')}${full ? ':ital,wght@0,400;0,700;1,400;1,700' : ''}&display=swap`

function addSheet(href) {
  return new Promise((resolve) => {
    const link = document.createElement('link')
    link.rel = 'stylesheet'
    link.href = href
    link.dataset.writeFont = ''
    link.onload = () => resolve(true)
    link.onerror = () => {
      link.remove()
      resolve(false)
    }
    document.head.appendChild(link)
  })
}

// A library family's every face, loaded now (so a page laid out next has
// them).
function loadLibraryFamily(fam) {
  if (!document.fonts?.load) return Promise.resolve(true)
  const faces = fam.files?.length ? fam.files : [{ weight: 400, italic: false }]
  return Promise.all(
    faces.map((f) => document.fonts.load(`${f.italic ? 'italic ' : ''}${Number(f.weight) || 400} 16px ${cssFamily(fam.name)}`).catch(() => null)),
  ).then(() => true)
}

// Loads a family (regular, bold, italic where it has them). Resolves to
// whether it was found (in the library, or at Google).
export async function loadFont(family) {
  const name = String(family || '').trim()
  const key = name.toLowerCase()
  if (!name || typeof document === 'undefined') return false
  const fam = findFamily(await loadFontLibrary(), name)
  if (fam) return loadLibraryFamily(fam)
  if (APP_FONTS.some((f) => f.toLowerCase() === key) || SYSTEM_FONTS.has(key)) return true
  if (!requested.has(key)) {
    // Not every family has bold and italic; ask for them, then for the
    // family alone.
    requested.set(key, addSheet(cssUrl(name, true)).then((ok) => ok || addSheet(cssUrl(name, false))))
  }
  return requested.get(key)
}

// The Google Fonts stylesheets loadFont asks for a family (with bold and
// italic, then alone); none for a family Google isn't asked for. Offline
// writing keeps them (offline/caches.js).
export function googleSheets(family, library = fontLibrary.value) {
  const name = String(family || '').trim()
  const key = name.toLowerCase()
  if (!name || findFamily(library, name) || APP_FONTS.some((f) => f.toLowerCase() === key) || SYSTEM_FONTS.has(key)) return []
  return [cssUrl(name, true), cssUrl(name, false)]
}

export const loadFonts = (families) => Promise.all([...new Set(families.filter(Boolean))].map(loadFont))

// Every font family a document's marks name.
export function docFonts(doc) {
  const out = new Set()
  doc.descendants((node) => {
    for (const m of node.marks) if (m.type.name === 'font' && m.attrs.family) out.add(m.attrs.family)
    return true
  })
  return [...out]
}
