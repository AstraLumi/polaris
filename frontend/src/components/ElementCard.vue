<template>
  <article :id="'element-' + element.id" class="element glass-panel" :class="{ 'is-target': target }">
    <button type="button" class="cover" :title="$t('View the pictures')" :disabled="!pictures.length" @click="view">
      <img v-if="pictures.length" :src="pictures[0].src" :alt="element.name" loading="lazy" />
      <span v-else class="no-pic">{{ $t('No picture') }}</span>
      <span v-if="pictures.length > 1" class="count">{{ pictures.length }}</span>
    </button>
    <div class="body">
      <div class="title">
        <strong>{{ element.name }}</strong>
        <RowMenu v-if="editable" :label="$t('More')" :items="[
          { label: $t('Edit'), action: () => $emit('edit', element) },
          { label: $t('Delete'), action: () => $emit('delete', element), danger: true },
        ]" />
      </div>
      <small class="kind">
        {{ kindLabel }}
        <template v-if="showCulture"> · <RouterLink :to="`/cultures/${element.culture_id}`">{{ element.culture_name }}</RouterLink></template>
      </small>
      <p v-if="element.description" class="desc">{{ linkText(element.description) }}</p>
      <div v-if="element.tags.length" class="tags">
        <button v-for="tg in element.tags" :key="tg" type="button" class="tag" @click="$emit('tag', tg)">#{{ tg }}</button>
      </div>
      <div v-if="refs.length" class="refs">
        <span class="refs-label">{{ $t('Based on') }}</span>
        <RouterLink v-for="r in refs" :key="r.id" :to="{ path: '/references', query: { ref: r.id } }" class="ref" :title="r.title">
          <img v-if="r.pictures.length" :src="r.pictures[0].url" :alt="r.title" loading="lazy" />
          <span v-else>{{ r.title.slice(0, 1) }}</span>
        </RouterLink>
      </div>
    </div>
  </article>
</template>

<script setup>
// One element of a culture (an outfit, a dish, a festival…): its first
// picture (all of them in the viewer), kind, text, tags, and the library
// references it is based on (missing ones, deleted from the library, are
// skipped).
import { computed } from 'vue'
import { t } from '../i18n'
import { viewGallery } from '../navigation'
import { linkText } from '../linkNames'
import { ELEMENT_KINDS } from '../cultures'
import { referenceLibrary } from '../referenceLibrary'
import RowMenu from './RowMenu.vue'

const props = defineProps({
  element: { type: Object, required: true },
  showCulture: { type: Boolean, default: false },
  editable: { type: Boolean, default: false },
  target: { type: Boolean, default: false },
})
defineEmits(['edit', 'delete', 'tag'])

const pictures = computed(() => props.element.pictures.map((p) => ({ src: p.url, alt: props.element.name })))
const kindLabel = computed(() => t(ELEMENT_KINDS.find((k) => k.id === props.element.kind)?.label || props.element.kind))
const refs = computed(() => props.element.reference_ids.map((id) => referenceLibrary.value.find((r) => r.id === id)).filter(Boolean))

function view() {
  viewGallery(pictures.value)
}
</script>

<style scoped>
.element {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  padding: 0;
}

.element.is-target {
  outline: 2px solid var(--accent);
}

.cover {
  position: relative;
  aspect-ratio: 3 / 4;
  padding: 0;
  border: 0;
  background: rgba(var(--wash), 0.04);
  cursor: zoom-in;
}

.cover:disabled {
  cursor: default;
}

.cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.no-pic {
  color: var(--text-faint);
  font-size: 0.85rem;
}

.count {
  position: absolute;
  right: 0.5rem;
  bottom: 0.5rem;
  padding: 0.1rem 0.5rem;
  border-radius: 999px;
  background: var(--surface);
  color: var(--text-primary);
  font-size: 0.75rem;
}

.body {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
  padding: 0.75rem 0.85rem 0.85rem;
  min-width: 0;
}

.title {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.5rem;
}

.title strong {
  color: var(--text-primary);
  overflow-wrap: anywhere;
}

.kind {
  color: var(--accent-text);
  font-size: 0.75rem;
  font-weight: 700;
}

.desc {
  margin: 0;
  font-size: 0.85rem;
  color: var(--text-muted);
  white-space: pre-line;
  display: -webkit-box;
  -webkit-line-clamp: 5;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.tags {
  display: flex;
  flex-wrap: wrap;
  gap: 0.3rem;
}

.tag {
  padding: 0.1rem 0.5rem;
  border-radius: 999px;
  border: 1px solid var(--glass-border);
  background: rgba(var(--wash), 0.03);
  color: var(--text-muted);
  font: inherit;
  font-size: 0.75rem;
  cursor: pointer;
}

.tag:hover {
  border-color: var(--accent);
  color: var(--text-primary);
}

.refs {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.3rem;
  margin-top: 0.2rem;
}

.refs-label {
  font-size: 0.75rem;
  color: var(--text-faint);
  margin-right: 0.15rem;
}

.ref {
  width: 2.2rem;
  height: 2.2rem;
  border-radius: 6px;
  overflow: hidden;
  border: 1px solid var(--glass-border);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  text-decoration: none;
  font-size: 0.8rem;
}

.ref:hover {
  border-color: var(--accent);
}

.ref img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
</style>
