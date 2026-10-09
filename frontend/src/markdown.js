// A small, safe Markdown renderer for the wiki. Everything the user types is
// HTML-escaped first, so the output can go straight into v-html: no script,
// no raw HTML, and only http(s), mailto, relative and #anchor links.
//
// Supported: # headings (shown as h3-h6, the article's own sections are h2),
// paragraphs (a single line break stays a line break), **bold**, *italic*,
// ~~strike~~, `code`, fenced code, > quotes, - / 1. lists (nested by two
// spaces), --- rules, pipe tables and [text](url) links. [[Article]],
// [[Article|text]] and [[type:Article]] link to other wiki articles; the
// `resolve` callback turns the text inside into { type, id, name } or null.

const escapeHtml = (s) =>
  s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')

const unescapeHtml = (s) =>
  s.replace(/&quot;/g, '"').replace(/&gt;/g, '>').replace(/&lt;/g, '<').replace(/&amp;/g, '&')

function safeUrl(url) {
  const u = unescapeHtml(url).trim()
  if (/^(https?:|mailto:)/i.test(u) || /^[/#]/.test(u)) return u
  return ''
}

const attr = (s) => escapeHtml(s)

function inline(raw, resolve) {
  const stash = []
  const keep = (html) => `\u0000${stash.push(html) - 1}\u0000`
  let s = escapeHtml(raw)

  s = s.replace(/`([^`\n]+)`/g, (m, code) => keep(`<code>${code}</code>`))

  s = s.replace(/\[\[([^\[\]|\n]+?)(?:\|([^\[\]\n]*))?\]\]/g, (m, target, text) => {
    const inner = unescapeHtml(target)
    const hit = resolve ? resolve(inner) : null
    const norm = (v) => v.toLowerCase().replace(/\s+/g, ' ').trim()
    const shown =
      text !== undefined && text.trim() !== ''
        ? text
        : hit && inner.includes(':') && norm(inner) !== norm(hit.name)
          ? escapeHtml(hit.name)
          : target
    if (hit) return keep(`<a class="wikilink" href="/wiki/${hit.type}/${hit.id}" data-wiki="${hit.type}/${hit.id}">${shown}</a>`)
    return keep(`<span class="wikilink is-missing" title="${attr(inner)}">${shown}</span>`)
  })

  s = s.replace(/\[([^\]\n]+)\]\(([^)\s]+)\)/g, (m, text, url) => {
    const href = safeUrl(url)
    if (!href) return m
    const external = /^https?:/i.test(href)
    return keep(`<a href="${attr(href)}"${external ? ' target="_blank" rel="noopener noreferrer"' : ''}>${text}</a>`)
  })

  s = s
    .replace(/\*\*([^*\n]+?)\*\*/g, '<strong>$1</strong>')
    .replace(/(^|[^*\w])\*([^*\n]+?)\*(?!\w)/g, '$1<em>$2</em>')
    .replace(/(^|[^\w])_([^_\n]+?)_(?!\w)/g, '$1<em>$2</em>')
    .replace(/~~([^~\n]+?)~~/g, '<del>$1</del>')

  return s.replace(/\u0000(\d+)\u0000/g, (m, i) => stash[Number(i)])
}

const LIST_RE = /^(\s*)([-*+]|\d+[.)])\s+(.*)$/
const TABLE_SEP_RE = /^\s*\|?\s*:?-{2,}:?\s*(\|\s*:?-{2,}:?\s*)*\|?\s*$/

function splitRow(line) {
  let t = line.trim()
  if (t.startsWith('|')) t = t.slice(1)
  if (t.endsWith('|')) t = t.slice(0, -1)
  return t.split('|').map((c) => c.trim())
}

function renderList(lines, i, resolve) {
  // Collects consecutive list lines; indentation (2 spaces) nests.
  const items = []
  while (i < lines.length) {
    const m = LIST_RE.exec(lines[i])
    if (!m) break
    items.push({ depth: Math.floor(m[1].replace(/\t/g, '  ').length / 2), ordered: /\d/.test(m[2]), text: m[3] })
    i++
  }
  let html = ''
  const stack = [] // open lists: { ordered, depth }
  const close = () => {
    const top = stack.pop()
    html += `</li></${top.ordered ? 'ol' : 'ul'}>`
  }
  for (const it of items) {
    const depth = Math.min(it.depth, stack.length) // never skip a level
    while (stack.length > depth + 1) close()
    if (stack.length === depth + 1) {
      html += '</li>'
    } else {
      stack.push({ ordered: it.ordered })
      html += it.ordered ? '<ol>' : '<ul>'
    }
    html += `<li>${inline(it.text, resolve)}`
  }
  while (stack.length) close()
  return { html, next: i }
}

export function renderMarkdown(text, resolve) {
  const lines = String(text || '').replace(/\r\n?/g, '\n').split('\n')
  const out = []
  let i = 0
  while (i < lines.length) {
    const line = lines[i]
    if (line.trim() === '') {
      i++
      continue
    }

    const fence = /^\s*```/.exec(line)
    if (fence) {
      const code = []
      i++
      while (i < lines.length && !/^\s*```/.test(lines[i])) code.push(lines[i++])
      i++
      out.push(`<pre><code>${escapeHtml(code.join('\n'))}</code></pre>`)
      continue
    }

    const h = /^(#{1,6})\s+(.*?)\s*#*\s*$/.exec(line)
    if (h) {
      const level = Math.min(h[1].length + 2, 6)
      out.push(`<h${level}>${inline(h[2], resolve)}</h${level}>`)
      i++
      continue
    }

    if (/^\s*([-*_])(\s*\1){2,}\s*$/.test(line)) {
      out.push('<hr />')
      i++
      continue
    }

    if (/^\s*>/.test(line)) {
      const quote = []
      while (i < lines.length && /^\s*>/.test(lines[i])) quote.push(lines[i++].replace(/^\s*>\s?/, ''))
      out.push(`<blockquote>${renderMarkdown(quote.join('\n'), resolve)}</blockquote>`)
      continue
    }

    if (LIST_RE.test(line)) {
      const r = renderList(lines, i, resolve)
      out.push(r.html)
      i = r.next
      continue
    }

    if (line.includes('|') && i + 1 < lines.length && TABLE_SEP_RE.test(lines[i + 1]) && lines[i + 1].includes('-')) {
      const head = splitRow(line)
      const aligns = splitRow(lines[i + 1]).map((c) => (/^:-+:$/.test(c) ? 'center' : /-:$/.test(c) ? 'right' : ''))
      i += 2
      const rows = []
      while (i < lines.length && lines[i].trim() !== '' && lines[i].includes('|')) rows.push(splitRow(lines[i++]))
      const cell = (tag, c, n) =>
        `<${tag}${aligns[n] ? ` style="text-align:${aligns[n]}"` : ''}>${inline(c, resolve)}</${tag}>`
      out.push(
        `<table><thead><tr>${head.map((c, n) => cell('th', c, n)).join('')}</tr></thead><tbody>` +
          rows.map((r) => `<tr>${head.map((_, n) => cell('td', r[n] || '', n)).join('')}</tr>`).join('') +
          '</tbody></table>',
      )
      continue
    }

    const para = []
    while (
      i < lines.length &&
      lines[i].trim() !== '' &&
      !/^\s*(```|#{1,6}\s|>)/.test(lines[i]) &&
      !LIST_RE.test(lines[i]) &&
      !/^\s*([-*_])(\s*\1){2,}\s*$/.test(lines[i])
    ) {
      para.push(lines[i++])
    }
    out.push(`<p>${para.map((l) => inline(l, resolve)).join('<br />')}</p>`)
  }
  return out.join('\n')
}
