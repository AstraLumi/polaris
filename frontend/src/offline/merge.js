// Merging two edits of a chapter's text (docs/EDITOR.md, "Offline writing"):
// the text both started from (`base`), this browser's (`mine`) and the one
// saved elsewhere meanwhile (`theirs`), all the editor's doc JSON. A
// three-way merge over the top-level blocks (paragraphs, headings, tables,
// lists, dialogue…), compared as JSON, so it knows nothing of the node
// types: edits to different blocks are both kept; two edits to the same
// block, or text added at the same place by both, are a conflict, left to
// the writer. Pure.

// Blocks of base replaced, as { start, end, items }: base[start, end) became
// items. Common runs are found with a longest common subsequence over the
// part between the common start and end (one hunk when that part is huge).
const LIMIT = 4_000_000

export function hunks(a, b) {
  let pre = 0
  while (pre < a.length && pre < b.length && a[pre] === b[pre]) pre++
  let suf = 0
  while (suf < a.length - pre && suf < b.length - pre && a[a.length - 1 - suf] === b[b.length - 1 - suf]) suf++
  const n = a.length - pre - suf
  const m = b.length - pre - suf
  if (!n && !m) return []
  if (!n || !m || n * m > LIMIT) return [{ start: pre, end: pre + n, items: b.slice(pre, pre + m) }]

  // lcs[i][j]: the common run length of a[pre+i..] and b[pre+j..].
  const w = m + 1
  const lcs = new Uint32Array((n + 1) * w)
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      lcs[i * w + j] = a[pre + i] === b[pre + j] ? lcs[(i + 1) * w + j + 1] + 1 : Math.max(lcs[(i + 1) * w + j], lcs[i * w + j + 1])
    }
  }
  const out = []
  let i = 0
  let j = 0
  let open = null
  const flush = () => {
    if (open) out.push({ start: pre + open.i, end: pre + i, items: b.slice(pre + open.j, pre + j) })
    open = null
  }
  while (i < n || j < m) {
    if (i < n && j < m && a[pre + i] === b[pre + j]) {
      flush()
      i++
      j++
    } else {
      open ||= { i, j }
      if (j < m && (i >= n || lcs[i * w + j + 1] >= lcs[(i + 1) * w + j])) j++
      else i++
    }
  }
  flush()
  return out
}

const isInsert = (h) => h.start === h.end
const sameHunk = (x, y) => x.start === y.start && x.end === y.end && x.items.length === y.items.length && x.items.every((v, k) => v === y.items[k])

// Two hunks touch the same blocks: they overlap, or both add text at the
// same place (which order?). Text added just before or after a block the
// other side changed is fine.
export function clash(x, y) {
  if (sameHunk(x, y)) return false
  if (x.start < y.end && y.start < x.end) return true
  return isInsert(x) && isInsert(y) && x.start === y.start
}

const blocks = (doc) => (Array.isArray(doc?.content) ? doc.content.map((n) => JSON.stringify(n)) : [])
const str = (v) => JSON.stringify(v ?? null)

// { ok: true, doc } or { ok: false, clashes }.
export function mergeDocs(base, mine, theirs) {
  const sb = str(base)
  const sm = str(mine)
  const st = str(theirs)
  if (sm === st || st === sb) return { ok: true, doc: mine }
  if (sm === sb) return { ok: true, doc: theirs }

  // The doc's own attrs (the chapter's mode…) merge as one value.
  let attrs = mine?.attrs
  const [ab, am, at] = [str(base?.attrs), str(mine?.attrs), str(theirs?.attrs)]
  if (am === ab) attrs = theirs?.attrs
  else if (at !== ab && at !== am) return { ok: false, clashes: 1 }

  const b = blocks(base)
  const hm = hunks(b, blocks(mine))
  const ht = hunks(b, blocks(theirs))
  let clashes = 0
  for (const x of hm) for (const y of ht) if (clash(x, y)) clashes++
  if (clashes) return { ok: false, clashes }

  const all = [...hm, ...ht.filter((y) => !hm.some((x) => sameHunk(x, y)))].sort((x, y) => x.start - y.start || x.end - y.end)
  const out = []
  let pos = 0
  for (const h of all) {
    out.push(...b.slice(pos, h.start), ...h.items)
    pos = h.end
  }
  out.push(...b.slice(pos))
  const doc = { ...mine, content: out.map((s) => JSON.parse(s)) }
  if (attrs === undefined) delete doc.attrs
  else doc.attrs = attrs
  return { ok: true, doc }
}
