<template>
  <div class="overlay" @click.self="$emit('close')">
    <div class="modal glass-panel is-wide">
      <h2>{{ asset ? text.edit : text.add }}</h2>
      <form @submit.prevent="submit">
        <div v-if="withIcon" class="icon-row">
          <div class="icon-preview"><IconImage :src="previewSrc" :name="name" /></div>
          <label class="field icon-field">
            <span>{{ $t('Icon') }}</span>
            <input ref="fileInput" type="file" accept="image/*" @change="onFileChange" />
          </label>
          <button v-if="previewSrc" type="button" class="btn btn-ghost small" @click="removeIcon">
            {{ $t('Remove') }}
          </button>
        </div>

        <label class="field">
          <span>{{ $t('Name') }}</span>
          <input v-model="name" type="text" required autofocus />
        </label>

        <p class="hint">
          {{ text.hint }}
        </p>

        <ModifierForm v-model="modifiers" />

        <p v-if="error" class="error-banner">{{ error }}</p>

        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="$emit('close')">{{ $t('Cancel') }}</button>
          <button type="submit" class="btn btn-primary" :disabled="submitting">
            {{ submitting ? $t('Saving…') : text.save }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onBeforeUnmount } from 'vue'
import { t } from '../i18n'
import ModifierForm from './ModifierForm.vue'
import IconImage from './IconImage.vue'

const props = defineProps({
  kind: { type: String, required: true }, // "class" | "race" | "body type"
  api: { type: Object, required: true }, // { fetch, create, update, delete }
  asset: { type: Object, default: null }, // { id, name, modifiers } when editing, null when adding
  withIcon: { type: Boolean, default: false }, // classes can carry an uploaded icon
})
const emit = defineEmits(['close', 'saved'])

// Whole sentences per kind, so each language can word them its own way.
const text = computed(() => {
  if (props.kind === 'race') {
    return {
      add: t('Add a race'),
      edit: t('Edit race'),
      save: t('Save race'),
      failed: t("Couldn't save that race. Try again."),
      hint: t('Leave a stat blank to leave it untouched. Every value here is a flat bonus (or penalty, with a minus sign) applied whenever this race is set on a character — except Base Stats, which scale per level automatically.'),
    }
  }
  if (props.kind === 'body type') {
    return {
      add: t('Add a body type'),
      edit: t('Edit body type'),
      save: t('Save body type'),
      failed: t("Couldn't save that body type. Try again."),
      hint: t('Leave a stat blank to leave it untouched. Every value here is a flat bonus (or penalty, with a minus sign) applied whenever this body type is set on a character — except Base Stats, which scale per level automatically.'),
    }
  }
  return {
    add: t('Add a class'),
    edit: t('Edit class'),
    save: t('Save class'),
    failed: t("Couldn't save that class. Try again."),
    hint: t('Leave a stat blank to leave it untouched. Every value here is a flat bonus (or penalty, with a minus sign) applied whenever this class is set on a character — except Base Stats, which scale per level automatically.'),
  }
})

const name = ref(props.asset?.name ?? '')
const modifiers = ref({ ...(props.asset?.modifiers ?? {}) })
const submitting = ref(false)
const error = ref('')

// Icon (classes only): a newly chosen file, or a request to drop the old one.
const fileInput = ref(null)
const iconFile = ref(null)
const fileUrl = ref('')
const iconRemoved = ref(false)

const previewSrc = computed(() => {
  if (fileUrl.value) return fileUrl.value
  if (iconRemoved.value) return ''
  return props.asset?.icon_path || ''
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
    iconRemoved.value = false
  }
}

function removeIcon() {
  dropFile()
  iconRemoved.value = true
}

onBeforeUnmount(() => dropFile())

async function submit() {
  if (!name.value.trim()) return
  submitting.value = true
  error.value = ''
  try {
    const saved = props.asset
      ? await props.api.update(props.asset.id, name.value.trim(), modifiers.value)
      : await props.api.create(name.value.trim(), modifiers.value)
    if (props.withIcon) {
      if (iconFile.value) await props.api.setIcon(saved.id, iconFile.value)
      else if (iconRemoved.value && props.asset?.icon_path) await props.api.clearIcon(saved.id)
    }
    emit('saved')
  } catch (err) {
    error.value = err.message || text.value.failed
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

.hint {
  font-size: 0.8rem;
  color: var(--text-faint);
  line-height: 1.5;
  margin: -0.5rem 0 1.25rem;
}

.modal-actions {
  position: sticky;
  bottom: 0;
  background: var(--surface);
  backdrop-filter: blur(6px);
  padding-top: 0.75rem;
}

</style>
