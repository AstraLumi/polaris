<template>
  <div class="map-host">
    <BodyMap v-if="ready && otherBody" :key="otherBody.id" :body-id="otherBody.id" :name="otherBody.name" />
    <PlanetMap v-else-if="ready && worldMap.mode === '3d'" />
    <MapView v-else-if="ready" />
    <p v-else class="loading-hint">{{ $t('Loading…') }}</p>

    <!-- Which planet's map: the main one unless another is picked. -->
    <div v-if="ready && others.length" class="planet-switch">
      <button type="button" class="switch-btn glass-panel panel-solid" :aria-expanded="open" @click="open = !open">
        <span class="globe-dot" :style="{ background: current.color }"></span>
        {{ current.name }}
        <span class="caret">▾</span>
      </button>
      <ul v-if="open" class="switch-list glass-panel panel-solid">
        <li>
          <button type="button" :class="{ active: !otherBody }" @click="pick(null)">
            <span class="globe-dot" :style="{ background: mainEntry.color }"></span>
            {{ mainEntry.name }}
            <span class="main-tag">{{ $t('Main') }}</span>
          </button>
        </li>
        <li v-for="b in others" :key="b.id">
          <button type="button" :class="{ active: otherBody?.id === b.id, 'is-moon': b.kind === 'moon' }" @click="pick(b.id)">
            <span class="globe-dot" :style="{ background: bodyColor(b) }"></span>
            {{ b.name }}
          </button>
        </li>
      </ul>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, defineAsyncComponent, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { t } from '../i18n'
import { worldMap } from '../worldMap'
import { calendar } from '../calendar'
import { solarApi } from '../api'
import { systemTree } from '../orbits'
import { bodyColor } from '../solarScale'
import MapView from './MapView.vue'

// The planet (and three.js with it) loads only for stories that use it.
const PlanetMap = defineAsyncComponent(() => import('./PlanetMap.vue'))
const BodyMap = defineAsyncComponent(() => import('./BodyMap.vue'))

// The Map page: the story's map (the flat hex map when the story is 2D only,
// the planet when it is 3D, Settings → Map), which is the main planet's.
// With a solar system, a switch at the top opens the other planets' and
// moons' own maps (BodyMap.vue), which are painting only. The map always
// opens on the main planet; ?body= picks another.
const ready = computed(() => worldMap.loaded || calendar.loaded)
const route = useRoute()
const router = useRouter()
const bodies = ref([])
const open = ref(false)

onMounted(async () => {
  try {
    bodies.value = (await solarApi.get()).bodies
  } catch {
    // No switch; the story's map is all there is.
  }
})

// Planets and moons other than the main one, in the system's order.
const others = computed(() => {
  const tree = systemTree(bodies.value)
  const out = []
  const walk = (b) => {
    if (b.kind !== 'star' && !b.is_main) out.push(b)
    for (const c of tree.childrenOf(b.id)) walk(c)
  }
  for (const root of tree.childrenOf(null)) walk(root)
  return out
})
const mainBody = computed(() => bodies.value.find((b) => b.is_main) ?? null)
const mainEntry = computed(() => ({
  name: mainBody.value?.name || t('The map'),
  color: mainBody.value ? bodyColor(mainBody.value) : '#6cc3b0',
}))
const otherBody = computed(() => others.value.find((b) => String(b.id) === String(route.query.body)) ?? null)
const current = computed(() => (otherBody.value ? { name: otherBody.value.name, color: bodyColor(otherBody.value) } : mainEntry.value))

function pick(id) {
  open.value = false
  const query = { ...route.query }
  if (id == null) delete query.body
  else query.body = String(id)
  router.replace({ query })
}
</script>

<style scoped>
.map-host {
  position: relative;
}

.planet-switch {
  position: absolute;
  top: 16px;
  left: 50%;
  translate: -50% 0;
  z-index: 9;
}

.switch-btn {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  padding: 0.4rem 0.8rem;
  border-radius: 999px;
  color: var(--text-primary);
  font: inherit;
  font-size: 0.82rem;
  cursor: pointer;
}

.caret {
  color: var(--text-faint);
  font-size: 0.7rem;
}

.globe-dot {
  width: 0.7rem;
  height: 0.7rem;
  border-radius: 50%;
  flex-shrink: 0;
}

.switch-list {
  list-style: none;
  margin: 0.35rem 0 0;
  padding: 0.3rem;
  min-width: 12rem;
  max-height: 60vh;
  overflow-y: auto;
}

.switch-list button {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  width: 100%;
  padding: 0.35rem 0.5rem;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--text-primary);
  font: inherit;
  font-size: 0.82rem;
  text-align: left;
  cursor: pointer;
}

.switch-list button.is-moon {
  padding-left: 1.4rem;
}

.switch-list button.active,
.switch-list button:hover {
  background: var(--accent-soft);
}

.main-tag {
  margin-left: auto;
  font-size: 0.64rem;
  font-weight: 700;
  color: var(--accent);
}

/* Phones: in the top right corner, clear of the toolbar. */
@media (max-width: 720px) {
  .planet-switch {
    top: 8px;
    left: auto;
    right: 8px;
    translate: none;
  }
  .switch-list {
    position: absolute;
    right: 0;
  }
}
</style>
