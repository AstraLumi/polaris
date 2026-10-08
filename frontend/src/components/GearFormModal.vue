<template>
  <div class="overlay" @click.self="$emit('close')">
    <div class="modal glass-panel is-wide">
      <h2>{{ gear ? $t('Edit gear') : $t('Add gear') }}</h2>
      <form @submit.prevent="submit">
        <div class="icon-row">
          <div class="icon-preview">
            <IconImage :src="previewSrc" :name="name" />
          </div>
          <label class="field icon-field">
            <span>{{ $t('Icon') }}</span>
            <input ref="fileInput" type="file" accept="image/*" @change="onFileChange" />
          </label>
          <button v-if="hasOwnIcon" type="button" class="btn btn-ghost small" @click="clearIcon">
            {{ $t('Remove') }}
          </button>
        </div>
        <div class="field">
          <span>{{ $t('No icon of your own yet? Pick a built-in one') }}</span>
          <div class="icon-grid" role="radiogroup" :aria-label="$t('Built-in icons')">
            <button
              v-for="i in GEAR_ICONS"
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
          <small class="hint">{{ $t("With no icon chosen, the gear shows its slot's icon.") }}</small>
        </div>

        <div class="field-row">
          <label class="field name-field">
            <span>{{ $t('Name') }}</span>
            <input v-model="name" type="text" required />
          </label>
          <label class="field">
            <span>{{ $t('Slot') }}</span>
            <select v-model="slot" required>
              <option value="" disabled>{{ $t('Choose a slot') }}</option>
              <option v-for="s in GEAR_SLOT_TYPES" :key="s.id" :value="s.id">{{ $t(s.label) }}</option>
            </select>
          </label>
          <label class="field weight-field">
            <span>{{ $t('Weight (kg)') }}</span>
            <input v-model="weight" type="number" min="0" step="0.01" placeholder="0" />
          </label>
        </div>
        <p class="hint slot-hint">
          {{ $t('A glove fits either Glove slot and a ring either Ring slot.') }}
        </p>

        <p class="hint">
          {{ $t('Leave a stat blank to leave it untouched. Every value here is a flat bonus (or penalty, with a minus sign) applied whenever this gear is equipped, whatever the character\'s level.') }}
        </p>
        <ModifierForm v-model="modifiers" />

        <p v-if="error" class="error-banner">{{ error }}</p>
        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="$emit('close')">{{ $t('Cancel') }}</button>
          <button type="submit" class="btn btn-primary" :disabled="submitting || !name.trim() || !slot">
            {{ submitting ? $t('Saving…') : $t('Save gear') }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onBeforeUnmount } from 'vue'
import { t } from '../i18n'
import IconImage from './IconImage.vue'
import ModifierForm from './ModifierForm.vue'
import { GEAR_ICONS, BUILTIN_PREFIX } from '../builtinIcons'
import { GEAR_SLOT_TYPES, gearIconSrc } from '../gear'
import { gearApi, buildGearFormData } from '../api'

const props = defineProps({
  gear: { type: Object, default: null }, // null = adding
  defaultSlot: { type: String, default: '' },
})
const emit = defineEmits(['close', 'saved'])

const name = ref(props.gear?.name ?? '')
const slot = ref(props.gear?.slot ?? props.defaultSlot ?? '')
const weight = ref(props.gear?.weight ?? '')
const modifiers = ref({ ...(props.gear?.modifiers ?? {}) })
const iconFile = ref(null)
const fileUrl = ref('')
const fileInput = ref(null)
// '' = leave as is, 'none' = remove, 'builtin:<id>' = a built-in one.
const iconChoice = ref('')
const submitting = ref(false)
const error = ref('')

// What the icon will be after saving: the chosen/uploaded one, the current
// one, or (with none) the slot's own icon.
const ownSrc = computed(() => {
  if (fileUrl.value) return fileUrl.value
  if (iconChoice.value === 'none') return ''
  if (iconChoice.value) return iconChoice.value
  return props.gear?.icon_path || ''
})
const hasOwnIcon = computed(() => !!ownSrc.value)
const previewSrc = computed(() => ownSrc.value || gearIconSrc({ slot: slot.value }))

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
  if (!name.value.trim() || !slot.value) return
  submitting.value = true
  error.value = ''
  try {
    const fd = buildGearFormData(
      { name: name.value.trim(), slot: slot.value, weight: weight.value, modifiers: modifiers.value },
      iconFile.value,
      iconChoice.value,
    )
    const saved = props.gear ? await gearApi.update(props.gear.id, fd) : await gearApi.create(fd)
    emit('saved', saved)
  } catch (err) {
    error.value = err.message || t("Couldn't save that gear. Try again.")
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

.name-field {
  flex: 3;
}

.weight-field {
  flex: 1.2;
}

.hint {
  display: block;
  margin: 0.4rem 0 1rem;
  font-size: 0.78rem;
  line-height: 1.45;
  color: var(--text-faint);
}

.slot-hint {
  margin-top: -0.4rem;
}
</style>
