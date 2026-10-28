// Tables in the writing editor (src/editor/tableSchema.js, tables.js):
// the stored JSON (widths in pt) in and out, how a table is drawn, reading
// pasted tables, and the commands: inserting, Tab between cells (a new row
// from the last one), the header row, shading, merging, word counts.
// node frontend/tests/editorTables.test.mjs
import assert from 'node:assert/strict'
import { EditorState, TextSelection } from 'prosemirror-state'
import { CellSelection } from 'prosemirror-tables'
import { schema, docFromJSON, docToJSON } from '../src/editor/schema.js'
import { columnWidths, tableLayout, tableDOMAttrs, cellFromDOM, cellDOMAttrs, ptToPx, pxToPt, tableNodeSpecs } from '../src/editor/tableSchema.js'
import { insertTable, tableTab, tableShiftTab, tableInfo, setTableAttrs, setCellsAttr, makeTable, toggleHeaderRow, mergeCells, splitCell, addColumnAfter, deleteRow, tablePlugins } from '../src/editor/tables.js'
import { countDocJSON, countNode } from '../src/editor/wordCount.js'
import { editorPlugins } from '../src/editor/keymap.js'
import { styleSheet, builtinStyles } from '../src/editor/styles.js'

const txt = (text, ...marks) => (marks.length ? { type: 'text', text, marks: marks.map((m) => ({ type: m })) } : { type: 'text', text })
const para = (...content) => ({ type: 'paragraph', ...(content.length ? { content } : {}) })
const cell = (attrs, ...content) => ({ type: 'table_cell', ...(attrs ? { attrs } : {}), content: content.length ? content : [para()] })
const head = (attrs, ...content) => ({ type: 'table_header', ...(attrs ? { attrs } : {}), content: content.length ? content : [para()] })
const row = (...cells) => ({ type: 'table_row', content: cells })

// ---- The schema ----------------------------------------------------------------
for (const n of ['table', 'table_row', 'table_cell', 'table_header']) assert.ok(schema.nodes[n], n)
assert.equal(schema.nodes.table.spec.tableRole, 'table')
assert.equal(schema.nodes.table_header.spec.tableRole, 'header_cell')
assert.ok(!schema.nodes.table.isInGroup('block'), 'a table is not a block: no tables in lists or cells')
assert.ok(schema.nodes.doc.contentMatch.matchType(schema.nodes.table), 'tables at the top level')
assert.ok(schema.nodes.table_cell.contentMatch.matchType(schema.nodes.paragraph))
assert.ok(schema.nodes.table_cell.contentMatch.matchType(schema.nodes.bullet_list))
assert.ok(!schema.nodes.table_cell.contentMatch.matchType(schema.nodes.heading), 'no headings in cells')
assert.ok(!schema.nodes.table_cell.contentMatch.matchType(schema.nodes.table), 'no nested tables')
assert.ok(!schema.nodes.list_item.contentMatch.matchType(schema.nodes.paragraph).matchType(schema.nodes.table), 'no tables in list items')

// ---- JSON in and out: pt stored, px in the editor ------------------------------------
assert.equal(ptToPx(75), 100)
assert.equal(pxToPt(100), 75)
const stored = {
  type: 'doc',
  content: [
    para(txt('Before.')),
    {
      type: 'table',
      attrs: { borders: 'outer', align: 'center', width: 240, wordStyle: 'TableGrid' },
      content: [
        row(head({ colwidth: [90] }, para(txt('Stat', 'bold'))), head({ colwidth: [60.5] }, para(txt('Value')))),
        row(cell({ background: '#d9d9d9', valign: 'middle' }, para(txt('Strength')), { type: 'bullet_list', content: [{ type: 'list_item', content: [para(txt('raw'))] }] }), cell(null, para(txt('12')))),
        row(cell({ colspan: 2, colwidth: [90, 60.5] }, para(txt('Skills: Sword, Bow')))),
      ],
    },
  ],
}
const doc = docFromJSON(stored)
const table = doc.child(1)
assert.equal(table.type.name, 'table')
assert.equal(table.attrs.width, 320, 'width 240 pt is 320 px in the editor')
assert.deepEqual(table.attrs.extra, { wordStyle: 'TableGrid' }, 'unknown attrs are kept aside')
assert.deepEqual(table.child(0).child(0).attrs.colwidth, [120])
assert.equal(table.child(2).child(0).attrs.colspan, 2)
const back = docToJSON(doc)
assert.deepEqual(back, stored, 'stored JSON comes back the same (pt, colspan 1 left out, unknown attrs kept)')

// A table with cells the schema refuses (a heading) keeps its text.
const odd = docFromJSON({ type: 'doc', content: [{ type: 'table', content: [row({ type: 'table_cell', content: [{ type: 'heading', attrs: { level: 1 }, content: [txt('Hi')] }] })] }] })
assert.equal(odd.textContent, 'Hi')

// ---- Drawing ------------------------------------------------------------------------
assert.deepEqual(columnWidths(table).map((w) => Math.round(w * 100) / 100), [120, 80.67])
assert.deepEqual(tableLayout({ width: null }, [120, 80]), { width: '200px', cols: ['60%', '40%'] }, 'the columns’ total, in shares')
assert.deepEqual(tableLayout({ width: 'full' }, [120, 80]).width, '100%')
assert.deepEqual(tableLayout({ width: null }, [null, null]), { width: '100%', cols: ['', ''] }, 'no widths: the text’s width, equal columns')
assert.deepEqual(tableLayout({ width: null }, [100, null]), { width: '100%', cols: ['50%', '50%'] }, 'a column without a width counts as the others’ average')
const drawn = tableDOMAttrs(table)
assert.equal(drawn.attrs['data-borders'], 'outer')
assert.equal(drawn.attrs['data-align'], 'center')
assert.equal(drawn.attrs['data-width'], '240')
assert.match(drawn.attrs.style, /width: 320px; margin-left: auto; margin-right: auto/)
assert.deepEqual(cellDOMAttrs(table.child(1).child(0)), { 'data-background': '#d9d9d9', 'data-valign': 'middle', style: 'background-color: #d9d9d9; vertical-align: middle' })
assert.deepEqual(cellDOMAttrs(table.child(2).child(0)), { colspan: '2', 'data-colwidth': '90,60.5' })
const dom = tableNodeSpecs.table.toDOM(table)
assert.equal(dom[0], 'table')
assert.equal(dom[2][0], 'colgroup')
assert.equal(dom[2].length - 1, 2, 'one col per column')
assert.deepEqual(dom[3], ['tbody', 0])

// Cells in Normal have no first-line indent and no paragraph spacing.
assert.ok(styleSheet(builtinStyles(), '.write-doc').includes('.write-doc :where(td, th) > p:not([data-style]) { text-indent: 0; margin-top: 0; margin-bottom: 0 }'))

// ---- Pasted cells (a stand-in for a DOM element) ------------------------------------------
const el = (attrs, style = {}) => ({ getAttribute: (k) => (k in attrs ? attrs[k] : null), style })
assert.deepEqual(cellFromDOM(el({ colspan: '2', 'data-colwidth': '90,60' })), { colspan: 2, rowspan: 1, colwidth: [120, 80], background: null, valign: null })
assert.deepEqual(cellFromDOM(el({ valign: 'center' }, { width: '75pt', backgroundColor: '#D9E2F3' })), { colspan: 1, rowspan: 1, colwidth: [100], background: '#d9e2f3', valign: 'middle' }, 'Word: width in pt, shading, vertical alignment')
assert.equal(cellFromDOM(el({}, { backgroundColor: 'white' })).background, null, 'white is no shading')
assert.equal(cellFromDOM(el({ rowspan: '999999' })).rowspan, 1000, 'spans are kept sane')
const tableEl = (attrs, cellStyle) => ({ ...el(attrs, { width: attrs.styleWidth || '' }), querySelector: () => (cellStyle == null ? null : el({ style: cellStyle })) })
const parseTable = tableNodeSpecs.table.parseDOM[0].getAttrs
assert.equal(parseTable(tableEl({}, 'border:solid windowtext 1.0pt;padding:0')).borders, 'all')
assert.equal(parseTable(tableEl({}, 'border:none;padding:0')).borders, 'none')
assert.equal(parseTable(tableEl({ border: '0' }, '')).borders, 'none')
assert.equal(parseTable(tableEl({}, null)).borders, 'all', 'nothing said: all borders')
assert.equal(parseTable(tableEl({ 'data-borders': 'outer', 'data-width': 'full', 'data-align': 'center' }, null)).width, 'full')
assert.equal(parseTable(tableEl({ styleWidth: '100%' }, null)).width, 'full')

// ---- Commands ---------------------------------------------------------------------------
const plugins = [...tablePlugins(), ...editorPlugins({ getStyles: () => [] })]
const stateOf = (json, at) => {
  const d = docFromJSON(json)
  const st = EditorState.create({ schema, doc: d, plugins })
  return at == null ? st : st.apply(st.tr.setSelection(TextSelection.create(st.doc, at)))
}
const run = (st, cmd) => {
  let next = null
  const ok = cmd(st, (tr) => (next = st.apply(tr)))
  assert.ok(ok, 'the command runs')
  return next || st
}
const cellText = (st) => {
  const { $from } = st.selection
  for (let d = $from.depth; d > 0; d--) if ($from.node(d).type.spec.tableRole?.includes('cell')) return $from.node(d).textContent
  return null
}

// Insert a 2 × 3 table in the middle of a paragraph: it is split there.
let st = stateOf({ type: 'doc', content: [para(txt('Before after'))] }, 7)
st = run(st, insertTable(2, 3))
let j = docToJSON(st.doc)
assert.deepEqual(j.content.map((n) => n.type), ['paragraph', 'table', 'paragraph'])
assert.equal(j.content[0].content[0].text, 'Before')
assert.equal(j.content[2].content[0].text, ' after')
assert.equal(j.content[1].content.length, 2)
assert.equal(j.content[1].content[0].content.length, 3)
assert.equal(j.content[1].attrs.borders, 'all')
assert.equal(cellText(st), '', 'the cursor is in the first cell')
assert.equal(insertTable(2, 2)(st), false, 'tables don’t nest')

// In an empty paragraph the table takes its place.
st = run(stateOf({ type: 'doc', content: [para(txt('One')), para()] }, 6), insertTable(1, 1))
assert.deepEqual(docToJSON(st.doc).content.map((n) => n.type), ['paragraph', 'table', 'paragraph'])

// Tab and Shift+Tab move between cells; Tab in the last cell adds a row.
st = stateOf({ type: 'doc', content: [{ type: 'table', content: [row(cell(null, para(txt('a'))), cell(null, para(txt('b')))), row(cell(null, para(txt('c'))), cell(null, para(txt('d'))))] }] }, 4)
assert.equal(cellText(st), 'a')
st = run(st, tableTab)
assert.equal(cellText(st), 'b')
assert.equal(st.selection.from !== st.selection.to, true, 'the next cell’s text is selected, as in Word')
st = run(st, tableTab)
st = run(st, tableTab)
assert.equal(cellText(st), 'd')
st = run(st, tableTab)
assert.equal(st.doc.firstChild.childCount, 3, 'Tab in the last cell adds a row')
assert.equal(cellText(st), '', 'and goes to its first cell')
st = run(st, tableShiftTab)
assert.equal(cellText(st), 'd')
st = stateOf(docToJSON(st.doc), 4)
st = run(st, tableShiftTab)
assert.equal(cellText(st), 'a', 'Shift+Tab in the first cell stays')
assert.equal(tableTab(stateOf({ type: 'doc', content: [para(txt('x'))] }, 1)), false, 'outside a table Tab is the editor’s')

// Tab at the start of a list item in a cell indents the item.
st = stateOf({ type: 'doc', content: [{ type: 'table', content: [row(cell(null, { type: 'bullet_list', content: [{ type: 'list_item', content: [para(txt('one'))] }, { type: 'list_item', content: [para(txt('two'))] }] }), cell(null, para(txt('x'))))] }] })
let two = null
st.doc.descendants((n, pos) => {
  if (n.isText && n.text === 'two') two = pos
})
st = st.apply(st.tr.setSelection(TextSelection.create(st.doc, two)))
st = run(st, tableTab)
assert.equal(cellText(st), 'onetwo', 'still in the same cell')
assert.equal(st.selection.$from.depth > 6, true, 'the item went one level in')

// The table box's information, header row, attrs, shading, merge and split.
st = stateOf(stored, 13)
let info = tableInfo(st)
assert.equal(info.rows, 3)
assert.equal(info.cols, 2)
assert.equal(info.headerRow, true)
assert.equal(info.attrs.borders, 'outer')
st = run(st, toggleHeaderRow)
assert.equal(tableInfo(st).headerRow, false)
st = run(st, setTableAttrs({ borders: 'none', align: null, width: 'full' }))
info = tableInfo(st)
assert.deepEqual([info.attrs.borders, info.attrs.align, info.attrs.width], ['none', null, 'full'])
st = run(st, setCellsAttr('background', '#fff2cc'))
assert.equal(tableInfo(st).cell.background, '#fff2cc')
assert.equal(docToJSON(st.doc).content[1].content[0].content[0].attrs.background, '#fff2cc')
// Select the two cells of the second row and merge them.
const t0 = st.doc.child(1)
const rowStart = st.doc.child(0).nodeSize + 1 + t0.child(0).nodeSize // the second row
const a = rowStart + 1
const b = a + t0.child(1).child(0).nodeSize
st = st.apply(st.tr.setSelection(CellSelection.create(st.doc, a, b)))
assert.equal(tableInfo(st).canMerge, true)
st = run(st, mergeCells)
const merged = docToJSON(st.doc).content[1].content[1].content
assert.equal(merged.length, 1)
assert.equal(merged[0].attrs.colspan, 2)
st = run(st, splitCell)
assert.equal(docToJSON(st.doc).content[1].content[1].content.length, 2)
st = run(st, addColumnAfter)
assert.equal(tableInfo(st).cols, 3)
st = run(st, deleteRow)
assert.equal(tableInfo(st).rows, 2)
assert.equal(setTableAttrs({ borders: 'all' })(stateOf({ type: 'doc', content: [para()] }, 1)), false, 'outside a table')

// A new table with a header row.
const made = makeTable(2, 2, { header: true })
assert.equal(made.child(0).child(0).type.name, 'table_header')
assert.equal(made.child(1).child(0).type.name, 'table_cell')

// ---- Word counts: words in cells count -----------------------------------------------------
assert.equal(countDocJSON(stored), 9)
assert.equal(countNode(docFromJSON(stored)), 9)

console.log('editorTables: ok')
