<template>
  <div class="spell-row glass-panel">
    <div class="spell-icon">
      <IconImage :src="spell.icon_path" :name="spell.name" :alt="spell.name" />
    </div>

    <div class="spell-body">
      <div class="spell-title">
        <span class="spell-name">{{ spell.name }}</span>
        <span class="spell-level">{{ formatSpellLevel(spell.level) }}</span>
      </div>
      <div class="spell-meta">
        <span class="spell-cost" :class="{ 'is-free': isFree }">{{ cost }}</span>
        <span v-if="source" class="spell-meta-item">{{ source }}</span>
        <span v-if="spell.origin" class="spell-meta-item">{{ spell.origin }}</span>
      </div>
      <p v-if="spell.description" class="spell-description">{{ spell.description }}</p>
    </div>

    <div v-if="$slots.actions" class="spell-actions">
      <slot name="actions" />
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import IconImage from './IconImage.vue'
import { formatSpellCost, formatSpellLevel, formatSpellSource } from '../spellUtils'

const props = defineProps({
  spell: { type: Object, required: true },
})

const cost = computed(() => formatSpellCost(props.spell))
const isFree = computed(() => !props.spell.mp_cost && !props.spell.hp_cost)
const source = computed(() => formatSpellSource(props.spell))
</script>

<style scoped>
.spell-row {
  display: flex;
  align-items: flex-start;
  gap: 1rem;
  padding: 0.9rem 1.1rem;
}

.spell-icon {
  width: 52px;
  height: 52px;
  border-radius: 10px;
  background: var(--accent-soft);
  overflow: hidden;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
}

.spell-body {
  flex: 1;
  min-width: 0;
}

.spell-title {
  display: flex;
  align-items: baseline;
  gap: 0.6rem;
  flex-wrap: wrap;
}

.spell-name {
  font-family: 'Fraunces', serif;
  font-size: 1rem;
  color: var(--text-primary);
}

.spell-level {
  font-size: 0.75rem;
  color: var(--text-faint);
}

.spell-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem 0.5rem;
  margin-top: 0.35rem;
}

.spell-cost {
  font-size: 0.72rem;
  font-weight: 700;
  color: var(--accent);
  background: var(--accent-soft);
  border-radius: 999px;
  padding: 0.15rem 0.6rem;
}

.spell-cost.is-free {
  color: var(--text-muted);
  background: rgba(255, 255, 255, 0.05);
}

.spell-meta-item {
  font-size: 0.72rem;
  color: var(--text-muted);
  border: 1px solid var(--glass-border);
  border-radius: 999px;
  padding: 0.15rem 0.6rem;
}

.spell-description {
  margin: 0.55rem 0 0;
  font-size: 0.82rem;
  line-height: 1.5;
  color: var(--text-muted);
  white-space: pre-wrap;
}

.spell-actions {
  display: flex;
  gap: 0.5rem;
  flex-shrink: 0;
}
</style>
