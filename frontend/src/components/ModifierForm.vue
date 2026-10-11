<template>
  <div class="modifier-form">
    <div v-for="group in groups" :key="group.id" class="modifier-group">
      <h4>{{ $t(group.label) }}</h4>
      <div class="modifier-grid">
        <label v-for="s in group.stats" :key="s.key" class="modifier-field">
          <span :title="statName(s)">{{ statShort(s) }}</span>
          <input
            type="number"
            step="0.1"
            :value="modelValue[s.key] ?? ''"
            placeholder="0"
            @input="update(s.key, $event.target.value)"
          />
        </label>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { STAT_CATEGORIES, statSystem, statsIn, statName, statShort } from '../stats'

// One group per category, with the story's own stats (Settings → Stats).
const groups = computed(() =>
  STAT_CATEGORIES.map((c) => ({ ...c, stats: statsIn(c.id, statSystem.stats) })).filter((g) => g.stats.length),
)

const props = defineProps({
  modelValue: { type: Object, required: true }, // flat { target_stat: value }
})
const emit = defineEmits(['update:modelValue'])

function update(key, rawValue) {
  const next = { ...props.modelValue }
  if (rawValue === '') {
    delete next[key]
  } else {
    const num = parseFloat(rawValue)
    if (!Number.isNaN(num)) next[key] = num
  }
  emit('update:modelValue', next)
}
</script>

<style scoped>
.modifier-group {
  margin-bottom: 1.5rem;
}

.modifier-group h4 {
  font-family: 'Fraunces', serif;
  font-weight: 500;
  font-size: 0.9rem;
  margin: 0 0 0.6rem;
  color: var(--text-primary);
}

.modifier-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(130px, 1fr));
  gap: 0.6rem;
}

.modifier-field {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  font-size: 0.72rem;
  color: var(--text-faint);
}

.modifier-field input {
  width: 100%;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--glass-border);
  border-radius: 6px;
  padding: 0.4rem 0.5rem;
  color: var(--text-primary);
  font-family: 'Manrope', sans-serif;
  font-size: 0.85rem;
}
</style>
