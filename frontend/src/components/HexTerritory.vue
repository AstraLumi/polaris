<template>
  <canvas
    v-if="editable"
    ref="canvasEl"
    class="territory is-editable"
    :style="{ width: cssW + 'px', height: cssH + 'px' }"
    :aria-label="$t('Territory: click or drag over hexes to add or remove them')"
    @pointerdown="onDown"
    @pointermove="onMove"
    @pointerup="onUp"
    @pointerleave="onUp"
  ></canvas>
  <img
    v-else-if="imageUrl"
    class="territory"
    :src="imageUrl"
    :alt="alt"
    :style="{ width: cssW + 'px' }"
    @click="$emit('open', imageUrl)"
  />
</template>

<script setup>
import { ref, computed, watch, onMounted, nextTick } from 'vue'
import { hexCenter, pixelToHex, CORNERS, SQRT3, keyOf } from '../hexMath'

// A creature's territory drawn on the map's hexes, as a still picture: its
// hexes in the colour of its tag, the rest of the map grey, empty hexes
// black. Editable, it's a canvas that hexes are painted on and off with a
// click or a drag.
const props = defineProps({
  territory: { type: Array, default: () => [] }, // [[q, r]]
  mapHexes: { type: Array, default: () => [] }, // every hex something on the map covers, [[q, r]]
  kind: { type: String, default: 'monster' },
  editable: { type: Boolean, default: false },
  width: { type: Number, default: 280 },
  maxHeight: { type: Number, default: 360 },
  alt: { type: String, default: '' },
})
const emit = defineEmits(['update:territory', 'open'])

const canvasEl = ref(null)
const imageUrl = ref('')
const cssW = ref(props.width)
const cssH = ref(0)

const SIZE = 10 // world units per hex (center to corner)
const MARGIN = 2 // hexes of context around what's shown

const owned = computed(() => new Set(props.territory.map(([q, r]) => keyOf(q, r))))
const mapped = computed(() => new Set(props.mapHexes.map(([q, r]) => keyOf(q, r))))

// What the picture covers: the territory (editing: the whole map too), with
// a margin. In world units. While editing it stays put as hexes are painted
// (or every hex would shift under the pointer), until the map changes.
let editExtent = null
function extent() {
  if (props.editable && editExtent) return editExtent
  const focus = props.editable ? [...props.mapHexes, ...props.territory] : props.territory
  const pts = (focus.length ? focus : [[0, 0]]).map(([q, r]) => hexCenter(q, r, SIZE))
  const pad = SIZE * SQRT3 * (MARGIN + (props.editable ? 2 : 0) + (focus.length ? 0.5 : 4))
  const xs = pts.map((p) => p[0])
  const ys = pts.map((p) => p[1])
  const b = { x0: Math.min(...xs) - pad, x1: Math.max(...xs) + pad, y0: Math.min(...ys) - pad, y1: Math.max(...ys) + pad }
  if (props.editable && props.mapHexes.length) editExtent = b
  return b
}

// Every hex whose centre falls inside the extent.
function hexesIn(b) {
  const out = []
  const a = pixelToHex(b.x0, b.y0, SIZE)
  const z = pixelToHex(b.x1, b.y1, SIZE)
  const spread = z.r - a.r + 2
  for (let r = a.r - 1; r <= z.r + 1; r++) {
    for (let q = a.q - spread; q <= z.q + spread; q++) {
      const [x, y] = hexCenter(q, r, SIZE)
      if (x >= b.x0 && x <= b.x1 && y >= b.y0 && y <= b.y1) out.push([q, r])
    }
  }
  return out
}

function colors() {
  const css = getComputedStyle(document.documentElement)
  const v = (name, fallback) => css.getPropertyValue(name).trim() || fallback
  const tag = { monster: v('--danger', '#d0605a'), animal: v('--kind-founding', '#7fbf8f'), construct: v('--kind-event', '#7fa6e0') }
  return { owned: tag[props.kind] || tag.monster, mapped: '#6b7080', empty: '#0b0c11', line: 'rgba(0, 0, 0, 0.55)' }
}

let view = null // { b, scale } of the last drawing, to find the hex under the pointer

function draw(canvas) {
  const b = extent()
  const w = b.x1 - b.x0
  const h = b.y1 - b.y0
  // As wide as asked, but no wider than the space it sits in.
  const room = props.editable ? canvas.parentElement?.clientWidth || props.width : props.width
  const scale = Math.min(Math.min(props.width, room) / w, props.maxHeight / h)
  cssW.value = Math.round(w * scale)
  cssH.value = Math.round(h * scale)
  const dpr = props.editable ? window.devicePixelRatio || 1 : 2
  canvas.width = cssW.value * dpr
  canvas.height = cssH.value * dpr
  const ctx = canvas.getContext('2d')
  ctx.setTransform(scale * dpr, 0, 0, scale * dpr, -b.x0 * scale * dpr, -b.y0 * scale * dpr)
  const c = colors()
  ctx.fillStyle = c.empty
  ctx.fillRect(b.x0, b.y0, w, h)
  for (const [q, r] of hexesIn(b)) {
    const k = keyOf(q, r)
    const [cx, cy] = hexCenter(q, r, SIZE)
    ctx.beginPath()
    CORNERS.forEach(([dx, dy], i) => (i ? ctx.lineTo : ctx.moveTo).call(ctx, cx + dx * SIZE, cy + dy * SIZE))
    ctx.closePath()
    ctx.fillStyle = owned.value.has(k) ? c.owned : mapped.value.has(k) ? c.mapped : c.empty
    ctx.fill()
    ctx.lineWidth = 1.2
    ctx.strokeStyle = owned.value.has(k) || mapped.value.has(k) ? c.line : 'rgba(255, 255, 255, 0.06)'
    ctx.stroke()
  }
  view = { b, scale }
}

async function render() {
  if (props.editable) {
    await nextTick()
    if (canvasEl.value) draw(canvasEl.value)
    return
  }
  if (!props.territory.length) {
    imageUrl.value = ''
    return
  }
  const canvas = document.createElement('canvas')
  draw(canvas)
  imageUrl.value = canvas.toDataURL('image/png')
}

// ---- painting ----
let painting = null // true: adding, false: removing
let lastKey = ''
// The territory as painted so far in this stroke: the prop only catches up
// on the next tick, and a drag paints many hexes before that.
let working = null

function hexAt(e) {
  const rect = canvasEl.value.getBoundingClientRect()
  // By the size it's shown at, whatever CSS made of it.
  const x = view.b.x0 + ((e.clientX - rect.left) / rect.width) * (view.b.x1 - view.b.x0)
  const y = view.b.y0 + ((e.clientY - rect.top) / rect.height) * (view.b.y1 - view.b.y0)
  return pixelToHex(x, y, SIZE)
}

function paint(h) {
  const k = keyOf(h.q, h.r)
  if (k === lastKey) return
  lastKey = k
  const has = working.some(([q, r]) => keyOf(q, r) === k)
  if (painting && !has) working = [...working, [h.q, h.r]]
  else if (!painting && has) working = working.filter(([q, r]) => keyOf(q, r) !== k)
  else return
  emit('update:territory', working)
}

function onDown(e) {
  if (!view) return
  const h = hexAt(e)
  painting = !owned.value.has(keyOf(h.q, h.r))
  working = [...props.territory]
  lastKey = ''
  canvasEl.value.setPointerCapture?.(e.pointerId)
  paint(h)
}

function onMove(e) {
  if (painting !== null && view) paint(hexAt(e))
}

function onUp() {
  painting = null
  working = null
}

watch(
  () => props.mapHexes,
  () => (editExtent = null),
)
watch(() => [props.territory, props.mapHexes, props.kind], render, { deep: true })
onMounted(render)
</script>

<style scoped>
.territory {
  display: block;
  max-width: 100%;
  border-radius: 8px;
  border: 1px solid var(--glass-border);
}

img.territory {
  height: auto;
  cursor: zoom-in;
}

canvas.is-editable {
  cursor: crosshair;
  touch-action: none;
}
</style>
