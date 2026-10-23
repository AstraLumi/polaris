<template>
  <div class="page">
    <Breadcrumbs :missing="missing"
      :crumbs="[{ label: $t('Characters'), to: '/characters' }, { label: characterName || (missing ? '' : '…'), to: `/characters/${id}` }, { label: $t('Versions') }]"
    >
      <ItemLinks v-if="characterName" type="character" :id="id" />
    </Breadcrumbs>

    <NotFoundPanel v-if="missing" :message="$t('This character was deleted, or never existed.')">
      <RouterLink to="/characters" class="btn btn-ghost">{{ $t('All characters') }}</RouterLink>
    </NotFoundPanel>

    <header v-else class="page-header">
      <div>
        <h1>{{ characterName || $t('Versions') }}</h1>
        <p class="page-sub">{{ $t('What belongs to the character comes first, shared by every version. Below, pick a version to edit, or start a new one from the current version.') }}</p>
      </div>
    </header>

    <p v-if="loadError && !missing" class="error-banner">{{ loadError }}</p>
    <p v-else-if="loading" class="loading-hint">{{ $t('Loading…') }}</p>

    <template v-else-if="!missing">
      <section class="glass-panel shared-panel">
        <h2>{{ $t('The character') }}</h2>
        <p class="section-sub">{{ $t('These belong to the character, not to one version: changing them changes them in every version.') }}</p>
        <div class="field-grid">
          <DateField v-model="shared.birth_date" :label="$t('Birth date (in-story)')" />
          <LocationSelect
            v-model="shared.born_in_location_id"
            :label="$t('Born in')"
            :options="locationOptions"
            :legacy-text="shared.born_in"
          />
          <label class="field">
            <span>{{ $t('Earth Name') }}</span>
            <input v-model="shared.earth_name" type="text" maxlength="200" :placeholder="$t('Who they are on Earth, if anyone')" />
          </label>
          <DateField v-model="shared.human_birth_date" :label="$t('Earth Birthdate')" earth />
          <BloodTypePicker v-model="shared.blood_type" class="span-2" />
        </div>
        <TagInput
          v-model="shared.tags"
          :suggestions="tagSuggestions"
          list-id="character-tag-suggestions"
          :hint="$t('Filter by tags on the Characters page.')"
        />
        <p v-if="sharedError" class="error-banner">{{ sharedError }}</p>
        <div class="save-bar">
          <button class="btn btn-primary" :disabled="sharedSaving || !sharedDirty()" @click="saveShared">
            {{ sharedSaving ? $t('Saving…') : $t('Save changes') }}
          </button>
          <span v-if="sharedSaved" class="saved-hint">{{ $t('Saved.') }}</span>
        </div>
      </section>

      <div class="section-head">
        <h2>{{ $t('Versions') }}</h2>
        <div class="header-actions">
          <RouterLink v-if="versions.length > 1" :to="`/characters/${id}/compare`" class="btn btn-ghost small">
            {{ $t('Compare versions') }}
          </RouterLink>
          <button class="btn btn-primary small" :title="$t('Start a new version as a copy of the current one')" @click="newOpen = true">{{ $t('New version') }}</button>
        </div>
      </div>

      <ul class="versions">
        <li
          v-for="v in versions"
          :key="v.id"
          class="version-row glass-panel"
          :class="{ 'is-current': v.is_current }"
        >
          <RouterLink :to="`/characters/${id}/versions/${v.id}/edit`" class="version-main">
            <span v-if="v.is_current" class="current-badge">{{ $t('Current') }}</span>
            <span class="version-name">{{ v.name }}<span v-if="v.nickname"> "{{ v.nickname }}"</span></span>
            <span class="version-meta">
              {{ $t('Lv.') }} {{ v.level }}
              <span v-if="v.class_name"> · {{ v.class_name }}</span>
              <span v-if="v.version_date"> · {{ v.version_date }}</span>
              <span v-if="v.version_reference"> · {{ v.version_reference }}</span>
              <span v-if="v.chapter_id && chapterLabel(v.chapter_id)"> · {{ chapterLabel(v.chapter_id) }}</span>
            </span>
          </RouterLink>
          <div class="version-actions">
            <RouterLink :to="`/characters/${id}/versions/${v.id}/edit`" class="btn btn-ghost small">{{ $t('Edit') }}</RouterLink>
            <RouterLink
              v-if="!v.is_current && currentVersion"
              :to="{ path: `/characters/${id}/compare`, query: { a: v.id, b: currentVersion.id } }"
              class="btn btn-ghost small"
              :title="$t('Compare with the current version')"
            >
              {{ $t('Compare') }}
            </RouterLink>
            <button v-if="!v.is_current" class="btn btn-ghost small" :title="$t('Make this the version shown on the sheet, the wiki and the timeline')" @click="handleSetCurrent(v.id)">
              {{ $t('Set current') }}
            </button>
            <!-- Deleting is kept apart from the everyday actions; the only
                 version can't go (delete the character instead). -->
            <RowMenu
              v-if="versions.length > 1"
              :label="$t('More actions')"
              :items="[{ label: $t('Delete version'), danger: true, action: () => (confirmTarget = v) }]"
            />
          </div>
        </li>
      </ul>

      <section class="glass-panel relations-panel">
        <RelationsTab :character-id="Number(id)" :character-name="characterName" :point="relationPoint" />
        <p class="section-sub">{{ $t('Relations belong to the character, not to one version, and are saved as soon as you add or change them.') }}</p>
      </section>
    </template>

    <NewVersionModal
      v-if="newOpen"
      :character-id="id"
      @close="newOpen = false"
      @created="onCreated"
    />

    <ConfirmDialog
      v-if="confirmTarget"
      :title="$t('Delete this version?')"
      :message="$t('Its Story and Build data goes with it. This can\'t be undone.')"
      :confirm-label="$t('Delete')"
      @cancel="confirmTarget = null"
      @confirm="handleDelete"
    />
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { t } from '../i18n'
import {
  fetchVersions, deleteVersion, setCurrentVersion, fetchCurrentVersion, fetchCharacters, fetchLocationOptions, updateSharedCharacter, isMissing,
} from '../api'
import NewVersionModal from '../components/NewVersionModal.vue'
import Breadcrumbs from '../components/Breadcrumbs.vue'
import ItemLinks from '../components/ItemLinks.vue'
import NotFoundPanel from '../components/NotFoundPanel.vue'
import DateField from '../components/DateField.vue'
import LocationSelect from '../components/LocationSelect.vue'
import TagInput from '../components/TagInput.vue'
import BloodTypePicker from '../components/BloodTypePicker.vue'
import RelationsTab from '../components/RelationsTab.vue'
import { chapterLabel, loadChapters } from '../chapters'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import RowMenu from '../components/RowMenu.vue'
import { pageTitle, useUnsavedGuard, useSaveShortcut } from '../navigation'

const props = defineProps({ id: { type: String, required: true } })
loadChapters()

const versions = ref([])
const loading = ref(true)
const loadError = ref('')
const missing = ref(false)
const newOpen = ref(false)
const confirmTarget = ref(null)
const currentVersion = computed(() => versions.value.find((v) => v.is_current) || null)

// What belongs to the character rather than to a version
// (backend/charactershared.go), saved with its own button.
const characterName = ref('')
const shared = reactive({ birth_date: '', human_birth_date: '', earth_name: '', blood_type: '', born_in: '', born_in_location_id: null, tags: [] })
const sharedSaving = ref(false)
const sharedSaved = ref(false)
const sharedError = ref('')
const locationOptions = ref([])
const tagSuggestions = ref([])
// Relations that hold at the current version are listed first.
const relationPoint = computed(() =>
  currentVersion.value ? { chapterId: currentVersion.value.chapter_id, date: currentVersion.value.version_date } : null,
)

let sharedSnapshot = ''
const sharedState = () => JSON.stringify(shared)
const sharedDirty = () => !loading.value && !loadError.value && sharedState() !== sharedSnapshot
useUnsavedGuard(sharedDirty)
useSaveShortcut(() => sharedDirty() && !sharedSaving.value && saveShared())

function takeCharacter(current) {
  characterName.value = current.name
  pageTitle.value = current.name
  Object.assign(shared, {
    birth_date: current.story.birth_date || '',
    human_birth_date: current.story.human_birth_date || '',
    earth_name: current.story.earth_name || '',
    blood_type: current.story.blood_type || '',
    born_in: current.story.born_in || '',
    born_in_location_id: current.story.born_in_location_id ?? null,
    tags: current.tags || [],
  })
  sharedSnapshot = sharedState()
}

async function load() {
  loading.value = true
  loadError.value = ''
  try {
    const [list, current] = await Promise.all([fetchVersions(props.id), fetchCurrentVersion(props.id)])
    versions.value = list
    takeCharacter(current)
  } catch (err) {
    missing.value = await isMissing(`/characters/${props.id}/current`)
    loadError.value = t("Couldn't load versions. Try refreshing.")
  } finally {
    loading.value = false
  }
}

async function loadVersions() {
  try {
    versions.value = await fetchVersions(props.id)
  } catch (err) {
    loadError.value = t("Couldn't load versions. Try refreshing.")
  }
}

async function saveShared() {
  sharedSaving.value = true
  sharedError.value = ''
  sharedSaved.value = false
  try {
    takeCharacter(await updateSharedCharacter(props.id, { ...shared }))
    sharedSaved.value = true
    setTimeout(() => (sharedSaved.value = false), 2500)
  } catch (err) {
    sharedError.value = err.message || t("Couldn't save. Try again.")
  } finally {
    sharedSaving.value = false
  }
}

function onCreated() {
  newOpen.value = false
  loadVersions()
}

async function handleSetCurrent(versionId) {
  try {
    await setCurrentVersion(versionId)
    await loadVersions()
  } catch (err) {
    loadError.value = t("Couldn't set that version as current.")
  }
}

async function handleDelete() {
  try {
    await deleteVersion(confirmTarget.value.id)
    confirmTarget.value = null
    await loadVersions()
  } catch (err) {
    loadError.value = t("Couldn't delete that version.")
    confirmTarget.value = null
  }
}

onMounted(async () => {
  load()
  // Every tag already on a character, offered while typing.
  fetchCharacters()
    .then((list) => {
      const seen = new Map()
      for (const c of list) for (const tag of c.tags || []) seen.set(tag.toLowerCase(), tag)
      tagSuggestions.value = [...seen.values()].sort((a, b) => a.localeCompare(b))
    })
    .catch(() => {})
  locationOptions.value = await fetchLocationOptions()
})
</script>

<style scoped>
.shared-panel,
.relations-panel {
  padding: 1.5rem;
  margin-bottom: 1.5rem;
}

.shared-panel h2,
.section-head h2 {
  font-family: 'Fraunces', serif;
  font-weight: 500;
  font-size: 1.15rem;
  margin: 0 0 0.35rem;
}

.section-sub {
  margin: 0 0 1rem;
  font-size: 0.8rem;
  line-height: 1.5;
  color: var(--text-faint);
}

.relations-panel .section-sub {
  margin: 1rem 0 0;
}

.save-bar {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-top: 0.5rem;
}

.saved-hint {
  font-size: 0.85rem;
  color: var(--accent);
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 0.75rem;
  margin-bottom: 0.75rem;
}

.section-head h2 {
  margin: 0;
}

.header-actions {
  display: flex;
  gap: 0.6rem;
  flex-wrap: wrap;
}

.versions {
  list-style: none;
  margin: 0 0 1.5rem;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.version-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 1rem 1.25rem;
}

.version-row.is-current {
  border-color: var(--accent);
}

.version-main {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
  text-decoration: none;
  min-width: 0;
  flex: 1;
}

.current-badge {
  align-self: flex-start;
  font-size: 0.68rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  color: var(--accent);
  background: var(--accent-soft);
  border-radius: 999px;
  padding: 0.15rem 0.55rem;
}

.version-name {
  font-family: 'Fraunces', serif;
  font-size: 1.02rem;
  color: var(--text-primary);
}

.version-meta {
  font-size: 0.8rem;
  color: var(--text-faint);
}

.version-actions {
  display: flex;
  gap: 0.5rem;
  flex-shrink: 0;
}
</style>
