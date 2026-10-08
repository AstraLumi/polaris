<template>
  <div class="overlay" @click.self="$emit('close')">
    <div class="modal glass-panel">
      <h2>{{ story ? $t('Edit story') : $t('New story') }}</h2>
      <form @submit.prevent="submit">
        <div class="icon-row">
          <div class="icon-preview">
            <IconImage :src="previewSrc" :name="name" />
          </div>
          <label class="field icon-field">
            <span>{{ $t('Icon') }}</span>
            <input ref="fileInput" type="file" accept="image/*" @change="onFileChange" />
          </label>
          <button v-if="previewSrc" type="button" class="btn btn-ghost small" @click="clearIcon">
            {{ $t('Remove') }}
          </button>
        </div>
        <div class="field">
          <span>{{ $t('No icon of your own yet? Pick a built-in one') }}</span>
          <div class="icon-grid" role="radiogroup" :aria-label="$t('Built-in icons')">
            <button
              v-for="i in STORY_ICONS"
              :key="i.id"
              type="button"
              class="icon-choice"
              :class="{ 'is-selected': previewSrc === BUILTIN_PREFIX + i.id }"
              role="radio"
              :aria-checked="previewSrc === BUILTIN_PREFIX + i.id"
              :title="$t(i.label)"
              @click="pickBuiltin(i.id)"
            >
              <IconImage :src="BUILTIN_PREFIX + i.id" :name="$t(i.label)" />
            </button>
          </div>
        </div>

        <label class="field">
          <span>{{ $t('Story name') }}</span>
          <input v-model="name" type="text" maxlength="60" required autofocus />
        </label>

        <div v-if="!story && others.length" class="field">
          <label for="copy-from">{{ $t('Start from') }}</label>
          <select id="copy-from" v-model="copyFrom">
            <option value="">{{ $t('Blank — nothing yet') }}</option>
            <option v-for="o in others" :key="o.id" :value="o.id">
              {{ $t('Copy Character Assets from {name}', { name: o.name }) }}
            </option>
          </select>
          <small class="hint">
            {{ $t('Copies the Character Assets library: classes, subclasses, specializations, races, body types, spells and anything else under Character Assets. Characters, events, the map and the calendar start empty.') }}
          </small>
        </div>

        <p v-if="error" class="error-banner">{{ error }}</p>
        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="$emit('close')">{{ $t('Cancel') }}</button>
          <button type="submit" class="btn btn-primary" :disabled="submitting || !name.trim()">
            {{ submitting ? (story ? $t('Saving…') : $t('Creating…')) : story ? $t('Save changes') : $t('Create story') }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onBeforeUnmount } from 'vue'
import IconImage from './IconImage.vue'
import { STORY_ICONS, BUILTIN_PREFIX } from '../builtinIcons'
import { createStory, updateStory } from '../stories'
import { t } from '../i18n'

const props = defineProps({
  story: { type: Object, default: null }, // null = creating
  others: { type: Array, default: () => [] }, // existing stories, for "copy from"
})
const emit = defineEmits(['close', 'saved'])

const name = ref(props.story?.name ?? '')
const copyFrom = ref('')
const iconFile = ref(null)
const fileUrl = ref('')
const fileInput = ref(null)
// '' = leave as is, 'none' = no icon, 'builtin:<id>' = a built-in one.
const iconChoice = ref('')
const submitting = ref(false)
const error = ref('')

const previewSrc = computed(() => {
  if (fileUrl.value) return fileUrl.value
  if (iconChoice.value === 'none') return ''
  if (iconChoice.value) return iconChoice.value
  return props.story?.icon || ''
})

// keepInput: leave the <input> alone, so a file the user just picked isn't
// wiped before it is read.
function dropFile(keepInput = false) {
  if (fileUrl.value) URL.revokeObjectURL(fileUrl.value)
  fileUrl.value = ''
  iconFile.value = null
  if (!keepInput && fileInput.value) fileInput.value.value = ''
}

function onFileChange(e) {
  const file = e.target.files[0] || null
  dropFile(true)
  if (file) {
    iconFile.value = file
    fileUrl.value = URL.createObjectURL(file)
    iconChoice.value = ''
  }
}

function pickBuiltin(id) {
  dropFile()
  iconChoice.value = BUILTIN_PREFIX + id
}

function clearIcon() {
  dropFile()
  iconChoice.value = 'none'
}

onBeforeUnmount(() => dropFile())

async function submit() {
  if (!name.value.trim()) return
  submitting.value = true
  error.value = ''
  try {
    const fields = { name: name.value.trim(), file: iconFile.value, iconChoice: iconChoice.value }
    const saved = props.story
      ? await updateStory(props.story.id, fields)
      : await createStory({ ...fields, copyFrom: copyFrom.value })
    emit('saved', saved, !props.story)
  } catch (err) {
    error.value = err.message || t("Couldn't save that story.")
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.icon-row {
  display: flex;
  align-items: flex-end;
  gap: 1rem;
  margin-bottom: 1rem;
}

.icon-preview {
  width: 64px;
  height: 64px;
  border-radius: 12px;
  background: var(--accent-soft);
  overflow: hidden;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
}

.icon-field {
  flex: 1;
  margin-bottom: 0;
}

.icon-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(2.6rem, 1fr));
  gap: 0.4rem;
}

.icon-choice {
  aspect-ratio: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 10px;
  border: 1px solid var(--glass-border);
  background: rgba(255, 255, 255, 0.04);
  cursor: pointer;
  padding: 0;
}

.icon-choice:hover {
  background: var(--accent-soft);
}

.icon-choice.is-selected {
  border-color: var(--accent);
  background: var(--accent-soft);
  box-shadow: 0 0 0 2px var(--accent-soft);
}

.hint {
  display: block;
  margin-top: 0.4rem;
  font-size: 0.78rem;
  line-height: 1.45;
  color: var(--text-faint);
}
</style>
