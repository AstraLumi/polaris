<template>
  <div class="overlay" @click.self="$emit('close')">
    <div class="modal glass-panel is-wide">
      <h2>{{ $t('Equip {slot}', { slot: $t(slotDef.label) }) }}</h2>
      <p class="picker-sub">{{ $t('Showing {type} gear.', { type: typeLabel }) }}</p>

      <input v-if="options.length > 6" v-model="query" class="picker-search" type="search" :placeholder="$t('Search gear')" />

      <p v-if="loading" class="picker-note">{{ $t('Loading…') }}</p>
      <p v-else-if="error" class="error-banner">{{ error }}</p>
      <div v-else-if="options.length === 0" class="picker-empty">
        <p class="picker-note">{{ $t('No {type} gear yet.', { type: typeLabel }) }}</p>
        <p class="picker-hint">{{ $t('Create some on the Character Assets page, under Gear.') }}</p>
      </div>
      <ul v-else class="picker-list">
        <li v-for="g in shown" :key="g.id">
          <button
            type="button"
            class="picker-item"
            :class="{ 'is-current': current && current.id === g.id }"
            @click="$emit('pick', g)"
          >
            <GearRow :gear="g" />
          </button>
        </li>
        <li v-if="!shown.length" class="picker-note">{{ $t('Nothing matches that search.') }}</li>
      </ul>

      <div class="modal-actions">
        <button v-if="current" type="button" class="btn btn-ghost" @click="$emit('pick', null)">{{ $t('Unequip') }}</button>
        <button type="button" class="btn btn-ghost" @click="$emit('close')">{{ $t('Cancel') }}</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { t } from '../i18n'
import { gearApi } from '../api'
import { slotTypeLabel } from '../gear'
import GearRow from './GearRow.vue'

const props = defineProps({
  slotDef: { type: Object, required: true }, // an EQUIP_SLOTS entry
  current: { type: Object, default: null },
})
defineEmits(['pick', 'close'])

const all = ref([])
const loading = ref(true)
const error = ref('')
const query = ref('')

const typeLabel = computed(() => slotTypeLabel(props.slotDef.type))
const options = computed(() => all.value.filter((g) => g.slot === props.slotDef.type))
const shown = computed(() => {
  const q = query.value.trim().toLowerCase()
  return q ? options.value.filter((g) => g.name.toLowerCase().includes(q)) : options.value
})

onMounted(async () => {
  try {
    all.value = await gearApi.list()
  } catch (err) {
    error.value = t("Couldn't load gear. Try again.")
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.picker-sub { margin: -0.4rem 0 1rem; font-size: 0.85rem; color: var(--text-muted); }
.picker-search { width: 100%; margin-bottom: 0.8rem; }
.picker-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 0.5rem; max-height: 50vh; overflow-y: auto; }
.picker-item { display: block; width: 100%; padding: 0; background: none; border: 0; text-align: left; font: inherit; color: inherit; cursor: pointer; border-radius: 14px; }
.picker-item:hover :deep(.gear-row), .picker-item:focus-visible :deep(.gear-row) { border-color: var(--accent); }
.picker-item.is-current :deep(.gear-row) { border-color: var(--accent); background: var(--accent-soft); }
.picker-note { color: var(--text-muted); font-size: 0.9rem; }
.picker-hint { color: var(--text-faint); font-size: 0.8rem; margin-top: 0.2rem; }
.picker-empty { padding: 1.2rem 0; text-align: center; }
</style>
