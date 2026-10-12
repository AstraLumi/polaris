// The planets' weather, as far as the solar system draws it: how cloudy
// each one is. No dependencies (no three.js): the drawing is in
// three/bodyShaders.js.
//
// Today every planet's clouds are generic, matched to its type. The main
// planet's come through mainPlanetCloudCover, one value per hex of the
// map, so that later it can be worked out from the map itself (wind
// currents, mountain ranges, humidity, temperature) without touching the
// drawing: change that function, keep what it returns.

// How much of a planet of each type is under cloud, 0 to 1. Gas giants are
// cloud all through: their bands are drawn on the planet instead.
const COVERAGE = { rocky: 0.45, ocean: 0.65, gas: 0 }

export const cloudCoverage = (body) => (body.kind === 'planet' ? COVERAGE[body.body_type] ?? 0 : 0)

// ---- Generic clouds ------------------------------------------------------------------
// Smooth random blobs on the sphere (value noise, a few octaves), the same
// every time for the same seed.
function hash(x, y, z, seed) {
  let h = Math.imul(x, 374761393) ^ Math.imul(y, 668265263) ^ Math.imul(z, 2147483647) ^ Math.imul(seed, 1274126177)
  h = Math.imul(h ^ (h >>> 13), 1274126177)
  return ((h ^ (h >>> 16)) >>> 0) / 4294967295
}

function valueNoise(p, seed) {
  const [x, y, z] = p
  const xi = Math.floor(x)
  const yi = Math.floor(y)
  const zi = Math.floor(z)
  const f = (t) => t * t * (3 - 2 * t)
  const [u, v, w] = [f(x - xi), f(y - yi), f(z - zi)]
  const lerp = (a, b, t) => a + (b - a) * t
  const c = (dx, dy, dz) => hash(xi + dx, yi + dy, zi + dz, seed)
  return lerp(
    lerp(lerp(c(0, 0, 0), c(1, 0, 0), u), lerp(c(0, 1, 0), c(1, 1, 0), u), v),
    lerp(lerp(c(0, 0, 1), c(1, 0, 1), u), lerp(c(0, 1, 1), c(1, 1, 1), u), v),
    w,
  )
}

// Cloud cover (0 to 1) at a point of the unit sphere, for a planet whose
// overall coverage is `coverage`.
export function genericCloudCover(dir, coverage, seed = 1) {
  let sum = 0
  let amp = 0.5
  let freq = 2.5
  for (let o = 0; o < 4; o++) {
    sum += amp * valueNoise([dir[0] * freq, dir[1] * freq, dir[2] * freq], seed + o)
    amp /= 2
    freq *= 2.1
  }
  // sum is about 0..0.94, centred near 0.47: shift it so `coverage` of the
  // sphere ends up cloudy.
  const t = sum + (coverage - 0.5) * 0.9
  return Math.max(0, Math.min(1, (t - 0.35) / 0.3))
}

// ---- The main planet -----------------------------------------------------------------
// The cloud cover of every hex of the map: (cell) -> 0..1. `grid` is the
// planet's (planetGrid.js); `body` the main planet. Generic for now, from
// its type. Later: read the layers (wind currents, mountain ranges) and the
// humidity and temperature maps here, and return what they make.
export function mainPlanetCloudCover({ grid, body }) {
  const coverage = cloudCoverage(body)
  const cache = new Map()
  return (cell) => {
    if (!cache.has(cell)) cache.set(cell, genericCloudCover(grid.center(cell), coverage, 7))
    return cache.get(cell)
  }
}
