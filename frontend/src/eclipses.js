// Which bodies can cast a shadow on which, in the drawn solar system (the
// squeezed scene of solarScale.js): the star's light is a disc, and a body
// is shadowed wherever another stands between it and part of that disc
// (eclipses, a moon's shadow on its planet, a planet's on its moon). The
// shaders (three/bodyShaders.js) work out how much of the disc is hidden at
// each point; this only picks, for each body, the few others worth asking
// about, so they needn't ask every body in the system.
// No dependencies: node frontend/tests/eclipses.test.mjs
export const MAX_CASTERS = 8

// centers: scene positions, three numbers a body; radii: each body's drawn
// radius; reach: how far from its centre it can catch a shadow (its radius,
// or its rings'); star: the index of the light, lightRadius its size.
// Returns, for each body, the indices of up to MAX_CASTERS bodies that can
// shadow some of it, the most centred first (empty for the star).
export function shadowCasters({ centers, radii, reach = radii, star, lightRadius }) {
  const n = radii.length
  const out = Array.from({ length: n }, () => [])
  if (star == null || star < 0) return out
  const [sx, sy, sz] = [centers[star * 3], centers[star * 3 + 1], centers[star * 3 + 2]]
  for (let i = 0; i < n; i++) {
    if (i === star) continue
    const [cx, cy, cz] = [centers[i * 3], centers[i * 3 + 1], centers[i * 3 + 2]]
    const [tx, ty, tz] = [sx - cx, sy - cy, sz - cz]
    const ds = Math.hypot(tx, ty, tz)
    if (!(ds > 0)) continue
    const found = []
    for (let j = 0; j < n; j++) {
      if (j === i || j === star) continue
      const [vx, vy, vz] = [centers[j * 3] - cx, centers[j * 3 + 1] - cy, centers[j * 3 + 2] - cz]
      // How far towards the star it is, and how far off the line to it.
      const t = (vx * tx + vy * ty + vz * tz) / ds
      if (t <= -radii[j] || t >= ds) continue
      const off = Math.hypot(vx - (tx * t) / ds, vy - (ty * t) / ds, vz - (tz * t) / ds)
      // The shadow (with its soft edge) widens away from the occluder by
      // the light's size over the distance to it.
      const along = Math.max(t, 0)
      const wide = reach[i] + radii[j] + ((lightRadius + reach[i]) * along) / Math.max(ds - along, 1e-9)
      if (off < wide) found.push([off / wide, j])
    }
    found.sort((a, b) => a[0] - b[0])
    for (let k = 0; k < Math.min(found.length, MAX_CASTERS); k++) out[i].push(found[k][1])
  }
  return out
}
