// Where the pages of a chapter break while writing (docs/EDITOR.md, "Page
// breaks while writing"). Pure: no DOM. The editor measures the laid-out
// text (pagination.js) and gives the lines in order; this says which line
// starts each new page, the way Word's Print Layout does.
//
// An item is one line of text, or one block that cannot be split (a
// picture, a table, a scene break, a hard page break):
//   { pos, top, bottom, block, first, last, keepNext, hard, unit, spaceBefore }
// pos: the document position a gap goes at (a line's first character, a
// unit's position); top, bottom: the line's box, in px as if there were no
// gaps; block: an id shared by the lines of one paragraph; first, last:
// the item is its block's first / last line; keepNext: the block stays on
// the page of the next one (a heading); hard: a forced page break (the
// next item starts a page; the break itself never takes room); unit: not
// splittable (a unit taller than a page starts its own page and overflows);
// spaceBefore: the block's space before (px), which stays after a hard
// break and is dropped after a natural one.
//
// paginate(items, contentHeight, { origin }) → [{ index, pos, hard }]: the
// items that start a page after the first. Rules, as Word:
//  - a line fits when its bottom is within the page's content height;
//  - widow/orphan control: a paragraph is never split leaving one line at
//    a page's foot or head; a paragraph of 2-3 lines moves whole;
//  - keepNext keeps a block with the first lines of the next one (unless
//    that would leave the page empty);
//  - a hard break starts a new page with the item after it.

const EPS = 0.6

export function paginate(items, contentHeight, opts = {}) {
  const breaks = []
  const n = items.length
  if (!n || !(contentHeight > 0)) return breaks
  const eps = opts.eps ?? EPS
  let start = 0
  let pageTop = opts.origin ?? items[0].top
  let i = 0
  while (i < n) {
    const it = items[i]
    if (it.hard) {
      if (i + 1 < n) {
        start = i + 1
        pageTop = items[start].top - (items[start].spaceBefore || 0)
        breaks.push({ index: start, pos: items[start].pos, hard: true })
      }
      i++
      continue
    }
    if (i > start && it.bottom - pageTop > contentHeight + eps) {
      const c = adjust(items, i, start)
      breaks.push({ index: c, pos: items[c].pos, hard: false })
      start = c
      pageTop = items[c].top
      i = c
      continue
    }
    i++
  }
  return breaks
}

// Where the page really breaks when `raw` is the first item that does not
// fit: back to a block start for widows, orphans and keepNext.
function adjust(items, raw, start) {
  const n = items.length
  const blockStart = (idx) => {
    let s = idx
    while (s > start && items[s - 1].block === items[idx].block && !items[s].first) s--
    return s
  }
  const blockEnd = (idx) => {
    let e = idx
    while (e + 1 < n && items[e + 1].block === items[idx].block && !items[e].last) e++
    return e
  }
  let c = raw
  for (let guard = 0; guard < 60; guard++) {
    const it = items[c]
    let next = c
    if (!it.first && !it.unit) {
      const s = blockStart(c)
      const e = blockEnd(c)
      const begins = items[s].first // the paragraph starts on this page
      const total = e - s + 1
      const head = c - s
      const tail = e - c + 1
      if (begins) {
        if (total <= 3) next = s > start ? s : c
        else if (head < 2) next = s > start ? s : c
        else if (tail < 2) next = c - 1 - s >= 2 ? c - 1 : s > start ? s : c
      } else if (tail < 2 && c - 1 > start) {
        next = c - 1
      }
    } else if (c > start && items[c - 1].keepNext) {
      // between blocks: a keepNext block does not end the page
      const s = blockStart(c - 1)
      if (s > start && items[s].first) next = s
    }
    if (next === c) break
    c = next
  }
  if (c <= start || items[c].hard) return raw
  return c
}

// The extra height of the gap that goes before a page's first item: the
// rest of the page, the foot, the space between sheets, the next head.
//   w: where the gap starts (px, as if no gaps); pageTop: where the page it
// ends began; contentHeight; extra: foot + space + head; keep: a space
// before kept at the top of the page (after a hard break).
export function gapHeight({ w, pageTop, contentHeight, extra, keep = 0 }) {
  const base = Math.max(pageTop + contentHeight, w)
  return base - w + extra + keep
}

// The top of each page (as if there were no gaps) for the breaks of
// paginate(): page 0 starts at `origin`, the others at their first item.
export function pageTops(items, breaks, origin = 0) {
  return [origin, ...breaks.map((b) => items[b.index].top - (b.hard ? items[b.index].spaceBefore || 0 : 0))]
}
