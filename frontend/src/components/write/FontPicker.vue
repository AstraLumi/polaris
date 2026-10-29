<template>
  <div class="font-picker" :class="$attrs.class">
    <input
      ref="input"
      type="text"
      class="fp-input"
      role="combobox"
      aria-autocomplete="list"
      :aria-expanded="open"
      :aria-label="$t('Font')"
      :title="$t('Font')"
      :placeholder="mixed ? $t('Several fonts') : ''"
      :value="open && typed ? query : value"
      data-font-picker
      @focus="onFocus"
      @input="onInput"
      @keydown="onKey"
      @blur="close"
    />
    <div v-if="open" ref="listEl" class="fp-list glass-panel panel-solid" role="listbox" data-font-list @mousedown.prevent>
      <button
        v-if="!typed"
        type="button"
        role="option"
        class="fp-item fp-style"
        :class="{ hi: hi === 0 }"
        :aria-selected="hi === 0"
        @click="pick('')"
      >
        {{ $t("The style's font") }}
      </button>
      <template v-for="s in sections" :key="s.key">
        <div class="fp-head">{{ $t(HEADS[s.key]) }}</div>
        <div v-for="f in s.fonts" :key="s.key + f" class="fp-row" :class="{ hi: hi === indexOf(f) }" :data-font-option="f">
          <button type="button" role="option" class="fp-item" :aria-selected="hi === indexOf(f)" :style="{ fontFamily: css(f) }" @click="pick(f)">
            <span class="fp-check">{{ same(f, value) ? '✓' : '' }}</span>{{ f }}
          </button>
          <button
            type="button"
            class="fp-star"
            :class="{ on: isFavourite(f) }"
            :aria-pressed="isFavourite(f)"
            :title="isFavourite(f) ? $t('Remove from favourites') : $t('Add to favourites')"
            :aria-label="isFavourite(f) ? $t('Remove from favourites') : $t('Add to favourites')"
            data-font-star
            @click.stop="favourites = toggleFavourite(favourites, f)"
          >
            {{ isFavourite(f) ? '★' : '☆' }}
          </button>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup>
// The toolbar's font box (docs/EDITOR.md, "Font picker"): type to pull the
// best matches to the top (every font stays listed), favourites (★) and the
// last fonts used next, each name in its own font where the browser has it.
// Enter or a click puts the font on the selection, even the one shown, so a
// line in several fonts can be made one; Esc or a click away changes
// nothing. Favourites and recent fonts are kept on this computer.
import { ref, computed, watch, nextTick } from 'vue'
import { tr } from '../../i18n'
import { fontSections, pushRecent, toggleFavourite } from '../../editor/fontSearch.js'
import { BUNDLED_FONTS } from '../../editor/bundledFonts.js'
import { loadFont } from '../../editor/fonts.js'
import { fontLibrary } from '../../fontLibrary.js'

defineOptions({ inheritAttrs: false })
const props = defineProps({
  value: { type: String, default: '' }, // the selection's font ('' when mixed)
  mixed: { type: Boolean, default: false }, // the selection is in several fonts
  fonts: { type: Array, default: () => [] },
})
const emit = defineEmits(['pick'])

const HEADS = { matches: tr('Best matches'), favourites: tr('Favourites'), recent: tr('Recently used'), all: tr('All fonts') }
const css = (f) => `"${String(f).replace(/["\\]/g, '')}", var(--font-ui, sans-serif)`
const same = (a, b) => !!a && !!b && a.toLowerCase() === b.toLowerCase()

const stored = (key) => {
  try {
    const v = JSON.parse(localStorage.getItem(key) || '[]')
    return Array.isArray(v) ? v.filter((x) => typeof x === 'string').slice(0, 40) : []
  } catch {
    return []
  }
}
const FAV_KEY = 'polaris-write-font-favourites'
const RECENT_KEY = 'polaris-write-font-recent'
const favourites = ref(stored(FAV_KEY))
const recent = ref(stored(RECENT_KEY))
const keep = (key, list) => {
  try {
    localStorage.setItem(key, JSON.stringify(list))
  } catch {
    /* kept for this visit only */
  }
}
watch(favourites, (v) => keep(FAV_KEY, v))
watch(recent, (v) => keep(RECENT_KEY, v))
const isFavourite = (f) => favourites.value.some((x) => same(x, f))

const input = ref(null)
const listEl = ref(null)
const open = ref(false)
const typed = ref(false) // something was typed since the box was opened
const query = ref('')
const hi = ref(-1) // the highlighted row (0 is the style's font while not typing)

const sections = computed(() => fontSections({ fonts: props.fonts, favourites: favourites.value, recent: recent.value, query: typed.value ? query.value : '' }))
// The rows in order, for the arrow keys: the style's font first while not typing.
const flat = computed(() => [...(typed.value ? [] : [null]), ...sections.value.flatMap((s) => s.fonts)])
const indexOf = (f) => flat.value.indexOf(f, typed.value ? 0 : 1)

// The fonts that ship with the app or are in the library are on this
// server: loaded once so the list shows each in its own face.
let previewed = false
function preview() {
  if (previewed) return
  previewed = true
  for (const f of [...BUNDLED_FONTS.map((b) => b.family), ...fontLibrary.value.map((f) => f.name)]) loadFont(f).catch(() => {})
}

function onFocus(e) {
  open.value = true
  typed.value = false
  query.value = ''
  hi.value = Math.max(-1, indexOf(props.value))
  preview()
  e.target.select()
  nextTick(showHi)
}
function onInput(e) {
  typed.value = true
  query.value = e.target.value
  hi.value = query.value.trim() && sections.value[0]?.key === 'matches' ? 0 : -1
  nextTick(() => listEl.value && (listEl.value.scrollTop = 0))
}
function showHi() {
  listEl.value?.querySelectorAll('[role=option]')[hi.value]?.scrollIntoView({ block: 'nearest' })
}
function onKey(e) {
  if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
    e.preventDefault()
    if (!open.value) return onFocus(e)
    const n = flat.value.length
    hi.value = e.key === 'ArrowDown' ? (hi.value + 1) % n : (hi.value - 1 + n) % n
    nextTick(showHi)
  } else if (e.key === 'Enter') {
    e.preventDefault()
    if (hi.value >= 0 && hi.value < flat.value.length) pick(flat.value[hi.value] ?? '')
    else pick(typed.value ? query.value.trim() : props.value)
  } else if (e.key === 'Escape') {
    e.preventDefault()
    input.value?.blur()
  }
}
function close() {
  open.value = false
  typed.value = false
}
function pick(f) {
  const family = String(f || '').trim()
  if (family) recent.value = pushRecent(recent.value, family)
  close()
  input.value?.blur()
  emit('pick', family)
}
</script>

<style scoped>
.font-picker {
  position: relative;
  display: inline-block;
}

.fp-input {
  width: 100%;
}

.fp-list {
  position: absolute;
  top: calc(100% + 0.3rem);
  left: 0;
  z-index: 40;
  width: max(100%, 17rem);
  max-height: min(24rem, 70vh);
  overflow-y: auto;
  padding: 0.3rem;
}

.fp-head {
  padding: 0.45rem 0.5rem 0.15rem;
  font: 600 0.7rem var(--font-ui, sans-serif);
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-faint);
}

.fp-row {
  display: flex;
  align-items: center;
  border-radius: 6px;
}

.fp-item {
  flex: 1;
  min-width: 0;
  padding: 0.3rem 0.5rem;
  border: none;
  border-radius: 6px;
  background: none;
  color: inherit;
  font-size: 1rem;
  text-align: left;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  cursor: pointer;
}

.fp-style {
  width: 100%;
  font-size: 0.85rem;
  color: var(--text-muted);
}

.fp-check {
  display: inline-block;
  width: 1.1rem;
  font-family: var(--font-ui, sans-serif);
  color: var(--accent);
}

.fp-row:hover,
.fp-row.hi,
.fp-style:hover,
.fp-style.hi {
  background: var(--glass);
}

.fp-row.hi,
.fp-style.hi {
  outline: 1px solid var(--accent);
}

.fp-star {
  padding: 0.2rem 0.45rem;
  border: none;
  background: none;
  color: var(--text-faint);
  font-size: 0.95rem;
  cursor: pointer;
  opacity: 0;
}

.fp-row:hover .fp-star,
.fp-row.hi .fp-star,
.fp-star.on,
.fp-star:focus-visible {
  opacity: 1;
}

.fp-star.on {
  color: #d4a72c;
}
</style>
