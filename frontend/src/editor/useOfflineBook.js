// Offline writing on the writing page (docs/EDITOR.md, "Offline writing"):
// whether this book is kept in the browser, keeping it (texts, settings,
// comments, the story's names and words, fonts, pictures, dictionary) and
// forgetting it, the chapters' records the editor reads and writes, and
// sending back what was written without a connection. The logic is in
// src/offline/ (store, sync, merge); this joins it to the page.
import { ref, computed } from 'vue'
import { t, locale } from '../i18n'
import { offlineStore as store } from '../offline/index.js'
import { createSync } from '../offline/sync.js'
import { connected } from '../offline/connection.js'
import { offlineSupported, keepFiles, pruneFiles, keepGoogleFont, pruneDictionaries, askPersistence, isPersisted, storyCache, DICT_CACHE } from '../offline/caches.js'
import { findFamily } from '../fontRules.js'
import { adoptFontLibrary } from '../fontLibrary.js'
import { googleSheets } from './fonts.js'
import { spellLanguage, dictionaryUrl, DICT_REVISION } from './spellWords.js'
import { booksApi, textApi, commentsApi, spellingApi } from './api.js'
import { wikiApi } from '../wiki'

async function fetchLibrary() {
  const res = await fetch('/api/fonts')
  if (!res.ok) throw Object.assign(new Error('failed to load the fonts'), { status: res.status })
  return res.json()
}

export function useOfflineBook({ story, volume, normalize, storyInfo = () => null }) {
  const supported = offlineSupported()
  const record = ref(null) // this book's record, null when not kept
  const available = computed(() => !!record.value)
  const books = ref([]) // every book of the story kept here
  const busy = ref(false)
  const progress = ref(null) // { done, total } while keeping
  const error = ref('')
  const dirty = ref([]) // chapter records of the story with text not sent yet
  const persisted = ref(null)

  const engine = () =>
    createSync({
      store,
      story,
      normalize,
      mergeLabel: t('Before merging with changes made elsewhere'),
      api: {
        getBook: (v) => booksApi.get(v),
        saveBook: (v, s) => booksApi.save(v, s),
        getText: (id) => textApi.get(id),
        saveText: (id, doc, base) => textApi.save(id, doc, base),
        saveVersion: (id, label, doc) => textApi.saveVersion(id, label, doc),
        comments: (id) => commentsApi.list(id),
      },
    })

  async function recount() {
    try {
      dirty.value = await store.dirtyChapters(story)
      books.value = await store.listBooks(story)
    } catch (e) {
      dirty.value = []
    }
  }

  async function load() {
    try {
      record.value = supported ? await store.getBook(story, volume()) : null
    } catch (e) {
      record.value = null
    }
    persisted.value = await isPersisted()
    await recount()
    return record.value
  }

  // A value the page loads from the server (the story's names, its
  // dictionary words), kept while the story has a book here and answered
  // from there when the server can't be reached.
  async function cached(key, fetcher) {
    try {
      const value = await fetcher()
      store.keepForStory(story, key, value).catch(() => {})
      return value
    } catch (e) {
      const kept = await store.getStory(story).catch(() => null)
      if (kept && kept[key] !== undefined) return kept[key]
      throw e
    }
  }

  // The fonts library as kept, when the server can't give it.
  async function adoptLibrary() {
    const kept = await store.getStory(story).catch(() => null)
    if (kept?.library) adoptFontLibrary(kept.library)
  }

  // The files the book needs besides its texts, in the browser's cache.
  async function keepAssets(rec) {
    const library = (await store.getStory(story))?.library || []
    const fonts = rec.families.flatMap((f) => (findFamily(library, f)?.files || []).map((x) => x.url)).filter(Boolean)
    await store.patchBook(story, rec.volume, { fonts })
    const failed = await keepFiles(storyCache(story), [...rec.pictures, ...fonts], (done, total) => (progress.value = { done, total, what: 'files' }))
    const keep = (await store.listBooks(story)).flatMap((b) => [...(b.pictures || []), ...(b.fonts || [])])
    await pruneFiles(storyCache(story), keep)
    // Google Fonts: the book's, and the interface's (index.html).
    for (const f of rec.families) {
      const [full, plain] = googleSheets(f, library)
      if (full && !(await keepGoogleFont(full))) await keepGoogleFont(plain)
    }
    for (const l of document.querySelectorAll('link[rel="stylesheet"][href^="https://fonts.googleapis.com/"]')) await keepGoogleFont(l.href)
    const code = spellLanguage(rec.book?.settings?.meta?.language, locale.value).code
    if (code) await keepFiles(DICT_CACHE, [dictionaryUrl(code, 'aff'), dictionaryUrl(code, 'dic')])
    await pruneDictionaries(DICT_REVISION)
    return failed
  }

  // Keeps the book, or brings the kept copy up to date (only what changed).
  async function refresh({ quiet = false } = {}) {
    if (!supported || busy.value) return null
    busy.value = true
    if (!quiet) error.value = ''
    try {
      const rec = await engine().refreshBook(volume(), { onProgress: (done, total) => (progress.value = { done, total, what: 'chapters' }) })
      record.value = rec
      // What the editor needs besides the book.
      await cached('names', () => wikiApi.list()).catch(() => {})
      await cached('spelling', () => spellingApi.words()).catch(() => {})
      await cached('library', fetchLibrary).catch(() => {})
      const info = storyInfo()
      if (info) await store.keepForStory(story, 'info', info).catch(() => {})
      const failed = await keepAssets(rec)
      if (failed.length && !quiet) error.value = t('{n} pictures or fonts could not be kept; they show when the server is back.', { n: failed.length })
      record.value = await store.getBook(story, volume())
      return record.value
    } catch (e) {
      if (!quiet) error.value = e.message || t("can't reach the server")
      return null
    } finally {
      busy.value = false
      progress.value = null
      await recount()
    }
  }

  async function enable() {
    if (!supported) return
    persisted.value = await askPersistence()
    await refresh()
  }

  // Forgets the book here (the page asks first when some of its text isn't
  // on the server yet).
  async function disable() {
    busy.value = true
    try {
      const left = await store.removeBook(story, volume())
      record.value = null
      const keep = left.flatMap((b) => [...(b.pictures || []), ...(b.fonts || [])])
      await pruneFiles(storyCache(story), left.length ? keep : null)
    } finally {
      busy.value = false
      await recount()
    }
  }

  let running = null
  // Sends what was written here; `skip(id)`: chapters the editor sends itself.
  function sync(skip = () => false) {
    if (!supported) return Promise.resolve(null)
    running ||= (async () => {
      try {
        return await engine().syncStory({ skip })
      } catch (e) {
        return null
      } finally {
        running = null
        await recount()
      }
    })()
    return running
  }
  const idle = () => running || Promise.resolve()

  const v = () => volume()
  return {
    supported,
    record,
    available,
    books,
    busy,
    progress,
    error,
    dirty,
    persisted,
    connected,
    pendingHere: computed(() => dirty.value.filter((c) => c.volume === v())),
    conflicts: computed(() => dirty.value.filter((c) => c.conflict || c.missing)),
    load,
    refresh,
    enable,
    disable,
    sync,
    idle,
    cached,
    adoptLibrary,
    recount,
    // The editor's side of a chapter's record.
    chapter: (id) => (available.value ? store.getChapter(story, id).catch(() => null) : Promise.resolve(null)),
    edited: async (id, doc, base) => {
      if (!available.value) return null
      const rec = await store.markEdited(story, v(), id, doc, base).catch(() => null)
      if (rec && !dirty.value.some((c) => c.id === id)) recount()
      return rec
    },
    saved: async (id, doc, res) => {
      if (!available.value || !res) return
      await store.markSynced(story, v(), id, doc, res.version ?? 0, { words: res.words ?? 0, updated_at: res.updated_at ?? '' }).catch(() => {})
      recount()
    },
    // The editor holds this text, on the server as `version` (a conflict
    // settled, a copy discarded).
    settle: async (id, doc, version) => {
      if (!available.value) return
      await store.markSynced(story, v(), id, doc, version ?? 0, {}, { replace: true }).catch(() => {})
      recount()
    },
    comments: async (id, list) => {
      if (available.value) await store.setComments(story, id, list).catch(() => {})
    },
    settingsEdited: async (settings) => {
      if (!available.value) return false
      record.value = await store.patchBook(story, v(), (old) => ({ settings_dirty: true, book: { ...old.book, settings, is_set_up: true } }))
      return true
    },
    // The server has these settings: the kept copy follows.
    settingsSaved: async (settings) => {
      if (!available.value) return
      record.value = (await store.patchBook(story, v(), (old) => ({ settings_dirty: false, book: { ...old.book, settings, is_set_up: true } })).catch(() => null)) || record.value
    },
  }
}
