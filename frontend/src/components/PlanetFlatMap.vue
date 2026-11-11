<template>
  <div ref="wrapEl" class="flat-wrap" :class="{ 'is-grabbing': drag }">
    <canvas
      ref="canvasEl"
      class="flat-canvas"
      @pointerdown="onDown"
      @pointermove="onMove"
      @pointerup="onUp"
      @pointercancel="onUp"
      @pointerleave="onLeave"
    ></canvas>
  </div>
</template>

<script setup>
import { ref, watch, onMounted, onBeforeUnmount } from 'vue'
import { PlanetGrid, fromLatLon } from '../planetGrid'
import { projectionOf } from '../projection'
import { ProjectedCells, drawPlanet, mapBounds, byColor } from '../planetImage'
import { pickLabels } from '../planetLabels'
import { drawDot, drawCity, drawCapital, lighten } from '../mapMarkers'

// The planet laid flat in a projection, to look at only: drag to move,
// scroll or pinch to zoom, hover a cell to see what is there. Painting
// happens on the globe.
const props = defineProps({
  frequency: { type: Number, required: true },
  projection: { type: String, default: 'equirectangular' },
  // Every cell with a colour (the rest is sea).
  cells: { type: Array, default: () => [] },
  colorOf: { type: Function, required: true },
  version: { type: Number, default: 0 },
  markers: { type: Array, default: () => [] },
  lines: { type: Object, default: null },
  // As PlanetGlobe: territory names, and oceans drawn as outlines.
  areas: { type: Array, default: () => [] },
  borders: { type: Array, default: () => [] },
  // Pixels covered on the right (the side list): the map fits beside them.
  insetRight: { type: Number, default: 0 },
})
const emit = defineEmits(['hover'])

const wrapEl = ref(null)
const canvasEl = ref(null)
const drag = ref(null)

let grid = null
let projected = null
const view = { scale: 100, ox: 0, oy: 0 }
const size = { w: 0, h: 0, dpr: 1 }
let queued = false
let resizeObserver = null

function fit() {
  const [x0, y0, x1, y1] = mapBounds(props.projection)
  const w = Math.max(100, size.w - props.insetRight)
  view.scale = Math.min((w - 32) / (x1 - x0), (size.h - 32) / (y1 - y0))
  view.ox = w / 2 - ((x0 + x1) / 2) * view.scale
  view.oy = size.h / 2 + ((y0 + y1) / 2) * view.scale
}

function prepare() {
  grid = new PlanetGrid(props.frequency)
  projected = new ProjectedCells(grid, props.projection)
  groups = null
  fullAt = 0
  fit()
  schedule()
}

function schedule() {
  if (queued) return
  queued = true
  requestAnimationFrame(() => {
    queued = false
    frame()
  })
}

// ---- Moving without drawing every hex ---------------------------------------------
// A full picture is every coloured hex as a path: fine once, far too much
// sixty times a second on a phone with a painted planet. So while the map
// is being moved, and only when a full picture is slow here, the last one
// is kept and shown moved and stretched to where the map now is; a full
// picture follows now and then, when the fingers pause, and when they lift.
const SLOW_MS = 12 // a full picture that takes longer is not drawn per frame
let fullMs = 0 // how long the last full picture took
let fullAt = 0
let snap = null // { canvas, scale, ox, oy }: the last full picture and its view
let settleTimer = 0

function keepPicture() {
  const canvas = canvasEl.value
  if (!canvas || !canvas.width) return
  if (!snap || snap.canvas.width !== canvas.width || snap.canvas.height !== canvas.height) {
    snap = { canvas: document.createElement('canvas') }
    snap.canvas.width = canvas.width
    snap.canvas.height = canvas.height
  }
  const c = snap.canvas.getContext('2d')
  c.clearRect(0, 0, canvas.width, canvas.height)
  c.drawImage(canvas, 0, 0)
  Object.assign(snap, { scale: view.scale, ox: view.ox, oy: view.oy })
}

function drawFull() {
  const start = performance.now()
  draw()
  fullAt = performance.now()
  fullMs = fullAt - start
  if (moving() && fullMs > SLOW_MS) keepPicture()
}

function drawMoved() {
  const canvas = canvasEl.value
  const ctx = canvas.getContext('2d')
  const k = view.scale / snap.scale
  ctx.setTransform(1, 0, 0, 1, 0, 0)
  ctx.clearRect(0, 0, canvas.width, canvas.height)
  ctx.drawImage(snap.canvas, (view.ox - snap.ox * k) * size.dpr, (view.oy - snap.oy * k) * size.dpr, canvas.width * k, canvas.height * k)
}

const moving = () => pointers.size > 0 || wheeling

function frame() {
  clearTimeout(settleTimer)
  const quick = fullMs <= SLOW_MS
  // Now and then a full one all the same, so what slid into view fills in.
  const due = performance.now() - fullAt > Math.max(250, fullMs * 8)
  if (!moving() || quick || !snap || due) drawFull()
  else drawMoved()
  if (moving() && !quick) settleTimer = setTimeout(drawFull, 140)
}

// The cells by colour, worked out when they change and not for every picture.
let groups = null
function grouped() {
  if (!groups) groups = byColor(props.cells, props.colorOf)
  return groups
}

function css(name, fallback) {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim() || fallback
}

function draw() {
  const canvas = canvasEl.value
  if (!canvas || !projected) return
  const ctx = canvas.getContext('2d')
  ctx.setTransform(size.dpr, 0, 0, size.dpr, 0, 0)
  ctx.clearRect(0, 0, size.w, size.h)
  drawPlanet(ctx, projected, view, {
    cells: grouped(),
    lines: props.lines,
    land: (c, sx, sy) => {
      // Oceans: the outline of their water.
      for (const border of props.borders) {
        c.beginPath()
        for (const [[x0, y0], [x1, y1]] of projected.edges(border.cells)) {
          c.moveTo(sx(x0), sy(y0))
          c.lineTo(sx(x1), sy(y1))
        }
        c.strokeStyle = border.color
        c.lineWidth = 1.5
        c.stroke()
      }
      // Names and markers by zoom, as on the globe (planetLabels.js).
      const cellPx = view.scale * (Math.atan(2) / grid.n)
      const inView = (x, y) => x > -40 && x < size.w + 40 && y > -40 && y < size.h + 40
      const candidates = []
      for (const a of props.areas) {
        if (!a.cells?.length) continue
        const [x, y] = projected.labelPoint(a.cells)
        if (inView(sx(x), sy(y))) candidates.push({ kind: 'area', key: a.key, name: a.name, x: sx(x), y: sy(y), size: a.size, color: a.color, ocean: a.ocean })
      }
      for (const m of props.markers) {
        if (m.cell == null || m.cell >= grid.count) continue
        const [x, y] = projected.point(m.cell)
        if (inView(sx(x), sy(y))) candidates.push({ kind: 'marker', key: m.cell, name: m.name, rank: m.rank, x: sx(x), y: sy(y) })
      }
      c.lineJoin = 'round'
      c.textAlign = 'center'
      for (const l of pickLabels(candidates, cellPx).reverse()) {
        if (l.kind === 'area') {
          c.font = `700 ${l.size}px Fraunces, 'Times New Roman', serif`
          if ('letterSpacing' in c) c.letterSpacing = '1.5px'
          c.textBaseline = 'middle'
          // Kept whole inside the view, however near its edge the land lies.
          const half = c.measureText(l.name.toUpperCase()).width / 2 + 6
          const x = Math.min(Math.max(l.x, half), size.w - half)
          const y = Math.min(Math.max(l.y, l.size / 2 + 6), size.h - l.size / 2 - 6)
          c.lineWidth = 4
          c.strokeStyle = 'rgba(0, 0, 0, 0.75)'
          c.strokeText(l.name.toUpperCase(), x, y)
          c.fillStyle = lighten(l.color, 0.45)
          c.fillText(l.name.toUpperCase(), x, y)
          if ('letterSpacing' in c) c.letterSpacing = '0px'
          continue
        }
        const r = l.icon / 2
        if (l.rank === 2) drawCapital(c, l.x, l.y, r / 2.1, 1)
        else if (l.rank === 1) drawCity(c, l.x, l.y, r - 1, 1)
        else drawDot(c, l.x, l.y, r - 1, 1)
        if (!l.named) continue
        c.font = `${l.rank === 2 ? 700 : 600} ${l.rank === 2 ? 12 : 11}px Manrope, sans-serif`
        c.textBaseline = 'top'
        c.lineWidth = 3
        c.strokeStyle = css('--surface', '#10131c')
        const ty = l.y + (l.rank === 2 ? r * 0.55 : r) + 2
        const text = l.rank === 2 ? l.name.toUpperCase() : l.name
        c.strokeText(text, l.x, ty)
        c.fillStyle = l.rank === 2 ? css('--accent', '#e6c06a') : css('--text-primary', '#fff')
        c.fillText(text, l.x, ty)
      }
    },
  })
}

function resize() {
  const el = wrapEl.value
  const canvas = canvasEl.value
  if (!el || !canvas) return
  const first = !size.w
  size.w = el.clientWidth
  size.h = el.clientHeight
  size.dpr = window.devicePixelRatio || 1
  canvas.width = Math.round(size.w * size.dpr)
  canvas.height = Math.round(size.h * size.dpr)
  if (first) fit()
  fullAt = 0 // the kept picture was of another size
  schedule()
}

// ---- Moving and hovering ----------------------------------------------------------
function cellAt(e) {
  const rect = canvasEl.value.getBoundingClientRect()
  const x = (e.clientX - rect.left - view.ox) / view.scale
  const y = (view.oy - (e.clientY - rect.top)) / view.scale
  const ll = projectionOf(props.projection).inverse(x, y)
  return ll ? grid.cellAt(fromLatLon(ll[1], ll[0])) : null
}

const SCALE_MIN = 20
const SCALE_MAX = 20000

// Every finger (or the mouse button) that is down, by pointer id. One moves
// the map; two move it by their middle and zoom it by their distance.
const pointers = new Map() // pointerId -> { x, y } on the canvas
let grip = null // where the fingers and the view were when the last one came or went

function gripNow() {
  const pts = [...pointers.values()]
  if (!pts.length) return null
  const x = pts.reduce((sum, p) => sum + p.x, 0) / pts.length
  const y = pts.reduce((sum, p) => sum + p.y, 0) / pts.length
  const dist = pts.length > 1 ? Math.hypot(pts[0].x - pts[1].x, pts[0].y - pts[1].y) : 0
  return { x, y, dist }
}

// Taken again whenever a finger comes or goes, so the map never jumps.
function regrip() {
  const g = gripNow()
  grip = g && { ...g, scale: view.scale, ox: view.ox, oy: view.oy }
  drag.value = grip
}

function place(e) {
  const rect = canvasEl.value.getBoundingClientRect()
  return { x: e.clientX - rect.left, y: e.clientY - rect.top }
}

let lastHover = null
function onMove(e) {
  if (pointers.has(e.pointerId)) {
    pointers.set(e.pointerId, place(e))
    const now = gripNow()
    const k = grip.dist && now.dist ? now.dist / grip.dist : 1
    const scale = Math.max(SCALE_MIN, Math.min(SCALE_MAX, grip.scale * k))
    const f = scale / grip.scale
    // The point of the map that was under the fingers' middle stays there.
    view.scale = scale
    view.ox = now.x - (grip.x - grip.ox) * f
    view.oy = now.y - (grip.y - grip.oy) * f
    schedule()
    return
  }
  if (e.pointerType === 'touch') return
  const c = cellAt(e)
  if (c !== lastHover) {
    lastHover = c
    emit('hover', c)
  }
}

function onDown(e) {
  canvasEl.value.setPointerCapture(e.pointerId)
  const first = !pointers.size
  pointers.set(e.pointerId, place(e))
  regrip()
  if (first && fullMs > SLOW_MS) keepPicture()
}

function onUp(e) {
  if (!pointers.delete(e.pointerId)) return
  if (canvasEl.value?.hasPointerCapture?.(e.pointerId)) canvasEl.value.releasePointerCapture(e.pointerId)
  regrip()
  if (!pointers.size) schedule() // a full picture, where the map was left
}

function onLeave() {
  lastHover = null
  emit('hover', null)
}

function onWheel(e) {
  e.preventDefault()
  const rect = canvasEl.value.getBoundingClientRect()
  const mx = e.clientX - rect.left
  const my = e.clientY - rect.top
  const k = Math.exp(-e.deltaY * 0.0015)
  const next = Math.max(SCALE_MIN, Math.min(SCALE_MAX, view.scale * k))
  const f = next / view.scale
  view.ox = mx - (mx - view.ox) * f
  view.oy = my - (my - view.oy) * f
  view.scale = next
  // The wheel moves the map as fingers do, for as long as it keeps turning.
  if (!wheeling && fullMs > SLOW_MS) keepPicture()
  wheeling = true
  clearTimeout(wheelTimer)
  wheelTimer = setTimeout(() => {
    wheeling = false
    schedule()
  }, 160)
  schedule()
}
let wheeling = false
let wheelTimer = 0

onMounted(() => {
  prepare()
  resizeObserver = new ResizeObserver(resize)
  resizeObserver.observe(wrapEl.value)
  canvasEl.value.addEventListener('wheel', onWheel, { passive: false })
})

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  clearTimeout(settleTimer)
  clearTimeout(wheelTimer)
})

// What is drawn changed: the kept picture is of something else now.
function redraw() {
  groups = null
  fullAt = 0
  schedule()
}

watch(() => [props.frequency, props.projection], prepare)
watch(() => [props.version, props.cells, props.colorOf, props.markers, props.lines, props.areas, props.borders], redraw, { deep: false })

defineExpose({ fit: () => (fit(), (fullAt = 0), schedule()) })
</script>

<style scoped>
.flat-wrap {
  position: relative;
  width: 100%;
  height: 100%;
  overflow: hidden;
  cursor: grab;
}

.flat-wrap.is-grabbing {
  cursor: grabbing;
}

.flat-canvas {
  display: block;
  width: 100%;
  height: 100%;
  touch-action: none;
}
</style>
