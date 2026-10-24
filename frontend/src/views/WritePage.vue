<template>
  <div class="write-page" :class="{ 'is-focus': focusMode, 'is-dark-paper': darkPaper }" :style="pageVars">
    <div v-if="!focusMode" class="write-head">
      <Breadcrumbs :crumbs="crumbs">
        <RouterLink v-if="chapterId" :to="`/chapters/${chapterId}`" class="notes-link">{{ $t('Chapter notes') }}</RouterLink>
      </Breadcrumbs>
    </div>

    <p v-if="error" class="error-banner">
      {{ error }}
      <button type="button" class="link-btn" @click="error = ''">{{ $t('Dismiss') }}</button>
    </p>

    <WriteToolbar
      v-if="editorState && !focusMode"
      class="write-tools"
      :state="editorState"
      :styles="styles"
      :exec="exec"
      :online="online"
      :focus-mode="focusMode"
      :panel="panel"
      @find="openFind(false)"
      @panel="togglePanel"
      @focus="focusMode = !focusMode"
    />
    <div v-if="focusMode" class="focus-bar">
      <button type="button" class="btn btn-ghost small" @click="focusMode = false">{{ $t('Leave focus mode (Esc)') }}</button>
    </div>

    <div class="write-body" :class="{ 'has-side': panel && !focusMode }">
      <nav v-if="!focusMode" class="chapter-nav glass-panel" :aria-label="$t('Chapters of this book')">
        <p class="book-title">{{ bookTitle }}</p>
        <ol class="chapter-list">
          <li v-for="(c, i) in chapters" :key="c.id">
            <button type="button" class="chapter-item" :class="{ 'is-here': c.id === chapterId }" :aria-current="c.id === chapterId ? 'true' : undefined" @click="goChapter(c.id)">
              <span class="ch-num">{{ i + 1 }}</span>
              <span class="ch-title">{{ c.title }}</span>
              <span class="ch-words">{{ formatNum(c.id === chapterId ? words : c.words || 0) }}</span>
            </button>
          </li>
        </ol>
        <button type="button" class="btn btn-ghost small add-chapter" :disabled="adding" @click="addChapter">{{ $t('Add a chapter') }}</button>
        <button type="button" class="btn btn-ghost small add-chapter" :disabled="importing" :title="$t('Its chapters are added at the end of this book, split at each Heading 1')" @click="pickDocx">
          {{ importing ? $t('Importing…') : $t('Upload .docx') }}
        </button>
        <RouterLink to="/chapters" class="order-link">{{ $t('Reorder in Chapters') }}</RouterLink>
      </nav>
      <select v-if="!focusMode && chapters.length" class="chapter-select" :value="chapterId ?? ''" :aria-label="$t('Chapter')" @change="goChapter(+$event.target.value)">
        <option v-for="(c, i) in chapters" :key="c.id" :value="c.id">{{ i + 1 }}. {{ c.title }}</option>
      </select>

      <div ref="paperArea" class="paper-area">
        <FindPanel v-if="find.open && view && editorState" ref="findPanel" :view="view" :state="editorState" :replace="find.replace" :initial="find.initial" @close="closeFind" />

        <div v-if="offer" class="offer glass-panel panel-solid" role="alert">
          <p>{{ $t('This browser kept a copy of this chapter that was never saved ({when}).', { when: ago(offer.local.saved_at) }) }}</p>
          <div class="offer-actions">
            <button type="button" class="btn btn-primary small" :disabled="busy" @click="useLocal">{{ $t('Use that copy') }}</button>
            <button type="button" class="btn btn-ghost small" :disabled="busy" @click="compareLocal">{{ $t('Compare') }}</button>
            <button type="button" class="btn btn-ghost small" :disabled="busy" @click="discardLocal">{{ $t('Discard it') }}</button>
          </div>
        </div>

        <div v-if="book && !chapters.length" class="empty-state glass-panel">
          <p class="empty-title">{{ $t('This book has no chapters yet.') }}</p>
          <div class="empty-actions">
            <button type="button" class="btn btn-primary" :disabled="adding" @click="addChapter">{{ $t('Add a chapter') }}</button>
            <button type="button" class="btn btn-ghost" :disabled="importing" @click="pickDocx">{{ importing ? $t('Importing…') : $t('Upload .docx') }}</button>
          </div>
        </div>
        <p v-else-if="!book && !error" class="loading-hint">{{ $t('Loading…') }}</p>

        <div v-show="book && chapters.length" class="sheet" :class="{ 'is-online': online, 'is-compact': compact }" :style="sheetStyle">
          <div ref="editorEl" class="write-host"></div>
        </div>
      </div>

      <StylesPanel v-if="panel === 'styles' && !focusMode && settings" :styles="styles" :current-id="format?.styleId || null" @update="setStyles" @apply="applyStyle" @close="panel = ''" />
      <VersionsPanel v-if="panel === 'versions' && !focusMode && chapterId" :chapter-id="chapterId" :online="online" :flush="flushText" @restored="onRestored" @close="panel = ''" />
      <BookPanel v-if="panel === 'book' && !focusMode && settings" :settings="settings" @update="updateSettings" @change-type="typeDialog = 'switch'" @close="panel = ''" />
    </div>

    <footer class="write-status" :class="{ 'is-quiet': focusMode }">
      <span class="stat"><strong>{{ formatNum(words) }}</strong> {{ $tn('word in this chapter', 'words in this chapter', words) }}</span>
      <span v-if="!focusMode" class="stat">{{ $tn('{n} word in the book', '{n} words in the book', bookWords, { n: formatNum(bookWords) }) }}</span>
      <span v-if="selWords" class="stat">{{ $tn('{n} selected', '{n} selected', selWords, { n: formatNum(selWords) }) }}</span>
      <span class="save-state" :class="`is-${saveState}`" aria-live="polite">{{ saveLabel }}</span>
      <span class="spacer"></span>
      <label v-if="!focusMode" class="zoom">
        <span class="sr-only">{{ $t('Zoom') }}</span>
        <select v-model="zoomChoice" :aria-label="$t('Zoom')">
          <option value="auto">{{ $t('Fit ({n}%)', { n: Math.round(autoZoom * 100) }) }}</option>
          <option v-for="z in ZOOMS" :key="z" :value="String(z)">{{ Math.round(z * 100) }}%</option>
        </select>
      </label>
      <button type="button" class="btn btn-ghost small" :aria-pressed="darkPaper" @click="darkPaper = !darkPaper">{{ darkPaper ? $t('Light paper') : $t('Dark paper') }}</button>
    </footer>

    <input ref="docxInput" type="file" accept=".docx,application/vnd.openxmlformats-officedocument.wordprocessingml.document" hidden @change="onDocx" />
    <ImportReport v-if="importReport && settings" :report="importReport" :settings="settings" :set-up="book?.is_set_up !== false" :busy="busy" @apply="applyImport" @close="importReport = null" />
    <datalist id="write-fonts"><option v-for="f in fontList" :key="f" :value="f"></option></datalist>

    <PageTypeDialog v-if="typeDialog && settings" :current="settings" :switching="typeDialog === 'switch'" :busy="busy" @choose="chooseType" @cancel="typeDialog = ''" @import="importInstead" />
    <ConflictView
      v-if="conflict"
      :title="conflict.kind === 'save' ? $t('This chapter was changed somewhere else') : $t('Two texts of this chapter')"
      :message="conflict.kind === 'save'
        ? $t('While you wrote here, another tab or device saved this chapter. Keep one of the two texts.')
        : $t('The copy kept in this browser and the saved text differ. Keep one of the two.')"
      :mine="conflict.mine"
      :theirs="conflict.theirs"
      :mine-label="conflict.kind === 'save' ? $t('Your text, in this tab') : $t('The copy in this browser')"
      :theirs-label="$t('The saved text')"
      :keep-mine-label="conflict.kind === 'save' ? $t('Keep mine') : $t('Use this browser\'s copy')"
      :keep-theirs-label="$t('Keep the saved text')"
      :mine-when="conflict.mineWhen"
      :theirs-when="conflict.theirsWhen"
      :online="online"
      :busy="busy"
      :cancellable="conflict.kind === 'local'"
      @keep="keepFromConflict"
      @cancel="conflict = null"
    />
  </div>
</template>

<script setup>
// Writing a book: one volume's chapters (or the chapters without a volume,
// book 0), the text of one at a time, formatted for the book's page type.
// docs/EDITOR.md is the contract; the editor's logic lives in src/editor/.
import 'prosemirror-view/style/prosemirror.css'
import 'prosemirror-gapcursor/style/gapcursor.css'
import '../editor/editor.css'
import { ref, shallowRef, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRoute, useRouter, onBeforeRouteLeave } from 'vue-router'
import { EditorState } from 'prosemirror-state'
import { EditorView } from 'prosemirror-view'
import { t, tr, locale } from '../i18n'
import { chaptersApi } from '../api'
import { activeStoryId } from '../stories'
import { chapterIndex, loadChapters } from '../chapters'
import { pageTitle } from '../navigation'
import { schema, docFromJSON, docToJSON, emptyDocJSON } from '../editor/schema.js'
import { editorPlugins } from '../editor/keymap.js'
import { findPlugin } from '../editor/findPlugin.js'
import { selectionFormat, setStyle, pasteIntoEmpty } from '../editor/commands.js'
import { styleSheet, styleFonts, resolveStyle } from '../editor/styles.js'
import { normaliseSettings, defaultSettings, switchPageType, textBlockMm, pageTypeName } from '../editor/pageTypes.js'
import { countNode } from '../editor/wordCount.js'
import { loadFonts, docFonts, fontChoices } from '../editor/fonts.js'
import { createSaver } from '../editor/autosave.js'
import { keepCopy, readCopy, dropCopy } from '../editor/localCopy.js'
import { booksApi, textApi } from '../editor/api.js'
import { ago } from '../editor/when.js'
import Breadcrumbs from '../components/Breadcrumbs.vue'
import WriteToolbar from '../components/write/WriteToolbar.vue'
import FindPanel from '../components/write/FindPanel.vue'
import StylesPanel from '../components/write/StylesPanel.vue'
import VersionsPanel from '../components/write/VersionsPanel.vue'
import BookPanel from '../components/write/BookPanel.vue'
import PageTypeDialog from '../components/write/PageTypeDialog.vue'
import ConflictView from '../components/write/ConflictView.vue'
import ImportReport from '../components/write/ImportReport.vue'

void [tr('Custom size'), tr('Online Post'), tr('Not kept after a conflict'), tr('Before using the copy kept in this browser')]

const props = defineProps({ volume: { type: String, required: true } })
const route = useRoute()
const router = useRouter()

const ZOOMS = [0.75, 1, 1.25, 1.5, 2]

// ---- The book ----------------------------------------------------------------
const book = ref(null) // { settings, chapters, is_set_up }
const error = ref('')
const busy = ref(false)
const adding = ref(false)
const typeDialog = ref('') // '' | 'first' | 'switch'
const panel = ref('')
const focusMode = ref(false)

const volumeKey = computed(() => (/^\d+$/.test(props.volume) ? props.volume : '0'))
const settings = computed(() => book.value?.settings || null)
const styles = computed(() => settings.value?.styles || [])
const chapters = computed(() => book.value?.chapters || [])
const online = computed(() => settings.value?.page_type === 'online')
const bookTitle = computed(() => {
  if (volumeKey.value === '0') return t('Chapters without a volume')
  return book.value?.title || chapterIndex.value.volumes.find((v) => String(v.id) === volumeKey.value)?.title || t('Book')
})

// ---- The open chapter ------------------------------------------------------------
const chapterId = ref(null)
const chapter = computed(() => chapters.value.find((c) => c.id === chapterId.value) || null)
const editorEl = ref(null)
const paperArea = ref(null)
const editorState = shallowRef(null)
let view = null
let saver = null
const saveState = ref('saved')
const loadingText = ref(false)
const offer = ref(null) // { local, server }: a copy in this browser the server doesn't have
const conflict = ref(null) // { kind: 'save' | 'local', mine, theirs, version, … }
const words = ref(0)
const selWords = ref(0)

const crumbs = computed(() => [{ label: t('Chapters'), to: '/chapters' }, { label: bookTitle.value }, { label: chapter.value?.title || t('Write') }])
watch(
  () => chapter.value?.title,
  (title) => (pageTitle.value = title || ''),
)

const bookWords = computed(() => chapters.value.reduce((n, c) => n + (c.id === chapterId.value ? words.value : c.words || 0), 0))
const format = computed(() => (editorState.value ? selectionFormat(editorState.value, styles.value) : null))
const readOnly = computed(() => loadingText.value || !!conflict.value || !!offer.value || chapterId.value == null)
const saveLabel = computed(
  () =>
    ({
      saved: t('Saved'),
      saving: t('Saving…'),
      local: t('Not saved yet: kept in this browser'),
      conflict: t('Changed somewhere else: choose a text'),
    })[saveState.value],
)
const formatNum = (n) => Number(n || 0).toLocaleString(locale.value)

// ---- Paper, zoom and the page's size -------------------------------------------------
const PAPER_KEY = 'polaris-write-paper'
const ZOOM_KEY = 'polaris-write-zoom'
const stored = (k, d) => {
  try {
    return localStorage.getItem(k) ?? d
  } catch (e) {
    return d
  }
}
const store = (k, v) => {
  try {
    localStorage.setItem(k, v)
  } catch (e) {
    /* remembered for this visit only */
  }
}
const darkPaper = ref(stored(PAPER_KEY, '') === 'dark')
watch(darkPaper, (d) => store(PAPER_KEY, d ? 'dark' : 'light'))
const zoomChoice = ref(stored(ZOOM_KEY, 'auto'))
watch(zoomChoice, (z) => store(ZOOM_KEY, z))

const areaWidth = ref(800)
let resizer = null
const compact = computed(() => areaWidth.value < 560)
const normalSize = computed(() => resolveStyle(styles.value, 'normal').text.size || 12)
const MM = 96 / 25.4
const PT = 96 / 72

// The sheet: the text block exactly as wide as the page type's, with the
// margins round it (narrow screens keep only a little of them).
const geometry = computed(() => {
  const s = settings.value
  if (!s) return null
  if (online.value) {
    const width = normalSize.value * 34
    const pad = compact.value ? normalSize.value * 1.2 : normalSize.value * 3
    return { widthPx: width * PT, padPx: pad * PT * 2, width: `${width}pt`, padding: `${pad}pt`, minHeight: null }
  }
  const p = s.page || {}
  const text = textBlockMm(s)
  const inner = (+p.margin_inner_mm || 0) + (+p.gutter_mm || 0)
  const outer = +p.margin_outer_mm || 0
  const pad = compact.value ? [4, 4, 6, 4] : [+p.margin_top_mm || 0, outer, +p.margin_bottom_mm || 0, inner]
  return {
    widthPx: text * MM,
    padPx: (pad[1] + pad[3]) * MM,
    width: `${text}mm`,
    padding: pad.map((v) => `${v}mm`).join(' '),
    minHeight: `${Math.max(20, (+p.height_mm || 0) - (+p.margin_top_mm || 0) - (+p.margin_bottom_mm || 0))}mm`,
  }
})

// "Fit": the body text at a comfortable size on screen, as far as the
// sheet fits the space.
const autoZoom = computed(() => {
  const g = geometry.value
  if (!g) return 1
  const comfy = Math.min(2.5, Math.max(1, 16 / (normalSize.value * PT)))
  const fit = (areaWidth.value - 24) / (g.widthPx + g.padPx)
  return Math.max(0.5, Math.floor(Math.min(comfy, fit) * 20) / 20)
})
const zoom = computed(() => (zoomChoice.value === 'auto' ? autoZoom.value : +zoomChoice.value || 1))

const sheetStyle = computed(() => {
  const g = geometry.value
  if (!g) return {}
  return { width: g.width, padding: g.padding, minHeight: g.minHeight, zoom: zoom.value }
})
const pageVars = computed(() => ({ '--page-break-label': JSON.stringify(t('Page break')) }))

// ---- Styles and fonts ----------------------------------------------------------------------
const styleEl = document.createElement('style')
styleEl.dataset.write = ''
watch(styles, (s) => (styleEl.textContent = styleSheet(s, '.write-doc')), { immediate: true, deep: true })
watch(styles, (s) => loadFonts(styleFonts(s)), { immediate: true })
const docFontList = ref([])
const fontList = computed(() => fontChoices([...styleFonts(styles.value), ...docFontList.value]))
let fontTimer = null
function refreshDocFonts() {
  clearTimeout(fontTimer)
  fontTimer = setTimeout(() => {
    if (!view) return
    docFontList.value = docFonts(view.state.doc)
    loadFonts(docFontList.value)
  }, 600)
}

// ---- The editor ----------------------------------------------------------------------------
const plugins = [...editorPlugins({ getStyles: () => styles.value }), findPlugin()]
const makeState = (json) => EditorState.create({ schema, doc: docFromJSON(json), plugins })

let countTimer = null
function recount() {
  clearTimeout(countTimer)
  countTimer = null
  if (!view) return
  words.value = countNode(view.state.doc)
}
function countSelection() {
  if (!view) return
  const { from, to, empty } = view.state.selection
  selWords.value = empty ? 0 : countNode(view.state.doc, from, to)
}

function setDoc(json) {
  const st = makeState(json)
  view.updateState(st)
  editorState.value = st
  recount()
  countSelection()
  refreshDocFonts()
}

// Runs a toolbar command and puts the cursor back in the text.
function exec(cmd) {
  if (!view || readOnly.value) return
  cmd(view.state, view.dispatch, view)
  view.focus()
}

function createView() {
  view = new EditorView(editorEl.value, {
    state: makeState(emptyDocJSON()),
    editable: () => !readOnly.value,
    attributes: () => ({ class: `write-doc${online.value ? ' is-online' : ''}`, spellcheck: 'true' }),
    dispatchTransaction(tr) {
      const st = view.state.apply(tr)
      view.updateState(st)
      editorState.value = st
      if (tr.docChanged) {
        saver?.changed()
        clearTimeout(countTimer)
        countTimer = setTimeout(recount, 250)
        refreshDocFonts()
      }
      if (tr.docChanged || tr.selectionSet) countSelection()
    },
    handlePaste(v, e, slice) {
      const tr = pasteIntoEmpty(v.state, slice)
      if (!tr) return false
      v.dispatch(tr)
      return true
    },
    handleDOMEvents: {
      // Ctrl+click opens a link (a plain click only places the cursor).
      click(v, e) {
        const a = e.target.closest?.('a')
        if (!a || !(e.ctrlKey || e.metaKey)) return false
        e.preventDefault()
        if (a.dataset.storyType) router.push(`/wiki/${a.dataset.storyType}/${a.dataset.storyId}`)
        else if (a.getAttribute('href')) window.open(a.getAttribute('href'), '_blank', 'noopener')
        return true
      },
    },
  })
  editorState.value = view.state
  // For the screenshot script (frontend/tests/visual/writeShots.mjs).
  if (import.meta.env.DEV) window.__writeView = view
}
watch([readOnly, online], () => view?.setProps({}))

// ---- Loading ------------------------------------------------------------------------------------
const normal = (json) => docToJSON(docFromJSON(json))
const sameDoc = (a, b) => JSON.stringify(normal(a)) === JSON.stringify(normal(b))
const wait = (ms) => new Promise((r) => setTimeout(r, ms))

async function loadBook() {
  error.value = ''
  try {
    const res = await booksApi.get(volumeKey.value)
    book.value = { ...res, settings: normaliseSettings(res.settings), chapters: res.chapters || [] }
    if (res.is_set_up === false) typeDialog.value = 'first'
    pickChapter()
  } catch (e) {
    error.value = e.message
  }
}

// The chapter in the address, or the book's first.
function pickChapter() {
  if (!book.value) return
  const want = Number(route.query.chapter)
  const list = chapters.value
  const id = list.some((c) => c.id === want) ? want : list[0]?.id ?? null
  if (id !== chapterId.value) openChapter(id)
  if (id != null && want !== id) router.replace({ query: { ...route.query, chapter: String(id) } })
}

function goChapter(id) {
  if (id !== chapterId.value) router.replace({ query: { ...route.query, chapter: String(id) } })
}

// Sends what was typed (waiting a few seconds at most).
async function flushText() {
  if (!saver) return true
  return Promise.race([saver.flush(), wait(5000).then(() => false)])
}

async function closeChapter() {
  const s = saver
  saver = null
  if (!s) return
  await Promise.race([s.flush(), wait(5000)]).catch(() => {})
  s.dispose()
}

let opening = 0
async function openChapter(id) {
  const token = ++opening
  loadingText.value = true
  offer.value = null
  conflict.value = null
  if (find.value.open) closeFind()
  try {
    await closeChapter()
    if (token !== opening) return
    chapterId.value = id
    if (id == null) {
      setDoc(emptyDocJSON())
      return
    }
    let server = null
    let local = null
    try {
      server = await textApi.get(id)
    } catch (e) {
      // The server can't be reached: what this browser kept is better than nothing.
      local = await readCopy(activeStoryId, id)
      if (token !== opening) return
      if (!local) throw e
      error.value = t("The server can't be reached. Showing the copy kept in this browser; it is sent when the server is back.")
      setDoc(local.doc)
      startSaver(id, local.base_version)
      saver.changed()
      return
    }
    if (token !== opening) return
    const serverDoc = normal(server.doc)
    setDoc(serverDoc)
    startSaver(id, server.version || 0)
    local = await readCopy(activeStoryId, id)
    if (token !== opening) return
    if (local && local.doc) {
      if (sameDoc(local.doc, serverDoc)) dropCopy(activeStoryId, id)
      else offer.value = { local, server: { ...server, doc: serverDoc } }
    }
  } catch (e) {
    error.value = e.message
  } finally {
    if (token === opening) {
      loadingText.value = false
      nextTick(() => view?.focus())
    }
  }
}

function startSaver(id, version) {
  saver?.dispose()
  saveState.value = 'saved'
  saver = createSaver({
    getDoc: () => docToJSON(view.state.doc),
    save: (doc, base) => textApi.save(id, doc, base),
    keep: (entry) => keepCopy(activeStoryId, id, entry),
    drop: () => dropCopy(activeStoryId, id),
    base: version,
    onState: (s) => {
      if (chapterId.value === id) saveState.value = s
    },
    onSaved: (res) => {
      const c = book.value?.chapters.find((x) => x.id === id)
      if (c && res) {
        if (res.words != null) c.words = res.words
        if (res.version != null) c.version = res.version
      }
    },
    onConflict: (stored, mine) => {
      if (chapterId.value !== id) return
      conflict.value = {
        kind: 'save',
        mine,
        theirs: normal(stored?.doc),
        version: stored?.version ?? 0,
        theirsWhen: stored?.updated_at ? ago(stored.updated_at) : '',
      }
    },
  })
}

// ---- A copy kept in this browser, and conflicts -----------------------------------------------
async function useLocal() {
  const { local, server } = offer.value
  busy.value = true
  try {
    if (local.base_version !== server.version) await textApi.saveVersion(chapterId.value, t('Before using the copy kept in this browser'))
    offer.value = null
    conflict.value = null
    setDoc(local.doc)
    saver.base = server.version
    saver.changed()
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

function discardLocal() {
  dropCopy(activeStoryId, chapterId.value)
  offer.value = null
  conflict.value = null
}

function compareLocal() {
  const { local, server } = offer.value
  conflict.value = { kind: 'local', mine: normal(local.doc), theirs: server.doc, mineWhen: ago(local.saved_at), theirsWhen: server.updated_at ? ago(server.updated_at) : '' }
}

async function keepFromConflict(which) {
  const c = conflict.value
  if (!c) return
  if (c.kind === 'local') return which === 'mine' ? useLocal() : discardLocal()
  const id = chapterId.value
  busy.value = true
  try {
    if (which === 'mine') {
      // The other text becomes a version; mine goes on top of it.
      await textApi.saveVersion(id, t('Not kept after a conflict'))
      conflict.value = null
      saver.resolved(c.version, true)
    } else {
      // Mine is saved as a version; the stored text stays.
      await textApi.saveVersion(id, t('Not kept after a conflict'), c.mine)
      conflict.value = null
      setDoc(c.theirs)
      saver.resolved(c.version, false)
    }
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

// A saved version was put back.
async function onRestored(res) {
  try {
    const text = res && res.doc ? res : await textApi.get(chapterId.value)
    setDoc(normal(text.doc))
    saver?.resolved(text.version, false)
  } catch (e) {
    error.value = e.message
  }
}

// ---- Book settings ----------------------------------------------------------------------------
let settingsTimer = null
function updateSettings(next) {
  book.value = { ...book.value, settings: next }
  clearTimeout(settingsTimer)
  settingsTimer = setTimeout(saveSettings, 700)
}
async function saveSettings() {
  clearTimeout(settingsTimer)
  settingsTimer = null
  try {
    await booksApi.save(volumeKey.value, settings.value)
    if (book.value) book.value = { ...book.value, is_set_up: true }
  } catch (e) {
    error.value = e.message
  }
}
const setStyles = (list) => updateSettings({ ...settings.value, styles: list })
const applyStyle = (id) => exec(setStyle(id))

const typeLabel = (s) => (s.page_type === 'online' ? t('Online Post') : pageTypeName(s))

async function chooseType({ type, width_mm, height_mm }) {
  busy.value = true
  try {
    if (typeDialog.value === 'first') {
      const next = defaultSettings(type, { width_mm, height_mm }, book.value?.defaults)
      const res = await booksApi.save(volumeKey.value, next)
      book.value = { ...book.value, settings: normaliseSettings(res?.settings || next), is_set_up: true }
    } else {
      await flushText()
      const next = switchPageType(settings.value, type, { width_mm, height_mm }, book.value?.defaults)
      const label = t('Before switching from {from} to {to}', { from: typeLabel(settings.value), to: typeLabel(next) })
      const res = await booksApi.save(volumeKey.value, next, label)
      book.value = { ...book.value, settings: normaliseSettings(res?.settings || next), is_set_up: true }
    }
    typeDialog.value = ''
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

// ---- Upload .docx ----------------------------------------------------------------------------------
const docxInput = ref(null)
const importing = ref(false)
const importReport = ref(null)
const pickDocx = () => docxInput.value?.click()
function importInstead() {
  typeDialog.value = ''
  pickDocx()
}

async function onDocx(e) {
  // Read the file before clearing the input (see CLAUDE.md).
  const file = e.target.files[0]
  e.target.value = ''
  if (!file) return
  importing.value = true
  error.value = ''
  try {
    await flushText()
    const report = await booksApi.importDocx(volumeKey.value, file)
    loadChapters(true)
    const res = await booksApi.get(volumeKey.value)
    book.value = { ...book.value, chapters: res.chapters || [], words: res.words }
    loadFonts(report.fonts || [])
    importReport.value = { ...report, chapters: report.chapters || [] }
    if (report.chapters?.length) goChapter(report.chapters[0].id)
  } catch (err) {
    error.value = err.message
  } finally {
    importing.value = false
  }
}

// Takes the file's page type and page, and/or its styles (a style of the
// same id is replaced, the book's others stay).
async function applyImport({ page, styles: takeStyles }) {
  const r = importReport.value
  const old = settings.value
  let next = { ...old }
  if (page && r.page_type_guess) {
    next.page_type = r.page_type_guess
    if (r.page && r.page_type_guess !== 'online') next.page = { ...old.page, ...r.page }
  }
  if (takeStyles && r.styles?.length) {
    const byId = new Map(old.styles.map((s) => [s.id, s]))
    for (const s of r.styles) byId.set(s.id, s)
    next.styles = [...byId.values()]
  }
  next = normaliseSettings(next)
  const switching = book.value?.is_set_up !== false && next.page_type !== old.page_type
  const label = switching ? t('Before switching from {from} to {to}', { from: typeLabel(old), to: typeLabel(next) }) : ''
  busy.value = true
  try {
    const res = await booksApi.save(volumeKey.value, next, label)
    book.value = { ...book.value, settings: normaliseSettings(res?.settings || next), is_set_up: true }
    typeDialog.value = ''
    importReport.value = null
  } catch (err) {
    error.value = err.message
  } finally {
    busy.value = false
  }
}

async function addChapter() {
  adding.value = true
  try {
    const c = await chaptersApi.create({ title: t('New chapter'), volume_id: volumeKey.value === '0' ? null : +volumeKey.value })
    loadChapters(true)
    const res = await booksApi.get(volumeKey.value)
    book.value = { ...book.value, chapters: res.chapters || [] }
    goChapter(c.id)
  } catch (e) {
    error.value = e.message
  } finally {
    adding.value = false
  }
}

// ---- Find, panels, keys ------------------------------------------------------------------------
const find = ref({ open: false, replace: false, initial: '' })
const findPanel = ref(null)
function openFind(replace) {
  if (!view) return
  const { from, to, empty } = view.state.selection
  const picked = empty ? '' : view.state.doc.textBetween(from, to, ' ')
  const initial = picked && picked.length < 80 && !picked.includes('\n') ? picked : find.value.initial
  if (find.value.open) {
    findPanel.value?.focus(replace ? 'replace' : 'query')
    return
  }
  find.value = { open: true, replace, initial }
}
function closeFind() {
  find.value = { ...find.value, open: false }
  nextTick(() => view?.focus())
}
function togglePanel(name) {
  panel.value = panel.value === name ? '' : name
}

function onKey(e) {
  const mod = (e.ctrlKey || e.metaKey) && !e.altKey
  const key = e.key.toLowerCase()
  if (mod && key === 's') {
    e.preventDefault()
    flushText()
    if (settingsTimer) saveSettings()
  } else if (mod && (key === 'f' || key === 'h') && !e.shiftKey) {
    e.preventDefault()
    openFind(key === 'h')
  } else if (e.key === 'Escape' && !document.querySelector('.overlay')) {
    if (find.value.open) closeFind()
    else if (focusMode.value) focusMode.value = false
  }
}

// Leaving: send what was typed first; if that can't be done, it stays in
// this browser and is offered back next time.
onBeforeRouteLeave(async () => {
  if (settingsTimer) await saveSettings()
  if (!saver || !saver.dirty) return true
  if (await flushText()) return true
  return window.confirm(t("This chapter isn't saved yet. It is kept in this browser and offered back when you open it again. Leave anyway?"))
})
const onUnload = (e) => {
  if (saver?.dirty) {
    saver.flush()
    e.preventDefault()
  }
}

watch(() => props.volume, async () => {
  book.value = null
  panel.value = ''
  await closeChapter()
  chapterId.value = null
  loadBook()
})
watch(() => route.query.chapter, () => pickChapter())

onMounted(() => {
  document.head.appendChild(styleEl)
  createView()
  loadChapters()
  loadBook()
  window.addEventListener('keydown', onKey)
  window.addEventListener('beforeunload', onUnload)
  resizer = new ResizeObserver(([entry]) => (areaWidth.value = entry.contentRect.width))
  if (paperArea.value) resizer.observe(paperArea.value)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKey)
  window.removeEventListener('beforeunload', onUnload)
  resizer?.disconnect()
  saver?.dispose()
  saver = null
  view?.destroy()
  view = null
  styleEl.remove()
  clearTimeout(countTimer)
  clearTimeout(fontTimer)
  pageTitle.value = ''
})

</script>

<style scoped>
.write-page {
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
  height: calc(100vh - 3rem);
  min-height: 30rem;
}

.write-page.is-focus {
  position: fixed;
  inset: 0;
  z-index: 70;
  height: auto;
  padding: 0.75rem 1rem;
  background: var(--bg);
}

.write-head :deep(.page-trail) {
  margin: 0;
}

.notes-link {
  font-size: 0.85rem;
  color: var(--text-muted);
  text-decoration: none;
}

.notes-link:hover {
  color: var(--accent);
}

.error-banner {
  margin: 0;
}

.link-btn {
  margin-left: 0.5rem;
  background: none;
  border: none;
  color: inherit;
  text-decoration: underline;
  cursor: pointer;
}

.focus-bar {
  display: flex;
  justify-content: flex-end;
  opacity: 0.35;
  transition: opacity 0.2s;
}

.focus-bar:hover,
.focus-bar:focus-within {
  opacity: 1;
}

.write-body {
  flex: 1;
  min-height: 0;
  display: grid;
  grid-template-columns: 14rem minmax(0, 1fr);
  gap: 0.55rem;
}

.write-body.has-side {
  grid-template-columns: 14rem minmax(0, 1fr) 21rem;
}

.is-focus .write-body {
  grid-template-columns: minmax(0, 1fr);
}

/* The chapters of the book. */
.chapter-nav {
  min-height: 0;
  overflow-y: auto;
  padding: 0.7rem 0.5rem;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
}

.book-title {
  margin: 0 0.4rem 0.3rem;
  font-family: 'Fraunces', serif;
  font-size: 1.02rem;
  color: var(--text-primary);
}

.chapter-list {
  list-style: none;
  margin: 0;
  padding: 0;
}

.chapter-item {
  display: grid;
  grid-template-columns: 1.4rem minmax(0, 1fr) auto;
  align-items: baseline;
  gap: 0.35rem;
  width: 100%;
  padding: 0.35rem 0.45rem;
  border: none;
  border-radius: 8px;
  background: none;
  color: var(--text-muted);
  text-align: left;
  font: inherit;
  font-size: 0.85rem;
  cursor: pointer;
}

.chapter-item:hover {
  background: var(--glass);
  color: var(--text-primary);
}

.chapter-item.is-here {
  background: var(--accent-soft);
  color: var(--text-primary);
}

.ch-num {
  color: var(--accent);
  font-family: 'Fraunces', serif;
  text-align: right;
}

.ch-title {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ch-words {
  font-size: 0.7rem;
  color: var(--text-faint);
}

.add-chapter {
  margin-top: 0.3rem;
  align-self: flex-start;
}

.empty-actions {
  display: flex;
  justify-content: center;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.order-link {
  margin: 0.2rem 0.45rem 0;
  font-size: 0.74rem;
  color: var(--text-faint);
}

.chapter-select {
  display: none;
}

/* The desk the page lies on. */
.paper-area {
  position: relative;
  min-height: 0;
  overflow: auto;
  border-radius: 14px;
  background: color-mix(in srgb, var(--surface) 55%, transparent);
  border: 1px solid var(--glass-border);
  padding: 1.5rem 0.75rem 3rem;
}

.sheet {
  box-sizing: content-box;
  margin: 0 auto;
  background: var(--paper);
  color: var(--ink);
  box-shadow: var(--paper-shadow);
  border-radius: 2px;
}

.sheet.is-online {
  border-radius: 10px;
}

.write-host {
  min-height: 100%;
}

.write-host :deep(.write-doc) {
  min-height: 6em;
}

.offer {
  max-width: 40rem;
  margin: 0 auto 1rem;
  padding: 0.8rem 1rem;
  border-color: color-mix(in srgb, var(--accent) 55%, transparent);
}

.offer p {
  margin: 0 0 0.6rem;
  font-size: 0.9rem;
}

.offer-actions {
  display: flex;
  gap: 0.4rem;
  flex-wrap: wrap;
}

.empty-state {
  max-width: 30rem;
  margin: 2rem auto;
  padding: 1.5rem;
  text-align: center;
}

/* Words and the save state. */
.write-status {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.3rem 1rem;
  padding: 0.1rem 0.4rem;
  font-size: 0.8rem;
  color: var(--text-muted);
}

.write-status strong {
  color: var(--text-primary);
}

.write-status.is-quiet {
  opacity: 0.55;
}

.save-state.is-saved {
  color: var(--text-faint);
}

.save-state.is-local,
.save-state.is-conflict {
  color: var(--accent);
}

.spacer {
  flex: 1;
}

.zoom select {
  min-height: 1.9rem;
  height: 1.9rem;
  padding: 0.1rem 1.6rem 0.1rem 0.5rem;
  font-size: 0.78rem;
  width: auto;
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
}

@media (max-width: 1100px) {
  .write-body,
  .write-body.has-side {
    grid-template-columns: 12rem minmax(0, 1fr);
  }
  .write-body.has-side > .side-panel {
    position: fixed;
    top: 1.5rem;
    right: 1.5rem;
    bottom: 1.5rem;
    width: min(21rem, calc(100vw - 3rem));
    z-index: 65;
  }
}

/* Phones: the page scrolls as a whole, chapters are a list box, the
   toolbar stays at the top. */
@media (max-width: 720px) {
  .write-page {
    height: auto;
    min-height: calc(100vh - 6rem);
  }
  .write-tools {
    position: sticky;
    top: 4.1rem;
    z-index: 30;
  }
  .write-body,
  .write-body.has-side {
    display: flex;
    flex-direction: column;
  }
  .chapter-nav {
    display: none;
  }
  .chapter-select {
    display: block;
  }
  .paper-area {
    overflow: visible;
    padding: 0.75rem 0 2rem;
    border: none;
    background: none;
  }
}
</style>
