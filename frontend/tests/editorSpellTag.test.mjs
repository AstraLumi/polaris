// "Tag spell" in the writing editor (src/editor/speaker.js, spellTag.js,
// schema.js): the spell mark's JSON round trip, tagging next to a speaker
// tag, changing and removing it, and the hover card's words.
// node frontend/tests/editorSpellTag.test.mjs
import assert from 'node:assert/strict'
import { EditorState, TextSelection } from 'prosemirror-state'
import { schema, docFromJSON, docToJSON } from '../src/editor/schema.js'
import { tagSpeaker, tagSpell, removeSpell, spellAt, speakerAt } from '../src/editor/speaker.js'
import { spellCard, spellCost, spellChoices } from '../src/editor/spellTag.js'

const text = 'She cast  Fireball  at the door.'
const json = { type: 'doc', content: [{ type: 'paragraph', content: [{ type: 'text', text }] }] }
const stateWith = (from, to) => {
  const st = EditorState.create({ schema, doc: docFromJSON(json) })
  return st.apply(st.tr.setSelection(TextSelection.create(st.doc, from, to)))
}
const run = (st, cmd) => {
  let out = st
  assert.ok(cmd(st, (tr) => (out = st.apply(tr))))
  return out
}

const a = 1 + text.indexOf('  Fireball') // takes the spaces along
const b = 1 + text.indexOf('  at')
let st = run(stateWith(a, b), tagSpell({ id: 4, name: 'Fireball' }))
let out = docToJSON(st.doc).content[0].content
assert.equal(out.length, 3)
assert.equal(out[1].text, 'Fireball', 'trimmed of the spaces around')
assert.deepEqual(out[1].marks, [{ type: 'spell', attrs: { spell: 4, name: 'Fireball' } }])

// Round trip.
assert.deepEqual(docToJSON(docFromJSON(docToJSON(st.doc))), docToJSON(st.doc))

// A speaker tag and a spell tag can cover the same words.
const at = spellAt(st, 1 + text.indexOf('Fire'))
assert.deepEqual({ from: at.from, to: at.to, id: at.attrs.spell }, { from: a + 2, to: b, id: 4 })
st = run(st, tagSpeaker({ id: 2, name: 'Oren' }, { from: at.from, to: at.to }))
assert.deepEqual(docToJSON(st.doc).content[0].content[1].marks.map((m) => m.type).sort(), ['speaker', 'spell'])
assert.equal(speakerAt(st, at.from + 3).attrs.character, 2)

// Changing the spell replaces it (one spell mark at most).
st = run(st, tagSpell({ id: 5, name: 'Frost' }, { from: at.from, to: at.to }))
out = docToJSON(st.doc).content[0].content
assert.equal(out.length, 3)
assert.deepEqual(out[1].marks.find((m) => m.type === 'spell').attrs, { spell: 5, name: 'Frost' })

// Removing it keeps the speaker tag and the text.
st = run(st, removeSpell({ from: at.from, to: at.to }))
assert.deepEqual(docToJSON(st.doc).content[0].content[1].marks, [{ type: 'speaker', attrs: { character: 2, speaker: 'Oren' } }])
assert.equal(spellAt(st, at.from + 3), null)
assert.equal(st.doc.textContent, text)

// Nothing to tag: no selection, or no spell id.
assert.equal(tagSpell({ id: 1, name: 'X' })(stateWith(3, 3), null), false)
assert.equal(tagSpell({ id: null, name: 'X' })(stateWith(3, 8), null), false)

// The card's words (the identity "t" fills the placeholders).
const t = (s, v = {}) => s.replace(/\{(\w+)\}/g, (m, k) => v[k] ?? m)
const spell = { id: 4, name: 'Fireball', level: 3, mp_cost: 12, hp_cost: null, source_type: 'class', source_name: 'Mage' }
assert.deepEqual(spellCard(spell, t), { name: 'Fireball', cost: '12 MP', source: 'Class: Mage', level: 'Level 3' })
assert.equal(spellCost({ mp_cost: 12, hp_cost: 5 }, t), '12 MP · 5 HP')
assert.equal(spellCost({ mp_cost: null, hp_cost: 5 }, t), '5 HP')
assert.equal(spellCost({ mp_cost: 0, hp_cost: null }, t), 'Free')
assert.deepEqual(spellCard({ name: 'Blink', level: 1, source_type: '', source_name: '' }, t), { name: 'Blink', cost: 'Free', source: '', level: 'Level 1' })
assert.equal(spellCard({ name: 'Odd', source_type: 'subclass', source_name: 'Pyro' }, t).source, 'Subclass: Pyro')
assert.deepEqual(spellChoices([{ id: 2, name: 'Zap', level: 1, x: 1 }, { id: 1, name: 'Arc', level: 2 }]).map((s) => s.name), ['Arc', 'Zap'])

console.log('editorSpellTag.test.mjs ok')
