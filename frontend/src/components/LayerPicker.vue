<template>
  <div class="layer-picker">
    <template v-for="g in groups" :key="g.title">
      <span class="picker-title">{{ $t(g.title) }}</span>
      <div class="layers" role="radiogroup" :aria-label="$t(g.title)">
        <button
          v-for="l in g.layers"
          :key="l.id"
          type="button"
          role="radio"
          class="layer-btn"
          :class="{ active: modelValue === l.id }"
          :aria-checked="modelValue === l.id"
          :title="$t(l.about)"
          @click="$emit('update:modelValue', l.id)"
        >
          {{ $t(l.label) }}
        </button>
      </div>
    </template>
  </div>
</template>

<script setup>
import { tr } from '../i18n'
import { LAYERS } from '../planetLayers'

// The planet map's layers (planetLayers.js): the map itself, or one drawn
// over it while the map shows faintly underneath; then the climate worked
// out from them.
defineProps({ modelValue: { type: String, default: 'map' } })
defineEmits(['update:modelValue'])

const groups = [
  { title: tr('Layer'), layers: LAYERS.filter((l) => !l.climate) },
  { title: tr('Climate'), layers: LAYERS.filter((l) => l.climate) },
]
</script>

<style scoped>
.layer-picker {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
}

.picker-title {
  font-size: 0.68rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-faint);
}

.layers {
  display: flex;
  flex-wrap: wrap;
  gap: 0.3rem;
}

.layer-btn {
  padding: 0.25rem 0.55rem;
  border: 1px solid var(--glass-border);
  border-radius: 999px;
  background: transparent;
  color: var(--text-muted);
  font: inherit;
  font-size: 0.74rem;
  cursor: pointer;
}

.layer-btn.active {
  color: var(--text-primary);
  border-color: var(--accent);
  background: var(--accent-soft);
}
</style>
