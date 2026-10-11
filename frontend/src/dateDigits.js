// The single-box date input (components/DateField.vue): the digits are typed
// and shown left to right as usual, and read from the right: the last four
// are the year, the two before them the month, the rest the day.
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

// What the box shows: the digits just as typed, left to right, with dashes
// put in from the right where the parts split ("2-1456", "13-02-1456"), so
// typing reads like normal typing. Before year 1: "-50", "02-(-0050)".
export function displayDigits(digits, negative) {
  if (!digits) return negative ? '-' : ''
  const year = digits.slice(-4)
  const month = digits.length > 4 ? digits.slice(-6, -4) : ''
  const day = digits.length > 6 ? digits.slice(0, -6) : ''
  if (!month) return negative ? `-${year}` : year
  return [day, month, negative ? `(-${year})` : year].filter(Boolean).join('-')
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
