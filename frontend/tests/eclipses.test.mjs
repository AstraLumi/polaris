// Which bodies can shadow which (src/eclipses.js). No dependencies:
// node frontend/tests/eclipses.test.mjs
import assert from 'node:assert/strict'
import { shadowCasters, MAX_CASTERS } from '../src/eclipses.js'

// A star at the origin, a planet at x = 10 and its moon.
const scene = (moon) => ({ centers: [0, 0, 0, 10, 0, 0, ...moon], radii: [2, 0.5, 0.2], star: 0, lightRadius: 0.5 })

// The moon between the planet and the star: it shadows the planet, not
// the other way round.
let c = shadowCasters(scene([9, 0, 0]))
assert.deepEqual(c[1], [2], 'moon over the planet')
assert.deepEqual(c[2], [], 'nothing between the moon and the star')
assert.deepEqual(c[0], [], 'the star is never shadowed')

// Behind the planet: the planet shadows the moon (a lunar eclipse).
c = shadowCasters(scene([11, 0, 0]))
assert.deepEqual(c[2], [1])
assert.deepEqual(c[1], [])

// Off to the side, a quarter turn round: neither.
c = shadowCasters(scene([10, 3, 0]))
assert.deepEqual(c[1], [])
assert.deepEqual(c[2], [])

// Just off the line, the soft edge still reaches: counted.
c = shadowCasters(scene([9, 0.75, 0]))
assert.deepEqual(c[1], [2], 'the penumbra counts')

// Rings: a planet with wide rings catches shadows farther out.
c = shadowCasters({ ...scene([9, 1.1, 0]), reach: [2, 1.5, 0.2] })
assert.deepEqual(c[1], [2], 'on the rings')

// No more than MAX_CASTERS, the most central first.
const many = { centers: [0, 0, 0, 10, 0, 0], radii: [2, 0.5], star: 0, lightRadius: 0.5 }
for (let k = 0; k < 12; k++) {
  many.centers.push(9, (k - 6) * 0.05, 0)
  many.radii.push(0.1)
}
c = shadowCasters(many)
assert.equal(c[1].length, MAX_CASTERS)
assert.equal(c[1][0], 2 + 6, 'the one on the line first')
console.log('eclipses: ok')
