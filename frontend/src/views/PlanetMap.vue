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
          <button v-for="t in tools" :key="t.key" type="button" class="tool-btn" :class="{ active: tool === t.key }" :title="$t(t.title)" @click="tool = t.key">
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
        :majors="majors"
        :active-id="activeId"
        @pick="toggleActive"
        @edit="editMajor = $event"
        @new="newMajorOpen = true"
        @closed="listClosed = $event"
      />
    </div>

    <MajorLocationModal
      v-if="newMajorOpen || editMajor"
      :location="editMajor"
      :taken-colors="takenColors"
      @close="closeMajorModal"
      @saved="onMajorSaved"
      @deleted="onMajorDeleted"
    />
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { useRoute } from 'vue-router'
import { t, tr } from '../i18n'
import { planetApi, mapApi } from '../api'
import { worldMap, PROJECTIONS, realmLabel, applyMapSettings, mapName, skyUrl } from '../worldMap'
import { PlanetGrid, latLon, middleCell } from '../planetGrid'
import { areaFontPx } from '../planetLabels'
import { formatLatitude, zoneOf } from '../planet'
import PlanetGlobe from '../components/PlanetGlobe.vue'
import PlanetFlatMap from '../components/PlanetFlatMap.vue'
import MapHoverPane from '../components/MapHoverPane.vue'
import MajorLocationList from '../components/MajorLocationList.vue'
import HexPanel from '../components/HexPanel.vue'
import MajorLocationModal from '../components/MajorLocationModal.vue'

// The map when the story is a planet (backend/planetmap.go): painted on the
// globe; the flat views are projections to look at. Mirrors MapView.vue
// (the 2D map) in what it shows and how painting, undo and the panels work.

const route = useRoute()

const tools = [
  { key: 'select', label: tr('Select'), title: tr('Click a hex to inspect it and add locations. Drag to turn the globe.') },
  { key: 'paint', label: tr('Paint'), title: tr('Paint hexes with the selected major location') },
  { key: 'erase', label: tr('Erase'), title: tr('Erase the selected major location from hexes (everything, if none is selected)') },
]
const BRUSHES = [
  { radius: 0, hexes: 1, dot: 5 },
  { radius: 1, hexes: 7, dot: 9 },
  { radius: 2, hexes: 19, dot: 13 },
]
const LAND = '#7c7f74' // land with no kingdom, region or ocean on it

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
  return [...new Set([...cellMembers.keys(), ...minorsByCell.keys()])]
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

const activeName = computed(() => majors.value.find((m) => m.id === activeId.value)?.name ?? '')
const hint = computed(() => {
  if (view.value === 'flat') return t('A projection of the planet, to look at. Paint on the globe.')
  if (tool.value === 'paint') {
    return activeName.value
      ? t('Scroll to zoom · Hold Space to turn the globe · Painting {name}', { name: activeName.value })
      : t('Scroll to zoom · Hold Space to turn the globe · Pick a major location below to paint with')
  }
  if (tool.value === 'erase') {
    return activeName.value
      ? t('Scroll to zoom · Hold Space to turn the globe · Erasing {name}', { name: activeName.value })
      : t('Scroll to zoom · Hold Space to turn the globe · Erasing everything on a hex')
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
// as it is; its territory is outlined instead (borders).
function colorOf(cell) {
  const ids = cellMembers.get(cell)
  if (ids && ids.length) {
    const owner = coloredOf(ids)
    if (owner) return owner.realm === 'ocean' ? '' : owner.color
    return LAND
  }
  return minorsByCell.has(cell) ? LAND : ''
}

// Each kingdom's, region's and ocean's name, at the middle of its land.
const areas = computed(() => {
  version.value // eslint-disable-line no-unused-expressions
  return majors.value
    .filter((m) => m.color)
    .map((m) => {
      const cells = cellsOf(m.id)
      return { key: m.id, name: mapName(m), color: m.color, cell: middleCell(grid, cells), cells, size: areaFontPx(cells.length) }
    })
    .filter((a) => a.cell != null)
})

// Oceans are drawn as the outline of their water.
const borders = computed(() => {
  version.value // eslint-disable-line no-unused-expressions
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
  if (tool.value === 'paint' && !locMap.has(activeId.value)) {
    flash(t('Pick a major location in the bar below to paint with.'))
    stroke = null
    return
  }
  stroke = { type: tool.value, locId: activeId.value, before: new Map(), changed: new Set() }
}

function onPaint(cells) {
  if (!stroke) return
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
  if (!s || !s.changed.size) return
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

// ---- Undo / redo -----------------------------------------------------------------------
const UNDO_LIMIT = 50
const undoStack = ref([])
const redoStack = ref([])

function pushUndo(cells) {
  undoStack.value = [...undoStack.value.slice(1 - UNDO_LIMIT), cells]
  redoStack.value = []
}

function restore(cells, which) {
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
