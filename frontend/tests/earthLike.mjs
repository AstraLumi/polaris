// A rough Earth for the climate tests (climateModel.test.mjs): continents as
// crude outlines in degrees of longitude and latitude, a few mountain ranges,
// deserts, rainforests and ice sheets, painted on a planet grid.
import { PlanetGrid, latLon } from '../src/planetGrid.js'

const CONTINENTS = [
  // North America
  [[-168, 66], [-162, 70], [-140, 70], [-120, 72], [-95, 72], [-80, 73], [-62, 62], [-55, 52], [-66, 45], [-70, 42], [-76, 35], [-81, 31], [-80, 25], [-83, 29], [-90, 30], [-97, 27], [-97, 21], [-90, 21], [-87, 15], [-83, 9], [-78, 8], [-80, 7], [-86, 11], [-92, 15], [-105, 20], [-112, 30], [-117, 32], [-124, 40], [-124, 48], [-133, 56], [-150, 60], [-165, 60]],
  // South America
  [[-78, 8], [-72, 12], [-62, 10], [-50, 0], [-35, -7], [-40, -22], [-48, -28], [-58, -38], [-65, -42], [-68, -52], [-73, -52], [-75, -40], [-72, -30], [-71, -18], [-76, -14], [-81, -6], [-80, 1], [-77, 7]],
  // Africa
  [[-17, 15], [-17, 21], [-10, 30], [-6, 35], [10, 37], [20, 32], [32, 31], [35, 28], [43, 12], [51, 12], [40, -2], [40, -15], [35, -24], [32, -29], [20, -35], [17, -30], [12, -17], [13, -6], [9, 4], [-8, 4], [-14, 10]],
  // Eurasia
  [[-10, 36], [-9, 43], [-1, 46], [-5, 48], [0, 50], [5, 53], [8, 57], [5, 62], [15, 69], [25, 71], [40, 68], [60, 70], [80, 73], [100, 77], [120, 73], [140, 72], [160, 70], [179, 68], [179, 66], [170, 60], [160, 55], [142, 47], [140, 40], [130, 35], [121, 31], [122, 25], [110, 20], [106, 10], [100, 13], [100, 3], [104, 1], [98, 8], [92, 20], [80, 15], [78, 8], [72, 20], [67, 25], [57, 25], [56, 27], [50, 30], [48, 30], [56, 24], [59, 22], [52, 16], [43, 12], [35, 28], [33, 31], [35, 36], [27, 36], [26, 40], [23, 38], [20, 40], [16, 38], [12, 44], [8, 44], [3, 43], [-3, 36]],
  // Australia
  [[114, -22], [122, -18], [130, -12], [137, -12], [142, -11], [146, -19], [153, -26], [150, -37], [141, -38], [135, -35], [129, -32], [115, -34]],
  // Greenland
  [[-55, 60], [-45, 60], [-20, 70], [-18, 80], [-40, 83], [-65, 80], [-60, 70]],
]

function inside(poly, x, y) {
  let ins = false
  for (let i = 0, j = poly.length - 1; i < poly.length; j = i++) {
    const [xi, yi] = poly[i]
    const [xj, yj] = poly[j]
    if (yi > y !== yj > y && x < ((xj - xi) * (y - yi)) / (yj - yi) + xi) ins = !ins
  }
  return ins
}

const box = (lon, lat, [x0, x1, y0, y1]) => lon >= x0 && lon <= x1 && lat >= y0 && lat <= y1

export function earthLike(frequency) {
  const grid = new PlanetGrid(frequency)
  const n = grid.count
  const land = new Uint8Array(n)
  const veg = new Float32Array(n).fill(0.45)
  const albedo = new Float32Array(n).fill(0.2)
  const height = new Float32Array(n)
  const water = new Uint8Array(n)
  for (let c = 0; c < n; c++) {
    const [la, lo] = latLon(grid.center(c))
    const lat = (la * 180) / Math.PI
    const lon = (lo * 180) / Math.PI
    const isLand = lat < -68 || CONTINENTS.some((p) => inside(p, lon, lat))
    if (!isLand) continue
    land[c] = 1
    height[c] = 300
    if (lat < -68 || box(lon, lat, [-60, -15, 60, 84])) {
      albedo[c] = 0.7
      veg[c] = 0
      height[c] = 2200
    } else if (box(lon, lat, [-15, 35, 16, 31]) || box(lon, lat, [40, 58, 16, 30]) || box(lon, lat, [120, 140, -32, -20])) {
      albedo[c] = 0.35
      veg[c] = 0.02
    } else if (box(lon, lat, [-75, -48, -12, 4]) || box(lon, lat, [10, 30, -5, 5]) || box(lon, lat, [95, 120, -5, 8])) {
      albedo[c] = 0.12
      veg[c] = 1
    }
    if (box(lon, lat, [75, 100, 28, 38])) height[c] = 4500
    else if (box(lon, lat, [-78, -66, -45, 5]) && lon < -66 - (lat > -18 ? 0 : 2)) height[c] = 3500
    else if (box(lon, lat, [-122, -105, 35, 58])) height[c] = 2200
  }
  return { grid, land, veg, albedo, height, water }
}

// The map cell nearest a place given in degrees.
export function cellAtDeg(grid, lon, lat) {
  const la = (lat * Math.PI) / 180
  const lo = (lon * Math.PI) / 180
  return grid.cellAt([Math.cos(la) * Math.cos(lo), Math.cos(la) * Math.sin(lo), Math.sin(la)])
}
