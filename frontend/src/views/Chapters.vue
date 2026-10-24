<template>
  <div class="page">
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
        <div v-if="idx.volumes.length" class="volume-head">
          <h2 class="loose-title">{{ $t('Chapters without a volume') }}</h2>
          <RouterLink to="/write/0" class="btn btn-ghost small">{{ $t('Write') }}</RouterLink>
        </div>
        <div v-else class="write-row">
          <RouterLink to="/write/0" class="btn btn-ghost small">{{ $t('Write') }}</RouterLink>
        </div>
        <ChapterRows :chapters="loose" :volume-id="null" @move="moveChapter" @delete="(c) => (confirmChapter = c)" @drop-on="dropChapter" />
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
            <RouterLink :to="`/write/${v.id}`" class="btn btn-ghost small" :title="$t('Write this volume\'s text')">{{ $t('Write') }}</RouterLink>
            <button type="button" class="btn btn-ghost small" @click="startRename(v)">{{ $t('Rename') }}</button>
            <RowMenu
              :label="$t('Volume actions')"
              :items="[
                { label: $t('Move up'), disabled: vi === 0, action: () => moveVolume(vi, -1) },
                { label: $t('Move down'), disabled: vi === idx.volumes.length - 1, action: () => moveVolume(vi, 1) },
                { label: $t('Delete volume'), danger: true, action: () => (confirmVolume = v) },
              ]"
            />
          </div>
        </div>
        <ChapterRows :chapters="chaptersOf(v.id)" :volume-id="v.id" @move="moveChapter" @delete="(c) => (confirmChapter = c)" @drop-on="dropChapter" />
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
import { useQuickAdd } from '../quickAdd'
import { chapterIndex as idx, loadChapters } from '../chapters'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import RowMenu from '../components/RowMenu.vue'

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

// The chapter being dragged, shared by every list so one can go to another
// volume.
const dragged = ref(null)
const dropTarget = ref(null) // the chapter id (or 'end:<volume>') under the pointer

// One list of chapter rows. A row is dragged by its handle to a new place
// (before the row it is dropped on, or to the end of a volume); its "…"
// menu moves it up or down (the keyboard's way) or deletes it.
const ChapterRows = {
  props: { chapters: Array, volumeId: { type: Number, default: null } },
  emits: ['move', 'delete', 'drop-on'],
  setup(props, { emit }) {
    const over = (key) => (e) => {
      if (!dragged.value) return
      e.preventDefault()
      dropTarget.value = key
    }
    const drop = (target) => (e) => {
      e.preventDefault()
      const c = dragged.value
      dragged.value = null
      dropTarget.value = null
      if (c) emit('drop-on', c, target, props.volumeId)
    }
    const endKey = `end:${props.volumeId}`
    return () => {
      const rows = props.chapters.map((c, i) =>
        h(
          'li',
          {
            class: ['chapter-row', { 'is-dragging': dragged.value === c, 'is-drop-target': dropTarget.value === c.id && dragged.value !== c }],
            key: c.id,
            onDragover: over(c.id),
            onDrop: drop(c),
          },
          [
            h(
              'span',
              {
                class: 'grip',
                draggable: 'true',
                title: t('Drag to move'),
                'aria-hidden': 'true',
                onDragstart: (e) => {
                  dragged.value = c
                  e.dataTransfer.effectAllowed = 'move'
                  e.dataTransfer.setData('text/plain', String(c.id))
                },
                onDragend: () => {
                  dragged.value = null
                  dropTarget.value = null
                },
              },
              '⠿',
            ),
            h('span', { class: 'num' }, String(c.number)),
            h(RouterLink, { to: `/chapters/${c.id}`, class: 'chapter-title' }, () => c.title),
            h('span', { class: 'counts' }, [
              c.event_count ? tn('{n} event', '{n} events', c.event_count) : '',
              c.event_count && c.version_count ? ' · ' : '',
              c.version_count ? tn('{n} character change', '{n} character changes', c.version_count) : '',
            ].join('')),
            h(RowMenu, {
              label: t('Chapter actions'),
              items: [
                { label: t('Move up'), disabled: i === 0, action: () => emit('move', c, -1) },
                { label: t('Move down'), disabled: i === props.chapters.length - 1, action: () => emit('move', c, 1) },
                { label: t('Delete chapter'), danger: true, action: () => emit('delete', c) },
              ],
            }),
          ],
        ),
      )
      // Dropping below the last row (or on an empty volume) puts it at the end.
      const end = h('li', {
        class: ['drop-end', { 'is-drop-target': dropTarget.value === endKey, 'is-active': !!dragged.value }],
        key: 'end',
        onDragover: over(endKey),
        onDrop: drop(null),
      }, props.chapters.length ? '' : t('No chapters in this volume yet.'))
      return h('ol', { class: 'chapter-list' }, [...rows, end])
    }
  },
}

// A dragged chapter dropped before `target` (null: the end of the volume),
// moving into that volume.
function dropChapter(c, target, volumeId) {
  if (c === target) return
  const list = idx.value.chapters.filter((x) => x !== c)
  const moved = { ...c, volume_id: volumeId }
  if (target) list.splice(list.indexOf(target), 0, moved)
  else {
    // After the volume's last chapter, or where the volume's chapters would start.
    const inVolume = list.filter((x) => (x.volume_id ?? null) === (volumeId ?? null))
    const last = inVolume[inVolume.length - 1]
    list.splice(last ? list.indexOf(last) + 1 : list.length, 0, moved)
  }
  saveOrder(idx.value.volumes, list)
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

// /chapters?new=1 (the search's "New chapter") adds one, as the button does.
useQuickAdd(() => addChapter(null))
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

.loose-title {
  color: var(--text-muted);
}

.write-row {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 0.3rem;
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

:deep(.grip) {
  cursor: grab;
  padding: 0 0.15rem;
  color: var(--text-faint);
  user-select: none;
}

:deep(.grip:hover) {
  color: var(--text-primary);
}

:deep(.chapter-row.is-dragging) {
  opacity: 0.45;
}

/* Where a dragged chapter would land: a line above the row. */
:deep(.chapter-row.is-drop-target) {
  box-shadow: inset 0 2px 0 var(--accent);
}

:deep(.drop-end) {
  min-height: 0.4rem;
  font-size: 0.85rem;
  color: var(--text-faint);
}

:deep(.drop-end.is-active) {
  min-height: 1.6rem;
}

:deep(.drop-end.is-drop-target) {
  box-shadow: inset 0 2px 0 var(--accent);
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
