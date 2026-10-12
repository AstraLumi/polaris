// How the solar system is drawn: true sizes and distances would put every
// planet a pixel wide and a screen apart, so both are squeezed (a power
// below 1 keeps bigger bigger and farther farther). The physics in
// orbits.js always uses the true values; only the picture uses these.
import { orbitModel } from './orbits.js'

const SIZE_POWER = 0.35
const SIZE_SCALE = 0.02
const DISTANCE_POWER = 0.4
const DISTANCE_SCALE = 0.004

// A body's radius in scene units.
export const sceneRadius = (body) => SIZE_SCALE * Math.max(body.radius_km, 1) ** SIZE_POWER

// How far from its parent a body is drawn: the squeezed orbit, plus both
// radii so it never sits inside what it orbits.
export function sceneDistance(orbitKm, parent = null, body = null) {
  const gap = DISTANCE_SCALE * Math.max(orbitKm, 1) ** DISTANCE_POWER
  return gap + (parent ? sceneRadius(parent) : 0) + (body ? sceneRadius(body) : 0)
}

// sceneDistance backwards: the orbit, in km, that is drawn `distance` away
// from the parent. Never closer than just outside the parent, for real.
export function orbitForSceneDistance(distance, parent, body) {
  const gap = Math.max(distance - sceneRadius(parent) - sceneRadius(body), 0)
  const km = (gap / DISTANCE_SCALE) ** (1 / DISTANCE_POWER)
  return Math.max(km, (parent.radius_km + body.radius_km) * 1.05)
}

// Where each body is drawn at `days`: in the direction the orbit model puts
// it, at its drawn distance, added up the tree like the true positions.
export function scenePositions(bodies, days, orbits = orbitModel()) {
  const byId = new Map(bodies.map((b) => [b.id, b]))
  const out = new Map()
  const place = (b) => {
    if (out.has(b.id)) return out.get(b.id)
    const parent = b.parent_id != null ? byId.get(b.parent_id) : null
    let p = [0, 0, 0]
    if (parent && b.orbit_km) {
      const base = place(parent)
      const [x, y, z] = orbits.position(b, parent, days)
      const len = Math.hypot(x, y, z) || 1
      const d = sceneDistance(b.orbit_km, parent, b)
      p = [base[0] + (x / len) * d, base[1] + (y / len) * d, base[2] + (z / len) * d]
    }
    out.set(b.id, p)
    return p
  }
  for (const b of bodies) place(b)
  return out
}
