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
import { t, tr } from '../i18n'
import { mapApi } from '../api'
import { themeRgba } from '../theme'
import { HEX_SIZE, SQRT3, CORNERS, keyOf, hexCenter, pixelToHex, hexLine } from '../hexMath'
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
      type: 'Location',
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

  // 5. Minor locations: a marker per hex (a ring when there are several).
  for (const list of minorsByKey.values()) {
    const [cx, cy] = hexCenter(list[0].q, list[0].r)
    if (!inView(cx, cy)) continue
    const rad = HEX_SIZE * 0.26
    if (list.length > 1) {
      ctx.beginPath()
      ctx.arc(cx, cy, rad * 1.6, 0, Math.PI * 2)
      ctx.strokeStyle = themeRgba('--accent', 0.75)
      ctx.lineWidth = 1.5 / z
      ctx.stroke()
    }
    ctx.beginPath()
    ctx.arc(cx, cy, rad, 0, Math.PI * 2)
    ctx.fillStyle = themeRgba('--accent')
    ctx.fill()
    ctx.strokeStyle = themeRgba('--surface', 0.95)
    ctx.lineWidth = 1.5 / z
    ctx.stroke()

    if (screenSize >= 26) {
      const first = list[0].name
      const text = first.length > 16 ? `${first.slice(0, 15)}…` : first
      const label = list.length > 1 ? `${text} +${list.length - 1}` : text
      ctx.font = `600 ${11 / z}px Manrope, sans-serif`
      ctx.textAlign = 'center'
      ctx.textBaseline = 'top'
      ctx.lineJoin = 'round'
      ctx.lineWidth = 3 / z
      ctx.strokeStyle = themeRgba('--surface', 0.9)
      ctx.strokeText(label, cx, cy + rad + 3 / z)
      ctx.fillStyle = themeRgba('--text-primary')
      ctx.fillText(label, cx, cy + rad + 3 / z)
    }
  }

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
    const [cx, cy] = hexCenter(hoverHex.value.q, hoverHex.value.r)
    ctx.beginPath()
    addHexPath(ctx, cx, cy, HEX_SIZE)
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

function onPointerDown(e) {
  const { sx, sy } = eventPoint(e)
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
    drag = { type: tool.value, locId: activeId.value, last: null, changed: new Map() }
    strokeTo(hexAt(sx, sy))
    return
  }

  // Select tool: dragging moves the view, a plain click selects a hex.
  startPan(e, sx, sy, true)
}

function onPointerMove(e) {
  const { sx, sy } = eventPoint(e)
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
  const path = drag.last ? hexLine(drag.last, hex) : [hex]
  for (const h of path) {
    const k = keyOf(h.q, h.r)
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
    q: hex.q,
    r: hex.r,
  })
  await load({ keepView: true })
}

// ---- Lifecycle --------------------------------------------------------------------------------------
watch([tool, activeId, selectedHex, panning], scheduleDraw)

onMounted(async () => {
  updateSize()
  resizeObserver = new ResizeObserver(updateSize)
  resizeObserver.observe(wrapEl.value)
  canvasEl.value.addEventListener('wheel', onWheel, { passive: false })
  window.addEventListener('keydown', onKeyDown)
  window.addEventListener('keyup', onKeyUp)
  window.addEventListener('blur', onWindowBlur)
  await load()
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
</style>
