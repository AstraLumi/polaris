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

export const densityOf = (body) => BODY_TYPES[body.body_type]?.density ?? BODY_TYPES.rocky.density

// Mass in kg of a uniform sphere.
export function massOf(body) {
  const r = body.radius_km * 1000
  return densityOf(body) * (4 / 3) * Math.PI * r ** 3
}

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

// Problems with a moon's orbit: inside its planet, or too far out for the
// planet to hold it.
export function moonProblem(moon, planet, star) {
  if (moon.orbit_km <= planet.radius_km + moon.radius_km) return 'inside'
  if (star && planet.orbit_km && moon.orbit_km > hillRadius(planet, star) / 2) return 'loose'
  return ''
}
