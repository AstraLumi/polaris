<template>
  <div class="overlay" @click.self="$emit('cancel')">
    <div class="modal glass-panel panel-solid">
      <h2>{{ title }}</h2>
      <p class="message">{{ message }}</p>
      <div class="modal-actions">
        <button class="btn btn-ghost" @click="$emit('cancel')">{{ $t('Cancel') }}</button>
        <button ref="confirmBtn" class="btn btn-danger" @click="$emit('confirm')">{{ confirmLabel || $t('Confirm') }}</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'

// Focused on open, so Enter confirms and Escape (App.vue) cancels.
const confirmBtn = ref(null)
onMounted(() => confirmBtn.value?.focus())

defineProps({
  title: { type: String, required: true },
  message: { type: String, default: '' },
  confirmLabel: { type: String, default: '' },
})
defineEmits(['cancel', 'confirm'])
</script>

<style scoped>
.overlay {
  z-index: 60;
}

.message {
  margin: 0;
  font-size: 0.88rem;
  color: var(--text-muted);
  line-height: 1.5;
}

</style>
