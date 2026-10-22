// Arrows along a flow (winds, ocean currents) on the sphere: from seed
// points spread evenly over the planet, each line follows the field a
// little way, curving with it, its length showing the speed. Drawing them is
// three/flowArrows.js's job; this only finds where they go.
// No dependencies but planetGrid.js and climateModel.js:
// node frontend/tests/flowLines.test.mjs
import { PlanetGrid } from './planetGrid.js'
import { sampleVector } from './climateModel.js'

// g: the field's grid geometry (climateModel.gridGeometry); field: 3 numbers
// per cell (m/s). Options:
//   seeds      frequency of the grid whose cell centres start the lines
//   fullSpeed  the speed (m/s) whose line is as long as the gap between seeds
//   minSpeed   slower than this, no line
//   keep(p)    whether a seed point may start a line (e.g. only at sea)
// Answers [{ points: [[x, y, z], …] from tail to head, speed, seed }].
export function flowLines(g, field, { seeds = 12, fullSpeed = 10, minSpeed = 0.5, steps = 8, keep = null } = {}) {
  const seedGrid = new PlanetGrid(seeds)
  const gap = Math.atan(2) / seeds // the angle between neighbouring seeds
  const out = []
  const v = [0, 0, 0]
  for (let s = 0; s < seedGrid.count; s++) {
    const p0 = seedGrid.center(s)
    if (keep && !keep(p0)) continue
    sampleVector(g, field, p0, v)
    const speed = Math.hypot(v[0], v[1], v[2])
    if (!(speed >= minSpeed)) continue
    const length = gap * 0.85 * Math.min(1, Math.max(0.3, speed / fullSpeed))
    const step = length / steps
    // Half the line behind the seed, half ahead, so the arrows sit evenly.
    const back = trace(g, field, p0, -step, steps / 2, minSpeed * 0.5)
    const ahead = trace(g, field, p0, step, steps / 2, minSpeed * 0.5)
    const points = [...back.reverse(), p0, ...ahead]
    if (points.length < 3) continue
    out.push({ points, speed, seed: p0 })
  }
  return out
}

// Follows the field from p in steps of `step` radians (negative: against
// it), with a midpoint step each time; stops where the flow dies away.
function trace(g, field, p, step, count, minSpeed) {
  const out = []
  const v = [0, 0, 0]
  let at = p
  for (let k = 0; k < count; k++) {
    sampleVector(g, field, at, v)
    let speed = Math.hypot(v[0], v[1], v[2])
    if (speed < minSpeed) break
    const mid = move(at, v, step / 2 / speed)
    sampleVector(g, field, mid, v)
    speed = Math.hypot(v[0], v[1], v[2])
    if (speed < minSpeed) break
    at = move(at, v, step / speed)
    out.push(at)
  }
  return out
}

function move(p, v, k) {
  const x = p[0] + v[0] * k
  const y = p[1] + v[1] * k
  const z = p[2] + v[2] * k
  const len = Math.hypot(x, y, z)
  return [x / len, y / len, z / len]
}
