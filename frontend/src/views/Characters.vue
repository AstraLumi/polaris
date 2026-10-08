<template>
  <div class="page">
    <header class="page-header">
      <div>
        <h1>{{ $t('Characters') }}</h1>
        <p class="page-sub">{{ $tn('{n} saved', '{n} saved', characters.length) }}</p>
      </div>
      <div class="actions">
        <button
          v-if="selectMode"
          class="btn btn-danger"
          :disabled="selectedIds.size === 0"
          @click="confirmOpen = true"
        >
          {{ $t('Delete selected ({n})', { n: selectedIds.size }) }}
        </button>
        <button class="btn btn-ghost" @click="toggleSelectMode">
          {{ selectMode ? $t('Cancel') : $t('Delete characters') }}
        </button>
        <button class="btn btn-primary" @click="addOpen = true">{{ $t('Add character') }}</button>
      </div>
    </header>

    <p v-if="loadError" class="error-banner">{{ loadError }}</p>

    <div v-if="!loading && characters.length === 0" class="empty-state glass-panel">
      <p class="empty-title">{{ $t('No characters yet.') }}</p>
      <p class="empty-hint">{{ $t('Add your first one to start building their story and stats.') }}</p>
    </div>

    <div class="grid">
      <CharacterCard
        v-for="c in characters"
        :key="c.id"
        :character="c"
        :select-mode="selectMode"
        :selected="selectedIds.has(c.id)"
        @toggle-select="toggleSelected(c.id)"
      />
    </div>

    <AddCharacterModal v-if="addOpen" @close="addOpen = false" @created="onCreated" />

    <ConfirmDialog
      v-if="confirmOpen"
      :title="$tn('Delete {n} character?', 'Delete {n} characters?', selectedIds.size)"
      :message="$t('This removes their Story and Build data too. This can\'t be undone.')"
      :confirm-label="$t('Delete')"
      @cancel="confirmOpen = false"
      @confirm="handleDelete"
    />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { t } from '../i18n'
import CharacterCard from '../components/CharacterCard.vue'
import AddCharacterModal from '../components/AddCharacterModal.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import { fetchCharacters, deleteCharacters } from '../api'

const characters = ref([])
const loading = ref(true)
const loadError = ref('')
const selectMode = ref(false)
const selectedIds = ref(new Set())
const addOpen = ref(false)
const confirmOpen = ref(false)

async function load() {
  loading.value = true
  loadError.value = ''
  try {
    characters.value = await fetchCharacters()
  } catch (err) {
    loadError.value = t("Couldn't load characters. Try refreshing.")
  } finally {
    loading.value = false
  }
}

function toggleSelectMode() {
  selectMode.value = !selectMode.value
  selectedIds.value = new Set()
}

function toggleSelected(id) {
  const next = new Set(selectedIds.value)
  if (next.has(id)) {
    next.delete(id)
  } else {
    next.add(id)
  }
  selectedIds.value = next
}

function onCreated() {
  addOpen.value = false
  load()
}

async function handleDelete() {
  try {
    await deleteCharacters([...selectedIds.value])
    confirmOpen.value = false
    selectMode.value = false
    selectedIds.value = new Set()
    await load()
  } catch (err) {
    loadError.value = t("Couldn't delete those characters. Try again.")
    confirmOpen.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.actions {
  display: flex;
  gap: 0.6rem;
  flex-wrap: wrap;
}

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 1.1rem;
}

</style>
