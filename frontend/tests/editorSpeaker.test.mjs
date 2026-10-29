// "Tag character" in the writing editor (src/editor/speaker.js, schema.js):
// the speaker mark's JSON round trip, tagging a trimmed selection, changing
// and removing the tag, and finding the tag at a position.
// node frontend/tests/editorSpeaker.test.mjs
import assert from 'node:assert/strict'
import { EditorState, TextSelection } from 'prosemirror-state'
import { schema, docFromJSON, docToJSON } from '../src/editor/schema.js'
import { tagSpeaker, removeSpeaker, speakerAt, taggableRange } from '../src/editor/speaker.js'

const json = { type: 'doc', content: [{ type: 'paragraph', content: [{ type: 'text', text: 'He said  "Obby! Go!"  and ran.' }] }] }
const text = 'He said  "Obby! Go!"  and ran.'

function stateWith(from, to) {
  const st = EditorState.create({ schema, doc: docFromJSON(json) })
  return st.apply(st.tr.setSelection(TextSelection.create(st.doc, from, to)))
}
function run(st, cmd) {
  let out = st
  assert.ok(cmd(st, (tr) => (out = st.apply(tr))))
  return out
}

// Positions are 1-based inside the paragraph. The selection takes the
// spaces around the quote with it.
const a = 1 + text.indexOf('  "') // two spaces before the quote
const b = 1 + text.indexOf('!"') + 2 + 2 // the quote's end and two spaces
let st = stateWith(a, b)
assert.deepEqual(taggableRange(st), { from: 1 + text.indexOf('"'), to: 1 + text.indexOf('!"') + 2 }, 'trimmed')

st = run(st, tagSpeaker({ id: 5, name: 'Valiente' }))
let out = docToJSON(st.doc).content[0].content
assert.equal(out.length, 3)
assert.equal(out[1].text, '"Obby! Go!"')
assert.deepEqual(out[1].marks, [{ type: 'speaker', attrs: { character: 5, speaker: 'Valiente' } }])
assert.equal(out[0].marks, undefined)

// Round trip: the stored JSON gives the same doc and the same JSON.
const again = docToJSON(docFromJSON(docToJSON(st.doc)))
assert.deepEqual(again, docToJSON(st.doc))

// A speaker who is not a character keeps a null id (left out of the JSON).
let st2 = run(stateWith(a, b), tagSpeaker({ id: null, name: 'A guard' }))
assert.deepEqual(docToJSON(st2.doc).content[0].content[1].marks, [{ type: 'speaker', attrs: { speaker: 'A guard' } }])

// Changing the tag replaces it (one speaker mark at most).
const at = speakerAt(st, 1 + text.indexOf('Obby'))
assert.deepEqual({ from: at.from, to: at.to, c: at.attrs.character }, { from: 1 + text.indexOf('"'), to: 1 + text.indexOf('!"') + 2, c: 5 })
st = run(st, tagSpeaker({ id: 6, name: 'Obby' }, { from: at.from, to: at.to }))
out = docToJSON(st.doc).content[0].content
assert.equal(out.length, 3)
assert.equal(out[1].marks.length, 1)
assert.equal(out[1].marks[0].attrs.character, 6)

// Partly tagging another range inside splits nothing else.
assert.equal(speakerAt(st, 3), null)

// Removing the tag gives the plain paragraph back.
st = run(st, removeSpeaker({ from: at.from, to: at.to }))
assert.deepEqual(docToJSON(st.doc), json)

// Nothing to tag: an empty or all-space selection.
assert.equal(tagSpeaker({ id: 1, name: 'X' })(stateWith(3, 3), null), false)
assert.equal(tagSpeaker({ id: 1, name: 'X' })(stateWith(8, 10), null), false)

console.log('editorSpeaker: ok')
