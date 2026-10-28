<template>
  <div v-if="show" class="offline-banner" role="status">
    <span>{{ $t('No connection to the server. Only books kept for writing offline can be opened; the rest of Polaris needs the connection.') }}</span>
    <span v-if="books.length" class="kept">
      <RouterLink v-for="b in books" :key="b.volume" :to="`/write/${b.volume}`" class="kept-link">{{ b.volume === '0' ? $t('Chapters without a volume') : b.title || $t('Book') }}</RouterLink>
    </span>
  </div>
</template>

<script setup>
// The rest of the app with no connection (docs/EDITOR.md, "Offline
// writing"): it says so, with links to the books kept for writing offline.
// The writing page shows its own state instead.
import { ref, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import { connected } from '../offline/connection.js'
import { offlineStore } from '../offline/index.js'
import { activeStoryId, stories, currentStory } from '../stories'

const route = useRoute()
const books = ref([])
const show = computed(() => !connected.value && route.name !== 'write')

// The story's name as a kept book remembers it, while the list of stories
// can't be loaded (the sidebar shows it).
watch(
  connected,
  async (on) => {
    if (on || !activeStoryId || currentStory.value) return
    const info = (await offlineStore.getStory(activeStoryId).catch(() => null))?.info
    if (info?.name && !currentStory.value) stories.value = [...stories.value, { id: activeStoryId, name: info.name, icon: '' }]
  },
  { immediate: true },
)
watch(
  show,
  async (on) => {
    if (!on || !activeStoryId) return
    try {
      books.value = await offlineStore.listBooks(activeStoryId)
    } catch (e) {
      books.value = []
    }
  },
  { immediate: true },
)
</script>

<style scoped>
.offline-banner {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0.4rem 0.9rem;
  margin: 0 0 1rem;
  padding: 0.6rem 0.9rem;
  border-radius: 10px;
  border: 1px solid color-mix(in srgb, var(--accent) 40%, transparent);
  background: var(--accent-soft);
  font-size: 0.86rem;
}

.kept {
  display: flex;
  flex-wrap: wrap;
  gap: 0.3rem 0.7rem;
}

.kept-link {
  color: var(--accent);
  font-weight: 600;
}
</style>
