<template>
  <div class="page">
    <header class="page-header">
      <div>
        <h1>{{ $t('References') }}</h1>
        <p class="page-sub">{{ $t('Real-world pictures for inspiration, shared by all your stories.') }}</p>
      </div>
      <button class="btn btn-primary" @click="openForm(null)">{{ $t('New reference') }}</button>
    </header>

    <p v-if="loadError" class="error-banner" role="alert">{{ loadError }} <button type="button" class="retry" @click="$retryPage()">{{ $t('Try again') }}</button></p>
    <p v-else-if="loading" class="loading-hint">{{ $t('Loading…') }}</p>

    <div v-else-if="!referenceLibrary.length" class="empty-state glass-panel">
      <p class="empty-title">{{ $t('No references yet.') }}</p>
      <p class="empty-hint">{{ $t('Save outfits, armour or buildings you find online, with their link and tags, then base your designs on them.') }}</p>
    </div>

    <template v-else>
      <RefFilters v-model:search="search" v-model:kind="kind" v-model:tags="tags" :kinds="REFERENCE_KINDS" :all-tags="tagList" />

      <p v-if="!shown.length" class="note">{{ $t('No reference matches.') }}</p>
      <div class="grid">
        <article v-for="r in shown" :key="r.id" :id="'ref-' + r.id" class="card glass-panel" :class="{ 'is-target': target === r.id }">
          <button type="button" class="cover" :title="$t('View the pictures')" :disabled="!r.pictures.length" @click="view(r)">
            <img v-if="r.pictures.length" :src="r.pictures[0].url" :alt="r.title" loading="lazy" />
            <span v-else class="no-pic">{{ $t('No picture') }}</span>
            <span v-if="r.pictures.length > 1" class="count">{{ r.pictures.length }}</span>
          </button>
          <div class="card-body">
            <div class="card-title">
              <strong>{{ r.title }}</strong>
              <RowMenu :label="$t('More')" :items="[
                { label: $t('Edit'), action: () => openForm(r) },
                r.source_url && { label: $t('Open the source'), action: () => openSource(r) },
              ]" />
            </div>
            <small class="kind">{{ kindLabel(r.kind) }}</small>
            <a v-if="r.source_url" :href="r.source_url" target="_blank" rel="noopener noreferrer" class="source">{{ hostOf(r.source_url) }} ↗</a>
            <p v-if="r.description" class="desc">{{ r.description }}</p>
            <div v-if="r.tags.length" class="tags">
              <button v-for="tg in r.tags" :key="tg" type="button" class="tag" @click="toggleTag(tg)">#{{ tg }}</button>
            </div>
            <p v-if="uses[r.id]?.length" class="used">
              {{ $t('Used by') }}
              <template v-for="(u, i) in uses[r.id]" :key="u.story_id + u.element_id">
                <template v-if="i">, </template>
                <RouterLink v-if="u.story_id === storyId" :to="`/cultures/${u.culture_id}?element=${u.element_id}`">{{ u.element }}</RouterLink>
                <span v-else :title="u.story">{{ u.element }} ({{ u.story }})</span>
              </template>
            </p>
          </div>
        </article>
      </div>
    </template>

    <ReferenceFormModal v-if="formOpen" :reference="editing" @close="formOpen = false" @saved="formOpen = false" @deleted="formOpen = false" />
  </div>
</template>

<script setup>
import { ref, computed, onMounted, nextTick } from 'vue'
import { useRoute } from 'vue-router'
import { t } from '../i18n'
import { useQuickAdd } from '../quickAdd'
import { viewGallery } from '../navigation'
import { activeStoryId } from '../stories'
import { REFERENCE_KINDS, referenceLibrary, loadReferences, referenceUses } from '../referenceLibrary'
import { allTags, filterItems } from '../cultures'
import ReferenceFormModal from '../components/ReferenceFormModal.vue'
import RefFilters from '../components/RefFilters.vue'
import RowMenu from '../components/RowMenu.vue'

// The reference library: every story's shared mood board. A card's picture
// opens the viewer; its tags filter the list; "Used by" lists the designs
// (culture elements) based on it.

const route = useRoute()
const loading = ref(true)
const loadError = ref('')
const formOpen = ref(false)
const editing = ref(null)
const search = ref('')
const kind = ref('')
const tags = ref(route.query.tag ? [String(route.query.tag)] : [])
const uses = ref({})
const target = ref(Number(route.query.ref) || 0)
const storyId = activeStoryId

const tagList = computed(() => allTags(referenceLibrary.value))
const shown = computed(() =>
  filterItems(referenceLibrary.value, { search: search.value, kind: kind.value, tags: tags.value, fields: ['title', 'description', 'tags'] }),
)

const kindLabel = (k) => t(REFERENCE_KINDS.find((x) => x.id === k)?.label || k)

function hostOf(url) {
  try {
    return new URL(url).hostname.replace(/^www\./, '')
  } catch (e) {
    return url
  }
}

function toggleTag(tg) {
  const low = tg.toLowerCase()
  tags.value = tags.value.some((x) => x.toLowerCase() === low) ? tags.value.filter((x) => x.toLowerCase() !== low) : [...tags.value, tg]
}

function view(r) {
  viewGallery(r.pictures.map((p) => ({ src: p.url, alt: r.title })))
}

function openSource(r) {
  window.open(r.source_url, '_blank', 'noopener,noreferrer')
}

function openForm(r) {
  editing.value = r
  formOpen.value = true
}

onMounted(async () => {
  try {
    await loadReferences(true)
  } catch (e) {
    loadError.value = t("Couldn't load the references. Try refreshing.")
  } finally {
    loading.value = false
  }
  referenceUses()
    .then((u) => (uses.value = u || {}))
    .catch(() => {})
  if (target.value) {
    await nextTick()
    document.getElementById('ref-' + target.value)?.scrollIntoView({ block: 'center' })
  }
})

// /references?new=1 opens the form.
useQuickAdd(() => openForm(null))
</script>

<style scoped>
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 0.85rem;
}

.card {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  padding: 0;
}

.card.is-target {
  outline: 2px solid var(--accent);
}

.cover {
  position: relative;
  aspect-ratio: 3 / 4;
  padding: 0;
  border: 0;
  background: rgba(var(--wash), 0.04);
  cursor: zoom-in;
}

.cover:disabled {
  cursor: default;
}

.cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.no-pic {
  color: var(--text-faint);
  font-size: 0.85rem;
}

.count {
  position: absolute;
  right: 0.5rem;
  bottom: 0.5rem;
  padding: 0.1rem 0.5rem;
  border-radius: 999px;
  background: var(--surface);
  color: var(--text-primary);
  font-size: 0.75rem;
}

.card-body {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
  padding: 0.75rem 0.85rem 0.85rem;
  min-width: 0;
}

.card-title {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.5rem;
}

.card-title strong {
  color: var(--text-primary);
  overflow-wrap: anywhere;
}

.kind {
  color: var(--accent-text);
  font-size: 0.75rem;
  font-weight: 700;
}

.source {
  font-size: 0.8rem;
  color: var(--text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.desc {
  margin: 0;
  font-size: 0.85rem;
  color: var(--text-muted);
  white-space: pre-line;
  display: -webkit-box;
  -webkit-line-clamp: 4;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.tags {
  display: flex;
  flex-wrap: wrap;
  gap: 0.3rem;
}

.tag {
  padding: 0.1rem 0.5rem;
  border-radius: 999px;
  border: 1px solid var(--glass-border);
  background: rgba(var(--wash), 0.03);
  color: var(--text-muted);
  font: inherit;
  font-size: 0.75rem;
  cursor: pointer;
}

.tag:hover {
  border-color: var(--accent);
  color: var(--text-primary);
}

.used {
  margin: 0.2rem 0 0;
  font-size: 0.78rem;
  color: var(--text-faint);
}

.note {
  color: var(--text-faint);
  font-size: 0.85rem;
}
</style>
