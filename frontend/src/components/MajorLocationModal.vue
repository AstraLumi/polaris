<template>
  <div class="overlay" @click.self="requestClose" @pointerdown.capture="touched" @keydown.capture="touched" @input.capture="touched">
    <div class="modal glass-panel panel-solid">
      <h2>{{ location ? $t('Edit major location') : $t('New major location') }}</h2>

      <form @submit.prevent="submit">
        <div class="field-row">
          <label class="field name-field">
            <span>{{ $t('Name') }}</span>
            <input v-model="name" type="text" :required="realm !== 'unknown'" :placeholder="realm === 'unknown' ? '???' : ''" autofocus />
          </label>
          <DateField v-model="foundingDate" :label="$t('Founding date')" class="date-field" />
        </div>

        <div class="field">
          <span>{{ $t('What it is') }}</span>
          <div class="realms" role="radiogroup" :aria-label="$t('What it is')">
            <label v-for="r in REALMS" :key="r.id" class="realm" :class="{ 'is-on': realm === r.id }">
              <input :checked="realm === r.id" type="radio" :value="r.id" @change="pickRealm(r.id)" />
              <strong>{{ $t(r.label) }}</strong>
              <span>{{ $t(r.blurb) }}</span>
            </label>
          </div>
        </div>

        <div v-if="realm !== 'unknown'" class="field">
          <span>{{ $t('Color') }}</span>
          <ColorPicker v-model="color" :taken-colors="otherColors" />
          <p class="hint">
            {{ $t('A colored location has borders: painting with it claims hexes for that color, and a hex belongs to one of them at a time. Pick no color (the dashed swatch) for a location that sits inside one — painting with it marks its hexes without changing whose they are.') }}
          </p>
        </div>

        <label class="field">
          <span>{{ $t('Description') }}</span>
          <textarea v-model="description" rows="4"></textarea>
        </label>

        <p v-if="error" class="error-banner">{{ error }}</p>

        <div class="modal-actions">
          <button v-if="location" type="button" class="btn btn-danger" @click="confirmOpen = true">
            {{ $t('Delete') }}
          </button>
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
        :title="$t('Delete {name}?', { name: location?.name })"
        :message="$t('Its hexes are released, and any locations that belonged to it go back to automatic.')"
        :confirm-label="$t('Delete')"
        @cancel="confirmOpen = false"
        @confirm="remove"
      />
    </Teleport>
  </div>
</template>

<script setup>
import { t } from '../i18n'
import { useModalGuard } from '../navigation'
import { linkField } from '../linkNames'
import { ref, computed } from 'vue'
import { mapApi } from '../api'
import ColorPicker from './ColorPicker.vue'
import DateField from './DateField.vue'
import ConfirmDialog from './ConfirmDialog.vue'
import { REALMS, UNKNOWN_COLOR } from '../worldMap'

const props = defineProps({
  location: { type: Object, default: null }, // an existing major location when editing
  takenColors: { type: Array, default: () => [] }, // every kingdom color in use
})
const emit = defineEmits(['close', 'saved', 'deleted'])

const name = ref(props.location?.name ?? '')
const foundingDate = ref(props.location?.founding_date ?? '')
const description = linkField(props.location?.description)
const color = ref(props.location?.color ?? '')
const realm = ref(props.location?.color ? props.location.realm || 'kingdom' : 'kingdom')

// Unknown land is always grey; leaving it, a colour of its own is picked.
function pickRealm(id) {
  if (id === 'unknown') color.value = UNKNOWN_COLOR
  else if (realm.value === 'unknown') color.value = ''
  realm.value = id
}
const saving = ref(false)
const error = ref('')
// A stray click outside or Esc asks before dropping what was typed.
const { requestClose, touched } = useModalGuard(() => [name.value, foundingDate.value, description.value, color.value, realm.value], () => emit('close'))
const confirmOpen = ref(false)

// Your own current color stays pickable; everyone else's is taken.
const otherColors = computed(() => props.takenColors.filter((c) => c !== props.location?.color))

async function submit() {
  if (!name.value.trim() && realm.value !== 'unknown') return
  saving.value = true
  error.value = ''
  const payload = {
    kind: 'major',
    name: name.value.trim(),
    color: color.value,
    realm: realm.value,
    founding_date: foundingDate.value,
    description: description.value,
  }
  try {
    const saved = props.location
      ? await mapApi.updateLocation(props.location.id, payload)
      : await mapApi.createLocation(payload)
    emit('saved', saved)
  } catch (err) {
    error.value = err.message || t("Couldn't save that location.")
  } finally {
    saving.value = false
  }
}

async function remove() {
  confirmOpen.value = false
  try {
    await mapApi.deleteLocation(props.location.id)
    emit('deleted')
  } catch (err) {
    error.value = err.message || t("Couldn't delete that location.")
  }
}
</script>

<style scoped>
.name-field {
  flex: 3;
}

.date-field {
  flex: 2;
}

.hint {
  font-size: 0.78rem;
  color: var(--text-faint);
  line-height: 1.5;
  margin: 0.7rem 0 0;
}

.spacer {
  flex: 1;
}

.realms {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 0.5rem;
}

.realm {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
  padding: 0.55rem 0.7rem;
  border: 1px solid var(--glass-border);
  border-radius: 10px;
  cursor: pointer;
}

.realm:focus-within {
  outline: 2px solid var(--accent-soft);
}

.realm.is-on {
  border-color: var(--accent);
  background: var(--accent-soft);
}

.realm input {
  position: absolute;
  opacity: 0;
  pointer-events: none;
}

.realm strong {
  font-size: 0.85rem;
  color: var(--text-primary);
}

.realm span {
  font-size: 0.74rem;
  line-height: 1.4;
  color: var(--text-faint);
}
</style>
