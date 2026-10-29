<template>
  <div ref="root" class="picture-options glass-panel panel-solid" role="group" :aria-label="$t('Picture options')" :style="place" @mousedown.stop>
    <div class="row">
      <label class="width" :class="{ 'is-off': attrs.fullPage }">
        <span>{{ $t('Width (pt)') }}</span>
        <input
          type="number"
          min="10"
          max="2000"
          step="1"
          :value="attrs.width ?? ''"
          :placeholder="$t('Natural')"
          :disabled="attrs.fullPage"
          :aria-label="$t('Width (pt)')"
          @change="setWidth($event.target.value)"
          @keydown.enter.prevent="$event.target.blur()"
        />
      </label>
      <button type="button" class="btn btn-ghost small" :disabled="attrs.fullPage || attrs.width == null" @mousedown.prevent @click="run({ width: null })">{{ $t('Natural size') }}</button>
      <div class="aligns" role="group" :aria-label="$t('Alignment')">
        <button v-for="a in ALIGNS" :key="a.key" type="button" class="align-btn" :class="{ 'is-active': (attrs.align || 'center') === a.key }" :title="a.label" :aria-label="a.label" :aria-pressed="(attrs.align || 'center') === a.key" @mousedown.prevent @click="run({ align: a.key })">
          <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" aria-hidden="true"><path :d="a.icon" /></svg>
        </button>
      </div>
    </div>
    <div class="row wraps" role="group" :aria-label="$t('Text wrapping')">
      <span>{{ $t('Text wrapping') }}</span>
      <button v-for="w in WRAPS" :key="w.key" type="button" class="btn btn-ghost small" :class="{ 'is-active': (attrs.wrap || 'inline') === w.key }" :disabled="attrs.fullPage" :aria-pressed="(attrs.wrap || 'inline') === w.key" @mousedown.prevent @click="run({ wrap: w.key })">{{ w.label }}</button>
    </div>
    <label class="check" :title="$t('Alone on its own page, like a cover (ignored in Online Post)')">
      <input type="checkbox" :checked="!!attrs.fullPage" @change="run({ fullPage: $event.target.checked })" />
      <span>{{ $t('Full page') }}</span>
    </label>
    <label class="alt">
      <span>{{ $t('Alt text') }}</span>
      <input type="text" maxlength="500" :value="attrs.alt || ''" :placeholder="$t('Describe the picture')" :aria-label="$t('Alt text')" @change="run({ alt: $event.target.value.trim() })" @keydown.enter.prevent="$event.target.blur()" />
    </label>
    <div class="row end">
      <button type="button" class="btn btn-danger small" @mousedown.prevent @click="$emit('remove')">{{ $t('Remove picture') }}</button>
    </div>
  </div>
</template>

<script setup>
// What can be set on the selected picture: width, alignment, full page,
// alt text, and taking it out. It sits by the picture; `attrs` are the
// picture's, `change` is told what to set (the page runs the command).
import { ref, computed } from 'vue'
import { t } from '../../i18n'

const props = defineProps({
  attrs: { type: Object, required: true },
  top: { type: Number, default: 0 },
  left: { type: Number, default: 0 },
})
const emit = defineEmits(['change', 'remove'])

const root = ref(null)
defineExpose({ root })

const WRAPS = computed(() => [
  { key: 'inline', label: t('In line') },
  { key: 'left', label: t('Text on the right') },
  { key: 'right', label: t('Text on the left') },
])

const ALIGNS = computed(() => [
  { key: 'left', label: t('Align left'), icon: 'M3 6h18M3 10h12M3 14h18M3 18h12' },
  { key: 'center', label: t('Centre'), icon: 'M3 6h18M6 10h12M3 14h18M6 18h12' },
  { key: 'right', label: t('Align right'), icon: 'M3 6h18M9 10h12M3 14h18M9 18h12' },
])

const place = computed(() => ({ top: `${props.top}px`, left: `${props.left}px` }))

const run = (patch) => emit('change', patch)

// An empty or unreadable box goes back to the natural size.
function setWidth(value) {
  const n = +value
  run({ width: value === '' || !Number.isFinite(n) || n < 10 ? null : Math.min(2000, Math.round(n * 10) / 10) })
}
</script>

<style scoped>
.picture-options {
  position: absolute;
  z-index: 20;
  display: grid;
  gap: 0.5rem;
  width: min(23rem, calc(100% - 1rem));
  padding: 0.6rem 0.7rem;
  border-radius: 12px;
  font-size: 0.82rem;
}

.row {
  display: flex;
  align-items: flex-end;
  flex-wrap: wrap;
  gap: 0.4rem;
}

.row.end {
  justify-content: flex-end;
}

.width {
  display: grid;
  gap: 0.15rem;
  color: var(--text-muted);
  font-size: 0.74rem;
}

.width.is-off {
  opacity: 0.5;
}

.width input,
.alt input[type='text'] {
  min-height: 2rem;
  height: 2rem;
  padding: 0.15rem 0.45rem;
  font-size: 0.82rem;
}

.width input {
  width: 6rem;
}

.aligns {
  display: flex;
  gap: 0.1rem;
  margin-left: auto;
}

.align-btn {
  display: inline-grid;
  place-items: center;
  width: 2rem;
  height: 2rem;
  border: 1px solid transparent;
  border-radius: 7px;
  background: none;
  color: var(--text-primary);
  cursor: pointer;
}

.align-btn:hover {
  background: var(--glass);
  border-color: var(--glass-border);
}

.wraps {
  align-items: center;
  color: var(--text-muted);
  font-size: 0.8rem;
}

.wraps .is-active,
.align-btn.is-active {
  background: var(--accent-soft);
  border-color: color-mix(in srgb, var(--accent) 55%, transparent);
  color: var(--accent);
}

.check {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  color: var(--text-primary);
}

.alt {
  display: grid;
  gap: 0.15rem;
  color: var(--text-muted);
  font-size: 0.74rem;
}
</style>
