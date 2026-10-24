// Word counts, by the rule in docs/EDITOR.md (the server counts the same
// way): the text of every block, split on whitespace, counting the runs that
// hold a letter or digit. A line break or tab inside a paragraph separates
// words like a space. Pure.

// Letters and decimal digits, as Go's unicode.IsLetter / IsDigit (pmdoc.go).
const WORDISH = /[\p{L}\p{Nd}]/u

export function countText(text) {
  let n = 0
  for (const run of String(text || '').split(/\s+/)) if (run && WORDISH.test(run)) n++
  return n
}

// Each textblock's text, from stored JSON.
function blockTexts(json, out) {
  if (!json || typeof json !== 'object') return out
  if (Array.isArray(json.content)) {
    const inline = json.content.some((c) => c && (c.type === 'text' || c.type === 'hard_break' || c.type === 'tab'))
    if (inline) out.push(json.content.map((c) => (c.type === 'text' ? c.text || '' : ' ')).join(''))
    else for (const c of json.content) blockTexts(c, out)
  }
  return out
}

export const countDocJSON = (json) => blockTexts(json, []).reduce((n, t) => n + countText(t), 0)

// The same for a ProseMirror node (the whole document) or a range of it.
export function countNode(doc, from = 0, to = doc.content.size) {
  return countText(doc.textBetween(from, to, '\n', ' '))
}
