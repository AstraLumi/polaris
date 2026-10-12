// Latitudes and climate zones from the equator; mirrors TestLatitude in
// backend/seasons_test.go.
// No dependencies: node frontend/tests/planet.test.mjs
import assert from 'node:assert/strict'
import { rowsToPole, latitudeOf, zoneOf, formatLatitude, zoneLines, latitudeRow } from '../src/planet.js'

const eq = { row: 100, center: 0, bend: 0, hex_km: 50, planet_km: 40075, tilt: 23.44 }
const near = (a, b) => Math.abs(a - b) < 0.1

assert.ok(near(rowsToPole(eq), 231.4), 'about 231 rows from the equator to a pole')
for (const [q, r, lat, zone] of [
  [-50, 100, 0, 'Tropical'],
  [-50, 0, 38.9, 'Temperate'],
  [-120, 240, -54.5, 'Temperate'],
  [-200, -100, 77.8, 'Polar'],
  [0, -1000, 90, 'Polar'],
]) {
  const got = latitudeOf(eq, q, r)
  assert.ok(near(got, lat), `hex ${q},${r}: ${got}`)
  assert.equal(zoneOf(eq, got), zone, `hex ${q},${r} zone`)
}
assert.ok(latitudeOf({ ...eq, bend: 10 }, 100, 100) > 0, 'a bent equator is lower away from its centre')
assert.ok(near(latitudeRow(eq, 0, 90), 100 - rowsToPole(eq)), 'the pole is rowsToPole above')

assert.equal(formatLatitude(38.9), '39°N')
assert.equal(formatLatitude(-4.4), '4°S')
assert.equal(formatLatitude(0.2), '0°')

assert.deepEqual(zoneLines(eq).map((l) => l.lat), [23.44, -23.44, 66.56, -66.56])
assert.equal(zoneLines({ ...eq, tilt: 45 }).length, 2, 'tropic and polar circle meet')
assert.equal(zoneLines({ ...eq, tilt: 0 }).length, 0, 'no tilt, no zones')
console.log('planet: ok')
