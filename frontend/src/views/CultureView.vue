<template>
  <div class="page culture-page">
    <Breadcrumbs :missing="missing" :crumbs="[{ label: $t('Cultures'), to: '/cultures' }, { label: culture?.name }]">
      <ItemLinks v-if="culture" type="culture" :id="culture.id" />
    </Breadcrumbs>

    <NotFoundPanel v-if="missing" :message="$t('This culture was deleted, or never existed.')">
      <RouterLink to="/cultures" class="btn btn-ghost">{{ $t('All cultures') }}</RouterLink>
    </NotFoundPanel>
    <p v-else-if="loadError" class="error-banner" role="alert">{{ loadError }} <button type="button" class="retry" @click="$retryPage()">{{ $t('Try again') }}</button></p>
    <p v-else-if="!culture" class="loading-hint">{{ $t('Loading…') }}</p>

    <template v-else>
      <article class="glass-panel hero">
        <img
          v-if="culture.picture_path"
          :src="culture.picture_path"
          :alt="culture.name"
          class="hero-pic"
          role="button"
          tabindex="0"
          :title="$t('View the picture')"
          @click="viewPicture(culture.picture_path, culture.name)"
          @keydown.enter.prevent="viewPicture(culture.picture_path, culture.name)"
        />
        <div class="hero-text">
          <div class="hero-title">
            <h1>{{ culture.name }}</h1>
            <button type="button" class="btn btn-ghost small" @click="formOpen = true">{{ $t('Edit') }}</button>
          </div>
          <p v-if="culture.kingdoms.length" class="kingdoms">
            <RouterLink v-for="k in culture.kingdoms" :key="k.id" :to="mapPath(k.id)" class="kingdom">
              <span class="swatch" :style="{ background: k.color }"></span>{{ k.name }}
            </RouterLink>
          </p>
          <p v-else class="no-kingdom">{{ $t('Not tied to a kingdom') }}</p>
          <p v-if="culture.description" class="desc">{{ linkText(culture.description) }}</p>
        </div>
      </article>

      <div class="tabs" role="tablist">
        <button type="button" role="tab" class="tab" :class="{ 'is-on': !kind }" :aria-selected="!kind" @click="kind = ''">
          {{ $t('All') }} <span class="n">{{ culture.elements.length }}</span>
        </button>
        <button
          v-for="k in ELEMENT_KINDS"
          :key="k.id"
          type="button"
          role="tab"
          class="tab"
          :class="{ 'is-on': kind === k.id, 'is-empty': !counts[k.id] }"
          :aria-selected="kind === k.id"
          @click="kind = k.id"
        >
          {{ $t(k.label) }} <span v-if="counts[k.id]" class="n">{{ counts[k.id] }}</span>
        </button>
      </div>

      <div class="toolbar">
        <RefFilters v-model:search="search" v-model:tags="tags" :all-tags="tagList" />
        <button type="button" class="btn btn-primary" @click="openElement(null)">{{ addLabel }}</button>
      </div>

      <div v-if="!culture.elements.length" class="empty-state glass-panel">
        <p class="empty-title">{{ $t('Nothing here yet.') }}</p>
        <p class="empty-hint">{{ $t('Add the clothing, food, festivals and customs of this culture, with pictures and tags.') }}</p>
      </div>
      <p v-else-if="!shown.length" class="note">{{ $t('No element matches.') }}</p>
      <div class="element-grid">
        <ElementCard
          v-for="e in shown"
          :key="e.id"
          :element="e"
          editable
          :target="target === e.id"
          @edit="openElement"
          @delete="removeElement"
          @tag="toggleTag"
        />
      </div>

      <Pager :prev="neighbours.prev" :next="neighbours.next" :prev-label="$t('← Previous culture')" :next-label="$t('Next culture →')" :label="$t('Cultures')" />
    </template>

    <CultureFormModal v-if="formOpen" :culture="culture" @close="formOpen = false" @saved="onSaved" @deleted="$router.push('/cultures')" />
    <CultureElementModal
      v-if="elementOpen"
      :culture-id="culture.id"
      :element="editingElement"
      :kind="kind || 'clothing'"
      :tag-suggestions="tagList"
      @close="elementOpen = false"
      @saved="onElementSaved"
    />
  </div>
</template>

<script setup>
import { ref, computed, watch, nextTick } from 'vue'
import { useRoute } from 'vue-router'
import { t } from '../i18n'
import { culturesApi, isMissing } from '../api'
import { mapPath, viewPicture, pageTitle } from '../navigation'
import { askConfirm } from '../confirm'
import { useLinkNames, linkText } from '../linkNames'
import { ELEMENT_KINDS, allTags, filterItems, countByKind } from '../cultures'
import { loadReferences } from '../referenceLibrary'
import Breadcrumbs from '../components/Breadcrumbs.vue'
import ItemLinks from '../components/ItemLinks.vue'
import NotFoundPanel from '../components/NotFoundPanel.vue'
import Pager from '../components/Pager.vue'
import RefFilters from '../components/RefFilters.vue'
import ElementCard from '../components/ElementCard.vue'
import CultureFormModal from '../components/CultureFormModal.vue'
import CultureElementModal from '../components/CultureElementModal.vue'

// One culture: its kingdoms and description, and its elements by kind
// (clothing first), each with pictures, tags and the references it is
// based on. ?element=<id> picks out one element (from the library's
// "Used by").

const props = defineProps({ id: { type: [String, Number], required: true } })
const route = useRoute()

const culture = ref(null)
const all = ref([])
const missing = ref(false)
const loadError = ref('')
const formOpen = ref(false)
const elementOpen = ref(false)
const editingElement = ref(null)
const kind = ref('')
const search = ref('')
const tags = ref([])
const target = ref(Number(route.query.element) || 0)

const counts = computed(() => countByKind(culture.value?.elements))
const tagList = computed(() => allTags(culture.value?.elements))
const shown = computed(() =>
  filterItems(culture.value?.elements, { search: search.value, kind: kind.value, tags: tags.value, fields: ['name', 'description', 'tags'] }),
)
const addLabel = computed(() => (kind.value === 'clothing' || !kind.value ? t('Add clothing') : t('Add an element')))

const neighbours = computed(() => {
  const i = all.value.findIndex((c) => String(c.id) === String(props.id))
  if (i < 0) return { prev: null, next: null }
  const link = (c) => c && { to: `/cultures/${c.id}`, name: c.name }
  return { prev: link(all.value[i - 1]), next: link(all.value[i + 1]) }
})

async function load() {
  missing.value = false
  loadError.value = ''
  try {
    culture.value = await culturesApi.get(props.id)
    pageTitle.value = culture.value.name
  } catch (e) {
    culture.value = null
    missing.value = await isMissing(`/cultures/${props.id}`)
    if (!missing.value) loadError.value = t("Couldn't load the culture. Try refreshing.")
    return
  }
  if (target.value) {
    kind.value = culture.value.elements.find((e) => e.id === target.value)?.kind || ''
    await nextTick()
    document.getElementById('element-' + target.value)?.scrollIntoView({ block: 'center' })
  }
}

watch(() => props.id, load, { immediate: true })
culturesApi
  .list()
  .then((list) => (all.value = list))
  .catch(() => {})
loadReferences().catch(() => {})

function toggleTag(tg) {
  const low = tg.toLowerCase()
  tags.value = tags.value.some((x) => x.toLowerCase() === low) ? tags.value.filter((x) => x.toLowerCase() !== low) : [...tags.value, tg]
}

function openElement(e) {
  editingElement.value = e
  elementOpen.value = true
}

function onSaved(c) {
  formOpen.value = false
  culture.value = c
}

function onElementSaved(saved) {
  elementOpen.value = false
  const list = culture.value.elements.filter((e) => e.id !== saved.id)
  culture.value = { ...culture.value, elements: [...list, saved].sort((a, b) => a.name.localeCompare(b.name)) }
}

async function removeElement(e) {
  const ok = await askConfirm(t('Delete {name}?', { name: e.name }), {
    message: t('Its pictures go too. The references it was based on stay in the library.'),
    confirmLabel: t('Delete'),
  })
  if (!ok) return
  try {
    await culturesApi.removeElement(e.id)
    culture.value = { ...culture.value, elements: culture.value.elements.filter((x) => x.id !== e.id) }
  } catch (err) {
    loadError.value = err.message
  }
}

useLinkNames()
</script>

<style scoped>
.hero {
  display: flex;
  gap: 1.25rem;
  padding: 1.25rem;
  margin-bottom: 1rem;
  flex-wrap: wrap;
}

.hero-pic {
  width: 10rem;
  height: 10rem;
  object-fit: cover;
  border-radius: 12px;
  cursor: zoom-in;
}

.hero-text {
  flex: 1 1 18rem;
  min-width: 0;
}

.hero-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.hero-title h1 {
  margin: 0;
}

.kingdoms {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
  margin: 0.5rem 0 0;
}

.kingdom {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.15rem 0.6rem;
  border: 1px solid var(--glass-border);
  border-radius: 999px;
  font-size: 0.82rem;
  color: var(--text-muted);
  text-decoration: none;
}

.kingdom:hover {
  border-color: var(--accent);
  color: var(--text-primary);
}

.swatch {
  width: 0.65rem;
  height: 0.65rem;
  border-radius: 50%;
}

.no-kingdom {
  margin: 0.5rem 0 0;
  font-size: 0.82rem;
  color: var(--text-faint);
}

.desc {
  margin: 0.75rem 0 0;
  color: var(--text-muted);
  white-space: pre-line;
}

.tabs {
  display: flex;
  gap: 0.2rem;
  margin-bottom: 0.85rem;
  border-bottom: 1px solid var(--line);
  overflow-x: auto;
}

.tab {
  padding: 0.5rem 0.8rem;
  border: 0;
  border-bottom: 2px solid transparent;
  background: none;
  color: var(--text-muted);
  font: inherit;
  white-space: nowrap;
  cursor: pointer;
}

.tab.is-empty {
  color: var(--text-faint);
}

.tab.is-on {
  color: var(--text-primary);
  border-bottom-color: var(--accent);
}

.n {
  font-size: 0.75rem;
  color: var(--text-faint);
}

.toolbar {
  display: flex;
  gap: 0.75rem;
  align-items: flex-start;
  justify-content: space-between;
  flex-wrap: wrap;
}

.toolbar > :first-child {
  flex: 1 1 20rem;
}

.element-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 0.85rem;
  margin-bottom: 1.25rem;
}

.note {
  color: var(--text-faint);
  font-size: 0.85rem;
}
</style>
