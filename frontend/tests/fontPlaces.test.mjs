// Where a font is used in the text (src/editor/fontPlaces.js): its runs in a
// doc, counted as backend/fontlib.go's docFontRuns counts them (the same
// doc is TestDocFontRuns's), and stepping through a book's places.
// node frontend/tests/fontPlaces.test.mjs
import assert from 'node:assert/strict'
import { docFromJSON } from '../src/editor/schema.js'
import { fontRuns, placeAt, placeCount } from '../src/editor/fontPlaces.js'

const font = (family, more = []) => [...more, { type: 'font', attrs: { family } }]
const doc = docFromJSON({
  type: 'doc',
  content: [
    {
      type: 'paragraph',
      content: [
        { type: 'text', marks: font('Cambria'), text: 'a' },
        { type: 'text', marks: font('Cambria', [{ type: 'bold' }]), text: 'b' },
        { type: 'text', text: 'c' },
        { type: 'text', marks: font('cambria'), text: 'd' },
        { type: 'hard_break', marks: font('Cambria') },
        { type: 'text', marks: font('Other'), text: 'e' },
      ],
    },
    { type: 'paragraph', content: [{ type: 'text', marks: font('Cambria'), text: 'f' }] },
  ],
})

const runs = fontRuns(doc, 'Cambria')
assert.equal(runs.length, 3, 'three runs of Cambria, as the server counts')
assert.deepEqual(runs.map((r) => doc.textBetween(r.from, r.to)), ['ab', 'd', 'f'])
assert.equal(fontRuns(doc, ' other ').length, 1, 'the family in any case, trimmed')
assert.equal(fontRuns(doc, 'Missing').length, 0)
assert.equal(fontRuns(doc, '').length, 0)

const places = [
  { chapter: 7, runs: 2 },
  { chapter: 3, runs: 1 },
]
assert.equal(placeCount(places), 3)
assert.deepEqual(placeAt(places, 0), { chapter: 7, run: 0 })
assert.deepEqual(placeAt(places, 2), { chapter: 3, run: 0 })
assert.deepEqual(placeAt(places, 3), { chapter: 7, run: 0 }, 'past the last wraps to the first')
assert.deepEqual(placeAt(places, -1), { chapter: 3, run: 0 }, 'before the first wraps to the last')
assert.equal(placeAt([], 0), null)
assert.equal(placeAt(undefined, 0), null)

console.log('fontPlaces: ok')
