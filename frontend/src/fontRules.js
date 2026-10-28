// The fonts library's rules, pure (no Vue, no DOM; tests/fontLibrary.test.mjs):
// finding a library family by name, the @font-face rules that serve the
// library to the editor and the page view, merging it into the font
// pickers, and the license warnings (docs/EDITOR.md, "Fonts library").
//
// A library is GET /api/fonts: [{ id, name, license_kind, uses: { print,
// ebook, web, commercial }, files: [{ url, format, weight, italic,
// embedding, bitmap_only, … }], … }]. Names match ignoring case, as CSS
// font-family names do.

export const LICENSE_KINDS = ['ofl', 'apache', 'commercial', 'personal', 'other']
export const USES = ['print', 'ebook', 'web', 'commercial']

export const familyKey = (name) =>
  String(name || '')
    .trim()
    .toLowerCase()

// The library family a name means, or null.
export function findFamily(lib, name) {
  const k = familyKey(name)
  if (!k) return null
  return (lib || []).find((f) => familyKey(f.name) === k) || null
}

// A family name inside a CSS string.
export const cssFamily = (name) => `"${String(name).replace(/["\\\n\r{}<>;]/g, '')}"`

const CSS_FORMAT = { ttf: 'truetype', otf: 'opentype', woff: 'woff', woff2: 'woff2' }

// One @font-face per file of every family given.
export function fontFaceRules(families) {
  const out = []
  for (const fam of families || []) {
    for (const f of fam.files || []) {
      if (!f.url || !/^\/uploads\/fonts\/[0-9a-f]+\.(ttf|otf|woff2?)$/.test(f.url)) continue
      const format = CSS_FORMAT[f.format] ? ` format("${CSS_FORMAT[f.format]}")` : ''
      out.push(
        `@font-face { font-family: ${cssFamily(fam.name)}; src: url("${f.url}")${format}; ` +
          `font-weight: ${Number(f.weight) || 400}; font-style: ${f.italic ? 'italic' : 'normal'}; font-display: swap; }`,
      )
    }
  }
  return out.join('\n')
}

// The font pickers' list: each list in turn, first spelling of a name kept.
export function mergeChoices(...lists) {
  const seen = new Set()
  const out = []
  for (const list of lists) {
    for (const f of list || []) {
      const name = String(f || '').trim()
      if (name && !seen.has(name.toLowerCase())) {
        seen.add(name.toLowerCase())
        out.push(name)
      }
    }
  }
  return out
}

// A file a document may not carry (its fsType: restricted, or bitmaps only).
export const fileForbidsEmbedding = (f) => f.embedding === 'restricted' || !!f.bitmap_only
export const familyForbidsEmbedding = (fam) => (fam.files || []).some(fileForbidsEmbedding)
// Some file's permissions couldn't be read (a WOFF2).
export const familyEmbeddingUnknown = (fam) => (fam.files || []).some((f) => f.embedding === 'unknown')

// What a family's own record says is wrong for publishing a book with it:
// 'unrecorded' (no license kind yet), 'personal' (personal use only),
// 'no-commercial' (commercial use not ticked).
export function licenseProblem(fam) {
  if (!fam) return null
  if (fam.license_kind === 'personal') return 'personal'
  if (!fam.license_kind) return 'unrecorded'
  if (!fam.uses?.commercial) return 'no-commercial'
  return null
}

// Warnings for a book's styles: library families used by a style that is
// personal use only or not cleared for commercial use. A family the
// library doesn't have (Google, the computer's) gets none.
// → [{ family, reason: 'personal' | 'no-commercial', styles: [names] }]
export function styleWarnings(styles, lib) {
  const byFam = new Map()
  for (const s of styles || []) {
    const fam = findFamily(lib, s?.text?.font)
    if (!fam) continue
    const reason = licenseProblem(fam)
    if (reason !== 'personal' && reason !== 'no-commercial') continue
    if (!byFam.has(fam.id)) byFam.set(fam.id, { family: fam.name, reason, styles: [] })
    byFam.get(fam.id).styles.push(s.name)
  }
  return [...byFam.values()]
}

// The use an export needs: PDF and Word are print, EPUB is an e-book.
export const exportNeeds = (format) => (format === 'epub' ? 'ebook' : format === 'pdf' || format === 'docx' ? 'print' : null)

// Warnings before an export, from the families the book names
// (GET /api/books/{volume}/fonts: [{ family, styles, bands, chapters }]).
// EPUB doesn't carry headers and footers, so a family only bands use
// doesn't count there.
// → [{ family, reason: 'no-print' | 'no-ebook' | 'no-embedding' }]
export function exportWarnings(format, uses, lib) {
  const need = exportNeeds(format)
  if (!need) return []
  const out = []
  const done = new Set()
  for (const u of uses || []) {
    if (format === 'epub' && !(u.styles?.length || u.chapters)) continue
    const fam = findFamily(lib, u.family)
    if (!fam || done.has(fam.id)) continue
    done.add(fam.id)
    if (!fam.uses?.[need]) out.push({ family: fam.name, reason: need === 'print' ? 'no-print' : 'no-ebook' })
    if (familyForbidsEmbedding(fam)) out.push({ family: fam.name, reason: 'no-embedding' })
  }
  return out
}

// Where a family is used (GET /api/fonts/used's list for it), counted.
export function usedSummary(uses) {
  const stories = new Set()
  let books = 0
  for (const u of uses || []) {
    stories.add(u.story_id)
    books++
  }
  return { stories: stories.size, books }
}

// What the library page warns about one family: its license problem
// (only when something uses it, except "unrecorded"), and embedding.
export function familyWarnings(fam, uses) {
  const out = []
  const used = (uses || []).length > 0
  const p = licenseProblem(fam)
  if (p === 'unrecorded') out.push('unrecorded')
  else if (p && used && (uses || []).some((u) => u.styles?.length)) out.push(p)
  if (familyForbidsEmbedding(fam)) out.push('no-embedding')
  return out
}
