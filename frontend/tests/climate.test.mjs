// The planets' clouds (src/climate.js).
// No dependencies: node frontend/tests/climate.test.mjs
import assert from 'node:assert/strict'
import { cloudCoverage, genericCloudCover, mainPlanetCloudCover } from '../src/climate.js'
import { PlanetGrid } from '../src/planetGrid.js'

// Coverage follows the type; moons and gas giants get no cloud shell.
assert.ok(cloudCoverage({ kind: 'planet', body_type: 'ocean' }) > cloudCoverage({ kind: 'planet', body_type: 'rocky' }))
assert.equal(cloudCoverage({ kind: 'planet', body_type: 'gas' }), 0)
assert.equal(cloudCoverage({ kind: 'moon', body_type: 'rocky' }), 0)

// Cover is 0..1, the same for the same point and seed, and a cloudier
// type covers more of the sphere.
const grid = new PlanetGrid(12)
const share = (coverage) => {
  let n = 0
  for (let c = 0; c < grid.count; c++) {
    const v = genericCloudCover(grid.center(c), coverage, 3)
    assert.ok(v >= 0 && v <= 1)
    n += v > 0.5 ? 1 : 0
  }
  return n / grid.count
}
assert.equal(genericCloudCover([0.6, 0.8, 0], 0.5, 3), genericCloudCover([0.6, 0.8, 0], 0.5, 3))
const rocky = share(0.45)
const ocean = share(0.65)
assert.ok(ocean > rocky && rocky > 0.05 && ocean < 0.95, `rocky ${rocky}, ocean ${ocean}`)

// The main planet's hook: one value per hex.
const cover = mainPlanetCloudCover({ grid, body: { kind: 'planet', body_type: 'rocky' } })
for (const c of [0, 1, 100, grid.count - 1]) assert.ok(cover(c) >= 0 && cover(c) <= 1)
console.log('climate: ok')
