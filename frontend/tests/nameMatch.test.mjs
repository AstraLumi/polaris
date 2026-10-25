// Tab-to-link and Script Mode's speakers (src/editor/nameMatch.js): which
// names a word being typed could start, the order they are offered in, and
// "Name, manner:" lines. node frontend/tests/nameMatch.test.mjs
import assert from 'node:assert/strict'
import { fold, buildNameIndex, findMatches, resolveSpeaker, parseSpeakerLine, speakerLead } from '../src/editor/nameMatch.js'

const items = [
  { type: 'character', id: 1, name: 'Mira Vey', nicknames: ['Mimi'], aliases: ['Mira Thorne'] },
  { type: 'character', id: 2, name: 'Mirabel', nicknames: [] },
  { type: 'location', id: 7, name: 'Mirador' },
  { type: 'location', id: 8, name: 'Íllvar' },
  { type: 'faction', id: 3, name: 'The Grey Order' },
  { type: 'character', id: 4, name: 'Jõao Straße' },
  { type: 'event', id: 5, name: 'Ash' },
]
const index = buildNameIndex(items)

assert.equal(fold('Íllvar ÉTÉ'), 'illvar ete')
assert.equal(fold('Straße'), 'strasse')
assert.equal(index.length, 9)
assert.deepEqual(
  index.filter((e) => e.id === 1).map((e) => [e.label, e.via]),
  [['Mira Vey', 'name'], ['Mimi', 'nickname'], ['Mira Thorne', 'alias']],
)

const labels = (r) => r && r.matches.map((m) => m.label)

// Three letters at least, at a word's start, with the caret at a word's end.
assert.equal(findMatches(index, 'Mi'), null)
assert.equal(findMatches(index, 'She saw xMir'), null)
assert.equal(findMatches(index, 'She saw Mir', 'a'), null)
assert.equal(findMatches(index, 'She saw Mir '), null)
assert.equal(findMatches(index, ''), null)

// Several matches: characters first, then exact case, then shorter.
let r = findMatches(index, 'She saw Mir')
assert.equal(r.start, 8)
assert.equal(r.typed, 'Mir')
assert.deepEqual(labels(r), ['Mirabel', 'Mira Vey', 'Mira Thorne', 'Mirador'])
assert.equal(r.matches[1].rest, 'a Vey')

// Case- and accent-insensitive; the rest comes from the name.
r = findMatches(index, 'to illv')
assert.deepEqual(labels(r), ['Íllvar'])
assert.equal(r.matches[0].rest, 'ar')
r = findMatches(index, 'MIRAD')
assert.deepEqual(labels(r), ['Mirador'])
assert.equal(r.matches[0].rest, 'or')
r = findMatches(index, 'joao stras')
assert.deepEqual(labels(r), ['Jõao Straße'])
assert.equal(r.matches[0].rest, 'e') // the typed s took half of ß

// Names of several words: the longest typed start that matches.
r = findMatches(index, 'and then Mira V')
assert.equal(r.typed, 'Mira V')
assert.deepEqual(labels(r), ['Mira Vey'])
assert.equal(r.matches[0].rest, 'ey')
r = findMatches(index, 'joined the grey or')
assert.equal(r.typed, 'the grey or')
assert.deepEqual(labels(r), ['The Grey Order'])

// Nicknames and former names are offered as themselves.
r = findMatches(index, 'Mim')
assert.deepEqual(r.matches.map((m) => [m.label, m.via, m.id]), [['Mimi', 'nickname', 1]])
r = findMatches(index, 'Mira T')
assert.deepEqual(labels(r), ['Mira Thorne'])

// A whole name typed still matches (nothing left to type).
r = findMatches(index, 'Mirador')
assert.equal(r.matches[0].rest, '')

// Not across an atom or link (￼) or a new line; capped; by type.
assert.equal(findMatches(index, 'The￼grey or'), null)
assert.equal(findMatches(index, 'Ash\nMir').typed, 'Mir')
assert.equal(findMatches(index, 'Mir', '', { max: 2 }).matches.length, 2)
assert.deepEqual(labels(findMatches(index, 'Mir', '', { types: ['location'] })), ['Mirador'])
// After an apostrophe is inside a word.
assert.equal(findMatches(index, "d'Mir"), null)

// Speakers.
assert.deepEqual(resolveSpeaker(index, 'mira vey'), { id: 1, name: 'Mira Vey' })
assert.deepEqual(resolveSpeaker(index, ' MIMI '), { id: 1, name: 'Mira Vey' })
assert.deepEqual(resolveSpeaker(index, 'Mira Thorne'), { id: 1, name: 'Mira Vey' })
assert.equal(resolveSpeaker(index, 'Mirador'), null)
assert.equal(resolveSpeaker(index, 'A guard'), null)
assert.equal(resolveSpeaker(index, ''), null)

assert.deepEqual(parseSpeakerLine('Mira:'), { speaker: 'Mira', manner: '' })
assert.deepEqual(parseSpeakerLine('Mira Vey, softly:'), { speaker: 'Mira Vey', manner: 'softly' })
assert.deepEqual(parseSpeakerLine('  A guard ,  shouting from the wall :'), { speaker: 'A guard', manner: 'shouting from the wall' })
assert.equal(parseSpeakerLine('Mira'), null)
assert.equal(parseSpeakerLine(':'), null)
assert.equal(parseSpeakerLine('123:'), null)
assert.equal(parseSpeakerLine('This is far too long a sentence to be anybody’s name:'), null)
assert.equal(parseSpeakerLine('Mira: hello:'), null)

assert.equal(speakerLead({ speaker: 'Mira', manner: 'softly' }), 'Mira, softly: ')
assert.equal(speakerLead({ speaker: 'Mira', manner: '' }), 'Mira: ')
assert.equal(speakerLead({ speaker: '', manner: '' }), '')

console.log('nameMatch: ok')
