// How story dates read (src/storyDates.js).
// No dependencies: node frontend/tests/storyDates.test.mjs
import assert from 'node:assert/strict'
import { yearText, dayText, MINUS } from '../src/storyDates.js'

assert.equal(yearText(1054), '1054')
assert.equal(yearText(430), '430', 'no zeros in front')
assert.equal(yearText(-54), `${MINUS}54`, 'a real minus sign')
assert.equal(yearText(-54, { before: 'BC' }), '54 BC')
assert.equal(yearText(1054, { before: 'BC' }), '1054', 'the before name only before year 1')
assert.equal(yearText(1054, { after: 'AD' }), '1054 AD')
assert.equal(yearText(-2000, { before: 'BC', after: 'AD' }), '2000 BC')

assert.equal(dayText({ day: 1, month: 9, year: 430 }), '01/09/430')
assert.equal(dayText({ day: 15, month: 3, year: -2000 }), `15/03/${MINUS}2000`)
assert.equal(dayText({ day: 15, month: 3, year: -2000 }, false, { before: 'before the Crown' }), '15/03/2000 before the Crown')
assert.equal(dayText({ day: 1, month: 1, year: 1054 }, true), '1054', 'a year alone stays a year')
assert.equal(dayText({ day: 1, month: 1, year: -5 }, true, { before: 'BC' }), '5 BC')
const names = ['Frostfall', '', 'Thaw']
assert.equal(dayText({ day: 5, month: 1, year: 1054 }, false, {}, names), '5 Frostfall 1054', 'a named month reads as words')
assert.equal(dayText({ day: 5, month: 2, year: 1054 }, false, {}, names), '05/02/1054', 'an unnamed month keeps its number')
assert.equal(dayText({ day: 5, month: 7, year: 1054 }, false, {}, names), '05/07/1054', 'past the names, numbers')
assert.equal(dayText({ day: 15, month: 3, year: -54 }, false, { before: 'BC' }, names), '15 Thaw 54 BC')
assert.equal(dayText({ day: 1, month: 1, year: 1054 }, true, {}, names), '1054', 'a year alone stays a year')

console.log('storyDates: ok')
