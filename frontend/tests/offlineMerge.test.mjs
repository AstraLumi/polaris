// Merging two edits of a chapter (src/offline/merge.js) and what a kept book
// needs besides its texts (src/offline/docRefs.js).
// node frontend/tests/offlineMerge.test.mjs
import assert from 'node:assert/strict'
import { mergeDocs, hunks, clash } from '../src/offline/merge.js'
import { docPictures, bookFamilies, googleFontFiles } from '../src/offline/docRefs.js'

const p = (text, attrs) => ({ type: 'paragraph', ...(attrs ? { attrs } : {}), content: [{ type: 'text', text }] })
const doc = (...texts) => ({ type: 'doc', content: texts.map((t) => (typeof t === 'string' ? p(t) : t)) })
const texts = (d) => d.content.map((n) => n.content?.[0]?.text ?? n.type)

// ---- hunks ----
assert.deepEqual(hunks(['a', 'b', 'c'], ['a', 'b', 'c']), [])
assert.deepEqual(hunks(['a', 'b', 'c'], ['a', 'B', 'c']), [{ start: 1, end: 2, items: ['B'] }])
assert.deepEqual(hunks(['a', 'b'], ['a', 'x', 'b']), [{ start: 1, end: 1, items: ['x'] }])
assert.deepEqual(hunks(['a', 'b', 'c'], ['a', 'c']), [{ start: 1, end: 2, items: [] }])
assert.deepEqual(hunks(['a', 'b', 'c', 'd', 'e'], ['A', 'b', 'c', 'd', 'E']), [
  { start: 0, end: 1, items: ['A'] },
  { start: 4, end: 5, items: ['E'] },
])
assert.deepEqual(hunks([], ['a']), [{ start: 0, end: 0, items: ['a'] }])

// ---- clash ----
assert.equal(clash({ start: 1, end: 2, items: ['x'] }, { start: 1, end: 2, items: ['y'] }), true)
assert.equal(clash({ start: 1, end: 2, items: ['x'] }, { start: 1, end: 2, items: ['x'] }), false, 'the same edit on both sides')
assert.equal(clash({ start: 1, end: 2, items: ['x'] }, { start: 2, end: 3, items: ['y'] }), false, 'neighbouring blocks')
assert.equal(clash({ start: 2, end: 2, items: ['x'] }, { start: 2, end: 2, items: ['y'] }), true, 'both add at the same place')
assert.equal(clash({ start: 2, end: 2, items: ['x'] }, { start: 2, end: 3, items: ['y'] }), false, 'added just before a changed block')
assert.equal(clash({ start: 2, end: 2, items: ['x'] }, { start: 1, end: 3, items: [] }), true, 'added inside a deleted run')

// ---- mergeDocs ----
const base = doc('one', 'two', 'three', 'four', 'five')

// Different paragraphs: both kept.
{
  const mine = doc('one', 'TWO (mine)', 'three', 'four', 'five')
  const theirs = doc('one', 'two', 'three', 'four', 'FIVE (theirs)')
  const m = mergeDocs(base, mine, theirs)
  assert.equal(m.ok, true)
  assert.deepEqual(texts(m.doc), ['one', 'TWO (mine)', 'three', 'four', 'FIVE (theirs)'])
}
// Neighbouring paragraphs.
{
  const m = mergeDocs(base, doc('one', 'two', 'THREE', 'four', 'five'), doc('one', 'two', 'three', 'FOUR', 'five'))
  assert.deepEqual(texts(m.doc), ['one', 'two', 'THREE', 'FOUR', 'five'])
}
// Added and deleted paragraphs on each side.
{
  const mine = doc('one', 'new after one', 'two', 'three', 'four', 'five')
  const theirs = doc('one', 'two', 'three', 'five', 'the end')
  const m = mergeDocs(base, mine, theirs)
  assert.equal(m.ok, true)
  assert.deepEqual(texts(m.doc), ['one', 'new after one', 'two', 'three', 'five', 'the end'])
}
// The same paragraph changed on both sides: a conflict.
{
  const m = mergeDocs(base, doc('one', 'two!', 'three', 'four', 'five'), doc('one', 'two?', 'three', 'four', 'five'))
  assert.equal(m.ok, false)
  assert.equal(m.clashes, 1)
}
// The same change on both sides is no conflict.
{
  const both = doc('one', 'two', 'THREE', 'four', 'five')
  const m = mergeDocs(base, both, both)
  assert.deepEqual(texts(m.doc), texts(both))
  const m2 = mergeDocs(base, doc('ONE', 'two', 'THREE', 'four', 'five'), both)
  assert.deepEqual(texts(m2.doc), ['ONE', 'two', 'THREE', 'four', 'five'])
}
// One side unchanged: the other wins as is.
{
  const theirs = doc('x')
  assert.equal(mergeDocs(base, base, theirs).doc, theirs)
  assert.equal(mergeDocs(base, theirs, base).doc, theirs)
}
// Both add a paragraph at the same place: which first? A conflict.
assert.equal(mergeDocs(base, doc('one', 'a', 'two', 'three', 'four', 'five'), doc('one', 'b', 'two', 'three', 'four', 'five')).ok, false)
// One deletes a paragraph the other changed.
assert.equal(mergeDocs(base, doc('one', 'three', 'four', 'five'), doc('one', 'two (edited)', 'three', 'four', 'five')).ok, false)
// Any block counts as one, whatever its type (a table, a list, a node this
// code has never heard of).
{
  const table = { type: 'table', content: [{ type: 'table_row', content: [{ type: 'table_cell', content: [p('cell')] }] }] }
  const future = { type: 'footnote_like', attrs: { n: 1 }, content: [p('note')] }
  const b = doc('one', table, future)
  const mine = doc('ONE', table, future)
  const theirs = doc('one', table, { ...future, attrs: { n: 2 } })
  const m = mergeDocs(b, mine, theirs)
  assert.equal(m.ok, true)
  assert.deepEqual(m.doc.content, [p('ONE'), table, { ...future, attrs: { n: 2 } }])
}
// The doc's own attrs (a chapter's mode) merge as one value.
{
  const b = { ...doc('a', 'b'), attrs: { mode: 'prose' } }
  const mine = { ...doc('A', 'b'), attrs: { mode: 'prose' } }
  const theirs = { ...doc('a', 'B'), attrs: { mode: 'script' } }
  const m = mergeDocs(b, mine, theirs)
  assert.deepEqual(m.doc.attrs, { mode: 'script' })
  assert.deepEqual(texts(m.doc), ['A', 'B'])
  assert.equal(mergeDocs(b, { ...mine, attrs: { mode: 'x' } }, theirs).ok, false)
}
// Repeated blocks (empty paragraphs, scene breaks) still merge in place.
{
  const sb = { type: 'scene_break' }
  const b = doc('a', sb, 'b', sb, 'c')
  const m = mergeDocs(b, doc('A', sb, 'b', sb, 'c'), doc('a', sb, 'b', sb, 'C'))
  assert.deepEqual(texts(m.doc), ['A', 'scene_break', 'b', 'scene_break', 'C'])
}
// A long chapter edited at both ends.
{
  const many = Array.from({ length: 2000 }, (_, i) => `p${i}`)
  const b = doc(...many)
  const mine = doc(...many.map((t, i) => (i === 3 ? 'mine' : t)))
  const theirs = doc(...many.map((t, i) => (i === 1990 ? 'theirs' : t)))
  const m = mergeDocs(b, mine, theirs)
  assert.equal(m.ok, true)
  assert.equal(texts(m.doc)[3], 'mine')
  assert.equal(texts(m.doc)[1990], 'theirs')
  assert.equal(m.doc.content.length, 2000)
}
// Empty docs.
assert.equal(mergeDocs({ type: 'doc', content: [] }, doc('a'), { type: 'doc', content: [] }).ok, true)

// ---- docRefs ----
{
  const img = (src) => ({ type: 'image', attrs: { src } })
  const d1 = doc('x', img('/uploads/s/abc/editor/a.png'), img('https://example.com/b.png'))
  const d2 = { type: 'doc', content: [{ type: 'table', content: [{ type: 'row', content: [img('/uploads/s/abc/editor/c.png'), img('/uploads/s/abc/editor/a.png')] }] }] }
  assert.deepEqual(docPictures([d1, d2]), ['/uploads/s/abc/editor/a.png', '/uploads/s/abc/editor/c.png'])

  const settings = {
    styles: [{ id: 'normal', text: { font: 'Lora' } }, { id: 'h1', text: { font: 'Cinzel' } }, { id: 'q', text: {} }],
    layout: { header: { odd: { font: 'Crimson Pro' }, even: { font: null } }, footer: { odd: {} } },
  }
  const marked = doc({ type: 'paragraph', content: [{ type: 'text', text: 'x', marks: [{ type: 'font', attrs: { family: 'Special Elite' } }] }] })
  assert.deepEqual(bookFamilies(settings, [marked]), ['Lora', 'Cinzel', 'Crimson Pro', 'Special Elite'])
  assert.deepEqual(bookFamilies(null), [])

  const css = `/* cyrillic */
@font-face { font-family: 'Lora'; src: url(https://fonts.gstatic.com/s/lora/v1/cyr.woff2) format('woff2'); }
/* latin-ext */
@font-face { font-family: 'Lora'; src: url(https://fonts.gstatic.com/s/lora/v1/ext.woff2) format('woff2'); }
/* latin */
@font-face { font-family: 'Lora'; src: url(https://fonts.gstatic.com/s/lora/v1/lat.woff2) format('woff2'); }`
  assert.deepEqual(googleFontFiles(css), ['https://fonts.gstatic.com/s/lora/v1/ext.woff2', 'https://fonts.gstatic.com/s/lora/v1/lat.woff2'])
  assert.deepEqual(googleFontFiles("@font-face { src: url('https://x/a.ttf') }"), ['https://x/a.ttf'])
}

console.log('offlineMerge: ok')
