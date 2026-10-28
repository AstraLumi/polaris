// Keeping a book in this browser and sending back what was written without a
// connection (docs/EDITOR.md, "Offline writing"). Pure: the store
// (store.js) and the server calls come in, so it runs under node tests.
//
// api: { getBook(volume), saveBook(volume, settings), getText(id),
//        saveText(id, doc, baseVersion), saveVersion(id, label, doc),
//        comments(id) }
// A failed call throws { status } (0: the server couldn't be reached; a 409
// carries the stored text in `stored`).

import { mergeDocs } from './merge.js'
import { docPictures, bookFamilies } from './docRefs.js'

export const isOffline = (e) => !e || e.status === 0 || e.offline === true
const same = (a, b) => JSON.stringify(a ?? null) === JSON.stringify(b ?? null)

export function createSync({ store, api, story, normalize = (d) => d, mergeLabel = 'Before merging', now = () => Date.now() }) {
  // One chapter's text written here, sent with the version it was based on.
  // A 409 merges the two texts when they changed different blocks (this
  // browser's text is kept as a version first); otherwise the chapter waits
  // for the writer. Answers what happened.
  async function upload(rec) {
    try {
      const res = await api.saveText(rec.id, rec.doc, rec.version)
      await store.markSynced(story, rec.volume, rec.id, rec.doc, res?.version ?? rec.version, { words: res?.words ?? rec.words ?? 0, updated_at: res?.updated_at ?? '' })
      return 'saved'
    } catch (e) {
      if (isOffline(e)) return 'offline'
      if (e.status === 404) {
        await store.markMissing(story, rec.id)
        return 'missing'
      }
      if (e.status !== 409 || !e.stored) return 'failed'
      const theirs = normalize(e.stored.doc)
      const m = mergeDocs(normalize(rec.base_doc ?? rec.doc), normalize(rec.doc), theirs)
      if (!m.ok) {
        await store.markConflict(story, rec.id)
        return 'conflict'
      }
      try {
        // Written again meanwhile (another tab): next time.
        const now = await store.getChapter(story, rec.id)
        if (!now || !same(now.doc, rec.doc)) return 'failed'
        await api.saveVersion(rec.id, mergeLabel, rec.doc)
        const res = await api.saveText(rec.id, m.doc, e.stored.version || 0)
        await store.markSynced(story, rec.volume, rec.id, m.doc, res?.version ?? 0, { words: res?.words ?? 0, updated_at: res?.updated_at ?? '' }, { replace: true })
        return 'merged'
      } catch (e2) {
        return isOffline(e2) ? 'offline' : 'failed'
      }
    }
  }

  // Sends everything of the story written here: chapters (but the ones
  // `skip(id)` says the editor has open: it sends those itself) and book
  // settings changed without a connection (the last one saved wins).
  async function syncStory({ skip = () => false } = {}) {
    const out = { saved: [], merged: [], conflict: [], missing: [], failed: [], offline: false }
    for (const rec of await store.dirtyChapters(story)) {
      if (skip(rec.id)) continue
      const r = await upload(rec)
      if (r === 'offline') {
        out.offline = true
        return out
      }
      out[r].push(rec.id)
    }
    for (const b of await store.listBooks(story)) {
      if (!b.settings_dirty) continue
      const sent = b.book?.settings
      try {
        const res = await api.saveBook(b.volume, sent)
        await store.patchBook(story, b.volume, (old) =>
          same(old.book?.settings, sent) ? { settings_dirty: false, book: { ...old.book, settings: res?.settings || sent, is_set_up: true } } : {},
        )
      } catch (e) {
        if (isOffline(e)) {
          out.offline = true
          return out
        }
        out.failed.push(`book:${b.volume}`)
      }
    }
    return out
  }

  // Keeps a book (or brings the kept copy up to date): its settings and
  // chapter list, the text and comments of every chapter changed since
  // (chapters with text not yet sent are left as they are), what it needs
  // besides (pictures, font families). Chapters no longer in the book are
  // forgotten unless they hold unsent text. Answers the book's record.
  async function refreshBook(volume, { onProgress = () => {} } = {}) {
    const v = String(volume)
    const book = await api.getBook(v)
    const list = book.chapters || []
    const have = new Map((await store.chaptersOf(story, v)).map((c) => [c.id, c]))
    const ids = new Set(list.map((c) => c.id))
    const todo = list.filter((c) => {
      const r = have.get(c.id)
      return !r || (!r.dirty && (r.version !== (c.version || 0) || r.comments === undefined))
    })
    let done = 0
    onProgress(0, todo.length)
    for (const c of todo) {
      const r = have.get(c.id)
      if (!r || r.version !== (c.version || 0)) {
        const text = await api.getText(c.id)
        await store.putServerText(story, v, c.id, { ...text, doc: normalize(text.doc) })
      }
      let comments = null
      try {
        comments = await api.comments(c.id)
      } catch (e) {
        if (isOffline(e)) throw e
      }
      if (Array.isArray(comments)) await store.setComments(story, c.id, comments)
      onProgress(++done, todo.length)
    }
    for (const [id, r] of have) if (!ids.has(id) && !r.dirty) await store.removeChapter(story, id)

    const docs = (await store.chaptersOf(story, v)).filter((c) => ids.has(c.id)).map((c) => c.doc)
    const old = await store.getBook(story, v)
    const settings = old?.settings_dirty ? old.book.settings : book.settings
    const rec = {
      ...(old || {}),
      story,
      volume: v,
      title: book.title || '',
      book: { ...book, settings },
      pictures: docPictures(docs),
      families: bookFamilies(settings, docs),
      saved_at: now(),
    }
    await store.putBook(rec)
    return rec
  }

  return { upload, syncStory, refreshBook }
}
