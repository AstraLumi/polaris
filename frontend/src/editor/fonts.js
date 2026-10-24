// Fonts for the writing page: the families the app already loads, a short
// list of common book fonts, and any family a document names, loaded from
// Google Fonts when first used (a family Google doesn't have is left to the
// computer's own fonts). The fonts library with uploads is stage 5.

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

export const fontChoices = (extra = []) => {
  const seen = new Set()
  const out = []
  for (const f of [...BOOK_FONTS, ...extra, ...APP_FONTS]) {
    const k = String(f || '').trim()
    if (k && !seen.has(k.toLowerCase())) {
      seen.add(k.toLowerCase())
      out.push(k)
    }
  }
  return out
}

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

// Loads a family (regular, bold, italic where it has them). Resolves to
// whether Google had it.
export function loadFont(family) {
  const name = String(family || '').trim()
  const key = name.toLowerCase()
  if (!name || typeof document === 'undefined') return Promise.resolve(false)
  if (APP_FONTS.some((f) => f.toLowerCase() === key) || SYSTEM_FONTS.has(key)) return Promise.resolve(true)
  if (!requested.has(key)) {
    // Not every family has bold and italic; ask for them, then for the
    // family alone.
    requested.set(key, addSheet(cssUrl(name, true)).then((ok) => ok || addSheet(cssUrl(name, false))))
  }
  return requested.get(key)
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
