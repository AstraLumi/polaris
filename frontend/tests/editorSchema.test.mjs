// The writing editor's document schema and CSS readers (src/editor/schema.js,
// css.js): JSON in and out keeps what it doesn't know, pasted CSS reads into
// the document's units. node frontend/tests/editorSchema.test.mjs
import assert from 'node:assert/strict'
import { schema, docFromJSON, docToJSON, emptyDocJSON, ownPara } from '../src/editor/schema.js'
import {
  cssLengthToPt,
  cssColorToHex,
  cssLineHeight,
  cssFontFamily,
  cssFontSizeToPt,
  paraFromCSS,
  paraDeclarations,
  textDeclarations,
  mmToPt,
  ptToMm,
} from '../src/editor/css.js'

// ---- The schema has exactly EDITOR.md's nodes and marks -----------------------
assert.deepEqual(Object.keys(schema.nodes).sort(), ['bullet_list', 'doc', 'hard_break', 'heading', 'image', 'list_item', 'ordered_list', 'page_break', 'paragraph', 'scene_break', 'tab', 'text', 'toc', 'dialogue', 'table', 'table_row', 'table_cell', 'table_header'].sort())
assert.deepEqual(
  Object.keys(schema.marks).sort(),
  ['bold', 'char_style', 'color', 'font', 'highlight', 'italic', 'link', 'size', 'smallcaps', 'story_link', 'strike', 'subscript', 'superscript', 'underline'],
)
const paraAttrs = Object.keys(schema.nodes.paragraph.attrs).filter((k) => k !== 'extra').sort()
assert.deepEqual(paraAttrs, ['align', 'indentFirst', 'indentLeft', 'indentRight', 'keepNext', 'lineHeight', 'spaceAfter', 'spaceBefore', 'style'])
assert.deepEqual(Object.keys(schema.nodes.heading.attrs).filter((k) => k !== 'extra').sort(), ['level', ...paraAttrs].sort())

// ---- Round trips ------------------------------------------------------------
const stored = {
  type: 'doc',
  content: [
    {
      type: 'heading',
      attrs: { level: 1, align: 'center' },
      content: [{ type: 'text', text: 'One' }],
    },
    {
      type: 'paragraph',
      attrs: { style: 'quote', indentFirst: 14.2, keepNext: true, wordShading: 'D9D9D9' },
      content: [
        { type: 'text', text: 'Hello ', marks: [{ type: 'font', attrs: { family: 'Merriweather' } }, { type: 'size', attrs: { pt: 8 } }] },
        { type: 'text', text: 'there', marks: [{ type: 'bold' }, { type: 'color', attrs: { color: '#aa0000', theme: 'accent1' } }] },
        { type: 'hard_break' },
        { type: 'tab' },
        { type: 'text', text: 'end', marks: [{ type: 'story_link', attrs: { type: 'character', id: 12 } }] },
      ],
    },
    { type: 'scene_break' },
    { type: 'scene_break', attrs: { glyph: '~', image: null } },
    { type: 'page_break' },
    { type: 'image', attrs: { src: '/uploads/a.png', alt: 'A', width: 120, align: 'left', fullPage: true } },
    { type: 'bullet_list', content: [{ type: 'list_item', content: [{ type: 'paragraph', content: [{ type: 'text', text: 'item' }] }] }] },
    { type: 'ordered_list', attrs: { start: 3 }, content: [{ type: 'list_item', content: [{ type: 'paragraph' }] }] },
  ],
}
const doc = docFromJSON(stored)
assert.equal(doc.childCount, 8)
const p = doc.child(1)
assert.deepEqual(p.attrs.extra, { wordShading: 'D9D9D9' }, 'an unknown paragraph attr is kept aside')
const back = docToJSON(doc)
assert.deepEqual(back.content[1].attrs, { style: 'quote', indentFirst: 14.2, keepNext: true, wordShading: 'D9D9D9' }, 'and put back')
assert.deepEqual(back.content[1].content[1].marks.find((m) => m.type === 'color'), { type: 'color', attrs: { color: '#aa0000', theme: 'accent1' } }, 'unknown mark attrs too')
assert.deepEqual(back.content[0].attrs, { level: 1, align: 'center' }, 'nulls and false are left out')
assert.deepEqual(back.content[2], { type: 'scene_break', attrs: { glyph: '* * *' } }, 'other defaults are written')
assert.deepEqual(back.content[1].content[4].marks[0].attrs, { type: 'character', id: 12 })
assert.deepEqual(back, docToJSON(docFromJSON(back)), 'stable')
assert.equal(JSON.stringify(docToJSON(docFromJSON(back))), JSON.stringify(back))

// Text nodes don't get attrs or empty marks.
assert.deepEqual(back.content[0].content[0], { type: 'text', text: 'One' })

// ---- Bad input -----------------------------------------------------------------
assert.deepEqual(docToJSON(docFromJSON(null)), emptyDocJSON(), 'nothing → an empty paragraph')
assert.deepEqual(docToJSON(docFromJSON({ type: 'doc', content: [] })), emptyDocJSON())
// An unknown block keeps its text as a paragraph; an unknown mark goes.
const odd = docToJSON(
  docFromJSON({
    type: 'doc',
    content: [
      { type: 'table', content: [{ type: 'row', content: [{ type: 'text', text: 'cell' }] }] },
      { type: 'paragraph', content: [{ type: 'text', text: 'x', marks: [{ type: 'glow' }] }, { type: 'emoji', attrs: { name: 'star' } }] },
    ],
  }),
)
assert.deepEqual(odd.content[0], { type: 'paragraph', content: [{ type: 'text', text: 'cell' }] })
assert.deepEqual(odd.content[1], { type: 'paragraph', content: [{ type: 'text', text: 'x' }] })
// Content in the wrong place (a heading in a list item) keeps the text.
const wrong = docFromJSON({ type: 'doc', content: [{ type: 'bullet_list', content: [{ type: 'list_item', content: [{ type: 'heading', content: [{ type: 'text', text: 'deep' }] }] }] }] })
assert.equal(wrong.textContent, 'deep')

assert.deepEqual(ownPara({ style: 'x', align: null, indentFirst: 0, keepNext: false }), { indentFirst: 0 })

// ---- CSS readers -----------------------------------------------------------------
assert.equal(cssLengthToPt('12pt'), 12)
assert.equal(cssLengthToPt('16px'), 12)
assert.equal(cssLengthToPt('1in'), 72)
assert.equal(cssLengthToPt('2.54cm'), 72)
assert.equal(cssLengthToPt('5mm'), 14.17)
assert.equal(cssLengthToPt('0'), 0)
assert.equal(cssLengthToPt('0cm'), 0)
assert.equal(cssLengthToPt('-18pt'), -18)
assert.equal(cssLengthToPt('2em'), null)
assert.equal(cssLengthToPt('auto'), null)
assert.equal(cssFontSizeToPt('11pt'), 11)
assert.equal(cssFontSizeToPt('medium'), null)
assert.equal(mmToPt(5), 14.17)
assert.equal(ptToMm(14.17), 5)

assert.equal(cssColorToHex('#ABC'), '#aabbcc')
assert.equal(cssColorToHex('#a1b2c3'), '#a1b2c3')
assert.equal(cssColorToHex('rgb(255, 0, 10)'), '#ff000a')
assert.equal(cssColorToHex('rgba(0, 0, 0, 0)'), null)
assert.equal(cssColorToHex('rgb(1 2 3 / 50%)'), '#010203')
assert.equal(cssColorToHex('transparent'), null)
assert.equal(cssColorToHex('windowtext'), null)
assert.equal(cssColorToHex('navy'), '#000080')

assert.equal(cssLineHeight('150%'), 1.5)
assert.equal(cssLineHeight('1.38'), 1.38)
assert.equal(cssLineHeight('normal'), 1)
assert.equal(cssLineHeight('12pt'), null)

assert.equal(cssFontFamily('"Times New Roman", serif'), 'Times New Roman')
assert.equal(cssFontFamily("'Turret Road'"), 'Turret Road')
assert.equal(cssFontFamily('serif'), null)
assert.equal(cssFontFamily(''), null)

// A Word paragraph's inline style.
assert.deepEqual(
  paraFromCSS({ textAlign: 'justify', textIndent: '14.2pt', marginTop: '0cm', marginBottom: '8pt', marginLeft: '', lineHeight: '115%' }),
  { align: 'justify', indentFirst: 14.2, spaceBefore: 0, spaceAfter: 8, lineHeight: 1.15 },
)
assert.deepEqual(paraFromCSS({ textAlign: 'start' }), { align: 'left' })
assert.deepEqual(paraFromCSS({ textAlign: 'inherit' }), {})

assert.deepEqual(paraDeclarations({ align: 'center', indentFirst: -10, lineHeight: 1, keepNext: false }), ['text-align: center', 'text-indent: -10pt', 'line-height: 1.2'])
assert.deepEqual(textDeclarations({ font: 'Turret Road', size: 8, color: null, bold: false, italic: null }), ['font-family: "Turret Road", serif', 'font-size: 8pt', 'font-weight: 400'])

console.log('editorSchema: ok')
