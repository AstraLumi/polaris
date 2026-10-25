// Character styles in the writing editor (src/editor/styles.js, commands.js,
// schema.js): the `char_style` mark, how a style's look is worked out (the
// text's own marks > its character style and what that is based on > its
// paragraph's style) and how the style picker applies and removes one.
// node frontend/tests/editorCharStyles.test.mjs
import assert from 'node:assert/strict'
import { EditorState, TextSelection } from 'prosemirror-state'
import { schema, docFromJSON, docToJSON } from '../src/editor/schema.js'
import { builtinStyles, resolveCharStyle, resolveBlock, styleSheet, basedOnChoices, newStyleId } from '../src/editor/styles.js'
import { setCharStyle, selectionFormat, clearMarks, toggleMark } from '../src/editor/commands.js'
import { charDeclarations } from '../src/editor/css.js'

const styles = [
  ...builtinStyles(10, { font: 'Merriweather' }),
  { id: 'loud', name: 'Loud', kind: 'character', based_on: null, builtin: false, text: { bold: true, color: '#c00000' } },
  { id: 'louder', name: 'Louder', kind: 'character', based_on: 'loud', builtin: false, text: { font: 'Impact', size: 16, bold: false, underline: true } },
  { id: 'loopA', name: 'A', kind: 'character', based_on: 'loopB', builtin: false, text: { italic: true } },
  { id: 'loopB', name: 'B', kind: 'character', based_on: 'loopA', builtin: false, text: { size: 5 } },
]

// ---- The mark ------------------------------------------------------------------
assert.ok(schema.marks.char_style, 'the schema has char_style')
const stored = {
  type: 'doc',
  content: [
    {
      type: 'paragraph',
      content: [
        { type: 'text', text: 'plain ' },
        { type: 'text', text: 'loud', marks: [{ type: 'char_style', attrs: { id: 'loud' } }] },
        { type: 'text', text: ' and bold', marks: [{ type: 'char_style', attrs: { id: 'loud' } }, { type: 'bold' }] },
      ],
    },
  ],
}
assert.deepEqual(docToJSON(docFromJSON(stored)), stored, 'JSON in and out')
const dom = schema.marks.char_style.spec.toDOM({ attrs: { id: 'a "b"' } })
assert.deepEqual(dom, ['span', { class: 'cs', 'data-cstyle': 'a "b"' }, 0])

// ---- Resolution --------------------------------------------------------------------
assert.deepEqual(resolveCharStyle(styles, 'loud'), { bold: true, color: '#c00000' })
assert.deepEqual(resolveCharStyle(styles, 'louder'), { bold: false, color: '#c00000', font: 'Impact', size: 16, underline: true }, 'its own over its base')
assert.deepEqual(resolveCharStyle(styles, 'nope'), {}, 'an unknown style gives nothing')
assert.deepEqual(resolveCharStyle(styles, 'quote'), {}, 'a paragraph style is not a character style')
assert.deepEqual(resolveCharStyle(styles, 'loopA'), { italic: true, size: 5 }, 'a loop stops instead of hanging')

const mark = (id) => schema.marks.char_style.create({ id })
const normal = resolveBlock(styles, 'paragraph', {}, [])
assert.equal(normal.text.bold, false)
assert.equal(normal.text.font, 'Merriweather')

const loud = resolveBlock(styles, 'paragraph', {}, [mark('loud')])
assert.equal(loud.text.bold, true, 'the character style over the paragraph style')
assert.equal(loud.text.color, '#c00000')
assert.equal(loud.text.font, 'Merriweather', 'what it does not set stays')
assert.equal(loud.text.size, 10)

const louder = resolveBlock(styles, 'paragraph', {}, [mark('louder')])
assert.equal(louder.text.bold, false, 'a style can turn bold off')
assert.equal(louder.text.size, 16)
assert.equal(louder.text.font, 'Impact')

// The text's own marks come last, whatever the order of the marks.
const own = resolveBlock(styles, 'paragraph', {}, [
  schema.marks.size.create({ pt: 30 }),
  mark('louder'),
  schema.marks.color.create({ color: '#00aa00' }),
  schema.marks.bold.create(),
])
assert.equal(own.text.size, 30)
assert.equal(own.text.color, '#00aa00')
assert.equal(own.text.bold, true)
assert.equal(own.text.font, 'Impact')

// A heading's bold is its style's; a character style that turns it off wins.
const head = resolveBlock(styles, 'heading', { level: 1 }, [mark('louder')])
assert.equal(head.text.bold, false)
assert.equal(resolveBlock(styles, 'heading', { level: 1 }, []).text.bold, true)

// ---- The stylesheet -----------------------------------------------------------------
const sheet = styleSheet(styles, '.write-doc')
assert.ok(sheet.includes('.write-doc [data-cstyle="loud"] { color: #c00000; font-weight: 700 }'), sheet)
const louderRule = sheet.split('\n').find((l) => l.includes('[data-cstyle="louder"]'))
assert.ok(louderRule.includes('font-family: "Impact"') && louderRule.includes('font-size: 16pt') && louderRule.includes('font-weight: 400') && louderRule.includes('text-decoration: underline'), louderRule)
assert.ok(!sheet.includes('[data-style="loud"]'), 'a character style is not a paragraph rule')
assert.equal(styleSheet([...styles, { id: 'x"y', name: 'Q', kind: 'character', text: { italic: true } }]).includes('[data-cstyle="x\\"y"]'), true, 'quotes in an id are escaped')
assert.deepEqual(charDeclarations({ smallcaps: true, superscript: true, highlight: '#ffff00', strike: true }), [
  'font-size: 0.83em',
  'text-decoration: line-through',
  'font-variant: small-caps',
  'vertical-align: super',
  'background-color: #ffff00',
])

// ---- Based on, new ids --------------------------------------------------------------
assert.deepEqual(basedOnChoices(styles, 'louder').map((s) => s.id), ['loud', 'loopA', 'loopB'], 'only character styles, not itself')
assert.ok(!basedOnChoices(styles, 'loud').some((s) => s.id === 'louder'), 'no loops')
assert.ok(!basedOnChoices(styles, 'normal').some((s) => s.kind === 'character'), 'a paragraph style takes paragraph styles')
assert.equal(newStyleId(styles, 'Loud'), 'loud-2')

// ---- Applying and removing ------------------------------------------------------------
const state0 = EditorState.create({ schema, doc: docFromJSON({ type: 'doc', content: [{ type: 'paragraph', content: [{ type: 'text', text: 'one two three' }] }] }) })
const select = (st, from, to) => st.apply(st.tr.setSelection(TextSelection.create(st.doc, from, to)))
const run = (st, cmd) => {
  let next = st
  assert.ok(cmd(st, (tr) => (next = st.apply(tr))))
  return next
}

let st = run(select(state0, 5, 8), setCharStyle('loud'))
assert.deepEqual(docToJSON(st.doc).content[0].content, [
  { type: 'text', text: 'one ' },
  { type: 'text', text: 'two', marks: [{ type: 'char_style', attrs: { id: 'loud' } }] },
  { type: 'text', text: ' three' },
])
assert.equal(selectionFormat(st, styles).charStyleId, 'loud')
assert.equal(selectionFormat(st, styles).bold, true, 'the toolbar shows what the style gives')
assert.equal(selectionFormat(select(st, 1, 4), styles).charStyleId, null)

// Another style replaces it (one character style per text).
st = run(select(st, 5, 8), setCharStyle('louder'))
assert.equal(docToJSON(st.doc).content[0].content[1].marks.length, 1)
assert.equal(docToJSON(st.doc).content[0].content[1].marks[0].attrs.id, 'louder')

// Direct formatting goes on top and stays when the style goes.
st = run(select(st, 5, 8), toggleMark('italic'))
st = run(select(st, 5, 8), setCharStyle(null))
assert.deepEqual(docToJSON(st.doc).content[0].content[1], { type: 'text', text: 'two', marks: [{ type: 'italic' }] })

// With nothing selected it is for the next typing; "clear formatting" takes it off.
let caret = run(select(state0, 4, 4), setCharStyle('loud'))
assert.equal(caret.storedMarks[0].attrs.id, 'loud')
caret = run(caret, (s, d) => d(s.tr.insertText('X')))
assert.equal(docToJSON(caret.doc).content[0].content[1].text, 'X')
const cleared = run(select(run(select(state0, 1, 4), setCharStyle('loud')), 1, 4), clearMarks)
assert.deepEqual(docToJSON(cleared.doc), docToJSON(state0.doc))

console.log('editorCharStyles: ok')
