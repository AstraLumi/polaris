// Symbols in fonts (src/editor/glyphs.js: blocks, unicode-range) and the
// Symbols panel's insert command (insertSymbol in src/editor/commands.js).
// node frontend/tests/glyphs.test.mjs
import assert from 'node:assert/strict'
import { EditorState, TextSelection } from 'prosemirror-state'
import { SYMBOL_BLOCKS, blockChars, codeOf, parseUnicodeRange, inRanges, quoted } from '../src/editor/glyphs.js'
import { schema, docFromJSON, docToJSON } from '../src/editor/schema.js'
import { insertSymbol } from '../src/editor/commands.js'

// Blocks: unique keys, ranges in order, only characters that can show.
assert.equal(new Set(SYMBOL_BLOCKS.map((b) => b.key)).size, SYMBOL_BLOCKS.length)
for (const b of SYMBOL_BLOCKS) for (const [from, to] of b.ranges) assert.ok(from <= to, b.key)
const misc = blockChars(SYMBOL_BLOCKS.find((b) => b.key === 'misc'))
assert.ok(misc.includes('★') && misc.includes('☀') && misc.length === 256, 'the miscellaneous symbols block is all there')
const punct = blockChars(SYMBOL_BLOCKS.find((b) => b.key === 'punctuation'))
assert.ok(punct.includes('—') && punct.includes('§'))
assert.ok(!punct.includes('​') && !punct.includes(' ') && !punct.includes('­'), 'no invisible characters')
const emoji = blockChars(SYMBOL_BLOCKS.find((b) => b.key === 'emoji'))
assert.ok(emoji.includes('🗡') && emoji.includes('🐉'), 'astral characters')
assert.deepEqual(blockChars(null), [])
assert.equal(codeOf('★'), 'U+2605')
assert.equal(codeOf('🐉'), 'U+1F409')
assert.equal(codeOf('A'), 'U+0041')

// unicode-range, as the bundled fonts' stylesheets write it.
const css = `@font-face { font-family: 'X'; unicode-range: U+2800-28FF; }
@font-face { unicode-range: U+0302-0303, U+0305, U+26??, junk; }`
const ranges = parseUnicodeRange(css)
assert.deepEqual(ranges, [[0x2800, 0x28ff], [0x302, 0x303], [0x305, 0x305], [0x2600, 0x26ff]])
assert.ok(inRanges(ranges, '⠁') && inRanges(ranges, '★') && !inRanges(ranges, 'A'))
assert.deepEqual(parseUnicodeRange(''), [])
assert.equal(quoted('Noto "Sans"'), '"Noto Sans"')

// insertSymbol: at the cursor, in the font chosen, or with no font mark.
const json = {
  type: 'doc',
  content: [{ type: 'paragraph', content: [{ type: 'text', text: 'ab', marks: [{ type: 'bold' }, { type: 'font', attrs: { family: 'Cambria' } }] }] }],
}
const at = (pos) => {
  const st = EditorState.create({ schema, doc: docFromJSON(json) })
  return st.apply(st.tr.setSelection(TextSelection.create(st.doc, pos)))
}
const run = (st, cmd) => {
  let out = st
  assert.ok(cmd(st, (tr) => (out = st.apply(tr))))
  return out
}
let st = run(at(2), insertSymbol('★', 'Noto Sans Symbols 2'))
let content = docToJSON(st.doc).content[0].content
assert.deepEqual(content.map((n) => n.text), ['a', '★', 'b'])
const byType = (ms) => [...ms].sort((a, b) => a.type.localeCompare(b.type))
assert.deepEqual(byType(content[1].marks), [{ type: 'bold' }, { type: 'font', attrs: { family: 'Noto Sans Symbols 2' } }], 'the symbol keeps the bold, in its own font')
assert.equal(st.selection.from, 3, 'the cursor after it')
st = run(at(2), insertSymbol('∞', ''))
content = docToJSON(st.doc).content[0].content
assert.deepEqual(content[1], { type: 'text', text: '∞', marks: [{ type: 'bold' }] }, 'the body font: no font mark')
assert.equal(insertSymbol('', 'X')(at(1)), false)

console.log('glyphs: ok')
