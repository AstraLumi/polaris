<template>
  <div class="field picture-list" @paste="onPaste">
    <span>{{ label || $t('Pictures') }}</span>
    <ul v-if="kept.length || added.length" class="thumbs">
      <li v-for="(p, i) in kept" :key="'k' + p.id" class="thumb">
        <img :src="p.url" alt="" />
        <div class="thumb-actions">
          <button v-if="i > 0" type="button" :title="$t('Move earlier')" :aria-label="$t('Move earlier')" @click="moveEarlier(i)">‹</button>
          <button type="button" :title="$t('Remove')" :aria-label="$t('Remove')" @click="removeKept(i)">✕</button>
        </div>
      </li>
      <li v-for="(f, i) in added" :key="'n' + i + f.name" class="thumb is-new">
        <img :src="urls[i]" alt="" />
        <div class="thumb-actions">
          <button type="button" :title="$t('Remove')" :aria-label="$t('Remove')" @click="removeAdded(i)">✕</button>
        </div>
      </li>
    </ul>
    <FilePick ref="pick" multiple :label="$t('Add pictures')" @change="onFiles" />
    <p class="hint">{{ $t('You can also paste a picture here (Ctrl+V).') }}</p>
  </div>
</template>

<script setup>
// The pictures of a reference or a culture element: the ones it has
// (moved or removed here) and new ones (picked, dropped or pasted), which
// the server puts after them. The parent reads `kept`, `removed` and `added`
// through v-model:state and sends them as order / remove_pictures / pictures.
import { ref, computed, watch, onBeforeUnmount } from 'vue'
import { pastedPictures } from '../cultures'
import FilePick from './FilePick.vue'

const props = defineProps({
  pictures: { type: Array, default: () => [] }, // [{ id, url }] it has
  label: { type: String, default: '' }, // already translated
})
const emit = defineEmits(['update:state'])

const kept = ref([...props.pictures])
const added = ref([])
const urls = ref([])
const removed = computed(() => props.pictures.filter((p) => !kept.value.some((k) => k.id === p.id)).map((p) => p.id))

watch(
  [kept, added],
  () => emit('update:state', { order: kept.value.map((p) => p.id), removed: removed.value, added: added.value }),
  { deep: true, immediate: true },
)

function addFiles(files) {
  const pics = [...files].filter((f) => String(f.type).startsWith('image/'))
  if (!pics.length) return
  added.value = [...added.value, ...pics]
  urls.value = [...urls.value, ...pics.map((f) => URL.createObjectURL(f))]
}

function onFiles(e) {
  // Read the files before clearing the input (see dropFile(keepInput)).
  const files = [...(e.target.files || [])]
  e.target.value = ''
  addFiles(files)
}

function onPaste(e) {
  const files = pastedPictures(e)
  if (files.length) {
    e.preventDefault()
    addFiles(files)
  }
}

function moveEarlier(i) {
  const list = [...kept.value]
  ;[list[i - 1], list[i]] = [list[i], list[i - 1]]
  kept.value = list
}

function removeKept(i) {
  kept.value = kept.value.filter((_, j) => j !== i)
}

function removeAdded(i) {
  URL.revokeObjectURL(urls.value[i])
  added.value = added.value.filter((_, j) => j !== i)
  urls.value = urls.value.filter((_, j) => j !== i)
}

onBeforeUnmount(() => urls.value.forEach((u) => URL.revokeObjectURL(u)))

// A paste anywhere in the form lands here too (the forms forward it).
defineExpose({ onPaste })
</script>

<style scoped>
.thumbs {
  list-style: none;
  margin: 0 0 0.5rem;
  padding: 0;
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.thumb {
  position: relative;
  width: 6rem;
  height: 6rem;
  border-radius: 10px;
  overflow: hidden;
  border: 1px solid var(--glass-border);
  background: rgba(var(--wash), 0.04);
}

.thumb.is-new {
  border-color: var(--accent);
}

.thumb img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.thumb-actions {
  position: absolute;
  top: 0.2rem;
  right: 0.2rem;
  display: flex;
  gap: 0.2rem;
}

.thumb-actions button {
  width: 1.6rem;
  height: 1.6rem;
  border-radius: 6px;
  border: 1px solid var(--glass-border);
  background: var(--surface);
  color: var(--text-primary);
  font: inherit;
  font-size: 0.8rem;
  line-height: 1;
  cursor: pointer;
}

.hint {
  margin: 0.3rem 0 0;
  font-size: 0.76rem;
  color: var(--text-faint);
}
</style>
