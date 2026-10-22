<template>
  <div class="map-page">
    <div class="map-wrap" :class="{ 'has-list': !listClosed }">
      <PlanetGlobe
        v-if="ready && view === 'globe'"
        ref="globeRef"
        :frequency="frequency"
        :color-of="colorOf"
        :version="version"
        :markers="markers"
        :lines="lines"
        :tool="tool"
        :brush="brush"
        :selected="selectedCell"
        :outlined="outlined"
        :focus="homeCell"
        :areas="areas"
        :borders="borders"
        :inset-right="listInset"
        :sky="skyUrl()"
        :flows="flows"
        @hover="onHover"
        @select="onSelect"
        @paint-start="onPaintStart"
        @paint="onPaint"
        @paint-end="onPaintEnd"
      />
      <PlanetFlatMap
        v-else-if="ready"
        :frequency="frequency"
        :projection="flatProjection"
        :cells="coloredCells"
        :color-of="colorOf"
        :version="version"
        :markers="markers"
        :lines="lines"
        :areas="areas"
        :borders="borders"
        :inset-right="listInset"
        @hover="onHover"
      />

      <div class="toolbar glass-panel panel-solid">
        <button type="button" class="tool-btn" :class="{ active: view === 'globe' }" :title="$t('The planet, to paint on')" @click="setView('globe')">
          {{ $t('Globe') }}
        </button>
        <button type="button" class="tool-btn" :class="{ active: view === 'flat' }" :title="$t('The planet laid flat in a projection, to look at')" @click="setView('flat')">
          {{ $t('Flat') }}
        </button>
        <div class="tool-sep"></div>
        <template v-if="view === 'globe'">
          <button v-for="t in shownTools" :key="t.key" type="button" class="tool-btn" :class="{ active: tool === t.key }" :title="$t(t.title)" @click="tool = t.key">
            {{ $t(t.label) }}
          </button>
          <template v-if="tool === 'paint' || tool === 'erase'">
            <div class="tool-sep"></div>
            <button
              v-for="b in BRUSHES"
              :key="b.radius"
              type="button"
              class="tool-btn brush-btn"
              :class="{ active: brush === b.radius }"
              :title="$tn('Brush: {n} hex', 'Brush: {n} hexes', b.hexes)"
              :aria-label="$tn('Brush: {n} hex', 'Brush: {n} hexes', b.hexes)"
              @click="brush = b.radius"
            >
              <i class="brush-dot" :style="{ width: b.dot + 'px', height: b.dot + 'px' }"></i>
            </button>
          </template>
          <div class="tool-sep"></div>
          <button type="button" class="tool-btn history-btn" :disabled="!undoStack.length" :title="$t('Undo painting (Ctrl+Z)')" @click="undo">↶</button>
          <button type="button" class="tool-btn history-btn" :disabled="!redoStack.length" :title="$t('Redo painting (Ctrl+Shift+Z)')" @click="redo">↷</button>
        </template>
        <select v-else v-model="flatProjection" class="proj-select" :aria-label="$t('Projection')">
          <option v-for="p in PROJECTIONS" :key="p.id" :value="p.id">{{ $t(p.label) }}</option>
        </select>
        <div class="tool-sep"></div>
        <button type="button" class="tool-btn" :class="{ active: lines.equator }" :title="$t('Show the equator')" @click="toggleLine('show_equator')">
          {{ $t('Equator') }}
        </button>
        <button type="button" class="tool-btn" :class="{ active: lines.zones }" :title="$t('Show the tropics and polar circles')" @click="toggleLine('show_zones')">
          {{ $t('Zones') }}
        </button>
        <div class="tool-sep"></div>
        <button type="button" class="tool-btn" :class="{ active: oceanView.shown }" :title="$t('Show the oceans and their names')" @click="oceanView.shown = !oceanView.shown">
          {{ $t('Oceans') }}
        </button>
        <template v-if="oceanView.shown">
          <button type="button" class="tool-btn" :class="{ active: oceanView.tint }" :title="$t('Tint the oceans with their colour')" @click="oceanView.tint = !oceanView.tint">
            {{ $t('Tint') }}
          </button>
          <button type="button" class="tool-btn" :class="{ active: oceanView.outline }" :title="$t('Outline the oceans')" @click="oceanView.outline = !oceanView.outline">
            {{ $t('Outlines') }}
          </button>
        </template>
      </div>

      <div class="top-notes">
        <p class="hint-line">{{ hint }}</p>
        <p v-if="hoverLatitude" class="lat-line">{{ hoverLatitude }}</p>
        <p v-if="notice" class="error-banner notice">{{ notice }}</p>
      </div>

      <MapHoverPane :entries="hoverEntries" />

      <HexPanel
        v-if="selectedCell != null && panel"
        :hex="{ cell: selectedCell, label: cellLabel(selectedCell) }"
        :panel="panel"
        :all-majors="majors"
        :unplaced="unplaced"
        :save-minor="saveMinor"
        :delete-minor="deleteMinor"
        :place-location="placeLocation"
        @close="closePanel"
        @edit-major="editMajor = $event"
      />

      <MajorLocationList
        :majors="onClimate ? [] : onLayer ? layerFeatures : majors"
        :active-id="onLayer ? activeFeatureId : activeId"
        :title="onLayer || onClimate ? layerInfo.label : undefined"
        :empty-text="onClimate ? '' : onLayer ? tr('Nothing drawn on this layer yet') : undefined"
        :groups="onLayer ? [{ id: layer, label: layerInfo.label }] : null"
        :group-of="onLayer ? (f) => f.layer : null"
        :can-add="!onClimate"
        @pick="onListPick"
        @edit="onListEdit"
        @new="onListNew"
        @closed="listClosed = $event"
      >
        <template v-if="layer === 'continents'" #extra>
          <BiomePalette v-model="pickedBiome" v-model:height="biomeHeight" :counts="biomeCounts" />
        </template>
        <template v-else-if="onClimate" #extra>
          <ClimatePanel
            v-model:month="climateMonth"
            v-model:arrows="arrows"
            :layer="layer"
            :months="calendar.months_per_year"
            :working="climate.working.value"
            :error="climate.error.value"
            :assumed="sunlight.assumed"
            :unit="calendar.seasons?.unit || 'C'"
          />
        </template>
        <template #footer>
          <LayerPicker v-model="layer" />
          <PlanetSwitch />
        </template>
      </MajorLocationList>
    </div>

    <FeatureModal
      v-if="featureModal"
      :layer-id="layer"
      :feature="featureModal.feature"
      :save="layers.saveFeature"
      :remove="layers.removeFeature"
      @close="featureModal = null"
      @saved="onFeatureSaved"
      @deleted="onFeatureDeleted"
    />

    <MajorLocationModal
      v-if="newMajorOpen || editMajor"
      :location="editMajor"
      :taken-colors="takenColors"
      @close="closeMajorModal"
      @saved="onMajorSaved"
      @deleted="onMajorDeleted"
    />

    <ConfirmDialog
      v-if="bucketAsk"
      :title="$tn('Fill {n} hex?', 'Fill {n} hexes?', bucketAsk.cells.length)"
      :message="$t('That is a big area. It is painted with {name}; Undo takes it back.', { name: bucketAsk.name })"
      :confirm-label="$t('Fill')"
      @cancel="bucketAsk = null"
      @confirm="confirmBucket"
    />
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { useRoute } from 'vue-router'
import { t, tr } from '../i18n'
import { planetApi, mapApi, solarApi } from '../api'
import { worldMap, PROJECTIONS, realmLabel, applyMapSettings, mapName, skyUrl } from '../worldMap'
import { PlanetGrid, latLon, middleCell } from '../planetGrid'
import { cellColor } from '../planetColors'
import { oceanView } from '../oceanView'
import { areaFontPx } from '../planetLabels'
import { formatLatitude, zoneOf } from '../planet'
import PlanetGlobe from '../components/PlanetGlobe.vue'
import PlanetFlatMap from '../components/PlanetFlatMap.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import { floodFill, BUCKET_WARN } from '../bucketFill'
import MapHoverPane from '../components/MapHoverPane.vue'
import MajorLocationList from '../components/MajorLocationList.vue'
import HexPanel from '../components/HexPanel.vue'
import MajorLocationModal from '../components/MajorLocationModal.vue'
import LayerPicker from '../components/LayerPicker.vue'
import PlanetSwitch from '../components/PlanetSwitch.vue'
import FeatureModal from '../components/FeatureModal.vue'
import BiomePalette from '../components/BiomePalette.vue'
import ClimatePanel from '../components/ClimatePanel.vue'
import { useLayers, layerOf, layerCellColor, mixColor } from '../planetLayers'
import { biomeOf, PLAIN_LAND } from '../biomes'
import { calendar } from '../calendar'
import { usePlanetClimate, climateInput, heatOf } from '../planetClimate'
import { temperatureColor, rainColor, koppenColor, koppenName, formatTemperature, formatRain, compassPoint } from '../climateColors'

// The map when the story is a planet (backend/planetmap.go): painted on the
// globe; the flat views are projections to look at. Mirrors MapView.vue
// (the 2D map) in what it shows and how painting, undo and the panels work.

const route = useRoute()

const tools = [
  { key: 'select', label: tr('Select'), title: tr('Click a hex to inspect it and add locations. Drag to turn the globe.') },
  { key: 'paint', label: tr('Paint'), title: tr('Paint hexes with the selected major location') },
  { key: 'erase', label: tr('Erase'), title: tr('Erase the selected major location from hexes (everything, if none is selected)') },
  { key: 'fill', label: tr('Bucket'), title: tr('Fill the empty hexes connected to the clicked one with the selected major location') },
]
const BRUSHES = [
  { radius: 0, hexes: 1, dot: 5 },
  { radius: 1, hexes: 7, dot: 9 },
  { radius: 2, hexes: 19, dot: 13 },
]

const VIEW_KEY = 'polaris-planet-view'
const PROJ_KEY = 'polaris-planet-projection'
const remembered = (key, fallback) => {
  try {
    return localStorage.getItem(key) || fallback
  } catch {
    return fallback
  }
}
const remember = (key, value) => {
  try {
    localStorage.setItem(key, value)
  } catch {
    // Not remembered; nothing else depends on it.
  }
}

const ready = ref(false)
const view = ref(remembered(VIEW_KEY, 'globe') === 'flat' ? 'flat' : 'globe')
const flatProjection = ref(remembered(PROJ_KEY, 'equirectangular'))
const tool = ref('select')
const brush = ref(0)
const locations = ref([])
const activeId = ref(null)
const focusId = ref(null)
const selectedCell = ref(null)
const panel = ref(null)
const hoverCell = ref(null)
const hoverEntries = ref([])
const newMajorOpen = ref(false)
const editMajor = ref(null)
const notice = ref('')
const version = ref(0)
const globeRef = ref(null)
const listClosed = ref(false) // the side list of major locations
// On phones the list sits along the bottom instead (MajorLocationList.vue).
const phone = window.matchMedia('(max-width: 720px)')
const narrow = ref(phone.matches)
const onNarrow = (e) => (narrow.value = e.matches)
phone.addEventListener('change', onNarrow)
const listInset = computed(() => (listClosed.value || narrow.value ? 0 : 256))

let grid = new PlanetGrid(worldMap.planet.frequency)
let cellMembers = new Map() // cell -> [major location ids]
let locMap = new Map()
let minorsByCell = new Map() // cell -> [minor locations], most important first
let saveChain = Promise.resolve()
let noticeTimer = null

// ---- Layers ---------------------------------------------------------------------------
// Plates, continents and ocean depths are drawn over the map
// (planetLayers.js); the map shows faintly under them. The continents
// layer also paints each hex's biome. The climate layers are worked out
// from all of it and only looked at.
const layer = ref('map')
const layers = useLayers()
const layerInfo = computed(() => layerOf(layer.value))
const onClimate = computed(() => !!layerInfo.value.climate)
const onLayer = computed(() => layer.value !== 'map' && !onClimate.value)
const layerFeatures = computed(() => layers.features.value.filter((f) => f.layer === layer.value))
const activeFeatureId = ref(null)
const featureModal = ref(null) // { feature } (null feature for a new one)
const pickedBiome = ref(null) // a biome key, on the continents layer
const biomeHeight = ref(null) // the ground height it is painted with (m), or its usual one
const shownTools = computed(() => (onClimate.value ? tools.filter((x) => x.key === 'select') : tools))

// Which arrows each climate layer starts with.
const ARROWS = { wind: { wind: true, currents: false }, currents: { wind: false, currents: true } }
const arrows = ref({ wind: false, currents: false })
const climateMonth = ref(null) // 0-based calendar month, or null for the year

watch(layer, async () => {
  activeFeatureId.value = null
  pickedBiome.value = null
  if (onClimate.value) {
    tool.value = 'select'
    arrows.value = { ...(ARROWS[layer.value] ?? { wind: false, currents: false }) }
  }
  try {
    if (layer.value !== 'map') await layers.ensure()
  } catch (err) {
    flash(err.message || t("Couldn't load the layers."))
  }
  if (onClimate.value) workOutClimate()
  version.value++
})

function onListPick(item) {
  if (!onLayer.value) return toggleActive(item)
  activeFeatureId.value = activeFeatureId.value === item.id ? null : item.id
  if (activeFeatureId.value != null) pickedBiome.value = null
}

watch(pickedBiome, (b) => {
  if (b) {
    activeFeatureId.value = null
    if (tool.value === 'select') tool.value = 'paint'
  }
  biomeHeight.value = null
})

// How many hexes each biome covers (on land).
const biomeCounts = computed(() => {
  version.value // eslint-disable-line no-unused-expressions
  const out = new Map()
  if (layer.value !== 'continents') return out
  for (const c of layers.biomeCells()) {
    if (!isLand(c)) continue
    const b = layers.biomeAt(c).biome
    out.set(b, (out.get(b) ?? 0) + 1)
  }
  return out
})

// ---- Land and climate ---------------------------------------------------------------------
// Land is wherever a continent is drawn, or the map has a kingdom, a region,
// an unknown land or a place.
function isLand(cell) {
  if (layers.featureAt('continents', cell)) return true
  if (minorsByCell.has(cell)) return true
  for (const id of cellMembers.get(cell) ?? []) {
    const l = locMap.get(id)
    if (l?.color && l.realm !== 'ocean') return true
  }
  return false
}

const climate = usePlanetClimate()
const sunlight = ref({ insolation: 1, evenHeat: 0, assumed: true })
let solarLoaded = false

const monthStep = (m) => {
  const months = calendar.months_per_year
  const steps = Math.min(24, months)
  return m == null ? null : Math.min(steps - 1, Math.floor(((m + 0.5) * steps) / months))
}
const climateStep = computed(() => monthStep(climateMonth.value))

async function workOutClimate() {
  if (!solarLoaded) {
    solarLoaded = true
    try {
      sunlight.value = heatOf(await solarApi.get())
    } catch {
      // No solar system to read: Earth's sunlight.
    }
  }
  const input = climateInput({
    frequency: grid.n,
    isLand,
    biomeAt: (c) => layers.biomeAt(c),
    tilt: worldMap.planet.tilt,
    dayHours: calendar.hours_per_day,
    months: Math.min(24, calendar.months_per_year),
    daysPerMonth: calendar.days_per_month,
    planetKm: worldMap.planet.planet_km,
    insolation: sunlight.value.insolation,
    evenHeat: sunlight.value.evenHeat,
  })
  climate.run(input, input.height)
}

// A finished climate recolours the map.
watch(climate.view, () => version.value++)
watch(climateMonth, () => version.value++)

const flows = computed(() => {
  const v = climate.view.value
  if (!onClimate.value || !v) return []
  const step = climateStep.value
  return [...(arrows.value.wind ? v.windArrows(step) : []), ...(arrows.value.currents ? v.currentArrows(step) : [])]
})

function onListEdit(item) {
  if (onLayer.value) featureModal.value = { feature: item }
  else editMajor.value = item
}

function onListNew() {
  if (onLayer.value) featureModal.value = { feature: null }
  else newMajorOpen.value = true
}

function onFeatureSaved(f) {
  const wasNew = !featureModal.value?.feature
  featureModal.value = null
  if (wasNew) {
    activeFeatureId.value = f.id
    tool.value = 'paint'
  }
  version.value++
}

function onFeatureDeleted() {
  featureModal.value = null
  activeFeatureId.value = null
  version.value++
}

const frequency = computed(() => worldMap.planet.frequency)
const majors = computed(() => locations.value.filter((l) => l.kind === 'major'))
const unplaced = computed(() => locations.value.filter((l) => l.kind === 'minor' && l.cell == null))
const takenColors = computed(() => majors.value.filter((m) => m.color && m.realm !== 'unknown').map((m) => m.color))
const lines = computed(() => ({ equator: worldMap.planet.show_equator, zones: worldMap.planet.show_zones, tilt: worldMap.planet.tilt }))

const rankOf = (l) => (l.is_capital ? 2 : l.is_city ? 1 : 0)
const markers = computed(() => {
  version.value // eslint-disable-line no-unused-expressions
  return [...minorsByCell.entries()].map(([cell, list]) => ({
    cell,
    rank: rankOf(list[0]),
    name: list.length > 1 ? `${list[0].name} +${list.length - 1}` : list[0].name,
  }))
})

// Every cell with something on it (for the flat view).
const coloredCells = computed(() => {
  version.value // eslint-disable-line no-unused-expressions
  if (onClimate.value && climate.view.value) return [...Array(grid.count).keys()]
  const drawn = onLayer.value ? layerFeatures.value.flatMap((f) => layers.cellsOfFeature(f)) : []
  const painted = layer.value === 'continents' ? layers.biomeCells().filter(isLand) : []
  return [...new Set([...cellMembers.keys(), ...minorsByCell.keys(), ...drawn, ...painted])]
})

// The middle of everything painted, for the globe to face when it opens.
const homeCell = computed(() => {
  const cells = coloredCells.value
  if (!cells.length) return null
  const step = Math.max(1, Math.floor(cells.length / 2000))
  const sum = [0, 0, 0]
  for (let k = 0; k < cells.length; k += step) {
    const v = grid.center(cells[k])
    sum[0] += v[0]
    sum[1] += v[1]
    sum[2] += v[2]
  }
  const len = Math.hypot(...sum)
  return len > 1e-9 ? grid.cellAt(sum.map((x) => x / len)) : cells[0]
})

const outlined = computed(() => {
  version.value // eslint-disable-line no-unused-expressions
  if (onLayer.value) return []
  const id = focusId.value ?? ((tool.value === 'paint' || tool.value === 'erase') && activeId.value != null && !locMap.get(activeId.value)?.color ? activeId.value : null)
  return id == null ? [] : cellsOf(id)
})

function cellsOf(id) {
  const out = []
  for (const [c, ids] of cellMembers) if (ids.includes(id)) out.push(c)
  const l = locMap.get(id)
  if (l?.kind === 'minor' && l.cell != null) out.push(l.cell)
  return out
}

function flash(message) {
  notice.value = message
  clearTimeout(noticeTimer)
  noticeTimer = setTimeout(() => notice.value === message && (notice.value = ''), 4500)
}

const activeName = computed(() => {
  if (pickedBiome.value) return t(biomeOf(pickedBiome.value)?.label ?? '')
  return onLayer.value
    ? layerFeatures.value.find((f) => f.id === activeFeatureId.value)?.name ?? ''
    : majors.value.find((m) => m.id === activeId.value)?.name ?? ''
})
const hint = computed(() => {
  if (view.value === 'flat') return t('A projection of the planet, to look at. Paint on the globe.')
  if (onClimate.value) return t('Scroll to zoom · Drag to turn the globe · Hover a hex to read its climate')
  if (tool.value === 'paint') {
    return activeName.value
      ? t('Scroll to zoom · Hold Space to turn the globe · Painting {name}', { name: activeName.value })
      : t('Scroll to zoom · Hold Space to turn the globe · Pick a major location below to paint with')
  }
  if (tool.value === 'erase') {
    if (activeName.value) return t('Scroll to zoom · Hold Space to turn the globe · Erasing {name}', { name: activeName.value })
    return onLayer.value
      ? t('Scroll to zoom · Hold Space to turn the globe · Erasing this layer on a hex')
      : t('Scroll to zoom · Hold Space to turn the globe · Erasing everything on a hex')
  }
  if (tool.value === 'fill') {
    return activeName.value
      ? t('Scroll to zoom · Drag to turn the globe · Click to fill an area with {name}', { name: activeName.value })
      : t('Scroll to zoom · Hold Space to turn the globe · Pick a major location below to paint with')
  }
  return t('Scroll to zoom · Drag to turn the globe · Click a hex to inspect it')
})

// ---- Colours -----------------------------------------------------------------------
function coloredOf(ids) {
  for (const id of ids) {
    const l = locMap.get(id)
    if (l && l.color) return l
  }
  return null
}

// What colour a cell is: its kingdom's or region's, land when something
// without a colour is there, or '' for the sea. An ocean leaves its water
// as it is, or tints it (oceanView.js); its territory is outlined (borders).
function colorOf(cell) {
  const base = cellColor(cellMembers.get(cell), locMap, minorsByCell.has(cell), oceanView.shown && oceanView.tint)
  if (onClimate.value) return climateColorOf(cell, base)
  if (layer.value === 'continents') {
    const b = layers.biomeAt(cell)
    if (b && isLand(cell)) return biomeOf(b.biome)?.color ?? base
  }
  return onLayer.value ? layerCellColor(base, layers.featureAt(layer.value, cell)) : base
}

const SEA_DARK = '#0a1426'

// Land under the arrows: its biome's colour (or the map's), dimmed.
function landUnder(cell, base) {
  const b = layers.biomeAt(cell)
  const color = (b && biomeOf(b.biome)?.color) || base || PLAIN_LAND.color
  return mixColor(color, SEA_DARK, 0.45)
}

// A cell on a climate layer: a heat map, rain, climate types; winds and
// currents over the land's biomes, dimmed.
function climateColorOf(cell, base) {
  const v = climate.view.value
  if (!v) return layerCellColor(base, null)
  const step = climateStep.value
  const land = isLand(cell)
  switch (layer.value) {
    case 'temperature': {
      const c = temperatureColor(v.temperature(step)[cell])
      return land ? c : mixColor(c, SEA_DARK, 0.25)
    }
    case 'rain': {
      const c = rainColor(v.rain(step)[cell], step == null)
      return land ? c : mixColor(c, SEA_DARK, 0.65)
    }
    case 'zones':
      return land ? koppenColor(v.koppenOf(cell, true, grid)) : mixColor('#1b2d4a', SEA_DARK, 0.4)
    case 'currents':
      return land ? landUnder(cell, base) : layerCellColor('', null)
    default:
      return land ? landUnder(cell, base) : layerCellColor('', null)
  }
}

// Each kingdom's, region's and ocean's name, at the middle of its land;
// oceans' only while they are shown.
const areas = computed(() => {
  version.value // eslint-disable-line no-unused-expressions
  if (onLayer.value) {
    return layerFeatures.value
      .map((f) => {
        const cells = layers.cellsOfFeature(f)
        return { key: 'f' + f.id, name: f.name, color: f.color, cell: middleCell(grid, cells), cells, size: areaFontPx(cells.length) }
      })
      .filter((a) => a.cell != null)
  }
  return majors.value
    .filter((m) => m.color && (m.realm !== 'ocean' || oceanView.shown))
    .map((m) => {
      const cells = cellsOf(m.id)
      return { key: m.id, name: mapName(m), color: m.color, cell: middleCell(grid, cells), cells, size: areaFontPx(cells.length), ocean: m.realm === 'ocean' }
    })
    .filter((a) => a.cell != null)
})

// Oceans are drawn as the outline of their water, unless that is turned off.
const borders = computed(() => {
  version.value // eslint-disable-line no-unused-expressions
  if (layer.value === 'continents') return layerFeatures.value.map((f) => ({ color: f.color, cells: layers.cellsOfFeature(f) }))
  if (onLayer.value || onClimate.value || !oceanView.shown || !oceanView.outline) return []
  return majors.value.filter((m) => m.color && m.realm === 'ocean').map((m) => ({ color: m.color, cells: cellsOf(m.id) }))
})

// ---- Loading -------------------------------------------------------------------------
async function load() {
  try {
    const data = await planetApi.get()
    applyMapSettings(data.settings)
    grid = new PlanetGrid(data.settings.planet.frequency)
    locations.value = data.locations
    locMap = new Map(data.locations.map((l) => [l.id, l]))
    cellMembers = new Map()
    for (const [id, c] of data.cells) {
      if (!cellMembers.has(c)) cellMembers.set(c, [])
      cellMembers.get(c).push(id)
    }
    minorsByCell = new Map()
    for (const l of data.locations) {
      if (l.kind !== 'minor' || l.cell == null) continue
      if (!minorsByCell.has(l.cell)) minorsByCell.set(l.cell, [])
      minorsByCell.get(l.cell).push(l)
    }
    for (const list of minorsByCell.values()) list.sort((a, b) => rankOf(b) - rankOf(a))
    if (activeId.value != null && !locMap.has(activeId.value)) activeId.value = null
    version.value++
    ready.value = true
    refreshPanel()
  } catch (err) {
    flash(err.message || t("Couldn't load the planet. Try refreshing."))
  }
}

// ---- Describing a cell -----------------------------------------------------------------
function lonText(lon) {
  const v = Math.round(Math.abs((lon * 180) / Math.PI))
  return v === 0 || v === 180 ? `${v}°` : `${v}°${lon > 0 ? 'E' : 'W'}`
}

function cellLabel(cell) {
  const [lat, lon] = latLon(grid.center(cell))
  return `${formatLatitude((lat * 180) / Math.PI)}, ${lonText(lon)}`
}

const hoverLatitude = computed(() => {
  if (hoverCell.value == null) return ''
  const lat = (latLon(grid.center(hoverCell.value))[0] * 180) / Math.PI
  return `${cellLabel(hoverCell.value)} · ${t(zoneOf(worldMap.planet, lat))}`
})

function belongsToName(m) {
  if (m.belongs_to_id) {
    const parent = locMap.get(m.belongs_to_id)
    if (parent) return parent.name
  }
  return coloredOf(cellMembers.get(m.cell) ?? [])?.name ?? ''
}

function describeCell(cell) {
  const entries = []
  if (onClimate.value) entries.push(...describeClimate(cell))
  const f = onLayer.value ? layers.featureAt(layer.value, cell) : null
  if (f) entries.push({ type: layerInfo.value.one, name: f.name, color: f.color })
  if (layer.value === 'continents' && isLand(cell)) {
    const b = layers.biomeAt(cell)
    const info = b && biomeOf(b.biome)
    if (info) {
      const h = b.height != null ? ` · ${Math.round(b.height).toLocaleString()} m` : ''
      entries.push({ type: 'Biome', name: t(info.label) + h, color: info.color, bare: true })
    }
  }
  for (const m of minorsByCell.get(cell) ?? []) {
    entries.push({
      type: m.is_capital ? 'Capital' : m.is_city ? 'City' : 'Location',
      name: m.name,
      color: '',
      belongsTo: belongsToName(m),
      founding_date: m.founding_date,
      description: m.description,
    })
  }
  const here = (cellMembers.get(cell) ?? []).map((id) => locMap.get(id)).filter(Boolean)
  for (const m of [...here.filter((l) => l.color), ...here.filter((l) => !l.color)]) {
    entries.push({
      type: m.color ? realmLabel(m.realm) : 'Major location',
      name: m.name,
      color: m.color || '',
      founding_date: m.founding_date,
      description: m.description,
    })
  }
  if (!entries.length) entries.push({ type: 'Ocean', name: t('Unnamed ocean'), color: '', bare: true })
  return entries
}

// What a hex's climate is: temperature, rain, wind, current, climate type.
function describeClimate(cell) {
  const v = climate.view.value
  if (!v) return []
  const unit = calendar.seasons?.unit || 'C'
  const step = climateStep.value
  const land = isLand(cell)
  const p = grid.center(cell)
  const rows = []
  const { low, high } = v.range()
  rows.push({
    label: step == null ? 'Mean temperature' : 'Temperature',
    value: formatTemperature(v.temperature(step)[cell], unit),
  })
  rows.push({ label: 'Coldest to warmest month', value: `${formatTemperature(low[cell], unit)} … ${formatTemperature(high[cell], unit)}` })
  rows.push({ label: step == null ? 'Rain in a year' : 'Rain in the month', value: formatRain(v.rain(step)[cell]) })
  rows.push({ label: 'Humidity', value: `${Math.round(v.humidity(step)[cell] * 100)}%` })
  const w = v.vectorAt(v.wind(step), p)
  const ws = Math.hypot(w.east, w.north)
  if (ws >= 0.5) rows.push({ label: 'Wind from', value: `${compassPoint(-w.east, -w.north)} · ${Math.round(ws)} m/s` })
  else rows.push({ label: 'Wind', value: t('Calm') })
  if (!land) {
    const c = v.vectorAt(v.current, p)
    const cs = Math.hypot(c.east, c.north)
    if (cs >= 0.03) rows.push({ label: 'Current towards', value: `${compassPoint(c.east, c.north)} · ${cs.toFixed(1)} m/s` })
  }
  const code = land ? v.koppenOf(cell, true, grid) : ''
  return [
    {
      type: 'Climate',
      name: code ? `${t(koppenName(code))} (${code})` : t('Open sea'),
      color: code ? koppenColor(code) : '',
      bare: true,
      rows,
    },
  ]
}

function onHover(cell) {
  hoverCell.value = cell
  hoverEntries.value = cell == null ? [] : describeCell(cell)
}

function refreshPanel() {
  if (selectedCell.value == null) {
    panel.value = null
    return
  }
  const c = selectedCell.value
  panel.value = {
    majors: (cellMembers.get(c) ?? []).map((id) => locMap.get(id)).filter(Boolean),
    minors: minorsByCell.get(c) ?? [],
    kingdom: coloredOf(cellMembers.get(c) ?? []),
  }
}

function onSelect(cell) {
  if (tool.value === 'fill') return bucketAt(cell)
  selectedCell.value = cell
  focusId.value = null
  refreshPanel()
}

function closePanel() {
  selectedCell.value = null
  panel.value = null
}

// ---- Painting ------------------------------------------------------------------------
// As on the 2D map: the cells change on screen at once and the stroke is saved
// when it ends.
let stroke = null

function idsAt(c) {
  return [...(cellMembers.get(c) ?? [])]
}

function setIds(c, ids) {
  if (ids.length) cellMembers.set(c, ids)
  else cellMembers.delete(c)
}

function paintLocal(c, locId) {
  const loc = locMap.get(locId)
  if (!loc || loc.kind !== 'major') return false
  let ids = idsAt(c)
  if (ids.includes(locId)) return false
  if (loc.color) ids = ids.filter((id) => !locMap.get(id)?.color)
  setIds(c, [...ids, locId])
  return true
}

function eraseLocal(c, locId) {
  const ids = idsAt(c)
  if (!ids.length) return false
  if (locId == null) setIds(c, [])
  else if (ids.includes(locId)) setIds(c, ids.filter((id) => id !== locId))
  else return false
  return true
}

function onPaintStart() {
  if (onClimate.value) {
    stroke = null
    return
  }
  if (layer.value === 'continents' && pickedBiome.value) {
    stroke = { biomes: true, type: tool.value, value: { biome: pickedBiome.value, height: biomeHeight.value }, before: new Map(), changed: new Set(), skipped: 0 }
    return
  }
  if (onLayer.value) {
    if (tool.value === 'paint' && activeFeatureId.value == null) {
      flash(t('Pick one in the list to paint with.'))
      stroke = null
      return
    }
    stroke = { layer: layer.value, type: tool.value, fid: activeFeatureId.value, before: new Map(), changed: new Set() }
    return
  }
  if (tool.value === 'paint' && !locMap.has(activeId.value)) {
    flash(t('Pick a major location in the bar below to paint with.'))
    stroke = null
    return
  }
  stroke = { type: tool.value, locId: activeId.value, before: new Map(), changed: new Set() }
}

// A stroke on a layer: each cell takes the feature, or loses it (any, or
// only the picked one).
function paintLayerCells(cells) {
  const touched = []
  for (const c of cells) {
    const now = layers.featureAt(stroke.layer, c)?.id ?? null
    if (!stroke.before.has(c)) stroke.before.set(c, now)
    let changed
    if (stroke.type === 'paint') changed = layers.setLocal(stroke.layer, c, stroke.fid)
    else changed = (stroke.fid == null || now === stroke.fid) && layers.setLocal(stroke.layer, c, null)
    if (changed) {
      stroke.changed.add(c)
      touched.push(c)
    }
  }
  if (touched.length) globeRef.value?.recolor(touched)
}

// A biome stroke: land cells take the biome, or lose it (only the picked one).
function paintBiomeCells(cells) {
  const touched = []
  for (const c of cells) {
    if (!isLand(c)) {
      stroke.skipped++
      continue
    }
    const now = layers.biomeAt(c)
    if (!stroke.before.has(c)) stroke.before.set(c, now)
    let changed
    if (stroke.type === 'paint') changed = layers.setBiomeLocal(c, stroke.value)
    else changed = now?.biome === stroke.value.biome && layers.setBiomeLocal(c, null)
    if (changed) {
      stroke.changed.add(c)
      touched.push(c)
    }
  }
  if (touched.length) globeRef.value?.recolor(touched)
}

function onPaint(cells) {
  if (!stroke) return
  if (stroke.biomes) return paintBiomeCells(cells)
  if (stroke.layer) return paintLayerCells(cells)
  const touched = []
  for (const c of cells) {
    if (!stroke.before.has(c)) stroke.before.set(c, idsAt(c))
    const changed = stroke.type === 'paint' ? paintLocal(c, stroke.locId) : eraseLocal(c, stroke.locId)
    if (changed) {
      stroke.changed.add(c)
      touched.push(c)
    }
  }
  if (touched.length) globeRef.value?.recolor(touched)
}

function onPaintEnd() {
  const s = stroke
  stroke = null
  if (s?.biomes && s.skipped && !s.changed.size) flash(t('Biomes go on land: draw a continent there first.'))
  if (!s || !s.changed.size) return
  if (s.biomes) {
    const changes = [...s.changed].map((c) => ({ c, before: s.before.get(c), after: layers.biomeAt(c) }))
    pushUndo({ biomes: true, cells: changes })
    version.value++
    layers.saveBiomes(changes, () => {
      flash(t("Couldn't save that stroke, so the map was reloaded."))
      version.value++
    })
    return
  }
  if (s.layer) {
    const changes = [...s.changed].map((c) => ({ c, before: s.before.get(c), after: layers.featureAt(s.layer, c)?.id ?? null }))
    pushUndo({ layer: s.layer, cells: changes })
    version.value++
    layers.save(s.layer, changes, () => {
      flash(t("Couldn't save that stroke, so the map was reloaded."))
      version.value++
    })
    return
  }
  const cells = [...s.changed]
  pushUndo(cells.map((c) => ({ c, before: s.before.get(c), after: idsAt(c) })))
  version.value++
  saveChain = saveChain.then(async () => {
    try {
      if (s.type === 'paint') await planetApi.paint(s.locId, cells)
      else await planetApi.erase(s.locId, cells)
    } catch (err) {
      flash(t("Couldn't save that stroke, so the map was reloaded."))
      await load()
    }
    refreshPanel()
  })
}

// ---- Bucket --------------------------------------------------------------------------------
// A click fills the connected empty hexes around the clicked one
// (bucketFill.js); hexes with anything on them are left alone. The planet is
// finite, so any area ends somewhere.
const bucketAsk = ref(null) // { cells, locId, name }: a big fill waiting for a yes

function bucketAt(cell) {
  if (cell == null) return
  if (layer.value === 'continents' && pickedBiome.value) {
    const name = t(biomeOf(pickedBiome.value)?.label ?? '')
    const empty = (c) => isLand(c) && !layers.biomeAt(c)
    if (!empty(cell)) return flash(isLand(cell) ? t('The bucket only fills empty hexes.') : t('Biomes go on land: draw a continent there first.'))
    const { cells, tooBig } = floodFill(cell, { neighbors: (c) => grid.neighbors(c), empty })
    if (tooBig) return flash(t('That area is too big to fill at once.'))
    if (cells.length > BUCKET_WARN) bucketAsk.value = { cells, name, biome: { biome: pickedBiome.value, height: biomeHeight.value } }
    else fillBiome(cells, { biome: pickedBiome.value, height: biomeHeight.value })
    return
  }
  if (onLayer.value) {
    const f = layerFeatures.value.find((x) => x.id === activeFeatureId.value)
    if (!f) return flash(t('Pick one in the list to paint with.'))
    const empty = (c) => !layers.featureAt(layer.value, c)
    if (!empty(cell)) return flash(t('The bucket only fills empty hexes.'))
    const { cells, tooBig } = floodFill(cell, { neighbors: (c) => grid.neighbors(c), empty })
    if (tooBig) return flash(t('That area is too big to fill at once.'))
    if (cells.length > BUCKET_WARN) bucketAsk.value = { cells, locId: f.id, name: f.name, layer: layer.value }
    else fill(cells, f.id, layer.value)
    return
  }
  const loc = locMap.get(activeId.value)
  if (!loc) {
    flash(t('Pick a major location in the bar below to paint with.'))
    return
  }
  if (idsAt(cell).length) return flash(t('The bucket only fills empty hexes.'))
  const { cells, tooBig } = floodFill(cell, { neighbors: (c) => grid.neighbors(c), empty: (c) => !idsAt(c).length })
  if (tooBig) return flash(t('That area is too big to fill at once.'))
  if (cells.length > BUCKET_WARN) bucketAsk.value = { cells, locId: loc.id, name: loc.name }
  else fill(cells, loc.id)
}

function confirmBucket() {
  const { cells, locId, layer: on, biome } = bucketAsk.value
  bucketAsk.value = null
  if (biome) fillBiome(cells, biome)
  else fill(cells, locId, on)
}

function fillBiome(cells, value) {
  stroke = { biomes: true, type: 'paint', value, before: new Map(), changed: new Set(), skipped: 0 }
  onPaint(cells)
  onPaintEnd()
}

// Painted like one stroke, so Undo takes it back whole.
function fill(cells, locId, on = null) {
  stroke = on
    ? { layer: on, type: 'paint', fid: locId, before: new Map(), changed: new Set() }
    : { type: 'paint', locId, before: new Map(), changed: new Set() }
  onPaint(cells)
  onPaintEnd()
}

// ---- Undo / redo -----------------------------------------------------------------------
const UNDO_LIMIT = 50
const undoStack = ref([])
const redoStack = ref([])

function pushUndo(cells) {
  undoStack.value = [...undoStack.value.slice(1 - UNDO_LIMIT), cells]
  redoStack.value = []
}

// A layer's stroke back or forth: each cell to the feature it held.
function restoreLayer(entry, which) {
  const known = new Set(layers.features.value.map((f) => f.id))
  const changes = entry.cells.map((h) => ({ c: h.c, after: known.has(h[which]) ? h[which] : null }))
  for (const { c, after } of changes) layers.setLocal(entry.layer, c, after)
  globeRef.value?.recolor(changes.map((h) => h.c))
  version.value++
  layers.save(entry.layer, changes, () => {
    flash(t("Couldn't save that change, so the map was reloaded."))
    version.value++
  })
}

// A biome stroke back or forth: each cell to the biome it had.
function restoreBiomes(entry, which) {
  const changes = entry.cells.map((h) => ({ c: h.c, after: h[which] ?? null }))
  for (const { c, after } of changes) layers.setBiomeLocal(c, after)
  globeRef.value?.recolor(changes.map((h) => h.c))
  version.value++
  layers.saveBiomes(changes, () => {
    flash(t("Couldn't save that change, so the map was reloaded."))
    version.value++
  })
}

function restore(cells, which) {
  if (cells.biomes) return restoreBiomes(cells, which)
  if (cells.layer) return restoreLayer(cells, which)
  const erase = new Map()
  const paint = new Map()
  const add = (m, id, c) => (m.has(id) ? m.get(id).push(c) : m.set(id, [c]))
  for (const h of cells) {
    const target = h[which].filter((id) => locMap.has(id))
    const now = idsAt(h.c)
    for (const id of now) if (!target.includes(id)) add(erase, id, h.c)
    for (const id of target) if (!now.includes(id)) add(paint, id, h.c)
    setIds(h.c, target)
  }
  globeRef.value?.recolor(cells.map((h) => h.c))
  version.value++
  saveChain = saveChain.then(async () => {
    try {
      for (const [id, list] of erase) await planetApi.erase(id, list)
      const order = [...paint].sort(([a], [b]) => Number(!!locMap.get(a)?.color) - Number(!!locMap.get(b)?.color))
      for (const [id, list] of order) await planetApi.paint(id, list)
    } catch (err) {
      flash(t("Couldn't save that change, so the map was reloaded."))
      await load()
    }
    refreshPanel()
  })
}

function undo() {
  const last = undoStack.value.at(-1)
  if (!last) return
  undoStack.value = undoStack.value.slice(0, -1)
  redoStack.value = [...redoStack.value, last]
  restore(last, 'before')
}

function redo() {
  const next = redoStack.value.at(-1)
  if (!next) return
  redoStack.value = redoStack.value.slice(0, -1)
  undoStack.value = [...undoStack.value, next]
  restore(next, 'after')
}

function isTyping(e) {
  const el = e.target
  return !!el && (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.tagName === 'SELECT' || el.isContentEditable)
}

function onKeyDown(e) {
  if (isTyping(e)) return
  if (e.key === 'Escape') closePanel()
  else if ((e.ctrlKey || e.metaKey) && !stroke) {
    const key = e.key.toLowerCase()
    if (key === 'z' && !e.shiftKey) {
      e.preventDefault()
      undo()
    } else if ((key === 'z' && e.shiftKey) || key === 'y') {
      e.preventDefault()
      redo()
    }
  }
}

// ---- Toolbar ----------------------------------------------------------------------------
function setView(v) {
  view.value = v
  remember(VIEW_KEY, v)
  onHover(null)
}

watch(flatProjection, (p) => remember(PROJ_KEY, p))
// Every ocean cell changes colour.
watch(() => [oceanView.shown, oceanView.tint], () => version.value++)

async function toggleLine(key) {
  const p = { ...worldMap.planet, [key]: !worldMap.planet[key] }
  worldMap.planet = p
  try {
    applyMapSettings(
      await planetApi.saveSettings({
        mode: worldMap.mode,
        planet_km: p.planet_km,
        hex_km: worldMap.hex_km,
        tilt: p.tilt,
        bestiary_projection: p.bestiary_projection,
        show_equator: p.show_equator,
        show_zones: p.show_zones,
      }),
    )
  } catch (err) {
    flash(err.message)
  }
}

function toggleActive(m) {
  activeId.value = activeId.value === m.id ? null : m.id
}

// ---- Locations --------------------------------------------------------------------------
function closeMajorModal() {
  newMajorOpen.value = false
  editMajor.value = null
}

async function onMajorSaved(loc) {
  const wasNew = !editMajor.value
  closeMajorModal()
  await load()
  if (wasNew) {
    activeId.value = loc.id
    tool.value = 'paint'
  }
}

async function onMajorDeleted() {
  closeMajorModal()
  await load()
}

// These throw on failure; the hex panel shows the message. The 2D hex is
// left out, so it is kept.
async function saveMinor(payload) {
  const body = {
    kind: 'minor',
    name: payload.name,
    founding_date: payload.founding_date,
    description: payload.description,
    belongs_to_id: payload.belongs_to_id,
    is_city: payload.is_city,
    is_capital: payload.is_capital,
  }
  if (payload.id) await mapApi.updateLocation(payload.id, body)
  else await mapApi.createLocation({ ...body, cell: selectedCell.value })
  await load()
}

async function deleteMinor(loc) {
  await mapApi.deleteLocation(loc.id)
  await load()
}

async function placeLocation(loc) {
  await mapApi.updateLocation(loc.id, {
    kind: 'minor',
    name: loc.name,
    founding_date: loc.founding_date,
    description: loc.description,
    belongs_to_id: loc.belongs_to_id,
    is_city: loc.is_city,
    is_capital: loc.is_capital,
    cell: selectedCell.value,
  })
  await load()
}

// A link to a place (?focus=id) turns the globe to it and outlines it.
async function applyFocus() {
  const id = Number(route.query.focus)
  if (!Number.isInteger(id) || !locMap.has(id)) return
  focusId.value = id
  const cells = cellsOf(id)
  if (cells.length) {
    view.value = 'globe'
    setTimeout(() => globeRef.value?.focusOn(cells[0]), 100)
  }
}

onMounted(async () => {
  window.addEventListener('keydown', onKeyDown)
  await load()
  applyFocus()
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeyDown)
  phone.removeEventListener('change', onNarrow)
})

watch(() => route.query.focus, applyFocus)
</script>

<style scoped src="../mapChrome.css"></style>

<style scoped>
.proj-select {
  max-width: 7.5rem;
  min-height: 1.9rem;
  padding: 0.15rem 0.3rem;
  font-size: 0.72rem;
}
</style>
