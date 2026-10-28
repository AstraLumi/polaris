// Tables in the editor (docs/EDITOR.md, stage 5): inserting one, Tab and
// Shift+Tab between cells (Tab in the last cell adds a row, as in Word),
// column resizing, rows and columns, merging and splitting, the header row,
// shading, alignment, width and borders. prosemirror-tables does the cell
// selection, the arrow keys and the pasting of cells; this adds the
// commands the table box (TableOptions.vue) and the toolbar run. The nodes
// and how they are drawn are in tableSchema.js.
import { keymap } from 'prosemirror-keymap'
import { Plugin, TextSelection } from 'prosemirror-state'
import {
  CellSelection,
  TableMap,
  TableView,
  addColumnAfter,
  addColumnBefore,
  addRowAfter,
  addRowBefore,
  columnResizing,
  deleteCellSelection,
  deleteColumn,
  deleteRow,
  deleteTable,
  goToNextCell,
  isInTable,
  mergeCells,
  selectedRect,
  setCellAttr,
  splitCell,
  tableEditing,
  toggleHeaderRow,
} from 'prosemirror-tables'
import { schema } from './schema.js'
import { MAX_TABLE_COLS, MAX_TABLE_ROWS, tableDOMAttrs } from './tableSchema.js'
import { cssColorToHex } from './css.js'
import { indentListItem, listLevels } from './keymap.js'

const { table, table_row: tableRow, table_cell: tableCell, table_header: tableHeader, paragraph } = schema.nodes

export { isInTable, addColumnAfter, addColumnBefore, addRowAfter, addRowBefore, deleteColumn, deleteRow, deleteTable, mergeCells, splitCell, toggleHeaderRow }

// The table the selection is in: { node, pos } or null.
export function tableAt(state) {
  const { $from } = state.selection
  for (let d = $from.depth; d > 0; d--) if ($from.node(d).type === table) return { node: $from.node(d), pos: $from.before(d) }
  return null
}

// A new table of rows × cols empty cells (a header row when header).
export function makeTable(rows, cols, { header = false } = {}) {
  rows = Math.max(1, Math.min(MAX_TABLE_ROWS, rows | 0))
  cols = Math.max(1, Math.min(MAX_TABLE_COLS, cols | 0))
  const list = []
  for (let r = 0; r < rows; r++) {
    const type = header && r === 0 ? tableHeader : tableCell
    const cells = []
    for (let c = 0; c < cols; c++) cells.push(type.create(null, paragraph.create()))
    list.push(tableRow.create(null, cells))
  }
  return table.create({ borders: 'all' }, list)
}

// Puts a table at the cursor (after the list or table the cursor is in;
// the paragraph is split where the cursor is) and the cursor in its first
// cell. Not inside a table: tables don't nest.
export const insertTable = (rows, cols, opts) => (state, dispatch) => {
  if (tableAt(state)) return false
  if (!dispatch) return true
  const tr = state.tr
  if (!state.selection.empty) tr.deleteSelection()
  const $pos = tr.selection.$from
  let at
  if (!$pos.parent.isTextblock) at = $pos.pos
  else if ($pos.depth > 1) at = $pos.after(1)
  else if ($pos.parentOffset === 0 && $pos.parent.content.size) at = $pos.before()
  else if ($pos.parentOffset === $pos.parent.content.size) at = $pos.after()
  else {
    tr.split($pos.pos)
    at = $pos.pos + 1
  }
  // An empty paragraph the table goes after is replaced by it.
  const before = tr.doc.resolve(at).nodeBefore
  const node = makeTable(rows, cols, opts)
  if (before && before.type === paragraph && !before.content.size && $pos.depth === 1 && at === $pos.after()) {
    tr.replaceWith(at - before.nodeSize, at, node)
    at -= before.nodeSize
  } else tr.insert(at, node)
  const after = at + node.nodeSize
  // Text can always be typed after the table.
  if (!tr.doc.resolve(after).nodeAfter) tr.insert(after, paragraph.create())
  tr.setSelection(TextSelection.near(tr.doc.resolve(at + 3)))
  dispatch(tr.scrollIntoView())
  return true
}

const inCellList = (state) => {
  const { $from, empty } = state.selection
  return empty && $from.parentOffset === 0 && listLevels($from) > 0
}

// Tab: at the start of a list item, the item goes one level deeper (as
// outside tables); else the next cell, and in the last cell a new row.
export const tableTab = (state, dispatch) => {
  if (!isInTable(state)) return false
  if (inCellList(state) && indentListItem(schema.nodes.list_item)(state, dispatch)) return true
  if (goToNextCell(1)(state, dispatch)) return true
  if (!dispatch) return true
  // The last cell: a row after it, and the cursor in its first cell.
  let tr = null
  addRowAfter(state, (t) => (tr = t))
  if (!tr) return true
  let sel = null
  goToNextCell(1)(state.apply(tr), (t) => (sel = t.selection))
  if (sel) tr.setSelection(TextSelection.create(tr.doc, sel.anchor, sel.head))
  dispatch(tr.scrollIntoView())
  return true
}

// Shift+Tab: back a cell (out a list level at an item's start). In the
// first cell it does nothing (Word stays there).
export const tableShiftTab = (state, dispatch) => {
  if (!isInTable(state)) return false
  if (inCellList(state)) {
    const { $from } = state.selection
    if ($from.depth >= 2 && $from.node(-1).type === schema.nodes.list_item) return false // the list's Shift+Tab
  }
  goToNextCell(-1)(state, dispatch)
  return true
}

// Sets attrs on the table the selection is in.
export const setTableAttrs = (patch) => (state, dispatch) => {
  const t = tableAt(state)
  if (!t) return false
  if (dispatch) dispatch(state.tr.setNodeMarkup(t.pos, null, { ...t.node.attrs, ...patch }))
  return true
}

// Sets an attr on the selected cells (or the cell the cursor is in).
export const setCellsAttr = (name, value) => (state, dispatch) => {
  if (!isInTable(state)) return false
  return setCellAttr(name, value)(state, dispatch)
}

// What the table box shows: the table's attrs, the cell's, what can be done.
export function tableInfo(state) {
  const t = tableAt(state)
  if (!t) return null
  const map = TableMap.get(t.node)
  let cell = null
  const sel = state.selection
  if (sel instanceof CellSelection) cell = sel.$anchorCell.nodeAfter
  else {
    const { $from } = sel
    for (let d = $from.depth; d > 0; d--) {
      const n = $from.node(d)
      if (n.type === tableCell || n.type === tableHeader) {
        cell = n
        break
      }
    }
  }
  let headerRow = t.node.firstChild?.childCount > 0
  t.node.firstChild?.forEach((c) => {
    if (c.type !== tableHeader) headerRow = false
  })
  let rect = null
  try {
    rect = selectedRect(state)
  } catch (e) {
    rect = null
  }
  return {
    pos: t.pos,
    attrs: t.node.attrs,
    cell: cell ? cell.attrs : null,
    rows: map.height,
    cols: map.width,
    headerRow,
    canMerge: mergeCells(state),
    canSplit: splitCell(state),
    row: rect ? rect.top : 0,
  }
}

// The table drawn with its attrs: prosemirror-tables' view (the columns'
// colgroup, kept up to date while resizing) with tableSchema.js's drawing.
export class PolarisTableView extends TableView {
  constructor(node, cellMinWidth) {
    super(node, cellMinWidth)
    this.draw(node)
  }

  update(node) {
    if (!super.update(node)) return false
    this.draw(node)
    return true
  }

  draw(node) {
    const { attrs, cols } = tableDOMAttrs(node)
    for (const name of ['data-align', 'data-width']) if (!(name in attrs)) this.table.removeAttribute(name)
    for (const [k, v] of Object.entries(attrs)) this.table.setAttribute(k, v)
    this.dom.className = `tableWrapper${node.attrs.align ? ` is-${node.attrs.align}` : ''}`
    let col = this.colgroup.firstChild
    for (const w of cols) {
      if (!col) break
      col.style.width = w
      col = col.nextSibling
    }
  }

  ignoreMutation(record) {
    return record.type === 'attributes' && (record.target === this.dom || super.ignoreMutation(record))
  }
}

// Pasted HTML: a cell's shading goes to its data-background, so it is the
// cell's background and isn't also read as a highlight on its text.
export function cellShadingToData(html) {
  if (!/<t[dh][\s>]/i.test(html) || typeof DOMParser === 'undefined') return html
  const doc = new DOMParser().parseFromString(html, 'text/html')
  let changed = false
  for (const el of doc.querySelectorAll('td, th')) {
    const bg = el.style.backgroundColor
    if (!bg) continue
    const c = cssColorToHex(bg)
    if (c && c !== '#ffffff' && !el.hasAttribute('data-background')) el.setAttribute('data-background', c)
    el.style.removeProperty('background')
    el.style.removeProperty('background-color')
    changed = true
  }
  return changed ? doc.body.innerHTML : html
}

// The plugins: cell selection, arrows and pasting (tableEditing), column
// resizing, and the keys. Put before the editor's own keymap, so Tab in a
// table moves between cells.
export function tablePlugins() {
  return [
    columnResizing({ View: PolarisTableView, cellMinWidth: 24, defaultCellMinWidth: 48 }),
    tableEditing(),
    new Plugin({ props: { transformPastedHTML: cellShadingToData } }),
    keymap({
      Tab: tableTab,
      'Shift-Tab': tableShiftTab,
      Backspace: deleteCellSelection,
      Delete: deleteCellSelection,
      'Mod-Backspace': deleteCellSelection,
      'Mod-Delete': deleteCellSelection,
    }),
  ]
}
