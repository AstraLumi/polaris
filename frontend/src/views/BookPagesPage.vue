<template>
  <div class="pages-page" :class="{ 'is-online': online }">
    <div class="pages-head">
      <Breadcrumbs :crumbs="crumbs" />
      <div class="pages-actions">
        <span class="pages-status" aria-live="polite">{{ statusText }}</span>
        <label v-if="!online && done" class="zoom">
          <span class="sr-only">{{ $t('Zoom') }}</span>
          <select v-model="zoomChoice" :aria-label="$t('Zoom')">
            <option value="auto">{{ $t('Fit ({n}%)', { n: Math.round(fitZoom * 100) }) }}</option>
            <option v-for="z in ZOOMS" :key="z" :value="String(z)">{{ Math.round(z * 100) }}%</option>
          </select>
        </label>
        <button v-if="!online" type="button" class="btn btn-ghost small" :disabled="!done" @click="relayout">{{ $t('Lay out again') }}</button>
        <button v-if="!online" type="button" class="btn btn-primary small" :disabled="!done" @click="printBook">{{ $t('Print / save as PDF') }}</button>
        <ExportMenu v-if="book" :volume="volumeKey" :online="online" @pdf="printWhenDone" />
        <RouterLink :to="editorLink()" class="btn btn-ghost small">{{ $t('Back to writing') }}</RouterLink>
      </div>
    </div>

    <p v-if="error" class="error-banner">{{ error }}</p>
    <p v-if="!online && done" class="pages-hint">{{ $t('Click a page to write there.') }}</p>

    <div ref="area" class="pages-area" @click="onClick">
      <div v-if="laying" class="laying glass-panel panel-solid" role="status">
        <p>{{ $tn('Laying out the book… {n} page so far', 'Laying out the book… {n} pages so far', pages) }}</p>
        <div class="bar"><span :style="{ width: `${Math.round(progress * 100)}%` }"></span></div>
        <button type="button" class="btn btn-ghost small" @click="stopLayout">{{ $t('Stop') }}</button>
      </div>
      <p v-else-if="loading" class="loading-hint">{{ $t('Loading…') }}</p>
      <div v-if="online" ref="readingEl" class="pv-reading"></div>
      <div v-else ref="pagesEl" class="pv-pages pv-print-keep" :class="{ 'is-hidden': !done }" :style="{ zoom: done ? zoom : 1 }"></div>
    </div>
  </div>
</template>

<script setup>
// The page view of a book (docs/EDITOR.md, "Client"): the whole book laid
// out in pages by paged.js, read-only, as it prints; a click opens that
// chapter in the editor near that spot. An online post has no pages: it is
// a plain reading view. paged.js is loaded only here (editor/pageView.js).
import 'prosemirror-view/style/prosemirror.css'
import '../editor/editor.css'
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { t } from '../i18n'
import { chapterIndex, loadChapters } from '../chapters'
import { pageTitle } from '../navigation'
import { booksApi } from '../editor/api.js'
import { normaliseSettings } from '../editor/pageTypes.js'
import { styleSheet, styleFonts, resolveStyle } from '../editor/styles.js'
import { loadFonts } from '../editor/fonts.js'
import { measureRatios } from '../editor/fontMetrics.js'
import { pagedCSS, normaliseLayout, normaliseMeta, BAND_SLOTS } from '../editor/bookLayout.js'
import { buildBook, layOutBook } from '../editor/pageView.js'
import Breadcrumbs from '../components/Breadcrumbs.vue'
import ExportMenu from '../components/write/ExportMenu.vue'

const props = defineProps({ volume: { type: String, required: true } })
const router = useRouter()
const route = useRoute()

const ZOOMS = [0.5, 0.75, 1, 1.25, 1.5, 2]
const MM = 96 / 25.4

const volumeKey = computed(() => (/^\d+$/.test(props.volume) ? props.volume : '0'))
const book = ref(null)
const error = ref('')
const loading = ref(true)
const laying = ref(false)
const done = ref(false)
const pages = ref(0)
const progress = ref(0)
const area = ref(null)
const pagesEl = ref(null)
const readingEl = ref(null)
const areaWidth = ref(900)
const zoomChoice = ref('auto')

const settings = computed(() => book.value?.settings || null)
const online = computed(() => settings.value?.page_type === 'online')
const bookName = computed(() => {
  if (volumeKey.value === '0') return t('Chapters without a volume')
  return book.value?.title || chapterIndex.value.volumes.find((v) => String(v.id) === volumeKey.value)?.title || t('Book')
})
const crumbs = computed(() => [
  { label: t('Chapters'), to: '/chapters' },
  { label: bookName.value, to: `/write/${volumeKey.value}` },
  { label: online.value ? t('Reading view') : t('Page view') },
])
const statusText = computed(() => {
  if (online.value || loading.value) return ''
  if (laying.value) return t('Laying out…')
  if (done.value) return t('{n} pages', { n: pages.value })
  return ''
})

const pageWidthPx = computed(() => (+settings.value?.page?.width_mm || 148) * MM)
const fitZoom = computed(() => {
  const spread = areaWidth.value < 720 ? pageWidthPx.value : pageWidthPx.value * 2
  return Math.max(0.3, Math.min(2, Math.floor(((areaWidth.value - 32) / spread) * 20) / 20))
})
const zoom = computed(() => (zoomChoice.value === 'auto' ? fitZoom.value : +zoomChoice.value || 1))

const editorLink = (chapter, at) => ({ path: `/write/${volumeKey.value}`, query: chapter ? { chapter: String(chapter), ...(at != null ? { at: String(at) } : {}) } : {} })

// ---- Styles --------------------------------------------------------------------------
const styleEl = document.createElement('style')
styleEl.dataset.pageView = ''
const restartsEl = document.createElement('style') // page number restarts, once laid out

let handle = null
let docs = []

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [res, list] = await Promise.all([booksApi.get(volumeKey.value), booksApi.docs(volumeKey.value)])
    book.value = { ...res, settings: normaliseSettings(res.settings) }
    docs = list || []
    pageTitle.value = bookName.value
    await nextTick()
    await render()
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

// Every font the book names: its styles', its bands', its text's marks'.
function bookFonts(s, list) {
  const fonts = new Set(styleFonts(s.styles))
  const layout = normaliseLayout(s.layout)
  for (const part of ['header', 'footer']) for (const slot of BAND_SLOTS) if (layout[part][slot].font) fonts.add(layout[part][slot].font)
  const walk = (n) => {
    if (!n || typeof n !== 'object') return
    for (const m of n.marks || []) if (m?.type === 'font' && m.attrs?.family) fonts.add(m.attrs.family)
    for (const c of n.content || []) walk(c)
  }
  for (const d of list) walk(d.doc)
  return [...fonts]
}

async function render() {
  const s = settings.value
  if (!s) return
  done.value = false
  pages.value = 0
  progress.value = 0
  restartsEl.textContent = ''
  const fonts = bookFonts(s, docs)
  await loadFonts(fonts)
  await document.fonts?.ready
  await measureRatios(fonts) // line heights follow each font's own spacing
  styleEl.textContent = styleSheet(s.styles, '.pv-doc')
  const { fragment, plan, blocks } = buildBook(docs, s, { paged: !online.value })
  if (online.value) {
    readingEl.value.textContent = ''
    readingEl.value.appendChild(fragment)
    return
  }
  const normal = resolveStyle(s.styles, 'normal').text
  const meta = normaliseMeta(s.meta)
  const css = pagedCSS(s, { plan, book: meta.title || book.value?.title || '', author: meta.author, font: normal.font, size: normal.size, color: '#000' })
  pagesEl.value.textContent = ''
  laying.value = true
  try {
    const run = await layOutBook(fragment, css, pagesEl.value, {
      onPage: (n, gi) => {
        pages.value = n
        if (gi >= 0 && blocks) progress.value = Math.min(1, (gi + 1) / blocks)
      },
    })
    handle = run.handle
    const finished = await run.done
    restartsEl.textContent = handle.restartsCSS()
    pages.value = pagesEl.value?.querySelectorAll('.pagedjs_page').length || pages.value
    done.value = finished || pages.value > 0
    if (done.value && printAfter) {
      printAfter = false
      router.replace({ query: {} })
      printBook()
    }
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    laying.value = false
  }
}

// PDF from an export menu: printed as soon as the pages are laid out
// (?print=1 when coming from the writing page).
let printAfter = false
function printWhenDone() {
  if (done.value) printBook()
  else printAfter = true
}

function stopLayout() {
  handle?.cancel()
}

async function relayout() {
  handle?.destroy()
  handle = null
  await render()
}

// Every picture shown once first: Chromium leaves a picture that was never
// on screen out of the print (seen with a cover deep in the book).
const frame = () => new Promise((r) => requestAnimationFrame(() => r()))
async function printBook() {
  const imgs = [...(pagesEl.value?.querySelectorAll('img') || [])]
  await Promise.all(imgs.map((i) => i.decode().catch(() => {})))
  const box = area.value
  const top = box?.scrollTop ?? 0
  for (const img of imgs) {
    img.scrollIntoView({ block: 'nearest' })
    await frame()
    await frame()
  }
  if (box) box.scrollTop = top
  window.print()
}

// A click on a page: that chapter in the editor, at the nearest block. A
// link in the table of contents goes to its page instead.
function onClick(e) {
  const link = e.target.closest?.('.pv-toc a[href^="#"]')
  if (link) {
    e.preventDefault()
    const id = link.getAttribute('href').slice(1)
    const target = area.value?.querySelector(`[data-id="${CSS.escape(id)}"], #${CSS.escape(id)}`)
    target?.closest('.pagedjs_page')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    return
  }
  if (!done.value && !online.value) return
  let el = e.target.closest?.('[data-blk]')
  if (!el) {
    const page = e.target.closest?.('.pagedjs_page')
    el = page?.querySelector('[data-blk]')
  }
  if (!el) return
  router.push(editorLink(el.dataset.ch, el.dataset.blk))
}

let resizer = null
onMounted(() => {
  document.head.appendChild(styleEl)
  document.head.appendChild(restartsEl)
  document.documentElement.classList.add('pv-active')
  loadChapters()
  printAfter = route.query.print === '1'
  resizer = new ResizeObserver(([entry]) => (areaWidth.value = entry.contentRect.width))
  if (area.value) resizer.observe(area.value)
  load()
  if (import.meta.env.DEV) window.__pageView = { pages, done, laying }
})

onBeforeUnmount(() => {
  handle?.destroy()
  handle = null
  resizer?.disconnect()
  styleEl.remove()
  restartsEl.remove()
  document.documentElement.classList.remove('pv-active')
  pageTitle.value = ''
})
</script>

<style scoped>
.pages-page {
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
  height: calc(100vh - 3rem);
  min-height: 30rem;
}

.pages-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 0.4rem 1rem;
}

.pages-head :deep(.page-trail) {
  margin: 0;
}

.pages-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.4rem;
}

.pages-status {
  font-size: 0.82rem;
  color: var(--text-muted);
}

.pages-hint {
  margin: -0.2rem 0 0;
  font-size: 0.75rem;
  color: var(--text-faint);
}

.zoom select {
  min-height: 1.9rem;
  height: 1.9rem;
  padding: 0.1rem 1.6rem 0.1rem 0.5rem;
  font-size: 0.78rem;
  width: auto;
}

.pages-area {
  position: relative;
  flex: 1;
  min-height: 0;
  overflow: auto;
  border-radius: 14px;
  background: color-mix(in srgb, var(--surface) 55%, transparent);
  border: 1px solid var(--glass-border);
  padding: 1.5rem 1rem 3rem;
}

.laying {
  position: sticky;
  top: 1rem;
  z-index: 5;
  max-width: 24rem;
  margin: 2rem auto;
  padding: 1rem 1.2rem;
  text-align: center;
}

.laying p {
  margin: 0 0 0.7rem;
}

.bar {
  height: 6px;
  margin-bottom: 0.8rem;
  border-radius: 3px;
  background: var(--glass-border);
  overflow: hidden;
}

.bar span {
  display: block;
  height: 100%;
  background: var(--accent);
  transition: width 0.2s;
}

.pv-pages.is-hidden {
  visibility: hidden;
  height: 0;
  overflow: hidden;
}

.pv-reading {
  max-width: 40rem;
  margin: 0 auto;
  padding: 2rem 2.5rem;
  background: #fbf8f1;
  color: #1f1c17;
  border-radius: 10px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.25), 0 12px 40px rgba(0, 0, 0, 0.35);
  cursor: text;
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
}

@media (max-width: 720px) {
  .pages-page {
    height: auto;
    min-height: calc(100vh - 6rem);
  }
  .pages-area {
    padding: 0.75rem 0 2rem;
  }
  .pv-reading {
    padding: 1.2rem 1rem;
  }
}
</style>

<style>
/* The laid-out pages (paged.js's elements, not this component's). */
.pv-pages {
  --pv-gap: 22px;
  --ink: #111;
  --ink-faint: rgba(0, 0, 0, 0.4);
  --paper: #fff;
}

.pv-pages .pagedjs_pages {
  display: flex;
  flex-wrap: wrap;
  row-gap: var(--pv-gap);
  width: calc(var(--pagedjs-width) * 2);
  margin: 0 auto;
}

.pv-pages .pagedjs_page {
  background: #fff;
  color: #111;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.3), 0 8px 24px rgba(0, 0, 0, 0.25);
  cursor: pointer;
}

.pv-pages .pagedjs_first_page {
  margin-left: var(--pagedjs-width);
}

.pv-pages .pagedjs_blank_page .pagedjs_margin-content {
  visibility: hidden;
}

/* The text as the editor draws it, without the editor's helpers. */
.pv-doc {
  color: #111;
  white-space: pre-wrap;
}

.pv-doc .page-break,
.pv-doc .pv-page-break {
  border: 0;
  margin: 0;
  height: 0;
}

.pv-doc .page-break::after {
  content: none;
}

.pv-reading .pv-chapter + .pv-chapter,
.pv-reading .pv-full + .pv-chapter {
  margin-top: 3em;
  padding-top: 2em;
  border-top: 1px solid rgba(0, 0, 0, 0.12);
}

.pv-reading .page-break {
  display: none;
}

/* The table of contents with page numbers. */
.pv-toc {
  margin: 0 0 1em;
  white-space: normal;
  text-indent: 0;
}

/* Two classes: the book's stylesheet (one class) would win otherwise. */
.pv-toc .pv-toc-title {
  margin: 0 0 1em;
  text-indent: 0;
  text-align: center;
  font-weight: 700;
  font-size: 1.3em;
}

.pv-toc ol {
  margin: 0;
  padding: 0;
  list-style: none;
}

.pv-toc li {
  margin: 0 0 0.35em;
}

.pv-toc .pv-toc-l2 {
  padding-left: 1.2em;
}

.pv-toc .pv-toc-l3 {
  padding-left: 2.4em;
}

.pv-toc a {
  display: flex;
  align-items: baseline;
  color: inherit;
  text-decoration: none;
}

.pv-toc-dots {
  flex: 1;
  min-width: 1em;
  margin: 0 0.3em;
}

.pv-toc.has-dots .pv-toc-dots {
  border-bottom: 1px dotted currentColor;
  transform: translateY(-0.25em);
}

.pv-reading .pv-toc a::after {
  content: none;
}

/* Printing: only the pages, at their real size. */
@media print {
  html.pv-active,
  html.pv-active body {
    background: #fff !important;
  }
  html.pv-active body *:not(.pv-print-keep):not(.pv-print-keep *):not(:has(.pv-print-keep)) {
    display: none !important;
  }
  html.pv-active body :has(.pv-print-keep) {
    display: block !important;
    position: static !important;
    inset: auto !important;
    margin: 0 !important;
    padding: 0 !important;
    border: 0 !important;
    background: none !important;
    box-shadow: none !important;
    overflow: visible !important;
    height: auto !important;
    min-height: 0 !important;
    max-height: none !important;
    width: auto !important;
    max-width: none !important;
    transform: none !important;
    backdrop-filter: none !important;
    zoom: 1 !important;
  }
  html.pv-active .pv-print-keep {
    zoom: 1 !important;
    margin: 0 !important;
    padding: 0 !important;
  }
  html.pv-active .pv-pages .pagedjs_pages {
    width: auto;
  }
  html.pv-active .pv-pages .pagedjs_page {
    box-shadow: none;
    margin: 0 !important;
  }
}

@media (max-width: 720px) {
  .pv-pages .pagedjs_pages {
    width: var(--pagedjs-width);
  }
  .pv-pages .pagedjs_first_page {
    margin-left: 0;
  }
}
</style>
