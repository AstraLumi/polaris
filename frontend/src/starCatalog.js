// The stars in the story's sky picture. The picture (an equirectangular
// image, twice as wide as it is tall) is the author's star map; this finds the
// small bright spots standing out from their surroundings and gives each a
// direction, a brightness (as a magnitude) and a colour, so a sky view can draw
// them as sharp points. What is left once they are taken out (the galaxy's
// band, nebulae) comes back as a "glow" picture at half the size.
// Pure and without the DOM: starCatalog.worker.js runs it off the page's
// thread, useStarCatalog.js asks for it. No dependencies:
// node frontend/tests/starCatalog.test.mjs
//
// The picture's format (the sky shader in three/skyDome.js and
// backend/cmd/skygen define it too): the top edge is above the north pole
// (+z), the bottom edge the south pole, the middle column above longitude 0
// (+x), and east is to the LEFT.
//
// How the stars are found:
//   1. Luminance, as one byte a pixel.
//   2. The local background: the median of every 32 x 32 block (a histogram
//      each, so the stars in it hardly matter), read back smoothly between
//      the blocks' centres. The galaxy's band is just a slowly changing
//      background.
//   3. Candidates: pixels standing more than `threshold` above the
//      background that are the highest of their 3 x 3 (flat tops count once).
//   4. Brightest first, each candidate is measured in a window that grows
//      until its edge is quiet: its light summed over the window (above the
//      background), its centroid (to a fraction of a pixel) and its colour. A
//      candidate whose window never quiets down is not a star but something
//      bigger; if it is a compact blob much brighter than its surroundings
//      (a moon painted in the picture) it is a disc (see findDisc), else
//      (the band, a nebula) it is left in the glow. Candidates inside the
//      window of a brighter star are the same star (or its halo) and dropped.
//   5. Magnitudes are relative: the star with the most light gets `brightest`
//      and each other one `brightest - 2.5 log10(light / that light)`. An
//      8-bit picture clips its brightest stars, so those read a little dim
//      and everything else a little faint next to them: the author's scale
//      is "the brightest star I drew is as bright as Sirius".
//
// The poles: the picture stretches a star sideways by 1 / cos(latitude) (a
// star near the pole is a long streak a row or two high). So the window is
// as many times wider as the rows are stretched (up to 1024 times, about the
// whole row), and every pixel's light counts in proportion to cos(latitude),
// the sky it really covers, so the streak adds up to the star it is.
// A star's peak does not fade with the stretch (the picture is a remap, not a
// blur), so the detection needs nothing special.
//
// Discs: the sky view draws the real moons at their true places, so a moon
// painted in the picture must not stay in the glow. Such a disc is found (not
// as a star: its window never quiets down), reported in `discs`, and filled
// in the glow like a star is, with the band's light across it.

export const CATALOGUE_VERSION = 2

const BLOCK = 32 // the background's blocks, in pixels
const MIN_HALF = 2 // a star's window is at least 5 x 5 pixels (before the stretch)
const MAX_HALF = 8 // and at most 17 x 17
const MAX_STRETCH = 1024
const MAX_CANDIDATES = 2000000
const DISC_REACH = 256 // a disc is looked for this far (pixels) round a bright peak
const DISC_MIN_RISE = 60 // and must rise this far above its surroundings (0-255)
const DISC_MIN_RADIUS = 6
const DISC_MAX_TRIES = 2000
const DISC_FILL = 2 // the glow's fill reaches this many radii (a moon has its own glow round it)

// The direction a pixel's centre looks at (x = 0 is the left column, y = 0
// the top row; fractions give the points between). x, y may be fractions.
export function skyDirection(x, y, width, height) {
  const lon = (0.5 - (x + 0.5) / width) * 2 * Math.PI
  const lat = (0.5 - (y + 0.5) / height) * Math.PI
  const c = Math.cos(lat)
  return [c * Math.cos(lon), c * Math.sin(lon), Math.sin(lat)]
}

// The other way: where a direction lands, [x, y], with the same pixel
// numbering (x wrapped into 0..width).
export function skyPixel(dir, width, height) {
  const len = Math.hypot(dir[0], dir[1], dir[2]) || 1
  const lon = Math.atan2(dir[1], dir[0])
  const lat = Math.asin(Math.max(-1, Math.min(1, dir[2] / len)))
  let x = (0.5 - lon / (2 * Math.PI)) * width - 0.5
  x = ((x % width) + width) % width
  return [x, (0.5 - lat / Math.PI) * height - 0.5]
}

// image: { width, height, data } with RGBA bytes, like ImageData.
// Options:
//   brightest  magnitude of the brightest star found (Sirius)
//   faintest   dimmer than this stays in the glow
//   maxStars   at most this many, the brightest
//   threshold  how far above the background (0-255 luminance) a peak must stand
// Answers { stars, discs, glow }: discs are the compact bright blobs bigger
// than any star (moons), each { dir, radius (radians), x, y } (the centre in
// pixels), which the glow no longer holds. Stars: stars brightest first, each { dir, mag, color:
// '#rrggbb', x, y } (x, y the centroid in pixels); glow { width, height, data }
// the picture at half the size, the stars replaced by their surroundings.
export function findStars(image, options = {}) {
  const { brightest = -1.46, faintest = 6.5, maxStars = 20000, threshold = 18 } = options
  const { width: W, height: H, data } = image
  const lum = luminance(data, W * H)
  const bg = new Background(lum, W, H)
  const rowCos = new Float64Array(H)
  for (let y = 0; y < H; y++) rowCos[y] = Math.max(Math.cos((0.5 - (y + 0.5) / H) * Math.PI), 1 / MAX_STRETCH)

  // Candidates: standing out and the highest of their 3 x 3.
  const candIdx = []
  const candPeak = []
  const bgRow = new Float32Array(W)
  for (let y = 0; y < H; y++) {
    bg.row(y, bgRow)
    for (let x = 0; x < W; x++) {
      const i = y * W + x
      const v = lum[i]
      if (v - bgRow[x] <= threshold) continue
      if (!isPeak(lum, W, H, x, y, i, v)) continue
      candIdx.push(i)
      candPeak.push(v - bgRow[x])
      if (candIdx.length >= MAX_CANDIDATES) break
    }
    if (candIdx.length >= MAX_CANDIDATES) break
  }
  const order = new Int32Array(candIdx.length)
  for (let k = 0; k < order.length; k++) order[k] = k
  order.sort((a, b) => candPeak[b] - candPeak[a] || candIdx[a] - candIdx[b])

  // Measure them, brightest peak first.
  const found = []
  const discs = []
  const tried = [] // peaks that were looked at as discs and were not
  const buckets = new Map() // rows in groups of 16 -> the stars found there
  for (const k of order) {
    const px = candIdx[k] % W
    const py = (candIdx[k] - px) / W
    if (claimed(buckets, found, px, py, W)) continue
    if (discs.some((d) => within(d, px, py, W, 1.1))) continue
    const star = measure(image, lum, bg, rowCos, px, py, candPeak[k], threshold)
    if (star === EXTENDED) {
      if (tried.length >= DISC_MAX_TRIES || tried.some((t) => Math.abs(t[1] - py) < 6 && wrapDist(t[0], px, W) < 6)) continue
      const disc = findDisc(lum, W, H, px, py)
      if (disc) discs.push(disc)
      else tried.push([px, py])
      continue
    }
    if (!star) continue
    found.push(star)
    const b = py >> 4
    if (!buckets.has(b)) buckets.set(b, [])
    buckets.get(b).push(found.length - 1)
  }

  // A star seen inside a disc's circle is the disc's surface, not a star.
  const real = found.filter((s) => !discs.some((d) => within(d, s.px, s.py, W, 1.1)))
  real.sort((a, b) => b.flux - a.flux || a.idx - b.idx)
  const top = real[0]?.flux || 1
  const stars = []
  const kept = []
  for (const s of real) {
    const mag = brightest - 2.5 * Math.log10(s.flux / top)
    if (mag > faintest || stars.length >= maxStars) break
    stars.push({ dir: skyDirection(s.x, s.y, W, H), mag, color: s.color, x: s.x, y: s.y })
    kept.push(s)
  }
  const out = discs.map((d) => ({ dir: skyDirection(d.x, d.y, W, H), radius: (d.r * Math.PI) / H, x: d.x, y: d.y }))
  return { stars, discs: out, glow: makeGlow(data, W, H, kept, discs) }
}

const EXTENDED = { extended: true }

// Whether (px, py) lies inside the disc's circle, grown by `grow`.
function within(d, px, py, W, grow) {
  return Math.hypot(wrapDist(d.x, px, W), py - d.y) <= d.r * grow
}

function wrapDist(a, b, W) {
  const d = Math.abs(a - b)
  return d > W / 2 ? W - d : d
}

// A disc round the bright peak (px, py), or null when the peak is not on one.
// The surroundings' level is the lower quarter of the pixels within reach
// (a disc is mostly smaller than that, and the median blocks are no use inside
// one). The disc is what is connected to the peak above half way up from there
// to the peak. It must not run into the edge of the reach (then it is the band,
// not a moon), must be bigger than a star, and its edge must be steep: well
// outside it the picture is back at the surroundings' level. The circle is the
// smallest one round the lit part (Ritter's), which is the whole moon at any
// phase (the terminator ends on the limb, diametrically opposite): a crescent
// leaves the unlit side to the circle.
function findDisc(lum, W, H, px, py) {
  const R = DISC_REACH
  const side = 2 * R + 1
  const hist = new Uint32Array(256)
  let n = 0
  for (let dy = -R; dy <= R; dy += 4) {
    const y = py + dy
    if (y < 0 || y >= H) continue
    for (let dx = -R; dx <= R; dx += 4) {
      hist[lum[y * W + ((px + dx + W * 4) % W)]]++
      n++
    }
  }
  let ref = 0
  for (let acc = 0; ref < 255; ref++) {
    acc += hist[ref]
    if (acc >= n / 4) break
  }
  const rise = lum[py * W + px] - ref
  if (rise < DISC_MIN_RISE) return null
  // First the lit part (above half way up), then, within a little way of its
  // circle, everything above a faint level: the dim side of a crescent.
  const level = ref + rise / 2
  let blob = floodBlob(lum, W, H, px, py, level, null)
  if (!blob) return null
  let circle = enclosing(blob.edge)
  // (Not below what the sky is like just outside the circle, or the band
  // round a moon would be taken for the moon.)
  const reach = 1.6 * circle.r + 5
  const outside = []
  for (let k = 0; k < 48; k++) {
    const y = py + Math.round(circle.y + reach * Math.sin((k * Math.PI) / 24)) - R
    if (y >= 0 && y < H) outside.push(lum[y * W + ((px + Math.round(circle.x + reach * Math.cos((k * Math.PI) / 24)) - R + W * 4) % W)])
  }
  outside.sort((u, v) => u - v)
  const low = Math.max(ref + Math.max(6, 0.1 * rise), (outside[Math.floor(outside.length * 0.9)] ?? 0) + 3)
  const grown = low < level ? floodBlob(lum, W, H, px, py, low, { x: circle.x, y: circle.y, r: reach }) : null
  if (grown) {
    blob = { edge: grown.edge, area: grown.area }
    circle = enclosing(blob.edge)
  }
  const area = blob.area
  const { x: cx, y: cy } = circle
  let r = circle.r
  r += 0.5
  if (r < DISC_MIN_RADIUS || area < 0.08 * Math.PI * r * r) return null
  // A steep edge: a ring well outside the disc is back near the surroundings.
  let ring = 0
  let count = 0
  const rr = DISC_FILL * r + 3
  for (let k = 0; k < 24; k++) {
    const x = Math.round(cx + rr * Math.cos((k * Math.PI) / 12))
    const y = py + Math.round(cy + rr * Math.sin((k * Math.PI) / 12)) - R
    if (y < 0 || y >= H) continue
    ring += lum[y * W + ((px + x - R + W * 4) % W)] - ref
    count++
  }
  if (count && ring / count > 0.3 * rise) return null
  let x = px + cx - R
  x = ((x % W) + W) % W
  return { x, y: py + cy - R, r }
}

// The pixels connected to the peak (the middle of the reach's square) that are
// at least `level`: their number and their edge, as flat x, y pairs in the
// square's own coordinates. Null when they run into the square's edge. With
// `limit` ({ x, y, r } in the square's coordinates) it does not go outside it.
function floodBlob(lum, W, H, px, py, level, limit) {
  const R = DISC_REACH
  const side = 2 * R + 1
  const seen = new Uint8Array(side * side)
  const queue = new Int32Array(side * side)
  let head = 0
  let tail = 0
  queue[tail++] = R * side + R
  seen[R * side + R] = 1
  const edge = []
  let area = 0
  while (head < tail) {
    const q = queue[head++]
    const qx = q % side
    const qy = (q - qx) / side
    area++
    if (qx === 0 || qy === 0 || qx === side - 1 || qy === side - 1) return null
    let isEdge = false
    for (let k = 0; k < 4; k++) {
      const nx = qx + (k === 0 ? 1 : k === 1 ? -1 : 0)
      const ny = qy + (k === 2 ? 1 : k === 3 ? -1 : 0)
      const y = py + ny - R
      const inside =
        y >= 0 && y < H && lum[y * W + ((px + nx - R + W * 4) % W)] >= level && (!limit || Math.hypot(nx - limit.x, ny - limit.y) <= limit.r)
      if (!inside) {
        isEdge = true
        continue
      }
      const ni = ny * side + nx
      if (!seen[ni]) {
        seen[ni] = 1
        queue[tail++] = ni
      }
    }
    if (isEdge) edge.push(qx, qy)
  }
  return { edge, area }
}

// The smallest circle round points (flat x, y pairs), near enough (Ritter:
// two sweeps after the first guess from the two furthest apart).
function enclosing(pts) {
  const far = (fx, fy) => {
    let best = -1
    let bi = 0
    for (let i = 0; i < pts.length; i += 2) {
      const d = Math.hypot(pts[i] - fx, pts[i + 1] - fy)
      if (d > best) {
        best = d
        bi = i
      }
    }
    return bi
  }
  const a = far(pts[0], pts[1])
  const b = far(pts[a], pts[a + 1])
  let x = (pts[a] + pts[b]) / 2
  let y = (pts[a + 1] + pts[b + 1]) / 2
  let r = Math.hypot(pts[a] - pts[b], pts[a + 1] - pts[b + 1]) / 2
  for (let sweep = 0; sweep < 2; sweep++) {
    for (let i = 0; i < pts.length; i += 2) {
      const d = Math.hypot(pts[i] - x, pts[i + 1] - y)
      if (d > r) {
        const nr = (r + d) / 2
        x += ((pts[i] - x) * (d - nr)) / d
        y += ((pts[i + 1] - y) * (d - nr)) / d
        r = nr
      }
    }
  }
  return { x, y, r }
}

// One byte of luminance (Rec. 709 weights) for every pixel.
function luminance(data, n) {
  const out = new Uint8Array(n)
  for (let i = 0, j = 0; i < n; i++, j += 4) out[i] = (54 * data[j] + 183 * data[j + 1] + 19 * data[j + 2]) >> 8
  return out
}

// Whether the pixel is the highest of its 3 x 3. Where neighbours are equal
// (a flat top) only the first in reading order counts, so one peak a plateau.
function isPeak(lum, W, H, x, y, i, v) {
  for (let dy = -1; dy <= 1; dy++) {
    const yy = y + dy
    if (yy < 0 || yy >= H) continue
    for (let dx = -1; dx <= 1; dx++) {
      if (dx === 0 && dy === 0) continue
      const ni = yy * W + ((x + dx + W) % W)
      if (ni < i ? lum[ni] >= v : lum[ni] > v) return false
    }
  }
  return true
}

// The sky's background level: the median of each block, read smoothly.
class Background {
  constructor(lum, W, H) {
    this.W = W
    this.H = H
    this.bw = Math.ceil(W / BLOCK)
    this.bh = Math.ceil(H / BLOCK)
    this.grid = new Float32Array(this.bw * this.bh)
    const hist = new Uint32Array(256)
    for (let by = 0; by < this.bh; by++) {
      const y0 = by * BLOCK
      const y1 = Math.min(H, y0 + BLOCK)
      for (let bx = 0; bx < this.bw; bx++) {
        const x0 = bx * BLOCK
        const x1 = Math.min(W, x0 + BLOCK)
        hist.fill(0)
        for (let y = y0; y < y1; y++) for (let i = y * W + x0, e = y * W + x1; i < e; i++) hist[lum[i]]++
        this.grid[by * this.bw + bx] = histMedian(hist, (x1 - x0) * (y1 - y0))
      }
    }
    // Where each column sits between two block centres.
    this.xb0 = new Int32Array(W)
    this.xb1 = new Int32Array(W)
    this.xt = new Float32Array(W)
    for (let x = 0; x < W; x++) {
      const u = (x + 0.5) / BLOCK - 0.5
      const b = Math.floor(u)
      this.xb0[x] = (b + this.bw) % this.bw
      this.xb1[x] = (b + 1 + this.bw) % this.bw
      this.xt[x] = u - b
    }
    this.mix = new Float32Array(this.bw)
  }

  // The background along row y into out[0..W).
  row(y, out) {
    const { grid, bw, bh, mix, xb0, xb1, xt } = this
    const v = (y + 0.5) / BLOCK - 0.5
    const b = Math.floor(v)
    const t = v - b
    const r0 = Math.min(bh - 1, Math.max(0, b)) * bw
    const r1 = Math.min(bh - 1, Math.max(0, b + 1)) * bw
    for (let k = 0; k < bw; k++) mix[k] = grid[r0 + k] * (1 - t) + grid[r1 + k] * t
    for (let x = 0; x < this.W; x++) out[x] = mix[xb0[x]] * (1 - xt[x]) + mix[xb1[x]] * xt[x]
  }

  // The background at one pixel (x may lie outside 0..W: the picture wraps).
  at(x, y) {
    const { grid, bw, bh } = this
    x = ((x % this.W) + this.W) % this.W
    const u = (x + 0.5) / BLOCK - 0.5
    const v = (y + 0.5) / BLOCK - 0.5
    const bx = Math.floor(u)
    const by = Math.floor(v)
    const tx = u - bx
    const ty = v - by
    const x0 = (bx + bw) % bw
    const x1 = (bx + 1 + bw) % bw
    const y0 = Math.min(bh - 1, Math.max(0, by)) * bw
    const y1 = Math.min(bh - 1, Math.max(0, by + 1)) * bw
    return (grid[y0 + x0] * (1 - tx) + grid[y0 + x1] * tx) * (1 - ty) + (grid[y1 + x0] * (1 - tx) + grid[y1 + x1] * tx) * ty
  }
}

function histMedian(hist, n) {
  const half = n / 2
  let sum = 0
  for (let v = 0; v < 256; v++) {
    if (sum + hist[v] >= half) return v - 0.5 + (hist[v] ? (half - sum) / hist[v] : 0.5)
    sum += hist[v]
  }
  return 255
}

// Whether the candidate lies inside the window of a star already found:
// then it is that star's halo or its twin.
function claimed(buckets, found, px, py, W) {
  for (let b = (py >> 4) - 1; b <= (py >> 4) + 1; b++) {
    const list = buckets.get(b)
    if (!list) continue
    for (const j of list) {
      const s = found[j]
      if (Math.abs(py - s.py) > s.h) continue
      let dx = Math.abs(px - s.px)
      if (dx > W / 2) dx = W - dx
      if (dx <= s.hx) return true
    }
  }
  return false
}

// How much of the sideways stretch a window follows on a row.
const stretchOf = (rowCos, y) => 1 / rowCos[y]

// The star at the peak (px, py), or null when it is not one. The window
// grows (MIN_HALF up to MAX_HALF rows each way, stretched sideways) until its
// edge is quiet; one that stays loud is something bigger than a star.
function measure(image, lum, bg, rowCos, px, py, peak, threshold) {
  const { width: W, height: H } = image
  const sx = Math.min(stretchOf(rowCos, py), MAX_STRETCH)
  const maxHx = Math.floor(W / 2) - 1
  const quiet = Math.max(0.5 * threshold, 0.05 * peak)
  let h = MIN_HALF
  let hx = Math.min(maxHx, Math.ceil(h * sx))
  let edge = edgeLevel(lum, bg, W, H, px, py, hx, h)
  while (edge > quiet && h < MAX_HALF) {
    h++
    hx = Math.min(maxHx, Math.ceil(h * sx))
    edge = edgeLevel(lum, bg, W, H, px, py, hx, h)
  }
  if (edge > quiet && edge > 0.5 * peak) return EXTENDED

  // The light and the centroid over the window.
  let flux = 0
  let sw = 0
  let sxw = 0
  let syw = 0
  const y0 = Math.max(0, py - h)
  const y1 = Math.min(H - 1, py + h)
  for (let y = y0; y <= y1; y++) {
    for (let dx = -hx; dx <= hx; dx++) {
      const e = lum[y * W + ((px + dx + W) % W)] - bg.at(px + dx, y)
      flux += e * rowCos[y]
      if (e > 0) {
        sw += e
        sxw += e * dx
        syw += e * (y - py)
      }
    }
  }
  if (!(flux > 0) || !(sw > 0)) return null
  let x = px + sxw / sw
  x = ((x % W) + W) % W
  const y = py + syw / sw
  return { idx: py * W + px, px, py, h, hx, x, y, flux, color: colourOf(image, lum, bg, px, py, hx, h) }
}

// The strongest level above the background along the edge of the window.
function edgeLevel(lum, bg, W, H, px, py, hx, h) {
  let worst = 0
  const top = py - h
  const bottom = py + h
  for (let dx = -hx; dx <= hx; dx++) {
    if (top >= 0) worst = Math.max(worst, lum[top * W + ((px + dx + W) % W)] - bg.at(px + dx, top))
    if (bottom < H) worst = Math.max(worst, lum[bottom * W + ((px + dx + W) % W)] - bg.at(px + dx, bottom))
  }
  for (let y = Math.max(0, top + 1); y <= Math.min(H - 1, bottom - 1); y++) {
    worst = Math.max(worst, lum[y * W + ((px - hx + W) % W)] - bg.at(px - hx, y))
    worst = Math.max(worst, lum[y * W + ((px + hx + W) % W)] - bg.at(px + hx, y))
  }
  return worst
}

// The star's colour: its 3 x 3 brightest pixels, less the colour round the
// window, weighted by how far each stands out; scaled to full strength so the
// colour tells the hue only (brightness is the magnitude's business). A
// star that is clipped white in the picture comes out white.
function colourOf(image, lum, bg, px, py, hx, h) {
  const { width: W, height: H, data } = image
  const ring = [0, 0, 0]
  let n = 0
  const add = (x, y) => {
    if (y < 0 || y >= H) return
    const j = (y * W + ((x + W) % W)) * 4
    ring[0] += data[j]
    ring[1] += data[j + 1]
    ring[2] += data[j + 2]
    n++
  }
  for (let dx = -hx; dx <= hx; dx++) {
    add(px + dx, py - h)
    add(px + dx, py + h)
  }
  for (let dy = -h + 1; dy < h; dy++) {
    add(px - hx, py + dy)
    add(px + hx, py + dy)
  }
  if (n) for (let c = 0; c < 3; c++) ring[c] /= n
  const sum = [0, 0, 0]
  let sw = 0
  for (let dy = -1; dy <= 1; dy++) {
    const y = py + dy
    if (y < 0 || y >= H) continue
    for (let dx = -1; dx <= 1; dx++) {
      const x = (px + dx + W) % W
      const w = lum[y * W + x] - bg.at(x, y)
      if (!(w > 0)) continue
      const j = (y * W + x) * 4
      for (let c = 0; c < 3; c++) sum[c] += w * Math.max(0, data[j + c] - ring[c])
      sw += w
    }
  }
  const top = Math.max(sum[0], sum[1], sum[2])
  if (!(sw > 0) || top < 1e-6 * sw) return '#ffffff'
  const hex = (v) => Math.round((255 * v) / top).toString(16).padStart(2, '0')
  return '#' + hex(sum[0]) + hex(sum[1]) + hex(sum[2])
}

// The picture at half the size with the stars' windows filled in from their
// surroundings. Every pixel under a window is blended from the nearest
// pixels outside any window (or disc) along its row and along its column (straight
// lines between them, so a star on the band's slope leaves the slope, and
// two stars close together do not feed each other).
function makeGlow(data, W, H, found, discs) {
  const gw = Math.floor(W / 2)
  const gh = Math.floor(H / 2)
  const out = new Uint8ClampedArray(gw * gh * 4)
  for (let y = 0; y < gh; y++) {
    for (let x = 0; x < gw; x++) {
      const a = (2 * y * W + 2 * x) * 4
      const b = a + 4
      const c = a + W * 4
      const d = c + 4
      const o = (y * gw + x) * 4
      for (let k = 0; k < 3; k++) out[o + k] = (data[a + k] + data[b + k] + data[c + k] + data[d + k] + 2) >> 2
      out[o + 3] = 255
    }
  }
  if (!found.length && !discs.length) return { width: gw, height: gh, data: out }

  // 1 under a window, 2 once the row has filled it in.
  const mask = new Uint8Array(gw * gh)
  for (const s of found) {
    const gx0 = Math.floor((s.px - s.hx - 1) / 2)
    const gx1 = Math.min(gx0 + gw - 3, Math.floor((s.px + s.hx + 1) / 2))
    const gy0 = Math.max(0, Math.floor((s.py - s.h - 1) / 2))
    const gy1 = Math.min(gh - 1, Math.floor((s.py + s.h + 1) / 2))
    for (let y = gy0; y <= gy1; y++) for (let x = gx0; x <= gx1; x++) mask[y * gw + (((x % gw) + gw) % gw)] = 1
  }

  // A disc's circle, with its own glow round it.
  for (const d of discs) {
    const reach = (DISC_FILL * d.r + 4) / 2
    const gcx = d.x / 2
    const gcy = d.y / 2
    for (let y = Math.max(0, Math.floor(gcy - reach)); y <= Math.min(gh - 1, Math.ceil(gcy + reach)); y++) {
      const half = Math.sqrt(Math.max(0, reach * reach - (y - gcy) * (y - gcy)))
      const x1 = Math.min(Math.ceil(gcx + half), Math.floor(gcx - half) + gw - 3)
      for (let x = Math.floor(gcx - half); x <= x1; x++) mask[y * gw + (((x % gw) + gw) % gw)] = 1
    }
  }

  // Along each row (it wraps round the picture).
  for (let y = 0; y < gh; y++) {
    const row = y * gw
    let start = -1
    for (let x = 0; x < gw; x++) if (!mask[row + x]) { start = x; break }
    if (start < 0) continue
    let runStart = -1
    for (let n = 1; n <= gw; n++) {
      const x = (start + n) % gw
      if (mask[row + x] === 1 && n < gw) {
        if (runStart < 0) runStart = n
        continue
      }
      if (runStart >= 0) {
        // The run is runStart..n-1 steps from `start`, between two clear pixels.
        const left = row + ((start + runStart - 1) % gw)
        const right = row + x
        const len = n - runStart + 1
        for (let m = runStart; m < n; m++) {
          const t = (m - runStart + 1) / len
          const at = row + ((start + m) % gw)
          for (let k = 0; k < 3; k++) out[at * 4 + k] = (1 - t) * out[left * 4 + k] + t * out[right * 4 + k]
          mask[at] = 2
        }
        runStart = -1
      }
    }
  }

  // Along each column (it does not wrap), averaged with the row's answer.
  for (let x = 0; x < gw; x++) {
    let y = 0
    while (y < gh) {
      if (!mask[y * gw + x]) { y++; continue }
      let end = y
      while (end < gh && mask[end * gw + x]) end++
      const above = y > 0 ? y - 1 : -1
      const below = end < gh ? end : -1
      if (above >= 0 || below >= 0) {
        for (let m = y; m < end; m++) {
          const at = m * gw + x
          const had = mask[at] === 2
          for (let k = 0; k < 3; k++) {
            let v
            if (above >= 0 && below >= 0) {
              const t = (m - above) / (below - above)
              v = (1 - t) * out[(above * gw + x) * 4 + k] + t * out[(below * gw + x) * 4 + k]
            } else {
              v = out[((above >= 0 ? above : below) * gw + x) * 4 + k]
            }
            out[at * 4 + k] = had ? (out[at * 4 + k] + v) / 2 : v
          }
        }
      }
      y = end
    }
  }
  return { width: gw, height: gh, data: out }
}
