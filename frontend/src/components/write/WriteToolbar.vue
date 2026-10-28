<template>
  <div class="write-toolbar glass-panel" :class="{ 'show-more': more }" role="toolbar" :aria-label="$t('Formatting')">
    <div class="tb-group">
      <button v-bind="btn(t('Undo') + ' (Ctrl+Z)')" :disabled="!canUndo" @click="exec(undo)"><TbIcon name="undo" /></button>
      <button v-bind="btn(t('Redo') + ' (Ctrl+Y)')" :disabled="!canRedo" @click="exec(redo)"><TbIcon name="redo" /></button>
    </div>

    <div class="tb-group">
      <select class="tb-style" :value="fmt.styleId || ''" :aria-label="$t('Style')" :title="$t('Style')" @change="exec(setStyle($event.target.value))">
        <option v-if="!fmt.styleId" value="" disabled>—</option>
        <option v-for="s in paragraphStyles" :key="s.id" :value="s.id">{{ nameOf(s) }}</option>
      </select>
      <select v-if="characterStyles.length" class="tb-style tb-cstyle foldable" :value="fmt.charStyleId || ''" :aria-label="$t('Character style')" :title="$t('Character style (for the selected text)')" @change="exec(setCharStyle($event.target.value || null))">
        <option value="">{{ $t('No character style') }}</option>
        <option v-if="fmt.charStyleId && !characterStyles.some((s) => s.id === fmt.charStyleId)" :value="fmt.charStyleId">—</option>
        <option v-for="s in characterStyles" :key="s.id" :value="s.id">{{ nameOf(s) }}</option>
      </select>
      <input
        class="tb-font"
        type="text"
        list="write-fonts"
        :value="fmt.font || ''"
        :aria-label="$t('Font')"
        :title="$t('Font')"
        @change="setFont($event.target.value)"
        @keydown.enter.prevent="$event.target.blur()"
      />
      <input
        class="tb-size"
        type="number"
        list="write-sizes"
        min="1"
        max="400"
        step="0.5"
        :value="fmt.size ?? ''"
        :aria-label="$t('Font size (pt)')"
        :title="$t('Font size (pt)') + ' (Ctrl+Shift+> / <)'"
        @change="setSize($event.target.value)"
        @keydown.enter.prevent="$event.target.blur()"
      />
      <datalist id="write-sizes"><option v-for="s in FONT_SIZES" :key="s" :value="s"></option></datalist>
    </div>

    <div class="tb-group">
      <button v-bind="btn(t('Bold') + ' (Ctrl+B)', fmt.bold)" @click="exec(toggleMark('bold'))"><b>B</b></button>
      <button v-bind="btn(t('Italic') + ' (Ctrl+I)', fmt.italic)" @click="exec(toggleMark('italic'))"><i class="serif">I</i></button>
      <button v-bind="btn(t('Underline') + ' (Ctrl+U)', fmt.underline)" @click="exec(toggleMark('underline'))"><u>U</u></button>
      <button v-bind="btn(t('Strikethrough'), fmt.strike)" class="foldable" @click="exec(toggleMark('strike'))"><s>S</s></button>
      <button v-bind="btn(t('Superscript') + ' (Ctrl+Shift+=)', fmt.superscript)" class="foldable" @click="exec(toggleMark('superscript'))"><span>x<sup>2</sup></span></button>
      <button v-bind="btn(t('Subscript') + ' (Ctrl+=)', fmt.subscript)" class="foldable" @click="exec(toggleMark('subscript'))"><span>x<sub>2</sub></span></button>
      <button v-bind="btn(t('Small caps') + ' (Ctrl+Shift+K)', fmt.smallcaps)" class="foldable" @click="exec(toggleMark('smallcaps'))"><span class="sc">Sc</span></button>
      <div class="tb-pop foldable">
        <button v-bind="btn(t('Text colour'))" @click="toggle('color')">
          <span class="swatch-letter">A<span class="bar" :style="{ background: fmt.color || 'currentColor' }"></span></span>
        </button>
        <div v-if="open === 'color'" class="pop glass-panel panel-solid" @mousedown.stop>
          <div class="swatches">
            <button v-for="c in TEXT_COLORS" :key="c" type="button" class="sw" :style="{ background: c }" :title="c" @mousedown.prevent @click="pickColor(c)"></button>
          </div>
          <div class="pop-row">
            <label class="custom-color">{{ $t('Other') }} <input type="color" :value="fmt.color || '#000000'" @change="pickColor($event.target.value)" /></label>
            <button type="button" class="btn btn-ghost small" @mousedown.prevent @click="pickColor(null)">{{ $t('Automatic') }}</button>
          </div>
        </div>
      </div>
      <div class="tb-pop foldable">
        <button v-bind="btn(t('Highlight'), !!fmt.highlight)" @click="toggle('highlight')">
          <span class="swatch-letter hl">ab<span class="bar" :style="{ background: fmt.highlight || '#fff176' }"></span></span>
        </button>
        <div v-if="open === 'highlight'" class="pop glass-panel panel-solid" @mousedown.stop>
          <div class="swatches">
            <button v-for="c in HIGHLIGHTS" :key="c" type="button" class="sw" :style="{ background: c }" :title="c" @mousedown.prevent @click="pickHighlight(c)"></button>
          </div>
          <div class="pop-row">
            <button type="button" class="btn btn-ghost small" @mousedown.prevent @click="pickHighlight(null)">{{ $t('No highlight') }}</button>
          </div>
        </div>
      </div>
      <button v-bind="btn(t('Clear formatting'))" class="foldable" @click="exec(clearFormatting)"><TbIcon name="clear" /></button>
    </div>

    <div class="tb-group foldable">
      <button v-for="a in ALIGNS" :key="a.key" v-bind="btn(a.label, fmt.align === a.key)" @click="exec(setParaAttrs({ align: a.key }))"><TbIcon :name="`align-${a.key}`" /></button>
    </div>

    <div class="tb-group foldable para">
      <label class="tb-num" :title="$t('First-line indent (mm)')">
        <TbIcon name="indent-first" />
        <input type="number" step="0.5" min="-100" max="200" :value="mm(fmt.indentFirst)" :aria-label="$t('First-line indent (mm)')" @change="setPara('indentFirst', $event.target.value, true)" />
      </label>
      <label class="tb-num" :title="$t('Left indent (mm)')">
        <TbIcon name="indent-left" />
        <input type="number" step="0.5" min="-100" max="200" :value="mm(fmt.indentLeft)" :aria-label="$t('Left indent (mm)')" @change="setPara('indentLeft', $event.target.value, true)" />
      </label>
      <label class="tb-num" :title="$t('Space before (pt)')">
        <TbIcon name="space-before" />
        <input type="number" step="1" min="0" max="500" :value="pt(fmt.spaceBefore)" :aria-label="$t('Space before (pt)')" @change="setPara('spaceBefore', $event.target.value)" />
      </label>
      <label class="tb-num" :title="$t('Space after (pt)')">
        <TbIcon name="space-after" />
        <input type="number" step="1" min="0" max="500" :value="pt(fmt.spaceAfter)" :aria-label="$t('Space after (pt)')" @change="setPara('spaceAfter', $event.target.value)" />
      </label>
      <label class="tb-num" :title="$t('Line height') + ' (Ctrl+1 / 5 / 2)'">
        <TbIcon name="line-height" />
        <select :value="lineValue" :aria-label="$t('Line height')" @change="setPara('lineHeight', $event.target.value)">
          <option v-if="!LINE_HEIGHTS.includes(fmt.lineHeight)" :value="lineValue">{{ lineValue }}</option>
          <option v-for="l in LINE_HEIGHTS" :key="l" :value="String(l)">{{ l }}</option>
        </select>
      </label>
    </div>

    <div class="tb-group foldable">
      <button v-bind="btn(t('Bulleted list'), fmt.list === 'bullet_list')" @click="exec(toggleList('bullet_list'))"><TbIcon name="list-bullet" /></button>
      <button v-bind="btn(t('Numbered list'), fmt.list === 'ordered_list')" @click="exec(toggleList('ordered_list'))"><TbIcon name="list-number" /></button>
      <button v-bind="btn(t('Scene break'))" @click="exec(insertBlock('scene_break'))"><span class="glyph">⁂</span></button>
      <button v-bind="btn(t('Page break') + ' (Ctrl+Enter)')" :disabled="online" @click="exec(insertBlock('page_break'))"><TbIcon name="page-break" /></button>
      <button v-bind="btn(t('Table of contents'))" class="foldable" :disabled="online" @click="exec(insertBlock('toc', { title: t('Contents') }))"><TbIcon name="toc" /></button>
      <button v-bind="btn(t('Insert picture'), fmt.atom === 'image')" :disabled="uploading" @click="$emit('picture')"><TbIcon name="picture" /></button>
      <div class="tb-pop foldable">
        <button v-bind="btn(t('Insert table'))" :disabled="!canTable" @click="toggle('table')"><TbIcon name="table" /></button>
        <div v-if="open === 'table'" class="pop glass-panel panel-solid table-pop" @mousedown.stop @mouseleave="grid = { r: 0, c: 0 }">
          <div class="grid" role="grid" :aria-label="$t('Insert table')">
            <template v-for="r in GRID" :key="r">
              <button
                v-for="c in GRID"
                :key="c"
                type="button"
                class="cell"
                :class="{ 'is-on': r <= grid.r && c <= grid.c }"
                :aria-label="$t('{rows} × {cols} table', { rows: r, cols: c })"
                @mousedown.prevent
                @mouseenter="grid = { r, c }"
                @focus="grid = { r, c }"
                @click="addTable(r, c)"
              ></button>
            </template>
          </div>
          <p class="grid-size">{{ grid.r ? $t('{rows} × {cols} table', { rows: grid.r, cols: grid.c }) : $t('Insert table') }}</p>
        </div>
      </div>
      <div class="tb-pop">
        <button v-bind="btn(t('Web link'), !!fmt.link)" @click="openLink"><TbIcon name="link" /></button>
        <form v-if="open === 'link'" class="pop glass-panel panel-solid link-pop" @mousedown.stop @submit.prevent="applyLink">
          <input ref="linkEl" v-model="linkText" type="url" placeholder="https://" :aria-label="$t('Web address')" />
          <div class="pop-row">
            <button type="submit" class="btn btn-primary small">{{ $t('Apply') }}</button>
            <button v-if="fmt.link" type="button" class="btn btn-ghost small" @click="removeLink">{{ $t('Remove link') }}</button>
          </div>
        </form>
      </div>
    </div>

    <div class="tb-group tools">
      <button v-bind="btn(t('Find and replace') + ' (Ctrl+F)')" @click="$emit('find')"><TbIcon name="find" /></button>
      <button v-bind="btn(t('Styles'), panel === 'styles')" class="foldable" @click="$emit('panel', 'styles')"><TbIcon name="styles" /></button>
      <button v-bind="btn(t('New comment') + ' (Ctrl+Alt+M)')" @click="$emit('comment')"><TbIcon name="comment" /></button>
      <button v-bind="btn(t('Comments'), panel === 'comments')" class="foldable" @click="$emit('panel', 'comments')"><TbIcon name="comments" /></button>
      <button v-bind="btn(t('Saved versions'), panel === 'versions')" class="foldable" @click="$emit('panel', 'versions')"><TbIcon name="versions" /></button>
      <button v-bind="btn(t('Book and page'), panel === 'book')" class="foldable" @click="$emit('panel', 'book')"><TbIcon name="book" /></button>
      <button v-bind="btn(scriptMode ? t('Script Mode: on (this chapter)') : t('Script Mode: type “Name:” at a line’s start for dialogue'), scriptMode)" class="foldable" @click="exec(setScriptMode(!scriptMode))"><TbIcon name="script" /></button>
      <button v-bind="btn(readTitle, reading)" class="foldable" :disabled="!canRead" @click="$emit('read')"><TbIcon name="speaker" /></button>
      <button v-bind="btn(spellOn ? t('Check spelling: on') : t('Check spelling: off'), spellOn)" class="foldable" @click="$emit('spell')"><TbIcon name="spell" /></button>
      <button v-bind="btn(t('Focus mode'), focusMode)" @click="$emit('focus')"><TbIcon name="focus" /></button>
      <button type="button" class="tb-btn more-btn" :aria-expanded="more" :title="$t('More tools')" :aria-label="$t('More tools')" @click="more = !more">{{ more ? '▴' : '⋯' }}</button>
    </div>
  </div>
</template>

<script setup>
// The writing page's toolbar. Every button runs an editor command (see
// src/editor/commands.js) through `exec`, which also puts the cursor back
// in the text. On a phone the rarer groups fold behind "⋯".
import { ref, computed, h, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { undo, redo, undoDepth, redoDepth } from 'prosemirror-history'
import { t } from '../../i18n'
import { BUILTIN_NAMES } from '../../editor/styles.js'
import { ptToMm, mmToPt, num } from '../../editor/css.js'
import { loadFont } from '../../editor/fonts.js'
import {
  FONT_SIZES,
  LINE_HEIGHTS,
  toggleMark,
  setMark,
  setParaAttrs,
  setStyle,
  selectionFormat,
  toggleList,
  insertBlock,
  clearFormatting,
  setLink,
  setCharStyle,
} from '../../editor/commands.js'
import { setScriptMode } from '../../editor/script.js'
import { insertTable } from '../../editor/tables.js'

const props = defineProps({
  state: { type: Object, required: true },
  styles: { type: Array, required: true },
  exec: { type: Function, required: true }, // runs a command on the editor
  online: { type: Boolean, default: false },
  focusMode: { type: Boolean, default: false },
  panel: { type: String, default: '' },
  uploading: { type: Boolean, default: false }, // a picture is on its way
  canRead: { type: Boolean, default: false }, // the browser can read aloud
  reading: { type: Boolean, default: false }, // the chapter is being read
  spellOn: { type: Boolean, default: false }, // the spellchecker (useSpellcheck)
})
defineEmits(['find', 'panel', 'focus', 'picture', 'read', 'comment', 'spell'])
const readTitle = computed(() =>
  props.canRead ? t('Read aloud') + ' (Ctrl+Alt+Space)' : t('Your browser cannot read aloud'),
)

const TEXT_COLORS = ['#000000', '#444444', '#7f7f7f', '#c00000', '#e36c09', '#9c6500', '#2e7d32', '#1f4e79', '#3949ab', '#7030a0', '#ad1457', '#ffffff']
const HIGHLIGHTS = ['#fff176', '#c5e1a5', '#80deea', '#f8bbd0', '#ffcc80', '#d1c4e9']
const ALIGNS = computed(() => [
  { key: 'left', label: t('Align left') + ' (Ctrl+L)' },
  { key: 'center', label: t('Centre') + ' (Ctrl+E)' },
  { key: 'right', label: t('Align right') + ' (Ctrl+R)' },
  { key: 'justify', label: t('Justify') + ' (Ctrl+J)' },
])

const more = ref(false)
const open = ref('')
const linkText = ref('')
const linkEl = ref(null)

const fmt = computed(() => selectionFormat(props.state, props.styles))
const canUndo = computed(() => undoDepth(props.state) > 0)
const canRedo = computed(() => redoDepth(props.state) > 0)
const paragraphStyles = computed(() => props.styles.filter((s) => (s.kind || 'paragraph') === 'paragraph'))
const characterStyles = computed(() => props.styles.filter((s) => s.kind === 'character'))
// Script Mode is the chapter's (its doc root's mode).
const scriptMode = computed(() => props.state.doc.attrs.mode === 'script')
const nameOf = (s) => (s.builtin && s.name === BUILTIN_NAMES[s.id] ? t(s.name) : s.name)
const mm = (v) => (v == null ? '' : String(ptToMm(v)))
const pt = (v) => (v == null ? '' : num(v))
const lineValue = computed(() => (fmt.value.lineHeight == null ? '' : String(fmt.value.lineHeight)))

// Buttons keep the text's selection: they never take the focus on click.
function btn(label, active) {
  return {
    type: 'button',
    class: ['tb-btn', { 'is-active': !!active }],
    title: label,
    'aria-label': label,
    'aria-pressed': active === undefined ? undefined : !!active,
    onMousedown: (e) => e.preventDefault(),
  }
}

const exec = (cmd) => {
  open.value = ''
  props.exec(cmd)
}

function setFont(value) {
  const family = value.trim()
  if (family) loadFont(family)
  exec(setMark('font', family ? { family } : null))
}

function setSize(value) {
  const n = +value
  if (value === '' || !Number.isFinite(n) || n < 1 || n > 400) return exec(setMark('size', null))
  exec(setMark('size', { pt: Math.round(n * 2) / 2 }))
}

// An empty box goes back to the style's value.
function setPara(key, value, inMm = false) {
  const n = value === '' ? null : +value
  if (n != null && !Number.isFinite(n)) return
  exec(setParaAttrs({ [key]: n == null ? null : inMm ? mmToPt(n) : n }))
}

const pickColor = (c) => exec(setMark('color', c ? { color: c } : null))
const pickHighlight = (c) => exec(setMark('highlight', c ? { color: c } : null))

function toggle(name) {
  open.value = open.value === name ? '' : name
}

function openLink() {
  linkText.value = fmt.value.link || ''
  toggle('link')
  if (open.value === 'link') nextTick(() => linkEl.value?.focus())
}
function applyLink() {
  let href = linkText.value.trim()
  if (href && !/^(https?:|mailto:|#|\/)/i.test(href)) href = `https://${href}`
  exec(setLink(href))
}
const removeLink = () => exec(setLink(''))

// Insert table: a grid to pick rows × columns, as in Word.
const GRID = [1, 2, 3, 4, 5, 6, 7, 8]
const grid = ref({ r: 0, c: 0 })
const canTable = computed(() => insertTable(1, 1)(props.state))
function addTable(r, c) {
  grid.value = { r: 0, c: 0 }
  exec(insertTable(r, c))
}

// A click anywhere else closes an open pop-up.
const closePops = () => (open.value = '')
onMounted(() => document.addEventListener('mousedown', closePops))
onBeforeUnmount(() => document.removeEventListener('mousedown', closePops))

// Small line icons (24 × 24, drawn with the text colour).
const ICONS = {
  undo: 'M9 14 4 9l5-5M4 9h10.5a5.5 5.5 0 0 1 0 11H11',
  redo: 'M15 14l5-5-5-5M20 9H9.5a5.5 5.5 0 0 0 0 11H13',
  clear: 'M4 7V4h16v3M9 20h6M12 4 8 20M3 3l18 18',
  'align-left': 'M3 6h18M3 10h12M3 14h18M3 18h12',
  'align-center': 'M3 6h18M6 10h12M3 14h18M6 18h12',
  'align-right': 'M3 6h18M9 10h12M3 14h18M9 18h12',
  'align-justify': 'M3 6h18M3 10h18M3 14h18M3 18h18',
  'indent-first': 'M9 6h12M3 10h18M3 14h18M3 18h18M3 4v4l3-2z',
  'indent-left': 'M9 6h12M9 10h12M9 14h12M9 18h12M3 9l3 3-3 3',
  'space-before': 'M3 12h18M3 16h18M3 20h18M12 2v6M9 5l3-3 3 3',
  'space-after': 'M3 4h18M3 8h18M3 12h18M12 22v-6M9 19l3 3 3-3',
  'line-height': 'M10 6h11M10 12h11M10 18h11M5 4v16M3 6l2-2 2 2M3 18l2 2 2-2',
  'list-bullet': 'M9 6h12M9 12h12M9 18h12M4 6h.01M4 12h.01M4 18h.01',
  'list-number': 'M10 6h11M10 12h11M10 18h11M4 4v4M3 8h2M3 11h2l-2 3h2M3 16h2v4H3M3 18h2',
  'page-break': 'M6 3v5h12V3M6 21v-5h12v5M2 12h3M8 12h3M13 12h3M19 12h3',
  toc: 'M4 6h10M4 12h10M4 18h10M17 6h3M17 12h3M17 18h3',
  picture: 'M4 5h16v14H4zM4 16l5-5 4 4 2-2 5 5M15 9h.01',
  link: 'M10 14a4 4 0 0 0 5.66 0l3-3a4 4 0 0 0-5.66-5.66l-1 1M14 10a4 4 0 0 0-5.66 0l-3 3a4 4 0 0 0 5.66 5.66l1-1',
  find: 'M11 4a7 7 0 1 0 0 14 7 7 0 0 0 0-14zM21 21l-5-5',
  styles: 'M4 20h16M7 16 12 4l5 12M9 12h6',
  versions: 'M12 8v4l3 2M3.05 11a9 9 0 1 1 .5 4M3 4v7h7',
  book: 'M4 4h11a3 3 0 0 1 3 3v13H7a3 3 0 0 1-3-3zM4 17a3 3 0 0 1 3-3h11',
  focus: 'M4 9V4h5M20 9V4h-5M4 15v5h5M20 15v5h-5',
  speaker: 'M11 5 6 9H3v6h3l5 4zM15.5 8.5a5 5 0 0 1 0 7M18.5 5.5a9 9 0 0 1 0 13',
  script: 'M4 5h16v11H10l-4 3v-3H4zM8 9h8M8 12h5',
  comment: 'M4 4h16v12H9l-5 4zM12 7v6M9 10h6',
  comments: 'M3 4h12v8H8l-3 3v-3H3zM18 8h3v9h-2v3l-3-3h-5v-2',
  spell: 'M3 14 6.5 5l3.5 9M4.3 11h4.4M12 5v9h2.5a2.2 2.2 0 0 0 0-4.5H12M13 18l3 3 6-7',
  table: 'M3 5h18v14H3zM3 10h18M3 14.5h18M9 5v14M15 5v14',
}
const TbIcon = (p) =>
  h('svg', { viewBox: '0 0 24 24', width: 18, height: 18, fill: 'none', stroke: 'currentColor', 'stroke-width': 1.8, 'stroke-linecap': 'round', 'stroke-linejoin': 'round', 'aria-hidden': 'true' }, [h('path', { d: ICONS[p.name] || '' })])
TbIcon.props = ['name']
</script>

<style scoped>
.write-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.25rem 0.5rem;
  padding: 0.35rem 0.5rem;
  border-radius: 12px;
}

.tb-group {
  display: flex;
  align-items: center;
  gap: 0.1rem;
  padding-right: 0.5rem;
  border-right: 1px solid var(--glass-border);
}

.tb-group:last-child {
  border-right: none;
  padding-right: 0;
}

.tools {
  margin-left: auto;
}

.tb-btn {
  display: inline-grid;
  place-items: center;
  min-width: 2rem;
  height: 2rem;
  padding: 0 0.35rem;
  border: 1px solid transparent;
  border-radius: 7px;
  background: none;
  color: var(--text-primary);
  font: 600 0.9rem 'Manrope', sans-serif;
  cursor: pointer;
}

.tb-btn:hover:not(:disabled) {
  background: var(--glass);
  border-color: var(--glass-border);
}

.tb-btn.is-active {
  background: var(--accent-soft);
  border-color: color-mix(in srgb, var(--accent) 55%, transparent);
  color: var(--accent);
}

.tb-btn:disabled {
  opacity: 0.38;
  cursor: default;
}

.serif {
  font-family: 'Fraunces', serif;
}

.sc {
  font-variant: small-caps;
  font-size: 0.95rem;
}

.glyph {
  font-size: 1.05rem;
}

sup,
sub {
  font-size: 0.6rem;
}

.write-toolbar select,
.write-toolbar input {
  min-height: 2rem;
  height: 2rem;
  padding: 0.15rem 0.45rem;
  font-size: 0.82rem;
}

.tb-style {
  width: 8.5rem;
  margin-right: 0.15rem;
}

.tb-cstyle {
  width: 8rem;
}

.tb-font {
  width: 9.5rem;
  margin-right: 0.15rem;
}

.tb-size {
  width: 4rem;
}

.tb-num {
  display: inline-flex;
  align-items: center;
  gap: 0.15rem;
  color: var(--text-muted);
}

.tb-num input {
  width: 3.4rem;
}

.tb-num select {
  width: 4.2rem;
  padding-right: 1.2rem;
}

.para {
  gap: 0.35rem;
}

.swatch-letter {
  display: inline-grid;
  justify-items: center;
  line-height: 1;
  font-size: 0.9rem;
}

.swatch-letter .bar {
  display: block;
  width: 1rem;
  height: 0.22rem;
  margin-top: 0.1rem;
  border-radius: 1px;
}

.hl {
  font-size: 0.8rem;
}

.tb-pop {
  position: relative;
}

.pop {
  position: absolute;
  top: calc(100% + 0.35rem);
  left: 0;
  z-index: 30;
  padding: 0.6rem;
  display: grid;
  gap: 0.5rem;
  min-width: 11rem;
}

.swatches {
  display: grid;
  grid-template-columns: repeat(6, 1.5rem);
  gap: 0.3rem;
}

.sw {
  width: 1.5rem;
  height: 1.5rem;
  border-radius: 5px;
  border: 1px solid rgba(127, 127, 127, 0.5);
  cursor: pointer;
}

.pop-row {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  justify-content: space-between;
}

.custom-color {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  font-size: 0.8rem;
  color: var(--text-muted);
}

.custom-color input {
  width: 2rem;
  height: 1.6rem;
  min-height: 0;
  padding: 0;
  border: none;
  background: none;
}

.link-pop {
  min-width: 18rem;
  right: auto;
}

.table-pop {
  min-width: 0;
  gap: 0.35rem;
}

.grid {
  display: grid;
  grid-template-columns: repeat(8, 1.05rem);
  gap: 0.2rem;
}

.grid .cell {
  width: 1.05rem;
  height: 1.05rem;
  padding: 0;
  border: 1px solid rgba(127, 127, 127, 0.55);
  border-radius: 3px;
  background: none;
  cursor: pointer;
}

.grid .cell.is-on {
  background: var(--accent-soft);
  border-color: var(--accent);
}

.grid-size {
  margin: 0;
  text-align: center;
  font-size: 0.78rem;
  color: var(--text-muted);
}

.more-btn {
  display: none;
}

/* Phones: the everyday buttons stay, the rest folds behind "⋯". */
@media (max-width: 720px) {
  .write-toolbar {
    gap: 0.2rem 0.3rem;
  }
  .tb-group {
    border-right: none;
    padding-right: 0;
    flex-wrap: wrap;
    max-width: 100%;
  }
  .foldable {
    display: none;
  }
  .show-more .foldable {
    display: flex;
  }
  .show-more .tb-btn.foldable,
  .show-more .tb-pop.foldable {
    display: inline-grid;
  }
  .more-btn {
    display: inline-grid;
  }
  .tb-style {
    width: 6.6rem;
  }
  .tb-font {
    width: 7rem;
  }
  .tb-size {
    width: 3.7rem;
  }
}
</style>
