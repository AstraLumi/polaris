// What lights and warms a body, kept apart: light (what the eye sees, what
// the pictures draw) and heat (what the climate runs on) need not come from
// the same place. Every part of the app that lights or warms something asks
// here instead of looking at the star itself (docs/SKY_ROADMAP.md, stage 5).
//
// Two sources:
//   the star    light and heat from its place in the sky: directional, so
//               there is day and night, seasons and shadows. A star in a
//               Dyson sphere gives no light, but its heat still comes
//               through the vents (stars.js: luminosity vs heatOutput).
//   a shell     the whole system closed in a sphere whose inside is the
//               sky picture (Solar system → "Inside a shell", stored as
//               `enclosed`): light from every direction at once, the same
//               everywhere on the planet (no time zones, no night side),
//               on one global day and night, the main planet's. By day the
//               shell glows; by night only its painted stars give light.
//               Its light may carry a little heat or none ("cold light");
//               the warmth then comes from the star alone.
//
// Moonlight follows the same rule (moonlightOf): with no shell a moon
// reflects the star, from one side, so it has phases; inside a shell it is
// lit from all round, evenly, with no phases: by the glowing shell by day
// and, at night, only by the shell's painted starlight. A star that still
// shines inside the shell (no Dyson sphere) lights it from one side too.
//
// The enclosure is described as { kind: 'none' } or
//   { kind: 'shell', brightness, color, cycleHours, dayFraction, warmth,
//     nightGlow }
//     brightness   its light on a surface, as a part of the sunlight Earth
//                  gets facing the Sun (1 = 1361 W/m²; Earth's overcast day
//                  is about 0.05, a clear noon about 0.75)
//     color        '#rrggbb', the colour of its light
//     cycleHours   how long its day and night last together; the cycle
//                  starts at midnight, noon is halfway
//     dayFraction  the part of the cycle it is lit
//     warmth       the part of its light that warms (0: cold light)
//     nightGlow    its painted stars' light at night, as a part of its
//                  daylight
// enclosureFor(solar) makes it from the solar system (GET /api/solar).
// No dependencies but stars.js: node frontend/tests/lighting.test.mjs
import { luminosity, heatOutput, starColor, AU_KM } from './stars.js'

export const S_EARTH = 1361 // W/m², sunlight at Earth facing the Sun

export const NO_ENCLOSURE = { kind: 'none' }

export const DEFAULT_SHELL = { kind: 'shell', brightness: 0.6, color: '#fff4e0', cycleHours: 24, dayFraction: 0.5, warmth: 0, nightGlow: 0.002 }

// The enclosure with every value filled in and kept in range.
export function enclosureOf(e) {
  if (!e || e.kind !== 'shell') return NO_ENCLOSURE
  const num = (x, lo, hi, fallback) => (Number.isFinite(x) ? Math.max(lo, Math.min(hi, x)) : fallback)
  return {
    kind: 'shell',
    brightness: num(e.brightness, 0, 10, DEFAULT_SHELL.brightness),
    color: /^#[0-9a-f]{6}$/i.test(e.color ?? '') ? e.color : DEFAULT_SHELL.color,
    cycleHours: num(e.cycleHours, 0.01, 1e6, DEFAULT_SHELL.cycleHours),
    dayFraction: num(e.dayFraction, 0, 1, DEFAULT_SHELL.dayFraction),
    warmth: num(e.warmth, 0, 1, DEFAULT_SHELL.warmth),
    nightGlow: num(e.nightGlow, 0, 1, DEFAULT_SHELL.nightGlow),
  }
}

// The enclosure of a solar system as GET /api/solar gives it: a shell on
// the main planet's day when it is enclosed, else none.
export function enclosureFor(solar) {
  if (!solar?.enclosed) return NO_ENCLOSURE
  return enclosureOf({ ...DEFAULT_SHELL, cycleHours: solar.main?.day_hours || DEFAULT_SHELL.cycleHours })
}

const smooth = (x) => x * x * (3 - 2 * x)

// How far into its day the shell is, 0 (night) to 1 (day): `hours` since a
// midnight. Dawn and dusk take a twentieth of the cycle each, centred on
// the day's edges (6:00 and 18:00 on a 24-hour day lit half the time).
export function shellGlow(e, hours) {
  const s = enclosureOf(e)
  if (s.kind !== 'shell' || s.dayFraction <= 0) return 0
  if (s.dayFraction >= 1) return 1
  const t = (((hours / s.cycleHours) % 1) + 1) % 1
  const ramp = Math.min(0.05, s.dayFraction, 1 - s.dayFraction)
  const fromNoon = Math.abs(t - 0.5)
  const edge = s.dayFraction / 2
  if (fromNoon <= edge - ramp / 2) return 1
  if (fromNoon >= edge + ramp / 2) return 0
  return smooth((edge + ramp / 2 - fromNoon) / ramp)
}

// The shell's light at a moment, relative to Earth's sunlight: its glow by
// day, its painted stars' by night.
export function shellLight(e, hours) {
  const s = enclosureOf(e)
  if (s.kind !== 'shell') return 0
  const glow = shellGlow(s, hours)
  return s.brightness * (glow + (1 - glow) * s.nightGlow)
}

// The shell's glow over a whole cycle (what the climate's day-long means
// see), not counting the painted stars: the ramps balance out.
export function shellMeanGlow(e) {
  const s = enclosureOf(e)
  return s.kind === 'shell' ? s.dayFraction : 0
}

// The star a body goes round (through its planet, for a moon), and how far
// out the body's family is from it: the orbit of the planet in line.
function starAndDistance(bodies, body) {
  const byId = new Map(bodies.map((b) => [b.id, b]))
  let at = body
  while (at) {
    const parent = byId.get(at.parent_id)
    if (!parent) return { star: at.kind === 'star' ? at : null, km: 0 }
    if (parent.kind === 'star') return { star: parent, km: at.orbit_km > 0 ? at.orbit_km : 0 }
    at = parent
  }
  return { star: null, km: 0 }
}

// What lights and warms `body` among `bodies` (the solar system's), in the
// enclosure, at `hours` into the shell's cycle (for the light right now).
// Every amount is relative to Earth's sunlight (S_EARTH = 1):
//   light.star      the star's light arriving, facing it (0 in a Dyson
//                   sphere, or with no star)
//   light.even      the shell's light now, the same from every direction
//   light.color     the star's light's colour; light.evenColor the shell's
//   heat.star       the star's heat arriving, facing it (directional: it
//                   makes seasons and the day's warmth)
//   heat.even       heat from all round, as a mean over the day (the
//                   shell's warm part)
//   star            the star itself, for where light and heat come from
export function lightingOf(bodies, body, { enclosure = null, hours = 0 } = {}) {
  const { star, km } = starAndDistance(bodies ?? [], body ?? {})
  const shell = enclosureOf(enclosure)
  const reach = km > 0 ? (AU_KM / km) ** 2 : 0
  return {
    star,
    light: {
      star: star ? luminosity(star) * reach : 0,
      color: star ? starColor(star) : '#ffffff',
      even: shellLight(shell, hours),
      evenColor: shell.kind === 'shell' ? shell.color : '#ffffff',
    },
    heat: {
      star: star ? heatOutput(star) * reach : 0,
      even: shell.kind === 'shell' ? shell.brightness * shell.warmth * shellMeanGlow(shell) : 0,
    },
  }
}

// The main planet's (the map's) lighting, from GET /api/solar.
export function mainLighting(solar, options) {
  const bodies = solar?.bodies ?? []
  const main = bodies.find((b) => b.is_main)
  return main ? lightingOf(bodies, main, { enclosure: enclosureFor(solar), ...options }) : null
}

// How a moon is lit, for the sky seen from the planet (the sky view, its
// moonlight at night). Relative to Earth's sunlight:
//   star     light from the star's side: it makes phases (none when the
//            star is hidden in a Dyson sphere)
//   even     light from all round: the whole disc evenly, no phases (the
//            shell by day, its painted starlight by night)
//   phases   whether it shows phases at all
//   own      the light of a moon that shines on its own (`glows`), counted
//            in `even` too
//   source   'star', 'own' (it glows, with no star lighting it), 'shell'
//            (day), 'starlight' (the shell's night) or 'none'
export function moonlightOf(bodies, moon, { enclosure = null, hours = 0 } = {}) {
  const lit = lightingOf(bodies, moon, { enclosure, hours })
  const shell = enclosureOf(enclosure)
  const own = moon?.glows ? MOON_GLOW : 0
  let source = 'none'
  if (lit.light.star > 0) source = 'star'
  else if (own > 0) source = 'own'
  else if (shell.kind === 'shell') source = shellGlow(shell, hours) > 0.5 ? 'shell' : 'starlight'
  return { star: lit.light.star, even: lit.light.even + own, own, phases: lit.light.star > 0, source }
}

// How bright a moon that shines on its own is, as if lit all over by most
// of Earth's sunlight: a full moon's brightness, whatever lights it. A star
// still lighting one side shows its phase on top.
export const MOON_GLOW = 0.7
