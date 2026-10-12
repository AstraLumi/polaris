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
// in the parent's equatorial plane (x, y). Gravity decides the speed; the
// body gives the shape. Swapping the model swaps how every orbit moves.

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
  position(body, parent, days) {
    const turn = days / this.period(body, parent)
    const angle = rad(body.phase) + 2 * Math.PI * turn
    return [body.orbit_km * Math.cos(angle), body.orbit_km * Math.sin(angle), 0]
  },
}

// An ellipse (Kepler's laws): orbit_km is its semi-major axis, eccentricity
// how stretched it is (0 a circle), periapsis the direction (degrees) where
// it comes closest, and phase where the body is on day 0. It moves fastest
// near periapsis and slowest far out, as gravity has it. Computed once from
// these values; the live simulation below lets gravity reshape it.
const eccOf = (b) => Math.min(Math.max(b.eccentricity || 0, 0), 0.95)

// The eccentric anomaly for a mean anomaly (Kepler's equation, by Newton).
function eccentricAnomaly(M, e) {
  let E = e < 0.8 ? M : Math.PI
  for (let i = 0; i < 30; i++) {
    const d = (E - e * Math.sin(E) - M) / (1 - e * Math.cos(E))
    E -= d
    if (Math.abs(d) < 1e-12) break
  }
  return E
}

const trueFromEccentric = (E, e) => 2 * Math.atan2(Math.sqrt(1 + e) * Math.sin(E / 2), Math.sqrt(1 - e) * Math.cos(E / 2))
const eccentricFromTrue = (nu, e) => 2 * Math.atan2(Math.sqrt(1 - e) * Math.sin(nu / 2), Math.sqrt(1 + e) * Math.cos(nu / 2))

// The point at true anomaly nu (from periapsis), in km.
export function orbitPoint(body, nu) {
  const e = eccOf(body)
  const r = (body.orbit_km * (1 - e * e)) / (1 + e * Math.cos(nu))
  const angle = nu + rad(body.periapsis)
  return [r * Math.cos(angle), r * Math.sin(angle), 0]
}

export const keplerOrbit = {
  period: circularOrbit.period,
  position(body, parent, days) {
    const e = eccOf(body)
    const nu0 = rad(body.phase) - rad(body.periapsis)
    const E0 = eccentricFromTrue(nu0, e)
    const M = E0 - e * Math.sin(E0) + (2 * Math.PI * days) / this.period(body, parent)
    return orbitPoint(body, trueFromEccentric(eccentricAnomaly(M, e), e))
  },
  // Velocity in km/s at `days` (to start the live simulation from).
  velocity(body, parent, days) {
    const e = eccOf(body)
    const [x, y] = this.position(body, parent, days)
    const nu = Math.atan2(y, x) - rad(body.periapsis)
    const p = body.orbit_km * 1000 * (1 - e * e)
    const v = Math.sqrt(mu(body, parent) / p) / 1000
    const w = rad(body.periapsis)
    const [vx, vy] = [-v * Math.sin(nu), v * (e + Math.cos(nu))]
    return [vx * Math.cos(w) - vy * Math.sin(w), vx * Math.sin(w) + vy * Math.cos(w), 0]
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

// Where a body dragged to `distance` km at `angle` degrees sits on an orbit
// of its shape: the semi-major axis that puts it there, and its phase.
export function orbitThrough(body, distance, angle) {
  const e = eccOf(body)
  const nu = rad(angle) - rad(body.periapsis)
  return { orbit_km: (distance * (1 + e * Math.cos(nu))) / (1 - e * e), phase: angle }
}

let model = keplerOrbit
export const orbitModel = () => model
export function useOrbitModel(m) {
  model = m
}

// ---- Live gravity ---------------------------------------------------------------------
// Every body pulling on every other, stepped forward in time (leapfrog,
// which keeps orbits from drifting). It starts from the Kepler orbits on
// day 0 and from then on only gravity moves things: moons too far out are
// taken, close planets tug at each other. Going back in time starts again
// from day 0. A step is a small part of the quickest orbit; past maxSteps
// a call stops short, and `days` lags behind what was asked.
export function createGravitySim(bodies, { stepsPerOrbit = 200, maxSteps = 20000 } = {}) {
  const { byId } = systemTree(bodies)
  const list = bodies.filter((b) => b.kind === 'star' || (b.orbit_km > 0 && byId.has(b.parent_id)))
  const n = list.length
  const index = new Map(list.map((b, i) => [b.id, i]))
  const mass = list.map(massOf)
  let quickest = Infinity
  for (const b of list) if (b.parent_id != null) quickest = Math.min(quickest, keplerOrbit.period(b, byId.get(b.parent_id)))
  const dt = (Number.isFinite(quickest) ? Math.max(quickest / stepsPerOrbit, 1e-4) : 1) * DAY // s
  const pos = new Float64Array(n * 3) // m
  const vel = new Float64Array(n * 3) // m/s
  const acc = new Float64Array(n * 3)
  let days = 0

  function accelerate() {
    acc.fill(0)
    for (let i = 0; i < n; i++) {
      for (let j = i + 1; j < n; j++) {
        const dx = pos[j * 3] - pos[i * 3]
        const dy = pos[j * 3 + 1] - pos[i * 3 + 1]
        const dz = pos[j * 3 + 2] - pos[i * 3 + 2]
        const r2 = dx * dx + dy * dy + dz * dz || 1
        const f = G / (r2 * Math.sqrt(r2))
        acc[i * 3] += f * mass[j] * dx
        acc[i * 3 + 1] += f * mass[j] * dy
        acc[i * 3 + 2] += f * mass[j] * dz
        acc[j * 3] -= f * mass[i] * dx
        acc[j * 3 + 1] -= f * mass[i] * dy
        acc[j * 3 + 2] -= f * mass[i] * dz
      }
    }
  }

  function reset() {
    days = 0
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
  }

  function step() {
    const h = dt / 2
    for (let k = 0; k < n * 3; k++) vel[k] += acc[k] * h
    for (let k = 0; k < n * 3; k++) pos[k] += vel[k] * dt
    accelerate()
    for (let k = 0; k < n * 3; k++) vel[k] += acc[k] * h
  }

  reset()
  return {
    get days() {
      return days
    },
    // Moves on to `target` days (or as far as maxSteps allows); false when
    // it fell short.
    advanceTo(target) {
      if (target < days) reset()
      const want = Math.max(0, Math.floor(((target - days) * DAY) / dt))
      const steps = Math.min(want, maxSteps)
      for (let s = 0; s < steps; s++) step()
      days += (steps * dt) / DAY
      return steps === want
    },
    // Where a body is relative to what it orbits, in km.
    relative(id) {
      const b = byId.get(id)
      const i = index.get(id)
      if (i == null || b?.parent_id == null || !index.has(b.parent_id)) return [0, 0, 0]
      const j = index.get(b.parent_id)
      return [0, 1, 2].map((k) => (pos[i * 3 + k] - pos[j * 3 + k]) / 1000)
    },
  }
}

// The region round a body where its own gravity, not its parent's, holds a
// moon (the Hill sphere), in km. Moons farther out than about half of it
// wouldn't stay.
// On an ellipse, it is smallest where the orbit comes closest.
export function hillRadius(body, parent) {
  return periapsisOf(body) * Math.cbrt(massOf(body) / (3 * massOf(parent)))
}

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

// Where every body is at `days`, in km from the star: its orbit round its
// parent, added up the tree.
export function positionsAt(bodies, days, orbits = orbitModel()) {
  const { byId } = systemTree(bodies)
  const out = new Map()
  const place = (b) => {
    if (out.has(b.id)) return out.get(b.id)
    const parent = b.parent_id != null ? byId.get(b.parent_id) : null
    let p = [0, 0, 0]
    if (parent && b.orbit_km) {
      const base = place(parent)
      const rel = orbits.position(b, parent, days)
      p = [base[0] + rel[0], base[1] + rel[1], base[2] + rel[2]]
    }
    out.set(b.id, p)
    return p
  }
  for (const b of bodies) place(b)
  return out
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
