<template>
  <div class="overlay" @click.self="$emit('close')">
    <div class="modal glass-panel is-wide">
      <h2>{{ specialization ? $t('Edit specialization') : $t('Add a specialization') }}</h2>
      <form @submit.prevent="submit">
        <div class="field-row">
          <label class="field">
            <span>{{ $t('Name') }}</span>
            <input v-model="name" type="text" required autofocus />
          </label>
          <label class="field">
            <span>{{ $t('Class') }}</span>
            <select v-model.number="classId" required>
              <option value="" disabled>{{ $t('Choose a class') }}</option>
              <option v-for="c in classes" :key="c.id" :value="c.id">{{ c.name }}</option>
            </select>
          </label>
        </div>

        <p v-if="classes.length === 0" class="hint warning">
          {{ $t('No classes exist yet — add one on the Classes tab first.') }}
        </p>
        <p class="hint">
          {{ $t('Leave a stat blank to leave it untouched. Every value here is a flat bonus (or penalty, with a minus sign) applied whenever this specialization is set on a character — except Base Stats, which scale per level automatically.') }}
        </p>

        <ModifierForm v-model="modifiers" />

        <p v-if="error" class="error-banner">{{ error }}</p>

        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="$emit('close')">{{ $t('Cancel') }}</button>
          <button type="submit" class="btn btn-primary" :disabled="submitting || classes.length === 0">
            {{ submitting ? $t('Saving…') : $t('Save specialization') }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { t } from '../i18n'
import { specializationsApi } from '../api'
import ModifierForm from './ModifierForm.vue'

const props = defineProps({
  classes: { type: Array, required: true }, // [{id, name}]
  specialization: { type: Object, default: null }, // { id, name, class_id, modifiers } when editing
})
const emit = defineEmits(['close', 'saved'])

const name = ref(props.specialization?.name ?? '')
const classId = ref(props.specialization?.class_id ?? '')
const modifiers = ref({ ...(props.specialization?.modifiers ?? {}) })
const submitting = ref(false)
const error = ref('')

async function submit() {
  if (!name.value.trim() || !classId.value) return
  submitting.value = true
  error.value = ''
  try {
    if (props.specialization) {
      await specializationsApi.update(props.specialization.id, name.value.trim(), classId.value, modifiers.value)
    } else {
      await specializationsApi.create(name.value.trim(), classId.value, modifiers.value)
    }
    emit('saved')
  } catch (err) {
    error.value = err.message || t("Couldn't save that specialization. Try again.")
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.hint {
  font-size: 0.8rem;
  color: var(--text-faint);
  line-height: 1.5;
  margin: -0.5rem 0 1.25rem;
}

.hint.warning {
  color: var(--danger-text);
}

.modal-actions {
  position: sticky;
  bottom: 0;
  background: var(--surface);
  backdrop-filter: blur(6px);
  padding-top: 0.75rem;
}

</style>
