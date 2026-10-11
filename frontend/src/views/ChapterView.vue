<template>
  <div class="page is-narrow chapter-page">
    <BackLink to="/chapters" :label="$t('← Chapters')" />

    <p v-if="loadError" class="error-banner">{{ loadError }}</p>
    <p v-else-if="!chapter" class="loading-hint">{{ $t('Loading…') }}</p>

    <template v-else>
      <header class="glass-panel head">
        <template v-if="!editing">
          <p class="kicker">{{ kicker }}</p>
          <div class="title-row">
            <h1>{{ chapter.title }}</h1>
            <button type="button" class="btn btn-ghost small" @click="startEdit">{{ $t('Edit') }}</button>
          </div>
        </template>
        <form v-else class="edit" @submit.prevent="save">
          <div class="field-row">
            <label class="field">
              <span>{{ $t('Title') }}</span>
              <input ref="titleEl" v-model="draft.title" type="text" maxlength="200" required />
            </label>
            <label class="field">
              <span>{{ $t('Volume') }}</span>
              <select v-model="draft.volumeId">
                <option :value="null">{{ $t('No volume') }}</option>
                <option v-for="v in idx.volumes" :key="v.id" :value="v.id">{{ v.title }}</option>
              </select>
            </label>
          </div>
          <div class="field">
            <label for="chapter-notes">{{ $t('Notes') }}</label>
            <MarkdownEditor id="chapter-notes" v-model="draft.notes" :rows="14" :items="wikiItems" :resolve="resolver" :placeholder="$t('What happens, threads to pick up, anything you want to remember. Markdown and [[links]] work as in the wiki.')" />
          </div>
          <p v-if="saveError" class="error-banner">{{ saveError }}</p>
          <div class="edit-actions">
            <button type="button" class="btn btn-danger" @click="confirmOpen = true">{{ $t('Delete') }}</button>
            <span class="spacer"></span>
            <button type="button" class="btn btn-ghost" :disabled="saving" @click="cancelEdit">{{ $t('Cancel') }}</button>
            <button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? $t('Saving…') : $t('Save') }}</button>
          </div>
        </form>
      </header>

      <section v-if="!editing" class="glass-panel block">
        <h2>{{ $t('Notes') }}</h2>
        <!-- eslint-disable-next-line vue/no-v-html -->
        <div v-if="chapter.notes" class="md" @click="onContentClick" v-html="renderMarkdown(chapter.notes, resolver)"></div>
        <p v-else class="empty">{{ $t('No notes yet.') }} <button type="button" class="link-btn" @click="startEdit">{{ $t('Write some') }}</button></p>
      </section>

      <section class="glass-panel block">
        <div class="block-head">
          <h2>{{ $t('Events in this chapter') }}</h2>
          <span class="head-links">
            <RouterLink :to="{ path: '/events/new', query: { chapter: chapter.id } }" class="more">{{ $t('New event here') }}</RouterLink>
            <RouterLink v-if="chapter.events.length" :to="{ path: '/events', query: { chapter: chapter.id } }" class="more">{{ $t('On the Events page →') }}</RouterLink>
          </span>
        </div>
        <ul v-if="chapter.events.length" class="list">
          <li v-for="e in chapter.events" :key="e.id">
            <RouterLink :to="`/events/${e.id}`">{{ e.name }}</RouterLink>
            <span class="faint">{{ fullDate(e.event_date) || e.event_date }}</span>
          </li>
        </ul>
        <p v-else class="empty">{{ $t('No events point to this chapter. Pick it on an event to list it here.') }}</p>
      </section>

      <section class="glass-panel block">
        <h2>{{ $t('Character changes') }}</h2>
        <ul v-if="chapter.versions.length" class="list">
          <li v-for="v in chapter.versions" :key="v.version_id">
            <RouterLink :to="`/characters/${v.character_id}/versions/${v.version_id}/edit`">{{ v.name }}</RouterLink>
            <span class="faint">
              <template v-if="v.is_current">{{ $t('Current version') }}</template>
              <template v-if="v.version_date"> · {{ fullDate(v.version_date) || v.version_date }}</template>
              <template v-if="v.version_reference"> · {{ v.version_reference }}</template>
            </span>
          </li>
        </ul>
        <p v-else class="empty">{{ $t('No character versions point to this chapter. Pick it on a version to list it here.') }}</p>
      </section>

      <nav class="pager">
        <RouterLink v-if="chapter.prev_id" :to="`/chapters/${chapter.prev_id}`" class="btn btn-ghost">{{ $t('← Previous chapter') }}</RouterLink>
        <span class="spacer"></span>
        <RouterLink v-if="chapter.next_id" :to="`/chapters/${chapter.next_id}`" class="btn btn-ghost">{{ $t('Next chapter →') }}</RouterLink>
      </nav>
    </template>

    <ConfirmDialog
      v-if="confirmOpen"
      :title="$t('Delete {name}?', { name: chapter?.title })"
      :message="$t('Its notes are deleted. Events and character versions that pointed to it are kept, without a chapter.')"
      :confirm-label="$t('Delete')"
      @cancel="confirmOpen = false"
      @confirm="remove"
    />
  </div>
</template>

<script setup>
import { ref, reactive, computed, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { t } from '../i18n'
import { chaptersApi } from '../api'
import { chapterIndex as idx, loadChapters } from '../chapters'
import { fullDate } from '../calendar'
import { renderMarkdown } from '../markdown'
import { wikiApi, makeLinkResolver } from '../wiki'
import { pageTitle, useUnsavedGuard, useSaveShortcut } from '../navigation'
import BackLink from '../components/BackLink.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import MarkdownEditor from '../components/MarkdownEditor.vue'

const props = defineProps({ id: { type: String, required: true } })
const route = useRoute()
const router = useRouter()

const chapter = ref(null)
const loadError = ref('')
const editing = ref(false)
const saving = ref(false)
const saveError = ref('')
const confirmOpen = ref(false)
const titleEl = ref(null)
const draft = reactive({ title: '', volumeId: null, notes: '' })
const wikiItems = ref([])
let snapshot = ''

const resolver = computed(() => makeLinkResolver(wikiItems.value))

const kicker = computed(() => {
  const c = chapter.value
  const num = t('Chapter {n}', { n: c.number })
  return c.volume_title ? `${c.volume_title} · ${num}` : num
})

const draftState = () => JSON.stringify(draft)
const dirty = () => editing.value && draftState() !== snapshot
useUnsavedGuard(dirty)
useSaveShortcut(() => editing.value && !saving.value && save())

// [[links]] in the notes are plain anchors; send them through the router.
function onContentClick(e) {
  const a = e.target.closest && e.target.closest('a[data-wiki]')
  if (!a) return
  e.preventDefault()
  router.push(a.getAttribute('href'))
}

function startEdit() {
  const c = chapter.value
  Object.assign(draft, { title: c.title, volumeId: c.volume_id ?? null, notes: c.notes })
  snapshot = draftState()
  saveError.value = ''
  editing.value = true
  nextTick(() => titleEl.value?.focus())
}

function cancelEdit() {
  editing.value = false
}

async function save() {
  saving.value = true
  saveError.value = ''
  try {
    chapter.value = await chaptersApi.update(chapter.value.id, {
      title: draft.title,
      volume_id: draft.volumeId,
      notes: draft.notes,
    })
    pageTitle.value = chapter.value.title
    editing.value = false
    loadChapters(true)
  } catch (e) {
    saveError.value = e.message
  } finally {
    saving.value = false
  }
}

async function remove() {
  confirmOpen.value = false
  try {
    await chaptersApi.remove(chapter.value.id)
    editing.value = false
    await loadChapters(true)
    router.replace('/chapters')
  } catch (e) {
    saveError.value = e.message
  }
}

async function load() {
  loadError.value = ''
  editing.value = false
  chapter.value = null
  try {
    const [c] = await Promise.all([chaptersApi.get(props.id), loadChapters()])
    chapter.value = c
    pageTitle.value = c.title
    // A chapter just made from the Chapters page opens ready to name.
    if (route.query.edit) {
      startEdit()
      router.replace({ query: {} })
    }
  } catch (e) {
    loadError.value = e.message
  }
}

watch(() => props.id, load, { immediate: true })
wikiApi.list().then((list) => (wikiItems.value = list)).catch(() => {})
</script>

<style scoped>
.chapter-page {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.chapter-page .back-link {
  margin-bottom: 0;
}

.head {
  padding: 1.25rem 1.5rem;
}

.kicker {
  margin: 0 0 0.2rem;
  font-size: 0.78rem;
  letter-spacing: 0.04em;
  color: var(--accent);
}

.title-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.title-row h1 {
  margin: 0;
  font-family: 'Fraunces', serif;
  font-weight: 500;
  font-size: 1.7rem;
}

.edit-actions,
.pager {
  display: flex;
  gap: 0.6rem;
  align-items: center;
}

.spacer {
  flex: 1;
}

.block {
  padding: 1rem 1.5rem 1.2rem;
}

.block h2 {
  margin: 0 0 0.6rem;
  font-family: 'Fraunces', serif;
  font-weight: 500;
  font-size: 1.1rem;
}

.block-head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 1rem;
}

.head-links {
  display: flex;
  gap: 1rem;
}

.more {
  font-size: 0.78rem;
  color: var(--text-muted);
  text-decoration: none;
}

.md {
  font-size: 0.95rem;
  line-height: 1.65;
  overflow-wrap: anywhere;
}

.md :deep(a) {
  color: var(--line);
  text-decoration: none;
}

.md :deep(a:hover) {
  text-decoration: underline;
}

.list {
  list-style: none;
  margin: 0;
  padding: 0;
}

.list li {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.35rem 0;
  border-top: 1px solid var(--glass-border);
  font-size: 0.9rem;
}

.list li:first-child {
  border-top: none;
}

.list a {
  color: var(--text-primary);
  font-weight: 600;
  text-decoration: none;
}

.list a:hover {
  color: var(--accent);
}

.faint {
  color: var(--text-faint);
  font-size: 0.8rem;
  text-align: right;
}

.empty {
  margin: 0;
  font-size: 0.85rem;
  color: var(--text-faint);
}

.link-btn {
  border: none;
  background: none;
  padding: 0;
  color: var(--accent);
  font: inherit;
  cursor: pointer;
}
</style>
