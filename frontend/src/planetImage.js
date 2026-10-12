// Drawing the planet flat, in a projection (projection.js): the sea inside
// the map's outline, coloured cells, and lines of latitude. Used by the flat
// view of the planet and by the bestiary's territory pictures.

import { latLon } from './planetGrid'
import { projectionOf, outline } from './projection'

// A cell's outline in projected coordinates: one ring of [x, y], or two when
// it straddles the edge of the map (drawn once on each side).
export function projectCell(grid, proj, cell) {
  const p = projectionOf(proj)
  if (cell < 2) {
    // A pole: a band along the top or bottom of the map.
    const [lat] = latLon(grid.corners(cell)[0])
    const edge = cell === 0 ? Math.PI / 2 : -Math.PI / 2
    const ring = []
    for (let k = 0; k <= 36; k++) ring.push(p.forward(-Math.PI + (k / 36) * 2 * Math.PI, lat))
    for (let k = 36; k >= 0; k--) ring.push(p.forward(-Math.PI + (k / 36) * 2 * Math.PI, edge))
    return [ring]
  }
  const [, lonC] = latLon(grid.center(cell))
  let crosses = false
  const corners = grid.corners(cell).map((v) => {
    let [lat, lon] = latLon(v)
    while (lon - lonC > Math.PI) lon -= 2 * Math.PI
    while (lon - lonC < -Math.PI) lon += 2 * Math.PI
    if (Math.abs(lon) > Math.PI) crosses = true
    return [lon, lat]
  })
  const rings = [corners.map(([lon, lat]) => p.forward(lon, lat))]
  if (crosses) {
    const shift = lonC > 0 ? -2 * Math.PI : 2 * Math.PI
    rings.push(corners.map(([lon, lat]) => p.forward(lon + shift, lat)))
  }
  return rings
}

// A cache of projected outlines, per grid and projection.
export class ProjectedCells {
  constructor(grid, proj) {
    this.grid = grid
    this.proj = proj
    this.cache = new Map()
  }

  rings(cell) {
    let r = this.cache.get(cell)
    if (!r) {
      r = projectCell(this.grid, this.proj, cell)
      this.cache.set(cell, r)
    }
    return r
  }

  // The outline of a set of cells in projected coordinates: [[x, y], [x, y]]
  // segments, each cell's edge k−1→k being the one it shares with
  // neighbour k (planetGrid.js corners). Poles are left out.
  edges(cells) {
    // Cached by the list itself: the views redraw every frame while panning.
    this.edgeCache ||= new WeakMap()
    const hit = this.edgeCache.get(cells)
    if (hit) return hit
    const set = cells instanceof Set ? cells : new Set(cells)
    const out = []
    for (const c of set) {
      if (c < 2) continue
      const around = this.grid.neighbors(c)
      const m = around.length
      for (const ring of this.rings(c)) {
        for (let k = 0; k < m; k++) if (!set.has(around[k])) out.push([ring[(k - 1 + m) % m], ring[k]])
      }
    }
    this.edgeCache.set(cells, out)
    return out
  }

  // Where a cell's centre lands.
  point(cell) {
    const [lat, lon] = latLon(this.grid.center(cell))
    return projectionOf(this.proj).forward(lon, lat)
  }
}

const LINE_COLORS = { equator: 'rgba(232, 86, 70, 0.95)', tropic: 'rgba(236, 168, 64, 0.9)', polar: 'rgba(120, 180, 236, 0.9)' }

// Draws the planet into ctx. view: { scale, ox, oy } turns projected
// coordinates into canvas pixels (y up). cells: Map colour -> [cell].
export function drawPlanet(ctx, projected, view, { cells, sea = '#1b2d4a', land = null, lines = null, border = 'rgba(0, 0, 0, 0.35)' }) {
  const { scale, ox, oy } = view
  const sx = (x) => ox + x * scale
  const sy = (y) => oy - y * scale
  const edge = outline(projected.proj)

  ctx.save()
  ctx.beginPath()
  edge.forEach(([x, y], k) => (k ? ctx.lineTo(sx(x), sy(y)) : ctx.moveTo(sx(x), sy(y))))
  ctx.closePath()
  ctx.fillStyle = sea
  ctx.fill()
  ctx.clip()

  for (const [color, list] of cells) {
    ctx.beginPath()
    for (const c of list) {
      for (const ring of projected.rings(c)) {
        ring.forEach(([x, y], k) => (k ? ctx.lineTo(sx(x), sy(y)) : ctx.moveTo(sx(x), sy(y))))
        ctx.closePath()
      }
    }
    ctx.fillStyle = color
    ctx.fill()
    if (border && scale * (Math.atan(2) / projected.grid.n) > 5) {
      ctx.strokeStyle = border
      ctx.lineWidth = 0.6
      ctx.stroke()
    }
  }
  if (land) land(ctx, sx, sy)

  if (lines) {
    const p = projectionOf(projected.proj)
    const circle = (latDeg, color) => {
      const lat = (latDeg * Math.PI) / 180
      ctx.beginPath()
      for (let k = 0; k <= 120; k++) {
        const [x, y] = p.forward(-Math.PI + (k / 120) * 2 * Math.PI, lat)
        k ? ctx.lineTo(sx(x), sy(y)) : ctx.moveTo(sx(x), sy(y))
      }
      ctx.strokeStyle = color
      ctx.lineWidth = 1.5
      ctx.setLineDash(latDeg === 0 ? [10, 6] : [3, 5])
      ctx.stroke()
      ctx.setLineDash([])
    }
    const { equator, zones, tilt } = lines
    if (zones && tilt > 0 && tilt < 90) {
      for (const lat of [tilt, -tilt]) circle(lat, LINE_COLORS.tropic)
      if (90 - tilt !== tilt) for (const lat of [90 - tilt, tilt - 90]) circle(lat, LINE_COLORS.polar)
    }
    if (equator) circle(0, LINE_COLORS.equator)
  }
  ctx.restore()

  // The map's edge.
  ctx.beginPath()
  edge.forEach(([x, y], k) => (k ? ctx.lineTo(sx(x), sy(y)) : ctx.moveTo(sx(x), sy(y))))
  ctx.closePath()
  ctx.strokeStyle = 'rgba(255, 255, 255, 0.25)'
  ctx.lineWidth = 1
  ctx.stroke()
}

// The projected bounds of the whole map: [x0, y0, x1, y1].
export function mapBounds(proj) {
  const pts = outline(proj)
  const xs = pts.map((p) => p[0])
  const ys = pts.map((p) => p[1])
  return [Math.min(...xs), Math.min(...ys), Math.max(...xs), Math.max(...ys)]
}

// Groups cells by colour for drawPlanet: colorOf(cell) -> colour or ''.
export function byColor(cells, colorOf) {
  const out = new Map()
  for (const c of cells) {
    const col = colorOf(c)
    if (!col) continue
    if (!out.has(col)) out.set(col, [])
    out.get(col).push(c)
  }
  return out
}
