// The geometry of a drafts board (components/drafts/DraftBoard.vue): cards
// as rectangles in board units (pixels at zoom 1), shown small (a tile:
// the title over the category) until opened at the size the writer gave
// them. Pure: node frontend/tests/draftBoard.test.mjs

// A closed card's tile.
export const TILE = { w: 180, h: 64 }
// An open card's size limits (the server keeps the same, drafts.go).
export const MIN_W = 160
export const MAX_W = 1600
export const MIN_H = 80
export const MAX_H = 1600

const clamp = (v, lo, hi) => Math.max(lo, Math.min(hi, v))

// The rectangle a card takes now: its tile, or its own size when open.
export function cardRect(card, open = false) {
  return open ? { x: card.x, y: card.y, w: card.w, h: card.h } : { x: card.x, y: card.y, w: TILE.w, h: TILE.h }
}

const centre = (r) => ({ x: r.x + r.w / 2, y: r.y + r.h / 2 })

// Where the line from the rectangle's centre towards (px, py) leaves it.
function edgePoint(r, px, py) {
  const c = centre(r)
  const dx = px - c.x
  const dy = py - c.y
  if (!dx && !dy) return c
  const sx = dx ? r.w / 2 / Math.abs(dx) : Infinity
  const sy = dy ? r.h / 2 / Math.abs(dy) : Infinity
  const s = Math.min(sx, sy)
  return { x: c.x + dx * s, y: c.y + dy * s }
}

// A line between two cards: from edge to edge along the line joining
// their centres, and its middle (for the label). null when they overlap
// so much that there is nothing to draw.
export function linkLine(ra, rb) {
  const ca = centre(ra)
  const cb = centre(rb)
  const a = edgePoint(ra, cb.x, cb.y)
  const b = edgePoint(rb, ca.x, ca.y)
  if ((b.x - a.x) * (cb.x - ca.x) + (b.y - a.y) * (cb.y - ca.y) <= 0) return null
  return { x1: a.x, y1: a.y, x2: b.x, y2: b.y, mx: (a.x + b.x) / 2, my: (a.y + b.y) / 2 }
}

// Resizing an open card by dragging its `edge` ('n', 'se', 'w'…) by
// (dx, dy): the far side stays put, the size stays within the limits.
export function resizeRect(r, edge, dx, dy) {
  let { x, y, w, h } = r
  if (edge.includes('e')) w = clamp(r.w + dx, MIN_W, MAX_W)
  if (edge.includes('s')) h = clamp(r.h + dy, MIN_H, MAX_H)
  if (edge.includes('w')) {
    w = clamp(r.w - dx, MIN_W, MAX_W)
    x = r.x + r.w - w
  }
  if (edge.includes('n')) {
    h = clamp(r.h - dy, MIN_H, MAX_H)
    y = r.y + r.h - h
  }
  return { x: Math.round(x), y: Math.round(y), w: Math.round(w), h: Math.round(h) }
}

// A free spot for a new tile near (x, y): moved down, then right, until it
// covers no other tile.
export function freeSpot(cards, x, y) {
  const hits = (px, py) =>
    cards.some((c) => px < c.x + TILE.w + 12 && c.x < px + TILE.w + 12 && py < c.y + TILE.h + 12 && c.y < py + TILE.h + 12)
  const x0 = Math.round(x - TILE.w / 2)
  const y0 = Math.round(y - TILE.h / 2)
  for (let col = 0; col < 40; col++) {
    for (let row = 0; row < 8; row++) {
      const px = x0 + col * (TILE.w + 24)
      const py = y0 + row * (TILE.h + 20)
      if (!hits(px, py)) return { x: px, y: py }
    }
  }
  return { x: x0, y: y0 }
}

// The box round every tile (and open card), for "show everything".
export function boardBounds(rects) {
  if (!rects.length) return null
  let minX = Infinity
  let minY = Infinity
  let maxX = -Infinity
  let maxY = -Infinity
  for (const r of rects) {
    minX = Math.min(minX, r.x)
    minY = Math.min(minY, r.y)
    maxX = Math.max(maxX, r.x + r.w)
    maxY = Math.max(maxY, r.y + r.h)
  }
  return { minX, minY, maxX, maxY }
}

// The view (pan and zoom) that shows the box in a w × h viewport.
export function fitView(box, w, h, { pad = 40, maxZoom = 1.25, minZoom = 0.2 } = {}) {
  if (!box) return { x: w / 2, y: h / 2, k: 1 }
  const bw = Math.max(1, box.maxX - box.minX)
  const bh = Math.max(1, box.maxY - box.minY)
  const k = clamp(Math.min((w - 2 * pad) / bw, (h - 2 * pad) / bh), minZoom, maxZoom)
  return { x: w / 2 - ((box.minX + box.maxX) / 2) * k, y: h / 2 - ((box.minY + box.maxY) / 2) * k, k }
}

// Black or white text, whichever reads better on the colour (#rrggbb).
export function inkOn(hex) {
  const m = /^#?([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})$/i.exec(hex || '')
  if (!m) return ''
  const [r, g, b] = m.slice(1).map((v) => parseInt(v, 16) / 255)
  const lin = (c) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4)
  const lum = 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b)
  return lum > 0.4 ? '#14161c' : '#ffffff'
}

// Colours offered for a new category, in turn.
export const CATEGORY_COLORS = ['#d0605a', '#e39b3c', '#d8c04a', '#5fae6e', '#4aa6b8', '#5b7fd6', '#9a6ad0', '#d06aa5', '#8a8f9c']
