// Map projections between the planet (longitude, latitude in radians) and a
// flat map (x east, y north, in planet radii); the same as
// backend/projection.go. inverse answers null off the map.
// No dependencies: node frontend/tests/planetGrid.test.mjs

const MERCATOR_MAX = (85.0511 * Math.PI) / 180
const SQRT2 = Math.SQRT2

function mollweideTheta(lat) {
  if (Math.abs(lat) >= Math.PI / 2 - 1e-9) return Math.sign(lat) * (Math.PI / 2)
  const target = Math.PI * Math.sin(lat)
  let theta = lat
  for (let k = 0; k < 30; k++) {
    const step = (2 * theta + Math.sin(2 * theta) - target) / (2 + 2 * Math.cos(2 * theta))
    theta -= step
    if (Math.abs(step) < 1e-12) break
  }
  return theta
}

const RX = [1.0, 0.9986, 0.9954, 0.99, 0.9822, 0.973, 0.96, 0.9427, 0.9216, 0.8962, 0.8679, 0.835, 0.7986, 0.7597, 0.7186, 0.6732, 0.6213, 0.5722, 0.5322]
const RY = [0.0, 0.062, 0.124, 0.186, 0.248, 0.31, 0.372, 0.434, 0.4958, 0.5571, 0.6176, 0.6769, 0.7346, 0.7903, 0.8435, 0.8936, 0.9394, 0.9761, 1.0]

function robinsonAt(lat) {
  const deg = Math.min(90, (Math.abs(lat) * 180) / Math.PI)
  const k = Math.min(17, Math.floor(deg / 5))
  const f = (deg - k * 5) / 5
  return [RX[k] + (RX[k + 1] - RX[k]) * f, RY[k] + (RY[k + 1] - RY[k]) * f]
}

const IMPL = {
  equirectangular: {
    forward: (lon, lat) => [lon, lat],
    inverse: (x, y) => (Math.abs(x) <= Math.PI && Math.abs(y) <= Math.PI / 2 ? [x, y] : null),
  },
  mercator: {
    forward: (lon, lat) => {
      const l = Math.max(-MERCATOR_MAX, Math.min(MERCATOR_MAX, lat))
      return [lon, Math.log(Math.tan(Math.PI / 4 + l / 2))]
    },
    inverse: (x, y) => (Math.abs(x) <= Math.PI ? [x, 2 * Math.atan(Math.exp(y)) - Math.PI / 2] : null),
  },
  mollweide: {
    forward: (lon, lat) => {
      const theta = mollweideTheta(lat)
      return [((2 * SQRT2) / Math.PI) * lon * Math.cos(theta), SQRT2 * Math.sin(theta)]
    },
    inverse: (x, y) => {
      if (Math.abs(y) > SQRT2) return null
      const theta = Math.asin(y / SQRT2)
      const lat = Math.asin(Math.max(-1, Math.min(1, (2 * theta + Math.sin(2 * theta)) / Math.PI)))
      const c = Math.cos(theta)
      if (c < 1e-12) return Math.abs(x) < 1e-9 ? [0, lat] : null
      const lon = (Math.PI * x) / (2 * SQRT2 * c)
      return Math.abs(lon) <= Math.PI ? [lon, lat] : null
    },
  },
  robinson: {
    forward: (lon, lat) => {
      const [px, py] = robinsonAt(lat)
      return [0.8487 * px * lon, Math.sign(lat) * 1.3523 * py]
    },
    inverse: (x, y) => {
      const py = Math.abs(y) / 1.3523
      if (py > 1) return null
      let k = 0
      while (k < 17 && RY[k + 1] < py) k++
      const f = (py - RY[k]) / (RY[k + 1] - RY[k])
      const lat = (Math.sign(y) || 1) * (((k + f) * 5 * Math.PI) / 180)
      const [px] = robinsonAt(lat)
      const lon = x / (0.8487 * px)
      return Math.abs(lon) <= Math.PI ? [lon, lat] : null
    },
  },
}

export const projectionOf = (id) => IMPL[id] || IMPL.equirectangular

// The edge of the whole map as a closed outline of [x, y] points, for
// clipping and for drawing the sea.
export function outline(id) {
  const p = projectionOf(id)
  const pts = []
  const steps = 72
  const top = id === 'mercator' ? MERCATOR_MAX : Math.PI / 2
  for (let k = 0; k <= steps; k++) pts.push(p.forward(Math.PI, top - (2 * top * k) / steps))
  for (let k = 0; k <= steps; k++) pts.push(p.forward(-Math.PI, -top + (2 * top * k) / steps))
  return pts
}
