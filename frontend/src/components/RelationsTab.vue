<template>
  <div class="relations-tab">
    <p v-if="error" class="error-banner">{{ error }}</p>

    <section>
      <div class="section-head">
        <h3>{{ $t('Relations') }}</h3>
        <span class="head-links no-print">
          <RouterLink :to="`/characters/${characterId}/family`" class="btn btn-ghost small" :title="$t('Parents, children, siblings and partners, as a tree')">
            {{ $t('Family tree') }}
          </RouterLink>
          <RouterLink
            :to="{ path: '/wiki/graph', query: { focus: 'character:' + characterId, depth: 1, types: 'character' } }"
            class="btn btn-ghost small"
            :title="$t('Everyone this character is connected to, on the graph')"
          >
            {{ $t('Relationship web') }}
          </RouterLink>
          <button type="button" class="btn btn-ghost small" @click="openForm(null)">{{ $t('Add relation') }}</button>
        </span>
      </div>
      <p v-if="!relations.length" class="empty">{{ $t('No relations yet.') }}</p>
      <ul v-else class="rows">
        <li v-for="row in rows" :key="row.rel.id" class="row">
          <RouterLink :to="`/characters/${row.other.id}`" class="avatar" :title="row.other.name">
            <IconImage :src="row.other.picture_path" :name="row.other.name" />
          </RouterLink>
          <div class="row-text">
            <p class="line">
              <span class="wording">{{ row.wording }}</span>
              <RouterLink :to="`/characters/${row.other.id}`">{{ row.other.name }}</RouterLink>
            </p>
            <p v-if="row.span" class="meta">{{ row.span }}</p>
            <p v-if="row.rel.notes" class="notes">{{ row.rel.notes }}</p>
          </div>
          <button type="button" class="btn btn-ghost small no-print" @click="openForm(row.rel)">{{ $t('Edit') }}</button>
        </li>
      </ul>
    </section>

    <section>
      <div class="section-head">
        <h3>{{ $t('Factions') }}</h3>
        <RouterLink to="/factions" class="btn btn-ghost small no-print">{{ $t('Manage factions') }}</RouterLink>
      </div>
      <p v-if="!factions.length" class="empty">{{ $t('Not in any faction.') }}</p>
      <ul v-else class="rows">
        <li v-for="f in factions" :key="f.faction_id + f.role + f.since" class="row">
          <RouterLink :to="wikiPath('faction', f.faction_id)" class="avatar" :title="f.name" :style="f.color ? { borderColor: f.color } : {}">
            <IconImage :src="f.picture_path" :name="f.name" />
          </RouterLink>
          <div class="row-text">
            <p class="line">
              <RouterLink :to="wikiPath('faction', f.faction_id)">{{ f.name }}</RouterLink>
              <span v-if="f.role" class="wording"> · {{ f.role }}</span>
            </p>
            <p v-if="spanText(f.since, f.until)" class="meta">{{ spanText(f.since, f.until) }}</p>
          </div>
        </li>
      </ul>
    </section>

    <RelationFormModal
      v-if="formOpen"
      :me="me"
      :relation="editing"
      :characters="characters"
      @close="formOpen = false"
      @saved="onSaved"
      @delete="confirmTarget = $event"
    />

    <Teleport to="body">
      <ConfirmDialog
        v-if="confirmTarget"
        :title="$t('Delete this relation?')"
        :message="$t('It disappears from both characters.')"
        :confirm-label="$t('Delete')"
        @cancel="confirmTarget = null"
        @confirm="remove"
      />
    </Teleport>
  </div>
</template>

<script setup>
// A character's relations to other characters, and the factions they are or
// were in. Relations are edited here; memberships on the Factions page.
import { ref, computed, watch } from 'vue'
import { t } from '../i18n'
import { fetchConnections, fetchCharacterOptions, relationsApi } from '../api'
import { relationFor } from '../relations'
import { shortDate } from '../calendar'
import { wikiPath } from '../wiki'
import IconImage from './IconImage.vue'
import RelationFormModal from './RelationFormModal.vue'
import ConfirmDialog from './ConfirmDialog.vue'

const props = defineProps({
  characterId: { type: Number, required: true },
  characterName: { type: String, required: true },
})

const relations = ref([])
const factions = ref([])
const characters = ref([])
const error = ref('')
const formOpen = ref(false)
const editing = ref(null)
const confirmTarget = ref(null)

const me = computed(() => ({ id: props.characterId, name: props.characterName }))

function spanText(since, until) {
  if (since && until) return t('{from} to {to}', { from: shortDate(since) || since, to: shortDate(until) || until })
  if (since) return t('since {date}', { date: shortDate(since) || since })
  if (until) return t('until {date}', { date: shortDate(until) || until })
  return ''
}

const rows = computed(() =>
  relations.value.map((rel) => {
    const { other, wording } = relationFor(rel, props.characterId)
    return { rel, other, wording, span: spanText(rel.since, rel.until) }
  }),
)

async function load() {
  error.value = ''
  try {
    const c = await fetchConnections(props.characterId)
    relations.value = c.relations
    factions.value = c.factions
  } catch (e) {
    error.value = e.message
  }
}

async function openForm(rel) {
  if (!characters.value.length) characters.value = await fetchCharacterOptions()
  editing.value = rel
  formOpen.value = true
}

function onSaved() {
  formOpen.value = false
  load()
}

async function remove() {
  try {
    await relationsApi.remove(confirmTarget.value.id)
    confirmTarget.value = null
    formOpen.value = false
    load()
  } catch (e) {
    error.value = e.message
    confirmTarget.value = null
  }
}

watch(() => props.characterId, load, { immediate: true })
</script>

<style scoped>
.head-links {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
}

.relations-tab {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.section-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0.6rem;
}

.section-head h3 {
  margin: 0;
  font-family: 'Fraunces', serif;
  font-weight: 500;
  font-size: 1.05rem;
}

.empty {
  margin: 0;
  font-size: 0.85rem;
  color: var(--text-faint);
}

.rows {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
}

.row {
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
  padding: 0.5rem 0.4rem;
  border-radius: 8px;
}

.row:hover {
  background: rgba(255, 255, 255, 0.03);
}

.avatar {
  width: 2.4rem;
  height: 2.4rem;
  flex-shrink: 0;
  border-radius: 50%;
  overflow: hidden;
  border: 2px solid transparent;
  background: var(--accent-soft);
  display: flex;
  align-items: center;
  justify-content: center;
  text-decoration: none;
  color: var(--accent);
}

.row-text {
  flex: 1;
  min-width: 0;
}

.line {
  margin: 0;
  font-size: 0.92rem;
}

.line a {
  color: var(--text-primary);
  font-weight: 600;
  text-decoration: none;
}

.line a:hover {
  color: var(--accent);
}

.wording {
  color: var(--text-muted);
  margin-right: 0.3rem;
}

.meta {
  margin: 0.1rem 0 0;
  font-size: 0.75rem;
  color: var(--text-faint);
}

.notes {
  margin: 0.25rem 0 0;
  font-size: 0.82rem;
  color: var(--text-muted);
  white-space: pre-wrap;
}
</style>
