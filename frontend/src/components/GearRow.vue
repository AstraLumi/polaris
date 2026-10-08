<template>
  <div class="gear-row glass-panel">
    <div class="gear-icon">
      <IconImage :src="gearIconSrc(gear)" :name="gear.name" :alt="gear.name" />
    </div>

    <div class="gear-body">
      <div class="gear-title">
        <span class="gear-name">{{ gear.name }}</span>
        <span class="gear-slot-pill">{{ slotTypeLabel(gear.slot) }}</span>
      </div>
      <div class="gear-meta">
        <span class="gear-weight">{{ formatKg(gear.weight) }}</span>
        <span v-for="b in bonuses" :key="b.key" class="bonus" :class="{ 'is-negative': b.value < 0 }">{{ b.text }}</span>
        <span v-if="!bonuses.length" class="no-bonus">{{ $t('No stat bonuses') }}</span>
      </div>
    </div>

    <div v-if="$slots.actions" class="gear-actions">
      <slot name="actions" />
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import IconImage from './IconImage.vue'
import { gearIconSrc, slotTypeLabel, formatBonuses, formatKg } from '../gear'

const props = defineProps({
  gear: { type: Object, required: true },
})

const bonuses = computed(() => formatBonuses(props.gear.modifiers))
</script>

<style scoped>
.gear-row {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 0.8rem 1.1rem;
}

.gear-icon {
  width: 52px;
  height: 52px;
  flex-shrink: 0;
  border-radius: 12px;
  background: var(--accent-soft);
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
}

.gear-body {
  flex: 1;
  min-width: 0;
}

.gear-title {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  flex-wrap: wrap;
}

.gear-name {
  font-family: 'Fraunces', serif;
  font-size: 1.02rem;
  color: var(--text-primary);
}

.gear-slot-pill {
  font-size: 0.68rem;
  padding: 0.1rem 0.5rem;
  border-radius: 999px;
  color: var(--accent);
  background: var(--accent-soft);
  border: 1px solid color-mix(in srgb, var(--accent) 30%, transparent);
}

.gear-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.35rem 0.5rem;
  margin-top: 0.3rem;
  font-size: 0.78rem;
}

.gear-weight {
  color: var(--text-muted);
  margin-right: 0.25rem;
}

.bonus {
  padding: 0.05rem 0.45rem;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.05);
  color: var(--text-primary);
}

.bonus.is-negative {
  color: var(--danger, #e5798a);
}

.no-bonus {
  color: var(--text-faint);
  font-style: italic;
}

.gear-actions {
  display: flex;
  gap: 0.5rem;
  flex-shrink: 0;
}
</style>
