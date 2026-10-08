<template>
  <div class="field people-field">
    <span>{{ $t(label) }}</span>
    <ul v-if="selected.length" class="people-list">
      <li v-for="p in selected" :key="p.id" class="person">
        <span class="avatar">
          <img v-if="p.picture_path" :src="p.picture_path" :alt="p.name" />
          <template v-else>{{ initials(p.name) }}</template>
        </span>
        <span class="person-name">{{ p.name }}</span>
        <button type="button" class="chip-x" :aria-label="$t('Remove {name}', { name: p.name })" @click="remove(p.id)">×</button>
      </li>
    </ul>
    <select :value="''" :disabled="!available.length" @change="onPick">
      <option value="">
        {{ available.length ? $t('Add a character…') : options.length ? $t('Everyone is already added') : $t('No characters yet') }}
      </option>
      <option v-for="o in available" :key="o.id" :value="o.id">{{ o.name }}</option>
    </select>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { tr } from '../i18n'

// Picks characters (by id) from the character list.
const props = defineProps({
  modelValue: { type: Array, default: () => [] }, // character ids
  options: { type: Array, default: () => [] }, // [{ id, name, picture_path }]
  label: { type: String, default: tr('Involved people') },
})
const emit = defineEmits(['update:modelValue'])

const selected = computed(() =>
  props.modelValue.map((id) => props.options.find((o) => o.id === id)).filter(Boolean),
)
const available = computed(() => props.options.filter((o) => !props.modelValue.includes(o.id)))

function onPick(e) {
  const id = Number(e.target.value)
  e.target.value = ''
  if (id) emit('update:modelValue', [...props.modelValue, id])
}

function remove(id) {
  emit('update:modelValue', props.modelValue.filter((x) => x !== id))
}

function initials(name) {
  return (name || '').split(' ').filter(Boolean).map((p) => p[0]).join('').slice(0, 2).toUpperCase()
}
</script>

<style scoped>
.people-list {
  list-style: none;
  margin: 0 0 0.4rem;
  padding: 0;
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
}

.person {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  border: 1px solid var(--glass-border);
  background: rgba(255, 255, 255, 0.04);
  border-radius: 999px;
  padding: 0.2rem 0.4rem 0.2rem 0.25rem;
  font-size: 0.82rem;
  color: var(--text-primary);
}

.avatar {
  width: 24px;
  height: 24px;
  border-radius: 50%;
  overflow: hidden;
  display: grid;
  place-items: center;
  background: var(--accent-soft);
  color: var(--accent);
  font-size: 0.6rem;
  font-weight: 700;
}

.avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.chip-x {
  background: transparent;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 1rem;
  line-height: 1;
  padding: 0 0.15rem;
}

.chip-x:hover {
  color: var(--text-primary);
}
</style>
