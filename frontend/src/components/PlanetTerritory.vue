<template>
  <div v-if="editable" class="territory-edit">
    <div class="edit-tools">
      <button type="button" class="btn btn-ghost small" :class="{ 'is-on': editTool === 'paint' }" @click="editTool = 'paint'">
        {{ $t('Paint') }}
      </button>
      <button type="button" class="btn btn-ghost small" :class="{ 'is-on': editTool === 'select' }" @click="editTool = 'select'">
        {{ $t('Turn') }}
      </button>
      <span class="hint">{{ $t('Drag over hexes to add or remove them; hold Space or pick Turn to turn the globe.') }}</span>
    </div>
    <div class="globe-box" :style="{ height: maxHeight + 'px' }">
      <PlanetGlobe
        ref="globeRef"
        :frequency="frequency"
        :color-of="colorOf"
        :version="version"
        :tool="editTool"
        :focus="cells[0] ?? landCells[0] ?? null"
        @paint-start="onStart"
        @paint="onPaint"
        @paint-end="onEnd"
      />
    </div>
  </div>
  <img
    v-else-if="imageUrl"
    class="territory"
    :src="imageUrl"
    :alt="alt"
    :style="{ width: width + 'px' }"
    @click="$emit('open', imageUrl)"
  />
</template>

<script setup>
import { ref, computed, watch, onMounted, defineAsyncComponent } from 'vue'
import { PlanetGrid } from '../planetGrid'
import { ProjectedCells, drawPlanet, mapBounds } from '../planetImage'
import { worldMap } from '../worldMap'

// A creature's territory on the planet (PlanetMap.vue): as a picture, drawn
// flat in the projection Settings picks for the bestiary; or, editable, on
// a globe where hexes are painted on and off. The planet's version of
// HexTerritory.vue.
const PlanetGlobe = defineAsyncComponent(() => import('./PlanetGlobe.vue'))

const props = defineProps({
  cells: { type: Array, default: () => [] }, // the territory
  landCells: { type: Array, default: () => [] }, // every cell something on the map covers
  kind: { type: String, default: 'monster' },
  editable: { type: Boolean, default: false },
  frequency: { type: Number, required: true },
  width: { type: Number, default: 280 },
  maxHeight: { type: Number, default: 360 },
  alt: { type: String, default: '' },
})
const emit = defineEmits(['update:cells', 'open'])

const imageUrl = ref('')
const editTool = ref('paint')
const globeRef = ref(null)
const version = ref(0)

const owned = computed(() => new Set(props.cells))
const land = computed(() => new Set(props.landCells))

function colors() {
  const css = getComputedStyle(document.documentElement)
  const v = (name, fallback) => css.getPropertyValue(name).trim() || fallback
  const tag = { monster: v('--danger', '#d0605a'), animal: v('--kind-founding', '#7fbf8f'), construct: v('--kind-event', '#7fa6e0') }
  return { owned: tag[props.kind] || tag.monster, land: '#6b7080', sea: '#0b0c11' }
}

let palette = colors()
// Mid-stroke the territory is the stroke's own set: the prop only catches up
// on the next tick.
let working = null
const colorOf = (c) => ((working ? working.has(c) : owned.value.has(c)) ? palette.owned : land.value.has(c) ? palette.land : '')

// ---- The picture ----------------------------------------------------------------------
function render() {
  if (props.editable) return
  if (!props.cells.length) {
    imageUrl.value = ''
    return
  }
  palette = colors()
  const grid = new PlanetGrid(props.frequency)
  const projected = new ProjectedCells(grid, worldMap.planet.bestiary_projection)
  // Framed on the territory, with some land round it; the whole map when the
  // territory wraps round the edge of it.
  const pts = props.cells.map((c) => projected.point(c))
  const xs = pts.map((p) => p[0])
  const ys = pts.map((p) => p[1])
  const pad = (Math.atan(2) / grid.n) * 4
  let [x0, y0, x1, y1] = [Math.min(...xs) - pad, Math.min(...ys) - pad, Math.max(...xs) + pad, Math.max(...ys) + pad]
  if (x1 - x0 > Math.PI) [x0, y0, x1, y1] = mapBounds(projected.proj)
  // Never closer in than a few hexes.
  const least = pad * 3
  if (x1 - x0 < least) [x0, x1] = [(x0 + x1) / 2 - least / 2, (x0 + x1) / 2 + least / 2]
  if (y1 - y0 < least) [y0, y1] = [(y0 + y1) / 2 - least / 2, (y0 + y1) / 2 + least / 2]
  const scale = Math.min(props.width / (x1 - x0), props.maxHeight / (y1 - y0))
  const w = Math.round((x1 - x0) * scale)
  const h = Math.round((y1 - y0) * scale)
  const dpr = 2
  const canvas = document.createElement('canvas')
  canvas.width = w * dpr
  canvas.height = h * dpr
  const ctx = canvas.getContext('2d')
  ctx.scale(dpr, dpr)
  ctx.fillStyle = palette.sea
  ctx.fillRect(0, 0, w, h)
  const cells = new Map([
    [palette.land, props.landCells.filter((c) => !owned.value.has(c))],
    [palette.owned, props.cells],
  ])
  drawPlanet(ctx, projected, { scale, ox: -x0 * scale, oy: y1 * scale }, { cells, sea: palette.sea, border: 'rgba(0, 0, 0, 0.55)' })
  imageUrl.value = canvas.toDataURL('image/png')
}

// ---- Painting -----------------------------------------------------------------------------
// The first hex a stroke touches decides whether it adds or removes.
let adding = null

function onStart() {
  adding = null
  working = new Set(props.cells)
}

function onPaint(cells) {
  if (!working) return
  if (adding === null) adding = !working.has(cells[0])
  const changed = []
  for (const c of cells) {
    if (adding && !working.has(c)) {
      working.add(c)
      changed.push(c)
    } else if (!adding && working.has(c)) {
      working.delete(c)
      changed.push(c)
    }
  }
  if (!changed.length) return
  emit('update:cells', [...working])
  globeRef.value?.recolor(changed)
}

function onEnd() {
  working = null
}

onMounted(render)
watch(() => [props.cells, props.landCells, props.kind, props.frequency, worldMap.planet.bestiary_projection], () => {
  render()
  // Mid-stroke the globe is recoloured as it goes.
  if (props.editable && !working) {
    palette = colors()
    version.value++
  }
})
</script>

<style scoped>
.territory {
  display: block;
  max-width: 100%;
  border-radius: 8px;
  cursor: zoom-in;
}

.territory-edit {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
}

.edit-tools {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.4rem;
}

.edit-tools .is-on {
  border-color: var(--accent);
  color: var(--accent);
}

.hint {
  font-size: 0.76rem;
  color: var(--text-faint);
}

.globe-box {
  border: 1px solid var(--glass-border);
  border-radius: 10px;
  overflow: hidden;
  background: #05070c;
}
</style>
