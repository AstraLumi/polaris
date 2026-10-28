// The story's names for the writing page (Tab-to-link and Script Mode's
// speakers): the wiki's article list, loaded once and again whenever the
// page gets the focus back (a character may have been added or renamed in
// another tab), made into nameMatch.js's index.
import { shallowRef, computed, onMounted, onBeforeUnmount } from 'vue'
import { wikiApi, TYPE_LABELS } from '../wiki'
import { t } from '../i18n'
import { buildNameIndex, resolveSpeaker } from './nameMatch.js'

// fetchItems: the article list (the writing page answers it from an offline
// book's copy when the server can't be reached).
export function useStoryNames({ fetchItems = () => wikiApi.list() } = {}) {
  const index = shallowRef([])
  let loading = null
  function load() {
    loading ||= Promise.resolve()
      .then(fetchItems)
      .then((items) => (index.value = buildNameIndex(items)))
      .catch(() => {})
      .finally(() => (loading = null))
    return loading
  }
  const onFocus = () => load()
  onMounted(() => {
    load()
    window.addEventListener('focus', onFocus)
  })
  onBeforeUnmount(() => window.removeEventListener('focus', onFocus))

  // Every character by its current name, for the speaker picker.
  const characters = computed(() =>
    index.value
      .filter((e) => e.type === 'character' && e.via === 'name')
      .map((e) => ({ id: e.id, name: e.label }))
      .sort((a, b) => a.name.localeCompare(b.name)),
  )
  return {
    index,
    characters,
    resolve: (name) => resolveSpeaker(index.value, name),
    typeLabel: (type) => (TYPE_LABELS[type] ? t(TYPE_LABELS[type]) : type),
  }
}
