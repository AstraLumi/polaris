// Links between wiki articles, written in Markdown text as [[…]]. Pure, no
// dependencies, so node can test it (frontend/tests/markdown.test.mjs).
//
// Stored form: [[type#id]] shows the article's current name, so a rename
// carries through; [[type#id|shown text]] shows the given text.
// Typed form, read everywhere and forever: [[Name]], [[Name|text]],
// [[type:Name]] and [[type:Name|text]], found by name (and a character's
// former names).
//
// The server turns typed links into the stored form when text is saved
// (backend/wikilinks.go, normalizeWikiLinks). Text editors show stored links
// by name again (linksForEditing), so loading and saving unchanged text gives
// back the same bytes. A text shown as plain text goes through linksAsText
// (linkNames.js gives the pages the names). Both sides must read links the
// same way: the shared cases are in frontend/tests/wikiLinks.fixtures.json.

// Every kind of wiki article, in the order the wiki lists them. The same keys
// as wikiTypes in backend/wiki.go.
export const TYPE_ORDER = [
  'character', 'location', 'faction', 'event', 'class', 'subclass', 'specialization', 'race', 'body_type', 'spell', 'gear', 'creature',
  'map_feature', 'solar_system', 'celestial_body', 'lore',
]

export const WIKI_LINK_RE = /\[\[([^\[\]|\n]+?)(?:\|([^\[\]\n]*))?\]\]/g
// Spaces as Go's regexp reads \s: ASCII only.
const ID_LINK_RE = /^[\t\n\f\r ]*([a-z_]+)#([1-9][0-9]{0,17})[\t\n\f\r ]*$/
const CODE_SPAN_RE = /`[^`\n]+`/g
const CODE_FENCE_RE = /^[\t\f\r ]*(?:>[\t\f\r ]?)*[\t\f\r ]*```/

// A stored link's target ("character#12") as { type, id } (id a string), or
// null for a typed one.
export function parseIdLink(target) {
  const m = ID_LINK_RE.exec(target)
  return m && TYPE_ORDER.includes(m[1]) ? { type: m[1], id: m[2] } : null
}

// Calls fn({ full, target, text, hasText }) for every link outside code and
// puts what it returns in the link's place. Code (a `span` or a ``` fenced
// block) holds text, not links.
export function mapWikiLinks(text, fn) {
  const s = String(text ?? '')
  if (!s.includes('[[')) return s
  let inFence = false
  return s
    .split('\n')
    .map((line) => {
      if (CODE_FENCE_RE.test(line)) {
        inFence = !inFence
        return line
      }
      if (inFence || !line.includes('[[')) return line
      const gap = (part) =>
        part.replace(WIKI_LINK_RE, (full, target, shown) =>
          fn({ full, target, text: shown ?? '', hasText: shown !== undefined }),
        )
      let out = ''
      let at = 0
      for (const m of line.matchAll(CODE_SPAN_RE)) {
        out += gap(line.slice(at, m.index)) + m[0]
        at = m.index + m[0].length
      }
      return out + gap(line.slice(at))
    })
    .join('\n')
}

// The links in a text, outside code.
export function findWikiLinks(text) {
  const out = []
  mapWikiLinks(text, (m) => {
    out.push(m)
    return m.full
  })
  return out
}

const norm = (s) => s.replace(/_/g, ' ').toLowerCase().split(/\s+/).filter(Boolean).join(' ')

// A resolver over the wiki's article list (wikiApi.list()): resolve(target)
// gives the { type, id, name, … } a link's target points at, or null. Same
// rules as wikiLinkIndex.resolve in backend/wikilinks.go. Also:
// resolve.byId(type, id) and resolve.nameTarget(item), the typed target that
// leads back to item (null when no name can).
export function makeLinkResolver(items) {
  const index = new Map()
  const byId = new Map()
  const sameName = new Map() // "type\0name" -> how many articles of that kind have it
  const add = (name, it) => {
    const k = norm(name)
    if (!index.has(k)) index.set(k, [])
    index.get(k).push(it)
  }
  for (const it of items) {
    add(it.name, it)
    byId.set(`${it.type}#${it.id}`, it)
    const k = `${it.type}\0${norm(it.name)}`
    sameName.set(k, (sameName.get(k) || 0) + 1)
  }
  // A character's former names, after every real name (as on the server).
  for (const it of items) for (const a of it.aliases || []) add(a, it)

  const resolve = (target) => {
    const id = parseIdLink(target)
    if (id) return byId.get(`${id.type}#${id.id}`) || null
    const inner = target.trim()
    const colon = inner.indexOf(':')
    if (colon > 0) {
      const prefix = norm(inner.slice(0, colon))
      const type = TYPE_ORDER.find((k) => norm(k) === prefix)
      if (type) return (index.get(norm(inner.slice(colon + 1))) || []).find((x) => x.type === type) || null
    }
    return (index.get(norm(inner)) || [])[0] || null
  }
  resolve.byId = (type, id) => byId.get(`${type}#${id}`) || null
  resolve.nameTarget = (item) => {
    const name = item.name || ''
    if (!name.trim() || /[[\]|\n]/.test(name)) return null
    // Two of a kind with the same name: only the id tells them apart.
    if ((sameName.get(`${item.type}\0${norm(name)}`) || 0) > 1) return null
    for (const target of [name, `${item.type}:${name}`]) {
      const hit = resolve(target)
      if (hit && hit.type === item.type && String(hit.id) === String(item.id)) return target
    }
    return null
  }
  return resolve
}

// A typed target taken apart: the article kind it names (null when none)
// and the name after it. Same as splitWikiTarget in backend/wikilinks.go.
function splitTarget(target) {
  const inner = target.trim()
  const colon = inner.indexOf(':')
  if (colon > 0) {
    const prefix = norm(inner.slice(0, colon))
    const type = TYPE_ORDER.find((k) => norm(k) === prefix)
    if (type) return { type, name: inner.slice(colon + 1) }
  }
  return { type: null, name: inner }
}

// Text with each link outside code as the words a reader sees, for the
// places that show a text as plain text (a gear row, the map's hover, a
// timeline card…): never print a text that can hold links raw. A link's
// |text when it has one; else a stored link's article's current name
// (resolve.deletedLabel, or nothing, when the article is gone or the names
// haven't loaded); else the typed name without its kind. `resolve` is
// makeLinkResolver() or null. Same rules as wikiLinksAsText in
// backend/wikilinks.go (the fixtures' "plain" cases).
export function linksAsText(text, resolve) {
  return mapWikiLinks(text, (m) => {
    if (m.hasText && m.text.trim() !== '') return m.text
    const id = parseIdLink(m.target)
    if (id) {
      const item = resolve?.byId ? resolve.byId(id.type, id.id) : null
      return item ? item.name : (resolve && resolve.deletedLabel) || ''
    }
    return splitTarget(m.target).name.trim()
  })
}

// Stored links shown by name, for a plain-text editor: [[character#12]]
// becomes [[Aria Vale]] (or [[character:Aria Vale]] when another kind has the
// name), keeping any |text. A link to a deleted article, or one whose name
// can't lead back to it (shared with another of its kind, or holding [ ] |),
// stays as it is. The server stores them by id again on save.
export function linksForEditing(text, resolve) {
  if (!resolve?.byId) return String(text ?? '')
  return mapWikiLinks(text, (m) => {
    const id = parseIdLink(m.target)
    if (!id) return m.full
    const item = resolve.byId(id.type, id.id)
    const target = item && resolve.nameTarget(item)
    if (!target) return m.full
    return `[[${target}${m.hasText ? `|${m.text}` : ''}]]`
  })
}
