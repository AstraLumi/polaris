// Where an ocean's name lies on the globe.
// No dependencies: node frontend/tests/oceanNames.test.mjs
import assert from 'node:assert/strict'
import { PlanetGrid, fromLatLon } from '../src/planetGrid.js'
import { nameFrame, framePoint, placeOceanName } from '../src/oceanNames.js'

const rad = (d) => (d * Math.PI) / 180
const dot = (a, b) => a[0] * b[0] + a[1] * b[1] + a[2] * b[2]
const near = (a, b, eps = 1e-9) => a.every((x, k) => Math.abs(x - b[k]) < eps)

// A level name runs along the line of latitude, up toward the north.
{
  const at = fromLatLon(rad(20), rad(40))
  const f = nameFrame(at, 0)
  assert.ok(near(framePoint(f, 0, 0), at), 'the middle of the name')
  const east = framePoint(f, rad(10) * Math.cos(rad(20)), 0)
  assert.ok(near(east, fromLatLon(rad(20), rad(50))), 'along the latitude, eastward')
  assert.ok(near(framePoint(f, 0, rad(5)), fromLatLon(rad(25), rad(40))), 'up is north')
}

// Near a pole the line bends no more than at 50°, and a tilted line
// still runs through the middle.
{
  const at = fromLatLon(rad(85), rad(10))
  const f = nameFrame(at, 0)
  assert.ok(Math.abs(f.lat0) <= rad(50) + 1e-9)
  assert.ok(near(framePoint(f, 0, 0), at))
  const g = nameFrame(fromLatLon(rad(-10), rad(100)), rad(30))
  assert.ok(near(framePoint(g, 0, 0), fromLatLon(rad(-10), rad(100))))
  // Every point is on the sphere.
  for (const p of [framePoint(g, 0.3, 0.05), framePoint(f, -0.2, -0.04)]) assert.ok(Math.abs(dot(p, p) - 1) < 1e-9)
}

// The cells within some angle of a point.
const grid = new PlanetGrid(40)
const cap = (centre, deg) => {
  const out = []
  for (let c = 0; c < grid.count; c++) if (dot(grid.center(c), centre) > Math.cos(rad(deg))) out.push(c)
  return out
}

// A round ocean: the name sits at its middle, level, and wholly on the water.
{
  const mid = fromLatLon(rad(10), rad(30))
  const cells = cap(mid, 30)
  const spot = placeOceanName(grid, cells, 5)
  assert.ok(spot, 'a name fits')
  assert.equal(spot.tilt, 0)
  assert.ok(dot(spot.at, mid) > Math.cos(rad(3)), 'near the middle')
  const inside = new Set(cells)
  const f = nameFrame(spot.at, spot.tilt)
  for (let k = 0; k <= 20; k++) {
    for (const v of [-0.5, 0, 0.5]) {
      const p = framePoint(f, (k / 20 - 0.5) * spot.length, v * spot.height)
      assert.ok(inside.has(grid.cellAt(p)), 'on the water')
    }
  }
}

// An ocean whose middle is land (a ring): the name moves onto the water.
{
  const mid = fromLatLon(0, 0)
  const land = new Set(cap(mid, 15))
  const cells = cap(mid, 45).filter((c) => !land.has(c))
  const spot = placeOceanName(grid, cells, 4)
  assert.ok(spot)
  assert.ok(dot(spot.at, mid) < Math.cos(rad(15)), 'off the land')
}

// Too small for any name.
assert.equal(placeOceanName(grid, cap(fromLatLon(0, 0), 2), 6), null)
assert.equal(placeOceanName(grid, [], 3), null)

console.log('oceanNames: ok')
