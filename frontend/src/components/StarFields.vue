<template>
  <div class="star-fields">
    <label class="field">
      <span>{{ $t('Class') }}</span>
      <select :value="modelValue.star_class || 'yellow_dwarf'" @change="setClass($event.target.value)">
        <option v-for="(c, id) in STAR_CLASSES" :key="id" :value="id">{{ $t(CLASS_LABELS[id]) }}</option>
      </select>
    </label>

    <div v-for="row in rows" :key="row.key" class="value-row">
      <div class="value-head">
        <span class="value-label">{{ $t(row.label) }}</span>
        <label class="real-tick" :title="$t('Keep it within what is real for this class')">
          <input type="checkbox" :checked="!row.free" @change="setFree(row.key, !$event.target.checked)" />
          <span>{{ $t('Reality Following') }}</span>
        </label>
      </div>

      <!-- Free: any number. -->
      <div v-if="row.free" class="free-input">
        <input
          type="number"
          step="any"
          :min="row.key === 'temperature_k' ? 0 : 0.000001"
          :value="row.toUnit(modelValue[row.key])"
          @input="setValue(row.key, row.fromUnit(Number($event.target.value)))"
        />
        <span class="unit">{{ row.unit }}</span>
      </div>
      <!-- Real: a slider over the class's range, or nothing to set. -->
      <p v-else-if="row.fixed" class="fixed-note">{{ row.fixed }}</p>
      <div v-else class="slider">
        <input
          type="range"
          min="0"
          max="1000"
          step="1"
          :value="toSlider(row.range, modelValue[row.key])"
          @input="setValue(row.key, fromSlider(row.range, Number($event.target.value)))"
        />
        <div class="slider-ends">
          <span>{{ row.show(row.range[0]) }}</span>
          <strong>{{ row.show(modelValue[row.key]) }}</strong>
          <span>{{ row.show(row.range[1]) }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { t, tr } from '../i18n'
import { STAR_CLASSES, STAR_VALUES, SUN, classOf, isFree, fitToClass, schwarzschildRadius } from '../stars'

// A star's class, temperature, size and mass. Each value follows reality by
// default: a slider over the range real stars of the class have (on a log
// scale, as these ranges span orders of magnitude). Unticking "Reality
// Following" lets any number be typed. Edits the star given as v-model.
const props = defineProps({ modelValue: { type: Object, required: true } })
const emit = defineEmits(['update:modelValue'])

const CLASS_LABELS = {
  yellow_dwarf: tr('Yellow dwarf (like the Sun)'),
  red_giant: tr('Red giant'),
  white_dwarf: tr('White dwarf'),
  brown_dwarf: tr('Brown dwarf'),
  neutron_star: tr('Neutron star'),
  black_hole: tr('Black hole'),
}
const num = (x) => (x !== 0 && Math.abs(x) < 0.001 ? Number(x).toExponential(2) : Number(Number(x).toPrecision(3)).toLocaleString())
const UNITS = {
  temperature_k: {
    label: tr('Temperature'),
    unit: 'K',
    toUnit: (x) => x,
    fromUnit: (x) => x,
    show: (x) => `${num(x)} K`,
  },
  radius_km: {
    label: tr('Radius'),
    unit: 'km',
    toUnit: (x) => x,
    fromUnit: (x) => x,
    show: (x) => (x >= SUN.radius_km * 0.1 ? t('{n} Suns', { n: num(x / SUN.radius_km) }) : `${num(x)} km`),
  },
  mass_kg: {
    label: tr('Mass'),
    unit: '× ' + t('Sun'),
    toUnit: (x) => Number((x / SUN.mass_kg).toPrecision(6)),
    fromUnit: (x) => x * SUN.mass_kg,
    show: (x) => t('{n} Suns', { n: num(x / SUN.mass_kg) }),
  },
}

const rows = computed(() => {
  const star = props.modelValue
  const cls = classOf(star)
  return STAR_VALUES.map(({ key }) => {
    const free = isFree(star, key)
    let fixed = ''
    if (!cls[key]) {
      fixed = key === 'temperature_k'
        ? t('None: a black hole gives no light of its own.')
        : t('Its event horizon, which follows its mass: {n} km.', { n: num(schwarzschildRadius(star.mass_kg)) })
    }
    return { key, free, fixed, range: cls[key], ...UNITS[key] }
  })
})

// Log-scale slider: 0..1000 across [min, max].
const toSlider = (range, v) => Math.round((1000 * Math.log(Math.max(v, range[0]) / range[0])) / Math.log(range[1] / range[0]))
const fromSlider = (range, x) => Number((range[0] * (range[1] / range[0]) ** (x / 1000)).toPrecision(4))

function update(next) {
  emit('update:modelValue', fitToClass(next))
}

function setClass(id) {
  // A new class starts from a typical star of it, except for free values.
  const star = { ...props.modelValue, star_class: id }
  for (const { key } of STAR_VALUES) if (!isFree(star, key)) star[key] = NaN
  update(star)
}

function setFree(key, free) {
  const bit = STAR_VALUES.find((v) => v.key === key).bit
  const flags = props.modelValue.free_values || 0
  update({ ...props.modelValue, free_values: free ? flags | bit : flags & ~bit })
}

function setValue(key, v) {
  if (!Number.isFinite(v)) return
  update({ ...props.modelValue, [key]: v })
}
</script>

<style scoped>
.star-fields {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.value-row {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  padding: 0.45rem 0;
  border-top: 1px solid var(--glass-border);
}

.value-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.value-label {
  font-size: 0.8rem;
  color: var(--text-muted);
}

.real-tick {
  display: flex;
  align-items: center;
  gap: 0.3rem;
  font-size: 0.72rem;
  color: var(--text-faint);
  cursor: pointer;
}

.free-input {
  display: flex;
  align-items: center;
  gap: 0.4rem;
}

.free-input input {
  flex: 1;
  min-width: 0;
}

.unit {
  font-size: 0.75rem;
  color: var(--text-faint);
  white-space: nowrap;
}

.slider input {
  width: 100%;
  accent-color: var(--accent);
}

.slider-ends {
  display: flex;
  justify-content: space-between;
  gap: 0.4rem;
  font-size: 0.68rem;
  color: var(--text-faint);
  font-variant-numeric: tabular-nums;
}

.slider-ends strong {
  color: var(--text-primary);
  font-size: 0.76rem;
}

.fixed-note {
  margin: 0;
  font-size: 0.74rem;
  line-height: 1.4;
  color: var(--text-faint);
}
</style>
