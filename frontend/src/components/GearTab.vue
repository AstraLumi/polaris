<template>
  <div class="gear-tab">
    <div class="gear-layout">
      <div class="col">
        <GearSlot v-for="s in left" :key="s.key" :slot-def="s" :gear="modelValue[s.key]" :editable="editable" @open="open(s)" />
      </div>

      <div class="figure" aria-hidden="true">
        <svg viewBox="0 0 120 260" class="silhouette">
          <circle cx="60" cy="30" r="17" />
          <path d="M44 56 Q60 50 76 56 L90 62 Q98 66 98 76 L98 128 Q98 134 92 134 Q86 134 86 128 L86 88 L82 92 L82 148 L78 244 Q78 250 72 250 L64 250 L62 160 L58 160 L56 250 L48 250 Q42 250 42 244 L38 148 L38 92 L34 88 L34 128 Q34 134 28 134 Q22 134 22 128 L22 76 Q22 66 30 62 Z" />
        </svg>
      </div>

      <div class="col">
        <GearSlot v-for="s in right" :key="s.key" :slot-def="s" :gear="modelValue[s.key]" :editable="editable" @open="open(s)" />
      </div>
    </div>

    <div class="boots-row">
      <GearSlot :slot-def="bootsSlot" :gear="modelValue.boots" :editable="editable" @open="open(bootsSlot)" />
    </div>

    <div class="summary">
      <div class="weight-card">
        <div class="weight-head">
          <span>{{ $t('Total weight equipped') }}</span>
          <strong :class="{ 'is-over': over }">
            {{ formatKg(weight) }}<template v-if="limit > 0"> / {{ formatKg(limit) }}</template>
          </strong>
        </div>
        <div v-if="limit > 0" class="bar"><div class="fill" :class="{ 'is-over': over }" :style="{ width: pct + '%' }"></div></div>
        <p v-if="over" class="over-note">{{ $t('Over your carry limit by {kg}.', { kg: formatKg(weight - limit) }) }}</p>
      </div>

      <div class="bonus-card">
        <span class="bonus-title">{{ $t('Bonuses from gear') }}</span>
        <div v-if="bonuses.length" class="bonus-chips">
          <span v-for="b in bonuses" :key="b.key" class="bonus" :class="{ 'is-negative': b.value < 0 }">{{ b.text }}</span>
        </div>
        <span v-else class="no-bonus">{{ $t('No stat bonuses') }}</span>
      </div>
    </div>

    <GearPickerModal
      v-if="picking"
      :slot-def="picking"
      :current="modelValue[picking.key] || null"
      @close="picking = null"
      @pick="pick"
    />
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { EQUIP_SLOTS, totalBonuses, totalWeight, formatBonuses, formatKg } from '../gear'
import GearSlot from './GearSlot.vue'
import GearPickerModal from './GearPickerModal.vue'

const props = defineProps({
  modelValue: { type: Object, default: () => ({}) }, // slot key -> gear
  editable: { type: Boolean, default: false },
  carryLimit: { type: Number, default: 0 },
})
const emit = defineEmits(['update:modelValue'])

const by = (k) => EQUIP_SLOTS.find((s) => s.key === k)
const left = ['helmet', 'necklace', 'torso', 'glove1', 'ring1', 'mainhand'].map(by)
const right = ['face', 'cape', 'pants', 'glove2', 'ring2', 'offhand'].map(by)
const bootsSlot = by('boots')

const picking = ref(null)

function open(s) {
  if (props.editable) picking.value = s
}

function pick(g) {
  const next = { ...props.modelValue }
  if (g) next[picking.value.key] = g
  else delete next[picking.value.key]
  emit('update:modelValue', next)
  picking.value = null
}

const weight = computed(() => totalWeight(props.modelValue))
const limit = computed(() => Number(props.carryLimit) || 0)
const over = computed(() => limit.value > 0 && weight.value > limit.value)
const pct = computed(() => (limit.value > 0 ? Math.min(100, (weight.value / limit.value) * 100) : 0))
const bonuses = computed(() => formatBonuses(totalBonuses(props.modelValue)))
</script>

<style scoped>
.gear-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(90px, 160px) minmax(0, 1fr);
  gap: 1rem;
  max-width: 760px;
  margin: 0 auto;
  align-items: stretch;
}
.col { min-width: 0; display: flex; flex-direction: column; gap: 0.6rem; }
.figure { display: flex; align-items: center; justify-content: center; }
.silhouette { width: 100%; max-height: 420px; fill: color-mix(in srgb, var(--accent) 10%, transparent); stroke: color-mix(in srgb, var(--accent) 35%, transparent); stroke-width: 1.5; }
.boots-row { max-width: 360px; margin: 0.8rem auto 0; }

.summary { display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap: 1rem; margin-top: 1.5rem; }
.weight-card, .bonus-card { padding: 0.9rem 1.1rem; background: rgba(255,255,255,0.03); border: 1px solid var(--glass-border); border-radius: 12px; }
.weight-head { display: flex; justify-content: space-between; gap: 1rem; font-size: 0.88rem; color: var(--text-muted); }
.weight-head strong { color: var(--text-primary); }
.weight-head strong.is-over, .over-note { color: var(--danger, #e5798a); }
.over-note { margin: 0.5rem 0 0; font-size: 0.78rem; }
.bar { height: 8px; border-radius: 999px; background: rgba(255,255,255,0.07); margin-top: 0.7rem; overflow: hidden; }
.fill { height: 100%; background: var(--accent); border-radius: 999px; transition: width 0.25s; }
.fill.is-over { background: var(--danger, #e5798a); }
.bonus-title { display: block; font-size: 0.88rem; color: var(--text-muted); margin-bottom: 0.55rem; }
.bonus-chips { display: flex; flex-wrap: wrap; gap: 0.35rem; }
.bonus { padding: 0.1rem 0.5rem; border-radius: 6px; background: rgba(255,255,255,0.06); font-size: 0.8rem; color: var(--text-primary); }
.bonus.is-negative { color: var(--danger, #e5798a); }
.no-bonus { color: var(--text-faint); font-style: italic; font-size: 0.82rem; }

@media (max-width: 560px) {
  .gear-layout { grid-template-columns: minmax(0, 1fr); }
  .col { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); }
  .figure { display: none; }
}
</style>
