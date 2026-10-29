// The Format Painter (src/editor/formatPainter.js): what is copied (looks,
// not links or comments), what is painted and when the paragraph comes too.
// node frontend/tests/editorFormatPainter.test.mjs
import assert from 'node:assert/strict'
import { EditorState, TextSelection } from 'prosemirror-state'
import { schema, docFromJSON } from '../src/editor/schema.js'
import { copyFormat, applyFormat, FORMAT_MARKS } from '../src/editor/formatPainter.js'

const t = (text, ...marks) => ({ type: 'text', text, marks: marks.length ? marks.map((m) => (typeof m === 'string' ? { type: m } : m)) : undefined })
const p = (attrs, ...content) => ({ type: 'paragraph', attrs, content })
const stateOf = (...blocks) => EditorState.create({ schema, doc: docFromJSON({ type: 'doc', content: blocks }) })
// Selects text within the nth text block, by its offsets.
function select(state, block, a, b = a) {
  let pos = null
  let n = 0
  state.doc.descendants((node, nodePos) => {
    if (node.isTextblock && n++ === block) pos = nodePos + 1
  })
  return state.apply(state.tr.setSelection(TextSelection.create(state.doc, pos + a, pos + b)))
}
const marksOf = (state, block) => {
  const out = []
  let n = 0
  state.doc.descendants((node) => {
    if (node.isTextblock && n++ === block) node.forEach((c) => out.push(`${c.text}:${c.marks.map((m) => m.type.name).sort().join('+')}`))
  })
  return out
}
const paint = (state, fmt) => {
  const tr = applyFormat(state, fmt)
  return tr ? state.apply(tr) : null
}

const src = stateOf(
  p({ align: 'center', indentFirst: 12 }, t('Fancy', 'bold', { type: 'color', attrs: { color: '#ff0000' } }, { type: 'link', attrs: { href: 'https://x.test' } }), t(' plain')),
  p({}, t('Target words', 'italic', { type: 'comment', attrs: { id: 3 } })),
  p({}, t('Second')),
)

// Copy: only the looks.
const fmt = copyFormat(select(src, 0, 1))
assert.deepEqual(fmt.marks.map((m) => m.type).sort(), ['bold', 'color'], 'the link is not copied')
assert.equal(fmt.para.align, 'center')
assert.equal(fmt.para.indentFirst, 12)
assert.equal(fmt.whole, false)
assert.ok(FORMAT_MARKS.every((n) => schema.marks[n]), 'every format mark exists in the schema')
for (const n of ['comment', 'speaker', 'link', 'story_link']) assert.ok(!FORMAT_MARKS.includes(n), n + ' is not a look')

// A caret copies the marks it would type with.
assert.deepEqual(copyFormat(select(src, 1, 3)).marks.map((m) => m.type), ['italic'])
// A whole paragraph's text copied is "whole".
assert.equal(copyFormat(select(src, 2, 0, 6)).whole, true)

// Paint a stretch: the old looks go, the comment stays, the paragraph is untouched.
let s = paint(select(src, 1, 0, 6), fmt)
assert.deepEqual(marksOf(s, 1), ['Target:bold+color+comment', ' words:comment+italic'])
assert.equal(s.doc.child(1).attrs.align, null, 'part of a paragraph: the paragraph keeps its look')

// A whole paragraph selected: its paragraph look is painted too.
s = paint(select(src, 1, 0, 12), fmt)
assert.deepEqual(marksOf(s, 1), ['Target words:bold+color+comment'])
assert.equal(s.doc.child(1).attrs.align, 'center')
assert.equal(s.doc.child(1).attrs.indentFirst, 12)

// A click paints the paragraph only.
s = paint(select(src, 2, 2), fmt)
assert.equal(s.doc.child(2).attrs.align, 'center')
assert.deepEqual(marksOf(s, 2), ['Second:'])

// A whole-paragraph source paints the paragraph even on a part.
const whole = copyFormat(select(src, 0, 0, 11))
assert.equal(whole.whole, true)
s = paint(select(src, 1, 0, 3), whole)
assert.equal(s.doc.child(1).attrs.align, 'center')

// A heading source makes a heading.
const h = stateOf({ type: 'heading', attrs: { level: 2 }, content: [t('Title')] }, p({}, t('Body')))
s = paint(select(h, 1, 0, 4), copyFormat(select(h, 0, 0, 5)))
assert.equal(s.doc.child(1).type.name, 'heading')
assert.equal(s.doc.child(1).attrs.level, 2)

// Nothing to change: no transaction.
assert.ok(applyFormat(select(src, 2, 1), { marks: [], para: { align: null }, level: null, whole: false }) === null)
assert.ok(applyFormat(src, null) === null)
console.log('format painter ok')
