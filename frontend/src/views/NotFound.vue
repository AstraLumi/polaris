<template>
  <div class="page is-narrow">
    <NotFoundPanel />
  </div>
</template>

<script setup>
// The catch-all route (router.js): a mistyped address or an old link.
import NotFoundPanel from '../components/NotFoundPanel.vue'
</script>
