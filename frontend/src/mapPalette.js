// A swatch palette for kingdom colors: 14 hues x 5 lightness steps, plus a
// row of greys and browns — 84 colors, stored as plain #rrggbb strings.

function hslToHex(h, s, l) {
  s /= 100
  l /= 100
  const k = (n) => (n + h / 30) % 12
  const a = s * Math.min(l, 1 - l)
  const f = (n) => l - a * Math.max(-1, Math.min(k(n) - 3, Math.min(9 - k(n), 1)))
  const toHex = (x) => Math.round(255 * x).toString(16).padStart(2, '0')
  return `#${toHex(f(0))}${toHex(f(8))}${toHex(f(4))}`
}

const HUES = [0, 18, 36, 52, 80, 120, 155, 180, 200, 220, 250, 275, 300, 330]
const LIGHTNESS = [30, 42, 54, 66, 78]

const hueRows = LIGHTNESS.flatMap((l) => HUES.map((h) => hslToHex(h, 62, l)))
const greys = [14, 26, 38, 50, 62, 74, 86].map((l) => hslToHex(220, 6, l))
const browns = [20, 30, 40, 50, 60, 70, 80].map((l) => hslToHex(28, 35, l))

export const MAP_PALETTE = [...hueRows, ...greys, ...browns]
