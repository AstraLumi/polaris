// The solar system in time, apart from any page or picture: which bodies
// there are, what moves them (the orbits as set, or live gravity), what day
// it is, where everything is, and what broke. The Solar system page runs
// the clock and the scene draws; this decides where things are, so anything
// later worked out from the bodies' places (moon phases, tides, the light
// each planet gets) reads them here and agrees with the picture.
//
// Static orbits are worked out at once. Gravity runs in a worker
// (gravityEngine.js) and catches up as it can: the day it reached, and the
// places, come in later. Whenever anything changes, onChange is called;
// read the state then.
// No dependencies but orbits.js: node frontend/tests/solarSimulation.test.mjs
import { compileSystem, physicsKey } from './orbits.js'
import { createGravityEngine } from './gravityEngine.js'

export function createSimulation({ gravityOptions, onChange = () => {} } = {}) {
  let bodies = []
  let key = null
  let system = compileSystem([])
  let pairs = [] // the pairs whose orbits can meet (static orbits only)
  let rel = new Float64Array(0) // each body from its parent, km, in the system's order
  let abs = new Float64Array(0) // each from the star
  let gravity = false
  let engine = null
  let days = 0
  let event = null
  let lagging = false
  let shapes = null // with gravity: id -> the orbit it is on now
  const dismissed = new Set() // static meetings already shown ("a:b")

  // Each body from the star: the relative places added up the tree.
  function sumUp() {
    abs.set(rel)
    for (let i = 0; i < system.list.length; i++) {
      const p = system.parent[i]
      if (p >= 0) for (let k = 0; k < 3; k++) abs[i * 3 + k] += abs[p * 3 + k]
    }
  }

  // What the gravity engine reports (it may be from a run since replaced:
  // only the current engine's count).
  function take(from, state) {
    if (from !== engine) return
    days = state.days
    event = state.event
    lagging = state.lagging
    if (state.rel.length === rel.length) rel.set(state.rel)
    if (state.shapes) shapes = new Map(state.shapes)
    sumUp()
    onChange()
  }

  // Made again from the bodies: gravity starts over from day 0 and catches
  // up with the clock.
  function rebuild() {
    system = compileSystem(bodies)
    pairs = system.meetingPairs()
    rel = new Float64Array(system.list.length * 3)
    abs = new Float64Array(system.list.length * 3)
    event = null
    lagging = false
    shapes = null
    engine?.dispose()
    engine = null
    if (gravity) {
      const mine = createGravityEngine(bodies, system.list.map((b) => b.id), gravityOptions, (s) => take(mine, s))
      engine = mine
      lagging = days > 0
      engine.advance(days, true)
    } else {
      system.relative(days, rel)
      sumUp()
    }
    onChange()
  }

  // With static orbits: two bodies that overlap where they are now.
  function touching() {
    for (let k = 0; k < pairs.length; k += 2) {
      const [i, j] = [pairs[k], pairs[k + 1]]
      const [a, b] = [system.list[i], system.list[j]]
      const pair = a.id < b.id ? `${a.id}:${b.id}` : `${b.id}:${a.id}`
      if (dismissed.has(pair)) continue
      const d = Math.hypot(abs[i * 3] - abs[j * 3], abs[i * 3 + 1] - abs[j * 3 + 1], abs[i * 3 + 2] - abs[j * 3 + 2])
      if (d < a.radius_km + b.radius_km) return { type: 'collision', a: a.id, b: b.id, days, key: pair }
    }
    return null
  }

  return {
    get days() {
      return days
    },
    // What stopped it, if anything: { type: 'collision', a, b } or
    // { type: 'escape', id, from }, with the day.
    get event() {
      return event
    },
    // Gravity hasn't reached the time last asked for.
    get lagging() {
      return lagging
    },
    get gravity() {
      return gravity
    },
    get system() {
      return system
    },
    // With gravity, the orbit each body is on now, as last asked for with
    // advanceTo(…, { shapes: true }) (orbits.js orbitNow); else null.
    get shapes() {
      return shapes
    },
    // The bodies to move. Only a change the physics reads (physicsKey)
    // starts it again; true when it did.
    setBodies(list) {
      bodies = list
      const next = physicsKey(list)
      if (next === key) return false
      key = next
      rebuild()
      return true
    },
    // Live gravity, or every body on the orbit it was given.
    setGravity(on) {
      if (on === gravity) return
      gravity = on
      rebuild()
    },
    // Moves the clock to `target` days (back as well as on). Gravity gets
    // there when it can; static orbits at once, stopping at a meeting.
    advanceTo(target, { shapes: wantShapes = false } = {}) {
      if (target < days) event = null
      if (engine) return engine.advance(target, wantShapes)
      days = target
      system.relative(days, rel)
      sumUp()
      event = touching()
      onChange()
    },
    // Back to day 0, with every static meeting shown again.
    restart() {
      dismissed.clear()
      event = null
      days = 0
      if (engine) return engine.reset(true)
      system.relative(0, rel)
      sumUp()
      onChange()
    },
    // The meeting shown is not shown again (static orbits; gravity stops
    // for good at its first).
    dismiss() {
      if (event?.key) dismissed.add(event.key)
      if (!engine) event = null
    },
    // Where a body is from what it orbits, in km (a view into the array:
    // copy it to keep it).
    relative(id) {
      const i = system.index.get(id)
      return i == null ? [0, 0, 0] : rel.subarray(i * 3, i * 3 + 3)
    },
    // Where it is from the star, in km.
    absolute(id) {
      const i = system.index.get(id)
      return i == null ? [0, 0, 0] : abs.subarray(i * 3, i * 3 + 3)
    },
    // Stops the worker, if there is one.
    dispose() {
      engine?.dispose()
      engine = null
    },
  }
}
