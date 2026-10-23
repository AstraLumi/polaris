<template>
  <div class="page">
    <Breadcrumbs :crumbs="[{ label: $t('Characters'), to: '/characters' }, { label: $t('Height chart') }]" />
    <header class="page-header">
      <div>
        <h1>{{ $t('Height chart') }}</h1>
        <p class="page-sub">{{ $t('Characters side by side, to scale, drawn with their body type\'s outline.') }}</p>
      </div>
      <div class="actions">
        <div class="unit-switch" role="radiogroup" :aria-label="$t('Units')">
          <button
            v-for="u in UNITS"
            :key="u.key"
            type="button"
            role="radio"
            :class="{ 'is-on': unit === u.key }"
            :aria-checked="unit === u.key"
            @click="setUnit(u.key)"
          >
            {{ $t(u.label) }}
          </button>
        </div>
      </div>
    </header>

    <p v-if="loadError" class="error-banner">{{ loadError }}</p>

    <div v-if="!loading && characters.length === 0" class="empty-state glass-panel">
      <p class="empty-title">{{ $t('No characters yet.') }}</p>
      <p class="empty-hint">{{ $t('Add your first one to start building their story and stats.') }}</p>
      <RouterLink :to="{ path: '/characters', query: { new: '1' } }" class="btn btn-primary small">{{ $t('Add character') }}</RouterLink>
    </div>

    <template v-else-if="!loading">
      <div class="controls">
        <select :aria-label="$t('Add a character')" value="" @change="add($event.target.value); $event.target.value = ''">
          <option value="" disabled>{{ $t('Add a character…') }}</option>
          <option v-for="c in addable" :key="c.id" :value="c.id">
            {{ c.current.name }}{{ c.current.height ? '' : ' ' + $t('(no height)') }}
          </option>
        </select>
        <button
          type="button"
          class="btn btn-ghost small"
          :disabled="!withHeightLeft"
          :title="withHeightLeft ? '' : $t('Nobody left to add has a height: set one on their version first.')"
          @click="addAll"
        >
          {{ $t('Add everyone with a height') }}
        </button>
        <button type="button" class="btn btn-ghost small" :disabled="drawn.length < 2" @click="sortByHeight">
          {{ $t('Sort by height') }}
        </button>
        <button type="button" class="btn btn-ghost small" :disabled="!picked.length" @click="setPicked([])">
          {{ $t('Clear') }}
        </button>
      </div>

      <div v-if="!picked.length" class="empty-state glass-panel">
        <p class="empty-title">{{ $t('Nobody on the chart yet.') }}</p>
        <p class="empty-hint">{{ $t('Add characters to stand them side by side. Their height comes from their version, their outline from its body type.') }}</p>
      </div>

      <div v-else ref="box" class="chart glass-panel">
        <svg v-if="drawn.length" :width="layout.width" :height="layout.height" class="chart-svg" role="img" :aria-label="$t('Height chart')">
          <g class="grid-lines">
            <template v-for="g in layout.grid" :key="g.y">
              <line :x1="AXIS" :x2="layout.width - 8" :y1="g.y" :y2="g.y" />
              <text :x="AXIS - 8" :y="g.y + 4" text-anchor="end">{{ g.label }}</text>
            </template>
          </g>
          <line class="ground" :x1="AXIS" :x2="layout.width - 8" :y1="layout.ground" :y2="layout.ground" />
          <line
            v-if="hovered != null && layout.figures[hovered]"
            class="head-line"
            :x1="AXIS"
            :x2="layout.width - 8"
            :y1="layout.figures[hovered].top"
            :y2="layout.figures[hovered].top"
          />
          <g
            v-for="(f, i) in layout.figures"
            :key="f.key"
            class="figure"
            :class="{ 'is-hovered': hovered === i }"
            @mouseenter="hovered = i"
            @mouseleave="hovered = null"
            @click="router.push(`/characters/${f.entry.character.id}`)"
          >
            <title>{{ f.entry.version.name }} · {{ formatHeight(f.entry.version.height) }}</title>
            <rect :x="f.x - f.slot / 2" :y="f.top - 24" :width="f.slot" :height="layout.ground - f.top + 48" fill="transparent" />
            <OutlineFigure
              :outline="f.entry.outline"
              :x="f.x"
              :ground="layout.ground"
              :height="f.px"
              :fill="f.fill"
              :stroke="f.stroke"
              :line="1.5"
            />
            <text class="figure-height" :x="f.x" :y="f.top - 8" text-anchor="middle">{{ formatHeight(f.entry.version.height) }}</text>
            <text class="figure-name" :x="f.x" :y="layout.ground + 18" text-anchor="middle">{{ clip(f.entry.version.name, f.slot) }}</text>
          </g>
        </svg>
        <p v-else class="no-heights">{{ $t('None of these have a height yet.') }}</p>
      </div>

      <ul v-if="picked.length" class="cards">
        <li v-for="(e, i) in entries" :key="e.character.id" class="card glass-panel" :style="{ '--swatch': e.stroke }">
          <div class="card-top">
            <span class="swatch" aria-hidden="true"></span>
            <router-link :to="`/characters/${e.character.id}`" class="card-name">{{ e.version.name }}</router-link>
            <span class="card-height">{{ e.version.height ? formatHeight(e.version.height) : $t('No height') }}</span>
          </div>
          <select
            v-if="e.character.versions.length > 1"
            :value="e.version.id"
            :aria-label="$t('Version')"
            @change="setVersion(i, +$event.target.value)"
          >
            <option v-for="v in e.character.versions" :key="v.id" :value="v.id">{{ versionLabel(v) }}</option>
          </select>
          <p class="card-outline">
            {{ outlineText(e.outline) }}
            <span class="card-source">{{ e.bodyType ? $t('Body type: {name}', { name: e.bodyType.name }) : $t('No body type') }}</span>
          </p>
          <p v-if="!e.version.height" class="card-missing">
            <router-link :to="`/characters/${e.character.id}/versions/${e.version.id}/edit`">{{ $t('Set a height on this version') }}</router-link>
          </p>
          <div class="card-actions">
            <button type="button" class="btn btn-ghost small" :disabled="i === 0" :aria-label="$t('Move left')" @click="move(i, -1)">‹</button>
            <button type="button" class="btn btn-ghost small" :disabled="i === entries.length - 1" :aria-label="$t('Move right')" @click="move(i, 1)">›</button>
            <button type="button" class="btn btn-ghost small" @click="remove(i)">{{ $t('Remove') }}</button>
          </div>
        </li>
      </ul>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount, nextTick, watch } from 'vue'
import { fullDate } from '../calendar'
import { useRoute, useRouter } from 'vue-router'
import { t, tr } from '../i18n'
import Breadcrumbs from '../components/Breadcrumbs.vue'
import OutlineFigure from '../components/OutlineFigure.vue'
import { fetchCharacterHeights } from '../api'
import { figure, outlineFor, OUTLINE_SEXES, OUTLINE_AGES, OUTLINE_BUILDS } from '../bodyOutlines'

const route = useRoute()
const router = useRouter()

const characters = ref([]) // [{ id, versions, current }]
const bodyTypes = ref(new Map())
const loading = ref(true)
const loadError = ref('')

onMounted(async () => {
  try {
    const data = await fetchCharacterHeights()
    characters.value = data.characters.map((c) => ({ ...c, current: c.versions.find((v) => v.is_current) || c.versions[c.versions.length - 1] }))
    bodyTypes.value = new Map(data.body_types.map((b) => [b.id, b]))
  } catch (err) {
    loadError.value = t("Couldn't load the characters.")
  } finally {
    loading.value = false
  }
})

// Who is on the chart, in order, lives in the URL: ?c=4,9.31 (character 9
// at its version 31; without one, the current version).
const picked = computed(() =>
  String(route.query.c || '')
    .split(',')
    .filter(Boolean)
    .map((s) => {
      const [id, version] = s.split('.').map(Number)
      return { id, version: version || null }
    })
    .filter((p) => Number.isInteger(p.id)),
)

function setPicked(list) {
  const query = { ...route.query }
  if (list.length) query.c = list.map((p) => (p.version ? `${p.id}.${p.version}` : `${p.id}`)).join(',')
  else delete query.c
  router.replace({ query })
}

const byId = computed(() => new Map(characters.value.map((c) => [c.id, c])))

// Each picked character with the version shown, its outline and colours.
const entries = computed(() =>
  picked.value
    .map((p) => byId.value.get(p.id) && { pick: p, character: byId.value.get(p.id) })
    .filter(Boolean)
    .map(({ pick, character }, i) => {
      const version = character.versions.find((v) => v.id === pick.version) || character.current
      const bodyType = version.body_type_id != null ? bodyTypes.value.get(version.body_type_id) : null
      const hue = (205 + i * 137.508) % 360
      return {
        character,
        version,
        bodyType,
        outline: outlineFor(bodyType?.outline, version),
        // Opaque, so the parts' outlines underneath stay hidden.
        fill: `hsl(${hue.toFixed(0)} 38% 30%)`,
        stroke: `hsl(${hue.toFixed(0)} 70% 74%)`,
      }
    }),
)
const drawn = computed(() => entries.value.filter((e) => e.version.height > 0))

const addable = computed(() => {
  const on = new Set(picked.value.map((p) => p.id))
  return characters.value.filter((c) => !on.has(c.id))
})
const withHeightLeft = computed(() => addable.value.some((c) => c.current.height > 0))

function add(id) {
  if (id) setPicked([...picked.value, { id: +id, version: null }])
}
function addAll() {
  setPicked([...picked.value, ...addable.value.filter((c) => c.current.height > 0).map((c) => ({ id: c.id, version: null }))])
}
function remove(i) {
  setPicked(picked.value.filter((_, j) => j !== i))
}
function move(i, by) {
  const list = [...picked.value]
  ;[list[i], list[i + by]] = [list[i + by], list[i]]
  setPicked(list)
}
function setVersion(i, versionId) {
  const list = [...picked.value]
  const current = byId.value.get(list[i].id)?.current
  list[i] = { ...list[i], version: versionId === current?.id ? null : versionId }
  setPicked(list)
}
// Shortest to tallest; pressed again, the other way round.
function sortByHeight() {
  const height = (p) => entries.value.find((e) => e.character.id === p.id)?.version.height || 0
  const up = [...picked.value].sort((a, b) => height(a) - height(b))
  const same = up.every((p, i) => p.id === picked.value[i].id)
  setPicked(same ? up.reverse() : up)
}

// Units: centimetres, or feet and inches (remembered on this device).
const UNITS = [
  { key: 'cm', label: tr('cm') },
  { key: 'ft', label: tr('ft / in') },
]
const UNIT_KEY = 'polaris-height-unit'
function readUnit() {
  try {
    return localStorage.getItem(UNIT_KEY) === 'ft' ? 'ft' : 'cm'
  } catch {
    return 'cm'
  }
}
const unit = ref(readUnit())
function setUnit(u) {
  unit.value = u
  try {
    localStorage.setItem(UNIT_KEY, u)
  } catch {
    // Private windows can refuse; the choice just isn't remembered.
  }
}

function feet(cm, round = 1) {
  const inches = Math.round(cm / 2.54 / round) * round
  const ft = Math.floor(inches / 12)
  const rest = inches - ft * 12
  return rest ? `${ft}′ ${rest}″` : `${ft}′`
}
function formatHeight(cm) {
  if (!(cm > 0)) return ''
  if (unit.value === 'ft') return cm < 30.48 ? `${(cm / 2.54).toFixed(1)}″` : feet(cm)
  return `${+cm.toFixed(1)} cm`
}

// The chart's size follows its box.
const box = ref(null)
const boxWidth = ref(800)
let observer = null
watch(box, (el) => {
  observer?.disconnect()
  if (!el) return
  observer = new ResizeObserver(() => (boxWidth.value = el.clientWidth))
  observer.observe(el)
  boxWidth.value = el.clientWidth
})
onBeforeUnmount(() => observer?.disconnect())
const viewHeight = ref(window.innerHeight)
const onResize = () => (viewHeight.value = window.innerHeight)
onMounted(() => window.addEventListener('resize', onResize))
onBeforeUnmount(() => window.removeEventListener('resize', onResize))

const AXIS = 64
const TOP = 34
const BOTTOM = 34

// Grid lines: about eight, on round numbers of the unit.
function gridStep(maxCm) {
  if (unit.value === 'ft') {
    const steps = [1, 2, 3, 6, 12, 24, 60, 120, 300, 600, 1200, 6000, 12000]
    return (steps.find((s) => (maxCm / 2.54) / s <= 9) || steps[steps.length - 1]) * 2.54
  }
  const steps = [1, 2, 5, 10, 20, 25, 50, 100, 200, 500, 1000, 2000, 5000, 10000, 50000, 100000]
  return steps.find((s) => maxCm / s <= 9) || steps[steps.length - 1]
}

const hovered = ref(null)

const layout = computed(() => {
  const list = drawn.value
  const room = Math.round(Math.min(Math.max(viewHeight.value * 0.62, 340), 760)) - TOP - BOTTOM
  const tallest = Math.max(1, ...list.map((e) => e.version.height))
  // Each figure's width per px of scale (the outline is 1 tall, halfWidth wide).
  const widths = list.map((e) => e.version.height * figureHalf(e.outline) * 2)
  const slotsAt = (scale) => widths.map((w) => Math.max(w * scale + 28, 92))
  const neededAt = (scale) => AXIS + 16 + slotsAt(scale).reduce((a, b) => a + b, 0) + 8
  // As tall as the room allows; shorter when that is too wide for the box,
  // down to half size, then the chart scrolls.
  const full = room / (tallest * 1.04) // px per cm
  let scale = full
  while (scale > full * 0.5 && neededAt(scale) > boxWidth.value - 2) scale *= 0.95
  const height = Math.round(TOP + tallest * 1.04 * scale + BOTTOM)
  const ground = height - BOTTOM
  const figures = list.map((e) => ({ entry: e, px: e.version.height * scale, key: e.character.id }))
  const slots = slotsAt(scale)
  const needed = neededAt(scale)
  const width = Math.max(boxWidth.value - 2, needed)
  // Spread over the room there is, but not further apart than needed.
  const extra = Math.min((width - needed) / Math.max(1, figures.length), 60)
  let x = AXIS + 16 + (width - needed - extra * figures.length) / 2
  figures.forEach((f, i) => {
    f.slot = slots[i] + extra
    f.x = x + f.slot / 2
    f.top = ground - f.px
    f.fill = f.entry.fill
    f.stroke = f.entry.stroke
    x += f.slot
  })
  const step = gridStep(tallest * 1.04)
  const grid = []
  for (let v = step; v * scale <= ground - TOP + 1e-6 + 12; v += step) {
    grid.push({ y: ground - v * scale, label: unit.value === 'ft' ? feet(v, 1) : `${+v.toFixed(1)} cm` })
  }
  return { width, height, ground, figures, grid }
})

const halfCache = new Map()
function figureHalf(o) {
  const k = `${o.sex}/${o.age}/${o.build}`
  if (!halfCache.has(k)) halfCache.set(k, figure(o).halfWidth)
  return halfCache.get(k)
}

// Names shortened to fit their slot (about 7 px a letter).
function clip(name, slot) {
  const room = Math.max(4, Math.floor(slot / 7))
  return name.length > room ? name.slice(0, room - 1) + '…' : name
}

function versionLabel(v) {
  const parts = [v.name]
  if (v.version_date) parts.push(fullDate(v.version_date))
  if (v.version_reference) parts.push(v.version_reference)
  if (v.is_current) parts.push(t('current'))
  return parts.join(' · ')
}

const label = (list, key) => list.find((x) => x.key === key)?.label
function outlineText(o) {
  return [t(label(OUTLINE_SEXES, o.sex) || tr('In between')), t(label(OUTLINE_AGES, o.age)), t(label(OUTLINE_BUILDS, o.build))].join(' · ')
}

// A newly added figure scrolls into view when the chart is wider than its box.
watch(
  () => picked.value.length,
  async (n, was) => {
    if (n <= was) return
    await nextTick()
    if (box.value) box.value.scrollLeft = box.value.scrollWidth
  },
)
</script>

<style scoped>
.actions {
  display: flex;
  gap: 0.6rem;
  align-items: center;
}

.unit-switch {
  display: inline-flex;
  border: 1px solid var(--glass-border);
  border-radius: var(--radius-control);
  overflow: hidden;
}

.unit-switch button {
  background: transparent;
  border: none;
  color: var(--text-muted);
  font: inherit;
  font-size: 0.82rem;
  font-weight: 600;
  padding: 0.45rem 0.85rem;
  cursor: pointer;
}

.unit-switch button.is-on {
  background: var(--accent-soft);
  color: var(--text-primary);
}

.controls {
  display: flex;
  flex-wrap: wrap;
  gap: 0.6rem;
  align-items: center;
  margin-bottom: 1rem;
}

.controls select {
  width: auto;
  min-width: 14rem;
}

.chart {
  overflow-x: auto;
  padding: 0;
  margin-bottom: 1.25rem;
}

.chart-svg {
  display: block;
}

.grid-lines line {
  stroke: var(--glass-border);
  stroke-width: 1;
}

.grid-lines text {
  fill: var(--text-faint);
  font-size: 11px;
  font-family: 'Manrope', sans-serif;
}

.ground {
  stroke: var(--text-muted);
  stroke-width: 1.5;
}

.head-line {
  stroke: var(--accent);
  stroke-width: 1;
  stroke-dasharray: 4 4;
}

.figure {
  cursor: pointer;
}

.figure-height {
  fill: var(--text-muted);
  font-size: 11px;
  font-family: 'Manrope', sans-serif;
}

.figure-name {
  fill: var(--text-primary);
  font-size: 12px;
  font-weight: 600;
  font-family: 'Manrope', sans-serif;
}

.figure.is-hovered .figure-height {
  fill: var(--accent);
}

.no-heights {
  padding: 2rem;
  color: var(--text-muted);
  text-align: center;
}

.cards {
  list-style: none;
  padding: 0;
  margin: 0;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 0.8rem;
}

.card {
  padding: 0.8rem 0.95rem;
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
}

.card-top {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.swatch {
  width: 0.7rem;
  height: 0.7rem;
  border-radius: 50%;
  background: var(--swatch);
  flex-shrink: 0;
}

.card-name {
  font-family: 'Fraunces', serif;
  color: var(--text-primary);
  text-decoration: none;
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.card-name:hover {
  text-decoration: underline;
}

.card-height {
  font-size: 0.82rem;
  color: var(--text-muted);
  white-space: nowrap;
}

.card select {
  font-size: 0.8rem;
}

.card-outline {
  margin: 0;
  font-size: 0.8rem;
  color: var(--text-muted);
  display: flex;
  flex-direction: column;
  gap: 0.1rem;
}

.card-source {
  color: var(--text-faint);
  font-size: 0.75rem;
}

.card-missing {
  margin: 0;
  font-size: 0.8rem;
}

.card-missing a {
  color: var(--accent);
}

.card-actions {
  display: flex;
  gap: 0.35rem;
}

.card-actions .btn:last-child {
  margin-left: auto;
}
</style>
