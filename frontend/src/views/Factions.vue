<template>
  <div class="page">
    <header class="page-header">
      <div>
        <h1>{{ $t('Factions') }}</h1>
        <p class="page-sub">{{ $tn('{n} faction', '{n} factions', factions.length) }}</p>
      </div>
      <button class="btn btn-primary" @click="openForm(null)">{{ $t('New faction') }}</button>
    </header>

    <p v-if="loadError" class="error-banner">{{ loadError }}</p>
    <p v-else-if="loading" class="loading-hint">{{ $t('Loading…') }}</p>

    <div v-else-if="!factions.length" class="empty-state glass-panel">
      <p class="empty-title">{{ $t('No factions yet.') }}</p>
      <p class="empty-hint">{{ $t('Guilds, orders, churches, noble houses: anything your characters belong to.') }}</p>
    </div>

    <div v-else class="grid">
      <article
        v-for="f in ordered"
        :key="f.id"
        class="card glass-panel"
        :style="{ '--depth': f.depth, ...(f.color ? { '--faction': f.color } : {}) }"
      >
        <RouterLink :to="wikiPath('faction', f.id)" class="card-main">
          <span class="badge" :class="{ 'has-color': f.color }">
            <IconImage :src="f.picture_path" :name="f.name" />
          </span>
          <span class="card-text">
            <strong>{{ f.name }} <span v-if="f.kingdom_id" class="kingdom-badge" :title="$t('Tied to the kingdom of the same name on the map')">{{ $t('Kingdom') }}</span></strong>
            <small v-if="f.parent_name">{{ $t('Part of {name}', { name: f.parent_name }) }}</small>
            <small>
              {{ $tn('{n} member', '{n} members', f.member_count) }}<template v-if="f.hq_name"> · {{ f.hq_name }}</template>
            </small>
          </span>
        </RouterLink>
        <button type="button" class="btn btn-ghost small" @click="openForm(f)">{{ $t('Edit') }}</button>
      </article>
    </div>

    <FactionFormModal
      v-if="formOpen"
      :faction="editing"
      :all-factions="factions"
      @close="formOpen = false"
      @saved="onChanged"
      @deleted="onChanged"
    />
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { t } from '../i18n'
import { factionsApi } from '../api'
import { wikiPath } from '../wiki'
import IconImage from '../components/IconImage.vue'
import FactionFormModal from '../components/FactionFormModal.vue'

const factions = ref([])
const loading = ref(true)
const loadError = ref('')
const formOpen = ref(false)
const editing = ref(null)

// Parents first, each followed by the factions inside it (indented).
const ordered = computed(() => {
  const byParent = new Map()
  for (const f of factions.value) {
    const k = f.parent_id ?? 0
    if (!byParent.has(k)) byParent.set(k, [])
    byParent.get(k).push(f)
  }
  const out = []
  const seen = new Set()
  const walk = (parent, depth) => {
    for (const f of byParent.get(parent) || []) {
      if (seen.has(f.id)) continue
      seen.add(f.id)
      out.push({ ...f, depth: Math.min(depth, 3) })
      walk(f.id, depth + 1)
    }
  }
  walk(0, 0)
  for (const f of factions.value) if (!seen.has(f.id)) out.push({ ...f, depth: 0 })
  return out
})

async function load() {
  loadError.value = ''
  try {
    factions.value = await factionsApi.list()
  } catch (e) {
    loadError.value = t("Couldn't load factions. Try refreshing.")
  } finally {
    loading.value = false
  }
}

async function openForm(f) {
  try {
    editing.value = f ? await factionsApi.get(f.id) : null
    formOpen.value = true
  } catch (e) {
    loadError.value = e.message
  }
}

function onChanged() {
  formOpen.value = false
  load()
}

onMounted(load)
</script>

<style scoped>
.kingdom-badge {
  margin-left: 0.35rem;
  padding: 0.02rem 0.45rem;
  border: 1px solid var(--glass-border);
  border-radius: 999px;
  font-size: 0.66rem;
  font-weight: 600;
  color: var(--text-muted);
  vertical-align: middle;
}

.grid {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
}

.card {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.75rem 1rem;
  margin-left: calc(var(--depth, 0) * 1.75rem);
  border-left: 3px solid var(--faction, var(--glass-border));
}

.card-main {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 0.85rem;
  text-decoration: none;
}

.badge {
  width: 2.8rem;
  height: 2.8rem;
  flex-shrink: 0;
  border-radius: 10px;
  overflow: hidden;
  background: var(--accent-soft);
  color: var(--accent);
  display: flex;
  align-items: center;
  justify-content: center;
}

.badge.has-color {
  background: color-mix(in srgb, var(--faction) 22%, transparent);
  color: var(--text-primary);
}

.card-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.card-text strong {
  color: var(--text-primary);
  font-size: 0.98rem;
}

.card-main:hover strong {
  color: var(--accent);
}

.card-text small {
  font-size: 0.76rem;
  color: var(--text-faint);
}

@media (max-width: 720px) {
  .card {
    margin-left: calc(var(--depth, 0) * 0.75rem);
  }
}
</style>
