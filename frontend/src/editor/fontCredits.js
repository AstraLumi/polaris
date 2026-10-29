// The credits for a book's copyright page, from the rows of
// GET /api/books/{volume}/fonts (docs/EDITOR.md, "Fonts and licenses").
// Pure: plain text only, nothing is drawn here.

const LICENSE_NAMES = {
  'OFL-1.1': 'SIL Open Font License 1.1',
  'Apache-2.0': 'Apache License 2.0',
  'UFL-1.0': 'Ubuntu Font License 1.0',
}

const oneLine = (s) => String(s ?? '').replace(/\s+/g, ' ').trim()

// The license as words: a known id by its name, else what the font says.
export function licenseLabel(row) {
  if (LICENSE_NAMES[row.license_id]) return LICENSE_NAMES[row.license_id]
  const own = oneLine(row.license)
  return own.length > 120 ? own.slice(0, 117) + '…' : own
}

// "Merriweather (Copyright 2016 The Merriweather Project Authors; SIL Open
// Font License 1.1)": what a font file says, or just the name.
export function creditFor(row) {
  const copy = oneLine(row.copyright) || (row.designer ? `Designed by ${oneLine(row.designer)}` : '')
  const parts = [copy, licenseLabel(row)].filter(Boolean)
  return parts.length ? `${oneLine(row.family)} (${parts.join('; ')})` : oneLine(row.family)
}

// "Set in A (…), B (…) and C (…)." for the families a book uses. System
// fonts are left out unless asked for: they are the reader's own.
export function creditsText(rows, { includeSystem = false } = {}) {
  const list = (rows || []).filter((r) => r && oneLine(r.family) && (includeSystem || r.source !== 'system')).map(creditFor)
  if (!list.length) return ''
  const head = list.length > 1 ? `${list.slice(0, -1).join(', ')} and ${list[list.length - 1]}` : list[0]
  return `Set in ${head}.`
}
