// Where a font is used in the text ("Fonts and licenses" in the Book and page
// panel): its runs of text in a chapter, and stepping through them across
// the book. A run is neighbouring inline nodes whose font mark names the
// family (any case); docFontRuns in backend/fontlib.go counts the same way
// and sends each chapter's count as the font's places. Pure.

const familyOf = (node) => {
  const m = node.marks.find((x) => x.type.name === 'font')
  return (m?.attrs.family || '').trim().toLowerCase()
}

// Every run of the family in a doc, in order, as { from, to }.
export function fontRuns(doc, family) {
  const want = (family || '').trim().toLowerCase()
  const out = []
  if (!want) return out
  const scan = (node, start) => {
    let prev = ''
    node.forEach((child, offset) => {
      const fam = familyOf(child)
      const from = start + offset
      if (fam === want && prev === want) out[out.length - 1].to = from + child.nodeSize
      else if (fam === want) out.push({ from, to: from + child.nodeSize })
      prev = fam
    })
  }
  scan(doc, 0)
  doc.descendants((node, pos) => scan(node, pos + 1))
  return out
}

// How many runs the places hold ([{ chapter, runs }], as the server sends them).
export const placeCount = (places) => (places || []).reduce((n, p) => n + (p.runs || 0), 0)

// The k-th run of the places (0-based, wrapping both ways): its chapter and
// its index there.
export function placeAt(places, k) {
  const total = placeCount(places)
  if (!total) return null
  let i = ((k % total) + total) % total
  for (const p of places) {
    if (i < p.runs) return { chapter: p.chapter, run: i }
    i -= p.runs
  }
  return null
}
