import { ref } from 'vue'
import { tr } from './i18n'
import { layersApi } from './api'

// The planet's layers (backend/planetlayers.go): features drawn over the
// map like major locations, one per layer on each hex. The map itself is
// the 'map' layer; the currents are for a later system.
export const LAYERS = [
  { id: 'map', label: tr('Map'), about: tr('Kingdoms, regions, oceans and places') },
  { id: 'plates', label: tr('Tectonic plates'), about: tr('Draw and name the plates the crust is made of'), one: tr('Tectonic plate') },
  { id: 'mountains', label: tr('Mountain ranges'), about: tr('Draw and name the mountain ranges'), one: tr('Mountain range'), value: tr('Highest peak (m)') },
  { id: 'depth', label: tr('Ocean depth'), about: tr('Draw and name the depths of the seas'), one: tr('Ocean depth'), value: tr('Depth (m)') },
  { id: 'wind', label: tr('Wind currents'), later: true },
  { id: 'currents', label: tr('Ocean currents'), later: true },
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

  async function saveFeature(f) {
    const saved = f.id ? await layersApi.update(f.id, f) : await layersApi.create(f)
    await load()
    return saved
  }

  async function removeFeature(f) {
    await layersApi.remove(f.id)
    await load()
  }

  return { features, load, ensure, featureAt, featuresOf, cellsOfFeature, setLocal, save, saveFeature, removeFeature }
}
