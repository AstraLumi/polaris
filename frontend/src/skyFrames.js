// The sky seen from a place on the main planet at a moment: time, the
// planet's axis and spin, the local horizon, and where every body and star
// stands in it, true to scale (docs/SKY_ROADMAP.md, stages 1 and 3).
//
// Time. The solar system's day 0 is the calendar's 01-01-0001 at midnight
// on longitude 0. A moment is a calendar date and an hour of the world's
// day; the solar system counts Earth days (86 400 s), and a world day is
// the calendar's `hours_per_day` long. Orbits are Kepler orbits, worked out
// for the moment directly from day 0 (no step-by-step simulation, so a date
// ten thousand years on costs the same as tomorrow, and stable orbits
// simply come round to the same places: the sky of a date is the sky of
// day 0 moved on). Live gravity is for watching what-ifs and is not used.
//
// The axis. The main planet's north pole leans away from its star by the
// axial tilt at the northern winter solstice, which falls just before each
// year begins (10/365 of a year, as Earth's does before 1 January; the
// climate uses the same rule, climateModel.js).
//
// The spin. At day 0, midnight on longitude 0: longitude 0 faces straight
// away from the star. The planet turns once a sidereal day (a little
// shorter than its solar day, as it moves along its orbit), so noon and
// midnight come round every `hours_per_day` hours.
//
// The stars. The sky picture is fixed to the stars, in the planet's
// equatorial frame as it was at day 0, midnight on longitude 0: its top
// above the north pole, its middle column above longitude 0 (the format of
// backend/cmd/skygen). So the map's globe shows the sky as it stood then,
// and from then on the planet turns under it.
//
// Directions only leave this module: everything is worked out in double
// precision, in km, and the sky view gets unit vectors and angles.
// No dependencies but orbits.js, stars.js and lighting.js:
// node frontend/tests/skyFrames.test.mjs
import { compileSystem, orbitFrame, keplerOrbit } from './orbits.js'
import { luminosity, starColor, AU_KM } from './stars.js'
import { moonlightOf, shellGlow, enclosureOf } from './lighting.js'

const DEG = Math.PI / 180
const SOLSTICE_BEFORE_YEAR = 10 / 365.25 // of a year, as climateModel.js

// ---- Vectors --------------------------------------------------------------------------------
const dot = (a, b) => a[0] * b[0] + a[1] * b[1] + a[2] * b[2]
const sub = (a, b) => [a[0] - b[0], a[1] - b[1], a[2] - b[2]]
const add = (a, b) => [a[0] + b[0], a[1] + b[1], a[2] + b[2]]
const scale = (a, k) => [a[0] * k, a[1] * k, a[2] * k]
const cross = (a, b) => [a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0]]
const len = (a) => Math.hypot(a[0], a[1], a[2])
const unit = (a) => scale(a, 1 / (len(a) || 1))
// a with its part along unit b taken out
const flatten = (a, b) => sub(a, scale(b, dot(a, b)))

// ---- Time -----------------------------------------------------------------------------------
// World days since 01-01-0001 for a date ({ day, month, year }, no year 0)
// in the story's calendar.
export function worldDayOf({ day, month, year }, cal) {
  const yearIndex = year > 0 ? year - 1 : year
  return (yearIndex * cal.months_per_year + (month - 1)) * cal.days_per_month + (day - 1)
}

// The solar system's clock (Earth days since day 0) at `hour` (0 to the
// world's hours per day, on longitude 0's clock) of a date.
export function simDaysOf(date, hour, cal) {
  const hpd = cal.hours_per_day || 24
  return (worldDayOf(date, cal) + hour / hpd) * (hpd / 24)
}

// The world's hour on longitude 0 when it is `localHour` at longitude
// `lon` (radians, east positive): a place east of it sees the day earlier.
export function globalHour(localHour, lon, cal) {
  const hpd = cal.hours_per_day || 24
  return localHour - (lon / (2 * Math.PI)) * hpd
}

// ---- The main planet's frame -----------------------------------------------------------------
// bodies: the solar system (GET /api/solar's bodies; the main planet's
// orbit radius is already its year's); tilt in degrees; dayHours the
// calendar's. Null when there is no main planet going round a star.
export function planetFrame(bodies, { tilt = 23.44, dayHours = 24 } = {}) {
  const system = compileSystem(bodies, keplerOrbit)
  const mainIndex = system.list.findIndex((b) => b.is_main)
  if (mainIndex < 0) return null
  const main = system.list[mainIndex]
  const parentIndex = system.parent[mainIndex]
  if (parentIndex < 0) return null
  // The star it goes round, through its planet if it is a moon.
  let starIndex = parentIndex
  while (system.parent[starIndex] >= 0) starIndex = system.parent[starIndex]
  const star = system.list[starIndex]
  // The planet whose orbit round the star makes the year.
  let yearIndex = mainIndex
  while (system.parent[yearIndex] !== starIndex) yearIndex = system.parent[yearIndex]
  const yearBody = system.list[yearIndex]
  const yearDays = keplerOrbit.period(yearBody, star)

  const at = (days) => {
    const pos = system.absolute(days)
    return {
      planet: [pos[mainIndex * 3], pos[mainIndex * 3 + 1], pos[mainIndex * 3 + 2]],
      star: [pos[starIndex * 3], pos[starIndex * 3 + 1], pos[starIndex * 3 + 2]],
      all: pos,
    }
  }
  const { P, Q } = orbitFrame(main)
  const normal = unit(cross(P, Q))
  // North leans away from the star at the northern winter solstice.
  const s = at(-SOLSTICE_BEFORE_YEAR * yearDays)
  const away = unit(flatten(sub(s.planet, s.star), normal))
  const e = tilt * DEG
  const axis = unit(add(scale(normal, Math.cos(e)), scale(away, Math.sin(e))))
  // Longitude 0 faces away from the star at day 0.
  const z = at(0)
  let x0 = flatten(sub(z.planet, z.star), axis)
  if (len(x0) < 1e-9) x0 = flatten(P, axis)
  x0 = unit(x0)
  const y0 = cross(axis, x0)
  // Prograde spin: the sidereal day is the solar day less a turn a year.
  const solarDays = (dayHours || 24) / 24
  const siderealDays = 1 / (1 / solarDays + 1 / yearDays)
  return { system, main, mainIndex, star, starIndex, yearDays, siderealDays, solarDays, axis, x0, y0, at }
}

// Where longitude 0 points at `days` (its meridian turned by the spin).
function meridian(frame, days, lon) {
  const a = (2 * Math.PI * days) / frame.siderealDays + lon
  return add(scale(frame.x0, Math.cos(a)), scale(frame.y0, Math.sin(a)))
}

// The local horizon frame at latitude/longitude (radians) at `days`:
// up (the zenith), north and east, as unit vectors in the solar system's
// frame.
export function horizonFrame(frame, lat, lon, days) {
  const m = meridian(frame, days, lon)
  const up = add(scale(m, Math.cos(lat)), scale(frame.axis, Math.sin(lat)))
  let east = cross(frame.axis, up)
  if (len(east) < 1e-9) east = cross(frame.axis, frame.x0) // at a pole
  east = unit(east)
  const north = cross(up, east)
  return { up, north, east }
}

// A direction's altitude and azimuth (radians; azimuth 0 north, π/2 east)
// in a horizon frame, and the same direction in local coordinates
// [east, north, up].
export function toHorizon(h, dir) {
  const local = [dot(dir, h.east), dot(dir, h.north), dot(dir, h.up)]
  return { alt: Math.asin(Math.max(-1, Math.min(1, local[2]))), az: Math.atan2(local[0], local[1]), local }
}

// The matrix (rows, 3×3) that turns a direction in the sky picture's frame
// (x above longitude 0 at day 0, z the north pole) into local [east,
// north, up]: the stars' place in this sky.
export function starMatrix(frame, h) {
  const rows = [h.east, h.north, h.up]
  return rows.map((r) => [dot(r, frame.x0), dot(r, frame.y0), dot(r, frame.axis)])
}

// ---- Light on a body ---------------------------------------------------------------------------
// How much of the star's disc (angular radius a) is hidden by a disc of
// angular radius b whose centre is c away (all radians): a part, 0 to 1.
export function hiddenPart(a, b, c) {
  if (!(a > 0)) return 0
  if (c >= a + b) return 0
  if (c <= Math.abs(a - b)) return Math.min(1, (Math.min(a, b) / a) ** 2)
  const a2 = a * a
  const b2 = b * b
  const x = Math.max(-1, Math.min(1, (c * c + a2 - b2) / (2 * c * a)))
  const y = Math.max(-1, Math.min(1, (c * c + b2 - a2) / (2 * c * b)))
  const area = a2 * Math.acos(x) + b2 * Math.acos(y) - 0.5 * Math.sqrt(Math.max((-c + a + b) * (c + a - b) * (c - a + b) * (c + a + b), 0))
  return Math.min(1, area / (Math.PI * a2))
}

// How much reflected light a sphere sends back at phase angle alpha (0 full,
// π new), Lambert's law, 1 at full.
const lambert = (alpha) => (Math.sin(alpha) + (Math.PI - alpha) * Math.cos(alpha)) / Math.PI

// How much light a body's surface sends back.
const ALBEDO = { rocky: 0.12, ocean: 0.3, gas: 0.5 }

// ---- The sky --------------------------------------------------------------------------------------
// Everything seen from latitude/longitude (radians) on the main planet at
// `days`. bodies: the solar system's; options: tilt, dayHours, enclosure
// (lighting.js; the shell's day is one clock for the whole planet, so it
// is read at the global hour). Answers null without a main planet, else:
//   horizon     the local frame (up, north, east)
//   stars       the matrix for the sky picture's directions (starMatrix)
//   sun         { alt, az, local, angularRadius, mag, color, shines } — the
//               star (shines false inside a Dyson sphere)
//   bodies      every other body: { body, alt, az, local, distanceKm,
//               angularRadius (radians), lit (0 to 1 of the disc, from the
//               star's side), litFrom (local unit vector towards the
//               star, as seen at the body), even (light from all round, the
//               shell's), eclipsed (0 to 1 of the star hidden from it),
//               mag (apparent magnitude) }, brightest first
//   shell       { glow: 0 night … 1 day } inside a shell, else null
export function skyAt(bodies, lat, lon, days, { tilt = 23.44, dayHours = 24, enclosure = null } = {}) {
  const frame = planetFrame(bodies, { tilt, dayHours })
  if (!frame) return null
  const h = horizonFrame(frame, lat, lon, days)
  const { all } = frame.at(days)
  const posOf = (i) => [all[i * 3], all[i * 3 + 1], all[i * 3 + 2]]
  const planetPos = posOf(frame.mainIndex)
  const observer = add(planetPos, scale(h.up, frame.main.radius_km))
  const starPos = posOf(frame.starIndex)
  const star = frame.star
  const shell = enclosureOf(enclosure)
  const hours = days * 24

  const towards = (p) => {
    const v = sub(p, observer)
    const d = len(v)
    return { dir: scale(v, 1 / d), d }
  }
  const s = towards(starPos)
  const sunH = toHorizon(h, s.dir)
  const L = luminosity(star)
  const sunAngular = Math.asin(Math.min(1, star.radius_km / s.d))
  const sun = {
    ...sunH,
    dir: s.dir,
    distanceKm: s.d,
    angularRadius: sunAngular,
    // Earth's Sun from Earth is −26.74.
    mag: L > 0 ? -26.74 - 2.5 * Math.log10(L / (s.d / AU_KM) ** 2) : Infinity,
    color: starColor(star),
    shines: L > 0,
  }

  const others = []
  frame.system.list.forEach((b, i) => {
    if (i === frame.mainIndex || i === frame.starIndex) return
    const p = posOf(i)
    const t = towards(p)
    const where = toHorizon(h, t.dir)
    const angularRadius = Math.asin(Math.min(1, b.radius_km / t.d))
    // Seen from the body: the star, and how much of it something hides.
    const toStar = sub(starPos, p)
    const dStar = len(toStar)
    const starDir = scale(toStar, 1 / dStar)
    const a = Math.asin(Math.min(1, star.radius_km / dStar))
    let eclipsed = 0
    frame.system.list.forEach((o, j) => {
      if (j === i || j === frame.starIndex) return
      const v = sub(posOf(j), p)
      const dv = len(v)
      if (dv >= dStar || dot(v, starDir) <= 0) return
      const bAng = Math.asin(Math.min(1, o.radius_km / dv))
      const c = Math.acos(Math.max(-1, Math.min(1, dot(scale(v, 1 / dv), starDir))))
      eclipsed = Math.max(eclipsed, hiddenPart(a, bAng, c))
    })
    // The phase: the angle at the body between the star and us.
    const alpha = Math.acos(Math.max(-1, Math.min(1, dot(starDir, scale(t.dir, -1)))))
    const lightHere = moonlightOf(bodies, b, { enclosure: shell, hours })
    const direct = lightHere.star * (1 - eclipsed)
    const albedo = ALBEDO[b.body_type] ?? 0.15
    const ratio = (b.radius_km / t.d) ** 2
    const flux = albedo * ratio * (direct * lambert(alpha) + lightHere.even)
    others.push({
      body: b,
      ...where,
      dir: t.dir,
      distanceKm: t.d,
      angularRadius,
      lit: (1 + Math.cos(alpha)) / 2,
      litFrom: toHorizon(h, starDir).local,
      direct,
      even: lightHere.even,
      eclipsed,
      mag: flux > 0 ? -26.74 - 2.5 * Math.log10(flux) : Infinity,
    })
  })
  others.sort((x, y) => x.mag - y.mag)

  return {
    horizon: h,
    stars: starMatrix(frame, h),
    sun,
    bodies: others,
    shell: shell.kind === 'shell' ? { glow: shellGlow(shell, hours) } : null,
    frame,
  }
}

// The local solar time (hours, 0 to the day's length) at a longitude at
// `days`: where the star stands round the axis from the place's meridian
// (midnight when it is behind).
export function localSolarHour(frame, lon, days) {
  const { planet, star } = frame.at(days)
  const s = unit(flatten(sub(star, planet), frame.axis))
  const m = meridian(frame, days, lon)
  const east = cross(frame.axis, m)
  const angle = Math.atan2(dot(s, east), dot(s, m)) // 0 when the star is overhead the meridian
  const hours = frame.solarDays * 24
  return ((((Math.PI - angle) / (2 * Math.PI)) * hours) % hours + hours) % hours
}
