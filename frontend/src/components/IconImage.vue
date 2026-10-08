<template>
  <!-- Fills its parent (which sets the size and shape). Shows, in order: a
       built-in icon, an uploaded picture, or the first letter of the name. -->
  <svg
    v-if="builtin"
    class="icon-builtin"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.7"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <path v-for="(d, i) in builtin.d" :key="i" :d="d" />
  </svg>
  <img v-else-if="src" class="icon-img" :src="src" :alt="alt" />
  <span v-else class="icon-letter">{{ letter }}</span>
</template>

<script setup>
import { computed } from 'vue'
import { builtinIconFor } from '../builtinIcons'

const props = defineProps({
  src: { type: String, default: '' },
  name: { type: String, default: '' },
  alt: { type: String, default: '' },
})

const builtin = computed(() => builtinIconFor(props.src))
const letter = computed(() => (props.name || '?').trim().charAt(0).toUpperCase())
</script>

<style scoped>
.icon-builtin {
  width: 62%;
  height: 62%;
  color: var(--accent);
}

.icon-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.icon-letter {
  font-family: 'Fraunces', serif;
  color: var(--accent);
  font-size: 1.2em;
}
</style>
