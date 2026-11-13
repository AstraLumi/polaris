<template>
  <div class="overlay" @click.self="requestClose" @pointerdown.capture="touched" @keydown.capture="touched" @input.capture="touched">
    <div class="modal is-wide glass-panel panel-solid" @paste="onPaste">
      <h2>{{ element ? $t('Edit element') : $t('New element') }}</h2>
      <form @submit.prevent="submit">
        <div class="field-row">
          <label class="field grow">
            <span>{{ $t('Name') }}</span>
            <input v-model="name" type="text" maxlength="200" required autofocus />
          </label>
          <label class="field">
            <span>{{ $t('Kind') }}</span>
            <select v-model="kind">
              <option v-for="k in ELEMENT_KINDS" :key="k.id" :value="k.id">{{ $t(k.label) }}</option>
            </select>
          </label>
        </div>
        <label class="field">
          <span>{{ $t('Description') }}</span>
          <textarea v-model="description" rows="5"></textarea>
        </label>
        <PictureListField ref="pictureField" :pictures="element?.pictures || []" @update:state="pictures = $event" />
        <TagInput v-model="tags" :suggestions="tagSuggestions" list-id="element-tags" />

        <div class="field">
          <span>{{ $t('Based on') }}</span>
          <ul v-if="picked.length" class="picked">
            <li v-for="r in picked" :key="r.id" class="picked-ref">
              <img v-if="r.pictures.length" :src="r.pictures[0].url" alt="" />
              <span class="picked-title">{{ r.title }}</span>
              <button type="button" :aria-label="$t('Remove')" :title="$t('Remove')" @click="toggleRef(r.id)">✕</button>
            </li>
          </ul>
          <button type="button" class="btn btn-ghost small" @click="pickerOpen = !pickerOpen">
            {{ pickerOpen ? $t('Close the library') : $t('Pick from the reference library…') }}
          </button>
          <div v-if="pickerOpen" class="picker">
            <p v-if="!referenceLibrary.length" class="hint">
              {{ $t('The library is empty.') }} <RouterLink to="/references?new=1" target="_blank">{{ $t('Add a reference') }}</RouterLink>
            </p>
            <template v-else>
              <input v-model="refSearch" type="search" :placeholder="$t('Search titles, notes and tags…')" :aria-label="$t('Search')" />
              <div class="picker-grid">
                <button
                  v-for="r in refOptions"
                  :key="r.id"
                  type="button"
                  class="pick"
                  :class="{ 'is-on': refIds.includes(r.id) }"
                  :aria-pressed="refIds.includes(r.id)"
                  :title="r.title"
                  @click="toggleRef(r.id)"
                >
                  <img v-if="r.pictures.length" :src="r.pictures[0].url" alt="" loading="lazy" />
                  <span class="pick-title">{{ r.title }}</span>
                </button>
              </div>
            </template>
          </div>
        </div>

        <p v-if="error" class="error-banner" role="alert">{{ error }}</p>
        <div class="modal-actions sticky-save">
          <span class="spacer"></span>
          <button type="button" class="btn btn-ghost" @click="$emit('close')">{{ $t('Cancel') }}</button>
          <button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? $t('Saving…') : $t('Save') }}</button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup>
// Adds or edits an element of a culture: a design of the story (an outfit,
// say) with its own pictures, text and tags, and the library references it
// is based on.
import { ref, computed } from 'vue'
import { useModalGuard } from '../navigation'
import { linkField } from '../linkNames'
import { culturesApi } from '../api'
import { ELEMENT_KINDS, allTags, filterItems } from '../cultures'
import { referenceLibrary } from '../referenceLibrary'
import PictureListField from './PictureListField.vue'
import TagInput from './TagInput.vue'

const props = defineProps({
  cultureId: { type: Number, required: true },
  element: { type: Object, default: null }, // when editing
  kind: { type: String, default: 'clothing' }, // for a new one
  tagSuggestions: { type: Array, default: () => [] },
})
const emit = defineEmits(['close', 'saved'])

const e = props.element
const name = ref(e?.name ?? '')
const kind = ref(e?.kind ?? props.kind)
const description = linkField(e?.description)
const tags = ref([...(e?.tags ?? [])])
const refIds = ref([...(e?.reference_ids ?? [])])
const pictures = ref({ order: [], removed: [], added: [] })
const pictureField = ref(null)
const pickerOpen = ref(false)
const refSearch = ref('')
const saving = ref(false)
const error = ref('')

const picked = computed(() => refIds.value.map((id) => referenceLibrary.value.find((r) => r.id === id)).filter(Boolean))
const refOptions = computed(() => filterItems(referenceLibrary.value, { search: refSearch.value, fields: ['title', 'description', 'tags'] }))
// Tags already used here and in the library, so the same spelling is reused.
const tagSuggestions = computed(() => allTags([{ tags: props.tagSuggestions }, ...referenceLibrary.value]))

const { requestClose, touched } = useModalGuard(
  () => [name.value, kind.value, description.value, tags.value, refIds.value, pictures.value.removed, pictures.value.added.length, pictures.value.order],
  () => emit('close'),
)

function toggleRef(id) {
  refIds.value = refIds.value.includes(id) ? refIds.value.filter((x) => x !== id) : [...refIds.value, id]
}

function onPaste(ev) {
  if (ev.target.closest?.('.picture-list')) return
  pictureField.value?.onPaste(ev)
}

async function submit() {
  saving.value = true
  error.value = ''
  const fd = new FormData()
  fd.append('kind', kind.value)
  fd.append('name', name.value)
  fd.append('description', description.value)
  fd.append('tags', JSON.stringify(tags.value))
  // References deleted from the library meanwhile drop out.
  fd.append('reference_ids', JSON.stringify(refIds.value.filter((id) => referenceLibrary.value.some((r) => r.id === id))))
  fd.append('order', JSON.stringify(pictures.value.order))
  fd.append('remove_pictures', JSON.stringify(pictures.value.removed))
  for (const f of pictures.value.added) fd.append('pictures', f, f.name || 'pasted.png')
  try {
    emit('saved', await culturesApi.saveElement(props.cultureId, e?.id, fd))
  } catch (err) {
    error.value = err.message
  } finally {
    saving.value = false
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

.picked {
  list-style: none;
  margin: 0 0 0.5rem;
  padding: 0;
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
}

.picked-ref {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.2rem 0.4rem 0.2rem 0.2rem;
  border: 1px solid var(--glass-border);
  border-radius: 8px;
  font-size: 0.82rem;
}

.picked-ref img {
  width: 1.8rem;
  height: 1.8rem;
  object-fit: cover;
  border-radius: 5px;
}

.picked-ref button {
  border: 0;
  background: none;
  color: var(--text-muted);
  cursor: pointer;
  font: inherit;
}

.picker {
  margin-top: 0.5rem;
  padding: 0.6rem;
  border: 1px solid var(--glass-border);
  border-radius: 10px;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.picker-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(6.5rem, 1fr));
  gap: 0.4rem;
  max-height: 18rem;
  overflow-y: auto;
}

.pick {
  display: flex;
  flex-direction: column;
  padding: 0;
  border: 2px solid transparent;
  border-radius: 8px;
  overflow: hidden;
  background: rgba(var(--wash), 0.04);
  color: var(--text-muted);
  font: inherit;
  cursor: pointer;
  text-align: left;
}

.pick.is-on {
  border-color: var(--accent);
  color: var(--text-primary);
}

.pick img {
  width: 100%;
  aspect-ratio: 3 / 4;
  object-fit: cover;
  display: block;
}

.pick-title {
  padding: 0.2rem 0.35rem;
  font-size: 0.72rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.hint {
  margin: 0;
  font-size: 0.8rem;
  color: var(--text-faint);
}
</style>
