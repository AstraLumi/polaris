// Arrows along a flow on the sphere (src/flowLines.js). No dependencies:
// node frontend/tests/flowLines.test.mjs
import assert from 'node:assert/strict'
import { PlanetGrid } from '../src/planetGrid.js'
import { gridGeometry } from '../src/climateModel.js'
import { flowLines } from '../src/flowLines.js'

const g = gridGeometry(new PlanetGrid(10))
const dot = (a, b) => a[0] * b[0] + a[1] * b[1] + a[2] * b[2]

// A wind blowing east everywhere at a given speed.
function eastwind(speed) {
  const f = new Float32Array(g.count * 3)
  for (let c = 0; c < g.count; c++) f.set([g.east[c * 3] * speed, g.east[c * 3 + 1] * speed, 0], c * 3)
  return f
}

{
  const lines = flowLines(g, eastwind(10), { seeds: 6, fullSpeed: 10, minSpeed: 0.5 })
  assert.ok(lines.length > 100)
  for (const { points } of lines) {
    // On the sphere, and heading east (the head is east of the tail).
    for (const p of points) assert.ok(Math.abs(Math.hypot(...p) - 1) < 1e-9)
    const tail = points[0]
    const head = points[points.length - 1]
    const lonT = Math.atan2(tail[1], tail[0])
    const lonH = Math.atan2(head[1], head[0])
    let d = lonH - lonT
    if (d > Math.PI) d -= 2 * Math.PI
    if (d < -Math.PI) d += 2 * Math.PI
    if (Math.abs(tail[2]) < 0.9) assert.ok(d > 0, 'heads east')
  }
  // Slower winds draw shorter lines; still air draws none.
  const len = (ls) => ls.reduce((s, l) => s + Math.acos(Math.min(1, dot(l.points[0], l.points.at(-1)))), 0) / ls.length
  const slow = flowLines(g, eastwind(4), { seeds: 6, fullSpeed: 10, minSpeed: 0.5 })
  assert.ok(len(slow) < len(lines) * 0.6)
  assert.equal(flowLines(g, eastwind(0), { seeds: 6 }).length, 0)
}

// keep() decides where lines may start.
{
  const north = flowLines(g, eastwind(10), { seeds: 6, keep: (p) => p[2] > 0 })
  assert.ok(north.length > 0)
  for (const l of north) assert.ok(l.seed[2] > 0)
}

console.log('flowLines: all tests passed')
