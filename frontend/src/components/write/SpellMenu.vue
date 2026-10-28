<template>
  <!-- In <body>, at the click: the paper's zoom and frosted panels can't
       touch it (like RowMenu's list). -->
  <Teleport to="body">
    <ul ref="root" class="row-menu-list spell-menu" role="menu" :aria-label="$t('Spelling: {word}', { word })" :style="style" @keydown="onKey">
      <li v-if="suggestions === null" class="spell-note" role="none">{{ $t('Looking for suggestions…') }}</li>
      <li v-else-if="!suggestions.length" class="spell-note" role="none">{{ $t('No suggestions') }}</li>
      <li v-for="s in suggestions || []" :key="s" role="none">
        <button type="button" role="menuitem" class="menu-item spell-sug" @mousedown.prevent @click="$emit('pick', s)">{{ s }}</button>
      </li>
      <li class="spell-sep" role="separator"></li>
      <li role="none">
        <button type="button" role="menuitem" class="menu-item" @mousedown.prevent @click="$emit('add')">{{ $t('Add to dictionary') }}</button>
      </li>
      <li role="none">
        <button type="button" role="menuitem" class="menu-item" @mousedown.prevent @click="$emit('ignore')">{{ $t('Ignore') }}</button>
      </li>
      <li role="none">
        <button type="button" role="menuitem" class="menu-item" @mousedown.prevent @click="$emit('options')">{{ $t('Spelling options…') }}</button>
      </li>
    </ul>
  </Teleport>
</template>

<script setup>
// The spellchecker's menu on an underlined word: up to six suggestions,
// "Add to dictionary" (the story's), "Ignore" (until the page is left) and
// the spelling panel. ↑/↓ move, Esc or a click elsewhere closes.
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount } from 'vue'

const props = defineProps({
  word: { type: String, required: true },
  x: { type: Number, required: true },
  y: { type: Number, required: true },
  suggestions: { type: Array, default: null }, // null while looking
})
const emit = defineEmits(['pick', 'add', 'ignore', 'options', 'close'])

const root = ref(null)
const size = ref({ w: 220, h: 200 })
const style = computed(() => {
  const vw = window.innerWidth
  const vh = window.innerHeight
  const left = Math.max(8, Math.min(props.x, vw - size.value.w - 8))
  const top = props.y + size.value.h + 8 > vh ? Math.max(8, props.y - size.value.h - 4) : props.y + 4
  return { left: `${left}px`, top: `${top}px` }
})

function measure() {
  const el = root.value
  if (el) size.value = { w: el.offsetWidth, h: el.offsetHeight }
}
watch(() => props.suggestions, () => nextTick(measure))

function items() {
  return [...(root.value?.querySelectorAll('.menu-item') || [])]
}
function onKey(e) {
  const list = items()
  const i = list.indexOf(document.activeElement)
  if (e.key === 'Escape') {
    e.preventDefault()
    e.stopPropagation()
    emit('close')
  } else if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
    e.preventDefault()
    const next = (i + (e.key === 'ArrowDown' ? 1 : -1) + list.length) % list.length
    list[next]?.focus()
  } else if (e.key === 'Tab') emit('close')
}

const onPointer = (e) => {
  if (!root.value?.contains(e.target)) emit('close')
}
const onScroll = (e) => {
  if (!root.value?.contains(e.target)) emit('close')
}
const onDocKey = (e) => {
  if (e.key === 'Escape') emit('close')
}
onMounted(() => {
  measure()
  document.addEventListener('pointerdown', onPointer, true)
  window.addEventListener('scroll', onScroll, true)
  window.addEventListener('resize', onScroll)
  document.addEventListener('keydown', onDocKey)
})
onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', onPointer, true)
  window.removeEventListener('scroll', onScroll, true)
  window.removeEventListener('resize', onScroll)
  document.removeEventListener('keydown', onDocKey)
})
defineExpose({ focusFirst: () => items()[0]?.focus() })
</script>

<style>
.spell-menu {
  max-width: 18rem;
}

.spell-menu .spell-sug {
  font-weight: 700;
}

.spell-menu .spell-note {
  padding: 0.45rem 0.65rem;
  font-size: 0.82rem;
  color: var(--text-faint);
}

.spell-menu .spell-sep {
  height: 1px;
  margin: 0.25rem 0.3rem;
  background: var(--glass-border);
}
</style>
