<template>
  <div ref="wrapEl" class="globe-wrap" :class="cursorClass">
    <canvas
      ref="canvasEl"
      class="globe-canvas"
      @pointerdown="onPointerDown"
      @pointermove="onPointerMove"
      @pointerup="onPointerUp"
      @pointercancel="onPointerUp"
      @pointerleave="onPointerLeave"
      @contextmenu.prevent
    ></canvas>
    <div class="labels" aria-hidden="true">
      <template v-for="l in labels" :key="l.key">
        <span
          v-if="l.kind === 'area'"
          class="area-name"
          :style="{ transform: `translate(${l.x}px, ${l.y}px) translate(-50%, -50%)`, fontSize: l.size + 'px', '--area': l.color }"
        >{{ l.name }}</span>
        <div v-else class="marker" :style="{ transform: `translate(${l.x}px, ${l.y}px) translate(-50%, ${-l.icon / 2}px)` }">
          <img :src="markerIcon(l.rank, l.icon)" :width="l.icon" :height="l.icon" alt="" />
          <span v-if="l.named" class="label" :class="'rank-' + l.rank">{{ l.name }}</span>
        </div>
      </template>
    </div>
    <p v-if="building" class="building">{{ $t('Building the planet…') }}</p>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import * as THREE from 'three'
import { OrbitControls } from 'three/addons/controls/OrbitControls.js'
import { LineSegments2 } from 'three/addons/lines/LineSegments2.js'
import { LineSegmentsGeometry } from 'three/addons/lines/LineSegmentsGeometry.js'
import { LineMaterial } from 'three/addons/lines/LineMaterial.js'
import { PlanetGrid, cellsWithin, normalize, territoryEdges } from '../planetGrid'
import { pickLabels } from '../planetLabels'
import { markerIcon } from '../mapMarkers'

// The planet drawn in 3D: every cell of the grid (planetGrid.js) coloured by
// colorOf, with an outline once cells are big enough on screen. Drag to turn
// it, scroll to zoom. With the paint or erase tool a drag paints instead
// (hold Space to turn it): the component works out the cells under the
// brush and emits them; the parent decides what painting means and calls
// recolor. Places are markers; names show by zoom (planetLabels.js), the
// names of kingdoms, regions and oceans at the middle of their land.
const props = defineProps({
  frequency: { type: Number, required: true },
  // cell -> '#rrggbb', or '' for open sea
  colorOf: { type: Function, required: true },
  // Bumped by the parent when every cell may have changed colour.
  version: { type: Number, default: 0 },
  // [{ cell, rank (0 place, 1 city, 2 capital), name }]
  markers: { type: Array, default: () => [] },
  // { equator, zones, tilt }
  lines: { type: Object, default: () => ({ equator: false, zones: false, tilt: 23.44 }) },
  tool: { type: String, default: 'select' }, // select | paint | erase | pan
  brush: { type: Number, default: 0 },
  selected: { type: Number, default: null },
  // Cells to outline (a location picked in the bar).
  outlined: { type: Array, default: () => [] },
  // A cell to face when the globe is first drawn.
  focus: { type: Number, default: null },
  // Territory names: [{ key, name, color, cell (their middle), size (px) }]
  areas: { type: Array, default: () => [] },
  // Territories drawn as an outline only (oceans): [{ color, cells }]
  borders: { type: Array, default: () => [] },
  // Pixels covered on the right (the side list): the globe centres beside them.
  insetRight: { type: Number, default: 0 },
})
const emit = defineEmits(['hover', 'select', 'paint-start', 'paint', 'paint-end'])

const wrapEl = ref(null)
const canvasEl = ref(null)
const labels = ref([])
const building = ref(true)
const spaceDown = ref(false)
const dragging = ref(false)

const SEA = [27, 45, 74]
const COLUMNS = 2048 // the colour texture's width

let grid = null
let renderer, scene, camera, controls
let globe, colorTex, colorData, globeMat
let borderGroup = null
let lineGroup = null
let hoverLine = null
let selectLine = null
let outlineLines = null
let raf = 0
let resizeObserver = null

const painting = computed(() => (props.tool === 'paint' || props.tool === 'erase') && !spaceDown.value)
const cursorClass = computed(() => ({
  'is-painting': painting.value,
  'is-grabbing': dragging.value && !painting.value,
}))

// ---- Building --------------------------------------------------------------------
const hexRgb = (hex) => {
  const n = parseInt(hex.slice(1), 16)
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255]
}

function writeColor(cell) {
  const c = props.colorOf(cell)
  const [r, g, b] = c ? hexRgb(c) : SEA
  colorData[cell * 4] = r
  colorData[cell * 4 + 1] = g
  colorData[cell * 4 + 2] = b
  colorData[cell * 4 + 3] = 255
}

function recolorAll() {
  if (!colorData) return
  for (let c = 0; c < grid.count; c++) writeColor(c)
  colorTex.needsUpdate = true
  requestRender()
}

// Recolours some cells (after painting them).
function recolor(cells) {
  if (!colorData) return
  for (const c of cells) writeColor(c)
  colorTex.needsUpdate = true
  requestRender()
}

const VERTEX = `
attribute float aCell;
attribute float aEdge;
uniform sampler2D uColors;
uniform vec2 uSize;
varying vec3 vColor;
varying float vEdge;
varying vec3 vNormal;
void main() {
  float x = mod(aCell, uSize.x);
  float y = floor(aCell / uSize.x);
  vColor = texture2D(uColors, vec2((x + 0.5) / uSize.x, (y + 0.5) / uSize.y)).rgb;
  vEdge = aEdge;
  vNormal = normalize(position);
  gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
}`

const FRAGMENT = `
uniform vec3 uLight;
uniform float uGrid;
varying vec3 vColor;
varying float vEdge;
varying vec3 vNormal;
void main() {
  float light = 0.6 + 0.4 * max(dot(vNormal, uLight), 0.0);
  vec3 c = vColor * light;
  float line = smoothstep(0.84, 0.96, vEdge) * uGrid;
  gl_FragColor = vec4(mix(c, c * 0.5, line), 1.0);
}`

// One fan of triangles per cell: its centre and its corners, each vertex
// carrying the cell's number (for its colour) and whether it is a corner
// (for the outline).
function buildGlobe() {
  const n = grid.count
  const pos = new Float32Array(n * 7 * 3)
  const cellAttr = new Float32Array(n * 7)
  const edge = new Float32Array(n * 7)
  const index = new Uint32Array(n * 6 * 3)
  let v = 0
  let ix = 0
  for (let c = 0; c < n; c++) {
    const corners = grid.corners(c)
    const centre = grid.center(c)
    const base = v
    pos.set(centre, v * 3)
    cellAttr[v] = c
    edge[v] = 0
    v++
    for (const k of corners) {
      pos.set(k, v * 3)
      cellAttr[v] = c
      edge[v] = 1
      v++
    }
    const m = corners.length
    for (let k = 0; k < m; k++) {
      index[ix++] = base
      index[ix++] = base + 1 + k
      index[ix++] = base + 1 + ((k + 1) % m)
    }
  }
  const geo = new THREE.BufferGeometry()
  geo.setAttribute('position', new THREE.BufferAttribute(pos.subarray(0, v * 3), 3))
  geo.setAttribute('aCell', new THREE.BufferAttribute(cellAttr.subarray(0, v), 1))
  geo.setAttribute('aEdge', new THREE.BufferAttribute(edge.subarray(0, v), 1))
  geo.setIndex(new THREE.BufferAttribute(index.subarray(0, ix), 1))

  const rows = Math.ceil(n / COLUMNS)
  colorData = new Uint8Array(COLUMNS * rows * 4)
  colorTex = new THREE.DataTexture(colorData, COLUMNS, rows, THREE.RGBAFormat, THREE.UnsignedByteType)
  colorTex.magFilter = THREE.NearestFilter
  colorTex.minFilter = THREE.NearestFilter
  globeMat = new THREE.ShaderMaterial({
    vertexShader: VERTEX,
    fragmentShader: FRAGMENT,
    uniforms: {
      uColors: { value: colorTex },
      uSize: { value: new THREE.Vector2(COLUMNS, rows) },
      uLight: { value: new THREE.Vector3(0, 0, 1) },
      uGrid: { value: 0 },
    },
  })
  globe = new THREE.Mesh(geo, globeMat)
  scene.add(globe)
  recolorAll()
}

// ---- Lines, markers, highlights ---------------------------------------------------
const LINE_COLORS = { equator: 0xe85646, tropic: 0xeca840, polar: 0x78b4ec }

function circleAt(latDeg, color) {
  const lat = (latDeg * Math.PI) / 180
  const pts = []
  for (let k = 0; k <= 180; k++) {
    const lon = (k / 180) * Math.PI * 2
    pts.push(new THREE.Vector3(Math.cos(lat) * Math.cos(lon), Math.cos(lat) * Math.sin(lon), Math.sin(lat)).multiplyScalar(1.002))
  }
  const line = new THREE.Line(
    new THREE.BufferGeometry().setFromPoints(pts),
    new THREE.LineDashedMaterial({ color, dashSize: 0.02, gapSize: 0.012 }),
  )
  line.computeLineDistances()
  return line
}

function buildLines() {
  if (lineGroup) {
    scene.remove(lineGroup)
    lineGroup.traverse((o) => o.geometry?.dispose())
  }
  lineGroup = new THREE.Group()
  const { equator, zones, tilt } = props.lines
  if (zones && tilt > 0 && tilt < 90) {
    for (const lat of [tilt, -tilt]) lineGroup.add(circleAt(lat, LINE_COLORS.tropic))
    if (90 - tilt !== tilt) for (const lat of [90 - tilt, tilt - 90]) lineGroup.add(circleAt(lat, LINE_COLORS.polar))
  }
  if (equator) lineGroup.add(circleAt(0, LINE_COLORS.equator))
  scene.add(lineGroup)
  requestRender()
}

const markerColor = () => {
  const v = getComputedStyle(document.documentElement).getPropertyValue('--accent').trim()
  return new THREE.Color(v || '#e6c06a')
}

// Oceans and the like: the edge of their territory, in their colour.
function buildBorders() {
  if (borderGroup) {
    scene.remove(borderGroup)
    borderGroup.traverse((o) => {
      o.geometry?.dispose()
      o.material?.dispose?.()
    })
  }
  borderGroup = new THREE.Group()
  for (const b of props.borders) {
    const pts = []
    for (const [a, c] of territoryEdges(grid, b.cells.filter((x) => x < grid.count))) {
      for (const v of [a, c]) pts.push(v[0] * 1.0012, v[1] * 1.0012, v[2] * 1.0012)
    }
    if (!pts.length) continue
    // Plain WebGL lines are one pixel wide; these keep a width on screen.
    const geo = new LineSegmentsGeometry().setPositions(pts)
    const mat = new LineMaterial({ color: b.color, linewidth: 2.5 })
    mat.resolution.set(renderer.domElement.width, renderer.domElement.height)
    borderGroup.add(new LineSegments2(geo, mat))
  }
  scene.add(borderGroup)
  requestRender()
}

// An outline round some cells, just above the surface.
function outlineOf(cells, color, lift = 1.0015) {
  const pts = []
  for (const c of cells) {
    const corners = grid.corners(c)
    for (let k = 0; k < corners.length; k++) {
      pts.push(new THREE.Vector3(...corners[k]).multiplyScalar(lift))
      pts.push(new THREE.Vector3(...corners[(k + 1) % corners.length]).multiplyScalar(lift))
    }
  }
  return new THREE.LineSegments(new THREE.BufferGeometry().setFromPoints(pts), new THREE.LineBasicMaterial({ color }))
}

function replace(old, next) {
  if (old) {
    scene.remove(old)
    old.geometry.dispose()
  }
  if (next) scene.add(next)
  requestRender()
  return next
}

let hoverCell = null
function setHover(cell) {
  if (cell === hoverCell) return
  hoverCell = cell
  emit('hover', cell)
  const cells = cell == null ? [] : painting.value && props.brush ? cellsWithin(grid, cell, props.brush) : [cell]
  hoverLine = replace(hoverLine, cells.length ? outlineOf(cells, 0xffffff) : null)
}

function drawSelected() {
  selectLine = replace(selectLine, props.selected != null ? outlineOf([props.selected], markerColor().getHex(), 1.002) : null)
}

function drawOutlined() {
  outlineLines = replace(outlineLines, props.outlined.length ? outlineOf(props.outlined, markerColor().getHex(), 1.0018) : null)
}

// ---- Rendering ---------------------------------------------------------------------
function requestRender() {
  if (!raf) raf = requestAnimationFrame(render)
}

function render() {
  raf = 0
  // Not built yet (the grid is made a moment after mounting).
  if (!renderer || !globeMat) return
  const dist = camera.position.length()
  globeMat.uniforms.uLight.value.copy(camera.position).normalize()
  // Outlines once a cell is about 7 pixels across.
  const cellPx = ((Math.atan(2) / grid.n) / ((dist - 1) * Math.tan((camera.fov * Math.PI) / 360))) * (renderer.domElement.clientHeight / 2)
  globeMat.uniforms.uGrid.value = Math.max(0, Math.min(1, (cellPx - 5) / 6))
  // Turning slows as you zoom in.
  controls.rotateSpeed = Math.max(0.05, Math.min(1, (dist - 1) * 0.5))
  renderer.render(scene, camera)
  placeLabels(dist, cellPx)
}

// Markers and names on the side facing the camera, by zoom (planetLabels.js).
function placeLabels(dist, cellPx) {
  const w = renderer.domElement.clientWidth
  const h = renderer.domElement.clientHeight
  const eye = camera.position.clone().normalize()
  const onScreen = (cell, limit) => {
    if (cell == null || cell >= grid.count) return null
    const p = new THREE.Vector3(...grid.center(cell))
    if (p.dot(eye) < limit) return null
    p.multiplyScalar(1.003).project(camera)
    return [((p.x + 1) / 2) * w, ((1 - p.y) / 2) * h]
  }
  const candidates = []
  for (const a of props.areas) {
    const at = onScreen(a.cell, 0.35)
    if (at) candidates.push({ kind: 'area', key: 'a:' + a.key, name: a.name, x: at[0], y: at[1], size: a.size, color: a.color })
  }
  for (const m of props.markers) {
    const at = onScreen(m.cell, 0.2)
    if (at) candidates.push({ kind: 'marker', key: m.cell + ':' + m.name, name: m.name, rank: m.rank, x: at[0], y: at[1] })
  }
  labels.value = pickLabels(candidates, cellPx)
}

function resize() {
  const el = wrapEl.value
  if (!el || !renderer) return
  const w = el.clientWidth
  const h = el.clientHeight
  renderer.setSize(w, h, false)
  borderGroup?.traverse((o) => o.material?.resolution?.set(renderer.domElement.width, renderer.domElement.height))
  // Drawn as the left part of a wider view, so the globe's centre sits in
  // the middle of the space the side list leaves.
  const inset = Math.min(props.insetRight, w / 2)
  camera.aspect = (w + inset) / Math.max(1, h)
  if (inset) camera.setViewOffset(w + inset, h, inset, 0, w, h)
  else camera.clearViewOffset()
  camera.updateProjectionMatrix()
  requestRender()
}

// ---- Pointer -------------------------------------------------------------------------
const raycaster = new THREE.Raycaster()
const unitSphere = new THREE.Sphere(new THREE.Vector3(), 1)

// The point on the globe under the pointer, or null off it.
function pointAt(e) {
  const rect = canvasEl.value.getBoundingClientRect()
  const ndc = new THREE.Vector2(((e.clientX - rect.left) / rect.width) * 2 - 1, -((e.clientY - rect.top) / rect.height) * 2 + 1)
  raycaster.setFromCamera(ndc, camera)
  const hit = raycaster.ray.intersectSphere(unitSphere, new THREE.Vector3())
  return hit ? [hit.x, hit.y, hit.z] : null
}

let down = null // { x, y, moved }
let stroke = null // { last: point }

function cellsAlong(a, b) {
  // Steps of half a cell along the arc from a to b, so a fast drag leaves
  // no gaps.
  const angle = Math.acos(Math.max(-1, Math.min(1, a[0] * b[0] + a[1] * b[1] + a[2] * b[2])))
  const steps = Math.max(1, Math.ceil(angle / (Math.atan(2) / grid.n / 2)))
  const out = new Set()
  for (let k = 1; k <= steps; k++) {
    const t = k / steps
    const p = normalize([a[0] + (b[0] - a[0]) * t, a[1] + (b[1] - a[1]) * t, a[2] + (b[2] - a[2]) * t])
    out.add(grid.cellAt(p))
  }
  return [...out]
}

function brushed(cells) {
  if (!props.brush) return cells
  const all = new Set()
  for (const c of cells) for (const x of cellsWithin(grid, c, props.brush)) all.add(x)
  return [...all]
}

function onPointerDown(e) {
  down = { x: e.clientX, y: e.clientY, moved: false }
  dragging.value = true
  if (e.button === 0 && painting.value) {
    const p = pointAt(e)
    if (!p) return
    try {
      canvasEl.value.setPointerCapture(e.pointerId)
    } catch {
      // Not a live pointer (it already lifted); painting works without.
    }
    stroke = { last: p }
    emit('paint-start')
    emit('paint', brushed([grid.cellAt(p)]))
  }
}

function onPointerMove(e) {
  if (down && Math.hypot(e.clientX - down.x, e.clientY - down.y) > 4) down.moved = true
  const p = pointAt(e)
  setHover(p ? grid.cellAt(p) : null)
  if (stroke && p) {
    emit('paint', brushed(cellsAlong(stroke.last, p)))
    stroke.last = p
  }
}

function onPointerUp(e) {
  dragging.value = false
  if (canvasEl.value?.hasPointerCapture?.(e.pointerId)) canvasEl.value.releasePointerCapture(e.pointerId)
  if (stroke) {
    stroke = null
    emit('paint-end')
  } else if (down && !down.moved && e.button === 0 && !painting.value) {
    const p = pointAt(e)
    if (p) emit('select', grid.cellAt(p))
  }
  down = null
}

function onPointerLeave() {
  if (!stroke) setHover(null)
}

function onKeyDown(e) {
  const t = e.target
  if (e.code === 'Space' && !(t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.tagName === 'SELECT' || t.isContentEditable))) {
    if (document.activeElement?.tagName === 'BUTTON') document.activeElement.blur()
    e.preventDefault()
    spaceDown.value = true
  }
}
const onKeyUp = (e) => e.code === 'Space' && (spaceDown.value = false)
const onBlur = () => (spaceDown.value = false)

// Left-drag turns the globe unless painting; right-drag always does.
function setControls() {
  if (!controls) return
  controls.mouseButtons = {
    LEFT: painting.value ? null : THREE.MOUSE.ROTATE,
    MIDDLE: THREE.MOUSE.DOLLY,
    RIGHT: THREE.MOUSE.ROTATE,
  }
  controls.touches = { ONE: painting.value ? null : THREE.TOUCH.ROTATE, TWO: THREE.TOUCH.DOLLY_ROTATE }
}

// Turns the globe so a cell faces the camera.
function focusOn(cell) {
  if (!grid || cell == null) return
  const dist = camera.position.length()
  camera.position.set(...grid.center(cell)).multiplyScalar(dist)
  camera.lookAt(0, 0, 0)
  controls.update()
  requestRender()
}

// ---- Lifecycle -------------------------------------------------------------------------
function setup() {
  renderer = new THREE.WebGLRenderer({ canvas: canvasEl.value, antialias: true, alpha: true })
  renderer.setPixelRatio(window.devicePixelRatio || 1)
  scene = new THREE.Scene()
  camera = new THREE.PerspectiveCamera(40, 1, 0.001, 50)
  camera.up.set(0, 0, 1)
  camera.position.set(3.2, 0, 0.8)
  camera.lookAt(0, 0, 0)
  controls = new OrbitControls(camera, canvasEl.value)
  controls.enablePan = false
  controls.minDistance = 1.03
  controls.maxDistance = 6
  controls.zoomSpeed = 0.8
  controls.addEventListener('change', requestRender)
  setControls()
}

function rebuild() {
  building.value = true
  // Let the "building" note paint before the heavy work.
  setTimeout(() => {
    if (globe) {
      scene.remove(globe)
      globe.geometry.dispose()
      globeMat.dispose()
      colorTex.dispose()
    }
    grid = new PlanetGrid(props.frequency)
    buildGlobe()
    buildLines()
    buildBorders()
    drawSelected()
    drawOutlined()
    building.value = false
    resize()
    if (props.focus != null) focusOn(props.focus)
  }, 30)
}

onMounted(() => {
  setup()
  rebuild()
  resizeObserver = new ResizeObserver(resize)
  resizeObserver.observe(wrapEl.value)
  window.addEventListener('keydown', onKeyDown)
  window.addEventListener('keyup', onKeyUp)
  window.addEventListener('blur', onBlur)
})

onBeforeUnmount(() => {
  cancelAnimationFrame(raf)
  resizeObserver?.disconnect()
  window.removeEventListener('keydown', onKeyDown)
  window.removeEventListener('keyup', onKeyUp)
  window.removeEventListener('blur', onBlur)
  controls?.dispose()
  scene?.traverse((o) => {
    o.geometry?.dispose()
    o.material?.dispose?.()
  })
  colorTex?.dispose()
  renderer?.dispose()
  renderer = null
})

watch(() => props.frequency, rebuild)
watch(() => props.version, recolorAll)
watch(() => [props.markers, props.areas], requestRender)
watch(() => props.borders, () => grid && buildBorders())
watch(() => props.lines, () => grid && buildLines(), { deep: true })
watch(() => props.selected, () => grid && drawSelected())
watch(() => props.outlined, () => grid && drawOutlined())
watch(painting, setControls)
watch(() => props.insetRight, resize)

defineExpose({ recolor, focusOn })
</script>

<style scoped>
.globe-wrap {
  position: relative;
  width: 100%;
  height: 100%;
  overflow: hidden;
  cursor: grab;
}

.globe-wrap.is-grabbing {
  cursor: grabbing;
}

.globe-wrap.is-painting {
  cursor: crosshair;
}

.globe-canvas {
  display: block;
  width: 100%;
  height: 100%;
  touch-action: none;
}

.labels {
  position: absolute;
  inset: 0;
  pointer-events: none;
  overflow: hidden;
}

.label {
  position: absolute;
  top: 0;
  left: 0;
  padding: 0.05rem 0.35rem;
  border-radius: 999px;
  background: color-mix(in srgb, var(--surface) 82%, transparent);
  color: var(--text-primary);
  font-size: 0.7rem;
  font-weight: 600;
  white-space: nowrap;
}

.marker {
  position: absolute;
  top: 0;
  left: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.marker img {
  display: block;
}

.marker .label {
  position: static;
  margin-top: -2px;
}

.area-name {
  position: absolute;
  top: 0;
  left: 0;
  color: color-mix(in srgb, var(--area) 55%, #ffffff);
  font-family: 'Fraunces', serif;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  white-space: nowrap;
  text-shadow:
    0 0 3px rgba(0, 0, 0, 0.9),
    0 0 8px rgba(0, 0, 0, 0.6);
}

.label.rank-2 {
  border: 1px solid var(--accent);
  color: var(--accent);
  font-family: 'Fraunces', serif;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
}

.building {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 0;
  color: var(--text-muted);
  font-size: 0.9rem;
  pointer-events: none;
}
</style>
