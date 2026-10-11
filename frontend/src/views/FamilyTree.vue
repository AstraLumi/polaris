<template>
  <div class="page is-wide family-page">
    <BackLink :to="`/characters/${id}`" :label="$t('← Back to the character')" />

    <header class="page-header">
      <div>
        <h1>{{ rootPerson ? $t('Family of {name}', { name: rootPerson.name }) : $t('Family tree') }}</h1>
        <p class="page-sub">
          <template v-if="data">{{ $tn('{n} person', '{n} people', data.people.length) }}</template>
          <template v-else>&nbsp;</template>
        </p>
      </div>
      <div v-if="data" class="head-actions">
        <RouterLink :to="`/characters/${id}`" class="btn btn-ghost">{{ $t('Character sheet') }}</RouterLink>
        <RouterLink :to="wikiPath('character', id)" class="btn btn-ghost">{{ $t('Wiki article') }}</RouterLink>
        <RouterLink
          :to="{ path: '/wiki/graph', query: { focus: 'character:' + id, depth: 1, types: 'character' } }"
          class="btn btn-ghost"
          :title="$t('Everyone this character is connected to, on the graph')"
        >
          {{ $t('Relationship web') }}
        </RouterLink>
      </div>
    </header>

    <p v-if="loadError" class="error-banner">{{ loadError }}</p>
    <p v-else-if="!data" class="loading-hint">{{ $t('Loading…') }}</p>

    <div v-else-if="data.people.length === 1" class="empty-state glass-panel">
      <p class="empty-title">{{ $t('No family yet.') }}</p>
      <p class="empty-hint">
        {{ $t('Add parents, children, siblings or partners in the Relations tab of the character, and they appear here.') }}
      </p>
    </div>

    <template v-else>
      <div class="legend">
        <span><i class="key is-couple"></i>{{ $t('Partners') }}</span>
        <span><i class="key is-ended"></i>{{ $t('Former partners') }}</span>
        <span><i class="key is-sibling"></i>{{ $t('Siblings (no parents known)') }}</span>
        <span class="zoom">
          <button type="button" class="tool-btn" :title="$t('Zoom out')" @click="setZoom(zoom / 1.2)">−</button>
          <span class="zoom-value">{{ Math.round(zoom * 100) }}%</span>
          <button type="button" class="tool-btn" :title="$t('Zoom in')" @click="setZoom(zoom * 1.2)">+</button>
        </span>
      </div>

      <div ref="scroller" class="tree-wrap">
        <svg
          :width="(tree.width + PAD * 2) * zoom"
          :height="(tree.height + PAD * 2) * zoom"
          :viewBox="`${-PAD} ${-PAD} ${tree.width + PAD * 2} ${tree.height + PAD * 2}`"
          class="tree"
          role="img"
          :aria-label="$t('Family tree')"
        >
          <path v-for="(d, i) in connectors" :key="'f' + i" :d="d" class="line" />
          <line
            v-for="(c, i) in coupleLines"
            :key="'c' + i"
            :x1="c.x1"
            :y1="c.y1"
            :x2="c.x2"
            :y2="c.y2"
            class="line couple"
            :class="{ ended: c.ended }"
          />
          <path v-for="(d, i) in siblingLines" :key="'s' + i" :d="d" class="line sibling" />

          <foreignObject
            v-for="p in data.people"
            :key="p.id"
            :x="tree.pos.get(p.id).x"
            :y="tree.pos.get(p.id).y"
            :width="CARD_W"
            :height="CARD_H"
          >
            <button
              type="button"
              class="person"
              :class="{ 'is-root': p.id === data.root, ['is-' + p.status]: true }"
              :title="p.id === data.root ? $t('Open the character sheet') : $t('Centre the tree on {name}', { name: p.name })"
              @click="openPerson(p)"
            >
              <span class="pic"><IconImage :src="p.picture" :name="p.name" :alt="p.name" /></span>
              <span class="who">
                <strong data-tip-overflow>{{ p.name }}</strong>
                <small>
                  <template v-if="bornYear(p)">{{ $t('b. {year}', { year: bornYear(p) }) }}</template>
                  <template v-if="p.status !== 'alive'">{{ bornYear(p) ? ' · ' : '' }}{{ statusLabel(p.status) }}</template>
                </small>
              </span>
            </button>
          </foreignObject>
        </svg>
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, watch, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import IconImage from '../components/IconImage.vue'
import BackLink from '../components/BackLink.vue'
import { fetchFamily } from '../api'
import { wikiPath } from '../wiki'
import { dateParts } from '../calendar'
import { statusLabel } from '../characterStatus'
import { pageTitle } from '../navigation'
import { t } from '../i18n'
import { layoutFamily, CARD_W, CARD_H } from '../familyLayout'

// A character's family tree: everyone joined to them through parent,
// sibling, spouse and partner relations (backend/family.go), laid out in
// generations (familyLayout.js). Clicking someone redraws the tree around
// them.

const props = defineProps({ id: { type: String, required: true } })
const router = useRouter()

const PAD = 40
const data = ref(null)
const loadError = ref('')
const scroller = ref(null)
const zoom = ref(1)

const rootPerson = computed(() => data.value?.people.find((p) => p.id === data.value.root) || null)
const tree = computed(() => layoutFamily(data.value.people, data.value.links, data.value.root))

const box = (id) => tree.value.pos.get(id)
const cx = (id) => box(id).x + CARD_W / 2

// Parents to children: down from the parents (from the middle of a couple's
// line), along a bar, and down into each child.
const connectors = computed(() =>
  tree.value.families.map((f) => {
    let ox
    let oy
    const [a, b] = f.parents
    const couple = f.parents.length === 2 && tree.value.couples.some((c) => c.adjacent && ((c.a === a && c.b === b) || (c.a === b && c.b === a)))
    if (couple) {
      ox = (cx(a) + cx(b)) / 2
      oy = box(a).y + CARD_H / 2
    } else {
      ox = f.parents.reduce((s, p) => s + cx(p), 0) / f.parents.length
      oy = Math.max(...f.parents.map((p) => box(p).y)) + CARD_H
    }
    const top = Math.min(...f.children.map((c) => box(c).y))
    const barY = top - 28
    const xs = f.children.map(cx)
    let d = `M ${ox} ${oy} V ${barY} M ${Math.min(ox, ...xs)} ${barY} H ${Math.max(ox, ...xs)}`
    for (const c of f.children) d += ` M ${cx(c)} ${barY} V ${box(c).y}`
    return d
  }),
)

const coupleLines = computed(() =>
  tree.value.couples.map((c) => {
    const [l, r] = box(c.a).x <= box(c.b).x ? [c.a, c.b] : [c.b, c.a]
    if (c.adjacent) {
      const y = box(l).y + CARD_H / 2
      return { x1: box(l).x + CARD_W, y1: y, x2: box(r).x, y2: y, ended: c.ended }
    }
    return { x1: cx(l), y1: box(l).y + CARD_H / 2, x2: cx(r), y2: box(r).y + CARD_H / 2, ended: c.ended }
  }),
)

// Siblings with no parents known: a bracket over the two cards.
const siblingLines = computed(() =>
  tree.value.siblingPairs.map(({ a, b }) => {
    const y = Math.min(box(a).y, box(b).y) - 14
    return `M ${cx(a)} ${box(a).y} V ${y} H ${cx(b)} V ${box(b).y}`
  }),
)

const bornYear = (p) => dateParts(p.born)?.year ?? ''

function openPerson(p) {
  if (p.id === data.value.root) router.push(`/characters/${p.id}`)
  else router.push(`/characters/${p.id}/family`)
}

function setZoom(z) {
  zoom.value = Math.min(2, Math.max(0.3, z))
}

// Bring the person the tree is about into view.
async function centreRoot() {
  await nextTick()
  const el = scroller.value
  if (!el || !data.value) return
  const p = box(data.value.root)
  el.scrollLeft = (p.x + PAD + CARD_W / 2) * zoom.value - el.clientWidth / 2
  el.scrollTop = (p.y + PAD + CARD_H / 2) * zoom.value - el.clientHeight / 2
}

async function load() {
  data.value = null
  loadError.value = ''
  try {
    data.value = await fetchFamily(props.id)
    pageTitle.value = t('Family of {name}', { name: rootPerson.value?.name || '' })
    // Start zoomed out enough for a wide tree to fit, within reason.
    await nextTick()
    const w = scroller.value?.clientWidth || 900
    zoom.value = Math.min(1, Math.max(0.7, w / (tree.value.width + PAD * 2)))
    centreRoot()
  } catch (e) {
    loadError.value = e.message
  }
}

watch(() => props.id, load, { immediate: true })
</script>

<style scoped>
.head-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.legend {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem 1.4rem;
  margin-bottom: 0.7rem;
  font-size: 0.78rem;
  color: var(--text-muted);
}

.legend > span {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
}

.key {
  display: inline-block;
  width: 22px;
  border-top: 2px solid var(--line);
}

.key.is-ended {
  border-top-style: dashed;
}

.key.is-sibling {
  border-top: 2px dotted var(--text-muted);
}

.zoom {
  margin-left: auto;
}

.zoom-value {
  min-width: 3rem;
  text-align: center;
  font-size: 0.74rem;
  color: var(--text-faint);
}

.tool-btn {
  min-width: 2rem;
  height: 2rem;
  border: 1px solid var(--glass-border);
  border-radius: 8px;
  background: color-mix(in srgb, var(--surface) 90%, transparent);
  color: var(--text-primary);
  font-size: 0.95rem;
  font-weight: 600;
  cursor: pointer;
}

.tool-btn:hover {
  border-color: var(--accent);
}

.tree-wrap {
  height: calc(100vh - 15rem);
  min-height: 380px;
  overflow: auto;
  border: 1px solid var(--glass-border);
  border-radius: 16px;
  background:
    radial-gradient(circle at 30% 15%, color-mix(in srgb, var(--tint) 10%, transparent), transparent 55%),
    var(--bg);
}

.tree {
  display: block;
  margin: 0 auto;
}

.line {
  fill: none;
  stroke: color-mix(in srgb, var(--text-muted) 70%, transparent);
  stroke-width: 1.6;
}

.line.couple {
  stroke: var(--line);
  stroke-width: 2.4;
}

.line.couple.ended {
  stroke-dasharray: 6 5;
}

.line.sibling {
  stroke-dasharray: 2 4;
}

.person {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  width: 100%;
  height: 100%;
  padding: 0.45rem 0.55rem;
  border: 1px solid var(--glass-border);
  border-radius: 12px;
  background: color-mix(in srgb, var(--surface) 92%, transparent);
  color: var(--text-primary);
  font-family: 'Manrope', sans-serif;
  text-align: left;
  cursor: pointer;
  transition: border-color 0.15s ease, transform 0.15s ease;
}

.person:hover,
.person:focus-visible {
  border-color: var(--accent);
}

.person.is-root {
  border-color: var(--accent);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--accent) 35%, transparent);
}

.person.is-dead {
  opacity: 0.75;
}

.pic {
  flex: none;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2.6rem;
  height: 2.6rem;
  overflow: hidden;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.06);
  color: var(--text-muted);
  font-weight: 700;
}

.pic :deep(img) {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.who {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.who strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.85rem;
}

.who small {
  font-size: 0.7rem;
  color: var(--text-faint);
}
</style>
