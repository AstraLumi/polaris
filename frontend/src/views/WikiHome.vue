<template>
  <div class="page wiki-home">
    <header class="page-header">
      <div>
        <h1>{{ $t('Wiki') }}</h1>
        <p class="page-sub">
          {{ $tn('{n} article, built from your story', '{n} articles, built from your story', items.length) }}
          · {{ $tn('{n} written', '{n} written', writtenCount) }}
        </p>
      </div>
      <div class="head-actions">
        <RouterLink v-if="items.length" to="/wiki/graph" class="btn btn-ghost" :title="$t('Every wiki article and how they connect')">
          {{ $t('Graph') }}
        </RouterLink>
        <button v-if="items.length" type="button" class="btn btn-ghost" :title="$t('Open an article at random')" @click="openRandom">
          {{ $t('Random article') }}
        </button>
        <button type="button" class="btn btn-primary" :title="$t('A free-form page for gods, history, magic, languages or anything else')" @click="openNew">
          {{ $t('New article') }}
        </button>
      </div>
    </header>

    <div v-if="newOpen" class="overlay" @click.self="newOpen = false">
      <form class="modal is-small glass-panel panel-solid" @submit.prevent="createArticle">
        <h2>{{ $t('New article') }}</h2>
        <label class="field">
          <span>{{ $t('Title') }}</span>
          <input ref="newTitleEl" v-model="newTitle" type="text" maxlength="120" required />
        </label>
        <p v-if="newError" class="error-banner">{{ newError }}</p>
        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="newOpen = false">{{ $t('Cancel') }}</button>
          <button type="submit" class="btn btn-primary" :disabled="creating || !newTitle.trim()">{{ $t('Create') }}</button>
        </div>
      </form>
    </div>

    <p class="intro">
      {{ $t('Every character, location, event and asset already has an article here; open one to add to it, and deleting the original deletes its article too. New article adds a free-form page for anything else.') }}
    </p>

    <div class="controls">
      <input
        v-model="search"
        type="text"
        class="search"
        :placeholder="$t('Search articles…')"
        :aria-label="$t('Search articles')"
      />
      <div class="chips">
        <button type="button" class="chip" :class="{ 'is-active': !filter }" @click="filter = ''">
          {{ $t('All') }} <span class="count">{{ items.length }}</span>
        </button>
        <button
          v-for="g in typeCounts"
          :key="g.type"
          type="button"
          class="chip"
          :class="{ 'is-active': filter === g.type }"
          @click="filter = filter === g.type ? '' : g.type"
        >
          {{ $t(TYPE_PLURALS[g.type]) }} <span class="count">{{ g.count }}</span>
        </button>
      </div>
      <label class="check">
        <input v-model="writtenOnly" type="checkbox" />
        {{ $t('Only articles with written text') }}
      </label>
    </div>

    <p v-if="loadError" class="error-banner">{{ loadError }}</p>
    <p v-else-if="loading" class="loading-hint">{{ $t('Loading…') }}</p>

    <div v-else-if="groups.length === 0" class="empty-state glass-panel">
      <p class="empty-title">{{ items.length ? $t('No articles match.') : $t('Nothing to show yet.') }}</p>
      <p class="empty-hint">
        {{ items.length ? $t('Try clearing the filter or search.') : $t('Add characters, locations or events and they appear here by themselves.') }}
      </p>
    </div>

    <section v-if="recent.length && !search.trim() && !filter" class="glass-panel category recent">
      <h2>{{ $t('Recently changed') }}</h2>
      <ul class="recent-list">
        <li v-for="it in recent" :key="it.type + it.id">
          <RouterLink :to="wikiPath(it.type, it.id)" class="article-link">
            <span class="thumb">
              <IconImage :src="it.picture" :name="it.name" />
            </span>
            <span class="article-name" data-tip-overflow>{{ it.name }}</span>
            <span class="recent-meta">{{ $t(TYPE_LABELS[it.type]) }} · {{ timeAgo(it.edited_at) }}</span>
          </RouterLink>
        </li>
      </ul>
    </section>

    <section v-for="g in groups" :key="g.type" class="glass-panel category">
      <h2>
        {{ $t(TYPE_PLURALS[g.type]) }}
        <span class="count">{{ g.items.length }}</span>
      </h2>
      <ul class="article-list">
        <li v-for="it in g.items" :key="it.id">
          <RouterLink :to="wikiPath(it.type, it.id)" class="article-link" :class="{ 'is-stub': !it.written }">
            <span class="thumb">
              <IconImage :src="it.picture" :name="it.name" />
            </span>
            <span class="article-name" data-tip-overflow>{{ it.name }}</span>
          </RouterLink>
        </li>
      </ul>
    </section>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import IconImage from '../components/IconImage.vue'
import { TYPE_ORDER, TYPE_PLURALS, TYPE_LABELS, wikiApi, wikiPath } from '../wiki'
import { timeAgo } from '../timeAgo'

const router = useRouter()

const items = ref([])
const loading = ref(true)
const loadError = ref('')
const search = ref('')
const filter = ref('')
const writtenOnly = ref(false)

// A new lore article: named here, then written on its own page.
const newOpen = ref(false)
const newTitle = ref('')
const newError = ref('')
const creating = ref(false)
const newTitleEl = ref(null)

async function openNew() {
  newTitle.value = ''
  newError.value = ''
  newOpen.value = true
  await nextTick()
  newTitleEl.value?.focus()
}

async function createArticle() {
  creating.value = true
  newError.value = ''
  try {
    const fd = new FormData()
    fd.append('name', newTitle.value.trim())
    const { id } = await wikiApi.saveLore(null, fd)
    router.push({ path: wikiPath('lore', id), query: { edit: '1' } })
  } catch (e) {
    newError.value = e.message
  } finally {
    creating.value = false
  }
}

const writtenCount = computed(() => items.value.filter((i) => i.written).length)

// The articles whose written text changed last.
const recent = computed(() =>
  items.value
    .filter((i) => i.edited_at)
    .sort((a, b) => b.edited_at.localeCompare(a.edited_at))
    .slice(0, 8),
)

// Any article, written or not: a stub is a nudge to write it.
function openRandom() {
  const it = items.value[Math.floor(Math.random() * items.value.length)]
  if (it) router.push(wikiPath(it.type, it.id))
}

const typeCounts = computed(() =>
  TYPE_ORDER.map((type) => ({ type, count: items.value.filter((i) => i.type === type).length })).filter((g) => g.count),
)

const groups = computed(() => {
  const q = search.value.trim().toLowerCase()
  return TYPE_ORDER.filter((type) => !filter.value || filter.value === type)
    .map((type) => ({
      type,
      items: items.value.filter(
        (i) => i.type === type && (!q || i.name.toLowerCase().includes(q)) && (!writtenOnly.value || i.written),
      ),
    }))
    .filter((g) => g.items.length)
})

onMounted(async () => {
  try {
    items.value = await wikiApi.list()
  } catch (e) {
    loadError.value = e.message
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.head-actions {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.recent-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(260px, 100%), 1fr));
  gap: 0.25rem 1rem;
}

.recent-meta {
  margin-left: auto;
  padding-left: 0.5rem;
  font-size: 0.72rem;
  color: var(--text-faint);
  white-space: nowrap;
}

.intro {
  margin: -0.5rem 0 1.25rem;
  max-width: 46rem;
  font-size: 0.9rem;
  color: var(--text-muted);
}

.controls {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  margin-bottom: 1.25rem;
}

.search {
  max-width: 26rem;
}

.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
}

.chip {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.3rem 0.8rem;
  border-radius: 999px;
  border: 1px solid var(--glass-border);
  background: transparent;
  color: var(--text-muted);
  font: inherit;
  font-size: 0.82rem;
  cursor: pointer;
}

.chip:hover {
  color: var(--text-primary);
}

.chip.is-active {
  color: var(--text-primary);
  border-color: var(--accent);
  background: var(--accent-soft);
}

.count {
  font-size: 0.75rem;
  color: var(--text-faint);
}

.check {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.85rem;
  color: var(--text-muted);
}

.category {
  padding: 1.1rem 1.4rem 1.3rem;
  margin-bottom: 1rem;
}

.category h2 {
  margin: 0 0 0.8rem;
  padding-bottom: 0.35rem;
  font-size: 1.35rem;
  border-bottom: 1px solid var(--glass-border);
}

.article-list {
  list-style: none;
  margin: 0;
  padding: 0;
  columns: 16rem;
  column-gap: 1.5rem;
}

.article-list li {
  break-inside: avoid;
}

.article-link {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  padding: 0.22rem 0;
  color: var(--accent);
  text-decoration: none;
  font-size: 0.92rem;
}

.article-link:hover .article-name {
  text-decoration: underline;
}

/* An article nobody has written anything on yet (a "stub"). */
.article-link.is-stub {
  color: var(--text-muted);
}

.thumb {
  width: 1.5rem;
  height: 1.5rem;
  flex-shrink: 0;
  border-radius: 5px;
  background: var(--accent-soft);
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 0.7rem;
}

.article-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
