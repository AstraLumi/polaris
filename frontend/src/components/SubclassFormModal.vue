<template>
  <div class="overlay" @click.self="$emit('close')">
    <div class="modal glass-panel is-wide">
      <h2>{{ subclass ? $t('Edit subclass') : $t('Add a subclass') }}</h2>
      <form @submit.prevent="submit">
        <label class="field">
          <span>{{ $t('Name') }}</span>
          <input v-model="name" type="text" required autofocus />
        </label>

        <div class="field">
          <span>{{ $t('Classes') }}</span>
          <p class="hint">
            {{ $t('Check one or more to restrict this subclass to those classes. Leave all unchecked and it shows up as an option regardless of class.') }}
          </p>
          <div class="class-checkboxes">
            <label v-for="c in classes" :key="c.id" class="checkbox-row">
              <input type="checkbox" :value="c.id" v-model="classIds" />
              <span>{{ c.name }}</span>
            </label>
            <p v-if="classes.length === 0" class="hint warning">{{ $t('No classes exist yet.') }}</p>
          </div>
        </div>

        <p class="hint">
          {{ $t('Leave a stat blank to leave it untouched. Every value here is a flat bonus (or penalty, with a minus sign) applied whenever this subclass is set on a character — except Base Stats, which scale per level automatically.') }}
        </p>

        <ModifierForm v-model="modifiers" />

        <p v-if="error" class="error-banner">{{ error }}</p>

        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="$emit('close')">{{ $t('Cancel') }}</button>
          <button type="submit" class="btn btn-primary" :disabled="submitting">
            {{ submitting ? $t('Saving…') : $t('Save subclass') }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { t } from '../i18n'
import { subclassesApi } from '../api'
import ModifierForm from './ModifierForm.vue'

const props = defineProps({
  classes: { type: Array, required: true }, // [{id, name}]
  subclass: { type: Object, default: null }, // { id, name, class_ids, modifiers } when editing
})
const emit = defineEmits(['close', 'saved'])

const name = ref(props.subclass?.name ?? '')
const classIds = ref([...(props.subclass?.class_ids ?? [])])
const modifiers = ref({ ...(props.subclass?.modifiers ?? {}) })
const submitting = ref(false)
const error = ref('')

async function submit() {
  if (!name.value.trim()) return
  submitting.value = true
  error.value = ''
  try {
    if (props.subclass) {
      await subclassesApi.update(props.subclass.id, name.value.trim(), classIds.value, modifiers.value)
    } else {
      await subclassesApi.create(name.value.trim(), classIds.value, modifiers.value)
    }
    emit('saved')
  } catch (err) {
    error.value = err.message || t("Couldn't save that subclass. Try again.")
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
  margin: 0.2rem 0 0.75rem;
}

.hint.warning {
  color: var(--danger-text);
}

.class-checkboxes {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem 1.25rem;
  margin-bottom: 1.25rem;
}

.checkbox-row {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.85rem;
  color: var(--text-primary);
}

.modal-actions {
  position: sticky;
  bottom: 0;
  background: var(--surface);
  backdrop-filter: blur(6px);
  padding-top: 0.75rem;
}

</style>
