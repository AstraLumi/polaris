// How the solar system is drawn: true sizes and distances would put every
// planet a pixel wide and a screen apart, so both are squeezed (a power
// below 1 keeps bigger bigger and farther farther). The physics in
// orbits.js always uses the true values; only the picture uses these.
import { orbitPath } from './orbits.js'
import { starColor } from './stars.js'

// The colour a body is drawn in: its own, else a star's by its temperature
// and anything else by its type.
const TYPE_COLORS = { rocky: '#9c8a74', gas: '#d9b38c', ocean: '#3f7fbf' }
export const bodyColor = (b) => b.color || (b.kind === 'star' ? starColor(b) : TYPE_COLORS[b.body_type] || TYPE_COLORS.rocky)

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

// A point `rel` km from a parent, drawn: same direction, squeezed distance.
export function scenePoint(rel, parent, body) {
  const len = Math.hypot(rel[0], rel[1], rel[2]) || 1
  const d = sceneDistance(len, parent, body)
  return [(rel[0] / len) * d, (rel[1] / len) * d, (rel[2] / len) * d]
}

// A body's whole orbit, drawn round its parent (an ellipse squeezed the
// same way as the bodies on it, so they stay on their line).
export const sceneOrbitPath = (body, parent, steps = 128) => orbitPath(body, steps).map((p) => scenePoint(p, parent, body))

// Where each body of a compiled system (orbits.js compileSystem) is drawn,
// from where each is from what it orbits (`rel`, km, three numbers a body,
// in the system's order): the same direction, its distance squeezed, added
// up the tree. Written into `out`.
export function sceneLayout(system, rel, out = new Float64Array(rel.length)) {
  const { list, parent } = system
  for (let i = 0; i < list.length; i++) {
    const o = i * 3
    const p = parent[i]
    if (p < 0) {
      out[o] = out[o + 1] = out[o + 2] = 0
      continue
    }
    const [x, y, z] = [rel[o], rel[o + 1], rel[o + 2]]
    const len = Math.hypot(x, y, z) || 1
    const k = sceneDistance(len, list[p], list[i]) / len
    out[o] = out[p * 3] + x * k
    out[o + 1] = out[p * 3 + 1] + y * k
    out[o + 2] = out[p * 3 + 2] + z * k
  }
  return out
}
