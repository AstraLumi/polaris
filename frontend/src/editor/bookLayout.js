// A book's layout (docs/EDITOR.md, "Stage 3 and 4 additions"): its details
// (meta), the header and footer bands, page numbers and where chapters
// start, the options per chapter, and the CSS paged media the page view
// lays the book out with (paged.js). Pure: no DOM, no Vue. The server's
// defaults are booklayout.go's (defaultBookLayout); keep the two the same.
import { noteSize } from './css.js'

export const BAND_FIELDS = ['{page}', '{chapter}', '{chapter_number}', '{book}', '{author}']
export const NUMBER_FORMATS = ['arabic', 'roman', 'upper_roman', 'alpha', 'upper_alpha']
export const META_KEYS = ['title', 'subtitle', 'author', 'language', 'publisher', 'isbn', 'rights', 'description']
export const BAND_SLOTS = ['odd', 'even', 'first']

const COUNTER_STYLES = { arabic: 'decimal', roman: 'lower-roman', upper_roman: 'upper-roman', alpha: 'lower-alpha', upper_alpha: 'upper-alpha' }

export const defaultBand = (center = '') => ({ left: '', center, right: '', font: null, size: null, italic: false, smallcaps: false, uppercase: false })

export function defaultLayout() {
  return {
    different_first: true,
    different_odd_even: true,
    header: { odd: defaultBand('{chapter}'), even: defaultBand('{book}'), first: defaultBand() },
    footer: { odd: defaultBand('{page}'), even: defaultBand('{page}'), first: defaultBand('{page}') },
    chapter_start: 'next',
    numbering: { format: 'arabic', start: 1 },
    toc_dots: true,
  }
}

export const defaultMeta = () => ({ title: '', subtitle: '', author: '', language: 'en', publisher: '', isbn: '', rights: '', description: '' })

export const defaultChapterOption = () => ({ front_matter: false, number_restart: null, no_header: false })

const isObj = (v) => v && typeof v === 'object' && !Array.isArray(v)

// Missing keys (and nulls where a default isn't null) take the defaults;
// keys this code doesn't know are kept.
function fill(have, def) {
  const out = isObj(have) ? { ...have } : {}
  for (const [k, d] of Object.entries(def)) {
    if (out[k] === undefined || (out[k] === null && d !== null)) out[k] = isObj(d) ? fill(null, d) : d
    else if (isObj(d) && isObj(out[k])) out[k] = fill(out[k], d)
  }
  return out
}

export function normaliseBand(b) {
  const out = fill(b, defaultBand())
  for (const k of ['left', 'center', 'right']) out[k] = typeof out[k] === 'string' ? out[k] : ''
  return out
}

export function normaliseLayout(layout) {
  const out = fill(layout, defaultLayout())
  for (const part of ['header', 'footer']) for (const slot of BAND_SLOTS) out[part][slot] = normaliseBand(out[part][slot])
  if (!NUMBER_FORMATS.includes(out.numbering.format)) out.numbering.format = 'arabic'
  const start = Math.round(+out.numbering.start)
  out.numbering.start = Number.isFinite(start) && start >= 0 ? Math.min(start, 99999) : 1
  if (out.chapter_start !== 'odd') out.chapter_start = 'next'
  return out
}

export const normaliseMeta = (meta) => fill(meta, defaultMeta())

export function normaliseChapterOptions(opts) {
  const out = {}
  if (!isObj(opts)) return out
  for (const [k, v] of Object.entries(opts)) if (/^[1-9]\d*$/.test(k)) out[k] = fill(v, defaultChapterOption())
  return out
}

export const chapterOption = (settings, id) => fill(settings?.chapter_options?.[String(id)], defaultChapterOption())

// A chapter's options changed; options back at their defaults are dropped.
export function setChapterOption(settings, id, patch) {
  const opts = { ...(settings.chapter_options || {}) }
  const next = { ...chapterOption(settings, id), ...patch }
  const def = defaultChapterOption()
  const plain = Object.keys(next).every((k) => (k in def ? next[k] === def[k] : false))
  if (plain) delete opts[String(id)]
  else opts[String(id)] = next
  return { ...settings, chapter_options: opts }
}

// ---- Bands and fields ----------------------------------------------------------

// A band's text as pieces: { text } literal or { field } ('page', 'chapter'…).
// Anything in braces that isn't a field stays literal.
export function bandTokens(text) {
  const out = []
  const s = String(text || '')
  const re = /\{(page|chapter|chapter_number|book|author)\}/g
  let last = 0
  for (const m of s.matchAll(re)) {
    if (m.index > last) out.push({ text: s.slice(last, m.index) })
    out.push({ field: m[1] })
    last = m.index + m[0].length
  }
  if (last < s.length) out.push({ text: s.slice(last) })
  return out
}

export const counterStyle = (format) => COUNTER_STYLES[format] || 'decimal'

const ROMAN = [
  [1000, 'm'], [900, 'cm'], [500, 'd'], [400, 'cd'], [100, 'c'], [90, 'xc'],
  [50, 'l'], [40, 'xl'], [10, 'x'], [9, 'ix'], [5, 'v'], [4, 'iv'], [1, 'i'],
]

// A page number written in a format (as CSS counters write it).
export function formatNumber(n, format) {
  n = Math.round(+n)
  if (!Number.isFinite(n)) return ''
  if ((format === 'roman' || format === 'upper_roman') && n > 0 && n < 4000) {
    let s = ''
    for (const [v, r] of ROMAN) while (n >= v) (s += r), (n -= v)
    return format === 'upper_roman' ? s.toUpperCase() : s
  }
  if ((format === 'alpha' || format === 'upper_alpha') && n > 0) {
    let s = ''
    for (let x = n; x > 0; x = Math.floor((x - 1) / 26)) s = String.fromCharCode(97 + ((x - 1) % 26)) + s
    return format === 'upper_alpha' ? s.toUpperCase() : s
  }
  return String(n)
}

// A CSS string.
export const cssString = (s) => `"${String(s).replace(/\\/g, '\\\\').replace(/"/g, '\\"').replace(/\n/g, '\\a ')}"`

// The CSS `content` of a band's text. ctx: { counter (CSS counter style),
// book, author }. {chapter} and {chapter_number} are the named strings
// pvchapter and pvchapnum.
export function bandContent(text, ctx = {}) {
  const parts = bandTokens(text).map((t) => {
    if (t.text != null) return cssString(t.text)
    switch (t.field) {
      case 'page':
        return `counter(page, ${ctx.counter || 'decimal'})`
      case 'chapter':
        return 'string(pvchapter)'
      case 'chapter_number':
        return 'string(pvchapnum)'
      case 'book':
        return cssString(ctx.book || '')
      case 'author':
        return cssString(ctx.author || '')
    }
    return '""'
  })
  return parts.length ? parts.join(' ') : 'none'
}

// A band's text with the fields filled in (for previews).
export function bandText(text, values = {}) {
  return bandTokens(text)
    .map((t) => (t.text != null ? t.text : values[t.field] ?? ''))
    .join('')
}

// ---- The book's plan -------------------------------------------------------------

// How each chapter is laid out: { id, front, header (drawn), format,
// restart (a page number or null), number (its chapter number, "" for
// front matter) }. Front matter is numbered in lower roman from i; the
// first other chapter restarts at numbering.start, any chapter at its
// number_restart.
export function bookPlan(settings, chapters) {
  const layout = normaliseLayout(settings?.layout)
  let firstMain = true
  let n = 0
  let anyFront = false
  return (chapters || []).map((c, i) => {
    const o = chapterOption(settings, c.id)
    const front = !!o.front_matter
    let restart = null
    if (front) {
      if (!anyFront) restart = 1
      anyFront = true
    } else if (firstMain) {
      restart = layout.numbering.start
      firstMain = false
    }
    if (o.number_restart != null && Number.isFinite(+o.number_restart)) restart = Math.max(0, Math.round(+o.number_restart))
    if (i === 0 && restart == null) restart = 1
    return {
      id: c.id,
      title: c.title || '',
      front,
      header: !front && !o.no_header,
      format: front ? 'roman' : layout.numbering.format,
      restart,
      number: front ? '' : String(++n),
    }
  })
}

// The page names a chapter's pages take (each with its own header rules).
export const pageName = (plan) => (plan.front ? 'pvfront' : plan.header ? 'pvmain' : 'pvnohead')

// ---- CSS paged media -----------------------------------------------------------

const BOXES = ['left', 'center', 'right']
const mm = (v) => `${Math.round((+v || 0) * 100) / 100}mm`

function boxDecls(band, base) {
  const out = []
  const font = band.font || base.font
  if (font) out.push(`font-family: ${cssString(font)}, serif`)
  out.push(`font-size: ${+band.size || base.size || 9}pt`)
  out.push(`font-style: ${band.italic ? 'italic' : 'normal'}`)
  if (band.smallcaps) out.push('font-variant: small-caps')
  if (band.uppercase) out.push('text-transform: uppercase')
  out.push(`color: ${base.color || '#000'}`)
  return out
}

// The @page margin boxes for one band (top or bottom), or none.
function bandBoxes(band, edge, ctx, base) {
  return BOXES.map((pos) => {
    const content = band ? bandContent(band[pos], ctx) : 'none'
    const decls = content === 'none' ? ['content: none'] : [`content: ${content}`, ...boxDecls(band, base)]
    return `  @${edge}-${pos} { ${decls.join('; ')} }`
  }).join('\n')
}

// The CSS the page view gives paged.js: page size and margins (mirrored,
// gutter), headers and footers per page name and side, chapter starts,
// page number restarts, named strings for {chapter}, full-page pictures,
// the table of contents' page numbers. ctx: { book, author, font, size,
// color (the body text's), plan (bookPlan) }.
export function pagedCSS(settings, ctx = {}) {
  const p = settings?.page || {}
  const layout = normaliseLayout(settings?.layout)
  const plan = ctx.plan || []
  const inner = (+p.margin_inner_mm || 0) + (+p.gutter_mm || 0)
  const outer = +p.margin_outer_mm || 0
  const top = +p.margin_top_mm || 0
  const bottom = +p.margin_bottom_mm || 0
  const w = +p.width_mm || 148
  const h = +p.height_mm || 210
  const mirror = p.mirror !== false
  const base = { font: ctx.font, size: ctx.size, color: ctx.color }
  const css = []
  css.push(`@page { size: ${mm(w)} ${mm(h)}; margin: ${mm(top)} ${mm(outer)} ${mm(bottom)} ${mm(inner)}; }`)
  if (mirror) {
    css.push(`@page :left { margin-left: ${mm(outer)}; margin-right: ${mm(inner)}; }`)
    css.push(`@page :right { margin-left: ${mm(inner)}; margin-right: ${mm(outer)}; }`)
  }

  const band = (part, slot) => {
    const bands = layout[part]
    if (slot === 'even' && !layout.different_odd_even) slot = 'odd'
    const b = bands[slot]
    return b && (b.left + b.center + b.right).trim() ? b : null
  }
  const names = [
    { name: 'pvmain', header: true, counter: counterStyle(layout.numbering.format) },
    { name: 'pvnohead', header: false, counter: counterStyle(layout.numbering.format) },
    { name: 'pvfront', header: false, counter: 'lower-roman' },
  ]
  for (const n of names) {
    const c = { ...ctx, counter: n.counter }
    for (const [side, slot] of [['right', 'odd'], ['left', 'even']]) {
      css.push(`@page ${n.name}:${side} {\n${bandBoxes(n.header ? band('header', slot) : null, 'top', c, base)}\n${bandBoxes(band('footer', slot), 'bottom', c, base)}\n}`)
    }
  }
  // A chapter's first page (after the sides, so it wins).
  if (layout.different_first) {
    for (const n of names) {
      const c = { ...ctx, counter: n.counter }
      css.push(`@page ${n.name}:first {\n${bandBoxes(n.header ? band('header', 'first') : null, 'top', c, base)}\n${bandBoxes(band('footer', 'first'), 'bottom', c, base)}\n}`)
    }
  }
  // Full-page pictures: the whole page, nothing round them.
  css.push(`@page pvfull { margin: 0;\n${bandBoxes(null, 'top', ctx, base)}\n${bandBoxes(null, 'bottom', ctx, base)}\n}`)
  css.push(`@page :blank {\n${bandBoxes(null, 'top', ctx, base)}\n${bandBoxes(null, 'bottom', ctx, base)}\n}`)

  css.push('.pv-chapter { string-set: pvchapter attr(data-title), pvchapnum attr(data-number); }')
  for (const n of names) css.push(`.pv-chapter.pv-${n.name} { page: ${n.name}; }`)
  const start = layout.chapter_start === 'odd' ? 'right' : 'page'
  css.push(`.pv-chapter.pv-later, .pv-full.pv-later { break-before: ${start}; }`)
  css.push('.pv-page-break { break-after: page; height: 0; margin: 0; border: 0; }')
  // Tables break between rows, not inside one (stage 5).
  css.push('.pm-table tr { break-inside: avoid; }')
  css.push(`.pv-full { page: pvfull; width: ${mm(w)}; height: ${mm(h - 0.5)}; margin: 0; padding: 0; overflow: hidden; }`)
  css.push('.pv-full > figure { margin: 0; width: 100%; height: 100%; }')
  css.push('.pv-full img { width: 100% !important; height: 100% !important; max-width: none; object-fit: contain; display: block; }')
  for (const c of plan) if (c.restart != null) css.push(`#pv-ch-${c.id} { counter-reset: page ${c.restart}; }`)
  // The table of contents' page numbers, in each chapter's own format.
  css.push(`.pv-toc a.pv-n-main::after { content: target-counter(attr(href), page, ${counterStyle(layout.numbering.format)}); }`)
  css.push('.pv-toc a.pv-n-front::after { content: target-counter(attr(href), page, lower-roman); }')
  css.push(...notesCSS(ctx))
  return css.join('\n')
}

// Footnotes (stage 5) at the foot of the page they are called on: paged.js
// moves each span.pv-fn.pv-footnote (pageView.js) into the page's footnote
// area. Its own call and marker (a counter running through the whole book)
// are hidden: the note's number is written in the text (sup.pv-fn-call)
// and in the note (span.pv-fn-num), counted per chapter. Notes are set in
// the body's font at 5/6 of its size, on the half point (Word's 10 pt
// notes under 12 pt text: noteSize), under a short rule.

export function notesCSS(ctx = {}) {
  const font = ctx.font ? `font-family: ${cssString(ctx.font)}, serif; ` : ''
  return [
    '@page { @footnote { margin-top: 0.8em; padding-top: 0.5em; background: linear-gradient(currentColor, currentColor) top left / 30% 0.5pt no-repeat; } }',
    '.pv-fn.pv-footnote { float: footnote; }',
    '.pv-fn-wrap > a[data-footnote-call]::after { content: none; }',
    `.pv-fn[data-footnote-marker] { display: block; list-style: none; ${font}font-size: ${noteSize(ctx.size)}pt; font-weight: normal; font-style: normal; color: ${ctx.color || '#000'}; text-align: left; text-indent: 0; line-height: 1.2; }`,
    '.pv-fn[data-footnote-marker]::marker { content: none; }',
    '.pv-fn-num { vertical-align: super; font-size: 0.75em; line-height: 0; margin-right: 0.3em; }',
  ]
}

// The entries of a table of contents: every heading up to `levels` deep
// in the chapters' docs (JSON), each { level, text, chapter, block (its
// index in the chapter's doc), front }; a chapter without a Heading 1
// (other than front matter) is listed by its title. A note in a heading
// isn't part of its text.
export function tocEntries(chapters, plan, levels = 1) {
  const out = []
  const textOf = (n) =>
    n.type === 'text' ? n.text || '' : n.type === 'hard_break' || n.type === 'tab' ? ' ' : n.type === 'footnote' ? '' : (n.content || []).map(textOf).join('')
  ;(chapters || []).forEach((ch, ci) => {
    const pl = plan[ci] || {}
    const at = out.length
    let hasH1 = false
    ;(ch.doc?.content || []).forEach((n, bi) => {
      if (n?.type !== 'heading') return
      const level = +n.attrs?.level || 1
      if (level > levels) return
      const text = textOf(n).replace(/\s+/g, ' ').trim()
      if (!text) return
      if (level === 1) hasH1 = true
      out.push({ level, text, chapter: ch.id, block: bi, front: !!pl.front })
    })
    if (!hasH1 && !pl.front && ch.title) out.splice(at, 0, { level: 1, text: ch.title, chapter: ch.id, block: null, front: false })
  })
  return out
}
