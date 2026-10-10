// The story's chapters, loaded once and shared by every page that labels or
// picks one (events, character versions). Call loadChapters() again after
// changing them.
import { ref, computed } from 'vue'
import { t } from './i18n'
import { chaptersApi } from './api'

export const chapterIndex = ref({ volumes: [], chapters: [] })
let loading = null

export function loadChapters(force = false) {
  if (!loading || force) {
    loading = chaptersApi
      .list()
      .then((idx) => (chapterIndex.value = idx))
      .catch(() => chapterIndex.value)
  }
  return loading
}

const volumeTitle = (id) => chapterIndex.value.volumes.find((v) => v.id === id)?.title || ''

// "Book One · Ch. 3: Ashes", or "Ch. 3: Ashes" without a volume.
export function chapterLabel(id, { short = false } = {}) {
  const c = chapterIndex.value.chapters.find((x) => x.id === id)
  if (!c) return ''
  const num = t('Ch. {n}', { n: c.number })
  const head = c.volume_id ? `${volumeTitle(c.volume_id)} · ${num}` : num
  return short ? head : `${head}: ${c.title}`
}

// For a <select>: chapters grouped by volume, in reading order.
export const chapterGroups = computed(() => {
  const { volumes, chapters } = chapterIndex.value
  const groups = [{ id: null, title: '', chapters: chapters.filter((c) => !c.volume_id) }]
  for (const v of volumes) groups.push({ id: v.id, title: v.title, chapters: chapters.filter((c) => c.volume_id === v.id) })
  return groups.filter((g) => g.chapters.length)
})
