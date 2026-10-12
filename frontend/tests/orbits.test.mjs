// The solar system's physics (src/orbits.js) and its drawing scale
// (src/solarScale.js). No dependencies: node frontend/tests/orbits.test.mjs
import assert from 'node:assert/strict'
import { massOf, surfaceGravity, circularOrbit, positionsAt, hillRadius, moonProblem, systemTree, orbitLimits, orbitProblem } from '../src/orbits.js'
import { sceneRadius, sceneDistance, orbitForSceneDistance } from '../src/solarScale.js'
import { SUN, AU_KM, luminosity, habitableZone, schwarzschildRadius, fitToClass, starColor } from '../src/stars.js'

const close = (a, b, rel, msg) => assert.ok(Math.abs(a - b) <= Math.abs(b) * rel, `${msg}: ${a} vs ${b}`)

const sun = { id: 1, kind: 'star', body_type: 'star', radius_km: 696000 }
const earth = { id: 2, kind: 'planet', body_type: 'rocky', radius_km: 6371, orbit_km: 1.496e8, parent_id: 1, phase: 0 }
const moon = { id: 3, kind: 'moon', body_type: 'rocky', radius_km: 1737, orbit_km: 384400, parent_id: 2, phase: 90 }

// Close to the real thing with one density per kind.
close(massOf(sun), 1.989e30, 0.01, "the Sun's mass")
close(massOf(earth), 5.97e24, 0.01, "Earth's mass")
close(surfaceGravity(earth), 9.81, 0.01, "Earth's gravity")
close(circularOrbit.period(earth, sun), 365.25, 0.01, "Earth's year")
// The Moon's real density is lower than Earth's; one rocky density makes its
// month a little short, but in range.
close(circularOrbit.period(moon, earth), 27.3, 0.1, "the Moon's month")

// Positions add up the tree: the Moon starts a quarter turn round Earth.
const pos = positionsAt([sun, earth, moon], 0)
assert.deepEqual(pos.get(1), [0, 0, 0])
close(pos.get(2)[0], 1.496e8, 1e-9, 'Earth at its phase')
close(pos.get(3)[1], 384400, 1e-9, 'the Moon a quarter turn on')
const half = positionsAt([sun, earth, moon], circularOrbit.period(earth, sun) / 2)
close(half.get(2)[0], -1.496e8, 1e-6, 'half a year later')

assert.ok(hillRadius(earth, sun) > 1.4e6 && hillRadius(earth, sun) < 1.6e6, "Earth's Hill sphere")
assert.equal(moonProblem(moon, earth, sun), '')
assert.equal(moonProblem({ ...moon, orbit_km: 5000 }, earth, sun), 'inside')
assert.equal(moonProblem({ ...moon, orbit_km: 1.2e6 }, earth, sun), 'loose')
assert.equal(systemTree([sun, earth, moon]).star, sun)
assert.deepEqual(systemTree([sun, earth, moon]).childrenOf(2), [moon])

// Drawing scale: bigger stays bigger, farther stays farther, and a moon is
// always drawn outside its planet.
assert.ok(sceneRadius(sun) > sceneRadius(earth) && sceneRadius(earth) > sceneRadius(moon))
assert.ok(sceneDistance(7.8e8) > sceneDistance(1.5e8))
assert.ok(sceneDistance(384400, earth, moon) > sceneRadius(earth) + sceneRadius(moon))
// Dragging in the editor turns a drawn distance back into km.
close(orbitForSceneDistance(sceneDistance(1.496e8, sun, earth), sun, earth), 1.496e8, 1e-9, 'round trip')
assert.ok(orbitForSceneDistance(0, earth, moon) > earth.radius_km + moon.radius_km, 'never inside its planet')
// Stars (src/stars.js): the Sun's numbers come back, black holes are
// dark and their size follows their mass.
assert.ok(Math.abs(luminosity({ ...SUN }) - 1) < 0.01, "the Sun's light")
const hz = habitableZone({ ...SUN })
assert.ok(hz.inner_km / AU_KM > 0.9 && hz.inner_km / AU_KM < 1 && hz.outer_km / AU_KM > 1.3 && hz.outer_km / AU_KM < 1.4, "the Sun's habitable zone")
assert.equal(habitableZone({ star_class: 'black_hole', temperature_k: 0, radius_km: 30 }), null)
close(schwarzschildRadius(SUN.mass_kg), 2.95, 0.01, 'the Sun as a black hole')
const bh = fitToClass({ star_class: 'black_hole', radius_km: 696000, temperature_k: 5772, mass_kg: SUN.mass_kg, free_values: 0 })
assert.equal(bh.temperature_k, 0)
close(bh.mass_kg, 10 * SUN.mass_kg, 1e-9, 'a typical black hole')
close(bh.radius_km, schwarzschildRadius(bh.mass_kg), 1e-9, 'its horizon')
const free = fitToClass({ star_class: 'red_giant', radius_km: 5, temperature_k: 5772, mass_kg: SUN.mass_kg, free_values: 2 })
assert.equal(free.radius_km, 5, 'a free value stays as typed')
assert.equal(starColor({ temperature_k: 3000 }).slice(1, 3), 'ff', 'cool stars are red')
assert.equal(starColor({ temperature_k: 20000 }).slice(5), 'ff', 'hot ones blue')
// A star's own mass counts, not its size.
close(massOf({ ...SUN, body_type: 'star', mass_kg: 2 * SUN.mass_kg }), 2 * SUN.mass_kg, 1e-9, "a star's mass")

// Orbit limits: the Moon's orbit is fine, one at 8 050 km is torn apart,
// one at 1.2 million km is taken by the Sun; Earth is far within reach.
const lim = orbitLimits(moon, earth, sun)
assert.ok(lim.near_km > 8000 && lim.near_km < 20000, `Roche ${lim.near_km}`)
assert.equal(orbitProblem({ ...moon, orbit_km: 8050 }, earth, sun), "inside")
// A lighter moon (an ocean world) is torn apart farther out.
assert.equal(orbitProblem({ ...moon, body_type: "ocean", orbit_km: 9000 }, earth, sun), "close")
assert.equal(orbitProblem(earth, sun), '')
assert.ok(orbitLimits(earth, sun).far_km > 1e4 * AU_KM)
console.log('orbits: ok')
