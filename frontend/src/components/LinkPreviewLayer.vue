<template>
  <Transition name="preview">
    <div
      v-if="article"
      ref="box"
      class="link-preview glass-panel panel-solid"
      :style="{ left: pos.x + 'px', top: pos.y + 'px' }"
      role="tooltip"
    >
      <ArticleCard :article="article" />
    </div>
  </Transition>
</template>

<script setup>
import { ref, reactive, nextTick, onMounted, onBeforeUnmount } from 'vue'
import ArticleCard from './ArticleCard.vue'
import { articleOfLink, loadArticle } from '../articlePreview'

// Hovering any link to a wiki article ([[links]] in text, related lists, the
// wiki's index) shows a card with the article's picture, a few facts and the
// start of its text. Mounted once in App.vue, like TooltipLayer.

const DELAY = 450
const GAP = 8

const article = ref(null)
const box = ref(null)
const pos = reactive({ x: -9999, y: -9999 })
let target = null // the link being hovered
let timer = 0
let seq = 0

function hide() {
  clearTimeout(timer)
  seq++
  target = null
  article.value = null
}

async function show(link) {
  const mine = ++seq
  let a
  try {
    a = await loadArticle(link.type, link.id)
  } catch (e) {
    return
  }
  if (mine !== seq || target !== link.el || !link.el.isConnected) return
  article.value = a
  await nextTick()
  position(link.el)
}

function position(el) {
  if (!box.value) return
  const r = el.getBoundingClientRect()
  const w = box.value.offsetWidth
  const h = box.value.offsetHeight
  const vw = document.documentElement.clientWidth
  const vh = document.documentElement.clientHeight
  let y = r.bottom + GAP
  if (y + h > vh - GAP && r.top - h - GAP > GAP) y = r.top - h - GAP
  pos.x = Math.round(Math.min(Math.max(GAP, r.left), vw - w - GAP))
  pos.y = Math.round(y)
}

function onOver(e) {
  if (e.pointerType === 'touch') return
  const link = articleOfLink(e.target)
  if (!link) return
  if (link.el === target) return
  hide()
  target = link.el
  timer = setTimeout(() => show(link), DELAY)
}

function onOut(e) {
  if (!target) return
  if (e.relatedTarget && target.contains(e.relatedTarget)) return
  hide()
}

const listeners = [
  ['pointerover', onOver],
  ['pointerout', onOut],
  ['pointerdown', hide],
  ['keydown', hide],
  ['scroll', hide],
  ['wheel', hide],
]
onMounted(() => {
  for (const [ev, fn] of listeners) document.addEventListener(ev, fn, { capture: true, passive: true })
  window.addEventListener('blur', hide)
})
onBeforeUnmount(() => {
  for (const [ev, fn] of listeners) document.removeEventListener(ev, fn, { capture: true })
  window.removeEventListener('blur', hide)
  clearTimeout(timer)
})
</script>

<style scoped>
.link-preview {
  position: fixed;
  z-index: 990;
  width: min(20rem, calc(100vw - 16px));
  padding: 0.8rem 0.9rem;
  border-radius: 12px;
  box-shadow: 0 12px 32px rgba(0, 0, 0, 0.4);
  pointer-events: none;
}

.preview-enter-active,
.preview-leave-active {
  transition: opacity 0.14s ease, translate 0.14s ease;
}

.preview-enter-from,
.preview-leave-to {
  opacity: 0;
  translate: 0 4px;
}

@media (prefers-reduced-motion: reduce) {
  .preview-enter-active,
  .preview-leave-active {
    transition: none;
  }
}
</style>
