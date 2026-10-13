// The solar system in time (src/solarSimulation.js) and the gravity
// simulation's snapshots (src/orbits.js). No dependencies:
// node frontend/tests/solarSimulation.test.mjs
import assert from 'node:assert/strict'
import { createSimulation } from '../src/solarSimulation.js'
import { createGravitySim, keplerOrbit } from '../src/orbits.js'

const close = (a, b, tol, msg) => assert.ok(Math.abs(a - b) <= tol, `${msg}: ${a} vs ${b}`)

const sun = { id: 1, kind: 'star', body_type: 'star', radius_km: 696000 }
const earth = { id: 2, kind: 'planet', body_type: 'rocky', radius_km: 6371, orbit_km: 1.496e8, parent_id: 1, phase: 0 }
const moon = { id: 3, kind: 'moon', body_type: 'rocky', radius_km: 1737, orbit_km: 384400, parent_id: 2, phase: 90 }
// A comet whose orbit crosses Earth's.
const comet = { id: 4, kind: 'planet', body_type: 'rocky', radius_km: 3000, orbit_km: 4e8, parent_id: 1, phase: 180, eccentricity: 0.7, periapsis: 0 }

// Static: positions are the orbits', and only a change the physics reads
// starts it again.
{
  const s = createSimulation()
  assert.equal(s.setBodies([sun, earth, moon]), true)
  assert.equal(s.setBodies([sun, { ...earth, name: 'Terra', color: '#123456' }, moon]), false, 'a rename changes nothing')
  s.advanceTo(100)
  const want = keplerOrbit.position(earth, sun, 100)
  close(s.relative(2)[0], want[0], 1e-3, 'Earth where its orbit says')
  close(s.absolute(3)[1] - s.absolute(2)[1], s.relative(3)[1], 1e-3, "the Moon's place adds up")
  assert.equal(s.event, null)
}

// Static meetings: found where the orbits cross, dismissed once, and shown
// again after a restart.
{
  const twin = { ...earth, id: 5, phase: 0.001 } // the same orbit, a hair ahead
  const s = createSimulation()
  s.setBodies([sun, earth, twin])
  const hit = s.advanceTo(1)
  assert.equal(hit?.type, 'collision')
  assert.deepEqual([hit.a, hit.b].sort(), [2, 5])
  s.dismiss()
  assert.equal(s.advanceTo(2), null, 'dismissed: not shown again')
  s.restart()
  assert.equal(s.days, 0)
  assert.equal(s.advanceTo(1)?.type, 'collision', 'shown again after a restart')
}

// Gravity: the same day reached straight or by going back from later gives
// the same places (from a snapshot, not day 0), within the integrator's
// small differences.
{
  const bodies = [sun, earth, moon, comet]
  const straight = createGravitySim(bodies)
  assert.ok(straight.advanceTo(400))
  const sim = createGravitySim(bodies)
  assert.ok(sim.advanceTo(900))
  assert.ok(sim.advanceTo(400), 'back to day 400')
  close(sim.days, 400, 1e-9, 'the day')
  for (const id of [2, 3, 4]) {
    const [a, b] = [straight.relative(id), sim.relative(id)]
    close(Math.hypot(a[0] - b[0], a[1] - b[1]), 0, Math.hypot(...a) * 1e-6, `body ${id} after going back`)
  }
  // Going back is cheaper than starting over: one step budget too small to
  // redo 400 days from 0 still gets there.
  const tight = createGravitySim(bodies, { maxSteps: 3000 })
  for (let d = 50; d <= 900; d += 50) assert.ok(tight.advanceTo(d), `on to ${d}`)
  assert.ok(tight.advanceTo(850), 'back a little, from a snapshot')
}

// Gravity through the simulation: on and off, and the clock follows it.
{
  const s = createSimulation()
  s.setBodies([sun, earth, moon])
  s.advanceTo(30)
  s.setGravity(true)
  assert.equal(s.gravity, true)
  close(s.days, 30, 1e-9, 'gravity caught up with the clock')
  const g = s.relative(2)
  const k = keplerOrbit.position(earth, sun, 30)
  close(Math.hypot(g[0] - k[0], g[1] - k[1]), 0, 1.496e8 * 1e-3, 'gravity close to the orbit after a month')
  assert.ok(s.orbitNow(2)?.bound)
  s.setGravity(false)
  assert.equal(s.orbitNow(2), null)
}

console.log('solarSimulation: ok')
