<template>
  <div class="overlay viewer" @click.self="close">
    <figure>
      <img :src="picture.src" :alt="picture.alt" @click="close" />
      <figcaption v-if="picture.alt">{{ picture.alt }}</figcaption>
    </figure>
    <button type="button" class="viewer-close" :aria-label="$t('Close')" @click="close">✕</button>
  </div>
</template>

<script setup>
// A picture at full size over the page. Click anywhere or press Escape to
// close (Escape is handled for every .overlay in App.vue).
import { viewedPicture } from '../navigation'

defineProps({ picture: { type: Object, required: true } })
const close = () => (viewedPicture.value = null)
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
}

.viewer-close {
  position: absolute;
  top: 1rem;
  right: 1rem;
  width: 2.4rem;
  height: 2.4rem;
  border-radius: 50%;
  border: 1px solid var(--glass-border);
  background: var(--surface);
  color: var(--text-primary);
  cursor: pointer;
}
</style>
