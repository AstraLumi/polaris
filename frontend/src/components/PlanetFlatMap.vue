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

// The planet laid flat in a projection, to look at only: drag to move,
// scroll to zoom, hover a cell to see what is there. Painting happens on the
// globe.
const props = defineProps({
  frequency: { type: Number, required: true },
  projection: { type: String, default: 'equirectangular' },
  // Every cell with a colour (the rest is sea).
  cells: { type: Array, default: () => [] },
  colorOf: { type: Function, required: true },
  version: { type: Number, default: 0 },
  markers: { type: Array, default: () => [] },
  lines: { type: Object, default: null },
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
  view.scale = Math.min((size.w - 32) / (x1 - x0), (size.h - 32) / (y1 - y0))
  view.ox = size.w / 2 - ((x0 + x1) / 2) * view.scale
  view.oy = size.h / 2 + ((y0 + y1) / 2) * view.scale
}

function prepare() {
  grid = new PlanetGrid(props.frequency)
  projected = new ProjectedCells(grid, props.projection)
  fit()
  schedule()
}

function schedule() {
  if (queued) return
  queued = true
  requestAnimationFrame(() => {
    queued = false
    draw()
  })
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
    cells: byColor(props.cells, props.colorOf),
    lines: props.lines,
    land: (c, sx, sy) => {
      // Places: dots, capitals bigger, labelled once there is room.
      c.fillStyle = css('--accent', '#e6c06a')
      c.font = '600 11px Manrope, sans-serif'
      c.textAlign = 'center'
      c.textBaseline = 'top'
      c.lineJoin = 'round'
      const cellPx = view.scale * (Math.atan(2) / grid.n)
      for (const m of props.markers) {
        if (m.cell == null || m.cell >= grid.count) continue
        const [x, y] = projected.point(m.cell)
        const r = m.rank === 2 ? 4.5 : m.rank === 1 ? 3.5 : 2.5
        c.beginPath()
        c.arc(sx(x), sy(y), r, 0, Math.PI * 2)
        c.fill()
        if (m.rank === 2 || (m.rank === 1 && cellPx > 3) || cellPx > 12) {
          c.lineWidth = 3
          c.strokeStyle = css('--surface', '#10131c')
          c.strokeText(m.name, sx(x), sy(y) + r + 2)
          c.fillStyle = css('--text-primary', '#fff')
          c.fillText(m.name, sx(x), sy(y) + r + 2)
          c.fillStyle = css('--accent', '#e6c06a')
        }
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

let lastHover = null
function onMove(e) {
  if (drag.value) {
    view.ox = drag.value.ox + (e.clientX - drag.value.x)
    view.oy = drag.value.oy + (e.clientY - drag.value.y)
    schedule()
    return
  }
  const c = cellAt(e)
  if (c !== lastHover) {
    lastHover = c
    emit('hover', c)
  }
}

function onDown(e) {
  canvasEl.value.setPointerCapture(e.pointerId)
  drag.value = { x: e.clientX, y: e.clientY, ox: view.ox, oy: view.oy }
}

function onUp(e) {
  drag.value = null
  if (canvasEl.value?.hasPointerCapture?.(e.pointerId)) canvasEl.value.releasePointerCapture(e.pointerId)
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
  const next = Math.max(20, Math.min(20000, view.scale * k))
  const f = next / view.scale
  view.ox = mx - (mx - view.ox) * f
  view.oy = my - (my - view.oy) * f
  view.scale = next
  schedule()
}

onMounted(() => {
  prepare()
  resizeObserver = new ResizeObserver(resize)
  resizeObserver.observe(wrapEl.value)
  canvasEl.value.addEventListener('wheel', onWheel, { passive: false })
})

onBeforeUnmount(() => resizeObserver?.disconnect())

watch(() => [props.frequency, props.projection], prepare)
watch(() => [props.version, props.cells, props.markers, props.lines], schedule, { deep: false })

defineExpose({ fit: () => (fit(), schedule()) })
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
