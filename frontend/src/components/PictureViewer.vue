<template>
  <div class="overlay viewer" @click.self="close">
    <figure>
      <img :src="current.src" :alt="current.alt" @click="close" />
      <figcaption v-if="current.alt || many">
        {{ current.alt }}
        <span v-if="many" class="count">{{ index + 1 }} / {{ list.length }}</span>
      </figcaption>
    </figure>
    <template v-if="many">
      <button type="button" class="viewer-nav prev" :aria-label="$t('Previous picture')" @click="step(-1)">‹</button>
      <button type="button" class="viewer-nav next" :aria-label="$t('Next picture')" @click="step(1)">›</button>
    </template>
    <button type="button" class="viewer-close" :aria-label="$t('Close')" @click="close">✕</button>
  </div>
</template>

<script setup>
// A picture at full size over the page. Click anywhere or press Escape to
// close (Escape is handled for every .overlay in App.vue). Opened with a
// list (a wiki gallery), the arrows and the arrow keys step through it.
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { viewedPicture } from '../navigation'

const props = defineProps({ picture: { type: Object, required: true } })
const close = () => (viewedPicture.value = null)

const list = computed(() => props.picture.list || [{ src: props.picture.src, alt: props.picture.alt }])
const many = computed(() => list.value.length > 1)
const index = ref(props.picture.index || 0)
watch(
  () => props.picture,
  (p) => (index.value = p.index || 0),
)
const current = computed(() => list.value[index.value] || list.value[0])

function step(by) {
  const n = list.value.length
  index.value = (index.value + by + n) % n
}

function onKey(e) {
  if (!many.value) return
  if (e.key === 'ArrowLeft') step(-1)
  else if (e.key === 'ArrowRight') step(1)
}
onMounted(() => window.addEventListener('keydown', onKey))
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))
</script>

<style scoped>
.viewer {
  z-index: 70;
  cursor: zoom-out;
}

figure {
  margin: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.6rem;
  max-width: 100%;
  max-height: 100%;
}

img {
  max-width: min(92vw, 1400px);
  max-height: 84vh;
  object-fit: contain;
  border-radius: 10px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.5);
}

figcaption {
  font-size: 0.85rem;
  color: var(--text-muted);
  text-align: center;
}

.count {
  margin-left: 0.6rem;
  color: var(--text-faint);
  font-size: 0.78rem;
}

.viewer-close,
.viewer-nav {
  position: absolute;
  border: 1px solid var(--glass-border);
  background: var(--surface);
  color: var(--text-primary);
  cursor: pointer;
}

.viewer-close {
  top: 1rem;
  right: 1rem;
  width: 2.4rem;
  height: 2.4rem;
  border-radius: 50%;
}

.viewer-nav {
  top: 50%;
  width: 2.8rem;
  height: 2.8rem;
  margin-top: -1.4rem;
  border-radius: 50%;
  font-size: 1.6rem;
  line-height: 1;
}

.viewer-nav.prev {
  left: 1rem;
}

.viewer-nav.next {
  right: 1rem;
}
</style>
