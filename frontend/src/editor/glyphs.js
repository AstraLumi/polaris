// Symbols in fonts (the Symbols panel, and the symbols of a font in "Fonts
// and licenses"): Unicode blocks to browse, the characters of a block, and
// whether a font draws a character. The browser can't list a font's
// characters, so drawsGlyph compares a canvas drawing of the character in
// the font (then a generic family) with the generic family alone: the same
// pixels mean the font lacks it. The rest is pure (tests/glyphs.test.mjs).

// The blocks the panel offers, in its order: key (its label is the
// panel's), then code point ranges.
export const SYMBOL_BLOCKS = [
  { key: 'misc', ranges: [[0x2600, 0x26ff]] },
  { key: 'dingbats', ranges: [[0x2700, 0x27bf]] },
  { key: 'arrows', ranges: [[0x2190, 0x21ff], [0x27f0, 0x27ff], [0x2900, 0x297f], [0x2b00, 0x2bff]] },
  { key: 'shapes', ranges: [[0x25a0, 0x25ff], [0x1f780, 0x1f7ff]] },
  { key: 'math', ranges: [[0x2200, 0x22ff], [0x27c0, 0x27ef], [0x2980, 0x2aff]] },
  { key: 'math_letters', ranges: [[0x1d400, 0x1d7ff]] },
  { key: 'letterlike', ranges: [[0x2100, 0x214f], [0x2150, 0x218b]] },
  { key: 'enclosed', ranges: [[0x2460, 0x24ff], [0x1f100, 0x1f1ad]] },
  { key: 'punctuation', ranges: [[0x00a1, 0x00bf], [0x2010, 0x205e]] },
  { key: 'currency', ranges: [[0x20a0, 0x20c0]] },
  { key: 'technical', ranges: [[0x2300, 0x23ff]] },
  { key: 'boxes', ranges: [[0x2500, 0x259f]] },
  { key: 'games', ranges: [[0x1f000, 0x1f0ff], [0x1fa00, 0x1fa6f]] },
  { key: 'alchemy', ranges: [[0x1f700, 0x1f77f]] },
  { key: 'runes', ranges: [[0x16a0, 0x16f8]] },
  { key: 'greek', ranges: [[0x0370, 0x03ff]] },
  { key: 'latin', ranges: [[0x00c0, 0x024f]] },
  { key: 'braille', ranges: [[0x2800, 0x28ff]] },
  { key: 'cjk', ranges: [[0x3000, 0x303f], [0x3040, 0x30ff], [0xff01, 0xff60]] },
  { key: 'emoji', ranges: [[0x1f300, 0x1f64f], [0x1f680, 0x1f6ff], [0x1f900, 0x1f9ff], [0x1fa70, 0x1faff]] },
  { key: 'private', ranges: [[0xf020, 0xf0ff]] },
]

// Characters nothing can show on its own: controls, format characters,
// unassigned code points, combining marks, spaces and separators.
const INVISIBLE = /[\p{Cc}\p{Cf}\p{Cs}\p{Cn}\p{M}\p{Z}]/u

// A block's characters, in order (strings).
export function blockChars(block) {
  const out = []
  for (const [from, to] of block?.ranges || []) {
    for (let cp = from; cp <= to; cp++) {
      const ch = String.fromCodePoint(cp)
      if (!INVISIBLE.test(ch)) out.push(ch)
    }
  }
  return out
}

// "U+2605" for a character.
export const codeOf = (ch) => 'U+' + (ch.codePointAt(0) ?? 0).toString(16).toUpperCase().padStart(4, '0')

// The code point ranges in a stylesheet's unicode-range lines (U+0302-0303,
// U+0305, U+26??), as [from, to] pairs.
export function parseUnicodeRange(css) {
  const out = []
  for (const m of String(css || '').matchAll(/unicode-range\s*:\s*([^;}]*)/gi)) {
    for (const part of m[1].split(',')) {
      const r = /^\s*U\+([0-9A-F?]+)(?:-([0-9A-F]+))?\s*$/i.exec(part)
      if (!r) continue
      const lo = parseInt(r[1].replace(/\?/g, '0'), 16)
      const hi = r[2] ? parseInt(r[2], 16) : parseInt(r[1].replace(/\?/g, 'F'), 16)
      if (Number.isFinite(lo) && Number.isFinite(hi) && hi >= lo) out.push([lo, hi])
    }
  }
  return out
}

export const inRanges = (ranges, ch) => {
  const cp = ch.codePointAt(0)
  return ranges.some(([a, b]) => cp >= a && cp <= b)
}

// A family name as CSS.
export const quoted = (family) => `"${String(family).replace(/["\\]/g, '')}"`

// ---- In the browser ---------------------------------------------------------

const SIZE = 32
let ctx = null
const seen = new Map() // family (lower case) → Map(char → bool)

function pixels(font, ch) {
  ctx.clearRect(0, 0, SIZE, SIZE)
  ctx.font = font
  ctx.fillText(ch, 4, Math.round(SIZE * 0.75))
  return ctx.getImageData(0, 0, SIZE, SIZE).data
}

const same = (a, b) => {
  for (let i = 3; i < a.length; i += 4) if (a[i] !== b[i]) return false
  return true
}

// Whether a loaded family draws ch itself (not a fallback font).
export function drawsGlyph(family, ch) {
  if (typeof document === 'undefined' || !family) return false
  const key = family.toLowerCase()
  let known = seen.get(key)
  if (!known) seen.set(key, (known = new Map()))
  if (known.has(ch)) return known.get(ch)
  if (!ctx) {
    const c = document.createElement('canvas')
    c.width = c.height = SIZE
    ctx = c.getContext('2d', { willReadFrequently: true })
    ctx.textBaseline = 'alphabetic' // not 'middle': that moves with the first font's metrics
  }
  const px = `${SIZE - 8}px`
  let has = false
  for (const generic of ['monospace', 'serif']) {
    if (!same(pixels(`${px} ${quoted(family)}, ${generic}`, ch), pixels(`${px} ${generic}`, ch))) {
      has = true
      break
    }
  }
  known.set(ch, has)
  return has
}

// The characters of chars that family draws, once its faces for them are
// loaded (load: loadFont from fonts.js). Works in slices so a long block
// doesn't freeze the page; stop() makes it give up.
export async function glyphsOf(family, chars, { load, stop = () => false } = {}) {
  if (typeof document === 'undefined' || !family || !chars.length) return []
  if (load) await load(family)
  try {
    await document.fonts?.load(`24px ${quoted(family)}`, chars.join(''))
  } catch {
    /* drawn with what there is */
  }
  const out = []
  for (let i = 0; i < chars.length; i += 128) {
    if (stop()) return out
    for (const ch of chars.slice(i, i + 128)) if (drawsGlyph(family, ch)) out.push(ch)
    await new Promise((r) => setTimeout(r))
  }
  return out
}
