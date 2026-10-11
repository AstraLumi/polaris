<template>
  <Teleport to="body">
    <div class="overlay" @click.self="emit('close')">
      <form class="modal is-small glass-panel panel-solid link-panel" @submit.prevent="pick(matches[active])">
        <h2>{{ $t('Link to an article') }}</h2>
        <label class="field">
          <span>{{ $t('Link text') }}</span>
          <input v-model="linkText" type="text" maxlength="200" :placeholder="$t('Leave empty to show the article name')" />
        </label>
        <label class="field">
          <span>{{ $t('Article') }}</span>
          <input
            ref="searchEl"
            v-model="query"
            type="search"
            autocomplete="off"
            :placeholder="$t('Type a name to search the wiki')"
            role="combobox"
            aria-autocomplete="list"
            :aria-expanded="matches.length > 0"
            aria-controls="wiki-link-results"
            @keydown.down.prevent="moveActive(1)"
            @keydown.up.prevent="moveActive(-1)"
          />
        </label>
        <ul v-if="matches.length" id="wiki-link-results" class="results" role="listbox">
          <li v-for="(m, i) in matches" :key="m.type + m.id" role="option" :aria-selected="i === active">
            <button type="button" class="result" :class="{ active: i === active }" @mouseenter="active = i" @click="pick(m)">
              <span class="thumb"><IconImage :src="m.picture" :name="m.name" alt="" /></span>
              <span class="r-name" data-tip-overflow>{{ m.name }}</span>
              <span class="r-kind">{{ $t(TYPE_LABELS[m.type]) }}</span>
            </button>
          </li>
        </ul>
        <p v-else class="no-results">
          {{ query.trim() ? $t('No article has that name.') : $t('Start typing to see matching articles.') }}
        </p>
        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="emit('close')">{{ $t('Cancel') }}</button>
          <button type="submit" class="btn btn-primary" :disabled="!matches.length">{{ $t('Insert link') }}</button>
        </div>
      </form>
    </div>
  </Teleport>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import IconImage from './IconImage.vue'
import { searchArticles, wikiLinkMarkup } from '../markdownEdit'
import { TYPE_LABELS } from '../wiki'

// The "Link to an article" panel: type the link's text and part of an
// article's name, pick it from the matches, and get back the [[…]] to insert
// (emitted as `pick`). Used by the Markdown editor and the infobox rows.
const props = defineProps({
  // The wiki's article list (wikiApi.list()) and makeLinkResolver() over it.
  items: { type: Array, default: () => [] },
  resolve: { type: Function, default: null },
  // Selected text to start from: both the link text and the search.
  text: { type: String, default: '' },
})
const emit = defineEmits(['pick', 'close'])

// A selected [[link]], a line break or a whole paragraph isn't a useful start.
const start = props.text.replace(/\s+/g, ' ').trim()
const usable = start && !/[[\]]/.test(start) && start.length <= 200 ? start : ''
const linkText = ref(usable)
const query = ref(usable)
const active = ref(0)
const searchEl = ref(null)

const matches = computed(() => searchArticles(props.items, query.value))
watch(matches, () => (active.value = 0))

onMounted(() => {
  searchEl.value?.focus()
  searchEl.value?.select()
})

function moveActive(by) {
  const n = matches.value.length
  if (n) active.value = (active.value + by + n) % n
}

function pick(item) {
  if (item) emit('pick', wikiLinkMarkup(item, linkText.value, props.resolve))
}
</script>

<style scoped>
.link-panel .field {
  margin-bottom: 0.75rem;
}

.results {
  list-style: none;
  margin: 0;
  padding: 0;
  max-height: 16rem;
  overflow-y: auto;
  border: 1px solid var(--glass-border);
  border-radius: 8px;
}

.result {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  width: 100%;
  padding: 0.4rem 0.6rem;
  border: none;
  background: none;
  color: var(--text-primary);
  font-family: 'Manrope', sans-serif;
  font-size: 0.88rem;
  text-align: left;
  cursor: pointer;
}

.result.active {
  background: color-mix(in srgb, var(--line) 22%, transparent);
}

.thumb {
  flex: none;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 1.7rem;
  height: 1.7rem;
  overflow: hidden;
  border-radius: 5px;
  background: rgba(255, 255, 255, 0.06);
  font-size: 0.8rem;
  color: var(--text-muted);
}

.thumb :deep(img) {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.thumb :deep(svg) {
  width: 70%;
  height: 70%;
}

.r-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.r-kind {
  flex: none;
  font-size: 0.74rem;
  color: var(--text-faint);
}

.no-results {
  margin: 0;
  padding: 0.6rem 0;
  font-size: 0.85rem;
  color: var(--text-faint);
}
</style>
