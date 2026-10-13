<template>
  <div class="editor-2d">
    <svg
      ref="svgEl"
      class="orrery"
      :viewBox="viewBox"
      @pointerdown.self="onPanStart"
      @wheel.prevent="onWheel"
      @pointermove="onMove"
      @pointerup="onUp"
      @pointercancel="onUp"
      @click.self="onBackground"
    >
      <!-- The habitable zone: a band round the star. -->
      <g v-if="zone" class="zone">
        <path :d="zonePath" fill-rule="evenodd" />
        <text :x="zone.outer" :y="vu * 0.02" :font-size="vu * 0.026" text-anchor="end" :dx="-vu * 0.01">{{ $t('Habitable zone') }}</text>
      </g>
      <!-- How close and how far the picked body can orbit. -->
      <g v-if="limits" class="limits">
        <circle :r="limits.near" class="near" :stroke-width="vu * 0.005" :stroke-dasharray="`${vu * 0.02} ${vu * 0.012}`" />
        <text :y="-limits.near" :dy="-vu * 0.008" :font-size="vu * 0.024" text-anchor="middle" class="near">{{ $t('Too close') }}</text>
        <template v-if="limits.far < extent * 1.5">
          <circle :r="limits.far" class="far" :stroke-width="vu * 0.005" :stroke-dasharray="`${vu * 0.02} ${vu * 0.012}`" />
          <text :y="limits.far" :dy="vu * 0.03" :font-size="vu * 0.024" text-anchor="middle" class="far">{{ $t('Too far') }}</text>
        </template>
      </g>
      <!-- Orbits, with a dot where each comes closest -->
      <template v-for="{ c, path, peri } in drawn" :key="'o' + c.id">
        <path :d="path" class="ring" :class="{ 'is-selected': c.id === selected }" :stroke-width="vu * 0.004" />
        <circle v-if="peri" :cx="peri[0]" :cy="peri[1]" :r="vu * 0.006" class="peri" :class="{ 'is-selected': c.id === selected }" />
      </template>
      <!-- What everything here orbits -->
      <g v-if="focus" class="body is-focus" @click="$emit('select', focus.id)">
        <circle :r="size(focus)" :fill="colorOf(focus)" />
        <circle v-if="focus.kind === 'star'" :r="size(focus) * 1.35" :fill="focus.star_class === 'black_hole' ? '#ff8c3a' : colorOf(focus)" opacity="0.18" />
      </g>
      <!-- The bodies on their orbits, at their starting angle -->
      <g
        v-for="{ c, at, moons } in drawn"
        :key="c.id"
        class="body"
        :class="{ 'is-selected': c.id === selected, 'is-dragging': drag?.id === c.id }"
        :transform="`translate(${at[0]} ${at[1]})`"
        @pointerdown.stop="onDown($event, c)"
        @dblclick.stop="moons && $emit('focus', c.id)"
      >
        <circle :r="hitSize(c)" fill="transparent" />
        <circle :r="size(c)" :fill="colorOf(c)" class="disc" :stroke-width="vu * 0.004" />
        <text :y="-hitSize(c) - vu * 0.012" :font-size="vu * 0.032" text-anchor="middle">
          {{ c.name }}<tspan v-if="moons" class="moons"> · {{ moons }}☾</tspan>
        </text>
      </g>
    </svg>
    <div class="zoom-tools glass-panel panel-solid">
      <button type="button" class="zoom-btn" :title="$t('Zoom in')" @click="zoomBy(1.4)">+</button>
      <button type="button" class="zoom-btn" :title="$t('Zoom out')" @click="zoomBy(1 / 1.4)">−</button>
      <button type="button" class="zoom-btn fit" :title="$t('Fit everything in view')" @click="resetView">{{ $t('Fit') }}</button>
    </div>
    <p v-if="drag" class="drag-note">{{ dragNote }}</p>
    <p v-else-if="limits && limits.far >= extent * 1.5" class="hint far-note">{{ $t('Too far: {far}, beyond this view', { far: limits.farText }) }}</p>
    <p v-else class="hint">{{ $t('Drag a body to move it: out or in changes its distance, round changes where it starts.') }}</p>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { t } from '../i18n'
import { systemTree, orbitLimits, keplerOrbit, orbitPoint, orbitThrough } from '../orbits'
import { sceneRadius, sceneDistance, orbitForSceneDistance, bodyColor, scenePoint, sceneOrbitPath } from '../solarScale'
import { habitableZone } from '../stars'

// The solar system's editor, seen from above and flat: what `focusId` is
// (the star, or a planet to see its moons) in the middle, and what orbits
// it on circles at the drawn scale of solarScale.js, each at its starting
// angle. Dragging a body sets its distance and its angle; while dragging it
// emits `move` ({ id, orbit_km, phase }), and `commit` (id) when let go.
// Scroll (or the buttons) zooms, dragging the background moves the view.
const props = defineProps({
  bodies: { type: Array, default: () => [] },
  focusId: { type: Number, default: null },
  selected: { type: Number, default: null },
  showZone: { type: Boolean, default: true },
  showLimits: { type: Boolean, default: true },
})
const emit = defineEmits(['select', 'focus', 'move', 'commit'])

const svgEl = ref(null)
const AU = 1.496e8
const colorOf = bodyColor

const tree = computed(() => systemTree(props.bodies))
const childrenOf = (id) => tree.value.childrenOf(id)
const focus = computed(() => tree.value.byId.get(props.focusId) ?? tree.value.star)
const children = computed(() => (focus.value ? childrenOf(focus.value.id) : []))

// Where a body is on day 0, and its orbit, drawn (y down on screen, so
// angles turn anticlockwise as seen from above the north pole).
// Worked out when the bodies change, not on every redraw (zooming, panning).
// An orbit's line is kept while its shape is the same: dragging a body
// round its orbit only moves the body.
const flip = ([x, y]) => [x, -y]
const paths = new Map() // body id -> { key, d }
function orbitD(c) {
  const f = focus.value
  const key = [f.id, f.radius_km, c.radius_km, c.orbit_km, c.eccentricity, c.periapsis].join(':')
  const kept = paths.get(c.id)
  if (kept?.key === key) return kept.d
  const d = 'M ' + sceneOrbitPath(c, f).map((p) => flip(p).join(' ')).join(' L ') + ' Z'
  paths.set(c.id, { key, d })
  return d
}
const drawn = computed(() => {
  const f = focus.value
  return children.value.map((c) => ({
    c,
    at: flip(scenePoint(keplerOrbit.position(c, f, 0), f, c)),
    path: orbitD(c),
    peri: c.eccentricity > 0 ? flip(scenePoint(orbitPoint(c, 0), f, c)) : null,
    moons: childrenOf(c.id).length,
  }))
})
const reach = (c) => sceneDistance(c.orbit_km * (1 + (c.eccentricity || 0)), focus.value, c)

// The markers, at the drawn scale. The habitable zone shows round the star;
// the gravity limits round what the picked body orbits, when that is in
// the middle.
const zone = computed(() => {
  const f = focus.value
  if (!props.showZone || f?.kind !== 'star') return null
  const hz = habitableZone(f)
  return hz && { inner: sceneDistance(hz.inner_km, f), outer: sceneDistance(hz.outer_km, f) }
})
const zonePath = computed(() => {
  const ring = (r) => `M ${r} 0 A ${r} ${r} 0 1 0 ${-r} 0 A ${r} ${r} 0 1 0 ${r} 0 Z`
  return zone.value ? ring(zone.value.outer) + ' ' + ring(zone.value.inner) : ''
})
const limits = computed(() => {
  const b = tree.value.byId.get(props.selected)
  const f = focus.value
  if (!props.showLimits || !b || !f || b.parent_id !== f.id) return null
  const grand = f.parent_id != null ? tree.value.byId.get(f.parent_id) : null
  const { near_km, far_km } = orbitLimits(b, f, grand)
  const n = (x) => Number(x.toPrecision(3)).toLocaleString()
  return {
    near: sceneDistance(near_km, f, b),
    far: sceneDistance(far_km, f, b),
    farText: far_km > AU * 0.1 ? t('{n} AU', { n: n(far_km / AU) }) : t('{n} km', { n: n(far_km) }),
  }
})
// The view fits the farthest orbit (kept while dragging, so the view
// doesn't run away from the pointer).
const fitted = computed(() => {
  let far = focus.value ? sceneRadius(focus.value) * 1.6 : 1
  for (const c of children.value) far = Math.max(far, reach(c) + sceneRadius(c))
  // The zone shows whole, and a near limit; a far one only when close by.
  if (zone.value) far = Math.max(far, zone.value.outer)
  if (limits.value) {
    far = Math.max(far, limits.value.near)
    if (limits.value.far < far * 1.3) far = Math.max(far, limits.value.far)
  }
  return far * 1.15
})
const frozen = ref(1)
const extent = computed(() => (drag.value ? frozen.value : fitted.value))

// ---- Zoom and pan ----------------------------------------------------------------------
// zoom 1 fits everything; center is the middle of the view, in scene units.
const zoom = ref(1)
const center = ref([0, 0])
const vu = computed(() => extent.value / zoom.value) // half the visible width
const viewBox = computed(() => {
  const h = vu.value
  return `${center.value[0] - h} ${center.value[1] - h} ${h * 2} ${h * 2}`
})
const ZOOM_MIN = 0.5
const ZOOM_MAX = 400

function resetView() {
  zoom.value = 1
  center.value = [0, 0]
}
watch(() => props.focusId, resetView)

// Zooms by `k` keeping `at` (scene units) where it is on screen.
function zoomBy(k, at = center.value) {
  const next = Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, zoom.value * k))
  const f = zoom.value / next
  center.value = [at[0] + (center.value[0] - at[0]) * f, at[1] + (center.value[1] - at[1]) * f]
  zoom.value = next
}

function onWheel(e) {
  const p = point(e)
  zoomBy(Math.exp(-e.deltaY * 0.0015), [p.x, p.y])
}

let pan = null
function onPanStart(e) {
  pan = { x: e.clientX, y: e.clientY, center: [...center.value], scale: svgEl.value.getScreenCTM().a, moved: false }
  try {
    svgEl.value.setPointerCapture(e.pointerId)
  } catch {
    // Not a live pointer.
  }
}

// Small bodies still get a size you can see and grab.
const size = (b) => Math.max(sceneRadius(b), vu.value * 0.012)
const hitSize = (b) => Math.max(size(b), vu.value * 0.03)

// ---- Dragging --------------------------------------------------------------------------
const drag = ref(null) // { id, moved }
let justDragged = false
function point(e) {
  const p = svgEl.value.createSVGPoint()
  p.x = e.clientX
  p.y = e.clientY
  return p.matrixTransform(svgEl.value.getScreenCTM().inverse())
}

function onDown(e, c) {
  emit('select', c.id)
  frozen.value = fitted.value
  drag.value = { id: c.id, moved: false }
  try {
    svgEl.value.setPointerCapture(e.pointerId)
  } catch {
    // Not a live pointer; the drag still works while over the view.
  }
}

function onMove(e) {
  if (pan) {
    const dx = e.clientX - pan.x
    const dy = e.clientY - pan.y
    if (Math.hypot(dx, dy) > 3) pan.moved = true
    center.value = [pan.center[0] - dx / pan.scale, pan.center[1] - dy / pan.scale]
    return
  }
  if (!drag.value) return
  const c = tree.value.byId.get(drag.value.id)
  if (!c) return
  const p = point(e)
  const d = Math.hypot(p.x, p.y)
  const phase = Math.round(((Math.atan2(-p.y, p.x) * 180) / Math.PI + 360) % 360)
  // The orbit, of the same shape, that passes where the pointer is; its
  // size rounded to three significant figures, for tidy numbers.
  const through = orbitThrough(c, orbitForSceneDistance(d, focus.value, c), phase)
  drag.value.moved = true
  emit('move', { id: c.id, orbit_km: Number(through.orbit_km.toPrecision(3)), phase })
}

function onUp(e) {
  if (pan) {
    // A drag of the background isn't a click on it.
    if (pan.moved) {
      justDragged = true
      setTimeout(() => (justDragged = false), 0)
    }
    pan = null
    if (svgEl.value.hasPointerCapture?.(e.pointerId)) svgEl.value.releasePointerCapture(e.pointerId)
    return
  }
  if (!drag.value) return
  const { id, moved } = drag.value
  drag.value = null
  if (svgEl.value.hasPointerCapture?.(e.pointerId)) svgEl.value.releasePointerCapture(e.pointerId)
  if (moved) emit('commit', id)
  // The click that ends a press on a body (the view holds the pointer, so
  // that click lands on the background) mustn't pick the background.
  justDragged = true
  setTimeout(() => (justDragged = false), 0)
}

function onBackground() {
  if (!justDragged) emit('select', focus.value?.id ?? null)
}

const dragNote = computed(() => {
  const c = tree.value.byId.get(drag.value?.id)
  if (!c) return ''
  const n = (x) => Number(x.toPrecision(3)).toLocaleString()
  const far = c.kind === 'moon' ? t('{n} km', { n: n(c.orbit_km) }) : t('{n} AU', { n: n(c.orbit_km / AU) })
  return `${c.name} · ${far} · ${Math.round(c.phase)}°`
})
</script>

<style scoped>
.editor-2d {
  position: absolute;
  inset: 0;
  background: radial-gradient(circle at 50% 50%, #0c1224, #02030a 70%);
}

.orrery {
  display: block;
  width: 100%;
  height: 100%;
  touch-action: none;
  user-select: none;
}

.zone path {
  fill: #4caf6e;
  opacity: 0.16;
}

.zone text {
  fill: #7fd39a;
  pointer-events: none;
}

/* The circles are outlines; their labels are filled text only (an
   outline on text this small, in view units, drew it as a huge blot). */
.limits circle {
  fill: none;
  opacity: 0.85;
}

.limits circle.near {
  stroke: #e05a5a;
}

.limits circle.far {
  stroke: #e0a04a;
}

.limits text {
  stroke: none;
  pointer-events: none;
}

.limits text.near {
  fill: #ff8a8a;
}

.limits text.far {
  fill: #f0b866;
}

.far-note {
  color: #e0a04a;
}

.ring {
  fill: none;
  stroke: #8aa0c8;
  opacity: 0.35;
}

.peri {
  fill: #8aa0c8;
  opacity: 0.6;
}

.peri.is-selected {
  fill: var(--accent);
  opacity: 1;
}

.ring.is-selected {
  stroke: var(--accent);
  opacity: 0.9;
}

.body {
  cursor: grab;
}

.body.is-dragging {
  cursor: grabbing;
}

.body.is-focus {
  cursor: pointer;
}

.disc {
  stroke: transparent;
}

.body.is-selected .disc {
  stroke: var(--accent);
}

.body text {
  fill: #dfe6f5;
  paint-order: stroke;
  stroke: rgba(2, 3, 10, 0.8);
  stroke-width: 0.35em;
  pointer-events: none;
}

.body.is-selected text {
  fill: var(--accent);
}

.moons {
  fill: #9fb0d0;
}

.zoom-tools {
  position: absolute;
  right: 12px;
  bottom: 12px;
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 4px;
  z-index: 2;
}

.zoom-btn {
  min-width: 2rem;
  height: 2rem;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--text-muted);
  font: inherit;
  font-size: 1rem;
  cursor: pointer;
}

.zoom-btn.fit {
  font-size: 0.7rem;
  font-weight: 600;
}

.zoom-btn:hover {
  color: var(--text-primary);
  background: var(--accent-soft);
}

.hint,
.drag-note {
  position: absolute;
  left: 50%;
  bottom: 12px;
  translate: -50% 0;
  margin: 0;
  max-width: calc(100% - 32px);
  padding: 0.3rem 0.75rem;
  border-radius: 999px;
  background: rgba(5, 8, 18, 0.7);
  color: #c9d3e8;
  font-size: 0.76rem;
  text-align: center;
  pointer-events: none;
}

.drag-note {
  color: var(--accent);
  font-variant-numeric: tabular-nums;
}
</style>
