<template>
  <div class="board">
    <div class="board-bar">
      <button type="button" class="btn btn-primary small" :disabled="busy" @click="addCard">+ {{ $t('Card') }}</button>
      <div class="cat-wrap">
        <button type="button" class="btn btn-ghost small" :aria-expanded="catsOpen" @click="catsOpen = !catsOpen">
          <span class="cat-dots" aria-hidden="true">
            <span v-for="c in categories.slice(0, 4)" :key="c.id" class="dot" :style="{ background: c.color }"></span>
          </span>
          {{ $t('Categories') }}
        </button>
        <div v-if="catsOpen" class="cat-pop glass-panel panel-solid" @pointerdown.stop>
          <p v-if="!categories.length" class="muted small-text">{{ $t('A category gives cards a colour, like Plot or Worldbuilding.') }}</p>
          <div v-for="c in categories" :key="c.id" class="cat-row">
            <input type="color" :value="c.color" :aria-label="$t('Colour')" @change="saveCategory(c, c.name, $event.target.value)" />
            <input type="text" :value="c.name" maxlength="60" :aria-label="$t('Name')" @change="saveCategory(c, $event.target.value, c.color)" />
            <button type="button" class="btn btn-ghost small" :title="$t('Delete')" :aria-label="$t('Delete')" @click="removeCategory(c)">×</button>
          </div>
          <button type="button" class="btn btn-ghost small" @click="addCategory">+ {{ $t('Category') }}</button>
        </div>
      </div>
      <span class="spacer"></span>
      <button type="button" class="btn btn-ghost small" :disabled="cards.length < 2 || busy" :title="$t('Spread the cards out by their lines')" @click="tidy">{{ $t('Tidy up') }}</button>
      <div class="zoom">
        <button type="button" class="btn btn-ghost small" :title="$t('Zoom out')" @click="zoomBy(1 / 1.25)">−</button>
        <button type="button" class="btn btn-ghost small" :title="$t('Show everything')" @click="fit(true)">⤢</button>
        <button type="button" class="btn btn-ghost small" :title="$t('Zoom in')" @click="zoomBy(1.25)">+</button>
      </div>
    </div>
    <p v-if="error" class="error-text board-error">{{ error }}</p>

    <div
      ref="wrapEl"
      class="board-wrap"
      :class="{ 'is-panning': gesture?.kind === 'pan', 'is-connecting': !!connecting }"
      @pointerdown.capture="onTouchDown"
      @pointerdown="onBackgroundDown"
      @wheel.prevent="onWheel"
    >
      <p v-if="loaded && !cards.length" class="empty muted">{{ $t('An empty board. Add a card, then drag from the dot on its edge to another card to join them.') }}</p>
      <div class="world" :style="{ transform: `translate(${view.x}px, ${view.y}px) scale(${view.k})` }">
        <svg class="lines" aria-hidden="true">
          <g v-for="l in drawnLinks" :key="l.id" class="link" :class="{ 'is-picked': pickedLink === l.id }">
            <line class="hit" :x1="l.x1" :y1="l.y1" :x2="l.x2" :y2="l.y2" @pointerdown.stop="pickLink(l.id)" />
            <line class="stroke" :x1="l.x1" :y1="l.y1" :x2="l.x2" :y2="l.y2" />
            <text v-if="l.label" class="link-label" :x="l.mx" :y="l.my - 6">{{ l.label }}</text>
          </g>
          <line v-if="connecting" class="stroke is-temp" :x1="connecting.x1" :y1="connecting.y1" :x2="connecting.x2" :y2="connecting.y2" />
        </svg>

        <div v-if="pickedLinkLine" class="link-pop glass-panel panel-solid" :style="{ left: `${pickedLinkLine.mx}px`, top: `${pickedLinkLine.my}px` }" @pointerdown.stop>
          <input ref="labelEl" type="text" :value="pickedLinkRow?.label || ''" maxlength="80" :placeholder="$t('Label (optional)')" @change="saveLinkLabel($event.target.value)" @keydown.enter="$event.target.blur()" />
          <button type="button" class="btn btn-ghost small danger-text" @click="removeLink">{{ $t('Remove line') }}</button>
        </div>

        <!-- Closed cards: a tile with the title over the category. -->
        <div
          v-for="c in cards"
          v-show="c.id !== openId"
          :key="c.id"
          class="tile"
          :data-card="c.id"
          :style="tileStyle(c)"
          tabindex="0"
          role="button"
          :aria-label="c.title || $t('Untitled card')"
          @pointerdown.stop="startMove($event, c)"
          @keydown.enter.prevent="openCard(c)"
        >
          <span class="tile-title">{{ c.title || $t('Untitled card') }}</span>
          <span class="tile-cat">{{ categoryOf(c)?.name || '' }}</span>
          <span class="knob" :title="$t('Drag to another card to join them')" @pointerdown.stop="startConnect($event, c)"></span>
        </div>

        <!-- The open card, at the size it was given. -->
        <div v-if="openCardRow" class="card glass-panel panel-solid" :data-card="openCardRow.id" :style="openStyle" @pointerdown.stop>
          <div class="card-head" :style="headStyle(openCardRow)" @pointerdown="startMove($event, openCardRow)">
            <input
              ref="titleEl"
              class="card-title"
              type="text"
              :value="openCardRow.title"
              maxlength="200"
              :placeholder="$t('Untitled card')"
              :aria-label="$t('Title')"
              @pointerdown.stop
              @change="save(openCardRow, { title: $event.target.value })"
              @keydown.enter="$event.target.blur()"
            />
            <select class="card-cat" :value="openCardRow.category_id ?? ''" :aria-label="$t('Category')" @pointerdown.stop @change="setCategory(openCardRow, $event.target.value)">
              <option value="">{{ $t('No category') }}</option>
              <option v-for="cat in categories" :key="cat.id" :value="cat.id">{{ cat.name }}</option>
            </select>
            <RowMenu :items="cardMenu(openCardRow)" @pointerdown.stop />
            <button type="button" class="close" :title="$t('Close (Esc)')" :aria-label="$t('Close')" @pointerdown.stop @click="closeCard">×</button>
            <span class="knob" :title="$t('Drag to another card to join them')" @pointerdown.stop="startConnect($event, openCardRow)"></span>
          </div>
          <div class="card-body">
            <template v-if="editing">
              <MarkdownEditor v-model="bodyDraft" class="card-editor" :rows="8" :items="wikiItems" :resolve="resolver" headings="full" :placeholder="$t('Anything. **bold**, __underline__, ||spoiler||, # headings, - lists and [[links]] work as on Discord.')" />
              <div class="edit-actions">
                <button type="button" class="btn btn-primary small" @click="saveBody">{{ $t('Done') }}</button>
                <button type="button" class="btn btn-ghost small" @click="editing = false">{{ $t('Cancel') }}</button>
              </div>
            </template>
            <!-- eslint-disable-next-line vue/no-v-html -->
            <div v-else-if="openCardRow.body.trim()" class="md card-md" @dblclick="startEdit" @click="onBodyClick" v-html="renderMarkdown(openCardRow.body, resolver, { headings: 'full' })"></div>
            <button v-else type="button" class="write-here muted" @click="startEdit">{{ $t('Write something…') }}</button>
          </div>
          <span v-for="e in EDGES" :key="e" class="handle" :class="`h-${e}`" @pointerdown.stop="startResize($event, openCardRow, e)"></span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
// A drafts board (views/DraftsPage.vue): cards placed by hand on a surface
// that pans and zooms. A card is a tile (title over category) until it is
// clicked; then it opens at the size it was given, its edges drag to resize
// it, and its body is Markdown (Discord's syntax, markdown.js). The dot on a
// card's edge dragged onto another card joins them with a line. Geometry in
// drafts/board.js.
import { ref, reactive, computed, watch, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import { t } from '../../i18n'
import { draftsApi } from '../../api'
import { wikiApi, makeLinkResolver, linksForEditing } from '../../wiki'
import { renderMarkdown } from '../../markdown'
import { askConfirm } from '../../confirm'
import { createSimulation } from '../../graphLayout'
import { TILE, cardRect, linkLine, resizeRect, freeSpot, boardBounds, fitView, inkOn, CATEGORY_COLORS } from '../../drafts/board.js'
import MarkdownEditor from '../MarkdownEditor.vue'
import RowMenu from '../RowMenu.vue'

const props = defineProps({
  id: { type: Number, required: true },
  categories: { type: Array, required: true },
})
const emit = defineEmits(['update:categories', 'cards'])
const router = useRouter()

const EDGES = ['n', 'e', 's', 'w', 'ne', 'nw', 'se', 'sw']

const cards = ref([])
const links = ref([])
const loaded = ref(false)
const error = ref('')
const busy = ref(false)
const openId = ref(null)
const editing = ref(false)
const bodyDraft = ref('')
const pickedLink = ref(null)
const catsOpen = ref(false)
const view = reactive({ x: 0, y: 0, k: 1 })
const wrapEl = ref(null)
const titleEl = ref(null)
const labelEl = ref(null)

// The wiki's articles, for [[links]] in the cards (they link one way: the
// wiki never sees the drafts).
const wikiItems = ref([])
const resolver = computed(() => makeLinkResolver(wikiItems.value))
wikiApi.list().then((items) => (wikiItems.value = items)).catch(() => {})

const byId = computed(() => new Map(cards.value.map((c) => [c.id, c])))
const categoryOf = (c) => props.categories.find((x) => x.id === c.category_id) || null
const openCardRow = computed(() => byId.value.get(openId.value) || null)
const rectOf = (c) => cardRect(c, c.id === openId.value)

const drawnLinks = computed(() =>
  links.value
    .map((l) => {
      const a = byId.value.get(l.a)
      const b = byId.value.get(l.b)
      const line = a && b ? linkLine(rectOf(a), rectOf(b)) : null
      return line && { ...line, id: l.id, label: l.label }
    })
    .filter(Boolean),
)
const pickedLinkRow = computed(() => links.value.find((l) => l.id === pickedLink.value) || null)
const pickedLinkLine = computed(() => drawnLinks.value.find((l) => l.id === pickedLink.value) || null)

function tileStyle(c) {
  const cat = categoryOf(c)
  return {
    left: `${c.x}px`,
    top: `${c.y}px`,
    width: `${TILE.w}px`,
    height: `${TILE.h}px`,
    ...(cat ? { '--cat': cat.color, '--cat-ink': inkOn(cat.color) } : {}),
  }
}
const openStyle = computed(() => {
  const c = openCardRow.value
  return c ? { left: `${c.x}px`, top: `${c.y}px`, width: `${c.w}px`, height: `${c.h}px` } : {}
})
function headStyle(c) {
  const cat = categoryOf(c)
  return cat ? { '--cat': cat.color, '--cat-ink': inkOn(cat.color) } : {}
}

// ---- loading ----

let opening = 0
async function load() {
  const token = ++opening
  loaded.value = false
  error.value = ''
  openId.value = null
  pickedLink.value = null
  try {
    const b = await draftsApi.board(props.id)
    if (token !== opening) return
    cards.value = b.cards
    links.value = b.links
    loaded.value = true
    await nextTick()
    fit(false)
  } catch (e) {
    if (token === opening) error.value = e.message
  }
}
watch(() => props.id, load)
watch(() => cards.value.length, (n) => emit('cards', props.id, n))

// ---- the view: pan and zoom ----

function size() {
  const r = wrapEl.value?.getBoundingClientRect()
  return r ? { w: r.width, h: r.height } : { w: 800, h: 600 }
}
function fit() {
  const { w, h } = size()
  Object.assign(view, fitView(boardBounds(cards.value.map(rectOf)), w, h))
}
function zoomAt(sx, sy, factor) {
  const k = Math.max(0.15, Math.min(2.5, view.k * factor))
  view.x = sx - ((sx - view.x) * k) / view.k
  view.y = sy - ((sy - view.y) * k) / view.k
  view.k = k
}
function zoomBy(f) {
  const { w, h } = size()
  zoomAt(w / 2, h / 2, f)
}
function onWheel(e) {
  const r = wrapEl.value.getBoundingClientRect()
  zoomAt(e.clientX - r.left, e.clientY - r.top, Math.exp(-e.deltaY * 0.0015))
}
// A point on the screen in board units.
function toBoard(e) {
  const r = wrapEl.value.getBoundingClientRect()
  return { x: (e.clientX - r.left - view.x) / view.k, y: (e.clientY - r.top - view.y) / view.k }
}

// ---- gestures: pan, move, resize, connect ----

const gesture = ref(null)
const connecting = ref(null)

function begin(e, g) {
  if (e.button !== 0) return
  catsOpen.value = false
  gesture.value = { ...g, sx: e.clientX, sy: e.clientY, moved: false }
  window.addEventListener('pointermove', onMove)
  window.addEventListener('pointerup', onUp)
  window.addEventListener('pointercancel', onUp)
}

function onBackgroundDown(e) {
  catsOpen.value = false
  begin(e, { kind: 'pan', vx: view.x, vy: view.y })
}
function startMove(e, c) {
  if (e.target.closest('input, select, button, .row-menu')) return
  begin(e, { kind: 'move', card: c, x: c.x, y: c.y })
}
function startResize(e, c, edge) {
  begin(e, { kind: 'resize', card: c, edge, rect: { x: c.x, y: c.y, w: c.w, h: c.h } })
}
function startConnect(e, c) {
  const r = rectOf(c)
  begin(e, { kind: 'connect', card: c })
  const p = toBoard(e)
  connecting.value = { from: c.id, x1: r.x + r.w / 2, y1: r.y + r.h / 2, x2: p.x, y2: p.y }
}

function onMove(e) {
  const g = gesture.value
  if (!g) return
  const dx = e.clientX - g.sx
  const dy = e.clientY - g.sy
  if (!g.moved && Math.hypot(dx, dy) < 4) return
  g.moved = true
  if (g.kind === 'pan') {
    view.x = g.vx + dx
    view.y = g.vy + dy
  } else if (g.kind === 'move') {
    g.card.x = Math.round(g.x + dx / view.k)
    g.card.y = Math.round(g.y + dy / view.k)
  } else if (g.kind === 'resize') {
    Object.assign(g.card, resizeRect(g.rect, g.edge, dx / view.k, dy / view.k))
  } else if (g.kind === 'connect') {
    const p = toBoard(e)
    connecting.value = { ...connecting.value, x2: p.x, y2: p.y }
  }
}

async function onUp(e) {
  window.removeEventListener('pointermove', onMove)
  window.removeEventListener('pointerup', onUp)
  window.removeEventListener('pointercancel', onUp)
  const g = gesture.value
  gesture.value = null
  if (!g) return
  if (g.kind === 'pan') {
    if (!g.moved) {
      pickedLink.value = null
      closeCard()
    }
  } else if (g.kind === 'move') {
    if (g.moved) save(g.card, { x: g.card.x, y: g.card.y })
    else if (g.card.id !== openId.value) openCard(g.card)
  } else if (g.kind === 'resize') {
    if (g.moved) save(g.card, { x: g.card.x, y: g.card.y, w: g.card.w, h: g.card.h })
  } else if (g.kind === 'connect') {
    connecting.value = null
    const hit = document.elementFromPoint(e.clientX, e.clientY)?.closest('[data-card]')
    const to = hit && Number(hit.dataset.card)
    if (to && to !== g.card.id) join(g.card.id, to)
  }
}

// ---- touch: two fingers pinch to zoom and move the board together ----
// The first finger works as a mouse does (pan, move, resize, join). A
// second one undoes what the first had started and pinches instead; the
// finger left after a pinch does nothing until it is lifted.

const touches = new Map() // pointerId -> { x, y } on the screen
let pinch = null // { dist, mx, my } while two fingers are down

function pinchState() {
  const [a, b] = [...touches.values()]
  const r = wrapEl.value.getBoundingClientRect()
  return { dist: Math.max(1, Math.hypot(a.x - b.x, a.y - b.y)), mx: (a.x + b.x) / 2 - r.left, my: (a.y + b.y) / 2 - r.top }
}

// What the first finger had started is put back as it was.
function cancelGesture() {
  const g = gesture.value
  if (g?.kind === 'move') Object.assign(g.card, { x: g.x, y: g.y })
  if (g?.kind === 'resize') Object.assign(g.card, g.rect)
  connecting.value = null
  gesture.value = null
  window.removeEventListener('pointermove', onMove)
  window.removeEventListener('pointerup', onUp)
  window.removeEventListener('pointercancel', onUp)
}

function onTouchDown(e) {
  if (e.pointerType !== 'touch') return
  touches.set(e.pointerId, { x: e.clientX, y: e.clientY })
  if (touches.size < 2) return
  // The second finger (or more): no card or line takes it.
  e.stopPropagation()
  e.preventDefault()
  if (pinch) return
  cancelGesture()
  pinch = pinchState()
}

function onTouchMove(e) {
  if (!touches.has(e.pointerId)) return
  touches.set(e.pointerId, { x: e.clientX, y: e.clientY })
  if (!pinch || touches.size < 2) return
  const now = pinchState()
  zoomAt(now.mx, now.my, now.dist / pinch.dist)
  view.x += now.mx - pinch.mx
  view.y += now.my - pinch.my
  pinch = now
}

function onTouchUp(e) {
  if (!touches.delete(e.pointerId)) return
  if (touches.size < 2) pinch = null
}

// ---- cards ----

async function openCard(c) {
  commitFields()
  if (editing.value && !(await leaveEditing())) return
  pickedLink.value = null
  openId.value = c.id
  editing.value = false
}
// A field being typed in (the title) is committed while the card is still
// open: its change event would otherwise come after the card is gone.
function commitFields() {
  const el = document.activeElement
  if (el && el.closest && el.closest('.card') && /^(INPUT|SELECT)$/.test(el.tagName)) el.blur()
}
async function closeCard() {
  commitFields()
  if (editing.value && !(await leaveEditing())) return
  editing.value = false
  openId.value = null
}
// Leaving a card being written: what was typed is saved.
async function leaveEditing() {
  await saveBody()
  return !editing.value
}

async function save(c, patch) {
  if (!c) return false
  error.value = ''
  try {
    const saved = await draftsApi.saveCard(c.id, patch)
    Object.assign(c, saved)
    return true
  } catch (e) {
    error.value = e.message
    return false
  }
}
function setCategory(c, value) {
  save(c, value === '' ? { no_category: true } : { category_id: Number(value) })
}

async function addCard() {
  busy.value = true
  error.value = ''
  try {
    const { w, h } = size()
    const at = freeSpot(cards.value, (w / 2 - view.x) / view.k, (h / 2 - view.y) / view.k)
    const c = await draftsApi.addCard(props.id, { title: t('New card'), ...at, w: 320, h: 260 })
    cards.value.push(c)
    await openCard(c)
    await nextTick()
    titleEl.value?.focus()
    titleEl.value?.select()
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

function cardMenu(c) {
  return [
    { label: t('Edit the text'), action: startEdit },
    { label: t('Delete the card'), danger: true, action: () => removeCard(c) },
  ]
}

async function removeCard(c) {
  if (!(await askConfirm(t('Delete the card “{name}”?', { name: c.title || t('Untitled card') }), { message: t('Its lines go with it. This cannot be undone.'), confirmLabel: t('Delete') }))) return
  try {
    await draftsApi.removeCard(c.id)
    cards.value = cards.value.filter((x) => x.id !== c.id)
    links.value = links.value.filter((l) => l.a !== c.id && l.b !== c.id)
    if (openId.value === c.id) {
      openId.value = null
      editing.value = false
    }
  } catch (e) {
    error.value = e.message
  }
}

// The body is written by name ([[Aurel]]) and stored by id.
function startEdit() {
  const c = openCardRow.value
  if (!c) return
  bodyDraft.value = linksForEditing(c.body, resolver.value)
  editing.value = true
}
async function saveBody() {
  const c = openCardRow.value
  if (!c || !editing.value) return
  if (bodyDraft.value === linksForEditing(c.body, resolver.value) || (await save(c, { body: bodyDraft.value }))) editing.value = false
}

// A [[link]] in a card opens its article in the app.
function onBodyClick(e) {
  const a = e.target.closest && e.target.closest('a[data-wiki]')
  if (!a) return
  e.preventDefault()
  router.push(a.getAttribute('href'))
}

// ---- lines ----

async function join(a, b) {
  try {
    const l = await draftsApi.addLink(props.id, a, b)
    if (!links.value.some((x) => x.id === l.id)) links.value.push(l)
  } catch (e) {
    error.value = e.message
  }
}
async function pickLink(id) {
  pickedLink.value = id
  await nextTick()
  labelEl.value?.focus()
}
async function saveLinkLabel(label) {
  const l = pickedLinkRow.value
  if (!l) return
  try {
    Object.assign(l, await draftsApi.saveLink(l.id, label))
  } catch (e) {
    error.value = e.message
  }
}
async function removeLink() {
  const l = pickedLinkRow.value
  if (!l) return
  try {
    await draftsApi.removeLink(l.id)
    links.value = links.value.filter((x) => x.id !== l.id)
    pickedLink.value = null
  } catch (e) {
    error.value = e.message
  }
}

// ---- categories (shared by every board of the story) ----

async function addCategory() {
  try {
    const color = CATEGORY_COLORS[props.categories.length % CATEGORY_COLORS.length]
    const c = await draftsApi.addCategory(t('New category'), color)
    emit('update:categories', [...props.categories, c])
  } catch (e) {
    error.value = e.message
  }
}
async function saveCategory(c, name, color) {
  try {
    const saved = await draftsApi.saveCategory(c.id, name, color)
    emit('update:categories', props.categories.map((x) => (x.id === c.id ? saved : x)))
  } catch (e) {
    error.value = e.message
  }
}
async function removeCategory(c) {
  if (!(await askConfirm(t('Delete the category “{name}”?', { name: c.name }), { message: t('Its cards stay, without a category.'), confirmLabel: t('Delete') }))) return
  try {
    await draftsApi.removeCategory(c.id)
    for (const card of cards.value) if (card.category_id === c.id) card.category_id = null
    emit('update:categories', props.categories.filter((x) => x.id !== c.id))
  } catch (e) {
    error.value = e.message
  }
}

// ---- tidy up: the wiki graph's layout (graphLayout.js), tiles spread out ----

const SPREAD = 3.2
async function tidy() {
  if (!(await askConfirm(t('Spread every card out by its lines?'), { message: t('The places you gave the cards are lost.'), confirmLabel: t('Tidy up'), danger: false }))) return
  closeCard()
  const index = new Map(cards.value.map((c, i) => [c.id, i]))
  const nodes = cards.value.map(() => ({}))
  const sim = createSimulation(
    nodes,
    links.value.map((l) => ({ source: index.get(l.a), target: index.get(l.b) })),
  )
  for (let i = 0; i < 600 && !sim.cool; i++) sim.tick()
  busy.value = true
  cards.value.forEach((c, i) => {
    c.x = Math.round(nodes[i].x * SPREAD - TILE.w / 2)
    c.y = Math.round(nodes[i].y * SPREAD - TILE.h / 2)
  })
  fit()
  try {
    await Promise.all(cards.value.map((c) => draftsApi.saveCard(c.id, { x: c.x, y: c.y })))
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

// ---- keys ----

function onKey(e) {
  if (e.key !== 'Escape' || e.defaultPrevented || document.querySelector('.overlay')) return
  if (pickedLink.value) pickedLink.value = null
  else if (openId.value && !editing.value) closeCard()
  else if (catsOpen.value) catsOpen.value = false
  else return
  e.preventDefault()
}

// Saves what is being written (leaving the board).
async function flush() {
  if (editing.value) await saveBody()
}
defineExpose({ flush })

onMounted(() => {
  load()
  window.addEventListener('keydown', onKey)
  window.addEventListener('pointermove', onTouchMove)
  window.addEventListener('pointerup', onTouchUp)
  window.addEventListener('pointercancel', onTouchUp)
})
onBeforeUnmount(() => {
  flush()
  window.removeEventListener('keydown', onKey)
  window.removeEventListener('pointermove', onTouchMove)
  window.removeEventListener('pointerup', onTouchUp)
  window.removeEventListener('pointercancel', onTouchUp)
  window.removeEventListener('pointermove', onMove)
  window.removeEventListener('pointerup', onUp)
  window.removeEventListener('pointercancel', onUp)
})
</script>

<style scoped>
.board {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  gap: 0.5rem;
}

.board-bar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.4rem;
}

.spacer {
  flex: 1;
}

.zoom {
  display: flex;
  gap: 0.2rem;
}

.board-error {
  margin: 0;
}

.cat-wrap {
  position: relative;
}

.cat-dots {
  display: inline-flex;
  gap: 2px;
  margin-right: 0.2rem;
}

.cat-dots .dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.cat-pop {
  position: absolute;
  z-index: 30;
  top: calc(100% + 6px);
  left: 0;
  width: 18rem;
  padding: 0.6rem;
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.cat-row {
  display: flex;
  gap: 0.35rem;
  align-items: center;
}

.cat-row input[type='text'] {
  flex: 1;
  min-width: 0;
}

.cat-row input[type='color'] {
  width: 2rem;
  height: 1.8rem;
  padding: 0;
  border: none;
  background: none;
}

.small-text {
  margin: 0;
  font-size: 0.82rem;
}

.board-wrap {
  position: relative;
  flex: 1;
  min-height: 320px;
  overflow: hidden;
  border: 1px solid var(--glass-border);
  border-radius: 12px;
  background-color: rgba(var(--wash), 0.02);
  background-image: radial-gradient(rgba(var(--wash), 0.09) 1px, transparent 1px);
  background-size: 22px 22px;
  touch-action: none;
  cursor: grab;
  user-select: none;
}

.board-wrap.is-panning {
  cursor: grabbing;
}

.board-wrap.is-connecting,
.board-wrap.is-connecting .tile {
  cursor: crosshair;
}

.empty {
  position: absolute;
  inset: 40% 1rem auto;
  text-align: center;
  pointer-events: none;
}

.world {
  position: absolute;
  left: 0;
  top: 0;
  transform-origin: 0 0;
}

.lines {
  position: absolute;
  left: 0;
  top: 0;
  width: 1px;
  height: 1px;
  overflow: visible;
  pointer-events: none;
}

.link .hit {
  stroke: transparent;
  stroke-width: 14;
  pointer-events: stroke;
  cursor: pointer;
}

.stroke {
  stroke: var(--text-muted);
  stroke-width: 2;
  opacity: 0.7;
}

.link.is-picked .stroke {
  stroke: var(--accent);
  opacity: 1;
}

.stroke.is-temp {
  stroke: var(--accent);
  stroke-dasharray: 6 5;
  opacity: 1;
}

.link-label {
  font-size: 12px;
  text-anchor: middle;
  fill: var(--text-primary);
  paint-order: stroke;
  stroke: var(--bg);
  stroke-width: 4px;
  stroke-linejoin: round;
}

.link-pop {
  position: absolute;
  z-index: 25;
  transform: translate(-50%, 10px);
  display: flex;
  gap: 0.3rem;
  padding: 0.4rem;
  cursor: auto;
}

.link-pop input {
  width: 11rem;
}

.tile {
  position: absolute;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  border-radius: 10px;
  border: 1px solid var(--glass-border);
  background: var(--bg-elevated, var(--bg));
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.25);
  overflow: visible;
  cursor: pointer;
}

.tile:focus-visible {
  outline: 2px solid var(--accent);
}

.tile-title {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0 0.6rem;
  font-weight: 600;
  font-size: 0.92rem;
  text-align: center;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  border-bottom: 1px solid var(--glass-border);
}

.tile-cat {
  height: 1.45rem;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 0.75rem;
  border-radius: 0 0 9px 9px;
  background: var(--cat, rgba(var(--wash), 0.05));
  color: var(--cat-ink, var(--text-muted));
  white-space: nowrap;
  overflow: hidden;
}

.knob {
  position: absolute;
  right: -7px;
  top: 50%;
  width: 12px;
  height: 12px;
  margin-top: -6px;
  border-radius: 50%;
  background: var(--accent);
  border: 2px solid var(--bg);
  cursor: crosshair;
  opacity: 0;
  transition: opacity 0.12s;
}

.tile:hover .knob,
.card:hover .knob,
.tile:focus-visible .knob {
  opacity: 1;
}

@media (hover: none) {
  .knob {
    opacity: 0.8;
  }
}

.card {
  position: absolute;
  z-index: 20;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  border-radius: 12px;
  cursor: auto;
  user-select: text;
}

.card-head {
  position: relative;
  display: flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.4rem 0.5rem;
  border-radius: 11px 11px 0 0;
  border-bottom: 3px solid var(--cat, var(--glass-border));
  cursor: move;
}

.card-title {
  flex: 1 1 8rem;
  width: auto;
  min-width: 4rem;
  font-weight: 600;
  background: transparent;
  border-color: transparent;
}

.card-title:hover,
.card-title:focus {
  border-color: var(--glass-border);
}

.card-cat {
  flex: 0 1 auto;
  width: auto;
  min-width: 0;
  max-width: 8rem;
}

.close {
  border: none;
  background: none;
  color: var(--text-muted);
  font-size: 1.2rem;
  cursor: pointer;
  padding: 0 0.3rem;
}

.card-head .knob {
  top: auto;
  bottom: -9px;
  right: 50%;
  margin: 0 -6px 0 0;
}

.card-body {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 0.6rem 0.8rem;
}

.card-md {
  min-height: 100%;
}

/* The board takes every touch (touch-action: none); an open card's text
   still scrolls with a finger. */
.card-body,
.card-body :deep(textarea) {
  touch-action: pan-y;
}

/* Fingers: a bigger dot to join cards and wider edges to resize them. */
@media (pointer: coarse) {
  .knob {
    width: 20px;
    height: 20px;
    margin-top: -10px;
    right: -11px;
  }

  .card-head .knob {
    bottom: -13px;
    margin-right: -10px;
  }

  .h-n,
  .h-s {
    height: 18px;
  }

  .h-e,
  .h-w {
    width: 18px;
  }

  .h-n {
    top: -9px;
  }

  .h-s {
    bottom: -9px;
  }

  .h-e {
    right: -9px;
  }

  .h-w {
    left: -9px;
  }

  .h-ne,
  .h-nw,
  .h-se,
  .h-sw {
    width: 26px;
    height: 26px;
  }
}

.write-here {
  width: 100%;
  height: 100%;
  border: 1px dashed var(--glass-border);
  border-radius: 8px;
  background: none;
  cursor: text;
}

.edit-actions {
  display: flex;
  gap: 0.4rem;
  margin-top: 0.4rem;
}

.handle {
  position: absolute;
  z-index: 2;
}

.h-n,
.h-s {
  left: 10px;
  right: 10px;
  height: 8px;
  cursor: ns-resize;
}

.h-e,
.h-w {
  top: 10px;
  bottom: 10px;
  width: 8px;
  cursor: ew-resize;
}

.h-n {
  top: -4px;
}

.h-s {
  bottom: -4px;
}

.h-e {
  right: -4px;
}

.h-w {
  left: -4px;
}

.h-ne,
.h-nw,
.h-se,
.h-sw {
  width: 14px;
  height: 14px;
}

.h-ne {
  top: -5px;
  right: -5px;
  cursor: nesw-resize;
}

.h-sw {
  bottom: -5px;
  left: -5px;
  cursor: nesw-resize;
}

.h-nw {
  top: -5px;
  left: -5px;
  cursor: nwse-resize;
}

.h-se {
  bottom: -5px;
  right: -5px;
  cursor: nwse-resize;
}
</style>
