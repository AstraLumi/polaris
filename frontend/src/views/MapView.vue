<template>
  <div class="map-page">
    <div ref="wrapEl" class="map-wrap" :class="wrapClass">
      <canvas
        ref="canvasEl"
        class="map-canvas"
        @pointerdown="onPointerDown"
        @pointermove="onPointerMove"
        @pointerup="onPointerUp"
        @pointercancel="onPointerUp"
        @pointerleave="onPointerLeave"
        @contextmenu.prevent
      ></canvas>

      <div class="toolbar glass-panel panel-solid">
        <button
          v-for="t in tools"
          :key="t.key"
          type="button"
          class="tool-btn"
          :class="{ active: tool === t.key }"
          :title="$t(t.title)"
          @click="tool = t.key"
        >
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
        <div class="tool-sep"></div>
        <button type="button" class="tool-btn" :title="$t('Zoom in')" @click="zoomBy(1.25)">+</button>
        <button type="button" class="tool-btn" :title="$t('Zoom out')" @click="zoomBy(0.8)">−</button>
        <button type="button" class="tool-btn" :title="$t('Fit the map in view')" @click="fitAndDraw">{{ $t('Fit') }}</button>
        <span class="zoom-label">{{ zoomLabel }}</span>
        <div class="tool-sep"></div>
        <button type="button" class="tool-btn" :class="{ active: equatorOpen }" :title="$t('Show and place the equator')" @click="equatorOpen = !equatorOpen">
          {{ $t('Equator') }}
        </button>
      </div>

      <div v-if="equatorOpen" class="equator-panel glass-panel panel-solid">
        <div class="eq-head">
          <strong>{{ $t('Equator') }}</strong>
          <button type="button" class="eq-close" :aria-label="$t('Close')" @click="equatorOpen = false">×</button>
        </div>
        <label class="eq-check">
          <input v-model="equator.show" type="checkbox" @change="saveEquatorSoon" />
          {{ $t('Show the equator') }}
        </label>
        <button type="button" class="btn btn-ghost small" :class="{ 'is-placing': placingEquator }" @click="togglePlacing">
          {{ placingEquator ? $t('Click a hex on the map…') : $t('Place it on a hex') }}
        </button>
        <label class="eq-field">
          <span>{{ $t('Row') }}</span>
          <input v-model.number="equator.row" type="number" step="0.5" @change="saveEquatorSoon" />
        </label>
        <label class="eq-field">
          <span>{{ $t('Bend') }}</span>
          <input v-model.number="equator.bend" type="range" min="-20" max="20" step="0.5" @input="saveEquatorSoon" />
          <span class="eq-val">{{ equator.bend }}</span>
        </label>
        <p class="eq-note">
          {{ $t('Bend curves the line away from the hex you placed it on, for a map drawn from a globe. 0 keeps it straight. Hexes above the line are in the north.') }}
        </p>

        <div class="eq-sep"></div>
        <label class="eq-check">
          <input v-model="equator.show_zones" type="checkbox" @change="saveEquatorSoon" />
          {{ $t('Show the tropics and polar circles') }}
        </label>
        <label class="eq-field eq-wide">
          <span>{{ $t('Hex width (km)') }}</span>
          <input v-model.number="equator.hex_km" type="number" min="0.01" step="any" @change="saveEquatorSoon" />
        </label>
        <label class="eq-field eq-wide">
          <span>{{ $t('Planet circumference (km)') }}</span>
          <input v-model.number="equator.planet_km" type="number" min="1" step="any" @change="saveEquatorSoon" />
        </label>
        <label class="eq-field eq-wide">
          <span>{{ $t('Axial tilt (°)') }}</span>
          <input v-model.number="equator.tilt" type="number" min="0" max="90" step="any" @change="saveEquatorSoon" />
        </label>
        <p class="eq-note">
          {{ $t('From the equator to a pole is {n} rows of hexes. The tropics lie at the tilt and the polar circles at 90° minus it; places between them are temperate. Each place\'s wiki article shows its hemisphere, latitude and zone.', { n: poleRows }) }}
        </p>
      </div>

      <div class="top-notes">
        <p class="hint-line">{{ hint }}</p>
        <p v-if="hoverLatitude" class="lat-line">{{ hoverLatitude }}</p>
        <p v-if="notice" class="error-banner notice">{{ notice }}</p>
      </div>

      <MapHoverPane :entries="hoverEntries" />

      <HexPanel
        v-if="selectedHex && panel"
        :hex="selectedHex"
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
      >
        <template #footer>
          <PlanetSwitch />
        </template>
      </MajorLocationList>
    </div>

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
import { ref, reactive, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { t, tr } from '../i18n'
import { mapApi } from '../api'
import { themeRgba } from '../theme'
import { HEX_SIZE, SQRT3, CORNERS, keyOf, hexCenter, pixelToHex, hexLine, hexesWithin } from '../hexMath'
import { rowsToPole, latitudeRow, latitudeOf, zoneOf, zoneLines, formatLatitude } from '../planet'
import { realmLabel, mapName } from '../worldMap'
import { drawDot, drawCity, drawCapital, lighten } from '../mapMarkers'
import { TIERS, areaFontPx } from '../planetLabels'
import MapHoverPane from '../components/MapHoverPane.vue'
import MajorLocationList from '../components/MajorLocationList.vue'
import PlanetSwitch from '../components/PlanetSwitch.vue'
import HexPanel from '../components/HexPanel.vue'
import MajorLocationModal from '../components/MajorLocationModal.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import { floodFill, BUCKET_WARN } from '../bucketFill'

const ZOOM_MIN = 0.15
const ZOOM_MAX = 4

const tools = [
  { key: 'select', label: tr('Select'), title: tr('Click a hex to inspect it and add locations. Drag to move the view.') },
  { key: 'paint', label: tr('Paint'), title: tr('Paint hexes with the selected major location') },
  { key: 'erase', label: tr('Erase'), title: tr('Erase the selected major location from hexes (everything, if none is selected)') },
  { key: 'fill', label: tr('Bucket'), title: tr('Fill the empty hexes connected to the clicked one with the selected major location') },
  { key: 'pan', label: tr('Pan'), title: tr('Drag to move the view (or hold Space with any tool)') },
]

// ---- Reactive UI state -------------------------------------------------------
const wrapEl = ref(null)
const canvasEl = ref(null)
const tool = ref('select')
// Brush radius in hexes: 0 paints one hex, 1 a patch of 7, 2 a patch of 19.
const BRUSHES = [
  { radius: 0, hexes: 1, dot: 5 },
  { radius: 1, hexes: 7, dot: 9 },
  { radius: 2, hexes: 19, dot: 13 },
]
const brush = ref(0)
const locations = ref([])
const activeId = ref(null) // the major location being painted/erased
const focusId = ref(null) // the major location a link pointed at (outlined)
const hoverHex = ref(null)
const hoverEntries = ref([])
const selectedHex = ref(null)
const panel = ref(null)
const newMajorOpen = ref(false)
const editMajor = ref(null)
const spaceDown = ref(false)
const panning = ref(false)
const zoomLabel = ref('100%')
const notice = ref('')
// The equator (backend/seasons.go): a line along hex row `row` at column
// `center`, curving by `bend` rows per 100 columns squared away from it.
const equator = reactive({
  show: false,
  row: 0,
  center: 0,
  bend: 0,
  show_zones: false,
  hex_km: 50,
  planet_km: 40075,
  tilt: 23.44,
  placed: false,
})
const equatorOpen = ref(false)
const placingEquator = ref(false)

// ---- Map data (plain objects: thousands of hexes shouldn't be reactive) ------
let hexMembers = new Map() // "q,r" -> { q, r, x, y, ids: [major location ids] }
let painted = new Map() // "q,r" -> { q, r, x, y, color } — hexes owned by a kingdom
let minorsByKey = new Map() // "q,r" -> [minor locations on that hex]
let minorSpots = [] // one marker per hex with minor locations: { x, y, rank, count, name }
let locMap = new Map() // id -> location
let hexesOfCache = new Map() // location id -> [hex entries]

const view = { x: 0, y: 0, zoom: 1 }
const size = { w: 0, h: 0, dpr: 1 }
let hoverKey = null
let drag = null
let resizeObserver = null
let drawQueued = false
let saveChain = Promise.resolve()
let noticeTimer = null

const majors = computed(() => locations.value.filter((l) => l.kind === 'major'))
const unplaced = computed(() =>
  locations.value.filter((l) => l.kind === 'minor' && (l.q == null || l.r == null)),
)
const takenColors = computed(() => majors.value.filter((m) => m.color && m.realm !== 'unknown').map((m) => m.color))
const activeName = computed(() => majors.value.find((m) => m.id === activeId.value)?.name ?? '')

const listClosed = ref(false) // the side list of major locations
const wrapClass = computed(() => ({
  'has-list': !listClosed.value,
  'cursor-grab': (tool.value === 'pan' || spaceDown.value) && !panning.value,
  'cursor-grabbing': panning.value,
  'cursor-cross': (tool.value === 'paint' || tool.value === 'erase' || tool.value === 'fill') && !spaceDown.value,
}))

const hint = computed(() => {
  if (placingEquator.value) return t('Click the hex the equator runs through')
  if (tool.value === 'paint') {
    return activeName.value
      ? t('Scroll to zoom · Hold Space to drag the view · Painting {name}', { name: activeName.value })
      : t('Scroll to zoom · Hold Space to drag the view · Pick a major location below to paint with')
  }
  if (tool.value === 'erase') {
    return activeName.value
      ? t('Scroll to zoom · Hold Space to drag the view · Erasing {name}', { name: activeName.value })
      : t('Scroll to zoom · Hold Space to drag the view · Erasing everything on a hex')
  }
  if (tool.value === 'fill') {
    return activeName.value
      ? t('Scroll to zoom · Hold Space to drag the view · Click to fill an area with {name}', { name: activeName.value })
      : t('Scroll to zoom · Hold Space to drag the view · Pick a major location below to paint with')
  }
  if (tool.value === 'select') return t('Scroll to zoom · Hold Space to drag the view · Click a hex to inspect it')
  return t('Scroll to zoom · Hold Space to drag the view')
})

function flash(message) {
  notice.value = message
  clearTimeout(noticeTimer)
  noticeTimer = setTimeout(() => {
    if (notice.value === message) notice.value = ''
  }, 4500)
}

// ---- Indexes -----------------------------------------------------------------
function coloredOf(ids) {
  for (const id of ids) {
    const l = locMap.get(id)
    if (l && l.color) return l
  }
  return null
}

function kingdomAt(q, r) {
  const h = hexMembers.get(keyOf(q, r))
  return h ? coloredOf(h.ids) : null
}

function rebuildIndexes() {
  locMap = new Map(locations.value.map((l) => [l.id, l]))

  minorsByKey = new Map()
  for (const l of locations.value) {
    if (l.kind === 'minor' && l.q != null && l.r != null) {
      const k = keyOf(l.q, l.r)
      if (!minorsByKey.has(k)) minorsByKey.set(k, [])
      minorsByKey.get(k).push(l)
    }
  }
  // Within a hex the most important location comes first (it names the marker).
  minorSpots = []
  for (const list of minorsByKey.values()) {
    list.sort((a, b) => rankOf(b) - rankOf(a))
    const [x, y] = hexCenter(list[0].q, list[0].r)
    minorSpots.push({ x, y, rank: rankOf(list[0]), count: list.length, name: list[0].name })
  }
  minorSpots.sort((a, b) => a.rank - b.rank) // draw order: capitals last

  painted = new Map()
  for (const [k, h] of hexMembers) {
    const owner = coloredOf(h.ids)
    if (owner) painted.set(k, { q: h.q, r: h.r, x: h.x, y: h.y, color: owner.color })
  }
  hexesOfCache = new Map()
}

function hexesOf(locId) {
  let list = hexesOfCache.get(locId)
  if (!list) {
    list = [...hexMembers.values()].filter((h) => h.ids.includes(locId))
    hexesOfCache.set(locId, list)
  }
  return list
}

// ---- Describing a hex (hover pane + hex panel) ---------------------------------
function belongsToName(m) {
  if (m.belongs_to_id) {
    const parent = locMap.get(m.belongs_to_id)
    if (parent) return parent.name
  }
  const k = kingdomAt(m.q, m.r)
  return k ? k.name : ''
}

function describeHex(q, r) {
  const k = keyOf(q, r)
  const entries = []
  for (const m of minorsByKey.get(k) ?? []) {
    entries.push({
      type: m.is_capital ? 'Capital' : m.is_city ? 'City' : 'Location',
      name: m.name,
      color: '',
      belongsTo: belongsToName(m),
      founding_date: m.founding_date,
      description: m.description,
    })
  }
  const here = (hexMembers.get(k)?.ids ?? []).map((id) => locMap.get(id)).filter(Boolean)
  const kingdom = here.find((l) => l.color)
  if (kingdom) {
    entries.push({
      type: realmLabel(kingdom.realm),
      name: kingdom.name,
      color: kingdom.color,
      founding_date: kingdom.founding_date,
      description: kingdom.description,
    })
  }
  for (const m of here) {
    if (m.color) continue
    entries.push({
      type: 'Major location',
      name: m.name,
      color: '',
      founding_date: m.founding_date,
      description: m.description,
    })
  }
  // Where nothing is, is the open sea.
  if (!entries.length) entries.push({ type: 'Ocean', name: t('Unnamed ocean'), color: '', bare: true })
  return entries
}

function refreshPanels() {
  hoverEntries.value = hoverHex.value ? describeHex(hoverHex.value.q, hoverHex.value.r) : []
  const s = selectedHex.value
  if (!s) {
    panel.value = null
    return
  }
  const k = keyOf(s.q, s.r)
  panel.value = {
    majors: (hexMembers.get(k)?.ids ?? []).map((id) => locMap.get(id)).filter(Boolean),
    minors: minorsByKey.get(k) ?? [],
    kingdom: kingdomAt(s.q, s.r),
  }
}

// ---- Loading -------------------------------------------------------------------
async function load({ keepView = false } = {}) {
  try {
    const data = await mapApi.get()
    locations.value = data.locations
    if (data.equator) Object.assign(equator, data.equator)
    hexMembers = new Map()
    for (const [id, q, r] of data.hexes) {
      const k = keyOf(q, r)
      let entry = hexMembers.get(k)
      if (!entry) {
        const [x, y] = hexCenter(q, r)
        entry = { q, r, x, y, ids: [] }
        hexMembers.set(k, entry)
      }
      entry.ids.push(id)
    }
    rebuildIndexes()
    if (activeId.value != null && !locMap.has(activeId.value)) activeId.value = null
    if (!keepView) fitToContent()
    refreshPanels()
    scheduleDraw()
  } catch (err) {
    flash(t("Couldn't load the map. Try refreshing."))
  }
}

// ---- Canvas drawing --------------------------------------------------------------
function scheduleDraw() {
  if (drawQueued) return
  drawQueued = true
  requestAnimationFrame(() => {
    drawQueued = false
    draw()
  })
}

function addHexPath(ctx, cx, cy, s) {
  ctx.moveTo(cx + s * CORNERS[0][0], cy + s * CORNERS[0][1])
  for (let i = 1; i < 6; i++) ctx.lineTo(cx + s * CORNERS[i][0], cy + s * CORNERS[i][1])
  ctx.closePath()
}

// The grid tile, redrawn only when the zoom, pixel ratio or theme changes.
let gridTile = null
function gridPattern(ctx, z, dpr) {
  const color = themeRgba('--tint', 0.08)
  const key = `${z}|${dpr}|${color}`
  if (!gridTile || gridTile.key !== key) {
    const w = HEX_SIZE * SQRT3
    const h = HEX_SIZE * 3
    const pw = Math.max(2, Math.round(w * z * dpr))
    const ph = Math.max(2, Math.round(h * z * dpr))
    const tile = document.createElement('canvas')
    tile.width = pw
    tile.height = ph
    const t = tile.getContext('2d')
    t.scale(pw / w, ph / h) // one tile = exactly one lattice period
    t.beginPath()
    for (let r = -2; r <= 4; r++) {
      for (let q = -4; q <= 4; q++) {
        const [cx, cy] = hexCenter(q, r)
        if (cx < -HEX_SIZE * 2 || cx > w + HEX_SIZE * 2 || cy < -HEX_SIZE * 2 || cy > h + HEX_SIZE * 2) continue
        addHexPath(t, cx, cy, HEX_SIZE)
      }
    }
    t.strokeStyle = color
    t.lineWidth = 1 / z
    t.stroke()
    const pattern = ctx.createPattern(tile, 'repeat')
    if (pattern && pattern.setTransform) pattern.setTransform(new DOMMatrix().scale(w / pw, h / ph))
    gridTile = { key, pattern }
  }
  return gridTile.pattern
}

// ---- Minor location markers ------------------------------------------------------
// rank: 2 = capital, 1 = city, 0 = anything else.
const rankOf = (l) => (l.is_capital ? 2 : l.is_city ? 1 : 0)

// A label is drawn once a hex is at least this many screen pixels wide
// (HEX_SIZE * zoom); capitals are always labelled.
const LABEL_AT = [26, 12, 0]
const LABEL_MAX = [16, 20, 26]

// Marker radii: the world size at normal zoom, and the smallest it may get on
// screen, so cities and capitals stay readable when zoomed far out.
const MARKER = [
  { rad: HEX_SIZE * 0.26, minPx: 0 },
  { rad: HEX_SIZE * 0.5, minPx: 7 },
  { rad: HEX_SIZE * 0.82, minPx: 11 },
]

// Far out, the on-screen sizes shrink a little so a cluster of capitals and
// cities stays legible instead of piling up.
const compact = (z) => Math.min(1, Math.max(0.72, (HEX_SIZE * z) / 14))

function markerRadius(rank, z) {
  return Math.max(MARKER[rank].rad, (MARKER[rank].minPx * compact(z)) / z)
}

function drawMinorMarker(ctx, spot, z) {
  const { x: cx, y: cy, rank, count } = spot
  const rad = markerRadius(rank, z)
  if (count > 1) {
    ctx.beginPath()
    ctx.arc(cx, cy, rad * (rank === 2 ? 0.8 : rank === 1 ? 1.25 : 1.6), 0, Math.PI * 2)
    ctx.strokeStyle = themeRgba('--accent', 0.75)
    ctx.lineWidth = 1.5 / z
    ctx.stroke()
  }
  if (rank === 2) drawCapital(ctx, cx, cy, rad, z)
  else if (rank === 1) drawCity(ctx, cx, cy, rad, z)
  else drawDot(ctx, cx, cy, rad, z)
}

function roundRect(ctx, x, y, w, h, r) {
  ctx.beginPath()
  ctx.moveTo(x + r, y)
  ctx.arcTo(x + w, y, x + w, y + h, r)
  ctx.arcTo(x + w, y + h, x, y + h, r)
  ctx.arcTo(x, y + h, x, y, r)
  ctx.arcTo(x, y, x + w, y, r)
  ctx.closePath()
}

// Labels, most important first. A capital's label is always drawn; the rest
// are skipped when they would land on a label that is already there.
// The hex sharing the edge between corners i and i+1 (pointy-top, CORNERS).
const HEX_SIDES = [[1, 0], [0, 1], [-1, 1], [-1, 0], [0, -1], [1, -1]]

// Each kingdom's, region's and ocean's name at the middle of its hexes,
// while zoomed out (planetLabels.js tiers).
function drawAreaNames(ctx, z, inView) {
  ctx.textAlign = 'center'
  ctx.textBaseline = 'middle'
  ctx.lineJoin = 'round'
  for (const m of majors.value) {
    if (!m.color) continue
    const hexes = hexesOf(m.id)
    if (!hexes.length) continue
    let x = 0
    let y = 0
    for (const h of hexes) {
      x += h.x
      y += h.y
    }
    x /= hexes.length
    y /= hexes.length
    if (!inView(x, y)) continue
    const px = areaFontPx(hexes.length)
    ctx.font = `700 ${px / z}px Fraunces, 'Times New Roman', serif`
    if ('letterSpacing' in ctx) ctx.letterSpacing = `${1.5 / z}px`
    const text = mapName(m).toUpperCase()
    ctx.lineWidth = 4 / z
    ctx.strokeStyle = 'rgba(0, 0, 0, 0.75)'
    ctx.strokeText(text, x, y)
    ctx.fillStyle = lighten(m.color, 0.45)
    ctx.fillText(text, x, y)
  }
  if ('letterSpacing' in ctx) ctx.letterSpacing = '0px'
}

function drawMinorLabels(ctx, spots, z, screenSize) {
  const placed = []
  const free = (r) => !placed.some((p) => r.x < p.x + p.w && r.x + r.w > p.x && r.y < p.y + p.h && r.y + r.h > p.y)
  const ordered = [...spots].sort((a, b) => b.rank - a.rank)

  ctx.textAlign = 'center'
  ctx.textBaseline = 'middle'
  ctx.lineJoin = 'round'
  for (const spot of ordered) {
    const { rank, count } = spot
    if (rank < 2 && screenSize < LABEL_AT[rank]) continue
    const name = spot.name
    let text = name.length > LABEL_MAX[rank] ? `${name.slice(0, LABEL_MAX[rank] - 1)}…` : name
    if (count > 1) text += ` +${count - 1}`
    if (rank === 2) text = text.toUpperCase()

    const px = (rank === 2 ? 12.5 : rank === 1 ? 12 : 11) * compact(z)
    const family = rank === 2 ? "Fraunces, 'Times New Roman', serif" : 'Manrope, sans-serif'
    ctx.font = `${rank === 2 ? 700 : 600} ${px / z}px ${family}`
    if ('letterSpacing' in ctx) ctx.letterSpacing = rank === 2 ? `${1.2 / z}px` : '0px'
    const tw = ctx.measureText(text).width
    const padX = ((rank === 2 ? 9 : 3) * compact(z)) / z
    const padY = ((rank === 2 ? 4 : 2) * compact(z)) / z
    const h = px / z + padY * 2
    const w = tw + padX * 2
    const top = spot.y + markerRadius(rank, z) + (rank === 2 ? 5 : 3) / z
    const rect = { x: spot.x - w / 2, y: top, w, h }
    if (rank < 2 && !free(rect)) continue
    placed.push(rect)

    const ty = top + h / 2
    if (rank === 2) {
      roundRect(ctx, rect.x, rect.y, w, h, h / 2)
      ctx.fillStyle = themeRgba('--surface', 0.88)
      ctx.fill()
      ctx.strokeStyle = themeRgba('--accent', 0.9)
      ctx.lineWidth = 1.2 / z
      ctx.stroke()
      ctx.fillStyle = themeRgba('--accent')
      ctx.fillText(text, spot.x, ty)
    } else {
      ctx.lineWidth = 3.2 / z
      ctx.strokeStyle = themeRgba('--surface', 0.92)
      ctx.strokeText(text, spot.x, ty)
      ctx.fillStyle = themeRgba('--text-primary')
      ctx.fillText(text, spot.x, ty)
    }
  }
  if ('letterSpacing' in ctx) ctx.letterSpacing = '0px'
}

// ---- The equator -------------------------------------------------------------------
const COLUMN_W = HEX_SIZE * SQRT3

// The world y of the line of latitude `lat` (0: the equator) at world x.
const latitudeY = (x, lat) => HEX_SIZE * 1.5 * latitudeRow(equator, x / COLUMN_W, lat)

// Each line: its colour, width and dashes (in screen pixels).
const LINE_STYLES = {
  equator: { color: [232, 86, 70], width: 2, dash: [12, 7] },
  tropic: { color: [236, 168, 64], width: 1.5, dash: [3, 6] },
  polar: { color: [120, 180, 236], width: 1.5, dash: [3, 6] },
}

function drawLatitude(ctx, z, fromX, toX, lat, kind, label) {
  const style = LINE_STYLES[kind]
  const rgba = (a) => `rgba(${style.color.join(', ')}, ${a})`
  const steps = equator.bend ? 160 : 1
  ctx.beginPath()
  for (let i = 0; i <= steps; i++) {
    const x = fromX + ((toX - fromX) * i) / steps
    if (i === 0) ctx.moveTo(x, latitudeY(x, lat))
    else ctx.lineTo(x, latitudeY(x, lat))
  }
  ctx.save()
  ctx.setLineDash(style.dash.map((d) => d / z))
  ctx.lineWidth = (style.width + 1.5) / z
  ctx.strokeStyle = themeRgba('--surface', 0.7)
  ctx.stroke()
  ctx.lineWidth = style.width / z
  ctx.strokeStyle = rgba(0.95)
  ctx.stroke()
  ctx.restore()

  // Its name near the left edge of the view, clear of the toolbar.
  const lx = fromX + HEX_SIZE * 2 + 96 / z
  ctx.font = `700 ${11 / z}px Manrope, sans-serif`
  ctx.textAlign = 'left'
  ctx.textBaseline = 'bottom'
  ctx.lineJoin = 'round'
  ctx.lineWidth = 3 / z
  ctx.strokeStyle = themeRgba('--surface', 0.9)
  ctx.strokeText(label, lx, latitudeY(lx, lat) - 5 / z)
  ctx.fillStyle = rgba(1)
  ctx.fillText(label, lx, latitudeY(lx, lat) - 5 / z)
}

// The equator and, when asked, the tropics and polar circles.
function drawLatitudes(ctx, z, fromX, toX) {
  if (equator.show_zones && rowsToPole(equator) > 0) {
    for (const l of zoneLines(equator)) {
      const name = l.kind === 'tropic' ? t('Tropic') : t('Polar circle')
      drawLatitude(ctx, z, fromX, toX, l.lat, l.kind, `${name} · ${formatLatitude(l.lat)}`)
    }
  }
  if (equator.show) drawLatitude(ctx, z, fromX, toX, 0, 'equator', t('Equator'))
}

// "39°N, Temperate" for the hovered hex, once the equator is placed.
const hoverLatitude = computed(() => {
  if (!equator.placed || !hoverHex.value || !(rowsToPole(equator) > 0)) return ''
  const lat = latitudeOf(equator, hoverHex.value.q, hoverHex.value.r)
  return `${formatLatitude(lat)}, ${t(zoneOf(equator, lat))}`
})

const poleRows = computed(() => Math.round(rowsToPole(equator)) || 0)

let equatorTimer = null
function saveEquatorSoon() {
  scheduleDraw()
  clearTimeout(equatorTimer)
  equatorTimer = setTimeout(async () => {
    try {
      Object.assign(equator, await mapApi.saveEquator({ ...equator, row: Number(equator.row) || 0 }))
    } catch (err) {
      flash(err.message)
    }
  }, 400)
}

function togglePlacing() {
  placingEquator.value = !placingEquator.value
  if (placingEquator.value) tool.value = 'select'
}

// The equator runs through hex (q, r) and is shown.
function placeEquator({ q, r }) {
  placingEquator.value = false
  Object.assign(equator, { show: true, row: r, center: q + r / 2 })
  saveEquatorSoon()
}

function draw() {
  const canvas = canvasEl.value
  if (!canvas || !size.w) return
  const ctx = canvas.getContext('2d')
  const z = view.zoom

  ctx.setTransform(size.dpr, 0, 0, size.dpr, 0, 0)
  ctx.clearRect(0, 0, size.w, size.h)
  ctx.save()
  ctx.translate(view.x, view.y)
  ctx.scale(z, z)

  const minX = -view.x / z
  const maxX = (size.w - view.x) / z
  const minY = -view.y / z
  const maxY = (size.h - view.y) / z
  const pad = HEX_SIZE * 2
  const inView = (x, y) => x > minX - pad && x < maxX + pad && y > minY - pad && y < maxY + pad
  const screenSize = HEX_SIZE * z

  // 1. A faint grid, only once the hexes are big enough to be worth drawing.
  //    The lattice repeats every SQRT3*S by 3*S, so one small tile is drawn
  //    once per zoom level and then painted across the view as a pattern:
  //    a single fill instead of thousands of hex outlines every frame.
  if (screenSize >= 10) {
    const pattern = gridPattern(ctx, z, size.dpr)
    if (pattern) {
      ctx.fillStyle = pattern
      ctx.fillRect(minX - pad, minY - pad, maxX - minX + 2 * pad, maxY - minY + 2 * pad)
    }
  }

  // 2. Kingdom and region fills, batched per color. An ocean leaves its
  //    water unpainted: its territory gets an outline in its colour.
  const oceanColors = new Set(majors.value.filter((m) => m.color && m.realm === 'ocean').map((m) => m.color))
  const groups = new Map()
  const oceans = new Map()
  for (const h of painted.values()) {
    if (!inView(h.x, h.y)) continue
    const into = oceanColors.has(h.color) ? oceans : groups
    let g = into.get(h.color)
    if (!g) {
      g = []
      into.set(h.color, g)
    }
    g.push(h)
  }
  for (const [color, list] of groups) {
    ctx.beginPath()
    for (const h of list) addHexPath(ctx, h.x, h.y, HEX_SIZE)
    ctx.fillStyle = color
    ctx.fill()
    ctx.strokeStyle = 'rgba(0, 0, 0, 0.3)'
    ctx.lineWidth = 1 / z
    ctx.stroke()
  }
  for (const [color, list] of oceans) {
    ctx.beginPath()
    for (const h of list) {
      HEX_SIDES.forEach(([dq, dr], i) => {
        if (painted.get(keyOf(h.q + dq, h.r + dr))?.color === color) return
        const [ax, ay] = CORNERS[i]
        const [bx, by] = CORNERS[(i + 1) % 6]
        ctx.moveTo(h.x + ax * HEX_SIZE, h.y + ay * HEX_SIZE)
        ctx.lineTo(h.x + bx * HEX_SIZE, h.y + by * HEX_SIZE)
      })
    }
    ctx.strokeStyle = color
    ctx.lineWidth = 2 / z
    ctx.stroke()
  }

  // 3. Hexes that belong to a colorless major location get an inset outline,
  //    since they have no color of their own to show.
  if (screenSize >= 8) {
    ctx.beginPath()
    for (const h of hexMembers.values()) {
      if (!inView(h.x, h.y)) continue
      if (!h.ids.some((id) => !locMap.get(id)?.color)) continue
      addHexPath(ctx, h.x, h.y, HEX_SIZE * 0.72)
    }
    ctx.strokeStyle = 'rgba(255, 255, 255, 0.5)'
    ctx.lineWidth = 1.2 / z
    ctx.stroke()
  }

  // 4. Outline the extent of a colorless location: the one being painted,
  //    and any under the cursor.
  const outline = (hexes, style, width) => {
    ctx.beginPath()
    for (const h of hexes) if (inView(h.x, h.y)) addHexPath(ctx, h.x, h.y, HEX_SIZE)
    ctx.strokeStyle = style
    ctx.lineWidth = width / z
    ctx.stroke()
  }
  if ((tool.value === 'paint' || tool.value === 'erase') && activeId.value != null) {
    const active = locMap.get(activeId.value)
    if (active && !active.color) outline(hexesOf(activeId.value), 'rgba(255, 255, 255, 0.8)', 2)
  }
  if (focusId.value != null) outline(hexesOf(focusId.value), themeRgba('--accent', 0.95), 3)
  if (hoverHex.value) {
    const ids = hexMembers.get(keyOf(hoverHex.value.q, hoverHex.value.r))?.ids ?? []
    for (const id of ids) {
      const l = locMap.get(id)
      if (l && !l.color) outline(hexesOf(id), themeRgba('--accent', 0.95), 2)
    }
  }

  drawLatitudes(ctx, z, minX - pad, maxX + pad)

  // 5. Minor locations. Capitals and cities get their own markers and are
  //    labelled from further out than ordinary places (see LABEL_AT); markers
  //    are drawn lowest rank first so a capital is never buried, then the
  //    labels go on top of everything.
  const spots = minorSpots.filter((spot) => inView(spot.x, spot.y))
  for (const spot of spots) drawMinorMarker(ctx, spot, z)
  drawMinorLabels(ctx, spots, z, screenSize)
  if (screenSize < TIERS.areaUntil) drawAreaNames(ctx, z, inView)

  // 6. Selected and hovered hex.
  if (selectedHex.value) {
    const [cx, cy] = hexCenter(selectedHex.value.q, selectedHex.value.r)
    ctx.beginPath()
    addHexPath(ctx, cx, cy, HEX_SIZE)
    ctx.strokeStyle = themeRgba('--accent')
    ctx.lineWidth = 2.5 / z
    ctx.stroke()
  }
  if (hoverHex.value && !panning.value) {
    // With Paint or Erase, show every hex the brush will touch.
    const painting = tool.value === 'paint' || tool.value === 'erase'
    const under = painting && brush.value ? hexesWithin(hoverHex.value, brush.value) : [hoverHex.value]
    ctx.beginPath()
    for (const h of under) {
      const [cx, cy] = hexCenter(h.q, h.r)
      addHexPath(ctx, cx, cy, HEX_SIZE)
    }
    ctx.fillStyle = 'rgba(255, 255, 255, 0.07)'
    ctx.fill()
    ctx.strokeStyle = 'rgba(255, 255, 255, 0.65)'
    ctx.lineWidth = 1.5 / z
    ctx.stroke()
  }

  ctx.restore()
}

// ---- View: size, zoom, fit -----------------------------------------------------------
function updateSize() {
  const el = wrapEl.value
  const canvas = canvasEl.value
  if (!el || !canvas) return
  size.w = el.clientWidth
  size.h = el.clientHeight
  size.dpr = window.devicePixelRatio || 1
  canvas.width = Math.round(size.w * size.dpr)
  canvas.height = Math.round(size.h * size.dpr)
  scheduleDraw()
}

function setZoomLabel() {
  zoomLabel.value = `${Math.round(view.zoom * 100)}%`
}

function zoomAt(sx, sy, factor) {
  const old = view.zoom
  const next = Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, old * factor))
  if (next === old) return
  // Keep the world point under the cursor fixed while zooming.
  const wx = (sx - view.x) / old
  const wy = (sy - view.y) / old
  view.zoom = next
  view.x = sx - wx * next
  view.y = sy - wy * next
  setZoomLabel()
  scheduleDraw()
}

function zoomBy(factor) {
  zoomAt(size.w / 2, size.h / 2, factor)
}

function fitToContent() {
  const pts = []
  for (const h of hexMembers.values()) pts.push([h.x, h.y])
  for (const list of minorsByKey.values()) pts.push(hexCenter(list[0].q, list[0].r))

  let zoom = 1
  let cx = 0
  let cy = 0
  if (pts.length) {
    let minX = Infinity
    let maxX = -Infinity
    let minY = Infinity
    let maxY = -Infinity
    for (const [x, y] of pts) {
      minX = Math.min(minX, x)
      maxX = Math.max(maxX, x)
      minY = Math.min(minY, y)
      maxY = Math.max(maxY, y)
    }
    const fitX = (size.w - 200) / (maxX - minX + HEX_SIZE * 2)
    const fitY = (size.h - 220) / (maxY - minY + HEX_SIZE * 2)
    zoom = Math.min(1.4, Math.max(0.25, Math.min(fitX, fitY)))
    cx = (minX + maxX) / 2
    cy = (minY + maxY) / 2
  }
  view.zoom = zoom
  view.x = size.w / 2 - cx * zoom
  view.y = size.h / 2 - cy * zoom
  setZoomLabel()
}

// Centre the view on a box of world points and zoom so it fills most of the
// screen: a big kingdom ends up far out, a single hex close in.
const FOCUS_ZOOM_MAX = 2.5
function fitBox(minX, maxX, minY, maxY) {
  const fitX = (size.w * 0.7) / (maxX - minX + HEX_SIZE * 4)
  const fitY = (size.h * 0.7) / (maxY - minY + HEX_SIZE * 4)
  const zoom = Math.min(FOCUS_ZOOM_MAX, Math.max(ZOOM_MIN, Math.min(fitX, fitY)))
  view.zoom = zoom
  view.x = size.w / 2 - ((minX + maxX) / 2) * zoom
  view.y = size.h / 2 - ((minY + maxY) / 2) * zoom
  setZoomLabel()
}

// Show one location: a minor one's hex is selected (its panel opens), a
// major one's hexes are outlined. Returns false if it isn't on the map.
function focusLocation(loc) {
  if (loc.kind === 'minor') {
    if (loc.q == null || loc.r == null) return false
    const [x, y] = hexCenter(loc.q, loc.r)
    fitBox(x, x, y, y)
    focusId.value = null
    selectedHex.value = { q: loc.q, r: loc.r }
  } else {
    const hexes = hexesOf(loc.id)
    if (!hexes.length) return false
    let minX = Infinity
    let maxX = -Infinity
    let minY = Infinity
    let maxY = -Infinity
    for (const h of hexes) {
      minX = Math.min(minX, h.x)
      maxX = Math.max(maxX, h.x)
      minY = Math.min(minY, h.y)
      maxY = Math.max(maxY, h.y)
    }
    fitBox(minX, maxX, minY, maxY)
    focusId.value = loc.id
    selectedHex.value = null
  }
  refreshPanels()
  scheduleDraw()
  return true
}

function fitAndDraw() {
  fitToContent()
  scheduleDraw()
}

// ---- Pointer interaction ------------------------------------------------------------------
function eventPoint(e) {
  const rect = canvasEl.value.getBoundingClientRect()
  return { sx: e.clientX - rect.left, sy: e.clientY - rect.top }
}

function hexAt(sx, sy) {
  return pixelToHex((sx - view.x) / view.zoom, (sy - view.y) / view.zoom)
}

function setHover(hex) {
  const k = hex ? keyOf(hex.q, hex.r) : null
  if (k === hoverKey) return
  hoverKey = k
  hoverHex.value = hex
  hoverEntries.value = hex ? describeHex(hex.q, hex.r) : []
  scheduleDraw()
}

function startPan(e, sx, sy, selectOnClick) {
  canvasEl.value.setPointerCapture(e.pointerId)
  drag = { type: 'pan', sx, sy, viewX: view.x, viewY: view.y, moved: false, selectOnClick }
  if (!selectOnClick) panning.value = true
}

// ---- Touch: two fingers pinch to zoom and drag to move ---------------------------
const touches = new Map() // pointerId -> { sx, sy }
let pinch = null // { dist, mx, my }

function pinchState() {
  const [a, b] = [...touches.values()]
  return { dist: Math.hypot(b.sx - a.sx, b.sy - a.sy), mx: (a.sx + b.sx) / 2, my: (a.sy + b.sy) / 2 }
}

function onPointerDown(e) {
  const { sx, sy } = eventPoint(e)
  if (e.pointerType === 'touch') {
    touches.set(e.pointerId, { sx, sy })
    try {
      canvasEl.value.setPointerCapture(e.pointerId)
    } catch (err) {
      // Not a live pointer (it already lifted); nothing to capture.
    }
    if (touches.size === 2) {
      // A second finger turns whatever the first one started into a pinch;
      // a stroke painted so far is kept.
      if (drag && (drag.type === 'paint' || drag.type === 'erase')) commitStroke(drag)
      drag = null
      panning.value = false
      pinch = pinchState()
      return
    }
    if (touches.size > 2) return
  }
  const wantsPan = e.button === 1 || (e.button === 0 && (spaceDown.value || tool.value === 'pan'))
  if (wantsPan) {
    startPan(e, sx, sy, false)
    return
  }
  if (e.button !== 0) return

  if (tool.value === 'paint' || tool.value === 'erase') {
    if (tool.value === 'paint' && !locMap.has(activeId.value)) {
      flash(t('Pick a major location in the bar below to paint with.'))
      return
    }
    canvasEl.value.setPointerCapture(e.pointerId)
    drag = { type: tool.value, locId: activeId.value, last: null, changed: new Map(), before: new Map() }
    strokeTo(hexAt(sx, sy))
    return
  }

  // Select tool: dragging moves the view, a plain click selects a hex.
  startPan(e, sx, sy, true)
}

function onPointerMove(e) {
  const { sx, sy } = eventPoint(e)
  if (touches.has(e.pointerId)) touches.set(e.pointerId, { sx, sy })
  if (pinch && touches.size >= 2) {
    const now = pinchState()
    view.x += now.mx - pinch.mx
    view.y += now.my - pinch.my
    if (pinch.dist > 0) zoomAt(now.mx, now.my, now.dist / pinch.dist)
    pinch = now
    scheduleDraw()
    return
  }
  if (drag && drag.type === 'pan') {
    const dx = sx - drag.sx
    const dy = sy - drag.sy
    if (!drag.moved && Math.hypot(dx, dy) > 4) {
      drag.moved = true
      panning.value = true
    }
    if (drag.moved || !drag.selectOnClick) {
      view.x = drag.viewX + dx
      view.y = drag.viewY + dy
      scheduleDraw()
    }
    return
  }
  const hex = hexAt(sx, sy)
  if (drag && (drag.type === 'paint' || drag.type === 'erase')) strokeTo(hex)
  setHover(hex)
}

function onPointerUp(e) {
  touches.delete(e.pointerId)
  if (pinch) {
    // Lifting one finger of a pinch ends it; the other finger does nothing
    // until it is lifted too.
    if (touches.size === 0) pinch = null
    const canvas = canvasEl.value
    if (canvas?.hasPointerCapture?.(e.pointerId)) canvas.releasePointerCapture(e.pointerId)
    return
  }
  const finished = drag
  drag = null
  panning.value = false
  const canvas = canvasEl.value
  if (canvas && canvas.hasPointerCapture && canvas.hasPointerCapture(e.pointerId)) {
    canvas.releasePointerCapture(e.pointerId)
  }
  if (!finished) return
  if (finished.type === 'pan' && finished.selectOnClick && !finished.moved && placingEquator.value) {
    const { sx, sy } = eventPoint(e)
    placeEquator(hexAt(sx, sy))
  } else if (finished.type === 'pan' && finished.selectOnClick && !finished.moved && tool.value === 'fill') {
    const { sx, sy } = eventPoint(e)
    bucketAt(hexAt(sx, sy))
  } else if (finished.type === 'pan' && finished.selectOnClick && !finished.moved) {
    const { sx, sy } = eventPoint(e)
    selectedHex.value = hexAt(sx, sy)
    focusId.value = null
    refreshPanels()
    scheduleDraw()
  } else if (finished.type === 'paint' || finished.type === 'erase') {
    commitStroke(finished)
  }
}

function onPointerLeave() {
  if (!drag) setHover(null)
}

function onWheel(e) {
  e.preventDefault()
  const { sx, sy } = eventPoint(e)
  zoomAt(sx, sy, Math.exp(-e.deltaY * 0.0015))
}

// ---- Painting ------------------------------------------------------------------------------
// The local edits below mirror what the server does for the same request, so
// the map responds instantly and the stroke is saved once it ends.

function paintLocal(k, q, r, locId) {
  const loc = locMap.get(locId)
  if (!loc || loc.kind !== 'major') return false
  const entry = hexMembers.get(k)
  let ids = entry ? entry.ids : []
  if (ids.includes(locId)) return false
  // A hex has one kingdom: painting a colored location replaces any other.
  if (loc.color) ids = ids.filter((id) => !locMap.get(id)?.color)
  const [x, y] = entry ? [entry.x, entry.y] : hexCenter(q, r)
  hexMembers.set(k, { q, r, x, y, ids: [...ids, locId] })
  if (loc.color) painted.set(k, { q, r, x, y, color: loc.color })
  hexesOfCache.clear()
  return true
}

function eraseLocal(k, locId) {
  const entry = hexMembers.get(k)
  if (!entry) return false
  let ids
  if (locId == null) {
    ids = []
  } else {
    if (!entry.ids.includes(locId)) return false
    ids = entry.ids.filter((id) => id !== locId)
  }
  if (ids.length === 0) hexMembers.delete(k)
  else hexMembers.set(k, { ...entry, ids })
  const owner = coloredOf(ids)
  if (owner) painted.set(k, { q: entry.q, r: entry.r, x: entry.x, y: entry.y, color: owner.color })
  else painted.delete(k)
  hexesOfCache.clear()
  return true
}

function strokeTo(hex) {
  if (!hex || !drag) return
  const line = drag.last ? hexLine(drag.last, hex) : [hex]
  const path = brush.value ? line.flatMap((c) => hexesWithin(c, brush.value)) : line
  for (const h of path) {
    const k = keyOf(h.q, h.r)
    if (!drag.before.has(k)) drag.before.set(k, idsAt(k))
    const changed =
      drag.type === 'paint' ? paintLocal(k, h.q, h.r, drag.locId) : eraseLocal(k, drag.locId)
    if (changed) drag.changed.set(k, [h.q, h.r])
  }
  drag.last = hex
  scheduleDraw()
}

// Strokes are saved one after another so quick back-to-back strokes can't
// reach the server out of order.
function commitStroke(stroke) {
  const hexes = [...stroke.changed.values()]
  if (!hexes.length) return
  remember([...stroke.changed].map(([k, [q, r]]) => ({ k, q, r, before: stroke.before.get(k), after: idsAt(k) })))
  saveChain = saveChain.then(async () => {
    try {
      if (stroke.type === 'paint') await mapApi.paint(stroke.locId, hexes)
      else await mapApi.erase(stroke.locId, hexes)
    } catch (err) {
      flash(t("Couldn't save that stroke, so the map was reloaded."))
      await load({ keepView: true })
    }
    refreshPanels()
  })
}

// ---- Bucket --------------------------------------------------------------------------------
// A click fills the connected empty hexes around the clicked one
// (bucketFill.js); hexes with anything on them are left alone. The area must
// be closed in: past the painted part of the map, the 2D map runs on forever.
const HEX_STEPS = [[1, 0], [-1, 0], [0, 1], [0, -1], [1, -1], [-1, 1]]
const bucketAsk = ref(null) // { cells, locId, name }: a big fill waiting for a yes

function bucketAt(hex) {
  if (!hex) return
  const loc = locMap.get(activeId.value)
  if (!loc) {
    flash(t('Pick a major location in the bar below to paint with.'))
    return
  }
  if (idsAt(keyOf(hex.q, hex.r)).length) return flash(t('The bucket only fills empty hexes.'))
  let minQ = Infinity, maxQ = -Infinity, minR = Infinity, maxR = -Infinity
  for (const h of hexMembers.values()) {
    minQ = Math.min(minQ, h.q)
    maxQ = Math.max(maxQ, h.q)
    minR = Math.min(minR, h.r)
    maxR = Math.max(maxR, h.r)
  }
  const { cells, open, tooBig } = floodFill(keyOf(hex.q, hex.r), {
    neighbors: (k) => {
      const [q, r] = k.split(',').map(Number)
      return HEX_STEPS.map(([dq, dr]) => keyOf(q + dq, r + dr))
    },
    empty: (k) => !idsAt(k).length,
    escapes: (k) => {
      const [q, r] = k.split(',').map(Number)
      return q < minQ || q > maxQ || r < minR || r > maxR
    },
  })
  if (open) return flash(t("That area isn't closed in, so it would never end. Paint around it first."))
  if (tooBig) return flash(t('That area is too big to fill at once.'))
  if (cells.length > BUCKET_WARN) bucketAsk.value = { cells, locId: loc.id, name: loc.name }
  else fill(cells, loc.id)
}

function confirmBucket() {
  const { cells, locId } = bucketAsk.value
  bucketAsk.value = null
  fill(cells, locId)
}

// Painted like one stroke, so Undo takes it back whole.
function fill(cells, locId) {
  const stroke = { type: 'paint', locId, changed: new Map(), before: new Map() }
  for (const k of cells) {
    const [q, r] = k.split(',').map(Number)
    stroke.before.set(k, idsAt(k))
    if (paintLocal(k, q, r, locId)) stroke.changed.set(k, [q, r])
  }
  commitStroke(stroke)
  scheduleDraw()
}

// ---- Undo / redo ---------------------------------------------------------------------------
// Every stroke remembers what its hexes held before and after it, so it can
// be played back either way. Only painting and erasing are undoable.
const UNDO_LIMIT = 50
const undoStack = ref([])
const redoStack = ref([])

const idsAt = (k) => [...(hexMembers.get(k)?.ids ?? [])]

function remember(hexes) {
  undoStack.value = [...undoStack.value.slice(1 - UNDO_LIMIT), hexes]
  redoStack.value = []
}

// Puts each hex back to the given list of location ids, on screen at once
// and then on the server, as the erases and paints that turn one into the
// other. Locations deleted since are skipped.
function restore(hexes, which) {
  const erase = new Map()
  const paint = new Map()
  const add = (m, id, h) => {
    if (!m.has(id)) m.set(id, [])
    m.get(id).push([h.q, h.r])
  }
  for (const h of hexes) {
    const target = h[which].filter((id) => locMap.has(id))
    const now = idsAt(h.k)
    for (const id of now) if (!target.includes(id)) add(erase, id, h)
    for (const id of target) if (!now.includes(id)) add(paint, id, h)
    const entry = hexMembers.get(h.k)
    const [x, y] = entry ? [entry.x, entry.y] : hexCenter(h.q, h.r)
    if (target.length) hexMembers.set(h.k, { q: h.q, r: h.r, x, y, ids: target })
    else hexMembers.delete(h.k)
    const owner = coloredOf(target)
    if (owner) painted.set(h.k, { q: h.q, r: h.r, x, y, color: owner.color })
    else painted.delete(h.k)
  }
  hexesOfCache.clear()
  scheduleDraw()
  saveChain = saveChain.then(async () => {
    try {
      for (const [id, list] of erase) await mapApi.erase(id, list)
      // Kingdoms last: painting one claims the hex from any other.
      const order = [...paint].sort(([a], [b]) => Number(!!locMap.get(a)?.color) - Number(!!locMap.get(b)?.color))
      for (const [id, list] of order) await mapApi.paint(id, list)
    } catch (err) {
      flash(t("Couldn't save that change, so the map was reloaded."))
      await load({ keepView: true })
    }
    refreshPanels()
  })
}

function undo() {
  const last = undoStack.value[undoStack.value.length - 1]
  if (!last) return
  undoStack.value = undoStack.value.slice(0, -1)
  redoStack.value = [...redoStack.value, last]
  restore(last, 'before')
}

function redo() {
  const next = redoStack.value[redoStack.value.length - 1]
  if (!next) return
  redoStack.value = redoStack.value.slice(0, -1)
  undoStack.value = [...undoStack.value, next]
  restore(next, 'after')
}

// ---- Keyboard --------------------------------------------------------------------------------
function isTyping(e) {
  const t = e.target
  return !!t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.tagName === 'SELECT' || t.isContentEditable)
}

function onKeyDown(e) {
  if (e.code === 'Space' && !isTyping(e)) {
    e.preventDefault()
    // A focused button would otherwise be "clicked" by Space.
    if (document.activeElement && document.activeElement.tagName === 'BUTTON') document.activeElement.blur()
    spaceDown.value = true
  } else if (e.key === 'Escape' && !isTyping(e)) {
    closePanel()
  } else if ((e.ctrlKey || e.metaKey) && !isTyping(e) && !drag) {
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

function onKeyUp(e) {
  if (e.code === 'Space') {
    if (!isTyping(e)) e.preventDefault()
    spaceDown.value = false
  }
}

function onWindowBlur() {
  spaceDown.value = false
}

// ---- Panels & location editing ------------------------------------------------------------------
function closePanel() {
  selectedHex.value = null
  panel.value = null
  scheduleDraw()
}

function toggleActive(m) {
  activeId.value = activeId.value === m.id ? null : m.id
}

function closeMajorModal() {
  newMajorOpen.value = false
  editMajor.value = null
}

async function onMajorSaved(loc) {
  const wasNew = !editMajor.value
  closeMajorModal()
  await load({ keepView: true })
  if (wasNew) {
    // Straight into painting with the location that was just made.
    activeId.value = loc.id
    tool.value = 'paint'
  }
}

async function onMajorDeleted() {
  closeMajorModal()
  await load({ keepView: true })
}

// These throw on failure; the hex panel shows the message.
async function saveMinor(payload) {
  const hex = selectedHex.value
  const body = {
    kind: 'minor',
    name: payload.name,
    founding_date: payload.founding_date,
    description: payload.description,
    belongs_to_id: payload.belongs_to_id,
    is_city: payload.is_city,
    is_capital: payload.is_capital,
    q: hex.q,
    r: hex.r,
  }
  if (payload.id) {
    const existing = locMap.get(payload.id)
    body.q = existing.q
    body.r = existing.r
    await mapApi.updateLocation(payload.id, body)
  } else {
    await mapApi.createLocation(body)
  }
  await load({ keepView: true })
}

async function deleteMinor(loc) {
  await mapApi.deleteLocation(loc.id)
  await load({ keepView: true })
}

async function placeLocation(loc) {
  const hex = selectedHex.value
  await mapApi.updateLocation(loc.id, {
    kind: 'minor',
    name: loc.name,
    founding_date: loc.founding_date,
    description: loc.description,
    belongs_to_id: loc.belongs_to_id,
    is_city: loc.is_city,
    is_capital: loc.is_capital,
    q: hex.q,
    r: hex.r,
  })
  await load({ keepView: true })
}

// ---- Lifecycle --------------------------------------------------------------------------------------
watch([tool, activeId, selectedHex, panning, brush], scheduleDraw)
watch(equator, scheduleDraw)

onMounted(async () => {
  updateSize()
  resizeObserver = new ResizeObserver(updateSize)
  resizeObserver.observe(wrapEl.value)
  canvasEl.value.addEventListener('wheel', onWheel, { passive: false })
  window.addEventListener('keydown', onKeyDown)
  window.addEventListener('keyup', onKeyUp)
  window.addEventListener('blur', onWindowBlur)
  await load()
  openFromQuery()
  focusFromQuery()
})

// /map?edit=<id> (the Home page's loose ends) opens that major location's
// editor straight away. The query is dropped so a reload doesn't reopen it.
const route = useRoute()
const router = useRouter()
function openFromQuery() {
  const id = Number(route.query.edit)
  if (!id) return
  const loc = locMap.get(id)
  if (loc && loc.kind === 'major') {
    activeId.value = loc.id
    editMajor.value = loc
  }
  const query = { ...route.query }
  delete query.edit
  router.replace({ query })
}

// /map?focus=<id> (links from events, the timeline, the wiki…) centres and
// zooms the view on that location. The query stays, so a reload keeps it.
function focusFromQuery() {
  const id = Number(route.query.focus)
  if (!id) return
  const loc = locMap.get(id)
  if (!loc) return
  if (!focusLocation(loc)) flash(t('{name} isn\'t drawn on the map yet.', { name: loc.name }))
}
// Following another such link while the map is already open.
watch(() => route.query.focus, () => {
  if (route.path === '/map') focusFromQuery()
})

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  canvasEl.value?.removeEventListener('wheel', onWheel)
  window.removeEventListener('keydown', onKeyDown)
  window.removeEventListener('keyup', onKeyUp)
  window.removeEventListener('blur', onWindowBlur)
  clearTimeout(noticeTimer)
})
</script>

<style scoped src="../mapChrome.css"></style>

<style scoped>
.map-canvas {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  display: block;
  touch-action: none;
}

.cursor-grab .map-canvas {
  cursor: grab;
}

.cursor-grabbing .map-canvas {
  cursor: grabbing;
}

.cursor-cross .map-canvas {
  cursor: crosshair;
}

.equator-panel {
  position: absolute;
  top: 16px;
  left: 100px;
  width: 250px;
  max-height: calc(100% - 32px);
  overflow-y: auto;
  padding: 0.75rem 0.85rem;
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 0.55rem;
  font-size: 0.8rem;
  z-index: 8;
}

.eq-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.eq-close {
  border: none;
  background: none;
  color: var(--text-muted);
  font-size: 1.1rem;
  line-height: 1;
  cursor: pointer;
}

.eq-check {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  color: var(--text-primary);
}

.eq-field {
  display: grid;
  grid-template-columns: 3.2rem 1fr auto;
  align-items: center;
  gap: 0.5rem;
  color: var(--text-muted);
}

.eq-field input[type='number'] {
  grid-column: 2 / 4;
  min-height: 1.9rem;
  padding: 0.2rem 0.45rem;
}

.eq-sep {
  border-top: 1px solid var(--glass-border);
}

/* A longer label above its box. */
.eq-wide {
  grid-template-columns: 1fr;
  gap: 0.2rem;
}

.eq-wide input[type='number'] {
  grid-column: 1;
}

.eq-val {
  min-width: 2.2rem;
  text-align: right;
  color: var(--text-primary);
}

.is-placing {
  border-color: var(--accent);
  color: var(--accent);
}

.eq-note {
  margin: 0;
  font-size: 0.72rem;
  line-height: 1.45;
  color: var(--text-faint);
}

@media (max-width: 720px) {
  .equator-panel {
    left: 80px;
    right: 8px;
    width: auto;
  }
}
</style>
