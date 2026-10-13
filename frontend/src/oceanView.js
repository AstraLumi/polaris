import { reactive, watch } from 'vue'

// How the maps (MapView.vue, PlanetMap.vue) draw oceans, kept in this
// browser: shown at all (their names, outlines and tint), their water
// tinted with their colour, and their edge outlined.

const KEY = 'polaris-ocean-view'

function read() {
  try {
    return JSON.parse(localStorage.getItem(KEY)) || {}
  } catch {
    return {}
  }
}

const saved = read()
export const oceanView = reactive({
  shown: saved.shown !== false,
  tint: saved.tint === true,
  outline: saved.outline !== false,
})

watch(oceanView, (v) => {
  try {
    localStorage.setItem(KEY, JSON.stringify(v))
  } catch {
    // Not remembered; the maps still follow it until the page reloads.
  }
})
