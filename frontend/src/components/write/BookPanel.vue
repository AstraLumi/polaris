<template>
  <aside class="side-panel glass-panel panel-solid book-panel" :aria-label="$t('Book and page')">
    <header class="side-head">
      <h2>{{ $t('Book and page') }}</h2>
      <button type="button" class="btn btn-ghost small" :aria-label="$t('Close')" @click="$emit('close')">✕</button>
    </header>

    <div class="type-line">
      <div>
        <span class="label">{{ $t('Page type') }}</span>
        <strong>{{ typeName }}</strong>
      </div>
      <button type="button" class="btn btn-ghost small" @click="$emit('change-type')">{{ $t('Change…') }}</button>
    </div>

    <!-- Library fonts the styles use that aren't cleared for a book that is sold. -->
    <div v-if="fontWarnings.length" class="font-warn" role="note">
      <p v-for="w in fontWarnings" :key="w.family">⚠ {{ fontWarningText(w) }}</p>
      <a href="/fonts" target="_blank" rel="noopener">{{ $t('Open the fonts library') }}</a>
    </div>

    <template v-if="!online">
      <p class="hint">{{ $t('Text block: {w} mm wide.', { w: textWidth }) }}</p>
      <label class="check">
        <input type="checkbox" :checked="showPages" @change="$emit('show-pages', $event.target.checked)" />
        {{ $t('Show pages while writing') }}
      </label>
      <p class="hint">{{ $t('Draws the gap between pages as you write, where each page ends in the page view. Only on this browser.') }}</p>
      <div v-if="settings.page_type === 'custom'" class="grid">
        <label class="field">
          <span>{{ $t('Width (mm)') }}</span>
          <input type="number" min="50" max="500" step="0.1" :value="page.width_mm" @change="setPage('width_mm', $event.target.value, 50, 500)" />
        </label>
        <label class="field">
          <span>{{ $t('Height (mm)') }}</span>
          <input type="number" min="50" max="500" step="0.1" :value="page.height_mm" @change="setPage('height_mm', $event.target.value, 50, 500)" />
        </label>
      </div>
      <h3>{{ $t('Margins (mm)') }}</h3>
      <div class="grid">
        <label class="field">
          <span>{{ $t('Top') }}</span>
          <input type="number" min="0" max="100" step="0.5" :value="page.margin_top_mm" @change="setPage('margin_top_mm', $event.target.value)" />
        </label>
        <label class="field">
          <span>{{ $t('Bottom') }}</span>
          <input type="number" min="0" max="100" step="0.5" :value="page.margin_bottom_mm" @change="setPage('margin_bottom_mm', $event.target.value)" />
        </label>
        <label class="field">
          <span>{{ page.mirror ? $t('Inside') : $t('Left') }}</span>
          <input type="number" min="0" max="100" step="0.5" :value="page.margin_inner_mm" @change="setPage('margin_inner_mm', $event.target.value)" />
        </label>
        <label class="field">
          <span>{{ page.mirror ? $t('Outside') : $t('Right') }}</span>
          <input type="number" min="0" max="100" step="0.5" :value="page.margin_outer_mm" @change="setPage('margin_outer_mm', $event.target.value)" />
        </label>
        <label class="field">
          <span>{{ $t('Gutter') }}</span>
          <input type="number" min="0" max="50" step="0.5" :value="page.gutter_mm" @change="setPage('gutter_mm', $event.target.value, 0, 50)" />
        </label>
        <label class="check">
          <input type="checkbox" :checked="!!page.mirror" @change="setPage('mirror', $event.target.checked)" />
          {{ $t('Mirror margins (facing pages)') }}
        </label>
      </div>
    </template>
    <p v-else class="hint">{{ $t('An online post has no pages: each chapter is one post, read at a comfortable width.') }}</p>

    <!-- The book's details. -->
    <details class="section" :open="openSection === 'meta'" @toggle="onToggle('meta', $event)">
      <summary>{{ $t('Book details') }}</summary>
      <label class="field">
        <span>{{ $t('Title') }}</span>
        <input type="text" maxlength="300" :value="meta.title" :placeholder="bookTitle" @input="setMeta('title', $event.target.value)" />
      </label>
      <label class="field">
        <span>{{ $t('Subtitle') }}</span>
        <input type="text" maxlength="300" :value="meta.subtitle" @input="setMeta('subtitle', $event.target.value)" />
      </label>
      <label class="field">
        <span>{{ $t('Author') }}</span>
        <input type="text" maxlength="300" :value="meta.author" @input="setMeta('author', $event.target.value)" />
      </label>
      <div class="grid">
        <label class="field">
          <span>{{ $t('Language') }}</span>
          <input type="text" maxlength="20" :value="meta.language" placeholder="en" @input="setMeta('language', $event.target.value)" />
        </label>
        <label class="field">
          <span>{{ $t('ISBN') }}</span>
          <input type="text" maxlength="40" :value="meta.isbn" @input="setMeta('isbn', $event.target.value)" />
        </label>
      </div>
      <label class="field">
        <span>{{ $t('Publisher') }}</span>
        <input type="text" maxlength="300" :value="meta.publisher" @input="setMeta('publisher', $event.target.value)" />
      </label>
      <label class="field">
        <span>{{ $t('Rights') }}</span>
        <input type="text" maxlength="300" :value="meta.rights" :placeholder="$t('© 2026 Your Name')" @input="setMeta('rights', $event.target.value)" />
      </label>
      <label class="field">
        <span>{{ $t('Description') }}</span>
        <textarea rows="3" maxlength="5000" :value="meta.description" @input="setMeta('description', $event.target.value)"></textarea>
      </label>
    </details>

    <template v-if="!online">
      <!-- Headers and footers. -->
      <details class="section" :open="openSection === 'bands'" @toggle="onToggle('bands', $event)">
        <summary>{{ $t('Headers and footers') }}</summary>
        <label class="check">
          <input type="checkbox" :checked="layout.different_first" @change="setLayout({ different_first: $event.target.checked })" />
          {{ $t("A chapter's first page has its own") }}
        </label>
        <label class="check">
          <input type="checkbox" :checked="layout.different_odd_even" @change="setLayout({ different_odd_even: $event.target.checked })" />
          {{ $t('Left and right pages differ') }}
        </label>

        <div class="seg" role="tablist" :aria-label="$t('Headers and footers')">
          <button v-for="p in PARTS" :key="p.key" type="button" role="tab" class="seg-btn" :class="{ 'is-on': part === p.key }" :aria-selected="part === p.key" @click="part = p.key">{{ p.label }}</button>
        </div>
        <div class="seg" role="tablist" :aria-label="$t('Pages')">
          <button
            v-for="s in slots"
            :key="s.key"
            type="button"
            role="tab"
            class="seg-btn"
            :class="{ 'is-on': slot === s.key }"
            :aria-selected="slot === s.key"
            @click="slot = s.key"
          >
            {{ s.label }}
          </button>
        </div>

        <div class="band">
          <label v-for="pos in POSITIONS" :key="pos.key" class="field band-text">
            <span>{{ pos.label }}</span>
            <span class="with-picker">
              <input
                :ref="(el) => (inputs[pos.key] = el)"
                type="text"
                maxlength="300"
                :value="band[pos.key]"
                @input="setBand({ [pos.key]: $event.target.value })"
                @focus="lastPos = pos.key"
              />
              <select class="field-pick" :aria-label="$t('Insert a field')" :title="$t('Insert a field')" value="" @change="insertField(pos.key, $event)">
                <option value="" disabled>+</option>
                <option v-for="f in FIELDS" :key="f.key" :value="f.key">{{ f.label }}</option>
              </select>
            </span>
          </label>
          <p class="preview" :class="{ 'is-empty': !previewText.trim() }" :style="previewStyle">
            <span>{{ preview.left }}</span><span>{{ preview.center }}</span><span>{{ preview.right }}</span>
          </p>
          <div class="grid">
            <label class="field">
              <span>{{ $t('Font') }}</span>
              <input type="text" list="write-fonts" :value="band.font || ''" :placeholder="$t('As the text')" @change="setBand({ font: $event.target.value.trim() || null })" />
            </label>
            <label class="field">
              <span>{{ $t('Size (pt)') }}</span>
              <input type="number" min="4" max="72" step="0.5" :value="band.size ?? ''" :placeholder="String(bodySize)" @change="setSize($event.target.value)" />
            </label>
          </div>
          <div class="checks">
            <label class="check"><input type="checkbox" :checked="band.italic" @change="setBand({ italic: $event.target.checked })" /> {{ $t('Italic') }}</label>
            <label class="check"><input type="checkbox" :checked="band.smallcaps" @change="setBand({ smallcaps: $event.target.checked })" /> {{ $t('Small caps') }}</label>
            <label class="check"><input type="checkbox" :checked="band.uppercase" @change="setBand({ uppercase: $event.target.checked })" /> {{ $t('Capitals') }}</label>
          </div>
        </div>
        <p class="hint">{{ $t('Front matter has no header; its pages are numbered i, ii, iii.') }}</p>
      </details>

      <!-- Page numbers and chapters. -->
      <details class="section" :open="openSection === 'numbers'" @toggle="onToggle('numbers', $event)">
        <summary>{{ $t('Page numbers and chapters') }}</summary>
        <div class="grid">
          <label class="field">
            <span>{{ $t('Numbers') }}</span>
            <select :value="layout.numbering.format" @change="setLayout({ numbering: { ...layout.numbering, format: $event.target.value } })">
              <option v-for="f in FORMATS" :key="f.key" :value="f.key">{{ f.label }}</option>
            </select>
          </label>
          <label class="field">
            <span>{{ $t('First number') }}</span>
            <input type="number" min="0" max="99999" step="1" :value="layout.numbering.start" @change="setStart($event.target.value)" />
          </label>
        </div>
        <label class="field">
          <span>{{ $t('Chapters start on') }}</span>
          <select :value="layout.chapter_start" @change="setLayout({ chapter_start: $event.target.value })">
            <option value="next">{{ $t('The next page') }}</option>
            <option value="odd">{{ $t('A right-hand page') }}</option>
          </select>
        </label>
        <label class="check">
          <input type="checkbox" :checked="layout.toc_dots" @change="setLayout({ toc_dots: $event.target.checked })" />
          {{ $t('Dots between titles and page numbers in the contents') }}
        </label>
      </details>

      <!-- Options per chapter. -->
      <details class="section" :open="openSection === 'chapters'" @toggle="onToggle('chapters', $event)">
        <summary>{{ $t('Chapters') }}</summary>
        <p class="hint">{{ $t('Front matter (title page, dedication, contents) is numbered in lower roman and has no header.') }}</p>
        <ul class="ch-opts">
          <li v-for="c in chapters" :key="c.id" :ref="(el) => (rows[c.id] = el)" :class="{ 'is-focus': c.id === focusChapter }">
            <span class="ch-name">{{ c.title }}</span>
            <label class="check small"><input type="checkbox" :checked="opt(c.id).front_matter" @change="setOpt(c.id, { front_matter: $event.target.checked })" /> {{ $t('Front matter') }}</label>
            <label class="check small"><input type="checkbox" :checked="opt(c.id).no_header" @change="setOpt(c.id, { no_header: $event.target.checked })" /> {{ $t('No header') }}</label>
            <label class="restart small">
              {{ $t('Restart at') }}
              <input type="number" min="0" max="99999" step="1" :value="opt(c.id).number_restart ?? ''" placeholder="—" @change="setRestart(c.id, $event.target.value)" />
            </label>
          </li>
        </ul>
      </details>
    </template>

    <!-- Every font the book uses, and whether it may be published with. -->
    <details class="section" :open="openSection === 'fonts'" @toggle="onToggle('fonts', $event)">
      <summary>{{ $t('Fonts and licenses') }}</summary>
      <p v-if="fontsError" class="hint">{{ fontsError }}</p>
      <p v-else-if="fontsLoading && !fontRows.length" class="hint">{{ $t('Loading…') }}</p>
      <p v-else-if="!fontRows.length" class="hint">{{ $t('This book names no fonts yet.') }}</p>
      <ul v-else class="font-rows">
        <li v-for="r in fontRows" :key="r.family" class="font-row" data-font-row :data-verdict="r.verdict">
          <div class="font-head">
            <span class="verdict" :class="'v-' + r.verdict" :title="verdictText(r)" role="img" :aria-label="verdictText(r)">{{ VERDICT_ICON[r.verdict] || '?' }}</span>
            <span class="font-name" :style="{ fontFamily: `'${r.family}', serif` }">{{ r.family }}</span>
          </div>
          <div class="font-where">{{ whereUsed(r) }}</div>
          <div class="font-meta">
            <span>{{ sourceName(r.source) }}</span>
            <template v-if="licenseLabel(r)"> · <a v-if="r.license_url" :href="r.license_url" target="_blank" rel="noopener noreferrer">{{ licenseLabel(r) }}</a><span v-else>{{ licenseLabel(r) }}</span></template>
            <template v-if="r.source_url"> · <a v-if="/^https?:/.test(r.source_url)" :href="r.source_url" target="_blank" rel="noopener noreferrer">{{ $t('Source') }}</a><span v-else>{{ r.source_url }}</span></template>
          </div>
          <div v-if="r.allowed" class="font-tags">
            <span v-for="u in USES" :key="u.key" class="tag" :class="{ off: !r.allowed[u.key] }">{{ u.label }}</span>
          </div>
          <div v-if="r.copyright" class="font-copy">{{ r.copyright }}</div>
          <div class="font-verdict" :class="'v-' + r.verdict">{{ verdictText(r) }}</div>
          <!-- A flagged font used in the text: step through those places. -->
          <div v-if="r.verdict !== 'ok' && placeCount(r.places)" class="font-find" data-font-find>
            <button type="button" class="seg-btn step" :title="$t('Previous')" :aria-label="$t('Previous')" @click="stepFont(r, -1)">‹</button>
            <button type="button" class="seg-btn" @click="stepFont(r, 1)">{{ $t('Show in the text') }}</button>
            <button type="button" class="seg-btn step" :title="$t('Next')" :aria-label="$t('Next')" @click="stepFont(r, 1)">›</button>
            <span class="font-step">{{ fontStep[r.family] != null ? $t('{n} of {total}', { n: fontStep[r.family] + 1, total: placeCount(r.places) }) : $tn('{n} place', '{n} places', placeCount(r.places)) }}</span>
          </div>
        </li>
      </ul>
      <button v-if="creditsLine" type="button" class="seg-btn copy-credits" @click="copyCredits">{{ creditsCopied ? $t('Copied') : $t('Copy credits') }}</button>
    </details>

    <!-- Exports: the book or the open chapter. -->
    <details class="section" :open="openSection === 'export'" @toggle="onToggle('export', $event)">
      <summary>{{ $t('Export') }}</summary>
      <ExportMenu inline :volume="volume" :chapter-id="openChapter" :online="online" @pdf="printPdf" />
    </details>
  </aside>
</template>

<script setup>
// The book's page (type, size, margins), its details, its headers and
// footers (bands of three texts with fields), page numbers, where
// chapters start and the options of each chapter (docs/EDITOR.md, "Stage
// 3 and 4 additions"). Every change goes up as the whole settings.
import { ref, computed, watch, nextTick, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import ExportMenu from './ExportMenu.vue'
import { t, tn } from '../../i18n'
import { textBlockMm, pageTypeName } from '../../editor/pageTypes.js'
import { resolveStyle } from '../../editor/styles.js'
import { normaliseLayout, normaliseMeta, chapterOption, setChapterOption, bandText, formatNumber } from '../../editor/bookLayout.js'
import { fontLibrary, fontWarningText } from '../../fontLibrary.js'
import { styleWarnings } from '../../fontRules.js'
import { fetchBookFonts } from '../../fontLibrary.js'
import { loadFonts } from '../../editor/fonts.js'
import { licenseLabel, creditsText } from '../../editor/fontCredits.js'
import { placeAt, placeCount } from '../../editor/fontPlaces.js'

const props = defineProps({
  settings: { type: Object, required: true },
  chapters: { type: Array, default: () => [] },
  bookTitle: { type: String, default: '' },
  focusChapter: { type: Number, default: null },
  showPages: { type: Boolean, default: true },
})
const emit = defineEmits(['update', 'change-type', 'show-pages', 'close'])
const route = useRoute()
const router = useRouter()
// The writing page's book and open chapter (from its address).
const volume = computed(() => String(route.params.volume ?? '0'))
const openChapter = computed(() => (/^\d+$/.test(String(route.query.chapter || '')) ? +route.query.chapter : null))
const printPdf = () => router.push({ path: `/write/${volume.value}/pages`, query: { print: '1' } })

const PARTS = computed(() => [
  { key: 'header', label: t('Header') },
  { key: 'footer', label: t('Footer') },
])
const POSITIONS = computed(() => [
  { key: 'left', label: t('Left') },
  { key: 'center', label: t('Centre') },
  { key: 'right', label: t('Right') },
])
const FIELDS = computed(() => [
  { key: '{page}', label: t('Page number') },
  { key: '{chapter}', label: t('Chapter title') },
  { key: '{chapter_number}', label: t('Chapter number') },
  { key: '{book}', label: t('Book title') },
  { key: '{author}', label: t('Author') },
])
const FORMATS = computed(() => [
  { key: 'arabic', label: '1, 2, 3' },
  { key: 'roman', label: 'i, ii, iii' },
  { key: 'upper_roman', label: 'I, II, III' },
  { key: 'alpha', label: 'a, b, c' },
  { key: 'upper_alpha', label: 'A, B, C' },
])

const online = computed(() => props.settings.page_type === 'online')
const fontWarnings = computed(() => styleWarnings(props.settings.styles, fontLibrary.value))
const page = computed(() => props.settings.page || {})
const textWidth = computed(() => textBlockMm(props.settings))
const typeName = computed(() => (online.value ? t('Online Post') : props.settings.page_type === 'custom' ? `${t('Custom size')} · ${pageTypeName(props.settings)}` : pageTypeName(props.settings)))
const layout = computed(() => normaliseLayout(props.settings.layout))
const meta = computed(() => normaliseMeta(props.settings.meta))
const normal = computed(() => resolveStyle(props.settings.styles || [], 'normal').text)
const bodySize = computed(() => normal.value.size || 10)

const openSection = ref(props.focusChapter != null ? 'chapters' : '')
const onToggle = (name, e) => {
  if (e.target.open) openSection.value = name
  else if (openSection.value === name) openSection.value = ''
}

// ---- Fonts and licenses -------------------------------------------------------------
const fontRows = ref([])
const fontsLoading = ref(false)
const fontsError = ref('')
const creditsCopied = ref(false)
const VERDICT_ICON = { ok: '✓', check: '?', warn: '!' }
const USES = computed(() => [
  { key: 'print', label: t('Print') },
  { key: 'ebook', label: t('E-book') },
  { key: 'web', label: t('Web') },
  { key: 'commercial', label: t('Commercial') },
])
const SOURCES = computed(() => ({
  library: t('Fonts library'),
  bundled: t('Comes with Polaris'),
  google: t('Google Fonts'),
  system: t('System font'),
  unknown: t('Unknown source'),
}))
const sourceName = (s) => SOURCES.value[s] || SOURCES.value.unknown
const REASONS = computed(() => ({
  open: t('Open license: fine to embed and publish.'),
  uses: t('Check the allowed uses of this font in your library.'),
  unknown: t('The license could not be found. Check it before publishing.'),
  embedding: t('The font file forbids embedding. Check before exporting.'),
  proprietary: t('Proprietary: needs a license from its owner to embed in an e-book or PDF.'),
}))
const verdictText = (r) => REASONS.value[r.reason] || ''
function whereUsed(r) {
  const parts = [...(r.styles || [])]
  if (r.bands) parts.push(r.bands === 1 ? t('Header or footer') : t('{n} headers and footers', { n: r.bands }))
  const out = parts.join(', ')
  const ch = r.chapters ? tn('{n} chapter', '{n} chapters', r.chapters) : ''
  return [out, ch].filter(Boolean).join(' · ')
}
const creditsLine = computed(() => creditsText(fontRows.value))
// Where each flagged font was last shown (family → its place, 0-based). The
// writing page opens the chapter and selects the run (?font=&run=).
const fontStep = ref({})
function stepFont(r, by) {
  const cur = fontStep.value[r.family]
  const k = cur == null ? (by > 0 ? 0 : -1) : cur + by
  const total = placeCount(r.places)
  const place = placeAt(r.places, k)
  if (!place) return
  fontStep.value = { ...fontStep.value, [r.family]: ((k % total) + total) % total }
  router.replace({ query: { ...route.query, chapter: String(place.chapter), font: r.family, run: String(place.run) } })
}
async function copyCredits() {
  try {
    await navigator.clipboard.writeText(creditsLine.value)
    creditsCopied.value = true
    setTimeout(() => (creditsCopied.value = false), 2000)
  } catch {
    fontsError.value = t('Copying failed.')
  }
}
let fontsSeq = 0
async function loadBookFonts() {
  const seq = ++fontsSeq
  fontsLoading.value = true
  try {
    const rows = await fetchBookFonts(volume.value)
    if (seq !== fontsSeq) return
    fontRows.value = rows
    fontsError.value = ''
    loadFonts(rows.map((r) => r.family)).catch(() => {})
  } catch (e) {
    if (seq === fontsSeq) fontsError.value = e.message || t('Failed to load the fonts')
  } finally {
    if (seq === fontsSeq) fontsLoading.value = false
  }
}
let fontsTimer = null
watch(
  () => [openSection.value, volume.value, props.settings.styles, props.settings.layout],
  () => {
    clearTimeout(fontsTimer)
    if (openSection.value === 'fonts') fontsTimer = setTimeout(loadBookFonts, 600)
  },
  { deep: true },
)

function setPage(key, raw, min = 0, max = 100) {
  let v = raw
  if (typeof raw !== 'boolean') {
    v = +raw
    if (raw === '' || !Number.isFinite(v)) return
    v = Math.min(max, Math.max(min, Math.round(v * 10) / 10))
  }
  emit('update', { ...props.settings, page: { ...page.value, [key]: v } })
}

const setMeta = (key, value) => emit('update', { ...props.settings, meta: { ...meta.value, [key]: value } })
const setLayout = (patch) => emit('update', { ...props.settings, layout: { ...layout.value, ...patch } })

// ---- The band being edited ----------------------------------------------------------
const part = ref('header')
const slot = ref('odd')
const slots = computed(() => {
  const out = [{ key: 'odd', label: layout.value.different_odd_even ? t('Right pages') : t('Every page') }]
  if (layout.value.different_odd_even) out.push({ key: 'even', label: t('Left pages') })
  if (layout.value.different_first) out.push({ key: 'first', label: t("Chapter's first page") })
  return out
})
watch(slots, (list) => {
  if (!list.some((s) => s.key === slot.value)) slot.value = 'odd'
})
const band = computed(() => layout.value[part.value][slot.value])
function setBand(patch) {
  const l = layout.value
  emit('update', { ...props.settings, layout: { ...l, [part.value]: { ...l[part.value], [slot.value]: { ...band.value, ...patch } } } })
}
function setSize(raw) {
  const n = +raw
  setBand({ size: raw === '' || !Number.isFinite(n) ? null : Math.min(72, Math.max(4, Math.round(n * 2) / 2)) })
}

// A field goes in where the cursor was in that box (at the end otherwise).
const inputs = {}
const lastPos = ref('center')
function insertField(pos, e) {
  const field = e.target.value
  e.target.value = ''
  if (!field) return
  const el = inputs[pos]
  const text = band.value[pos] || ''
  const at = el && document.activeElement === el ? el.selectionStart : el?.selectionStart ?? text.length
  const start = Number.isFinite(at) ? at : text.length
  setBand({ [pos]: text.slice(0, start) + field + text.slice(start) })
  nextTick(() => {
    el?.focus()
    el?.setSelectionRange(start + field.length, start + field.length)
  })
}

const sample = computed(() => ({
  page: formatNumber(slot.value === 'even' ? 12 : slot.value === 'first' ? 1 : 13, layout.value.numbering.format),
  chapter: props.chapters.find((c) => !opt(c.id).front_matter)?.title || t('Chapter title'),
  chapter_number: '1',
  book: meta.value.title || props.bookTitle,
  author: meta.value.author,
}))
const preview = computed(() => ({
  left: bandText(band.value.left, sample.value),
  center: bandText(band.value.center, sample.value),
  right: bandText(band.value.right, sample.value),
}))
const previewText = computed(() => preview.value.left + preview.value.center + preview.value.right)
const previewStyle = computed(() => ({
  fontFamily: band.value.font || normal.value.font || undefined,
  fontStyle: band.value.italic ? 'italic' : 'normal',
  fontVariant: band.value.smallcaps ? 'small-caps' : 'normal',
  textTransform: band.value.uppercase ? 'uppercase' : 'none',
}))

function setStart(raw) {
  const n = Math.round(+raw)
  if (raw === '' || !Number.isFinite(n)) return
  setLayout({ numbering: { ...layout.value.numbering, start: Math.min(99999, Math.max(0, n)) } })
}

// ---- Chapters ----------------------------------------------------------------------------
const rows = {}
const opt = (id) => chapterOption(props.settings, id)
const setOpt = (id, patch) => emit('update', setChapterOption(props.settings, id, patch))
function setRestart(id, raw) {
  const n = Math.round(+raw)
  setOpt(id, { number_restart: raw === '' || !Number.isFinite(n) ? null : Math.min(99999, Math.max(0, n)) })
}

function showFocus() {
  if (props.focusChapter == null) return
  openSection.value = 'chapters'
  nextTick(() => rows[props.focusChapter]?.scrollIntoView({ block: 'center' }))
}
watch(() => props.focusChapter, showFocus)
onMounted(showFocus)
</script>

<style scoped>
.font-warn {
  margin: 0.6rem 0 0;
  padding: 0.5rem 0.65rem;
  border: 1px solid var(--danger);
  border-radius: 8px;
  font-size: 0.78rem;
  line-height: 1.4;
  color: var(--danger-text);
}

.font-warn p {
  margin: 0 0 0.3rem;
}

.font-warn a {
  font-size: 0.75rem;
  color: var(--accent);
}

.type-line {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.6rem;
  padding: 0.6rem 0.7rem;
  border-radius: 10px;
  background: var(--glass);
  border: 1px solid var(--glass-border);
}

.label {
  display: block;
  font-size: 0.72rem;
  color: var(--text-faint);
}

h3 {
  margin: 0.9rem 0 0.4rem;
  font-size: 0.95rem;
}

.grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.5rem 0.6rem;
}

.field {
  margin: 0 0 0.5rem;
}

.grid .field {
  margin: 0;
}

.field > span {
  font-size: 0.75rem;
}

.check {
  grid-column: span 2;
  display: flex;
  align-items: center;
  gap: 0.4rem;
  margin: 0.3rem 0;
  font-size: 0.82rem;
  color: var(--text-muted);
}

.check.small,
.restart.small {
  font-size: 0.75rem;
  margin: 0;
}

.hint {
  margin: 0.6rem 0 0;
  font-size: 0.75rem;
  color: var(--text-faint);
  line-height: 1.4;
}

.section {
  margin-top: 0.8rem;
  padding: 0.5rem 0.7rem;
  border-radius: 10px;
  border: 1px solid var(--glass-border);
  background: color-mix(in srgb, var(--glass) 60%, transparent);
}

.section > summary {
  cursor: pointer;
  font-weight: 600;
  font-size: 0.92rem;
  padding: 0.2rem 0;
}

.section[open] > summary {
  margin-bottom: 0.5rem;
}

textarea {
  width: 100%;
  resize: vertical;
}

.seg {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem;
  margin: 0.5rem 0 0.3rem;
}

.seg-btn {
  flex: 1;
  min-height: 1.8rem;
  padding: 0.15rem 0.45rem;
  border: 1px solid var(--glass-border);
  border-radius: 7px;
  background: none;
  color: var(--text-muted);
  font: inherit;
  font-size: 0.76rem;
  cursor: pointer;
}

.seg-btn.is-on {
  background: var(--accent-soft);
  border-color: color-mix(in srgb, var(--accent) 55%, transparent);
  color: var(--text-primary);
}

.band {
  margin-top: 0.4rem;
}

.with-picker {
  display: flex;
  gap: 0.25rem;
}

.with-picker input {
  flex: 1;
  min-width: 0;
}

.field-pick {
  width: 2.6rem;
  flex: none;
  padding: 0.1rem 0.3rem;
}

.preview {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 0.3rem;
  margin: 0.2rem 0 0.6rem;
  padding: 0.45rem 0.6rem;
  border-radius: 6px;
  background: #fbf8f1;
  color: #1f1c17;
  font-size: 0.78rem;
  min-height: 1.9rem;
}

.preview span:nth-child(2) {
  text-align: center;
}

.preview span:nth-child(3) {
  text-align: right;
}

.preview.is-empty {
  opacity: 0.5;
}

.checks {
  display: flex;
  flex-wrap: wrap;
  gap: 0 0.9rem;
  margin-top: 0.4rem;
}

.checks .check {
  margin: 0;
}

.ch-opts {
  list-style: none;
  margin: 0.5rem 0 0;
  padding: 0;
  display: grid;
  gap: 0.35rem;
}

.ch-opts li {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.2rem 0.7rem;
  padding: 0.35rem 0.5rem;
  border-radius: 8px;
  border: 1px solid transparent;
}

.ch-opts li.is-focus {
  border-color: color-mix(in srgb, var(--accent) 55%, transparent);
  background: var(--accent-soft);
}

.ch-name {
  flex-basis: 100%;
  font-size: 0.84rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.restart {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  color: var(--text-muted);
}

.restart input {
  width: 4.2rem;
  min-height: 1.6rem;
  height: 1.6rem;
  padding: 0.05rem 0.35rem;
}

.font-rows {
  list-style: none;
  margin: 0 0 0.5rem;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
}
.font-row {
  display: flex;
  flex-direction: column;
  gap: 0.1rem;
  padding-bottom: 0.45rem;
  border-bottom: 1px solid var(--glass-border);
  font-size: 0.82rem;
}
.font-head {
  display: flex;
  align-items: center;
  gap: 0.4rem;
}
.font-name {
  font-size: 1.05rem;
}
.verdict {
  display: inline-grid;
  place-items: center;
  width: 1.15rem;
  height: 1.15rem;
  border-radius: 50%;
  font-size: 0.72rem;
  font-weight: 700;
  color: #fff;
  flex: none;
}
.verdict.v-ok { background: #3f8f5a; }
.verdict.v-check { background: #b58a1f; }
.verdict.v-warn { background: #c4503f; }
.font-where, .font-meta, .font-copy { opacity: 0.8; overflow-wrap: anywhere; }
.font-copy { font-size: 0.76rem; }
.font-tags { display: flex; flex-wrap: wrap; gap: 0.25rem; margin: 0.15rem 0; }
.font-tags .tag {
  padding: 0 0.4rem;
  border-radius: 999px;
  border: 1px solid var(--glass-border);
  font-size: 0.72rem;
}
.font-tags .tag.off { opacity: 0.45; text-decoration: line-through; }
.font-verdict.v-ok { color: #3f8f5a; }
.font-verdict.v-check { color: #b58a1f; }
.font-verdict.v-warn { color: #c4503f; }

.font-find {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.3rem;
  margin-top: 0.35rem;
}

.font-find .seg-btn {
  flex: 0 0 auto;
  white-space: nowrap;
}

.font-find .step {
  min-width: 1.8rem;
}

.font-step {
  font-size: 0.78rem;
  color: var(--text-faint);
}
.copy-credits { flex: none; width: 100%; }
</style>
