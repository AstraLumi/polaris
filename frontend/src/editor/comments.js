// Comments on a chapter's text (stage 5; docs/EDITOR.md, "Comments"): the
// pure part. A comment is a row on the server (id, parent_id, text,
// resolved…); the text it is about carries a `comment` mark with that id.
// These functions read the marks of a ProseMirror doc and build the side
// panel's list; they import nothing from ProseMirror, so node tests run them
// on docs made with the schema.

export const COMMENT_MAX = 10000

const isComment = (m) => m.type.name === 'comment'

// Where each comment is: Map id → { from, to, text } (from the first text
// it marks to the end of its last; text is what it covers, blocks joined by
// a space).
export function commentRanges(doc) {
  const out = new Map()
  doc.descendants((node, pos, parent) => {
    if (!node.isText) return true
    for (const m of node.marks) {
      if (!isComment(m)) continue
      const id = +m.attrs.id
      if (!(id > 0)) continue
      const end = pos + node.nodeSize
      const r = out.get(id)
      if (!r) out.set(id, { from: pos, to: end, text: node.text, block: parent })
      else {
        r.text += (r.block !== parent ? ' ' : '') + node.text
        r.to = Math.max(r.to, end)
        r.block = parent
      }
    }
    return false
  })
  for (const r of out.values()) delete r.block
  return out
}

// A short quote of what a comment covers (one line, at most max characters).
export function quoteOf(text, max = 120) {
  const s = String(text || '').replace(/\s+/g, ' ').trim()
  return s.length > max ? s.slice(0, max - 1).trimEnd() + '…' : s
}

// The ids of the comments around a position or selection, innermost
// (shortest) first. `ids` limits them to those (e.g. the open ones).
export function commentsAt(ranges, from, to = from, ids = null) {
  const out = []
  for (const [id, r] of ranges) {
    if (ids && !ids.has(id)) continue
    const inside = from === to ? r.from <= from && from <= r.to : r.from <= from && to <= r.to
    if (inside) out.push([id, r.to - r.from])
  }
  return out.sort((a, b) => a[1] - b[1] || a[0] - b[0]).map(([id]) => id)
}

// The comment a selection points at: one whose range is exactly the
// selection (a comment picked in the panel), else the innermost around it.
export function activeAt(ranges, from, to, ids = null) {
  for (const [id, r] of ranges) if ((!ids || ids.has(id)) && r.from === from && r.to === to) return id
  return commentsAt(ranges, from, to, ids)[0] ?? null
}

const wordChar = (c) => /[\p{L}\p{N}'’\-]/u.test(c)

// The word around a position, as { from, to } (null when there is none):
// what a comment covers when nothing is selected, as in Word.
export function wordRange(doc, pos) {
  const $pos = doc.resolve(pos)
  const block = $pos.parent
  if (!block.isTextblock) return null
  const start = $pos.start()
  const text = block.textBetween(0, block.content.size, '\n', '\n')
  if (text.length !== block.content.size) {
    // Inline atoms count one position each: fall back to walking positions.
    return wordRangeSlow(doc, pos, start, block)
  }
  let a = pos - start
  let b = a
  while (a > 0 && wordChar(text[a - 1])) a--
  while (b < text.length && wordChar(text[b])) b++
  if (a === b) {
    // Not in a word: the one just before, else just after.
    let c = pos - start
    while (c > 0 && !wordChar(text[c - 1]) && text[c - 1] !== '\n') c--
    if (c > 0 && wordChar(text[c - 1])) return wordRange(doc, start + c)
    c = pos - start
    while (c < text.length && !wordChar(text[c]) && text[c] !== '\n') c++
    if (c < text.length && wordChar(text[c])) return wordRange(doc, start + c + 1)
    return null
  }
  return { from: start + a, to: start + b }
}

function wordRangeSlow(doc, pos, start, block) {
  const charAt = (p) => (p >= start && p < start + block.content.size ? doc.textBetween(p, p + 1, '\n', '\n') : '')
  let a = pos
  let b = pos
  while (a > start && wordChar(charAt(a - 1))) a--
  while (b < start + block.content.size && wordChar(charAt(b))) b++
  return a === b ? null : { from: a, to: b }
}

// Whether a range holds any text (a comment needs some).
export function hasText(doc, from, to) {
  let found = false
  doc.nodesBetween(from, to, (node) => {
    if (found) return false
    if (node.isText) found = true
    return !found
  })
  return found
}

// The side panel: the threads whose text is in the doc, in the order of
// the text (then by id), each with its replies by id. Resolved threads only
// with showResolved. `detached` counts the threads whose text is gone (kept
// for an undo or a restore, not listed).
export function panelThreads(comments, ranges, { showResolved = false } = {}) {
  const roots = []
  const replies = new Map()
  for (const c of comments || []) {
    if (c.parent_id) {
      if (!replies.has(c.parent_id)) replies.set(c.parent_id, [])
      replies.get(c.parent_id).push(c)
    } else roots.push(c)
  }
  let detached = 0
  let resolved = 0
  const threads = []
  for (const c of roots) {
    const r = ranges.get(c.id)
    if (!r) {
      detached++
      continue
    }
    if (c.resolved) {
      resolved++
      if (!showResolved) continue
    }
    threads.push({ comment: c, replies: (replies.get(c.id) || []).sort((a, b) => a.id - b.id), from: r.from, to: r.to, quote: quoteOf(r.text) })
  }
  threads.sort((a, b) => a.from - b.from || a.to - b.to || a.comment.id - b.comment.id)
  return { threads, detached, resolved }
}

// The ids whose highlight shows: open comments (not resolved) the panel
// knows. Marks with any other id draw as plain text.
export function shownIds(comments) {
  return new Set((comments || []).filter((c) => !c.parent_id && !c.resolved).map((c) => c.id))
}

// CSS for the highlights: every shown comment softly, the active one more
// strongly. scope is the editor's selector.
export function highlightCSS(ids, activeId, scope) {
  const list = [...ids].filter((id) => Number.isInteger(id) && id > 0)
  if (!list.length) return ''
  const sel = (id) => `${scope} span.pm-comment[data-comment="${id}"]`
  let css = `${list.map(sel).join(',\n')} { background: var(--comment); border-bottom: 1px solid var(--comment-line); }\n`
  if (activeId != null && ids.has(activeId)) css += `${sel(activeId)} { background: var(--comment-active); border-bottom-color: var(--comment-line-active); }\n`
  return css
}

// For a paste: the comment ids a pasted piece may keep. A comment the
// chapter doesn't know (from another chapter) or one still in the text (a
// copy, not a move) is left out, so one comment never covers two places.
export function keepOnPaste(id, known, inDoc) {
  return known.has(id) && !inDoc.has(id)
}
