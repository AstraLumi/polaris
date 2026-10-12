// The map's bucket tool (src/bucketFill.js).
// No dependencies: node frontend/tests/bucketFill.test.mjs
import assert from 'node:assert/strict'
import { floodFill } from '../src/bucketFill.js'

// A 5×5 square grid (keys "x,y") with a wall down column 2 and one painted
// hex at 0,4.
const full = new Set(['0,4'])
for (let y = 0; y < 5; y++) full.add(`2,${y}`)
const neighbors = (k) => {
  const [x, y] = k.split(',').map(Number)
  return [[x + 1, y], [x - 1, y], [x, y + 1], [x, y - 1]].map(([a, b]) => `${a},${b}`)
}
const inside = (k) => {
  const [x, y] = k.split(',').map(Number)
  return x >= 0 && x < 5 && y >= 0 && y < 5
}
const empty = (k) => inside(k) && !full.has(k)

let r = floodFill('0,0', { neighbors, empty })
assert.equal(r.cells.length, 9, 'the empty hexes left of the wall')
assert.ok(!r.cells.includes('0,4'), 'a painted hex is not filled')
assert.ok(!r.cells.includes('3,0'), 'stops at the wall')

r = floodFill('2,3', { neighbors, empty })
assert.equal(r.cells.length, 0, 'a painted hex fills nothing')

r = floodFill('0,0', { neighbors, empty, max: 6 })
assert.ok(r.tooBig && !r.cells.length, 'too many hexes')

// Without the edge the area would run on forever; escapes stops it.
r = floodFill('0,0', { neighbors, empty: (k) => !full.has(k), escapes: (k) => !inside(k) })
assert.ok(r.open && !r.cells.length, 'an area that is not closed in')

console.log('bucketFill: ok')
