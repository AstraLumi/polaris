<template>
  <div ref="root" class="export-menu" :class="{ 'is-inline': inline }">
    <button v-if="!inline" type="button" class="btn btn-ghost small export-btn" aria-haspopup="menu" :aria-expanded="open" @click="open = !open">
      {{ $t('Export') }} <span aria-hidden="true">▾</span>
    </button>
    <div v-if="inline || open" class="export-list" :class="{ 'glass-panel panel-solid': !inline }" role="menu" @keydown.esc.stop="open = false">
      <template v-if="!online">
        <p class="group">{{ $t('For print and e-readers') }}</p>
        <a role="menuitem" class="item" :href="url('docx')" download @click="checked($event, 'docx')">{{ $t('Word (.docx), whole book') }}</a>
        <a v-if="chapterId" role="menuitem" class="item" :href="url('docx', chapterId)" download @click="checked($event, 'docx', chapterId)">{{ $t('Word (.docx), this chapter') }}</a>
        <a role="menuitem" class="item" :href="url('epub')" download @click="checked($event, 'epub')">{{ $t('EPUB (e-book)') }}</a>
        <button type="button" role="menuitem" class="item" @click="pdf">{{ $t('PDF (print the page view)') }}</button>
      </template>
      <p class="group">{{ online ? $t('Online post') : $t('For posting online') }}</p>
      <button type="button" role="menuitem" class="item" :disabled="copying" @click="copy">
        {{ chapterId ? $t('Copy chapter as formatted text') : $t('Copy book as formatted text') }}
      </button>
      <template v-for="f in POST" :key="f.ext">
        <a v-if="chapterId" role="menuitem" class="item" :href="url(f.ext, chapterId)" download @click="done">{{ $t('{format}, this chapter', { format: f.label }) }}</a>
        <a role="menuitem" class="item" :href="url(f.ext)" download @click="done">{{ $t('{format}, whole book', { format: f.label }) }}</a>
      </template>
      <p v-if="note" class="note" role="status">{{ note }}</p>
    </div>
  </div>
</template>

<script setup>
// The book's exports (docs/EDITOR.md, "Exports"): downloads from GET
// /api/books/{volume}/export.docx|epub|html|md|txt (?chapter=ID for one
// chapter), the PDF by printing the page view, and a copy of the HTML
// export on the clipboard (formatted text for a post). As a "▾" menu, or
// inline as a list (the book panel).
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { t } from '../../i18n'
import { STORY_API } from '../../stories'
import { loadFontLibrary, fetchBookFonts, fontWarningText } from '../../fontLibrary.js'
import { exportWarnings } from '../../fontRules.js'

const props = defineProps({
  volume: { type: [String, Number], required: true },
  chapterId: { type: [Number, String], default: null },
  online: { type: Boolean, default: false },
  inline: { type: Boolean, default: false },
})
const emit = defineEmits(['pdf'])

const open = ref(false)
const copying = ref(false)
const note = ref('')
const root = ref(null)
const POST = computed(() => [
  { ext: 'html', label: 'HTML' },
  { ext: 'md', label: 'Markdown' },
  { ext: 'txt', label: t('Text') },
])

// Under the open story's prefix, like every other call (editor/api.js).
const url = (ext, chapter) => `${STORY_API}/books/${props.volume}/export.${ext}${chapter ? `?chapter=${chapter}` : ''}`

function done() {
  open.value = false
}

// Before a file for print or e-readers: the library's fonts the book names
// whose license lacks that use, or whose file forbids embedding. A
// confirm, never a block (nor a reason to fail when it can't be checked).
async function fontsAllow(format) {
  try {
    const [lib, uses] = await Promise.all([loadFontLibrary(), fetchBookFonts(props.volume)])
    const warns = exportWarnings(format, uses, lib)
    if (!warns.length) return true
    const lines = warns.map((w) => `• ${fontWarningText(w)}`).join('\n')
    const epub = format === 'epub' ? `\n\n${t('Fonts an e-book may not carry are left out of it; e-readers then show their own.')}` : ''
    return window.confirm(`${t('About the fonts:')}\n${lines}${epub}\n\n${t('Export anyway?')}`)
  } catch (e) {
    return true
  }
}

async function checked(e, format, chapter) {
  e.preventDefault()
  open.value = false
  if (!(await fontsAllow(format))) return
  const a = document.createElement('a')
  a.href = url(format, chapter)
  a.download = ''
  document.body.appendChild(a)
  a.click()
  a.remove()
}

async function pdf() {
  open.value = false
  if (await fontsAllow('pdf')) emit('pdf')
}

async function fetchText(ext) {
  const res = await fetch(url(ext, props.chapterId))
  if (!res.ok) throw new Error(t('failed to export the book'))
  return res.text()
}

// The HTML export as rich text, its plain text beside it.
async function copy() {
  copying.value = true
  note.value = ''
  try {
    const [html, text] = await Promise.all([fetchText('html'), fetchText('txt')])
    const body = new DOMParser().parseFromString(html, 'text/html').body.innerHTML
    try {
      await navigator.clipboard.write([
        new ClipboardItem({ 'text/html': new Blob([body], { type: 'text/html' }), 'text/plain': new Blob([text], { type: 'text/plain' }) }),
      ])
    } catch (e) {
      // No rich clipboard here: copy a selection of the text as drawn.
      const box = document.createElement('div')
      box.contentEditable = 'true'
      box.style.cssText = 'position:fixed;left:-9999px;top:0;white-space:normal'
      box.innerHTML = body
      document.body.appendChild(box)
      const range = document.createRange()
      range.selectNodeContents(box)
      const sel = window.getSelection()
      sel.removeAllRanges()
      sel.addRange(range)
      const ok = document.execCommand('copy')
      sel.removeAllRanges()
      box.remove()
      if (!ok) throw e
    }
    note.value = t('Copied: paste it into the post.')
  } catch (e) {
    note.value = e.message || t("Couldn't copy")
  } finally {
    copying.value = false
  }
}

const away = (e) => {
  if (open.value && root.value && !root.value.contains(e.target)) open.value = false
}
onMounted(() => document.addEventListener('mousedown', away))
onBeforeUnmount(() => document.removeEventListener('mousedown', away))
</script>

<style scoped>
.export-menu {
  position: relative;
}

.export-menu:not(.is-inline) .export-list {
  position: absolute;
  right: 0;
  top: calc(100% + 0.35rem);
  z-index: 40;
  min-width: 16rem;
  padding: 0.45rem;
}

.export-list {
  display: grid;
  gap: 0.1rem;
}

.group {
  margin: 0.35rem 0.4rem 0.15rem;
  font-size: 0.7rem;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-faint);
}

.item {
  display: block;
  width: 100%;
  padding: 0.35rem 0.5rem;
  border: none;
  border-radius: 7px;
  background: none;
  color: var(--text-primary);
  font: inherit;
  font-size: 0.84rem;
  text-align: left;
  text-decoration: none;
  cursor: pointer;
}

.item:hover:not(:disabled),
.item:focus-visible {
  background: var(--glass);
}

.item:disabled {
  opacity: 0.5;
}

.note {
  margin: 0.3rem 0.5rem 0.1rem;
  font-size: 0.75rem;
  color: var(--accent);
}
</style>
