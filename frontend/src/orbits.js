// The solar system's physics, kept apart from drawing so it can grow on its
// own: today bodies are uniform spheres on circular orbits; tides, winds,
// moon phases and proper Kepler orbits are meant to plug in here later.
// No dependencies: node frontend/tests/orbits.test.mjs
//
// Units: kilometres in, SI inside (metres, kilograms, seconds), and times out
// in Earth days.

export const G = 6.674e-11 // m³ kg⁻¹ s⁻²
export const EARTH_G = 9.80665 // m/s²
const DAY = 86400 // s

// The four kinds of body, each with one density (kg/m³) standing for the
// whole body: a Sun-like star, an Earth-like rock, a Jupiter-like gas giant,
// and a water world (rock wrapped in deep ocean, between the two).
export const BODY_TYPES = {
  star: { density: 1410 },
  rocky: { density: 5510 },
  gas: { density: 1330 },
  ocean: { density: 3000 },
}

const volumeOf = (body) => (4 / 3) * Math.PI * (body.radius_km * 1000) ** 3

// Mass in kg: a star's own (stars.js keeps it real for its class), else a
// uniform sphere of its type's density.
export function massOf(body) {
  if (body.mass_kg > 0) return body.mass_kg
  return (BODY_TYPES[body.body_type]?.density ?? BODY_TYPES.rocky.density) * volumeOf(body)
}

// Average density, kg/m³.
export const densityOf = (body) => massOf(body) / volumeOf(body)

// Gravity at the surface, in m/s².
export function surfaceGravity(body) {
  const r = body.radius_km * 1000
  return (G * massOf(body)) / r ** 2
}

// Speed needed to leave the surface for good, in km/s.
export function escapeVelocity(body) {
  const r = body.radius_km * 1000
  return Math.sqrt((2 * G * massOf(body)) / r) / 1000
}

// ---- Orbits ---------------------------------------------------------------------------
// An orbit model says where a body is, relative to what it orbits, at a
// time: period(body, parent) in days and position(body, parent, days) in km,
// x and y in the system's plane, z north of it. Gravity decides the speed; the
// body gives the shape. Swapping the model swaps how every orbit moves.
// prepare(body, parent) works out what doesn't change with time once and
// returns (days, out, o) => writes the position into out[o..o+2]: what
// compileSystem calls every frame. position() is the same, one at a time.

const mu = (body, parent) => G * (massOf(parent) + massOf(body)) // m³/s²
const rad = (deg) => ((deg || 0) * Math.PI) / 180

// A circle (kept for comparison and the tests).
export const circularOrbit = {
  // Days for one turn (Kepler's third law, both masses counted).
  period(body, parent) {
    const a = body.orbit_km * 1000
    return (2 * Math.PI * Math.sqrt(a ** 3 / mu(body, parent))) / DAY
  },
  // Position in km at `days`, starting at the body's phase (degrees).
  prepare(body, parent) {
    const a = body.orbit_km
    const start = rad(body.phase)
    const n = (2 * Math.PI) / circularOrbit.period(body, parent) // radians a day
    return (days, out, o) => {
      const angle = start + n * days
      out[o] = a * Math.cos(angle)
      out[o + 1] = a * Math.sin(angle)
      out[o + 2] = 0
    }
  },
  position(body, parent, days) {
    return positionOnce(this, body, parent, days)
  },
}

function positionOnce(model, body, parent, days) {
  const out = [0, 0, 0]
  model.prepare(body, parent)(days, out, 0)
  return out
}

// An ellipse (Kepler's laws): orbit_km is its semi-major axis, eccentricity
// how stretched it is (0 a circle), periapsis the direction (degrees) where
// it comes closest, and phase where the body is on day 0. It moves fastest
// near periapsis and slowest far out, as gravity has it. Computed once from
// these values; the live simulation below lets gravity reshape it.
// Orbits typed in stop at 0.95 (the server says so); the orbit gravity has
// a body on can be more stretched than that, and is drawn as it is.
const eccOf = (b) => Math.min(Math.max(b.eccentricity || 0, 0), 0.9999)

// The eccentric anomaly for a mean anomaly (Kepler's equation, by Newton).
function eccentricAnomaly(M, e) {
  // Wrapped into one turn first: many turns on (a fast moon, years in),
  // Newton's method from far away jumped about, and the orbit with it.
  M = ((M % (2 * Math.PI)) + 2 * Math.PI) % (2 * Math.PI)
  let E = e < 0.8 ? M : Math.PI
  for (let i = 0; i < 30; i++) {
    const d = (E - e * Math.sin(E) - M) / (1 - e * Math.cos(E))
    E -= d
    if (Math.abs(d) < 1e-12) break
  }
  return E
}

const eccentricFromTrue = (nu, e) => 2 * Math.atan2(Math.sqrt(1 - e) * Math.sin(nu / 2), Math.sqrt(1 + e) * Math.cos(nu / 2))

// The plane an orbit lies in, as two unit vectors: P towards its closest
// point, Q a quarter turn on along the orbit. It is tilted `inclination`
// degrees from the system's plane about the line through its ascending
// node (where it crosses going north). Angles along the orbit (periapsis,
// phase) count from the same zero as in the plane: the node's direction,
// then on along the orbit. Flat (inclination 0), P points at periapsis.
export function orbitFrame(body) {
  const i = rad(body.inclination)
  const node = rad(body.ascending_node)
  const w = rad(body.periapsis) - node // from the node to periapsis
  const [ci, si, cn, sn, cw, sw] = [Math.cos(i), Math.sin(i), Math.cos(node), Math.sin(node), Math.cos(w), Math.sin(w)]
  return {
    P: [cn * cw - sn * sw * ci, sn * cw + cn * sw * ci, sw * si],
    Q: [-cn * sw - sn * cw * ci, -sn * sw + cn * cw * ci, cw * si],
  }
}

// x along P plus y along Q, into out[o..o+2].
function inFrame({ P, Q }, x, y, out, o) {
  out[o] = x * P[0] + y * Q[0]
  out[o + 1] = x * P[1] + y * Q[1]
  out[o + 2] = x * P[2] + y * Q[2]
}

// The point at true anomaly nu (from periapsis), in km.
export function orbitPoint(body, nu) {
  const e = eccOf(body)
  const r = (body.orbit_km * (1 - e * e)) / (1 + e * Math.cos(nu))
  const out = [0, 0, 0]
  inFrame(orbitFrame(body), r * Math.cos(nu), r * Math.sin(nu), out, 0)
  return out
}

// What an orbit's movement needs, worked out once: its frame, its shape,
// and the mean anomaly (M0 on day 0, n radians a day).
function keplerConstants(body, parent) {
  const e = eccOf(body)
  const E0 = eccentricFromTrue(rad(body.phase) - rad(body.periapsis), e)
  return {
    e,
    a: body.orbit_km,
    b: body.orbit_km * Math.sqrt(1 - e * e),
    frame: orbitFrame(body),
    M0: E0 - e * Math.sin(E0),
    n: (2 * Math.PI) / keplerOrbit.period(body, parent),
  }
}

export const keplerOrbit = {
  period: circularOrbit.period,
  // From the eccentric anomaly E straight to the point: (a(cos E - e),
  // b sin E) on the ellipse, laid in its plane.
  prepare(body, parent) {
    const { e, a, b, frame, M0, n } = keplerConstants(body, parent)
    return (days, out, o) => {
      const E = e ? eccentricAnomaly(M0 + n * days, e) : M0 + n * days
      inFrame(frame, a * (Math.cos(E) - e), b * Math.sin(E), out, o)
    }
  },
  position(body, parent, days) {
    return positionOnce(this, body, parent, days)
  },
  // Velocity in km/s at `days` (to start the live simulation from).
  velocity(body, parent, days) {
    const { e, frame, M0, n } = keplerConstants(body, parent)
    const E = eccentricAnomaly(M0 + n * days, e)
    const nu = 2 * Math.atan2(Math.sqrt(1 + e) * Math.sin(E / 2), Math.sqrt(1 - e) * Math.cos(E / 2))
    const v = Math.sqrt(mu(body, parent) / (body.orbit_km * 1000 * (1 - e * e))) / 1000
    const out = [0, 0, 0]
    inFrame(frame, -v * Math.sin(nu), v * (e + Math.cos(nu)), out, 0)
    return out
  },
}

// Points round a body's whole orbit, for drawing it.
export function orbitPath(body, steps = 128) {
  const out = []
  for (let k = 0; k <= steps; k++) out.push(orbitPoint(body, (k / steps) * 2 * Math.PI))
  return out
}

// Its closest and farthest distances, in km.
export const periapsisOf = (b) => b.orbit_km * (1 - eccOf(b))
export const apoapsisOf = (b) => b.orbit_km * (1 + eccOf(b))

// Where a body dragged to `distance` km at `angle` degrees along its orbit
// sits on an orbit of its shape: the semi-major axis that puts it there,
// and its phase.
export function orbitThrough(body, distance, angle) {
  const e = eccOf(body)
  const nu = rad(angle) - rad(body.periapsis)
  return { orbit_km: (distance * (1 + e * Math.cos(nu))) / (1 - e * e), phase: angle }
}

// Seen from above (the 2D editor), an inclined orbit is squashed across its
// node line. For the direction `skyAngle` (degrees, in the system's plane)
// on a body's orbit: the angle along the orbit there (degrees, a phase),
// and how much shorter its distance looks from above (1 for a flat orbit).
export function alongOrbit(body, skyAngle) {
  const node = rad(body.ascending_node)
  const th = rad(skyAngle) - node
  let c = Math.cos(rad(body.inclination))
  // Edge-on, the whole orbit is a line: read it as nearly so.
  if (Math.abs(c) < 0.02) c = c < 0 ? -0.02 : 0.02
  const u = Math.atan2(Math.sin(th) / c, Math.cos(th)) // from the node, along the orbit
  return {
    phase: (((((node + u) * 180) / Math.PI) % 360) + 360) % 360,
    squash: Math.max(Math.hypot(Math.cos(u), Math.sin(u) * c), 0.02),
  }
}

let model = keplerOrbit
export const orbitModel = () => model
export function useOrbitModel(m) {
  model = m
}

// ---- Gravity ------------------------------------------------------------------------
// Every body pulling on every other, stepped forward in time with a
// fourth-order symplectic integrator (Yoshida), which keeps orbits from
// drifting over thousands of turns. It starts from the Kepler orbits on day
// 0 and from then on only gravity moves things: as a planet passes another,
// each bends the other's path; moons too far out are taken. Going back in
// time starts again from the last snapshot before (one is kept every so
// often; when there are too many, every other one goes and they are kept
// twice as far apart), or from day 0.
//
// Steps adapt: each is a small part of the shortest orbit any pair of
// bodies would have at their present distance, so a comet diving at its
// star takes small steps there and big ones far out (fixed steps sent it
// straight through). Past maxSteps a call stops short, and `days` lags
// behind what was asked.
//
// It watches for the system coming apart: two bodies touching ('collision')
// or a body no longer held by what it orbits ('escape': moving fast enough
// to leave it, or, for a moon, out past its planet's reach). It stops at
// the first, and reports it.
const CBRT2 = Math.cbrt(2)
const W1 = 1 / (2 - CBRT2)
const W0 = -CBRT2 / (2 - CBRT2)
const YOSHIDA_C = [W1 / 2, (W0 + W1) / 2, (W0 + W1) / 2, W1 / 2]
const YOSHIDA_D = [W1, W0, W1]

export function createGravitySim(bodies, { stepsPerOrbit = 120, maxSteps = 20000, checkEvery = 20, maxSnapshots = 256 } = {}) {
  const { byId } = systemTree(bodies)
  const list = bodies.filter((b) => b.kind === 'star' || (b.orbit_km > 0 && byId.has(b.parent_id)))
  const n = list.length
  const index = new Map(list.map((b, i) => [b.id, i]))
  const mass = list.map(massOf)
  const radius = list.map((b) => b.radius_km * 1000)
  const parentIndex = list.map((b) => (b.parent_id != null && index.has(b.parent_id) ? index.get(b.parent_id) : -1))
  // How far a moon can go from its planet before the star takes it (m).
  const reach = list.map((b) => {
    const p = byId.get(b.parent_id)
    const g = p && byId.get(p.parent_id)
    return b.kind === 'moon' && p && g && p.orbit_km ? hillRadius(p, g) * 1000 : Infinity
  })
  let quickest = Infinity
  for (const b of list) if (b.parent_id != null) quickest = Math.min(quickest, keplerOrbit.period(b, byId.get(b.parent_id)))
  const maxDt = (Number.isFinite(quickest) ? quickest / stepsPerOrbit : 1) * DAY // s
  const eta = (2 * Math.PI) / stepsPerOrbit
  let pairTime = Infinity // the shortest r³/(G·m) time of any pair, s
  const pos = new Float64Array(n * 3) // m
  const vel = new Float64Array(n * 3) // m/s
  const acc = new Float64Array(n * 3)
  let days = 0
  let event = null
  let touching = null // [i, j] found while computing forces
  const snapshots = [] // { days, pos, vel }, oldest first; the first is day 0
  let snapEvery = (maxDt / DAY) * 200 // days: every 200 of the longest steps

  function snapshot() {
    snapshots.push({ days, pos: pos.slice(), vel: vel.slice() })
    if (snapshots.length > maxSnapshots) {
      const kept = snapshots.filter((_, k) => k % 2 === 0)
      snapshots.length = 0
      snapshots.push(...kept)
      snapEvery *= 2
    }
  }

  // Back to the last snapshot at or before `target`.
  function rewind(target) {
    let k = snapshots.length - 1
    while (k > 0 && snapshots[k].days > target) k--
    snapshots.length = k + 1
    const s = snapshots[k]
    days = s.days
    pos.set(s.pos)
    vel.set(s.vel)
    event = null
    accelerate()
  }

  function accelerate() {
    acc.fill(0)
    touching = null
    let shortest = Infinity // pairTime squared, so one square root in all
    for (let i = 0; i < n; i++) {
      for (let j = i + 1; j < n; j++) {
        const dx = pos[j * 3] - pos[i * 3]
        const dy = pos[j * 3 + 1] - pos[i * 3 + 1]
        const dz = pos[j * 3 + 2] - pos[i * 3 + 2]
        const r2 = dx * dx + dy * dy + dz * dz || 1
        const rr = radius[i] + radius[j]
        if (r2 < rr * rr) touching = [i, j]
        const r3 = r2 * Math.sqrt(r2)
        shortest = Math.min(shortest, r3 / (G * (mass[i] + mass[j])))
        const f = G / r3
        acc[i * 3] += f * mass[j] * dx
        acc[i * 3 + 1] += f * mass[j] * dy
        acc[i * 3 + 2] += f * mass[j] * dz
        acc[j * 3] -= f * mass[i] * dx
        acc[j * 3 + 1] -= f * mass[i] * dy
        acc[j * 3 + 2] -= f * mass[i] * dz
      }
    }
    pairTime = Math.sqrt(shortest)
  }

  function reset() {
    days = 0
    event = null
    pos.fill(0)
    vel.fill(0)
    const done = new Set()
    const place = (b) => {
      if (done.has(b.id)) return
      done.add(b.id)
      const parent = b.parent_id != null ? byId.get(b.parent_id) : null
      if (!parent || !index.has(parent.id)) return
      place(parent)
      const i = index.get(b.id)
      const j = index.get(parent.id)
      const p = keplerOrbit.position(b, parent, 0)
      const v = keplerOrbit.velocity(b, parent, 0)
      for (let k = 0; k < 3; k++) {
        pos[i * 3 + k] = pos[j * 3 + k] + p[k] * 1000
        vel[i * 3 + k] = vel[j * 3 + k] + v[k] * 1000
      }
    }
    for (const b of list) place(b)
    accelerate()
    snapshots.length = 0
    snapshot()
  }

  function step(dt) {
    for (let s = 0; s < 4; s++) {
      const c = YOSHIDA_C[s] * dt
      for (let k = 0; k < n * 3; k++) pos[k] += vel[k] * c
      if (s < 3) {
        accelerate()
        const d = YOSHIDA_D[s] * dt
        for (let k = 0; k < n * 3; k++) vel[k] += acc[k] * d
      }
    }
  }

  // Relative position (m) and velocity (m/s) of body i to its parent.
  function rel(i) {
    const j = parentIndex[i]
    const r = [0, 1, 2].map((k) => pos[i * 3 + k] - pos[j * 3 + k])
    const v = [0, 1, 2].map((k) => vel[i * 3 + k] - vel[j * 3 + k])
    return { r, v, mu: G * (mass[i] + mass[j]) }
  }

  function checkEscapes() {
    for (let i = 0; i < n; i++) {
      if (parentIndex[i] < 0) continue
      const { r, v, mu } = rel(i)
      const d = Math.hypot(...r)
      const energy = (v[0] ** 2 + v[1] ** 2 + v[2] ** 2) / 2 - mu / d
      if (energy >= 0 || d > reach[i]) return { type: 'escape', id: list[i].id, from: list[parentIndex[i]].id }
    }
    return null
  }

  reset()
  return {
    get days() {
      return days
    },
    // What broke the system, if anything: { type, id, from } or { type,
    // a, b }, with the day.
    get event() {
      return event
    },
    reset,
    // Moves on to `target` days, as far as maxSteps allows, stopping at
    // an event. True when it got there.
    advanceTo(target) {
      if (target < days) rewind(target)
      if (event) return false
      for (let s = 0; days < target; s++) {
        if (s >= maxSteps) return false
        const dt = Math.min(maxDt, eta * pairTime, (target - days) * DAY)
        step(dt)
        days += dt / DAY
        if (touching) {
          event = { type: 'collision', a: list[touching[0]].id, b: list[touching[1]].id, days }
          return false
        }
        if (s % checkEvery === 0) {
          const e = checkEscapes()
          if (e) {
            event = { ...e, days }
            return false
          }
        }
        if (days - snapshots[snapshots.length - 1].days >= snapEvery) snapshot()
      }
      return true
    },
    // Where a body is relative to what it orbits, in km.
    relative(id) {
      const out = [0, 0, 0]
      this.relativeInto(id, out, 0)
      return out
    },
    // The same, written into out[o..o+2].
    relativeInto(id, out, o) {
      const i = index.get(id)
      const j = i == null ? -1 : parentIndex[i]
      for (let k = 0; k < 3; k++) out[o + k] = j < 0 ? 0 : (pos[i * 3 + k] - pos[j * 3 + k]) / 1000
    },
    // The orbit a body is on right now, by its position and speed (the
    // osculating orbit): how gravity has reshaped it so far.
    orbitNow(id) {
      const i = index.get(id)
      if (i == null || parentIndex[i] < 0) return null
      const { r, v, mu } = rel(i)
      return elementsFromState(r, v, mu)
    },
  }
}

// The ellipse through a position (m) with a velocity (m/s) round a mass
// (mu = G·M): { orbit_km, eccentricity, periapsis, inclination,
// ascending_node (degrees, as orbitFrame reads them), bound }. An unbound
// path (eccentricity 1 or more) is flying off.
export function elementsFromState(r, v, mu) {
  const d = Math.hypot(r[0], r[1], r[2])
  const v2 = v[0] ** 2 + v[1] ** 2 + v[2] ** 2
  const energy = v2 / 2 - mu / d
  const rv = r[0] * v[0] + r[1] * v[1] + r[2] * v[2]
  // The eccentricity vector, pointing at periapsis.
  const ev = [0, 1, 2].map((k) => ((v2 - mu / d) * r[k] - rv * v[k]) / mu)
  const e = Math.hypot(ev[0], ev[1], ev[2])
  // The orbit's pole (r × v, unit), its tilt, and the node line (z × pole).
  const h = [r[1] * v[2] - r[2] * v[1], r[2] * v[0] - r[0] * v[2], r[0] * v[1] - r[1] * v[0]]
  const hn = Math.hypot(h[0], h[1], h[2]) || 1
  const [hx, hy, hz] = [h[0] / hn, h[1] / hn, h[2] / hn]
  const flat = Math.hypot(hx, hy) < 1e-9
  const node = flat ? 0 : Math.atan2(hx, -hy)
  const [nx, ny] = [Math.cos(node), Math.sin(node)]
  // From the node to periapsis, along the orbit (the way it runs):
  // atan2((n × e) · pole, n · e).
  const along = ny * ev[2] * hx - nx * ev[2] * hy + (nx * ev[1] - ny * ev[0]) * hz
  const w = Math.atan2(along, nx * ev[0] + ny * ev[1])
  const deg = (x) => ((((x * 180) / Math.PI) % 360) + 360) % 360
  const bound = energy < 0 && e < 1
  return {
    orbit_km: bound ? -mu / (2 * energy) / 1000 : d / 1000,
    eccentricity: e,
    periapsis: deg(node + w),
    inclination: (Math.acos(Math.max(-1, Math.min(1, hz))) * 180) / Math.PI,
    ascending_node: deg(node),
    bound,
  }
}

// The region round a body where its own gravity, not its parent's, holds a
// moon (the Hill sphere), in km. Moons farther out than about half of it
// wouldn't stay.
// On an ellipse, it is smallest where the orbit comes closest.
export function hillRadius(body, parent) {
  return periapsisOf(body) * Math.cbrt(massOf(body) / (3 * massOf(parent)))
}

// What the physics reads of each body. Anything else (its name, colour,
// spin, rings) can change without the orbits changing: a page compares
// physicsKey before redoing any physics. A new physical value goes here.
const PHYSICS_FIELDS = ['id', 'parent_id', 'kind', 'body_type', 'radius_km', 'mass_kg', 'orbit_km', 'phase', 'eccentricity', 'periapsis', 'inclination', 'ascending_node']
export const physicsKey = (bodies) => JSON.stringify(bodies.map((b) => PHYSICS_FIELDS.map((k) => b[k] ?? null)))

// ---- The system as a tree -------------------------------------------------------------

// Bodies by id and the children of each, nearest orbit first: the order of
// a solar system is the order of its distances.
export function systemTree(bodies) {
  const byId = new Map(bodies.map((b) => [b.id, b]))
  const children = new Map()
  for (const b of bodies) {
    const key = b.parent_id ?? null
    if (!children.has(key)) children.set(key, [])
    children.get(key).push(b)
  }
  for (const list of children.values()) list.sort((a, b) => (a.orbit_km ?? 0) - (b.orbit_km ?? 0) || a.id - b.id)
  return { byId, childrenOf: (id) => children.get(id ?? null) ?? [], star: (children.get(null) ?? []).find((b) => b.kind === 'star') ?? null }
}

// ---- The system, compiled ----------------------------------------------------------------
// The bodies made ready to be moved many times a second: in an order where
// every parent comes before what orbits it, with each orbit's constants
// worked out once (orbits.prepare). Positions are written into flat arrays,
// three numbers per body (list[i] at 3i), in km. Compile again when the
// bodies change (physicsKey says when that matters).
// A body with no orbit, or whose parent is missing, sits at the origin.
export function compileSystem(bodies, orbits = orbitModel()) {
  const { byId, childrenOf, star } = systemTree(bodies)
  const list = []
  const depths = [] // how deep in the tree: everything round list[i] follows it, deeper
  const walk = (b, d) => {
    list.push(b)
    depths.push(d)
    for (const c of childrenOf(b.id)) walk(c, d + 1)
  }
  for (const b of bodies) if (b.parent_id == null || !byId.has(b.parent_id)) walk(b, 0)
  const n = list.length
  const index = new Map(list.map((b, i) => [b.id, i]))
  const parent = new Int32Array(n).fill(-1)
  const moves = list.map((b, i) => {
    const p = byId.get(b.parent_id)
    if (!p || !b.orbit_km) return null
    parent[i] = index.get(p.id)
    return orbits.prepare(b, p)
  })
  return {
    list,
    index,
    parent,
    star,
    // Where each body is from what it orbits, at `days`.
    relative(days, out = new Float64Array(n * 3)) {
      for (let i = 0; i < n; i++) {
        if (moves[i]) moves[i](days, out, i * 3)
        else out[i * 3] = out[i * 3 + 1] = out[i * 3 + 2] = 0
      }
      return out
    },
    // Where each body is from the star: the relative positions added up the
    // tree (parents come first, so theirs are already summed).
    absolute(days, out = new Float64Array(n * 3)) {
      this.relative(days, out)
      for (let i = 0; i < n; i++) {
        const p = parent[i]
        if (p < 0) continue
        for (let k = 0; k < 3; k++) out[i * 3 + k] += out[p * 3 + k]
      }
      return out
    },
    // The pairs of bodies (indices, flat: a, b, a, b…) whose orbits can
    // ever bring them together; the others needn't be checked for touching.
    // Each body with everything round it (its moons' orbits) fills a ring
    // round its parent from periapsis to apoapsis: siblings whose rings
    // overlap can meet, everything in one with everything in the other;
    // and a body whose ring reaches down to its parent can hit it.
    meetingPairs() {
      const reach = new Float64Array(n) // its radius and its moons' orbits
      for (let i = n - 1; i >= 0; i--) {
        reach[i] = Math.max(reach[i], list[i].radius_km)
        const p = parent[i]
        if (p >= 0) reach[p] = Math.max(reach[p], apoapsisOf(list[i]) + reach[i])
      }
      // A body and everything round it: the run of deeper ones after it.
      const within = (i) => {
        const out = [i]
        for (let j = i + 1; j < n && depths[j] > depths[i]; j++) out.push(j)
        return out
      }
      const pairs = []
      const all = (a, b) => {
        for (const i of a) for (const j of b) pairs.push(i, j)
      }
      for (let i = 0; i < n; i++) {
        const p = parent[i]
        if (p < 0) continue
        const [near, far] = [periapsisOf(list[i]) - reach[i], apoapsisOf(list[i]) + reach[i]]
        if (near <= list[p].radius_km) all([p], within(i))
        for (let j = i + 1; j < n; j++) {
          if (parent[j] !== p) continue
          if (near <= apoapsisOf(list[j]) + reach[j] && periapsisOf(list[j]) - reach[j] <= far) all(within(i), within(j))
        }
      }
      return pairs
    },
  }
}

// Where every body is at `days`, in km from the star (id -> [x, y, z]).
export function positionsAt(bodies, days, orbits = orbitModel()) {
  const system = compileSystem(bodies, orbits)
  const pos = system.absolute(days)
  return new Map(system.list.map((b, i) => [b.id, [pos[i * 3], pos[i * 3 + 1], pos[i * 3 + 2]]]))
}

// What a body's facts say about it, for the side panel: mass, gravity, and,
// for anything orbiting, its year and whether the orbit makes sense.
export function bodyFacts(body, parent) {
  const facts = {
    mass: massOf(body),
    gravity: surfaceGravity(body),
    escape: escapeVelocity(body),
    density: densityOf(body),
  }
  if (parent && body.orbit_km) {
    facts.period = orbitModel().period(body, parent)
    facts.inside = body.orbit_km <= parent.radius_km + body.radius_km
  }
  return facts
}

// How close and how far a body can orbit its parent, in km, by the gravity
// of both (and of the parent's own parent, when there is one):
// - too close: inside the Roche limit, the parent's tides would pull a body
//   of its density apart (and never closer than touching);
// - too far: past half the parent's Hill sphere a moon is taken by the
//   star; a planet round a star, past the star's reach against the rest of
//   the galaxy (about 230 000 AU for the Sun, growing with the mass).
const SUN_MASS = 1.989e30
const AU = 1.496e8
export function orbitLimits(body, parent, grandparent = null) {
  const touching = parent.radius_km + body.radius_km
  const roche = parent.radius_km * Math.cbrt((2 * densityOf(parent)) / densityOf(body))
  const far = grandparent && parent.orbit_km
    ? hillRadius(parent, grandparent) / 2
    : 230000 * AU * Math.cbrt((massOf(parent) + massOf(body)) / SUN_MASS)
  return { near_km: Math.max(roche, touching), far_km: far }
}

// What is wrong with an orbit, if anything: 'inside' what it orbits, 'close'
// enough to be torn apart, or too far out ('loose') to be held. An ellipse
// is judged where it comes closest and where it goes farthest.
export function orbitProblem(body, parent, grandparent = null) {
  if (periapsisOf(body) <= parent.radius_km + body.radius_km) return 'inside'
  const { near_km, far_km } = orbitLimits(body, parent, grandparent)
  if (periapsisOf(body) < near_km) return 'close'
  if (apoapsisOf(body) > far_km) return 'loose'
  return ''
}

// The same, for a moon: kept for the tests' sake.
export const moonProblem = (moon, planet, star) => orbitProblem(moon, planet, star)
