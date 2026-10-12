<template>
  <div class="overlay" @click.self="$emit('cancel')">
    <div class="modal is-small glass-panel panel-solid">
      <h2>{{ title }}</h2>
      <p v-if="message" class="message">{{ message }}</p>
      <form @submit.prevent="submit">
        <label class="field">
          <span>{{ $t('Type {word} to confirm', { word }) }}</span>
          <input ref="input" v-model="typed" type="text" autocomplete="off" spellcheck="false" />
        </label>
        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="$emit('cancel')">{{ $t('Cancel') }}</button>
          <button type="submit" class="btn btn-danger" :disabled="!matches">{{ $t('Restore default') }}</button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { t } from '../i18n'

// Asks before putting a setting back as Polaris ships it: the button only
// works once the confirmation word is typed, so a stray click or Enter
// can't throw away someone's customizations.
defineProps({
  title: { type: String, required: true },
  message: { type: String, default: '' },
})
const emit = defineEmits(['cancel', 'confirm'])

const word = computed(() => t('restore'))
const typed = ref('')
const matches = computed(() => typed.value.trim().toLowerCase() === word.value.toLowerCase())

// Focused on open; Escape (App.vue) cancels.
const input = ref(null)
onMounted(() => input.value?.focus())

function submit() {
  if (matches.value) emit('confirm')
}
</script>

<style scoped>
.overlay {
  z-index: 60;
}

.message {
  margin: 0 0 0.9rem;
  font-size: 0.88rem;
  color: var(--text-muted);
  line-height: 1.5;
}
</style>
