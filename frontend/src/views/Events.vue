<template>
  <div class="page">
    <header class="page-header">
      <div>
        <h1>{{ $t('Events') }}</h1>
        <p class="page-sub">
          {{ activeTag ? $tn('{n} event tagged #{tag}', '{n} events tagged #{tag}', events.length, { tag: activeTag }) : $tn('{n} event', '{n} events', events.length) }}
        </p>
      </div>
      <RouterLink :to="newLink" class="btn btn-primary add-btn">{{ $t('New event') }}</RouterLink>
    </header>

    <div class="controls">
      <input
        v-model="search"
        type="text"
        class="search"
        :placeholder="$t('Search names and descriptions…')"
        :aria-label="$t('Search events')"
      />
      <div v-if="allTags.length" class="tag-row">
        <button
          v-for="t in allTags"
          :key="t.tag"
          type="button"
          class="tag-chip"
          :class="{ 'is-active': activeTag && t.tag.toLowerCase() === activeTag.toLowerCase() }"
          @click="toggleTag(t.tag)"
        >
          #{{ t.tag }} <span class="count">{{ t.count }}</span>
        </button>
        <button v-if="activeTag" type="button" class="clear" @click="toggleTag(activeTag)">{{ $t('Clear filter') }}</button>
      </div>
    </div>

    <p v-if="loadError" class="error-banner">{{ loadError }}</p>

    <div v-if="!loading && !loadError && events.length === 0" class="empty-state glass-panel">
      <p class="empty-title">{{ activeTag || search ? $t('No events match.') : $t('No events yet.') }}</p>
      <p class="empty-hint">
        {{ activeTag || search ? $t('Try clearing the filter or search.') : $t('Create one and it will appear on the timeline.') }}
      </p>
    </div>

    <ul class="event-list">
      <li v-for="e in events" :key="e.id">
        <RouterLink :to="`/events/${e.id}`" class="event-row glass-panel">
          <div class="date-block">
            <template v-if="parts(e)">
              <span class="year">{{ parts(e).year }}</span>
              <span class="quarter">{{ $t('Q{n}', { n: parts(e).quarter }) }}</span>
            </template>
            <span v-else class="quarter">{{ $t('undated') }}</span>
          </div>
          <div class="event-main">
            <div class="event-title">
              <strong>{{ e.name }}</strong>
              <span v-if="e.source_type" class="badge">{{ e.source_type === 'character' ? $t('Birth') : $t('Founding') }}</span>
            </div>
            <div class="meta">
              <span v-if="e.location_name">{{ e.location_name }}</span>
              <span v-if="e.people_count">{{ $tn('{n} person', '{n} people', e.people_count) }}</span>
            </div>
            <div v-if="e.tags.length" class="row-tags">
              <span v-for="t in e.tags" :key="t" class="mini-tag" @click.prevent.stop="toggleTag(t)">#{{ t }}</span>
            </div>
          </div>
          <img v-if="e.picture_path" :src="e.picture_path" :alt="e.name" class="thumb" />
        </RouterLink>
      </li>
    </ul>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { eventsApi } from '../api'
import { dateParts } from '../calendar'
import { t } from '../i18n'

const route = useRoute()
const router = useRouter()

const events = ref([])
const allTags = ref([])
const loading = ref(true)
const loadError = ref('')
const search = ref(typeof route.query.q === 'string' ? route.query.q : '')

const activeTag = computed(() => (typeof route.query.tag === 'string' ? route.query.tag : ''))
const newLink = computed(() => ({ path: '/events/new', query: activeTag.value ? { tag: activeTag.value } : {} }))

const parts = (e) => dateParts(e.event_date)

function toggleTag(tag) {
  const same = activeTag.value && tag.toLowerCase() === activeTag.value.toLowerCase()
  const query = { ...route.query }
  if (same) delete query.tag
  else query.tag = tag
  router.replace({ path: '/events', query })
}

async function load() {
  loading.value = true
  loadError.value = ''
  try {
    events.value = await eventsApi.list({ tag: activeTag.value, q: search.value.trim() })
  } catch (err) {
    loadError.value = t("Couldn't load events. Try refreshing.")
  } finally {
    loading.value = false
  }
}

let searchTimer = null
watch(search, () => {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(load, 250)
})
watch(() => route.query.tag, load)

onMounted(async () => {
  load()
  try {
    allTags.value = await eventsApi.tags()
  } catch (err) {
    // Filtering by tag is a convenience; the list still works without it.
  }
})
</script>

<style scoped>
.add-btn {
  text-decoration: none;
}

.controls {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  margin-bottom: 1.25rem;
}

.search {
  max-width: 420px;
}

.tag-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
  align-items: center;
}

.tag-chip {
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--glass-border);
  color: var(--text-muted);
  border-radius: 999px;
  padding: 0.2rem 0.7rem;
  font-family: 'Manrope', sans-serif;
  font-size: 0.78rem;
  font-weight: 600;
  cursor: pointer;
}

.tag-chip:hover {
  color: var(--text-primary);
}

.tag-chip.is-active {
  background: var(--accent-soft);
  border-color: color-mix(in srgb, var(--accent) 40%, transparent);
  color: var(--accent);
}

.count {
  opacity: 0.6;
  margin-left: 0.15rem;
}

.clear {
  background: transparent;
  border: none;
  color: var(--text-faint);
  font-family: 'Manrope', sans-serif;
  font-size: 0.78rem;
  cursor: pointer;
  text-decoration: underline;
}

.event-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.event-row {
  display: flex;
  align-items: center;
  gap: 1.1rem;
  padding: 0.9rem 1.1rem;
  text-decoration: none;
  transition: transform 0.12s ease;
}

.event-row:hover {
  transform: translateY(-1px);
}

.date-block {
  width: 4.2rem;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding-right: 1rem;
  border-right: 1px solid var(--glass-border);
}

.year {
  font-family: 'Fraunces', serif;
  font-size: 1.25rem;
  line-height: 1.1;
}

.quarter {
  font-size: 0.7rem;
  font-weight: 700;
  letter-spacing: 0.05em;
  color: var(--accent);
}

.event-main {
  flex: 1;
  min-width: 0;
}

.event-title {
  display: flex;
  align-items: center;
  gap: 0.6rem;
}

.event-title strong {
  font-size: 1rem;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.badge {
  font-size: 0.64rem;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: var(--text-muted);
  border: 1px solid var(--glass-border);
  border-radius: 4px;
  padding: 0.05rem 0.4rem;
  flex-shrink: 0;
}

.meta {
  display: flex;
  gap: 0.9rem;
  margin-top: 0.15rem;
  font-size: 0.78rem;
  color: var(--text-faint);
}

.row-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
  margin-top: 0.4rem;
}

.mini-tag {
  font-size: 0.72rem;
  font-weight: 600;
  color: var(--accent);
  background: var(--accent-soft);
  border-radius: 999px;
  padding: 0.05rem 0.5rem;
  cursor: pointer;
}

.thumb {
  width: 56px;
  height: 56px;
  border-radius: 8px;
  object-fit: cover;
  flex-shrink: 0;
}
</style>
