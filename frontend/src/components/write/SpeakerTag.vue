<template>
  <!-- In <body>, at the click (like SpellMenu). First a small menu, then the
       character picker; the keyboard shortcut opens the picker at once. -->
  <Teleport to="body">
    <ul v-if="step === 'menu'" ref="root" class="row-menu-list speaker-tag-menu" role="menu" :aria-label="$t('Tag character')" :style="style" @keydown="onMenuKey">
      <li role="none">
        <button type="button" role="menuitem" class="menu-item" @mousedown.prevent @click="step = 'pick'">{{ hasTag ? $t('Change character…') : $t('Tag character…') }}</button>
      </li>
      <li v-if="hasTag" role="none">
        <button type="button" role="menuitem" class="menu-item" @mousedown.prevent @click="$emit('remove')">{{ $t('Remove tag') }}</button>
      </li>
    </ul>
    <div v-else ref="root" class="row-menu-list speaker-tag-pick" role="dialog" :aria-label="$t('Who said this?')" :style="style" @mousedown.stop>
      <input ref="inputEl" v-model="typed" type="text" maxlength="200" class="speaker-tag-input" :placeholder="$t('Character…')" :aria-label="$t('Who said this?')" @keydown="onPickKey" />
      <ul class="speaker-tag-list" role="listbox">
        <li v-for="(c, i) in shown" :key="c.id" role="none">
          <button type="button" role="option" :aria-selected="i === at" class="menu-item" :class="{ 'is-on': i === at }" @mousedown.prevent @click="$emit('pick', { id: c.id, name: c.name })">{{ c.name }}</button>
        </li>
        <li v-if="!shown.length && !typed.trim()" class="spell-note" role="none">{{ $t('No characters yet') }}</li>
      </ul>
      <button v-if="typed.trim() && !exact" type="button" class="menu-item speaker-tag-other" @mousedown.prevent @click="$emit('pick', { id: null, name: typed.trim() })">{{ $t('Use “{name}” (not a character)', { name: typed.trim() }) }}</button>
    </div>
  </Teleport>
</template>

<script setup>
// "Tag character" (docs/EDITOR.md): the right-click menu on selected prose
// and the picker of the story's characters. Type to filter, ↑/↓ and Enter
// pick, Esc or a click elsewhere closes. A typed name that is not a
// character can be used too (a guard, a voice).
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount } from 'vue'

const props = defineProps({
  x: { type: Number, required: true },
  y: { type: Number, required: true },
  characters: { type: Array, default: () => [] }, // [{ id, name }]
  hasTag: { type: Boolean, default: false },
  direct: { type: Boolean, default: false }, // open the picker at once
  current: { type: String, default: '' },
})
const emit = defineEmits(['pick', 'remove', 'close'])

const step = ref(props.direct ? 'pick' : 'menu')
const root = ref(null)
const inputEl = ref(null)
const typed = ref('')
const at = ref(0)

const fold = (s) => String(s).normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase()
const shown = computed(() => {
  const q = fold(typed.value.trim())
  const list = q ? props.characters.filter((c) => fold(c.name).includes(q)) : props.characters
  return [...list].sort((a, b) => (fold(a.name).startsWith(q) ? 0 : 1) - (fold(b.name).startsWith(q) ? 0 : 1)).slice(0, 40)
})
const exact = computed(() => props.characters.some((c) => fold(c.name) === fold(typed.value.trim())))

const style = computed(() => {
  const vw = window.innerWidth
  const vh = window.innerHeight
  const w = 240
  const h = step.value === 'pick' ? 280 : 90
  const left = Math.max(8, Math.min(props.x, vw - w - 8))
  const top = props.y + h + 8 > vh ? Math.max(8, props.y - h - 4) : props.y + 4
  return { left: `${left}px`, top: `${top}px` }
})

function focusFirst() {
  nextTick(() => {
    if (step.value === 'pick') inputEl.value?.focus()
    else root.value?.querySelector('.menu-item')?.focus()
  })
}
function onMenuKey(e) {
  const items = [...root.value.querySelectorAll('.menu-item')]
  const i = items.indexOf(document.activeElement)
  if (e.key === 'ArrowDown') items[(i + 1) % items.length].focus()
  else if (e.key === 'ArrowUp') items[(i - 1 + items.length) % items.length].focus()
  else return
  e.preventDefault()
}
function onPickKey(e) {
  if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
    e.preventDefault()
    const n = shown.value.length
    if (n) at.value = (at.value + (e.key === 'ArrowDown' ? 1 : n - 1)) % n
  } else if (e.key === 'Enter') {
    e.preventDefault()
    const c = shown.value[at.value]
    if (c) emit('pick', { id: c.id, name: c.name })
    else if (typed.value.trim()) emit('pick', { id: null, name: typed.value.trim() })
  }
}
const onDown = (e) => {
  if (!root.value?.contains(e.target)) emit('close')
}
const onKeyEsc = (e) => {
  if (e.key === 'Escape') {
    e.preventDefault()
    emit('close')
  }
}
onMounted(() => {
  focusFirst()
  document.addEventListener('mousedown', onDown, true)
  document.addEventListener('keydown', onKeyEsc, true)
})
onBeforeUnmount(() => {
  document.removeEventListener('mousedown', onDown, true)
  document.removeEventListener('keydown', onKeyEsc, true)
})
watch(step, focusFirst)
watch(typed, () => (at.value = 0))
</script>

<style scoped>
.speaker-tag-pick {
  position: fixed;
  z-index: 60;
  width: 15rem;
  padding: 0.4rem;
  display: grid;
  gap: 0.3rem;
}
.speaker-tag-menu {
  position: fixed;
  z-index: 60;
}
.speaker-tag-input {
  width: 100%;
  box-sizing: border-box;
  padding: 0.3rem 0.5rem;
  font: inherit;
}
.speaker-tag-list {
  list-style: none;
  margin: 0;
  padding: 0;
  max-height: 11rem;
  overflow: auto;
}
.speaker-tag-list .is-on {
  background: var(--accent-soft, rgba(200, 150, 50, 0.18));
}
.speaker-tag-other {
  text-align: left;
}
</style>
