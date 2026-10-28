// Read aloud (src/editor/readAloud.js): cutting text into sentences, mapping
// them back to document positions, voices and remembered choices.
// node frontend/tests/readAloud.test.mjs
import assert from 'node:assert/strict'
import { docFromJSON } from '../src/editor/schema.js'
import { splitSentences, chunkText, collectSentences, groupVoices, pickVoice, loadPrefs, savePrefs, clampRate, PAUSE } from '../src/editor/readAloud.js'

const split = (t, lang) => splitSentences(t, lang).map((s) => t.slice(s.start, s.end))

// ---- Sentences ----
assert.deepEqual(split('One. Two! Three? Four'), ['One.', 'Two!', 'Three?', 'Four'])
assert.deepEqual(split('“Stop!” she said. He did.'), ['“Stop!” she said.', 'He did.'])
assert.deepEqual(split('She said, “Go.” Then he left.'), ['She said, “Go.”', 'Then he left.'])
assert.deepEqual(split('(He left.) She stayed.'), ['(He left.)', 'She stayed.'])
assert.deepEqual(split('Mr. Smith met Dr. Jones at St. Paul’s. They talked.'), ['Mr. Smith met Dr. Jones at St. Paul’s.', 'They talked.'])
assert.deepEqual(split('Use a tool, e.g. a knife. Done.'), ['Use a tool, e.g. a knife.', 'Done.'])
assert.deepEqual(split('It cost 3.5 coins. Fine.'), ['It cost 3.5 coins.', 'Fine.'])
assert.deepEqual(split('J. R. Smith left.'), ['J. R. Smith left.'])
assert.deepEqual(split('Well… maybe not. Go on…  Then what?'), ['Well… maybe not.', 'Go on…', 'Then what?'])
assert.deepEqual(split('He waited... and then ran. Fast.'), ['He waited... and then ran.', 'Fast.'])
assert.deepEqual(split('What?! No way!!! Yes.'), ['What?!', 'No way!!!', 'Yes.'])
assert.deepEqual(split('El Sr. López llegó. La Sra. Ruiz no.', 'es-ES'), ['El Sr. López llegó.', 'La Sra. Ruiz no.'])
assert.deepEqual(split('O Dr. Silva chegou. Ele sorriu.', 'pt-BR'), ['O Dr. Silva chegou.', 'Ele sorriu.'])
assert.deepEqual(split('   spaced   out.   Again.  '), ['spaced   out.', 'Again.'])
assert.deepEqual(split('* * *'), [])
assert.deepEqual(split('…'), [])
assert.deepEqual(split(''), [])
assert.deepEqual(split('No ending here'), ['No ending here'])

// ---- Chunks ----
assert.deepEqual(chunkText('short'), ['short'])
const long = 'word, '.repeat(100).trim()
const parts = chunkText(long, 100)
assert.ok(parts.length > 1 && parts.every((x) => x.length <= 100))
assert.equal(parts.join(' ').replace(/\s+/g, ' '), long)
assert.ok(chunkText('x'.repeat(500), 100).every((x) => x.length <= 100))

// ---- Mapping to the document ----
const P = (...c) => ({ type: 'paragraph', content: c })
const T = (text, marks) => ({ type: 'text', text, ...(marks ? { marks } : {}) })
const doc = docFromJSON({
  type: 'doc',
  content: [
    { type: 'heading', attrs: { level: 1 }, content: [T('The Gate')] },
    P(T('He ran. '), T('She', [{ type: 'bold' }]), T(' did not.')),
    { type: 'image', attrs: { src: '/uploads/a.png' } },
    { type: 'scene_break', attrs: {} },
    { type: 'dialogue', attrs: { character: 1, speaker: 'Mira', manner: '' }, content: [T('Stop! Please.')] },
    { type: 'page_break' },
    P(T('The end.')),
  ],
})
const all = collectSentences(doc, { from: 0, lang: 'en' })
assert.deepEqual(
  all.map((s) => s.text),
  ['The Gate', 'He ran.', 'She did not.', 'Stop!', 'Please.', 'The end.'],
)
for (const s of all) assert.equal(doc.textBetween(s.from, s.to, ' ').replace(/\s+/g, ' '), s.text)
assert.equal(all[0].after, PAUSE.heading)
assert.equal(all[1].after, 0, 'no pause inside a paragraph')
assert.equal(all[2].after, PAUSE.scene, 'a scene break follows this paragraph')
assert.equal(all[3].spoken, 'Mira: Stop!')
assert.equal(all[4].spoken, 'Please.', 'the speaker is named once')
assert.equal(all[5].after, PAUSE.paragraph)
assert.equal(collectSentences(doc, { from: 0, speakers: false })[3].spoken, 'Stop!')

// From the caret: the sentence holding it and everything after.
const mid = all[1].from + 3
assert.deepEqual(collectSentences(doc, { from: mid }).map((s) => s.text).slice(0, 2), ['He ran.', 'She did not.'])
assert.equal(collectSentences(doc, { from: all[1].to + 1 }).map((s) => s.text)[0], 'She did not.')

// A selection: only what is selected, cut at its edges.
const sel = collectSentences(doc, { from: all[1].from + 3, to: all[2].from + 2 })
assert.deepEqual(sel.map((s) => s.text), ['ran.', 'Sh'])
assert.deepEqual(collectSentences(doc, { from: all[3].from, to: all[4].to }).map((s) => s.text), ['Stop!', 'Please.'])
assert.deepEqual(collectSentences(doc, { from: 0, to: 1 }), [])

// Hard breaks read as spaces; lists are read.
const doc2 = docFromJSON({
  type: 'doc',
  content: [P(T('Line one'), { type: 'hard_break' }, T('line two.')), { type: 'bullet_list', content: [{ type: 'list_item', content: [P(T('Item.'))] }] }],
})
assert.deepEqual(collectSentences(doc2, { from: 0 }).map((s) => s.text), ['Line one line two.', 'Item.'])

// ---- Voices ----
const V = (name, lang, extra = {}) => ({ name, lang, voiceURI: name, ...extra })
const voices = [V('Zed', 'en-US'), V('Ana', 'pt-BR'), V('Bia', 'pt_PT'), V('Cleo', 'pt-BR', { default: true }), V('Dan', 'en-GB')]
const g = groupVoices(voices, 'pt-BR')
assert.deepEqual(g.matching.map((v) => v.name), ['Ana', 'Cleo', 'Bia'], 'same region first, then the language')
assert.deepEqual(g.others.map((v) => v.name), ['Dan', 'Zed'])
assert.equal(pickVoice(voices, '', 'pt-BR').name, 'Cleo', 'the default of the language')
assert.equal(pickVoice(voices, 'Zed', 'pt-BR').name, 'Zed', 'the remembered voice wins')
assert.equal(pickVoice(voices, 'Gone', 'en').name, 'Dan')
assert.equal(pickVoice([], '', 'en'), null)
assert.equal(pickVoice(voices, '', 'xx'), null)

// ---- Remembered choices ----
const mem = () => {
  const m = {}
  return { getItem: (k) => (k in m ? m[k] : null), setItem: (k, v) => (m[k] = String(v)) }
}
const st = mem()
assert.deepEqual(loadPrefs(st), { voice: '', rate: 1 })
savePrefs({ voice: 'Ana', rate: 1.5 }, st)
assert.deepEqual(loadPrefs(st), { voice: 'Ana', rate: 1.5 })
savePrefs({ voice: 'Ana', rate: 9 }, st)
assert.equal(loadPrefs(st).rate, 2)
st.setItem('polaris.readAloud', '{broken')
assert.deepEqual(loadPrefs(st), { voice: '', rate: 1 })
const blocked = {
  getItem: () => {
    throw new Error('blocked')
  },
  setItem: () => {
    throw new Error('blocked')
  },
}
assert.deepEqual(loadPrefs(blocked), { voice: '', rate: 1 })
savePrefs({ voice: 'x', rate: 1 }, blocked)
assert.equal(clampRate('abc'), 1)
assert.equal(clampRate(0.1), 0.5)

console.log('readAloud: ok')
