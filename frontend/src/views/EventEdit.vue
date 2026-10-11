<template>
  <div class="page is-narrow">
    <BackLink :to="id ? `/events/${id}` : '/events'" :label="id ? $t('← Back to event') : $t('← Back to events')" />

    <p v-if="loadError" class="error-banner">{{ loadError }}</p>
    <p v-else-if="loading" class="loading-hint">{{ $t('Loading…') }}</p>

    <form v-else class="glass-panel form" @submit.prevent="save">
      <h1>{{ id ? $t('Edit event') : $t('New event') }}</h1>

      <template v-if="source">
        <div class="linked glass-readonly">
          <strong>{{ form.name }}</strong>
          <span>{{ fullDate(form.eventDate) || $t('No date set') }}</span>
          <p>
            {{ importedParts[0] }}<RouterLink :to="source.type === 'character' ? `/characters/${source.id}` : mapPath(source.id)">{{ source.name }}</RouterLink>{{ importedParts[1] }}
          </p>
        </div>
      </template>
      <div v-else class="field-row">
        <label class="field name-field">
          <span>{{ $t('Name') }}</span>
          <input v-model="form.name" type="text" required autofocus />
        </label>
        <DateField v-model="form.eventDate" :label="$t('Date')" class="date-col" :hint="$t('Day and month are optional.')" />
      </div>

      <LocationSelect
        v-model="form.locationId"
        :label="$t('Location (optional)')"
        :options="locationOptions"
      />

      <ChapterSelect v-model="form.chapterId" :label="$t('Chapter (optional)')" />

      <label class="field">
        <span>{{ $t('Description') }}</span>
        <textarea v-model="form.description" rows="6"></textarea>
      </label>

      <TagInput
        v-model="form.tags"
        :suggestions="tagSuggestions"
        list-id="event-tag-suggestions"
        :hint="$t('Events that share a tag are linked — tag a war\'s start and end the same way to find one from the other.')"
      />

      <PeoplePicker v-model="form.characterIds" :options="characterOptions" />

      <div class="field picture-field">
        <span>{{ $t('Picture') }}</span>
        <div class="picture-row">
          <img v-if="previewUrl" :src="previewUrl" alt="" class="preview" />
          <div class="picture-controls">
            <input type="file" accept="image/*" @change="onFileChange" />
            <button v-if="previewUrl" type="button" class="btn btn-ghost small" @click="clearPicture">
              {{ $t('Remove picture') }}
            </button>
          </div>
        </div>
      </div>

      <p v-if="saveError" class="error-banner">{{ saveError }}</p>
      <div class="actions">
        <button class="btn btn-primary" type="submit" :disabled="saving">
          {{ saving ? $t('Saving…') : id ? $t('Save changes') : $t('Create event') }}
        </button>
        <button type="button" class="btn btn-ghost cancel" @click="cancel">{{ $t('Cancel') }}</button>
      </div>
    </form>
  </div>
</template>

<script setup>
import { reactive, ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { eventsApi, buildEventFormData, fetchLocationOptions, fetchCharacterOptions } from '../api'
import { fullDate } from '../calendar'
import { t } from '../i18n'
import DateField from '../components/DateField.vue'
import LocationSelect from '../components/LocationSelect.vue'
import TagInput from '../components/TagInput.vue'
import PeoplePicker from '../components/PeoplePicker.vue'
import ChapterSelect from '../components/ChapterSelect.vue'
import BackLink from '../components/BackLink.vue'
import { useUnsavedGuard, useSaveShortcut, mapPath } from '../navigation'

// No id means "new event".
const props = defineProps({ id: { type: String, default: '' } })
const route = useRoute()
const router = useRouter()

const loading = ref(true)
const loadError = ref('')
const saving = ref(false)
const saveError = ref('')
const source = ref(null)
const locationOptions = ref([])
const characterOptions = ref([])
const tagSuggestions = ref([])

const form = reactive({
  name: '',
  eventDate: '',
  description: '',
  locationId: null,
  chapterId: null,
  tags: [],
  characterIds: [],
})

const pictureFile = ref(null)
const existingPicture = ref('')
const removePicture = ref(false)
let objectUrl = ''

// One whole sentence with a {link} slot, split so the link can sit inside it.
const importedParts = computed(() => t('This event is imported from {link}, so its name and date are edited there. Everything below is yours.', { link: '\u0001' }).split('\u0001'))

const previewUrl = computed(() => {
  if (pictureFile.value) return objectUrl
  return removePicture.value ? '' : existingPicture.value
})

function onFileChange(e) {
  const file = e.target.files[0] || null
  if (objectUrl) URL.revokeObjectURL(objectUrl)
  pictureFile.value = file
  objectUrl = file ? URL.createObjectURL(file) : ''
  if (file) removePicture.value = false
}

function clearPicture() {
  if (objectUrl) URL.revokeObjectURL(objectUrl)
  objectUrl = ''
  pictureFile.value = null
  removePicture.value = true
}

onBeforeUnmount(() => objectUrl && URL.revokeObjectURL(objectUrl))

// What the form looked like when it was loaded, to tell whether it changed.
let snapshot = ''
let saved = false
const formState = () => JSON.stringify(form)
const dirty = () =>
  !loading.value && !loadError.value && !saved && (formState() !== snapshot || !!pictureFile.value || removePicture.value)
useUnsavedGuard(dirty)
useSaveShortcut(() => !saving.value && !loading.value && !loadError.value && save())

// Cancel returns to wherever the editor was opened from, like the back link.
function cancel() {
  if (router.options.history.state.back) router.back()
  else router.push(props.id ? `/events/${props.id}` : '/events')
}

async function save() {
  saving.value = true
  saveError.value = ''
  try {
    const body = buildEventFormData(form, pictureFile.value, removePicture.value)
    const result = props.id ? await eventsApi.update(props.id, body) : await eventsApi.create(body)
    saved = true
    // Leave the edit page out of history: back from the event goes wherever
    // you were before editing it.
    const target = `/events/${result.id}`
    if (props.id && router.options.history.state.back === target) router.back()
    else router.replace(target)
  } catch (err) {
    saveError.value = err.message || t("Couldn't save. Try again.")
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  // Pickers are best-effort: the form still works if one fails to load.
  const [locs, chars, tags] = await Promise.all([
    fetchLocationOptions().catch(() => []),
    fetchCharacterOptions().catch(() => []),
    eventsApi.tags().catch(() => []),
  ])
  locationOptions.value = locs
  characterOptions.value = chars
  tagSuggestions.value = tags.map((t) => t.tag)

  if (!props.id) {
    // /events/new?tag=a&tag=b pre-fills tags (used for "new event with these tags").
    const q = route.query.tag
    form.tags = (Array.isArray(q) ? q : q ? [q] : []).filter((t) => typeof t === 'string')
    if (Number(route.query.chapter)) form.chapterId = Number(route.query.chapter)
    // ?date=DD-MM-YYYY, from the calendar's "New event".
    if (typeof route.query.date === 'string') form.eventDate = route.query.date
    snapshot = formState()
    loading.value = false
    return
  }
  try {
    const e = await eventsApi.get(props.id)
    form.name = e.name
    form.eventDate = e.event_date
    form.description = e.description
    form.locationId = e.location_id
    form.chapterId = e.chapter_id ?? null
    form.tags = [...e.tags]
    form.characterIds = e.people.map((p) => p.id)
    existingPicture.value = e.picture_path
    source.value = e.source
    snapshot = formState()
  } catch (err) {
    loadError.value = t("Couldn't find that event.")
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.form {
  padding: 1.75rem;
}

.form h1 {
  font-family: 'Fraunces', serif;
  font-weight: 500;
  font-size: 1.6rem;
  margin: 0 0 1.25rem;
}

.name-field {
  flex: 3;
}

.date-col {
  flex: 2;
}

.linked {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
  padding: 0.9rem 1rem;
  margin-bottom: 1rem;
  border: 1px dashed var(--glass-border);
  border-radius: 10px;
}

.linked strong {
  font-size: 1.05rem;
}

.linked span {
  font-size: 0.82rem;
  color: var(--text-muted);
}

.linked p {
  margin: 0.4rem 0 0;
  font-size: 0.78rem;
  color: var(--text-faint);
  line-height: 1.5;
}

.linked a {
  color: var(--accent);
}

.picture-row {
  display: flex;
  gap: 1rem;
  align-items: center;
}

.preview {
  width: 120px;
  height: 90px;
  object-fit: cover;
  border-radius: 8px;
  flex-shrink: 0;
}

.picture-controls {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.5rem;
  min-width: 0;
  flex: 1;
}

.actions {
  display: flex;
  gap: 0.75rem;
  margin-top: 1.25rem;
}

.cancel {
  text-decoration: none;
}
</style>
