<template>
  <div class="picker">
    <div class="picker-inner">
      <header class="picker-head">
        <PolarisLogo class="picker-mark" />
        <h1>{{ $t('Your stories') }}</h1>
        <p class="page-sub">
          {{ $t('Every story is its own world, with its own characters, map, events, assets and calendar. Pick one to open it.') }}
        </p>
      </header>

      <p v-if="error" class="error-banner">{{ error }}</p>
      <p v-if="notice" class="notice-banner">{{ notice }}</p>
      <p v-if="!storiesLoaded && !error" class="loading-hint">{{ $t('Loading…') }}</p>

      <template v-if="storiesLoaded">
        <p v-if="!stories.length" class="empty">
          {{ $t('No stories yet. Create your first one, or import one from a backup.') }}
        </p>

        <ul class="grid">
          <li v-for="s in stories" :key="s.id" class="card glass-panel" :class="{ 'is-current': s.id === activeStoryId }">
            <button class="card-open" type="button" @click="openStory(s.id)">
              <span class="card-icon"><IconImage :src="s.icon" :name="s.name" /></span>
              <span class="card-name">{{ s.name }}</span>
              <span class="card-meta">
                <span v-if="s.id === activeStoryId" class="pill">{{ $t('Current') }}</span>
                {{ $tn('{n} character', '{n} characters', s.characters) }}
              </span>
            </button>
            <div class="card-actions">
              <button class="btn btn-ghost small" type="button" @click="formStory = s">{{ $t('Edit') }}</button>
              <a class="btn btn-ghost small" :href="storyExportUrl(s.id)" download>{{ $t('Export') }}</a>
              <button class="btn btn-ghost small danger" type="button" @click="toDelete = s">{{ $t('Delete') }}</button>
            </div>
          </li>

          <li class="card card-new">
            <button class="card-open dashed" type="button" @click="creating = true">
              <span class="plus">+</span>
              <span class="card-name">{{ $t('New story') }}</span>
            </button>
          </li>
          <li class="card card-new">
            <button class="card-open dashed" type="button" :disabled="importing" @click="fileInput.click()">
              <span class="plus">↥</span>
              <span class="card-name">{{ importing ? $t('Importing…') : $t('Import story') }}</span>
              <span class="card-meta">{{ $t('From an exported .zip') }}</span>
            </button>
          </li>
        </ul>
        <input ref="fileInput" type="file" accept=".zip,application/zip" class="hidden" @change="onImport" />
      </template>
    </div>

    <StoryFormModal
      v-if="creating || formStory"
      :story="formStory"
      :others="stories"
      @close="closeForm"
      @saved="onSaved"
    />
    <StoryDeleteModal v-if="toDelete" :story="toDelete" @cancel="toDelete = null" @deleted="onDeleted" />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import PolarisLogo from '../components/PolarisLogo.vue'
import IconImage from '../components/IconImage.vue'
import StoryFormModal from '../components/StoryFormModal.vue'
import StoryDeleteModal from '../components/StoryDeleteModal.vue'
import {
  stories, storiesLoaded, loadStories, activeStoryId, openStory, forgetStory,
  importStory, storyExportUrl,
} from '../stories'
import { t } from '../i18n'

const error = ref('')
const notice = ref('')
const creating = ref(false)
const formStory = ref(null)
const toDelete = ref(null)
const importing = ref(false)
const fileInput = ref(null)

async function refresh() {
  try {
    await loadStories()
  } catch (err) {
    error.value = err.message || t("Couldn't load your stories. Try refreshing.")
  }
}
onMounted(refresh)

function closeForm() {
  creating.value = false
  formStory.value = null
}

async function onSaved(saved, wasNew) {
  closeForm()
  if (wasNew) {
    openStory(saved.id) // straight into the new story
    return
  }
  await refresh()
}

async function onDeleted(story) {
  toDelete.value = null
  if (story.id === activeStoryId) forgetStory()
  await refresh()
}

async function onImport(e) {
  const file = e.target.files[0]
  e.target.value = ''
  if (!file) return
  importing.value = true
  error.value = ''
  notice.value = ''
  try {
    const s = await importStory(file)
    notice.value = t('Imported “{name}”.', { name: s.name })
    await refresh()
  } catch (err) {
    error.value = err.message || t("Couldn't import that file.")
  } finally {
    importing.value = false
  }
}
</script>

<style scoped>
.picker {
  min-height: 100vh;
  display: flex;
  justify-content: center;
  padding: 3rem 1.5rem;
}

.picker-inner {
  width: 100%;
  max-width: 980px;
}

.picker-head {
  text-align: center;
  margin-bottom: 2.25rem;
}

.picker-mark {
  display: block;
  height: 3.4rem;
  margin: 0 auto 1rem;
}

.picker-head h1 {
  margin: 0;
  font-size: 2rem;
}

.picker-head .page-sub {
  max-width: 38rem;
  margin: 0.5rem auto 0;
  line-height: 1.5;
}

.empty {
  text-align: center;
  color: var(--text-muted);
  margin: 0 0 1.5rem;
}

.notice-banner {
  padding: 0.7rem 1rem;
  border-radius: 10px;
  border: 1px solid color-mix(in srgb, var(--accent) 40%, transparent);
  background: var(--accent-soft);
  font-size: 0.88rem;
  margin: 0 0 1rem;
}

.grid {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(210px, 240px));
  justify-content: center;
  gap: 1rem;
}

.card {
  display: flex;
  flex-direction: column;
  border-radius: 14px;
  overflow: hidden;
}

.card.is-current {
  border-color: color-mix(in srgb, var(--accent) 55%, transparent);
}

.card-open {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.6rem;
  padding: 1.5rem 1rem 1rem;
  background: transparent;
  border: 0;
  color: inherit;
  font: inherit;
  cursor: pointer;
  text-align: center;
}

.card-open:hover:not(:disabled) {
  background: var(--accent-soft);
}

.card-icon {
  width: 72px;
  height: 72px;
  border-radius: 16px;
  background: var(--accent-soft);
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 1.6rem;
}

.card-name {
  font-family: 'Fraunces', serif;
  font-size: 1.1rem;
  color: var(--text-primary);
  overflow-wrap: anywhere;
}

.card-meta {
  font-size: 0.8rem;
  color: var(--text-muted);
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.pill {
  font-size: 0.68rem;
  padding: 0.1rem 0.45rem;
  border-radius: 999px;
  background: var(--accent-soft);
  color: var(--accent);
  border: 1px solid color-mix(in srgb, var(--accent) 35%, transparent);
}

.card-actions {
  display: flex;
  justify-content: center;
  flex-wrap: wrap;
  gap: 0.35rem;
  padding: 0.6rem 0.5rem 0.85rem;
}

.card-new {
  border: 1px dashed var(--glass-border);
  background: transparent;
  min-height: 190px;
}

.card-open.dashed {
  justify-content: center;
}

.plus {
  font-size: 2rem;
  line-height: 1;
  color: var(--accent);
}

.danger {
  color: var(--danger, #e5798a);
}

.hidden {
  display: none;
}
</style>
