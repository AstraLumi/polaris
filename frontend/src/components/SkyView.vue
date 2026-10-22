<template>
  <div class="sky-view" role="dialog" :aria-label="$t('The sky from this hex')" @keydown.esc="$emit('close')">
    <div ref="wrapEl" class="sky-wrap" :class="{ dragging: drag }" @pointerdown="onDown" @pointermove="onMove" @pointerup="onUp" @pointercancel="onUp" @wheel.prevent="onWheel">
      <canvas ref="canvasEl" class="sky-canvas"></canvas>
      <div class="labels" aria-hidden="true">
        <span v-for="l in labels" :key="l.key" class="label" :class="l.kind" :style="{ transform: `translate(${l.x}px, ${l.y}px) translate(-50%, -50%)` }">{{ l.text }}</span>
      </div>
    </div>

    <header class="sky-bar glass-panel panel-solid">
      <strong class="where">{{ $t('The sky from {place}', { place: placeText }) }}</strong>
      <div class="date-box">
        <DateField v-model="dateText" :label="$t('Date')" />
      </div>
      <label class="time-box">
        <span>{{ $t('Local time') }} <b>{{ timeText }}</b></span>
        <input v-model.number="localHour" type="range" min="0" :max="hoursPerDay" step="0.05" :aria-label="$t('Local time')" />
      </label>
      <div class="steps">
        <button type="button" class="tool-btn" :title="$t('An hour earlier')" @click="shiftHours(-1)">−1 h</button>
        <button type="button" class="tool-btn" :title="$t('An hour later')" @click="shiftHours(1)">+1 h</button>
        <button type="button" class="tool-btn" :title="$t('A day earlier')" @click="shiftDays(-1)">−1 d</button>
        <button type="button" class="tool-btn" :title="$t('A day later')" @click="shiftDays(1)">+1 d</button>
        <button type="button" class="tool-btn" :title="$t('A month later')" @click="shiftDays(cal.days_per_month)">+1 m</button>
        <button type="button" class="tool-btn" @click="localHour = 0">{{ $t('Midnight') }}</button>
      </div>
      <div class="toggles">
        <button type="button" class="tool-btn" :class="{ active: show.grid }" @click="show.grid = !show.grid">{{ $t('Grid') }}</button>
        <button type="button" class="tool-btn" :class="{ active: show.labels }" @click="show.labels = !show.labels">{{ $t('Names') }}</button>
        <button type="button" class="tool-btn" :class="{ active: show.clouds }" @click="show.clouds = !show.clouds">{{ $t('Clouds') }}</button>
      </div>
      <button type="button" class="btn btn-ghost close" @click="$emit('close')">{{ $t('Close') }}</button>
    </header>

    <aside class="sky-info glass-panel panel-solid">
      <p v-if="error" class="error-banner">{{ error }}</p>
      <template v-else-if="sky">
        <p class="row"><span>{{ $t('Field of view') }}</span><strong>{{ Math.round(fov) }}°</strong></p>
        <p class="row"><span>{{ sunName }}</span><strong>{{ sunText }}</strong></p>
        <p v-if="sky.shell" class="row"><span>{{ $t('The shell') }}</span><strong>{{ sky.shell.glow > 0.5 ? $t('Day') : sky.shell.glow > 0 ? $t('Dawn or dusk') : $t('Night: only its painted stars shine') }}</strong></p>
        <p class="row"><span>{{ $t('Cloud cover') }}</span><strong>{{ cloudText }}</strong></p>
        <h4 v-if="moons.length">{{ $t('Moons') }}</h4>
        <div v-for="m in moons" :key="m.body.id" class="body-row">
          <span class="dot" :style="{ background: colorOf(m.body) }"></span>
          <span class="name">{{ m.body.name }}</span>
          <span class="facts">{{ moonText(m) }}</span>
        </div>
        <h4 v-if="planets.length">{{ $t('Planets') }}</h4>
        <div v-for="p in planets" :key="p.body.id" class="body-row">
          <span class="dot" :style="{ background: colorOf(p.body) }"></span>
          <span class="name">{{ p.body.name }}</span>
          <span class="facts">{{ planetText(p) }}</span>
        </div>
        <p class="note">{{ starNote }}</p>
        <p class="note">{{ $t('Drag to look round, scroll to zoom. Sizes and brightness are true to scale.') }}</p>
      </template>
      <p v-else class="note">{{ $t('Working out the sky…') }}</p>
    </aside>
  </div>
</template>

<script setup>
import { ref, shallowRef, reactive, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import * as THREE from 'three'
import { t } from '../i18n'
import { solarApi } from '../api'
import { calendar as cal, parseStoryDate, formatStoryDate } from '../calendar'
import { worldMap, skyUrl } from '../worldMap'
import { PlanetGrid, latLon } from '../planetGrid'
import { formatLatitude } from '../planet'
import { bodyColor } from '../solarScale'
import { enclosureFor } from '../lighting'
import { skyAt, simDaysOf, globalHour } from '../skyFrames'
import { stepOfMonth } from '../planetClimate'
import { loadStarCatalog } from '../useStarCatalog'
import { toThree, pictureToThree, createSkyDome, createPoints, createBody, createSun, createClouds, createGround, createGrid, BODY_DISTANCE } from '../three/skyShaders'
import DateField from './DateField.vue'

// The sky seen from one hex of the planet at a date and hour
// (docs/SKY_ROADMAP.md, stage 3): the stars found in the story's sky
// picture (starCatalog.js), the star, moons and planets where they stand,
// at their true sizes and brightness (skyFrames.js), the clouds of that
// month (the climate), the horizon. Drag to look round, scroll to zoom.
const props = defineProps({
  cell: { type: Number, required: true },
  frequency: { type: Number, required: true },
  // The planet's worked-out climate (planetClimate.js), for its clouds.
  climate: { type: Object, default: null },
})
defineEmits(['close'])

const DEG = Math.PI / 180
const wrapEl = ref(null)
const canvasEl = ref(null)
const labels = ref([])
const error = ref('')
const solar = ref(null)
const sky = ref(null)
const dateText = ref('01-01-0001')
const localHour = ref(0)
const fov = ref(70)
const view = reactive({ az: 180 * DEG, alt: 25 * DEG })
const show = reactive({ grid: false, labels: true, clouds: true })
const drag = ref(null)
const catalogue = shallowRef(null)

const hoursPerDay = computed(() => cal.hours_per_day || 24)
const grid = new PlanetGrid(props.frequency)
const [lat, lon] = latLon(grid.center(props.cell))
const lonText = (l) => {
  const v = Math.round(Math.abs(l / DEG))
  return v === 0 || v === 180 ? `${v}°` : `${v}°${l > 0 ? 'E' : 'W'}`
}
const placeText = `${formatLatitude(lat / DEG)}, ${lonText(lon)}`
const colorOf = (b) => bodyColor(b)

const date = computed(() => parseStoryDate(dateText.value) ?? { day: 1, month: 1, year: 1 })
const timeText = computed(() => {
  const h = Math.floor(localHour.value)
  const m = Math.floor((localHour.value - h) * 60)
  return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}`
})
const days = computed(() => simDaysOf(date.value, globalHour(localHour.value, lon, cal), cal))

// ---- Moving in time ----------------------------------------------------------------------
function shiftDays(n) {
  const perYear = cal.months_per_year * cal.days_per_month
  const d = date.value
  let index = ((d.year > 0 ? d.year - 1 : d.year) * cal.months_per_year + d.month - 1) * cal.days_per_month + d.day - 1 + n
  const yearIndex = Math.floor(index / perYear)
  index -= yearIndex * perYear
  dateText.value = formatStoryDate({ year: yearIndex >= 0 ? yearIndex + 1 : yearIndex, month: Math.floor(index / cal.days_per_month) + 1, day: (index % cal.days_per_month) + 1 })
}
function shiftHours(n) {
  let h = localHour.value + n
  if (h < 0) {
    h += hoursPerDay.value
    shiftDays(-1)
  } else if (h >= hoursPerDay.value) {
    h -= hoursPerDay.value
    shiftDays(1)
  }
  localHour.value = h
}

// ---- What the view says ---------------------------------------------------------------------
const deg = (r, digits = 0) => `${(r / DEG).toFixed(digits)}°`
const compass = (az) => ['N', 'NE', 'E', 'SE', 'S', 'SW', 'W', 'NW'][Math.round((((az / DEG) % 360) + 360) % 360 / 45) % 8]
const sunName = computed(() => solar.value?.bodies.find((b) => b.kind === 'star')?.name || t('The star'))
const sunText = computed(() => {
  const s = sky.value?.sun
  if (!s) return ''
  if (!s.shines) return t('Hidden in its sphere')
  return s.alt > 0 ? t('{alt} up, {dir}', { alt: deg(s.alt), dir: compass(s.az) }) : t('{alt} below the horizon', { alt: deg(-s.alt) })
})
const moons = computed(() => (sky.value?.bodies ?? []).filter((b) => b.body.kind === 'moon'))
const planets = computed(() => (sky.value?.bodies ?? []).filter((b) => b.body.kind !== 'moon'))
function moonText(m) {
  const where = m.alt > 0 ? t('{alt} up, {dir}', { alt: deg(m.alt), dir: compass(m.az) }) : t('below the horizon')
  const size = deg(m.angularRadius * 2, 2)
  const light = m.eclipsed > 0.5 && m.direct < 0.5 ? t('eclipsed') : m.direct > 0 ? t('{n}% lit', { n: Math.round(m.lit * 100) }) : t('lit from all round')
  return `${where} · ${size} · ${light}`
}
function planetText(p) {
  const where = p.alt > 0 ? t('{alt} up, {dir}', { alt: deg(p.alt), dir: compass(p.az) }) : t('below the horizon')
  return Number.isFinite(p.mag) ? `${where} · ${t('magnitude {m}', { m: p.mag.toFixed(1) })}` : where
}
const cloudCover = computed(() => {
  if (!props.climate) return null
  const step = stepOfMonth(date.value.month - 1, cal)
  return props.climate.clouds(step)[props.cell]
})
const cloudText = computed(() => (cloudCover.value == null ? t('Working out…') : `${Math.round(cloudCover.value * 100)}%`))
const starNote = computed(() =>
  catalogue.value ? t('{n} stars found in the sky picture.', { n: catalogue.value.stars.length.toLocaleString() }) : t('Finding the stars in the sky picture…'),
)

// ---- Drawing ---------------------------------------------------------------------------------
let renderer, scene, camera, dome, stars, planetPoints, ground, gridLines, clouds, sunView
const bodyMeshes = new Map() // body id -> mesh
let raf = 0
let skyTexture = null

function requestRender() {
  if (!raf) raf = requestAnimationFrame(render)
}

function setup() {
  renderer = new THREE.WebGLRenderer({ canvas: canvasEl.value, antialias: true })
  renderer.setPixelRatio(window.devicePixelRatio || 1)
  scene = new THREE.Scene()
  camera = new THREE.PerspectiveCamera(fov.value, 1, 0.1, 2000)
  ground = createGround()
  gridLines = createGrid()
  clouds = createClouds()
  scene.add(ground, gridLines, clouds)
  new THREE.TextureLoader().load(skyUrl() || '/sky/default-sky.jpg', (tex) => {
    if (!renderer) return tex.dispose()
    tex.colorSpace = THREE.NoColorSpace
    tex.generateMipmaps = false
    tex.minFilter = THREE.LinearFilter
    tex.wrapS = THREE.RepeatWrapping
    skyTexture = tex
    // Until the stars are found, the whole picture is the sky.
    if (!dome) {
      dome = createSkyDome(tex)
      scene.add(dome)
      update()
    }
  })
  loadStarCatalog(skyUrl() || '/sky/default-sky.jpg')
    .then((found) => {
      if (!renderer) return
      catalogue.value = found
      const n = found.stars.length
      const dirs = new Float32Array(n * 3)
      const mags = new Float32Array(n)
      const colors = new Uint8Array(n * 3)
      found.stars.forEach((s, i) => {
        dirs.set(s.dir, i * 3)
        mags[i] = s.mag
        const c = parseInt(s.color.slice(1), 16)
        colors.set([(c >> 16) & 255, (c >> 8) & 255, c & 255], i * 3)
      })
      stars = createPoints(dirs, mags, colors)
      scene.add(stars)
      // The glow left once the stars are out becomes the sky behind them.
      const glow = new THREE.DataTexture(found.glow.data, found.glow.width, found.glow.height, THREE.RGBAFormat)
      glow.flipY = true
      glow.colorSpace = THREE.NoColorSpace
      glow.minFilter = THREE.LinearFilter
      glow.magFilter = THREE.LinearFilter
      glow.wrapS = THREE.RepeatWrapping
      glow.needsUpdate = true
      if (dome) {
        dome.material.uniforms.uSky.value = glow
      } else {
        dome = createSkyDome(glow)
        scene.add(dome)
      }
      update()
    })
    .catch(() => {})
}

// Everything that depends on the moment: worked out again, then drawn.
function update() {
  if (!solar.value || !renderer) return
  const p = worldMap.planet
  const s = skyAt(solar.value.bodies, lat, lon, days.value, { tilt: p.tilt, dayHours: hoursPerDay.value, enclosure: enclosureFor(solar.value) })
  if (!s) {
    error.value = t('The sky needs a main planet going round a star (Solar system tab).')
    return
  }
  error.value = ''
  sky.value = s
  const fromPicture = pictureToThree(s.stars)
  const toPicture = fromPicture.clone().transpose()

  // How light the air is: the star's height, or the shell's day.
  const sunDir = toThree(s.sun.local)
  const sunUp = s.sun.shines ? Math.max(0, Math.min(1, (s.sun.alt / DEG + 18) / 22)) ** 1.6 : 0
  const day = s.shell ? Math.max(sunUp, 0) : sunUp
  const shellGlow = s.shell ? s.shell.glow : 0
  if (dome) {
    const u = dome.material.uniforms
    u.uToPicture.value.copy(toPicture)
    u.uDay.value = day
    u.uSunDir.value.copy(sunDir)
    u.uAir.value.setRGB(...mixRgb([0.55, 0.3, 0.16], [0.32, 0.55, 0.95], Math.max(0, Math.min(1, (s.sun.alt / DEG + 4) / 14))))
    u.uShell.value = shellGlow
    u.uGain.value = stars ? 0.85 : 1
  }
  const darkness = (1 - day) * (1 - shellGlow)
  // The faintest star seen: about magnitude 6 on a dark night, more
  // through a narrow view (a telescope's), fewer as the sky brightens.
  const brightest = Math.max(0, ...s.bodies.filter((b) => b.alt > 0 && b.body.kind === 'moon').map((b) => Math.max(0, -b.mag - 4) / 9))
  const limit = 6.2 + 2.5 * Math.log10(70 / fov.value) - 9 * (1 - darkness) - 1.2 * brightest
  for (const pts of [stars, planetPoints]) {
    if (!pts) continue
    pts.material.uniforms.uLimit.value = limit
    pts.material.uniforms.uPx.value = renderer.getPixelRatio()
  }
  if (stars) stars.material.uniforms.uFromPicture.value.copy(fromPicture)

  // Bodies: a lit sphere at its true size; a point too while it is too
  // small to see as a disc.
  for (const mesh of bodyMeshes.values()) mesh.visible = false
  const pdirs = []
  const pmags = []
  const pcolors = []
  const pxPerRad = (renderer.domElement.clientHeight / 2) / Math.tan((fov.value * DEG) / 2)
  for (const b of s.bodies) {
    let mesh = bodyMeshes.get(b.body.id)
    if (!mesh) {
      mesh = createBody(colorOf(b.body))
      bodyMeshes.set(b.body.id, mesh)
      scene.add(mesh)
    }
    const dir = toThree(b.local)
    const r = BODY_DISTANCE * Math.tan(b.angularRadius)
    mesh.position.copy(dir).multiplyScalar(BODY_DISTANCE)
    mesh.scale.setScalar(r)
    mesh.visible = true
    const u = mesh.material.uniforms
    u.uLight.value.copy(toThree(b.litFrom))
    u.uDirect.value = b.direct
    u.uEven.value = b.even
    u.uDay.value = day
    if (Math.tan(b.angularRadius) * pxPerRad < 2.5 && Number.isFinite(b.mag)) {
      pdirs.push(dir.x, dir.y, dir.z)
      pmags.push(b.mag)
      const c = new THREE.Color(colorOf(b.body)).lerp(new THREE.Color('#ffffff'), 0.5)
      pcolors.push(c.r * 255, c.g * 255, c.b * 255)
    }
  }
  if (planetPoints) {
    scene.remove(planetPoints)
    planetPoints.geometry.dispose()
    planetPoints.material.dispose()
  }
  planetPoints = createPoints(new Float32Array(pdirs), new Float32Array(pmags), new Uint8Array(pcolors))
  planetPoints.material.uniforms.uLimit.value = limit
  planetPoints.material.uniforms.uPx.value = renderer.getPixelRatio()
  planetPoints.renderOrder = -1
  scene.add(planetPoints)

  // The star.
  if (!sunView && s.sun.shines) {
    sunView = createSun(s.sun.color)
    scene.add(sunView.group)
  }
  if (sunView) {
    sunView.group.visible = s.sun.shines
    sunView.disc.position.copy(sunDir).multiplyScalar(BODY_DISTANCE)
    sunView.disc.scale.setScalar(BODY_DISTANCE * Math.tan(s.sun.angularRadius))
    sunView.glare.position.copy(sunDir).multiplyScalar(BODY_DISTANCE * 0.99)
    const glareSize = Math.max(BODY_DISTANCE * Math.tan(s.sun.angularRadius) * 14, (BODY_DISTANCE * 40) / pxPerRad)
    sunView.glare.scale.setScalar(glareSize)
  }

  // Clouds of that month at this hex, and the ground.
  const cover = show.clouds && cloudCover.value != null ? cloudCover.value : 0
  const moonLight = Math.max(0, ...s.bodies.filter((b) => b.alt > 0 && b.body.kind === 'moon').map((b) => Math.min(1, (b.direct * b.lit + b.even) * (b.angularRadius / (0.26 * DEG)) ** 2)))
  for (const m of [clouds, stars, planetPoints]) {
    if (!m) continue
    m.material.uniforms.uCover.value = cover
    // The pattern drifts with the day, so each night looks a little different.
    m.material.uniforms.uCloudShift.value.set((days.value * 0.37) % 50, (days.value * 0.23) % 50)
  }
  clouds.material.uniforms.uDay.value = Math.max(day, shellGlow)
  clouds.material.uniforms.uMoon.value = moonLight
  ground.material.uniforms.uDay.value = Math.max(day, shellGlow)
  gridLines.visible = show.grid
  requestRender()
}

const mixRgb = (a, b, k) => [0, 1, 2].map((i) => a[i] + (b[i] - a[i]) * k)

function render() {
  raf = 0
  if (!renderer) return
  camera.fov = fov.value
  camera.updateProjectionMatrix()
  const d = new THREE.Vector3(Math.sin(view.az) * Math.cos(view.alt), Math.sin(view.alt), -Math.cos(view.az) * Math.cos(view.alt))
  camera.position.set(0, 0, 0)
  camera.lookAt(d)
  renderer.render(scene, camera)
  placeLabels()
}

// Names of the compass points and the bodies over the picture.
function placeLabels() {
  const out = []
  const w = renderer.domElement.clientWidth
  const h = renderer.domElement.clientHeight
  const put = (v, text, kind, key) => {
    const p = v.clone().project(camera)
    if (p.z > 1 || Math.abs(p.x) > 1.05 || Math.abs(p.y) > 1.05) return
    out.push({ key, text, kind, x: ((p.x + 1) / 2) * w, y: ((1 - p.y) / 2) * h })
  }
  const points = ['N', 'NE', 'E', 'SE', 'S', 'SW', 'W', 'NW']
  points.forEach((name, i) => {
    const az = i * 45 * DEG
    put(new THREE.Vector3(Math.sin(az), 0.02, -Math.cos(az)).multiplyScalar(100), name, 'compass', 'c' + name)
  })
  if (show.labels && sky.value) {
    for (const b of sky.value.bodies) {
      if (b.alt < 0) continue
      const v = toThree(b.local).multiplyScalar(100)
      v.y += (BODY_DISTANCE * Math.tan(b.angularRadius) + 2.5 * (100 / ((renderer.domElement.clientHeight / 2) / Math.tan((fov.value * DEG) / 2)))) * 1.6
      put(v, b.body.name, 'body', 'b' + b.body.id)
    }
    if (sky.value.sun.shines && sky.value.sun.alt > 0) put(toThree(sky.value.sun.local).multiplyScalar(100), sunName.value, 'body', 'sun')
  }
  labels.value = out
}

function resize() {
  const el = wrapEl.value
  if (!el || !renderer) return
  renderer.setSize(el.clientWidth, el.clientHeight, false)
  camera.aspect = el.clientWidth / Math.max(1, el.clientHeight)
  camera.updateProjectionMatrix()
  update()
}

// ---- Looking round --------------------------------------------------------------------------
function onDown(e) {
  if (e.button !== 0) return
  drag.value = { x: e.clientX, y: e.clientY, az: view.az, alt: view.alt }
  wrapEl.value.setPointerCapture?.(e.pointerId)
}
function onMove(e) {
  if (!drag.value) return
  const k = (fov.value * DEG) / Math.max(1, wrapEl.value.clientHeight)
  view.az = drag.value.az - (e.clientX - drag.value.x) * k
  view.alt = Math.max(-10 * DEG, Math.min(90 * DEG, drag.value.alt + (e.clientY - drag.value.y) * k))
  requestRender()
}
function onUp(e) {
  drag.value = null
  if (wrapEl.value?.hasPointerCapture?.(e.pointerId)) wrapEl.value.releasePointerCapture(e.pointerId)
}
function onWheel(e) {
  fov.value = Math.max(0.5, Math.min(120, fov.value * Math.exp(e.deltaY * 0.0012)))
  update()
}

// ---- Life ---------------------------------------------------------------------------------------
let resizeObserver = null
onMounted(async () => {
  setup()
  resizeObserver = new ResizeObserver(resize)
  resizeObserver.observe(wrapEl.value)
  try {
    solar.value = await solarApi.get()
    update()
  } catch (err) {
    error.value = err.message || t("Couldn't load the solar system.")
  }
  // For the screenshot scripts (tests/visual), on the dev server only.
  if (import.meta.env.DEV) {
    window.__skyView = {
      set: ({ date, hour, az, alt, fov: f }) => {
        if (date) dateText.value = date
        if (hour != null) localHour.value = hour
        if (az != null) view.az = az * DEG
        if (alt != null) view.alt = alt * DEG
        if (f) fov.value = f
        update()
      },
      ready: () => !!(sky.value && dome && catalogue.value),
      sky: () => sky.value,
    }
  }
})

onBeforeUnmount(() => {
  cancelAnimationFrame(raf)
  resizeObserver?.disconnect()
  scene?.traverse((o) => {
    o.geometry?.dispose()
    o.material?.dispose?.()
  })
  skyTexture?.dispose()
  renderer?.dispose()
  renderer = null
  if (import.meta.env.DEV) delete window.__skyView
})

watch([days, () => props.climate, () => show.clouds, () => show.grid, () => show.labels], update)
</script>

<style scoped>
.sky-view {
  position: fixed;
  inset: 0;
  z-index: 80;
  background: #000;
}

.sky-wrap {
  position: absolute;
  inset: 0;
  cursor: grab;
  touch-action: none;
}

.sky-wrap.dragging {
  cursor: grabbing;
}

.sky-canvas {
  display: block;
  width: 100%;
  height: 100%;
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
  white-space: nowrap;
  font-size: 0.72rem;
  text-shadow: 0 0 3px #000, 0 0 6px #000;
}

.label.compass {
  color: #e8c770;
  font-weight: 700;
  letter-spacing: 0.06em;
}

.label.body {
  color: #dfe7f5;
}

.sky-bar {
  position: absolute;
  top: 12px;
  left: 12px;
  right: 12px;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.6rem 1rem;
  padding: 0.5rem 0.8rem;
}

.where {
  font-family: 'Fraunces', serif;
  font-size: 0.95rem;
}

.date-box {
  width: 11rem;
}

.date-box :deep(.field) {
  margin: 0;
}

.time-box {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
  min-width: 12rem;
  font-size: 0.74rem;
  color: var(--text-muted);
}

.steps,
.toggles {
  display: flex;
  gap: 0.25rem;
}

.close {
  margin-left: auto;
}

.sky-info {
  position: absolute;
  right: 12px;
  bottom: 12px;
  width: 300px;
  max-height: calc(100% - 140px);
  overflow-y: auto;
  padding: 0.7rem 0.9rem;
  font-size: 0.78rem;
}

.sky-info h4 {
  margin: 0.6rem 0 0.25rem;
  font-size: 0.68rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--text-faint);
}

.row {
  display: flex;
  justify-content: space-between;
  gap: 0.6rem;
  margin: 0.15rem 0;
  color: var(--text-faint);
}

.row strong {
  color: var(--text-primary);
  text-align: right;
  font-weight: 600;
}

.body-row {
  display: grid;
  grid-template-columns: auto auto 1fr;
  align-items: center;
  gap: 0.4rem;
  margin: 0.2rem 0;
}

.body-row .dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
}

.body-row .name {
  color: var(--text-primary);
  font-weight: 600;
}

.body-row .facts {
  color: var(--text-muted);
  text-align: right;
  font-size: 0.72rem;
}

.note {
  margin: 0.5rem 0 0;
  font-size: 0.7rem;
  color: var(--text-faint);
}

@media (max-width: 720px) {
  .sky-info {
    width: auto;
    left: 12px;
    max-height: 40%;
  }
}
</style>
