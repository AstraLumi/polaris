<template>
  <Transition name="tip">
    <div
      v-if="tip"
      :id="TIP_ID"
      ref="box"
      class="tooltip"
      :class="'is-' + place"
      role="tooltip"
      :style="{ left: pos.x + 'px', top: pos.y + 'px', '--arrow-x': pos.arrow + 'px' }"
    >
      <span class="tip-text">{{ parts.text }}</span>
      <span v-if="parts.keys.length" class="tip-keys">
        <kbd v-for="(k, i) in parts.keys" :key="i">{{ k }}</kbd>
      </span>
    </div>
  </Transition>
</template>

<script setup>
import { ref, reactive, computed, nextTick, onMounted, onBeforeUnmount } from 'vue'

// One tooltip for the whole app, mounted once in App.vue. Anything with a
// `title` gets it (the attribute moves to data-tip so the browser's own
// tooltip doesn't show as well), and so does:
//   data-tip="…"          a tooltip without a native title
//   data-tip-overflow     the element's own text, only while it's cut off
//   an icon-only button or link with an aria-label (×, ↑, ✎ …)
// A shortcut at the end, "Bold (Ctrl+B)", is drawn as keys.

const TIP_ID = 'app-tooltip'
const DELAY = 450 // before the first tooltip
const WARM = 350 // moving to another element within this shows at once
const GAP = 8

const tip = ref(null) // { el, text }
const place = ref('top')
const pos = reactive({ x: -9999, y: -9999, arrow: 0 })
const box = ref(null)
let timer = 0
let hiddenAt = 0

const hasLetters = (s) => /\p{L}|\p{N}/u.test(s)

function tipOf(start) {
  for (let el = start; el && el.nodeType === 1 && el !== document.body; el = el.parentElement) {
    const title = el.getAttribute('title')
    if (title) {
      el.removeAttribute('title')
      el.setAttribute('data-tip', title)
      // The title was its only name: keep it for screen readers.
      if (!el.hasAttribute('aria-label') && !hasLetters(el.textContent || '')) el.setAttribute('aria-label', title)
    }
    const text = el.getAttribute('data-tip')
    if (text) return { el, text }
    if (el.hasAttribute('data-tip-overflow')) {
      if (el.scrollWidth > el.clientWidth + 1 || el.scrollHeight > el.clientHeight + 1) {
        return { el, text: (el.textContent || '').trim() }
      }
      continue
    }
    if (el.matches('button, a, [role="button"], [role="tab"]')) {
      const label = el.getAttribute('aria-label')
      if (label && !hasLetters(el.textContent || '')) return { el, text: label }
      return null // a button without a tip of its own doesn't borrow its parent's
    }
  }
  return null
}

const parts = computed(() => {
  const text = tip.value ? tip.value.text : ''
  const m = /^(.*\S)\s*\(((?:Ctrl|Cmd|⌘|Shift|Alt|Esc|Space|Enter)[^()]*)\)$/s.exec(text)
  if (!m) return { text, keys: [] }
  return { text: m[1], keys: m[2].split('+').map((k) => k.trim()).filter(Boolean) }
})

async function show(found) {
  clearTimeout(timer)
  const prev = tip.value?.el
  if (prev && prev !== found.el) prev.removeAttribute('aria-describedby')
  tip.value = found
  if (found.el.getAttribute('aria-label') !== found.text) found.el.setAttribute('aria-describedby', TIP_ID)
  await nextTick()
  position()
}

function hide() {
  clearTimeout(timer)
  if (!tip.value) return
  tip.value.el.removeAttribute('aria-describedby')
  tip.value = null
  hiddenAt = Date.now()
}

function position() {
  if (!tip.value || !box.value) return
  const r = tip.value.el.getBoundingClientRect()
  if (!r.width && !r.height) return hide() // the element went away
  const w = box.value.offsetWidth
  const h = box.value.offsetHeight
  const vw = document.documentElement.clientWidth
  let y = r.top - h - GAP
  place.value = 'top'
  if (y < GAP) {
    y = r.bottom + GAP
    place.value = 'bottom'
  }
  const center = r.left + r.width / 2
  const x = Math.min(Math.max(GAP, center - w / 2), vw - w - GAP)
  pos.x = Math.round(x)
  pos.y = Math.round(y)
  pos.arrow = Math.round(Math.min(Math.max(10, center - x), w - 10))
}

function onOver(e) {
  if (e.pointerType === 'touch') return
  const found = tipOf(e.target)
  if (!found) {
    if (tip.value && !tip.value.el.contains(e.target)) hide()
    clearTimeout(timer)
    return
  }
  if (tip.value && tip.value.el === found.el) {
    if (tip.value.text !== found.text) show(found)
    return
  }
  clearTimeout(timer)
  const warm = tip.value || Date.now() - hiddenAt < WARM
  if (warm) show(found)
  else timer = setTimeout(() => show(found), DELAY)
}

function onOut(e) {
  if (!tip.value) {
    // Left before the delay ran out.
    if (!e.relatedTarget || !tipOf(e.relatedTarget)) clearTimeout(timer)
    return
  }
  if (e.relatedTarget && tip.value.el.contains(e.relatedTarget)) return
  hide()
}

function onFocusIn(e) {
  const el = e.target
  if (!el.matches || !el.matches(':focus-visible')) return
  const found = tipOf(el)
  if (found && found.el === el) show(found)
}

function onKey(e) {
  if (e.key === 'Escape' || !tip.value || tip.value.el !== document.activeElement) hide()
}

const listeners = [
  ['pointerover', onOver],
  ['pointerout', onOut],
  ['pointerdown', hide],
  ['focusin', onFocusIn],
  ['focusout', hide],
  ['keydown', onKey],
  ['scroll', hide],
  ['wheel', hide],
]
onMounted(() => {
  for (const [ev, fn] of listeners) document.addEventListener(ev, fn, { capture: true, passive: true })
  window.addEventListener('blur', hide)
  window.addEventListener('resize', hide)
})
onBeforeUnmount(() => {
  for (const [ev, fn] of listeners) document.removeEventListener(ev, fn, { capture: true })
  window.removeEventListener('blur', hide)
  window.removeEventListener('resize', hide)
  clearTimeout(timer)
})
</script>

<style scoped>
.tooltip {
  position: fixed;
  z-index: 1000;
  max-width: min(18rem, calc(100vw - 16px));
  padding: 0.4rem 0.6rem;
  border: 1px solid color-mix(in srgb, var(--line) 30%, var(--glass-border));
  border-radius: 8px;
  background: color-mix(in srgb, var(--surface) 96%, transparent);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.35), 0 0 0 1px rgba(0, 0, 0, 0.15);
  backdrop-filter: blur(8px);
  color: var(--text-primary);
  font-family: 'Manrope', sans-serif;
  font-size: 0.76rem;
  font-weight: 500;
  line-height: 1.45;
  white-space: pre-line;
  overflow-wrap: anywhere;
  pointer-events: none;
}

/* The little pointer, aimed at the middle of the element. */
.tooltip::after {
  content: '';
  position: absolute;
  left: var(--arrow-x);
  width: 8px;
  height: 8px;
  background: inherit;
  border: inherit;
  transform: translateX(-50%) rotate(45deg);
}

.tooltip.is-top::after {
  bottom: -5px;
  border-top: none;
  border-left: none;
}

.tooltip.is-bottom::after {
  top: -5px;
  border-bottom: none;
  border-right: none;
}

.tip-keys {
  display: inline-flex;
  gap: 0.2rem;
  margin-left: 0.45rem;
  vertical-align: 1px;
}

.tip-keys kbd {
  padding: 0 0.3rem;
  border: 1px solid var(--glass-border);
  border-bottom-width: 2px;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.06);
  color: var(--text-muted);
  font-family: inherit;
  font-size: 0.68rem;
  font-weight: 700;
}

.tip-enter-active,
.tip-leave-active {
  transition: opacity 0.12s ease, translate 0.12s ease;
}

.tip-enter-from,
.tip-leave-to {
  opacity: 0;
}

.tooltip.is-top.tip-enter-from {
  translate: 0 3px;
}

.tooltip.is-bottom.tip-enter-from {
  translate: 0 -3px;
}

@media (prefers-reduced-motion: reduce) {
  .tip-enter-active,
  .tip-leave-active {
    transition: none;
  }
}
</style>
