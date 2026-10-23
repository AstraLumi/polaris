<template>
  <div ref="wrapEl" class="solar-wrap">
    <canvas ref="canvasEl" class="solar-canvas" @pointerdown="onDown" @pointerup="onUp"></canvas>
    <span
      v-for="l in labels"
      :key="l.id"
      :ref="(el) => setLabelEl(l.id, el)"
      class="body-label"
      :class="{ 'is-selected': l.id === selected, 'is-star': l.star }"
    >{{ l.name }}</span>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import * as THREE from 'three'
import { OrbitControls } from 'three/addons/controls/OrbitControls.js'
import { LineSegments2 } from 'three/addons/lines/LineSegments2.js'
import { LineSegmentsGeometry } from 'three/addons/lines/LineSegmentsGeometry.js'
import { LineMaterial } from 'three/addons/lines/LineMaterial.js'
import { PlanetGrid } from '../planetGrid'
import { sceneRadius, sceneDistance, sceneLayout, sceneOrbitPath, bodyColor } from '../solarScale'
import { orbitLimits, compileSystem, orbitFrame } from '../orbits'
import { habitableZone, luminosity } from '../stars'
import { createGlobeMesh } from '../three/globeMesh'
import { createStar, createPlanetMaterial, createAtmosphere, createClouds, createRings, setCloudCover } from '../three/bodyShaders'
import { cloudCoverage, genericCloudCover, mainPlanetCloudCover } from '../climate'
import { loadSkyDome, disposeSkyDome, setSkyGlow } from '../three/skyDome'
import { createLensPass, HOLE_LAYER } from '../three/lensPass'
import { setOccluders, setRingShadow, setFill } from '../three/shadows'
import { enclosureOf, shellGlow, shellLight, MOON_GLOW } from '../lighting'
import { shadowCasters } from '../eclipses'

// The solar system in 3D: the star at the centre, each body on its orbit at
// the time `days`, drawn at the squeezed scale of solarScale.js. Every
// planet or moon with a map wears it (three/globeMesh.js): the main one the
// story's map, the others their own; the rest are plain spheres in their
// colour. Each spins once a day on its tilted axis (drawn as a thin line).
// Click a body to pick it; the camera then follows it. The physics is all
// in orbits.js: this only draws.
const props = defineProps({
  bodies: { type: Array, default: () => [] },
  days: { type: Number, default: 0 },
  selected: { type: Number, default: null },
  // The maps the bodies wear: id -> { frequency, colorOf, version }.
  globes: { type: Map, default: () => new Map() },
  sky: { type: String, default: '' },
  // The markers: the star's habitable zone, and how close and how far the
  // picked body can orbit what it orbits.
  showZone: { type: Boolean, default: true },
  showLimits: { type: Boolean, default: true },
  // The climate's step (month) the main planet's clouds are drawn for.
  cloudStep: { type: Number, default: null },
  // A shell round the whole system, the sky itself, that glows by day
  // (lighting.js); null for none.
  enclosure: { type: Object, default: null },
  // Where each body is (id -> km from what it orbits: solarSimulation.js);
  // null to follow the orbits as set, at `days`.
  relative: { type: Function, default: null },
  // With gravity, the orbit each body is on now (id -> { orbit_km,
  // eccentricity, periapsis, inclination, ascending_node }, or no entry
  // when it has none); null to
  // draw the orbits as set.
  shapes: { type: Map, default: null },
})
const emit = defineEmits(['select'])

const wrapEl = ref(null)
const canvasEl = ref(null)
// The labels: made when the bodies change, moved every frame without Vue
// (placeLabels sets their transform).
const labels = computed(() => props.bodies.map((b) => ({ id: b.id, name: b.name, star: b.kind === 'star' })))
const labelEls = new Map() // body id -> its label's element
const labelAt = new Map() // body id -> where the last frame put it (a transform, or '' when hidden)
function setLabelEl(id, el) {
  if (!el) return labelEls.delete(id)
  labelEls.set(id, el)
  // Where it was (Vue may hand over the element again), else the next frame.
  if (labelAt.has(id)) showLabel(el, labelAt.get(id))
  else {
    showLabel(el, '')
    requestRender()
  }
}
function showLabel(el, transform) {
  el.style.display = transform ? '' : 'none'
  if (transform) el.style.transform = transform
}

const colorOf = bodyColor

let renderer, scene, camera, controls
let lens = null // three/lensPass.js
let raf = 0
let resizeObserver = null
let skyMesh = null
let cancelSky = () => {}
const globeViews = new Map() // body id -> its map's mesh, built once and moved onto it
const globeGrids = new Map() // body id -> its map's grid (the main planet's clouds read it)
let bodyGroup = null // everything rebuilt when the bodies change
const meshes = new Map() // body id -> { body, mesh, spin, orbit (trail, round its parent) }
let follow = null // { id, last: Vector3 }
let stars = [] // three/bodyShaders.js stars, their coronas facing the camera
let sunLit = [] // materials told where the star is each frame (planets, air, clouds, rings)
let glowing = new Set() // the materials of moons that shine on their own
// What falls in other bodies' shadows: per body, the materials it is drawn
// with ({ id, radius, reach, materials, rings }: its rings also catch its
// own shadow).
let shadowed = []
let cloudMaterials = [] // told the time, so clouds drift
let markerGroup = null // the habitable zone and the gravity limits
let limitsAround = null // the id of the body the limits are drawn round

// ---- Building ----------------------------------------------------------------------
function disposeGroup(g) {
  g?.traverse((o) => {
    if (o.userData.globe) return
    o.geometry?.dispose()
    o.material?.dispose?.()
  })
}

// A line through points (a circle of `radius`, or the given ones).
function orbitLine(radius, points = null) {
  const pts = points
    ? points.map((p) => new THREE.Vector3(...p))
    : Array.from({ length: 129 }, (_, k) => {
      const a = (k / 128) * Math.PI * 2
      return new THREE.Vector3(Math.cos(a) * radius, Math.sin(a) * radius, 0)
    })
  return new THREE.Line(
    new THREE.BufferGeometry().setFromPoints(pts),
    new THREE.LineBasicMaterial({ color: 0x8aa0c8, transparent: true, opacity: 0.35 }),
  )
}

// What the meshes are built from: everything but what only moves them (the
// starting angle, the day) or labels them. Picking a body or typing its name
// doesn't rebuild the scene.
const sceneKey = (bodies) => JSON.stringify(bodies, (k, v) => (k === 'name' || k === 'phase' || k === 'day_hours' ? undefined : v))
let builtKey = ''

function onBodies() {
  if (sceneKey(props.bodies) !== builtKey) return buildBodies()
  for (const b of props.bodies) {
    const m = meshes.get(b.id)
    if (m) m.body = b
  }
  compile()
  place()
}

function buildBodies() {
  builtKey = sceneKey(props.bodies)
  compile()
  if (bodyGroup) {
    for (const g of globeViews.values()) g.mesh.parent?.remove(g.mesh)
    scene.remove(bodyGroup)
    disposeGroup(bodyGroup)
  }
  bodyGroup = new THREE.Group()
  meshes.clear()
  stars = []
  sunLit = []
  glowing = new Set()
  shadowed = []
  cloudMaterials = []
  mainClouds = null
  const byId = new Map(props.bodies.map((b) => [b.id, b]))
  for (const b of props.bodies) {
    const r = sceneRadius(b)
    let mesh
    const globe = b.kind !== 'star' ? globeViews.get(b.id) : null
    // Air (planets have it) bends red light into the shadow it casts.
    const receives = { id: b.id, radius: r, reach: r, air: b.kind === 'planet', materials: [], rings: [] }
    if (b.kind !== 'star') shadowed.push(receives)
    if (b.kind === 'star') {
      const star = createStar(r, colorOf(b), { blackHole: b.star_class === 'black_hole', dyson: !!b.dyson })
      stars.push(star)
      mesh = star.group
      // A black hole is drawn apart from the rest, over its lensed sky.
      if (b.star_class === 'black_hole') mesh.traverse((o) => o.layers.set(HOLE_LAYER))
    } else if (globe) {
      globe.mesh.scale.setScalar(r)
      mesh = globe.mesh
      receives.materials.push(globe.material)
      if (b.kind === 'moon' && b.glows) glowing.add(globe.material)
    } else {
      const material = createPlanetMaterial(colorOf(b), { bands: b.body_type === 'gas' })
      sunLit.push(material)
      receives.materials.push(material)
      if (b.kind === 'moon' && b.glows) glowing.add(material)
      mesh = new THREE.Mesh(new THREE.SphereGeometry(r, 32, 16), material)
    }
    // holder (where it is) > tilt (its axis) > spin (its day) > the body.
    const holder = new THREE.Group()
    let spin = null
    if (b.kind === 'star') holder.add(mesh)
    else {
      const tilt = new THREE.Group()
      tilt.rotation.x = ((b.axial_tilt || 0) * Math.PI) / 180
      spin = new THREE.Group()
      spin.add(mesh)
      // Planets have air and (but gas giants) clouds; moons are bare.
      if (b.kind === 'planet') {
        const air = createAtmosphere(r, atmosphereColor(b))
        sunLit.push(air.material)
        receives.materials.push(air.material)
        tilt.add(air)
        const clouds = cloudsFor(b, r)
        if (clouds) {
          sunLit.push(clouds.material)
          receives.materials.push(clouds.material)
          cloudMaterials.push(clouds.material)
          spin.add(clouds)
        }
      }
      // Rings lie in the equator: they tilt with the planet, but don't spin.
      if (b.kind === 'planet' && b.ring_outer > b.ring_inner && b.ring_inner >= 1) {
        const ring = createRings(r, b.ring_inner, b.ring_outer, b.ring_color || ringColor(b))
        sunLit.push(ring.material)
        receives.rings.push(ring.material)
        receives.reach = r * b.ring_outer
        receives.ring = { mesh: ring, inner: r * b.ring_inner, outer: r * b.ring_outer, center: new THREE.Vector3(), normal: new THREE.Vector3() }
        tilt.add(ring)
      }
      tilt.add(spin)
      tilt.add(axisLine(r))
      holder.add(tilt)
    }
    holder.traverse((o) => (o.userData.id = b.id))
    bodyGroup.add(holder)
    let orbit = null
    const parent = byId.get(b.parent_id)
    if (parent && b.orbit_km) {
      orbit = makeTrail()
      setTrailPath(orbit, b, parent)
      bodyGroup.add(orbit.group)
    }
    meshes.set(b.id, { body: b, mesh: holder, spin, orbit })
  }
  scene.add(bodyGroup)
  buildMarkers()
  place()
}

// ---- Air, clouds and rings ---------------------------------------------------------------
// Rings with no colour of their own take a pale dusty tint of the planet's.
const ringColor = (b) => new THREE.Color(colorOf(b)).lerp(new THREE.Color('#e8dcc0'), 0.6).getStyle()
const AIR = { rocky: '#7fb2ff', ocean: '#6fc8ff' }
const atmosphereColor = (b) => AIR[b.body_type] || new THREE.Color(colorOf(b)).lerp(new THREE.Color('#ffffff'), 0.35).getStyle()

// The planet's clouds (climate.js): the main planet's from the map's
// hexes, any other's generic for its type. Null for a cloudless one. Each
// cover is kept (they take a while), until the maps change.
const cloudCovers = new Map() // key -> the clouds' aCover
let mainClouds = null // { mesh, body }: the main planet's, redrawn when the month changes
function mainCoverOf(b) {
  const grid = globeGrids.get(b.id)
  const climate = props.globes.get(b.id)?.climate ?? null
  const cover = mainPlanetCloudCover({ grid, body: b, climate, step: props.cloudStep })
  return (dir) => cover(grid.cellAt(dir))
}
function cloudsFor(b, r) {
  const coverage = cloudCoverage(b)
  if (!(coverage > 0)) return null
  const grid = b.is_main ? globeGrids.get(b.id) : null
  const climate = grid ? props.globes.get(b.id)?.climate : null
  const key = `${b.id}:${coverage}:${grid ? (climate ? `climate:${props.cloudStep}` : 'map') : 'generic'}`
  let coverOf = cloudCovers.get(key)
  if (!coverOf && grid) coverOf = mainCoverOf(b)
  const clouds = createClouds(r, coverOf ?? ((dir) => genericCloudCover(dir, coverage, b.id)))
  cloudCovers.set(key, clouds.geometry.attributes.aCover.array)
  if (b.is_main && grid) mainClouds = { mesh: clouds, body: b }
  return clouds
}

// The month moved on: the main planet's clouds follow its climate.
function updateMainClouds() {
  if (!mainClouds || !props.globes.get(mainClouds.body.id)?.climate) return
  setCloudCover(mainClouds.mesh, mainCoverOf(mainClouds.body))
  requestRender()
}

// ---- Orbit trails -------------------------------------------------------------------------
// Each orbit fades round from its body: brightest and thickest just behind
// it (where it has been), fading all the way round to faint and thin just
// ahead of it, with no edge in between. Brightness is drawn as colour on
// additive lines (lines can't fade their own opacity), so black is
// invisible: the whole orbit is two lines of segments, thick and thin,
// sharing one array of points, and each segment is lit on one and black on
// the other. The points change only with the orbit; as the body moves on,
// the colours are rewritten in place.
const TRAIL_STEPS = 128
const TRAIL_COLOR = new THREE.Color(0xaebfe0)
const trailMaterial = () => {
  const m = new LineMaterial({ color: 0xffffff, linewidth: 2, vertexColors: true, transparent: true, blending: THREE.AdditiveBlending, depthWrite: false })
  // No round caps: on additive lines each joint's two caps added up to a
  // bright dot, beading the orbit. At 128 segments the gaps don't show.
  m.onBeforeCompile = (shader) => {
    shader.fragmentShader = shader.fragmentShader.replace('if ( len2 > 1.0 ) discard;', 'discard;')
  }
  m.customProgramCacheKey = () => 'trail-no-caps'
  if (renderer) m.resolution.set(renderer.domElement.width, renderer.domElement.height)
  return m
}

function makeTrail() {
  const group = new THREE.Group()
  group.userData.trail = true
  // Per segment: start and end, three numbers each.
  const points = new Float32Array(TRAIL_STEPS * 6)
  const thick = new Float32Array(TRAIL_STEPS * 6)
  const thin = new Float32Array(TRAIL_STEPS * 6)
  const aheadGeometry = new THREE.BufferGeometry()
  aheadGeometry.setAttribute('position', new THREE.BufferAttribute(points, 3))
  aheadGeometry.setAttribute('color', new THREE.BufferAttribute(thin, 3))
  const ahead = new THREE.LineSegments(
    aheadGeometry,
    new THREE.LineBasicMaterial({ vertexColors: true, transparent: true, blending: THREE.AdditiveBlending, depthWrite: false }),
  )
  const behindGeometry = new LineSegmentsGeometry()
  behindGeometry.setPositions(points)
  behindGeometry.setColors(thick)
  const behind = new LineSegments2(behindGeometry, trailMaterial())
  group.add(ahead, behind)
  return { group, ahead, behind, points, thick, thin, frame: null, at: -1 }
}

// The orbit's points, at even steps of the angle from periapsis.
function setTrailPath(trail, body, parent, shape = body) {
  const orbit = { ...body, ...shape }
  const path = sceneOrbitPath(orbit, parent, TRAIL_STEPS)
  for (let s = 0; s < TRAIL_STEPS; s++) {
    trail.points.set(path[s], s * 6)
    trail.points.set(path[s + 1], s * 6 + 3)
  }
  trail.ahead.geometry.attributes.position.needsUpdate = true
  trail.behind.geometry.attributes.instanceStart.data.needsUpdate = true
  trail.ahead.geometry.computeBoundingSphere()
  trail.behind.geometry.computeBoundingBox()
  trail.behind.geometry.computeBoundingSphere()
  trail.frame = orbitFrame(orbit)
  trail.at = -1
}

// Lights the trail from where the body is (rel: its drawn place from its
// parent). Orbits run anticlockwise: behind is the lower angles. The thick
// half fades from the far side (0.32) up to the body (1); the thin half goes
// on from the body's front (0.12) back up to the far side (0.32).
function splitTrail(trail, rel) {
  const n = TRAIL_STEPS
  // The angle from periapsis, in the orbit's own plane (drawing squeezes
  // distances, not directions).
  const { P, Q } = trail.frame
  const nu = Math.atan2(rel.x * Q[0] + rel.y * Q[1] + rel.z * Q[2], rel.x * P[0] + rel.y * P[1] + rel.z * P[2])
  const at = Math.round(((((nu / (2 * Math.PI)) % 1) + 1) % 1) * n) % n
  if (at === trail.at) return
  trail.at = at
  const half = n / 2
  const paint = (colors, s, a, b) => {
    const o = s * 6
    colors[o] = TRAIL_COLOR.r * a
    colors[o + 1] = TRAIL_COLOR.g * a
    colors[o + 2] = TRAIL_COLOR.b * a
    colors[o + 3] = TRAIL_COLOR.r * b
    colors[o + 4] = TRAIL_COLOR.g * b
    colors[o + 5] = TRAIL_COLOR.b * b
  }
  for (let s = 0; s < n; s++) {
    const d = (s - at + n) % n // segments on from the body
    if (d < half) {
      paint(trail.thin, s, 0.12 + 0.2 * (d / half), 0.12 + 0.2 * ((d + 1) / half))
      paint(trail.thick, s, 0, 0)
    } else {
      paint(trail.thick, s, 0.32 + 0.68 * ((d - half) / half), 0.32 + 0.68 * ((d + 1 - half) / half))
      paint(trail.thin, s, 0, 0)
    }
  }
  trail.ahead.geometry.attributes.color.needsUpdate = true
  trail.behind.geometry.attributes.instanceColorStart.data.needsUpdate = true
}

// A body's axis, through its poles and a little beyond.
function axisLine(r) {
  return new THREE.Line(
    new THREE.BufferGeometry().setFromPoints([new THREE.Vector3(0, 0, -r * 1.6), new THREE.Vector3(0, 0, r * 1.6)]),
    new THREE.LineBasicMaterial({ color: 0xdfe6f5, transparent: true, opacity: 0.35 }),
  )
}

// ---- Markers -------------------------------------------------------------------------
function dashedCircle(radius, color) {
  const line = orbitLine(radius)
  line.material.dispose()
  line.material = new THREE.LineDashedMaterial({ color, dashSize: radius * 0.04, gapSize: radius * 0.025, transparent: true, opacity: 0.9 })
  line.computeLineDistances()
  return line
}

function buildMarkers() {
  if (markerGroup) {
    scene.remove(markerGroup)
    disposeGroup(markerGroup)
  }
  markerGroup = new THREE.Group()
  limitsAround = null
  const byId = new Map(props.bodies.map((b) => [b.id, b]))
  const star = props.bodies.find((b) => b.kind === 'star')
  const hz = props.showZone && star ? habitableZone(star) : null
  if (hz) {
    const ring = new THREE.Mesh(
      new THREE.RingGeometry(sceneDistance(hz.inner_km, star), sceneDistance(hz.outer_km, star), 128),
      // A faint glow added to what is behind it, drawn right after the sky
      // and before the bodies: it lies in the plane through every planet,
      // and drawn over them it veiled their lower halves.
      new THREE.MeshBasicMaterial({ color: new THREE.Color(0x4caf6e).multiplyScalar(0.06), blending: THREE.AdditiveBlending, side: THREE.DoubleSide, depthWrite: false }),
    )
    ring.renderOrder = -0.5
    ring.userData.zone = true
    markerGroup.add(ring)
  }
  const b = byId.get(props.selected)
  const parent = b && byId.get(b.parent_id)
  if (props.showLimits && parent) {
    const grand = parent.parent_id != null ? byId.get(parent.parent_id) : null
    const { near_km, far_km } = orbitLimits(b, parent, grand)
    const limits = new THREE.Group()
    limits.add(dashedCircle(sceneDistance(near_km, parent, b), 0xe05a5a))
    limits.add(dashedCircle(sceneDistance(far_km, parent, b), 0xe0a04a))
    limits.userData.limits = true
    markerGroup.add(limits)
    limitsAround = parent.id
  }
  scene.add(markerGroup)
}

// The maps, made again when the set changes (each is many cells).
function buildGlobes() {
  for (const g of globeViews.values()) {
    g.mesh.parent?.remove(g.mesh)
    g.dispose()
  }
  globeViews.clear()
  globeGrids.clear()
  cloudCovers.clear()
  for (const [id, m] of props.globes) {
    const grid = new PlanetGrid(m.frequency)
    globeGrids.set(id, grid)
    const g = createGlobeMesh(grid, m.colorOf, { ambient: 0.05, twilight: 0.12 })
    g.mesh.userData.globe = true
    globeViews.set(id, g)
  }
  buildBodies()
}

// The bodies compiled for moving (orbits.js), made again whenever they
// change, and the arrays each frame writes into.
let system = null
const fillColor = new THREE.Color(1, 1, 1)
const glowColor = new THREE.Color(1, 1, 1) // a glowing moon's own light
let relKm = null // each body from its parent, km
let drawn = null // each body in the scene
const positions = new Map() // body id -> where it is drawn (the same vector every frame)
function compile() {
  system = compileSystem(props.bodies)
  relKm = new Float64Array(system.list.length * 3)
  drawn = new Float64Array(system.list.length * 3)
  positions.clear()
  for (const b of system.list) positions.set(b.id, new THREE.Vector3())
}

// Puts every body where it is at `days`, and each orbit round its parent.
const origin = new THREE.Vector3()
const fromParent = new THREE.Vector3()
function place() {
  if (props.relative) {
    system.list.forEach((b, i) => relKm.set(props.relative(b.id), i * 3))
  } else system.relative(props.days, relKm)
  sceneLayout(system, relKm, drawn)
  system.list.forEach((b, i) => positions.get(b.id).fromArray(drawn, i * 3))
  for (const { body, mesh, spin, orbit } of meshes.values()) {
    mesh.position.copy(positions.get(body.id))
    // A turn a day (days are Earth days; day_hours its own).
    if (spin) spin.rotation.z = (((props.days * 24) / (body.day_hours || 24)) % 1) * 2 * Math.PI
    if (orbit) {
      const base = positions.get(body.parent_id) ?? origin
      orbit.group.position.copy(base)
      splitTrail(orbit, fromParent.copy(positions.get(body.id)).sub(base))
    }
  }
  // The limits ring what the picked body orbits, wherever it is.
  const limits = markerGroup?.children.find((o) => o.userData.limits)
  if (limits && positions.has(limitsAround)) limits.position.copy(positions.get(limitsAround))
  requestRender()
}

// The camera sees the whole system, from above and a little to the side.
function frame() {
  let extent = 1
  for (const p of positions.values()) extent = Math.max(extent, p.length())
  camera.position.set(0, -extent * 1.5, extent * 0.9)
  controls.target.set(0, 0, 0)
  controls.update()
}

// ---- Sky ---------------------------------------------------------------------------
function loadSky() {
  cancelSky()
  if (skyMesh) {
    scene.remove(skyMesh)
    disposeSkyDome(skyMesh)
    skyMesh = null
  }
  if (!props.sky) return requestRender()
  cancelSky = loadSkyDome(props.sky, (mesh) => {
    if (!scene) return disposeSkyDome(mesh)
    skyMesh = mesh
    scene.add(mesh)
    requestRender()
  })
}

// ---- Rendering ---------------------------------------------------------------------
function requestRender() {
  if (!raf && renderer) raf = requestAnimationFrame(render)
}

const tmp = new THREE.Vector3()
let framesDrawn = 0 // for the screenshots (below)
function render() {
  raf = 0
  if (!renderer) return
  framesDrawn++
  // Following a body: the camera moves along with it.
  if (follow && positions.has(follow.id)) {
    const now = positions.get(follow.id)
    tmp.copy(now).sub(follow.last)
    camera.position.add(tmp)
    controls.target.copy(now)
    follow.last.copy(now)
    controls.update()
  }
  skyMesh?.position.copy(camera.position)
  scene.updateMatrixWorld()
  // The maps are lit by the star: its direction, in each globe's own frame.
  const star = system.star
  const sun = (star && positions.get(star.id)) || origin
  for (const g of globeViews.values()) {
    if (!g.mesh.parent) continue
    g.material.uniforms.uLight.value.copy(g.mesh.worldToLocal(tmp.copy(sun)).normalize())
  }
  // A star with no light of its own (a black hole, or one inside a Dyson
  // sphere) leaves its planets dark, unless the shell lights them from all
  // round (lighting.js; the same for every body, at the clock's hour of the
  // shell's day; full daylight is about 0.6 of Earth's sun). The shell is
  // the sky: it glows by day too.
  const power = star && luminosity(star) > 0 ? 1 : 0
  const shell = enclosureOf(props.enclosure)
  const hours = props.days * 24
  const fill = shell.kind === 'shell' ? Math.min(1.5, shellLight(shell, hours) / 0.6) : 0
  if (shell.kind === 'shell') fillColor.set(shell.color)
  if (skyMesh) setSkyGlow(skyMesh, shell.kind === 'shell' ? shellGlow(shell, hours) : 0, fillColor)
  // A moon that shines on its own glows all over, as brightly as a full
  // moon, whatever lights it (lighting.js, MOON_GLOW).
  const glow = MOON_GLOW / 0.6
  const lightAll = (u, m) => (glowing.has(m) && glow > fill ? setFill(u, glow, glowColor) : setFill(u, fill, fillColor))
  for (const m of sunLit) {
    m.uniforms.uSun.value.copy(sun)
    m.uniforms.uPower.value = power
    lightAll(m.uniforms, m)
  }
  for (const g of globeViews.values()) {
    g.material.uniforms.uPower.value = power
    g.material.uniforms.uSun.value.copy(sun)
    lightAll(g.material.uniforms, g.material)
  }
  castShadows(star, sun)
  // Clouds drift with the clock (slowly: a turn of the pattern is years).
  for (const m of cloudMaterials) m.uniforms.uTime.value = (props.days % 100000) * 0.004
  const time = performance.now() / 1000
  for (const s of stars) s.faceCamera(camera, time)
  // A black hole bends the light of everything behind it (three/lensPass.js).
  const lensed = star?.star_class === 'black_hole' && lens.render(renderer, scene, camera, sun, sceneRadius(star))
  if (!lensed) renderer.render(scene, camera)
  placeLabels()
}

// Who shadows whom this frame (eclipses.js), told to each body's materials.
// The light is a disc smaller than the star is drawn (the drawn star is
// huge next to its squeezed orbits), so shadows have a core, not only a
// soft edge.
const LIGHT_SIZE = 0.35
function castShadows(star, sun) {
  const n = shadowed.length
  const centers = new Float64Array((n + 1) * 3)
  shadowed.forEach((s, i) => positions.get(s.id)?.toArray(centers, i * 3))
  sun.toArray(centers, n * 3)
  const radii = [...shadowed.map((s) => s.radius), 0]
  const reach = [...shadowed.map((s) => s.reach), 0]
  const lightRadius = star ? sceneRadius(star) * LIGHT_SIZE : 1
  const color = lightColor(star)
  const casters = shadowCasters({ centers, radii, reach, star: star ? n : -1, lightRadius })
  const sphere = (j) => [centers[j * 3], centers[j * 3 + 1], centers[j * 3 + 2], shadowed[j]?.air ? -radii[j] : radii[j]]
  shadowed.forEach((s, i) => {
    const occluders = casters[i].map(sphere)
    // A ringed planet lies in its rings' shadow, the rings in the planet's.
    if (s.ring) {
      s.ring.center.setFromMatrixPosition(s.ring.mesh.matrixWorld)
      s.ring.normal.set(0, 0, 1).transformDirection(s.ring.mesh.matrixWorld)
    }
    for (const m of s.materials) {
      setOccluders(m.uniforms, occluders, lightRadius, color)
      setRingShadow(m.uniforms, s.ring)
    }
    for (const m of s.rings) setOccluders(m.uniforms, [[...sphere(i).slice(0, 3), radii[i]], ...occluders], lightRadius, color)
  })
}

// The star's light, in its colour (half way to white: a red giant's
// planets warm, not red).
const light = new THREE.Color()
const WHITE = new THREE.Color(1, 1, 1)
function lightColor(star) {
  if (!star) return WHITE
  light.set(colorOf(star))
  const top = Math.max(light.r, light.g, light.b) || 1
  return light.multiplyScalar(1 / top).lerp(WHITE, 0.5)
}

// Where a body's label goes on screen (px), or null when behind the camera.
function labelPoint(id, w, h) {
  const m = meshes.get(id)
  const p = positions.get(id)
  if (!p || !m) return null
  tmp.copy(p)
  tmp.z += sceneRadius(m.body) * 1.2
  tmp.project(camera)
  return tmp.z <= 1 ? [((tmp.x + 1) / 2) * w, ((1 - tmp.y) / 2) * h] : null
}

function placeLabels() {
  const w = renderer.domElement.clientWidth
  const h = renderer.domElement.clientHeight
  for (const [id, el] of labelEls) {
    let pt = labelPoint(id, w, h)
    // A moon's label on top of its planet's (seen from afar) is left out,
    // unless it is the one picked.
    const body = meshes.get(id)?.body
    if (pt && body?.kind === 'moon' && id !== props.selected) {
      const q = labelPoint(body.parent_id, w, h)
      if (q && Math.abs(pt[0] - q[0]) < 60 && Math.abs(pt[1] - q[1]) < 18) pt = null
    }
    const at = pt ? `translate(${pt[0]}px, ${pt[1]}px)` : ''
    if (labelAt.get(id) !== at) {
      labelAt.set(id, at)
      showLabel(el, at)
    }
  }
}

function resize() {
  if (!renderer) return
  const w = wrapEl.value.clientWidth
  const h = wrapEl.value.clientHeight
  renderer.setSize(w, h, false)
  lens.setSize(renderer.domElement.width, renderer.domElement.height)
  // Thick lines are sized in pixels.
  for (const { orbit } of meshes.values()) orbit?.behind.material.resolution.set(renderer.domElement.width, renderer.domElement.height)
  camera.aspect = w / Math.max(h, 1)
  camera.updateProjectionMatrix()
  requestRender()
}

// ---- Picking -------------------------------------------------------------------------
let down = null
function onDown(e) {
  down = { x: e.clientX, y: e.clientY }
}

function onUp(e) {
  if (!down || Math.hypot(e.clientX - down.x, e.clientY - down.y) > 4) return
  down = null
  const rect = canvasEl.value.getBoundingClientRect()
  const ray = new THREE.Raycaster()
  ray.layers.enableAll()
  ray.setFromCamera(new THREE.Vector2(((e.clientX - rect.left) / rect.width) * 2 - 1, -((e.clientY - rect.top) / rect.height) * 2 + 1), camera)
  const hit = ray
    .intersectObjects(bodyGroup?.children.filter((o) => o.type !== 'Line' && !o.userData.trail) ?? [], true)
    .find((h) => !h.object.userData.corona)
  if (hit) emit('select', hit.object.userData.id)
}

function followSelected() {
  follow = props.selected != null && positions.has(props.selected) ? { id: props.selected, last: positions.get(props.selected).clone() } : null
  if (follow) {
    // Bring the camera near the body, keeping the direction it looks from.
    const body = meshes.get(props.selected)?.body
    const dist = Math.max(sceneRadius(body) * 8, 0.6)
    const dir = camera.position.clone().sub(controls.target).normalize()
    controls.target.copy(follow.last)
    camera.position.copy(follow.last).addScaledVector(dir, dist)
    controls.update()
  }
  requestRender()
}

// ---- Life ----------------------------------------------------------------------------
onMounted(() => {
  renderer = new THREE.WebGLRenderer({ canvas: canvasEl.value, antialias: true })
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2))
  lens = createLensPass()
  scene = new THREE.Scene()
  scene.background = new THREE.Color(0x02030a)
  camera = new THREE.PerspectiveCamera(45, 1, 0.005, 20000)
  camera.up.set(0, 0, 1)
  camera.layers.enable(HOLE_LAYER)
  scene.add(new THREE.AmbientLight(0xffffff, 0.08))
  const sun = new THREE.PointLight(0xffffff, 3, 0, 0)
  scene.add(sun)
  controls = new OrbitControls(camera, canvasEl.value)
  controls.enableDamping = false
  controls.addEventListener('change', requestRender)
  buildGlobes()
  frame()
  loadSky()
  resizeObserver = new ResizeObserver(resize)
  resizeObserver.observe(wrapEl.value)
  resize()
})

onBeforeUnmount(() => {
  cancelAnimationFrame(raf)
  cancelSky()
  resizeObserver?.disconnect()
  controls?.dispose()
  disposeGroup(bodyGroup)
  disposeGroup(markerGroup)
  for (const g of globeViews.values()) g.dispose()
  if (skyMesh) disposeSkyDome(skyMesh)
  lens?.dispose()
  renderer?.dispose()
  renderer = null
  scene = null
})

watch(() => props.bodies, onBodies, { deep: true })
watch(() => props.globes, buildGlobes)
watch(() => [props.days, props.relative], place)

// Redraws each orbit line as gravity has reshaped it (or as set).
function drawShapes() {
  const byId = new Map(props.bodies.map((b) => [b.id, b]))
  for (const { body, orbit } of meshes.values()) {
    if (!orbit) continue
    const shape = props.shapes ? props.shapes.get(body.id) : body
    orbit.group.visible = !!shape
    if (!shape) continue
    setTrailPath(orbit, body, byId.get(body.parent_id), shape)
  }
  place()
}
watch(() => props.shapes, drawShapes)
watch(() => props.selected, () => {
  buildMarkers()
  place()
  followSelected()
})
watch(() => [props.showZone, props.showLimits], () => {
  buildMarkers()
  place()
})
watch(() => props.sky, loadSky)
watch(() => props.enclosure, requestRender, { deep: true })
watch(() => props.cloudStep, updateMainClouds)

defineExpose({ frame: () => {
  follow = null
  frame()
  requestRender()
} })

// ---- For the screenshots (the dev server only) -------------------------------------------
// tests/visual/shots.mjs points the camera through this and waits for frames
// to be drawn; a build leaves it out.
if (import.meta.env.DEV) {
  window.__solarScene = {
    frames: () => framesDrawn,
    draw: () => requestRender(),
    skyLoaded: () => !props.sky || !!skyMesh,
    // 'angle' (the view it opens with), 'above', 'side' (edge-on), or a
    // body's name: close to it, following it (fromSun: from the star's
    // side, its day side in view). zoom > 1 goes farther out.
    view(name, { zoom = 1, fromSun = false } = {}) {
      follow = null
      const body = [...meshes.values()].find((m) => m.body.name === name)?.body
      if (body) {
        const at = positions.get(body.id)
        const d = Math.max(sceneRadius(body) * 8, 0.6) * zoom
        controls.target.copy(at)
        const sun = (system.star && positions.get(system.star.id)) || origin
        const from = fromSun ? sun.clone().sub(at).setLength(d).add(new THREE.Vector3(0, 0, d * 0.25)) : new THREE.Vector3(0, -d * 0.85, d * 0.5)
        camera.position.copy(at).add(from)
        follow = { id: body.id, last: at.clone() }
      } else {
        let d = 1
        for (const p of positions.values()) d = Math.max(d, p.length())
        d *= zoom
        controls.target.set(0, 0, 0)
        if (name === 'above') camera.position.set(0, -d * 0.001, d * 2.6)
        else if (name === 'side') camera.position.set(0, -d * 2.6, 0)
        else camera.position.set(0, -d * 1.5, d * 0.9)
      }
      controls.update()
      requestRender()
      return !!body || ['angle', 'above', 'side'].includes(name)
    },
  }
  onBeforeUnmount(() => delete window.__solarScene)
}
</script>

<style scoped>
.solar-wrap {
  position: absolute;
  inset: 0;
  overflow: hidden;
}

.solar-canvas {
  display: block;
  width: 100%;
  height: 100%;
  touch-action: none;
}

.body-label {
  position: absolute;
  top: 0;
  left: 0;
  padding: 0.05rem 0.4rem;
  border-radius: 999px;
  background: rgba(5, 8, 18, 0.55);
  color: #dfe6f5;
  font-size: 0.72rem;
  white-space: nowrap;
  pointer-events: none;
  translate: -50% -100%;
}

.body-label.is-star {
  color: #ffe2a0;
}

.body-label.is-selected {
  color: var(--accent);
  font-weight: 700;
}
</style>
