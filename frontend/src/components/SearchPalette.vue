<template>
  <div class="overlay search-overlay" @click.self="$emit('close')">
    <div class="palette glass-panel panel-solid" role="dialog" :aria-label="$t('Search')">
      <input
        ref="inputEl"
        v-model="query"
        type="text"
        class="palette-input"
        :placeholder="$t('Search characters, places, events, assets and pages…')"
        :aria-label="$t('Search')"
        @keydown.down.prevent="move(1)"
        @keydown.up.prevent="move(-1)"
        @keydown.enter.prevent="choose(results[cursor], $event.shiftKey)"
      />

      <p v-if="loadError" class="error-banner">{{ loadError }}</p>

      <ul v-if="results.length" ref="listEl" class="results">
        <template v-for="(r, i) in results" :key="r.key">
        <li v-if="r.group && r.group !== results[i - 1]?.group" class="group-head" aria-hidden="true">{{ $t(r.group) }}</li>
        <li
          class="result"
          :class="{ 'is-active': i === cursor }"
          @mousemove="cursor = i"
          @click="choose(r, false)"
        >
          <span class="result-icon">
            <IconImage v-if="r.kind === 'item'" :src="r.picture" :name="r.name" />
            <span v-else-if="r.kind === 'action'" class="page-mark">+</span>
            <span v-else class="page-mark">↦</span>
          </span>
          <span class="result-text">
            <strong>{{ r.name }}</strong>
            <small>{{ r.label }}</small>
          </span>
          <button
            v-if="r.wiki && r.path !== r.wiki"
            type="button"
            class="wiki-btn"
            :title="$t('Open the wiki article')"
            @click.stop="choose(r, true)"
          >
            {{ $t('Wiki') }}
          </button>
        </li>
        </template>
      </ul>
      <p v-else-if="query.trim() && items" class="nothing">{{ $t('Nothing matches.') }}</p>

      <p class="keys">{{ $t('↑↓ to move · Enter to open · Shift+Enter for the wiki article · Esc to close') }}</p>
    </div>
  </div>
</template>

<script setup>
// The Ctrl+K search: every character, location, event and asset (the wiki's
// index of the story) plus the app's own pages, in one list.
import { ref, computed, watch, nextTick, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { t, tr } from '../i18n'
import { wikiApi, wikiPath, TYPE_LABELS } from '../wiki'
import { chapterIndex, chapterLabel, loadChapters } from '../chapters'
import { recentPath, recentKey } from '../recent'
import { recentItems } from '../recentTracker'
import IconImage from './IconImage.vue'

const emit = defineEmits(['close'])
const router = useRouter()

const PAGES = [
  { name: tr('Home'), path: '/' },
  { name: tr('Characters'), path: '/characters' },
  { name: tr('Height chart'), path: '/characters/heights' },
  { name: tr('Factions'), path: '/factions' },
  { name: tr('Chapters'), path: '/chapters' },
  { name: tr('Character Assets'), path: '/assets' },
  { name: tr('Timeline'), path: '/timeline' },
  { name: tr('Calendar'), path: '/calendar' },
  { name: tr('Bestiary'), path: '/bestiary' },
  { name: tr('Map'), path: '/map' },
  { name: tr('Solar system'), path: '/solar-system' },
  { name: tr('Events'), path: '/events' },
  { name: tr('Wiki'), path: '/wiki' },
  { name: tr('Graph'), path: '/wiki/graph' },
  { name: tr('Settings'), path: '/settings' },
  { name: tr('Switch story'), path: '/stories' },
]

// Things to start. The list pages open their own add form when reached with
// ?new=1 (see quickAdd.js).
const ACTIONS = [
  { name: tr('New character'), path: '/characters?new=1' },
  { name: tr('New event'), path: '/events/new' },
  { name: tr('New chapter'), path: '/chapters?new=1' },
  { name: tr('New article'), path: '/wiki?new=1' },
  { name: tr('New faction'), path: '/factions?new=1' },
  { name: tr('New creature'), path: '/bestiary?new=1' },
]

const inputEl = ref(null)
const listEl = ref(null)
const query = ref('')
const cursor = ref(0)
const items = ref(null)
const loadError = ref('')

// Case and accent blind, so "eldmark" finds "Éldmark".
const fold = (s) => String(s).normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase()

// Characters and events have a page of their own; everything else is best
// seen through its wiki article.
function primaryPath(type, id) {
  if (type === 'character') return `/characters/${id}`
  if (type === 'event') return `/events/${id}`
  return wikiPath(type, id)
}

const entries = computed(() => {
  const pages = PAGES.map((p) => ({ key: `page:${p.path}`, kind: 'page', name: t(p.name), label: t('Page'), path: p.path }))
  const actions = ACTIONS.map((a) => ({ key: `action:${a.path}`, kind: 'action', name: t(a.name), label: t('Action'), path: a.path }))
  const things = (items.value || []).map((it) => ({
    key: `${it.type}:${it.id}`,
    kind: 'item',
    name: it.name,
    label: t(TYPE_LABELS[it.type] || it.type),
    picture: it.picture,
    path: primaryPath(it.type, it.id),
    wiki: wikiPath(it.type, it.id),
    aliases: it.aliases || [],
  }))
  // Chapters aren't wiki articles; they have their own page.
  const chapters = chapterIndex.value.chapters.map((c) => ({
    key: `chapter:${c.id}`,
    kind: 'item',
    name: c.title,
    label: chapterLabel(c.id, { short: true }),
    picture: '',
    path: `/chapters/${c.id}`,
  }))
  return [...things, ...chapters, ...actions, ...pages].map((e) => ({ ...e, folded: fold([e.name, ...(e.aliases || [])].join(' ')) }))
})

// Best first: the name starts with the query, then a word in it does, then
// it merely contains it. Every word typed has to appear somewhere.
// The items opened last, as entries. Once the story's index is here, ones that
// were deleted are left out and the pictures and current names are used.
const recent = computed(() => {
  const byKey = new Map(entries.value.filter((e) => e.kind === 'item').map((e) => [e.key, e]))
  const out = []
  for (const r of recentItems.value) {
    const key = recentKey(r)
    const found = byKey.get(key)
    if (items.value && !found) continue
    out.push({
      key: `recent:${key}`,
      kind: 'item',
      name: found?.name || r.name,
      label: r.type === 'chapter' ? t('Chapter') : t(TYPE_LABELS[r.type] || r.type),
      picture: found?.picture ?? '',
      path: recentPath(r),
      group: GROUP_RECENT,
    })
  }
  return out
})

const GROUP_RECENT = tr('Recently opened')
const GROUP_ACTIONS = tr('Actions')
const GROUP_PAGES = tr('Pages')

const results = computed(() => {
  const q = fold(query.value.trim())
  if (!q) {
    const rest = entries.value.filter((e) => e.kind === 'action' || e.kind === 'page')
    return [...recent.value, ...rest.map((e) => ({ ...e, group: e.kind === 'action' ? GROUP_ACTIONS : GROUP_PAGES }))]
  }
  const words = q.split(/\s+/)
  const scored = []
  for (const e of entries.value) {
    if (!words.every((w) => e.folded.includes(w))) continue
    let score = 2
    if (e.folded.startsWith(q)) score = 0
    else if (e.folded.split(/[\s\-'’]+/).some((part) => part.startsWith(words[0]))) score = 1
    scored.push([score, e])
  }
  scored.sort((a, b) => a[0] - b[0] || a[1].name.localeCompare(b[1].name))
  return scored.slice(0, 40).map(([, e]) => e)
})

watch(results, () => (cursor.value = 0))

function move(by) {
  const n = results.value.length
  if (!n) return
  cursor.value = (cursor.value + by + n) % n
  nextTick(() => listEl.value?.querySelectorAll('.result')[cursor.value]?.scrollIntoView({ block: 'nearest' }))
}

// Enter pressed before the index has loaded (a fast typist) is kept and
// acted on once it arrives.
let pendingEnter = null

function choose(r, toWiki) {
  if (!r && items.value === null && query.value.trim()) pendingEnter = toWiki
  if (!r) return
  emit('close')
  router.push(toWiki && r.wiki ? r.wiki : r.path)
}

onMounted(async () => {
  inputEl.value?.focus()
  loadChapters()
  try {
    items.value = await wikiApi.list()
  } catch (e) {
    items.value = []
    loadError.value = e.message
  }
  if (pendingEnter !== null) choose(results.value[0], pendingEnter)
})
</script>

<style scoped>
.search-overlay {
  align-items: flex-start;
  padding-top: 12vh;
}

.palette {
  width: 100%;
  max-width: 560px;
  padding: 0.75rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  background: color-mix(in srgb, var(--surface) 97%, transparent);
}

.palette-input {
  font-size: 1rem;
}

.results {
  list-style: none;
  margin: 0;
  padding: 0;
  max-height: 50vh;
  overflow-y: auto;
}

.group-head {
  margin: 0.45rem 0.55rem 0.15rem;
  font-size: 0.68rem;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--text-faint);
}

.group-head:first-child {
  margin-top: 0.1rem;
}

.result {
  display: flex;
  align-items: center;
  gap: 0.7rem;
  padding: 0.45rem 0.55rem;
  border-radius: 8px;
  cursor: pointer;
}

.result.is-active {
  background: var(--accent-soft);
}

.result-icon {
  width: 2rem;
  height: 2rem;
  flex-shrink: 0;
  border-radius: 7px;
  overflow: hidden;
  background: rgba(255, 255, 255, 0.05);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
}

.page-mark {
  font-size: 0.95rem;
}

.result-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
  flex: 1;
}

.result-text strong {
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.result-text small {
  font-size: 0.72rem;
  color: var(--text-faint);
}

.wiki-btn {
  font-size: 0.72rem;
  padding: 0.2rem 0.55rem;
  border-radius: 6px;
  border: 1px solid var(--glass-border);
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
}

.wiki-btn:hover {
  color: var(--accent);
  border-color: var(--accent);
}

.nothing {
  margin: 0.4rem 0.5rem;
  font-size: 0.85rem;
  color: var(--text-muted);
}

.keys {
  margin: 0.2rem 0.4rem 0;
  font-size: 0.7rem;
  color: var(--text-faint);
}
</style>
