// The single-box date input (components/DateField.vue): what's typed is a
// string of digits that fills in from the right — the last four are the
// year, the two before them the month, the two before those the day.
//   "2" -> year 2, "21456" -> month 2 of 1456, "3021456" -> 3rd of month 2, 1456
// No dependencies: node frontend/tests/dateDigits.test.mjs

export const MAX_DIGITS = 8

// { day, month, year } from typed digits (day/month null when not typed yet),
// or null for nothing typed. A negative year comes from the separate sign.
export function digitsToParts(digits, negative = false) {
  if (!digits) return null
  const year = Number(digits.slice(-4))
  const month = digits.length > 4 ? Number(digits.slice(-6, -4)) : null
  const day = digits.length > 6 ? Number(digits.slice(-8, -6)) : null
  return { day, month, year: negative ? -year : year }
}

// The digits that show a stored date again: just the year for 1 January of
// a year written without day and month, the full eight otherwise.
export function partsToDigits({ day, month, year }, bare = false) {
  const y = String(Math.abs(year))
  if (bare) return y
  return String(day).padStart(2, '0') + String(month).padStart(2, '0') + y.padStart(4, '0')
}

// What the box shows: DD-MM-YYYY with the typed digits in place and the
// rest as placeholders ("DD-02-1456"). Before year 1: "DD-MM-(-0050)".
export function displayDigits(digits, negative, labels = { day: 'DD', month: 'MM' }) {
  if (!digits) return negative ? `${labels.day}-${labels.month}-(-)` : ''
  const year = digits.slice(-4).padStart(4, '0')
  const month = digits.length > 4 ? digits.slice(-6, -4).padStart(2, '0') : labels.month
  const day = digits.length > 6 ? digits.slice(-8, -6).padStart(2, '0') : labels.day
  return `${day}-${month}-${negative ? `(-${year})` : year}`
}

// A pasted or dropped date ("13-02-1456", "13/02/1456", "1456", "-50"), as
// { digits, negative }, or null when it isn't one.
export function parsePasted(text) {
  const s = String(text || '').trim().replace(/[/.\s]+/g, '-')
  const m = /^(?:(\d{1,2})-(\d{1,2})-)?(-?)(\d{1,4})$/.exec(s)
  if (!m) return null
  const negative = m[3] === '-'
  if (m[1] === undefined) return { digits: String(Number(m[4])), negative }
  return { digits: m[1].padStart(2, '0') + m[2].padStart(2, '0') + m[4].padStart(4, '0'), negative }
}
