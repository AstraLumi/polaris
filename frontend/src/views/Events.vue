<template>
  <div class="page">
    <header class="page-header">
      <div>
        <h1>{{ $t('Events') }}</h1>
        <p class="page-sub">
          {{ activeTag ? $tn('{n} event tagged #{tag}', '{n} events tagged #{tag}', events.length, { tag: activeTag }) : $tn('{n} event', '{n} events', events.length) }}
        </p>
      </div>
      <div class="header-actions">
        <button
          v-if="selectMode"
          class="btn btn-danger"
          :disabled="selectedIds.size === 0"
          @click="confirmOpen = true"
        >
          {{ $t('Delete selected ({n})', { n: selectedIds.size }) }}
        </button>
        <button v-if="events.length || selectMode" class="btn btn-ghost" @click="toggleSelectMode">
          {{ selectMode ? $t('Cancel') : $t('Select several') }}
        </button>
        <RouterLink :to="newLink" class="btn btn-primary add-btn">{{ $t('New event') }}</RouterLink>
      </div>
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

    <p v-if="activeChapter" class="chapter-filter">
      {{ $t('Only events in {chapter}', { chapter: chapterLabel(activeChapter) || '…' }) }}
      <button type="button" class="clear" @click="clearChapter">{{ $t('Clear filter') }}</button>
    </p>

    <p v-if="loadError" class="error-banner">{{ loadError }}</p>

    <div v-if="!loading && !loadError && events.length === 0" class="empty-state glass-panel">
      <p class="empty-title">{{ activeTag || search ? $t('No events match.') : $t('No events yet.') }}</p>
      <p class="empty-hint">
        {{ activeTag || search ? $t('Try clearing the filter or search.') : $t('Create one and it will appear on the timeline.') }}
      </p>
    </div>

    <ul class="event-list">
      <li v-for="e in events" :key="e.id">
        <!-- While selecting, a row is a toggle rather than a link. -->
        <component
          :is="selectMode ? 'div' : RouterLink"
          :to="selectMode ? undefined : `/events/${e.id}`"
          class="event-row glass-panel"
          :class="{ 'is-selected': selectedIds.has(e.id), 'is-picking': selectMode }"
          @click="selectMode && toggleSelected(e.id)"
        >
          <input
            v-if="selectMode"
            type="checkbox"
            class="row-check"
            :checked="selectedIds.has(e.id)"
            :aria-label="e.name"
            tabindex="-1"
          />
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
              <span v-if="e.chapter_id && chapterLabel(e.chapter_id)">{{ chapterLabel(e.chapter_id, { short: true }) }}</span>
            </div>
            <div v-if="e.tags.length" class="row-tags">
              <span v-for="t in e.tags" :key="t" class="mini-tag" @click.prevent.stop="toggleTag(t)">#{{ t }}</span>
            </div>
          </div>
          <img v-if="e.picture_path" :src="e.picture_path" :alt="e.name" class="thumb" />
        </component>
      </li>
    </ul>

    <ConfirmDialog
      v-if="confirmOpen"
      :title="$tn('Delete {n} event?', 'Delete {n} events?', selectedIds.size)"
      :message="$t('Their pictures go with them. This can\'t be undone.')"
      :confirm-label="$t('Delete')"
      @cancel="confirmOpen = false"
      @confirm="handleDelete"
    />
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useRoute, useRouter, RouterLink } from 'vue-router'
import { eventsApi } from '../api'
import { dateParts } from '../calendar'
import { t, tn } from '../i18n'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import { chapterLabel, loadChapters } from '../chapters'

const route = useRoute()
const router = useRouter()

const events = ref([])
const allTags = ref([])
const loading = ref(true)
const loadError = ref('')
const search = ref(typeof route.query.q === 'string' ? route.query.q : '')

// Selecting several events to delete at once.
const selectMode = ref(false)
const selectedIds = ref(new Set())
const confirmOpen = ref(false)

function toggleSelectMode() {
  selectMode.value = !selectMode.value
  selectedIds.value = new Set()
}
function toggleSelected(id) {
  const next = new Set(selectedIds.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  selectedIds.value = next
}
async function handleDelete() {
  confirmOpen.value = false
  let failed = 0
  for (const id of selectedIds.value) {
    try {
      await eventsApi.remove(id)
    } catch (err) {
      failed++
    }
  }
  selectMode.value = false
  selectedIds.value = new Set()
  await load()
  if (failed) loadError.value = tn("Couldn't delete {n} of them.", "Couldn't delete {n} of them.", failed)
  try {
    allTags.value = await eventsApi.tags()
  } catch (err) {
    // Tag chips are a convenience; they refresh next visit.
  }
}

const activeTag = computed(() => (typeof route.query.tag === 'string' ? route.query.tag : ''))
// /events?chapter=3 (from a chapter page) lists only that chapter's events.
const activeChapter = computed(() => Number(route.query.chapter) || null)
loadChapters()
const newLink = computed(() => ({ path: '/events/new', query: activeTag.value ? { tag: activeTag.value } : {} }))

const parts = (e) => dateParts(e.event_date)

function clearChapter() {
  const query = { ...route.query }
  delete query.chapter
  router.replace({ path: '/events', query })
}

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
    events.value = await eventsApi.list({ tag: activeTag.value, q: search.value.trim(), chapterId: activeChapter.value })
  } catch (err) {
    loadError.value = t("Couldn't load events. Try refreshing.")
  } finally {
    loading.value = false
  }
}

let searchTimer = null
watch(search, () => {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    // Kept in the URL so coming back to the list keeps the search.
    const query = { ...route.query }
    if (search.value.trim()) query.q = search.value.trim()
    else delete query.q
    router.replace({ path: '/events', query })
    load()
  }, 250)
})
watch(() => [route.query.tag, route.query.chapter], load)

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

.event-row.is-picking {
  cursor: pointer;
  user-select: none;
}

.event-row.is-selected {
  border-color: var(--accent);
  background: var(--accent-soft);
}

.row-check {
  width: 1.1rem;
  height: 1.1rem;
  flex-shrink: 0;
  accent-color: var(--accent);
  pointer-events: none;
}

.chapter-filter {
  margin: -0.5rem 0 1rem;
  font-size: 0.85rem;
  color: var(--text-muted);
}

.header-actions {
  display: flex;
  gap: 0.6rem;
  flex-wrap: wrap;
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
