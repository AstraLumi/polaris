// Comments on a chapter's text (stage 5): the comment mark in the schema and
// the pure logic of src/editor/comments.js (ranges, the panel's threads, the
// word a comment takes with nothing selected, highlights, pastes).
// node frontend/tests/comments.test.mjs
import assert from 'node:assert/strict'
import { EditorState } from 'prosemirror-state'
import { schema, docFromJSON, docToJSON } from '../src/editor/schema.js'
import { commentRanges, quoteOf, commentsAt, activeAt, wordRange, hasText, panelThreads, shownIds, highlightCSS, keepOnPaste } from '../src/editor/comments.js'
import { addCommentMark, removeCommentMark } from '../src/editor/commentPlugin.js'

const cm = (id) => ({ type: 'comment', attrs: { id } })
const text = (t, ...marks) => ({ type: 'text', text: t, ...(marks.length ? { marks } : {}) })
const para = (...content) => ({ type: 'paragraph', content })

// "The quick brown fox" / "jumps over" — 1 covers "quick brown fox|jumps",
// 2 overlaps "brown", 3 is "over" (resolved).
const json = {
  type: 'doc',
  content: [
    para(text('The '), text('quick ', cm(1)), text('brown', cm(1), cm(2)), text(' fox', { type: 'italic' }, cm(1))),
    para(text('jumps', cm(1)), text(' '), text('over', cm(3))),
  ],
}

// ---- The mark: several on one text, kept through JSON ---------------------------
const doc = docFromJSON(json)
const brown = doc.child(0).child(2)
assert.equal(brown.marks.filter((m) => m.type.name === 'comment').length, 2, 'two comments on one text')
assert.deepEqual(docToJSON(doc), json, 'comments come back as stored')
assert.equal(schema.marks.comment.spec.inclusive, false)
// Unknown attrs on the mark are kept.
const extra = docFromJSON({ type: 'doc', content: [para(text('x', { type: 'comment', attrs: { id: 5, author: 'A' } }))] })
assert.deepEqual(docToJSON(extra).content[0].content[0].marks, [{ type: 'comment', attrs: { id: 5, author: 'A' } }])

// ---- Ranges ----------------------------------------------------------------------
const ranges = commentRanges(doc)
assert.deepEqual([...ranges.keys()].sort(), [1, 2, 3])
const r1 = ranges.get(1)
assert.equal(doc.textBetween(r1.from, r1.to, '|'), 'quick brown fox|jumps')
assert.equal(r1.text, 'quick brown fox jumps')
assert.equal(ranges.get(2).text, 'brown')
assert.equal(quoteOf('  a\n  b  '), 'a b')
assert.equal(quoteOf('x'.repeat(10), 5), 'xxxx…')

// The comments at a place, innermost first; the one a selection points at.
const inBrown = ranges.get(2).from + 1
assert.deepEqual(commentsAt(ranges, inBrown), [2, 1])
assert.deepEqual(commentsAt(ranges, 1), [])
assert.equal(activeAt(ranges, inBrown, inBrown), 2)
assert.equal(activeAt(ranges, r1.from, r1.to), 1, 'a whole comment selected is that comment')
assert.equal(activeAt(ranges, inBrown, inBrown, new Set([1])), 1, 'only the comments shown')

// ---- The word a comment takes with nothing selected --------------------------------
const plain = docFromJSON({ type: 'doc', content: [para(text("Mira's cloak, torn.")), para()] })
const w = (pos) => {
  const r = wordRange(plain, pos)
  return r && plain.textBetween(r.from, r.to)
}
assert.equal(w(3), "Mira's")
assert.equal(w(10), 'cloak')
assert.equal(w(13), 'cloak', 'after a word: that word')
assert.equal(w(1), "Mira's")
assert.equal(wordRange(plain, plain.content.size - 1), null, 'an empty paragraph has none')
assert.equal(hasText(plain, 1, 4), true)
assert.equal(hasText(plain, plain.content.size - 1, plain.content.size - 1), false)

// ---- The panel ----------------------------------------------------------------------
const rows = [
  { id: 3, parent_id: null, text: 'Over what?', resolved: true },
  { id: 2, parent_id: null, text: 'Colour?', resolved: false },
  { id: 1, parent_id: null, text: 'Long sentence.', resolved: false },
  { id: 9, parent_id: 1, text: 'Second reply', resolved: false },
  { id: 7, parent_id: 1, text: 'First reply', resolved: false },
  { id: 4, parent_id: null, text: 'Its text was deleted', resolved: false },
]
let model = panelThreads(rows, ranges)
assert.deepEqual(model.threads.map((t) => t.comment.id), [1, 2], 'in the order of the text; resolved hidden')
assert.deepEqual(model.threads[0].replies.map((r) => r.id), [7, 9])
assert.equal(model.threads[0].quote, 'quick brown fox jumps')
assert.equal(model.detached, 1)
assert.equal(model.resolved, 1)
model = panelThreads(rows, ranges, { showResolved: true })
assert.deepEqual(model.threads.map((t) => t.comment.id), [1, 2, 3])
assert.deepEqual([...shownIds(rows)].sort(), [1, 2, 4], 'open comments only, no replies')

// ---- Highlights ------------------------------------------------------------------------
const css = highlightCSS(new Set([1, 2]), 2, '.x')
assert.match(css, /\.x span\.pm-comment\[data-comment="1"\],\n\.x span\.pm-comment\[data-comment="2"\] \{ background: var\(--comment\)/)
assert.match(css, /\.x span\.pm-comment\[data-comment="2"\] \{ background: var\(--comment-active\)/)
assert.equal(highlightCSS(new Set(), null, '.x'), '')
assert.ok(!highlightCSS(new Set([1]), 3, '.x').includes('comment-active'), 'a hidden comment is never the active highlight')

// ---- Pastes: a known comment that left the text comes back, others don't -------------------
assert.equal(keepOnPaste(1, new Set([1, 2]), new Set([2])), true)
assert.equal(keepOnPaste(2, new Set([1, 2]), new Set([2])), false, 'a copy: the comment is still in the text')
assert.equal(keepOnPaste(8, new Set([1, 2]), new Set()), false, "another chapter's comment")

// ---- Adding and taking off a mark ------------------------------------------------------
let state = EditorState.create({ schema, doc: plain })
state = state.apply(addCommentMark(state, 1, 7, 40))
assert.equal(commentRanges(state.doc).get(40).text, "Mira's")
state = state.apply(addCommentMark(state, 3, 13, 41))
assert.equal(commentRanges(state.doc).get(41).text, "ra's cloak")
state = state.apply(removeCommentMark(state, 40))
assert.deepEqual([...commentRanges(state.doc).keys()], [41], 'only that comment goes')

console.log('comments: ok')
