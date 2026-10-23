<template>
  <!-- Filters and controls: always shown on a wide screen; on a phone,
       folded behind one button so the list or the drawing comes first. -->
  <div class="fold-on-phone">
    <button
      v-if="phone"
      type="button"
      class="btn btn-ghost small fold-toggle"
      :class="{ 'is-open': open, 'is-active': active > 0 }"
      :aria-expanded="open"
      @click="open = !open"
    >
      {{ active ? $t('{label} ({n})', { label, n: active }) : label }}
      <span class="caret" aria-hidden="true">{{ open ? '▴' : '▾' }}</span>
    </button>
    <div v-show="!phone || open" class="fold-body">
      <slot />
    </div>
  </div>
</template>

<script setup>
import { ref, onBeforeUnmount } from 'vue'

defineProps({
  label: { type: String, required: true }, // already translated: "Filters", "Controls"
  active: { type: Number, default: 0 }, // how many are set, shown on the button
})

const query = window.matchMedia('(max-width: 720px)')
const phone = ref(query.matches)
const open = ref(false)
const onChange = (e) => (phone.value = e.matches)
query.addEventListener('change', onChange)
onBeforeUnmount(() => query.removeEventListener('change', onChange))
</script>

<style scoped>
.fold-toggle {
  margin-bottom: 0.6rem;
}

.fold-toggle.is-active {
  border-color: var(--accent);
  color: var(--accent);
}

.caret {
  font-size: 0.7rem;
}
</style>
