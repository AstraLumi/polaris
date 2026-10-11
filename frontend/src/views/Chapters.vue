<template>
  <div class="page is-narrow">
    <header class="page-header">
      <div>
        <h1>{{ $t('Chapters') }}</h1>
        <p class="page-sub">
          {{ $tn('{n} chapter', '{n} chapters', idx.chapters.length) }}
          <template v-if="idx.volumes.length"> · {{ $tn('{n} volume', '{n} volumes', idx.volumes.length) }}</template>
        </p>
      </div>
      <div class="header-actions">
        <button class="btn btn-ghost" @click="addVolume">{{ $t('New volume') }}</button>
        <button class="btn btn-primary" @click="addChapter(null)">{{ $t('New chapter') }}</button>
      </div>
    </header>

    <p v-if="error" class="error-banner">{{ error }}</p>
    <p v-if="loading" class="loading-hint">{{ $t('Loading…') }}</p>

    <div v-else-if="!idx.chapters.length && !idx.volumes.length" class="empty-state glass-panel">
      <p class="empty-title">{{ $t('No chapters yet.') }}</p>
      <p class="empty-hint">
        {{ $t('Chapters hold your notes on each part of the story. Events and character versions can say which chapter they happen in, and the chapter lists them.') }}
      </p>
    </div>

    <template v-else>
      <section v-if="loose.length" class="glass-panel block">
        <ChapterRows :chapters="loose" @move="moveChapter" @delete="(c) => (confirmChapter = c)" />
      </section>

      <section v-for="(v, vi) in idx.volumes" :key="v.id" class="glass-panel block">
        <div class="volume-head">
          <input
            v-if="renaming === v.id"
            v-model="renameText"
            class="rename"
            type="text"
            maxlength="200"
            :aria-label="$t('Volume title')"
            @keydown.enter.prevent="saveRename(v)"
            @blur="saveRename(v)"
          />
          <h2 v-else>{{ v.title }}</h2>
          <div class="volume-actions">
            <button type="button" class="btn btn-ghost small" :disabled="vi === 0" :title="$t('Move up')" @click="moveVolume(vi, -1)">↑</button>
            <button type="button" class="btn btn-ghost small" :disabled="vi === idx.volumes.length - 1" :title="$t('Move down')" @click="moveVolume(vi, 1)">↓</button>
            <button type="button" class="btn btn-ghost small" @click="startRename(v)">{{ $t('Rename') }}</button>
            <button type="button" class="btn btn-ghost small" @click="confirmVolume = v">{{ $t('Delete') }}</button>
          </div>
        </div>
        <ChapterRows :chapters="chaptersOf(v.id)" @move="moveChapter" @delete="(c) => (confirmChapter = c)" />
        <button type="button" class="btn btn-ghost small add-here" @click="addChapter(v.id)">{{ $t('Add a chapter to {name}', { name: v.title }) }}</button>
      </section>
    </template>

    <ConfirmDialog
      v-if="confirmVolume"
      :title="$t('Delete {name}?', { name: confirmVolume.title })"
      :message="$t('Its chapters are kept and move out of the volume.')"
      :confirm-label="$t('Delete')"
      @cancel="confirmVolume = null"
      @confirm="removeVolume"
    />
    <ConfirmDialog
      v-if="confirmChapter"
      :title="$t('Delete {name}?', { name: confirmChapter.title })"
      :message="$t('Its notes are deleted. Events and character versions that pointed to it are kept, without a chapter.')"
      :confirm-label="$t('Delete')"
      @cancel="confirmChapter = null"
      @confirm="removeChapter"
    />
  </div>
</template>

<script setup>
// The story's chapters, grouped into volumes. Order is changed with the
// arrows; a chapter moves to another volume from its own page.
import { ref, computed, h, onMounted, nextTick } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { t, tn } from '../i18n'
import { chaptersApi } from '../api'
import { chapterIndex as idx, loadChapters } from '../chapters'
import ConfirmDialog from '../components/ConfirmDialog.vue'

const router = useRouter()
const loading = ref(true)
const error = ref('')
const renaming = ref(null)
const renameText = ref('')
const confirmVolume = ref(null)
const confirmChapter = ref(null)

function removeChapter() {
  const c = confirmChapter.value
  confirmChapter.value = null
  run(async () => {
    await chaptersApi.remove(c.id)
    await loadChapters(true)
  })
}

const loose = computed(() => idx.value.chapters.filter((c) => !c.volume_id))
const chaptersOf = (volumeId) => idx.value.chapters.filter((c) => c.volume_id === volumeId)

// One list of chapter rows with up/down arrows (within its group).
const ChapterRows = {
  props: { chapters: Array },
  emits: ['move', 'delete'],
  setup(props, { emit }) {
    return () =>
      props.chapters.length
        ? h(
            'ol',
            { class: 'chapter-list' },
            props.chapters.map((c, i) =>
              h('li', { class: 'chapter-row', key: c.id }, [
                h('span', { class: 'num' }, String(c.number)),
                h(RouterLink, { to: `/chapters/${c.id}`, class: 'chapter-title' }, () => c.title),
                h('span', { class: 'counts' }, [
                  c.event_count ? tn('{n} event', '{n} events', c.event_count) : '',
                  c.event_count && c.version_count ? ' · ' : '',
                  c.version_count ? tn('{n} character change', '{n} character changes', c.version_count) : '',
                ].join('')),
                h('button', { type: 'button', class: 'btn btn-ghost small', disabled: i === 0, title: t('Move up'), onClick: () => emit('move', c, -1) }, '↑'),
                h('button', { type: 'button', class: 'btn btn-ghost small', disabled: i === props.chapters.length - 1, title: t('Move down'), onClick: () => emit('move', c, 1) }, '↓'),
                h('button', { type: 'button', class: 'btn btn-ghost small row-delete', title: t('Delete chapter'), 'aria-label': t('Delete chapter'), onClick: () => emit('delete', c) }, '×'),
              ]),
            ),
          )
        : h('p', { class: 'empty' }, t('No chapters in this volume yet.'))
  },
}

async function run(fn) {
  error.value = ''
  try {
    await fn()
  } catch (e) {
    error.value = e.message
  }
}

// Sends the whole order with one change applied.
function saveOrder(volumes, chapters) {
  return run(async () => {
    idx.value = await chaptersApi.reorder({
      volumes: volumes.map((v) => v.id),
      chapters: chapters.map((c) => ({ id: c.id, volume_id: c.volume_id ?? null })),
    })
  })
}

function moveChapter(c, by) {
  const list = idx.value.chapters.slice()
  const group = list.filter((x) => (x.volume_id ?? null) === (c.volume_id ?? null))
  const i = group.indexOf(c)
  const other = group[i + by]
  if (!other) return
  const a = list.indexOf(c)
  const b = list.indexOf(other)
  ;[list[a], list[b]] = [list[b], list[a]]
  saveOrder(idx.value.volumes, list)
}

function moveVolume(i, by) {
  const vols = idx.value.volumes.slice()
  ;[vols[i], vols[i + by]] = [vols[i + by], vols[i]]
  saveOrder(vols, idx.value.chapters)
}

function addVolume() {
  run(async () => {
    const n = idx.value.volumes.length + 1
    const v = await chaptersApi.createVolume(t('Volume {n}', { n }))
    await loadChapters(true)
    startRename(v)
  })
}

function addChapter(volumeId) {
  run(async () => {
    const c = await chaptersApi.create({ title: t('New chapter'), volume_id: volumeId })
    await loadChapters(true)
    router.push({ path: `/chapters/${c.id}`, query: { edit: '1' } })
  })
}

function startRename(v) {
  renaming.value = v.id
  renameText.value = v.title
  nextTick(() => document.querySelector('.rename')?.select())
}

function saveRename(v) {
  if (renaming.value !== v.id) return
  renaming.value = null
  const title = renameText.value.trim()
  if (!title || title === v.title) return
  run(async () => {
    await chaptersApi.renameVolume(v.id, title)
    await loadChapters(true)
  })
}

function removeVolume() {
  const v = confirmVolume.value
  confirmVolume.value = null
  run(async () => {
    await chaptersApi.removeVolume(v.id)
    await loadChapters(true)
  })
}

onMounted(async () => {
  await run(() => loadChapters(true))
  loading.value = false
})
</script>

<style scoped>
.header-actions {
  display: flex;
  gap: 0.6rem;
  flex-wrap: wrap;
}

.block {
  padding: 1rem 1.25rem;
  margin-bottom: 1rem;
}

.volume-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 0.5rem;
  flex-wrap: wrap;
}

.volume-head h2 {
  margin: 0;
  font-family: 'Fraunces', serif;
  font-weight: 500;
  font-size: 1.15rem;
}

.rename {
  max-width: 22rem;
}

.volume-actions {
  display: flex;
  gap: 0.3rem;
}

.add-here {
  margin-top: 0.5rem;
}

:deep(.chapter-list) {
  list-style: none;
  margin: 0;
  padding: 0;
}

:deep(.chapter-row) {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  padding: 0.4rem 0.3rem;
  border-top: 1px solid var(--glass-border);
}

:deep(.chapter-row:first-child) {
  border-top: none;
}

:deep(.num) {
  width: 1.8rem;
  text-align: right;
  font-family: 'Fraunces', serif;
  color: var(--accent);
}

:deep(.chapter-title) {
  flex: 1;
  min-width: 0;
  color: var(--text-primary);
  text-decoration: none;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

:deep(.chapter-title:hover) {
  color: var(--accent);
}

:deep(.counts) {
  font-size: 0.75rem;
  color: var(--text-faint);
  white-space: nowrap;
}

:deep(.empty) {
  margin: 0;
  font-size: 0.85rem;
  color: var(--text-faint);
}

@media (max-width: 720px) {
  :deep(.counts) {
    display: none;
  }
}
</style>
