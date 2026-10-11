// The single-box date input's digits. No dependencies: node frontend/tests/dateDigits.test.mjs
import assert from 'node:assert/strict'
import { digitsToParts, partsToDigits, displayDigits, parsePasted, MAX_DIGITS } from '../src/dateDigits.js'

// Typing fills in from the right.
const shown = (d, neg = false) => displayDigits(d, neg)
assert.equal(shown(''), '')
assert.equal(shown('2'), 'DD-MM-0002')
assert.equal(shown('21'), 'DD-MM-0021')
assert.equal(shown('214'), 'DD-MM-0214')
assert.equal(shown('2145'), 'DD-MM-2145')
assert.equal(shown('21456'), 'DD-02-1456')
assert.equal(shown('121456'), 'DD-12-1456')
assert.equal(shown('3021456'), '03-02-1456')
assert.equal(shown('13021456'), '13-02-1456')
assert.equal(shown('50', true), 'DD-MM-(-0050)')
assert.equal(shown('', true), 'DD-MM-(-)')
assert.equal(displayDigits('21456', false, { day: 'DD', month: 'MM' }), 'DD-02-1456')
assert.equal(MAX_DIGITS, 8)

assert.equal(digitsToParts(''), null)
assert.deepEqual(digitsToParts('2'), { day: null, month: null, year: 2 })
assert.deepEqual(digitsToParts('21456'), { day: null, month: 2, year: 1456 })
assert.deepEqual(digitsToParts('3021456'), { day: 3, month: 2, year: 1456 })
assert.deepEqual(digitsToParts('13021456'), { day: 13, month: 2, year: 1456 })
assert.deepEqual(digitsToParts('50', true), { day: null, month: null, year: -50 })

// Showing a stored date again.
assert.equal(partsToDigits({ day: 13, month: 2, year: 1456 }), '13021456')
assert.equal(partsToDigits({ day: 1, month: 1, year: 7 }), '01010007')
assert.equal(partsToDigits({ day: 1, month: 1, year: 1200 }, true), '1200')
assert.equal(partsToDigits({ day: 1, month: 1, year: -50 }, true), '50')
for (const d of ['13021456', '01010007']) assert.equal(partsToDigits(digitsToParts(d)), d)

// Pasting a whole date.
assert.deepEqual(parsePasted('13-02-1456'), { digits: '13021456', negative: false })
assert.deepEqual(parsePasted('3/2/1456'), { digits: '03021456', negative: false })
assert.deepEqual(parsePasted(' 1456 '), { digits: '1456', negative: false })
assert.deepEqual(parsePasted('-50'), { digits: '50', negative: true })
assert.deepEqual(parsePasted('03-02--0050'), { digits: '03020050', negative: true })
assert.equal(parsePasted('spring of 1200'), null)
assert.equal(parsePasted('12345'), null)

console.log('dateDigits: ok')
