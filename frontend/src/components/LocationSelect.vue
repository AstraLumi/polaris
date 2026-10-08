<template>
  <label class="field">
    <span>{{ label }}</span>
    <select :value="modelValue ?? ''" @change="onChange">
      <option value="">{{ $t('— None —') }}</option>
      <optgroup v-for="g in groups" :key="g.label" :label="g.label">
        <option v-for="o in g.items" :key="o.id" :value="o.id">{{ o.name }}</option>
      </optgroup>
    </select>
    <small v-if="legacyText && modelValue == null" class="legacy-note">
      {{ $t('Previously typed “{text}” — it doesn\'t match anything on the Map. Pick a location to replace it.', { text: legacyText }) }}
    </small>
    <small v-else-if="!groups.length" class="legacy-note">
      {{ kingdomOnly ? $t('No kingdoms on the Map yet.') : $t('No locations on the Map yet.') }}
    </small>
  </label>
</template>

<script setup>
import { t } from '../i18n'
import { computed } from 'vue'

// Picks a Map location by id. kingdomOnly limits the list to kingdoms
// (colored major locations) — that's what a character's Nation is.
const props = defineProps({
  modelValue: { type: Number, default: null },
  label: { type: String, required: true },
  options: { type: Array, default: () => [] }, // [{ id, name, kind, color }]
  kingdomOnly: { type: Boolean, default: false },
  legacyText: { type: String, default: '' },
})
const emit = defineEmits(['update:modelValue'])

const groups = computed(() => {
  const kingdoms = props.options.filter((o) => o.kind === 'major' && o.color)
  if (props.kingdomOnly) return kingdoms.length ? [{ label: t('Kingdoms'), items: kingdoms }] : []
  const regions = props.options.filter((o) => o.kind === 'major' && !o.color)
  const places = props.options.filter((o) => o.kind === 'minor')
  return [
    { label: t('Kingdoms'), items: kingdoms },
    { label: t('Regions'), items: regions },
    { label: t('Locations'), items: places },
  ].filter((g) => g.items.length)
})

function onChange(e) {
  const v = e.target.value
  emit('update:modelValue', v === '' ? null : Number(v))
}
</script>

<style scoped>
.legacy-note {
  color: var(--accent);
  font-size: 0.74rem;
  line-height: 1.4;
}
</style>
