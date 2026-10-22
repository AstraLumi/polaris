import { tr } from './i18n'

// What the land of the planet is, hex by hex: the biomes painted on the
// Continents layer (backend/planetlayers.go keeps the same keys in
// `biomeKeys`; change both together). Grouped into categories for the
// picker. Each biome carries what the climate (climateModel.js) reads from
// it:
//   veg     how dense its plants are, 0 to 1: plants give moisture back to
//           the air, so dense vegetation keeps rain going further inland
//   height  the usual height of its ground in metres, when the hex has none
//           of its own (mountains painted with a peak height keep theirs)
//   albedo  how much sunlight its ground reflects
//   water   inland water: it gives moisture like the sea does
// No dependencies besides the translations' tr().
export const BIOME_GROUPS = [
  {
    id: 'forests',
    label: tr('Forests'),
    biomes: [
      { key: 'rainforest', label: tr('Tropical rainforest'), color: '#1f6b3a', veg: 1, height: 200, albedo: 0.12 },
      { key: 'dry_forest', label: tr('Tropical dry forest'), color: '#6f8f3a', veg: 0.7, height: 300, albedo: 0.15 },
      { key: 'temperate_rainforest', label: tr('Temperate rainforest'), color: '#2f7a5a', veg: 0.95, height: 300, albedo: 0.13 },
      { key: 'temperate_forest', label: tr('Temperate forest'), color: '#3f8a4a', veg: 0.8, height: 300, albedo: 0.15 },
      { key: 'boreal_forest', label: tr('Boreal forest'), color: '#2e5e4e', veg: 0.6, height: 300, albedo: 0.14 },
      { key: 'mangrove', label: tr('Mangroves'), color: '#3e7c63', veg: 0.9, height: 5, albedo: 0.12 },
    ],
  },
  {
    id: 'grasslands',
    label: tr('Grasslands and shrubs'),
    biomes: [
      { key: 'savanna', label: tr('Savanna'), color: '#b9a34a', veg: 0.4, height: 500, albedo: 0.2 },
      { key: 'grassland', label: tr('Grassland'), color: '#9dbb5a', veg: 0.45, height: 300, albedo: 0.2 },
      { key: 'steppe', label: tr('Steppe'), color: '#b5b77a', veg: 0.25, height: 500, albedo: 0.23 },
      { key: 'shrubland', label: tr('Mediterranean shrubland'), color: '#a39a5a', veg: 0.35, height: 300, albedo: 0.2 },
      { key: 'farmland', label: tr('Farmland'), color: '#c9c26a', veg: 0.5, height: 200, albedo: 0.2 },
    ],
  },
  {
    id: 'dry',
    label: tr('Deserts and barren land'),
    biomes: [
      { key: 'desert', label: tr('Hot desert'), color: '#e2c27a', veg: 0.02, height: 400, albedo: 0.35 },
      { key: 'cold_desert', label: tr('Cold desert'), color: '#c8b99a', veg: 0.05, height: 1000, albedo: 0.3 },
      { key: 'salt_flat', label: tr('Salt flats'), color: '#e8e4d8', veg: 0, height: 200, albedo: 0.45 },
      { key: 'barren', label: tr('Barren rock'), color: '#8c8580', veg: 0, height: 500, albedo: 0.25 },
    ],
  },
  {
    id: 'cold',
    label: tr('Cold lands'),
    biomes: [
      { key: 'tundra', label: tr('Tundra'), color: '#9fae9a', veg: 0.15, height: 200, albedo: 0.2 },
      { key: 'ice', label: tr('Ice sheet'), color: '#eef4f8', veg: 0, height: 2000, albedo: 0.7 },
    ],
  },
  {
    id: 'wet',
    label: tr('Wetlands and lakes'),
    biomes: [
      { key: 'wetland', label: tr('Wetlands'), color: '#4f8a7a', veg: 0.8, height: 50, albedo: 0.12 },
      { key: 'lake', label: tr('Lake'), color: '#4a7fb0', veg: 0, height: 100, albedo: 0.08, water: true },
    ],
  },
  {
    id: 'highlands',
    label: tr('Highlands'),
    biomes: [
      { key: 'mountains', label: tr('Mountains'), color: '#8a7f75', veg: 0.2, height: 3000, albedo: 0.22, peak: true },
      { key: 'hills', label: tr('Hills'), color: '#9c8f6c', veg: 0.4, height: 600, albedo: 0.18, peak: true },
      { key: 'plateau', label: tr('Plateau'), color: '#a8906a', veg: 0.25, height: 1500, albedo: 0.22, peak: true },
      { key: 'volcanic', label: tr('Volcanic land'), color: '#5a4a48', veg: 0.05, height: 1500, albedo: 0.12, peak: true },
    ],
  },
]

export const BIOMES = BIOME_GROUPS.flatMap((g) => g.biomes.map((b) => ({ ...b, group: g.id })))
const byKey = new Map(BIOMES.map((b) => [b.key, b]))
export const biomeOf = (key) => byKey.get(key) ?? null

// Land painted on a continent but given no biome: what the climate assumes.
export const PLAIN_LAND = { key: '', label: tr('Land'), color: '#7d8a5a', veg: 0.45, height: 300, albedo: 0.2 }

// The height of a hex's ground: its own (painted with a height), or its
// biome's usual one.
export const groundHeight = (biome, height) => (height != null ? height : (biome ?? PLAIN_LAND).height)
