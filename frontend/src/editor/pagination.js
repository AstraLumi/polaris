// Page breaks while writing (docs/EDITOR.md, "Page breaks while writing"):
// a ProseMirror plugin that measures the laid-out text, asks pageBreaks.js
// where each page ends and draws the gap between two pages as widget
// decorations. The document is never changed.
//
// Measuring reads the browser's own layout (line boxes through Range
// rects), in px as if the sheet were not zoomed (the sheet is scaled with
// CSS `zoom`; the scale is the doc's measured width over its known width).
// Positions are taken as if no gap were there: the gaps already drawn are
// known (each one's `shift`) and taken off, so measuring again after a gap
// was put in gives the same answer.
//
// A gap is one of two things:
//  - a float inside the paragraph, just before the first character of the
//    page's first line (a float as wide as the sheet, so no text fits
//    beside it and the paragraph's first-line indent still applies to the
//    text below it, and a list marker stays with the text);
//  - a block before a picture, a table, a scene break... (a unit).
// Both are as high as: the rest of the page, its foot (the bottom margin,
// with the footer), the space between two sheets and the next page's head
// (the top margin, with the header), so the next page's text starts exactly
// where a page's text starts.
import { Plugin, PluginKey } from 'prosemirror-state'
import { Decoration, DecorationSet } from 'prosemirror-view'
import { paginate, gapHeight, pageTops } from './pageBreaks.js'
import { resolveBlock } from './styles.js'

export const pagesKey = new PluginKey('pages')

export const MM = 96 / 25.4
export const SPACE_PX = 26 // the dark space between two sheets
const DELAY = 140 // idle time after a change before measuring
const MAX_CHAIN = 5 // measuring again after our own change, at most this often

// How long the last measuring took (for tests and tuning).
export const stats = { ms: 0, passes: 0 }

const EMPTY = { deco: DecorationSet.empty, gaps: [] }

// Options (getOptions() → null when pages are not shown):
//   widthPx: the text block's width (px, not zoomed); heightMm, topMm,
//   bottomMm: the page; bands(k): { head, foot } for page k (0-based), each
//   [left, centre, right] text, or null; sig: changes when any of it does.
// getStyles(): the book's styles (for keepNext).
export function pagesPlugin({ getOptions, getStyles = () => [] }) {
  return new Plugin({
    key: pagesKey,
    state: {
      init: () => ({ ...EMPTY, nonce: 0 }),
      apply(tr, value) {
        const meta = tr.getMeta(pagesKey)
        if (meta?.set) return { deco: meta.set.deco, gaps: meta.set.gaps, nonce: value.nonce }
        if (meta?.clear) return { ...EMPTY, nonce: value.nonce }
        let next = value
        if (tr.docChanged && value.gaps.length) {
          const deco = value.deco.map(tr.mapping, tr.doc)
          next = { ...value, deco, gaps: deco.find().map((d) => ({ ...d.spec, pos: d.from })) }
        }
        if (meta?.refresh) next = { ...next, nonce: value.nonce + 1 }
        return next
      },
    },
    props: {
      decorations: (state) => pagesKey.getState(state).deco,
    },
    view(view) {
      let timer = null
      let raf = 0
      let chain = 0
      const run = () => {
        raf = 0
        if (!view.dom.isConnected) return
        if (view.composing) return schedule(DELAY)
        const opts = getOptions()
        const st = pagesKey.getState(view.state)
        if (!opts) {
          if (st.gaps.length) view.dispatch(view.state.tr.setMeta(pagesKey, { clear: true }).setMeta('addToHistory', false))
          return
        }
        const t0 = performance.now()
        const made = measureAndPlan(view, opts, st, getStyles())
        stats.ms = performance.now() - t0
        stats.passes++
        if (!made) return
        if (sameGaps(st.gaps, made.gaps)) {
          chain = 0
          return
        }
        view.dispatch(view.state.tr.setMeta(pagesKey, { set: made }).setMeta('addToHistory', false))
        if (++chain < MAX_CHAIN) schedule(40)
        else chain = 0
      }
      function schedule(delay = DELAY) {
        clearTimeout(timer)
        if (raf) cancelAnimationFrame(raf)
        raf = 0
        timer = setTimeout(() => (raf = requestAnimationFrame(run)), delay)
      }
      const onFonts = () => schedule()
      document.fonts?.addEventListener?.('loadingdone', onFonts)
      const ro = typeof ResizeObserver === 'function' ? new ResizeObserver(() => schedule()) : null
      ro?.observe(view.dom)
      schedule(0)
      return {
        update(v, prev) {
          const a = pagesKey.getState(v.state)
          const b = pagesKey.getState(prev)
          if (v.state.doc !== prev.doc) {
            chain = 0
            schedule()
          } else if (a.nonce !== b.nonce) {
            chain = 0
            schedule(0)
          }
        },
        destroy() {
          clearTimeout(timer)
          if (raf) cancelAnimationFrame(raf)
          document.fonts?.removeEventListener?.('loadingdone', onFonts)
          ro?.disconnect()
        },
      }
    },
  })
}

// Measure again (the page, the zoom, the styles or the switch changed).
export function refreshPages(view) {
  if (view && !view.isDestroyed) view.dispatch(view.state.tr.setMeta(pagesKey, { refresh: true }).setMeta('addToHistory', false))
}

// The gaps now drawn, for tests and the status line: [{ pos, page, hw, unit, hard }].
export const gapsOf = (state) => pagesKey.getState(state)?.gaps || []

function sameGaps(a, b) {
  if (a.length !== b.length) return false
  for (let i = 0; i < a.length; i++) {
    const x = a[i]
    const y = b[i]
    if (x.pos !== y.pos || x.unit !== y.unit || !!x.tail !== !!y.tail || Math.abs(x.hw - y.hw) > 1 || Math.abs(x.offL - y.offL) > 1 || Math.abs(x.width - y.width) > 1 || x.sig !== y.sig) return false
  }
  return true
}

const num = (v) => parseFloat(v) || 0

// ---- Measuring -------------------------------------------------------------------

const skipText = (t, stop) => {
  for (let n = t.parentElement; n && n !== stop; n = n.parentElement) {
    if (n.classList?.contains('ProseMirror-widget') || n.classList?.contains('pg-gap') || n.getAttribute?.('contenteditable') === 'false') return true
  }
  return false
}

// The first offset in text node `t` that is on its k-th line box (rects[k]).
function lineStart(range, t, rects, k) {
  const want = rects[k].top
  let lo = 0
  let hi = t.data.length
  while (lo < hi) {
    const mid = (lo + hi) >> 1
    range.setStart(t, mid)
    range.setEnd(t, mid + 1)
    const r = range.getClientRects()[0]
    if (r && r.height > 0 && r.top + r.height / 2 >= want) hi = mid
    else lo = mid + 1
  }
  return lo
}

// The text lines of a textblock: [{ pos, gt, gb }] (glyph top and bottom, px).
function textLines(view, content, range, y) {
  const lines = []
  const walker = document.createTreeWalker(content, NodeFilter.SHOW_TEXT)
  for (let t = walker.nextNode(); t; t = walker.nextNode()) {
    if (!t.data.length || skipText(t, content)) continue
    range.selectNodeContents(t)
    const rects = [...range.getClientRects()].filter((r) => r.height > 0.5)
    for (let k = 0; k < rects.length; k++) {
      const gt = y(rects[k].top)
      const gb = y(rects[k].bottom)
      const last = lines[lines.length - 1]
      if (last && (gt + gb) / 2 <= last.gb + 0.5) {
        last.gt = Math.min(last.gt, gt)
        last.gb = Math.max(last.gb, gb)
        continue
      }
      const off = k === 0 ? 0 : lineStart(range, t, rects, k)
      lines.push({ pos: view.posAtDOM(t, off), gt, gb })
    }
  }
  return lines
}

function collectItems(view, ctx) {
  const { doc } = view.state
  const { root, sr, s, y, gapAt, styles } = ctx
  const items = []
  const range = document.createRange()
  const kn = new Map()
  const keepNext = (node) => {
    const key = `${node.type.name}|${node.attrs.style}|${node.attrs.level}|${node.attrs.keepNext}`
    if (!kn.has(key)) kn.set(key, !!resolveBlock(styles, node.type.name, node.attrs).para.keepNext)
    return kn.get(key)
  }
  let block = 0
  const left = (rect) => (rect.left - sr.left) / s
  doc.descendants((node, pos) => {
    if (node.isTextblock) {
      const el = view.nodeDOM(pos)
      if (!(el instanceof HTMLElement)) return false
      const content = el.querySelector(':scope > .pm-line') || el
      const cs = getComputedStyle(el)
      const ccs = content === el ? cs : getComputedStyle(content)
      const cr = content.getBoundingClientRect()
      const contentTop = y(cr.top) + num(ccs.borderTopWidth) + num(ccs.paddingTop)
      const contentBottom = y(cr.bottom) - num(ccs.borderBottomWidth) - num(ccs.paddingBottom)
      const offL = left(cr) + num(ccs.borderLeftWidth) + num(ccs.paddingLeft)
      let lines = node.content.size ? textLines(view, content, range, y) : []
      if (!lines.length) lines = [{ pos: pos + 1, gt: null, gb: null }]
      const hwOf = (i) => {
        const g = gapAt.get(lines[i].pos)
        return g && !g.unit ? g.hw : 0
      }
      const mt = Math.max(0, num(cs.marginTop))
      const mb = Math.max(0, num(cs.marginBottom))
      const kNext = keepNext(node)
      const id = block++
      for (let i = 0; i < lines.length; i++) {
        const l = lines[i]
        const top = i === 0 ? contentTop + hwOf(0) : l.gt - (l.gt - lines[i - 1].gb - hwOf(i)) / 2
        const bottom = i === lines.length - 1 ? contentBottom : l.gb + (lines[i + 1].gt - l.gb - hwOf(i + 1)) / 2
        items.push({ pos: l.pos, top, bottom, block: id, first: i === 0, last: i === lines.length - 1, keepNext: kNext, hard: false, unit: false, spaceBefore: mt, mt, mb, offL })
      }
      return false
    }
    if (node.isLeaf || node.type.name === 'table' || node.type.name === 'toc') {
      const el = view.nodeDOM(pos)
      if (!(el instanceof HTMLElement)) return false
      const cs = getComputedStyle(el)
      if (cs.float !== 'none' || cs.display === 'none') return false // wrapped pictures take no room of their own
      const r = el.getBoundingClientRect()
      const parent = el.parentElement || root
      const pr = parent.getBoundingClientRect()
      const hard = node.type.name === 'page_break'
      const top = y(r.top)
      const mt = Math.max(0, num(cs.marginTop))
      items.push({
        pos,
        top,
        bottom: hard ? top : y(r.bottom),
        block: block++,
        first: true,
        last: true,
        keepNext: false,
        hard,
        unit: true,
        spaceBefore: mt,
        mt,
        mb: Math.max(0, num(cs.marginBottom)),
        offL: left(pr) + num(getComputedStyle(parent).paddingLeft),
      })
      return false
    }
    return true
  })
  // Take off the gaps already drawn (a gap sits before its position).
  const gaps = [...gapAt.values()].filter((g) => !g.tail).sort((a, b) => a.pos - b.pos)
  let gi = 0
  let shift = 0
  for (let i = 0; i < items.length; i++) {
    while (gi < gaps.length && gaps[gi].pos <= items[i].pos) shift += gaps[gi++].shift
    items[i].top -= shift
    items[i].bottom -= shift
  }
  return items
}

function measureAndPlan(view, opts, st, styles) {
  const root = view.dom
  const rr = root.getBoundingClientRect()
  const s = rr.width / opts.widthPx
  if (!(s > 0.05) || !(rr.height > 0)) return null
  const sheet = root.closest('.sheet')
  const sr = (sheet || root).getBoundingClientRect()
  const y = (v) => (v - rr.top) / s
  const gapAt = new Map()
  for (const g of st.gaps) gapAt.set(g.pos, g)
  const items = collectItems(view, { root, sr, s, y, gapAt, styles })
  // The sheet's content starts above the doc when the first block's space before
  // collapses through it (the page begins there, margin included).
  const origin = sheet ? Math.min(0, y(sr.top) + num(getComputedStyle(sheet).paddingTop)) : 0
  const H = (opts.heightMm - opts.topMm - opts.bottomMm) * MM
  if (!(H > 20) || !items.length) return { deco: DecorationSet.empty, gaps: [] }
  const topM = opts.topMm * MM
  const botM = opts.bottomMm * MM
  const extra = botM + SPACE_PX + topM
  const breaks = paginate(items, H, { origin })
  const tops = pageTops(items, breaks, origin)
  const sheetW = sr.width / s
  const textLeft = (root.getBoundingClientRect().left - sr.left) / s
  const padL = Math.max(0, textLeft)
  const padR = Math.max(0, sheetW - padL - opts.widthPx)
  const gaps = []
  breaks.forEach((b, i) => {
    const it = items[b.index]
    const prev = items[b.index - 1]
    const pageTop = tops[i]
    const bands = opts.bands?.(i) || null
    const bandsNext = opts.bands?.(i + 1) || null
    let spec
    if (it.unit) {
      const w = prev.bottom + prev.mb
      const fill = Math.max(0, Math.max(pageTop + H, w) - w)
      const head = Math.max(0, topM - (b.hard ? 0 : it.mt))
      const hw = fill + botM + SPACE_PX + head
      spec = { unit: true, fill, head, hw, shift: hw + Math.min(prev.mb, it.mt) }
    } else {
      const keep = b.hard ? it.spaceBefore : 0
      const hw = gapHeight({ w: it.top, pageTop, contentHeight: H, extra, keep })
      const fill = Math.max(0, Math.max(pageTop + H, it.top) - it.top)
      spec = { unit: false, fill, head: topM + keep, hw, shift: hw }
    }
    Object.assign(spec, { pos: it.pos, page: i, hard: b.hard, foot: botM, offL: it.offL, width: sheetW, padL, padR, tail: false, foots: bands?.foot || null, heads: bandsNext?.head || null })
    spec.sig = `${spec.foots?.join('\u0001')}|${spec.heads?.join('\u0001')}|${padL.toFixed(0)}|${padR.toFixed(0)}`
    gaps.push(spec)
  })
  // The last page, drawn to its foot.
  const last = items[items.length - 1]
  const lastTop = tops[tops.length - 1]
  const bands = opts.bands?.(breaks.length) || null
  const tail = { unit: true, tail: true, pos: view.state.doc.content.size, page: breaks.length, fill: Math.max(0, lastTop + H - last.bottom), foot: botM, shift: 0, offL: padL, width: sheetW, padL, padR, hw: 0, foots: bands?.foot || null, heads: null }
  tail.hw = tail.fill
  tail.sig = `${tail.foots?.join('\u0001')}|${padL.toFixed(0)}|${padR.toFixed(0)}`
  gaps.push(tail)
  return { gaps, deco: DecorationSet.create(view.state.doc, gaps.map(widgetFor)) }
}

// ---- Drawing -----------------------------------------------------------------------

function bandRow(texts, height, padL, padR) {
  const row = document.createElement('div')
  row.className = 'pg-band'
  row.style.height = `${height}px`
  row.style.padding = `0 ${padR}px 0 ${padL}px`
  const cells = texts || ['', '', '']
  for (let i = 0; i < 3; i++) {
    const c = document.createElement('span')
    c.textContent = cells[i] || ''
    row.appendChild(c)
  }
  return row
}

const block = (cls, height) => {
  const d = document.createElement('div')
  d.className = cls
  d.style.height = `${Math.max(0, height)}px`
  return d
}

function gapDOM(g) {
  const d = document.createElement('div')
  d.className = `pg-gap ${g.tail ? 'is-tail' : g.unit ? 'is-block' : 'is-float'}`
  d.setAttribute('contenteditable', 'false')
  d.setAttribute('aria-hidden', 'true')
  d.dataset.page = String(g.page + 1)
  if (g.hard) d.dataset.hard = '1'
  d.style.height = `${g.hw}px`
  d.style.width = `${g.width + (g.tail || g.unit ? 0 : 1)}px`
  d.style.marginLeft = `${-g.offL}px`
  const fill = block('pg-fill', g.fill)
  d.appendChild(fill)
  if (g.tail) {
    // The foot hangs in the sheet's bottom margin, below this box.
    const foot = bandRow(g.foots, g.foot, g.padL, g.padR)
    foot.classList.add('is-tail-foot')
    d.appendChild(foot)
    return d
  }
  d.appendChild(bandRow(g.foots, g.foot, g.padL, g.padR))
  d.appendChild(block('pg-space', SPACE_PX))
  const head = bandRow(g.heads, g.head, g.padL, g.padR)
  head.classList.add('is-head')
  d.appendChild(head)
  return d
}

function widgetFor(g) {
  const key = `pg:${g.page}:${g.pos}:${g.hw.toFixed(1)}:${g.offL.toFixed(1)}:${g.width.toFixed(1)}:${g.sig}:${g.unit ? 'b' : 'f'}`
  return Decoration.widget(g.pos, () => gapDOM(g), { ...g, key, side: g.tail ? 1 : -1, ignoreSelection: true, stopEvent: () => true })
}
