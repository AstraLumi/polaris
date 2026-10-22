import { ref } from 'vue'
import { tr } from './i18n'
import { layersApi } from './api'

// The planet's layers (backend/planetlayers.go): features drawn over the
// map like major locations, one per layer on each hex. The map itself is
// the 'map' layer. The continents layer also carries each hex's biome and
// terrain (biomes.js). The climate layers are worked out from all of it
// (planetClimate.js) and are only looked at, never painted.
export const LAYERS = [
  { id: 'map', label: tr('Map'), about: tr('Kingdoms, regions, oceans and places') },
  { id: 'plates', label: tr('Tectonic plates'), about: tr('Draw and name the plates the crust is made of'), one: tr('Tectonic plate') },
  { id: 'continents', label: tr('Continents'), about: tr('Draw and name the continents, and paint what their land is: forests, deserts, mountains…'), one: tr('Continent') },
  { id: 'depth', label: tr('Ocean depth'), about: tr('Draw and name the depths of the seas'), one: tr('Ocean depth'), value: tr('Depth (m)') },
  { id: 'wind', label: tr('Winds'), about: tr('The prevailing winds the geography makes'), climate: true },
  { id: 'currents', label: tr('Ocean currents'), about: tr('The main ocean currents, warm and cold'), climate: true },
  { id: 'temperature', label: tr('Temperature'), about: tr('A heat map of the planet, month by month'), climate: true },
  { id: 'rain', label: tr('Rain'), about: tr('How much rain and snow falls, month by month'), climate: true },
  { id: 'clouds', label: tr('Clouds'), about: tr('How much of the sky is cloud, month by month'), climate: true },
  { id: 'zones', label: tr('Climate types'), about: tr('Köppen climate types, from the temperatures and the rain'), climate: true },
]
export const layerOf = (id) => LAYERS.find((l) => l.id === id) ?? LAYERS[0]

// '#rrggbb' a part `k` of the way from a to b.
export function mixColor(a, b, k) {
  const pa = parseInt(a.slice(1), 16)
  const pb = parseInt(b.slice(1), 16)
  const ch = (p, s) => (p >> s) & 255
  const out = [16, 8, 0].map((s) => Math.round(ch(pa, s) + (ch(pb, s) - ch(pa, s)) * k))
  return '#' + out.map((x) => x.toString(16).padStart(2, '0')).join('')
}

const SEA = '#1b2d4a' // the globe's own sea (three/globeMesh.js)
const FADED = '#0a0e1a'

// A cell on a layer: its feature's colour over the map, or the map faded
// behind, so the map still shows through.
export function layerCellColor(base, feature) {
  const under = base || SEA
  return feature ? mixColor(feature.color, under, 0.25) : mixColor(under, FADED, 0.6)
}

// The layers' features and painted cells, loaded once, changed on screen
// at once and saved after. Strokes and undo work as on the map: each change
// records what every touched cell held before and after.
export function useLayers() {
  const features = ref([])
  const byId = new Map()
  const cells = new Map() // layer -> Map(cell -> feature id)
  // A hex's tags, apart from the features: its biome and its terrain, each
  // cell -> { key, height }.
  const tags = { biome: new Map(), relief: new Map() }
  let loaded = false
  let saveChain = Promise.resolve()

  const layerCells = (layer) => {
    if (!cells.has(layer)) cells.set(layer, new Map())
    return cells.get(layer)
  }

  async function load() {
    const data = await layersApi.get()
    features.value = data.features
    byId.clear()
    for (const f of data.features) byId.set(f.id, f)
    cells.clear()
    for (const [id, cell] of data.cells) {
      const f = byId.get(id)
      if (f) layerCells(f.layer).set(cell, id)
    }
    tags.biome.clear()
    tags.relief.clear()
    for (const [cell, key, height] of data.biomes ?? []) tags.biome.set(cell, { key, height })
    for (const [cell, key, height] of data.relief ?? []) tags.relief.set(cell, { key, height })
    loaded = true
  }

  const ensure = () => (loaded ? Promise.resolve() : load())
  const featureAt = (layer, cell) => byId.get(layerCells(layer).get(cell)) ?? null
  const featuresOf = (layer) => features.value.filter((f) => f.layer === layer)
  const cellsOfFeature = (f) => [...layerCells(f.layer)].filter(([, id]) => id === f.id).map(([c]) => c)

  // Sets a cell to a feature (or none); true when it changed.
  function setLocal(layer, cell, id) {
    const m = layerCells(layer)
    if ((m.get(cell) ?? null) === id) return false
    if (id == null) m.delete(cell)
    else m.set(cell, id)
    return true
  }

  // Saves cells set to `after` (a feature id, or null to erase), in order
  // with the strokes before it; onError is told when it fails.
  function save(layer, changes, onError) {
    const paint = new Map()
    const erase = []
    for (const { c, after } of changes) {
      if (after == null) erase.push(c)
      else (paint.get(after) ?? paint.set(after, []).get(after)).push(c)
    }
    saveChain = saveChain.then(async () => {
      try {
        if (erase.length) await layersApi.erase(layer, erase)
        for (const [id, list] of paint) await layersApi.paint(id, list)
      } catch (err) {
        await load().catch(() => {})
        onError(err)
      }
    })
    return saveChain
  }

  // ---- Tags: a biome and a terrain per hex, each { key, height }.
  // kind is 'biome' or 'relief'.
  const tagAt = (kind, cell) => tags[kind].get(cell) ?? null
  const sameTag = (a, b) => (a?.key ?? null) === (b?.key ?? null) && (a?.height ?? null) === (b?.height ?? null)

  // Sets a cell's tag ({ key, height } or null); true when it changed.
  function setTagLocal(kind, cell, value) {
    if (sameTag(tags[kind].get(cell), value)) return false
    if (value) tags[kind].set(cell, { key: value.key, height: value.height ?? null })
    else tags[kind].delete(cell)
    return true
  }

  const TAG_API = {
    biome: { paint: (key, height, cells) => layersApi.paintBiomes(key, height, cells), erase: (cells) => layersApi.eraseBiomes(cells) },
    relief: { paint: (key, height, cells) => layersApi.paintRelief(key, height, cells), erase: (cells) => layersApi.eraseRelief(cells) },
  }

  // Saves cells set to `after` ({ key, height } or null), in order with
  // the other strokes.
  function saveTags(kind, changes, onError) {
    const paint = new Map()
    const erase = []
    for (const { c, after } of changes) {
      if (!after) erase.push(c)
      else {
        const id = `${after.key}|${after.height ?? ''}`
        if (!paint.has(id)) paint.set(id, { key: after.key, height: after.height ?? null, cells: [] })
        paint.get(id).cells.push(c)
      }
    }
    saveChain = saveChain.then(async () => {
      try {
        if (erase.length) await TAG_API[kind].erase(erase)
        for (const p of paint.values()) await TAG_API[kind].paint(p.key, p.height, p.cells)
      } catch (err) {
        await load().catch(() => {})
        onError(err)
      }
    })
    return saveChain
  }

  async function saveFeature(f) {
    const saved = f.id ? await layersApi.update(f.id, f) : await layersApi.create(f)
    await load()
    return saved
  }

  async function removeFeature(f) {
    await layersApi.remove(f.id)
    await load()
  }

  return {
    features,
    load,
    ensure,
    featureAt,
    featuresOf,
    cellsOfFeature,
    setLocal,
    save,
    saveFeature,
    removeFeature,
    tagAt,
    setTagLocal,
    saveTags,
    tagCells: (kind) => [...tags[kind].keys()],
  }
}
