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
import { PlanetGrid } from '../planetGrid'
import { sceneRadius, sceneDistance, scenePositions } from '../solarScale'
import { createGlobeMesh } from '../three/globeMesh'
import { loadSkyDome, disposeSkyDome } from '../three/skyDome'

// The solar system in 3D: the star at the centre, each body on its orbit at
// the time `days`, drawn at the squeezed scale of solarScale.js. The main
// planet wears the map (three/globeMesh.js); the rest are plain spheres in
// their colour. Click a body to pick it; the camera then follows it. The
// physics is all in orbits.js: this only draws.
const props = defineProps({
  bodies: { type: Array, default: () => [] },
  days: { type: Number, default: 0 },
  selected: { type: Number, default: null },
  // The map on the main planet: { frequency, colorOf, version }, or null for
  // a story whose map is flat.
  globe: { type: Object, default: null },
  // The main planet's axial tilt, in degrees.
  tilt: { type: Number, default: 0 },
  sky: { type: String, default: '' },
})
const emit = defineEmits(['select'])

const wrapEl = ref(null)
const canvasEl = ref(null)
const labels = ref([])

const TYPE_COLORS = { star: '#ffd27a', rocky: '#9c8a74', gas: '#d9b38c', ocean: '#3f7fbf' }
const colorOf = (b) => b.color || TYPE_COLORS[b.body_type] || TYPE_COLORS.rocky

let renderer, scene, camera, controls
let raf = 0
let resizeObserver = null
let skyMesh = null
let cancelSky = () => {}
let globeView = null // the map, built once and moved onto the main planet
let bodyGroup = null // everything rebuilt when the bodies change
const meshes = new Map() // body id -> { body, mesh, orbit (line, round its parent) }
let follow = null // { id, last: Vector3 }

// ---- Building ----------------------------------------------------------------------
function disposeGroup(g) {
  g?.traverse((o) => {
    if (o === globeView?.mesh) return
    o.geometry?.dispose()
    o.material?.dispose?.()
  })
}

function orbitLine(radius) {
  const pts = []
  for (let k = 0; k <= 128; k++) {
    const a = (k / 128) * Math.PI * 2
    pts.push(new THREE.Vector3(Math.cos(a) * radius, Math.sin(a) * radius, 0))
  }
  return new THREE.Line(
    new THREE.BufferGeometry().setFromPoints(pts),
    new THREE.LineBasicMaterial({ color: 0x8aa0c8, transparent: true, opacity: 0.35 }),
  )
}

function buildBodies() {
  if (bodyGroup) {
    if (globeView) globeView.mesh.parent?.remove(globeView.mesh)
    scene.remove(bodyGroup)
    disposeGroup(bodyGroup)
  }
  bodyGroup = new THREE.Group()
  meshes.clear()
  const byId = new Map(props.bodies.map((b) => [b.id, b]))
  for (const b of props.bodies) {
    const r = sceneRadius(b)
    let mesh
    if (b.kind === 'star') {
      mesh = new THREE.Mesh(new THREE.SphereGeometry(r, 48, 24), new THREE.MeshBasicMaterial({ color: colorOf(b) }))
      // A soft glow round it.
      mesh.add(new THREE.Mesh(
        new THREE.SphereGeometry(r * 1.35, 32, 16),
        new THREE.MeshBasicMaterial({ color: colorOf(b), transparent: true, opacity: 0.18, depthWrite: false }),
      ))
    } else if (b.is_main && globeView) {
      // The map: its north pole tilted away from the orbit's pole.
      mesh = new THREE.Group()
      globeView.mesh.scale.setScalar(r)
      globeView.mesh.rotation.set((props.tilt * Math.PI) / 180, 0, 0)
      mesh.add(globeView.mesh)
    } else {
      mesh = new THREE.Mesh(
        new THREE.SphereGeometry(r, 32, 16),
        new THREE.MeshStandardMaterial({ color: colorOf(b), roughness: 0.9, metalness: 0 }),
      )
    }
    mesh.userData.id = b.id
    mesh.traverse((o) => (o.userData.id = b.id))
    bodyGroup.add(mesh)
    let orbit = null
    const parent = byId.get(b.parent_id)
    if (parent && b.orbit_km) {
      orbit = orbitLine(sceneDistance(b.orbit_km, parent, b))
      bodyGroup.add(orbit)
    }
    meshes.set(b.id, { body: b, mesh, orbit })
  }
  scene.add(bodyGroup)
  place()
}

function buildGlobe() {
  if (globeView) {
    globeView.mesh.parent?.remove(globeView.mesh)
    globeView.dispose()
    globeView = null
  }
  if (props.globe) {
    globeView = createGlobeMesh(new PlanetGrid(props.globe.frequency), props.globe.colorOf, { ambient: 0.06 })
  }
  buildBodies()
}

// Puts every body where it is at `days`, and each orbit round its parent.
const positions = new Map()
function place() {
  positions.clear()
  for (const [id, p] of scenePositions(props.bodies, props.days)) positions.set(id, new THREE.Vector3(...p))
  for (const { body, mesh, orbit } of meshes.values()) {
    mesh.position.copy(positions.get(body.id))
    if (orbit) orbit.position.copy(positions.get(body.parent_id) ?? new THREE.Vector3())
  }
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
  // The map is lit by the star: its direction, in the globe's own frame.
  if (globeView?.mesh.parent) {
    const local = globeView.mesh.worldToLocal(new THREE.Vector3(0, 0, 0))
    globeView.material.uniforms.uLight.value.copy(local.normalize())
  }
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
  const hit = ray.intersectObjects(bodyGroup?.children.filter((o) => o.type !== 'Line') ?? [], true)[0]
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
  buildGlobe()
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
  globeView?.dispose()
  if (skyMesh) disposeSkyDome(skyMesh)
  renderer?.dispose()
  renderer = null
  scene = null
})

watch(() => props.bodies, buildBodies, { deep: true })
watch(() => props.globe, buildGlobe)
watch(() => props.globe?.version, () => {
  globeView?.recolorAll()
  requestRender()
})
watch(() => props.tilt, buildBodies)
watch(() => props.days, place)
watch(() => props.selected, followSelected)
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
