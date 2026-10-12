// The story's seasons (backend/seasons.go): named spans of so many months
// that repeat from a start date. The cycle needn't be a year long. The
// settings describe one hemisphere; the other is half a cycle apart, so with
// four equal seasons its winter is the first one's summer.
// No dependencies: node frontend/tests/seasons.test.mjs
//
// cal is { months_per_year, days_per_month }; dates are { day, month, year }
// with no year 0.

// Days since 1-1-0001 (negative before it).
export function dayIndex({ day, month, year }, cal) {
  const yearIndex = year > 0 ? year - 1 : year
  return (yearIndex * cal.months_per_year + (month - 1)) * cal.days_per_month + (day - 1)
}

export function dateOfIndex(n, cal) {
  const perYear = cal.months_per_year * cal.days_per_month
  const yearIndex = Math.floor(n / perYear)
  const rest = n - yearIndex * perYear
  return {
    day: (rest % cal.days_per_month) + 1,
    month: Math.floor(rest / cal.days_per_month) + 1,
    year: yearIndex >= 0 ? yearIndex + 1 : yearIndex,
  }
}

const START_RE = /^(\d{1,3})-(\d{1,3})-(-?\d{1,9})$/

// The cycle laid out in days: each season's first day within it, and its
// length (at least one day). null when there are no seasons or no start.
export function seasonCycle(settings, cal) {
  const list = settings?.seasons || []
  const m = START_RE.exec(settings?.start || '')
  if (!list.length || !m) return null
  let at = 0
  const spans = list.map((s) => {
    const days = Math.max(1, Math.round(s.months * cal.days_per_month))
    const span = { season: s, from: at, days }
    at += days
    return span
  })
  const start = dayIndex({ day: +m[1], month: +m[2], year: +m[3] }, cal)
  return { spans, length: at, start }
}

// The season on day n (a dayIndex) in the settings' own hemisphere, or in
// the other one with other = true. Answers { season, first, last }: the
// season and the dayIndex of its first and last day around n.
export function seasonOn(n, cycle, other = false) {
  if (!cycle) return null
  const shift = other ? Math.floor(cycle.length / 2) : 0
  const pos = (((n - cycle.start + shift) % cycle.length) + cycle.length) % cycle.length
  const span = cycle.spans.find((s) => pos >= s.from && pos < s.from + s.days) || cycle.spans[cycle.spans.length - 1]
  const first = n - (pos - span.from)
  return { season: span.season, first, last: first + span.days - 1 }
}

// A month split into runs of one season: [{ season, from, to }] with from/to
// the days of the month.
export function monthSeasons(year, month, cycle, cal, other = false) {
  if (!cycle) return []
  const out = []
  const first = dayIndex({ day: 1, month, year }, cal)
  let day = 1
  while (day <= cal.days_per_month) {
    const s = seasonOn(first + day - 1, cycle, other)
    const to = Math.min(cal.days_per_month, day + (s.last - (first + day - 1)))
    out.push({ season: s.season, from: day, to })
    day = to + 1
  }
  return out
}

// How long the whole cycle is, in years (fractions allowed).
export function cycleYears(settings, cal) {
  const months = (settings?.seasons || []).reduce((sum, s) => sum + (Number(s.months) || 0), 0)
  return months / cal.months_per_year
}

// A fresh id for a season added in the editor.
export const newSeasonId = () => 's' + Date.now().toString(36) + Math.random().toString(36).slice(2, 6)
