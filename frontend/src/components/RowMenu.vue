<template>
  <!-- A "…" button that opens a short list of actions: the rarely used and
       the risky ones (Delete) live here instead of beside the main ones. -->
  <div ref="root" class="row-menu" :class="{ 'is-open': open }">
    <button
      type="button"
      class="btn btn-ghost small menu-btn"
      :aria-label="label || $t('More actions')"
      :title="label || $t('More actions')"
      aria-haspopup="menu"
      :aria-expanded="open"
      @click="toggle"
    >
      …
    </button>
    <!-- In <body>, placed under the button: a row's frosted panel would
         otherwise cover it with the next row. -->
    <Teleport to="body">
      <ul v-if="open" ref="menu" class="row-menu-list" role="menu" :style="menuStyle" @keydown.esc.stop.prevent="close(true)">
        <li v-for="(item, i) in items.filter(Boolean)" :key="i" role="none">
          <button
            type="button"
            role="menuitem"
            class="menu-item"
            :class="{ 'is-danger': item.danger }"
            :disabled="item.disabled"
            @click="pick(item)"
          >
            {{ item.label }}
          </button>
        </li>
      </ul>
    </Teleport>
  </div>
</template>

<script setup>
import { ref, watch, nextTick, onBeforeUnmount } from 'vue'

defineProps({
  // [{ label (translated), action(), danger?, disabled? }]; falsy entries are skipped.
  items: { type: Array, required: true },
  label: { type: String, default: '' },
})

const root = ref(null)
const menu = ref(null)
const open = ref(false)
const menuStyle = ref({})

// Under the button, its right edge on the button's; above it when there is
// no room below.
function place() {
  const b = root.value?.getBoundingClientRect()
  const m = menu.value
  if (!b || !m) return
  const top = b.bottom + 4 + m.offsetHeight > window.innerHeight - 8 ? b.top - 4 - m.offsetHeight : b.bottom + 4
  const left = Math.min(Math.max(8, b.right - m.offsetWidth), window.innerWidth - m.offsetWidth - 8)
  menuStyle.value = { top: `${Math.max(8, top)}px`, left: `${left}px` }
}

function toggle() {
  open.value = !open.value
  if (open.value) {
    nextTick(() => {
      place()
      menu.value?.querySelector('.menu-item:not(:disabled)')?.focus()
    })
  }
}

function close(refocus = false) {
  open.value = false
  if (refocus) root.value?.querySelector('.menu-btn')?.focus()
}

function pick(item) {
  close()
  item.action()
}

// A click anywhere else, or scrolling the page, closes it.
function onPointer(e) {
  if (!root.value?.contains(e.target) && !menu.value?.contains(e.target)) close()
}
const onScroll = () => close()
function listen(on) {
  const act = on ? 'addEventListener' : 'removeEventListener'
  document[act]('pointerdown', onPointer, true)
  window[act]('scroll', onScroll, true)
  window[act]('resize', onScroll)
}
watch(open, listen)
onBeforeUnmount(() => listen(false))
defineExpose({ close })
</script>

<style scoped>
.row-menu {
  position: relative;
  display: inline-flex;
}

.menu-btn {
  min-width: 2rem;
  padding: 0 0.5rem;
  letter-spacing: 0.08em;
}

</style>

<!-- The list is teleported into <body>, so its styles aren't scoped. -->
<style>
.row-menu-list {
  position: fixed;
  z-index: 3000;
  min-width: 11rem;
  margin: 0;
  padding: 0.3rem;
  list-style: none;
  background: var(--surface);
  border: 1px solid var(--glass-border);
  border-radius: 10px;
  box-shadow: 0 14px 34px rgba(0, 0, 0, 0.45);
  font-family: 'Manrope', sans-serif;
}

.row-menu-list .menu-item {
  display: block;
  width: 100%;
  padding: 0.45rem 0.65rem;
  border: none;
  border-radius: 7px;
  background: transparent;
  color: var(--text-primary);
  font: inherit;
  font-size: 0.85rem;
  text-align: left;
  cursor: pointer;
}

.row-menu-list .menu-item:hover:not(:disabled),
.row-menu-list .menu-item:focus-visible {
  background: rgba(255, 255, 255, 0.07);
  outline: none;
}

.row-menu-list .menu-item:disabled {
  opacity: 0.4;
  cursor: default;
}

.row-menu-list .menu-item.is-danger {
  color: var(--danger-text);
}
</style>
