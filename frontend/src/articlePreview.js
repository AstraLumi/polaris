import { wikiApi } from './wiki'

// Hover previews of wiki articles (components/LinkPreviewLayer.vue): which
// links get one, and a short summary of the article they point at.

// /wiki/<type>/<id>: rendered [[links]], related lists, the wiki's index.
const ARTICLE_PATH = /^\/wiki\/([a-z_]+)\/(\d+)$/

// The article a link points at, or null. Takes any element inside the link.
export function articleOfLink(el) {
  const a = el && el.closest ? el.closest('a[href]') : null
  if (!a) return null
  const m = ARTICLE_PATH.exec(a.getAttribute('href') || '')
  return m ? { el: a, type: m[1], id: m[2] } : null
}

// Markdown down to plain text, for an excerpt.
export function plainText(md) {
  return String(md || '')
    .replace(/```[\s\S]*?```/g, ' ')
    .replace(/\[\[([^\]|]+)\|([^\]]*)\]\]/g, '$2')
    .replace(/\[\[(?:[a-z_ ]+:)?([^\]]+)\]\]/gi, '$1')
    .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
    .replace(/^\s{0,3}(#{1,6}|>|[-*+]|\d+[.)])\s+/gm, '')
    .replace(/^\s*\|?\s*:?-{2,}.*$/gm, ' ')
    .replace(/[|*_~`]+/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
}

export function excerpt(text, max = 260) {
  if (text.length <= max) return text
  const cut = text.slice(0, max)
  const space = cut.lastIndexOf(' ')
  return `${cut.slice(0, space > max * 0.6 ? space : max).replace(/[\s,;:.]+$/, '')}…`
}

// The first written text of an article: its introduction, else the first
// filled box, else text from the original page, else a section.
export function articleText(a) {
  const candidates = [
    a.summary,
    ...(a.field_keys || []).map((k) => a.fields?.[k]),
    ...(a.sheet || []).map((s) => s.body),
    ...(a.sections || []).map((s) => s.body),
  ]
  for (const c of candidates) {
    const t = plainText(c)
    if (t) return excerpt(t)
  }
  return ''
}

const TTL = 60_000
const cache = new Map() // "type/id" -> { at, promise }

// The article, fetched once a minute at most.
export function loadArticle(type, id) {
  const key = `${type}/${id}`
  const hit = cache.get(key)
  if (hit && Date.now() - hit.at < TTL) return hit.promise
  const promise = wikiApi.get(type, id)
  promise.catch(() => cache.delete(key))
  cache.set(key, { at: Date.now(), promise })
  return promise
}
