// The climate worked out from the map (src/climateModel.js), checked on a
// rough Earth (earthLike.mjs): the broad patterns an atlas shows must come
// out of the model, within generous bounds. No dependencies:
// node frontend/tests/climateModel.test.mjs
import assert from 'node:assert/strict'
import { PlanetGrid, latLon } from '../src/planetGrid.js'
import {
  simulateClimate,
  temperatureAt,
  precipitationAt,
  windAt,
  koppen,
  gridGeometry,
  sampleVector,
  locate,
  insolation,
  declination,
  circulationCells,
} from '../src/climateModel.js'
import { earthLike, cellAtDeg } from './earthLike.mjs'

const DEG = Math.PI / 180

// ---- Sunlight -----------------------------------------------------------------------
{
  // At the equinox the equator gets S/π; the pole in its night none, in its
  // midsummer day more than the equator.
  assert.ok(Math.abs(insolation(0, 0, 1361) - 1361 / Math.PI) < 1e-6)
  const june = 23.44 * DEG
  assert.equal(insolation(-89 * DEG, june), 0)
  assert.ok(insolation(89 * DEG, june) > insolation(0, june))
  // The year begins after the northern winter solstice: the sun is south,
  // and halfway through the year it is north.
  assert.ok(declination(23.44, 0) < -22 * DEG)
  assert.ok(declination(23.44, 0.5) > 22 * DEG)
  assert.equal(declination(0, 0.3), 0)
}

// ---- Circulation cells -----------------------------------------------------------------
assert.equal(circulationCells(24), 3)
assert.equal(circulationCells(24 * 100), 1)
assert.ok(circulationCells(10) > 3)

// ---- Reading between cells ---------------------------------------------------------------
{
  const g = gridGeometry(new PlanetGrid(6))
  // Weights sum to one, and a cell's centre is read as that cell.
  for (const c of [0, 1, 17, 200, g.count - 1]) {
    const p = g.grid.center(c)
    const [a, b, d, wa, wb, wd] = locate(g, p)
    assert.ok(Math.abs(wa + wb + wd - 1) < 1e-9)
    const weightOf = new Map([[a, 0], [b, 0], [d, 0]])
    weightOf.set(a, weightOf.get(a) + wa)
    weightOf.set(b, weightOf.get(b) + wb)
    weightOf.set(d, weightOf.get(d) + wd)
    assert.ok((weightOf.get(c) ?? 0) > 0.99, `cell ${c} reads itself`)
  }
  // A field that is the same everywhere reads the same everywhere (in the
  // tangent plane).
  const field = new Float32Array(g.count * 3)
  for (let c = 0; c < g.count; c++) field.set([0, 0, 1], c * 3)
  const p = [Math.cos(0.3), 0, Math.sin(0.3)]
  const v = sampleVector(g, field, p)
  assert.ok(Math.abs(v[0] * p[0] + v[1] * p[1] + v[2] * p[2]) < 1e-6, 'tangent')
}

// ---- Köppen ---------------------------------------------------------------------------------
{
  const flat = (x) => Array(12).fill(x)
  assert.equal(koppen(flat(27), flat(250)), 'Af')
  assert.equal(koppen(flat(-30), flat(10)), 'EF')
  assert.equal(koppen([28, 30, 33, 35, 36, 35, 34, 33, 32, 30, 28, 27], flat(1)), 'BWh')
  // Mild, wet all year, warmest month under 22 °C: oceanic.
  assert.equal(koppen([5, 6, 8, 10, 13, 16, 18, 18, 15, 11, 8, 6], flat(70)), 'Cfb')
  // Dry summers in the north: Mediterranean.
  assert.equal(koppen([10, 11, 13, 16, 20, 24, 27, 27, 24, 19, 14, 11], [90, 80, 60, 40, 20, 5, 1, 2, 15, 50, 80, 90]), 'Csa')
  // A year of any length: six months, the same climate.
  assert.equal(koppen([27, 27, 27, 27, 27, 27], [250, 250, 250, 250, 250, 250]), 'Af')
}

// ---- A rough Earth ------------------------------------------------------------------------------
const F = 24
const map = earthLike(F)
const started = Date.now()
const r = simulateClimate({ frequency: F, ...map, tilt: 23.44, dayHours: 24, steps: 12 })
const took = Date.now() - started
assert.ok(took < 20000, `took ${took} ms`)
const g = map.grid
const cg = gridGeometry(new PlanetGrid(r.coarse.frequency))
const yearT = temperatureAt(r, map.height)
const yearP = precipitationAt(r)
const at = (lon, lat) => cellAtDeg(g, lon, lat)
const monthT = (k) => temperatureAt(r, map.height, k)
const jan = monthT(0)
const jul = monthT(6)

// Every number is a number.
for (const x of [...yearT, ...yearP, ...r.coarse.wind, ...r.coarse.current]) assert.ok(Number.isFinite(x))

// The planet's mean is Earth-like, the tropics warm and the poles frozen.
let mean = 0
for (const t of yearT) mean += t / yearT.length
assert.ok(mean > 8 && mean < 22, `mean ${mean}`)
assert.ok(yearT[at(-150, 0)] > 24, 'equatorial Pacific is warm')
assert.ok(yearT[at(0, 89)] < -5, 'the north pole is frozen')
assert.ok(yearT[at(0, -89)] < -20, 'the south pole is colder still (high ice)')

// Inland winters are harsh, sea-side ones mild; summers the other way.
const siberia = at(110, 62)
assert.ok(jul[siberia] - jan[siberia] > 25, `Siberia's seasons: ${jan[siberia]} / ${jul[siberia]}`)
const pacific = at(-160, 45)
assert.ok(Math.abs(jul[pacific] - jan[pacific]) < 10, 'the open sea changes little')
// The south's summer is in January.
const aus = at(133, -25)
assert.ok(jan[aus] > jul[aus] + 8, 'Australia is hot in January')

// Mountains are cold.
assert.ok(yearT[at(88, 33)] < yearT[at(80, 22)] - 15, 'Tibet is colder than India')

// Winds: trades from the east in the tropics, westerlies in mid-latitudes,
// each hemisphere.
function eastward(field, lon, lat) {
  const p = g.center(at(lon, lat))
  const v = sampleVector(cg, field, p)
  const e = [-p[1], p[0], 0]
  const len = Math.hypot(e[0], e[1])
  return (v[0] * e[0] + v[1] * e[1]) / len
}
function northward(field, lon, lat) {
  const p = g.center(at(lon, lat))
  const v = sampleVector(cg, field, p)
  const e = [-p[1] / Math.hypot(p[0], p[1]), p[0] / Math.hypot(p[0], p[1]), 0]
  const n = [-p[2] * e[1], p[2] * e[0], p[0] * e[1] - p[1] * e[0]]
  return v[0] * n[0] + v[1] * n[1] + v[2] * n[2]
}
const windYear = windAt(r)
assert.ok(eastward(windYear, -150, 15) < -2, 'north-east trades')
assert.ok(eastward(windYear, -120, -15) < -2, 'south-east trades')
assert.ok(eastward(windYear, -160, 47) > 3, 'northern westerlies')
assert.ok(eastward(windYear, -120, -48) > 3, 'southern westerlies (roaring forties)')
// Monsoon: India's wind turns from the land in winter to the sea in summer.
assert.ok(eastward(windAt(r, 0), 73, 19) < -3 && eastward(windAt(r, 6), 73, 19) > 0, 'the Indian monsoon reverses')

// Ocean currents: the gyres turn clockwise in the north (warm currents up
// the western side, cold down the eastern), anticlockwise in the south.
const cur = r.coarse.current
assert.ok(northward(cur, -70, 34) > 0.1, 'Gulf Stream goes north')
assert.ok(northward(cur, -126, 35) < -0.1, 'California Current goes south')
assert.ok(northward(cur, -40, -25) < -0.1, 'Brazil Current goes south')
assert.ok(northward(cur, 10, -25) > 0.1, 'Benguela Current goes north')
assert.ok(eastward(cur, -150, 12) < -0.3, 'North Equatorial Current goes west')
// Nothing flows on land.
for (let c = 0; c < r.coarse.count; c++) if (!r.coarse.sea[c]) assert.equal(Math.hypot(cur[c * 3], cur[c * 3 + 1], cur[c * 3 + 2]), 0)

// Rain: wet tropics and westerly coasts, dry subtropical deserts and poles.
assert.ok(yearP[at(-60, -3)] > 1000, `Amazon ${yearP[at(-60, -3)]}`)
assert.ok(yearP[at(10, 23)] < 300, `Sahara ${yearP[at(10, 23)]}`)
assert.ok(yearP[at(133, -25)] < 300, `Australian interior ${yearP[at(133, -25)]}`)
assert.ok(yearP[at(0, -85)] < 100, 'Antarctica is a desert')
assert.ok(yearP[at(-150, 2)] > 1500, 'the rain belt at the equator')
let globalP = 0
for (const p of yearP) globalP += p / yearP.length
assert.ok(globalP > 600 && globalP < 1800, `global rain ${globalP}`)
// The monsoon brings India's rain in summer.
const mumbai = at(73, 19)
assert.ok(precipitationAt(r, 6)[mumbai] > 5 * precipitationAt(r, 0)[mumbai] + 20, 'Mumbai is wet in July, dry in January')

// Plants make rain: the same map with the Amazon's forest cleared to desert
// rains less there.
{
  const bare = earthLike(F)
  for (let c = 0; c < g.count; c++) if (bare.veg[c] === 1) bare.veg[c] = 0.02
  const r2 = simulateClimate({ frequency: F, ...bare, tilt: 23.44, dayHours: 24, steps: 12 })
  const amazon = at(-62, -5)
  assert.ok(precipitationAt(r2)[amazon] < precipitationAt(r)[amazon] * 0.8, 'cleared forest, less rain')
}

// Deterministic: the same map gives the same climate.
{
  const again = simulateClimate({ frequency: F, ...map, tilt: 23.44, dayHours: 24, steps: 12 })
  assert.deepEqual(again.coarse.temp, r.coarse.temp)
}

// No tilt: no seasons.
{
  const flat = simulateClimate({ frequency: 12, ...earthLike(12), tilt: 0, dayHours: 24, steps: 12 })
  const t0 = temperatureAt(flat, null, 0)
  const t6 = temperatureAt(flat, null, 6)
  let most = 0
  for (let c = 0; c < t0.length; c++) most = Math.max(most, Math.abs(t0[c] - t6[c]))
  assert.ok(most < 2, `no seasons without tilt (${most})`)
}

// Heat from all round (an enclosing shell, lighting.js) warms every
// latitude alike and brings no seasons of its own; with no star's heat at
// all, it alone keeps the planet from freezing, evenly.
{
  const small = earthLike(12)
  const mean = (t) => t.reduce((x, y) => x + y, 0) / t.length
  const plain = simulateClimate({ frequency: 12, ...small, steps: 4 })
  const warmed = simulateClimate({ frequency: 12, ...small, steps: 4, evenHeat: 0.05 })
  assert.ok(mean(temperatureAt(warmed, null)) > mean(temperatureAt(plain, null)) + 3, 'even heat warms')
  const shellOnly = simulateClimate({ frequency: 12, ...small, tilt: 23.44, steps: 4, insolation: 0, evenHeat: 0.3 })
  const t = temperatureAt(shellOnly, null)
  const pole = t[cellAtDeg(small.grid, 0, -85)]
  const equator = t[cellAtDeg(small.grid, -150, 0)]
  assert.ok(Math.abs(pole - equator) < 15, `no star: poles near the equator's warmth (${pole} / ${equator})`)
  const t0 = temperatureAt(shellOnly, null, 0)
  const t2 = temperatureAt(shellOnly, null, 2)
  let most = 0
  for (let c = 0; c < t0.length; c++) most = Math.max(most, Math.abs(t0[c] - t2[c]))
  assert.ok(most < 2, `no star: no seasons (${most})`)
}

// High ground steers the wind. A continent across the westerlies with a
// thin north–south range down its middle (narrower than the climate's
// cells) and a pass through it: the wind turns to run along the range,
// speeds up through the pass, rains on the windward side and leaves the
// lee dry. The same land without the range is the yardstick.
{
  const F2 = 48
  function walled(wall) {
    const grid = new PlanetGrid(F2)
    const n = grid.count
    const map = { grid, land: new Uint8Array(n), veg: new Float32Array(n).fill(0.5), albedo: new Float32Array(n).fill(0.2), height: new Float32Array(n), water: new Uint8Array(n) }
    for (let c = 0; c < n; c++) {
      const [la, lo] = latLon(grid.center(c))
      const lat = la / DEG
      const lon = lo / DEG
      if (lon <= -30 || lon >= 30 || lat <= -50 || lat >= 70) continue
      map.land[c] = 1
      map.height[c] = wall && lon > -3 && lon < 3 && !(lat > 40 && lat < 47) ? 4000 : 300
    }
    return { map, r: simulateClimate({ frequency: F2, ...map, steps: 4 }) }
  }
  const flat = walled(false)
  const wall = walled(true)
  const g2 = gridGeometry(new PlanetGrid(wall.r.coarse.frequency))
  const windOf = (run, lon, lat) => {
    const p = [Math.cos(lat * DEG) * Math.cos(lon * DEG), Math.cos(lat * DEG) * Math.sin(lon * DEG), Math.sin(lat * DEG)]
    const v = sampleVector(g2, windAt(run.r), p)
    const e = [-p[1] / Math.hypot(p[0], p[1]), p[0] / Math.hypot(p[0], p[1]), 0]
    const nn = [-p[2] * e[1], p[2] * e[0], p[0] * e[1] - p[1] * e[0]]
    const east = v[0] * e[0] + v[1] * e[1]
    const north = v[0] * nn[0] + v[1] * nn[1] + v[2] * nn[2]
    return { east, north, speed: Math.hypot(east, north) }
  }
  const rainOf = (run, lon, lat) => precipitationAt(run.r)[cellAtDeg(run.map.grid, lon, lat)]

  const before = windOf(wall, -6, 52)
  const open = windOf(flat, -6, 52)
  assert.ok(before.east < 0.3 * open.east, `the westerlies stop at the range (${before.east} vs ${open.east})`)
  assert.ok(Math.abs(before.north) >= Math.abs(open.north), 'and turn to run along it')
  const pass = windOf(wall, 0, 43)
  const plain = windOf(flat, 0, 43)
  assert.ok(pass.speed > 1.15 * plain.speed, `faster through the pass (${pass.speed} vs ${plain.speed})`)
  assert.ok(rainOf(wall, -6, 52) > rainOf(flat, -6, 52), 'more rain on the windward side')
  assert.ok(rainOf(wall, 7, 52) < 0.5 * rainOf(flat, 7, 52), 'a rain shadow in the lee')
}

// What feeds back into the temperature and the winds.
{
  // Volcanic ground warms its own hexes, and the cell round them a little.
  const volcanic = earthLike(F)
  volcanic.geothermal = new Float32Array(volcanic.grid.count)
  const spot = []
  for (let c = 0; c < volcanic.grid.count; c++) {
    const [la, lo] = latLon(volcanic.grid.center(c))
    if (volcanic.land[c] && Math.abs(la / DEG - 50) < 5 && Math.abs(lo / DEG - 90) < 8) {
      volcanic.geothermal[c] = 40
      spot.push(c)
    }
  }
  assert.ok(spot.length > 3)
  const hot = simulateClimate({ frequency: F, ...volcanic, steps: 4 })
  const plainT = temperatureAt(simulateClimate({ frequency: F, ...earthLike(F), steps: 4 }), null)
  const hotT = temperatureAt(hot, null)
  const warmer = spot.reduce((sum, c) => sum + hotT[c] - plainT[c], 0) / spot.length
  assert.ok(warmer > 3, `volcanic land is warmer (${warmer} °C)`)

  // Dry air lets heat go: without water vapour's greenhouse the dry poles
  // would be warmer, the humid tropics cooler.
  const noVapour = simulateClimate({ frequency: F, ...map, tilt: 23.44, steps: 12, tune: { vapourGreenhouse: 0 } })
  const t0 = temperatureAt(noVapour, map.height)
  assert.ok(yearT[at(0, 89)] < t0[at(0, 89)] - 1, 'dry polar air is colder')
  assert.ok(yearT[at(-150, 0)] > t0[at(-150, 0)], 'humid tropics are warmer')

  // Katabatic winds: in the southern winter, cold air pours off the high
  // Antarctic ice towards the sea.
  const july = windAt(r, 6)
  let outward = 0
  for (let lon = -180; lon < 180; lon += 20) outward += northward(july, lon, -74) > 0 ? 1 : 0
  assert.ok(outward >= 12, `off the ice on ${outward} of 18 meridians`)
}

// A map finer than the climate's grid reads it smoothly.
{
  const fine = earthLike(48)
  const rf = simulateClimate({ frequency: 48, ...fine, tilt: 23.44, dayHours: 24, steps: 4 })
  assert.equal(rf.coarse.frequency, 24)
  const t = temperatureAt(rf, null)
  assert.equal(t.length, fine.grid.count)
  // Neighbours differ by little away from the coasts.
  let jumps = 0
  for (let c = 0; c < fine.grid.count; c += 37) for (const x of fine.grid.neighbors(c)) if (Math.abs(t[c] - t[x]) > 6) jumps++
  assert.ok(jumps < 10, `${jumps} jumps`)
}

console.log(`climateModel: all tests passed (${took} ms for the rough Earth)`)
