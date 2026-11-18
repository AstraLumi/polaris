<template>
  <div class="drafts-page">
    <aside class="drafts-side glass-panel">
      <div class="side-head">
        <h1 class="page-title">{{ $t('Drafts') }}</h1>
        <p class="muted hint">{{ $t('Loose notes and idea boards, kept apart from the story.') }}</p>
        <div class="add-row">
          <button type="button" class="btn btn-ghost small" :disabled="busy" @click="add('note')">+ {{ $t('Note') }}</button>
          <button type="button" class="btn btn-ghost small" :disabled="busy" @click="add('board')">+ {{ $t('Board') }}</button>
        </div>
      </div>
      <p v-if="error" class="error-text">{{ error }}</p>
      <ul class="page-list">
        <li v-for="(p, i) in pages" :key="p.id" class="page-row" :class="{ 'is-on': p.id === current?.id }">
          <input
            v-if="renaming === p.id"
            ref="renameEl"
            class="rename"
            type="text"
            :value="p.title"
            maxlength="200"
            :aria-label="$t('Title')"
            @keydown.enter="$event.target.blur()"
            @keydown.esc="renaming = null"
            @blur="rename(p, $event.target.value)"
          />
          <RouterLink v-else :to="`/drafts/${p.id}`" class="page-link">
            <span class="kind" :title="p.kind === 'board' ? $t('Board') : $t('Note')" aria-hidden="true">{{ p.kind === 'board' ? '▦' : '¶' }}</span>
            <span class="name">{{ p.title }}</span>
            <span class="count muted">{{ p.kind === 'board' ? tn('{n} card', '{n} cards', p.cards, { n: p.cards }) : tn('{n} word', '{n} words', p.words, { n: p.words }) }}</span>
          </RouterLink>
          <RowMenu
            :items="[
              { label: $t('Rename'), action: () => startRename(p) },
              i > 0 && { label: $t('Move up'), action: () => move(i, -1) },
              i < pages.length - 1 && { label: $t('Move down'), action: () => move(i, 1) },
              { label: $t('Delete'), danger: true, action: () => remove(p) },
            ]"
          />
        </li>
      </ul>
    </aside>

    <section class="drafts-main">
      <template v-if="current">
        <DraftBoard v-if="current.kind === 'board'" :id="current.id" ref="childEl" :key="`b${current.id}`" v-model:categories="categories" @cards="onCards" />
        <DraftNote v-else :id="current.id" ref="childEl" :key="`n${current.id}`" :autofocus="current.id !== justMade" @words="onWords" />
      </template>
      <NotFoundPanel v-else-if="loaded && id" :message="$t('This draft was deleted, or never existed.')">
        <RouterLink to="/drafts" class="btn btn-ghost">{{ $t('Drafts') }}</RouterLink>
      </NotFoundPanel>
      <div v-else-if="loaded" class="start glass-panel">
        <h2>{{ $t('Somewhere for ideas that are not story yet') }}</h2>
        <p class="muted">{{ $t('A note is a page to write anything on. A board holds cards you place, colour by category and join with lines. Nothing here shows in the wiki, the timeline or the search.') }}</p>
        <div class="add-row">
          <button type="button" class="btn btn-primary" :disabled="busy" @click="add('note')">{{ $t('New note') }}</button>
          <button type="button" class="btn btn-primary" :disabled="busy" @click="add('board')">{{ $t('New board') }}</button>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup>
// Drafts (backend/drafts.go): the story's notes and boards, kept apart from
// it. The list on the left; the page picked on the right, a note
// (components/drafts/DraftNote.vue) or a board (DraftBoard.vue).
import { ref, computed, watch, nextTick, onMounted } from 'vue'
import { useRouter, onBeforeRouteUpdate, onBeforeRouteLeave } from 'vue-router'
import { t, tn } from '../i18n'
import { draftsApi } from '../api'
import { askConfirm } from '../confirm'
import { pageTitle } from '../navigation'
import RowMenu from '../components/RowMenu.vue'
import NotFoundPanel from '../components/NotFoundPanel.vue'
import DraftBoard from '../components/drafts/DraftBoard.vue'
import DraftNote from '../components/drafts/DraftNote.vue'

const props = defineProps({
  id: { type: String, default: '' },
})
const router = useRouter()

const pages = ref([])
const categories = ref([])
const loaded = ref(false)
const error = ref('')
const busy = ref(false)
const renaming = ref(null)
const renameEl = ref(null)
const childEl = ref(null)
const justMade = ref(null) // the draft just added, whose name is being typed

const current = computed(() => pages.value.find((p) => String(p.id) === props.id) || null)
watch(current, (p) => (pageTitle.value = p ? p.title : ''), { immediate: true })

async function load() {
  error.value = ''
  try {
    const res = await draftsApi.list()
    pages.value = res.pages
    categories.value = res.categories
    loaded.value = true
    // The bare page opens the first draft.
    if (!props.id && pages.value.length) router.replace(`/drafts/${pages.value[0].id}`)
  } catch (e) {
    error.value = e.message
  }
}
onMounted(load)

// What is being written is saved before another draft or page opens.
const flush = () => childEl.value?.flush?.()
onBeforeRouteUpdate(flush)
onBeforeRouteLeave(flush)

async function add(kind) {
  busy.value = true
  error.value = ''
  try {
    const p = await draftsApi.create(kind, kind === 'board' ? t('New board') : t('New note'))
    pages.value.push({ cards: 0, words: 0, ...p })
    justMade.value = p.id
    await router.push(`/drafts/${p.id}`)
    startRename(p)
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

async function startRename(p) {
  renaming.value = p.id
  await nextTick()
  const el = Array.isArray(renameEl.value) ? renameEl.value[0] : renameEl.value
  el?.focus()
  el?.select()
}
async function rename(p, title) {
  if (renaming.value !== p.id) return
  renaming.value = null
  title = title.trim()
  if (!title || title === p.title) return
  try {
    await draftsApi.rename(p.id, title)
    p.title = title
    if (p.id === current.value?.id) pageTitle.value = title
  } catch (e) {
    error.value = e.message
  }
}

async function move(i, by) {
  const list = [...pages.value]
  const [p] = list.splice(i, 1)
  list.splice(i + by, 0, p)
  pages.value = list
  try {
    await draftsApi.reorder(list.map((x) => x.id))
  } catch (e) {
    error.value = e.message
  }
}

async function remove(p) {
  const what = p.kind === 'board' ? t('Delete the board “{name}” and its cards?', { name: p.title }) : t('Delete the note “{name}”?', { name: p.title })
  if (!(await askConfirm(what, { message: t('This cannot be undone.'), confirmLabel: t('Delete') }))) return
  try {
    await draftsApi.remove(p.id)
    pages.value = pages.value.filter((x) => x.id !== p.id)
    if (p.id === current.value?.id || String(p.id) === props.id) router.replace(pages.value.length ? `/drafts/${pages.value[0].id}` : '/drafts')
  } catch (e) {
    error.value = e.message
  }
}

function onCards(id, n) {
  const p = pages.value.find((x) => x.id === id)
  if (p) p.cards = n
}
function onWords(id, n) {
  const p = pages.value.find((x) => x.id === id)
  if (p) p.words = n
}
</script>

<style scoped>
.drafts-page {
  display: grid;
  grid-template-columns: 16rem minmax(0, 1fr);
  gap: 1rem;
  height: calc(100vh - 2.5rem);
  min-height: 30rem;
}

.drafts-side {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding: 0.9rem;
  min-height: 0;
  overflow: auto;
}

.side-head .page-title {
  margin: 0;
  font-size: 1.4rem;
}

.hint {
  margin: 0.2rem 0 0.6rem;
  font-size: 0.82rem;
}

.add-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
}

.page-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
}

.page-row {
  display: flex;
  align-items: center;
  gap: 0.2rem;
  border-radius: 8px;
}

.page-row.is-on {
  background: var(--accent-soft);
}

.page-link {
  flex: 1;
  min-width: 0;
  display: grid;
  grid-template-columns: 1.2rem minmax(0, 1fr);
  column-gap: 0.35rem;
  padding: 0.35rem 0.4rem;
  color: inherit;
  text-decoration: none;
}

.page-link:hover .name {
  color: var(--accent);
}

.kind {
  grid-row: span 2;
  color: var(--text-muted);
  text-align: center;
}

.name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.count {
  font-size: 0.75rem;
}

.rename {
  flex: 1;
  min-width: 0;
}

.drafts-main {
  min-width: 0;
  min-height: 0;
}

.start {
  max-width: 36rem;
  padding: 1.5rem;
}

.start h2 {
  margin-top: 0;
}

@media (max-width: 760px) {
  .drafts-page {
    grid-template-columns: 1fr;
    height: auto;
  }

  .drafts-side {
    max-height: 14rem;
  }

  .drafts-main {
    height: 75vh;
  }
}
</style>
