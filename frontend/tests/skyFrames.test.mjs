// The sky from a place at a moment (src/skyFrames.js): the calendar's
// clock, the planet's axis and spin, the horizon, and the bodies in it,
// checked on the Sun, Earth and the Moon. No dependencies:
// node frontend/tests/skyFrames.test.mjs
import assert from 'node:assert/strict'
import { worldDayOf, simDaysOf, globalHour, planetFrame, skyAt, localSolarHour, hiddenPart, starMatrix, horizonFrame } from '../src/skyFrames.js'
import { SUN, AU_KM } from '../src/stars.js'
import { enclosureFor } from '../src/lighting.js'

const DEG = Math.PI / 180
const close = (a, b, tol, msg) => assert.ok(Math.abs(a - b) <= tol, `${msg}: ${a} vs ${b}`)

const sun = { id: 1, kind: 'star', body_type: 'star', ...SUN }
const earth = { id: 2, kind: 'planet', body_type: 'rocky', radius_km: 6371, orbit_km: AU_KM, parent_id: 1, phase: 0, is_main: true }
const moon = { id: 3, kind: 'moon', body_type: 'rocky', radius_km: 1737, orbit_km: 384400, parent_id: 2, phase: 0, inclination: 5.14, ascending_node: 90 }
const bodies = [sun, earth, moon]
const cal = { months_per_year: 12, days_per_month: 30, hours_per_day: 24 }
const opts = { tilt: 23.44, dayHours: 24 }

// ---- Time --------------------------------------------------------------------------------------
assert.equal(worldDayOf({ day: 1, month: 1, year: 1 }, cal), 0)
assert.equal(worldDayOf({ day: 1, month: 1, year: 2 }, cal), 360)
assert.equal(worldDayOf({ day: 30, month: 12, year: -1 }, cal), -1, 'the day before 01-01-0001')
assert.equal(simDaysOf({ day: 1, month: 1, year: 1 }, 0, cal), 0, 'day 0 is 01-01-0001, midnight')
close(simDaysOf({ day: 2, month: 1, year: 1 }, 6, cal), 1.25, 1e-12, 'a day and six hours')
// A 30-hour world day: its days are longer in the solar system's clock.
close(simDaysOf({ day: 2, month: 1, year: 1 }, 0, { ...cal, hours_per_day: 30 }), 1.25, 1e-12, 'a 30-hour day')
close(globalHour(12, 90 * DEG, cal), 6, 1e-12, 'noon a quarter east is six on longitude 0')

// ---- The frame ------------------------------------------------------------------------------------
const frame = planetFrame(bodies, opts)
close(frame.yearDays, 365.25, 1, 'a year')
close(frame.siderealDays * 24, 23.934, 0.01, 'the sidereal day')
close(Math.hypot(...frame.axis), 1, 1e-12, 'a unit axis')

// Midnight on longitude 0 at day 0; noon half a day later.
close(localSolarHour(frame, 0, 0), 0, 0.01, 'midnight at day 0')
close(localSolarHour(frame, 0, 0.5), 12, 0.02, 'noon half a day on')
close(localSolarHour(frame, 90 * DEG, 0), 6, 0.02, 'six in the morning a quarter east')
// (Within the equation of time: the tilt makes sundials run up to a quarter
// of an hour early or late through the year.)
close(localSolarHour(frame, 0, 100), 0, 0.3, 'midnight again a hundred days on')

// The sun at the equator: well below at midnight, high at noon.
const midnight = skyAt(bodies, 0, 0, 0, opts)
const noon = skyAt(bodies, 0, 0, 0.5, opts)
assert.ok(midnight.sun.alt < -60 * DEG, `midnight sun ${midnight.sun.alt / DEG}`)
assert.ok(noon.sun.alt > 60 * DEG, `noon sun ${noon.sun.alt / DEG}`)
close(noon.sun.angularRadius / DEG, 0.266, 0.01, 'the Sun is half a degree across')
close(noon.sun.mag, -26.74, 0.05, "the Sun's brightness")

// Seasons: at 50°N the noon sun stands high at midsummer and low at
// midwinter, the solstices falling where the climate puts them.
const yearDays = frame.yearDays
const solstice = -10 / 365.25
const noonAt = (fraction) => {
  const day = Math.round((solstice + fraction) * yearDays)
  return skyAt(bodies, 50 * DEG, 0, day + 0.5, opts).sun.alt / DEG
}
close(noonAt(1), 90 - 50 - 23.44, 1.5, 'midwinter noon at 50°N')
close(noonAt(0.5), 90 - 50 + 23.44, 1.5, 'midsummer noon at 50°N')

// Stable orbits come round: the noon sun a year on, and ten thousand years
// on, stands where it did (only the leftover part of a day moves it).
const later = (years) => {
  const day = Math.round(years * yearDays + 100)
  return skyAt(bodies, 40 * DEG, 0, day + 0.5, opts).sun.alt / DEG
}
close(later(1), later(0), 1, 'a year on')
assert.ok(Number.isFinite(later(10000)), 'far in the future')
close(later(10000), later(0), 1, 'ten thousand years on')

// ---- The Moon -----------------------------------------------------------------------------------
{
  const m = midnight.bodies.find((b) => b.body.id === 3)
  close((m.angularRadius / DEG) * 2, 0.52, 0.03, 'the Moon is half a degree across')
  // Over a month it waxes and wanes: full when opposite the Sun.
  let fullest = 0
  let darkest = 1
  for (let d = 0; d < 30; d += 0.25) {
    const s = skyAt(bodies, 0, 0, d, opts)
    const mm = s.bodies.find((b) => b.body.id === 3)
    const apart = Math.acos(mm.local[0] * s.sun.local[0] + mm.local[1] * s.sun.local[1] + mm.local[2] * s.sun.local[2])
    if (apart > 175 * DEG) assert.ok(mm.lit > 0.97, 'opposite the Sun: full')
    if (apart < 5 * DEG) assert.ok(mm.lit < 0.03, 'by the Sun: new')
    fullest = Math.max(fullest, mm.lit)
    darkest = Math.min(darkest, mm.lit)
    if (mm.lit > 0.05) assert.ok(mm.mag > -14 && mm.mag < 5, `moon magnitude ${mm.mag}`)
  }
  assert.ok(fullest > 0.98 && darkest < 0.02, 'phases')
}

// A lunar eclipse: the Moon straight behind the planet from the Sun.
{
  const behind = { ...moon, phase: 0, inclination: 0 } // both at phase 0: the Moon straight out beyond Earth
  const s = skyAt([sun, earth, behind], 0, 0, 0, opts)
  const m = s.bodies.find((b) => b.body.id === 3)
  assert.ok(m.eclipsed > 0.99, `eclipsed ${m.eclipsed}`)
  assert.ok(m.direct < 0.01)
}
close(hiddenPart(1, 2, 0), 1, 1e-12, 'all of it')
close(hiddenPart(1, 0.5, 0), 0.25, 1e-12, 'a quarter')
assert.equal(hiddenPart(1, 1, 3), 0)

// Inside the shell, the star in a Dyson sphere: the Moon is lit from all
// round, without phases: by the shell by day, the painted stars by night.
{
  const dyson = { ...sun, dyson: true }
  const shell = enclosureFor({ enclosed: true, main: { day_hours: 24 } })
  const night = skyAt([dyson, earth, moon], 0, 0, 0, { ...opts, enclosure: shell })
  const day = skyAt([dyson, earth, moon], 0, 0, 0.5, { ...opts, enclosure: shell })
  const mn = night.bodies.find((b) => b.body.id === 3)
  const md = day.bodies.find((b) => b.body.id === 3)
  assert.equal(mn.direct, 0)
  assert.ok(mn.even > 0 && md.even > 100 * mn.even, 'dim by night, bright by day')
  assert.ok(md.mag < mn.mag)
  assert.equal(night.sun.shines, false)
  assert.ok(night.shell.glow < 0.01 && day.shell.glow > 0.99)
}

// ---- The stars --------------------------------------------------------------------------------------
{
  // At the north pole the picture's top (its pole) is overhead, at any time.
  for (const days of [0, 0.3, 123.7]) {
    const h = horizonFrame(frame, 90 * DEG, 0, days)
    const m = starMatrix(frame, h)
    close(m[2][2], 1, 1e-9, 'the pole star overhead')
  }
  // At day 0, midnight on longitude 0 at the equator, the picture's middle
  // column (longitude 0) is overhead: the map's globe shows the sky then.
  const m0 = starMatrix(frame, horizonFrame(frame, 0, 0, 0))
  close(m0[2][0], 1, 1e-9, "the picture's middle overhead")
  // The matrix is a rotation.
  for (let i = 0; i < 3; i++) for (let j = 0; j < 3; j++) {
    const d = m0[i][0] * m0[j][0] + m0[i][1] * m0[j][1] + m0[i][2] * m0[j][2]
    close(d, i === j ? 1 : 0, 1e-9, 'orthonormal')
  }
  // Half a sidereal day on, the other side of the picture is overhead.
  const half = starMatrix(frame, horizonFrame(frame, 0, 0, frame.siderealDays / 2))
  close(half[2][0], -1, 1e-6, 'the stars turn round in a sidereal day')
}

console.log('skyFrames: all tests passed')
