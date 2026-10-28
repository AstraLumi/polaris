// The spellchecker in the writing editor (src/editor/spellPlugin.js): what
// text of a paragraph is checked (not links, not a dialogue's speaker),
// the word being typed isn't underlined until the caret leaves it, and a
// suggestion replaces the word keeping its formatting. No DOM: the plugin's
// state on its own and a stand-in view. node frontend/tests/editorSpell.test.mjs
import assert from 'node:assert/strict'
import { EditorState, TextSelection } from 'prosemirror-state'
import { DecorationSet, Decoration } from 'prosemirror-view'
import { schema, docFromJSON } from '../src/editor/schema.js'
import { spellPlugin, spellKey, blockText, blockWords, replaceWord } from '../src/editor/spellPlugin.js'
import { wordSet } from '../src/editor/spellWords.js'

const doc = docFromJSON({
  type: 'doc',
  content: [
    {
      type: 'paragraph',
      content: [
        { type: 'text', text: 'Teh ' },
        { type: 'text', text: 'Hollowmer', marks: [{ type: 'story_link', attrs: { type: 'location', id: 7 } }] },
        { type: 'text', text: '’s gaet, ' },
        { type: 'text', text: 'wrng', marks: [{ type: 'link', attrs: { href: 'https://example.com' } }] },
        { type: 'hard_break' },
        { type: 'text', text: 'Mira ' },
        { type: 'text', text: 'sayd', marks: [{ type: 'bold' }] },
        { type: 'text', text: ' NASA.' },
      ],
    },
    { type: 'dialogue', attrs: { character: null, speaker: 'Zarthul', manner: 'qietly' }, content: [{ type: 'text', text: 'Helo there.' }] },
  ],
})

const spell = { gen: 1, skipCaps: true, known: (w) => wordSet(['Mira']).has(w), wrong: async (ws) => new Set(ws.filter((w) => /^(Teh|gaet|sayd|Helo)$/.test(w))) }
const para = doc.child(0)
assert.equal(blockText(para), 'Teh ' + ' '.repeat(9) + '’s gaet, ' + ' '.repeat(4) + ' Mira sayd NASA.')
assert.deepEqual(blockWords(para, spell).map((t) => t.word), ['Teh', 'gaet', 'sayd'])
// A dialogue: its line is checked; the speaker and manner are attrs.
assert.deepEqual(blockWords(doc.child(1), spell).map((t) => t.word), ['Helo', 'there'])

// The plugin's state: underlines given by a check, hidden at the caret
// while typing there, back when the caret leaves.
const plugin = spellPlugin({ getSpell: () => spell })
let state = EditorState.create({ schema, doc, plugins: [plugin] })
const at = (word) => {
  let found = null
  state.doc.descendants((n, pos) => {
    if (found == null && n.isText && n.text.includes(word)) found = pos + n.text.indexOf(word)
  })
  return found
}
const shown = () => plugin.props.decorations(state).find().map((d) => state.doc.textBetween(d.from, d.to))
const sayd = at('sayd')
state = state.apply(state.tr.setMeta(spellKey, { deco: DecorationSet.create(state.doc, [Decoration.inline(sayd, sayd + 4, { class: 'spell-wrong' }, { word: 'sayd' })]) }))
assert.deepEqual(shown(), ['sayd'])
// Typing at the word's end: hidden while there.
state = state.apply(state.tr.setSelection(TextSelection.create(state.doc, sayd + 4)))
state = state.apply(state.tr.insertText('s'))
assert.deepEqual(shown(), [])
// The caret leaves: shown again.
state = state.apply(state.tr.setSelection(TextSelection.create(state.doc, 2)))
assert.deepEqual(shown(), ['sayd'])
// Typing a space after the word: the caret is past it, so it shows.
state = state.apply(state.tr.setSelection(TextSelection.create(state.doc, sayd + 4)))
state = state.apply(state.tr.insertText(' '))
assert.deepEqual(shown(), ['sayd'])

// A suggestion replaces the word, keeping its formatting; a word that
// changed meanwhile is left alone.
const view = { state, dispatch: (tr) => (view.state = view.state.apply(tr)), focus() {} }
assert.equal(replaceWord(view, { from: sayd, to: sayd + 4, word: 'sayd' }, 'said'), true)
const node = view.state.doc.nodeAt(sayd)
assert.equal(node.text.slice(0, 4), 'said')
assert.ok(node.marks.some((m) => m.type.name === 'bold'))
assert.equal(replaceWord(view, { from: sayd, to: sayd + 4, word: 'sayd' }, 'said'), false)

console.log('editorSpell: ok')
