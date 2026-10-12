<template>
  <div class="editor-2d">
    <svg
      ref="svgEl"
      class="orrery"
      :viewBox="`${-extent} ${-extent} ${extent * 2} ${extent * 2}`"
      @pointermove="onMove"
      @pointerup="onUp"
      @pointercancel="onUp"
      @click.self="onBackground"
    >
      <!-- Orbits -->
      <circle
        v-for="c in children"
        :key="'o' + c.id"
        :r="dist(c)"
        class="ring"
        :class="{ 'is-selected': c.id === selected }"
        :stroke-width="extent * 0.004"
      />
      <!-- What everything here orbits -->
      <g v-if="focus" class="body is-focus" @click="$emit('select', focus.id)">
        <circle :r="size(focus)" :fill="colorOf(focus)" />
        <circle v-if="focus.kind === 'star'" :r="size(focus) * 1.35" :fill="colorOf(focus)" opacity="0.18" />
      </g>
      <!-- The bodies on their orbits, at their starting angle -->
      <g
        v-for="c in children"
        :key="c.id"
        class="body"
        :class="{ 'is-selected': c.id === selected, 'is-dragging': drag?.id === c.id }"
        :transform="`translate(${at(c)[0]} ${at(c)[1]})`"
        @pointerdown.stop="onDown($event, c)"
        @dblclick.stop="childrenOf(c.id).length && $emit('focus', c.id)"
      >
        <circle :r="hitSize(c)" fill="transparent" />
        <circle :r="size(c)" :fill="colorOf(c)" class="disc" :stroke-width="extent * 0.004" />
        <text :y="-hitSize(c) - extent * 0.012" :font-size="extent * 0.032" text-anchor="middle">
          {{ c.name }}<tspan v-if="childrenOf(c.id).length" class="moons"> · {{ childrenOf(c.id).length }}☾</tspan>
        </text>
      </g>
    </svg>
    <p v-if="drag" class="drag-note">{{ dragNote }}</p>
    <p v-else class="hint">{{ $t('Drag a body to move it: out or in changes its distance, round changes where it starts.') }}</p>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { t } from '../i18n'
import { systemTree } from '../orbits'
import { sceneRadius, sceneDistance, orbitForSceneDistance } from '../solarScale'

// The solar system's editor, seen from above and flat: what `focusId` is
// (the star, or a planet to see its moons) in the middle, and what orbits
// it on circles at the drawn scale of solarScale.js, each at its starting
// angle. Dragging a body sets its distance and its angle; while dragging it
// emits `move` ({ id, orbit_km, phase }), and `commit` (id) when let go.
const props = defineProps({
  bodies: { type: Array, default: () => [] },
  focusId: { type: Number, default: null },
  selected: { type: Number, default: null },
})
const emit = defineEmits(['select', 'focus', 'move', 'commit'])

const svgEl = ref(null)
const TYPE_COLORS = { star: '#ffd27a', rocky: '#9c8a74', gas: '#d9b38c', ocean: '#3f7fbf' }
const colorOf = (b) => b.color || TYPE_COLORS[b.body_type] || TYPE_COLORS.rocky

const tree = computed(() => systemTree(props.bodies))
const childrenOf = (id) => tree.value.childrenOf(id)
const focus = computed(() => tree.value.byId.get(props.focusId) ?? tree.value.star)
const children = computed(() => (focus.value ? childrenOf(focus.value.id) : []))

const dist = (c) => sceneDistance(c.orbit_km, focus.value, c)
// The view fits the farthest orbit (kept while dragging, so the view
// doesn't run away from the pointer).
const fitted = computed(() => {
  let far = focus.value ? sceneRadius(focus.value) * 1.6 : 1
  for (const c of children.value) far = Math.max(far, dist(c) + sceneRadius(c))
  return far * 1.15
})
const frozen = ref(1)
const extent = computed(() => (drag.value ? frozen.value : fitted.value))
// Small bodies still get a size you can see and grab.
const size = (b) => Math.max(sceneRadius(b), extent.value * 0.012)
const hitSize = (b) => Math.max(size(b), extent.value * 0.03)
const at = (c) => {
  const a = ((c.phase || 0) * Math.PI) / 180
  // Angles turn anticlockwise, as seen from above the north pole.
  return [Math.cos(a) * dist(c), -Math.sin(a) * dist(c)]
}

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
  if (!drag.value) return
  const c = tree.value.byId.get(drag.value.id)
  if (!c) return
  const p = point(e)
  const d = Math.hypot(p.x, p.y)
  const phase = Math.round(((Math.atan2(-p.y, p.x) * 180) / Math.PI + 360) % 360)
  // Rounded to three significant figures: tidy numbers in the form.
  const km = Number(orbitForSceneDistance(d, focus.value, c).toPrecision(3))
  drag.value.moved = true
  emit('move', { id: c.id, orbit_km: km, phase })
}

function onUp(e) {
  if (!drag.value) return
  const { id, moved } = drag.value
  drag.value = null
  if (svgEl.value.hasPointerCapture?.(e.pointerId)) svgEl.value.releasePointerCapture(e.pointerId)
  if (moved) {
    emit('commit', id)
    // The click that ends a drag isn't a click on the background.
    justDragged = true
    setTimeout(() => (justDragged = false), 0)
  }
}

function onBackground() {
  if (!justDragged) emit('select', focus.value?.id ?? null)
}

const AU = 1.496e8
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

.ring {
  fill: none;
  stroke: #8aa0c8;
  opacity: 0.35;
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
