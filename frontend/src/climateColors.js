import { tr } from './i18n'

// How the climate layers colour the map: a heat map for temperature, a
// dry-to-wet scale for rain, the usual Köppen colours for climate types,
// and what each type is called. Numbers come from climateModel.js.

// [value, '#rrggbb'] stops, linear between.
function scale(stops, x) {
  if (!(x > stops[0][0])) return stops[0][1]
  for (let i = 1; i < stops.length; i++) {
    const [b, cb] = stops[i]
    if (x <= b) {
      const [a, ca] = stops[i - 1]
      return mix(ca, cb, (x - a) / (b - a))
    }
  }
  return stops[stops.length - 1][1]
}

function mix(a, b, k) {
  const pa = parseInt(a.slice(1), 16)
  const pb = parseInt(b.slice(1), 16)
  const ch = (p, s) => (p >> s) & 255
  return '#' + [16, 8, 0].map((s) => Math.round(ch(pa, s) + (ch(pb, s) - ch(pa, s)) * k).toString(16).padStart(2, '0')).join('')
}

// °C
export const TEMPERATURE_STOPS = [
  [-40, '#3b0f5e'],
  [-30, '#3a2f9b'],
  [-20, '#2f63c4'],
  [-10, '#4a9ddb'],
  [0, '#a9dcec'],
  [6, '#dff0c8'],
  [12, '#f3e79a'],
  [18, '#f6c35a'],
  [24, '#ee8f3a'],
  [30, '#d6502a'],
  [38, '#9c1c25'],
]
export const temperatureColor = (t) => scale(TEMPERATURE_STOPS, t)

// mm in a year; a month's rain is read on the same scale times 12.
export const RAIN_STOPS = [
  [0, '#7a4f26'],
  [150, '#b88a48'],
  [300, '#dcc477'],
  [600, '#b9d36e'],
  [1000, '#68b25a'],
  [1600, '#2d8f6d'],
  [2500, '#2b6f9f'],
  [4000, '#2c3a8c'],
]
export const rainColor = (mm, perYear = true) => scale(RAIN_STOPS, perYear ? mm : mm * 12)

// Köppen–Geiger: colour and name of each type (Peel et al. colours).
export const KOPPEN = {
  Af: ['#1d24f0', tr('Tropical rainforest')],
  Am: ['#2a78f5', tr('Tropical monsoon')],
  Aw: ['#55aaf2', tr('Tropical savanna')],
  BWh: ['#ec1c24', tr('Hot desert')],
  BWk: ['#f59a9a', tr('Cold desert')],
  BSh: ['#f2a30f', tr('Hot steppe')],
  BSk: ['#fbd96a', tr('Cold steppe')],
  Csa: ['#f8f12a', tr('Hot-summer Mediterranean')],
  Csb: ['#c6c62a', tr('Warm-summer Mediterranean')],
  Csc: ['#959526', tr('Cold-summer Mediterranean')],
  Cwa: ['#96f596', tr('Monsoon subtropical')],
  Cwb: ['#64c864', tr('Subtropical highland')],
  Cwc: ['#349634', tr('Cold subtropical highland')],
  Cfa: ['#c6f550', tr('Humid subtropical')],
  Cfb: ['#66f24f', tr('Oceanic')],
  Cfc: ['#36c51f', tr('Subpolar oceanic')],
  Dsa: ['#f21ef2', tr('Hot-summer continental')],
  Dsb: ['#c41ec4', tr('Warm-summer continental')],
  Dsc: ['#953295', tr('Subarctic')],
  Dsd: ['#966496', tr('Extremely cold subarctic')],
  Dwa: ['#aab0f8', tr('Hot-summer continental')],
  Dwb: ['#5a78dc', tr('Warm-summer continental')],
  Dwc: ['#4b50b4', tr('Subarctic')],
  Dwd: ['#320087', tr('Extremely cold subarctic')],
  Dfa: ['#1ff5f5', tr('Hot-summer continental')],
  Dfb: ['#3dc6f8', tr('Warm-summer continental')],
  Dfc: ['#0b7d7d', tr('Subarctic')],
  Dfd: ['#06465f', tr('Extremely cold subarctic')],
  ET: ['#b2b2b2', tr('Tundra')],
  EF: ['#686868', tr('Ice cap')],
}
export const koppenColor = (code) => KOPPEN[code]?.[0] ?? '#888888'
export const koppenName = (code) => KOPPEN[code]?.[1] ?? ''

// The Köppen legend: one row per kind of climate (types sharing a name
// share a row), in the usual order.
export const KOPPEN_LEGEND = (() => {
  const seen = new Map()
  for (const [code, [color, name]] of Object.entries(KOPPEN)) {
    if (!seen.has(name)) seen.set(name, { name, color, codes: [] })
    seen.get(name).codes.push(code)
  }
  return [...seen.values()]
})()

// A temperature for people: in the story's unit ('C' or 'F'), rounded.
export function formatTemperature(c, unit = 'C') {
  const v = unit === 'F' ? (c * 9) / 5 + 32 : c
  return `${Math.round(v)} °${unit === 'F' ? 'F' : 'C'}`
}

export const formatRain = (mm) => `${Math.round(mm).toLocaleString()} mm`

// The sixteen points of the compass, for winds (where they blow from) and
// currents (where they flow to).
const POINTS = ['N', 'NNE', 'NE', 'ENE', 'E', 'ESE', 'SE', 'SSE', 'S', 'SSW', 'SW', 'WSW', 'W', 'WNW', 'NW', 'NNW']
export function compassPoint(east, north) {
  const angle = Math.atan2(east, north) // 0 = north, clockwise
  const i = Math.round((((angle * 180) / Math.PI + 360) % 360) / 22.5) % 16
  return POINTS[i]
}
