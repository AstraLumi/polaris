// Pointy-top hexes in axial (q, r) coordinates. Everything here works in
// "world" units; the map view scales and offsets those onto the screen.

export const HEX_SIZE = 24 // center-to-corner distance, in world units
export const SQRT3 = Math.sqrt(3)

// The six corners of a unit hex, pointy-top (first corner at -30 degrees).
export const CORNERS = Array.from({ length: 6 }, (_, i) => {
  const angle = (Math.PI / 180) * (60 * i - 30)
  return [Math.cos(angle), Math.sin(angle)]
})

export const keyOf = (q, r) => `${q},${r}`

export function hexCenter(q, r, size = HEX_SIZE) {
  return [size * SQRT3 * (q + r / 2), size * 1.5 * r]
}

function cubeRound(qf, rf) {
  const xf = qf
  const zf = rf
  const yf = -xf - zf
  let rx = Math.round(xf)
  let ry = Math.round(yf)
  let rz = Math.round(zf)
  const dx = Math.abs(rx - xf)
  const dy = Math.abs(ry - yf)
  const dz = Math.abs(rz - zf)
  if (dx > dy && dx > dz) rx = -ry - rz
  else if (dy > dz) ry = -rx - rz
  else rz = -rx - ry
  return { q: rx + 0, r: rz + 0 } // "+ 0" turns a -0 into 0
}

export function pixelToHex(x, y, size = HEX_SIZE) {
  const q = ((SQRT3 / 3) * x - y / 3) / size
  const r = ((2 / 3) * y) / size
  return cubeRound(q, r)
}

export function hexDistance(a, b) {
  const dq = b.q - a.q
  const dr = b.r - a.r
  return (Math.abs(dq) + Math.abs(dq + dr) + Math.abs(dr)) / 2
}

// Every hex on the straight line from a to b, inclusive.
export function hexLine(a, b) {
  const n = hexDistance(a, b)
  if (n === 0) return [{ q: a.q, r: a.r }]
  const hexes = []
  for (let i = 0; i <= n; i++) {
    const t = i / n
    // The tiny nudge keeps points that land exactly on a hex edge from
    // flip-flopping between two neighbours.
    hexes.push(cubeRound(a.q + (b.q - a.q) * t + 1e-6, a.r + (b.r - a.r) * t + 1e-6))
  }
  return hexes
}
