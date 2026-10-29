// The toolbar's font picker (components/write/FontPicker.vue): every font
// stays listed, what was typed only pulls the best matches to the top; the
// writer's favourite and recent fonts come next. Pure (tests/fontSearch.test.mjs).

const words = (s) => s.toLowerCase().split(/[^\p{L}\p{N}]+/u).filter(Boolean)

// How well a font name matches what was typed (0: not at all). Exact, then
// the start of the name, the start of a word, every typed word somewhere
// ("noto sym" → Noto Sans Symbols), anywhere, then its letters in order.
export function fontScore(name, query) {
  const n = String(name || '').toLowerCase()
  const q = String(query || '').trim().toLowerCase()
  if (!q || !n) return 0
  if (n === q) return 100
  if (n.startsWith(q)) return 80
  const ws = words(n)
  if (ws.some((w) => w.startsWith(q))) return 60
  const qs = words(q)
  if (qs.length > 1 && qs.every((x) => ws.some((w) => w.startsWith(x)))) return 50
  if (n.includes(q)) return 40
  let i = 0
  for (const ch of n) if (ch === q[i]) i++
  return i === q.length ? 20 : 0
}

// The fonts that match the query, best first (a shorter name first among
// equals, then the list's order).
export function bestMatches(list, query, max = 6) {
  return list
    .map((name, i) => ({ name, i, s: fontScore(name, query) }))
    .filter((x) => x.s > 0)
    .sort((a, b) => b.s - a.s || a.name.length - b.name.length || a.i - b.i)
    .slice(0, max)
    .map((x) => x.name)
}

const same = (a, b) => a.toLowerCase() === b.toLowerCase()

// The picker's sections, in order, each font once: what matches the query
// (only while typing), favourites, recent, then every font.
export function fontSections({ fonts = [], favourites = [], recent = [], query = '' }) {
  const out = []
  const taken = []
  const add = (key, names) => {
    const list = []
    for (const n of names) {
      if (n && !taken.some((t) => same(t, n))) {
        list.push(n)
        taken.push(n)
      }
    }
    if (list.length) out.push({ key, fonts: list })
  }
  if (String(query).trim()) add('matches', bestMatches([...favourites, ...recent, ...fonts], query))
  add('favourites', favourites)
  add('recent', recent)
  add('all', fonts)
  return out
}

// A font used just now goes to the front of the recent list.
export const pushRecent = (list, name, max = 6) => (name ? [name, ...list.filter((n) => !same(n, name))].slice(0, max) : list)

// Adds a font to the favourites, or takes it off.
export const toggleFavourite = (list, name) => (list.some((n) => same(n, name)) ? list.filter((n) => !same(n, name)) : [...list, name])
