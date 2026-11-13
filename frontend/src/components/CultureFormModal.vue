<template>
  <div class="overlay" @click.self="requestClose" @pointerdown.capture="touched" @keydown.capture="touched" @input.capture="touched">
    <div class="modal is-wide glass-panel panel-solid">
      <h2>{{ culture ? $t('Edit culture') : $t('New culture') }}</h2>
      <form @submit.prevent="submit">
        <div class="top">
          <div class="picture">
            <div class="picture-box">
              <img v-if="previewUrl" :src="previewUrl" alt="" />
              <span v-else>{{ (name || '?').slice(0, 1).toUpperCase() }}</span>
            </div>
            <FilePick :label="$t('Picture')" @change="onFile" />
            <button v-if="previewUrl" type="button" class="btn btn-ghost small" @click="clearPicture">{{ $t('Remove') }}</button>
          </div>
          <div class="top-fields">
            <label class="field">
              <span>{{ $t('Name') }}</span>
              <input v-model="name" type="text" maxlength="120" required autofocus />
            </label>
            <div class="field">
              <span>{{ $t('Kingdoms') }}</span>
              <p v-if="!kingdoms.length" class="hint">{{ $t('No kingdoms on the Map yet.') }}</p>
              <div v-else class="kingdoms">
                <label v-for="k in kingdoms" :key="k.id" class="kingdom" :class="{ 'is-on': kingdomIds.includes(k.id) }">
                  <input type="checkbox" :value="k.id" v-model="kingdomIds" />
                  <span class="swatch" :style="{ background: k.color }"></span>
                  {{ k.name }}
                </label>
              </div>
              <p class="hint">{{ $t('A culture can span several kingdoms, or none (nomads, a diaspora).') }}</p>
            </div>
          </div>
        </div>
        <label class="field">
          <span>{{ $t('Description') }}</span>
          <textarea v-model="description" rows="4"></textarea>
        </label>
        <p v-if="error" class="error-banner" role="alert">{{ error }}</p>
        <div class="modal-actions">
          <button v-if="culture" type="button" class="btn btn-danger" @click="remove">{{ $t('Delete') }}</button>
          <span class="spacer"></span>
          <button type="button" class="btn btn-ghost" @click="$emit('close')">{{ $t('Cancel') }}</button>
          <button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? $t('Saving…') : $t('Save') }}</button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup>
// Adds or edits a culture: its name, picture, description and kingdoms.
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { t } from '../i18n'
import { useModalGuard } from '../navigation'
import { askConfirm } from '../confirm'
import { linkField } from '../linkNames'
import { culturesApi, fetchLocationOptions } from '../api'
import FilePick from './FilePick.vue'

const props = defineProps({
  culture: { type: Object, default: null }, // as listed, when editing
})
const emit = defineEmits(['close', 'saved', 'deleted'])

const c = props.culture
const name = ref(c?.name ?? '')
const description = linkField(c?.description)
const kingdomIds = ref((c?.kingdoms ?? []).map((k) => k.id))
const options = ref([])
const saving = ref(false)
const error = ref('')

// A kingdom is a coloured major location of the kingdom realm (as
// LocationSelect reads them).
const kingdoms = computed(() =>
  options.value
    .filter((o) => o.kind === 'major' && o.color && (o.realm || 'kingdom') === 'kingdom')
    .sort((a, b) => a.name.localeCompare(b.name)),
)

const pictureFile = ref(null)
const removePicture = ref(false)
let objectUrl = ''
const previewUrl = computed(() => (pictureFile.value ? objectUrl : removePicture.value ? '' : c?.picture_path || ''))

const { requestClose, touched } = useModalGuard(
  () => [name.value, description.value, kingdomIds.value, pictureFile.value, removePicture.value],
  () => emit('close'),
)

function onFile(e) {
  const file = e.target.files[0] || null
  e.target.value = ''
  if (!file) return
  if (objectUrl) URL.revokeObjectURL(objectUrl)
  pictureFile.value = file
  objectUrl = URL.createObjectURL(file)
  removePicture.value = false
}

function clearPicture() {
  if (objectUrl) URL.revokeObjectURL(objectUrl)
  objectUrl = ''
  pictureFile.value = null
  removePicture.value = true
}

onMounted(async () => (options.value = await fetchLocationOptions()))
onBeforeUnmount(() => objectUrl && URL.revokeObjectURL(objectUrl))

async function submit() {
  saving.value = true
  error.value = ''
  const fd = new FormData()
  fd.append('name', name.value)
  fd.append('description', description.value)
  fd.append('kingdom_ids', JSON.stringify(kingdomIds.value))
  if (pictureFile.value) fd.append('picture', pictureFile.value)
  if (removePicture.value) fd.append('remove_picture', '1')
  try {
    emit('saved', await culturesApi.save(c?.id, fd))
  } catch (e) {
    error.value = e.message
  } finally {
    saving.value = false
  }
}

async function remove() {
  const ok = await askConfirm(t('Delete {name}?', { name: c.name }), {
    message: t('Its elements, their pictures and its wiki article go too. The references they were based on stay in the library.'),
    confirmLabel: t('Delete'),
  })
  if (!ok) return
  try {
    await culturesApi.remove(c.id)
    emit('deleted', c.id)
  } catch (e) {
    error.value = e.message
  }
}
</script>

<style scoped>
.top {
  display: flex;
  gap: 1rem;
  flex-wrap: wrap;
}

.picture {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.4rem;
  width: 9rem;
}

.picture-box {
  width: 8rem;
  height: 8rem;
  border-radius: 12px;
  overflow: hidden;
  background: var(--accent-soft);
  color: var(--accent);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 2rem;
  font-weight: 700;
}

.picture-box img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.top-fields {
  flex: 1 1 18rem;
  min-width: 0;
}

.kingdoms {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

.kingdom {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.25rem 0.65rem;
  border: 1px solid var(--glass-border);
  border-radius: 999px;
  font-size: 0.85rem;
  color: var(--text-muted);
  cursor: pointer;
}

.kingdom.is-on {
  border-color: var(--accent);
  background: var(--accent-soft);
  color: var(--text-primary);
}

.kingdom input {
  margin: 0;
}

.swatch {
  width: 0.7rem;
  height: 0.7rem;
  border-radius: 50%;
}

.hint {
  margin: 0.3rem 0 0;
  font-size: 0.76rem;
  color: var(--text-faint);
}
</style>
