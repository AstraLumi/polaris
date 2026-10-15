// Where an ocean's name lies on the globe (PlanetGlobe.vue): written on the
// water itself, bending with the planet, at the spot nearest the ocean's
// middle where the whole name fits on its hexes.
// No dependencies: node frontend/tests/oceanNames.test.mjs

const dot = (a, b) => a[0] * b[0] + a[1] * b[1] + a[2] * b[2]
const cross = (a, b) => [a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0]]
const scale = (a, k) => [a[0] * k, a[1] * k, a[2] * k]
const add = (a, b) => [a[0] + b[0], a[1] + b[1], a[2] + b[2]]
const unit = (a) => scale(a, 1 / Math.hypot(a[0], a[1], a[2]))

// How far from the line of latitude a name may tilt, tried in this order.
const TILTS = [0, 15, -15, 30, -30].map((d) => (d * Math.PI) / 180)
// A name's line keeps within this latitude of its own pole, so one written
// near a pole bends no more than one at 50°.
const MAX_SIN = Math.sin((50 * Math.PI) / 180)
// The smallest name, in hexes tall.
const MIN_CELLS = 2.2
// At most this many hexes are tried as the middle of the name.
const MAX_ANCHORS = 1500

// The line a name is written along: the circle round `axis` through `at`
// (the middle of the name), with up toward the axis. For an untilted name
// the axis is the north pole and the circle a line of latitude.
export function nameFrame(at, tilt = 0) {
  let north = [0, 0, 1]
  if (Math.abs(dot(at, north)) > 0.999999) north = [1, 0, 0]
  // The pole turned round `at` by the tilt (Rodrigues).
  const c = Math.cos(tilt)
  const s = Math.sin(tilt)
  let axis = add(add(scale(north, c), scale(cross(at, north), s)), scale(at, dot(at, north) * (1 - c)))
  let sin0 = dot(axis, at)
  if (Math.abs(sin0) > MAX_SIN) {
    const up = unit(add(axis, scale(at, -sin0)))
    sin0 = Math.sign(sin0) * MAX_SIN
    axis = add(scale(at, sin0), scale(up, Math.sqrt(1 - sin0 * sin0)))
  }
  const e1 = unit(add(at, scale(axis, -sin0)))
  return { axis, e1, e2: cross(axis, e1), lat0: Math.asin(sin0) }
}

// The point on the unit sphere u along the name's line and v above it (both
// angles, as arcs on the baseline and across it).
export function framePoint(f, u, v) {
  const lat = f.lat0 + v
  const lon = u / Math.cos(f.lat0)
  const r = Math.cos(lat)
  return add(add(scale(f.e1, r * Math.cos(lon)), scale(f.e2, r * Math.sin(lon))), scale(f.axis, Math.sin(lat)))
}

// Whether a name `length` long and `height` tall (angles) lies wholly on
// the water, checked every half hex along it and across it.
function fits(grid, inside, f, length, height, step) {
  const cols = Math.max(2, Math.ceil(length / step))
  const rows = Math.max(2, Math.ceil(height / step))
  const us = [0.5, 0, 1]
  for (let k = 1; k < cols; k++) us.push(k / cols)
  for (let r = 0; r <= rows; r++) {
    const v = (r / rows - 0.5) * height
    for (const t of us) if (!inside.has(grid.cellAt(framePoint(f, (t - 0.5) * length, v)))) return false
  }
  return true
}

// Where an ocean's name goes: cells is the ocean's, ratio the name's width
// over its height. The name is as big as suits the ocean, made smaller
// until it fits somewhere; at each size the hexes nearest the middle are
// tried first, the name level before tilted. Answers { at, tilt, height,
// length } (angles on the unit sphere), or null when even the smallest
// name fits nowhere.
export function placeOceanName(grid, cells, ratio) {
  if (!cells.length || !(ratio > 0)) return null
  const inside = cells instanceof Set ? cells : new Set(cells)
  const list = [...inside]
  const hex = Math.atan(2) / grid.n
  const extent = Math.sqrt((list.length * 4 * Math.PI) / grid.count)
  const sum = [0, 0, 0]
  for (const c of list) {
    const v = grid.center(c)
    sum[0] += v[0]
    sum[1] += v[1]
    sum[2] += v[2]
  }
  const mid = Math.hypot(...sum) > 1e-9 ? unit(sum) : grid.center(list[0])
  const stride = Math.max(1, Math.floor(list.length / MAX_ANCHORS))
  const anchors = []
  for (let k = 0; k < list.length; k += stride) {
    const at = grid.center(list[k])
    anchors.push({ at, d: -dot(at, mid) })
  }
  anchors.sort((a, b) => a.d - b.d)
  let height = Math.min(extent * 0.14, (extent * 0.8) / ratio)
  const smallest = hex * MIN_CELLS
  if (height < smallest) height = smallest
  for (;;) {
    const length = height * ratio
    for (const { at } of anchors) {
      for (const tilt of TILTS) {
        if (fits(grid, inside, nameFrame(at, tilt), length, height, hex / 2)) return { at, tilt, height, length }
      }
    }
    if (height <= smallest) return null
    height = Math.max(smallest, height * 0.8)
  }
}
