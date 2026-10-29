// A book's styles and page types (src/editor/styles.js, pageTypes.js).
// node frontend/tests/editorStyles.test.mjs
import assert from 'node:assert/strict'
import {
  BUILTIN_IDS,
  builtinStyles,
  resolveStyle,
  resolveBlock,
  blockStyleId,
  styleSheet,
  newStyleId,
  basedOnChoices,
  styleFonts,
} from '../src/editor/styles.js'
import {
  PAGE_TYPE_KEYS,
  defaultSettings,
  defaultPage,
  normaliseSettings,
  switchPageType,
  textBlockMm,
  pageTypeName,
} from '../src/editor/pageTypes.js'

// ---- Built-ins ----------------------------------------------------------------
const built = builtinStyles(8, { font: 'Merriweather' })
assert.deepEqual(built.map((s) => s.id), BUILTIN_IDS)
assert.ok(built.every((s) => s.builtin && s.kind === 'paragraph'))
assert.equal(built[0].based_on, null, 'Normal is the end of every chain')

// ---- Resolution: own attrs > style > based_on > normal ----------------------------
const styles = [
  ...built,
  { id: 'letter', name: 'Letter', kind: 'paragraph', based_on: 'quote', para: { align: 'right' }, text: { font: 'Turret Road' } },
  { id: 'loopA', name: 'A', kind: 'paragraph', based_on: 'loopB', para: { align: 'center' }, text: {} },
  { id: 'loopB', name: 'B', kind: 'paragraph', based_on: 'loopA', para: {}, text: { size: 20 } },
]
const normal = resolveStyle(styles, 'normal')
assert.equal(normal.para.align, 'justify')
assert.equal(normal.text.font, 'Merriweather')
assert.equal(normal.text.size, 8)

const letter = resolveStyle(styles, 'letter')
assert.equal(letter.para.align, 'right', 'its own')
assert.equal(letter.para.indentLeft, 28, 'from quote')
assert.equal(letter.text.italic, true, 'from quote')
assert.equal(letter.text.font, 'Turret Road')
assert.equal(letter.text.size, 8, 'from normal')
assert.equal(letter.para.lineHeight, 1.15, 'from normal')

const sub = resolveStyle(styles, 'subtitle')
assert.equal(sub.para.align, 'center', 'its own')
assert.equal(sub.para.spaceBefore, 0, 'from normal')

assert.deepEqual(resolveStyle(styles, 'nope'), normal, 'an unknown style is Normal')
const loop = resolveStyle(styles, 'loopA')
assert.equal(loop.para.align, 'center', 'a loop stops instead of hanging')
assert.equal(loop.text.font, 'Merriweather')

assert.equal(blockStyleId('heading', { level: 2, style: null }, styles), 'heading2')
assert.equal(blockStyleId('heading', { level: 5, style: null }, styles), 'normal')
assert.equal(blockStyleId('heading', { level: 1, style: 'letter' }, styles), 'letter')
assert.equal(blockStyleId('paragraph', { style: null }, styles), 'normal')

const own = resolveBlock(styles, 'paragraph', { style: 'letter', align: 'center', indentFirst: 0, keepNext: false }, [
  { type: { name: 'size' }, attrs: { pt: 12 } },
  { type: { name: 'bold' }, attrs: {} },
])
assert.equal(own.para.align, 'center', 'the paragraph over its style')
assert.equal(own.para.indentFirst, 0)
assert.equal(own.para.indentLeft, 28)
assert.equal(own.text.size, 12, 'marks over the style')
assert.equal(own.text.bold, true)
assert.equal(own.text.font, 'Turret Road')

// ---- The stylesheet -------------------------------------------------------------------
const css = styleSheet(styles, '.doc')
assert.match(css, /^\.doc \{ font-family: "Merriweather", serif; font-size: 8pt; font-weight: 400; font-style: normal \}/)
assert.match(css, /\.doc \[data-style="heading1"\], \.doc h1:not\(\[data-style\]\) \{[^}]*font-size: 12.8pt[^}]*font-weight: 700[^}]*text-align: center/)
assert.match(css, /\.doc \[data-style="letter"\] \{ font-family: "Turret Road", serif; font-size: 8pt;[^}]*font-style: italic; text-align: right;[^}]*margin-left: 28pt/)
assert.match(css, /\.doc :where\(p, h1, h2, h3, h4, h5, h6\) \{[^}]*text-indent: 14.2pt/)
assert.ok(!/data-style="x"y"/.test(styleSheet([{ id: 'x"y', kind: 'paragraph', para: {}, text: {} }])), 'ids are escaped')
assert.match(styleSheet([{ id: 'x"y', kind: 'paragraph', para: {}, text: {} }]), /\[data-style="x\\"y"\]/)

assert.deepEqual(styleFonts(styles), ['Merriweather', 'Turret Road'])
assert.equal(newStyleId(styles, 'Letter'), 'letter-2')
assert.equal(newStyleId(styles, 'Carta Ñandú'), 'carta-nandu')
assert.equal(newStyleId(styles, '***'), 'style')
const choices = basedOnChoices(styles, 'quote').map((s) => s.id)
assert.ok(!choices.includes('quote') && !choices.includes('letter'), 'not itself nor what is based on it')
assert.ok(choices.includes('normal') && choices.includes('title'))

// ---- Page types -------------------------------------------------------------------------
assert.deepEqual(PAGE_TYPE_KEYS, ['a4', 'a5', 'a6', 'custom', 'online'])
const a6 = defaultSettings('a6')
assert.equal(a6.page_type, 'a6')
assert.deepEqual(a6.page, { width_mm: 105, height_mm: 148, margin_top_mm: 12, margin_bottom_mm: 12, margin_inner_mm: 12, margin_outer_mm: 12, gutter_mm: 5, mirror: true })
assert.equal(resolveStyle(a6.styles, 'normal').text.size, 8)
assert.equal(resolveStyle(a6.styles, 'normal').para.indentFirst, 14.2)
assert.equal(resolveStyle(a6.styles, 'normal').text.font, 'Merriweather', 'as the server')
assert.equal(textBlockMm(a6), 76)
assert.equal(resolveStyle(defaultSettings('a4').styles, 'normal').text.size, 12)
assert.equal(resolveStyle(defaultSettings('a5').styles, 'normal').text.size, 10.5)
assert.equal(resolveStyle(defaultSettings('custom').styles, 'normal').text.size, 10.5)

const online = defaultSettings('online')
assert.equal(online.page.width_mm, 148, 'kept but ignored')
assert.equal(textBlockMm(online), null)
const on = resolveStyle(online.styles, 'normal')
assert.equal(on.para.align, 'left')
assert.equal(on.para.indentFirst, 0)
assert.equal(on.para.spaceAfter, 12, 'a blank line between paragraphs')
assert.equal(on.text.size, 12)

const custom = defaultSettings('custom', { width_mm: 127, height_mm: 203.2 })
assert.equal(custom.page.width_mm, 127)
assert.equal(pageTypeName(custom), '127 × 203.2 mm')
assert.equal(defaultPage('custom', { width_mm: 3 }).width_mm, 127, 'out of range: the default')
assert.equal(pageTypeName(a6), 'A6')

// Unknown or partial settings are made whole.
const n = normaliseSettings({ page_type: 'scroll', styles: [{ id: 'normal', name: 'Corpo', builtin: true, para: {}, text: { size: 9 } }] })
assert.equal(n.page_type, 'a5')
assert.deepEqual(n.styles.map((s) => s.id), BUILTIN_IDS)
assert.equal(n.styles[0].name, 'Corpo', 'what was there stays')
assert.equal(n.page.width_mm, 148)

// Switching keeps the names the writer gave and their own styles.
const mine = { ...a6, styles: a6.styles.map((s) => (s.id === 'normal' ? { ...s, name: 'Body' } : s)).concat(styles[7]) }
const sw = switchPageType(mine, 'a4')
assert.equal(sw.page_type, 'a4')
assert.equal(sw.page.width_mm, 210)
assert.equal(sw.styles[0].name, 'Body')
assert.equal(resolveStyle(sw.styles, 'normal').text.size, 12)
assert.ok(sw.styles.some((s) => s.id === 'letter'))
assert.equal(textBlockMm(sw), 155)

// The server's defaults win when it sends them.
const server = { a6: { page_type: 'a6', page: { width_mm: 105, height_mm: 148, margin_top_mm: 10, margin_bottom_mm: 10, margin_inner_mm: 10, margin_outer_mm: 10, gutter_mm: 4, mirror: true }, styles: builtinStyles(8, { font: 'Lora' }) } }
const fromServer = defaultSettings('a6', {}, server)
assert.equal(fromServer.page.margin_top_mm, 10)
assert.equal(resolveStyle(fromServer.styles, 'normal').text.font, 'Lora')
assert.notEqual(fromServer.styles, server.a6.styles, 'a copy')
assert.equal(switchPageType(mine, 'a6', {}, server).page.gutter_mm, 4)
assert.equal(defaultSettings('custom', { width_mm: 140, height_mm: 900 }, {}).page.height_mm, 900, 'up to 1000 mm')

console.log('editorStyles: ok')
