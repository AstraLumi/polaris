// The planet's grid and projections; mirrors backend/planetgrid_test.go.
// No dependencies: node frontend/tests/planetGrid.test.mjs
import assert from 'node:assert/strict'
import { PlanetGrid, fromLatLon, frequencyFor, cellsWithin } from '../src/planetGrid.js'
import { projectionOf } from '../src/projection.js'

const rad = (d) => (d * Math.PI) / 180

for (const n of [2, 3, 5, 8]) {
  const g = new PlanetGrid(n)
  let pentagons = 0
  for (let c = 0; c < g.count; c++) {
    assert.equal(g.cellAt(g.center(c)), c, `n=${n}: cell ${c} round trip`)
    const nb = g.neighbors(c)
    assert.ok(nb.length === 5 || nb.length === 6, `n=${n}: cell ${c} has ${nb.length} neighbours`)
    if (nb.length === 5) pentagons++
    for (const o of nb) assert.ok(g.neighbors(o).includes(c), `n=${n}: ${c} and ${o} both ways`)
    assert.equal(g.corners(c, nb).length, nb.length)
  }
  assert.equal(pentagons, 12, `n=${n}: twelve pentagons`)
}

// The same cells as the backend.
const g5 = new PlanetGrid(5)
for (const [lat, lon, cell] of [[90, 0, 0], [-90, 0, 1], [0, 0, 228], [45, 100, 34], [-30, -120, 181], [10, 179, 70]]) {
  assert.equal(g5.cellAt(fromLatLon(rad(lat), rad(lon))), cell, `${lat}°, ${lon}°`)
}
assert.equal(frequencyFor(40075, 50), 141)
assert.equal(cellsWithin(g5, 100, 1).length, 7)
assert.equal(cellsWithin(g5, 100, 2).length, 19)

for (const id of ['equirectangular', 'mercator', 'mollweide', 'robinson']) {
  const p = projectionOf(id)
  for (const [lon, lat] of [[0, 0], [1.2, 0.5], [-3, -1.1], [2.5, 1.3], [-0.4, 0.05]]) {
    const [x, y] = p.forward(lon, lat)
    const back = p.inverse(x, y)
    assert.ok(back && Math.abs(back[0] - lon) < 1e-6 && Math.abs(back[1] - lat) < 1e-6, `${id}: ${lon}, ${lat}`)
  }
  assert.equal(p.inverse(10, 0), null, `${id}: off the map`)
}
console.log('planetGrid: ok')
