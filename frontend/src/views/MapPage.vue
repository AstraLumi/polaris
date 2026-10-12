<template>
  <PlanetMap v-if="ready && worldMap.mode === '3d'" />
  <MapView v-else-if="ready" />
  <p v-else class="loading-hint">{{ $t('Loading…') }}</p>
</template>

<script setup>
import { computed, defineAsyncComponent } from 'vue'
import { worldMap } from '../worldMap'
import { calendar } from '../calendar'
import MapView from './MapView.vue'

// The planet (and three.js with it) loads only for stories that use it.
const PlanetMap = defineAsyncComponent(() => import('./PlanetMap.vue'))

// The Map page: the flat hex map when the story is 2D only, the planet when
// it is 3D (Settings → Map). Until the settings arrive (or if they can't),
// it waits, then falls back to 2D.
const ready = computed(() => worldMap.loaded || calendar.loaded)
</script>
