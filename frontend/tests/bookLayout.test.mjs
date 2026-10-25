// A book's layout and the page view's CSS (src/editor/bookLayout.js).
// No dependencies: node frontend/tests/bookLayout.test.mjs
import assert from 'node:assert/strict'
import {
  normaliseLayout,
  normaliseMeta,
  normaliseChapterOptions,
  setChapterOption,
  chapterOption,
  bandTokens,
  bandContent,
  bandText,
  formatNumber,
  bookPlan,
  pagedCSS,
  tocEntries,
  cssString,
} from '../src/editor/bookLayout.js'

// Defaults fill what is missing; what this code doesn't know is kept.
const l = normaliseLayout({ different_first: false, header: { odd: { right: '{book}', glow: 1 } }, numbering: { format: 'klingon', start: '7' }, future: [1] })
assert.equal(l.different_first, false)
assert.equal(l.different_odd_even, true)
assert.equal(l.header.odd.right, '{book}')
assert.equal(l.header.odd.center, '{chapter}', 'a band keeps the default of a text it was not sent')
assert.equal(l.header.odd.glow, 1)
assert.equal(l.header.even.center, '{book}')
assert.equal(l.footer.first.center, '{page}')
assert.equal(l.numbering.format, 'arabic', 'an unknown format is arabic')
assert.equal(l.numbering.start, 7)
assert.deepEqual(l.future, [1])
assert.equal(normaliseLayout(null).chapter_start, 'next')
assert.equal(normaliseLayout({ chapter_start: 'odd' }).chapter_start, 'odd')
assert.equal(normaliseMeta({ title: 'X' }).language, 'en')
assert.deepEqual(Object.keys(normaliseChapterOptions({ 3: {}, abc: {}, 0: {} })), ['3'])

// Chapter options: back at their defaults, they go.
let s = { chapter_options: {} }
s = setChapterOption(s, 4, { front_matter: true })
assert.equal(chapterOption(s, 4).front_matter, true)
assert.equal(chapterOption(s, 5).front_matter, false)
s = setChapterOption(s, 4, { front_matter: false })
assert.deepEqual(s.chapter_options, {})

// Fields in a band's text.
assert.deepEqual(bandTokens('Part One – {page}'), [{ text: 'Part One – ' }, { field: 'page' }])
assert.deepEqual(bandTokens('{nope} {chapter}'), [{ text: '{nope} ' }, { field: 'chapter' }], 'an unknown field is text')
assert.equal(bandContent('', {}), 'none')
assert.equal(bandContent('{book} · {page}', { book: 'Rain "Book"', counter: 'upper-roman' }), '"Rain \\"Book\\"" " · " counter(page, upper-roman)')
assert.equal(bandContent('{chapter}'), 'string(pvchapter)')
assert.equal(bandContent('{chapter_number}. {author}', { author: 'Ann' }), 'string(pvchapnum) ". " "Ann"')
assert.equal(bandText('{chapter} – {page}', { chapter: 'Dawn', page: 'iv' }), 'Dawn – iv')
assert.equal(cssString('a\\b'), '"a\\\\b"')

assert.equal(formatNumber(4, 'roman'), 'iv')
assert.equal(formatNumber(1994, 'upper_roman'), 'MCMXCIV')
assert.equal(formatNumber(28, 'alpha'), 'ab')
assert.equal(formatNumber(3, 'upper_alpha'), 'C')
assert.equal(formatNumber(12, 'arabic'), '12')

// The plan: front matter in lower roman from i, the first chapter at the
// book's start, a chapter's own restart.
const settings = {
  layout: { numbering: { format: 'upper_roman', start: 3 } },
  chapter_options: { 1: { front_matter: true }, 2: { front_matter: true }, 5: { number_restart: 1, no_header: true } },
}
const chapters = [{ id: 1, title: 'Front matter' }, { id: 2, title: 'Contents' }, { id: 3, title: 'One' }, { id: 4, title: 'Two' }, { id: 5, title: 'Appendix' }]
const plan = bookPlan(settings, chapters)
assert.deepEqual(plan.map((p) => p.restart), [1, null, 3, null, 1])
assert.deepEqual(plan.map((p) => p.format), ['roman', 'roman', 'upper_roman', 'upper_roman', 'upper_roman'])
assert.deepEqual(plan.map((p) => p.header), [false, false, true, true, false])
assert.deepEqual(plan.map((p) => p.number), ['', '', '1', '2', '3'])
assert.equal(bookPlan({}, [{ id: 9 }])[0].restart, 1, 'a book starts at its first number')

// The CSS: page, mirrored margins with the gutter, bands per page name and
// side, first pages after the sides, restarts, chapter starts.
const css = pagedCSS(
  {
    page: { width_mm: 105, height_mm: 148, margin_top_mm: 12, margin_bottom_mm: 12, margin_inner_mm: 12, margin_outer_mm: 10, gutter_mm: 5, mirror: true },
    layout: { chapter_start: 'odd', numbering: { format: 'upper_roman' }, header: { first: { center: '{chapter}', uppercase: true } } },
  },
  { plan, book: 'Rainbook', font: 'Merriweather', size: 8 },
)
assert.match(css, /@page \{ size: 105mm 148mm; margin: 12mm 10mm 12mm 17mm; \}/)
assert.match(css, /@page :left \{ margin-left: 10mm; margin-right: 17mm; \}/)
assert.match(css, /@page :right \{ margin-left: 17mm; margin-right: 10mm; \}/)
assert.match(css, /@page pvmain:right \{\n {2}@top-left \{ content: none \}\n {2}@top-center \{ content: string\(pvchapter\);/)
assert.match(css, /@page pvmain:left \{\n {2}@top-left \{ content: none \}\n {2}@top-center \{ content: "Rainbook";/)
assert.match(css, /@page pvfront:right \{\n {2}@top-left \{ content: none \}\n {2}@top-center \{ content: none \}/, 'front matter has no header')
assert.match(css, /@page pvfront:right [^]*?@bottom-center \{ content: counter\(page, lower-roman\)/)
assert.match(css, /@page pvmain:right [^]*?@bottom-center \{ content: counter\(page, upper-roman\)/)
assert.match(css, /@page pvmain:first \{\n {2}@top-left \{ content: none \}\n {2}@top-center \{ content: string\(pvchapter\);[^}]*text-transform: uppercase/)
assert.ok(css.indexOf('@page pvmain:first') > css.indexOf('@page pvmain:left'), 'first pages win over sides')
assert.match(css, /#pv-ch-1 \{ counter-reset: page 1; \}/)
assert.match(css, /#pv-ch-3 \{ counter-reset: page 3; \}/)
assert.ok(!css.includes('#pv-ch-2 '), 'no restart where there is none')
assert.match(css, /\.pv-chapter\.pv-later, \.pv-full\.pv-later \{ break-before: right; \}/)
assert.match(css, /target-counter\(attr\(href\), page, upper-roman\)/)
// Without a first page of their own, chapters' first pages take the sides' bands.
assert.ok(!pagedCSS({ layout: { different_first: false } }, { plan }).includes(':first'))
// Without left/right differences, left pages take the right pages' band.
const same = pagedCSS({ layout: { different_odd_even: false } }, { plan, book: 'B' })
assert.match(same, /@page pvmain:left \{\n {2}@top-left \{ content: none \}\n {2}@top-center \{ content: string\(pvchapter\)/)

// The table of contents: headings up to its depth; a chapter without a
// Heading 1 by its title; front matter left out unless it has headings.
const h = (level, text) => ({ type: 'heading', attrs: { level }, content: [{ type: 'text', text }] })
const docs = [
  { id: 1, title: 'Front matter', doc: { type: 'doc', content: [{ type: 'paragraph' }] } },
  { id: 3, title: 'One', doc: { type: 'doc', content: [h(1, '1. Dawn'), h(2, 'Morning'), { type: 'paragraph' }] } },
  { id: 4, title: 'Two', doc: { type: 'doc', content: [h(2, 'Only a part')] } },
]
const p2 = bookPlan({ chapter_options: { 1: { front_matter: true } } }, docs)
assert.deepEqual(tocEntries(docs, p2, 1).map((e) => [e.text, e.chapter, e.block]), [['1. Dawn', 3, 0], ['Two', 4, null]])
assert.deepEqual(tocEntries(docs, p2, 2).map((e) => e.text), ['1. Dawn', 'Morning', 'Two', 'Only a part'])

console.log('bookLayout: ok')
