// The credits text for the copyright page (src/editor/fontCredits.js).
// node frontend/tests/fontCredits.test.mjs
import assert from 'node:assert/strict'
import { licenseLabel, creditFor, creditsText } from '../src/editor/fontCredits.js'

const merri = { family: 'Merriweather', source: 'google', license_id: 'OFL-1.1', license: 'This Font Software is licensed under the SIL Open Font License, Version 1.1. This license is available…', copyright: 'Copyright 2016 The Merriweather Project Authors' }
const turret = { family: 'Turret Road', source: 'google', license_id: 'OFL-1.1', copyright: 'Copyright 2012\n The Turret Road   Authors' }
const georgia = { family: 'Georgia', source: 'system', license_id: '', license: '' }
const lib = { family: 'Shop Script', source: 'library', license_id: '', license: 'Shop licence 4', designer: 'A. Writer' }
const none = { family: 'Mystery', source: 'unknown' }

assert.equal(licenseLabel(merri), 'SIL Open Font License 1.1')
assert.equal(licenseLabel(lib), 'Shop licence 4')
assert.equal(licenseLabel(none), '')
assert.equal(licenseLabel({ license: 'x'.repeat(300) }).length, 118)

assert.equal(creditFor(merri), 'Merriweather (Copyright 2016 The Merriweather Project Authors; SIL Open Font License 1.1)')
assert.equal(creditFor(turret), 'Turret Road (Copyright 2012 The Turret Road Authors; SIL Open Font License 1.1)')
assert.equal(creditFor(lib), 'Shop Script (Designed by A. Writer; Shop licence 4)')
assert.equal(creditFor(none), 'Mystery')

assert.equal(
  creditsText([merri, turret]),
  'Set in Merriweather (Copyright 2016 The Merriweather Project Authors; SIL Open Font License 1.1) and Turret Road (Copyright 2012 The Turret Road Authors; SIL Open Font License 1.1).',
)
assert.equal(creditsText([merri, georgia, none]), 'Set in Merriweather (Copyright 2016 The Merriweather Project Authors; SIL Open Font License 1.1) and Mystery.')
assert.equal(creditsText([merri, georgia, none, turret]).includes('Georgia'), false)
assert.equal(creditsText([georgia], { includeSystem: true }), 'Set in Georgia.')
assert.equal(creditsText([georgia]), '')
assert.equal(creditsText([]), '')
assert.equal(creditsText(null), '')
console.log('fontCredits ok')
