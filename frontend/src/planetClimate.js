import { ref, shallowRef } from 'vue'
import { biomeOf, PLAIN_LAND, groundHeight } from './biomes'
import { PlanetGrid, latLon } from './planetGrid'
import { temperatureAt, precipitationAt, windAt, koppen, gridGeometry, sampleVector, fineField } from './climateModel.js'
import { flowLines } from './flowLines.js'
import { mainLighting } from './lighting.js'

// The main planet's climate on the Map page: what the model
// (climateModel.js) is given from the map, the continents and the biomes,
// running it in a worker (climate.worker.js), and reading its results on
// the map's hexes. Nothing here is stored: the climate follows from the
// map and is worked out again when the map changes.

// ---- What the model is given ---------------------------------------------------------------
// isLand(cell) and tagAt(kind, cell) -> { key, height } | null (kind
// 'biome' or 'relief', planetLayers.js) come from the page. Answers the
// model's input, and the ground's height on every cell (for the
// temperature up high).
export function climateInput({ frequency, isLand, tagAt, tilt, dayHours, months, daysPerMonth, planetKm, insolation, evenHeat = 0 }) {
  const count = new PlanetGrid(frequency).count
  const land = new Uint8Array(count)
  const veg = new Float32Array(count)
  const albedo = new Float32Array(count)
  const height = new Float32Array(count)
  const water = new Uint8Array(count)
  const geothermal = new Float32Array(count)
  for (let c = 0; c < count; c++) {
    if (!isLand(c)) continue
    const painted = tagAt('biome', c)
    const relief = tagAt('relief', c)
    const b = (painted && biomeOf(painted.key)) || PLAIN_LAND
    land[c] = 1
    veg[c] = b.veg
    albedo[c] = b.albedo
    height[c] = Math.max(0, groundHeight(painted, relief))
    water[c] = b.water ? 1 : 0
    geothermal[c] = b.heat ?? 0
  }
  return {
    frequency,
    land,
    veg,
    albedo,
    height,
    water,
    geothermal,
    tilt,
    dayHours: dayHours || 24,
    steps: months,
    insolation,
    evenHeat,
    radiusKm: planetKm / (2 * Math.PI),
    monthDays: (daysPerMonth * (dayHours || 24)) / 24,
  }
}

// A short fingerprint of the input, to know when it changed.
export function inputKey(input) {
  let h = 2166136261
  const eat = (x) => {
    h ^= x
    h = Math.imul(h, 16777619)
  }
  for (const arr of [input.land, input.water]) for (let i = 0; i < arr.length; i++) eat(arr[i])
  for (const arr of [input.veg, input.height, input.geothermal]) for (let i = 0; i < arr.length; i += 1) eat(Math.round(arr[i] * 100))
  return [input.frequency, input.tilt, input.dayHours, input.steps, input.insolation.toFixed(4), input.evenHeat.toFixed(4), input.radiusKm.toFixed(1), input.monthDays.toFixed(3), h >>> 0].join('|')
}

// How much heat the main planet gets, relative to Earth's sunlight, from
// the solar system (GET /api/solar) through the lighting model
// (lighting.js): the star's, directional (a Dyson sphere's still comes
// through its vents), and any even warmth of an enclosing shell. A planet
// warmed by nothing (no star, a black hole) is given Earth's, and
// `assumed` says so.
export function heatOf(solar) {
  const lit = mainLighting(solar)
  const star = lit?.heat.star ?? 0
  const even = lit?.heat.even ?? 0
  if (!(star > 0) && !(even > 0)) return { insolation: 1, evenHeat: 0, assumed: true }
  return { insolation: star, evenHeat: even, assumed: false }
}

// ---- Running it ---------------------------------------------------------------------------
export function usePlanetClimate() {
  const working = ref(false)
  const error = ref('')
  const view = shallowRef(null) // a ClimateView, once worked out
  let worker = null
  let lastKey = ''
  let latest = 0

  function run(input, heights) {
    const key = inputKey(input)
    if (key === lastKey && view.value) return
    lastKey = key
    const id = ++latest
    working.value = true
    error.value = ''
    if (!worker) {
      worker = new Worker(new URL('./climate.worker.js', import.meta.url), { type: 'module' })
      worker.onmessage = (e) => {
        if (e.data.id !== latest) return
        working.value = false
        if (e.data.error) {
          error.value = e.data.error
          lastKey = ''
          return
        }
        view.value = new ClimateView(e.data.result, heights)
      }
      worker.onerror = (e) => {
        working.value = false
        error.value = e.message || 'the climate could not be worked out'
        lastKey = ''
      }
    }
    worker.postMessage({ id, input })
  }

  function dispose() {
    worker?.terminate()
    worker = null
  }

  return { working, error, view, run, dispose }
}

// ---- Reading the results -------------------------------------------------------------------
// A worked-out climate on the map's hexes. step: a month's index, or null
// for the whole year. Everything is computed once and kept.
export class ClimateView {
  constructor(result, heights) {
    this.result = result
    this.heights = heights
    this.steps = result.steps
    this.cache = new Map()
    this.coarseGeometry = null
  }

  once(key, make) {
    if (!this.cache.has(key)) this.cache.set(key, make())
    return this.cache.get(key)
  }

  // °C on every hex.
  temperature(step) {
    return this.once('t' + step, () => temperatureAt(this.result, this.heights, step))
  }

  // How humid the air is on every hex, 0 to 1 (relative humidity), in the
  // month or on average over the year.
  humidity(step) {
    return this.once('h' + step, () => {
      const { steps, coarse } = this.result
      if (step != null) return fineField(this.result, coarse.humidity.subarray(step * coarse.count, (step + 1) * coarse.count))
      const mean = new Float32Array(coarse.count)
      for (let s = 0; s < steps; s++) for (let c = 0; c < coarse.count; c++) mean[c] += coarse.humidity[s * coarse.count + c] / steps
      return fineField(this.result, mean)
    })
  }

  // mm on every hex, in the month (or the year).
  rain(step) {
    return this.once('p' + step, () => precipitationAt(this.result, step))
  }

  // The coldest and warmest months' temperatures on every hex.
  range() {
    return this.once('range', () => {
      const n = this.temperature(null).length
      const low = new Float32Array(n).fill(Infinity)
      const high = new Float32Array(n).fill(-Infinity)
      for (let k = 0; k < this.steps; k++) {
        const t = this.temperature(k)
        for (let c = 0; c < n; c++) {
          if (t[c] < low[c]) low[c] = t[c]
          if (t[c] > high[c]) high[c] = t[c]
        }
      }
      return { low, high }
    })
  }

  // The Köppen type of a hex (land only; '' at sea).
  koppenOf(cell, land, grid) {
    const codes = this.once('koppen', () => new Map())
    if (!land) return ''
    if (!codes.has(cell)) {
      const temps = []
      const rains = []
      for (let k = 0; k < this.steps; k++) {
        temps.push(this.temperature(k)[cell])
        rains.push(this.rain(k)[cell])
      }
      codes.set(cell, koppen(temps, rains, latLon(grid.center(cell))[0] >= 0))
    }
    return codes.get(cell)
  }

  geometry() {
    if (!this.coarseGeometry) this.coarseGeometry = gridGeometry(new PlanetGrid(this.result.coarse.frequency))
    return this.coarseGeometry
  }

  wind(step) {
    return this.once('w' + step, () => windAt(this.result, step))
  }

  get current() {
    return this.result.coarse.current
  }

  // A vector field (wind or current) at a point of the sphere: its
  // eastward and northward parts, m/s.
  vectorAt(field, p) {
    const v = sampleVector(this.geometry(), field, p)
    const ex = -p[1]
    const ey = p[0]
    const el = Math.hypot(ex, ey) || 1
    const east = (v[0] * ex + v[1] * ey) / el
    const nx = (-p[2] * ey) / el
    const ny = (p[2] * ex) / el
    const nz = (p[0] * ey - p[1] * ex) / el
    return { east, north: v[0] * nx + v[1] * ny + v[2] * nz }
  }

  // The arrows to draw: winds of the step, coloured by speed; currents,
  // warm (flowing from warmer water) or cold.
  windArrows(step) {
    return this.once('wa' + step, () =>
      flowLines(this.geometry(), this.wind(step), { seeds: 14, fullSpeed: 12, minSpeed: 0.6 }).map((l) => ({
        points: l.points,
        color: l.speed > 12 ? '#fff4c2' : l.speed > 6 ? '#ffffff' : '#d6e6f5',
      })),
    )
  }

  currentArrows(step) {
    return this.once('ca' + step, () => {
      const g = this.geometry()
      const { sea, temp, count } = this.result.coarse
      // The water's temperature against its latitude's, at sea.
      const t = new Float64Array(count)
      const steps = step == null ? [...Array(this.steps).keys()] : [step]
      for (const k of steps) for (let c = 0; c < count; c++) t[c] += temp[k * count + c] / steps.length
      const bands = new Map()
      for (let c = 0; c < count; c++) {
        if (!sea[c]) continue
        const b = Math.round((g.lat[c] * 180) / Math.PI / 4)
        const s = bands.get(b) ?? [0, 0]
        s[0] += t[c]
        s[1]++
        bands.set(b, s)
      }
      const anomalyAt = (p) => {
        const c = g.grid.cellAt(p)
        const s = bands.get(Math.round((g.lat[c] * 180) / Math.PI / 4))
        return s ? t[c] - s[0] / s[1] : 0
      }
      const isSea = (p) => sea[g.grid.cellAt(p)] === 1
      return flowLines(g, this.current, { seeds: 15, fullSpeed: 0.5, minSpeed: 0.05, keep: isSea }).map((l) => {
        // Warm when it carries water towards the pole, cold when towards the
        // equator; along a parallel, by the water's own warmth.
        const tail = l.points[0]
        const head = l.points[l.points.length - 1]
        const arc = Math.acos(Math.min(1, tail[0] * head[0] + tail[1] * head[1] + tail[2] * head[2])) || 1
        const poleward = (Math.abs(head[2]) - Math.abs(tail[2])) / arc
        const warmth = Math.abs(poleward) > 0.3 ? poleward : anomalyAt(l.seed) / 3
        return {
          points: l.points,
          color: warmth > 0.15 ? '#ff8a5c' : warmth < -0.15 ? '#6cc8ff' : '#e9eef5',
          width: 0.7 + 0.9 * Math.min(1, l.speed / 0.6),
        }
      })
    })
  }
}
