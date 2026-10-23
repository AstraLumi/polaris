<template>
  <section class="not-found glass-panel">
    <p class="mark" aria-hidden="true">?</p>
    <h1>{{ $t('Nothing here') }}</h1>
    <p class="message">{{ message || $t("This page doesn't exist, or what it showed has been deleted.") }}</p>
    <div class="actions">
      <slot />
      <button type="button" class="btn btn-ghost" @click="openSearch">{{ $t('Search') }} <kbd>Ctrl K</kbd></button>
      <RouterLink to="/" class="btn btn-primary">{{ $t('Home') }}</RouterLink>
    </div>
  </section>
</template>

<script setup>
// What an unknown address, or an old link to something deleted, shows: the
// catch-all page (NotFound.vue) and every item page whose item is gone.
// The slot takes extra buttons, such as a link to the page's own list.
import { openSearch } from '../navigation'

defineProps({
  message: { type: String, default: '' }, // already translated
})
</script>

<style scoped>
.not-found {
  max-width: 34rem;
  margin: 2.5rem auto;
  padding: 2rem 1.75rem;
  text-align: center;
}

.mark {
  margin: 0 auto 0.6rem;
  width: 3.2rem;
  height: 3.2rem;
  line-height: 3.2rem;
  border-radius: 50%;
  background: var(--accent-soft);
  color: var(--accent);
  font-family: 'Fraunces', serif;
  font-size: 1.6rem;
}

h1 {
  margin: 0;
  font-family: 'Fraunces', serif;
  font-weight: 500;
  font-size: 1.5rem;
  color: var(--text-primary);
}

.message {
  margin: 0.5rem 0 1.4rem;
  font-size: 0.92rem;
  color: var(--text-muted);
}

.actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 0.5rem;
}

kbd {
  margin-left: 0.45rem;
  padding: 0 0.35rem;
  border: 1px solid var(--glass-border);
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.7rem;
  color: var(--text-faint);
}

/* Phones have no Ctrl key to press. */
@media (hover: none) {
  kbd {
    display: none;
  }
}
</style>
