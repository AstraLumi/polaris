// The spellchecker's words (src/editor/spellWords.js): what counts as a
// word in a text, the story's names as words, the language pick.
// node frontend/tests/spellWords.test.mjs
import assert from 'node:assert/strict'
import { tokenize, isCaps, wordSet, storyWords, spellLanguage, dictionaryFor, cleanWord, dictionaryUrl } from '../src/editor/spellWords.js'

const words = (s, o) => tokenize(s, o).map((t) => t.word)

// Letters with accents, apostrophes and hyphens inside words.
assert.deepEqual(words('Mira’s lantern—don\'t well-known fazê-lo, coração; Ælfwine.'), ['Mira’s', 'lantern', "don't", 'well-known', 'fazê-lo', 'coração', 'Ælfwine'])
// Quotes and dashes around words are not part of them.
assert.deepEqual(words("'Fine,' she said -- 'tis--the dogs'"), ['Fine', 'she', 'said', 'tis', 'the', 'dogs'])
// Numbers and words with digits are left out; one letter alone too.
assert.deepEqual(words('3rd floor, B12, 1054 Q2, a cat, I am 42'), ['floor', 'cat', 'am'])
// Addresses and dotted things.
assert.deepEqual(words('see https://example.com/some-path?x=1 or www.polaris.app, mail me@example.org now'), ['see', 'or', 'mail', 'now'])
assert.deepEqual(words('e.g. the a.C. date, file.txt and C:/path/to_this ok'), ['the', 'date', 'and', 'ok'])
// Capitals, when asked to skip them.
assert.deepEqual(words('LEVEL UP for Mira at NASA', { skipCaps: true }), ['for', 'Mira', 'at'])
assert.deepEqual(words('LEVEL UP'), ['LEVEL', 'UP'])
assert.ok(isCaps('NASA') && isCaps('DON’T') && !isCaps('Nasa') && !isCaps('A'))
// Offsets point into the text, and a full stop after is noted.
const [t1, t2] = tokenize('  Mr. Vey')
assert.deepEqual([t1.from, t1.to, t1.dot, t2.word, t2.dot], [2, 4, true, 'Vey', false])
// Other scripts.
assert.deepEqual(words('Привет мир'), ['Привет', 'мир'])

// The story's names, word by word, with nicknames and former names.
const items = [
  { type: 'character', id: 1, name: 'Mira Vey', nicknames: ['Mimi'], aliases: ['Mira Thorne'] },
  { type: 'location', id: 7, name: 'Hollowmere' },
  { type: 'faction', id: 3, name: 'The Ka-Ren Order' },
  { type: 'event', id: 5, name: 'the burning of 1054' },
]
const names = storyWords(items)
for (const w of ['Mira', 'Vey', 'Mimi', 'Thorne', 'Hollowmere', 'Ka-Ren', 'Ka', 'Ren', 'Order', 'burning']) assert.ok(names.includes(w), w)
assert.ok(!names.includes('1054'))
assert.deepEqual(storyWords(['Old Tam', 'Tam']).sort(), ['Old', 'Tam'])

// Matching: a name with a capital matches written with capitals in any
// case, not in lower case; a lower-case word matches in any case;
// possessives pass.
const set = wordSet([...names, 'bravo'])
for (const w of ['Mira', 'MIRA', 'MiRA', 'Mira’s', "Mira's", 'MIRA’S', 'Hollowmere', 'Bravo', 'BRAVO', 'bravo', 'burning', 'Burning']) assert.ok(set.has(w), w)
for (const w of ['mira', 'hollowmere', 'Miras', 'bravos', 'Vey’d']) assert.ok(!set.has(w), w)

// The language: the book's, else the interface's; unknown → none.
assert.equal(spellLanguage('en', 'pt-BR').code, 'en_US')
assert.equal(spellLanguage('en-GB', 'en-US').code, 'en_GB')
assert.equal(spellLanguage('en_AU', 'en-US').code, 'en_GB')
assert.equal(spellLanguage('pt', 'en-US').code, 'pt_BR')
assert.equal(spellLanguage('es-419', 'en-US').code, 'es_ES')
assert.deepEqual(spellLanguage('', 'pt-BR'), { code: 'pt_BR', tag: 'pt-BR', from: 'interface' })
assert.deepEqual(spellLanguage('  ', 'es-419'), { code: 'es_ES', tag: 'es-419', from: 'interface' })
assert.deepEqual(spellLanguage('fr', 'en-US'), { code: null, tag: 'fr', from: 'book' })
assert.equal(dictionaryFor('EN-us'), 'en_US')
assert.equal(dictionaryFor(null), null)
assert.ok(dictionaryUrl('pt_BR', 'dic').startsWith('/dict/pt_BR/pt_BR.dic.gz?v='))

// Words the story's dictionary takes.
assert.equal(cleanWord('  Zarathul '), 'Zarathul')
assert.equal(cleanWord('Cafe\u0301'), 'Café')
for (const bad of ['', 'two words', '1234', 'a'.repeat(65), 'tab\tx']) assert.equal(cleanWord(bad), '', bad)

console.log('spellWords: ok')
