// The editor's plugins: Word's keyboard shortcuts (docs/EDITOR.md), the
// typing helpers (smart quotes, -- to an em dash, ... to an ellipsis, each
// undone by Ctrl+Z right after), history, and the drop and gap cursors.
import { keymap } from 'prosemirror-keymap'
import { history, undo, redo } from 'prosemirror-history'
import { baseKeymap, chainCommands, createParagraphNear, liftEmptyBlock, selectAll } from 'prosemirror-commands'
import { InputRule, inputRules, undoInputRule } from 'prosemirror-inputrules'
import { splitListItem, sinkListItem, liftListItem } from 'prosemirror-schema-list'
import { dropCursor } from 'prosemirror-dropcursor'
import { gapCursor } from 'prosemirror-gapcursor'
import { schema } from './schema.js'
import {
  toggleMark,
  setParaAttrs,
  insertBlock,
  stepFontSize,
  nudgeFontSize,
  changeCase,
  insertTab,
  insertHardBreak,
  splitParagraph,
  clearMarks,
} from './commands.js'

// Quotes open at the start, after a space, a bracket, a dash or another quote.
const OPEN = '(?:^|[\\s{[(<\'"‘“—–])'

export const typingRules = [
  new InputRule(/--$/, '—'),
  new InputRule(/\.\.\.$/, '…'),
  new InputRule(new RegExp(`${OPEN}(")$`), '“'),
  new InputRule(/"$/, '”'),
  new InputRule(new RegExp(`${OPEN}(')$`), '‘'),
  new InputRule(/'$/, '’'),
]

// Word's nine list levels are as deep as a list goes here too (a deeper
// one would not fit the server's limit on nesting).
export const MAX_LIST_LEVELS = 9

export function listLevels($pos) {
  let n = 0
  for (let d = $pos.depth; d > 0; d--) if ($pos.node(d).type === schema.nodes.bullet_list || $pos.node(d).type === schema.nodes.ordered_list) n++
  return n
}

// Tab in a list item: one level deeper (under the item before it). Past the
// last level, or at the start of the list's first item, it does nothing
// rather than type a tab into the item.
export const indentListItem = (li) => (state, dispatch) => {
  const { $from, empty } = state.selection
  const levels = listLevels($from)
  if (!levels) return false
  if (levels >= MAX_LIST_LEVELS) return true
  if (sinkListItem(li)(state, dispatch)) return true
  return empty && $from.parentOffset === 0
}

// Enter in an empty item that has nothing under it: out one level (to the
// text after the list from the first level), as Word does.
export const outdentEmptyListItem = (li) => (state, dispatch) => {
  const { $from, empty } = state.selection
  if (!empty || $from.parent.type !== schema.nodes.paragraph || $from.parent.content.size) return false
  if ($from.depth < 2 || $from.node(-1).type !== li || $from.node(-1).childCount !== 1) return false
  return liftListItem(li)(state, dispatch)
}

// getStyles: the book's styles now (for the size steps).
export function wordKeymap(getStyles) {
  const li = schema.nodes.list_item
  const size = (dir) => (s, d) => stepFontSize(dir, getStyles())(s, d)
  const nudge = (by) => (s, d) => nudgeFontSize(by, getStyles())(s, d)
  const lines = (lineHeight) => setParaAttrs({ lineHeight })
  return {
    'Mod-b': toggleMark('bold'),
    'Mod-i': toggleMark('italic'),
    'Mod-u': toggleMark('underline'),
    'Mod-e': setParaAttrs({ align: 'center' }),
    'Mod-l': setParaAttrs({ align: 'left' }),
    'Mod-r': setParaAttrs({ align: 'right' }),
    'Mod-j': setParaAttrs({ align: 'justify' }),
    'Mod-Enter': insertBlock('page_break'),
    'Mod-1': lines(1),
    'Mod-2': lines(2),
    'Mod-5': lines(1.5),
    'Mod-z': chainCommands(undoInputRule, undo),
    'Mod-y': redo,
    'Shift-Mod-z': redo,
    'Mod->': size(1),
    'Shift-Mod->': size(1),
    'Shift-Mod-.': size(1),
    'Mod-<': size(-1),
    'Shift-Mod-<': size(-1),
    'Shift-Mod-,': size(-1),
    'Mod-]': nudge(1),
    'Mod-[': nudge(-1),
    'Mod-=': toggleMark('subscript'),
    'Mod-+': toggleMark('superscript'),
    'Shift-Mod-+': toggleMark('superscript'),
    'Shift-Mod-=': toggleMark('superscript'),
    'Shift-Mod-k': toggleMark('smallcaps'),
    'Mod-Space': clearMarks,
    'Shift-F3': changeCase,
    'Shift-Enter': insertHardBreak,
    Enter: chainCommands(outdentEmptyListItem(li), splitListItem(li), createParagraphNear, liftEmptyBlock, splitParagraph),
    Tab: chainCommands(indentListItem(li), insertTab),
    'Shift-Tab': liftListItem(li),
    'Mod-a': selectAll,
  }
}

export function editorPlugins({ getStyles }) {
  return [
    inputRules({ rules: typingRules }),
    keymap(wordKeymap(getStyles)),
    keymap(baseKeymap),
    history(),
    dropCursor({ color: 'var(--accent, #c90)' }),
    gapCursor(),
  ]
}
