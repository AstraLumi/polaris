import { reactive } from 'vue'
import { tr } from './i18n'

// Which map the story uses (backend/planetmap.go): "2d" (the flat hex map
// only) or "3d" (a planet; its flat views are projections). Filled with the
// calendar from /settings at startup and whenever Settings saves.
export const worldMap = reactive({
  loaded: false,
  mode: '2d',
  planet: { frequency: 141, planet_km: 40075, tilt: 23.44, bestiary_projection: 'equirectangular', show_equator: true, show_zones: false },
  hex_km: 50,
  cells: 0,
  sky_url: '',
})

// The default sky, also the template a story's own must follow
// (backend/cmd/skygen draws it and describes the format).
export const DEFAULT_SKY = '/sky/default-sky.jpg'
export const skyUrl = () => worldMap.sky_url || DEFAULT_SKY

export function applyMapSettings(s) {
  if (!s) return
  worldMap.mode = s.mode === '3d' ? '3d' : '2d'
  worldMap.planet = { ...worldMap.planet, ...s.planet }
  worldMap.hex_km = s.hex_km
  worldMap.cells = s.cells
  worldMap.sky_url = s.sky_url || ''
  worldMap.loaded = true
}

export const PROJECTIONS = [
  { id: 'equirectangular', label: tr('Equirectangular'), blurb: tr('Latitude and longitude as a grid. Matches the latitudes the 2D map shows.') },
  { id: 'mercator', label: tr('Mercator'), blurb: tr('Keeps shapes but swells the lands near the poles.') },
  { id: 'mollweide', label: tr('Mollweide'), blurb: tr('An oval that keeps every area true.') },
  { id: 'robinson', label: tr('Robinson'), blurb: tr('A rounded compromise, as in many atlases.') },
]

// What a coloured major location is (backend realms).
export const REALMS = [
  { id: 'kingdom', label: tr('Kingdom'), blurb: tr('A realm with a ruler: it gets a faction of the same name.') },
  { id: 'region', label: tr('Region'), blurb: tr('Land with borders but no ruler, too wild for a kingdom. No faction.') },
  { id: 'ocean', label: tr('Ocean'), blurb: tr('A named sea or ocean, drawn as an outline. No faction.') },
  { id: 'unknown', label: tr('Unknown land'), blurb: tr('Not yet explored: grey, and shown on the map as "???". Explored, make it anything else.') },
]

// Every unknown land is this grey (backend unknownColor).
export const UNKNOWN_COLOR = '#c9cacf'

// The name a territory shows on the map: "???" for unknown land.
export const mapName = (m) => (m.realm === 'unknown' ? '???' : m.name)

export const realmLabel = (id) => REALMS.find((r) => r.id === id)?.label || REALMS[0].label

// Messages the server can send back about the planet (backend/planetmap.go);
// listed so they get translated.
export const PLANET_MESSAGES = [
  tr("that hex isn't on the planet"),
  tr('failed to load the planet'),
  tr('failed to port the map'),
  tr('pick a projection'),
  tr('failed to save the map settings'),
  tr('the sky must be a JPEG or PNG image'),
  tr('the sky must be twice as wide as it is tall (for example 4096 × 2048)'),
  tr('the sky must be between 512 and 8192 pixels wide'),
  tr('the sky picture is too large'),
  tr('choose a picture for the sky'),
  tr('failed to save the sky'),
]
