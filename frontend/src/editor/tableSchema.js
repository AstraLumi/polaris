// Tables (docs/EDITOR.md, stage 5): the four nodes prosemirror-tables works
// with (table, table_row, table_cell, table_header, with its `tableRole`
// conventions: colspan, rowspan, colwidth per cell), how they are drawn and
// read back from HTML (our own copies, and tables pasted from Word, Google
// Docs and web pages), and the units.
//
// Units: the stored JSON gives column widths (`colwidth`) and a table's
// width in pt, like every length of the document. Inside the editor they
// are CSS px (what prosemirror-tables' column resizing reads and writes):
// docFromJSON turns pt into px (tableAttrsIn) and docToJSON px back into
// pt (tableAttrsOut). The data-* attributes of the HTML are in pt.
//
// How a table is drawn (here, in the page view and in the editor's
// TableView): column widths are shares of the columns' total (a column
// without a width counts as the others' average), so a table made for a
// wider page fits the text; the table is `width` wide (null: the columns'
// total, or the text's width when no column has one; 'full': the text's
// width; a number of pt), never wider than the text.
import { cssColorToHex, cssLengthToPt } from './css.js'

export const PX_PER_PT = 4 / 3
export const MAX_TABLE_COLS = 63 // Word's limit
export const MAX_TABLE_ROWS = 1000
export const TABLE_BORDERS = ['all', 'outer', 'none']
export const TABLE_ALIGNS = ['left', 'center', 'right']
export const CELL_VALIGNS = ['top', 'middle', 'bottom']

const round2 = (n) => Math.round(n * 100) / 100
export const ptToPx = (pt) => pt * PX_PER_PT
export const pxToPt = (px) => round2(px / PX_PER_PT)

// What a cell may hold: paragraphs and lists (no headings: Heading 1 starts
// a chapter; no tables: they don't nest).
export const CELL_CONTENT = '(paragraph | bullet_list | ordered_list)+'

const okWidth = (w) => typeof w === 'number' && Number.isFinite(w) && w > 0

// ---- Columns -------------------------------------------------------------------------

// Each column's width (px, or null when unknown) from the first row's
// cells, as prosemirror-tables keeps them.
export function columnWidths(table) {
  const out = []
  const row = table.firstChild
  if (!row) return out
  row.forEach((cell) => {
    const { colspan, colwidth } = cell.attrs
    for (let j = 0; j < (colspan || 1); j++) out.push(colwidth && okWidth(colwidth[j]) ? colwidth[j] : null)
  })
  return out
}

// The table's drawing: { width (CSS), cols (CSS widths, '' = share the
// rest) }. widths: columnWidths(); attrs: the table's.
export function tableLayout(attrs, widths) {
  const known = widths.filter(okWidth)
  const n = widths.length
  let cols = widths.map(() => '')
  let total = 0
  if (known.length) {
    const avg = known.reduce((a, b) => a + b, 0) / known.length
    const filled = widths.map((w) => (okWidth(w) ? w : avg))
    total = filled.reduce((a, b) => a + b, 0)
    cols = filled.map((w) => `${round2((w / total) * 100)}%`)
  }
  let width = '100%'
  if (attrs.width === 'full') width = '100%'
  else if (okWidth(attrs.width)) width = `${round2(attrs.width)}px`
  else if (known.length && known.length === n) width = `${round2(total)}px`
  return { width, cols }
}

// The table element's attributes (also given to the editor's TableView).
export function tableDOMAttrs(node) {
  const a = node.attrs
  const { width, cols } = tableLayout(a, columnWidths(node))
  const dom = { class: 'pm-table', 'data-borders': TABLE_BORDERS.includes(a.borders) ? a.borders : 'all' }
  if (TABLE_ALIGNS.includes(a.align) && a.align !== 'left') dom['data-align'] = a.align
  if (a.width === 'full') dom['data-width'] = 'full'
  else if (okWidth(a.width)) dom['data-width'] = String(pxToPt(a.width))
  const css = [`width: ${width}`]
  if (a.align === 'center') css.push('margin-left: auto', 'margin-right: auto')
  else if (a.align === 'right') css.push('margin-left: auto', 'margin-right: 0')
  dom.style = css.join('; ')
  return { attrs: dom, cols }
}

// ---- Cells -------------------------------------------------------------------------------

const clampInt = (v, lo, hi) => {
  const n = parseInt(v, 10)
  return Number.isFinite(n) ? Math.max(lo, Math.min(hi, n)) : lo
}

function valignOf(v) {
  const s = String(v || '').trim().toLowerCase()
  if (s === 'middle' || s === 'center') return 'middle'
  if (s === 'bottom') return 'bottom'
  return null
}

// A td or th → the cell's attrs (widths in px).
export function cellFromDOM(el) {
  const colspan = clampInt(el.getAttribute('colspan') || 1, 1, MAX_TABLE_COLS)
  const rowspan = clampInt(el.getAttribute('rowspan') || 1, 1, MAX_TABLE_ROWS)
  let colwidth = null
  const data = el.getAttribute('data-colwidth')
  if (data) {
    const ws = data.split(',').map(Number)
    if (ws.length === colspan && ws.some(okWidth) && ws.every((w) => w >= 0)) colwidth = ws.map((w) => (okWidth(w) ? ptToPx(w) : 0))
  } else if (colspan === 1) {
    let pt = cssLengthToPt(el.style?.width)
    const attr = el.getAttribute('width')
    if (pt == null && attr && /^\d+(\.\d+)?$/.test(attr.trim())) pt = +attr * 0.75
    if (pt > 0) colwidth = [ptToPx(pt)]
  }
  let background = null
  const own = el.getAttribute('data-background')
  if (own) background = cssColorToHex(own)
  else {
    const c = cssColorToHex(el.style?.backgroundColor) || cssColorToHex(el.getAttribute('bgcolor'))
    // White is how most sources write "no shading".
    if (c && c !== '#ffffff') background = c
  }
  const valign = valignOf(el.getAttribute('data-valign') || el.style?.verticalAlign || el.getAttribute('valign'))
  return { colspan, rowspan, colwidth, background, valign }
}

export function cellDOMAttrs(node) {
  const a = node.attrs
  const dom = {}
  if (a.colspan > 1) dom.colspan = String(a.colspan)
  if (a.rowspan > 1) dom.rowspan = String(a.rowspan)
  if (Array.isArray(a.colwidth) && a.colwidth.some(okWidth)) dom['data-colwidth'] = a.colwidth.map((w) => (okWidth(w) ? pxToPt(w) : 0)).join(',')
  const css = []
  if (a.background) {
    dom['data-background'] = a.background
    css.push(`background-color: ${a.background}`)
  }
  if (a.valign && a.valign !== 'top') {
    dom['data-valign'] = a.valign
    css.push(`vertical-align: ${a.valign}`)
  }
  if (css.length) dom.style = css.join('; ')
  return dom
}

// ---- Pasted tables -----------------------------------------------------------------------

// Borders of a table that isn't ours: a cell (or the table) drawing a
// border means all, cells saying "none" mean none; anything else all.
function bordersOf(el) {
  const own = el.getAttribute('data-borders')
  if (TABLE_BORDERS.includes(own)) return own
  const cell = el.querySelector?.('td, th')
  const style = (cell?.getAttribute('style') || '') + ';' + (el.getAttribute('style') || '')
  const decls = [...style.matchAll(/border(?:-(?:top|bottom|left|right))?(?:-style)?\s*:\s*([^;]+)/gi)].map((m) => m[1].trim().toLowerCase())
  if (decls.some((v) => !/^(none|0|0pt|0px|hidden)\b/.test(v))) return 'all'
  if (decls.length) return 'none'
  const attr = el.getAttribute('border')
  if (attr != null && attr.trim() === '0') return 'none'
  return 'all'
}

function tableFromDOM(el) {
  const out = { borders: bordersOf(el), align: null, width: null }
  const align = (el.getAttribute('data-align') || el.getAttribute('align') || '').toLowerCase()
  if (align === 'center' || align === 'right') out.align = align
  else if (/margin-left\s*:\s*auto/i.test(el.getAttribute('style') || '')) {
    out.align = /margin-right\s*:\s*auto/i.test(el.getAttribute('style') || '') ? 'center' : 'right'
  }
  const dw = el.getAttribute('data-width')
  if (dw === 'full') out.width = 'full'
  else if (dw && +dw > 0) out.width = ptToPx(+dw)
  else {
    const w = String(el.style?.width || el.getAttribute('width') || '').trim()
    if (/^(9[5-9]|100)(\.\d+)?%$/.test(w)) out.width = 'full'
  }
  return out
}

// ---- The node specs (schema.js puts them in the schema) ---------------------------------

const cellAttrs = () => ({
  colspan: { default: 1 },
  rowspan: { default: 1 },
  colwidth: { default: null },
  background: { default: null },
  valign: { default: null },
  extra: { default: null },
})

export const tableNodeSpecs = {
  // borders: 'all' | 'outer' | 'none'; align: null (left) | 'center' |
  // 'right'; width: null (from the columns) | 'full' | px (pt in JSON).
  table: {
    content: 'table_row+',
    tableRole: 'table',
    isolating: true,
    attrs: { borders: { default: 'all' }, align: { default: null }, width: { default: null }, extra: { default: null } },
    parseDOM: [{ tag: 'table', getAttrs: tableFromDOM }],
    toDOM(node) {
      const { attrs, cols } = tableDOMAttrs(node)
      return ['table', attrs, ['colgroup', ...cols.map((w) => ['col', w ? { style: `width: ${w}` } : {}])], ['tbody', 0]]
    },
  },
  table_row: {
    // Empty when cells above span all of it (as prosemirror-tables allows).
    content: '(table_cell | table_header)*',
    tableRole: 'row',
    attrs: { extra: { default: null } },
    parseDOM: [{ tag: 'tr' }],
    toDOM: () => ['tr', 0],
  },
  table_cell: {
    content: CELL_CONTENT,
    tableRole: 'cell',
    isolating: true,
    attrs: cellAttrs(),
    parseDOM: [{ tag: 'td', getAttrs: cellFromDOM }],
    toDOM: (node) => ['td', cellDOMAttrs(node), 0],
  },
  // A header cell: a row of them at the top of a table repeats on every
  // page the table runs onto (Word's "repeat as header row").
  table_header: {
    content: CELL_CONTENT,
    tableRole: 'header_cell',
    isolating: true,
    attrs: cellAttrs(),
    parseDOM: [{ tag: 'th', getAttrs: cellFromDOM }],
    toDOM: (node) => ['th', cellDOMAttrs(node), 0],
  },
}

export const TABLE_TYPES = new Set(['table', 'table_row', 'table_cell', 'table_header'])

// ---- JSON units ----------------------------------------------------------------------------

const widthIn = (w) => (w === 'full' ? 'full' : typeof w === 'number' && w > 0 ? ptToPx(w) : w == null ? null : w)
const widthOut = (w) => (w === 'full' ? 'full' : typeof w === 'number' && w > 0 ? pxToPt(w) : w)
// A column whose width isn't known is 0 (as prosemirror-tables writes it).
const colsIn = (c) => (Array.isArray(c) && c.some(okWidth) ? c.map((w) => (okWidth(w) ? ptToPx(w) : 0)) : null)
const colsOut = (c) => (Array.isArray(c) && c.some(okWidth) ? c.map((w) => (okWidth(w) ? pxToPt(w) : 0)) : null)

// Stored attrs (pt) → the editor's (px). Other attrs as they are.
export function tableAttrsIn(type, attrs) {
  if (!attrs) return attrs
  if (type === 'table' && 'width' in attrs) return { ...attrs, width: widthIn(attrs.width) }
  if ((type === 'table_cell' || type === 'table_header') && 'colwidth' in attrs) {
    const colwidth = colsIn(attrs.colwidth)
    const span = attrs.colspan > 0 ? attrs.colspan : 1
    return { ...attrs, colwidth: colwidth && colwidth.length === span ? colwidth : null }
  }
  return attrs
}

// The editor's attrs (px) → stored (pt). A cell's colspan and rowspan of 1
// are left out like other empty values.
export function tableAttrsOut(type, attrs) {
  if (!attrs) return attrs
  if (type === 'table' && 'width' in attrs) return { ...attrs, width: widthOut(attrs.width) }
  if (type === 'table_cell' || type === 'table_header') {
    const out = { ...attrs }
    if ('colwidth' in out) out.colwidth = colsOut(out.colwidth)
    if (out.colspan === 1) delete out.colspan
    if (out.rowspan === 1) delete out.rowspan
    return out
  }
  return attrs
}
