// How story dates read on screen (not how they are stored or typed, which
// is DD-MM-YYYY, see dateDigits.js). One shape everywhere: "15/03/1054",
// a year alone as just the year, and no zeros in front of a year. A year
// before 1 gets a real minus sign ("−54"), or the story's era names when
// it has them ("54 BC", "1054 AD"). No imports, so node can test it:
// node frontend/tests/storyDates.test.mjs

export const MINUS = '−'

const two = (n) => String(n).padStart(2, '0')

// A year as words: eras = { before, after } (the story's era names).
export function yearText(year, { before = '', after = '' } = {}) {
  if (year < 0) return before ? `${-year} ${before}` : `${MINUS}${-year}`
  return after ? `${year} ${after}` : String(year)
}

// A date's text ("15/03/1054"), or just the year when only a year was
// given (`yearOnly`).
export function dayText({ day, month, year }, yearOnly = false, eras = {}) {
  const y = yearText(year, eras)
  return yearOnly ? y : `${two(day)}/${two(month)}/${y}`
}
