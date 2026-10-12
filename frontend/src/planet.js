// The map as part of a planet, the same sums as backend/planet.go: the
// equator across the hexes, and from it latitudes, hemispheres and climate
// zones. eq is the map's equator: { row, center, bend, hex_km, planet_km,
// tilt }. Columns are q + r / 2, so a column runs straight down the map.
// No dependencies: node frontend/tests/planet.test.mjs

// How many hex rows lie between the equator and a pole. Rows are 3/4 of a
// hex's height apart, which is √3/2 of its width.
export const rowsToPole = (eq) => eq.planet_km / 4 / ((eq.hex_km * Math.sqrt(3)) / 2)

// The (fractional) row the line of latitude `lat` runs along at column col;
// lat 0 is the equator. Every line bends like the equator does.
export function latitudeRow(eq, col, lat = 0) {
  const off = (col - eq.center) / 100
  return eq.row + eq.bend * off * off - (lat / 90) * rowsToPole(eq)
}

// The latitude of hex (q, r) in degrees, north positive.
export function latitudeOf(eq, q, r) {
  const lat = ((latitudeRow(eq, q + r / 2) - r) / rowsToPole(eq)) * 90
  return Math.max(-90, Math.min(90, lat))
}

export function zoneOf(eq, lat) {
  const a = Math.abs(lat)
  if (a >= 90 - eq.tilt) return 'Polar'
  if (a < eq.tilt) return 'Tropical'
  return 'Temperate'
}

export function formatLatitude(lat) {
  const v = Math.round(Math.abs(lat))
  if (v === 0) return '0°'
  return `${v}°${lat > 0 ? 'N' : 'S'}`
}

// The lines between zones: the tropics at the tilt, the polar circles at 90
// minus it, north and south. A tilt of 0 or 90 leaves no tropics or circles.
export function zoneLines(eq) {
  const out = []
  const tropic = eq.tilt
  const polar = 90 - eq.tilt
  for (const [lat, kind] of [
    [tropic, 'tropic'],
    [polar, 'polar'],
  ]) {
    if (lat <= 0 || lat >= 90) continue
    if (kind === 'polar' && lat === tropic) continue
    out.push({ lat, kind }, { lat: -lat, kind })
  }
  return out
}
