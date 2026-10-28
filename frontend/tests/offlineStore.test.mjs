// The books kept for writing offline (src/offline/store.js) and sending back
// what was written (src/offline/sync.js), on the in-memory storage
// (src/offline/kv.js) and a fake server. node frontend/tests/offlineStore.test.mjs
import assert from 'node:assert/strict'
import { memoryKV } from '../src/offline/kv.js'
import { createOfflineStore } from '../src/offline/store.js'
import { createSync } from '../src/offline/sync.js'

const p = (text) => ({ type: 'paragraph', content: [{ type: 'text', text }] })
const doc = (...t) => ({ type: 'doc', content: t.map(p) })
const texts = (d) => d.content.map((n) => n.content[0].text)

// A fake server: one story's books and chapters, with the version check.
function fakeServer() {
  const s = {
    offline: false,
    calls: [],
    books: { 1: { volume: 1, title: 'Book One', settings: { page_type: 'a5', styles: [{ id: 'normal', text: { font: 'Lora' } }] } } },
    chapters: {
      10: { volume: 1, doc: doc('a', 'b', 'c'), version: 3, comments: [{ id: 1, text: 'note' }] },
      11: { volume: 1, doc: { type: 'doc', content: [{ type: 'image', attrs: { src: '/uploads/s/S1/editor/x.png' } }] }, version: 1, comments: [] },
      20: { volume: 2, doc: doc('other'), version: 1, comments: [] },
    },
    versions: [],
  }
  const fail = (status, extra = {}) => Object.assign(new Error(`status ${status}`), { status, ...extra })
  const guard = (name) => {
    s.calls.push(name)
    if (s.offline) throw fail(0)
  }
  s.api = {
    async getBook(v) {
      guard(`getBook ${v}`)
      const b = s.books[v]
      if (!b) throw fail(404)
      const chapters = Object.entries(s.chapters)
        .filter(([, c]) => String(c.volume) === String(v))
        .map(([id, c]) => ({ id: +id, title: `Ch ${id}`, version: c.version, words: 1 }))
      return structuredClone({ ...b, chapters })
    },
    async saveBook(v, settings) {
      guard(`saveBook ${v}`)
      s.books[v].settings = settings
      return { settings }
    },
    async getText(id) {
      guard(`getText ${id}`)
      const c = s.chapters[id]
      return structuredClone({ doc: c.doc, version: c.version, words: 3, updated_at: '2026-10-28 10:00:00' })
    },
    async saveText(id, d, base) {
      guard(`saveText ${id}`)
      const c = s.chapters[id]
      if (!c) throw fail(404)
      if (c.version !== base) throw fail(409, { stored: structuredClone({ doc: c.doc, version: c.version }) })
      c.doc = structuredClone(d)
      c.version++
      return { version: c.version, words: 9, updated_at: 'now' }
    },
    async saveVersion(id, label, d) {
      guard(`saveVersion ${id}`)
      s.versions.push({ id, label, doc: structuredClone(d) })
    },
    async comments(id) {
      guard(`comments ${id}`)
      return structuredClone(s.chapters[id].comments)
    },
  }
  return s
}

let clock = 1000
const store = createOfflineStore(memoryKV(), () => clock)
const server = fakeServer()
const sync = createSync({ store, api: server.api, story: 'S1', mergeLabel: 'merged', now: () => clock })

// ---- Keeping a book ----
{
  const progress = []
  const rec = await sync.refreshBook(1, { onProgress: (d, t) => progress.push(`${d}/${t}`) })
  assert.equal(rec.title, 'Book One')
  assert.equal(rec.volume, '1')
  assert.deepEqual(rec.pictures, ['/uploads/s/S1/editor/x.png'])
  assert.deepEqual(rec.families, ['Lora'])
  assert.deepEqual(progress, ['0/2', '1/2', '2/2'])
  const ch = await store.getChapter('S1', 10)
  assert.deepEqual(texts(ch.doc), ['a', 'b', 'c'])
  assert.equal(ch.version, 3)
  assert.deepEqual(ch.comments, [{ id: 1, text: 'note' }])
  assert.equal(await store.getChapter('S1', 20), null, 'only the kept book')
  assert.equal((await store.listBooks('S1')).length, 1)
  assert.equal((await store.listBooks('S2')).length, 0, 'by story')

  // Up to date: nothing fetched again.
  server.calls = []
  await sync.refreshBook(1)
  assert.deepEqual(server.calls, ['getBook 1'])
  // A chapter changed on the server comes again.
  server.chapters[10].version = 4
  server.chapters[10].doc = doc('a', 'b', 'c', 'd')
  server.calls = []
  await sync.refreshBook(1)
  assert.deepEqual(server.calls, ['getBook 1', 'getText 10', 'comments 10'])
  assert.equal((await store.getChapter('S1', 10)).version, 4)
}

// ---- The story's own things: only while it has a book here ----
assert.equal(await store.keepForStory('S1', 'names', [{ name: 'Mira' }]), true)
assert.equal(await store.keepForStory('S2', 'names', [{ name: 'Other' }]), false)
assert.deepEqual((await store.getStory('S1')).names, [{ name: 'Mira' }])
assert.equal(await store.getStory('S2'), null)

// ---- Writing offline ----
{
  server.offline = true
  clock = 2000
  await store.markEdited('S1', 1, 10, doc('a', 'B (offline)', 'c', 'd'))
  let ch = await store.getChapter('S1', 10)
  assert.equal(ch.dirty, true)
  assert.deepEqual(texts(ch.base_doc), ['a', 'b', 'c', 'd'], 'the server text it started from')
  assert.equal(ch.edited_at, 2000)
  await store.markEdited('S1', 1, 10, doc('a', 'B (offline, more)', 'c', 'd'))
  ch = await store.getChapter('S1', 10)
  assert.deepEqual(texts(ch.base_doc), ['a', 'b', 'c', 'd'], 'the base stays while unsent')
  // A refresh never overwrites unsent text.
  await store.putServerText('S1', 1, 10, { doc: doc('z'), version: 9 })
  assert.deepEqual(texts((await store.getChapter('S1', 10)).doc), ['a', 'B (offline, more)', 'c', 'd'])
  // No connection: the sync stops at once.
  const r = await sync.syncStory()
  assert.equal(r.offline, true)
  assert.equal((await store.dirtyChapters('S1')).length, 1)
}

// ---- Back online: sent with the version check ----
{
  server.offline = false
  const r = await sync.syncStory()
  assert.deepEqual(r.saved, [10])
  assert.deepEqual(texts(server.chapters[10].doc), ['a', 'B (offline, more)', 'c', 'd'])
  const ch = await store.getChapter('S1', 10)
  assert.equal(ch.dirty, false)
  assert.equal(ch.version, 5)
  assert.equal(ch.base_doc, undefined)
}

// ---- Changed elsewhere too, in other paragraphs: merged ----
{
  await store.markEdited('S1', 1, 10, doc('A (offline)', 'B (offline, more)', 'c', 'd'))
  server.chapters[10].doc = doc('a', 'B (offline, more)', 'c', 'D (elsewhere)')
  server.chapters[10].version = 6
  const r = await sync.syncStory()
  assert.deepEqual(r.merged, [10])
  assert.deepEqual(texts(server.chapters[10].doc), ['A (offline)', 'B (offline, more)', 'c', 'D (elsewhere)'])
  assert.equal(server.versions.length, 1, 'this browser\'s text is kept as a version first')
  assert.equal(server.versions[0].label, 'merged')
  assert.deepEqual(texts(server.versions[0].doc), ['A (offline)', 'B (offline, more)', 'c', 'd'])
  const ch = await store.getChapter('S1', 10)
  assert.equal(ch.dirty, false)
  assert.deepEqual(texts(ch.doc), ['A (offline)', 'B (offline, more)', 'c', 'D (elsewhere)'])
  assert.equal(ch.version, 7)
}

// ---- Changed elsewhere in the same paragraph: waits for the writer ----
{
  await store.markEdited('S1', 1, 10, doc('A (mine)', 'B (offline, more)', 'c', 'D (elsewhere)'))
  server.chapters[10].doc = doc('A (theirs)', 'B (offline, more)', 'c', 'D (elsewhere)')
  server.chapters[10].version = 8
  const r = await sync.syncStory()
  assert.deepEqual(r.conflict, [10])
  const ch = await store.getChapter('S1', 10)
  assert.equal(ch.dirty, true)
  assert.equal(ch.conflict, true)
  assert.deepEqual(texts(server.chapters[10].doc), ['A (theirs)', 'B (offline, more)', 'c', 'D (elsewhere)'], 'the server text is untouched')
  // The editor has the chapter open: the sync leaves it to the editor.
  server.calls = []
  const r2 = await sync.syncStory({ skip: (id) => id === 10 })
  assert.deepEqual(server.calls, [])
  assert.deepEqual(r2.conflict, [])
  // The writer chose (the editor saved their text on the server's version).
  await store.markSynced('S1', 1, 10, doc('A (mine)', 'B (offline, more)', 'c', 'D (elsewhere)'), 9)
  assert.equal((await store.getChapter('S1', 10)).conflict, false)
}

// ---- A newer text kept while a save was on its way stays to be sent ----
{
  await store.markEdited('S1', 1, 10, doc('one'))
  await store.markEdited('S1', 1, 10, doc('two'))
  await store.markSynced('S1', 1, 10, doc('one'), 10)
  const ch = await store.getChapter('S1', 10)
  assert.equal(ch.dirty, true)
  assert.deepEqual(texts(ch.doc), ['two'])
  assert.deepEqual(texts(ch.base_doc), ['one'])
  assert.equal(ch.version, 10)
  await store.markSynced('S1', 1, 10, doc('two'), 11)
  assert.equal((await store.getChapter('S1', 10)).dirty, false)
}

// ---- A chapter added after the book was kept ----
{
  assert.equal(await store.markEdited('S1', 1, 99, doc('x')), null, 'no record and no base: nothing')
  await store.markEdited('S1', 1, 99, doc('x'), { doc: doc(), version: 0 })
  const ch = await store.getChapter('S1', 99)
  assert.equal(ch.dirty, true)
  assert.equal(ch.volume, '1')
  // Deleted on the server meanwhile: kept here, marked.
  const r = await sync.syncStory()
  assert.deepEqual(r.missing, [99])
  assert.equal((await store.getChapter('S1', 99)).missing, true)
  // A refresh keeps a chapter with unsent text even when the book no longer lists it.
  await sync.refreshBook(1)
  assert.ok(await store.getChapter('S1', 99))
  await store.removeChapter('S1', 99)
}

// ---- Book settings changed offline: sent on reconnect ----
{
  const changed = { page_type: 'a5', styles: [{ id: 'normal', text: { font: 'Cardo' } }] }
  await store.patchBook('S1', 1, (old) => ({ settings_dirty: true, book: { ...old.book, settings: changed } }))
  // A refresh keeps the unsent settings.
  const rec = await sync.refreshBook(1)
  assert.deepEqual(rec.book.settings, changed)
  assert.deepEqual(rec.families, ['Cardo'])
  await sync.syncStory()
  assert.deepEqual(server.books[1].settings, changed)
  assert.equal((await store.getBook('S1', 1)).settings_dirty, false)
}

// ---- Forgetting a book ----
{
  const left = await store.removeBook('S1', 1)
  assert.deepEqual(left, [])
  assert.equal(await store.getChapter('S1', 10), null)
  assert.equal(await store.getStory('S1'), null, "the story's things go with its last book")
}

console.log('offlineStore: ok')
