// Reading CSS values into the document's units (docs/EDITOR.md): lengths in
// points, colours as #rrggbb, line height as a multiple of the font size;
// and writing paragraph and text formatting back out as CSS. Used by the
// schema (drawing, and pasting from Word, Google Docs and web pages) and by
// the styles. Pure, no DOM.

const PER_UNIT = { pt: 1, px: 0.75, in: 72, cm: 72 / 2.54, mm: 72 / 25.4, pc: 12, q: 72 / 101.6 }

const round2 = (n) => Math.round(n * 100) / 100

// A line height of 1 ("single", as in Word) is drawn this many times the
// font size: Word's single spacing follows the font's own line gap, which
// for book fonts is about this.
export const LINE_FACTOR = 1.2

// "12pt", "16px", "1.5cm", "0" → points; anything relative (em, %, auto) → null.
export function cssLengthToPt(value) {
  const s = String(value ?? '').trim().toLowerCase()
  if (!s) return null
  if (/^-?0+(\.0+)?$/.test(s)) return 0
  const m = /^(-?\d*\.?\d+)\s*(pt|px|in|cm|mm|pc|q)$/.exec(s)
  if (!m) return null
  return round2(parseFloat(m[1]) * PER_UNIT[m[2]])
}

export const mmToPt = (mm) => round2((mm * 72) / 25.4)
export const ptToMm = (pt) => Math.round(((pt * 25.4) / 72) * 10) / 10

// A font size: absolute lengths only, within reason.
export function cssFontSizeToPt(value) {
  const pt = cssLengthToPt(value)
  return pt != null && pt >= 1 && pt <= 400 ? pt : null
}

// "1.5", "150%", "normal" → a multiple. Absolute lengths can't be read
// without the font size, so they give null.
export function cssLineHeight(value) {
  const s = String(value ?? '').trim().toLowerCase()
  if (s === 'normal') return 1
  let m = /^(\d*\.?\d+)%$/.exec(s)
  if (m) return round2(parseFloat(m[1]) / 100) || null
  m = /^(\d*\.?\d+)$/.exec(s)
  if (m) return round2(parseFloat(m[1])) || null
  return null
}

const NAMED = {
  black: '#000000', white: '#ffffff', red: '#ff0000', green: '#008000', blue: '#0000ff', yellow: '#ffff00',
  gray: '#808080', grey: '#808080', silver: '#c0c0c0', maroon: '#800000', navy: '#000080', purple: '#800080',
  teal: '#008080', olive: '#808000', lime: '#00ff00', aqua: '#00ffff', cyan: '#00ffff', fuchsia: '#ff00ff',
  magenta: '#ff00ff', orange: '#ffa500', darkred: '#8b0000', darkblue: '#00008b', darkgreen: '#006400',
}

const hex2 = (n) => Math.max(0, Math.min(255, Math.round(n))).toString(16).padStart(2, '0')

// "#abc", "#aabbcc", "rgb(1, 2, 3)", "rgba(…, 1)", a few names → "#rrggbb";
// transparent or unreadable → null.
export function cssColorToHex(value) {
  const s = String(value ?? '').trim().toLowerCase()
  if (!s || s === 'transparent' || s === 'inherit' || s === 'initial' || s === 'currentcolor') return null
  if (NAMED[s]) return NAMED[s]
  let m = /^#([0-9a-f]{3})$/.exec(s)
  if (m) return '#' + m[1].split('').map((c) => c + c).join('')
  m = /^#([0-9a-f]{6})([0-9a-f]{2})?$/.exec(s)
  if (m) return m[2] === '00' ? null : '#' + m[1]
  m = /^rgba?\(\s*([\d.]+)[\s,]+([\d.]+)[\s,]+([\d.]+)(?:[\s,/]+([\d.]+%?))?\s*\)$/.exec(s)
  if (m) {
    if (m[4] != null) {
      const a = m[4].endsWith('%') ? parseFloat(m[4]) / 100 : parseFloat(m[4])
      if (a === 0) return null
    }
    return '#' + hex2(+m[1]) + hex2(+m[2]) + hex2(+m[3])
  }
  return null
}

const GENERIC = new Set(['serif', 'sans-serif', 'monospace', 'cursive', 'fantasy', 'system-ui', 'ui-serif', 'ui-sans-serif', 'ui-monospace', 'emoji', 'math', 'inherit', 'initial'])

// The first named family of a font-family list, without quotes; null for
// a generic family alone.
export function cssFontFamily(value) {
  const first = String(value ?? '').split(',')[0]?.trim().replace(/^["']|["']$/g, '').trim()
  if (!first || GENERIC.has(first.toLowerCase())) return null
  return first
}

// A family name as a CSS value: quoted, with a generic fallback.
export function familyCSS(family, fallback = 'serif') {
  return `"${String(family).replace(/["\\]/g, '')}", ${fallback}`
}

// A number for CSS: no trailing zeros.
export const num = (n) => String(round2(n))

const ALIGNS = { left: 'left', start: 'left', center: 'center', right: 'right', end: 'right', justify: 'justify' }

// Paragraph formatting from an element's inline style (anything with
// camelCase properties: a CSSStyleDeclaration or a plain object). Only what
// is set comes back.
export function paraFromCSS(style) {
  const out = {}
  const align = ALIGNS[String(style.textAlign || '').trim().toLowerCase()]
  if (align) out.align = align
  const pt = (k) => cssLengthToPt(style[k])
  const set = (key, v) => {
    if (v != null) out[key] = v
  }
  set('indentFirst', pt('textIndent'))
  set('indentLeft', pt('marginLeft'))
  set('indentRight', pt('marginRight'))
  set('spaceBefore', pt('marginTop'))
  set('spaceAfter', pt('marginBottom'))
  if (style.lineHeight) set('lineHeight', cssLineHeight(style.lineHeight))
  return out
}

// CSS declarations for paragraph formatting: { align, indentFirst, … } with
// nulls skipped.
export function paraDeclarations(p) {
  const out = []
  if (!p) return out
  if (p.align) out.push(`text-align: ${p.align}`)
  if (p.indentFirst != null) out.push(`text-indent: ${num(p.indentFirst)}pt`)
  if (p.indentLeft != null) out.push(`margin-left: ${num(p.indentLeft)}pt`)
  if (p.indentRight != null) out.push(`margin-right: ${num(p.indentRight)}pt`)
  if (p.spaceBefore != null) out.push(`margin-top: ${num(p.spaceBefore)}pt`)
  if (p.spaceAfter != null) out.push(`margin-bottom: ${num(p.spaceAfter)}pt`)
  if (p.lineHeight != null) out.push(`line-height: ${num(p.lineHeight * LINE_FACTOR)}`)
  if (p.keepNext) out.push('break-after: avoid')
  return out
}

// CSS declarations for a style's text: { font, size, color, bold, italic }.
export function textDeclarations(t) {
  const out = []
  if (!t) return out
  if (t.font) out.push(`font-family: ${familyCSS(t.font)}`)
  if (t.size != null) out.push(`font-size: ${num(t.size)}pt`)
  if (t.color) out.push(`color: ${t.color}`)
  if (t.bold != null) out.push(`font-weight: ${t.bold ? 700 : 400}`)
  if (t.italic != null) out.push(`font-style: ${t.italic ? 'italic' : 'normal'}`)
  return out
}
