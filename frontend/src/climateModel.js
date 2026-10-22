// The planet's climate, worked out from its map: the prevailing winds, the
// ocean currents, the temperature and the rain of every hex, month by month.
// Not a weather simulation: the large, lasting patterns the geography makes,
// the way an atlas draws them.
//
// How it is worked out, on a coarse copy of the map's grid (a few thousand
// cells; the map's own hexes are read back from it at the end):
//   1. Sunlight: the daily sunlight at every latitude, month by month, from
//      the axial tilt (insolation()).
//   2. Temperature: an energy balance (Budyko/Sellers/North): each cell
//      takes in sunlight (less what its ground and ice reflect), gives off
//      heat to space, stores heat (the sea a lot, land little, so inland
//      summers are hot and winters cold) and shares it with its neighbours
//      and downwind, by air and by the ocean's currents.
//   3. Winds: air pressure from the circulation cells (three per hemisphere
//      on an Earth-like day, fewer on slow planets, more on fast ones), which
//      follow the sun north and south, plus lows over warm land and highs
//      over cold land (monsoons). Wind blows from high to low pressure, turned
//      by the planet's spin (to the right in the north).
//   4. Ocean currents: the gyres the winds drive, with strong currents along
//      the western sides of the oceans (Stommel's model), plus the drift of
//      the surface water with the wind.
//   5. Rain: water evaporates from the sea (more when warm and windy) and
//      from plants (more where vegetation is dense), is carried by the
//      winds, and rains out where air rises (where winds meet, in lows, up
//      the windward side of mountains), not where it sinks (highs, the lee
//      of mountains).
// Steps 2–4 are repeated so the winds see the temperatures they make.
//
// Real units throughout (°C, m/s, mm, metres); the planet's size scales the
// distances. Deterministic: the same map gives the same climate.
// No dependencies but planetGrid.js: node frontend/tests/climateModel.test.mjs
import { PlanetGrid } from './planetGrid.js'

// Bumped whenever the results change for the same map.
export const CLIMATE_VERSION = 1

const EARTH_RADIUS_KM = 6371
const S_EARTH = 1361 // W/m², sunlight at Earth's distance
const OLR_A = 203.3 // W/m²: heat given off to space at 0 °C …
const OLR_B = 2.09 // … and per degree more (North & Coakley)
const LAPSE = 6.5 // °C colder per km up
const DEG = Math.PI / 180

// The model's constants. Heat capacities in W·year/m²/K (the sea's mixed
// layer; land with the air above it). Heat carried towards the poles by the
// atmosphere and oceans as a whole (diffusion of each latitude's mean, in
// W/m²/K on an Earth-sized planet's unit sphere; more in the tropics, the
// Hadley cells), and spread locally (weakly, so inland seasons stay
// harsh). What the wind and the currents carry, in W·year/m²/K per radian
// per year.
const K = {
  cSea: 5,
  cLand: 0.18,
  dPoleward: 0.3,
  dTropics: 2.5,
  dLocal: 0.12,
  cWind: 0.012,
  cCurrent: 0.4,
  albedoBase: 0.29,
  albedoSurface: 0.25,
  albedoLat: 0.078,
  desertClear: 0.1,
  snowSun: 300,
  thermalPressure: 1,
  iceAlbedo: 0.62,
  iceFrom: -2,
  iceSpan: 10,
}

const COARSE_FREQUENCY = 24


// ---- Vectors ---------------------------------------------------------------------------
const dot3 = (a, i, b, j) => a[i] * b[j] + a[i + 1] * b[j + 1] + a[i + 2] * b[j + 2]

// ---- A grid's geometry -------------------------------------------------------------------
// For every cell: its centre, east and north, latitude, and for each
// neighbour the direction to it along the surface (a unit vector in the
// cell's tangent plane) and the angle to it. Neighbours are listed CSR-style:
// cell c's are nb[start[c]] … nb[start[c + 1] − 1], in order round it.
export function gridGeometry(grid) {
  const n = grid.count
  const pos = new Float64Array(n * 3)
  const east = new Float64Array(n * 3)
  const north = new Float64Array(n * 3)
  const lat = new Float64Array(n)
  const start = new Int32Array(n + 1)
  const lists = []
  let total = 0
  for (let c = 0; c < n; c++) {
    const p = grid.center(c)
    pos.set(p, c * 3)
    lat[c] = Math.asin(Math.max(-1, Math.min(1, p[2])))
    let ex = -p[1]
    let ey = p[0]
    let len = Math.hypot(ex, ey)
    if (len < 1e-9) {
      ex = 0
      ey = 1
      len = 1
    }
    ex /= len
    ey /= len
    east.set([ex, ey, 0], c * 3)
    // north = up × east
    north.set([-p[2] * ey, p[2] * ex, p[0] * ey - p[1] * ex], c * 3)
    const around = grid.neighbors(c)
    lists.push(around)
    start[c] = total
    total += around.length
  }
  start[n] = total
  const nb = new Int32Array(total)
  const dir = new Float64Array(total * 3)
  const dist = new Float64Array(total)
  let sum = 0
  for (let c = 0; c < n; c++) {
    let k = start[c]
    for (const j of lists[c]) {
      nb[k] = j
      const d = [pos[j * 3] - pos[c * 3], pos[j * 3 + 1] - pos[c * 3 + 1], pos[j * 3 + 2] - pos[c * 3 + 2]]
      const along = d[0] * pos[c * 3] + d[1] * pos[c * 3 + 1] + d[2] * pos[c * 3 + 2]
      const t = [d[0] - along * pos[c * 3], d[1] - along * pos[c * 3 + 1], d[2] - along * pos[c * 3 + 2]]
      const tl = Math.hypot(...t) || 1
      dir.set([t[0] / tl, t[1] / tl, t[2] / tl], k * 3)
      dist[k] = Math.acos(Math.max(-1, Math.min(1, dot3(pos, c * 3, pos, j * 3))))
      sum += dist[k]
      k++
    }
  }
  return { grid, count: n, pos, east, north, lat, start, nb, dir, dist, spacing: sum / total }
}

// The gradient of a field at a cell, per radian, as a vector in its
// tangent plane (into out at o): (2/m) Σ (f_j − f_i) u_ij / h_ij.
function gradient(g, f, c, out, o = 0) {
  let x = 0
  let y = 0
  let z = 0
  const s = g.start[c]
  const e = g.start[c + 1]
  for (let k = s; k < e; k++) {
    const w = (f[g.nb[k]] - f[c]) / g.dist[k]
    x += w * g.dir[k * 3]
    y += w * g.dir[k * 3 + 1]
    z += w * g.dir[k * 3 + 2]
  }
  const m = 2 / (e - s)
  out[o] = x * m
  out[o + 1] = y * m
  out[o + 2] = z * m
}

// The divergence of a vector field (3 numbers per cell) at a cell, per radian.
function divergence(g, v, c) {
  let sum = 0
  const s = g.start[c]
  const e = g.start[c + 1]
  for (let k = s; k < e; k++) sum += dot3(v, g.nb[k] * 3, g.dir, k * 3) / g.dist[k]
  return (2 / (e - s)) * sum
}

// Smooths a field: each pass moves every cell halfway to its neighbours' mean.
function smooth(g, f, passes) {
  let a = Float64Array.from(f)
  let b = new Float64Array(f.length)
  for (let p = 0; p < passes; p++) {
    for (let c = 0; c < g.count; c++) {
      let sum = 0
      for (let k = g.start[c]; k < g.start[c + 1]; k++) sum += a[g.nb[k]]
      b[c] = 0.5 * a[c] + (0.5 * sum) / (g.start[c + 1] - g.start[c])
    }
    ;[a, b] = [b, a]
  }
  return a
}

// ---- Finding a point among the cells --------------------------------------------------------
// The three cells round a point of the unit sphere and their weights (they
// sum to 1), so fields can be read smoothly between cell centres.
export function locate(g, p) {
  const c = g.grid.cellAt(p)
  const s = g.start[c]
  const m = g.start[c + 1] - s
  let best = null
  let bestMin = -Infinity
  for (let k = 0; k < m; k++) {
    const a = g.nb[s + k]
    const b = g.nb[s + ((k + 1) % m)]
    const w = barycentric(g.pos, c, a, b, p)
    const low = Math.min(w[0], w[1], w[2])
    if (low > bestMin) {
      bestMin = low
      best = [c, a, b, w]
    }
    if (low >= -1e-9) break
  }
  const [c0, a, b, w] = best
  const w0 = Math.max(0, w[0])
  const w1 = Math.max(0, w[1])
  const w2 = Math.max(0, w[2])
  const sum = w0 + w1 + w2 || 1
  return [c0, a, b, w0 / sum, w1 / sum, w2 / sum]
}

// Weights of p in the flat triangle of three cells' centres (p taken onto
// the triangle's plane along the line from the planet's centre).
function barycentric(pos, i, j, k, p) {
  const A = [pos[i * 3], pos[i * 3 + 1], pos[i * 3 + 2]]
  const B = [pos[j * 3], pos[j * 3 + 1], pos[j * 3 + 2]]
  const C = [pos[k * 3], pos[k * 3 + 1], pos[k * 3 + 2]]
  // p = a A + b B + c C, solved with Cramer's rule.
  const det = (u, v, w) => u[0] * (v[1] * w[2] - v[2] * w[1]) - u[1] * (v[0] * w[2] - v[2] * w[0]) + u[2] * (v[0] * w[1] - v[1] * w[0])
  const d = det(A, B, C)
  if (Math.abs(d) < 1e-15) return [1, 0, 0]
  const a = det(p, B, C) / d
  const b = det(A, p, C) / d
  const c = det(A, B, p) / d
  const sum = a + b + c
  return [a / sum, b / sum, c / sum]
}

// A vector field read at a point (into out), turned into the tangent plane there.
export function sampleVector(g, field, p, out = [0, 0, 0]) {
  const [a, b, c, wa, wb, wc] = locate(g, p)
  for (let i = 0; i < 3; i++) out[i] = wa * field[a * 3 + i] + wb * field[b * 3 + i] + wc * field[c * 3 + i]
  const along = out[0] * p[0] + out[1] * p[1] + out[2] * p[2]
  for (let i = 0; i < 3; i++) out[i] -= along * p[i]
  return out
}

// ---- Sunlight ------------------------------------------------------------------------------
// Where the year is: step k of `steps`, as a fraction of the year at its
// middle. The year begins just after the northern winter solstice, as
// Earth's January does.
const SOLSTICE_OFFSET = 10 / 365.25
export const yearPhase = (k, steps) => (k + 0.5) / steps

// The sun's declination (radians) at a point of the year.
export function declination(tiltDeg, phase) {
  return -Math.asin(Math.sin(tiltDeg * DEG) * Math.cos(2 * Math.PI * (phase + SOLSTICE_OFFSET)))
}

// The daily mean sunlight (W/m²) at latitude lat for declination dec, with
// `flux` W/m² overhead.
export function insolation(lat, dec, flux = S_EARTH) {
  const x = -Math.tan(lat) * Math.tan(dec)
  const h0 = x >= 1 ? 0 : x <= -1 ? Math.PI : Math.acos(x)
  return (flux / Math.PI) * (h0 * Math.sin(lat) * Math.sin(dec) + Math.cos(lat) * Math.cos(dec) * Math.sin(h0))
}

// ---- Circulation cells ---------------------------------------------------------------------
// How many circulation cells each hemisphere has: three on a 24-hour day,
// one on a slowly turning planet, more on a fast one.
export const circulationCells = (dayHours) => Math.max(1, Math.min(7, Math.round(3 * Math.sqrt(24 / Math.max(0.5, dayHours)))))

// The air pressure (hPa from the mean) the cells make at latitude lat, with
// the thermal equator at latEq: low where air rises (the thermal equator,
// the polar fronts), high where it sinks (the subtropics, the poles).
function zonalPressure(lat, latEq, cells) {
  const span = lat >= latEq ? Math.PI / 2 - latEq : Math.PI / 2 + latEq
  const x = Math.min(1, Math.abs(lat - latEq) / span)
  return -9 * Math.cos(Math.PI * cells * x)
}

// The saturation vapour pressure over water (hPa) at t °C.
export const vapourPressure = (t) => 6.112 * Math.exp((17.62 * t) / (243.12 + t))

// ---- The model -----------------------------------------------------------------------------
// input:
//   frequency   the map's grid
//   land        Uint8Array, 1 for land, per map cell
//   veg         plant density 0..1 per cell (land only)
//   albedo      the ground's reflectivity per cell (land; the sea is 0.07)
//   height      ground height in metres per cell (land; the sea is 0)
//   water       1 for inland water (lakes) per cell
//   tilt        axial tilt, degrees
//   dayHours    the length of the day
//   steps       months in the year (worked out as up to 24 steps)
//   insolation  sunlight relative to Earth's (1 = Earth)
//   radiusKm    the planet's radius
//   monthDays   Earth days in a month (for rain per month)
export function simulateClimate(input) {
  const P = { ...K, ...input.tune }
  // How much of a cell is under ice or snow at t °C.
  const iceFrac = (t) => Math.max(0, Math.min(1, (P.iceFrom - t) / P.iceSpan))
  const fine = new PlanetGrid(input.frequency)
  const coarseN = Math.min(input.frequency, input.coarseFrequency ?? COARSE_FREQUENCY)
  const g = gridGeometry(coarseN === input.frequency ? fine : new PlanetGrid(coarseN))
  const n = g.count
  const steps = Math.max(1, Math.min(24, Math.round(input.steps ?? 12)))
  const tilt = input.tilt ?? 23.44
  const radiusKm = input.radiusKm ?? EARTH_RADIUS_KM
  const flux = S_EARTH * (input.insolation ?? 1)
  const cells = circulationCells(input.dayHours ?? 24)
  const monthDays = input.monthDays ?? 365.25 / 12

  // ---- The map, read onto the coarse cells.
  const owner = new Int32Array(fine.count)
  const interp = { idx: new Int32Array(fine.count * 3), w: new Float32Array(fine.count * 3) }
  const landFrac = new Float64Array(n)
  const lakeFrac = new Float64Array(n)
  const vegMean = new Float64Array(n)
  const albedoMean = new Float64Array(n)
  const heightMean = new Float64Array(n)
  const counts = new Float64Array(n)
  const same = g.grid === fine
  for (let f = 0; f < fine.count; f++) {
    const p = fine.center(f)
    let c
    if (same) {
      c = f
      interp.idx.set([f, f, f], f * 3)
      interp.w.set([1, 0, 0], f * 3)
    } else {
      const [a, b, d, wa, wb, wd] = locate(g, p)
      interp.idx.set([a, b, d], f * 3)
      interp.w.set([wa, wb, wd], f * 3)
      c = g.grid.cellAt(p)
    }
    owner[f] = c
    counts[c]++
    const isLand = input.land[f] === 1
    if (isLand) {
      landFrac[c]++
      vegMean[c] += input.veg?.[f] ?? 0.45
      if (input.water?.[f]) lakeFrac[c]++
      heightMean[c] += Math.max(0, input.height?.[f] ?? 0)
    }
    albedoMean[c] += isLand ? (input.albedo?.[f] ?? 0.2) : 0.07
  }
  for (let c = 0; c < n; c++) {
    const k = counts[c] || 1
    if (landFrac[c]) {
      vegMean[c] /= landFrac[c]
      lakeFrac[c] /= landFrac[c]
    }
    heightMean[c] /= k
    landFrac[c] /= k
    albedoMean[c] /= k
  }
  // Cells with no map cell of their own (a coarse grid finer than the map
  // can't happen, but a pentagon may miss out): copy a neighbour.
  for (let c = 0; c < n; c++) {
    if (counts[c]) continue
    const j = g.nb[g.start[c]]
    landFrac[c] = landFrac[j]
    vegMean[c] = vegMean[j]
    albedoMean[c] = albedoMean[j]
    heightMean[c] = heightMean[j]
  }
  const sea = new Uint8Array(n)
  const wetness = new Float64Array(n)
  for (let c = 0; c < n; c++) {
    sea[c] = landFrac[c] < 0.5 ? 1 : 0
    wetness[c] = 1 - landFrac[c] + landFrac[c] * Math.max(lakeFrac[c], Math.min(1, 0.4 + vegMean[c]))
  }

  // ---- Sunlight per step.
  const decl = new Float64Array(steps)
  for (let k = 0; k < steps; k++) decl[k] = declination(tilt, yearPhase(k, steps))
  const sun = new Float64Array(steps * n)
  for (let k = 0; k < steps; k++) for (let c = 0; c < n; c++) sun[k * n + c] = insolation(g.lat[c], decl[k], flux)
  const sunYear = new Float64Array(n)
  for (let c = 0; c < n; c++) {
    let s = 0
    for (let k = 0; k < steps; k++) s += sun[k * n + c]
    sunYear[c] = s / steps
  }

  // Distances scale with the planet: on a smaller one, air crosses more of
  // it in a year; eddies spread heat over more of it.
  const R = radiusKm * 1000
  const radPerYear = (ms) => (ms * 3.156e7) / R
  const scaleD = (EARTH_RADIUS_KM / radiusKm) ** 2

  // ---- Temperature (sea-level equivalent, °C) per step.
  const temp = new Float64Array(steps * n)
  const wind = new Float64Array(steps * n * 3)
  const windYear = new Float64Array(n * 3)
  const current = new Float64Array(n * 3)
  const pressure = new Float64Array(steps * n)

  // A first guess: each latitude in balance with its yearly sunlight.
  const T = new Float64Array(n)
  for (let c = 0; c < n; c++) {
    T[c] = (sunYear[c] * (1 - planetAlbedo(c, 15, sunYear[c])) - OLR_A) / OLR_B
    for (let k = 0; k < steps; k++) temp[k * n + c] = T[c]
  }

  // What a cell reflects: clouds and its ground, more where the sun is low,
  // and snow and ice when cold. Snow on land melts under a strong sun
  // whatever the air does, as it does in a real spring.
  function planetAlbedo(c, t, sunNow) {
    const s = Math.sin(g.lat[c])
    const lf = landFrac[c]
    // Dry, bare land has few clouds to reflect the sun (deserts bake).
    const clear = P.desertClear * lf * (1 - Math.min(1, vegMean[c] * 2))
    const base = P.albedoBase - clear + P.albedoSurface * (albedoMean[c] - 0.07) + P.albedoLat * ((3 * s * s - 1) / 2)
    const melt = Math.max(0, Math.min(1, (P.snowSun - sunNow) / 150))
    const ice = iceFrac(t) * ((1 - lf) + lf * melt * (1 - 0.5 * vegMean[c]))
    return Math.max(0.05, Math.min(0.85, base + Math.max(0, P.iceAlbedo - base) * ice))
  }

  // Conductances along every neighbour link: local mixing (fixed) and
  // what wind and currents carry (upwind only; set per step).
  const links = g.nb.length
  const kDiff = new Float64Array(links)
  for (let c = 0; c < n; c++) {
    for (let k = g.start[c]; k < g.start[c + 1]; k++) kDiff[k] = (P.dLocal * scaleD * 2) / (3 * g.dist[k] * g.dist[k])
  }

  // Latitude bands for the poleward transport: as many as the cells allow
  // with none empty.
  let bands = Math.max(4, Math.min(60, Math.round(Math.PI / (g.spacing * 1.2))))
  let bandOf
  let bandCount
  for (;;) {
    bandOf = new Int32Array(n)
    bandCount = new Float64Array(bands)
    for (let c = 0; c < n; c++) {
      const b = Math.min(bands - 1, Math.floor(((g.lat[c] + Math.PI / 2) / Math.PI) * bands))
      bandOf[c] = b
      bandCount[b]++
    }
    if (bandCount.every((x) => x > 0) || bands <= 4) break
    bands--
  }
  const bandStep = Math.PI / bands
  const edgeK = new Float64Array(bands + 1) // conductance across each edge, per band area
  const bandLat = (b) => -Math.PI / 2 + (b + 0.5) * bandStep
  for (let b = 1; b < bands; b++) {
    const lat = -Math.PI / 2 + b * bandStep
    const tropics = 1 + P.dTropics * Math.exp(-((lat / (25 * DEG)) ** 2))
    edgeK[b] = (P.dPoleward * scaleD * tropics * Math.cos(lat)) / (bandStep * bandStep)
  }
  const bandMeanT = new Float64Array(bands)
  const bandCap = new Float64Array(bands)
  const bandRhs = new Float64Array(bands)
  const bandNew = new Float64Array(bands)
  const bandHeat = new Float64Array(bands)
  // The poleward transport over a step: each band's mean temperature moves
  // (implicitly) with its neighbours'; the heat that brings is shared out
  // evenly over the band's cells.
  function poleward(absorbed, capOverDt, old) {
    bandMeanT.fill(0)
    bandCap.fill(0)
    bandRhs.fill(0)
    for (let c = 0; c < n; c++) {
      const b = bandOf[c]
      bandMeanT[b] += old[c] / bandCount[b]
      bandCap[b] += capOverDt[c] / bandCount[b]
      bandRhs[b] += (capOverDt[c] * old[c] + absorbed[c] - OLR_A) / bandCount[b]
    }
    // Tridiagonal: (cap + B) T_b − Σ k (T_nb − T_b) / cos φ_b = rhs
    const lower = new Float64Array(bands)
    const diag = new Float64Array(bands)
    const upper = new Float64Array(bands)
    for (let b = 0; b < bands; b++) {
      const w = Math.cos(bandLat(b))
      lower[b] = b > 0 ? -edgeK[b] / w : 0
      upper[b] = b < bands - 1 ? -edgeK[b + 1] / w : 0
      diag[b] = bandCap[b] + OLR_B - lower[b] - upper[b]
    }
    // Thomas algorithm
    const cp = new Float64Array(bands)
    const dp = new Float64Array(bands)
    for (let b = 0; b < bands; b++) {
      const m = diag[b] - (b > 0 ? lower[b] * cp[b - 1] : 0)
      cp[b] = upper[b] / m
      dp[b] = (bandRhs[b] - (b > 0 ? lower[b] * dp[b - 1] : 0)) / m
    }
    for (let b = bands - 1; b >= 0; b--) bandNew[b] = dp[b] - (b < bands - 1 ? cp[b] * bandNew[b + 1] : 0)
    for (let b = 0; b < bands; b++) {
      const w = Math.cos(bandLat(b))
      let h = 0
      if (b > 0) h += (edgeK[b] * (bandNew[b - 1] - bandNew[b])) / w
      if (b < bands - 1) h += (edgeK[b + 1] * (bandNew[b + 1] - bandNew[b])) / w
      bandHeat[b] = h
    }
  }
  const kAdv = new Float64Array(links)

  // Upwind carrying along the links into cell c by velocity v (m/s), with
  // capacity cap: −cap v·∇T ≈ Σ a_cj (T_j − T_c).
  function addCarry(v, o, c, cap, into) {
    const speed = Math.hypot(v[o], v[o + 1], v[o + 2])
    if (speed < 1e-6) return
    const m = g.start[c + 1] - g.start[c]
    const r = radPerYear(speed)
    for (let k = g.start[c]; k < g.start[c + 1]; k++) {
      const back = -dot3(g.dir, k * 3, v, o) / speed
      if (back > 0) into[k] += (cap * 4 * back * r) / (m * g.dist[k])
    }
  }

  function setCarry(stepWind) {
    kAdv.fill(0)
    for (let c = 0; c < n; c++) {
      addCarry(stepWind, c * 3, c, P.cWind, kAdv)
      if (sea[c]) addCarry(current, c * 3, c, P.cCurrent * (1 - landFrac[c]), kAdv)
    }
  }

  // Gauss-Seidel sweeps of the implicit energy balance:
  // (cap/dt + B + Σk) T_c − Σ k T_j = cap/dt T_c,old + absorbed − A.
  function balance(absorbed, capOverDt, old, sweeps) {
    poleward(absorbed, capOverDt, old)
    for (let s = 0; s < sweeps; s++) {
      for (let c = 0; c < n; c++) {
        let diag = capOverDt[c] + OLR_B
        let rhs = capOverDt[c] * old[c] + absorbed[c] - OLR_A + bandHeat[bandOf[c]]
        for (let k = g.start[c]; k < g.start[c + 1]; k++) {
          const w = kDiff[k] + kAdv[k]
          diag += w
          rhs += w * T[g.nb[k]]
        }
        T[c] = rhs / diag
      }
    }
  }

  const absorbed = new Float64Array(n)
  const capDt = new Float64Array(n)
  const old = new Float64Array(n)
  const zero = new Float64Array(n)
  const sub = 3
  const dt = 1 / (steps * sub)

  function runYears(years) {
    const sums = new Float64Array(steps * n)
    for (let y = 0; y < years; y++) {
      const last = y === years - 1
      for (let k = 0; k < steps; k++) {
        setCarry(wind.subarray(k * n * 3, (k + 1) * n * 3))
        for (let s = 0; s < sub; s++) {
          for (let c = 0; c < n; c++) {
            const ice = iceFrac(T[c])
            const cap = landFrac[c] * P.cLand + (1 - landFrac[c]) * P.cSea * (1 - 0.95 * ice)
            capDt[c] = cap / dt
            absorbed[c] = sun[k * n + c] * (1 - planetAlbedo(c, T[c], sun[k * n + c]))
            old[c] = T[c]
          }
          balance(absorbed, capDt, old, 4)
          if (last) for (let c = 0; c < n; c++) sums[k * n + c] += T[c] / sub
        }
      }
    }
    temp.set(sums)
  }

  // The yearly balance, solved outright: a good start for the seasons.
  function settleYear() {
    setCarry(windYear)
    for (let it = 0; it < 6; it++) {
      for (let c = 0; c < n; c++) absorbed[c] = sunYear[c] * (1 - planetAlbedo(c, T[c], sunYear[c]))
      for (let c = 0; c < n; c++) old[c] = T[c]
      balance(absorbed, zero, old, 60)
    }
  }

  // ---- Winds per step from the temperatures.
  const grad = new Float64Array(3)
  function makeWinds() {
    windYear.fill(0)
    for (let k = 0; k < steps; k++) {
      const Tk = temp.subarray(k * n, (k + 1) * n)
      // Land warmer than its latitude makes a low, colder a high.
      const anomaly = smooth(g, minusZonal(g, Tk), 4)
      const eq = thermalEquator(g, Tk, decl[k], cells, wetness)
      const press = pressure.subarray(k * n, (k + 1) * n)
      for (let c = 0; c < n; c++) press[c] = zonalPressure(g.lat[c], eq(c), cells) - P.thermalPressure * anomaly[c]
      const W = wind.subarray(k * n * 3, (k + 1) * n * 3)
      for (let c = 0; c < n; c++) {
        gradient(g, press, c, grad)
        // hPa per 1000 km
        const per1000 = 1e6 / R
        const gx = -grad[0] * per1000
        const gy = -grad[1] * per1000
        const gz = -grad[2] * per1000
        const mag = Math.hypot(gx, gy, gz)
        if (mag < 1e-9) {
          W.fill(0, c * 3, c * 3 + 3)
          continue
        }
        const lf = landFrac[c]
        // Calmer near the equator, where nothing turns the flow into a
        // steady wind (the doldrums).
        const calm = 0.45 + 0.55 * Math.min(1, Math.abs(g.lat[c]) / (15 * DEG))
        const speed = Math.min(30, 2.2 * mag) * (1 - 0.3 * lf) * calm
        // Turned by the spin: right in the north, left in the south, less
        // near the equator and over rough land.
        const lat = g.lat[c]
        const theta = (70 - 15 * lf) * DEG * Math.min(1, Math.abs(lat) / (12 * DEG))
        const side = lat >= 0 ? 1 : -1
        const ux = gx / mag
        const uy = gy / mag
        const uz = gz / mag
        const px = g.pos[c * 3]
        const py = g.pos[c * 3 + 1]
        const pz = g.pos[c * 3 + 2]
        // up × u
        const qx = py * uz - pz * uy
        const qy = pz * ux - px * uz
        const qz = px * uy - py * ux
        const cs = Math.cos(theta)
        const sn = side * Math.sin(theta)
        W[c * 3] = speed * (cs * ux - sn * qx)
        W[c * 3 + 1] = speed * (cs * uy - sn * qy)
        W[c * 3 + 2] = speed * (cs * uz - sn * qz)
      }
      for (let i = 0; i < n * 3; i++) windYear[i] += W[i] / steps
    }
  }

  // ---- Ocean currents from the year's winds.
  function makeCurrents() {
    current.set(oceanCurrents(g, sea, windYear))
  }

  makeWinds()
  makeCurrents()
  settleYear()
  for (let k = 0; k < steps; k++) temp.set(T, k * n)
  for (let round = 0; round < 3; round++) {
    runYears(round === 0 ? 3 : 2)
    makeWinds()
    makeCurrents()
  }

  // ---- Rain per step.
  const precip = new Float64Array(steps * n)
  const humid = new Float64Array(steps * n)
  for (let k = 0; k < steps; k++) {
    const r = rainfall(g, {
      temp: temp.subarray(k * n, (k + 1) * n),
      wind: wind.subarray(k * n * 3, (k + 1) * n * 3),
      pressure: pressure.subarray(k * n, (k + 1) * n),
      landFrac,
      lakeFrac,
      veg: vegMean,
      height: heightMean,
      radiusKm,
    })
    precip.set(r.precip.map((x) => (x * monthDays) / 30.44), k * n)
    humid.set(r.humidity, k * n)
  }

  return {
    version: CLIMATE_VERSION,
    steps,
    cells,
    frequency: input.frequency,
    coarse: {
      frequency: coarseN,
      count: n,
      landFrac: Float32Array.from(landFrac),
      sea,
      temp: Float32Array.from(temp),
      precip: Float32Array.from(precip),
      humidity: Float32Array.from(humid),
      pressure: Float32Array.from(pressure),
      wind: Float32Array.from(wind),
      current: Float32Array.from(current),
    },
    interp,
  }
}

// Where the thermal equator (the rain belt where the trade winds meet) runs
// at every cell's longitude: the warmest and wettest latitude of that
// meridian (hot deserts hold a heat low but not the rain belt), within
// reach of the sun, smoothed east–west. It swings far over hot green
// continents (monsoons) and little over the sea. wetness: 1 for sea, the
// plants' density on land. Answers (cell) -> latitude.
function thermalEquator(g, T, dec, cells, wetness) {
  const columns = 36
  const reach = Math.min(Math.abs(dec) + 6 * DEG, (Math.PI / 2 / cells) * 1.1)
  const best = new Float64Array(columns).fill(-Infinity)
  const at = new Float64Array(columns).fill(0.4 * dec)
  const colOf = (c) => {
    const lon = Math.atan2(g.pos[c * 3 + 1], g.pos[c * 3])
    return Math.min(columns - 1, Math.floor(((lon + Math.PI) / (2 * Math.PI)) * columns))
  }
  const smoothT = smooth(g, T, 3)
  for (let c = 0; c < g.count; c++) {
    const lat = g.lat[c]
    if (Math.abs(lat) > reach) continue
    // Leaning towards the sun's side, so a flat ocean picks the sun's latitude.
    const score = smoothT[c] + 3 * wetness[c] - Math.abs(lat - 0.5 * dec) / (10 * DEG)
    const col = colOf(c)
    if (score > best[col]) {
      best[col] = score
      at[col] = lat
    }
  }
  const eased = new Float64Array(columns)
  for (let i = 0; i < columns; i++) {
    let sum = 0
    for (let d = -2; d <= 2; d++) sum += at[(i + d + columns) % columns] * (3 - Math.abs(d))
    eased[i] = (0.85 * sum) / 9
  }
  return (c) => {
    const lon = Math.atan2(g.pos[c * 3 + 1], g.pos[c * 3])
    const x = ((lon + Math.PI) / (2 * Math.PI)) * columns - 0.5
    const i = Math.floor(x)
    const f = x - i
    return eased[(i + columns) % columns] * (1 - f) + eased[(i + 1) % columns] * f
  }
}

// A field less its latitude's mean (in bands of 3°).
function minusZonal(g, f) {
  const bands = 60
  const sum = new Float64Array(bands)
  const num = new Float64Array(bands)
  const band = (lat) => Math.min(bands - 1, Math.floor(((lat + Math.PI / 2) / Math.PI) * bands))
  for (let c = 0; c < g.count; c++) {
    sum[band(g.lat[c])] += f[c]
    num[band(g.lat[c])]++
  }
  const out = new Float64Array(g.count)
  for (let c = 0; c < g.count; c++) {
    const b = band(g.lat[c])
    out[c] = f[c] - sum[b] / num[b]
  }
  return out
}

// ---- Ocean currents ------------------------------------------------------------------------
// The wind-driven circulation: Stommel's model, r∇²ψ + β ∂ψ/∂x = curl τ,
// with ψ = 0 along every coast, solved on the sea cells. The flow along ψ
// (up × ∇ψ) gives the gyres and their strong western currents; the surface
// also drifts with the wind (which carries water round a planet where no
// land is in the way). Answers m/s, 3 per cell (0 on land).
export function oceanCurrents(g, sea, wind) {
  const n = g.count
  // The wind's stress on the water, ∝ |v| v.
  const stress = new Float64Array(n * 3)
  for (let c = 0; c < n; c++) {
    const s = Math.hypot(wind[c * 3], wind[c * 3 + 1], wind[c * 3 + 2])
    for (let i = 0; i < 3; i++) stress[c * 3 + i] = (s * wind[c * 3 + i]) / 50
  }
  // curl τ = −div(up × τ)
  const turned = new Float64Array(n * 3)
  for (let c = 0; c < n; c++) {
    const [px, py, pz] = [g.pos[c * 3], g.pos[c * 3 + 1], g.pos[c * 3 + 2]]
    const [tx, ty, tz] = [stress[c * 3], stress[c * 3 + 1], stress[c * 3 + 2]]
    turned[c * 3] = py * tz - pz * ty
    turned[c * 3 + 1] = pz * tx - px * tz
    turned[c * 3 + 2] = px * ty - py * tx
  }
  const curl = new Float64Array(n)
  for (let c = 0; c < n; c++) if (sea[c]) curl[c] = -divergence(g, turned, c)

  const h = g.spacing
  const r = 2 * h
  const psi = new Float64Array(n)
  // Coefficients of every link, and the diagonal.
  const coef = new Float64Array(g.nb.length)
  const diag = new Float64Array(n)
  for (let c = 0; c < n; c++) {
    if (!sea[c]) continue
    const m = g.start[c + 1] - g.start[c]
    const beta = Math.cos(g.lat[c])
    let d = 0
    for (let k = g.start[c]; k < g.start[c + 1]; k++) {
      const lap = (r * 4) / (m * g.dist[k] * g.dist[k])
      const adv = (beta * 2 * dot3(g.dir, k * 3, g.east, c * 3)) / (m * g.dist[k])
      coef[k] = lap + adv
      d -= lap + adv
    }
    diag[c] = d
  }
  // Successive over-relaxation.
  const omega = 1.8
  for (let it = 0; it < 1500; it++) {
    let change = 0
    let size = 0
    for (let c = 0; c < n; c++) {
      if (!sea[c]) continue
      let sum = 0
      for (let k = g.start[c]; k < g.start[c + 1]; k++) {
        const j = g.nb[k]
        if (sea[j]) sum += coef[k] * psi[j]
      }
      const next = (curl[c] - sum) / diag[c]
      const delta = omega * (next - psi[c])
      psi[c] += delta
      change = Math.max(change, Math.abs(delta))
      size = Math.max(size, Math.abs(psi[c]))
    }
    if (it > 50 && change <= 1e-6 * (size || 1)) break
  }

  // The flow along ψ, scaled so the strongest currents run at about 1.2 m/s.
  const flow = new Float64Array(n * 3)
  const grad = new Float64Array(3)
  const speeds = []
  for (let c = 0; c < n; c++) {
    if (!sea[c]) continue
    gradient(g, psi, c, grad)
    const [px, py, pz] = [g.pos[c * 3], g.pos[c * 3 + 1], g.pos[c * 3 + 2]]
    flow[c * 3] = py * grad[2] - pz * grad[1]
    flow[c * 3 + 1] = pz * grad[0] - px * grad[2]
    flow[c * 3 + 2] = px * grad[1] - py * grad[0]
    speeds.push(Math.hypot(flow[c * 3], flow[c * 3 + 1], flow[c * 3 + 2]))
  }
  speeds.sort((a, b) => a - b)
  const top = speeds[Math.floor(speeds.length * 0.98)] || 1
  const scale = top > 1e-12 ? 1.2 / top : 0

  const out = new Float64Array(n * 3)
  for (let c = 0; c < n; c++) {
    if (!sea[c]) continue
    // The surface drift: a few per cent of the wind, turned a little right
    // in the north (left in the south).
    const side = g.lat[c] >= 0 ? 1 : -1
    const [px, py, pz] = [g.pos[c * 3], g.pos[c * 3 + 1], g.pos[c * 3 + 2]]
    const [wx, wy, wz] = [wind[c * 3], wind[c * 3 + 1], wind[c * 3 + 2]]
    const q = [py * wz - pz * wy, pz * wx - px * wz, px * wy - py * wx]
    const cs = Math.cos(25 * DEG)
    const sn = side * Math.sin(25 * DEG)
    const v = [0, 1, 2].map((i) => flow[c * 3 + i] * scale + 0.035 * (cs * wind[c * 3 + i] - sn * q[i]))
    // Nothing flows into the land.
    for (let k = g.start[c]; k < g.start[c + 1]; k++) {
      if (sea[g.nb[k]]) continue
      const into = v[0] * g.dir[k * 3] + v[1] * g.dir[k * 3 + 1] + v[2] * g.dir[k * 3 + 2]
      if (into > 0) for (let i = 0; i < 3; i++) v[i] -= into * g.dir[k * 3 + i]
    }
    out.set(v, c * 3)
  }
  return out
}

// ---- Rain ----------------------------------------------------------------------------------
// The water in the air (precipitable water, mm) in balance: what evaporates
// into a cell, plus what the wind brings, equals what rains out plus what
// the wind takes on. Rain falls faster where the air is near saturation and
// where it rises. Answers mm per 30.44-day month.
export function rainfall(g, { temp, wind, pressure, landFrac, lakeFrac, veg, height, radiusKm }) {
  const n = g.count
  const R = radiusKm * 1000
  const MONTH_S = 30.44 * 86400
  // How far moisture really travels: winds turn and slow aloft.
  const carry = 0.35
  const q = new Float64Array(n)
  const cap = new Float64Array(n)
  const source = new Float64Array(n)
  const dyn = new Float64Array(n)
  const P = new Float64Array(n)
  const grad = new Float64Array(3)
  const anomaly = smooth(g, minusZonal(g, temp), 2)
  for (let c = 0; c < n; c++) {
    const t = temp[c]
    cap[c] = 2.2 * vapourPressure(t)
    const speed = Math.hypot(wind[c * 3], wind[c * 3 + 1], wind[c * 3 + 2])
    const ice = Math.max(0, Math.min(1, (-t - 2) / 10))
    const open = (1 - landFrac[c] + landFrac[c] * lakeFrac[c]) * (1 - ice)
    // Water evaporates from the sea and lakes, more when warm and windy;
    // plants breathe out water too, as much as their density allows.
    const potential = 3 * vapourPressure(t) * (0.45 + 0.55 * Math.min(1.6, speed / 7))
    const plants = landFrac[c] * (1 - lakeFrac[c]) * veg[c] * (t > 0 ? 0.7 : 0.2)
    source[c] = (open + plants) * potential
    // What makes air rise or sink here.
    const conv = (-divergence(g, wind, c) * 1e6) / R // (m/s) per 1000 km
    gradient(g, height, c, grad)
    const lift = (wind[c * 3] * grad[0] + wind[c * 3 + 1] * grad[1] + wind[c * 3 + 2] * grad[2]) / R // m/s
    const p = pressure[c]
    let f = Math.exp(-0.11 * Math.max(0, p)) * (1 + 0.03 * Math.max(0, -p))
    f *= 1 + 0.15 * Math.max(-0.8, Math.min(4, conv))
    // Over bare, dry land the rising air rarely rains (dust, high cloud
    // bases): rain comes easier the denser the plants.
    const bare = landFrac[c] * (1 - lakeFrac[c])
    f *= 1 - bare + bare * (0.2 + 0.8 * Math.sqrt(veg[c]))
    // Air over water colder than its latitude (cold currents) is stable
    // and keeps its rain; over warmer water it rises readily.
    f *= Math.max(0.3, Math.min(1.8, 1 + 0.08 * anomaly[c]))
    f *= lift > 0 ? 1 + 40 * Math.min(0.15, lift) : Math.max(0.25, 1 + 15 * lift)
    dyn[c] = Math.max(0.03, f)
    q[c] = 0.5 * cap[c]
  }
  // Upwind links, as for heat.
  const links = g.nb.length
  const a = new Float64Array(links)
  const d = new Float64Array(links)
  for (let c = 0; c < n; c++) {
    const m = g.start[c + 1] - g.start[c]
    const speed = Math.hypot(wind[c * 3], wind[c * 3 + 1], wind[c * 3 + 2])
    const r = (speed * MONTH_S * carry) / R // radians per month
    for (let k = g.start[c]; k < g.start[c + 1]; k++) {
      d[k] = (0.004 * 4) / (m * g.dist[k] * g.dist[k])
      if (speed < 1e-6) continue
      const back = -dot3(g.dir, k * 3, wind, c * 3) / speed
      if (back > 0) a[k] = (4 * back * r) / (m * g.dist[k])
    }
  }
  const rate = new Float64Array(n)
  const evap = new Float64Array(n)
  for (let it = 0; it < 240; it++) {
    if (it % 6 === 0) {
      for (let c = 0; c < n; c++) {
        const rh = Math.min(2, q[c] / cap[c])
        rate[c] = 3.4 * rh * rh * dyn[c]
        // Bare ground gives back a little of the rain.
        evap[c] = source[c] + 0.05 * landFrac[c] * (1 - lakeFrac[c]) * rate[c] * q[c]
      }
    }
    for (let c = 0; c < n; c++) {
      let diag = rate[c]
      let rhs = evap[c]
      for (let k = g.start[c]; k < g.start[c + 1]; k++) {
        const w = a[k] + d[k]
        diag += w
        rhs += w * q[g.nb[k]]
      }
      q[c] = diag > 0 ? rhs / diag : 0
    }
  }
  for (let c = 0; c < n; c++) {
    const rh = Math.min(2, q[c] / cap[c])
    P[c] = 3.4 * rh * rh * dyn[c] * q[c]
  }
  const humidity = new Float64Array(n)
  for (let c = 0; c < n; c++) humidity[c] = Math.min(1, q[c] / cap[c])
  return { precip: P, humidity }
}

// ---- Reading the results on the map's hexes ----------------------------------------------------
// A coarse field (one number per coarse cell, for one step) read on every
// map cell.
export function fineField(result, field) {
  const { idx, w } = result.interp
  const out = new Float32Array(idx.length / 3)
  for (let f = 0; f < out.length; f++) {
    out[f] = w[f * 3] * field[idx[f * 3]] + w[f * 3 + 1] * field[idx[f * 3 + 1]] + w[f * 3 + 2] * field[idx[f * 3 + 2]]
  }
  return out
}

const stepSlice = (result, field, k) => field.subarray(k * result.coarse.count, (k + 1) * result.coarse.count)

// The temperature (°C) of every map cell at step k, or the year's mean
// when k is null: the coarse sea-level temperature, colder up high.
export function temperatureAt(result, heights, k = null) {
  const { steps, coarse } = result
  let field
  if (k == null) {
    field = new Float32Array(coarse.count)
    for (let s = 0; s < steps; s++) for (let c = 0; c < coarse.count; c++) field[c] += coarse.temp[s * coarse.count + c] / steps
  } else field = stepSlice(result, coarse.temp, k)
  const out = fineField(result, field)
  if (heights) for (let f = 0; f < out.length; f++) out[f] -= (LAPSE * Math.max(0, heights[f])) / 1000
  return out
}

// The rain (mm) on every map cell in step k, or over the whole year when k
// is null.
export function precipitationAt(result, k = null) {
  const { steps, coarse } = result
  let field
  if (k == null) {
    field = new Float32Array(coarse.count)
    for (let s = 0; s < steps; s++) for (let c = 0; c < coarse.count; c++) field[c] += coarse.precip[s * coarse.count + c]
  } else field = stepSlice(result, coarse.precip, k)
  return fineField(result, field)
}

// The winds of step k (or the year's mean), 3 numbers per coarse cell.
export function windAt(result, k = null) {
  const { steps, coarse } = result
  const size = coarse.count * 3
  if (k != null) return coarse.wind.subarray(k * size, (k + 1) * size)
  const out = new Float32Array(size)
  for (let s = 0; s < steps; s++) for (let i = 0; i < size; i++) out[i] += coarse.wind[s * size + i] / steps
  return out
}

// ---- Climate types (Köppen) ---------------------------------------------------------------------
// The Köppen–Geiger type of a place from its monthly temperatures (°C) and
// rain (mm), for a year of any number of months (counts of months scale to
// a 12-month year). `north`: whether the summer half is the year's middle
// (true in the north with this calendar's seasons).
export function koppen(temps, rains, north = true) {
  const m = temps.length
  const per = 12 / m
  const mean = temps.reduce((s, x) => s + x, 0) / m
  const total = rains.reduce((s, x) => s + x, 0) * per
  const hot = Math.max(...temps)
  const cold = Math.min(...temps)
  // The summer half: the middle of the year in the north, its ends in the
  // south (the year begins after the northern winter solstice).
  const inSummer = (k) => {
    const mid = (k + 0.5) / m
    const middle = mid > 0.25 && mid <= 0.75
    return north ? middle : !middle
  }
  let summerRain = 0
  let sDry = Infinity
  let sWet = 0
  let wDry = Infinity
  let wWet = 0
  for (let k = 0; k < m; k++) {
    const r = rains[k] * per
    if (inSummer(k)) {
      summerRain += r
      sDry = Math.min(sDry, r)
      sWet = Math.max(sWet, r)
    } else {
      wDry = Math.min(wDry, r)
      wWet = Math.max(wWet, r)
    }
  }
  if (sDry === Infinity) sDry = sWet = 0
  if (wDry === Infinity) wDry = wWet = 0
  const driest = Math.min(...rains) * per

  if (hot < 10) return hot < 0 ? 'EF' : 'ET'
  const share = total > 0 ? summerRain / total : 0.5
  const threshold = 20 * mean + (share >= 0.7 ? 280 : share >= 0.3 ? 140 : 0)
  if (total < threshold) {
    const kind = total < threshold / 2 ? 'BW' : 'BS'
    return kind + (mean >= 18 ? 'h' : 'k')
  }
  if (cold >= 18) {
    if (driest >= 60) return 'Af'
    if (driest >= 100 - total / 25) return 'Am'
    return 'Aw'
  }
  const group = cold > -3 ? 'C' : 'D'
  let second = 'f'
  if (sDry < 40 && sDry < wWet / 3) second = 's'
  else if (wDry < sWet / 10) second = 'w'
  const warmMonths = temps.filter((t) => t >= 10).length * per
  let third
  if (hot >= 22) third = 'a'
  else if (warmMonths >= 4) third = 'b'
  else if (group === 'D' && cold < -38) third = 'd'
  else third = 'c'
  return group + second + third
}
