<template>
  <div ref="wrapEl" class="solar-wrap">
    <canvas ref="canvasEl" class="solar-canvas" @pointerdown="onDown" @pointerup="onUp"></canvas>
    <span
      v-for="l in labels"
      :key="l.id"
      class="body-label"
      :class="{ 'is-selected': l.id === selected, 'is-star': l.star }"
      :style="{ transform: `translate(${l.x}px, ${l.y}px)` }"
    >{{ l.name }}</span>
  </div>
</template>

<script setup>
import { ref, watch, onMounted, onBeforeUnmount } from 'vue'
import * as THREE from 'three'
import { OrbitControls } from 'three/addons/controls/OrbitControls.js'
import { Line2 } from 'three/addons/lines/Line2.js'
import { LineGeometry } from 'three/addons/lines/LineGeometry.js'
import { LineMaterial } from 'three/addons/lines/LineMaterial.js'
import { PlanetGrid } from '../planetGrid'
import { sceneRadius, sceneDistance, scenePositions, sceneOrbitPath, bodyColor } from '../solarScale'
import { orbitLimits } from '../orbits'
import { habitableZone } from '../stars'
import { createGlobeMesh } from '../three/globeMesh'
import { createStar, createPlanetMaterial, createAtmosphere, createClouds, createRings } from '../three/bodyShaders'
import { cloudCoverage, genericCloudCover, mainPlanetCloudCover } from '../climate'
import { loadSkyDome, disposeSkyDome } from '../three/skyDome'

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
  // With live gravity, where each body is (id -> km from what it orbits);
  // null to follow the orbits as drawn.
  relative: { type: Function, default: null },
  // With gravity, the orbit each body is on now (id -> { orbit_km,
  // eccentricity, periapsis }, or no entry when it has none); null to
  // draw the orbits as set.
  shapes: { type: Map, default: null },
})
const emit = defineEmits(['select'])

const wrapEl = ref(null)
const canvasEl = ref(null)
const labels = ref([])

const colorOf = bodyColor

let renderer, scene, camera, controls
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
let rings = [] // { material, id }: told where their planet is, for its shadow
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

function buildBodies() {
  if (bodyGroup) {
    for (const g of globeViews.values()) g.mesh.parent?.remove(g.mesh)
    scene.remove(bodyGroup)
    disposeGroup(bodyGroup)
  }
  bodyGroup = new THREE.Group()
  meshes.clear()
  stars = []
  sunLit = []
  rings = []
  cloudMaterials = []
  const byId = new Map(props.bodies.map((b) => [b.id, b]))
  for (const b of props.bodies) {
    const r = sceneRadius(b)
    let mesh
    const globe = b.kind !== 'star' ? globeViews.get(b.id) : null
    if (b.kind === 'star') {
      const star = createStar(r, colorOf(b), { blackHole: b.star_class === 'black_hole', dyson: !!b.dyson })
      stars.push(star)
      mesh = star.group
    } else if (globe) {
      globe.mesh.scale.setScalar(r)
      mesh = globe.mesh
    } else {
      const material = createPlanetMaterial(colorOf(b), { bands: b.body_type === 'gas' })
      sunLit.push(material)
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
        tilt.add(air)
        const clouds = cloudsFor(b, r)
        if (clouds) {
          sunLit.push(clouds.material)
          cloudMaterials.push(clouds.material)
          spin.add(clouds)
        }
      }
      // Rings lie in the equator: they tilt with the planet, but don't spin.
      if (b.kind === 'planet' && b.ring_outer > b.ring_inner && b.ring_inner >= 1) {
        const ring = createRings(r, b.ring_inner, b.ring_outer, b.ring_color || ringColor(b))
        sunLit.push(ring.material)
        rings.push({ material: ring.material, id: b.id })
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
// hexes, any other's generic for its type. Null for a cloudless one.
function cloudsFor(b, r) {
  const coverage = cloudCoverage(b)
  if (!(coverage > 0)) return null
  const grid = b.is_main ? globeGrids.get(b.id) : null
  if (grid) {
    const cover = mainPlanetCloudCover({ grid, body: b })
    return createClouds(r, (dir) => cover(grid.cellAt(dir)))
  }
  return createClouds(r, (dir) => genericCloudCover(dir, coverage, b.id))
}

// ---- Orbit trails -------------------------------------------------------------------------
// Each orbit fades round from its body: brightest and thickest just behind
// it (where it has been), fading all the way round to faint and thin just
// ahead of it, with no edge in between. The path is computed when the
// orbit changes; the fade moves with the body. Brightness is drawn as
// colour on additive lines (lines can't fade their own opacity).
const TRAIL_STEPS = 128
const TRAIL_COLOR = new THREE.Color(0xaebfe0)
const trailMaterial = () => {
  const m = new LineMaterial({ color: 0xffffff, linewidth: 2, vertexColors: true, transparent: true, blending: THREE.AdditiveBlending, depthWrite: false })
  if (renderer) m.resolution.set(renderer.domElement.width, renderer.domElement.height)
  return m
}

function makeTrail() {
  const group = new THREE.Group()
  group.userData.trail = true
  const ahead = new THREE.Line(
    new THREE.BufferGeometry(),
    new THREE.LineBasicMaterial({ vertexColors: true, transparent: true, blending: THREE.AdditiveBlending, depthWrite: false }),
  )
  const behind = new Line2(new LineGeometry(), trailMaterial())
  group.add(ahead, behind)
  return { group, ahead, behind, path: [], periapsis: 0, at: -1 }
}

// Colours along `count + 1` points, fading from strength a to b.
function fade(count, a, b) {
  const out = []
  for (let k = 0; k <= count; k++) {
    const t = a + (b - a) * (k / count)
    out.push(TRAIL_COLOR.r * t, TRAIL_COLOR.g * t, TRAIL_COLOR.b * t)
  }
  return out
}

// The orbit's points, at even steps of the angle from periapsis.
function setTrailPath(trail, body, parent, shape = body) {
  trail.path = sceneOrbitPath({ ...body, ...shape }, parent, TRAIL_STEPS).slice(0, TRAIL_STEPS)
  trail.periapsis = ((shape.periapsis || 0) * Math.PI) / 180
  trail.at = -1
}

// Splits the trail where the body is (rel: its drawn place from its parent).
function splitTrail(trail, rel) {
  const n = trail.path.length
  if (!n) return
  const nu = Math.atan2(rel.y, rel.x) - trail.periapsis
  const at = Math.round(((((nu / (2 * Math.PI)) % 1) + 1) % 1) * n) % n
  if (at === trail.at) return
  trail.at = at
  const half = n / 2
  const pick = (from, count) => {
    const out = []
    for (let k = 0; k <= count; k++) out.push(...trail.path[(from + k + n) % n])
    return out
  }
  // Orbits run anticlockwise: behind is the lower angles. The thick half
  // fades from the far side (0.32) up to the body (1); the thin half goes
  // on from the body's front (0.12) back up to the far side (0.32).
  trail.behind.geometry.setPositions(pick(at - half, half))
  trail.behind.geometry.setColors(fade(half, 0.32, 1))
  const ahead = pick(at, half)
  trail.ahead.geometry.setAttribute('position', new THREE.Float32BufferAttribute(ahead, 3))
  trail.ahead.geometry.setAttribute('color', new THREE.Float32BufferAttribute(fade(half, 0.12, 0.32), 3))
  trail.ahead.geometry.computeBoundingSphere()
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
      new THREE.MeshBasicMaterial({ color: 0x4caf6e, transparent: true, opacity: 0.16, side: THREE.DoubleSide, depthWrite: false }),
    )
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
  for (const [id, m] of props.globes) {
    const grid = new PlanetGrid(m.frequency)
    globeGrids.set(id, grid)
    const g = createGlobeMesh(grid, m.colorOf, { ambient: 0.05, twilight: 0.12 })
    g.mesh.userData.globe = true
    globeViews.set(id, g)
  }
  buildBodies()
}

// Puts every body where it is at `days`, and each orbit round its parent.
const positions = new Map()
function place() {
  positions.clear()
  for (const [id, p] of scenePositions(props.bodies, props.days, undefined, props.relative)) positions.set(id, new THREE.Vector3(...p))
  for (const { body, mesh, spin, orbit } of meshes.values()) {
    mesh.position.copy(positions.get(body.id))
    // A turn a day (days are Earth days; day_hours its own).
    if (spin) spin.rotation.z = (((props.days * 24) / (body.day_hours || 24)) % 1) * 2 * Math.PI
    if (orbit) {
      const base = positions.get(body.parent_id) ?? new THREE.Vector3()
      orbit.group.position.copy(base)
      splitTrail(orbit, positions.get(body.id).clone().sub(base))
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
function render() {
  raf = 0
  if (!renderer) return
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
  const star = props.bodies.find((b) => b.kind === 'star')
  const sun = (star && positions.get(star.id)) || new THREE.Vector3()
  for (const g of globeViews.values()) {
    if (!g.mesh.parent) continue
    const local = g.mesh.worldToLocal(sun.clone())
    g.material.uniforms.uLight.value.copy(local.normalize())
  }
  // Inside a Dyson sphere the star gives no light.
  const power = star?.dyson ? 0 : 1
  for (const m of sunLit) {
    m.uniforms.uSun.value.copy(sun)
    m.uniforms.uPower.value = power
  }
  for (const g of globeViews.values()) g.material.uniforms.uPower.value = power
  for (const ring of rings) ring.material.uniforms.uPlanet.value.copy(positions.get(ring.id) ?? sun)
  // A black hole bends the light of the sky behind it.
  if (skyMesh) {
    const lens = skyMesh.material.uniforms
    if (star?.star_class === 'black_hole') {
      const toHole = sun.clone().sub(camera.position)
      const dist = toHole.length()
      lens.uLensDir.value.copy(toHole.normalize())
      lens.uLensE.value = Math.atan((sceneRadius(star) * 3) / Math.max(dist, 1e-6))
    } else lens.uLensE.value = 0
  }
  // Clouds drift with the clock (slowly: a turn of the pattern is years).
  for (const m of cloudMaterials) m.uniforms.uTime.value = (props.days % 100000) * 0.004
  const time = performance.now() / 1000
  for (const s of stars) s.faceCamera(camera, time)
  renderer.render(scene, camera)
  placeLabels()
}

function placeLabels() {
  const w = renderer.domElement.clientWidth
  const h = renderer.domElement.clientHeight
  const out = []
  for (const { body } of meshes.values()) {
    const p = positions.get(body.id)
    if (!p) continue
    tmp.copy(p)
    tmp.z += sceneRadius(body) * 1.2
    tmp.project(camera)
    if (tmp.z > 1) continue
    out.push({ id: body.id, name: body.name, star: body.kind === 'star', x: ((tmp.x + 1) / 2) * w, y: ((1 - tmp.y) / 2) * h })
  }
  labels.value = out
}

function resize() {
  if (!renderer) return
  const w = wrapEl.value.clientWidth
  const h = wrapEl.value.clientHeight
  renderer.setSize(w, h, false)
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
  scene = new THREE.Scene()
  scene.background = new THREE.Color(0x02030a)
  camera = new THREE.PerspectiveCamera(45, 1, 0.005, 20000)
  camera.up.set(0, 0, 1)
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
  renderer?.dispose()
  renderer = null
  scene = null
})

watch(() => props.bodies, buildBodies, { deep: true })
watch(() => props.globes, buildGlobes)
watch(() => props.days, place)
watch(() => props.relative, place)

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

defineExpose({ frame: () => {
  follow = null
  frame()
  requestRender()
} })
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
