// The toolbar's font picker (src/editor/fontSearch.js): ranking, its
// sections, recent and favourite fonts; and the fonts a selection is in
// (fontsInSelection in src/editor/commands.js).
// node frontend/tests/fontSearch.test.mjs
import assert from 'node:assert/strict'
import { EditorState, TextSelection } from 'prosemirror-state'
import { fontScore, bestMatches, fontSections, pushRecent, toggleFavourite } from '../src/editor/fontSearch.js'
import { schema, docFromJSON } from '../src/editor/schema.js'
import { fontsInSelection } from '../src/editor/commands.js'
import { defaultSettings } from '../src/editor/pageTypes.js'

const FONTS = ['Merriweather', 'Turret Road', 'Noto Sans Symbols', 'Noto Sans Symbols 2', 'Noto Serif JP', 'Orbitron', 'Roboto', 'Cormorant Garamond', 'EB Garamond']

// Scores: exact > start > word start > every word > anywhere > letters in order.
assert.equal(fontScore('Turret Road', 'turret road'), 100)
assert.ok(fontScore('Turret Road', 'tur') > fontScore('Cormorant Garamond', 'gar'))
assert.ok(fontScore('Cormorant Garamond', 'gar') > fontScore('Noto Sans Symbols', 'noto sym'))
assert.ok(fontScore('Noto Sans Symbols', 'noto sym') > fontScore('Orbitron', 'bit'))
assert.ok(fontScore('Orbitron', 'bit') > fontScore('Orbitron', 'obn'))
assert.equal(fontScore('Orbitron', 'xyz'), 0)
assert.equal(fontScore('Orbitron', '  '), 0)

assert.deepEqual(bestMatches(FONTS, 'garamond'), ['EB Garamond', 'Cormorant Garamond'], 'word starts, the shorter name first')
assert.deepEqual(bestMatches(FONTS, 'noto sym').slice(0, 2), ['Noto Sans Symbols', 'Noto Sans Symbols 2'])
assert.deepEqual(bestMatches(FONTS, 'ro', 3), ['Roboto', 'Turret Road', 'Orbitron'])

// Sections: every font stays listed, each once; matches only while typing.
let s = fontSections({ fonts: FONTS, favourites: ['Orbitron'], recent: ['Turret Road', 'orbitron'], query: '' })
assert.deepEqual(s.map((x) => x.key), ['favourites', 'recent', 'all'])
assert.deepEqual(s[1].fonts, ['Turret Road'], 'a favourite is not repeated in recent (any case)')
assert.equal(s.flatMap((x) => x.fonts).length, FONTS.length, 'each font once')
s = fontSections({ fonts: FONTS, favourites: ['Orbitron'], recent: ['Turret Road'], query: 'turret' })
assert.deepEqual(s.map((x) => x.key), ['matches', 'favourites', 'all'])
assert.deepEqual(s[0].fonts, ['Turret Road'])
assert.equal(s.flatMap((x) => x.fonts).length, FONTS.length, 'the others are still there')
s = fontSections({ fonts: FONTS, query: 'zzz' })
assert.deepEqual(s.map((x) => x.key), ['all'], 'no match: the whole list')
s = fontSections({ fonts: FONTS, recent: ['My Own Face'], query: 'own' })
assert.deepEqual(s[0], { key: 'matches', fonts: ['My Own Face'] }, 'recent fonts not in the list are found too')

assert.deepEqual(pushRecent(['A', 'B', 'C'], 'b'), ['b', 'A', 'C'])
assert.deepEqual(pushRecent(['A', 'B'], 'C', 2), ['C', 'A'])
assert.deepEqual(pushRecent(['A'], ''), ['A'])
assert.deepEqual(toggleFavourite(['A'], 'B'), ['A', 'B'])
assert.deepEqual(toggleFavourite(['A', 'B'], 'a'), ['B'])

// The fonts of a selection: the font marks, else the paragraph style's font.
const styles = defaultSettings('a5').styles
const font = (family) => [{ type: 'font', attrs: { family } }]
const doc = docFromJSON({
  type: 'doc',
  content: [
    { type: 'paragraph', content: [{ type: 'text', text: 'One ', marks: font('Turret Road') }, { type: 'text', text: 'two', marks: font('Orbitron') }] },
    { type: 'paragraph', content: [{ type: 'text', text: 'plain' }] },
  ],
})
const sel = (from, to) => {
  const st = EditorState.create({ schema, doc })
  return st.apply(st.tr.setSelection(TextSelection.create(st.doc, from, to)))
}
assert.deepEqual(fontsInSelection(sel(1, 4), styles), ['Turret Road'])
assert.deepEqual(fontsInSelection(sel(1, 8), styles), ['Turret Road', 'Orbitron'], 'a line with two fonts')
assert.deepEqual(fontsInSelection(sel(5, 14), styles), ['Orbitron', 'Merriweather'], 'unmarked text is in its style’s font')
assert.deepEqual(fontsInSelection(sel(2, 2), styles), [], 'nothing selected')

console.log('fontSearch: ok')
