<template>
  <div class="overlay" @click.self="$emit('close')">
    <div class="modal is-wide glass-panel panel-solid">
      <h2>{{ faction ? $t('Edit faction') : $t('New faction') }}</h2>

      <form @submit.prevent="submit">
        <div class="top">
          <div class="picture">
            <div class="picture-box" :style="color ? { borderColor: color } : {}">
              <img v-if="previewUrl" :src="previewUrl" alt="" />
              <span v-else>{{ (name || '?').slice(0, 1).toUpperCase() }}</span>
            </div>
            <label class="btn btn-ghost small">
              {{ $t('Picture') }}
              <input type="file" accept="image/*" hidden @change="onFile" />
            </label>
            <button v-if="previewUrl" type="button" class="btn btn-ghost small" @click="clearPicture">{{ $t('Remove') }}</button>
          </div>
          <div class="top-fields">
            <label class="field">
              <span>{{ $t('Name') }}</span>
              <input v-model="name" type="text" maxlength="120" required autofocus />
            </label>
            <div class="field-row">
              <label class="field">
                <span>{{ $t('Part of') }}</span>
                <select v-model="parentId">
                  <option :value="null">{{ $t('None') }}</option>
                  <option v-for="f in parentOptions" :key="f.id" :value="f.id">{{ f.name }}</option>
                </select>
              </label>
              <LocationSelect v-model="hqId" :label="$t('Headquarters')" :options="locationOptions" />
            </div>
            <DateField v-model="foundingDate" :label="$t('Founding date')" />
          </div>
        </div>

        <div class="field">
          <span>{{ $t('Color') }}</span>
          <ColorPicker v-model="color" />
        </div>

        <label class="field">
          <span>{{ $t('Description') }}</span>
          <textarea v-model="description" rows="3"></textarea>
        </label>

        <div class="field">
          <span>{{ $t('Members') }}</span>
          <ul v-if="members.length" class="members">
            <li v-for="(m, i) in members" :key="m.key" class="member">
              <span class="member-name">{{ nameOf(m.character_id) }}</span>
              <input v-model="m.role" type="text" maxlength="120" :placeholder="$t('Role or rank')" :aria-label="$t('Role or rank')" />
              <button type="button" class="btn btn-ghost small" :title="$t('Remove')" @click="members.splice(i, 1)">✕</button>
              <div class="member-dates">
                <DateField v-model="m.since" :label="$t('Since')" />
                <DateField v-model="m.until" :label="$t('Until')" />
              </div>
            </li>
          </ul>
          <select class="add-member" :value="''" @change="addMember($event)">
            <option value="">{{ $t('Add a member…') }}</option>
            <option v-for="c in characters" :key="c.id" :value="c.id">{{ c.name }}</option>
          </select>
        </div>

        <p v-if="error" class="error-banner">{{ error }}</p>

        <div class="modal-actions">
          <button v-if="faction" type="button" class="btn btn-danger" @click="confirmOpen = true">{{ $t('Delete') }}</button>
          <span class="spacer"></span>
          <button type="button" class="btn btn-ghost" @click="$emit('close')">{{ $t('Cancel') }}</button>
          <button type="submit" class="btn btn-primary" :disabled="saving">
            {{ saving ? $t('Saving…') : $t('Save') }}
          </button>
        </div>
      </form>
    </div>

    <Teleport to="body">
      <ConfirmDialog
        v-if="confirmOpen"
        :title="$t('Delete {name}?', { name: faction?.name })"
        :message="$t('Its members stay; they just leave the faction. Factions inside it lose their parent.')"
        :confirm-label="$t('Delete')"
        @cancel="confirmOpen = false"
        @confirm="remove"
      />
    </Teleport>
  </div>
</template>

<script setup>
// Creates or edits a faction, members included, in one save.
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { factionsApi, fetchCharacterOptions, fetchLocationOptions } from '../api'
import ColorPicker from './ColorPicker.vue'
import DateField from './DateField.vue'
import LocationSelect from './LocationSelect.vue'
import ConfirmDialog from './ConfirmDialog.vue'

const props = defineProps({
  faction: { type: Object, default: null }, // full faction (with members) when editing
  allFactions: { type: Array, default: () => [] },
})
const emit = defineEmits(['close', 'saved', 'deleted'])

const f = props.faction
const name = ref(f?.name ?? '')
const description = ref(f?.description ?? '')
const color = ref(f?.color ?? '')
const parentId = ref(f?.parent_id ?? null)
const hqId = ref(f?.hq_location_id ?? null)
const foundingDate = ref(f?.founding_date ?? '')
let keySeq = 0
const members = ref((f?.members ?? []).map((m) => ({ ...m, key: ++keySeq })))
const characters = ref([])
const locationOptions = ref([])
const saving = ref(false)
const error = ref('')
const confirmOpen = ref(false)

const pictureFile = ref(null)
const removePicture = ref(false)
let objectUrl = ''
const previewUrl = computed(() => (pictureFile.value ? objectUrl : removePicture.value ? '' : f?.picture_path || ''))

// A faction can't sit inside itself (the server checks deeper loops too).
const parentOptions = computed(() => props.allFactions.filter((x) => x.id !== f?.id))
const nameOf = (id) => characters.value.find((c) => c.id === id)?.name || members.value.find((m) => m.character_id === id)?.name || '…'

function addMember(e) {
  const id = Number(e.target.value)
  e.target.value = ''
  if (id) members.value.push({ key: ++keySeq, character_id: id, role: '', since: '', until: '' })
}

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

onBeforeUnmount(() => objectUrl && URL.revokeObjectURL(objectUrl))

async function submit() {
  saving.value = true
  error.value = ''
  const fd = new FormData()
  fd.append('name', name.value)
  fd.append('description', description.value)
  fd.append('color', color.value || '')
  fd.append('parent_id', parentId.value ?? '')
  fd.append('hq_location_id', hqId.value ?? '')
  fd.append('founding_date', foundingDate.value || '')
  fd.append(
    'members',
    JSON.stringify(members.value.map((m) => ({ character_id: m.character_id, role: m.role, since: m.since, until: m.until }))),
  )
  if (pictureFile.value) fd.append('picture', pictureFile.value)
  if (removePicture.value) fd.append('remove_picture', '1')
  try {
    emit('saved', await factionsApi.save(f?.id, fd))
  } catch (e) {
    error.value = e.message
  } finally {
    saving.value = false
  }
}

async function remove() {
  try {
    await factionsApi.remove(f.id)
    confirmOpen.value = false
    emit('deleted')
  } catch (e) {
    error.value = e.message
    confirmOpen.value = false
  }
}

onMounted(async () => {
  ;[characters.value, locationOptions.value] = await Promise.all([fetchCharacterOptions(), fetchLocationOptions()])
})
</script>

<style scoped>
.top {
  display: flex;
  gap: 1.25rem;
  align-items: flex-start;
}

.picture {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.4rem;
  flex-shrink: 0;
}

.picture-box {
  width: 6rem;
  height: 6rem;
  border-radius: 14px;
  border: 2px solid var(--glass-border);
  background: var(--accent-soft);
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
  font-family: 'Fraunces', serif;
  font-size: 2rem;
  color: var(--accent);
}

.picture-box img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.top-fields {
  flex: 1;
  min-width: 0;
}

.members {
  list-style: none;
  margin: 0 0 0.5rem;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
}

.member {
  display: grid;
  grid-template-columns: minmax(6rem, 1fr) minmax(8rem, 1.5fr) auto;
  gap: 0.5rem;
  align-items: center;
  padding-bottom: 0.6rem;
  border-bottom: 1px solid var(--glass-border);
}

.member-dates {
  grid-column: 1 / -1;
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(13rem, 1fr));
  gap: 0.5rem;
}

.member-dates :deep(.field) {
  margin-bottom: 0;
}

.member-name {
  align-self: center;
  font-weight: 600;
  font-size: 0.88rem;
}

@media (max-width: 720px) {
  .top {
    flex-direction: column;
    align-items: stretch;
  }
  .picture {
    flex-direction: row;
  }
  .member {
    grid-template-columns: 1fr auto;
  }
  .member input {
    grid-column: 1 / -1;
    grid-row: 2;
  }
}
</style>
