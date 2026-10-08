<template>
  <div v-if="entries.length" class="hover-pane glass-panel panel-solid">
    <div v-for="(e, i) in entries" :key="i" class="entry">
      <div class="entry-head">
        <span v-if="e.color" class="dot" :style="{ background: e.color }"></span>
        <span class="type">{{ $t(e.type) }}</span>
      </div>
      <h4 class="name">{{ e.name }}</h4>
      <p v-if="e.type === 'Location'" class="row">
        <span>{{ $t('Belongs to') }}</span>
        <strong>{{ e.belongsTo || $t('No kingdom') }}</strong>
      </p>
      <p class="row">
        <span>{{ $t('Founded') }}</span>
        <strong>{{ e.founding_date || $t('Unknown') }}</strong>
      </p>
      <p class="desc" :class="{ empty: !e.description }">
        {{ e.description || $t('No description.') }}
      </p>
    </div>
  </div>
</template>

<script setup>
import { tr } from '../i18n'

defineProps({
  // [{ type: 'Kingdom' | 'Major location' | 'Location', name, color, belongsTo,
  //    founding_date, description }]
  entries: { type: Array, default: () => [] },
})

// Entry types arrive from the map and are rendered with $t(e.type).
tr('Kingdom')
tr('Major location')
tr('Location')
</script>

<style scoped>
.hover-pane {
  position: absolute;
  top: 16px;
  right: 16px;
  width: 300px;
  max-height: calc(100% - 100px);
  overflow: hidden;
  padding: 0.9rem 1.1rem;
  pointer-events: none;
  z-index: 5;
}

.entry + .entry {
  margin-top: 0.8rem;
  padding-top: 0.8rem;
  border-top: 1px solid var(--glass-border);
}

.entry-head {
  display: flex;
  align-items: center;
  gap: 0.45rem;
}

.dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.type {
  font-size: 0.68rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  color: var(--accent);
}

.name {
  font-family: 'Fraunces', serif;
  font-weight: 500;
  font-size: 1.05rem;
  margin: 0.2rem 0 0.4rem;
  color: var(--text-primary);
}

.row {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
  margin: 0.15rem 0;
  font-size: 0.8rem;
  color: var(--text-faint);
}

.row strong {
  color: var(--text-primary);
  font-weight: 600;
  text-align: right;
}

.desc {
  margin: 0.5rem 0 0;
  font-size: 0.8rem;
  line-height: 1.5;
  color: var(--text-muted);
  white-space: pre-wrap;
  display: -webkit-box;
  -webkit-line-clamp: 6;
  line-clamp: 6;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.desc.empty {
  color: var(--text-faint);
  font-style: italic;
}
</style>
