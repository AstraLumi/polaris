// A book's named styles (docs/EDITOR.md, "Styles"): the built-ins, how a
// paragraph's formatting is worked out (its own attrs and marks, then its
// style, then what that is based on, ending at Normal), and the stylesheet
// that draws them. Pure: no DOM, no Vue.
import { paraDeclarations, textDeclarations, charDeclarations } from './css.js'

export const BUILTIN_IDS = ['normal', 'heading1', 'heading2', 'heading3', 'title', 'subtitle', 'quote']
export const BUILTIN_NAMES = {
  normal: 'Normal',
  heading1: 'Heading 1',
  heading2: 'Heading 2',
  heading3: 'Heading 3',
  title: 'Title',
  subtitle: 'Subtitle',
  quote: 'Quote',
}

export const PARA_KEYS = ['align', 'indentFirst', 'indentLeft', 'indentRight', 'spaceBefore', 'spaceAfter', 'lineHeight', 'keepNext']
export const TEXT_KEYS = ['font', 'size', 'color', 'bold', 'italic']
// What a character style can set (docs/EDITOR.md, `char_style`): text formatting only.
export const CHAR_KEYS = [...TEXT_KEYS, 'underline', 'strike', 'smallcaps', 'superscript', 'subscript', 'highlight']

// The built-ins below mirror the server's defaults (defaultStyles in
// backend/editor.go), which win when it sends them (GET /api/books' defaults).
export const DEFAULT_FONT = 'Georgia'

const r1 = (n) => Math.round(n * 10) / 10

// The built-in styles for a body size. print: a printed book (justified,
// first-line indents, no space between paragraphs); otherwise an online
// post (left-aligned, no indent, a blank line between paragraphs).
export function builtinStyles(size, { print = true, indentFirst = 14.2, font = DEFAULT_FONT } = {}) {
  const s = (id, para, text) => ({ id, name: BUILTIN_NAMES[id], kind: 'paragraph', based_on: id === 'normal' ? null : 'normal', builtin: true, para, text })
  const align = print ? 'justify' : 'left'
  const center = print ? 'center' : 'left'
  return [
    s(
      'normal',
      { align, indentFirst: print ? indentFirst : 0, indentLeft: 0, indentRight: 0, spaceBefore: 0, spaceAfter: print ? 0 : size, lineHeight: 1.15 },
      { font, size, color: null },
    ),
    s('heading1', { align: center, indentFirst: 0, spaceBefore: r1(size * 3), spaceAfter: r1(size * 1.5), keepNext: true }, { size: r1(size * 1.6), bold: true }),
    s('heading2', { align: center, indentFirst: 0, spaceBefore: r1(size * 1.5), spaceAfter: r1(size * 0.75), keepNext: true }, { size: r1(size * 1.3), bold: true }),
    s('heading3', { align: 'left', indentFirst: 0, spaceBefore: r1(size), spaceAfter: r1(size * 0.5), keepNext: true }, { size: r1(size * 1.15), bold: true, italic: true }),
    s('title', { align: 'center', indentFirst: 0, spaceBefore: 0, spaceAfter: r1(size), keepNext: true }, { size: r1(size * 2.4) }),
    s('subtitle', { align: 'center', indentFirst: 0, spaceAfter: r1(size * 2), keepNext: true }, { size: r1(size * 1.4), italic: true }),
    s('quote', { align, indentFirst: 0, indentLeft: 28, indentRight: 28, spaceBefore: r1(size * 0.5), spaceAfter: r1(size * 0.5) }, { italic: true }),
  ]
}

export const styleById = (styles, id) => (styles || []).find((s) => s.id === id) || null

// A style's formatting with everything it is based on filled in, down to
// Normal. An unknown id is Normal. Every key comes back (null when no style
// in the chain sets it).
export function resolveStyle(styles, id) {
  const chain = []
  const seen = new Set()
  let s = styleById(styles, id) || styleById(styles, 'normal')
  while (s && !seen.has(s.id)) {
    seen.add(s.id)
    chain.unshift(s)
    if (s.id === 'normal') break
    s = styleById(styles, s.based_on || 'normal')
  }
  // A loop (or a missing based_on) still ends at Normal.
  const normal = styleById(styles, 'normal')
  if (normal && !seen.has('normal')) chain.unshift(normal)
  const para = Object.fromEntries(PARA_KEYS.map((k) => [k, null]))
  const text = Object.fromEntries(TEXT_KEYS.map((k) => [k, null]))
  for (const st of chain) {
    for (const k of PARA_KEYS) if (st.para?.[k] != null) para[k] = st.para[k]
    for (const k of TEXT_KEYS) if (st.text?.[k] != null) text[k] = st.text[k]
  }
  // Not bold, not italic unless a style says so.
  text.bold ??= false
  text.italic ??= false
  return { para, text }
}

// A character style's look: what it sets, over what the character styles it
// is based on set (an unknown id, or a paragraph style's, gives nothing).
// Only the keys some style in the chain sets come back.
export function resolveCharStyle(styles, id) {
  const chain = []
  const seen = new Set()
  for (let s = styleById(styles, id); s && s.kind === 'character' && !seen.has(s.id); s = s.based_on ? styleById(styles, s.based_on) : null) {
    seen.add(s.id)
    chain.unshift(s)
  }
  const out = {}
  for (const st of chain) for (const k of CHAR_KEYS) if (st.text?.[k] != null) out[k] = st.text[k]
  return out
}

// The style a block uses: its own, or for a heading without one the
// built-in heading of its level (Normal past Heading 3).
export function blockStyleId(typeName, attrs, styles) {
  if (attrs?.style) return attrs.style
  if (typeName === 'heading') {
    const id = `heading${attrs?.level || 1}`
    if (styleById(styles, id)) return id
  }
  return 'normal'
}

// What a paragraph or heading looks like: its own attrs over its style.
// marks: the marks at a point in it: the text's own over its character style
// over the paragraph's style.
export function resolveBlock(styles, typeName, attrs, marks = []) {
  const { para, text } = resolveStyle(styles, blockStyleId(typeName, attrs, styles))
  for (const k of PARA_KEYS) if (attrs?.[k] != null && !(k === 'keepNext' && attrs[k] === false)) para[k] = attrs[k]
  const cs = marks.find((m) => (m.type?.name ?? m.type) === 'char_style')
  if (cs) {
    const own = resolveCharStyle(styles, cs.attrs.id)
    for (const k of TEXT_KEYS) if (k in own) text[k] = own[k]
  }
  for (const m of marks) {
    const name = m.type?.name ?? m.type
    if (name === 'font') text.font = m.attrs.family
    else if (name === 'size') text.size = m.attrs.pt
    else if (name === 'color') text.color = m.attrs.color
    else if (name === 'bold') text.bold = true
    else if (name === 'italic') text.italic = true
  }
  return { para, text }
}

const esc = (id) => String(id).replace(/["\\]/g, '\\$&')
const sel = (id) => `[data-style="${esc(id)}"]`
const csel = (id) => `[data-cstyle="${esc(id)}"]`

// The stylesheet for a book's styles, under `scope` (the page's element).
// Each rule carries the style's whole resolved formatting, so a paragraph's
// own attrs (inline styles) are all that can override it.
export function styleSheet(styles, scope = '.write-doc') {
  const decls = (id) => {
    const { para, text } = resolveStyle(styles, id)
    return [...textDeclarations(text), ...paraDeclarations(para)].join('; ')
  }
  const normal = resolveStyle(styles, 'normal')
  const rules = [
    `${scope} { ${textDeclarations(normal.text).join('; ')} }`,
    // Paragraphs without a style, or with one that no longer exists.
    `${scope} :where(p, h1, h2, h3, h4, h5, h6) { ${decls('normal')} }`,
    `${scope} :where(li) > p:not([data-style]) { text-indent: 0 }`,
    // Table cells (stage 5): no first-line indent and no paragraph spacing
    // unless a paragraph sets its own (docs/EDITOR.md, "Tables").
    `${scope} :where(td, th) > p:not([data-style]) { text-indent: 0; margin-top: 0; margin-bottom: 0 }`,
  ]
  for (const s of styles || []) {
    if (s.kind === 'character') {
      // A run of text in the style (a mark's class); the text's own marks are inside it.
      const d = charDeclarations(resolveCharStyle(styles, s.id)).join('; ')
      if (d) rules.push(`${scope} ${csel(s.id)} { ${d} }`)
      continue
    }
    if (s.kind && s.kind !== 'paragraph') continue
    const selectors = [`${scope} ${sel(s.id)}`]
    const m = /^heading([1-6])$/.exec(s.id)
    if (m) selectors.push(`${scope} h${m[1]}:not([data-style])`)
    rules.push(`${selectors.join(', ')} { ${decls(s.id)} }`)
  }
  return rules.join('\n')
}

// Every font family the styles name.
export const styleFonts = (styles) => [...new Set((styles || []).map((s) => s.text?.font).filter(Boolean))]

// A new style's id from its name, unique in the book.
export function newStyleId(styles, name) {
  const base = String(name || 'style').normalize('NFKD').replace(/\p{M}/gu, '').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'style'
  let id = base
  for (let i = 2; styleById(styles, id); i++) id = `${base}-${i}`
  return id
}

// Styles that may be picked as "based on" for a style: of its kind, not
// itself nor any style based on it (no loops).
export function basedOnChoices(styles, id) {
  const kind = styleById(styles, id)?.kind || 'paragraph'
  const dependsOn = (s, target, seen = new Set()) => {
    for (let cur = s; cur && !seen.has(cur.id); cur = styleById(styles, cur.based_on)) {
      if (cur.id === target) return true
      seen.add(cur.id)
    }
    return false
  }
  return (styles || []).filter((s) => s.id !== id && (s.kind || 'paragraph') === kind && !dependsOn(s, id))
}
