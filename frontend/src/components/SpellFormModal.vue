<template>
  <div class="overlay" @click.self="$emit('close')">
    <div class="modal glass-panel">
      <h2>{{ spell ? $t('Edit spell') : $t('Add a spell') }}</h2>
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
              v-for="i in BUILTIN_ICONS"
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

        <div class="field-row">
          <label class="field name-field">
            <span>{{ $t('Name') }}</span>
            <input v-model="name" type="text" required autofocus />
          </label>
          <label class="field level-field">
            <span>{{ $t('Level') }}</span>
            <input v-model.number="level" type="number" min="0" />
          </label>
        </div>

        <label class="field">
          <span>{{ $t('Source') }}</span>
          <select v-model="source">
            <option value="">{{ $t('None') }}</option>
            <optgroup v-if="catalogs.classes.length" :label="$t('Class')">
              <option v-for="c in catalogs.classes" :key="`class:${c.id}`" :value="`class:${c.id}`">
                {{ c.name }}
              </option>
            </optgroup>
            <optgroup v-if="catalogs.subclasses.length" :label="$t('Subclass')">
              <option v-for="s in catalogs.subclasses" :key="`subclass:${s.id}`" :value="`subclass:${s.id}`">
                {{ s.name }}
              </option>
            </optgroup>
            <optgroup v-if="catalogs.specializations.length" :label="$t('Specialization')">
              <option
                v-for="s in catalogs.specializations"
                :key="`specialization:${s.id}`"
                :value="`specialization:${s.id}`"
              >
                {{ s.name }}{{ s.class_name ? ` (${s.class_name})` : '' }}
              </option>
            </optgroup>
          </select>
        </label>

        <div class="field-row">
          <label class="field">
            <span>{{ $t('MP cost') }}</span>
            <input v-model="mpCost" type="number" min="0" step="1" :placeholder="$t('None')" />
          </label>
          <label class="field">
            <span>{{ $t('HP cost') }}</span>
            <input v-model="hpCost" type="number" min="0" step="1" :placeholder="$t('None')" />
          </label>
        </div>
        <p class="hint">{{ $t('Fill in one, both, or neither — a spell with no cost shows as Free.') }}</p>

        <ComboBoxField
          v-model="origin"
          :label="$t('Origin')"
          :options="catalogs.locations.map((l) => l.name)"
          list-id="spell-origins"
          :placeholder="$t('Where this spell comes from')"
        />
        <p class="hint">
          {{ $t('Pick one of the locations on your Map. To use a new place, create it on the Map page first.') }}
        </p>

        <label class="field">
          <span>{{ $t('Description') }}</span>
          <textarea v-model="description" rows="4"></textarea>
        </label>

        <p v-if="error" class="error-banner">{{ error }}</p>

        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" @click="$emit('close')">{{ $t('Cancel') }}</button>
          <button type="submit" class="btn btn-primary" :disabled="submitting">
            {{ submitting ? $t('Saving…') : $t('Save spell') }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onBeforeUnmount } from 'vue'
import { t } from '../i18n'
import { spellsApi, buildSpellFormData } from '../api'
import { BUILTIN_ICONS, BUILTIN_PREFIX } from '../builtinIcons'
import ComboBoxField from './ComboBoxField.vue'
import IconImage from './IconImage.vue'

const props = defineProps({
  catalogs: { type: Object, required: true }, // { classes, subclasses, specializations, locations }
  spell: { type: Object, default: null }, // an existing spell when editing
})
const emit = defineEmits(['close', 'saved'])

const name = ref(props.spell?.name ?? '')
const level = ref(props.spell?.level ?? 1)
const source = ref(
  props.spell?.source_type && props.spell?.source_name
    ? `${props.spell.source_type}:${props.spell.source_id}`
    : '',
)
const mpCost = ref(props.spell?.mp_cost ?? '')
const hpCost = ref(props.spell?.hp_cost ?? '')
const origin = ref(props.spell?.origin ?? '')
const description = ref(props.spell?.description ?? '')
const iconFile = ref(null)
const submitting = ref(false)
const error = ref('')

// What the form says about the icon, apart from an uploaded file:
// '' = leave as is, 'none' = remove, 'builtin:<id>' = use a built-in one.
const iconChoice = ref('')
const fileInput = ref(null)
const fileUrl = ref('')

const previewSrc = computed(() => {
  if (fileUrl.value) return fileUrl.value
  if (iconChoice.value === 'none') return ''
  if (iconChoice.value) return iconChoice.value
  return props.spell?.icon_path || ''
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
    const formData = buildSpellFormData(
      {
        name: name.value.trim(),
        level: level.value,
        source: source.value,
        mpCost: mpCost.value,
        hpCost: hpCost.value,
        origin: origin.value,
        description: description.value,
      },
      iconFile.value,
      iconChoice.value,
    )
    if (props.spell) {
      await spellsApi.update(props.spell.id, formData)
    } else {
      await spellsApi.create(formData)
    }
    emit('saved')
  } catch (err) {
    error.value = err.message || t("Couldn't save that spell. Try again.")
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

.icon-field {
  flex: 1;
  margin-bottom: 0;
}

.name-field {
  flex: 3;
}

.level-field {
  flex: 1;
}

.hint {
  font-size: 0.78rem;
  color: var(--text-faint);
  line-height: 1.5;
  margin: -0.5rem 0 1rem;
}

</style>
