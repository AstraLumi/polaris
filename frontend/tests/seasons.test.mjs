// The season on a day, in either hemisphere (src/seasons.js).
// No dependencies: node frontend/tests/seasons.test.mjs
import assert from 'node:assert/strict'
import { dayIndex, dateOfIndex, seasonCycle, seasonOn, monthSeasons, cycleYears } from '../src/seasons.js'

const cal = { months_per_year: 12, days_per_month: 30 }
const four = {
  seasons: ['Winter', 'Spring', 'Summer', 'Fall'].map((name) => ({ id: name.toLowerCase(), name, months: 3 })),
  start: '01-12-0001',
}
const at = (day, month, year, settings = four, other = false) =>
  seasonOn(dayIndex({ day, month, year }, cal), seasonCycle(settings, cal), other)?.season.name

// Days count across year 1 with no year 0.
assert.equal(dayIndex({ day: 1, month: 1, year: 1 }, cal), 0)
assert.equal(dayIndex({ day: 30, month: 12, year: -1 }, cal), -1, 'the day before 1-1-0001')
for (const n of [-1000, -1, 0, 359, 360, 12345]) assert.equal(dayIndex(dateOfIndex(n, cal), cal), n, `round trip ${n}`)

assert.equal(at(1, 12, 1), 'Winter', 'the start day')
assert.equal(at(15, 1, 2), 'Winter', 'winter runs into the next year')
assert.equal(at(1, 3, 2), 'Spring')
assert.equal(at(20, 7, 500), 'Summer', 'repeats every year')
assert.equal(at(30, 11, -40), 'Fall', 'and before the start')
assert.equal(at(15, 1, 2, four, true), 'Summer', 'the other hemisphere is half a cycle on')
assert.equal(at(15, 4, 2, four, true), 'Fall')

// A three-year winter: the cycle isn't a year long.
const long = {
  seasons: [{ id: 'w', name: 'Long Winter', months: 36 }, { id: 't', name: 'Thaw', months: 6 }],
  start: '01-01-0100',
}
assert.equal(at(1, 1, 102, long), 'Long Winter', 'still winter two years on')
assert.equal(at(1, 1, 103, long), 'Thaw')
assert.equal(at(1, 7, 103, long), 'Long Winter', 'the next cycle')
assert.equal(cycleYears(long, cal), 3.5)

// A month split where the season changes.
const c = seasonCycle({ ...four, start: '16-12-0001' }, cal)
assert.deepEqual(
  monthSeasons(5, 3, c, cal).map((r) => [r.season.name, r.from, r.to]),
  [['Winter', 1, 15], ['Spring', 16, 30]],
)
assert.equal(seasonCycle({ seasons: [], start: '01-01-0001' }, cal), null, 'no seasons')
console.log('seasons: ok')
