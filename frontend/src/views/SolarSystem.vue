<template>
  <div class="map-page">
    <div class="map-wrap solar-page">
      <SolarEditor2D
        v-if="ready && bodies.length && mode === 'edit'"
        :bodies="liveBodies"
        :focus-id="focusId"
        :selected="selectedId"
        :show-zone="markers.zone"
        :show-limits="markers.limits"
        @select="pick"
        @focus="focusId = $event"
        @move="onMove"
        @commit="onCommit"
      />
      <SolarScene
        v-else-if="ready && bodies.length"
        ref="sceneRef"
        :bodies="liveBodies"
        :days="days"
        :selected="selectedId"
        :globe="globe"
        :tilt="main.tilt"
        :relative="live ? simRelative : null"
        :shapes="live ? shapes : null"
        :show-zone="markers.zone"
        :show-limits="markers.limits"
        :sky="skyUrl()"
        @select="pick"
      />

      <div v-if="ready && bodies.length" class="toolbar clock glass-panel panel-solid">
        <button type="button" class="tool-btn" :class="{ active: mode === 'edit' }" :title="$t('Edit the system from above, in 2D')" @click="setMode('edit')">{{ $t('Edit') }}</button>
        <button type="button" class="tool-btn" :class="{ active: mode === 'run' }" :title="$t('Watch it turn, in 3D')" @click="setMode('run')">{{ $t('Run') }}</button>
        <div class="tool-sep"></div>
        <button type="button" class="tool-btn marker-btn zone" :class="{ active: markers.zone }" :title="$t('Show where water can be liquid round the star')" @click="toggleMarker('zone')">
          {{ $t('Habitable zone') }}
        </button>
        <button type="button" class="tool-btn marker-btn limits" :class="{ active: markers.limits }" :title="$t('Show how close and how far the picked body can orbit')" @click="toggleMarker('limits')">
          {{ $t('Gravity limits') }}
        </button>
        <div class="tool-sep"></div>
        <template v-if="mode === 'edit'">
          <button type="button" class="tool-btn crumb" :class="{ active: focusId == null }" @click="focusId = null">{{ star?.name }}</button>
          <template v-if="focusBody">
            <span class="crumb-sep">›</span>
            <span class="tool-btn crumb active">{{ focusBody.name }}</span>
          </template>
        </template>
        <template v-else>
        <button type="button" class="tool-btn" :title="playing ? $t('Pause') : $t('Play')" @click="playing = !playing">
          {{ playing ? '❚❚' : '▶' }}
        </button>
        <button
          v-for="s in SPEEDS"
          :key="s"
          type="button"
          class="tool-btn"
          :class="{ active: speed === s }"
          :title="$tn('{n} day a second', '{n} days a second', s)"
          @click="speed = s"
        >×{{ s }}</button>
        <div class="tool-sep"></div>
        <button type="button" class="tool-btn" :title="$t('Back to day 0')" @click="days = 0">0</button>
        <button type="button" class="tool-btn" :title="$t('See the whole system')" @click="selectedId = null; sceneRef?.frame()">{{ $t('Fit') }}</button>
        <div class="tool-sep"></div>
        <span class="orbit-mode" role="radiogroup" :aria-label="$t('Orbits')">
          <button
            type="button"
            class="tool-btn"
            role="radio"
            :aria-checked="!live"
            :class="{ active: !live }"
            :title="$t('Each body on the orbit you set, always the same')"
            @click="live = false"
          >{{ $t('Static') }}</button>
          <button
            type="button"
            class="tool-btn"
            role="radio"
            :aria-checked="live"
            :class="{ active: live }"
            :title="$t('Every body pulls on every other: orbits bend as bodies pass each other, starting from the orbits you set on day 0')"
            @click="live = true"
          >{{ $t('Gravity') }}</button>
        </span>
        <span class="day-label">{{ $t('Day {n}', { n: Math.floor(days).toLocaleString() }) }}</span>
        <span v-if="live && lagging" class="lag" :title="$t('Too many steps to keep up at this speed; time runs slower')">{{ $t('catching up…') }}</span>
        </template>
      </div>

      <div v-if="unstable" class="unstable glass-panel panel-solid" role="alert">
        <strong>{{ $t('The orbits are not stable') }}</strong>
        <p>{{ unstableText }}</p>
        <div class="unstable-actions">
          <button type="button" class="btn btn-primary small" @click="restart">{{ $t('Back to day 0') }}</button>
          <button type="button" class="btn btn-ghost small" @click="dismissUnstable">{{ $t('Dismiss') }}</button>
        </div>
      </div>

      <p v-if="loadError" class="error-banner floating">{{ loadError }}</p>
      <div v-else-if="ready && !bodies.length" class="empty glass-panel panel-solid">
        <h2>{{ $t('Solar system') }}</h2>
        <p>{{ $t('Start with the star your world turns round. Then add planets, and moons round them, and pick which planet is the one on the map.') }}</p>
        <button type="button" class="btn btn-primary" :disabled="busy" @click="addStar">{{ $t('Add a star') }}</button>
      </div>

      <aside v-if="ready && bodies.length" class="system-panel glass-panel panel-solid" :class="{ 'is-closed': panelClosed }">
        <header class="panel-head">
          <button type="button" class="fold" :title="panelClosed ? $t('Show the list') : $t('Hide the list')" @click="panelClosed = !panelClosed">
            {{ panelClosed ? '‹' : '›' }}
          </button>
          <strong v-if="!panelClosed">{{ $t('Solar system') }}</strong>
          <RouterLink v-if="!panelClosed && systemName" :to="wikiPath('solar_system', 1)" class="wiki-link" :title="$t('Its wiki article')">{{ $t('Wiki') }}</RouterLink>
        </header>

        <template v-if="!panelClosed">
          <input
            v-model="systemName"
            class="system-name"
            type="text"
            maxlength="80"
            :placeholder="$t('Name the system')"
            :aria-label="$t('The name of the solar system')"
            @change="saveName"
            @keydown.enter="$event.target.blur()"
          />
          <ul class="tree">
            <li v-for="row in rows" :key="row.body.id" class="tree-item">
              <button
                type="button"
                class="tree-row"
                :class="{ active: row.body.id === selectedId }"
                :style="{ paddingLeft: 0.4 + row.depth * 1.1 + 'rem' }"
                @click="pick(row.body.id)"
              >
                <span class="dot" :style="{ background: bodyColor(row.body) }"></span>
                <span class="name">{{ row.body.name }}</span>
                <span v-if="row.body.is_main" class="main-badge" :title="$t('The planet of the map')">{{ $t('Main') }}</span>
              </button>
              <span v-if="row.body.kind !== 'star'" class="order">
                <button type="button" class="icon-btn" :disabled="busy || row.first" :aria-label="$t('Move closer in')" :title="$t('Swap orbits with the one closer in')" @click="swapOrbit(row.body, -1)">▲</button>
                <button type="button" class="icon-btn" :disabled="busy || row.last" :aria-label="$t('Move farther out')" :title="$t('Swap orbits with the one farther out')" @click="swapOrbit(row.body, 1)">▼</button>
              </span>
            </li>
          </ul>
          <div class="add-row">
            <button type="button" class="btn btn-ghost small" :disabled="busy || !star" @click="addPlanet">+ {{ $t('Planet') }}</button>
            <button
              type="button"
              class="btn btn-ghost small"
              :disabled="busy || !planetForMoon"
              :title="planetForMoon ? $t('A moon round {name}', { name: planetForMoon.name }) : $t('Pick a planet first')"
              @click="addMoon"
            >+ {{ $t('Moon') }}</button>
          </div>

          <form v-if="draft" class="body-form" @submit.prevent="save">
            <label class="field">
              <span>{{ $t('Name') }}</span>
              <input v-model="draft.name" type="text" maxlength="80" required />
            </label>
            <StarFields v-if="draft.kind === 'star'" v-model="draft" />
            <div class="field-row">
              <label v-if="draft.kind !== 'star'" class="field">
                <span>{{ $t('Type') }}</span>
                <select v-model="draft.body_type">
                  <option v-for="t in typesFor(draft.kind)" :key="t" :value="t">{{ $t(TYPE_LABELS[t]) }}</option>
                </select>
              </label>
              <label class="field color-field">
                <span>{{ $t('Colour') }}</span>
                <input :value="draft.color || bodyColor(draft)" type="color" @input="draft.color = $event.target.value" />
              </label>
            </div>
            <label v-if="draft.kind !== 'star'" class="field">
              <span>{{ $t('Radius (km)') }}</span>
              <input v-model.number="draft.radius_km" type="number" min="0.001" step="any" :disabled="draft.is_main" />
              <small v-if="draft.is_main" class="hint">
                {{ $t("The main planet's size is the map's.") }}
                <router-link to="/settings#map">{{ $t('Change it in Settings') }}</router-link>
              </small>
              <small v-else-if="draft.radius_km > 0" class="hint">{{ radiusHint(draft) }}</small>
            </label>
            <template v-if="draft.kind !== 'star'">
              <label class="field">
                <span>{{ $t('Orbit radius (km)') }}</span>
                <input v-model.number="draft.orbit_km" type="number" min="0.001" step="any" />
                <small v-if="draft.orbit_km > 0" class="hint">{{ orbitHint(draft) }}</small>
              </label>
              <label class="field">
                <span>{{ $t('Starting angle (°)') }}</span>
                <input v-model.number="draft.phase" type="number" step="any" />
                <small class="hint">{{ $t('Where on its orbit it is on day 0.') }}</small>
              </label>
              <label class="field">
                <span>{{ $t('Eccentricity') }}</span>
                <div class="ecc">
                  <input v-model.number="draft.eccentricity" type="range" min="0" max="0.95" step="0.01" />
                  <input v-model.number="draft.eccentricity" type="number" min="0" max="0.95" step="0.01" class="ecc-num" />
                </div>
                <small class="hint">{{ $t('0 is a circle; the higher, the more stretched, coming closer and going farther.') }}</small>
              </label>
              <label v-if="draft.eccentricity > 0" class="field">
                <span>{{ $t('Closest point at (°)') }}</span>
                <input v-model.number="draft.periapsis" type="number" step="any" />
                <small class="hint">{{ $t('The direction where the orbit comes closest.') }}</small>
              </label>
            </template>
            <button
              v-if="draft.kind === 'planet' && mode === 'edit' && focusId !== draft.id"
              type="button"
              class="btn btn-ghost small moons-btn"
              @click="focusId = draft.id"
            >{{ $t('Edit its moons') }} ›</button>
            <label v-if="draft.kind === 'planet'" class="check-field">
              <input v-model="draft.is_main" type="checkbox" />
              <span>{{ $t('The main planet: the one on the map') }}</span>
            </label>

            <dl v-if="facts" class="facts">
              <dt>{{ $t('Mass') }}</dt>
              <dd>{{ facts.mass }}</dd>
              <dt>{{ $t('Density') }}</dt>
              <dd>{{ facts.density }}</dd>
              <dt>{{ $t('Surface gravity') }}</dt>
              <dd>{{ facts.gravity }}</dd>
              <dt>{{ $t('Escape velocity') }}</dt>
              <dd>{{ facts.escape }}</dd>
              <template v-if="facts.light">
                <dt>{{ $t('Brightness') }}</dt>
                <dd>{{ facts.light }}</dd>
                <dt>{{ $t('Habitable zone') }}</dt>
                <dd>{{ facts.zone }}</dd>
              </template>
              <template v-if="facts.period">
                <dt>{{ draft.kind === 'moon' ? $t('One orbit') : $t('Year') }}</dt>
                <dd>{{ facts.period }}</dd>
              </template>
              <template v-if="facts.closest">
                <dt>{{ $t('Closest') }}</dt>
                <dd>{{ facts.closest }}</dd>
                <dt>{{ $t('Farthest') }}</dt>
                <dd>{{ facts.farthest }}</dd>
              </template>
            </dl>
            <p v-if="problem" class="problem">{{ problem }}</p>

            <p v-if="saveError" class="error-banner">{{ saveError }}</p>
            <RouterLink :to="wikiPath('celestial_body', draft.id)" class="wiki-link">{{ $t('Wiki article') }} ›</RouterLink>
            <div class="form-actions">
              <button type="submit" class="btn btn-primary small" :disabled="busy || !dirty">{{ $t('Save') }}</button>
              <button type="button" class="btn btn-ghost small danger-text" :disabled="busy" @click="confirmDelete = true">{{ $t('Delete') }}</button>
            </div>
          </form>
        </template>
      </aside>
    </div>

    <ConfirmDialog
      v-if="confirmDelete && draft"
      :title="$t('Delete {name}?', { name: draft.name })"
      :message="$t('Everything that orbits it is deleted with it.')"
      :confirm-label="$t('Delete')"
      @cancel="confirmDelete = false"
      @confirm="remove"
    />
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { t, tn, tr } from '../i18n'
import { solarApi, planetApi } from '../api'
import { skyUrl, applyMapSettings } from '../worldMap'
import { planetColorOf } from '../planetColors'
import { bodyFacts, orbitProblem, systemTree, periapsisOf, apoapsisOf, createGravitySim, positionsAt } from '../orbits'
import { useUnsavedGuard } from '../navigation'
import { wikiPath } from '../wiki'
import SolarScene from '../components/SolarScene.vue'
import SolarEditor2D from '../components/SolarEditor2D.vue'
import StarFields from '../components/StarFields.vue'
import { bodyColor } from '../solarScale'
import { luminosity, habitableZone, SUN } from '../stars'
import ConfirmDialog from '../components/ConfirmDialog.vue'

// The Solar system tab: a star, its planets and their moons, each sized and
// set on an orbit, turning as the clock runs. The physics is orbits.js, the
// picture SolarScene.vue; this page keeps the list, the form and the clock.
// One planet is the main one and wears the map.

const SPEEDS = [1, 10, 100, 1000]
const TYPE_LABELS = { star: tr('Star'), rocky: tr('Rocky'), gas: tr('Gas giant'), ocean: tr('Ocean world') }
const EARTH_RADIUS = 6371
const SUN_RADIUS = 696000
const AU = 1.496e8
const EARTH_MASS = 5.972e24
const SUN_MASS = 1.989e30

const ready = ref(false)
const loadError = ref('')
const bodies = ref([])
const main = ref({ radius_km: 0, tilt: 0, globe: false })
const globe = ref(null) // { frequency, colorOf, version } for the main planet
const selectedId = ref(null)
const draft = ref(null)
const saved = ref('')
const busy = ref(false)
const saveError = ref('')
const confirmDelete = ref(false)
const panelClosed = ref(window.matchMedia('(max-width: 720px)').matches)
const sceneRef = ref(null)
// Edit is the flat editor; Run the 3D scene with the clock.
const mode = ref('edit')
// What the editor is centred on: null for the star, or a planet (its moons).
const focusId = ref(null)

const tree = computed(() => systemTree(bodies.value))
const star = computed(() => tree.value.star)
// What the views draw: the saved bodies, with the one being edited as it is
// in the form, so the markers follow the sliders before saving.
const liveBodies = computed(() => (draft.value ? bodies.value.map((b) => (b.id === draft.value.id ? draft.value : b)) : bodies.value))
const focusBody = computed(() => tree.value.byId.get(focusId.value) ?? null)

function setMode(m) {
  mode.value = m
  if (m === 'edit') playing.value = false
}

// The list, as a tree: the star, each planet, and its moons under it.
const rows = computed(() => {
  const out = []
  const walk = (b, depth, first = true, last = true) => {
    out.push({ body: b, depth, first, last })
    const kids = tree.value.childrenOf(b.id)
    kids.forEach((c, i) => walk(c, depth + 1, i === 0, i === kids.length - 1))
  }
  for (const root of tree.value.childrenOf(null)) walk(root, 0)
  return out
})

// A new moon goes round the picked planet, or the picked moon's planet.
const planetForMoon = computed(() => {
  const b = tree.value.byId.get(selectedId.value)
  if (!b) return null
  if (b.kind === 'planet') return b
  if (b.kind === 'moon') return tree.value.byId.get(b.parent_id) ?? null
  return null
})

const typesFor = (kind) => (kind === 'star' ? ['star'] : ['rocky', 'gas', 'ocean'])

// The markers shown in both views, remembered on this device.
const MARKERS_KEY = 'polaris-solar-markers'
const markers = ref({ zone: true, limits: true })
try {
  Object.assign(markers.value, JSON.parse(localStorage.getItem(MARKERS_KEY) || '{}'))
} catch {
  // Defaults.
}
function toggleMarker(key) {
  markers.value = { ...markers.value, [key]: !markers.value[key] }
  try {
    localStorage.setItem(MARKERS_KEY, JSON.stringify(markers.value))
  } catch {
    // Not remembered.
  }
}

// ---- Loading -------------------------------------------------------------------------
// The system's name: saved as soon as it is changed. Empty, the system has
// no name (and no wiki article).
const systemName = ref('')
async function saveName() {
  await run(() => solarApi.setName(systemName.value))
}

function take(system) {
  systemName.value = system.name || ''
  bodies.value = system.bodies
  main.value = system.main
  if (selectedId.value != null && !tree.value.byId.has(selectedId.value)) selectedId.value = null
  if (focusId.value != null && !tree.value.byId.has(focusId.value)) focusId.value = null
  resetDraft()
}

async function loadGlobe() {
  globe.value = null
  if (!main.value.globe || !bodies.value.some((b) => b.is_main)) return
  try {
    const data = await planetApi.get()
    applyMapSettings(data.settings)
    globe.value = { frequency: data.settings.planet.frequency, colorOf: planetColorOf(data), version: 0 }
  } catch {
    // The planet is drawn plain instead.
  }
}

onMounted(async () => {
  try {
    take(await solarApi.get())
    ready.value = true
    loadGlobe()
  } catch (err) {
    loadError.value = t("Couldn't load the solar system. Try refreshing.")
  }
})

// ---- The clock -------------------------------------------------------------------------
const days = ref(0)
const playing = ref(false)
const speed = ref(10)
let raf = 0
let last = 0
function tick(now) {
  if (last) {
    const next = days.value + ((now - last) / 1000) * speed.value
    if (live.value && sim) {
      lagging.value = !sim.advanceTo(next) && !sim.event
      days.value = sim.days
      if (sim.event) stopFor(sim.event)
      else if (now - shapesAt > 200) updateShapes(now)
    } else {
      days.value = next
      const hit = touchingNow()
      if (hit) stopFor(hit)
    }
  }
  last = now
  if (playing.value) raf = requestAnimationFrame(tick)
}

// ---- Instability ---------------------------------------------------------------------------
// When bodies touch, or (with gravity) one flies off, the clock stops and
// says so. Static orbits can't fly off, but crossing ones can meet.
const unstable = ref(null) // { type, a, b } or { type, id, from }, with days
const dismissedPairs = new Set() // static meetings already shown
const nameOf = (id) => tree.value.byId.get(id)?.name ?? '?'
const unstableText = computed(() => {
  const u = unstable.value
  if (!u) return ''
  const day = Math.floor(u.days).toLocaleString()
  return u.type === 'collision'
    ? t('{a} and {b} collided on day {day}.', { a: nameOf(u.a), b: nameOf(u.b), day })
    : t('{a} flew off from {b} on day {day}: its gravity no longer holds it.', { a: nameOf(u.id), b: nameOf(u.from), day })
})

function stopFor(event) {
  playing.value = false
  unstable.value = event
}

// With static orbits: two bodies that overlap where they are now.
function touchingNow() {
  const list = liveBodies.value
  const pos = positionsAt(list, days.value)
  for (let i = 0; i < list.length; i++) {
    for (let j = i + 1; j < list.length; j++) {
      const [a, b] = [list[i], list[j]]
      const key = `${a.id}:${b.id}`
      if (dismissedPairs.has(key)) continue
      const pa = pos.get(a.id)
      const pb = pos.get(b.id)
      if (Math.hypot(pa[0] - pb[0], pa[1] - pb[1], pa[2] - pb[2]) < a.radius_km + b.radius_km) {
        return { type: 'collision', a: a.id, b: b.id, days: days.value, key }
      }
    }
  }
  return null
}

function dismissUnstable() {
  if (unstable.value?.key) dismissedPairs.add(unstable.value.key)
  unstable.value = null
}

function restart() {
  unstable.value = null
  dismissedPairs.clear()
  days.value = 0
  sim?.reset()
  updateShapes()
}

// ---- Gravity --------------------------------------------------------------------------------
// Static: every body on the orbit it was given. Gravity: orbits.js steps
// every body by the pull of all the others, from where the orbits put them
// on day 0, so orbits bend as bodies pass. Made again when the bodies
// change. The orbit lines then show the orbit each is on now.
const live = ref(false)
const lagging = ref(false)
let sim = null
const simRelative = (id) => sim?.relative(id) ?? [0, 0, 0]
const shapes = ref(new Map())
let shapesAt = 0
function updateShapes(now = performance.now()) {
  shapesAt = now
  const out = new Map()
  if (sim) {
    for (const b of liveBodies.value) {
      const o = sim.orbitNow(b.id)
      if (o?.bound) out.set(b.id, { orbit_km: o.orbit_km, eccentricity: Math.min(o.eccentricity, 0.95), periapsis: o.periapsis })
    }
  }
  shapes.value = out
}
function makeSim() {
  unstable.value = null
  sim = live.value ? createGravitySim(liveBodies.value) : null
  if (sim) {
    sim.advanceTo(days.value)
    if (sim.event) stopFor(sim.event)
  }
  lagging.value = false
  updateShapes()
}
watch(live, makeSim)
watch(liveBodies, () => live.value && makeSim(), { deep: true })
// Going back (day 0) starts again; the clock moves it on otherwise.
watch(days, (d) => {
  if (sim && d < sim.days) {
    unstable.value = null
    sim.advanceTo(d)
    updateShapes()
  }
})
watch(playing, (on) => {
  cancelAnimationFrame(raf)
  last = 0
  if (on) raf = requestAnimationFrame(tick)
})
onBeforeUnmount(() => cancelAnimationFrame(raf))

// ---- Editing ----------------------------------------------------------------------------
// A body with no colour of its own shows its type's.
const plain = (b) => ({ ...b, color: b.color || '', orbit_km: b.orbit_km ?? null, phase: b.phase ?? 0 })

function resetDraft() {
  const b = tree.value.byId.get(selectedId.value)
  draft.value = b ? plain(b) : null
  saved.value = draft.value ? JSON.stringify(draft.value) : ''
  saveError.value = ''
}

const dirty = computed(() => !!draft.value && JSON.stringify(draft.value) !== saved.value)
useUnsavedGuard(() => dirty.value)

// Picking a moon shows its planet's moons; picking another planet or the
// star goes back to the whole system.
function pick(id) {
  selectedId.value = id
  const b = tree.value.byId.get(id)
  if (b?.kind === 'moon') focusId.value = b.parent_id
  else if (b && b.id !== focusId.value) focusId.value = null
  if (panelClosed.value && id != null) panelClosed.value = false
}
watch(selectedId, resetDraft)

async function run(call) {
  busy.value = true
  saveError.value = ''
  try {
    const hadMain = bodies.value.find((b) => b.is_main)?.id
    take(await call())
    if (bodies.value.find((b) => b.is_main)?.id !== hadMain) loadGlobe()
    return true
  } catch (err) {
    saveError.value = err.message || t("Couldn't save. Try again.")
    return false
  } finally {
    busy.value = false
  }
}

// New bodies are saved straight away with sensible sizes, then picked.
async function addBody(body) {
  const before = new Set(bodies.value.map((b) => b.id))
  if (await run(() => solarApi.create(body))) {
    pick(bodies.value.find((b) => !before.has(b.id))?.id ?? null)
  }
}

function addStar() {
  addBody({ kind: 'star', name: t('Sun'), body_type: 'star', star_class: 'yellow_dwarf', ...SUN, free_values: 0 })
}

function addPlanet() {
  const planets = tree.value.childrenOf(star.value.id)
  const farthest = Math.max(0, ...planets.map((p) => p.orbit_km || 0))
  addBody({
    kind: 'planet',
    parent_id: star.value.id,
    name: t('Planet {n}', { n: planets.length + 1 }),
    body_type: 'rocky',
    radius_km: EARTH_RADIUS,
    orbit_km: farthest ? Math.round(farthest * 1.6) : AU,
    phase: Math.round(Math.random() * 360),
    // The first planet is the map's.
    is_main: !bodies.value.some((b) => b.is_main),
  })
}

function addMoon() {
  const planet = planetForMoon.value
  focusId.value = planet.id
  const moons = tree.value.childrenOf(planet.id)
  const farthest = Math.max(0, ...moons.map((m) => m.orbit_km || 0))
  addBody({
    kind: 'moon',
    parent_id: planet.id,
    name: t('{planet} {n}', { planet: planet.name, n: ['I', 'II', 'III', 'IV', 'V', 'VI', 'VII', 'VIII'][moons.length] ?? moons.length + 1 }),
    body_type: 'rocky',
    radius_km: Math.round(planet.radius_km * 0.27),
    orbit_km: farthest ? Math.round(farthest * 1.5) : Math.round(planet.radius_km * 60),
    phase: Math.round(Math.random() * 360),
  })
}

// ---- Moving bodies -----------------------------------------------------------------------
// The 2D editor moves a body as it is dragged, and saves it when let go.
function onMove({ id, orbit_km, phase }) {
  const b = tree.value.byId.get(id)
  if (!b) return
  b.orbit_km = orbit_km
  b.phase = phase
  if (draft.value?.id === id) Object.assign(draft.value, { orbit_km, phase })
}

function onCommit(id) {
  if (draft.value?.id === id) return save()
  const b = tree.value.byId.get(id)
  if (b) run(() => solarApi.update(id, b))
}

// Swaps a body's orbit with its neighbour's (closer in, -1, or farther
// out, 1): the way to change the order. Each keeps its own angle.
async function swapOrbit(b, dir) {
  const siblings = tree.value.childrenOf(b.parent_id)
  const other = siblings[siblings.indexOf(b) + dir]
  if (!other) return
  const [mine, theirs] = [b.orbit_km, other.orbit_km]
  const edited = draft.value?.id === b.id ? draft.value : draft.value?.id === other.id ? draft.value : null
  await run(async () => {
    await solarApi.update(b.id, { ...(edited?.id === b.id ? edited : b), orbit_km: theirs })
    return solarApi.update(other.id, { ...(edited?.id === other.id ? edited : other), orbit_km: mine })
  })
}

function save() {
  const d = draft.value
  run(() => solarApi.update(d.id, d))
}

async function remove() {
  confirmDelete.value = false
  const id = draft.value.id
  if (await run(() => solarApi.remove(id))) selectedId.value = null
}

// ---- What the form says about the body ---------------------------------------------------
// Three significant figures; tiny values (a red giant's density) in
// scientific notation rather than rounded to 0.
const num = (x, digits = 3) => (x !== 0 && Math.abs(x) < 0.001 ? x.toExponential(digits - 1) : Number(x.toPrecision(digits)).toLocaleString())

function radiusHint(b) {
  return b.kind === 'star'
    ? t('{n} times the Sun', { n: num(b.radius_km / SUN_RADIUS) })
    : t('{n} times Earth', { n: num(b.radius_km / EARTH_RADIUS) })
}

function orbitHint(b) {
  return b.kind === 'moon'
    ? t('{n} times its planet\'s radius', { n: num(b.orbit_km / (tree.value.byId.get(b.parent_id)?.radius_km || 1)) })
    : t('{n} AU (Earth to the Sun is 1)', { n: num(b.orbit_km / AU) })
}

const facts = computed(() => {
  const b = draft.value
  if (!b || !(b.radius_km > 0)) return null
  const parent = tree.value.byId.get(b.parent_id)
  const f = bodyFacts(b, b.orbit_km > 0 ? parent : null)
  const g = f.gravity / 9.80665
  let period = ''
  if (f.period) {
    const years = f.period >= 365.25 * 2
    const n = Number((years ? f.period / 365.25 : f.period).toPrecision(3))
    period = years
      ? tn('{n} Earth year', '{n} Earth years', n, { n: n.toLocaleString() })
      : tn('{n} Earth day', '{n} Earth days', n, { n: n.toLocaleString() })
  }
  return {
    mass: b.kind === 'star'
      ? t('{n} Suns', { n: num(f.mass / SUN_MASS) })
      : t('{n} Earths', { n: num(f.mass / EARTH_MASS) }),
    ...starFacts(b),
    density: `${num(f.density / 1000)} g/cm³`,
    gravity: `${num(f.gravity)} m/s² (${num(g)} g)`,
    escape: `${num(f.escape)} km/s`,
    period,
    ...shapeFacts(b),
  }
})

// How close and how far an elliptical orbit goes.
function shapeFacts(b) {
  if (b.kind === 'star' || !(b.eccentricity > 0) || !(b.orbit_km > 0)) return {}
  const show = (km) => (b.kind === 'moon' ? t('{n} km', { n: num(km) }) : t('{n} AU', { n: num(km / AU) }))
  return { closest: show(periapsisOf(b)), farthest: show(apoapsisOf(b)) }
}

// A star's light, and where its habitable zone lies.
function starFacts(b) {
  if (b.kind !== 'star') return {}
  const zone = habitableZone(b)
  const l = luminosity(b)
  return {
    light: l > 0 ? t('{n} Suns', { n: num(l) }) : t('None'),
    zone: zone ? t('{from} to {to} AU', { from: num(zone.inner_km / AU), to: num(zone.outer_km / AU) }) : t('None'),
  }
}

const problem = computed(() => {
  const b = draft.value
  if (!b || b.kind === 'star' || !(b.orbit_km > 0)) return ''
  const parent = tree.value.byId.get(b.parent_id)
  if (!parent) return ''
  const why = orbitProblem(b, parent, b.kind === 'moon' ? tree.value.byId.get(parent.parent_id) : null)
  if (why === 'inside') return b.kind === 'planet' ? t('This orbit is inside the star.') : t('This orbit is inside its planet.')
  if (why === 'close') return t("This orbit is too close: inside the Roche limit, where the tides of what it orbits would tear it apart.")
  if (why === 'loose') {
    return b.kind === 'moon'
      ? t("This orbit is too far out for its planet's gravity to hold the moon; the star would pull it away.")
      : t("This orbit is too far out for the star's gravity to hold it against the rest of the galaxy.")
  }
  return ''
})
</script>

<style scoped src="../mapChrome.css"></style>
<style scoped>
.solar-page {
  background: #02030a;
}

.clock {
  flex-direction: row;
  align-items: center;
  flex-wrap: wrap;
  max-width: calc(100% - 32px);
}

.clock .tool-btn {
  padding: 0.35rem 0.55rem;
}

.tool-btn.active {
  color: var(--text-primary);
  border-color: var(--accent);
  background: var(--accent-soft);
}

.tool-sep {
  width: 1px;
  height: 1.4rem;
  background: var(--glass-border);
  margin: 0 2px;
}

.day-label {
  padding: 0 0.5rem;
  font-size: 0.78rem;
  color: var(--text-muted);
  font-variant-numeric: tabular-nums;
}

.floating {
  position: absolute;
  top: 16px;
  left: 50%;
  translate: -50% 0;
  z-index: 8;
}

.empty {
  position: absolute;
  top: 50%;
  left: 50%;
  translate: -50% -50%;
  max-width: 420px;
  padding: 1.5rem;
  text-align: center;
}

.empty h2 {
  font-family: 'Fraunces', serif;
  font-weight: 500;
  margin: 0 0 0.5rem;
}

.empty p {
  color: var(--text-muted);
  font-size: 0.88rem;
  line-height: 1.5;
}

.system-panel {
  position: absolute;
  top: 16px;
  right: 16px;
  bottom: 16px;
  width: 280px;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding: 0.55rem;
  overflow-y: auto;
  z-index: 7;
}

.system-panel.is-closed {
  bottom: auto;
  width: auto;
}

.panel-head {
  display: flex;
  align-items: center;
  gap: 0.45rem;
}

.panel-head strong {
  flex: 1;
  font-size: 0.82rem;
}

.fold {
  width: 1.8rem;
  height: 1.8rem;
  border: 1px solid var(--glass-border);
  border-radius: 8px;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
}

.tree {
  list-style: none;
  margin: 0;
  padding: 0;
}

.system-name {
  font-family: 'Fraunces', serif;
  font-size: 1rem;
}

.wiki-link {
  font-size: 0.76rem;
  color: var(--accent);
  align-self: flex-start;
}

.ecc {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.ecc input[type='range'] {
  flex: 1;
  accent-color: var(--accent);
}

.ecc-num {
  width: 4.5rem;
}

.orbit-mode {
  display: inline-flex;
  gap: 2px;
}

.unstable {
  position: absolute;
  top: 76px;
  left: 50%;
  translate: -50% 0;
  max-width: min(460px, calc(100% - 32px));
  padding: 0.85rem 1rem;
  border-color: var(--danger);
  z-index: 8;
}

.unstable strong {
  color: var(--danger);
}

.unstable p {
  margin: 0.35rem 0 0.7rem;
  font-size: 0.85rem;
  line-height: 1.45;
}

.unstable-actions {
  display: flex;
  gap: 0.5rem;
}

.lag {
  font-size: 0.72rem;
  color: #e0a04a;
}

.marker-btn.zone.active {
  border-color: #4caf6e;
  background: rgba(76, 175, 110, 0.18);
}

.marker-btn.limits.active {
  border-color: #e0864a;
  background: rgba(224, 134, 74, 0.18);
}

.tree-item {
  display: flex;
  align-items: center;
  gap: 0.2rem;
}

.order {
  display: flex;
  flex-shrink: 0;
}

.icon-btn {
  width: 1.5rem;
  height: 1.5rem;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--text-faint);
  font-size: 0.6rem;
  cursor: pointer;
}

.icon-btn:hover:not(:disabled) {
  color: var(--text-primary);
  background: var(--accent-soft);
}

.icon-btn:disabled {
  opacity: 0.3;
  cursor: default;
}

.crumb {
  max-width: 9rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.crumb-sep {
  color: var(--text-faint);
  font-size: 0.8rem;
}

.moons-btn {
  align-self: flex-start;
  margin: 0.2rem 0;
}

.tree-row {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  width: 100%;
  padding: 0.35rem 0.4rem;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  color: var(--text-primary);
  font: inherit;
  font-size: 0.84rem;
  text-align: left;
  cursor: pointer;
}

.tree-row.active {
  border-color: var(--accent);
  background: var(--accent-soft);
}

.dot {
  width: 0.7rem;
  height: 0.7rem;
  border-radius: 50%;
  flex-shrink: 0;
}

.name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.main-badge {
  font-size: 0.64rem;
  font-weight: 700;
  color: var(--accent);
  border: 1px solid var(--accent);
  border-radius: 999px;
  padding: 0 0.4rem;
}

.add-row {
  display: flex;
  gap: 0.4rem;
}

.body-form {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
  border-top: 1px solid var(--glass-border);
  padding-top: 0.6rem;
}

.color-field input {
  height: var(--control-h);
  padding: 0.15rem;
}

.hint {
  font-size: 0.72rem;
  color: var(--text-faint);
  line-height: 1.4;
}

.check-field {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  font-size: 0.82rem;
  margin: 0.3rem 0;
}

.facts {
  display: grid;
  grid-template-columns: max-content 1fr;
  gap: 0.25rem 0.75rem;
  margin: 0.5rem 0;
  font-size: 0.78rem;
}

.facts dt {
  color: var(--text-muted);
}

.facts dd {
  margin: 0;
  color: var(--text-primary);
  font-variant-numeric: tabular-nums;
}

.problem {
  margin: 0.25rem 0;
  font-size: 0.78rem;
  color: var(--danger);
  line-height: 1.45;
}

.form-actions {
  display: flex;
  gap: 0.5rem;
  margin-top: 0.4rem;
}

/* Phones: the panel sits along the bottom, the clock stays on top. */
@media (max-width: 720px) {
  .system-panel {
    top: auto;
    right: 8px;
    bottom: 8px;
    left: 8px;
    width: auto;
    max-height: 55%;
  }
  .system-panel.is-closed {
    left: auto;
    bottom: 8px;
  }
  .clock {
    top: 8px;
    left: 8px;
  }
}
</style>
