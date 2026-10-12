<template>
  <aside class="loc-list glass-panel panel-solid" :class="{ 'is-closed': closed }" :aria-label="$t('Major locations')">
    <header class="head">
      <button type="button" class="fold" :title="closed ? $t('Show the list') : $t('Hide the list')" @click="setClosed(!closed)">
        {{ closed ? '‹' : '›' }}
      </button>
      <strong v-if="!closed">{{ $t('Major locations') }}</strong>
      <button v-if="!closed" type="button" class="btn btn-primary small add" @click="$emit('new')">{{ $t('New') }}</button>
    </header>

    <template v-if="!closed">
      <input v-if="majors.length > 6" v-model="filter" type="search" class="filter" :placeholder="$t('Find…')" :aria-label="$t('Find a major location')" />
      <p v-if="!majors.length" class="empty">{{ $t('No major locations yet') }}</p>

      <div class="groups">
        <section v-for="g in groups" :key="g.id" class="group">
          <button type="button" class="group-head" :aria-expanded="!folded[g.id]" @click="fold(g.id)">
            <span class="caret">{{ folded[g.id] ? '▸' : '▾' }}</span>
            {{ $t(g.label) }}
            <span class="count">{{ g.items.length }}</span>
          </button>
          <ul v-if="!folded[g.id]">
            <li
              v-for="m in g.items"
              :key="m.id"
              class="row"
              :class="{ active: activeId === m.id }"
              :title="activeId === m.id ? $t('Picked for painting; click to let go') : $t('Pick to paint with')"
              @click="$emit('pick', m)"
            >
              <span class="dot" :class="{ none: !m.color, ocean: m.realm === 'ocean' && m.color }" :style="m.color ? { '--c': m.color } : {}"></span>
              <span class="name" data-tip-overflow>{{ m.name }}</span>
              <button type="button" class="edit" :title="$t('Edit')" :aria-label="$t('Edit {name}', { name: m.name })" @click.stop="$emit('edit', m)">✎</button>
            </li>
          </ul>
        </section>
      </div>
    </template>
  </aside>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { tr } from '../i18n'

// The map's major locations down the side, grouped by what they are, each
// group folding away. Click one to paint with it, ✎ to edit it. Shared by
// the 2D map and the planet; what is folded (and whether the list is shown
// at all) is remembered on this device.
const props = defineProps({
  majors: { type: Array, default: () => [] },
  activeId: { type: Number, default: null },
})
const emit = defineEmits(['pick', 'edit', 'new', 'closed'])

const GROUPS = [
  { id: 'kingdom', label: tr('Kingdoms') },
  { id: 'region', label: tr('Regions') },
  { id: 'ocean', label: tr('Oceans') },
  { id: 'unknown', label: tr('Unknown lands') },
  { id: 'none', label: tr('Without borders') },
]

const KEY = 'polaris-location-list'
function readState() {
  try {
    return JSON.parse(localStorage.getItem(KEY) || '{}')
  } catch {
    return {}
  }
}
const saved = readState()
const folded = ref(saved.folded || {})
const closed = ref(!!saved.closed)
const filter = ref('')

function store() {
  try {
    localStorage.setItem(KEY, JSON.stringify({ folded: folded.value, closed: closed.value }))
  } catch {
    // Not remembered; the list still works.
  }
}

function fold(id) {
  folded.value = { ...folded.value, [id]: !folded.value[id] }
  store()
}

function setClosed(v) {
  closed.value = v
  store()
  emit('closed', v)
}

const groupOf = (m) => (m.color ? m.realm || 'kingdom' : 'none')
const groups = computed(() => {
  const f = filter.value.trim().toLowerCase()
  const list = props.majors.filter((m) => !f || m.name.toLowerCase().includes(f))
  return GROUPS.map((g) => ({
    ...g,
    items: list.filter((m) => groupOf(m) === g.id).sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' })),
  })).filter((g) => g.items.length)
})

// The map makes room for the list while it is shown.
onMounted(() => emit('closed', closed.value))
</script>

<style scoped>
.loc-list {
  position: absolute;
  top: 16px;
  right: 16px;
  bottom: 16px;
  width: 240px;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding: 0.55rem;
  z-index: 7;
}

.loc-list.is-closed {
  bottom: auto;
  width: auto;
}

.head {
  display: flex;
  align-items: center;
  gap: 0.45rem;
}

.head strong {
  flex: 1;
  font-size: 0.82rem;
}

.fold {
  border: 1px solid var(--glass-border);
  border-radius: 8px;
  background: none;
  color: var(--text-muted);
  width: 1.8rem;
  height: 1.8rem;
  font-size: 1rem;
  line-height: 1;
  cursor: pointer;
}

.fold:hover {
  color: var(--text-primary);
}

.filter {
  min-height: 1.9rem;
  padding: 0.25rem 0.55rem;
  font-size: 0.8rem;
}

.empty {
  margin: 0.2rem 0.3rem;
  font-size: 0.8rem;
  color: var(--text-faint);
}

.groups {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.group-head {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  width: 100%;
  padding: 0.3rem 0.35rem;
  border: none;
  border-radius: 6px;
  background: none;
  color: var(--text-faint);
  font-family: inherit;
  font-size: 0.7rem;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  cursor: pointer;
}

.group-head:hover {
  color: var(--text-primary);
  background: rgba(255, 255, 255, 0.04);
}

.caret {
  width: 0.8rem;
}

.count {
  margin-left: auto;
  font-weight: 600;
  letter-spacing: 0;
}

ul {
  margin: 0;
  padding: 0;
  list-style: none;
}

.row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.32rem 0.35rem 0.32rem 0.55rem;
  border: 1px solid transparent;
  border-radius: 8px;
  color: var(--text-muted);
  font-size: 0.82rem;
  cursor: pointer;
}

.row:hover {
  color: var(--text-primary);
  background: rgba(255, 255, 255, 0.04);
}

.row.active {
  border-color: var(--accent);
  background: var(--accent-soft);
  color: var(--text-primary);
}

.dot {
  flex: none;
  width: 11px;
  height: 11px;
  border-radius: 50%;
  background: var(--c);
}

.dot.none {
  border: 1.5px dashed var(--text-faint);
  background: none;
}

/* Oceans are drawn as an outline, so is their dot. */
.dot.ocean {
  background: none;
  border: 2px solid var(--c);
}

.name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.edit {
  border: none;
  background: none;
  color: var(--text-faint);
  font-size: 0.8rem;
  padding: 0 0.25rem;
  cursor: pointer;
}

.edit:hover {
  color: var(--text-primary);
}

@media (max-width: 720px) {
  .loc-list {
    top: auto;
    right: 8px;
    bottom: 8px;
    left: 80px;
    width: auto;
    max-height: 42%;
  }
  .loc-list.is-closed {
    left: auto;
  }
}
</style>
