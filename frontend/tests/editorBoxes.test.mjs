// Boxes and drop caps (paragraph attrs boxWidth, boxColor, shade, dropCap):
// the stored JSON in and out, the CSS, the markers the editor's stylesheet
// joins consecutive boxes by, and the paragraph controls' commands.
// node frontend/tests/editorBoxes.test.mjs
import assert from 'node:assert/strict'
import { EditorState, TextSelection } from 'prosemirror-state'
import { schema, docFromJSON, docToJSON, ownPara } from '../src/editor/schema.js'
import { paraDeclarations } from '../src/editor/css.js'
import { setParaAttrs } from '../src/editor/commands.js'

const p = (text, attrs) => ({ type: 'paragraph', ...(attrs ? { attrs } : {}), content: [{ type: 'text', text }] })
const box = { boxWidth: 1.5, boxColor: '#3366cc', shade: '#e8f0ff' }

// The JSON round trip keeps the attrs.
const json = { type: 'doc', content: [p('one', box), p('two', box), p('Drop', { dropCap: 3 }), p('plain')] }
const back = docToJSON(docFromJSON(json))
assert.deepEqual(back.content[0].attrs, box)
assert.equal(back.content[2].attrs.dropCap, 3)
assert.equal(back.content[3].attrs, undefined, 'a plain paragraph stays plain')


// ownPara carries them; the CSS draws border, shading and padding.
assert.deepEqual(ownPara({ ...box, dropCap: null, style: null, keepNext: false }), box)
const css = paraDeclarations(box).join('; ')
assert.match(css, /border: 1\.5pt solid #3366cc/)
assert.match(css, /background-color: #e8f0ff/)
assert.match(css, /padding: 4pt 6pt/)
assert.deepEqual(paraDeclarations({ dropCap: 3 }), [], 'a drop cap is not inline CSS')

// The DOM: boxed paragraphs carry data-box (the stylesheet joins consecutive
// ones), drop caps data-dropcap; copies read back from data-para.

const dom = (n) => ({ attrs: n.type.spec.toDOM(n)[1] })
const doc = docFromJSON(json)
const el = [0, 1, 2, 3].map((i) => dom(doc.child(i)).attrs)
assert.ok(('data-box' in el[0]) && ('data-box' in el[1]))
assert.ok(!('data-box' in el[2]) && !('data-box' in el[3]))
assert.equal(el[2]['data-dropcap'], '3')
assert.ok(!('data-dropcap' in el[3]))

// The command sets and clears them.
let state = EditorState.create({ schema, doc })
state = state.apply(state.tr.setSelection(TextSelection.create(state.doc, 2)))
setParaAttrs({ dropCap: 2, boxWidth: 2 })(state, (tr) => (state = state.apply(tr)))
assert.equal(state.doc.child(0).attrs.dropCap, 2)
assert.equal(state.doc.child(0).attrs.boxWidth, 2)
setParaAttrs({ boxWidth: null, shade: null, boxColor: null })(state, (tr) => (state = state.apply(tr)))
assert.equal(state.doc.child(0).attrs.boxWidth, null)
assert.equal(state.doc.child(0).attrs.shade, null)

console.log('editorBoxes: ok')
