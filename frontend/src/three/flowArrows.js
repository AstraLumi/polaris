// Arrows lying on the globe (flowLines.js gives their paths): each a ribbon
// that bends with the planet, thin and faint at its tail, widening to a
// head, over a soft dark shadow so it reads on any colour. One mesh for all
// of them. Only imported by pages that load three.js lazily.
import * as THREE from 'three'

const LIFT = 1.0025 // just above the surface, under the outlines

const hexRgb = (hex) => {
  const n = parseInt(hex.slice(1), 16)
  return [((n >> 16) & 255) / 255, ((n >> 8) & 255) / 255, (n & 255) / 255]
}

// lines: [{ points: [[x, y, z], …] tail to head, color: '#rrggbb', width }]
// (width: a line's own scale, 1 if none); width: the ribbon's half-width at
// the head, in radians.
export function createFlowArrows(lines, { width = 0.004, opacity = 0.95 } = {}) {
  const pos = []
  const col = []
  const index = []

  // A ribbon along points, its half-width and alpha growing from tail to
  // head, then a triangle for the head.
  function arrow(points, rgb, half, alpha, grow) {
    const n = points.length
    const base = pos.length / 3
    const side = (k) => {
      const p = points[k]
      const a = points[Math.max(0, k - 1)]
      const b = points[Math.min(n - 1, k + 1)]
      const d = [b[0] - a[0], b[1] - a[1], b[2] - a[2]]
      const s = [p[1] * d[2] - p[2] * d[1], p[2] * d[0] - p[0] * d[2], p[0] * d[1] - p[1] * d[0]]
      const len = Math.hypot(...s) || 1
      return [s[0] / len, s[1] / len, s[2] / len]
    }
    // The ribbon stops where the head begins.
    const body = n - 1
    for (let k = 0; k < body; k++) {
      const t = k / Math.max(1, body - 1)
      const w = half * (0.25 + 0.55 * t) * grow
      const s = side(k)
      const p = points[k]
      for (const sign of [1, -1]) {
        pos.push((p[0] + s[0] * w * sign) * LIFT, (p[1] + s[1] * w * sign) * LIFT, (p[2] + s[2] * w * sign) * LIFT)
        col.push(...rgb, alpha * (0.15 + 0.85 * t))
      }
      if (k > 0) {
        const a = base + (k - 1) * 2
        index.push(a, a + 1, a + 2, a + 1, a + 3, a + 2)
      }
    }
    // The head: from the last ribbon point to the tip.
    const from = points[body - 1]
    const tip = points[n - 1]
    const s = side(body - 1)
    const w = half * 2.1 * grow
    const h = pos.length / 3
    pos.push((from[0] + s[0] * w) * LIFT, (from[1] + s[1] * w) * LIFT, (from[2] + s[2] * w) * LIFT)
    pos.push((from[0] - s[0] * w) * LIFT, (from[1] - s[1] * w) * LIFT, (from[2] - s[2] * w) * LIFT)
    pos.push(tip[0] * LIFT, tip[1] * LIFT, tip[2] * LIFT)
    for (let k = 0; k < 3; k++) col.push(...rgb, alpha)
    index.push(h, h + 1, h + 2)
  }

  // Shadows first, so the arrows are drawn over them.
  for (const l of lines) if (l.points.length >= 3) arrow(l.points, [0, 0, 0], width * (l.width ?? 1), 0.35 * opacity, 1.7)
  for (const l of lines) if (l.points.length >= 3) arrow(l.points, hexRgb(l.color), width * (l.width ?? 1), opacity, 1)

  const geo = new THREE.BufferGeometry()
  geo.setAttribute('position', new THREE.Float32BufferAttribute(pos, 3))
  geo.setAttribute('color', new THREE.Float32BufferAttribute(col, 4))
  geo.setIndex(index)
  const mat = new THREE.MeshBasicMaterial({
    vertexColors: true,
    transparent: true,
    depthWrite: false,
    side: THREE.DoubleSide,
    polygonOffset: true,
    polygonOffsetFactor: -3,
    polygonOffsetUnits: -3,
  })
  const mesh = new THREE.Mesh(geo, mat)
  mesh.renderOrder = 2
  return mesh
}
