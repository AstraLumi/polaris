import { reactive } from 'vue'
import { t, tr } from './i18n'
import { STORY_API } from './stories'
import { applyMapSettings } from './worldMap'
import { applyBloodTypes } from './bloodTypes'
import { yearText, dayText } from './storyDates'

// The story world's calendar. Loaded once at startup (App.vue) and again
// whenever Settings saves, so every date input and the timeline agree on
// how many months a year has and how many days a month has.
export const calendar = reactive({
  months_per_year: 12,
  days_per_month: 30,
  // How long the world's day is (its spin, and with the rest, its year).
  hours_per_day: 24,
  // Era names after a year (before year 1, from year 1 on); '' for none.
  year_before: '',
  year_after: '',
  // The months' names in order; '' (or past the list) leaves a month to
  // its number. The list can be longer than the year (settings.go).
  month_names: [],
  // The story's seasons (seasons.js): { seasons, start, hemisphere, unit }.
  seasons: { seasons: [], start: '', hemisphere: 'north', unit: 'C' },
  loaded: false,
})

export async function loadCalendar() {
  try {
    const res = await fetch(`${STORY_API}/settings`)
    if (res.ok) {
      const data = await res.json()
      calendar.months_per_year = data.months_per_year
      calendar.days_per_month = data.days_per_month
      calendar.hours_per_day = data.hours_per_day || 24
      calendar.year_before = data.year_before || ''
      calendar.year_after = data.year_after || ''
      calendar.month_names = data.month_names || []
      if (data.seasons) calendar.seasons = data.seasons
      applyMapSettings(data.map)
      applyBloodTypes(data.blood_types)
    }
  } catch (err) {
    // Keep the defaults; date inputs still work.
  } finally {
    calendar.loaded = true
  }
}

// How many days a row of a month shows. The story's months have no weeks:
// rows of ten read well for 20, 30 or 40 days, otherwise rows of seven.
export const dayColumns = (days) => (days % 10 === 0 && days >= 20 ? 10 : 7)

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

// A year as the story writes it: "1054", "−54", or with its era names
// ("54 BC") from Settings → Calendar (storyDates.js).
export const yearLabel = (year) => yearText(year, { before: calendar.year_before, after: calendar.year_after })

// A month's name from Settings → Calendar, or '' when it has none.
export const monthNameOf = (month) => (month <= calendar.months_per_year && calendar.month_names[month - 1]) || ''

// A month as a title: its name, or "Month 3".
export const monthLabel = (month) => monthNameOf(month) || t('Month {n}', { n: month })

// "1033 Q2" — what the timeline shows instead of the full date.
export function shortDate(text) {
  const d = parseStoryDate(text)
  if (!d) return text || ''
  return `${yearLabel(d.year)} ${t('Q{n}', { n: quarterOf(d.month) })}`
}

// "15/03/1054" for anything readable ("15 Frostfall 1054" when the month
// has a name, "1054" for a year alone, "−54" or "54 BC" before year 1),
// the raw text otherwise. For display only; stored
// dates keep the DD-MM-YYYY form.
export function fullDate(text) {
  const d = parseStoryDate(text)
  if (!d) return text || ''
  const yearOnly = !/^\s*\d{1,3}-\d{1,3}-/.test(text)
  const names = calendar.month_names.slice(0, calendar.months_per_year)
  return dayText(d, yearOnly, { before: calendar.year_before, after: calendar.year_after }, names)
}

// { year, yearText, quarter } for the big year / small "Q2" blocks, or null
// when the text isn't a readable date.
export function dateParts(text) {
  const d = parseStoryDate(text)
  return d ? { year: d.year, yearText: yearLabel(d.year), quarter: quarterOf(d.month) } : null
}

// Messages the server can send back about seasons, climates and the
// equator (backend/seasons.go, planet.go); listed so they get translated.
export const SEASON_MESSAGES = [
  tr('a story can have at most 24 seasons'),
  tr('every season needs a name'),
  tr('that season name is too long'),
  tr('a season lasts more than 0 and at most 1200 months'),
  tr('the first season needs a start date'),
  tr('only a region has a climate'),
  tr('unknown season'),
  tr('that temperature is out of range'),
  tr('too many seasons'),
  tr('the equator is off the map'),
  tr('that bend is out of range'),
  tr('failed to save the equator'),
  tr("the hex size and the planet's size must be more than 0"),
  tr('the axial tilt is between 0 and 90 degrees'),
  tr('an era name is at most 24 characters, on one line'),
  tr('a month name is at most 24 characters, on one line'),
]
