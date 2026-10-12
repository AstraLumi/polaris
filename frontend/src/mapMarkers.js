import { themeRgba } from './theme'

// The map's place markers: a dot for a place, a castle medallion for a city
// and a glowing star for a capital. Drawn in world units at zoom z (line
// widths stay the same on screen). Used by the 2D map, and as pictures by
// the planet (markerIcon).

export function drawDot(ctx, cx, cy, rad, z) {
  ctx.beginPath()
  ctx.arc(cx, cy, rad, 0, Math.PI * 2)
  ctx.fillStyle = themeRgba('--accent')
  ctx.fill()
  ctx.strokeStyle = themeRgba('--surface', 0.95)
  ctx.lineWidth = 1.5 / z
  ctx.stroke()
}

// A city: a dark medallion with a little castle in the accent colour.
export function drawCity(ctx, cx, cy, r, z) {
  ctx.beginPath()
  ctx.arc(cx, cy, r, 0, Math.PI * 2)
  ctx.fillStyle = themeRgba('--surface', 0.96)
  ctx.fill()
  ctx.strokeStyle = themeRgba('--accent')
  ctx.lineWidth = Math.max(1.4, r * z * 0.11) / z
  ctx.stroke()

  const u = (r * 0.62) / 6 // the castle is drawn on a 12 x 11 grid
  ctx.save()
  ctx.translate(cx, cy + u * 0.4)
  ctx.scale(u, u)
  ctx.beginPath()
  ctx.moveTo(-6, 5.5)
  ctx.lineTo(-6, -5.5)
  ctx.lineTo(-3.6, -5.5)
  ctx.lineTo(-3.6, -3.4)
  ctx.lineTo(-1.2, -3.4)
  ctx.lineTo(-1.2, -5.5)
  ctx.lineTo(1.2, -5.5)
  ctx.lineTo(1.2, -3.4)
  ctx.lineTo(3.6, -3.4)
  ctx.lineTo(3.6, -5.5)
  ctx.lineTo(6, -5.5)
  ctx.lineTo(6, 5.5)
  ctx.closePath()
  ctx.fillStyle = themeRgba('--accent')
  ctx.fill()
  // The gate.
  ctx.beginPath()
  ctx.moveTo(-1.7, 5.5)
  ctx.lineTo(-1.7, 1.6)
  ctx.arc(0, 1.6, 1.7, Math.PI, 0)
  ctx.lineTo(1.7, 5.5)
  ctx.closePath()
  ctx.fillStyle = themeRgba('--surface', 0.96)
  ctx.fill()
  ctx.restore()
}

// A capital: a glowing four-pointed star (like the Polaris logo) rising out of
// a ring, larger than anything else on the map.
export function drawCapital(ctx, cx, cy, R, z) {
  const glow = ctx.createRadialGradient(cx, cy, R * 0.2, cx, cy, R * 2.1)
  glow.addColorStop(0, themeRgba('--accent', 0.38))
  glow.addColorStop(1, themeRgba('--accent', 0))
  ctx.beginPath()
  ctx.arc(cx, cy, R * 2.1, 0, Math.PI * 2)
  ctx.fillStyle = glow
  ctx.fill()

  ctx.beginPath()
  ctx.arc(cx, cy, R * 0.56, 0, Math.PI * 2)
  ctx.fillStyle = themeRgba('--surface', 0.92)
  ctx.fill()
  ctx.strokeStyle = themeRgba('--accent')
  ctx.lineWidth = Math.max(1.4, R * z * 0.07) / z
  ctx.stroke()

  const star = (len, waist, rot) => {
    ctx.save()
    ctx.translate(cx, cy)
    ctx.rotate(rot)
    ctx.beginPath()
    ctx.moveTo(0, -len)
    ctx.quadraticCurveTo(waist, -waist, len, 0)
    ctx.quadraticCurveTo(waist, waist, 0, len)
    ctx.quadraticCurveTo(-waist, waist, -len, 0)
    ctx.quadraticCurveTo(-waist, -waist, 0, -len)
    ctx.closePath()
    ctx.restore()
  }
  star(R, R * 0.13, 0)
  ctx.fillStyle = themeRgba('--accent')
  ctx.fill()
  ctx.strokeStyle = themeRgba('--surface', 0.95)
  ctx.lineWidth = 1.4 / z
  ctx.stroke()
  star(R * 0.5, R * 0.1, Math.PI / 4)
  ctx.fillStyle = themeRgba('--accent', 0.85)
  ctx.fill()

  ctx.beginPath()
  ctx.arc(cx, cy, R * 0.1, 0, Math.PI * 2)
  ctx.fillStyle = themeRgba('--surface')
  ctx.fill()
}

// A marker as a picture, px pixels across (the capital's glow included), for
// the planet's views. Cached per theme, as the colours follow it.
const icons = new Map()
export function markerIcon(rank, px) {
  const key = `${rank}|${px}|${themeRgba('--accent')}|${themeRgba('--surface')}`
  let url = icons.get(key)
  if (!url) {
    const dpr = 2
    const canvas = document.createElement('canvas')
    canvas.width = canvas.height = px * dpr
    const ctx = canvas.getContext('2d')
    ctx.scale(dpr, dpr)
    const c = px / 2
    // The capital's glow reaches 2.1 times its radius.
    if (rank === 2) drawCapital(ctx, c, c, px / 2 / 2.1, 1)
    else if (rank === 1) drawCity(ctx, c, c, px / 2 - 1, 1)
    else drawDot(ctx, c, c, px / 2 - 1, 1)
    url = canvas.toDataURL('image/png')
    icons.set(key, url)
  }
  return url
}

// A #rrggbb colour moved towards white by t (0–1): territory names over a
// dark sea.
export function lighten(hex, t) {
  const n = parseInt(hex.slice(1), 16)
  const ch = (v) => Math.round(v + (255 - v) * t)
  return `rgb(${ch((n >> 16) & 255)}, ${ch((n >> 8) & 255)}, ${ch(n & 255)})`
}
