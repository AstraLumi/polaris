// The planet's hex grid, the same as backend/planetgrid.go (read that for
// how it is laid out): an icosahedron cut into a triangular lattice of
// frequency n on the unit sphere, 10n²+2 cells, twelve of them pentagons.
// No dependencies: node frontend/tests/planetGrid.test.mjs

const add = (a, b) => [a[0] + b[0], a[1] + b[1], a[2] + b[2]]
const sub = (a, b) => [a[0] - b[0], a[1] - b[1], a[2] - b[2]]
const scale = (a, k) => [a[0] * k, a[1] * k, a[2] * k]
const dot = (a, b) => a[0] * b[0] + a[1] * b[1] + a[2] * b[2]
const cross = (a, b) => [a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0]]
export const normalize = (a) => scale(a, 1 / Math.sqrt(dot(a, a)))

export const fromLatLon = (lat, lon) => [Math.cos(lat) * Math.cos(lon), Math.cos(lat) * Math.sin(lon), Math.sin(lat)]
// [lat, lon] in radians.
export const latLon = (v) => [Math.asin(Math.max(-1, Math.min(1, v[2]))), Math.atan2(v[1], v[0])]

const NORTH = [0, 0, 1]
const SOUTH = [0, 0, -1]
const RING = Math.atan(0.5)
const UPPER = Array.from({ length: 5 }, (_, k) => fromLatLon(RING, (k * 2 * Math.PI) / 5))
const LOWER = Array.from({ length: 5 }, (_, k) => fromLatLon(-RING, (k * 2 * Math.PI) / 5 + Math.PI / 5))
// DIAMONDS[d] = [T, A, B, C]
const DIAMONDS = [
  ...Array.from({ length: 5 }, (_, k) => [NORTH, UPPER[k], LOWER[k], UPPER[(k + 1) % 5]]),
  ...Array.from({ length: 5 }, (_, k) => [UPPER[(k + 1) % 5], LOWER[k], SOUTH, LOWER[(k + 1) % 5]]),
]
const FACES = DIAMONDS.flatMap(([T, A, B, C], d) => [
  { d, upper: true, centre: normalize(add(add(T, A), C)) },
  { d, upper: false, centre: normalize(add(add(A, B), C)) },
])

function solve2(e1, e2, w) {
  const a11 = dot(e1, e1)
  const a12 = dot(e1, e2)
  const a22 = dot(e2, e2)
  const b1 = dot(w, e1)
  const b2 = dot(w, e2)
  const det = a11 * a22 - a12 * a12
  return [(b1 * a22 - b2 * a12) / det, (a11 * b2 - a12 * b1) / det]
}

// Rounds fractional axial coordinates to the nearest lattice point.
// Math.round(-0.5) is -0 in JS but -1 in Go, so round halves away from zero.
const round = (x) => Math.sign(x) * Math.round(Math.abs(x))
export function axialRound(a, b) {
  const x = a
  const z = b
  const y = -x - z
  let rx = round(x)
  const ry = round(y)
  let rz = round(z)
  const dx = Math.abs(rx - x)
  const dy = Math.abs(ry - y)
  const dz = Math.abs(rz - z)
  if (dx > dy && dx > dz) rx = -ry - rz
  else if (dy <= dz) rz = -rx - ry
  return [rx + 0, rz + 0]
}

const OFFSETS = [[1, 0], [1, -1], [0, -1], [-1, 0], [-1, 1], [0, 1]]

export class PlanetGrid {
  constructor(n) {
    this.n = n
    this.count = 10 * n * n + 2
  }

  // The (unnormalised) lattice point (i, j) of diamond d; past the diamond
  // it extends the face it falls in.
  lattice(d, i, j) {
    const [T, A, B, C] = DIAMONDS[d]
    const s = i / this.n
    const t = j / this.n
    if (s + t <= 1) return add(add(T, scale(sub(A, T), s)), scale(sub(C, T), t))
    return add(add(B, scale(sub(C, B), 1 - s)), scale(sub(A, B), 1 - t))
  }

  owner(d, i, j) {
    const n = this.n
    for (let step = 0; step < 4; step++) {
      const k = d % 5
      const north = d < 5
      if (i === 0 && j === 0) {
        if (north) return 0
        ;[d, i, j] = [(k + 1) % 5, n, 0]
      } else if (i === 0) {
        if (north) [d, i, j] = [(k + 1) % 5, j, 0]
        else [d, i] = [(k + 1) % 5, n]
      } else if (j === n) {
        if (north) [d, j] = [5 + k, 0]
        else if (i === n) return 1
        else [d, i, j] = [5 + ((k + 1) % 5), n, i]
      } else break
    }
    return 2 + d * n * n + (i - 1) * n + j
  }

  coords(cell) {
    if (cell < 2) return [-1, 0, 0]
    const nn = this.n * this.n
    const idx = cell - 2
    const rem = idx % nn
    return [Math.floor(idx / nn), Math.floor(rem / this.n) + 1, rem % this.n]
  }

  center(cell) {
    if (cell === 0) return NORTH
    if (cell === 1) return SOUTH
    const [d, i, j] = this.coords(cell)
    return normalize(this.lattice(d, i, j))
  }

  cellAt(v) {
    let face = FACES[0]
    let best = -Infinity
    for (const f of FACES) {
      const dt = dot(f.centre, v)
      if (dt > best) {
        best = dt
        face = f
      }
    }
    const [T, A, B, C] = DIAMONDS[face.d]
    const origin = face.upper ? T : B
    const p = scale(v, dot(face.centre, origin) / dot(face.centre, v))
    let s
    let t
    if (face.upper) [s, t] = solve2(sub(A, T), sub(C, T), sub(p, T))
    else {
      const [u, w] = solve2(sub(C, B), sub(A, B), sub(p, B))
      s = 1 - u
      t = 1 - w
    }
    const [i, j] = axialRound(s * this.n, t * this.n)
    const clamp = (x) => Math.max(0, Math.min(this.n, x))
    return this.owner(face.d, clamp(i), clamp(j))
  }

  // The cells round a cell, in order round it.
  neighbors(cell) {
    const n = this.n
    const out = []
    const addCell = (c) => {
      if (c !== cell && !out.includes(c)) out.push(c)
    }
    if (cell === 0) for (let d = 0; d < 5; d++) addCell(this.owner(d, 1, 0))
    else if (cell === 1) for (let d = 5; d < 10; d++) addCell(this.owner(d, n, n - 1))
    else {
      const [d, i, j] = this.coords(cell)
      for (const [oi, oj] of OFFSETS) {
        const ci = i + oi
        const cj = j + oj
        if (ci >= 0 && cj >= 0 && ci <= n && cj <= n) addCell(this.owner(d, ci, cj))
        else addCell(this.cellAt(normalize(this.lattice(d, ci, cj))))
      }
    }
    const c = this.center(cell)
    let e1 = cross([0, 0, 1], c)
    if (dot(e1, e1) < 1e-12) e1 = [1, 0, 0]
    e1 = normalize(e1)
    const e2 = cross(c, e1)
    const angle = (x) => {
      const v = sub(this.center(x), c)
      return Math.atan2(dot(v, e2), dot(v, e1))
    }
    return out.map((x) => [x, angle(x)]).sort((a, b) => a[1] - b[1]).map(([x]) => x)
  }

  // The corners of a cell's outline, on the unit sphere, in order round it:
  // where it meets each pair of neighbours.
  corners(cell, around = this.neighbors(cell)) {
    const c = this.center(cell)
    return around.map((x, k) => normalize(add(add(c, this.center(x)), this.center(around[(k + 1) % around.length]))))
  }
}

// The cells within `radius` steps of a cell.
export function cellsWithin(grid, cell, radius) {
  const seen = new Set([cell])
  let ring = [cell]
  for (let r = 0; r < radius; r++) {
    const next = []
    for (const c of ring) for (const x of grid.neighbors(c)) if (!seen.has(x)) {
      seen.add(x)
      next.push(x)
    }
    ring = next
  }
  return [...seen]
}

export const MIN_FREQUENCY = 2
export const MAX_FREQUENCY = 256

// The frequency whose cells are about hexKm across on a planet planetKm
// round (backend frequencyFor).
export function frequencyFor(planetKm, hexKm) {
  const n = Math.round(((planetKm / (2 * Math.PI)) * Math.atan(2)) / hexKm)
  return Math.max(MIN_FREQUENCY, Math.min(MAX_FREQUENCY, n))
}

export const hexKmOf = (planetKm, n) => ((planetKm / (2 * Math.PI)) * Math.atan(2)) / n
