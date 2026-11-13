// The filters of the culture pages and the reference library: pure, no
// imports (tested in tests/cultures.test.mjs).

const fold = (s) => String(s || '').toLocaleLowerCase()

// Every tag used in a list of items with `tags`, each spelling once (the
// first seen), sorted.
export function allTags(items) {
  const seen = new Map()
  for (const it of items || []) {
    for (const tag of it.tags || []) {
      if (!seen.has(fold(tag))) seen.set(fold(tag), tag)
    }
  }
  return [...seen.values()].sort((a, b) => a.localeCompare(b, undefined, { sensitivity: 'base' }))
}

// Whether an item carries every one of the tags (any case).
export const hasTags = (item, tags) => (tags || []).every((t) => (item.tags || []).some((x) => fold(x) === fold(t)))

// The items whose text holds every search word (somewhere in `fields`),
// of the kind asked (if any), carrying every picked tag.
export function filterItems(items, { search = '', tags = [], kind = '', fields = ['name'] } = {}) {
  const words = fold(search).split(/\s+/).filter(Boolean)
  return (items || []).filter((it) => {
    if (kind && it.kind !== kind) return false
    if (!hasTags(it, tags)) return false
    const text = fields.map((f) => fold(Array.isArray(it[f]) ? it[f].join(' ') : it[f])).join(' ')
    return words.every((w) => text.includes(w))
  })
}

// How many of a culture's elements are of each kind: { clothing: 3, … }.
export function countByKind(elements) {
  const out = {}
  for (const e of elements || []) out[e.kind] = (out[e.kind] || 0) + 1
  return out
}

// The pictures on the clipboard (navigator.clipboard, passed in), as files
// the forms can send: each item's first picture type. Throws when the
// browser refuses to read it.
export async function clipboardPictures(clipboard) {
  const out = []
  for (const item of await clipboard.read()) {
    const type = (item.types || []).find((ty) => String(ty).startsWith('image/'))
    if (!type) continue
    const blob = await item.getType(type)
    const ext = type.slice('image/'.length).replace('jpeg', 'jpg').replace(/[^a-z0-9]/g, '') || 'png'
    out.push(new File([blob], `pasted-${out.length + 1}.${ext}`, { type }))
  }
  return out
}

// The pictures in a paste (Ctrl+V of a screenshot or a copied image), as
// files the forms can send.
export function pastedPictures(event) {
  const out = []
  for (const item of event?.clipboardData?.items || []) {
    if (item.kind === 'file' && String(item.type).startsWith('image/')) {
      const f = item.getAsFile()
      if (f) out.push(f)
    }
  }
  return out
}
