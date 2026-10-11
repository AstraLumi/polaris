<template>
  <div class="page is-wide timeline-page">
    <header class="page-header">
      <div>
        <h1>{{ $t('Timeline') }}</h1>
        <p class="page-sub">
          <template v-if="data">
            <template v-if="filtering">{{ $t('{shown} of {total}', { shown: nodes.length, total: data.nodes.length }) }}</template>
            <template v-else>{{ $tn('{n} point', '{n} points', data.nodes.length) }}</template>
            <template v-if="highlighting"> · {{ $t('{n} highlighted', { n: matchCount }) }}</template>
            <template v-if="range"> · {{ range }}</template>
          </template>
          <template v-else>&nbsp;</template>
        </p>
      </div>
      <RouterLink to="/events/new" class="btn btn-primary add-btn">{{ $t('New event') }}</RouterLink>
    </header>

    <p v-if="loadError" class="error-banner">{{ loadError }}</p>
    <p v-else-if="!data" class="loading-hint">{{ $t('Loading…') }}</p>

    <template v-else>
      <section class="controls glass-panel panel-solid">
        <label class="ctl">
          <span>{{ $t('Zoom') }}</span>
          <input
            type="range"
            min="0"
            max="100"
            step="1"
            :value="zoomSlider"
            :aria-label="$t('Zoom')"
            @input="onZoom($event.target.value)"
          />
          <span class="ctl-value">{{ Math.round(pxPerYear) }} px/yr</span>
          <button type="button" class="chip-btn" :class="{ 'is-on': auto }" :title="$t('Fit the whole timeline to the window')" @click="setAuto">{{ $t('Auto') }}</button>
        </label>

        <label class="ctl" :title="$t('A gap between events longer than this becomes a coil instead of empty space. 0 never collapses.')">
          <span>{{ $t('Coil gaps over') }}</span>
          <input v-model.number="collapseYears" type="number" min="0" step="1" class="num" />
          <span class="ctl-value">{{ $t('years') }}</span>
        </label>

        <label v-if="allTags.length" class="ctl">
          <span>{{ $t('Highlight tag') }}</span>
          <select v-model="highlightTag">
            <option value="">{{ $t('None') }}</option>
            <option v-for="t in allTags" :key="t" :value="t">#{{ t }}</option>
          </select>
        </label>

        <label v-if="personOptions.length" class="ctl">
          <span>{{ $t('Person') }}</span>
          <select :value="personId" @change="setFilter('person', $event.target.value)">
            <option value="">{{ $t('Everyone') }}</option>
            <option v-for="p in personOptions" :key="p.id" :value="String(p.id)">{{ p.name }}</option>
          </select>
        </label>

        <label v-if="placeOptions.length" class="ctl">
          <span>{{ $t('Place') }}</span>
          <select :value="placeId" @change="setFilter('place', $event.target.value)">
            <option value="">{{ $t('Everywhere') }}</option>
            <option v-for="p in placeOptions" :key="p.id" :value="String(p.id)">{{ p.name }}</option>
          </select>
        </label>

        <div v-if="personId || placeId" class="ctl" role="group" :aria-label="$t('How to show the person or place')">
          <span>{{ $t('Show') }}</span>
          <button type="button" class="chip-btn" :class="{ 'is-on': !highlightMatch }" :aria-pressed="!highlightMatch" :title="$t('Hide every other point')" @click="setFilter('match', '')">
            {{ $t('Only theirs') }}
          </button>
          <button type="button" class="chip-btn" :class="{ 'is-on': highlightMatch }" :aria-pressed="highlightMatch" :title="$t('Keep every point and fade the others')" @click="setFilter('match', 'highlight')">
            {{ $t('Highlight') }}
          </button>
        </div>

        <div class="legend" :aria-label="$t('Legend')">
          <button
            v-for="k in KINDS"
            :key="k"
            type="button"
            class="key"
            :class="{ 'is-off': hiddenKinds.has(k) }"
            :title="$t('Show or hide these points')"
            :aria-pressed="!hiddenKinds.has(k)"
            @click="toggleKind(k)"
          >
            <i class="dot" :class="'kind-' + k"></i>{{ kindLabel(k) }}
          </button>
        </div>
      </section>

      <p v-if="data.undated" class="note">
        {{ $tn('{n} event has no readable date and isn\'t shown.', '{n} events have no readable date and aren\'t shown.', data.undated) }}
        <RouterLink to="/events">{{ $t('Fix on the Events page') }}</RouterLink>
      </p>

      <div v-if="data.nodes.length && !nodes.length" class="empty-state glass-panel">
        <p class="empty-title">{{ $t('Nothing matches these filters.') }}</p>
        <p class="empty-hint"><button type="button" class="link-btn" @click="clearFilters">{{ $t('Clear filter') }}</button></p>
      </div>

      <div v-else-if="!data.nodes.length" class="empty-state glass-panel">
        <p class="empty-title">{{ $t('Nothing to place yet.') }}</p>
        <p class="empty-hint">
          {{ $t('Create an event, give a character an in-story birth date, or set a founding date on the Map — each shows up here.') }}
        </p>
      </div>

      <div v-else ref="scrollerEl" class="scroller">
        <div ref="stageEl" class="stage" :style="{ width: layout.width + 'px', height: layout.height + 'px' }">
          <svg class="lines" :width="layout.width" :height="layout.height" aria-hidden="true">
            <path :d="layout.pathD" class="line-glow" />
            <path :d="layout.pathD" class="line" />
            <line
              v-for="n in layout.nodes"
              :key="'stem-' + n.key"
              class="stem"
              :x1="n.x"
              :y1="n.y"
              :x2="n.x"
              :y2="n.y + stemEnd(n)"
            />
            <text
              v-for="(c, i) in layout.coils"
              :key="'coil-' + i"
              class="coil-caption"
              :x="c.x"
:y="c.y + c.depth + 18"
              text-anchor="middle"
            >
              ≈ {{ formatYears(c.years) }}
            </text>
          </svg>

          <template v-for="n in layout.nodes" :key="n.key">
            <button
              type="button"
              class="label"
              :class="[labelClass(n), { 'is-selected': selectedKey === n.key, 'is-dim': isDim(n), 'is-marked': isMarked(n) }]"
              :style="{ left: n.x + 'px', top: n.y + labelOffset(n) + 'px', width: n.labelWidth + 'px' }"
              @click="select(n.key)"
            >
              <span class="label-name">{{ n.name }}</span>
              <span class="label-date">{{ shortDate(n.date) }}</span>
            </button>
            <button
              type="button"
              class="node"
              :class="[
                'kind-' + n.kind,
                { 'is-selected': selectedKey === n.key, 'is-related': related.has(n.key), 'is-dim': isDim(n), 'is-marked': isMarked(n) },
              ]"
              :style="{ left: n.x + 'px', top: n.y + 'px' }"
              :aria-label="`${n.name}, ${shortDate(n.date)}`"
              @click="select(n.key)"
            ></button>
          </template>
        </div>
      </div>
    </template>

    <Transition name="slide">
      <aside v-if="selected" class="detail glass-panel panel-solid" role="dialog" :aria-label="selected.name">
        <button type="button" class="close" :aria-label="$t('Close')" @click="selectedKey = null">×</button>

        <span class="kind-badge" :class="'kind-' + selected.kind">{{ kindLabel(selected.kind) }}</span>
        <h2>{{ selected.name }}</h2>
        <p class="when">
          <strong>{{ shortDate(selected.date) }}</strong>
          <span>{{ fullDate(selected.date) }}</span>
        </p>
        <p v-if="selected.location_name" class="where">{{ $t('at {place}', { place: selected.location_name }) }}</p>

        <img v-if="selected.picture_path" :src="selected.picture_path" :alt="selected.name" class="pic zoomable" @click="viewPicture(selected.picture_path, selected.name)" />

        <p v-if="selected.description" class="desc">{{ selected.description }}</p>
        <p v-else class="desc is-empty">{{ $t('No description yet.') }}</p>

        <div v-if="selected.tags.length" class="block">
          <h3>{{ $t('Tags') }}</h3>
          <div class="chips">
            <RouterLink
              v-for="t in selected.tags"
              :key="t"
              :to="{ path: '/events', query: { tag: t } }"
              class="tag-chip"
            >
              #{{ t }}
            </RouterLink>
          </div>
        </div>

        <div v-if="selected.people.length" class="block">
          <h3>{{ $t('Involved') }}</h3>
          <div class="chips">
            <RouterLink v-for="p in selected.people" :key="p.id" :to="`/characters/${p.id}`" class="person-chip">
              {{ p.name }}
            </RouterLink>
          </div>
        </div>

        <div v-if="linkedNodes.length" class="block">
          <h3>{{ $t('Linked by tag') }}</h3>
          <ul class="linked">
            <li v-for="l in linkedNodes" :key="l.key">
              <button type="button" class="linked-row" @click="select(l.key, true)">
                <span class="linked-date">{{ shortDate(l.date) }}</span>
                <span>{{ l.name }}</span>
              </button>
            </li>
          </ul>
        </div>

        <p v-if="actionError" class="error-banner">{{ actionError }}</p>
        <div class="actions">
          <RouterLink
            v-if="selected.kind === 'event'"
            :to="`/events/${selected.event_id}`"
            class="btn btn-primary"
          >
            {{ $t('Open event page') }}
          </RouterLink>

          <template v-else>
            <RouterLink
              v-if="selected.kind === 'birth'"
              :to="`/characters/${selected.source_id}`"
              class="btn btn-primary"
            >
              {{ $t('View character') }}
            </RouterLink>
            <RouterLink v-else :to="mapPath(selected.source_id)" class="btn btn-primary">{{ $t('View on map') }}</RouterLink>

            <RouterLink v-if="selected.event_id" :to="`/events/${selected.event_id}`" class="btn btn-ghost">
              {{ $t('Event page') }}
            </RouterLink>
            <button v-else type="button" class="btn btn-ghost" :disabled="busy" @click="addDetails">
              {{ busy ? $t('Opening…') : $t('Add event details') }}
            </button>
          </template>
        </div>
      </aside>
    </Transition>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { timelineApi, eventsApi } from '../api'
import { shortDate, fullDate } from '../calendar'
import { t, tn } from '../i18n'
import { layoutTimeline, autoPxPerYear, minimumWidth, DEFAULTS } from '../timelineLayout'
import { viewPicture, mapPath } from '../navigation'

const router = useRouter()
const route = useRoute()

const data = ref(null)
const loadError = ref('')
const scrollerEl = ref(null)
const stageEl = ref(null)
const measuredWidth = ref(1000)

const selectedKey = ref(null)
const highlightTag = ref('')
const actionError = ref('')
const busy = ref(false)

// ---- Settings (remembered between visits) -------------------------------------

const SETTINGS_KEY = 'timeline-settings'
function readSettings() {
  try {
    return JSON.parse(localStorage.getItem(SETTINGS_KEY) || '{}') || {}
  } catch (err) {
    return {}
  }
}
const saved = readSettings()
const auto = ref(saved.pxPerYear == null)
const manualPx = ref(typeof saved.pxPerYear === 'number' ? saved.pxPerYear : DEFAULTS.pxPerYear)
const collapseYears = ref(Number.isFinite(saved.collapseYears) ? saved.collapseYears : DEFAULTS.collapseYears)

watch([auto, manualPx, collapseYears], () => {
  try {
    localStorage.setItem(
      SETTINGS_KEY,
      JSON.stringify({ pxPerYear: auto.value ? null : manualPx.value, collapseYears: collapseYears.value }),
    )
  } catch (err) {
    // Remembering the settings is a convenience only.
  }
})

const opts = computed(() => ({ collapseYears: Math.max(0, Number(collapseYears.value) || 0) }))
const layoutWidth = computed(() => Math.max(measuredWidth.value, minimumWidth()))

const pxPerYear = computed(() => {
  if (!data.value) return manualPx.value
  return auto.value
    ? autoPxPerYear(nodes.value, data.value.year_days, layoutWidth.value, opts.value)
    : manualPx.value
})

const layout = computed(() => {
  const yd = data.value ? data.value.year_days : 360
  return layoutTimeline(nodes.value, yd, layoutWidth.value, { ...opts.value, pxPerYear: pxPerYear.value })
})

// The slider is logarithmic: 2 px/yr at the left, 400 at the right.
const PX_MIN = 2
const PX_MAX = 400
const zoomSlider = computed(
  () => (100 * Math.log(Math.max(PX_MIN, Math.min(PX_MAX, pxPerYear.value)) / PX_MIN)) / Math.log(PX_MAX / PX_MIN),
)
function onZoom(v) {
  auto.value = false
  manualPx.value = PX_MIN * Math.pow(PX_MAX / PX_MIN, Number(v) / 100)
}
function setAuto() {
  auto.value = true
}

// ---- Derived data ---------------------------------------------------------------

// ---- Filters (in the URL, so a character page can link to its own life) -------

const KINDS = ['event', 'birth', 'founding']
const queryText = (k) => (typeof route.query[k] === 'string' ? route.query[k] : '')
const personId = computed(() => queryText('person'))
const placeId = computed(() => queryText('place'))
const hiddenKinds = computed(() => new Set(queryText('hide').split(',').filter((k) => KINDS.includes(k))))
// match=highlight keeps every point and fades the ones that aren't the
// person's or place's (the character page links this way); without it the
// others are hidden.
const highlightMatch = computed(() => queryText('match') === 'highlight')
const highlighting = computed(() => highlightMatch.value && !!(personId.value || placeId.value))
const filtering = computed(() => !!(((personId.value || placeId.value) && !highlightMatch.value) || hiddenKinds.value.size))

function setFilter(key, value) {
  const query = { ...route.query }
  if (value) query[key] = value
  else delete query[key]
  selectedKey.value = null
  router.replace({ query })
}

function toggleKind(k) {
  const next = new Set(hiddenKinds.value)
  if (next.has(k)) next.delete(k)
  else next.add(k)
  setFilter('hide', [...next].join(','))
}

function clearFilters() {
  router.replace({ query: {} })
}

// A person's points: their birth and every event they're in. A place's: its
// founding and every event set there.
const inPerson = (n, id) => (n.kind === 'birth' && n.source_id === id) || n.people.some((p) => p.id === id)
const inPlace = (n, id) => (n.kind === 'founding' && n.source_id === id) || n.location_id === id

function matches(n) {
  const person = Number(personId.value)
  const place = Number(placeId.value)
  return (!person || inPerson(n, person)) && (!place || inPlace(n, place))
}

const nodes = computed(() => {
  const all = data.value ? data.value.nodes : []
  return all.filter((n) => !hiddenKinds.value.has(n.kind) && (highlightMatch.value || matches(n)))
})

const isMarked = (n) => highlighting.value && matches(n)
const matchCount = computed(() => nodes.value.filter(isMarked).length)

// Bring the first highlighted point into view when the highlight changes.
watch(
  () => [data.value, personId.value, placeId.value, highlightMatch.value],
  async () => {
    if (!highlighting.value) return
    await nextTick()
    const first = layout.value.nodes.find(isMarked)
    if (first && scrollerEl.value) {
      scrollerEl.value.scrollTo({ left: Math.max(0, first.x - scrollerEl.value.clientWidth / 3), behavior: 'smooth' })
    }
  },
  { flush: 'post' },
)

function optionsFrom(pairs) {
  const seen = new Map()
  for (const [id, name] of pairs) if (id != null && name) seen.set(id, name)
  return [...seen].map(([id, name]) => ({ id, name })).sort((a, b) => a.name.localeCompare(b.name))
}
const personOptions = computed(() =>
  optionsFrom((data.value?.nodes || []).flatMap((n) => [
    ...(n.kind === 'birth' ? [[n.source_id, n.source_name]] : []),
    ...n.people.map((p) => [p.id, p.name]),
  ])),
)
const placeOptions = computed(() =>
  optionsFrom((data.value?.nodes || []).flatMap((n) => [
    ...(n.kind === 'founding' ? [[n.source_id, n.source_name]] : []),
    [n.location_id, n.location_name],
  ])),
)

const range = computed(() => {
  const n = nodes.value
  if (!n || !n.length) return ''
  const a = shortDate(n[0].date)
  const b = shortDate(n[n.length - 1].date)
  return a === b ? a : `${a} → ${b}`
})

const allTags = computed(() => {
  const seen = new Map()
  for (const n of data.value?.nodes || []) for (const t of n.tags) seen.set(t.toLowerCase(), t)
  return [...seen.values()].sort((a, b) => a.localeCompare(b, undefined, { sensitivity: 'base' }))
})

const selected = computed(() => nodes.value.find((n) => n.key === selectedKey.value) || null)

const hasTag = (n, tag) => n.tags.some((t) => t.toLowerCase() === tag.toLowerCase())

// Other points that share a tag with the selected one.
const linkedNodes = computed(() => {
  const s = selected.value
  if (!s || !s.tags.length) return []
  return nodes.value.filter((n) => n.key !== s.key && s.tags.some((t) => hasTag(n, t)))
})
const related = computed(() => new Set(linkedNodes.value.map((n) => n.key)))

const isDim = (n) => (!!highlightTag.value && !hasTag(n, highlightTag.value)) || (highlighting.value && !matches(n))

// ---- Drawing helpers ------------------------------------------------------------

const LANE_OFFSET = { up1: 20, down1: 20, up2: 88, down2: 88 }
const isUp = (n) => n.lane.startsWith('up')
const labelOffset = (n) => (isUp(n) ? -LANE_OFFSET[n.lane] : LANE_OFFSET[n.lane])
const stemEnd = (n) => labelOffset(n)
const labelClass = (n) => (isUp(n) ? 'is-up' : 'is-down')

const kindLabel = (k) => (k === 'birth' ? t('Birth') : k === 'founding' ? t('Founding') : t('Event'))

function formatYears(y) {
  const r = Math.round(y)
  return tn('{n} year', '{n} years', r, { n: r.toLocaleString() })
}

// ---- Selection ------------------------------------------------------------------

function select(key, scroll = false) {
  selectedKey.value = selectedKey.value === key && !scroll ? null : key
  actionError.value = ''
  if (scroll) scrollToNode(key)
}

function scrollToNode(key) {
  const n = layout.value.nodes.find((x) => x.key === key)
  if (!n || !stageEl.value) return
  const top = stageEl.value.getBoundingClientRect().top + window.scrollY + n.y - window.innerHeight / 2
  window.scrollTo({ top: Math.max(0, top), behavior: 'smooth' })
  scrollerEl.value?.scrollTo({ left: Math.max(0, n.x - scrollerEl.value.clientWidth / 2), behavior: 'smooth' })
}

// An imported node's "event page" doesn't exist until details are first
// added; this finds or creates it, then opens it for editing.
async function addDetails() {
  const s = selected.value
  if (!s) return
  busy.value = true
  actionError.value = ''
  try {
    const { id } = await eventsApi.fromSource(s.kind === 'birth' ? 'character' : 'location', s.source_id)
    router.push(`/events/${id}/edit`)
  } catch (err) {
    actionError.value = err.message || t("Couldn't open event details.")
  } finally {
    busy.value = false
  }
}

function onKey(e) {
  if (e.key === 'Escape') selectedKey.value = null
}

// ---- Loading & sizing -----------------------------------------------------------

let resizeObserver = null
function measure() {
  if (scrollerEl.value) measuredWidth.value = Math.floor(scrollerEl.value.clientWidth)
}

watch(scrollerEl, (el) => {
  resizeObserver?.disconnect()
  if (!el) return
  measure()
  resizeObserver = new ResizeObserver(measure)
  resizeObserver.observe(el)
})

onMounted(async () => {
  window.addEventListener('keydown', onKey)
  try {
    data.value = await timelineApi.get()
  } catch (err) {
    loadError.value = t("Couldn't load the timeline. Try refreshing.")
    return
  }
  await nextTick()
  measure()
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKey)
  resizeObserver?.disconnect()
})
</script>

<style scoped>
.timeline-page {
  --kind-event: var(--accent);
}

.add-btn {
  text-decoration: none;
}

.loading-hint {
  color: var(--text-faint);
}

.controls {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.75rem 1.75rem;
  padding: 0.7rem 1.1rem;
  margin-bottom: 1rem;
  position: sticky;
  top: 0.75rem;
  z-index: 20;
}

.ctl {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  font-size: 0.8rem;
  color: var(--text-muted);
}

.ctl input[type='range'] {
  width: 150px;
  accent-color: var(--accent);
}

.ctl .num {
  width: 4.2rem;
  padding: 0.3rem 0.5rem;
}

.ctl select {
  width: auto;
  max-width: 14rem;
  padding: 0.3rem 2.2rem 0.3rem 0.5rem; /* room for the dropdown arrow */
}

.ctl-value {
  font-size: 0.74rem;
  color: var(--text-faint);
  min-width: 3.2rem;
}

.chip-btn {
  background: transparent;
  border: 1px solid var(--glass-border);
  color: var(--text-muted);
  border-radius: 999px;
  padding: 0.15rem 0.7rem;
  font-family: 'Manrope', sans-serif;
  font-size: 0.74rem;
  font-weight: 600;
  cursor: pointer;
}

.chip-btn.is-on {
  background: var(--accent-soft);
  border-color: color-mix(in srgb, var(--accent) 40%, transparent);
  color: var(--accent);
}

.legend {
  display: flex;
  gap: 1rem;
  margin-left: auto;
  font-size: 0.76rem;
  color: var(--text-muted);
}

.key {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0;
  border: none;
  background: none;
  color: inherit;
  font: inherit;
  cursor: pointer;
}

.key.is-off {
  opacity: 0.4;
  text-decoration: line-through;
}

.link-btn {
  border: none;
  background: none;
  padding: 0;
  color: var(--accent);
  font: inherit;
  cursor: pointer;
}

.dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  display: inline-block;
}

.kind-event {
  background: var(--kind-event);
}
.kind-birth {
  background: var(--kind-birth);
}
.kind-founding {
  background: var(--kind-founding);
}

.note {
  margin: 0 0 1rem;
  font-size: 0.82rem;
  color: var(--text-faint);
}

.note a {
  color: var(--accent);
}

/* ---- The drawing ---- */

.scroller {
  overflow-x: auto;
  overflow-y: visible;
}

.stage {
  position: relative;
}

.lines {
  position: absolute;
  inset: 0;
  pointer-events: none;
  overflow: visible;
}

.line {
  fill: none;
  stroke: var(--line);
  stroke-width: 3;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.line-glow {
  fill: none;
  stroke: var(--line);
  stroke-width: 9;
  opacity: 0.12;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.stem {
  stroke: color-mix(in srgb, var(--tint) 28%, transparent);
  stroke-width: 1;
}

.coil-caption {
  fill: var(--text-faint);
  font-size: 11px;
  font-family: 'Manrope', sans-serif;
}

.node {
  position: absolute;
  width: 15px;
  height: 15px;
  padding: 0;
  border-radius: 50%;
  border: 2px solid var(--bg);
  transform: translate(-50%, -50%);
  cursor: pointer;
  z-index: 3;
  transition: transform 0.12s ease, box-shadow 0.12s ease, opacity 0.12s ease;
}

.node.kind-event {
  background: var(--kind-event);
}
.node.kind-birth {
  background: var(--kind-birth);
}
.node.kind-founding {
  background: var(--kind-founding);
}

.node:hover {
  transform: translate(-50%, -50%) scale(1.3);
}

.node.is-selected {
  transform: translate(-50%, -50%) scale(1.45);
  box-shadow: 0 0 0 3px rgba(255, 255, 255, 0.85), 0 0 14px rgba(255, 255, 255, 0.35);
}

.node.is-related {
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 80%, transparent);
}

.node.is-dim,
.label.is-dim {
  opacity: 0.22;
}

.node.is-marked {
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--text-primary) 75%, transparent), 0 0 14px color-mix(in srgb, var(--accent) 55%, transparent);
}

.label.is-marked .label-name {
  color: var(--text-primary);
  font-weight: 700;
}

.label {
  position: absolute;
  transform: translateX(-50%);
  z-index: 2;
  display: flex;
  flex-direction: column;
  gap: 0.1rem;
  text-align: center;
  background: color-mix(in srgb, var(--surface) 82%, transparent);
  border: 1px solid transparent;
  border-radius: 8px;
  padding: 0.2rem 0.35rem;
  color: var(--text-primary);
  font-family: 'Manrope', sans-serif;
  cursor: pointer;
}

.label.is-up {
  transform: translate(-50%, -100%);
}

.label:hover,
.label.is-selected {
  border-color: var(--glass-border);
  background: color-mix(in srgb, var(--surface) 95%, transparent);
}

.label-name {
  font-size: 0.8rem;
  font-weight: 600;
  line-height: 1.25;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  overflow-wrap: anywhere;
}

.label-date {
  font-size: 0.7rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  color: var(--accent);
}

/* ---- Detail panel ---- */

.detail {
  position: fixed;
  top: 1.5rem;
  right: 1.5rem;
  bottom: 1.5rem;
  width: 380px;
  max-width: calc(100vw - 2rem);
  overflow-y: auto;
  padding: 1.5rem 1.5rem 1.25rem;
  z-index: 40;
}

.close {
  position: absolute;
  top: 0.7rem;
  right: 0.9rem;
  background: transparent;
  border: none;
  color: var(--text-muted);
  font-size: 1.6rem;
  line-height: 1;
  cursor: pointer;
}

.close:hover {
  color: var(--text-primary);
}

.kind-badge {
  display: inline-block;
  font-size: 0.66rem;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--accent-text);
  border-radius: 4px;
  padding: 0.1rem 0.5rem;
}

.detail h2 {
  font-family: 'Fraunces', serif;
  font-weight: 500;
  font-size: 1.45rem;
  line-height: 1.25;
  margin: 0.6rem 2rem 0.3rem 0;
}

.when {
  display: flex;
  align-items: baseline;
  gap: 0.7rem;
  margin: 0;
  font-size: 0.82rem;
  color: var(--text-faint);
}

.when strong {
  color: var(--accent);
  letter-spacing: 0.04em;
}

.where {
  margin: 0.3rem 0 0;
  font-size: 0.85rem;
  color: var(--text-muted);
}

.pic {
  width: 100%;
  max-height: 220px;
  object-fit: cover;
  border-radius: 10px;
  margin-top: 1rem;
}

.desc {
  margin: 1rem 0 0;
  font-size: 0.9rem;
  line-height: 1.6;
  white-space: pre-wrap;
}

.desc.is-empty {
  color: var(--text-faint);
  font-style: italic;
}

.block {
  margin-top: 1.1rem;
}

.block h3 {
  margin: 0 0 0.45rem;
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--text-faint);
}

.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
}

.tag-chip,
.person-chip {
  font-size: 0.78rem;
  font-weight: 600;
  border-radius: 999px;
  padding: 0.18rem 0.7rem;
  text-decoration: none;
}

.tag-chip {
  color: var(--accent);
  background: var(--accent-soft);
  border: 1px solid color-mix(in srgb, var(--accent) 30%, transparent);
}

.person-chip {
  color: var(--text-primary);
  border: 1px solid var(--glass-border);
  background: rgba(255, 255, 255, 0.04);
}

.linked {
  list-style: none;
  margin: 0;
  padding: 0;
}

.linked-row {
  display: flex;
  gap: 0.8rem;
  width: 100%;
  text-align: left;
  background: transparent;
  border: none;
  border-radius: 8px;
  padding: 0.35rem 0.4rem;
  color: var(--text-primary);
  font-family: 'Manrope', sans-serif;
  font-size: 0.84rem;
  cursor: pointer;
}

.linked-row:hover {
  background: rgba(255, 255, 255, 0.06);
}

.linked-date {
  width: 5rem;
  flex-shrink: 0;
  font-size: 0.74rem;
  font-weight: 700;
  color: var(--accent);
}

.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.6rem;
  margin-top: 1.4rem;
}

.actions .btn {
  text-decoration: none;
}

.slide-enter-active,
.slide-leave-active {
  transition: transform 0.18s ease, opacity 0.18s ease;
}

.slide-enter-from,
.slide-leave-to {
  transform: translateX(24px);
  opacity: 0;
}

@media (max-width: 720px) {
  .detail {
    top: auto;
    left: 0.75rem;
    right: 0.75rem;
    bottom: 0.75rem;
    width: auto;
    max-height: 70vh;
  }
  .legend {
    margin-left: 0;
  }
  .ctl {
    flex-wrap: wrap;
  }
  .ctl input[type='range'] {
    width: 120px;
  }
}
</style>
