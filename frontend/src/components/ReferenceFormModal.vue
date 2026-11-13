<template>
  <div class="overlay" @click.self="requestClose" @pointerdown.capture="touched" @keydown.capture="touched" @input.capture="touched">
    <div class="modal is-wide glass-panel panel-solid" @paste="onPaste">
      <h2>{{ reference ? $t('Edit reference') : $t('New reference') }}</h2>
      <form @submit.prevent="submit">
        <div class="field-row">
          <label class="field grow">
            <span>{{ $t('Title') }}</span>
            <input v-model="title" type="text" maxlength="200" required autofocus />
          </label>
          <label class="field">
            <span>{{ $t('Kind') }}</span>
            <select v-model="kind">
              <option v-for="k in REFERENCE_KINDS" :key="k.id" :value="k.id">{{ $t(k.label) }}</option>
            </select>
          </label>
        </div>
        <label class="field">
          <span>{{ $t('Source link') }}</span>
          <input v-model="sourceUrl" type="url" maxlength="2000" placeholder="https://…" />
        </label>
        <PictureListField ref="pictureField" :pictures="reference?.pictures || []" @update:state="pictures = $event" />
        <TagInput v-model="tags" :suggestions="tagSuggestions" list-id="reference-tags" />
        <label class="field">
          <span>{{ $t('Description') }}</span>
          <textarea v-model="description" rows="4"></textarea>
        </label>
        <p v-if="error" class="error-banner" role="alert">{{ error }}</p>
        <div class="modal-actions sticky-save">
          <button v-if="reference" type="button" class="btn btn-danger" @click="remove">{{ $t('Delete') }}</button>
          <span class="spacer"></span>
          <button type="button" class="btn btn-ghost" @click="$emit('close')">{{ $t('Cancel') }}</button>
          <button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? $t('Saving…') : $t('Save') }}</button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup>
// Adds or edits a reference of the shared library: what it is, where it
// came from, its pictures (picked, dropped or pasted), tags and notes.
import { ref, computed } from 'vue'
import { t } from '../i18n'
import { useModalGuard } from '../navigation'
import { askConfirm } from '../confirm'
import { REFERENCE_KINDS, referenceLibrary, saveReference, deleteReference } from '../referenceLibrary'
import { allTags } from '../cultures'
import PictureListField from './PictureListField.vue'
import TagInput from './TagInput.vue'

const props = defineProps({
  reference: { type: Object, default: null }, // as listed, when editing
})
const emit = defineEmits(['close', 'saved', 'deleted'])

const r = props.reference
const title = ref(r?.title ?? '')
const kind = ref(r?.kind ?? 'clothing')
const sourceUrl = ref(r?.source_url ?? '')
const description = ref(r?.description ?? '')
const tags = ref([...(r?.tags ?? [])])
const pictures = ref({ order: [], removed: [], added: [] })
const pictureField = ref(null)
const saving = ref(false)
const error = ref('')

const tagSuggestions = computed(() => allTags(referenceLibrary.value))

const { requestClose, touched } = useModalGuard(
  () => [title.value, kind.value, sourceUrl.value, description.value, tags.value, pictures.value.removed, pictures.value.added.length, pictures.value.order],
  () => emit('close'),
)

// A picture pasted anywhere in the form joins the pictures.
function onPaste(e) {
  if (e.target.closest?.('.picture-list')) return // the field handles its own
  pictureField.value?.onPaste(e)
}

async function submit() {
  saving.value = true
  error.value = ''
  const fd = new FormData()
  fd.append('title', title.value)
  fd.append('kind', kind.value)
  fd.append('source_url', sourceUrl.value.trim())
  fd.append('description', description.value)
  fd.append('tags', JSON.stringify(tags.value))
  fd.append('order', JSON.stringify(pictures.value.order))
  fd.append('remove_pictures', JSON.stringify(pictures.value.removed))
  for (const f of pictures.value.added) fd.append('pictures', f, f.name || 'pasted.png')
  try {
    emit('saved', await saveReference(r?.id, fd))
  } catch (e) {
    error.value = e.message
  } finally {
    saving.value = false
  }
}

async function remove() {
  const ok = await askConfirm(t('Delete {name}?', { name: r.title }), {
    message: t('Its pictures go too, from every story. Designs based on it stop showing it.'),
    confirmLabel: t('Delete'),
  })
  if (!ok) return
  try {
    await deleteReference(r.id)
    emit('deleted', r.id)
  } catch (e) {
    error.value = e.message
  }
}
</script>

<style scoped>
.field-row {
  display: flex;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.grow {
  flex: 1 1 16rem;
}
</style>
