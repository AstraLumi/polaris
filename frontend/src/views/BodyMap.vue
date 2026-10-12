<template>
  <div class="map-page">
    <div class="map-wrap">
      <PlanetGlobe
        v-if="ready"
        ref="globeRef"
        :frequency="frequency"
        :color-of="colorOf"
        :version="version"
        :tool="tool"
        :brush="brush"
        :lines="{ equator: false, zones: false, tilt: 0 }"
        :sky="skyUrl()"
        @paint-start="onPaintStart"
        @paint="onPaint"
        @paint-end="onPaintEnd"
      />
      <p v-else-if="loadError" class="error-banner floating">{{ loadError }}</p>
      <p v-else class="loading-hint floating">{{ $t('Loading…') }}</p>

      <div class="toolbar glass-panel panel-solid">
        <button
          v-for="t in TOOLS"
          :key="t.key"
          type="button"
          class="tool-btn"
          :class="{ active: tool === t.key }"
          :title="$t(t.title)"
          @click="tool = t.key"
        >{{ $t(t.label) }}</button>
        <template v-if="tool !== 'select'">
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
      </div>

      <div v-if="tool === 'paint'" class="palette glass-panel panel-solid">
        <button
          v-for="c in palette"
          :key="c"
          type="button"
          class="swatch"
          :class="{ active: color === c }"
          :style="{ background: c }"
          :aria-label="c"
          @click="color = c"
        ></button>
        <label class="custom" :title="$t('Pick any colour')">
          <input v-model="color" type="color" />
        </label>
      </div>
      <div class="switch-panel glass-panel panel-solid">
        <PlanetSwitch />
      </div>
      <p class="map-hint">{{ hint }}</p>
      <p v-if="notice" class="notice">{{ notice }}</p>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { t, tr } from '../i18n'
import { solarApi } from '../api'
import { skyUrl } from '../worldMap'
import PlanetGlobe from '../components/PlanetGlobe.vue'
import PlanetSwitch from '../components/PlanetSwitch.vue'

// A planet's or moon's own map, when it isn't the main planet: a globe to
// paint in colours, nothing more (no places, borders or wiki links). Its
// grid is the story's hex size round the body (backend/bodymaps.go).
const props = defineProps({ bodyId: { type: Number, required: true }, name: { type: String, default: '' } })

const TOOLS = [
  { key: 'select', label: tr('Look'), title: tr('Drag to turn the globe') },
  { key: 'paint', label: tr('Paint'), title: tr('Paint hexes in the picked colour') },
  { key: 'erase', label: tr('Erase'), title: tr('Clear the paint from hexes') },
]
const BRUSHES = [
  { radius: 0, hexes: 1, dot: 5 },
  { radius: 1, hexes: 7, dot: 9 },
  { radius: 2, hexes: 19, dot: 13 },
]
const BASE_COLORS = ['#c9b38a', '#7c9a5a', '#3f6e3a', '#d8d4c8', '#f2f2f2', '#8a5a3c', '#3f7fbf', '#b55a3c']

const ready = ref(false)
const loadError = ref('')
const frequency = ref(2)
const version = ref(0)
const tool = ref('paint')
const brush = ref(1)
const color = ref(BASE_COLORS[0])
const used = ref([])
const notice = ref('')
const globeRef = ref(null)
let cells = new Map() // cell -> '#rrggbb'

const colorOf = (cell) => cells.get(cell) || ''
// The colours already on the planet first, then a few to start with.
const palette = computed(() => [...new Set([...used.value, ...BASE_COLORS])].slice(0, 14))

const hint = computed(() => {
  const who = props.name || t('this planet')
  if (tool.value === 'paint') return t('Painting {name} · Hold Space to turn the globe', { name: who })
  if (tool.value === 'erase') return t('Erasing on {name} · Hold Space to turn the globe', { name: who })
  return t('Scroll to zoom · Drag to turn the globe')
})

async function load() {
  ready.value = false
  loadError.value = ''
  try {
    const m = await solarApi.bodyMap(props.bodyId)
    frequency.value = m.frequency
    cells = new Map(m.cells)
    used.value = m.colors
    if (m.colors.length) color.value = m.colors[0]
    version.value++
    ready.value = true
  } catch (err) {
    loadError.value = err.message || t("Couldn't load the map.")
  }
}
watch(() => props.bodyId, load, { immediate: true })

// ---- Painting -------------------------------------------------------------------------
// Each stroke is shown at once and saved when it ends, one after another.
let stroke = null
let saveChain = Promise.resolve()

function onPaintStart() {
  stroke = { erase: tool.value === 'erase', color: color.value, cells: new Set() }
}

function onPaint(list) {
  if (!stroke) return
  const touched = []
  for (const c of list) {
    const before = cells.get(c)
    if (stroke.erase ? before === undefined : before === stroke.color) continue
    if (stroke.erase) cells.delete(c)
    else cells.set(c, stroke.color)
    stroke.cells.add(c)
    touched.push(c)
  }
  if (touched.length) globeRef.value?.recolor(touched)
}

function onPaintEnd() {
  const s = stroke
  stroke = null
  if (!s || !s.cells.size) return
  const list = [...s.cells]
  if (!s.erase && !used.value.includes(s.color)) used.value = [s.color, ...used.value]
  saveChain = saveChain.then(async () => {
    try {
      if (s.erase) await solarApi.eraseBody(props.bodyId, list)
      else await solarApi.paintBody(props.bodyId, s.color, list)
    } catch (err) {
      notice.value = t("Couldn't save that stroke, so the map was reloaded.")
      setTimeout(() => (notice.value = ''), 4500)
      await load()
    }
  })
}
</script>

<style scoped src="../mapChrome.css"></style>
<style scoped>
.floating {
  position: absolute;
  top: 50%;
  left: 50%;
  translate: -50% -50%;
}

.tool-btn.active {
  color: var(--text-primary);
  border-color: var(--accent);
  background: var(--accent-soft);
}

.tool-sep {
  height: 1px;
  background: var(--glass-border);
  margin: 2px 0;
}

.brush-dot {
  display: inline-block;
  border-radius: 50%;
  background: currentColor;
}

.switch-panel {
  position: absolute;
  top: 16px;
  right: 16px;
  padding: 0.55rem;
  min-width: 12rem;
  z-index: 7;
}

.palette {
  position: absolute;
  left: 50%;
  bottom: 48px;
  translate: -50% 0;
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 6px;
  max-width: calc(100% - 32px);
  padding: 6px 8px;
  z-index: 7;
}

.swatch {
  width: 1.6rem;
  height: 1.6rem;
  border-radius: 50%;
  border: 2px solid transparent;
  cursor: pointer;
}

.swatch.active {
  border-color: var(--text-primary);
  box-shadow: 0 0 0 2px var(--accent);
}

.custom input {
  width: 1.8rem;
  height: 1.8rem;
  padding: 0;
  border: none;
  background: none;
  cursor: pointer;
}

.map-hint {
  position: absolute;
  left: 50%;
  bottom: 12px;
  translate: -50% 0;
  margin: 0;
  padding: 0.25rem 0.75rem;
  border-radius: 999px;
  background: rgba(5, 8, 18, 0.7);
  color: #c9d3e8;
  font-size: 0.74rem;
  white-space: nowrap;
  max-width: calc(100% - 32px);
  overflow: hidden;
  text-overflow: ellipsis;
  pointer-events: none;
}

.notice {
  position: absolute;
  top: 16px;
  left: 50%;
  translate: -50% 0;
  margin: 0;
  padding: 0.4rem 0.8rem;
  border-radius: 8px;
  background: var(--danger);
  color: #fff;
  font-size: 0.8rem;
  z-index: 8;
}
</style>
