<template>
  <div class="page">
    <header class="page-header">
      <div>
        <h1>{{ $t('Cultures') }}</h1>
        <p class="page-sub">{{ $tn('{n} culture', '{n} cultures', cultures.length) }}</p>
      </div>
      <div class="header-actions">
        <RouterLink to="/references" class="btn btn-ghost">{{ $t('References') }}</RouterLink>
        <button class="btn btn-primary" @click="openForm(null)">{{ $t('New culture') }}</button>
      </div>
    </header>

    <p v-if="loadError" class="error-banner" role="alert">{{ loadError }} <button type="button" class="retry" @click="$retryPage()">{{ $t('Try again') }}</button></p>
    <p v-else-if="loading" class="loading-hint">{{ $t('Loading…') }}</p>

    <div v-else-if="!cultures.length" class="empty-state glass-panel">
      <p class="empty-title">{{ $t('No cultures yet.') }}</p>
      <p class="empty-hint">{{ $t('How your peoples dress, eat, celebrate and believe, kingdom by kingdom.') }}</p>
    </div>

    <template v-else>
      <div class="tabs" role="tablist">
        <button type="button" role="tab" class="tab" :class="{ 'is-on': view === 'cultures' }" :aria-selected="view === 'cultures'" @click="view = 'cultures'">{{ $t('Cultures') }}</button>
        <button type="button" role="tab" class="tab" :class="{ 'is-on': view === 'elements' }" :aria-selected="view === 'elements'" @click="showElements">{{ $t('All elements') }}</button>
      </div>

      <template v-if="view === 'cultures'">
        <div class="filters">
          <input v-model="search" type="search" :placeholder="$t('Search names and kingdoms…')" :aria-label="$t('Search')" />
        </div>
        <p v-if="!shownCultures.length" class="note">{{ $t('No culture matches.') }}</p>
        <div class="grid">
          <article v-for="c in shownCultures" :key="c.id" class="card glass-panel">
            <RouterLink :to="`/cultures/${c.id}`" class="card-main">
              <span class="badge"><IconImage :src="c.picture_path" :name="c.name" /></span>
              <span class="card-text">
                <strong>{{ c.name }}</strong>
                <small>{{ $tn('{n} element', '{n} elements', c.element_count) }}</small>
                <span v-if="c.kingdoms.length" class="kingdoms">
                  <span v-for="k in c.kingdoms" :key="k.id" class="kingdom"><span class="swatch" :style="{ background: k.color }"></span>{{ k.name }}</span>
                </span>
                <small v-if="c.description" class="desc">{{ linkText(c.description) }}</small>
              </span>
            </RouterLink>
            <button type="button" class="btn btn-ghost small" @click="openForm(c)">{{ $t('Edit') }}</button>
          </article>
        </div>
      </template>

      <template v-else>
        <RefFilters v-model:search="elSearch" v-model:kind="elKind" v-model:tags="elTags" :kinds="ELEMENT_KINDS" :all-tags="elementTags" />
        <p v-if="!shownElements.length" class="note">{{ $t('No element matches.') }}</p>
        <div class="element-grid">
          <ElementCard v-for="e in shownElements" :key="e.id" :element="e" show-culture @tag="toggleTag" />
        </div>
      </template>
    </template>

    <CultureFormModal v-if="formOpen" :culture="editing" @close="formOpen = false" @saved="onChanged" @deleted="onChanged" />
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { t } from '../i18n'
import { culturesApi } from '../api'
import { useQuickAdd } from '../quickAdd'
import { useLinkNames, linkText } from '../linkNames'
import { ELEMENT_KINDS, allTags, filterItems } from '../cultures'
import { loadReferences } from '../referenceLibrary'
import IconImage from '../components/IconImage.vue'
import CultureFormModal from '../components/CultureFormModal.vue'
import RefFilters from '../components/RefFilters.vue'
import ElementCard from '../components/ElementCard.vue'

// Every culture of the story, and every element of all of them (to browse
// by tag: "every winter outfit"). A card opens the culture's page.

const route = useRoute()
const cultures = ref([])
const elements = ref([])
const loading = ref(true)
const loadError = ref('')
const formOpen = ref(false)
const editing = ref(null)
const search = ref('')
const view = ref(route.query.tag ? 'elements' : 'cultures')
const elSearch = ref('')
const elKind = ref('')
const elTags = ref(route.query.tag ? [String(route.query.tag)] : [])

const shownCultures = computed(() => {
  const q = search.value.trim().toLowerCase()
  return cultures.value.filter((c) => !q || c.name.toLowerCase().includes(q) || c.kingdoms.some((k) => k.name.toLowerCase().includes(q)))
})
const elementTags = computed(() => allTags(elements.value))
const shownElements = computed(() =>
  filterItems(elements.value, { search: elSearch.value, kind: elKind.value, tags: elTags.value, fields: ['name', 'description', 'tags', 'culture_name'] }),
)

async function load() {
  loadError.value = ''
  try {
    cultures.value = await culturesApi.list()
    if (view.value === 'elements') elements.value = await culturesApi.elements()
  } catch (e) {
    loadError.value = t("Couldn't load the cultures. Try refreshing.")
  } finally {
    loading.value = false
  }
}

async function showElements() {
  view.value = 'elements'
  try {
    elements.value = await culturesApi.elements()
  } catch (e) {
    loadError.value = t("Couldn't load the cultures. Try refreshing.")
  }
}

function toggleTag(tg) {
  const low = tg.toLowerCase()
  elTags.value = elTags.value.some((x) => x.toLowerCase() === low) ? elTags.value.filter((x) => x.toLowerCase() !== low) : [...elTags.value, tg]
}

function openForm(c) {
  editing.value = c
  formOpen.value = true
}

function onChanged() {
  formOpen.value = false
  load()
}

onMounted(load)
loadReferences().catch(() => {})
useQuickAdd(() => openForm(null))
useLinkNames()
</script>

<style scoped>
.header-actions {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.tabs {
  display: flex;
  gap: 0.35rem;
  margin-bottom: 0.85rem;
  border-bottom: 1px solid var(--line);
}

.tab {
  padding: 0.5rem 0.9rem;
  border: 0;
  border-bottom: 2px solid transparent;
  background: none;
  color: var(--text-muted);
  font: inherit;
  cursor: pointer;
}

.tab.is-on {
  color: var(--text-primary);
  border-bottom-color: var(--accent);
}

.filters {
  display: flex;
  gap: 0.6rem;
  margin-bottom: 1rem;
}

.filters input {
  flex: 1 1 16rem;
  max-width: 24rem;
}

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 0.75rem;
}

.element-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 0.85rem;
}

.card {
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
  padding: 0.85rem 1rem;
}

.card-main {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: flex-start;
  gap: 0.85rem;
  text-decoration: none;
}

.badge {
  width: 3.2rem;
  height: 3.2rem;
  flex-shrink: 0;
  border-radius: 10px;
  overflow: hidden;
  background: var(--accent-soft);
  color: var(--accent);
  display: flex;
  align-items: center;
  justify-content: center;
}

.card-text {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
  min-width: 0;
}

.card-text strong {
  color: var(--text-primary);
}

.card-main:hover strong {
  color: var(--accent);
}

.card-text small {
  font-size: 0.76rem;
  color: var(--text-faint);
}

.kingdoms {
  display: flex;
  flex-wrap: wrap;
  gap: 0.3rem;
}

.kingdom {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  font-size: 0.76rem;
  color: var(--text-muted);
}

.swatch {
  width: 0.6rem;
  height: 0.6rem;
  border-radius: 50%;
}

.desc {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  color: var(--text-muted) !important;
}

.note {
  color: var(--text-faint);
  font-size: 0.85rem;
}
</style>
