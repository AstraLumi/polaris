<template>
  <div class="build-panel">
    <div class="stats-header">
      <h3>{{ $t('Primary Stats') }}</h3>
      <span v-if="editable" class="points-badge">{{ $t('Points available: {n}', { n: pointsAvailable }) }}</span>
    </div>

    <div class="stats-grid">
      <div v-for="stat in statList" :key="stat.key" class="stat-box">
        <span class="stat-label">{{ stat.label }}</span>
        <span class="stat-value">{{ modelValue[stat.key] }}</span>
        <span v-if="effectiveDelta(stat.key) !== 0" class="stat-effective">
          → {{ Math.round(primary[stat.key]) }}
        </span>
        <div v-if="editable" class="stat-buttons">
          <button
            class="stat-btn"
            :disabled="modelValue[stat.key] < 10"
            @click="adjust(stat.key, -10)"
          >
            -10
          </button>
          <button
            class="stat-btn"
            :disabled="modelValue[stat.key] < 1"
            @click="adjust(stat.key, -1)"
          >
            -1
          </button>
          <button class="stat-btn" :disabled="pointsAvailable < 1" @click="adjust(stat.key, 1)">
            +1
          </button>
          <button class="stat-btn" :disabled="pointsAvailable < 10" @click="adjust(stat.key, 10)">
            +10
          </button>
        </div>
      </div>
    </div>

    <div class="derived-grid">
      <div class="derived-col-stack">
        <div class="derived-col glass-panel">
          <h4>{{ $t('Base Stats') }}</h4>
          <div v-for="row in baseRows" :key="row.label" class="derived-row">
            <span>{{ $t(row.label) }}</span>
            <span>{{ row.value }}</span>
          </div>
        </div>

        <div class="derived-col glass-panel">
          <h4>{{ $t('Special Stats') }}</h4>
          <div class="derived-row">
            <span>{{ $t('Luck') }}</span>
            <span>{{ special.luck.toFixed(2) }}x</span>
          </div>
          <div class="derived-row">
            <span>{{ $t('Carry Limit') }}</span>
            <span>{{ special.carry_limit.toFixed(1) }} kg</span>
          </div>
          <div class="derived-row">
            <span>{{ $t('Fall Damage Threshold') }}</span>
            <span>{{ special.fall_damage_threshold.toFixed(1) }} m</span>
          </div>
          <BaseFinalRow
            v-for="row in thresholdRows"
            :key="row.key"
            :label="row.label"
            :editable="editable"
            :base-value="specialBases[row.key]"
            :final-value="special[row.key]"
            :unit="row.unit"
            whole-numbers
            @update:base-value="updateBase(row.key, $event)"
          />
        </div>
      </div>

      <div class="derived-col-stack">
        <div class="derived-col glass-panel">
          <h4>{{ $t('Special Defenses') }}</h4>
          <BaseFinalRow
            v-for="row in elementalRows"
            :key="row.key"
            :label="row.label"
            :editable="editable"
            :base-value="specialBases[row.key]"
            :final-value="elemental[row.finalKey]"
            unit="x"
            :decimals="2"
            @update:base-value="updateBase(row.key, $event)"
          />
        </div>

        <div class="derived-col glass-panel">
          <h4>{{ $t('Lifeskill') }}</h4>
          <BaseFinalRow
            v-for="row in lifeskillRows"
            :key="row.key"
            :label="row.label"
            :editable="editable"
            :base-value="specialBases[row.key]"
            :final-value="lifeskills[row.key]"
            whole-numbers
            @update:base-value="updateBase(row.key, $event)"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, h } from 'vue'
import { t, tr } from '../i18n'

const props = defineProps({
  modelValue: { type: Object, required: true }, // { vit, def, res, str, dex, intel, wis, agl }
  computed: { type: Object, required: true }, // { base, special, elemental, lifeskills }
  specialBases: { type: Object, default: () => ({}) }, // flat map keyed like SPECIAL_BASE_KEYS
  level: { type: Number, default: 1 },
  editable: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue', 'update:specialBases'])

const statList = [
  { key: 'vit', label: 'VIT' },
  { key: 'def', label: 'DEF' },
  { key: 'res', label: 'RES' },
  { key: 'str', label: 'STR' },
  { key: 'dex', label: 'DEX' },
  { key: 'intel', label: 'INT' },
  { key: 'wis', label: 'WIS' },
  { key: 'agl', label: 'AGL' },
]

const thresholdRows = [
  { key: 'sanity', label: tr('Sanity'), unit: 'γ' },
  { key: 'heat_threshold', label: tr('Heat Threshold'), unit: '°C' },
  { key: 'cold_threshold', label: tr('Cold Threshold'), unit: '°C' },
]

// finalKey differs from key because the computed.elemental payload uses
// the same resist_* names as the bases now, but is kept as a separate
// lookup here in case that ever needs to diverge again.
const elementalRows = [
  { key: 'resist_water', finalKey: 'resist_water', label: tr('Water') },
  { key: 'resist_fire', finalKey: 'resist_fire', label: tr('Fire') },
  { key: 'resist_wind', finalKey: 'resist_wind', label: tr('Wind') },
  { key: 'resist_earth', finalKey: 'resist_earth', label: tr('Earth') },
  { key: 'resist_ice', finalKey: 'resist_ice', label: tr('Ice') },
  { key: 'resist_thunder', finalKey: 'resist_thunder', label: tr('Thunder') },
  { key: 'resist_impact', finalKey: 'resist_impact', label: tr('Impact') },
]

const lifeskillRows = [
  { key: 'cooking', label: tr('Cooking') },
  { key: 'crafting', label: tr('Crafting') },
  { key: 'alchemy', label: tr('Alchemy') },
  { key: 'hunting', label: tr('Hunting') },
  { key: 'gathering', label: tr('Gathering') },
  { key: 'farming', label: tr('Farming') },
  { key: 'blessing', label: tr('Blessing') },
  { key: 'enchanting', label: tr('Enchanting') },
  { key: 'smithing', label: tr('Smithing') },
]

const pointsAvailable = computed(() => {
  const spent = statList.reduce((sum, s) => sum + (props.modelValue[s.key] || 0), 0)
  return props.level - spent
})

function adjust(key, delta) {
  const current = props.modelValue[key] || 0
  const next = Math.max(0, current + delta)
  emit('update:modelValue', { ...props.modelValue, [key]: next })
}

function updateBase(key, num) {
  emit('update:specialBases', { ...props.specialBases, [key]: num })
}

const primary = computed(() => ({
  vit: props.modelValue.vit ?? 0,
  def: props.modelValue.def ?? 0,
  res: props.modelValue.res ?? 0,
  str: props.modelValue.str ?? 0,
  dex: props.modelValue.dex ?? 0,
  intel: props.modelValue.intel ?? 0,
  wis: props.modelValue.wis ?? 0,
  agl: props.modelValue.agl ?? 0,
  ...(props.computed.primary || {}),
}))

function effectiveDelta(key) {
  return Math.round(primary.value[key]) - (props.modelValue[key] || 0)
}

const baseRows = computed(() => {
  const b = props.computed.base || {}
  return [
    { label: 'HP', value: Math.round(b.hp ?? 0) },
    { label: 'MP', value: Math.round(b.mp ?? 0) },
    { label: tr('Attack'), value: Math.round(b.attack ?? 0) },
    { label: tr('Potency'), value: Math.round(b.potency ?? 0) },
    { label: tr('Defense'), value: Math.round(b.defense ?? 0) },
    { label: tr('Resistance'), value: Math.round(b.resistance ?? 0) },
    { label: tr('Speed'), value: Math.round(b.speed ?? 0) },
  ]
})

const special = computed(() => ({
  luck: 1,
  carry_limit: 0,
  fall_damage_threshold: 0,
  sanity: 0,
  heat_threshold: 0,
  cold_threshold: 0,
  ...(props.computed.special || {}),
}))

const elemental = computed(() => ({
  resist_water: 1,
  resist_fire: 1,
  resist_wind: 1,
  resist_earth: 1,
  resist_ice: 1,
  resist_thunder: 1,
  resist_impact: 1,
  ...(props.computed.elemental || {}),
}))

const lifeskills = computed(() => ({
  cooking: 0,
  crafting: 0,
  alchemy: 0,
  hunting: 0,
  gathering: 0,
  farming: 0,
  blessing: 0,
  enchanting: 0,
  smithing: 0,
  ...(props.computed.lifeskills || {}),
}))

// A single row shared by Special Stats' thresholds, Special Defenses, and
// Lifeskill: in edit mode it's [label] [editable base input] [final
// value]; in view mode the input collapses away and it's just
// [label] [final value] — same shape everything else uses.
const BaseFinalRow = {
  props: {
    label: String,
    editable: Boolean,
    baseValue: { type: Number, default: 0 },
    finalValue: { type: Number, default: 0 },
    unit: { type: String, default: '' },
    decimals: { type: Number, default: 1 },
    wholeNumbers: { type: Boolean, default: false },
  },
  emits: ['update:baseValue'],
  render() {
    let formatted
    if (this.unit === 'x') {
      formatted = `${this.finalValue.toFixed(this.decimals)}x`
    } else if (this.wholeNumbers) {
      formatted = `${Math.round(this.finalValue)}${this.unit ? ' ' + this.unit : ''}`
    } else {
      formatted = `${this.finalValue.toFixed(this.decimals)}${this.unit ? ' ' + this.unit : ''}`
    }

    const children = [h('span', t(this.label))]
    if (this.editable) {
      children.push(
        h('input', {
          class: 'inline-number',
          type: 'number',
          step: this.wholeNumbers ? '1' : '0.1',
          value: this.wholeNumbers ? Math.round(this.baseValue) : this.baseValue,
          onInput: (e) => {
            const num = parseFloat(e.target.value)
            const safe = Number.isNaN(num) ? 0 : num
            this.$emit('update:baseValue', this.wholeNumbers ? Math.round(safe) : safe)
          },
        }),
      )
    }
    children.push(h('span', formatted))

    return h('div', { class: ['derived-row', this.editable && 'is-editable'] }, children)
  },
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
  grid-template-columns: repeat(8, 1fr);
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
