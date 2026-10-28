// What a kept book needs besides its texts (docs/EDITOR.md, "Offline
// writing"): the pictures its docs show and the font families its styles,
// headers and text name. Read from the doc JSON without knowing its node
// types: any attr `src` that is an upload, any mark's `family`. Pure.

function walk(node, visit) {
  if (!node || typeof node !== 'object') return
  visit(node)
  for (const m of node.marks || []) visit(m)
  for (const c of node.content || []) walk(c, visit)
}

// Every upload a doc shows (/uploads/…), in order, once.
export function docPictures(docs) {
  const out = new Set()
  for (const doc of docs) {
    walk(doc, (n) => {
      const src = n.attrs?.src
      if (typeof src === 'string' && src.startsWith('/uploads/')) out.add(src)
    })
  }
  return [...out]
}

// Every font family the book names: its styles, its header and footer
// bands, and the font marks in its docs.
export function bookFamilies(settings, docs = []) {
  const out = new Set()
  const add = (f) => {
    if (typeof f === 'string' && f.trim()) out.add(f.trim())
  }
  for (const s of settings?.styles || []) add(s.text?.font)
  const layout = settings?.layout || {}
  for (const side of ['header', 'footer']) for (const band of Object.values(layout[side] || {})) add(band?.font)
  for (const doc of docs) walk(doc, (n) => add(n.attrs?.family))
  return [...out]
}

// The font files of a Google Fonts stylesheet worth keeping for a book in
// Latin script: the faces marked latin and latin-ext (Google's CSS names
// each @font-face's subset in a comment before it), or every face when the
// sheet names none.
export function googleFontFiles(css) {
  const text = String(css || '')
  const urls = (s) => [...s.matchAll(/url\((['"]?)([^'")]+)\1\)/g)].map((m) => m[2])
  const parts = [...text.matchAll(/\/\*\s*([\w-]+)\s*\*\/\s*@font-face\s*\{[^}]*\}/g)]
  if (!parts.length) return [...new Set(urls(text))]
  return [...new Set(parts.filter((p) => p[1] === 'latin' || p[1] === 'latin-ext').flatMap((p) => urls(p[0])))]
}
