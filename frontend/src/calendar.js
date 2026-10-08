import { reactive } from 'vue'
import { t } from './i18n'
import { STORY_API } from './stories'

// The story world's calendar. Loaded once at startup (App.vue) and again
// whenever Settings saves, so every date input and the timeline agree on
// how many months a year has and how many days a month has.
export const calendar = reactive({
  months_per_year: 12,
  days_per_month: 30,
  loaded: false,
})

export async function loadCalendar() {
  try {
    const res = await fetch(`${STORY_API}/settings`)
    if (res.ok) {
      const data = await res.json()
      calendar.months_per_year = data.months_per_year
      calendar.days_per_month = data.days_per_month
    }
  } catch (err) {
    // Keep the defaults; date inputs still work.
  } finally {
    calendar.loaded = true
  }
}

// ---- Canonical story-date text (mirrors backend/dates.go) -------------------

const DATE_RE = /^(?:(\d{1,3})-(\d{1,3})-)?(-?\d{1,9})$/

// "15-03-1054" / "1054" / "-54" -> { day, month, year }, or null when the
// text isn't something this app can place on a timeline.
export function parseStoryDate(text) {
  const m = DATE_RE.exec((text || '').trim())
  if (!m) return null
  const year = parseInt(m[3], 10)
  let day = 1
  let month = 1
  if (m[1] !== undefined) {
    day = parseInt(m[1], 10)
    month = parseInt(m[2], 10)
    if (day < 1 || month < 1) return null
  }
  return { day, month, year }
}

function padYear(year) {
  const abs = String(Math.abs(year)).padStart(4, '0')
  return year < 0 ? `-${abs}` : abs
}

export function formatStoryDate({ day, month, year }) {
  return `${String(day).padStart(2, '0')}-${String(month).padStart(2, '0')}-${padYear(year)}`
}

// Q1–Q4: the year split into four equal parts of however many months the
// calendar has (so a 10-month year gives 2.5 months per quarter).
export function quarterOf(month) {
  const q = Math.floor(((month - 1) * 4) / calendar.months_per_year) + 1
  return Math.min(4, Math.max(1, q))
}

// "1033 Q2" — what the timeline shows instead of the full date.
export function shortDate(text) {
  const d = parseStoryDate(text)
  if (!d) return text || ''
  return `${d.year} ${t('Q{n}', { n: quarterOf(d.month) })}`
}

// "15-03-1054" for anything readable, the raw text otherwise. This is for
// display only: a negative year is parenthesised ("15-03-(-0054)") so the
// two dashes don't read as a typo. Stored dates keep the plain form.
export function fullDate(text) {
  const d = parseStoryDate(text)
  if (!d) return text || ''
  const dd = String(d.day).padStart(2, '0')
  const mm = String(d.month).padStart(2, '0')
  return d.year < 0 ? `${dd}-${mm}-(${padYear(d.year)})` : `${dd}-${mm}-${padYear(d.year)}`
}

// { year, quarter } for the big year / small "Q2" blocks, or null when the
// text isn't a readable date.
export function dateParts(text) {
  const d = parseStoryDate(text)
  return d ? { year: d.year, quarter: quarterOf(d.month) } : null
}
