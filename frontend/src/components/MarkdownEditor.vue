<template>
  <div class="md-editor" :class="{ 'is-preview': preview }">
    <div class="md-toolbar" role="toolbar" :aria-label="$t('Formatting')">
      <div class="tools">
        <button v-for="b in buttons" :key="b.key" type="button" class="tool" :class="b.cls" :title="b.title" :aria-label="b.title" :disabled="preview" @mousedown.prevent @click="b.run">
          <svg v-if="b.d" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path v-for="(p, i) in b.d" :key="i" :d="p" />
          </svg>
          <span v-else aria-hidden="true">{{ b.text }}</span>
        </button>
        <span class="sep" aria-hidden="true"></span>
        <button type="button" class="tool tool-wiki" :title="$t('Link to another wiki article')" :disabled="preview" @mousedown.prevent @click="openLinkPanel">
          <span aria-hidden="true" class="brackets">[[ ]]</span> {{ $t('Link to article') }}
        </button>
      </div>
      <div class="modes" role="tablist">
        <button type="button" role="tab" class="mode" :class="{ active: !preview }" :aria-selected="!preview" @click="showWrite">{{ $t('Write') }}</button>
        <button type="button" role="tab" class="mode" :class="{ active: preview }" :aria-selected="preview" @click="preview = true">{{ $t('Preview') }}</button>
      </div>
    </div>

    <textarea
      v-show="!preview"
      :id="id"
      ref="area"
      :value="modelValue"
      :rows="rows"
      :placeholder="placeholder"
      :aria-label="ariaLabel || undefined"
      @input="emit('update:modelValue', $event.target.value)"
      @keydown="onKey"
    ></textarea>
    <!-- eslint-disable-next-line vue/no-v-html -->
    <div v-if="preview && modelValue.trim()" class="md md-preview" :style="{ minHeight: previewHeight }" @click="onPreviewClick" v-html="renderMarkdown(modelValue, resolve)"></div>
    <p v-else-if="preview" class="md-preview empty" :style="{ minHeight: previewHeight }">{{ $t('Nothing to preview yet.') }}</p>

    <WikiLinkPanel v-if="linkOpen" :items="items" :resolve="resolve" :text="linkStart" @pick="pick" @close="closeLinkPanel" />
  </div>
</template>

<script setup>
import { ref, computed, watch, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import WikiLinkPanel from './WikiLinkPanel.vue'
import { renderMarkdown } from '../markdown'
import { wrapInline, toggleLines, codeBlock, table, rule, webLink } from '../markdownEdit'
import { t } from '../i18n'

// A textarea for wiki-style Markdown with a formatting toolbar, a preview and
// a panel that searches the wiki and inserts a [[link]] to the chosen article.
// Labels outside point at it with for="<id>"; don't wrap it in a <label>, or
// clicking the label text would press the first toolbar button.
const props = defineProps({
  modelValue: { type: String, default: '' },
  id: { type: String, default: undefined },
  rows: { type: Number, default: 5 },
  placeholder: { type: String, default: '' },
  ariaLabel: { type: String, default: '' },
  // The wiki's article list (wikiApi.list()) and makeLinkResolver() over it.
  items: { type: Array, default: () => [] },
  resolve: { type: Function, default: null },
})
const emit = defineEmits(['update:modelValue'])
const router = useRouter()

const area = ref(null)
const preview = ref(false)
const previewHeight = ref('')

// ---- applying an edit (see markdownEdit.js) ----

function apply(edit) {
  const el = area.value
  el.focus()
  el.setSelectionRange(edit.from, edit.to)
  // insertText keeps Ctrl+Z working; fall back where it isn't supported.
  let done = false
  try {
    done = document.execCommand('insertText', false, edit.insert)
  } catch (e) {
    done = false
  }
  if (!done) el.setRangeText(edit.insert, edit.from, edit.to, 'end')
  emit('update:modelValue', el.value)
  el.setSelectionRange(edit.selStart, edit.selEnd)
}

const sel = () => [area.value.value, area.value.selectionStart, area.value.selectionEnd]
const run = (fn, ...args) => () => apply(fn(...sel(), ...args))

const buttons = computed(() => [
  { key: 'b', text: 'B', cls: 'is-bold', title: t('Bold (Ctrl+B)'), run: run(wrapInline, '**', t('bold text')) },
  { key: 'i', text: 'I', cls: 'is-italic', title: t('Italic (Ctrl+I)'), run: run(wrapInline, '*', t('italic text')) },
  { key: 's', text: 'S', cls: 'is-strike', title: t('Strikethrough'), run: run(wrapInline, '~~', t('struck text')) },
  { key: 'h', text: 'H', cls: 'is-bold', title: t('Heading'), run: run(toggleLines, 'heading') },
  { key: 'ul', title: t('Bulleted list'), d: ['M9 6h11', 'M9 12h11', 'M9 18h11', 'M4.5 6h.01', 'M4.5 12h.01', 'M4.5 18h.01'], run: run(toggleLines, 'bullet') },
  { key: 'ol', title: t('Numbered list'), d: ['M10 6h10', 'M10 12h10', 'M10 18h10', 'M4 4h1v4', 'M4 8h2', 'M4 14h2l-2 4h2'], run: run(toggleLines, 'numbered') },
  { key: 'q', title: t('Quote'), d: ['M4 6v12', 'M9 8h11', 'M9 12h11', 'M9 16h7'], run: run(toggleLines, 'quote') },
  { key: 'c', title: t('Inline code'), d: ['M16 18l6-6-6-6', 'M8 6l-6 6 6 6'], run: run(wrapInline, '`', t('code')) },
  { key: 'cb', title: t('Code block'), d: ['M4 4h16v16H4z', 'M10 9l-3 3 3 3', 'M14 9l3 3-3 3'], run: run(codeBlock, t('code')) },
  { key: 't', title: t('Table'), d: ['M3 5h18v14H3z', 'M3 10h18', 'M3 15h18', 'M10 5v14'], run: run(table, t('Column'), t('Cell')) },
  { key: 'hr', title: t('Horizontal line'), d: ['M3 12h18'], run: run(rule) },
  {
    key: 'url',
    title: t('Web link'),
    d: ['M10 13a5 5 0 0 0 7.5.5l3-3a5 5 0 0 0-7-7l-1.7 1.7', 'M14 11a5 5 0 0 0-7.5-.5l-3 3a5 5 0 0 0 7 7l1.7-1.7'],
    run: run(webLink, t('link text')),
  },
])

function onKey(e) {
  if (!(e.ctrlKey || e.metaKey) || e.altKey || e.shiftKey) return
  const k = e.key.toLowerCase()
  if (k === 'b') {
    e.preventDefault()
    buttons.value[0].run()
  } else if (k === 'i') {
    e.preventDefault()
    buttons.value[1].run()
  }
}

// ---- preview ----

watch(preview, (on) => {
  if (on && area.value) previewHeight.value = `${area.value.offsetHeight}px`
})

async function showWrite() {
  preview.value = false
  await nextTick()
  area.value?.focus()
}

// Links in the preview open in a new tab, so the unsaved text stays put.
function onPreviewClick(e) {
  const a = e.target.closest && e.target.closest('a[data-wiki]')
  if (!a) return
  e.preventDefault()
  e.stopPropagation()
  window.open(router.resolve(a.getAttribute('href')).href, '_blank', 'noopener')
}

// ---- the link panel (WikiLinkPanel.vue) ----

const linkOpen = ref(false)
const linkStart = ref('')
let range = [0, 0]

function openLinkPanel() {
  const [text, from, to] = sel()
  range = [from, to]
  linkStart.value = text.slice(from, to)
  linkOpen.value = true
}

async function closeLinkPanel() {
  linkOpen.value = false
  await nextTick()
  area.value?.focus()
  area.value?.setSelectionRange(range[0], range[1])
}

async function pick(insert) {
  linkOpen.value = false
  await nextTick()
  const at = range[0] + insert.length
  apply({ from: range[0], to: range[1], insert, selStart: at, selEnd: at })
}
</script>

<style scoped>
.md-editor {
  display: flex;
  flex-direction: column;
  border: 1px solid var(--glass-border);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.02);
}

.md-editor:focus-within {
  border-color: color-mix(in srgb, var(--line) 55%, var(--glass-border));
}

.md-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.3rem 0.6rem;
  padding: 0.3rem 0.4rem;
  border-bottom: 1px solid var(--glass-border);
}

.tools {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.1rem;
}

.tool {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.35rem;
  min-width: 1.9rem;
  height: 1.9rem;
  padding: 0 0.35rem;
  border: 1px solid transparent;
  border-radius: 6px;
  background: none;
  color: var(--text-muted);
  font-family: 'Manrope', sans-serif;
  font-size: 0.86rem;
  cursor: pointer;
}

.tool svg {
  width: 1.05rem;
  height: 1.05rem;
}

.tool:not(:disabled):hover {
  color: var(--text-primary);
  background: rgba(255, 255, 255, 0.07);
}

.tool:disabled {
  opacity: 0.35;
  cursor: default;
}

.tool.is-bold span {
  font-weight: 800;
}

.tool.is-italic span {
  font-style: italic;
  font-family: Georgia, serif;
}

.tool.is-strike span {
  text-decoration: line-through;
}

.tool-wiki {
  padding: 0 0.6rem;
  font-size: 0.78rem;
  font-weight: 600;
  color: var(--line);
  border-color: color-mix(in srgb, var(--line) 35%, transparent);
}

.brackets {
  font-family: ui-monospace, monospace;
  font-size: 0.75rem;
}

.sep {
  width: 1px;
  height: 1.2rem;
  margin: 0 0.3rem;
  background: var(--glass-border);
}

.modes {
  display: flex;
  gap: 0.15rem;
}

.mode {
  padding: 0.2rem 0.6rem;
  border: none;
  border-radius: 6px;
  background: none;
  color: var(--text-faint);
  font-family: 'Manrope', sans-serif;
  font-size: 0.76rem;
  font-weight: 600;
  cursor: pointer;
}

.mode.active {
  color: var(--text-primary);
  background: rgba(255, 255, 255, 0.08);
}

.md-editor textarea {
  border: none;
  border-radius: 0 0 8px 8px;
  background: none;
  resize: vertical;
  min-height: 6rem;
}

.md-editor textarea:focus {
  outline: none;
  box-shadow: none;
}

.md-preview {
  padding: 0.6rem 0.75rem;
  color: var(--text-primary);
}

.md-preview.empty {
  margin: 0;
  font-size: 0.86rem;
  font-style: italic;
  color: var(--text-faint);
}
</style>

