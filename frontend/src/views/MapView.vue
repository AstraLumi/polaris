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
      </div>

      <div class="top-notes">
        <p class="hint-line">{{ hint }}</p>
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

      <div class="kingdom-bar glass-panel panel-solid">
        <div class="chips">
          <div
            v-for="m in majors"
            :key="m.id"
            class="chip"
            :class="{ active: activeId === m.id }"
            :title="m.color ? $t('Kingdom') : $t('Colorless major location')"
            @click="toggleActive(m)"
          >
            <span class="chip-dot" :class="{ none: !m.color }" :style="m.color ? { background: m.color } : {}"></span>
            <span class="chip-name">{{ m.name }}</span>
            <button type="button" class="chip-edit" :title="$t('Edit')" @click.stop="editMajor = m">✎</button>
          </div>
          <span v-if="!majors.length" class="chip-empty">{{ $t('No major locations yet') }}</span>
        </div>
        <button type="button" class="btn btn-primary small" @click="newMajorOpen = true">
          {{ $t('New major location') }}
        </button>
      </div>
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
import { useRoute, useRouter } from 'vue-router'
import { t, tr } from '../i18n'
import { mapApi } from '../api'
import { themeRgba } from '../theme'
import { HEX_SIZE, SQRT3, CORNERS, keyOf, hexCenter, pixelToHex, hexLine, hexesWithin } from '../hexMath'
import MapHoverPane from '../components/MapHoverPane.vue'
import HexPanel from '../components/HexPanel.vue'
import MajorLocationModal from '../components/MajorLocationModal.vue'

const ZOOM_MIN = 0.15
const ZOOM_MAX = 4

const tools = [
  { key: 'select', label: tr('Select'), title: tr('Click a hex to inspect it and add locations. Drag to move the view.') },
  { key: 'paint', label: tr('Paint'), title: tr('Paint hexes with the selected major location') },
  { key: 'erase', label: tr('Erase'), title: tr('Erase the selected major location from hexes (everything, if none is selected)') },
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
const takenColors = computed(() => majors.value.filter((m) => m.color).map((m) => m.color))
const activeName = computed(() => majors.value.find((m) => m.id === activeId.value)?.name ?? '')

const wrapClass = computed(() => ({
  'cursor-grab': (tool.value === 'pan' || spaceDown.value) && !panning.value,
  'cursor-grabbing': panning.value,
  'cursor-cross': (tool.value === 'paint' || tool.value === 'erase') && !spaceDown.value,
}))

const hint = computed(() => {
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
      type: 'Kingdom',
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

function drawDot(ctx, cx, cy, rad, z) {
  ctx.beginPath()
  ctx.arc(cx, cy, rad, 0, Math.PI * 2)
  ctx.fillStyle = themeRgba('--accent')
  ctx.fill()
  ctx.strokeStyle = themeRgba('--surface', 0.95)
  ctx.lineWidth = 1.5 / z
  ctx.stroke()
}

// A city: a dark medallion with a little castle in the accent colour.
function drawCity(ctx, cx, cy, r, z) {
  ctx.beginPath()
  ctx.arc(cx, cy, r, 0, Math.PI * 2)
  ctx.fillStyle = themeRgba('--surface', 0.96)
  ctx.fill()
  ctx.strokeStyle = themeRgba('--accent')
  ctx.lineWidth = Math.max(1.4, r * z * 0.11) / z
  ctx.stroke()

  const u = (r * 0.62) / 6 // the castle is drawn on a 12 x 11 grid
  ctx.save()
  ctx.translate(cx, cy + u * 0.4)
  ctx.scale(u, u)
  ctx.beginPath()
  ctx.moveTo(-6, 5.5)
  ctx.lineTo(-6, -5.5)
  ctx.lineTo(-3.6, -5.5)
  ctx.lineTo(-3.6, -3.4)
  ctx.lineTo(-1.2, -3.4)
  ctx.lineTo(-1.2, -5.5)
  ctx.lineTo(1.2, -5.5)
  ctx.lineTo(1.2, -3.4)
  ctx.lineTo(3.6, -3.4)
  ctx.lineTo(3.6, -5.5)
  ctx.lineTo(6, -5.5)
  ctx.lineTo(6, 5.5)
  ctx.closePath()
  ctx.fillStyle = themeRgba('--accent')
  ctx.fill()
  // The gate.
  ctx.beginPath()
  ctx.moveTo(-1.7, 5.5)
  ctx.lineTo(-1.7, 1.6)
  ctx.arc(0, 1.6, 1.7, Math.PI, 0)
  ctx.lineTo(1.7, 5.5)
  ctx.closePath()
  ctx.fillStyle = themeRgba('--surface', 0.96)
  ctx.fill()
  ctx.restore()
}

// A capital: a glowing four-pointed star (like the Polaris logo) rising out of
// a ring, larger than anything else on the map.
function drawCapital(ctx, cx, cy, R, z) {
  const glow = ctx.createRadialGradient(cx, cy, R * 0.2, cx, cy, R * 2.1)
  glow.addColorStop(0, themeRgba('--accent', 0.38))
  glow.addColorStop(1, themeRgba('--accent', 0))
  ctx.beginPath()
  ctx.arc(cx, cy, R * 2.1, 0, Math.PI * 2)
  ctx.fillStyle = glow
  ctx.fill()

  ctx.beginPath()
  ctx.arc(cx, cy, R * 0.56, 0, Math.PI * 2)
  ctx.fillStyle = themeRgba('--surface', 0.92)
  ctx.fill()
  ctx.strokeStyle = themeRgba('--accent')
  ctx.lineWidth = Math.max(1.4, R * z * 0.07) / z
  ctx.stroke()

  const star = (len, waist, rot) => {
    ctx.save()
    ctx.translate(cx, cy)
    ctx.rotate(rot)
    ctx.beginPath()
    ctx.moveTo(0, -len)
    ctx.quadraticCurveTo(waist, -waist, len, 0)
    ctx.quadraticCurveTo(waist, waist, 0, len)
    ctx.quadraticCurveTo(-waist, waist, -len, 0)
    ctx.quadraticCurveTo(-waist, -waist, 0, -len)
    ctx.closePath()
    ctx.restore()
  }
  star(R, R * 0.13, 0)
  ctx.fillStyle = themeRgba('--accent')
  ctx.fill()
  ctx.strokeStyle = themeRgba('--surface', 0.95)
  ctx.lineWidth = 1.4 / z
  ctx.stroke()
  star(R * 0.5, R * 0.1, Math.PI / 4)
  ctx.fillStyle = themeRgba('--accent', 0.85)
  ctx.fill()

  ctx.beginPath()
  ctx.arc(cx, cy, R * 0.1, 0, Math.PI * 2)
  ctx.fillStyle = themeRgba('--surface')
  ctx.fill()
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

  // 2. Kingdom fills, batched per color.
  const groups = new Map()
  for (const h of painted.values()) {
    if (!inView(h.x, h.y)) continue
    let g = groups.get(h.color)
    if (!g) {
      g = []
      groups.set(h.color, g)
    }
    g.push(h.x, h.y)
  }
  for (const [color, pts] of groups) {
    ctx.beginPath()
    for (let i = 0; i < pts.length; i += 2) addHexPath(ctx, pts[i], pts[i + 1], HEX_SIZE)
    ctx.fillStyle = color
    ctx.fill()
    ctx.strokeStyle = 'rgba(0, 0, 0, 0.3)'
    ctx.lineWidth = 1 / z
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
  if (hoverHex.value) {
    const ids = hexMembers.get(keyOf(hoverHex.value.q, hoverHex.value.r))?.ids ?? []
    for (const id of ids) {
      const l = locMap.get(id)
      if (l && !l.color) outline(hexesOf(id), themeRgba('--accent', 0.95), 2)
    }
  }

  // 5. Minor locations. Capitals and cities get their own markers and are
  //    labelled from further out than ordinary places (see LABEL_AT); markers
  //    are drawn lowest rank first so a capital is never buried, then the
  //    labels go on top of everything.
  const spots = minorSpots.filter((spot) => inView(spot.x, spot.y))
  for (const spot of spots) drawMinorMarker(ctx, spot, z)
  drawMinorLabels(ctx, spots, z, screenSize)

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
  if (finished.type === 'pan' && finished.selectOnClick && !finished.moved) {
    const { sx, sy } = eventPoint(e)
    selectedHex.value = hexAt(sx, sy)
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

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  canvasEl.value?.removeEventListener('wheel', onWheel)
  window.removeEventListener('keydown', onKeyDown)
  window.removeEventListener('keyup', onKeyUp)
  window.removeEventListener('blur', onWindowBlur)
  clearTimeout(noticeTimer)
})
</script>

<style scoped>
.map-page {
  height: calc(100vh - 3rem);
  min-height: 480px;
}

.map-wrap {
  position: relative;
  height: 100%;
  overflow: hidden;
  border: 1px solid var(--glass-border);
  border-radius: 16px;
  background:
    radial-gradient(circle at 25% 15%, color-mix(in srgb, var(--tint) 10%, transparent), transparent 55%),
    radial-gradient(circle at 85% 90%, color-mix(in srgb, var(--accent) 6%, transparent), transparent 50%),
    var(--bg);
}

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

.toolbar {
  position: absolute;
  top: 16px;
  left: 16px;
  min-width: 64px;
  width: max-content;
  padding: 6px;
  display: flex;
  flex-direction: column;
  gap: 4px;
  z-index: 7;
}

.tool-btn {
  background: transparent;
  border: 1px solid transparent;
  border-radius: 8px;
  color: var(--text-muted);
  font-family: 'Manrope', sans-serif;
  font-size: 0.74rem;
  font-weight: 600;
  padding: 0.5rem 0.2rem;
  cursor: pointer;
}

.tool-btn:disabled {
  opacity: 0.35;
  cursor: default;
}

.history-btn {
  font-size: 1.05rem;
  padding: 0.25rem 0.2rem;
}

.brush-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 1.9rem;
}

.brush-dot {
  display: block;
  border-radius: 50%;
  background: currentColor;
}

.tool-btn:hover {
  color: var(--text-primary);
  background: rgba(255, 255, 255, 0.05);
}

.tool-btn.active {
  color: var(--text-primary);
  background: var(--accent-soft);
  border-color: color-mix(in srgb, var(--accent) 35%, transparent);
}

.tool-sep {
  height: 1px;
  background: var(--glass-border);
  margin: 4px 2px;
}

.zoom-label {
  text-align: center;
  font-size: 0.68rem;
  color: var(--text-faint);
  padding: 2px 0;
}

.top-notes {
  position: absolute;
  top: 16px;
  left: 50%;
  transform: translateX(-50%);
  max-width: min(560px, calc(100% - 760px));
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
  pointer-events: none;
  z-index: 4;
}

.hint-line {
  margin: 0;
  font-size: 0.75rem;
  color: var(--text-faint);
  text-align: center;
}

.notice {
  margin: 0;
}

.kingdom-bar {
  position: absolute;
  bottom: 16px;
  left: 50%;
  transform: translateX(-50%);
  max-width: min(900px, calc(100% - 200px));
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.55rem 0.7rem;
  z-index: 7;
}

.chips {
  display: flex;
  gap: 0.4rem;
  overflow-x: auto;
  min-width: 0;
  padding: 2px;
}

.chip {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  padding: 0.35rem 0.4rem 0.35rem 0.65rem;
  border: 1px solid var(--glass-border);
  border-radius: 999px;
  color: var(--text-muted);
  font-size: 0.8rem;
  cursor: pointer;
  flex-shrink: 0;
}

.chip:hover {
  color: var(--text-primary);
}

.chip.active {
  border-color: var(--accent);
  color: var(--text-primary);
  background: var(--accent-soft);
}

.chip-dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
}

.chip-dot.none {
  border: 1.5px dashed var(--text-faint);
}

.chip-edit {
  background: transparent;
  border: none;
  color: var(--text-faint);
  cursor: pointer;
  font-size: 0.8rem;
  padding: 0 0.3rem;
}

.chip-edit:hover {
  color: var(--text-primary);
}

.chip-empty {
  font-size: 0.8rem;
  color: var(--text-faint);
  padding: 0 0.4rem;
  white-space: nowrap;
}

/* Phones: the map fills the screen under the top bar, the keyboard hint is
   dropped (two fingers pinch and drag instead) and the location bar spans
   the width. */
@media (max-width: 720px) {
  .map-page {
    height: calc(100dvh - 6.5rem);
    min-height: 420px;
  }
  .hint-line {
    display: none;
  }
  .top-notes {
    left: 80px;
    right: 8px;
    max-width: none;
    transform: none;
  }
  .kingdom-bar {
    left: 8px;
    right: 8px;
    bottom: 8px;
    max-width: none;
    transform: none;
  }
  .toolbar {
    top: 8px;
    left: 8px;
  }
}
</style>
