<template>
  <div class="page is-narrow">
    <RouterLink :to="`/characters/${id}`" class="back-link">{{ $t('← Back to character') }}</RouterLink>

    <header class="page-header">
      <div>
        <h1>{{ $t('Versions') }}</h1>
        <p class="page-sub">{{ $t('Pick one to edit, or start a new one from the current version.') }}</p>
      </div>
      <button class="btn btn-primary" @click="newOpen = true">{{ $t('New version') }}</button>
    </header>

    <p v-if="loadError" class="error-banner">{{ loadError }}</p>
    <p v-else-if="loading" class="loading-hint">{{ $t('Loading…') }}</p>

    <ul v-else class="versions">
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
          </span>
        </RouterLink>
        <div class="version-actions">
          <button v-if="!v.is_current" class="btn btn-ghost small" @click="handleSetCurrent(v.id)">
            {{ $t('Set current') }}
          </button>
          <button
            class="btn btn-ghost small"
            :disabled="versions.length <= 1"
            :title="versions.length <= 1 ? $t('The only version — delete the character instead') : ''"
            @click="confirmTarget = v"
          >
            {{ $t('Delete') }}
          </button>
        </div>
      </li>
    </ul>

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
import { ref, onMounted } from 'vue'
import { t } from '../i18n'
import { fetchVersions, deleteVersion, setCurrentVersion } from '../api'
import NewVersionModal from '../components/NewVersionModal.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'

const props = defineProps({ id: { type: String, required: true } })

const versions = ref([])
const loading = ref(true)
const loadError = ref('')
const newOpen = ref(false)
const confirmTarget = ref(null)

async function load() {
  loading.value = true
  loadError.value = ''
  try {
    versions.value = await fetchVersions(props.id)
  } catch (err) {
    loadError.value = t("Couldn't load versions. Try refreshing.")
  } finally {
    loading.value = false
  }
}

function onCreated() {
  newOpen.value = false
  load()
}

async function handleSetCurrent(versionId) {
  try {
    await setCurrentVersion(versionId)
    await load()
  } catch (err) {
    loadError.value = t("Couldn't set that version as current.")
  }
}

async function handleDelete() {
  try {
    await deleteVersion(confirmTarget.value.id)
    confirmTarget.value = null
    await load()
  } catch (err) {
    loadError.value = t("Couldn't delete that version.")
    confirmTarget.value = null
  }
}

onMounted(load)
</script>

<style scoped>
.versions {
  list-style: none;
  margin: 0;
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
