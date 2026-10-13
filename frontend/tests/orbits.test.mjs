// The solar system's physics (src/orbits.js) and its drawing scale
// (src/solarScale.js). No dependencies: node frontend/tests/orbits.test.mjs
import assert from 'node:assert/strict'
import { massOf, surfaceGravity, circularOrbit, keplerOrbit, positionsAt, hillRadius, moonProblem, systemTree, orbitLimits, orbitProblem, periapsisOf, apoapsisOf, orbitThrough, createGravitySim, orbitPoint, physicsKey, compileSystem, elementsFromState, alongOrbit, orbitPath, G } from '../src/orbits.js'
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
// Ellipses: a comet at e = 0.9 starts at periapsis (phase = periapsis),
// passes apoapsis half a period later, and keeps Kepler's period.
const comet = { id: 9, kind: 'planet', body_type: 'rocky', radius_km: 5, orbit_km: 5 * AU_KM, parent_id: 1, eccentricity: 0.9, periapsis: 30, phase: 30 }
const P = keplerOrbit.period(comet, sun)
const dist = (p) => Math.hypot(...p)
close(dist(keplerOrbit.position(comet, sun, 0)), periapsisOf(comet), 1e-9, 'starts closest')
close(dist(keplerOrbit.position(comet, sun, P / 2)), apoapsisOf(comet), 1e-9, 'farthest half a period on')
close(dist(keplerOrbit.position(comet, sun, P)), periapsisOf(comet), 1e-6, 'back after a period')
// Faster when closer: a tenth of a period covers far more angle near periapsis.
const angleAt = (d) => Math.atan2(keplerOrbit.position(comet, sun, d)[1], keplerOrbit.position(comet, sun, d)[0])
assert.ok(Math.abs(angleAt(P * 0.05) - angleAt(0)) > Math.abs(angleAt(P * 0.55) - angleAt(P * 0.5)), 'faster near the star')
// Many turns on, a fast eccentric moon is still on its ellipse (the angle
// used to grow without bound and Kepler's equation went astray).
const fast = { ...moon, orbit_km: 30000, eccentricity: 0.9, periapsis: 40 }
for (const d of [1e3, 3.3e4, 1e6 + 0.37]) {
  const r = dist(keplerOrbit.position(fast, earth, d))
  assert.ok(r >= periapsisOf(fast) * 0.999999 && r <= apoapsisOf(fast) * 1.000001, `on the ellipse at day ${d}: ${r}`)
}
// A circle is the old circle.
close(keplerOrbit.position(earth, sun, 100)[0], circularOrbit.position(earth, sun, 100)[0], 1e-9, 'e = 0 is a circle')
// Dragging to a point puts the body exactly there.
const through = { ...comet, ...orbitThrough(comet, 2 * AU_KM, 120) }
close(dist(keplerOrbit.position(through, sun, 0)), 2 * AU_KM, 1e-9, 'dragged distance')
// An ellipse that dips too close is judged at its closest.
assert.equal(orbitProblem({ ...moon, body_type: 'ocean', orbit_km: 30000, eccentricity: 0.7 }, earth, sun), 'close')

// Live gravity: Earth and the Moon, a year on, are still where Kepler puts
// them (the Moon round Earth, within a few percent).
const sim = createGravitySim([sun, earth, moon])
assert.ok(sim.advanceTo(365.25), 'a year in one go')
const e1 = sim.relative(2)
close(dist(e1), 1.496e8, 0.01, 'Earth stays at 1 AU')
close(Math.atan2(e1[1], e1[0]) + 2 * Math.PI, 2 * Math.PI, 0.02, 'and comes round in a year')
close(dist(sim.relative(3)), 384400, 0.05, 'the Moon stays round Earth')
// A moon far beyond the Hill sphere is taken away.
const lost = { ...moon, id: 4, orbit_km: 3e6 }
const sim2 = createGravitySim([sun, earth, lost])
assert.equal(sim2.advanceTo(400), false, 'stops when the moon breaks away')
assert.equal(sim2.event?.type, 'escape')
assert.equal(sim2.event.id, 4)
assert.equal(sim2.event.from, 2)

// A comet whose closest point is inside the star hits it: the run stops
// at the collision.
const diver = { id: 5, kind: 'planet', body_type: 'rocky', radius_km: 50, orbit_km: 0.05 * AU_KM, parent_id: 1, eccentricity: 0.95, periapsis: 0, phase: 180 }
const sim3 = createGravitySim([sun, earth, diver])
assert.equal(sim3.advanceTo(30), false)
assert.equal(sim3.event?.type, 'collision', `event ${JSON.stringify(sim3.event)}`)
assert.deepEqual([sim3.event.a, sim3.event.b].sort(), [1, 5])

// The orbit gravity has the body on now: unchanged when nothing else pulls.
const solo = createGravitySim([sun, comet])
solo.advanceTo(300)
const now = solo.orbitNow(9)
close(now.orbit_km, comet.orbit_km, 1e-3, 'same size')
close(now.eccentricity, comet.eccentricity, 1e-3, 'same shape')
close(now.periapsis, comet.periapsis, 1e-3, 'same direction')
assert.ok(now.bound)
// Back in time starts again.
sim.advanceTo(0)
close(dist(sim.relative(2)), 1.496e8, 1e-9, 'reset')
// Wherever gravity takes a body, it is on the orbit it is on now: the line
// drawn through orbitNow passes through the body, even very stretched.
const swing = { ...comet, id: 7, eccentricity: 0.95, orbit_km: 3 * AU_KM, phase: 200 }
const big = { id: 8, kind: 'planet', body_type: 'gas', radius_km: 70000, orbit_km: 2.2 * AU_KM, parent_id: 1, phase: 210 }
const sim4 = createGravitySim([sun, big, swing])
sim4.advanceTo(400)
for (const id of [7, 8]) {
  const o = sim4.orbitNow(id)
  if (!o?.bound) continue
  const r = sim4.relative(id)
  const shape = { orbit_km: o.orbit_km, eccentricity: o.eccentricity, periapsis: o.periapsis }
  const nu = Math.atan2(r[1], r[0]) - (o.periapsis * Math.PI) / 180
  const onLine = dist(orbitPoint(shape, nu))
  close(onLine, dist(r), 1e-6, `body ${id} on its line (e ${o.eccentricity.toFixed(3)})`)
}

// The physics' key ignores what it doesn't read (a rename, a colour) and
// sees what it does.
const key = physicsKey([sun, earth, moon])
assert.equal(physicsKey([sun, { ...earth, name: 'Terra', color: '#fff', day_hours: 30 }, moon]), key)
assert.notEqual(physicsKey([sun, { ...earth, eccentricity: 0.1 }, moon]), key)
assert.notEqual(physicsKey([sun, earth]), key)

// The compiled system puts every body where the model does, one at a time.
{
  const comet2 = { id: 4, kind: 'planet', body_type: 'rocky', radius_km: 500, orbit_km: 4e8, parent_id: 1, phase: 200, eccentricity: 0.8, periapsis: 30 }
  const twin = { id: 5, kind: 'planet', body_type: 'rocky', radius_km: 6000, orbit_km: 1.496e8, parent_id: 1, phase: 0.001 }
  const far = { id: 6, kind: 'planet', body_type: 'gas', radius_km: 70000, orbit_km: 7.7e9, parent_id: 1, phase: 40 }
  const all = [moon, far, comet2, sun, earth, twin]
  const system = compileSystem(all)
  assert.equal(system.list[0], sun, 'parents first')
  for (const days of [0, 13.7, 4000]) {
    const pos = system.absolute(days)
    for (const b of all) {
      const i = system.index.get(b.id)
      const parent = all.find((p) => p.id === b.parent_id)
      const rel = parent ? keplerOrbit.position(b, parent, days) : [0, 0, 0]
      const base = parent ? pos.subarray(system.index.get(parent.id) * 3) : [0, 0, 0]
      for (let k = 0; k < 2; k++) assert.ok(Math.abs(pos[i * 3 + k] - base[k] - rel[k]) <= (b.orbit_km || 1) * 1e-9, `body ${b.id} at day ${days}`)
    }
  }
  // Only orbits that can meet are paired: the comet crosses Earth's (and
  // so its moon's), the twin shares it, the far giant meets no one.
  const pairs = system.meetingPairs()
  const met = new Set()
  for (let k = 0; k < pairs.length; k += 2) met.add([system.list[pairs[k]].id, system.list[pairs[k + 1]].id].sort().join('-'))
  for (const p of ['2-4', '3-4', '2-5', '3-5', '4-5']) assert.ok(met.has(p), `pair ${p}`)
  for (const p of ['1-2', '1-6', '2-6', '2-3']) assert.ok(!met.has(p), `no pair ${p}`)
  // And no pair that ever touches is left out (checked against every pair).
  for (let d = 0; d < 2000; d += 0.5) {
    const pos = system.absolute(d)
    for (let i = 0; i < all.length; i++) {
      for (let j = i + 1; j < all.length; j++) {
        const [a, b] = [system.list[i], system.list[j]]
        const gap = Math.hypot(pos[i * 3] - pos[j * 3], pos[i * 3 + 1] - pos[j * 3 + 1])
        if (gap < a.radius_km + b.radius_km) assert.ok(met.has([a.id, b.id].sort().join('-')), `missed ${a.id}-${b.id}`)
      }
    }
  }
}

// Inclined orbits: tilted about the node line, as far from the parent as
// flat, crossing the plane going north at the ascending node.
{
  const tilted = { ...comet, eccentricity: 0.3, periapsis: 70, inclination: 30, ascending_node: 40, phase: 40 }
  const flat = { ...tilted, inclination: 0 }
  const P = keplerOrbit.period(tilted, sun)
  for (const d of [0, P / 7, P / 3]) close(dist(keplerOrbit.position(tilted, sun, d)), dist(keplerOrbit.position(flat, sun, d)), 1e-9, 'same distance')
  const atNode = keplerOrbit.position(tilted, sun, 0)
  assert.ok(Math.abs(atNode[2]) < dist(atNode) * 1e-9, 'on the plane at the node')
  close(Math.atan2(atNode[1], atNode[0]), (40 * Math.PI) / 180, 1e-9, 'the node is where it says')
  assert.ok(keplerOrbit.velocity(tilted, sun, 0)[2] > 0, 'going north there')
  let top = 0
  for (const p of orbitPath(tilted)) top = Math.max(top, Math.asin(p[2] / dist(p)))
  close((top * 180) / Math.PI, 30, 1e-2, 'as high as its inclination')

  // The elements read back from a position and a velocity are the ones set
  // (tilted, flat, and backwards).
  const mu = G * (massOf(sun) + massOf(tilted))
  for (const b of [tilted, { ...tilted, inclination: 0 }, { ...tilted, inclination: 150 }, { ...tilted, inclination: 180 }]) {
    for (const d of [0, P / 5]) {
      const r = keplerOrbit.position(b, sun, d).map((x) => x * 1000)
      const v = keplerOrbit.velocity(b, sun, d).map((x) => x * 1000)
      const o = elementsFromState(r, v, mu)
      close(o.orbit_km, b.orbit_km, 1e-9, `size (i ${b.inclination})`)
      close(o.eccentricity, b.eccentricity, 1e-9, `shape (i ${b.inclination})`)
      close(o.inclination, b.inclination, 1e-6, `tilt (i ${b.inclination})`)
      // A flat orbit has no node: only its closest point's direction counts.
      const back = { ...b, ...o }
      for (const nu of [0, 1, 2.5]) {
        const [p1, p2] = [orbitPoint(b, nu), orbitPoint(back, nu)]
        assert.ok(Math.hypot(p1[0] - p2[0], p1[1] - p2[1], p1[2] - p2[2]) < b.orbit_km * 1e-6, `same orbit (i ${b.inclination}, nu ${nu})`)
      }
    }
  }

  // Seen from above: a direction on the orbit gives back the angle along
  // it, and how much shorter it looks.
  for (const phase of [10, 100, 200, 300]) {
    const p = keplerOrbit.position({ ...tilted, phase }, sun, 0)
    const { phase: back, squash } = alongOrbit(tilted, (Math.atan2(p[1], p[0]) * 180) / Math.PI)
    close(back, phase, 1e-6, `phase ${phase} from above`)
    close(squash, Math.hypot(p[0], p[1]) / dist(p), 1e-9, `squash at ${phase}`)
  }

  // Gravity starts on the tilted orbit.
  const sim = createGravitySim([sun, tilted])
  const now = sim.orbitNow(tilted.id)
  close(now.inclination, 30, 1e-3, 'gravity: tilt')
  close(now.ascending_node, 40, 1e-3, 'gravity: node')
  close(now.periapsis, 70, 1e-3, 'gravity: periapsis')
}
console.log('orbits: ok')
