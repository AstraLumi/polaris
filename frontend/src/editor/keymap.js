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
    Enter: chainCommands(splitListItem(li), createParagraphNear, liftEmptyBlock, splitParagraph),
    Tab: chainCommands(sinkListItem(li), insertTab),
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
