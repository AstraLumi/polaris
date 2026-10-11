<template>
  <div class="page is-wide graph-page">
    <header class="graph-head">
      <div>
        <h1>{{ $t('Graph') }}</h1>
        <p class="page-sub">
          <template v-if="data">
            {{ $tn('{n} article', '{n} articles', shownCount) }} · {{ $tn('{n} connection', '{n} connections', shownEdgeCount) }}
          </template>
          <template v-else>&nbsp;</template>
        </p>
      </div>
      <div class="find">
        <input
          v-model="query"
          type="search"
          :placeholder="$t('Find an article…')"
          :aria-label="$t('Find an article')"
          list="graph-names"
          @keydown.enter.prevent="findFirst"
        />
        <datalist id="graph-names">
          <option v-for="n in nameOptions" :key="n" :value="n" />
        </datalist>
      </div>
    </header>

    <section class="controls glass-panel panel-solid">
      <div class="types" role="group" :aria-label="$t('Kinds of article')">
        <button
          v-for="tp in presentTypes"
          :key="tp"
          type="button"
          class="type-chip"
          :class="{ 'is-off': hiddenTypes.has(tp) }"
          :aria-pressed="!hiddenTypes.has(tp)"
          :title="$t('Show or hide these articles')"
          @click="toggleType(tp)"
        >
          <i class="dot" :style="{ background: TYPE_COLORS[tp] }"></i>{{ $t(TYPE_PLURALS[tp]) }}
        </button>
      </div>
      <div class="toggles">
        <label class="check" :title="$t('Connections the story makes by itself: members, relations, places, classes…')">
          <input v-model="showStory" type="checkbox" /> <span class="line-key is-story"></span>{{ $t('Story connections') }}
        </label>
        <label class="check" :title="$t('Links written in the wiki text with [[…]]')">
          <input v-model="showLinks" type="checkbox" /> <span class="line-key is-link"></span>{{ $t('Text links') }}
        </label>
        <label class="check">
          <input v-model="hideLonely" type="checkbox" /> {{ $t('Hide unconnected') }}
        </label>
      </div>
    </section>

    <p v-if="loadError" class="error-banner">{{ loadError }}</p>

    <div ref="wrapEl" class="graph-wrap" :class="{ 'is-grabbing': dragging, 'is-pointer': hovered && !dragging }">
      <canvas
        ref="canvasEl"
        class="graph-canvas"
        @pointerdown="onDown"
        @pointermove="onMove"
        @pointerup="onUp"
        @pointercancel="onUp"
        @pointerleave="onLeave"
      ></canvas>
      <p v-if="!data && !loadError" class="overlay-note">{{ $t('Loading…') }}</p>
      <p v-else-if="data && !shownCount" class="overlay-note">{{ $t('Nothing to show with these filters.') }}</p>
      <div class="zoom">
        <button type="button" class="tool-btn" :title="$t('Zoom in')" @click="zoomBy(1.3)">+</button>
        <button type="button" class="tool-btn" :title="$t('Zoom out')" @click="zoomBy(1 / 1.3)">−</button>
        <button type="button" class="tool-btn fit" :title="$t('Fit everything in view')" @click="fit(true)">{{ $t('Fit') }}</button>
      </div>
      <p class="hint">{{ $t('Drag to move · scroll to zoom · drag a dot to pull it · click to open') }}</p>

      <div v-if="card" class="hover-card glass-panel panel-solid" :style="{ left: card.x + 'px', top: card.y + 'px' }">
        <ArticleCard :article="card.article" />
        <p class="card-foot">{{ $tn('{n} connection', '{n} connections', card.degree) }}</p>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, shallowRef, computed, watch, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import ArticleCard from '../components/ArticleCard.vue'
import { TYPE_ORDER, TYPE_PLURALS, wikiApi, wikiPath } from '../wiki'
import { loadArticle } from '../articlePreview'
import { createSimulation, bounds, nodeRadius } from '../graphLayout'
import { pageTitle } from '../navigation'
import { t } from '../i18n'

// Every wiki article as a dot and every connection between two articles as a
// line (backend/wikigraph.go). The layout is a force simulation
// (graphLayout.js) drawn on a canvas; hovering a dot shows the article's card
// and lights up its neighbours, clicking opens the article.
// /wiki/graph?focus=<type>:<id> starts centred on one article.

const TYPE_COLORS = {
  character: '#f2c66d',
  location: '#6cc3b0',
  faction: '#e58a6f',
  event: '#7fa6e0',
  class: '#b48cf0',
  subclass: '#9c8cf0',
  specialization: '#8c9cf0',
  race: '#8fd17a',
  body_type: '#a8c38a',
  spell: '#5ec8e8',
  gear: '#c8a27a',
  lore: '#f0a0c8',
}

const route = useRoute()
const router = useRouter()
pageTitle.value = t('Graph')

const data = ref(null)
const loadError = ref('')
const query = ref('')
const hiddenTypes = ref(new Set())
const showStory = ref(true)
const showLinks = ref(true)
const hideLonely = ref(false)

const wrapEl = ref(null)
const canvasEl = ref(null)
// Shallow: these hold the simulation's own node objects, compared by identity.
const hovered = shallowRef(null) // a node
const selected = shallowRef(null) // a node picked by search or ?focus
const dragging = ref(false)
const card = ref(null) // { article, degree, x, y }

const keyOf = (n) => `${n.type}:${n.id}`

// ---- what is shown -------------------------------------------------------------

const presentTypes = computed(() => TYPE_ORDER.filter((tp) => data.value?.nodes.some((n) => n.type === tp)))

function toggleType(tp) {
  const next = new Set(hiddenTypes.value)
  if (next.has(tp)) next.delete(tp)
  else next.add(tp)
  hiddenTypes.value = next
}

// The nodes and edges on screen, rebuilt (keeping positions) when a filter
// changes. Plain objects: the simulation moves them every frame.
let nodes = []
let edges = [] // { s, t, story, link } with s/t indexes into nodes
let degree = []
let labelCut = 0 // articles with at least this many connections are always named
let neighbours = [] // index -> Set of indexes
let sim = null
const shownCount = ref(0)
const shownEdgeCount = ref(0)
const placed = new Map() // key -> node object, so positions survive a rebuild

function rebuild() {
  if (!data.value) return
  const edgeOn = (e) => (e.story && showStory.value) || (e.link && showLinks.value)
  let list = data.value.nodes.filter((n) => !hiddenTypes.value.has(n.type))
  const keys = new Set(list.map(keyOf))
  let raw = data.value.edges.filter((e) => edgeOn(e) && keys.has(e.a) && keys.has(e.b))
  if (hideLonely.value) {
    const linked = new Set(raw.flatMap((e) => [e.a, e.b]))
    list = list.filter((n) => linked.has(keyOf(n)))
  }
  nodes = list.map((n) => {
    const k = keyOf(n)
    let p = placed.get(k)
    if (!p) {
      p = { ...n, key: k }
      placed.set(k, p)
    }
    p.fx = null
    p.fy = null
    return p
  })
  const index = new Map(nodes.map((n, i) => [n.key, i]))
  edges = raw
    .filter((e) => index.has(e.a) && index.has(e.b))
    .map((e) => ({ s: index.get(e.a), t: index.get(e.b), story: e.story, link: e.link }))
  degree = nodes.map(() => 0)
  neighbours = nodes.map(() => new Set())
  for (const e of edges) {
    degree[e.s]++
    degree[e.t]++
    neighbours[e.s].add(e.t)
    neighbours[e.t].add(e.s)
  }
  labelCut = Math.max(2, [...degree].sort((a, b) => b - a)[Math.min(degree.length - 1, 14)] ?? 0)
  shownCount.value = nodes.length
  shownEdgeCount.value = edges.length
  if (hovered.value && !index.has(hovered.value.key)) setHover(null)
  if (selected.value && !index.has(selected.value.key)) selected.value = null
  sim = createSimulation(nodes, edges.map((e) => ({ source: e.s, target: e.t })))
  run()
}

watch([hiddenTypes, showStory, showLinks, hideLonely], rebuild)

const nameOptions = computed(() => [...new Set((data.value?.nodes || []).map((n) => n.name))].sort((a, b) => a.localeCompare(b)))

// ---- view: pan and zoom ---------------------------------------------------------

const view = { x: 0, y: 0, k: 1 }
const size = { w: 0, h: 0, dpr: 1 }
const K_MIN = 0.08
const K_MAX = 4

function fit(animate = false) {
  const b = bounds(nodes)
  if (!b || !size.w) return
  const pad = 60
  const k = Math.min(K_MAX, Math.max(K_MIN, Math.min((size.w - pad * 2) / (b.maxX - b.minX + 1), (size.h - pad * 2) / (b.maxY - b.minY + 1), 1.6)))
  goTo((b.minX + b.maxX) / 2, (b.minY + b.maxY) / 2, k, animate)
}

// Centre the view on a world point at zoom k, gliding there if asked.
let glide = 0
function goTo(wx, wy, k, animate) {
  const target = { x: size.w / 2 - wx * k, y: size.h / 2 - wy * k, k }
  cancelAnimationFrame(glide)
  if (!animate || window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
    Object.assign(view, target)
    draw()
    return
  }
  const from = { ...view }
  const start = performance.now()
  const step = (now) => {
    const p = Math.min(1, (now - start) / 350)
    const e = 1 - Math.pow(1 - p, 3)
    view.x = from.x + (target.x - from.x) * e
    view.y = from.y + (target.y - from.y) * e
    view.k = from.k + (target.k - from.k) * e
    draw()
    if (p < 1) glide = requestAnimationFrame(step)
  }
  glide = requestAnimationFrame(step)
}

function zoomAt(sx, sy, factor) {
  const k = Math.min(K_MAX, Math.max(K_MIN, view.k * factor))
  const wx = (sx - view.x) / view.k
  const wy = (sy - view.y) / view.k
  view.k = k
  view.x = sx - wx * k
  view.y = sy - wy * k
  draw()
}

const zoomBy = (f) => zoomAt(size.w / 2, size.h / 2, f)

function onWheel(e) {
  e.preventDefault()
  const r = canvasEl.value.getBoundingClientRect()
  zoomAt(e.clientX - r.left, e.clientY - r.top, Math.exp(-e.deltaY * 0.0015))
}

// ---- running the layout ---------------------------------------------------------

let frame = 0
function run() {
  cancelAnimationFrame(frame)
  const loop = () => {
    sim.tick()
    draw()
    if (!sim.cool) frame = requestAnimationFrame(loop)
  }
  frame = requestAnimationFrame(loop)
}

// ---- drawing --------------------------------------------------------------------

const css = (name) => getComputedStyle(document.documentElement).getPropertyValue(name).trim()

function draw() {
  const canvas = canvasEl.value
  if (!canvas || !size.w) return
  const ctx = canvas.getContext('2d')
  ctx.setTransform(size.dpr, 0, 0, size.dpr, 0, 0)
  ctx.clearRect(0, 0, size.w, size.h)
  ctx.translate(view.x, view.y)
  ctx.scale(view.k, view.k)

  const focus = hovered.value || selected.value
  const fi = focus ? nodes.indexOf(focus) : -1
  const near = fi >= 0 ? neighbours[fi] : null
  const lit = (i) => fi < 0 || i === fi || near.has(i)

  // Lines: story connections solid, text links dashed. With a focus, only
  // its own lines stay bright.
  const text = css('--text-primary') || '#fff'
  const accent = css('--accent') || '#e8c46a'
  ctx.lineWidth = 1 / view.k
  for (const pass of ['dim', 'bright']) {
    for (const e of edges) {
      const bright = fi >= 0 && (e.s === fi || e.t === fi)
      if ((pass === 'bright') !== bright) continue
      const a = nodes[e.s]
      const b = nodes[e.t]
      ctx.beginPath()
      ctx.moveTo(a.x, a.y)
      ctx.lineTo(b.x, b.y)
      ctx.setLineDash(e.story ? [] : [4 / view.k, 3 / view.k])
      ctx.globalAlpha = bright ? 0.9 : fi >= 0 ? 0.06 : 0.24
      ctx.strokeStyle = bright ? accent : text
      ctx.lineWidth = (bright ? 1.6 : 1) / view.k
      ctx.stroke()
    }
  }
  ctx.setLineDash([])

  // Dots: an article nobody has written in is hollow.
  nodes.forEach((n, i) => {
    const r = nodeRadius(degree[i])
    ctx.globalAlpha = lit(i) ? 1 : 0.15
    ctx.beginPath()
    ctx.arc(n.x, n.y, r, 0, Math.PI * 2)
    const color = TYPE_COLORS[n.type] || text
    if (n.written) {
      ctx.fillStyle = color
      ctx.fill()
    } else {
      ctx.fillStyle = css('--surface') || '#0a1636'
      ctx.fill()
      ctx.lineWidth = 2 / view.k
      ctx.strokeStyle = color
      ctx.stroke()
    }
    if (n === focus) {
      ctx.lineWidth = 2.5 / view.k
      ctx.strokeStyle = text
      ctx.beginPath()
      ctx.arc(n.x, n.y, r + 3 / view.k, 0, Math.PI * 2)
      ctx.stroke()
    }
  })

  // Names: all of them once zoomed in, otherwise the focus and its
  // neighbours and the best-connected articles. Most important first, and a
  // name that would overlap one already drawn is left out.
  ctx.font = `600 ${12 / view.k}px Manrope, sans-serif`
  ctx.textAlign = 'center'
  ctx.textBaseline = 'top'
  ctx.lineJoin = 'round'
  const wanted = nodes
    .map((n, i) => i)
    .filter((i) => (fi >= 0 ? lit(i) : view.k > 1.25 || degree[i] >= labelCut))
    .sort((a, b) => (b === fi) - (a === fi) || degree[b] - degree[a])
  const taken = []
  const lineH = 15 / view.k
  const bg = css('--bg') || '#050b1f'
  for (const i of wanted) {
    const n = nodes[i]
    const y = n.y + nodeRadius(degree[i]) + 3 / view.k
    const w = ctx.measureText(n.name).width + 6 / view.k
    const box = { x0: n.x - w / 2, x1: n.x + w / 2, y0: y, y1: y + lineH }
    if (i !== fi && taken.some((b) => box.x0 < b.x1 && box.x1 > b.x0 && box.y0 < b.y1 && box.y1 > b.y0)) continue
    taken.push(box)
    ctx.globalAlpha = 1
    ctx.lineWidth = 3 / view.k
    ctx.strokeStyle = bg
    ctx.strokeText(n.name, n.x, y)
    ctx.fillStyle = text
    ctx.fillText(n.name, n.x, y)
  }
  ctx.globalAlpha = 1
}

// ---- pointer --------------------------------------------------------------------

function worldAt(e) {
  const r = canvasEl.value.getBoundingClientRect()
  const sx = e.clientX - r.left
  const sy = e.clientY - r.top
  return { sx, sy, x: (sx - view.x) / view.k, y: (sy - view.y) / view.k }
}

function nodeAt(p) {
  let best = null
  let bestD = Infinity
  nodes.forEach((n, i) => {
    const d = Math.hypot(n.x - p.x, n.y - p.y)
    const reach = nodeRadius(degree[i]) + 4 / view.k
    if (d <= reach && d < bestD) {
      best = n
      bestD = d
    }
  })
  return best
}

let drag = null // { node?, sx, sy, vx, vy, moved }

function onDown(e) {
  if (e.button !== 0 && e.pointerType === 'mouse') return
  const p = worldAt(e)
  const n = nodeAt(p)
  canvasEl.value.setPointerCapture(e.pointerId)
  drag = { node: n, sx: p.sx, sy: p.sy, vx: view.x, vy: view.y, moved: false }
  dragging.value = true
  hideCard()
}

function onMove(e) {
  const p = worldAt(e)
  if (drag) {
    if (Math.hypot(p.sx - drag.sx, p.sy - drag.sy) > 3) drag.moved = true
    if (!drag.moved) return
    if (drag.node) {
      drag.node.fx = p.x
      drag.node.fy = p.y
      sim.alphaTarget = 0.25
      sim.reheat(0.25)
      run()
    } else {
      view.x = drag.vx + (p.sx - drag.sx)
      view.y = drag.vy + (p.sy - drag.sy)
      draw()
    }
    return
  }
  const n = e.pointerType === 'touch' ? null : nodeAt(p)
  if (n !== hovered.value) setHover(n, e)
  else if (n && card.value) placeCard(e)
}

function onUp(e) {
  const d = drag
  drag = null
  dragging.value = false
  if (!d) return
  if (canvasEl.value?.hasPointerCapture(e.pointerId)) canvasEl.value.releasePointerCapture(e.pointerId)
  if (d.node && d.moved) {
    d.node.fx = null
    d.node.fy = null
    sim.alphaTarget = 0
    run()
    return
  }
  if (!d.moved) {
    if (d.node) router.push(wikiPath(d.node.type, d.node.id))
    else {
      selected.value = null
      draw()
    }
  }
}

function onLeave() {
  if (!drag) setHover(null)
}

// ---- the hover card ----------------------------------------------------------------

let cardTimer = 0
let cardSeq = 0
let lastPointer = { clientX: 0, clientY: 0 }

function setHover(n, e) {
  hovered.value = n
  draw()
  hideCard()
  if (!n) return
  lastPointer = { clientX: e.clientX, clientY: e.clientY }
  const mine = ++cardSeq
  cardTimer = setTimeout(async () => {
    let a
    try {
      a = await loadArticle(n.type, n.id)
    } catch (err) {
      return
    }
    if (mine !== cardSeq || hovered.value !== n) return
    card.value = { article: a, degree: degree[nodes.indexOf(n)] || 0, x: 0, y: 0 }
    await nextTick()
    placeCard(lastPointer)
  }, 250)
}

function hideCard() {
  clearTimeout(cardTimer)
  cardSeq++
  card.value = null
}

function placeCard(e) {
  lastPointer = { clientX: e.clientX, clientY: e.clientY }
  if (!card.value || !wrapEl.value) return
  const r = wrapEl.value.getBoundingClientRect()
  const el = wrapEl.value.querySelector('.hover-card')
  const w = el ? el.offsetWidth : 300
  const h = el ? el.offsetHeight : 160
  let x = e.clientX - r.left + 18
  let y = e.clientY - r.top + 18
  if (x + w > r.width - 8) x = e.clientX - r.left - w - 18
  if (y + h > r.height - 8) y = Math.max(8, r.height - h - 8)
  card.value.x = Math.max(8, x)
  card.value.y = y
}

// ---- finding an article -------------------------------------------------------------

function focusNode(n) {
  selected.value = n
  goTo(n.x, n.y, Math.max(view.k, 1.4), true)
}

function findFirst() {
  const q = query.value.trim().toLowerCase()
  if (!q) return
  const n = nodes.find((x) => x.name.toLowerCase() === q) || nodes.find((x) => x.name.toLowerCase().includes(q))
  if (n) focusNode(n)
}

watch(query, (q) => {
  // Picking a name from the list finds it straight away.
  if (nodes.some((n) => n.name === q)) findFirst()
})

// ---- setup ----------------------------------------------------------------------------

let resizeObserver = null
function updateSize() {
  const el = wrapEl.value
  const canvas = canvasEl.value
  if (!el || !canvas) return
  size.w = el.clientWidth
  size.h = el.clientHeight
  size.dpr = window.devicePixelRatio || 1
  canvas.width = Math.round(size.w * size.dpr)
  canvas.height = Math.round(size.h * size.dpr)
  draw()
}

onMounted(async () => {
  updateSize()
  resizeObserver = new ResizeObserver(updateSize)
  resizeObserver.observe(wrapEl.value)
  canvasEl.value.addEventListener('wheel', onWheel, { passive: false })
  try {
    data.value = await wikiApi.graph()
  } catch (e) {
    loadError.value = e.message
    return
  }
  rebuild()
  // Settle most of the layout before the first picture, so it doesn't jump.
  for (let i = 0; i < 400 && !sim.cool; i++) sim.tick()
  const focus = typeof route.query.focus === 'string' ? nodes.find((n) => n.key === route.query.focus) : null
  if (focus) {
    selected.value = focus
    goTo(focus.x, focus.y, 1.4, false)
  } else {
    fit()
  }
})

onBeforeUnmount(() => {
  cancelAnimationFrame(frame)
  cancelAnimationFrame(glide)
  clearTimeout(cardTimer)
  resizeObserver?.disconnect()
  canvasEl.value?.removeEventListener('wheel', onWheel)
})
</script>

<style scoped>
.graph-page {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 3rem);
  min-height: 520px;
}

.graph-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  gap: 1rem;
  flex-wrap: wrap;
  margin-bottom: 0.8rem;
}

.graph-head h1 {
  margin: 0;
}

.find input {
  width: 16rem;
  max-width: 100%;
}

.controls {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 0.6rem 1.5rem;
  padding: 0.6rem 0.9rem;
  margin-bottom: 0.8rem;
}

.types {
  display: flex;
  flex-wrap: wrap;
  gap: 0.3rem;
}

.type-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.15rem 0.6rem;
  border: 1px solid var(--glass-border);
  border-radius: 999px;
  background: none;
  color: var(--text-muted);
  font-family: 'Manrope', sans-serif;
  font-size: 0.74rem;
  font-weight: 600;
  cursor: pointer;
}

.type-chip.is-off {
  opacity: 0.4;
  text-decoration: line-through;
}

.dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  display: inline-block;
}

.toggles {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem 1rem;
}

.check {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  font-size: 0.78rem;
  color: var(--text-muted);
  cursor: pointer;
}

.line-key {
  display: inline-block;
  width: 18px;
  border-top: 2px solid var(--text-muted);
}

.line-key.is-link {
  border-top-style: dashed;
}

.graph-wrap {
  position: relative;
  flex: 1;
  min-height: 320px;
  overflow: hidden;
  border: 1px solid var(--glass-border);
  border-radius: 16px;
  background:
    radial-gradient(circle at 30% 20%, color-mix(in srgb, var(--tint) 10%, transparent), transparent 55%),
    var(--bg);
  touch-action: none;
}

.graph-wrap.is-pointer .graph-canvas {
  cursor: pointer;
}

.graph-wrap.is-grabbing .graph-canvas {
  cursor: grabbing;
}

.graph-canvas {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  display: block;
  cursor: grab;
}

.overlay-note {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 0;
  color: var(--text-faint);
  pointer-events: none;
}

.zoom {
  position: absolute;
  top: 0.7rem;
  left: 0.7rem;
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
}

.tool-btn {
  min-width: 2rem;
  height: 2rem;
  padding: 0 0.5rem;
  border: 1px solid var(--glass-border);
  border-radius: 8px;
  background: color-mix(in srgb, var(--surface) 90%, transparent);
  color: var(--text-primary);
  font-family: 'Manrope', sans-serif;
  font-size: 0.95rem;
  font-weight: 600;
  cursor: pointer;
}

.tool-btn.fit {
  font-size: 0.72rem;
}

.tool-btn:hover {
  border-color: var(--accent);
}

.hint {
  position: absolute;
  right: 0.8rem;
  bottom: 0.6rem;
  margin: 0;
  font-size: 0.72rem;
  color: var(--text-faint);
  pointer-events: none;
}

.hover-card {
  position: absolute;
  z-index: 5;
  width: min(19rem, calc(100% - 16px));
  padding: 0.8rem 0.9rem;
  border-radius: 12px;
  box-shadow: 0 12px 32px rgba(0, 0, 0, 0.4);
  pointer-events: none;
}

.card-foot {
  margin: 0.5rem 0 0;
  font-size: 0.7rem;
  color: var(--text-faint);
}

@media (max-width: 720px) {
  .hint {
    display: none;
  }
  .find input {
    width: 100%;
  }
}
</style>
