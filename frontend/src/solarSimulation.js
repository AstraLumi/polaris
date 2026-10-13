// The solar system in time, apart from any page or picture: which bodies
// there are, what moves them (the orbits as set, or live gravity), what day
// it is, where everything is, and what broke. The Solar system page runs
// the clock and the scene draws; this decides where things are, so anything
// later worked out from the bodies' places (moon phases, tides, the light
// each planet gets) reads them here and agrees with the picture.
// No dependencies but orbits.js: node frontend/tests/solarSimulation.test.mjs
import { compileSystem, createGravitySim, physicsKey } from './orbits.js'

export function createSimulation({ gravityOptions } = {}) {
  let bodies = []
  let key = null
  let system = compileSystem([])
  let pairs = [] // the pairs whose orbits can meet (static orbits only)
  let rel = new Float64Array(0) // each body from its parent, km, in the system's order
  let abs = new Float64Array(0) // each from the star
  let gravity = false
  let sim = null
  let days = 0
  let event = null
  let lagging = false
  const dismissed = new Set() // static meetings already shown ("a:b")

  // Where everything is on the day reached, without looking for trouble.
  function settle() {
    if (sim) {
      system.list.forEach((b, i) => sim.relativeInto(b.id, rel, i * 3))
      abs.set(rel)
      for (let i = 0; i < system.list.length; i++) {
        const p = system.parent[i]
        if (p >= 0) for (let k = 0; k < 3; k++) abs[i * 3 + k] += abs[p * 3 + k]
      }
    } else {
      system.relative(days, rel)
      system.absolute(days, abs)
    }
  }

  // Made again from the bodies: gravity starts over from day 0 and catches
  // up with the clock as far as one call goes.
  function rebuild() {
    system = compileSystem(bodies)
    pairs = system.meetingPairs()
    rel = new Float64Array(system.list.length * 3)
    abs = new Float64Array(system.list.length * 3)
    event = null
    lagging = false
    sim = gravity ? createGravitySim(bodies, gravityOptions) : null
    if (sim) {
      lagging = !sim.advanceTo(days) && !sim.event
      days = sim.days
      event = sim.event
    }
    settle()
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
    // Gravity couldn't reach the time asked for in one call.
    get lagging() {
      return lagging
    },
    get gravity() {
      return gravity
    },
    get system() {
      return system
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
    // Moves the clock to `target` days (back as well as on). With gravity
    // as far as one call goes. Returns what stopped it, if anything.
    advanceTo(target) {
      if (target < days) event = null
      if (sim) {
        lagging = !sim.advanceTo(target) && !sim.event
        days = sim.days
        event = sim.event
        settle()
      } else {
        days = target
        settle()
        event = touching()
      }
      return event
    },
    // Back to day 0, with every static meeting shown again.
    restart() {
      dismissed.clear()
      event = null
      days = 0
      sim?.reset()
      settle()
    },
    // The meeting shown is not shown again (static orbits; gravity stops
    // for good at its first).
    dismiss() {
      if (event?.key) dismissed.add(event.key)
      if (!sim) event = null
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
    // With gravity, the orbit a body is on now (orbits.js orbitNow).
    orbitNow(id) {
      return sim ? sim.orbitNow(id) : null
    },
  }
}
