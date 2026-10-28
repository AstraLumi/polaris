// Word counts, by the rule in docs/EDITOR.md (the server counts the same
// way): the text of every block, split on whitespace, counting the runs that
// hold a letter or digit. A line break or tab inside a paragraph separates
// words like a space. A note (footnote or endnote) counts like the text
// around it, its paragraphs on their own; its number separates words like
// a space. Pure.

// Letters and decimal digits, as Go's unicode.IsLetter / IsDigit (pmdoc.go).
const WORDISH = /[\p{L}\p{Nd}]/u

export function countText(text) {
  let n = 0
  for (const run of String(text || '').split(/\s+/)) if (run && WORDISH.test(run)) n++
  return n
}

const INLINE = new Set(['text', 'hard_break', 'tab', 'footnote'])

// Each textblock's text, from stored JSON (a note's paragraphs after the
// block it sits in).
function blockTexts(json, out) {
  if (!json || typeof json !== 'object') return out
  if (Array.isArray(json.content)) {
    const inline = json.content.some((c) => c && INLINE.has(c.type))
    if (inline) {
      out.push(json.content.map((c) => (c?.type === 'text' ? c.text || '' : ' ')).join(''))
      for (const c of json.content) if (c?.type === 'footnote') blockTexts(c, out)
    } else for (const c of json.content) blockTexts(c, out)
  }
  return out
}

export const countDocJSON = (json) => blockTexts(json, []).reduce((n, t) => n + countText(t), 0)

// The same for a ProseMirror node (the whole document) or a range of it.
export function countNode(doc, from = 0, to = doc.content.size) {
  let n = 0
  doc.nodesBetween(from, to, (node, pos) => {
    if (!node.isTextblock) return true
    let text = ''
    node.forEach((child, off) => {
      const a = pos + 1 + off
      if (a + child.nodeSize <= from || a >= to) return
      text += child.isText ? child.text.slice(Math.max(0, from - a), Math.max(0, to - a)) : ' '
    })
    n += countText(text)
    return true // on into its notes
  })
  return n
}
