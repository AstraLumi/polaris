<template>
  <div class="map-host">
    <BodyMap v-if="ready && otherBody" :key="otherBody.id" :body-id="otherBody.id" :name="otherBody.name" />
    <PlanetMap v-else-if="ready && worldMap.mode === '3d'" />
    <MapView v-else-if="ready" />
    <p v-else class="loading-hint">{{ $t('Loading…') }}</p>
  </div>
</template>

<script setup>
import { ref, computed, defineAsyncComponent, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { worldMap } from '../worldMap'
import { calendar } from '../calendar'
import { solarApi } from '../api'
import { systemTree } from '../orbits'
import MapView from './MapView.vue'

// The planet (and three.js with it) loads only for stories that use it.
const PlanetMap = defineAsyncComponent(() => import('./PlanetMap.vue'))
const BodyMap = defineAsyncComponent(() => import('./BodyMap.vue'))

// The Map page: the story's map (the flat hex map when the story is 2D only,
// the planet when it is 3D, Settings → Map), which is the main planet's.
// With a solar system, the switch under the side list (PlanetSwitch.vue)
// opens the other planets' and moons' own maps (BodyMap.vue), which are
// painting only. The map always opens on the main planet; ?body= picks
// another.
const ready = computed(() => worldMap.loaded || calendar.loaded)
const route = useRoute()
const bodies = ref([])

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
const otherBody = computed(() => others.value.find((b) => String(b.id) === String(route.query.body)) ?? null)
</script>

<style scoped>
.map-host {
  position: relative;
}
</style>
