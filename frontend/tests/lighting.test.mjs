// What lights and warms a body (src/lighting.js): the star's light and heat
// kept apart, a Dyson sphere's vents, and a glowing shell round the system.
// No dependencies: node frontend/tests/lighting.test.mjs
import assert from 'node:assert/strict'
import { lightingOf, mainLighting, moonlightOf, enclosureFor, shellGlow, shellLight, shellMeanGlow, enclosureOf, DEFAULT_SHELL } from '../src/lighting.js'
import { SUN, AU_KM, luminosity, heatOutput, habitableZone } from '../src/stars.js'

const close = (a, b, tol, msg) => assert.ok(Math.abs(a - b) <= tol, `${msg}: ${a} vs ${b}`)

const sun = { id: 1, kind: 'star', ...SUN }
const earth = { id: 2, kind: 'planet', parent_id: 1, orbit_km: AU_KM, is_main: true }
const moon = { id: 3, kind: 'moon', parent_id: 2, orbit_km: 384400 }
const far = { id: 4, kind: 'planet', parent_id: 1, orbit_km: 2 * AU_KM }

// A plain star: light and heat are the same, and fall with the distance
// squared; a moon gets its planet's.
{
  const at = lightingOf([sun, earth, moon, far], earth)
  close(at.light.star, 1, 0.01, 'Earth gets one Earth of light')
  close(at.heat.star, 1, 0.01, 'and one of heat')
  assert.equal(at.light.even, 0)
  assert.equal(at.heat.even, 0)
  close(lightingOf([sun, earth, moon], moon).heat.star, 1, 0.01, 'the Moon is as far as Earth')
  close(lightingOf([sun, far], far).heat.star, 0.25, 0.01, 'twice as far, a quarter')
  assert.equal(mainLighting({ bodies: [sun, earth] }).star, sun)
  assert.equal(mainLighting({ bodies: [sun] }), null)
}

// A Dyson sphere: no light gets out, but the heat still does (all of it,
// unless its vents let out less), and the habitable zone stays.
{
  const dyson = { ...sun, dyson: true }
  assert.equal(luminosity(dyson), 0)
  close(heatOutput(dyson), 1, 0.01, 'all its heat by default')
  close(heatOutput({ ...dyson, vents: 0.25 }), 0.25, 0.01, 'a quarter through narrow vents')
  close(heatOutput({ ...sun, vents: 0.25 }), 1, 0.01, 'vents mean nothing without a sphere')
  const at = lightingOf([dyson, earth], earth)
  assert.equal(at.light.star, 0)
  close(at.heat.star, 1, 0.01, 'the planet stays warm in the dark')
  assert.ok(habitableZone(dyson), 'a habitable zone of heat')
}

// A black hole gives neither.
{
  const hole = { id: 1, kind: 'star', star_class: 'black_hole', temperature_k: 0, radius_km: 30 }
  const at = lightingOf([hole, earth], earth)
  assert.equal(at.light.star, 0)
  assert.equal(at.heat.star, 0)
}

// The shell: lit for part of its cycle with soft dawns and dusks, the same
// at every hour of every place; its light warms only as much as asked.
{
  const shell = { ...DEFAULT_SHELL, cycleHours: 24, dayFraction: 0.5 }
  assert.equal(shellGlow(shell, 12), 1, 'noon')
  assert.equal(shellGlow(shell, 0), 0, 'midnight')
  assert.equal(shellGlow(shell, 12 + 24 * 7), 1, 'every day alike')
  const dawn = shellGlow(shell, 5.6)
  assert.ok(dawn > 0 && dawn < 1, 'a soft dawn')
  assert.ok(shellGlow(shell, 6.7) === 1 && shellGlow(shell, 17.3) === 1 && shellGlow(shell, 18.7) === 0, 'six to six')
  close(shellLight(shell, 0), shell.brightness * shell.nightGlow, 1e-12, 'at night, only the painted stars')
  close(shellMeanGlow(shell), 0.5, 1e-9, 'lit half the cycle')
  assert.equal(shellGlow({ ...shell, dayFraction: 1 }, 18), 1, 'never dark')
  assert.equal(shellGlow({ kind: 'none' }, 6), 0)

  const dyson = { ...sun, dyson: true }
  const at = lightingOf([dyson, earth], earth, { enclosure: shell, hours: 12 })
  close(at.light.even, shell.brightness, 1e-9, 'the shell lights the planet')
  assert.equal(at.light.star, 0, 'the star does not')
  close(at.heat.star, 1, 0.01, 'the star still warms it')
  assert.equal(at.heat.even, 0, 'cold light')
  const warm = lightingOf([dyson, earth], earth, { enclosure: { ...shell, warmth: 0.5 } })
  close(warm.heat.even, shell.brightness * 0.5 * 0.5, 1e-9, 'warm light, a day-long mean')
}

// The shell is the solar system's switch, on the main planet's day.
{
  assert.equal(enclosureFor({ enclosed: false }).kind, 'none')
  const e = enclosureFor({ enclosed: true, main: { day_hours: 30 } })
  assert.equal(e.kind, 'shell')
  assert.equal(e.cycleHours, 30)
  const dyson = { ...sun, dyson: true }
  const lit = mainLighting({ enclosed: true, main: { day_hours: 24 }, bodies: [dyson, earth] }, { hours: 12 })
  assert.ok(lit.light.even > 0.5 && lit.light.star === 0, 'lit by the shell at noon')
}

// Moonlight: the star's, with phases, in an open system; inside the shell,
// from all round with no phases, the shell's by day and only the painted
// starlight by night.
{
  const open = moonlightOf([sun, earth, moon], moon)
  assert.equal(open.source, 'star')
  assert.ok(open.phases && open.star > 0.9)
  assert.equal(open.even, 0)

  const dyson = { ...sun, dyson: true }
  const shell = enclosureFor({ enclosed: true, main: { day_hours: 24 } })
  const day = moonlightOf([dyson, earth, moon], moon, { enclosure: shell, hours: 12 })
  assert.equal(day.source, 'shell')
  assert.ok(!day.phases && day.even > 0.5)
  const night = moonlightOf([dyson, earth, moon], moon, { enclosure: shell, hours: 0 })
  assert.equal(night.source, 'starlight')
  assert.ok(!night.phases)
  close(night.even, shell.brightness * shell.nightGlow, 1e-12, 'only the painted stars at night')
  assert.ok(night.even < day.even / 100)

  // A star still shining inside the shell lights the moon from its side too.
  const both = moonlightOf([sun, earth, moon], moon, { enclosure: shell, hours: 0 })
  assert.equal(both.source, 'star')
  assert.ok(both.phases && both.even > 0)
}

// Bad values are kept in range; anything but a shell is no enclosure.
{
  const e = enclosureOf({ kind: 'shell', brightness: -3, dayFraction: 7, color: 'red', cycleHours: 0 })
  assert.equal(e.brightness, 0)
  assert.equal(e.dayFraction, 1)
  assert.equal(e.color, DEFAULT_SHELL.color)
  assert.ok(e.cycleHours > 0)
  assert.equal(enclosureOf({ kind: 'cube' }).kind, 'none')
  assert.equal(enclosureOf(null).kind, 'none')
}

console.log('lighting: all tests passed')
