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
// time. The only model for now is a circle in the parent's equatorial plane
// (x, y), gravity alone deciding its speed. A better model (eccentricity,
// inclination, n-body) only needs the same two functions.

export const circularOrbit = {
  // Days for one turn (Kepler's third law, both masses counted).
  period(body, parent) {
    const a = body.orbit_km * 1000
    return (2 * Math.PI * Math.sqrt(a ** 3 / (G * (massOf(parent) + massOf(body))))) / DAY
  },
  // Position in km at `days`, starting at the body's phase (degrees).
  position(body, parent, days) {
    const turn = days / this.period(body, parent)
    const angle = ((body.phase || 0) * Math.PI) / 180 + 2 * Math.PI * turn
    return [body.orbit_km * Math.cos(angle), body.orbit_km * Math.sin(angle), 0]
  },
}

let model = circularOrbit
export const orbitModel = () => model
export function useOrbitModel(m) {
  model = m
}

// The region round a body where its own gravity, not its parent's, holds a
// moon (the Hill sphere), in km. Moons farther out than about half of it
// wouldn't stay.
export function hillRadius(body, parent) {
  return body.orbit_km * Math.cbrt(massOf(body) / (3 * massOf(parent)))
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
// enough to be torn apart, or too far out ('loose') to be held.
export function orbitProblem(body, parent, grandparent = null) {
  if (body.orbit_km <= parent.radius_km + body.radius_km) return 'inside'
  const { near_km, far_km } = orbitLimits(body, parent, grandparent)
  if (body.orbit_km < near_km) return 'close'
  if (body.orbit_km > far_km) return 'loose'
  return ''
}

// The same, for a moon: kept for the tests' sake.
export const moonProblem = (moon, planet, star) => orbitProblem(moon, planet, star)
