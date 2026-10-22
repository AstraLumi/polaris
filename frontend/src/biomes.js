import { tr } from './i18n'

// What the land of the planet is, hex by hex, painted on the Continents
// layer, in two independent parts (backend/planetlayers.go keeps the same
// keys in `biomeKeys` and `reliefKeys`; change both together):
//   the biome    what covers the ground: forest, grassland, desert, ice…
//   the terrain  the ground's shape and height: hills, a plateau,
//                mountains (LANDFORMS), or lowland when none is painted.
// So a high plateau can be covered in boreal forest, mountains in ice.
//
// Each biome carries what the climate (climateModel.js) reads from it:
//   veg     how dense its plants are, 0 to 1: plants give moisture back to
//           the air, so dense vegetation keeps rain going further inland
//   height  the usual height of its ground in metres, when the hex has no
//           terrain painted (lowlands; an ice sheet stands high by itself)
//   albedo  how much sunlight its ground reflects
//   water   inland water: it gives moisture like the sea does
//   heat    the ground's own heat in W/m² (volcanic land): it warms the
//           hex and its surroundings, melts snow, draws in a little wind
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
      { key: 'volcanic', label: tr('Volcanic land'), color: '#5a4a48', veg: 0.05, height: 500, albedo: 0.12, heat: 40 },
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
]

export const BIOMES = BIOME_GROUPS.flatMap((g) => g.biomes.map((b) => ({ ...b, group: g.id })))
const byKey = new Map(BIOMES.map((b) => [b.key, b]))
export const biomeOf = (key) => byKey.get(key) ?? null

// The shapes of the land, from low to high, each with its usual height
// (metres; a hex can be painted with its own), how much it lightens the
// biome's colour on the map (shade), and its own colour where no biome is
// painted.
export const LANDFORMS = [
  { key: 'hills', label: tr('Hills'), height: 600, shade: 0.16, color: '#9c8f6c' },
  { key: 'plateau', label: tr('Plateau'), height: 1500, shade: 0.3, color: '#a8906a' },
  { key: 'mountains', label: tr('Mountains'), height: 3000, shade: 0.45, color: '#8a7f75' },
]
const landformByKey = new Map(LANDFORMS.map((l) => [l.key, l]))
export const landformOf = (key) => landformByKey.get(key) ?? null

// Land painted on a continent but given no biome: what the climate assumes.
export const PLAIN_LAND = { key: '', label: tr('Land'), color: '#7d8a5a', veg: 0.45, height: 300, albedo: 0.2 }

// The height of a hex's ground from what is painted on it (each tag
// { key, height }, planetLayers.js): its terrain at its own height or its
// landform's usual one; with no terrain, the biome's (a height kept from
// before terrain existed, else the biome's usual one); else plain land's.
export function groundHeight(biome, relief) {
  const form = relief && landformOf(relief.key)
  if (form) return relief.height != null ? relief.height : form.height
  if (biome?.height != null) return biome.height
  return (biomeOf(biome?.key) ?? PLAIN_LAND).height
}
