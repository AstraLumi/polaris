<template>
  <aside class="side-panel glass-panel panel-solid symbols-panel" :aria-label="$t('Symbols')" data-symbols-panel>
    <header class="side-head">
      <h2>{{ $t('Symbols') }}</h2>
      <button type="button" class="btn btn-ghost small" :aria-label="$t('Close')" @click="$emit('close')">✕</button>
    </header>

    <label class="field">
      <span>{{ $t('Font') }}</span>
      <select v-model="family" data-symbols-font>
        <option value="">{{ $t('Body font ({font})', { font: bodyFont }) }}</option>
        <optgroup v-if="bookFonts.length" :label="$t('This book')">
          <option v-for="f in bookFonts" :key="f" :value="f">{{ f }}</option>
        </optgroup>
        <optgroup v-for="g in groups" :key="g.label" :label="g.label">
          <option v-for="f in g.fonts" :key="f" :value="f">{{ f }}</option>
        </optgroup>
        <optgroup v-if="libraryFonts.length" :label="$t('Fonts library')">
          <option v-for="f in libraryFonts" :key="f" :value="f">{{ f }}</option>
        </optgroup>
      </select>
    </label>
    <label class="field">
      <span>{{ $t('Kind') }}</span>
      <select v-model="blockKey" data-symbols-block>
        <option v-for="b in SYMBOL_BLOCKS" :key="b.key" :value="b.key">{{ blockLabel(b.key) }}</option>
      </select>
    </label>

    <template v-if="recent.length">
      <h3>{{ $t('Recently used') }}</h3>
      <div class="glyph-grid recent">
        <button
          v-for="r in recent"
          :key="r.f + r.c"
          type="button"
          class="glyph"
          :style="{ fontFamily: css(r.f || bodyFont) }"
          :title="`${codeOf(r.c)} · ${r.f || $t('Body font ({font})', { font: bodyFont })}`"
          @mousedown.prevent
          @mouseenter="hover = { c: r.c, f: r.f }"
          @click="insert(r.c, r.f)"
        >
          {{ r.c }}
        </button>
      </div>
    </template>

    <div class="preview" :style="{ fontFamily: css((hover ? hover.f : family) || bodyFont) }" aria-hidden="true">
      <span class="big">{{ hover?.c || '' }}</span>
      <span class="code">{{ hover ? codeOf(hover.c) : '' }}</span>
    </div>

    <p class="hint" data-symbols-status>{{ status }}</p>
    <div class="glyph-grid" role="list" data-symbols-grid>
      <button
        v-for="c in glyphs"
        :key="c"
        type="button"
        role="listitem"
        class="glyph"
        :style="{ fontFamily: css(drawn) }"
        :title="codeOf(c)"
        :aria-label="codeOf(c)"
        @mousedown.prevent
        @mouseenter="hover = { c, f: family }"
        @click="insert(c, family)"
      >
        {{ c }}
      </button>
    </div>
  </aside>
</template>

<script setup>
// The Symbols panel (docs/EDITOR.md, "Symbols"): a font's symbols by kind,
// only those the font itself draws (glyphs.js), inserted at the cursor in
// that font, or in the body font (no font mark). The last ones used are
// kept on this computer.
import { ref, computed, watch, onBeforeUnmount } from 'vue'
import { t, tn, tr } from '../../i18n'
import { SYMBOL_BLOCKS, blockChars, codeOf, glyphsOf, quoted } from '../../editor/glyphs.js'
import { BUNDLED_FONTS } from '../../editor/bundledFonts.js'
import { loadFont } from '../../editor/fonts.js'
import { fontLibrary, loadFontLibrary } from '../../fontLibrary.js'

const props = defineProps({
  bodyFont: { type: String, required: true },
  bookFonts: { type: Array, default: () => [] },
})
const emit = defineEmits(['insert', 'close'])

const BLOCKS = {
  misc: tr('Miscellaneous symbols'),
  dingbats: tr('Dingbats'),
  arrows: tr('Arrows'),
  shapes: tr('Shapes'),
  math: tr('Math'),
  math_letters: tr('Math letters'),
  letterlike: tr('Letterlike symbols and number forms'),
  enclosed: tr('Enclosed letters and numbers'),
  punctuation: tr('Punctuation'),
  currency: tr('Currency'),
  technical: tr('Technical'),
  boxes: tr('Boxes and blocks'),
  games: tr('Cards, dominoes and chess'),
  alchemy: tr('Alchemy'),
  runes: tr('Runes'),
  greek: tr('Greek'),
  latin: tr('Accented letters'),
  braille: tr('Braille'),
  cjk: tr('Japanese punctuation and kana'),
  emoji: tr('Emoji'),
  private: tr('Private use (old Symbol and Wingdings text)'),
}
const GROUPS = {
  Symbols: tr('Symbol fonts'),
  'Book and headings': tr('Book and headings'),
  'System windows': tr('System windows'),
  'Handwriting and sound effects': tr('Handwriting and sound effects'),
  Japanese: tr('Japanese'),
}
const blockLabel = (key) => t(BLOCKS[key] || key)
const css = (f) => `${quoted(f)}, serif`

const KEY = 'polaris-write-symbol-panel'
const saved = (() => {
  try {
    return JSON.parse(localStorage.getItem(KEY) || '{}') || {}
  } catch {
    return {}
  }
})()
const family = ref(typeof saved.family === 'string' ? saved.family : '')
const blockKey = ref(SYMBOL_BLOCKS.some((b) => b.key === saved.block) ? saved.block : 'misc')
const recent = ref(Array.isArray(saved.recent) ? saved.recent.filter((r) => r && typeof r.c === 'string').slice(0, 24) : [])
const hover = ref(null)
watch([family, blockKey, recent], () => {
  try {
    localStorage.setItem(KEY, JSON.stringify({ family: family.value, block: blockKey.value, recent: recent.value }))
  } catch {
    /* kept for this visit only */
  }
})

// The bundled fonts by group, symbol fonts first; the book's are listed above them.
const groups = computed(() => {
  const own = new Set(props.bookFonts.map((f) => f.toLowerCase()))
  return Object.keys(GROUPS)
    .map((g) => ({ label: t(GROUPS[g]), fonts: BUNDLED_FONTS.filter((b) => b.group === g && !own.has(b.family.toLowerCase())).map((b) => b.family) }))
    .filter((g) => g.fonts.length)
})
loadFontLibrary().catch(() => {})
const libraryFonts = computed(() => {
  const listed = new Set([...props.bookFonts, ...BUNDLED_FONTS.map((b) => b.family)].map((f) => f.toLowerCase()))
  return fontLibrary.value.map((f) => f.name).filter((n) => !listed.has(n.toLowerCase()))
})

// The block's characters the chosen font draws.
const drawn = computed(() => family.value || props.bodyFont)
const glyphs = ref([])
const busy = ref(false)
let seq = 0
async function look() {
  const mine = ++seq
  busy.value = true
  glyphs.value = []
  const block = SYMBOL_BLOCKS.find((b) => b.key === blockKey.value)
  const found = await glyphsOf(drawn.value, blockChars(block), { load: loadFont, stop: () => mine !== seq })
  if (mine !== seq) return
  glyphs.value = found
  busy.value = false
}
watch([drawn, blockKey], look, { immediate: true })
onBeforeUnmount(() => seq++)

const status = computed(() => {
  if (busy.value) return t('Looking through the font…')
  if (!glyphs.value.length) return t('This font has none of these. Try another font or kind.')
  return tn('{n} symbol in {font}', '{n} symbols in {font}', glyphs.value.length, { font: drawn.value })
})

function insert(c, f) {
  emit('insert', { char: c, family: f })
  recent.value = [{ c, f }, ...recent.value.filter((r) => r.c !== c || r.f !== f)].slice(0, 24)
}
</script>

<style scoped>
h3 {
  margin: 0.9rem 0 0.4rem;
  font-size: 0.85rem;
}

.field {
  display: grid;
  gap: 0.25rem;
  margin-top: 0.6rem;
  font-size: 0.8rem;
  color: var(--text-muted);
}

.hint {
  margin: 0.4rem 0;
  font-size: 0.75rem;
  color: var(--text-faint);
  line-height: 1.4;
}

.preview {
  display: flex;
  align-items: center;
  gap: 0.8rem;
  min-height: 4rem;
  margin-top: 0.8rem;
  padding: 0.3rem 0.7rem;
  border: 1px solid var(--glass-border);
  border-radius: 10px;
  background: var(--glass);
}

.preview .big {
  font-size: 2.6rem;
  line-height: 1;
  min-width: 3rem;
  text-align: center;
}

.preview .code {
  font-family: var(--font-body, sans-serif);
  font-size: 0.78rem;
  color: var(--text-muted);
}

.glyph-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(2.2rem, 1fr));
  gap: 0.2rem;
}

.glyph {
  display: grid;
  place-items: center;
  height: 2.2rem;
  padding: 0;
  border: 1px solid transparent;
  border-radius: 6px;
  background: none;
  color: inherit;
  font-size: 1.25rem;
  line-height: 1;
  cursor: pointer;
}

.glyph:hover,
.glyph:focus-visible {
  border-color: var(--glass-border);
  background: var(--glass);
}
</style>
