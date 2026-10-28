// The fonts library's pure rules (src/fontRules.js): finding a family by
// name, the @font-face rules, the pickers' list and the license warnings.
// node frontend/tests/fontLibrary.test.mjs
import assert from 'node:assert/strict'
import {
  findFamily, fontFaceRules, mergeChoices, styleWarnings, exportWarnings, licenseProblem, familyWarnings,
  familyForbidsEmbedding, familyEmbeddingUnknown, exportNeeds, cssFamily, usedSummary,
} from '../src/fontRules.js'

const file = (over = {}) => ({ id: 1, url: '/uploads/fonts/0123456789abcdef0123456789abcdef.ttf', format: 'ttf', weight: 400, italic: false, embedding: 'installable', bitmap_only: false, ...over })
const ALL = { print: true, ebook: true, web: true, commercial: true }
const NONE = { print: false, ebook: false, web: false, commercial: false }
const lib = [
  { id: 1, name: 'Test Serif', license_kind: 'ofl', uses: ALL, files: [file(), file({ id: 2, weight: 700, italic: true, url: '/uploads/fonts/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.woff2', format: 'woff2', embedding: 'unknown' })] },
  { id: 2, name: 'Shop Script', license_kind: 'personal', uses: NONE, files: [file({ embedding: 'restricted' })] },
  { id: 3, name: 'Print Only', license_kind: 'commercial', uses: { ...NONE, print: true, commercial: true }, files: [file()] },
  { id: 4, name: 'Mystery', license_kind: '', uses: NONE, files: [file()] },
  { id: 5, name: 'Bitmap Face', license_kind: 'commercial', uses: { ...ALL, commercial: false }, files: [file({ bitmap_only: true, embedding: 'editable' })] },
]

// Names match ignoring case and spaces around.
assert.equal(findFamily(lib, ' test serif ')?.id, 1)
assert.equal(findFamily(lib, 'Merriweather'), null)
assert.equal(findFamily(lib, ''), null)
assert.equal(findFamily(null, 'Test Serif'), null)

// @font-face: one per file, with weight, style and format; a family name
// can't break out of its string; addresses that aren't library files are
// skipped.
const css = fontFaceRules([...lib.slice(0, 1), { name: 'Evil"}; body{x', files: [file({ url: 'https://example.com/x.ttf' }), file({ url: '/uploads/fonts/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.otf', format: 'otf' })] }])
assert.equal(css.match(/@font-face/g).length, 3)
assert.match(css, /font-family: "Test Serif"; src: url\("\/uploads\/fonts\/0123456789abcdef0123456789abcdef\.ttf"\) format\("truetype"\); font-weight: 400; font-style: normal;/)
assert.match(css, /format\("woff2"\); font-weight: 700; font-style: italic;/)
assert.match(css, /font-family: "Evil bodyx"; src: url\("\/uploads\/fonts\/b+\.otf"\) format\("opentype"\)/)
assert.ok(!css.includes('example.com'))
assert.equal(cssFamily('A "B"'), '"A B"')

// The pickers: the library first, then the rest, one spelling per name.
assert.deepEqual(mergeChoices(['Test Serif', 'Lora'], ['lora', 'Merriweather'], [' test serif', 'Inter', ''], null), ['Test Serif', 'Lora', 'Merriweather', 'Inter'])

// License problems.
assert.equal(licenseProblem(lib[0]), null)
assert.equal(licenseProblem(lib[1]), 'personal')
assert.equal(licenseProblem(lib[3]), 'unrecorded')
assert.equal(licenseProblem(lib[4]), 'no-commercial')
assert.ok(familyForbidsEmbedding(lib[1]) && familyForbidsEmbedding(lib[4]) && !familyForbidsEmbedding(lib[0]))
assert.ok(familyEmbeddingUnknown(lib[0]) && !familyEmbeddingUnknown(lib[2]))

// A book's styles: personal and not-commercial library fonts, grouped by
// family with the styles' names; Google fonts and unrecorded ones aren't
// warned about here.
const styles = [
  { id: 'normal', name: 'Normal', text: { font: 'test serif' } },
  { id: 'quote', name: 'Quote', text: { font: 'Shop Script' } },
  { id: 'title', name: 'Title', text: { font: 'shop script' } },
  { id: 'h1', name: 'Heading 1', text: { font: 'Bitmap Face' } },
  { id: 'h2', name: 'Heading 2', text: { font: 'Merriweather' } },
  { id: 'h3', name: 'Heading 3', text: { font: 'Mystery' } },
  { id: 'x', name: 'No text' },
]
assert.deepEqual(styleWarnings(styles, lib), [
  { family: 'Shop Script', reason: 'personal', styles: ['Quote', 'Title'] },
  { family: 'Bitmap Face', reason: 'no-commercial', styles: ['Heading 1'] },
])
assert.deepEqual(styleWarnings(styles, []), [])

// Exports: the use each needs, and embedding.
assert.equal(exportNeeds('pdf'), 'print')
assert.equal(exportNeeds('docx'), 'print')
assert.equal(exportNeeds('epub'), 'ebook')
assert.equal(exportNeeds('html'), null)
const uses = [
  { family: 'Test Serif', styles: ['Normal'], bands: 0, chapters: 0 },
  { family: 'Shop Script', styles: ['Quote'], bands: 0, chapters: 2 },
  { family: 'Print Only', styles: [], bands: 1, chapters: 0 }, // headers only
  { family: 'Merriweather', styles: ['Heading 2'], bands: 0, chapters: 0 },
  { family: 'print only', styles: [], bands: 0, chapters: 0 },
]
assert.deepEqual(exportWarnings('pdf', uses, lib), [
  { family: 'Shop Script', reason: 'no-print' },
  { family: 'Shop Script', reason: 'no-embedding' },
])
assert.deepEqual(exportWarnings('docx', uses, lib), exportWarnings('pdf', uses, lib))
// EPUB: a family only the headers use doesn't count (an e-book has none).
assert.deepEqual(exportWarnings('epub', uses, lib), [
  { family: 'Shop Script', reason: 'no-ebook' },
  { family: 'Shop Script', reason: 'no-embedding' },
])
assert.deepEqual(exportWarnings('epub', [{ family: 'Print Only', styles: ['Normal'] }], lib), [{ family: 'Print Only', reason: 'no-ebook' }])
assert.deepEqual(exportWarnings('html', uses, lib), [])

// The library page's warnings: "unrecorded" always; a license problem only
// when a book's styles use it; embedding always.
assert.deepEqual(familyWarnings(lib[3], []), ['unrecorded'])
assert.deepEqual(familyWarnings(lib[1], []), ['no-embedding'])
assert.deepEqual(familyWarnings(lib[1], [{ story_id: 'a', styles: ['Quote'] }]), ['personal', 'no-embedding'])
assert.deepEqual(familyWarnings(lib[1], [{ story_id: 'a', styles: [], chapters: 1 }]), ['no-embedding'])
assert.deepEqual(familyWarnings(lib[0], [{ story_id: 'a', styles: ['Normal'] }]), [])
assert.deepEqual(usedSummary([{ story_id: 'a' }, { story_id: 'a' }, { story_id: 'b' }]), { stories: 2, books: 3 })

console.log('fontLibrary: ok')
