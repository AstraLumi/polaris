<template>
  <div class="spells-tab">
    <p v-if="!modelValue.length" class="empty">
      {{ editable ? $t('No spells yet — pick some below.') : $t('No spells yet.') }}
    </p>

    <div v-else class="spell-list">
      <SpellRow v-for="s in sortedAssigned" :key="s.id" :spell="s">
        <template v-if="editable" #actions>
          <button type="button" class="btn btn-ghost small" @click="remove(s)">{{ $t('Remove') }}</button>
        </template>
      </SpellRow>
    </div>

    <div v-if="editable" class="picker">
      <h4>{{ $t('Add a spell') }}</h4>
      <input
        v-model="search"
        type="text"
        :placeholder="$t('Search by name, source, or origin')"
        class="picker-search"
      />

      <p v-if="loadError" class="error-banner">{{ loadError }}</p>
      <p v-else-if="allSpells.length === 0 && !loading" class="hint">
        {{ $t('No spells exist yet — create some on the Character Assets page first.') }}
      </p>
      <p v-else-if="available.length === 0 && !loading" class="hint">
        {{ search ? $t('No spells match that search.') : $t('Every spell is already on this character.') }}
      </p>

      <div class="picker-list">
        <SpellRow v-for="s in available" :key="s.id" :spell="s">
          <template #actions>
            <button type="button" class="btn btn-primary small" @click="add(s)">{{ $t('Add') }}</button>
          </template>
        </SpellRow>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { t } from '../i18n'
import { spellsApi } from '../api'
import { formatSpellSource } from '../spellUtils'
import SpellRow from './SpellRow.vue'

const props = defineProps({
  modelValue: { type: Array, default: () => [] }, // spells this version knows
  editable: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue'])

const allSpells = ref([])
const loading = ref(false)
const loadError = ref('')
const search = ref('')

const byLevelThenName = (a, b) => a.level - b.level || a.name.localeCompare(b.name)

const sortedAssigned = computed(() => [...props.modelValue].sort(byLevelThenName))

const available = computed(() => {
  const have = new Set(props.modelValue.map((s) => s.id))
  const q = search.value.trim().toLowerCase()
  return allSpells.value
    .filter((s) => !have.has(s.id))
    .filter((s) => {
      if (!q) return true
      const haystack = [s.name, formatSpellSource(s), s.origin].join(' ').toLowerCase()
      return haystack.includes(q)
    })
})

function add(spell) {
  emit('update:modelValue', [...props.modelValue, spell])
}

function remove(spell) {
  emit('update:modelValue', props.modelValue.filter((s) => s.id !== spell.id))
}

onMounted(async () => {
  if (!props.editable) return
  loading.value = true
  try {
    allSpells.value = await spellsApi.list()
  } catch (err) {
    loadError.value = t("Couldn't load the spell list. Try refreshing.")
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.empty {
  margin: 0 0 1rem;
  color: var(--text-faint);
  font-size: 0.9rem;
}

.spell-list,
.picker-list {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
}

.picker {
  margin-top: 1.75rem;
  padding-top: 1.5rem;
  border-top: 1px solid var(--glass-border);
}

.picker h4 {
  font-family: 'Fraunces', serif;
  font-weight: 500;
  font-size: 0.95rem;
  margin: 0 0 0.75rem;
  color: var(--text-primary);
}

.picker-search {
  margin-bottom: 0.9rem;
}

.picker-list {
  max-height: 420px;
  overflow-y: auto;
  padding-right: 0.25rem;
}

.hint {
  margin: 0 0 0.9rem;
  font-size: 0.82rem;
  color: var(--text-faint);
}
</style>
