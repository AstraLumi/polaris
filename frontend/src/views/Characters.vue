<template>
  <div class="page">
    <header class="page-header">
      <div>
        <h1>{{ $t('Characters') }}</h1>
        <p class="page-sub">
          {{ filtering ? $t('{shown} of {total}', { shown: shown.length, total: characters.length }) : $tn('{n} saved', '{n} saved', characters.length) }}
        </p>
      </div>
      <div class="actions">
        <button
          v-if="selectMode"
          class="btn btn-danger"
          :disabled="selectedIds.size === 0"
          @click="confirmOpen = true"
        >
          {{ $t('Delete selected ({n})', { n: selectedIds.size }) }}
        </button>
        <button class="btn btn-ghost" @click="toggleSelectMode">
          {{ selectMode ? $t('Cancel') : $t('Delete characters') }}
        </button>
        <button class="btn btn-primary" @click="addOpen = true">{{ $t('Add character') }}</button>
      </div>
    </header>

    <div v-if="characters.length" class="controls">
      <input
        v-model="search"
        type="text"
        class="search"
        :placeholder="$t('Search names, nicknames and classes…')"
        :aria-label="$t('Search characters')"
      />
      <select v-if="classOptions.length" :value="classFilter" :aria-label="$t('Class')" @change="setQuery('class', $event.target.value)">
        <option value="">{{ $t('All classes') }}</option>
        <option v-for="c in classOptions" :key="c" :value="c">{{ c }}</option>
      </select>
      <select v-if="raceOptions.length" :value="raceFilter" :aria-label="$t('Race')" @change="setQuery('race', $event.target.value)">
        <option value="">{{ $t('All races') }}</option>
        <option v-for="r in raceOptions" :key="r" :value="r">{{ r }}</option>
      </select>
      <select v-if="factionOptions.length" :value="factionFilter" :aria-label="$t('Faction')" @change="setQuery('faction', $event.target.value)">
        <option value="">{{ $t('Any faction') }}</option>
        <option v-for="f in factionOptions" :key="f.id" :value="String(f.id)">{{ f.name }}</option>
      </select>
      <select v-if="tagOptions.length" :value="tagFilter" :aria-label="$t('Tag')" @change="setQuery('tag', $event.target.value)">
        <option value="">{{ $t('Any tag') }}</option>
        <option v-for="tag in tagOptions" :key="tag" :value="tag">#{{ tag }}</option>
      </select>
      <select :value="statusFilter" :aria-label="$t('Status')" @change="setQuery('status', $event.target.value)">
        <option value="">{{ $t('Any status') }}</option>
        <option v-for="s in STATUSES" :key="s.key" :value="s.key">{{ $t(s.label) }}</option>
      </select>
      <select :value="sort" :aria-label="$t('Sort by')" @change="setQuery('sort', $event.target.value === 'name' ? '' : $event.target.value)">
        <option v-for="o in SORTS" :key="o.key" :value="o.key">{{ $t(o.label) }}</option>
      </select>
      <button v-if="filtering" type="button" class="btn btn-ghost small" @click="clearFilters">{{ $t('Clear filter') }}</button>
    </div>

    <p v-if="loadError" class="error-banner">{{ loadError }}</p>

    <div v-if="!loading && characters.length === 0" class="empty-state glass-panel">
      <p class="empty-title">{{ $t('No characters yet.') }}</p>
      <p class="empty-hint">{{ $t('Add your first one to start building their story and stats.') }}</p>
    </div>
    <div v-else-if="!loading && shown.length === 0" class="empty-state glass-panel">
      <p class="empty-title">{{ $t('No characters match.') }}</p>
      <p class="empty-hint">{{ $t('Try clearing the filter or search.') }}</p>
    </div>

    <div class="grid">
      <CharacterCard
        v-for="c in shown"
        :key="c.id"
        :character="c"
        :select-mode="selectMode"
        :selected="selectedIds.has(c.id)"
        @toggle-select="toggleSelected(c.id)"
      />
    </div>

    <AddCharacterModal v-if="addOpen" @close="addOpen = false" @created="onCreated" />

    <ConfirmDialog
      v-if="confirmOpen"
      :title="$tn('Delete {n} character?', 'Delete {n} characters?', selectedIds.size)"
      :message="$t('This removes their Story and Build data too. This can\'t be undone.')"
      :confirm-label="$t('Delete')"
      @cancel="confirmOpen = false"
      @confirm="handleDelete"
    />
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { t, tr } from '../i18n'
import { STATUSES } from '../characterStatus'
import CharacterCard from '../components/CharacterCard.vue'
import AddCharacterModal from '../components/AddCharacterModal.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import { fetchCharacters, deleteCharacters, factionsApi } from '../api'

const characters = ref([])
const loading = ref(true)
const loadError = ref('')
const selectMode = ref(false)
const selectedIds = ref(new Set())
const addOpen = ref(false)
const confirmOpen = ref(false)

// Search, filters and sort live in the URL, so going back to this page
// (or reloading it) keeps them.
const route = useRoute()
const router = useRouter()
const SORTS = [
  { key: 'name', label: tr('Name') },
  { key: 'level', label: tr('Highest level') },
  { key: 'recent', label: tr('Recently edited') },
]
const queryText = (k) => (typeof route.query[k] === 'string' ? route.query[k] : '')
const search = ref(queryText('q'))
const classFilter = computed(() => queryText('class'))
const raceFilter = computed(() => queryText('race'))
const statusFilter = computed(() => queryText('status'))
const factionFilter = computed(() => queryText('faction'))
const tagFilter = computed(() => queryText('tag'))
const factionOptions = ref([])
factionsApi.list().then((list) => (factionOptions.value = list)).catch(() => {})
const sort = computed(() => (SORTS.some((o) => o.key === queryText('sort')) ? queryText('sort') : 'name'))
const filtering = computed(() => !!(search.value.trim() || classFilter.value || raceFilter.value || statusFilter.value || factionFilter.value || tagFilter.value))

function setQuery(key, value) {
  const query = { ...route.query }
  if (value) query[key] = value
  else delete query[key]
  router.replace({ query })
}

let searchTimer = null
watch(search, (v) => {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => setQuery('q', v.trim()), 250)
})

function clearFilters() {
  search.value = ''
  router.replace({ query: route.query.sort ? { sort: route.query.sort } : {} })
}

const distinct = (key) =>
  [...new Set(characters.value.map((c) => c[key]).filter(Boolean))].sort((a, b) => a.localeCompare(b))
const classOptions = computed(() => distinct('class_name'))
const raceOptions = computed(() => distinct('race_name'))
const tagOptions = computed(() => {
  const seen = new Map()
  for (const c of characters.value) for (const tag of c.tags || []) seen.set(tag.toLowerCase(), tag)
  return [...seen.values()].sort((a, b) => a.localeCompare(b))
})
const hasTag = (c, tag) => (c.tags || []).some((x) => x.toLowerCase() === tag.toLowerCase())

const shown = computed(() => {
  const words = search.value.trim().toLowerCase().split(/\s+/).filter(Boolean)
  const list = characters.value.filter((c) => {
    if (classFilter.value && c.class_name !== classFilter.value) return false
    if (raceFilter.value && c.race_name !== raceFilter.value) return false
    if (statusFilter.value && (c.status || 'alive') !== statusFilter.value) return false
    if (factionFilter.value && !(c.faction_ids || []).includes(Number(factionFilter.value))) return false
    if (tagFilter.value && !hasTag(c, tagFilter.value)) return false
    const text = [c.name, c.nickname, c.class_name, c.subclass_name, c.race_name, ...(c.tags || [])].join(' ').toLowerCase()
    return words.every((w) => text.includes(w))
  })
  // The server sends them sorted by name.
  if (sort.value === 'level') return [...list].sort((a, b) => b.level - a.level)
  if (sort.value === 'recent') return [...list].sort((a, b) => b.updated_at.localeCompare(a.updated_at))
  return list
})

async function load() {
  loading.value = true
  loadError.value = ''
  try {
    characters.value = await fetchCharacters()
  } catch (err) {
    loadError.value = t("Couldn't load characters. Try refreshing.")
  } finally {
    loading.value = false
  }
}

function toggleSelectMode() {
  selectMode.value = !selectMode.value
  selectedIds.value = new Set()
}

function toggleSelected(id) {
  const next = new Set(selectedIds.value)
  if (next.has(id)) {
    next.delete(id)
  } else {
    next.add(id)
  }
  selectedIds.value = next
}

function onCreated() {
  addOpen.value = false
  load()
}

async function handleDelete() {
  try {
    await deleteCharacters([...selectedIds.value])
    confirmOpen.value = false
    selectMode.value = false
    selectedIds.value = new Set()
    await load()
  } catch (err) {
    loadError.value = t("Couldn't delete those characters. Try again.")
    confirmOpen.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.controls {
  display: flex;
  flex-wrap: wrap;
  gap: 0.6rem;
  align-items: center;
  margin-bottom: 1.25rem;
}

.controls .search {
  flex: 1 1 260px;
  max-width: 420px;
}

.controls select {
  width: auto;
}

.actions {
  display: flex;
  gap: 0.6rem;
  flex-wrap: wrap;
}

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 1.1rem;
}

</style>
