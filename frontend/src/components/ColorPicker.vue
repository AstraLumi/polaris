<template>
  <div class="color-picker">
    <div class="swatches">
      <button
        type="button"
        class="swatch swatch-none"
        :class="{ active: !modelValue }"
        :title="$t('No color: painting with this location won\'t change a hex\'s kingdom')"
        @click="$emit('update:modelValue', '')"
      >
        <span class="slash"></span>
      </button>
      <button
        v-for="c in MAP_PALETTE"
        :key="c"
        type="button"
        class="swatch"
        :class="{ active: modelValue === c, taken: isTaken(c) }"
        :style="{ background: c }"
        :disabled="isTaken(c)"
        :title="isTaken(c) ? $t('Already used by another kingdom') : c"
        @click="$emit('update:modelValue', c)"
      ></button>
    </div>
  </div>
</template>

<script setup>
import { MAP_PALETTE } from '../mapPalette'

const props = defineProps({
  modelValue: { type: String, default: '' }, // '#rrggbb', or '' for no color
  takenColors: { type: Array, default: () => [] },
})
defineEmits(['update:modelValue'])

const isTaken = (c) => props.takenColors.some((t) => t && t.toLowerCase() === c)
</script>

<style scoped>
.swatches {
  display: grid;
  grid-template-columns: repeat(14, 1fr);
  gap: 5px;
}

.swatch {
  position: relative;
  aspect-ratio: 1;
  min-width: 0;
  border-radius: 6px;
  border: 2px solid transparent;
  cursor: pointer;
  padding: 0;
}

.swatch:hover:not(:disabled) {
  transform: scale(1.12);
}

.swatch.active {
  border-color: var(--text-primary);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--surface) 90%, transparent) inset;
}

.swatch.taken {
  opacity: 0.22;
  cursor: not-allowed;
}

.swatch-none {
  background: rgba(255, 255, 255, 0.05);
  border: 2px dashed var(--glass-border);
}

.swatch-none.active {
  border-style: solid;
  border-color: var(--text-primary);
}

.slash {
  position: absolute;
  left: 18%;
  right: 18%;
  top: 50%;
  height: 2px;
  background: var(--text-faint);
  transform: rotate(-45deg);
}
</style>
