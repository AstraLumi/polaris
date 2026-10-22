<template>
  <div class="climate">
    <p class="about">{{ $t(info.about) }}</p>

    <label class="field-line">
      <span>{{ $t('When') }}</span>
      <select :value="month ?? ''" @change="$emit('update:month', $event.target.value === '' ? null : Number($event.target.value))">
        <option value="">{{ $t('The whole year') }}</option>
        <option v-for="m in months" :key="m" :value="m - 1">{{ $t('Month {n}', { n: m }) }}</option>
      </select>
    </label>

    <div class="arrows">
      <span class="small-title">{{ $t('Arrows') }}</span>
      <label class="check">
        <input type="checkbox" :checked="arrows.wind" @change="$emit('update:arrows', { ...arrows, wind: $event.target.checked })" />
        {{ $t('Winds') }}
      </label>
      <label class="check">
        <input type="checkbox" :checked="arrows.currents" @change="$emit('update:arrows', { ...arrows, currents: $event.target.checked })" />
        {{ $t('Ocean currents') }}
      </label>
    </div>

    <div class="legend">
      <template v-if="layer === 'temperature'">
        <div class="bar" :style="{ background: gradient(TEMPERATURE_STOPS) }"></div>
        <div class="ticks">
          <span v-for="t in [-40, -20, 0, 20, 38]" :key="t">{{ formatTemperature(t, unit) }}</span>
        </div>
      </template>
      <template v-else-if="layer === 'rain'">
        <div class="bar" :style="{ background: gradient(RAIN_STOPS) }"></div>
        <div class="ticks">
          <span v-for="mm in [0, 300, 1000, 2500, 4000]" :key="mm">{{ formatRain(month == null ? mm : mm / 12) }}</span>
        </div>
        <p class="note">{{ month == null ? $t('In a year') : $t('In the month') }}</p>
      </template>
      <template v-else-if="layer === 'clouds'">
        <div class="bar" style="background: linear-gradient(to right, #24324a, #f4f6fa)"></div>
        <div class="ticks"><span>{{ $t('Clear') }}</span><span>50%</span><span>{{ $t('Overcast') }}</span></div>
      </template>
      <ul v-else-if="layer === 'zones'" class="keys">
        <li v-for="k in KOPPEN_LEGEND" :key="k.name">
          <i class="chip" :style="{ background: k.color }"></i>
          <span>{{ $t(k.name) }}</span>
          <code>{{ k.codes.join(' ') }}</code>
        </li>
      </ul>
      <ul v-if="arrows.wind" class="keys">
        <li><i class="line" style="background: #d6e6f5"></i>{{ $t('Light wind') }}</li>
        <li><i class="line" style="background: #ffffff"></i>{{ $t('Steady wind') }}</li>
        <li><i class="line" style="background: #fff4c2"></i>{{ $t('Strong wind') }}</li>
      </ul>
      <ul v-if="arrows.currents" class="keys">
        <li><i class="line" style="background: #ff8a5c"></i>{{ $t('Warm current') }}</li>
        <li><i class="line" style="background: #6cc8ff"></i>{{ $t('Cold current') }}</li>
      </ul>
    </div>

    <p v-if="working" class="status">{{ $t('Working out the climate…') }}</p>
    <p v-else-if="error" class="status error">{{ $t("Couldn't work out the climate.") }} {{ error }}</p>
    <p v-if="assumed" class="note">{{ $t("Nothing in the solar system warms this planet, so the climate uses Earth's sunlight.") }}</p>
    <p class="note">
      {{ $t("Worked out from the continents, their biomes, mountains and volcanoes, the axial tilt, the day's length and the star. The year starts just after the northern winter solstice.") }}
    </p>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { layerOf } from '../planetLayers'
import { TEMPERATURE_STOPS, RAIN_STOPS, KOPPEN_LEGEND, formatTemperature, formatRain } from '../climateColors'

// What a climate layer of the planet map shows (planetClimate.js): the
// month, which arrows, and the key to its colours.
const props = defineProps({
  layer: { type: String, required: true },
  month: { type: Number, default: null }, // 0-based, or null for the year
  months: { type: Number, default: 12 },
  arrows: { type: Object, required: true }, // { wind, currents }
  working: { type: Boolean, default: false },
  error: { type: String, default: '' },
  assumed: { type: Boolean, default: false },
  unit: { type: String, default: 'C' },
})
defineEmits(['update:month', 'update:arrows'])

const info = computed(() => layerOf(props.layer))

function gradient(stops) {
  const lo = stops[0][0]
  const hi = stops[stops.length - 1][0]
  return `linear-gradient(to right, ${stops.map(([v, c]) => `${c} ${(((v - lo) / (hi - lo)) * 100).toFixed(1)}%`).join(', ')})`
}
</script>

<style scoped>
.climate {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  font-size: 0.76rem;
}

.about {
  margin: 0;
  color: var(--text-muted);
}

.field-line {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  color: var(--text-muted);
}

.field-line select {
  min-height: 1.8rem;
  padding: 0.1rem 0.3rem;
  font-size: 0.74rem;
}

.small-title {
  font-size: 0.68rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-faint);
}

.arrows {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.3rem 0.8rem;
}

.check {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  color: var(--text-muted);
  cursor: pointer;
}

.legend {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.bar {
  height: 0.7rem;
  border-radius: 4px;
  box-shadow: inset 0 0 0 1px rgba(0, 0, 0, 0.25);
}

.ticks {
  display: flex;
  justify-content: space-between;
  font-size: 0.66rem;
  color: var(--text-faint);
}

.keys {
  margin: 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
}

.keys li {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  color: var(--text-muted);
}

.keys code {
  margin-left: auto;
  font-size: 0.64rem;
  color: var(--text-faint);
}

.chip {
  flex: none;
  width: 0.8rem;
  height: 0.8rem;
  border-radius: 3px;
}

.line {
  flex: none;
  width: 1.2rem;
  height: 0.25rem;
  border-radius: 2px;
  box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.45);
}

.status {
  margin: 0;
  color: var(--accent);
}

.status.error {
  color: var(--danger, #e06060);
}

.note {
  margin: 0;
  font-size: 0.68rem;
  color: var(--text-faint);
}
</style>
