// Find and replace over a document: match case, whole word, replace one or
// all. Matches stay inside one paragraph. Pure (works on ProseMirror nodes
// and transactions, no view).

const escape = (s) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')

// The regular expression for a search, or null for an empty one.
export function searchRegExp(query, { matchCase = false, wholeWord = false } = {}) {
  if (!query) return null
  let src = escape(query)
  if (wholeWord) src = `(?<![\\p{L}\\p{N}_])${src}(?![\\p{L}\\p{N}_])`
  return new RegExp(src, `gu${matchCase ? '' : 'i'}`)
}

// [start, end] of every match in a string.
export function findInText(text, query, opts) {
  const re = searchRegExp(query, opts)
  if (!re) return []
  const out = []
  for (const m of text.matchAll(re)) if (m[0].length) out.push([m.index, m.index + m[0].length])
  return out
}

// Every match in a document, as { from, to } positions, in order. Inline
// leaves (line breaks, tabs) count as one character that matches nothing.
export function findMatches(doc, query, opts) {
  if (!query) return []
  const out = []
  doc.descendants((node, pos) => {
    if (!node.isTextblock) return true
    let text = ''
    const starts = [] // text offset → document position
    node.forEach((child, offset) => {
      const at = pos + 1 + offset
      if (child.isText) {
        for (let i = 0; i < child.text.length; i++) starts.push(at + i)
        text += child.text
      } else {
        starts.push(at)
        text += '￼'
      }
    })
    for (const [a, b] of findInText(text, query, opts)) {
      out.push({ from: starts[a], to: starts[b - 1] + 1 })
    }
    return false
  })
  return out
}

// Replaces the given matches (from findMatches) in one transaction, last
// first so earlier positions stay right. The new text takes the marks of
// the text it replaces.
export function replaceMatches(tr, matches, replacement) {
  for (const m of [...matches].sort((a, b) => b.from - a.from)) {
    const marks = tr.doc.resolve(m.from + 1).marks()
    if (replacement) tr.replaceWith(m.from, m.to, tr.doc.type.schema.text(replacement, marks))
    else tr.delete(m.from, m.to)
  }
  return tr
}

// The match at or after a position (wrapping round), for "find next".
export function nextMatchIndex(matches, pos, backwards = false) {
  if (!matches.length) return -1
  if (backwards) {
    for (let i = matches.length - 1; i >= 0; i--) if (matches[i].to <= pos) return i
    return matches.length - 1
  }
  const i = matches.findIndex((m) => m.from >= pos)
  return i < 0 ? 0 : i
}
