<template>
  <div class="build-panel">
    <div class="stats-header">
      <h3>{{ $t('Primary Stats') }}</h3>
      <span v-if="editable" class="points-badge">{{ $t('Points available: {n}', { n: pointsAvailable }) }}</span>
    </div>

    <div v-if="primaryStats.length" class="stats-grid" :style="{ '--stat-cols': Math.min(primaryStats.length, 8) }">
      <div v-for="stat in primaryStats" :key="stat.key" class="stat-box">
        <span class="stat-label" :title="statName(stat)">{{ statShort(stat) }}</span>
        <span class="stat-value">{{ points(stat.key) }}</span>
        <span v-if="effectiveDelta(stat.key) !== 0" class="stat-effective" :title="$t('With bonuses from class, race, body type and gear')">
          → {{ Math.round(final(stat.key)) }}
        </span>
        <div v-if="editable" class="stat-buttons">
          <button class="stat-btn" :disabled="points(stat.key) < 10" @click="adjust(stat.key, -10)">-10</button>
          <button class="stat-btn" :disabled="points(stat.key) < 1" @click="adjust(stat.key, -1)">-1</button>
          <button class="stat-btn" :disabled="pointsAvailable < 1" @click="adjust(stat.key, 1)">+1</button>
          <button class="stat-btn" :disabled="pointsAvailable < 10" @click="adjust(stat.key, 10)">+10</button>
        </div>
      </div>
    </div>

    <div class="derived-grid">
      <div v-for="(stack, i) in stacks" :key="i" class="derived-col-stack">
        <div v-for="group in stack" :key="group.id" class="derived-col glass-panel">
          <h4>{{ $t(group.label) }}</h4>
          <div
            v-for="stat in group.stats"
            :key="stat.key"
            class="derived-row"
            :class="{ 'is-editable': editable && isTyped(stat) }"
          >
            <span>{{ statName(stat) }}</span>
            <input
              v-if="editable && isTyped(stat)"
              class="inline-number"
              type="number"
              :step="stepOf(stat)"
              :value="typedValue(stat)"
              :aria-label="statName(stat)"
              @input="updateBase(stat, $event.target.value)"
            />
            <span>{{ formatStat(stat, final(stat.key)) }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { STAT_CATEGORIES, statSystem, statsIn, statName, statShort, isTyped, formatStat } from '../stats'

// The Build tab: the story's stats (Settings → Stats) for one version.
const props = defineProps({
  modelValue: { type: Object, required: true }, // points per primary stat
  computed: { type: Object, required: true }, // every stat's final value, by key
  specialBases: { type: Object, default: () => ({}) }, // typed values of calculated stats, by key
  points: { type: Number, default: 0 }, // points the version has to spend
  editable: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue', 'update:specialBases'])

const primaryStats = computed(() => statsIn('primary', statSystem.stats))

// Base and Special Stats on the left, Special Defenses and Lifeskills on the
// right; a category with no stats is left out.
const stacks = computed(() => {
  const group = (id) => ({ ...STAT_CATEGORIES.find((c) => c.id === id), stats: statsIn(id, statSystem.stats) })
  return [
    [group('base'), group('special')],
    [group('defense'), group('lifeskill')],
  ]
    .map((stack) => stack.filter((g) => g.stats.length))
    .filter((stack) => stack.length)
})

const points = (key) => props.modelValue[key] || 0
const final = (key) => props.computed?.[key] ?? 0

const pointsAvailable = computed(() => props.points - primaryStats.value.reduce((sum, s) => sum + points(s.key), 0))

function adjust(key, delta) {
  emit('update:modelValue', { ...props.modelValue, [key]: Math.max(0, points(key) + delta) })
}

function effectiveDelta(key) {
  return props.computed?.[key] === undefined ? 0 : Math.round(final(key)) - points(key)
}

// Whole numbers for stats shown without decimals, else as fine as shown.
const stepOf = (stat) => (stat.decimals ? String(10 ** -stat.decimals) : '1')
const typedValue = (stat) => {
  const v = props.specialBases[stat.key] ?? 0
  return stat.decimals ? v : Math.round(v)
}

function updateBase(stat, raw) {
  const num = parseFloat(raw)
  const safe = Number.isNaN(num) ? 0 : num
  emit('update:specialBases', { ...props.specialBases, [stat.key]: stat.decimals ? safe : Math.round(safe) })
}
</script>

<style scoped>
.stats-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
}

.stats-header h3 {
  font-family: 'Fraunces', serif;
  font-weight: 500;
  font-size: 1.1rem;
  margin: 0;
  color: var(--text-primary);
}

.points-badge {
  font-size: 0.78rem;
  font-weight: 600;
  color: var(--accent);
  background: var(--accent-soft);
  border: 1px solid color-mix(in srgb, var(--accent) 30%, transparent);
  border-radius: 999px;
  padding: 0.3rem 0.8rem;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(var(--stat-cols, 8), minmax(64px, 1fr));
  gap: 0.5rem;
  margin-bottom: 1.75rem;
  overflow-x: auto;
}

.stat-box {
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid var(--glass-border);
  border-radius: 10px;
  padding: 0.55rem 0.35rem;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.25rem;
  min-width: 0;
}

.stat-label {
  font-size: 0.68rem;
  font-weight: 700;
  letter-spacing: 0.03em;
  color: var(--text-faint);
}

.stat-value {
  font-family: 'Fraunces', serif;
  font-size: 1.1rem;
  color: var(--text-primary);
}

.stat-effective {
  font-size: 0.65rem;
  color: var(--accent);
  font-weight: 600;
  margin-top: -0.15rem;
}

.stat-buttons {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 0.2rem;
  margin-top: 0.1rem;
  width: 100%;
}

.stat-btn {
  background: transparent;
  border: 1px solid var(--glass-border);
  border-radius: 5px;
  color: var(--text-muted);
  font-size: 0.62rem;
  line-height: 1.4;
  padding: 0.1rem 0;
  cursor: pointer;
}

.stat-btn:disabled {
  opacity: 0.3;
  cursor: not-allowed;
}

.stat-btn:not(:disabled):hover {
  border-color: var(--accent);
  color: var(--accent);
}

.derived-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 1rem;
  align-items: start;
}

.derived-col-stack {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.derived-col {
  padding: 1.1rem 1.25rem;
}

.derived-col h4 {
  font-family: 'Fraunces', serif;
  font-weight: 500;
  font-size: 0.95rem;
  margin: 0 0 0.75rem;
  color: var(--text-primary);
}

.derived-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.4rem 0;
  border-top: 1px solid var(--glass-border);
  font-size: 0.85rem;
  color: var(--text-muted);
  gap: 0.6rem;
}

.derived-row:first-of-type {
  border-top: none;
}

.derived-row span:first-child {
  flex: 1;
  min-width: 0;
}

.derived-row span:last-child {
  color: var(--text-primary);
  font-weight: 600;
  white-space: nowrap;
}

.inline-number {
  width: 72px;
  flex-shrink: 0;
  text-align: right;
  padding: 0.25rem 0.4rem;
  font-size: 0.85rem;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--glass-border);
  border-radius: 6px;
  color: var(--text-primary);
  font-family: 'Manrope', sans-serif;
}
</style>
