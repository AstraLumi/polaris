// Text changes behind the Markdown toolbar (components/MarkdownEditor.vue).
// Pure functions, no dependencies: node frontend/tests/markdownEdit.test.mjs
//
// Every edit takes the whole text and the selection [from, to) and returns
// { from, to, insert, selStart, selEnd }: replace text[from, to) with
// `insert`, then select [selStart, selEnd) in the new text.

const lineStart = (text, i) => text.lastIndexOf('\n', i - 1) + 1
function lineEnd(text, i) {
  const n = text.indexOf('\n', i)
  return n < 0 ? text.length : n
}

// **bold**, *italic*, ~~strike~~, `code`. Clicking again on wrapped text
// unwraps it. Spaces at the edges of the selection stay outside the markers.
export function wrapInline(text, from, to, marker, placeholder) {
  const sel = text.slice(from, to)
  const lead = sel.length - sel.trimStart().length
  const trail = sel.length - sel.trimEnd().length
  const a = from + lead
  const b = Math.max(a, to - trail)
  const inner = text.slice(a, b)
  const m = marker.length

  if (text.slice(a - m, a) === marker && text.slice(b, b + m) === marker) {
    return { from: a - m, to: b + m, insert: inner, selStart: a - m, selEnd: b - m }
  }
  if (inner.length > 2 * m && inner.startsWith(marker) && inner.endsWith(marker)) {
    const bare = inner.slice(m, -m)
    return { from: a, to: b, insert: bare, selStart: a, selEnd: a + bare.length }
  }
  const body = inner || placeholder
  return { from: a, to: b, insert: marker + body + marker, selStart: a + m, selEnd: a + m + body.length }
}

const PREFIX_RE = {
  heading: /^#{1,6}\s+/,
  bullet: /^\s*[-*+]\s+/,
  numbered: /^\s*\d+[.)]\s+/,
  quote: /^\s*>\s?/,
}

// Headings, lists and quotes work on whole lines. If every non-empty line
// already has the prefix it is taken off again.
export function toggleLines(text, from, to, kind) {
  const a = lineStart(text, from)
  // A selection that ends right after a line break doesn't include the next line.
  const b = lineEnd(text, to > from && text[to - 1] === '\n' ? to - 1 : to)
  const lines = text.slice(a, b).split('\n')
  const re = PREFIX_RE[kind]
  const used = lines.filter((l) => l.trim() !== '')
  const remove = used.length > 0 && used.every((l) => re.test(l))
  let n = 0
  const out = lines.map((l) => {
    if (remove) return l.replace(re, '')
    if (l.trim() === '' && lines.length > 1) return l
    const bare = l.replace(PREFIX_RE.heading, '').replace(PREFIX_RE.bullet, '').replace(PREFIX_RE.numbered, '')
    if (kind === 'heading') return `# ${bare}`
    if (kind === 'bullet') return `- ${bare}`
    if (kind === 'numbered') return `${++n}. ${bare}`
    return `> ${l}`
  })
  const insert = out.join('\n')
  // One empty line: put the cursor after the new prefix.
  if (lines.length === 1 && lines[0] === '') return { from: a, to: b, insert, selStart: a + insert.length, selEnd: a + insert.length }
  return { from: a, to: b, insert, selStart: a, selEnd: a + insert.length }
}

// A block (code, table, rule) sits on its own lines with a blank line around
// it, or the renderer would read it as part of the paragraph next to it.
function padBlock(text, from, to, block) {
  const before = text.slice(0, from)
  const after = text.slice(to)
  const pre = before === '' || before.endsWith('\n\n') ? '' : before.endsWith('\n') ? '\n' : '\n\n'
  const post = after === '' || after.startsWith('\n\n') ? '' : after.startsWith('\n') ? '\n' : '\n\n'
  return { insert: pre + block + post, offset: from + pre.length }
}

export function codeBlock(text, from, to, placeholder) {
  const body = text.slice(from, to).replace(/^\n+|\n+$/g, '') || placeholder
  const { insert, offset } = padBlock(text, from, to, '```\n' + body + '\n```')
  return { from, to, insert, selStart: offset + 4, selEnd: offset + 4 + body.length }
}

export function table(text, from, to, column, cell) {
  const block = `| ${column} | ${column} |\n| --- | --- |\n| ${cell} | ${cell} |`
  const { insert, offset } = padBlock(text, from, to, block)
  return { from, to, insert, selStart: offset + 2, selEnd: offset + 2 + column.length }
}

export function rule(text, from, to) {
  const { insert, offset } = padBlock(text, from, to, '---')
  return { from, to, insert, selStart: offset + insert.trimEnd().length, selEnd: offset + insert.trimEnd().length }
}

// [text](https://…) with the address selected, ready to paste over.
export function webLink(text, from, to, placeholder) {
  const shown = text.slice(from, to).replace(/[[\]]/g, ' ').replace(/\s+/g, ' ').trim() || placeholder
  const url = 'https://'
  const insert = `[${shown}](${url})`
  const at = from + shown.length + 3
  return { from, to, insert, selStart: at, selEnd: at + url.length }
}

// ---- links to other articles ----

const fold = (s) =>
  String(s)
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .replace(/_/g, ' ')
    .toLowerCase()
    .split(/\s+/)
    .filter(Boolean)
    .join(' ')

// Wiki articles whose name matches what was typed: exact, then starts with,
// then a word that starts with, then anywhere. Accents and case don't count.
export function searchArticles(items, query, limit = 8) {
  const q = fold(query)
  if (!q) return []
  const scored = []
  for (const it of items) {
    const n = fold(it.name)
    const score = n === q ? 0 : n.startsWith(q) ? 1 : n.includes(` ${q}`) ? 2 : n.includes(q) ? 3 : -1
    if (score >= 0) scored.push({ it, score })
  }
  scored.sort((x, y) => x.score - y.score || x.it.name.localeCompare(y.it.name))
  return scored.slice(0, limit).map((s) => s.it)
}

// The [[…]] text that leads to `item`: [[Name]] when that name alone finds
// it, [[type:Name]] when another kind of article has the same name.
// `resolve` is makeLinkResolver() from wiki.js.
export function wikiLinkMarkup(item, text, resolve) {
  const hit = resolve ? resolve(item.name) : null
  const target = hit && hit.type === item.type && hit.id === item.id ? item.name : `${item.type}:${item.name}`
  const shown = String(text || '').replace(/[[\]]/g, ' ').replace(/\s+/g, ' ').trim()
  return shown && fold(shown) !== fold(item.name) ? `[[${target}|${shown}]]` : `[[${target}]]`
}
