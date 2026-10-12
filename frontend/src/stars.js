// Stars: the classes a star can be, the values real stars of each class
// have, and what follows from a star's temperature and size (how bright it
// is, its colour, where its habitable zone lies). Part of the solar
// system's physics, with orbits.js; nothing here draws.
// No dependencies: node frontend/tests/orbits.test.mjs

export const SUN = { radius_km: 696000, mass_kg: 1.989e30, temperature_k: 5772 }
const SIGMA = 5.670374419e-8 // W m⁻² K⁻⁴
const SUN_LUMINOSITY = 3.828e26 // W
const C = 299792458 // m/s
const G = 6.674e-11
export const AU_KM = 1.496e8

// Each class with the range science gives its temperature (K), radius (km)
// and mass (kg), and a typical star of it. Broad strokes: real stars spill
// over these edges, but a star well outside them isn't one of its class.
// A black hole has no surface to be hot, and its size follows its mass
// (radius: null, see schwarzschildRadius).
export const STAR_CLASSES = {
  yellow_dwarf: {
    temperature_k: [5300, 6000, 5772],
    radius_km: [0.85 * SUN.radius_km, 1.15 * SUN.radius_km, SUN.radius_km],
    mass_kg: [0.84 * SUN.mass_kg, 1.15 * SUN.mass_kg, SUN.mass_kg],
  },
  red_giant: {
    temperature_k: [3000, 5000, 4000],
    radius_km: [10 * SUN.radius_km, 200 * SUN.radius_km, 50 * SUN.radius_km],
    mass_kg: [0.3 * SUN.mass_kg, 8 * SUN.mass_kg, 1.2 * SUN.mass_kg],
  },
  white_dwarf: {
    temperature_k: [4000, 150000, 25000],
    radius_km: [5000, 14000, 7000],
    mass_kg: [0.17 * SUN.mass_kg, 1.33 * SUN.mass_kg, 0.6 * SUN.mass_kg],
  },
  brown_dwarf: {
    temperature_k: [250, 2800, 1300],
    radius_km: [42000, 105000, 70000],
    mass_kg: [0.012 * SUN.mass_kg, 0.076 * SUN.mass_kg, 0.04 * SUN.mass_kg],
  },
  neutron_star: {
    temperature_k: [100000, 1000000, 600000],
    radius_km: [10, 15, 12],
    mass_kg: [1.1 * SUN.mass_kg, 2.3 * SUN.mass_kg, 1.4 * SUN.mass_kg],
  },
  black_hole: {
    temperature_k: null,
    radius_km: null,
    mass_kg: [3 * SUN.mass_kg, 100 * SUN.mass_kg, 10 * SUN.mass_kg],
  },
}

export const classOf = (star) => STAR_CLASSES[star.star_class] ?? STAR_CLASSES.yellow_dwarf

// The values a star can have, and which of them are typed freely ("Reality
// Following" unticked): the bits match backend/solar.go.
export const STAR_VALUES = [
  { key: 'temperature_k', bit: 1 },
  { key: 'radius_km', bit: 2 },
  { key: 'mass_kg', bit: 4 },
]
export const isFree = (star, key) => !!((star.free_values || 0) & STAR_VALUES.find((v) => v.key === key).bit)

// The event horizon of a mass, in km.
export const schwarzschildRadius = (massKg) => (2 * G * massKg) / C ** 2 / 1000

// A star made to its class: typical values, then whatever it already has
// that is free or within range. Called when the class changes.
export function fitToClass(star) {
  const cls = classOf(star)
  const out = { ...star }
  for (const { key } of STAR_VALUES) {
    const range = cls[key]
    if (isFree(star, key)) continue
    if (!range) out[key] = key === 'temperature_k' ? 0 : out[key]
    else if (!(out[key] >= range[0] && out[key] <= range[1])) out[key] = range[2]
  }
  if (star.star_class === 'black_hole' && !isFree(star, 'radius_km')) out.radius_km = schwarzschildRadius(out.mass_kg)
  return out
}

// How bright a star is, in Suns: its surface times its temperature to the
// fourth (Stefan–Boltzmann). A black hole gives no light of its own.
export function luminosity(star) {
  // Closed in a Dyson sphere, none of it gets out.
  if (star.dyson) return 0
  const t = star.temperature_k || 0
  if (!(t > 0) || !(star.radius_km > 0)) return 0
  const r = star.radius_km * 1000
  return (4 * Math.PI * r * r * SIGMA * t ** 4) / SUN_LUMINOSITY
}

// The habitable ("Goldilocks") zone: where a planet like Earth gets the
// light to keep water liquid, in km from the star (Kasting's conservative
// edges, 1.1 and 0.53 of the light Earth gets). Null with no light.
export function habitableZone(star) {
  const l = luminosity(star)
  if (!(l > 0)) return null
  return { inner_km: Math.sqrt(l / 1.1) * AU_KM, outer_km: Math.sqrt(l / 0.53) * AU_KM }
}

// The colour of a glowing body at a temperature (a blackbody, roughly), as
// '#rrggbb'. Cool stars are red, the Sun white-yellow, hot ones blue.
export function starColor(star) {
  if (star.star_class === 'black_hole') return '#05060a'
  const k = Math.max(1000, Math.min(40000, star.temperature_k || SUN.temperature_k)) / 100
  const clamp = (x) => Math.max(0, Math.min(255, Math.round(x)))
  const r = k <= 66 ? 255 : 329.698727446 * (k - 60) ** -0.1332047592
  const g = k <= 66 ? 99.4708025861 * Math.log(k) - 161.1195681661 : 288.1221695283 * (k - 60) ** -0.0755148492
  const b = k >= 66 ? 255 : k <= 19 ? 0 : 138.5177312231 * Math.log(k - 10) - 305.0447927307
  return '#' + [r, g, b].map((x) => clamp(x).toString(16).padStart(2, '0')).join('')
}
