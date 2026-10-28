// The books kept in this browser for writing without a connection
// (docs/EDITOR.md, "Offline writing"). Pure apart from the storage it is
// given (kv.js), so it runs under node tests. Everything is keyed by story
// id first: the server holds many stories and this browser may keep books
// of several.
//
// books     "<story>:<volume>" → { story, volume, title, book (GET
//           /api/books/{volume} as it was, settings included), pictures,
//           fonts (addresses kept in the browser's cache), families,
//           saved_at, settings_dirty }
// chapters  "<story>:<chapter>" → { story, volume, id, doc, version,
//           words, updated_at, comments, dirty, base_doc, edited_at,
//           conflict, missing }
//           `version` is the server's version the record is based on.
//           While `dirty` (edited here, not yet on the server) `base_doc`
//           is the server's text at that version, for merging; `doc` is
//           always the newest text this browser has. Docs are stored as
//           the editor's JSON, whatever nodes they hold.
// stories   "<story>" → { names, spelling, library, info }: what the editor
//           needs besides the book (the story's names for links and the
//           spellchecker, the story's dictionary words, the fonts library,
//           the story's name), kept only while the story has a book here.

const same = (a, b) => JSON.stringify(a ?? null) === JSON.stringify(b ?? null)

export function createOfflineStore(kv, now = () => Date.now()) {
  const bookKey = (story, volume) => `${story}:${volume}`
  const chapterKey = (story, id) => `${story}:${id}`
  const values = async (table, prefix) => (await kv.entries(table, prefix)).map(([, v]) => v)

  const store = {
    // ---- Books ----
    getBook: (story, volume) => kv.get('books', bookKey(story, String(volume))),
    listBooks: (story) => values('books', `${story}:`),
    allBooks: () => values('books', ''),
    putBook: (rec) => kv.put('books', bookKey(rec.story, String(rec.volume)), { ...rec, volume: String(rec.volume) }),
    patchBook: (story, volume, patch) =>
      kv.update('books', bookKey(story, String(volume)), (old) => (old ? { ...old, ...(typeof patch === 'function' ? patch(old) : patch) } : undefined)),

    // Forgets a book and its chapters' texts (the caller asks first when
    // some aren't on the server yet). The story's own things go with its
    // last book. Answers what is left of the story's books.
    async removeBook(story, volume) {
      const v = String(volume)
      await kv.del('books', bookKey(story, v))
      for (const c of await store.chaptersOf(story, v)) await kv.del('chapters', chapterKey(story, c.id))
      const left = await store.listBooks(story)
      if (!left.length) await kv.del('stories', story)
      return left
    },

    // ---- Chapters ----
    getChapter: (story, id) => kv.get('chapters', chapterKey(story, id)),
    async chaptersOf(story, volume) {
      return (await values('chapters', `${story}:`)).filter((c) => c.volume === String(volume))
    },
    async dirtyChapters(story) {
      return (await values('chapters', `${story}:`)).filter((c) => c.dirty)
    },
    removeChapter: (story, id) => kv.del('chapters', chapterKey(story, id)),

    // The server's text of a chapter ({ doc, version, words, updated_at }).
    // A record with edits not yet sent keeps them (the upload settles it).
    putServerText: (story, volume, id, text) =>
      kv.update('chapters', chapterKey(story, id), (old) => {
        if (old?.dirty) return undefined
        return {
          ...(old || {}),
          story,
          volume: String(volume),
          id,
          doc: text.doc,
          version: text.version || 0,
          words: text.words || 0,
          updated_at: text.updated_at || '',
          dirty: false,
          conflict: false,
          missing: false,
          base_doc: undefined,
        }
      }),

    // The writer changed the text here. `base` ({ doc, version }) is the
    // server's text the editor started from, used when there is no record
    // yet (a chapter added after the book was kept).
    markEdited: (story, volume, id, doc, base) =>
      kv.update('chapters', chapterKey(story, id), (old) => {
        if (!old && !base) return undefined
        const rec = old || { story, volume: String(volume), id, doc: base.doc, version: base.version || 0, words: 0, updated_at: '' }
        if (rec.dirty && same(rec.doc, doc)) return undefined
        return {
          ...rec,
          base_doc: rec.dirty ? rec.base_doc : rec.doc,
          doc,
          dirty: true,
          edited_at: now(),
        }
      }),

    // `doc` is on the server now, as `version`. A newer text kept since
    // stays to be sent; otherwise the record is clean. `replace`: `doc` is
    // the text now whatever the record held (a merge of it).
    markSynced: (story, volume, id, doc, version, extra = {}, { replace = false } = {}) =>
      kv.update('chapters', chapterKey(story, id), (old) => {
        const rec = old || { story, volume: String(volume), id }
        const clean = replace || !rec.dirty || same(rec.doc, doc)
        return {
          ...rec,
          ...extra,
          version,
          doc: clean ? doc : rec.doc,
          base_doc: clean ? undefined : doc,
          dirty: !clean,
          conflict: false,
          missing: false,
        }
      }),

    // The upload found the chapter changed on the server in the same places
    // (or gone): the writer chooses when they open it.
    markConflict: (story, id) => kv.update('chapters', chapterKey(story, id), (old) => (old ? { ...old, conflict: true } : undefined)),
    markMissing: (story, id) => kv.update('chapters', chapterKey(story, id), (old) => (old ? { ...old, missing: true } : undefined)),
    setComments: (story, id, comments) => kv.update('chapters', chapterKey(story, id), (old) => (old ? { ...old, comments } : undefined)),

    // ---- The story's own things ----
    getStory: (story) => kv.get('stories', story),
    // Kept only while the story has a book here: nothing is stored for
    // stories nobody writes in offline.
    async keepForStory(story, key, value) {
      if (!(await store.listBooks(story)).length) return false
      await kv.update('stories', story, (old) => ({ ...(old || {}), [key]: value }))
      return true
    },
  }
  return store
}
