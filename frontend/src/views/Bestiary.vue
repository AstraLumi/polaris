<template>
  <div class="page">
    <header class="page-header">
      <div>
        <h1>{{ $t('Bestiary') }}</h1>
        <p class="page-sub">{{ $tn('{n} creature', '{n} creatures', creatures.length) }}</p>
      </div>
      <button class="btn btn-primary" @click="openForm(null)">{{ $t('New creature') }}</button>
    </header>

    <p v-if="loadError" class="error-banner">{{ loadError }}</p>
    <p v-else-if="loading" class="loading-hint">{{ $t('Loading…') }}</p>

    <div v-else-if="!creatures.length" class="empty-state glass-panel">
      <p class="empty-title">{{ $t('No creatures yet.') }}</p>
      <p class="empty-hint">{{ $t('Monsters, animals and constructs: what lives in your world and where.') }}</p>
    </div>

    <template v-else>
      <div class="filters">
        <input v-model="search" type="search" :placeholder="$t('Search names and regions…')" :aria-label="$t('Search')" />
        <div class="kind-filter" role="radiogroup" :aria-label="$t('Tag')">
          <button
            v-for="k in [{ id: '', label: tr('All') }, ...CREATURE_KINDS]"
            :key="k.id"
            type="button"
            role="radio"
            class="kind-chip"
            :class="{ 'is-on': kind === k.id }"
            :aria-checked="kind === k.id"
            @click="kind = k.id"
          >
            {{ $t(k.label) }}
          </button>
        </div>
      </div>

      <p v-if="!shown.length" class="note">{{ $t('No creature matches.') }}</p>
      <div class="grid">
        <article v-for="c in shown" :key="c.id" class="card glass-panel" :class="'kind-' + c.kind">
          <RouterLink :to="wikiPath('creature', c.id)" class="card-main">
            <span class="badge"><IconImage :src="c.picture_path" :name="c.name" /></span>
            <span class="card-text">
              <strong>{{ c.name }}</strong>
              <small>
                <span class="kind-tag">{{ kindLabel(c.kind) }}</span>
                <template v-if="levelText(c)"> · {{ $t('Lv. {range}', { range: levelText(c) }) }}</template>
                <template v-if="c.region_name"> · {{ c.region_name }}</template>
              </small>
              <small v-if="c.description" class="desc">{{ c.description }}</small>
              <small v-if="c.related.length" class="related">{{ $t('Related: {list}', { list: c.related.map((r) => r.name).join(', ') }) }}</small>
            </span>
          </RouterLink>
          <button type="button" class="btn btn-ghost small" @click="openForm(c)">{{ $t('Edit') }}</button>
        </article>
      </div>
    </template>

    <CreatureFormModal
      v-if="formOpen"
      :creature="editing"
      :all-creatures="creatures"
      @close="formOpen = false"
      @saved="onChanged"
      @deleted="onChanged"
    />
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { t, tr } from '../i18n'
import { creaturesApi } from '../api'
import { wikiPath } from '../wiki'
import { CREATURE_KINDS, kindLabel, levelText } from '../creatures'
import IconImage from '../components/IconImage.vue'
import CreatureFormModal from '../components/CreatureFormModal.vue'

// The bestiary: every creature of the world. A card opens its wiki article;
// Edit opens the form.

const creatures = ref([])
const loading = ref(true)
const loadError = ref('')
const formOpen = ref(false)
const editing = ref(null)
const search = ref('')
const kind = ref('')

const shown = computed(() => {
  const q = search.value.trim().toLowerCase()
  return creatures.value.filter(
    (c) =>
      (!kind.value || c.kind === kind.value) &&
      (!q || c.name.toLowerCase().includes(q) || (c.region_name || '').toLowerCase().includes(q)),
  )
})

async function load() {
  loadError.value = ''
  try {
    creatures.value = await creaturesApi.list()
  } catch (e) {
    loadError.value = t("Couldn't load the bestiary. Try refreshing.")
  } finally {
    loading.value = false
  }
}

function openForm(c) {
  editing.value = c
  formOpen.value = true
}

function onChanged() {
  formOpen.value = false
  load()
}

onMounted(load)
</script>

<style scoped>
.filters {
  display: flex;
  flex-wrap: wrap;
  gap: 0.6rem;
  margin-bottom: 1rem;
}

.filters input {
  flex: 1 1 16rem;
  max-width: 24rem;
}

.kind-filter {
  display: flex;
  gap: 0.35rem;
}

.kind-chip {
  min-height: var(--control-h);
  padding: 0 0.85rem;
  border-radius: 8px;
  border: 1px solid var(--glass-border);
  background: rgba(255, 255, 255, 0.03);
  color: var(--text-muted);
  font: inherit;
  font-size: 0.85rem;
  cursor: pointer;
}

.kind-chip.is-on {
  color: var(--text-primary);
  border-color: var(--accent);
  background: var(--accent-soft);
}

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 0.75rem;
}

.card {
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
  padding: 0.85rem 1rem;
  border-left: 3px solid var(--kind-color, var(--glass-border));
}

.card.kind-monster {
  --kind-color: var(--danger);
}
.card.kind-animal {
  --kind-color: var(--kind-founding);
}
.card.kind-construct {
  --kind-color: var(--kind-event, var(--accent));
}

.card-main {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: flex-start;
  gap: 0.85rem;
  text-decoration: none;
}

.badge {
  width: 3.2rem;
  height: 3.2rem;
  flex-shrink: 0;
  border-radius: 10px;
  overflow: hidden;
  background: var(--accent-soft);
  color: var(--accent);
  display: flex;
  align-items: center;
  justify-content: center;
}

.card-text {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
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

.kind-tag {
  color: var(--kind-color, var(--text-muted));
  font-weight: 700;
}

.desc {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  color: var(--text-muted) !important;
}

.note {
  color: var(--text-faint);
  font-size: 0.85rem;
}
</style>
