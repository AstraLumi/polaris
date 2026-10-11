<template>
  <div class="wiki-gallery" :class="{ 'is-editing': editable }">
    <ul v-if="items.length" class="grid">
      <li v-for="(g, i) in items" :key="g.id" class="item">
        <button type="button" class="thumb" :title="g.caption || $t('View full size')" @click="open(i)">
          <img :src="g.picture" :alt="g.caption" loading="lazy" />
        </button>
        <p v-if="!editable && g.caption" class="caption">{{ g.caption }}</p>
        <template v-if="editable">
          <input
            v-model="drafts[g.id]"
            type="text"
            class="caption-input"
            maxlength="300"
            :placeholder="$t('Caption')"
            :aria-label="$t('Caption')"
            @change="saveCaption(g)"
            @keydown.enter.prevent="$event.target.blur()"
          />
          <div class="item-actions">
            <button type="button" class="btn btn-ghost small" :disabled="busy || i === 0" :aria-label="$t('Move left')" @click="move(i, -1)">←</button>
            <button type="button" class="btn btn-ghost small" :disabled="busy || i === items.length - 1" :aria-label="$t('Move right')" @click="move(i, 1)">→</button>
            <button type="button" class="btn btn-ghost small" :disabled="busy" @click="remove(g)">{{ $t('Remove') }}</button>
          </div>
        </template>
      </li>
    </ul>

    <div v-if="editable" class="add">
      <button type="button" class="btn btn-ghost small" :disabled="busy" @click="fileInput.click()">
        {{ busy ? $t('Uploading…') : $t('Add pictures') }}
      </button>
      <input ref="fileInput" type="file" accept="image/*" multiple hidden @change="onFiles" />
      <span class="note">{{ $t('Pictures are saved as soon as you add or change them.') }}</span>
    </div>
    <p v-if="error" class="error-banner">{{ error }}</p>
  </div>
</template>

<script setup>
import { ref, reactive, watch } from 'vue'
import { wikiApi } from '../wiki'
import { viewGallery } from '../navigation'
import { t } from '../i18n'

// A wiki article's picture gallery (backend/wikigallery.go): a grid that
// opens the pictures full size, and in edit mode captions, order, removal and
// uploads. Gallery changes save at once and come back as `update:items`.
const props = defineProps({
  type: { type: String, required: true },
  id: { type: [String, Number], required: true },
  items: { type: Array, default: () => [] },
  editable: { type: Boolean, default: false },
})
const emit = defineEmits(['update:items'])

const fileInput = ref(null)
const busy = ref(false)
const error = ref('')
const drafts = reactive({})

watch(
  () => props.items,
  (list) => {
    for (const g of list) if (!(g.id in drafts) || !busy.value) drafts[g.id] = g.caption
  },
  { immediate: true },
)

const open = (i) => viewGallery(props.items.map((g) => ({ src: g.picture, alt: g.caption })), i)

async function run(fn) {
  busy.value = true
  error.value = ''
  try {
    emit('update:items', await fn())
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

function onFiles(e) {
  const files = [...e.target.files] // read before clearing the input
  e.target.value = ''
  if (files.length) run(() => wikiApi.galleryAdd(props.type, props.id, files))
}

function saveCaption(g) {
  const caption = (drafts[g.id] || '').trim()
  if (caption !== g.caption) run(() => wikiApi.galleryCaption(g.id, caption))
}

function move(i, by) {
  const ids = props.items.map((g) => g.id)
  ;[ids[i], ids[i + by]] = [ids[i + by], ids[i]]
  run(() => wikiApi.galleryOrder(props.type, props.id, ids))
}

function remove(g) {
  if (window.confirm(t('Remove this picture?'))) run(() => wikiApi.galleryRemove(g.id))
}
</script>

<style scoped>
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(10rem, 1fr));
  gap: 0.9rem;
  margin: 0 0 0.8rem;
  padding: 0;
  list-style: none;
}

.item {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  min-width: 0;
}

.thumb {
  display: block;
  aspect-ratio: 4 / 3;
  padding: 0;
  overflow: hidden;
  border: 1px solid var(--glass-border);
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.04);
  cursor: zoom-in;
}

.thumb img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  transition: transform 0.2s ease;
}

.thumb:hover img {
  transform: scale(1.04);
}

.caption {
  margin: 0;
  font-size: 0.8rem;
  line-height: 1.4;
  color: var(--text-muted);
  overflow-wrap: anywhere;
}

.caption-input {
  min-height: 0;
  padding: 0.3rem 0.5rem;
  font-size: 0.8rem;
}

.item-actions {
  display: flex;
  gap: 0.25rem;
}

.item-actions .btn:last-child {
  margin-left: auto;
}

.add {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.4rem 0.8rem;
}

.note {
  font-size: 0.76rem;
  color: var(--text-faint);
}

@media (prefers-reduced-motion: reduce) {
  .thumb img {
    transition: none;
  }
}
</style>
