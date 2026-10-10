<template>
  <div class="overlay" @click.self="$emit('close')">
    <div class="modal glass-panel is-small">
      <h2>{{ $t('New version') }}</h2>
      <p class="hint">
        {{ $t('Starts as a copy of the current version\'s Story and Build data — tag it with where it falls in the timeline, then edit it from there.') }}
      </p>
      <form @submit.prevent="submit">
        <DateField v-model="versionDate" :label="$t('Story date')" />
        <ChapterSelect v-model="chapterId" :label="$t('Chapter')" />
        <label class="field">
          <span>{{ $t('Volume / Chapter / Page') }}</span>
          <input v-model="versionReference" type="text" :placeholder="$t('Vol. 2 - Ch. 5 - Pg. 12')" />
        </label>

        <p v-if="error" class="error-banner">{{ error }}</p>

        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="$emit('close')">{{ $t('Cancel') }}</button>
          <button type="submit" class="btn btn-primary" :disabled="submitting">
            {{ submitting ? $t('Creating…') : $t('Create version') }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { t } from '../i18n'
import { createVersion } from '../api'
import DateField from './DateField.vue'
import ChapterSelect from './ChapterSelect.vue'

const props = defineProps({
  characterId: { type: [String, Number], required: true },
})
const emit = defineEmits(['close', 'created'])

const versionDate = ref('')
const versionReference = ref('')
const chapterId = ref(null)
const submitting = ref(false)
const error = ref('')

async function submit() {
  submitting.value = true
  error.value = ''
  try {
    const result = await createVersion(props.characterId, {
      versionDate: versionDate.value,
      versionReference: versionReference.value,
      chapterId: chapterId.value,
    })
    emit('created', result.id)
  } catch (err) {
    error.value = t("Couldn't create that version. Try again.")
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.hint {
  font-size: 0.85rem;
  color: var(--text-muted);
  line-height: 1.5;
  margin: 0 0 1.5rem;
}

</style>
