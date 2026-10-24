// The editor's commands: what the toolbar buttons and Word's shortcuts do.
// Each is a ProseMirror command, (state, dispatch) => bool.
import { TextSelection, NodeSelection } from 'prosemirror-state'
import { splitBlockAs, toggleMark as pmToggleMark } from 'prosemirror-commands'
import { wrapInList, liftListItem } from 'prosemirror-schema-list'
import { schema, PARA_ATTRS } from './schema.js'
import { resolveBlock, blockStyleId } from './styles.js'
import { nextCase, toCase } from './textCase.js'

const { nodes, marks } = schema

// Word's font size list (Ctrl+Shift+> / < step through it).
export const FONT_SIZES = [6, 7, 8, 9, 10, 10.5, 11, 12, 14, 16, 18, 20, 22, 24, 26, 28, 36, 48, 72]
export const LINE_HEIGHTS = [1, 1.15, 1.5, 2, 2.5, 3]

export const toggleMark = (name) => pmToggleMark(marks[name])

// The marks that apply at the selection: the stored ones, those before
// the cursor, or those of the selected text's start.
export function marksAtSelection(state) {
  const { $from, empty } = state.selection
  if (empty) return state.storedMarks || $from.marks()
  const after = $from.nodeAfter
  return after?.isText ? after.marks : $from.marks()
}

export function markActive(state, name) {
  const type = marks[name]
  const { from, to, empty } = state.selection
  if (empty) return !!type.isInSet(marksAtSelection(state))
  let all = true
  let any = false
  state.doc.nodesBetween(from, to, (node) => {
    if (!node.isText) return true
    if (type.isInSet(node.marks)) any = true
    else all = false
    return false
  })
  return any && all
}

export const markAttrs = (state, name) => marks[name].isInSet(marksAtSelection(state))?.attrs || null

// Puts a mark with these attrs on the selection (replacing one of the same
// kind), or takes it off when attrs is null. With nothing selected it
// applies to what is typed next.
export const setMark = (name, attrs) => (state, dispatch) => {
  const type = marks[name]
  const { empty, ranges } = state.selection
  if (!dispatch) return true
  const tr = state.tr
  if (empty) {
    const current = marksAtSelection(state)
    const next = attrs ? type.create(attrs).addToSet(current) : type.removeFromSet(current)
    tr.setStoredMarks(next)
  } else {
    for (const { $from, $to } of ranges) {
      tr.removeMark($from.pos, $to.pos, type)
      if (attrs) tr.addMark($from.pos, $to.pos, type.create(attrs))
    }
  }
  dispatch(tr.scrollIntoView())
  return true
}

// Every paragraph and heading the selection touches, with its position.
export function selectedBlocks(state) {
  const out = []
  const { from, to } = state.selection
  state.doc.nodesBetween(from, to, (node, pos) => {
    if (node.isTextblock) {
      out.push({ node, pos })
      return false
    }
    return true
  })
  if (!out.length && state.selection.$from.parent.isTextblock) {
    const $f = state.selection.$from
    out.push({ node: $f.parent, pos: $f.before() })
  }
  return out
}

// Sets paragraph attrs (align, indents, spacing, line height) on every
// selected paragraph; a null value goes back to the style's.
export const setParaAttrs = (patch) => (state, dispatch) => {
  const blocks = selectedBlocks(state)
  if (!blocks.length) return false
  if (dispatch) {
    const tr = state.tr
    for (const { node, pos } of blocks) tr.setNodeMarkup(pos, null, { ...node.attrs, ...patch })
    dispatch(tr.scrollIntoView())
  }
  return true
}

const cleared = (attrs) => {
  const out = { ...attrs }
  for (const k of PARA_ATTRS) out[k] = k === 'keepNext' ? false : null
  return out
}

// Applies a paragraph style. The built-in Heading 1–3 make headings of
// that level; anything else a paragraph. The paragraph's own formatting
// goes, so it looks like the style (its text formatting stays).
export const setStyle = (id) => (state, dispatch) => {
  const blocks = selectedBlocks(state)
  if (!blocks.length) return false
  if (dispatch) {
    const tr = state.tr
    const m = /^heading([1-3])$/.exec(id)
    for (const { node, pos } of blocks) {
      const attrs = cleared(node.attrs)
      if (m) tr.setNodeMarkup(pos, nodes.heading, { ...attrs, level: +m[1], style: null })
      else {
        const { level, ...rest } = attrs
        void level
        tr.setNodeMarkup(pos, nodes.paragraph, { ...rest, style: id === 'normal' ? null : id })
      }
    }
    dispatch(tr.scrollIntoView())
  }
  return true
}

// What the selection looks like, for the toolbar: the style, the paragraph
// formatting and the text formatting, all worked out from the styles.
export function selectionFormat(state, styles) {
  const { $from } = state.selection
  const block = $from.parent.isTextblock ? $from.parent : null
  const sel = state.selection
  const node = sel instanceof NodeSelection ? sel.node : null
  const ms = marksAtSelection(state)
  const r = block ? resolveBlock(styles, block.type.name, block.attrs, ms) : resolveBlock(styles, 'paragraph', {}, ms)
  let list = null
  for (let d = $from.depth; d > 0; d--) {
    const n = $from.node(d).type.name
    if (n === 'bullet_list' || n === 'ordered_list') {
      list = n
      break
    }
  }
  const has = (name) => markActive(state, name)
  return {
    styleId: block ? blockStyleId(block.type.name, block.attrs, styles) : null,
    ...r.para,
    ...r.text,
    bold: has('bold') || !!r.text.bold,
    italic: has('italic') || !!r.text.italic,
    underline: has('underline'),
    strike: has('strike'),
    superscript: has('superscript'),
    subscript: has('subscript'),
    smallcaps: has('smallcaps'),
    highlight: markAttrs(state, 'highlight')?.color || null,
    link: markAttrs(state, 'link')?.href || null,
    list,
    atom: node?.type.name || null,
  }
}

// One step up or down Word's size list from the size at the selection.
export const stepFontSize = (dir, styles) => (state, dispatch) => {
  const current = selectionFormat(state, styles).size || 12
  const next = dir > 0 ? FONT_SIZES.find((s) => s > current) ?? current + 12 : [...FONT_SIZES].reverse().find((s) => s < current) ?? Math.max(1, current - 1)
  return setMark('size', { pt: next })(state, dispatch)
}

// By one point (Ctrl+] / Ctrl+[).
export const nudgeFontSize = (by, styles) => (state, dispatch) => {
  const current = selectionFormat(state, styles).size || 12
  return setMark('size', { pt: Math.max(1, Math.min(400, current + by)) })(state, dispatch)
}

// A block (scene break, page break, picture) at the cursor: the paragraph
// is split there, and the cursor goes to the text after it.
export const insertBlock = (typeName, attrs = {}) => (state, dispatch) => {
  const type = nodes[typeName]
  if (!dispatch) return true
  const tr = state.tr
  if (!state.selection.empty) tr.deleteSelection()
  const $pos = tr.selection.$from
  let at
  if (!$pos.parent.isTextblock) at = $pos.pos
  else if ($pos.depth > 1) at = $pos.after(1) // inside a list: after it
  else if ($pos.parentOffset === 0 && $pos.parent.content.size) at = $pos.before()
  else if ($pos.parentOffset === $pos.parent.content.size) at = $pos.after()
  else {
    tr.split($pos.pos)
    at = $pos.pos + 1
  }
  const node = type.create(attrs)
  tr.insert(at, node)
  let after = at + node.nodeSize
  const next = tr.doc.resolve(after).nodeAfter
  if (!next || !next.isTextblock) {
    const prev = $pos.parent.isTextblock && $pos.depth === 1 ? $pos.parent : null
    tr.insert(after, nodes.paragraph.create(prev?.type === nodes.paragraph ? prev.attrs : null))
  }
  tr.setSelection(TextSelection.near(tr.doc.resolve(after + 1)))
  dispatch(tr.scrollIntoView())
  return true
}

// Bullet or numbered list on and off; switching kind changes the list.
export const toggleList = (typeName) => (state, dispatch) => {
  const type = nodes[typeName]
  const { $from } = state.selection
  for (let d = $from.depth; d > 0; d--) {
    const n = $from.node(d)
    if (n.type === nodes.bullet_list || n.type === nodes.ordered_list) {
      if (n.type === type) return liftListItem(nodes.list_item)(state, dispatch)
      if (dispatch) dispatch(state.tr.setNodeMarkup($from.before(d), type))
      return true
    }
  }
  return wrapInList(type)(state, dispatch)
}

export const insertTab = (state, dispatch) => {
  if (!state.selection.$from.parent.inlineContent) return false
  if (dispatch) dispatch(state.tr.replaceSelectionWith(nodes.tab.create(), true).scrollIntoView())
  return true
}

export const insertHardBreak = (state, dispatch) => {
  if (!state.selection.$from.parent.inlineContent) return false
  if (dispatch) dispatch(state.tr.replaceSelectionWith(nodes.hard_break.create(), true).scrollIntoView())
  return true
}

// Enter, as in Word: the new paragraph keeps the paragraph's formatting and
// the text formatting; after a heading, title or subtitle comes Normal.
export const splitParagraph = (state, dispatch) => {
  const { $from } = state.selection
  const block = $from.parent
  const atEnd = $from.parentOffset === block.content.size
  const toNormal = atEnd && (block.type === nodes.heading || ['title', 'subtitle'].includes(block.attrs.style))
  const split = splitBlockAs((node, end) => {
    if (node.type === nodes.heading) return end ? { type: nodes.paragraph } : { type: nodes.heading, attrs: node.attrs }
    if (node.type === nodes.paragraph) return toNormal ? { type: nodes.paragraph } : { type: nodes.paragraph, attrs: node.attrs }
    return null
  })
  return split(state, dispatch && ((tr) => {
    if (!toNormal) {
      const ms = state.storedMarks || ($from.parentOffset ? $from.marks() : null)
      if (ms) tr.ensureMarks(ms)
    }
    dispatch(tr)
  }))
}

// Shift+F3: the selection (or the word at the cursor) through lower,
// UPPER and Title case, keeping its formatting.
export const changeCase = (state, dispatch) => {
  let { from, to, empty, $from } = state.selection
  if (empty) {
    const text = $from.parent.textContent
    let a = $from.parentOffset
    let b = a
    while (a > 0 && /[\p{L}\p{N}'’]/u.test(text[a - 1])) a--
    while (b < text.length && /[\p{L}\p{N}'’]/u.test(text[b])) b++
    if (a === b) return false
    from = $from.start() + a
    to = $from.start() + b
  }
  const before = from > 0 ? state.doc.textBetween(Math.max(0, from - 1), from, ' ', ' ') : ''
  const mode = nextCase(state.doc.textBetween(from, to, ' ', ' '), before)
  if (!dispatch) return true
  const parts = []
  let prev = before
  state.doc.nodesBetween(from, to, (node, pos) => {
    if (!node.isText) return true
    const a = Math.max(from, pos)
    const b = Math.min(to, pos + node.nodeSize)
    const text = node.text.slice(a - pos, b - pos)
    parts.push({ a, b, text: toCase(text, mode, prev), marks: node.marks })
    prev = text.slice(-1)
    return false
  })
  const tr = state.tr
  for (const p of parts.reverse()) if (p.text) tr.replaceWith(p.a, p.b, schema.text(p.text, p.marks))
  tr.setSelection(TextSelection.create(tr.doc, from, tr.mapping.map(to)))
  dispatch(tr)
  return true
}

// Takes off text formatting (links stay) and the paragraphs' own formatting
// (their style stays).
export const clearFormatting = (state, dispatch) => {
  if (!dispatch) return true
  const tr = state.tr
  const { from, to, empty } = state.selection
  const keepMarks = new Set([marks.link, marks.story_link])
  if (empty) tr.setStoredMarks(marksAtSelection(state).filter((m) => keepMarks.has(m.type)))
  else for (const type of Object.values(marks)) if (!keepMarks.has(type)) tr.removeMark(from, to, type)
  for (const { node, pos } of selectedBlocks(state)) tr.setNodeMarkup(pos, null, { ...cleared(node.attrs), style: node.attrs.style, ...(node.type === nodes.heading ? { level: node.attrs.level } : {}) })
  dispatch(tr)
  return true
}

// Only text formatting (Ctrl+Space).
export const clearMarks = (state, dispatch) => {
  const { from, to, empty } = state.selection
  if (!dispatch) return true
  const tr = state.tr
  const keepMarks = new Set([marks.link, marks.story_link])
  if (empty) tr.setStoredMarks(marksAtSelection(state).filter((m) => keepMarks.has(m.type)))
  else for (const type of Object.values(marks)) if (!keepMarks.has(type)) tr.removeMark(from, to, type)
  dispatch(tr)
  return true
}

// A web link on the selection; with nothing selected the address itself
// is typed in as the link's text. An empty href takes the link off.
export const setLink = (href) => (state, dispatch) => {
  const { empty, from, to, $from } = state.selection
  if (!dispatch) return true
  const tr = state.tr
  if (!href) {
    if (empty) {
      // The whole link the cursor is in.
      const range = markRange($from, marks.link)
      if (range) tr.removeMark(range.from, range.to, marks.link)
    } else tr.removeMark(from, to, marks.link)
  } else if (empty) {
    const range = markRange($from, marks.link)
    if (range) {
      tr.removeMark(range.from, range.to, marks.link).addMark(range.from, range.to, marks.link.create({ href }))
    } else {
      tr.insert(from, schema.text(href, [...marksAtSelection(state).filter((m) => m.type !== marks.link), marks.link.create({ href })]))
    }
  } else tr.removeMark(from, to, marks.link).addMark(from, to, marks.link.create({ href }))
  dispatch(tr.scrollIntoView())
  return true
}

// The extent of a mark around a position in its paragraph.
export function markRange($pos, type) {
  const parent = $pos.parent
  const start = $pos.start()
  let found = null
  let cur = null
  parent.forEach((child, offset) => {
    const a = start + offset
    const b = a + child.nodeSize
    if (type.isInSet(child.marks)) {
      if (cur && cur.to === a) cur.to = b
      else cur = { from: a, to: b }
      if ($pos.pos >= cur.from && $pos.pos <= cur.to) found = { ...cur }
    } else cur = null
  })
  return found
}

// Pasting into an empty paragraph takes the pasted paragraph's formatting
// too (ProseMirror would keep the empty one's). Returns the transaction, or
// null to paste as usual.
export function pasteIntoEmpty(state, slice) {
  const { $from, empty } = state.selection
  const first = slice.content.firstChild
  if (!empty || !$from.parent.isTextblock || $from.parent.content.size || !first?.isTextblock || slice.openStart < 1) return null
  const pos = $from.before()
  const tr = state.tr.replaceSelection(slice)
  const target = tr.doc.nodeAt(tr.mapping.map(pos, -1))
  if (target?.isTextblock && first.type.validContent(target.content)) tr.setNodeMarkup(tr.mapping.map(pos, -1), first.type, first.attrs)
  return tr.scrollIntoView().setMeta('paste', true).setMeta('uiEvent', 'paste')
}
