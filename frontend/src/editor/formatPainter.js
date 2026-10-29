// Format Painter (docs/EDITOR.md): copy what a stretch of text looks like
// (its formatting marks and its paragraph's formatting) and paint it onto
// the next selection. Plain functions on an EditorState, no drawing.
import { schema, PARA_ATTRS } from './schema.js'

// The marks that are looks. Links, story links, comments and speaker tags
// say something about the text, so they are never copied nor removed.
export const FORMAT_MARKS = ['char_style', 'font', 'size', 'color', 'highlight', 'bold', 'italic', 'underline', 'strike', 'superscript', 'subscript', 'smallcaps']

const isBlock = (node) => node.type === schema.nodes.paragraph || node.type === schema.nodes.heading

// What the selection's start looks like: { marks, para, level, whole }.
// whole: the copied text was a whole paragraph (so painting also paints
// paragraph formatting, as Word does). null when there is nothing to copy.
export function copyFormat(state) {
  const { selection } = state
  const { $from, $to, empty } = selection
  if (!$from.parent.isTextblock) return null
  const block = $from.parent
  let marks
  if (empty) marks = state.storedMarks || $from.marks()
  else marks = ($from.nodeAfter && $from.nodeAfter.marks) || $from.marks()
  const para = {}
  for (const k of PARA_ATTRS) para[k] = block.attrs[k]
  const whole = !empty && $from.sameParent($to) && $from.parentOffset === 0 && $to.parentOffset === block.content.size
  return {
    marks: marks.filter((m) => FORMAT_MARKS.includes(m.type.name)).map((m) => ({ type: m.type.name, attrs: { ...m.attrs } })),
    para,
    level: block.type === schema.nodes.heading ? block.attrs.level : null,
    whole,
  }
}

// Paints the format on the selection: a transaction, or null when there is
// nothing to paint. A click (an empty selection) paints the paragraph only.
// A selection paints the text's marks (the old looks go, links and the like
// stay) and the paragraph formatting of the paragraphs it covers whole, or
// of all it touches when the copied text was a whole paragraph.
export function applyFormat(state, fmt) {
  if (!fmt) return null
  const { selection } = state
  if (!selection.$from.parent.isTextblock) return null
  const tr = state.tr
  const { from, to, empty } = selection
  const blocks = []
  if (empty) blocks.push({ node: selection.$from.parent, pos: selection.$from.before(), cover: true })
  else {
    state.doc.nodesBetween(from, to, (node, pos) => {
      if (!node.isTextblock) return true
      blocks.push({ node, pos, cover: from <= pos + 1 && to >= pos + 1 + node.content.size })
      return false
    })
    for (const name of FORMAT_MARKS) tr.removeMark(from, to, schema.marks[name])
    for (const m of fmt.marks) tr.addMark(from, to, schema.marks[m.type].create(m.attrs))
  }
  for (const { node, pos, cover } of blocks) {
    if (!isBlock(node) || !(cover || fmt.whole)) continue
    const type = fmt.level ? schema.nodes.heading : schema.nodes.paragraph
    const attrs = { ...node.attrs, ...fmt.para }
    if (fmt.level) attrs.level = fmt.level
    if (type === node.type && Object.keys(attrs).every((k) => attrs[k] === node.attrs[k])) continue
    tr.setNodeMarkup(pos, type, attrs)
  }
  return tr.docChanged ? tr : null
}
