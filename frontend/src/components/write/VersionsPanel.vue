<template>
  <aside class="side-panel glass-panel panel-solid versions-panel" :aria-label="$t('Saved versions')">
    <header class="side-head">
      <h2>{{ $t('Saved versions') }}</h2>
      <button type="button" class="btn btn-ghost small" :aria-label="$t('Close')" @click="$emit('close')">✕</button>
    </header>

    <form class="save-now" @submit.prevent="saveNow">
      <input v-model="label" type="text" maxlength="120" :placeholder="$t('Label (optional)')" :aria-label="$t('Label (optional)')" />
      <button type="submit" class="btn btn-primary small" :disabled="busy">{{ $t('Save a version now') }}</button>
    </form>
    <p class="hint">{{ $t('A version is also kept on its own every 30 minutes of writing.') }}</p>

    <p v-if="error" class="error-banner">{{ error }}</p>
    <p v-if="loading" class="loading-hint">{{ $t('Loading…') }}</p>
    <p v-else-if="!list.length" class="hint">{{ $t('No versions of this chapter yet.') }}</p>

    <ul v-else class="version-list">
      <li v-for="v in list" :key="v.id">
        <button type="button" class="version-row" :class="{ 'is-picked': picked?.id === v.id }" @click="open(v)">
          <span class="v-label">{{ labelOf(v) }}</span>
          <span class="v-meta" :title="exact(v.created_at)">{{ ago(v.created_at) }} · {{ $tn('{n} word', '{n} words', v.words || 0) }}</span>
        </button>
      </li>
    </ul>

    <section v-if="picked" class="preview">
      <div class="preview-head">
        <strong>{{ labelOf(picked) }}</strong>
        <button type="button" class="btn btn-primary small" :disabled="busy || !picked.doc" @click="restore">{{ $t('Restore this version') }}</button>
      </div>
      <p class="hint">{{ $t('Restoring keeps the text you have now as a version first.') }}</p>
      <div class="preview-paper">
        <p v-if="!picked.doc" class="loading-hint">{{ $t('Loading…') }}</p>
        <DocPreview v-else :doc="picked.doc" :online="online" />
      </div>
    </section>
  </aside>
</template>

<script setup>
// A chapter's saved versions: list, preview, save one now with a label,
// restore one (the server keeps the current text as "Before restoring").
import { ref, watch } from 'vue'
import { t, tr } from '../../i18n'
import { textApi } from '../../editor/api.js'
import { ago, exact } from '../../editor/when.js'
import DocPreview from './DocPreview.vue'

// Labels the server and this page give versions; shown translated.
void [tr('Autosave'), tr('Before restoring')]

const props = defineProps({
  chapterId: { type: [Number, String], required: true },
  online: { type: Boolean, default: false },
  // Sends what is typed first, so a version holds the latest text.
  flush: { type: Function, required: true },
})
const emit = defineEmits(['close', 'restored'])

const list = ref([])
const loading = ref(false)
const busy = ref(false)
const error = ref('')
const label = ref('')
const picked = ref(null)

const labelOf = (v) => (v.label ? t(v.label) : t('Version'))

async function load() {
  loading.value = true
  error.value = ''
  try {
    const res = await textApi.versions(props.chapterId)
    list.value = Array.isArray(res) ? res : res?.versions || []
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

watch(
  () => props.chapterId,
  () => {
    picked.value = null
    load()
  },
  { immediate: true },
)

async function open(v) {
  picked.value = { ...v, doc: null }
  try {
    const full = await textApi.version(props.chapterId, v.id)
    if (picked.value?.id === v.id) picked.value = { ...v, ...full }
  } catch (e) {
    error.value = e.message
  }
}

async function saveNow() {
  busy.value = true
  error.value = ''
  try {
    await props.flush()
    await textApi.saveVersion(props.chapterId, label.value.trim())
    label.value = ''
    await load()
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

async function restore() {
  const v = picked.value
  if (!v) return
  if (!window.confirm(t('Put "{label}" back as the chapter\'s text? What is there now is kept as a version.', { label: labelOf(v) }))) return
  busy.value = true
  error.value = ''
  try {
    await props.flush()
    const res = await textApi.restore(props.chapterId, v.id)
    emit('restored', res)
    picked.value = null
    await load()
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}
</script>

<style scoped>
.save-now {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
}

.save-now input {
  flex: 1 1 9rem;
  min-width: 0;
}

.save-now .btn {
  white-space: nowrap;
}

.hint {
  margin: 0.4rem 0 0.6rem;
  font-size: 0.75rem;
  color: var(--text-faint);
  line-height: 1.4;
}

.version-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 0.15rem;
}

.version-row {
  display: grid;
  width: 100%;
  gap: 0.1rem;
  padding: 0.4rem 0.6rem;
  border-radius: 8px;
  border: 1px solid transparent;
  background: none;
  color: var(--text-primary);
  text-align: left;
  cursor: pointer;
  font: inherit;
}

.version-row:hover {
  background: var(--glass);
}

.version-row.is-picked {
  border-color: var(--accent);
  background: var(--accent-soft);
}

.v-label {
  font-weight: 600;
  font-size: 0.88rem;
}

.v-meta {
  font-size: 0.74rem;
  color: var(--text-faint);
}

.preview {
  margin-top: 0.9rem;
  border-top: 1px solid var(--glass-border);
  padding-top: 0.7rem;
}

.preview-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.preview-paper {
  max-height: 22rem;
  overflow: auto;
  padding: 0.8rem 1rem;
  border-radius: 8px;
  background: var(--paper, #fbf8f1);
  color: var(--ink, #1f1c17);
}
</style>
