<template>
  <nav v-if="prev || next" class="pager no-print" :aria-label="label">
    <RouterLink v-if="prev" :to="prev.to" class="pager-link is-prev" rel="prev">
      <span class="pager-dir">{{ prevLabel }}</span>
      <span class="pager-name">{{ prev.name }}</span>
    </RouterLink>
    <span class="spacer"></span>
    <RouterLink v-if="next" :to="next.to" class="pager-link is-next" rel="next">
      <span class="pager-dir">{{ nextLabel }}</span>
      <span class="pager-name">{{ next.name }}</span>
    </RouterLink>
  </nav>
</template>

<script setup>
// "← Previous event · Next event →" at the foot of an item's page, each
// with the name of where it leads. Labels come translated, arrows included.
defineProps({
  prev: { type: Object, default: null }, // { to, name }
  next: { type: Object, default: null },
  prevLabel: { type: String, required: true },
  nextLabel: { type: String, required: true },
  label: { type: String, default: '' }, // for screen readers
})
</script>

<style scoped>
.pager {
  display: flex;
  align-items: stretch;
  gap: 0.6rem;
  margin-top: 1rem;
}

.spacer {
  flex: 1;
}

.pager-link {
  display: flex;
  flex-direction: column;
  gap: 0.1rem;
  min-width: 0;
  max-width: 48%;
  padding: 0.45rem 0.85rem;
  border: 1px solid var(--glass-border);
  border-radius: 10px;
  text-decoration: none;
}

.pager-link.is-next {
  align-items: flex-end;
  text-align: right;
}

.pager-dir {
  font-size: 0.72rem;
  color: var(--text-faint);
}

.pager-name {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.88rem;
  color: var(--text-primary);
}

.pager-link:hover,
.pager-link:focus-visible {
  border-color: color-mix(in srgb, var(--accent) 45%, transparent);
}

.pager-link:hover .pager-name,
.pager-link:focus-visible .pager-name {
  color: var(--accent);
}
</style>
