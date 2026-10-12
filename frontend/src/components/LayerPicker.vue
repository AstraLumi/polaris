<template>
  <div class="layer-picker">
    <span class="picker-title">{{ $t('Layer') }}</span>
    <div class="layers" role="radiogroup" :aria-label="$t('Layer')">
      <button
        v-for="l in LAYERS"
        :key="l.id"
        type="button"
        role="radio"
        class="layer-btn"
        :class="{ active: modelValue === l.id }"
        :aria-checked="modelValue === l.id"
        :disabled="l.later"
        :title="l.later ? $t('Coming later') : $t(l.about)"
        @click="$emit('update:modelValue', l.id)"
      >
        {{ $t(l.label) }}<span v-if="l.later" class="later">{{ $t('later') }}</span>
      </button>
    </div>
  </div>
</template>

<script setup>
import { LAYERS } from '../planetLayers'

// The planet map's layers (planetLayers.js): the map itself, or one drawn
// over it while the map shows faintly underneath.
defineProps({ modelValue: { type: String, default: 'map' } })
defineEmits(['update:modelValue'])
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

.layer-btn:disabled {
  opacity: 0.45;
  cursor: default;
}

.later {
  margin-left: 0.3rem;
  font-size: 0.62rem;
  color: var(--text-faint);
}
</style>
