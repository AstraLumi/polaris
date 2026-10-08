<template>
  <div class="page">
    <RouterLink :to="`/characters/${id}/versions`" class="back-link">{{ $t('← Back to versions') }}</RouterLink>

    <p v-if="loadError" class="error-banner">{{ loadError }}</p>
    <p v-else-if="loading" class="loading-hint">{{ $t('Loading…') }}</p>

    <template v-else>
      <header class="identity-form glass-panel">
        <div class="portrait-upload">
          <div class="portrait">
            <img v-if="previewUrl" :src="previewUrl" :alt="form.name" />
            <span v-else class="portrait-fallback">{{ initials }}</span>
          </div>
          <label class="picture-input">
            <span>{{ $t('Change picture') }}</span>
            <input type="file" accept="image/*" @change="onFileChange" />
          </label>
        </div>

        <div class="identity-fields">
          <div class="field-row">
            <label class="field">
              <span>{{ $t('Name') }}</span>
              <input v-model="form.name" type="text" required />
            </label>
            <label class="field">
              <span>{{ $t('Nickname') }}</span>
              <input v-model="form.nickname" type="text" />
            </label>
          </div>
          <div class="field-row">
            <label class="field">
              <span>{{ $t('Level') }}</span>
              <input v-model.number="form.level" type="number" min="1" />
            </label>
            <ComboBoxField
              v-model="form.class"
              :label="$t('Class')"
              :options="catalogs.classes.map((c) => c.name)"
              list-id="edit-classes"
            />
          </div>
          <div class="field-row">
            <ComboBoxField
              v-model="form.subclass"
              :label="$t('Subclass')"
              :options="availableSubclasses.map((c) => c.name)"
              list-id="edit-subclasses"
            />
            <ComboBoxField
              v-model="form.specialization"
              :label="$t('Specialization')"
              :options="catalogs.specializations.map((c) => c.name)"
              list-id="edit-specializations"
            />
          </div>
          <div class="field-row">
            <DateField v-model="form.version_date" :label="$t('Story date')" />
            <label class="field">
              <span>{{ $t('Volume / Chapter / Page') }}</span>
              <input v-model="form.version_reference" type="text" :placeholder="$t('Vol. 2 - Ch. 5 - Pg. 12')" />
            </label>
          </div>
        </div>
      </header>

      <div class="tab-bar">
        <button class="tab-button" :class="{ 'is-active': tab === 'story' }" @click="tab = 'story'">
          {{ $t('Story') }}
        </button>
        <button class="tab-button" :class="{ 'is-active': tab === 'build' }" @click="tab = 'build'">
          {{ $t('Build') }}
        </button>
        <button class="tab-button" :class="{ 'is-active': tab === 'gear' }" @click="tab = 'gear'">
          {{ $t('Gear') }}
        </button>
        <button class="tab-button" :class="{ 'is-active': tab === 'spells' }" @click="tab = 'spells'">
          {{ $t('Spells') }}
        </button>
      </div>

      <section v-if="tab === 'story'" class="story-tab glass-panel">
        <div class="field-grid">
          <label class="field">
            <span>{{ $t('Gender') }}</span>
            <input v-model="form.gender" type="text" />
          </label>
          <ComboBoxField
            v-model="form.race"
            :label="$t('Race')"
            :options="catalogs.races.map((c) => c.name)"
            list-id="edit-races"
          />
          <label class="field">
            <span>{{ $t('Height (cm)') }}</span>
            <input v-model="form.height" type="number" step="0.1" />
          </label>
          <label class="field">
            <span>{{ $t('Weight (kg)') }}</span>
            <input v-model="form.weight" type="number" step="0.1" />
          </label>
          <ComboBoxField
            v-model="form.body_type"
            :label="$t('Body type')"
            :options="catalogs.bodyTypes.map((c) => c.name)"
            list-id="edit-body-types"
          />
          <label class="field">
            <span>{{ $t('Age') }}</span>
            <input v-model.number="form.age" type="number" step="0.1" min="0" />
          </label>
          <label class="field">
            <span>{{ $t('Blood type') }}</span>
            <input v-model="form.blood_type" type="text" />
          </label>
          <LocationSelect
            v-model="form.born_in_location_id"
            :label="$t('Born in')"
            :options="locationOptions"
            :legacy-text="form.born_in"
          />
          <LocationSelect
            v-model="form.nation_location_id"
            :label="$t('Nation')"
            :options="locationOptions"
            :legacy-text="form.nation"
            kingdom-only
          />
          <DateField v-model="form.birth_date" :label="$t('Birth date (in-story)')" class="span-2" />
          <DateField v-model="form.human_birth_date" :label="$t('Human birth date')" class="span-2" />
          <label class="field">
            <span>{{ $t('Deaths') }}</span>
            <input v-model.number="form.deaths" type="number" min="0" />
          </label>
        </div>
        <div class="long-fields">
          <label class="field long">
            <span>{{ $t('Description') }}</span>
            <textarea v-model="form.description" rows="3"></textarea>
          </label>
          <label class="field long">
            <span>{{ $t('Bio') }}</span>
            <textarea v-model="form.bio" rows="4"></textarea>
          </label>
          <label class="field long">
            <span>{{ $t('Speech mannerisms') }}</span>
            <textarea v-model="form.speech_mannerisms" rows="3"></textarea>
          </label>
        </div>
      </section>

      <section v-if="tab === 'build'" class="build-tab glass-panel">
        <BuildStatsPanel
          v-model="buildStats"
          :special-bases="specialBases"
          :computed="previewComputed"
          :level="form.level"
          editable
          @update:special-bases="mergeSpecialBases"
        />
        <p class="preview-note">
          {{ $t('Base Stats, Special Stats, and Special Defenses above update a moment after you stop typing — they include bonuses from Class/Subclass/Specialization/Race/Body Type and equipped Gear.') }}
        </p>
      </section>

      <section v-else-if="tab === 'gear'" class="gear-tab-wrap glass-panel">
        <GearTab v-model="equippedGear" editable :carry-limit="previewComputed.special?.carry_limit || 0" />
        <p class="preview-note">{{ $t('Gear changes are saved with the Save button below, like everything else.') }}</p>
      </section>

      <section v-else-if="tab === 'spells'" class="gear-tab-wrap glass-panel">
        <SpellsTab v-model="assignedSpells" editable />
        <p class="preview-note">{{ $t('Spell changes are saved with the Save button below, like everything else.') }}</p>
      </section>

      <p v-if="saveError" class="error-banner save-error">{{ saveError }}</p>

      <div class="save-bar">
        <button class="btn btn-primary" :disabled="saving" @click="save">
          {{ saving ? $t('Saving…') : $t('Save changes') }}
        </button>
        <span v-if="savedAt" class="saved-hint">{{ $t('Saved.') }}</span>
      </div>
    </template>
  </div>
</template>

<script setup>
import { reactive, ref, computed, watch, onMounted } from 'vue'
import { t } from '../i18n'
import { fetchVersion, updateVersion, buildVersionFormData, fetchCatalogs, fetchLocationOptions, fetchComputePreview, SPECIAL_BASE_KEYS } from '../api'
import BuildStatsPanel from '../components/BuildStatsPanel.vue'
import ComboBoxField from '../components/ComboBoxField.vue'
import DateField from '../components/DateField.vue'
import LocationSelect from '../components/LocationSelect.vue'
import GearTab from '../components/GearTab.vue'
import SpellsTab from '../components/SpellsTab.vue'

const props = defineProps({
  id: { type: String, required: true },
  versionId: { type: String, required: true },
})

const loading = ref(true)
const loadError = ref('')
const saving = ref(false)
const saveError = ref('')
const savedAt = ref(false)
const tab = ref('story')
const pictureFile = ref(null)
const existingPicturePath = ref('')
const assignedSpells = ref([])
const equippedGear = ref({})
const catalogs = ref({ classes: [], subclasses: [], specializations: [], races: [], bodyTypes: [] })
const locationOptions = ref([])

const form = reactive({
  version_date: '',
  version_reference: '',
  name: '',
  nickname: '',
  level: 1,
  class: '',
  subclass: '',
  specialization: '',
  gender: '',
  race: '',
  height: '',
  weight: '',
  body_type: '',
  age: '',
  human_birth_date: '',
  blood_type: '',
  born_in: '',
  nation: '',
  born_in_location_id: null,
  nation_location_id: null,
  birth_date: '',
  deaths: 0,
  description: '',
  bio: '',
  speech_mannerisms: '',
  vit: 0,
  def: 0,
  res: 0,
  str: 0,
  dex: 0,
  intel: 0,
  wis: 0,
  agl: 0,
})

const buildStats = computed({
  get: () => ({
    vit: form.vit, def: form.def, res: form.res, str: form.str,
    dex: form.dex, intel: form.intel, wis: form.wis, agl: form.agl,
  }),
  set: (next) => {
    form.vit = next.vit
    form.def = next.def
    form.res = next.res
    form.str = next.str
    form.dex = next.dex
    form.intel = next.intel
    form.wis = next.wis
    form.agl = next.agl
  },
})

const specialBases = reactive(
  Object.fromEntries(SPECIAL_BASE_KEYS.map((key) => [key, 0])),
)

function mergeSpecialBases(next) {
  Object.assign(specialBases, next)
}

// Recomputes the Build tab's preview from the server, debounced so rapid
// stat-button clicks don't fire a request per click. This replaced a
// client-side JS mirror of the formulas that never included
// stat_modifiers — which was the bug where Race/Class/etc bonuses only
// showed on the View page, never here.
const previewComputed = ref({
  base: {}, special: {}, elemental: {}, lifeskills: {},
})
let previewTimer = null

function schedulePreview() {
  clearTimeout(previewTimer)
  previewTimer = setTimeout(refreshPreview, 250)
}

async function refreshPreview() {
  try {
    previewComputed.value = await fetchComputePreview({
      level: form.level || 1,
      age: form.age === '' || form.age === null ? 18 : Number(form.age),
      height: form.height === '' || form.height === null ? 0 : Number(form.height),
      weight: form.weight === '' || form.weight === null ? 0 : Number(form.weight),
      class: form.class,
      subclass: form.subclass,
      specialization: form.specialization,
      race: form.race,
      body_type: form.body_type,
      vit: form.vit, def: form.def, res: form.res, str: form.str,
      dex: form.dex, intel: form.intel, wis: form.wis, agl: form.agl,
      special_bases: { ...specialBases },
      gear_ids: Object.values(equippedGear.value).map((g) => g.id),
    })
  } catch (err) {
    // Keep showing the last known-good preview rather than blanking it.
  }
}

watch(form, schedulePreview, { deep: true })
watch(specialBases, schedulePreview, { deep: true })
watch(equippedGear, schedulePreview)

const availableSubclasses = computed(() => {
  const typedClass = (form.class || '').trim().toLowerCase()
  return catalogs.value.subclasses.filter((s) => {
    const names = s.class_names || []
    return names.length === 0 || names.some((n) => n.trim().toLowerCase() === typedClass)
  })
})

const initials = computed(() =>
  (form.name || '')
    .split(' ')
    .filter(Boolean)
    .map((p) => p[0])
    .join('')
    .slice(0, 2)
    .toUpperCase(),
)

const previewUrl = computed(() => {
  if (pictureFile.value) return URL.createObjectURL(pictureFile.value)
  return existingPicturePath.value || ''
})

function onFileChange(e) {
  pictureFile.value = e.target.files[0] || null
}

async function load() {
  loading.value = true
  loadError.value = ''
  try {
    const data = await fetchVersion(props.versionId)
    Object.assign(form, {
      version_date: data.version_date,
      version_reference: data.version_reference,
      name: data.name,
      nickname: data.nickname,
      level: data.level,
      class: data.class_name,
      subclass: data.subclass_name,
      specialization: data.specialization_name,
      gender: data.story.gender,
      race: data.story.race_name,
      height: data.story.height ?? '',
      weight: data.story.weight ?? '',
      body_type: data.story.body_type_name,
      age: data.story.age ?? '',
      human_birth_date: data.story.human_birth_date,
      blood_type: data.story.blood_type,
      born_in: data.story.born_in,
      nation: data.story.nation,
      born_in_location_id: data.story.born_in_location_id,
      nation_location_id: data.story.nation_location_id,
      birth_date: data.story.birth_date,
      deaths: data.story.deaths,
      description: data.story.description,
      bio: data.story.bio,
      speech_mannerisms: data.story.speech_mannerisms,
      vit: data.build.vit,
      def: data.build.def,
      res: data.build.res,
      str: data.build.str,
      dex: data.build.dex,
      intel: data.build.intel,
      wis: data.build.wis,
      agl: data.build.agl,
    })
    Object.assign(specialBases, Object.fromEntries(SPECIAL_BASE_KEYS.map((key) => [key, 0])))
    Object.assign(specialBases, data.special_bases || {})
    previewComputed.value = data.computed
    assignedSpells.value = data.spells || []
    equippedGear.value = data.gear || {}
    existingPicturePath.value = data.picture_path
  } catch (err) {
    loadError.value = t("Couldn't load this version. Try refreshing.")
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  saveError.value = ''
  savedAt.value = false
  try {
    const formData = buildVersionFormData(
      form,
      specialBases,
      pictureFile.value,
      assignedSpells.value.map((s) => s.id),
      equippedGear.value,
    )
    const updated = await updateVersion(props.versionId, formData)
    existingPicturePath.value = updated.picture_path
    previewComputed.value = updated.computed
    assignedSpells.value = updated.spells || []
    equippedGear.value = updated.gear || {}
    pictureFile.value = null
    savedAt.value = true
    setTimeout(() => (savedAt.value = false), 2500)
  } catch (err) {
    saveError.value = err.message || t("Couldn't save. Try again.")
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  load()
  catalogs.value = await fetchCatalogs()
  locationOptions.value = await fetchLocationOptions()
})
</script>

<style scoped>
.identity-form {
  display: flex;
  gap: 1.5rem;
  padding: 1.5rem;
  margin-bottom: 1.75rem;
}

.portrait-upload {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.6rem;
  flex-shrink: 0;
}

.portrait {
  width: 84px;
  height: 84px;
  border-radius: 14px;
  background: var(--accent-soft);
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
}

.portrait img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.portrait-fallback {
  font-family: 'Fraunces', serif;
  color: var(--accent);
  font-size: 1.5rem;
}

.picture-input {
  font-size: 0.72rem;
  color: var(--text-faint);
  text-align: center;
  cursor: pointer;
}

.picture-input input {
  display: block;
  margin-top: 0.3rem;
  width: 130px;
  padding: 0.3rem;
  font-size: 0.68rem;
  text-overflow: ellipsis;
}

.picture-input input::file-selector-button {
  display: block;
  margin: 0 0 0.3rem;
}

.identity-fields {
  flex: 1;
  min-width: 0;
}

.identity-fields .field-row {
  margin-bottom: 0.25rem;
}

.story-tab,
.build-tab,
.gear-tab-wrap {
  padding: 1.75rem;
  margin-bottom: 1.5rem;
}

.long-fields {
  margin-top: 0.5rem;
}

.preview-note {
  margin: 1rem 0 0;
  font-size: 0.78rem;
  color: var(--text-faint);
  line-height: 1.5;
}

.save-bar {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.saved-hint {
  font-size: 0.85rem;
  color: var(--accent);
}

.save-error {
  margin-bottom: 1rem;
}
</style>
