<template>
  <div v-if="others.length" class="planet-switch">
    <span class="switch-title">{{ $t('Planet') }}</span>
    <select :value="currentId" :aria-label="$t('Which planet\'s map')" @change="pick($event.target.value)">
      <option value="">{{ mainName }}</option>
      <option v-for="b in others" :key="b.id" :value="String(b.id)">{{ b.kind === 'moon' ? '↳ ' : '' }}{{ b.name }}</option>
    </select>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { t } from '../i18n'
import { solarApi } from '../api'
import { systemTree } from '../orbits'

// Which planet's map the Map page shows: the main planet's (the story's
// map) unless another is picked, as ?body= (MapPage.vue). Sits under the
// map's side list; shown only when the solar system has other planets.
const route = useRoute()
const router = useRouter()
const bodies = ref([])

onMounted(async () => {
  try {
    bodies.value = (await solarApi.get()).bodies
  } catch {
    // No switch.
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
const mainName = computed(() => {
  const main = bodies.value.find((b) => b.is_main)
  return main ? t('{name} (main)', { name: main.name }) : t('The map')
})
const currentId = computed(() => (others.value.some((b) => String(b.id) === String(route.query.body)) ? String(route.query.body) : ''))

function pick(id) {
  const query = { ...route.query }
  if (!id) delete query.body
  else query.body = id
  delete query.focus
  router.replace({ query })
}
</script>

<style scoped>
.planet-switch {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.switch-title {
  font-size: 0.68rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-faint);
}

select {
  min-height: 1.9rem;
  font-size: 0.8rem;
}
</style>
