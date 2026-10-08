<template>
  <div class="overlay" @click.self="$emit('cancel')">
    <div class="modal is-small glass-panel panel-solid">
      <h2>{{ $t('Delete {name}?', { name: story.name }) }}</h2>
      <p class="message">
        {{ $t("This permanently deletes the story and everything in it: characters, versions, events, the map, assets and uploaded pictures. This can't be undone.") }}
      </p>
      <p class="message">
        <a :href="storyExportUrl(story.id)" download>{{ $t('Export a backup first') }}</a>
      </p>
      <form @submit.prevent="submit">
        <label class="field">
          <span>{{ $t('Type {name} to confirm', { name: story.name }) }}</span>
          <input v-model="typed" type="text" autocomplete="off" autofocus />
        </label>
        <p v-if="error" class="error-banner">{{ error }}</p>
        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="$emit('cancel')">{{ $t('Cancel') }}</button>
          <button type="submit" class="btn btn-danger" :disabled="!matches || busy">
            {{ $t('Delete story') }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { deleteStory, storyExportUrl } from '../stories'
import { t } from '../i18n'

const props = defineProps({ story: { type: Object, required: true } })
const emit = defineEmits(['cancel', 'deleted'])

const typed = ref('')
const busy = ref(false)
const error = ref('')
const matches = computed(() => typed.value.trim() === props.story.name)

async function submit() {
  if (!matches.value) return
  busy.value = true
  error.value = ''
  try {
    await deleteStory(props.story.id, typed.value.trim())
    emit('deleted', props.story)
  } catch (err) {
    error.value = err.message || t("Couldn't delete that story.")
  } finally {
    busy.value = false
  }
}
</script>

<style scoped>
.message {
  margin: 0 0 0.9rem;
  font-size: 0.88rem;
  color: var(--text-muted);
  line-height: 1.5;
}
</style>
