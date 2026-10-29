<template>
  <div class="write-page" :class="{ 'is-focus': focusMode, 'is-dark-paper': darkPaper, 'hide-tags': !showTags }" :style="pageVars">
    <div v-if="!focusMode" class="write-head">
      <Breadcrumbs :crumbs="crumbs">
        <RouterLink v-if="chapterId" :to="`/chapters/${chapterId}`" class="notes-link">{{ $t('Chapter notes') }}</RouterLink>
        <RouterLink v-if="book && chapters.length" :to="`/write/${volumeKey}/pages`" class="notes-link pages-link">{{ online ? $t('Reading view') : $t('Page view') }}</RouterLink>
      </Breadcrumbs>
    </div>

    <p v-if="error" class="error-banner">
      {{ error }}
      <button type="button" class="link-btn" @click="error = ''">{{ $t('Dismiss') }}</button>
    </p>
    <p v-if="notice" class="write-notice" role="status">
      {{ notice }}
      <button type="button" class="link-btn" @click="notice = ''">{{ $t('Dismiss') }}</button>
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
      :uploading="uploading"
      :can-read="reader.supported"
      :reading="readInfo.status !== 'idle'"
      @read="reader.toggle()"
      :spell-on="spell.enabled.value"
      :show-tags="showTags"
      @tags="showTags = !showTags"
      @spell="spell.enabled.value = !spell.enabled.value"
      :painter="!!painter"
      @painter="togglePainter"
      @picture="pickPicture"
      @find="openFind(false)"
      @comment="newComment"
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
          <li v-for="c in chapters" :key="c.id" class="chapter-row">
            <button type="button" class="chapter-item" :class="{ 'is-here': c.id === chapterId }" :aria-current="c.id === chapterId ? 'true' : undefined" @click="goChapter(c.id)">
              <span class="ch-num" :title="isFrontMatter(c.id) ? $t('Front matter') : undefined">{{ isFrontMatter(c.id) ? '·' : chapterNumbers.get(c.id) }}</span>
              <span class="ch-title">{{ c.title }}</span>
              <span class="ch-words">{{ formatNum(c.id === chapterId ? words : c.words || 0) }}</span>
            </button>
            <RowMenu v-if="!online" class="ch-menu" :label="$t('Chapter layout')" :items="chapterMenu(c)" />
          </li>
        </ol>
        <button type="button" class="btn btn-ghost small add-chapter" :disabled="adding" @click="addChapter">{{ $t('Add a chapter') }}</button>
        <button type="button" class="btn btn-ghost small add-chapter" :disabled="importing" :title="$t('Its chapters are added at the end of this book, split at each Heading 1')" @click="pickDocx">
          {{ importing ? $t('Importing…') : $t('Upload .docx') }}
        </button>
        <RouterLink to="/chapters" class="order-link">{{ $t('Reorder in Chapters') }}</RouterLink>
      </nav>
      <select v-if="!focusMode && chapters.length" class="chapter-select" :value="chapterId ?? ''" :aria-label="$t('Chapter')" @change="goChapter(+$event.target.value)">
        <option v-for="c in chapters" :key="c.id" :value="c.id">{{ isFrontMatter(c.id) ? "·" : chapterNumbers.get(c.id) }}. {{ c.title }}</option>
      </select>

      <div ref="paperArea" class="paper-area" @scroll.passive="placePictureOptions(); placeSpeaker(); placeTableOptions()">
        <ReadAloudBar
          v-if="readInfo.status !== 'idle'"
          :info="readInfo"
          :language="book?.settings?.meta?.language || 'en'"
          @toggle="reader.toggle()"
          @stop="reader.stop(); view?.focus()"
          @step="(by) => reader.step(by)"
          @rate="(r) => reader.setRate(r)"
          @voice="(v) => reader.setVoice(v)"
        />
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
        <PictureOptions
          v-if="pictureNode && !focusMode && !readOnly"
          ref="pictureBox"
          :attrs="pictureNode.attrs"
          :top="picturePlace.top"
          :left="picturePlace.left"
          @change="(patch) => exec(setImageAttrs(patch))"
          @remove="exec(removeImage)"
        />
        <TableOptions v-if="tableBox && !focusMode && !readOnly" ref="tableBoxEl" :info="tableBox" :exec="exec" :top="tablePlace.top" :left="tablePlace.left" />
        <SpeakerOptions
          v-if="speaker && speakerNode && !focusMode && !readOnly"
          ref="speakerBox"
          :attrs="speakerNode.attrs"
          :characters="names.characters.value"
          :resolve="names.resolve"
          :top="speaker.top"
          :left="speaker.left"
          @change="(patch) => exec(setDialogueAttrs(speaker.pos, patch))"
          @unwrap="unwrapSpeaker"
          @close="closeSpeaker"
        />
      </div>

      <StylesPanel v-if="panel === 'styles' && !focusMode && settings" :styles="styles" :current-id="format?.styleId || null" :current-char-id="format?.charStyleId || null" @update="setStyles" @apply="applyStyle" @close="panel = ''" />
      <VersionsPanel v-if="panel === 'versions' && !focusMode && chapterId" :chapter-id="chapterId" :online="online" :flush="flushText" @restored="onRestored" @close="panel = ''" />
      <CommentsPanel
        v-if="panel === 'comments' && !focusMode && chapterId"
        :threads="commentModel.threads"
        :detached="commentModel.detached"
        :resolved-count="commentModel.resolved"
        :show-resolved="showResolved"
        :active-id="activeComment"
        :edit-id="editComment"
        :loading="commentsLoading"
        :busy="commentBusy"
        :read-only="readOnly"
        :error="commentError"
        @new="newComment"
        @select="selectComment"
        @save="saveComment"
        @cancel-new="dropComment"
        @reply="replyTo"
        @resolve="resolveComment"
        @remove="dropComment"
        @show-resolved="(v) => (showResolved = v)"
        @close="panel = ''"
      />
      <BookPanel
        v-if="panel === 'book' && !focusMode && settings"
        :settings="settings"
        :chapters="chapters"
        :book-title="bookTitle"
        :focus-chapter="panelChapter"
        :show-pages="showPages"
        @show-pages="(v) => (showPages = v)"
        @update="updateSettings"
        @change-type="typeDialog = 'switch'"
        @close="panel = ''"
      />
      <SpellPanel v-if="panel === 'spelling' && !focusMode" :spell="spell" @language="setBookLanguage" @close="panel = ''" />
      <OfflinePanel v-if="panel === 'offline' && !focusMode" :offline="offline" :chapters="chapters" :volume="volumeKey" @toggle="toggleOffline" @open="goChapter" @sync="sendKept" @close="panel = ''" />
    </div>

    <footer class="write-status" :class="{ 'is-quiet': focusMode }">
      <span class="stat"><strong>{{ formatNum(words) }}</strong> {{ $tn('word in this chapter', 'words in this chapter', words) }}</span>
      <span v-if="!focusMode" class="stat">{{ $tn('{n} word in the book', '{n} words in the book', bookWords, { n: formatNum(bookWords) }) }}</span>
      <span v-if="selWords" class="stat">{{ $tn('{n} selected', '{n} selected', selWords, { n: formatNum(selWords) }) }}</span>
      <span v-if="uploading" class="stat">{{ $t('Uploading the picture…') }}</span>
      <span class="save-state" :class="`is-${saveShown}`" :title="saveShown === 'device' ? $t('Sent to the server when it can be reached again.') : saveLabel" aria-live="polite">{{ saveLabel }}</span>
      <span class="spacer"></span>
      <button v-if="!focusMode" type="button" class="btn btn-ghost small spell-status" :class="`is-${spell.status.value}`" :aria-pressed="panel === 'spelling'" :title="$t('Spelling options')" @click="togglePanel('spelling')">{{ spell.statusLabel.value }}</button>
      <button v-if="!focusMode && settings" type="button" class="btn btn-ghost small" :aria-pressed="panel === 'book'" :title="$t('Book and page')" @click="togglePanel('book')">{{ $t('Page: {type}', { type: typeLabel(settings) }) }}</button>
      <button v-if="!focusMode" type="button" class="btn btn-ghost small offline-status" :class="{ 'is-on': offline.available.value, 'is-away': !offline.connected.value }" :aria-pressed="panel === 'offline'" :title="$t('Offline writing')" @click="togglePanel('offline')">{{ offlineLabel }}</button>
      <label v-if="!focusMode" class="zoom">
        <span class="sr-only">{{ $t('Zoom') }}</span>
        <select v-model="zoomChoice" :aria-label="$t('Zoom')">
          <option value="auto">{{ $t('Fit ({n}%)', { n: Math.round(autoZoom * 100) }) }}</option>
          <option v-for="z in ZOOMS" :key="z" :value="String(z)">{{ Math.round(z * 100) }}%</option>
        </select>
      </label>
      <button type="button" class="btn btn-ghost small" :aria-pressed="darkPaper" @click="darkPaper = !darkPaper">{{ darkPaper ? $t('Light paper') : $t('Dark paper') }}</button>
    </footer>

    <input ref="pictureInput" type="file" accept="image/png,image/jpeg,image/gif,image/webp" hidden @change="onPicture" />
    <input ref="docxInput" type="file" accept=".docx,application/vnd.openxmlformats-officedocument.wordprocessingml.document" hidden @change="onDocx" />
    <ImportReport v-if="importReport && settings" :report="importReport" :settings="settings" :set-up="book?.is_set_up !== false" :busy="busy" @apply="applyImport" @close="importReport = null" />
    <datalist id="write-fonts"><option v-for="f in fontList" :key="f" :value="f"></option></datalist>

    <SpeakerTag
      v-if="speakerTag"
      :key="speakerTag.n"
      :x="speakerTag.x"
      :y="speakerTag.y"
      :characters="names.characters.value"
      :spells="spellChoices(spellList)"
      :has-tag="!!speakerTag.tag"
      :has-spell="!!speakerTag.spell"
      :direct="!!speakerTag.direct"
      @pick="pickSpeakerTag"
      @pick-spell="pickSpellTag"
      @remove="removeSpeakerTag"
      @remove-spell="removeSpellTag"
      @close="closeSpeakerTag"
    />
    <TagCard :enabled="showTags" :load-spells="loadSpells" />
    <SpellMenu
      v-if="spellMenu"
      :word="spellMenu.word"
      :x="spellMenu.x"
      :y="spellMenu.y"
      :suggestions="spellMenu.suggestions"
      @pick="pickSuggestion"
      @add="addToDictionary"
      @ignore="ignoreWord"
      @options="spellOptions"
      @close="closeSpellMenu"
    />
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
import { copyFormat, applyFormat } from '../editor/formatPainter.js'
import { ref, shallowRef, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRoute, useRouter, onBeforeRouteLeave } from 'vue-router'
import { EditorState, NodeSelection, TextSelection } from 'prosemirror-state'
import { EditorView } from 'prosemirror-view'
import { t, tr, locale } from '../i18n'
import { chaptersApi, spellsApi } from '../api'
import { activeStoryId } from '../stories'
import { chapterIndex, loadChapters } from '../chapters'
import { pageTitle } from '../navigation'
import { schema, docFromJSON, docToJSON, emptyDocJSON } from '../editor/schema.js'
import { editorPlugins } from '../editor/keymap.js'
import { findPlugin } from '../editor/findPlugin.js'
import { readAloudPlugin, createReader } from '../editor/readAloudPlugin.js'
import { selectionFormat, setStyle, setCharStyle, pasteIntoEmpty, insertBlock, setImageAttrs, removeImage } from '../editor/commands.js'
import { styleSheet, styleFonts, resolveStyle, styleById } from '../editor/styles.js'
import { normaliseSettings, defaultSettings, switchPageType, textBlockMm, pageTypeName } from '../editor/pageTypes.js'
import { countNode, countDocJSON } from '../editor/wordCount.js'
import { loadFonts, docFonts, fontChoices } from '../editor/fonts.js'
import { createSaver } from '../editor/autosave.js'
import { keepCopy, readCopy, dropCopy } from '../editor/localCopy.js'
import { booksApi, textApi } from '../editor/api.js'
import { ago } from '../editor/when.js'
import Breadcrumbs from '../components/Breadcrumbs.vue'
import WriteToolbar from '../components/write/WriteToolbar.vue'
import FindPanel from '../components/write/FindPanel.vue'
import ReadAloudBar from '../components/write/ReadAloudBar.vue'
import StylesPanel from '../components/write/StylesPanel.vue'
import PictureOptions from '../components/write/PictureOptions.vue'
import VersionsPanel from '../components/write/VersionsPanel.vue'
import BookPanel from '../components/write/BookPanel.vue'
import PageTypeDialog from '../components/write/PageTypeDialog.vue'
import ConflictView from '../components/write/ConflictView.vue'
import ImportReport from '../components/write/ImportReport.vue'
import RowMenu from '../components/RowMenu.vue'
import { tocNodeView } from '../editor/tocView.js'
import { chapterOption, setChapterOption, normaliseLayout, bookPlan, bandText, formatNumber as formatPageNumber } from '../editor/bookLayout.js'
import { pagesPlugin, refreshPages } from '../editor/pagination.js'
import SpeakerOptions from '../components/write/SpeakerOptions.vue'
import TableOptions from '../components/write/TableOptions.vue'
import { tablePlugins, tableInfo } from '../editor/tables.js'
import { notesPlugin } from '../editor/footnotes.js'
import { noteNodeView } from '../editor/noteView.js'
import { nameSuggest } from '../editor/nameSuggest.js'
import { scriptPlugins, setDialogueAttrs, dialogueToParagraph } from '../editor/script.js'
import { useStoryNames } from '../editor/useStoryNames.js'
import CommentsPanel from '../components/write/CommentsPanel.vue'
import { commentsApi } from '../editor/api.js'
import { commentRanges, panelThreads, shownIds, activeAt, highlightCSS, wordRange, hasText } from '../editor/comments.js'
import { commentPlugin, startPending, takePending, addCommentMark, removeCommentMark } from '../editor/commentPlugin.js'
import { spellPlugin, refreshSpell, replaceWord } from '../editor/spellPlugin.js'
import { useSpellcheck } from '../editor/useSpellcheck.js'
import { DICTIONARY_LANG } from '../editor/spellWords.js'
import SpellMenu from '../components/write/SpellMenu.vue'
import SpeakerTag from '../components/write/SpeakerTag.vue'
import TagCard from '../components/write/TagCard.vue'
import { speakerPlugin, tagSpeaker, removeSpeaker, tagSpell, removeSpell } from '../editor/speaker.js'
import { spellChoices } from '../editor/spellTag.js'
import SpellPanel from '../components/write/SpellPanel.vue'
import OfflinePanel from '../components/write/OfflinePanel.vue'
import { useOfflineBook } from '../editor/useOfflineBook.js'
import { mergeDocs } from '../offline/merge.js'
import { whenBack } from '../offline/connection.js'
import { currentStory } from '../stories'
import { wikiApi } from '../wiki'
import { spellingApi } from '../editor/api.js'

void [tr('Custom size'), tr('Online Post'), tr('Not kept after a conflict'), tr('Before using the copy kept in this browser')]
const PICTURE_TYPES = ['image/png', 'image/jpeg', 'image/gif', 'image/webp']
const MAX_PICTURE = 32 << 20

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
// ---- Offline writing (stage 5) ---------------------------------------------------------
// A book kept on this device (editor/useOfflineBook.js): its chapters open
// from there with no connection, and a save that can't reach the server
// is kept there and sent when it is back.
const offline = useOfflineBook({
  story: activeStoryId,
  volume: () => volumeKey.value,
  normalize: (d) => normal(d),
  storyInfo: () => (currentStory.value ? { name: currentStory.value.name } : null),
})
const pendingHere = ref(false) // the open chapter's text is on this device only
let serverBase = null // { id, version, doc }: the open chapter's text on the server, as last known
const notice = ref('')
const offlineLabel = computed(() => {
  const n = offline.pendingHere.value.length
  if (!offline.connected.value) return t('No connection')
  if (offline.available.value && n) return t('Available offline: {n} to send', { n })
  return offline.available.value ? t('Available offline') : t('Offline: off')
})

const saveShown = computed(() => (pendingHere.value && saveState.value === 'saved' ? 'device' : saveState.value))
const saveLabel = computed(
  () =>
    ({
      saved: t('Saved'),
      saving: t('Saving…'),
      local: t('Not saved yet: kept in this browser'),
      conflict: t('Changed somewhere else: choose a text'),
      device: t('Saved on this device'),
    })[saveShown.value],
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
const TAGS_KEY = 'polaris-write-tags'
const showTags = ref(stored(TAGS_KEY, 'on') !== 'off')
watch(showTags, (on) => store(TAGS_KEY, on ? 'on' : 'off'))
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
// ---- Page breaks while writing (editor/pagination.js) --------------------------------------
// On by default for a page-based type; a per-browser switch in the Book and page panel.
const PAGES_KEY = 'polaris-write-pages'
const showPages = ref(stored(PAGES_KEY, 'on') !== 'off')
watch(showPages, (on) => store(PAGES_KEY, on ? 'on' : 'off'))
// The header and footer of page k (0-based) of this chapter: [left, centre, right] texts.
// Pages are counted in this chapter (the page view numbers the whole book).
function pageBands(k) {
  const s = settings.value
  const c = chapter.value
  const layout = normaliseLayout(s?.layout)
  const plan = bookPlan(s, chapters.value).find((p) => p.id === c?.id) || { header: true, format: layout.numbering.format, restart: 1, number: '' }
  const n = (plan.restart ?? 1) + k
  const slot = k === 0 && layout.different_first ? 'first' : layout.different_odd_even && n % 2 === 0 ? 'even' : 'odd'
  const values = { page: formatPageNumber(n, plan.format), chapter: c?.title || '', chapter_number: plan.number, book: s?.meta?.title || bookTitle.value, author: s?.meta?.author || '' }
  const row = (b) => (b ? [bandText(b.left, values), bandText(b.center, values), bandText(b.right, values)] : null)
  return { head: plan.header && !(plan.front) ? row(layout.header[slot]) : null, foot: row(layout.footer[slot]) }
}
const pagesOptions = () => {
  const s = settings.value
  const g = geometry.value
  if (!showPages.value || !s || online.value || !g) return null
  const p = s.page || {}
  if (!(+p.height_mm > 0)) return null
  return { widthPx: g.widthPx, heightMm: +p.height_mm, topMm: +p.margin_top_mm || 0, bottomMm: +p.margin_bottom_mm || 0, bands: pageBands }
}
const pagesSig = computed(() => {
  const o = pagesOptions()
  return o ? JSON.stringify([o.widthPx, o.heightMm, o.topMm, o.bottomMm, zoom.value, settings.value.layout, chapter.value?.title, settings.value.chapter_options, settings.value.meta, styles.value]) : 'off'
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

// ---- Comments (stage 5) ------------------------------------------------------------------------
// The rows are the chapter's (GET /api/chapters/{id}/comments); the marks in
// the text say where each is. A new comment's row is made first and its
// mark put in the text after, so the chapter's next save carries it and a
// mark never points at nothing. A comment's text is saved on its own.
const comments = ref([])
const commentsLoading = ref(false)
const commentBusy = ref(false)
const commentError = ref('')
const showResolved = ref(false)
const activeComment = ref(null)
const editComment = ref(null) // a new comment, opened for writing in the panel

// Where each comment is, worked out again only when the text changes.
let rangeCache = { doc: null, ranges: new Map() }
const commentPlaces = computed(() => {
  const doc = editorState.value?.doc
  if (!doc || !comments.value.length) return new Map()
  if (rangeCache.doc !== doc) rangeCache = { doc, ranges: commentRanges(doc) }
  return rangeCache.ranges
})
const commentModel = computed(() => panelThreads(comments.value, commentPlaces.value, { showResolved: showResolved.value }))
const shownComments = computed(() => shownIds(comments.value))

// Highlights: the open comments the chapter knows, the active one stronger.
const commentStyleEl = document.createElement('style')
commentStyleEl.dataset.writeComments = ''
watch([shownComments, activeComment], ([ids, active]) => (commentStyleEl.textContent = highlightCSS(ids, active, '.write-host .write-doc')), { immediate: true })

// The cursor in commented text makes that comment the active one.
watch(editorState, (st) => {
  if (!st || !comments.value.length) return
  const { from, to } = st.selection
  const ids = showResolved.value ? new Set(comments.value.filter((c) => !c.parent_id).map((c) => c.id)) : shownComments.value
  const id = activeAt(commentPlaces.value, from, to, ids)
  if (id != null || st.selection.empty) activeComment.value = id
})

async function loadComments(id) {
  comments.value = []
  activeComment.value = null
  commentError.value = ''
  if (id == null) return
  commentsLoading.value = true
  try {
    const list = await commentsApi.list(id)
    if (chapterId.value === id) comments.value = Array.isArray(list) ? list : []
    if (Array.isArray(list)) offline.comments(id, list)
  } catch (e) {
    // No connection: the comments as the book was kept (read only).
    const kept = e.offline ? await offline.chapter(id) : null
    if (chapterId.value === id) {
      if (Array.isArray(kept?.comments)) comments.value = kept.comments
      else commentError.value = e.message
    }
  } finally {
    commentsLoading.value = false
  }
}

const replaceComment = (c) => (comments.value = comments.value.map((x) => (x.id === c.id ? c : x)))

// A comment on the selection (Ctrl+Alt+M): with nothing selected, on the word
// at the cursor, as in Word.
async function newComment() {
  if (!view || readOnly.value || chapterId.value == null) return
  panel.value = 'comments'
  let { from, to, empty } = view.state.selection
  if (empty) {
    const w = wordRange(view.state.doc, from)
    if (w) ({ from, to } = w)
  }
  if (from === to || !hasText(view.state.doc, from, to)) {
    commentError.value = t('Select the text to comment on first.')
    return
  }
  commentError.value = ''
  const into = chapterId.value
  const key = startPending(view, from, to)
  commentBusy.value = true
  try {
    const c = await commentsApi.create(into, { text: '' })
    const place = view ? takePending(view, key) : null
    if (chapterId.value !== into || !place) {
      // The writer moved on or the text went meanwhile: the row isn't needed.
      commentsApi.remove(c.id).catch(() => {})
      return
    }
    comments.value = [...comments.value, c]
    view.dispatch(addCommentMark(view.state, place.from, place.to, c.id))
    activeComment.value = c.id
    editComment.value = null
    await nextTick()
    editComment.value = c.id
  } catch (e) {
    if (view) takePending(view, key)
    commentError.value = e.message
  } finally {
    commentBusy.value = false
  }
}

// Picking a comment in the panel selects its text and brings it into sight.
function selectComment(id) {
  const r = commentPlaces.value.get(id)
  activeComment.value = id
  if (!view || !r) return
  view.dispatch(view.state.tr.setSelection(TextSelection.create(view.state.doc, r.from, r.to)).scrollIntoView())
  const dom = view.domAtPos(r.from)?.node
  const el = dom?.nodeType === 1 ? dom : dom?.parentElement
  el?.scrollIntoView?.({ block: 'center' })
}

async function commentCall(fn) {
  commentBusy.value = true
  commentError.value = ''
  try {
    return await fn()
  } catch (e) {
    commentError.value = e.message
    return null
  } finally {
    commentBusy.value = false
  }
}

async function saveComment(id, text) {
  const c = await commentCall(() => commentsApi.update(id, { text }))
  if (c) replaceComment(c)
}

async function resolveComment(id, resolved) {
  const c = await commentCall(() => commentsApi.update(id, { resolved }))
  if (c) replaceComment(c)
}

async function replyTo(id, text) {
  const into = chapterId.value
  const c = await commentCall(() => commentsApi.create(into, { text, parent_id: id }))
  if (c && chapterId.value === into) comments.value = [...comments.value, c]
}

// Deletes a comment (and its replies) and takes its mark off the text.
async function dropComment(id) {
  const c = comments.value.find((x) => x.id === id)
  const ok = await commentCall(() => commentsApi.remove(id).then(() => true))
  if (!ok) return
  comments.value = comments.value.filter((x) => x.id !== id && x.parent_id !== id)
  if (activeComment.value === id) activeComment.value = null
  if (c && !c.parent_id && view && !readOnly.value) view.dispatch(removeCommentMark(view.state, id))
}

// ---- The editor ----------------------------------------------------------------------------
// Tab-to-link and Script Mode (stage 4) go first: their Tab, Enter and
// Backspace come before the usual ones.
const names = useStoryNames({ fetchItems: () => offline.cached('names', () => wikiApi.list()) })
// Read aloud (stage 5): typing or clicking in the text stops it.
const readInfo = ref({ status: 'idle', index: 0, total: 0, rate: 1, voice: '', voices: [] })
const reader = createReader({
  getView: () => view,
  getLang: () => book.value?.settings?.meta?.language || 'en',
  onChange: (s) => (readInfo.value = s),
})
// The spellchecker (stage 5): the book's language picks the dictionary.
const spell = useSpellcheck({ getLanguage: () => settings.value?.meta?.language, names, fetchWords: () => offline.cached('spelling', () => spellingApi.words()) })
const plugins = [
  nameSuggest({ getIndex: () => names.index.value, typeLabel: names.typeLabel, hint: t('Tab: link this name') }),
  ...scriptPlugins({ getIndex: () => names.index.value, onSpeaker: openSpeaker }),
  ...tablePlugins(),
  notesPlugin(),
  ...editorPlugins({ getStyles: () => styles.value }),
  findPlugin(),
  readAloudPlugin({ onInterrupt: () => reader.stop() }),
  commentPlugin({ known: () => new Set(comments.value.filter((c) => !c.parent_id).map((c) => c.id)) }),
  speakerPlugin({ onMenu: openSpeakerTag }),
  spellPlugin({ getSpell: () => spell.forPlugin(), onMenu: openSpellMenu }),
  pagesPlugin({ getOptions: pagesOptions, getStyles: () => styles.value }),
]
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
  reader.stop()
  const st = makeState(json)
  view.updateState(st)
  editorState.value = st
  recount()
  countSelection()
  refreshDocFonts()
}

// ---- Format Painter (editor/formatPainter.js) ----------------------------------------------
const painter = ref(null) // { fmt, locked } while armed
function togglePainter(how) {
  if (!view || readOnly.value) return
  if (how === 'lock') {
    const fmt = painter.value?.fmt || copyFormat(view.state)
    painter.value = fmt ? { fmt, locked: true } : null
  } else if (painter.value) painter.value = null
  else {
    const fmt = copyFormat(view.state)
    painter.value = fmt ? { fmt, locked: false } : null
  }
  view.setProps({})
  view.focus()
}
function paintNow() {
  if (!view || !painter.value) return
  const tr = applyFormat(view.state, painter.value.fmt)
  if (tr) view.dispatch(tr)
  if (!painter.value.locked) painter.value = null
  view.setProps({})
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
    nodeViews: nodeViews(),
    editable: () => !readOnly.value,
    attributes: () => ({ class: `write-doc${online.value ? ' is-online' : ''}${painter.value ? ' is-painting' : ''}`, spellcheck: spellOwn.value ? 'false' : 'true' }),
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
      // A picture on the clipboard (a screenshot, "copy image"): uploaded. Text
      // pasted with a picture beside it (some apps add one) stays text.
      const pics = [...(e.clipboardData?.files || [])].filter((f) => PICTURE_TYPES.includes(f.type))
      if (pics.length && !(e.clipboardData.getData('text/plain') || '').trim()) {
        e.preventDefault()
        addPictures(pics)
        return true
      }
      const tr = pasteIntoEmpty(v.state, slice)
      if (!tr) return false
      v.dispatch(tr)
      return true
    },
    // A picture file dropped on the page goes in where it was dropped.
    handleDrop(v, e, slice, moved) {
      const files = [...(e.dataTransfer?.files || [])]
      if (moved || !files.length) return false
      e.preventDefault()
      const pics = files.filter((f) => PICTURE_TYPES.includes(f.type))
      if (!pics.length) {
        error.value = t('this is not a picture the editor can use (PNG, JPEG, GIF or WebP)')
        return true
      }
      const at = v.posAtCoords({ left: e.clientX, top: e.clientY })
      if (at && !readOnly.value) v.dispatch(v.state.tr.setSelection(TextSelection.near(v.state.doc.resolve(at.pos))))
      addPictures(pics)
      return true
    },
    handleDOMEvents: {
      // Format Painter: paint what the mouse just selected (or clicked).
      mouseup() {
        if (painter.value) setTimeout(paintNow, 0)
        return false
      },
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
watch(pagesSig, () => nextTick(() => refreshPages(view)), { flush: 'post' })

// ---- Spelling ------------------------------------------------------------------------------
// While our checker is on (and has a dictionary), the browser's is off.
const spellOwn = computed(() => spell.enabled.value && !!spell.language.value.code)
watch(spellOwn, () => view?.setProps({}))
watch([spell.gen, spell.status], () => refreshSpell(view, !!spell.forPlugin()))
// "Tag character": the right-click menu / Ctrl+Alt+K picker.
// "Tag spell" shares the menu. The story's spells are fetched once per page
// (the picker and the hover card read the same list).
const spellList = shallowRef([])
let spellsLoad = null
function loadSpells() {
  spellsLoad ||= Promise.resolve()
    .then(() => offline.cached('spells', () => spellsApi.list()))
    .then((list) => (spellList.value = Array.isArray(list) ? list : []))
    .catch(() => {
      spellsLoad = null // try again next time
      return spellList.value
    })
  return spellsLoad
}
const speakerTag = ref(null) // { x, y, range, tag, spell, direct, n }
let speakerTagN = 0
function openSpeakerTag(info) {
  if (readOnly.value) return
  loadSpells()
  speakerTag.value = { ...info, n: ++speakerTagN }
}
function pickSpellTag(spell) {
  const s = speakerTag.value
  speakerTag.value = null
  const range = s?.range || (s?.spell && { from: s.spell.from, to: s.spell.to })
  if (view && range) exec(tagSpell(spell, range))
}
function removeSpellTag() {
  const s = speakerTag.value
  speakerTag.value = null
  if (s?.spell && view) exec(removeSpell({ from: s.spell.from, to: s.spell.to }))
}
function closeSpeakerTag() {
  speakerTag.value = null
  view?.focus()
}
function pickSpeakerTag(who) {
  const s = speakerTag.value
  speakerTag.value = null
  const range = s?.range || (s?.tag && { from: s.tag.from, to: s.tag.to })
  if (view && range) exec(tagSpeaker(who, range))
}
function removeSpeakerTag() {
  const s = speakerTag.value
  speakerTag.value = null
  if (s?.tag && view) exec(removeSpeaker({ from: s.tag.from, to: s.tag.to }))
}
const spellMenu = ref(null) // { word, from, to, x, y, suggestions }
async function openSpellMenu(hit) {
  if (readOnly.value) return
  spellMenu.value = { ...hit, suggestions: null }
  let list = []
  try {
    list = await spell.suggest(hit.word)
  } catch (e) {
    /* no suggestions */
  }
  if (spellMenu.value?.from === hit.from && spellMenu.value?.word === hit.word) spellMenu.value = { ...spellMenu.value, suggestions: list }
}
function closeSpellMenu() {
  spellMenu.value = null
  view?.focus()
}
function pickSuggestion(text) {
  const m = spellMenu.value
  spellMenu.value = null
  if (m && view && !readOnly.value) replaceWord(view, m, text)
}
function addToDictionary() {
  const m = spellMenu.value
  closeSpellMenu()
  if (m) spell.addWord(m.word)
}
function ignoreWord() {
  const m = spellMenu.value
  closeSpellMenu()
  if (m) spell.ignore(m.word)
}
function spellOptions() {
  spellMenu.value = null
  panel.value = 'spelling'
}
// Picking a dictionary sets the book's language (also the EPUB's).
function setBookLanguage(code) {
  const tag = DICTIONARY_LANG[code]
  if (!tag || !settings.value) return
  updateSettings({ ...settings.value, meta: { ...(settings.value.meta || {}), language: tag } })
}

// ---- Loading ------------------------------------------------------------------------------------
const normal = (json) => docToJSON(docFromJSON(json))
const sameDoc = (a, b) => JSON.stringify(normal(a)) === JSON.stringify(normal(b))
const wait = (ms) => new Promise((r) => setTimeout(r, ms))

async function loadBook() {
  error.value = ''
  const want = volumeKey.value
  // What was written on this device without a connection goes first (a
  // book's settings too).
  if ((await offline.load()) || offline.dirty.value.length) await offline.sync()
  if (want !== volumeKey.value) return
  try {
    const res = await booksApi.get(want)
    book.value = { ...res, settings: normaliseSettings(res.settings), chapters: res.chapters || [] }
    if (res.is_set_up === false) typeDialog.value = 'first'
    pickChapter()
    // The kept copy brought up to date (what changed only), now and then.
    const kept = offline.record.value
    if (kept && Date.now() - (kept.saved_at || 0) > 60000) offline.refresh({ quiet: true })
  } catch (e) {
    // No connection: the book as it was kept on this device.
    const kept = e.offline ? await offline.load() : null
    if (!kept || want !== volumeKey.value) {
      error.value = e.message
      return
    }
    book.value = { ...kept.book, settings: normaliseSettings(kept.book.settings), chapters: kept.book.chapters || [] }
    offline.adoptLibrary()
    pickChapter()
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
  reader.stop()
  loadingText.value = true
  offer.value = null
  conflict.value = null
  pendingHere.value = false
  serverBase = null
  if (find.value.open) closeFind()
  try {
    await closeChapter()
    if (token !== opening) return
    chapterId.value = id
    loadComments(id)
    if (id == null) {
      setDoc(emptyDocJSON())
      return
    }
    await offline.idle()
    const kept = await offline.chapter(id) // this chapter's record, when the book is kept here
    if (token !== opening) return
    let server = null
    let local = null
    try {
      server = await textApi.get(id)
    } catch (e) {
      if (kept && e.offline) {
        // No connection: the text kept on this device, sent when the server is back.
        setDoc(normal(kept.doc))
        serverBase = { id, version: kept.version || 0, doc: normal(kept.dirty ? kept.base_doc ?? kept.doc : kept.doc) }
        startSaver(id, kept.version || 0)
        pendingHere.value = !!kept.dirty
        return
      }
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
    serverBase = { id, version: server.version || 0, doc: serverDoc }
    startSaver(id, server.version || 0)
    if (kept?.dirty) return await takeKept(id, kept, server, serverDoc)
    if (kept) offline.saved(id, serverDoc, server) // the kept copy follows the server
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
      nextTick(() => {
        goToBlock()
        view?.focus()
      })
    }
  }
}

function startSaver(id, version) {
  saver?.dispose()
  saveState.value = 'saved'
  const baseOf = () => (serverBase?.id === id ? serverBase : null)
  saver = createSaver({
    getDoc: () => docToJSON(view.state.doc),
    save: async (doc, base) => {
      try {
        const res = await textApi.save(id, doc, base)
        offline.saved(id, doc, res)
        return res
      } catch (e) {
        // A kept book with no connection: kept on this device, sent later.
        if (e.offline && (await offline.edited(id, doc, baseOf()))) return { version: base, offline: true, words: countDocJSON(doc) }
        throw e
      }
    },
    // A kept book's chapters are kept in its record; others as a draft.
    keep: async (entry) => (offline.available.value && (await offline.edited(id, entry.doc, baseOf()))) || keepCopy(activeStoryId, id, entry),
    drop: () => dropCopy(activeStoryId, id),
    base: version,
    onState: (s) => {
      if (chapterId.value === id) saveState.value = s
    },
    onSaved: (res, doc) => {
      if (chapterId.value === id) {
        pendingHere.value = !!res?.offline
        if (res && !res.offline) serverBase = { id, version: res.version ?? 0, doc }
      }
      const c = book.value?.chapters.find((x) => x.id === id)
      if (c && res) {
        if (res.words != null) c.words = res.words
        if (res.version != null) c.version = res.version
      }
    },
    onConflict: async (stored, mine) => {
      if (chapterId.value !== id) return
      const theirs = normal(stored?.doc)
      if (await mergeConflict(id, mine, theirs, stored?.version ?? 0)) return
      if (chapterId.value !== id) return
      conflict.value = {
        kind: 'save',
        mine,
        theirs,
        version: stored?.version ?? 0,
        theirsWhen: stored?.updated_at ? ago(stored.updated_at) : '',
      }
    },
  })
}

// ---- Merging (stage 5, with offline writing) ----------------------------------------------------
const MERGE_LABEL = () => t('Before merging with changes made elsewhere')

// Puts a doc in the editor as a change of the text (not a new state), so
// the cursor stays where it was and undo goes on; the change itself isn't
// undone by Ctrl+Z.
function replaceDoc(json) {
  if (!view) return
  const next = docFromJSON(json)
  const cur = view.state.doc
  const tr = view.state.tr
  const start = cur.content.findDiffStart(next.content)
  if (start != null) {
    let { a: endA, b: endB } = cur.content.findDiffEnd(next.content)
    const overlap = start - Math.min(endA, endB)
    if (overlap > 0) {
      endA += overlap
      endB += overlap
    }
    tr.replace(start, endA, next.slice(start, endB))
  }
  if (typeof tr.setDocAttribute === 'function') for (const [k, v] of Object.entries(next.attrs)) if (cur.attrs[k] !== v) tr.setDocAttribute(k, v)
  if (!tr.docChanged) return
  view.dispatch(tr.setMeta('addToHistory', false))
}

// A save found the chapter changed elsewhere. When the two texts changed
// different paragraphs both are kept (what was sent is saved as a version
// first) and true is answered; otherwise the writer chooses.
async function mergeConflict(id, mine, theirs, version) {
  const base = serverBase?.id === id ? serverBase : null
  if (!base || !view) return false
  const m = mergeDocs(normal(base.doc), normal(mine), theirs)
  if (!m.ok) return false
  try {
    await textApi.saveVersion(id, MERGE_LABEL(), mine)
  } catch (e) {
    return false
  }
  if (chapterId.value !== id || !view) return true
  // Typed while that was on its way: merged too.
  const now = normal(docToJSON(view.state.doc))
  let doc = m.doc
  if (!sameDoc(now, mine)) {
    const again = mergeDocs(normal(mine), now, normal(m.doc))
    if (!again.ok) return false
    doc = again.doc
  }
  serverBase = { id, version, doc: theirs }
  saver.resolved(version, true)
  replaceDoc(doc)
  notice.value = t('This chapter was also changed elsewhere, in other paragraphs: both changes are kept. Your text before the merge is saved as a version.')
  return true
}

// A chapter written on this device without a connection, opened with the
// server back: sent as it is when the server's text hasn't changed since,
// merged when the two changed different paragraphs, else offered as the
// copy kept in this browser.
async function takeKept(id, kept, server, serverDoc) {
  const mine = normal(kept.doc)
  const version = server.version || 0
  if (sameDoc(mine, serverDoc)) return offline.settle(id, serverDoc, version)
  if ((kept.version || 0) === version) {
    setDoc(mine)
    pendingHere.value = true
    saver.changed()
    return
  }
  const m = mergeDocs(normal(kept.base_doc ?? kept.doc), mine, serverDoc)
  if (m.ok) {
    try {
      await textApi.saveVersion(id, MERGE_LABEL(), mine)
      setDoc(normal(m.doc))
      saver.changed()
      notice.value = t('This chapter was also changed elsewhere, in other paragraphs: both changes are kept. Your text before the merge is saved as a version.')
      return
    } catch (e) {
      /* the writer chooses */
    }
  }
  offer.value = { source: 'offline', local: { doc: mine, base_version: kept.version || 0, saved_at: kept.edited_at }, server: { ...server, doc: serverDoc } }
}

// Back online: the open chapter's text kept here goes through the editor's
// save (and its merge), the rest of the story's through the sync.
async function sendKept() {
  if (settingsTimer) saveSettings()
  if (saver && chapterId.value != null && !conflict.value && !offer.value && (pendingHere.value || saver.dirty)) saver.changed()
  const r = await offline.sync((id) => id === chapterId.value)
  if (!r) return
  if (r.conflict.length || r.missing.length) notice.value = t('Some chapters written offline were also changed elsewhere in the same places. Open them to choose a text (Offline writing lists them).')
  else if (r.merged.length) notice.value = t('Chapters written offline were merged with changes made elsewhere (different paragraphs); your texts before the merge are saved as versions.')
}
const stopBack = whenBack(sendKept)

async function toggleOffline(on) {
  if (on) {
    await offline.enable()
    // The open chapter's record starts from what the editor holds.
    if (saver?.dirty) saver.changed()
    return
  }
  const unsent = offline.pendingHere.value.length
  if (unsent && !window.confirm(t('{n} chapters of this book hold text not sent to the server yet. Forget them on this device anyway?', { n: unsent }))) return
  await offline.disable()
  // The open chapter's unsent text is still in the editor: a draft again.
  const had = pendingHere.value
  pendingHere.value = false
  if (had) saver?.changed()
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
    serverBase = { id: chapterId.value, version: server.version || 0, doc: server.doc }
    saver.changed()
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

function discardLocal() {
  const o = offer.value
  if (o?.source === 'offline') offline.settle(chapterId.value, o.server.doc, o.server.version || 0)
  else dropCopy(activeStoryId, chapterId.value)
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
      serverBase = { id, version: c.version, doc: c.theirs }
      saver.resolved(c.version, true)
    } else {
      // Mine is saved as a version; the stored text stays.
      await textApi.saveVersion(id, t('Not kept after a conflict'), c.mine)
      conflict.value = null
      setDoc(c.theirs)
      serverBase = { id, version: c.version, doc: c.theirs }
      saver.resolved(c.version, false)
      offline.settle(id, c.theirs, c.version)
      pendingHere.value = false
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
    const doc = normal(text.doc)
    setDoc(doc)
    serverBase = { id: chapterId.value, version: text.version || 0, doc }
    saver?.resolved(text.version, false)
    offline.settle(chapterId.value, doc, text.version)
    pendingHere.value = false
    loadComments(chapterId.value) // a restored version may bring comments back
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
  const sent = settings.value
  try {
    await booksApi.save(volumeKey.value, sent)
    if (book.value) book.value = { ...book.value, is_set_up: true }
    offline.settingsSaved(sent)
  } catch (e) {
    // A kept book with no connection: kept on this device, sent later (the
    // last settings saved win).
    if (e.offline && (await offline.settingsEdited(sent))) return
    error.value = e.message
  }
}
const setStyles = (list) => updateSettings({ ...settings.value, styles: list })

// ---- Book layout: options per chapter, the table of contents, the page view ---------------
const panelChapter = ref(null) // the chapter the book panel opens at
const isFrontMatter = (id) => !!settings.value && chapterOption(settings.value, id).front_matter
// A chapter's number counts only the chapters that aren't front matter, as
// the book's {chapter_number} does.
const chapterNumbers = computed(() => {
  let n = 0
  return new Map(chapters.value.map((c) => [c.id, isFrontMatter(c.id) ? 0 : ++n]))
})
const toggleChapterOption = (id, key) => updateSettings(setChapterOption(settings.value, id, { [key]: !chapterOption(settings.value, id)[key] }))
function chapterMenu(c) {
  const o = chapterOption(settings.value || {}, c.id)
  const check = (on, label) => (on ? '✓ ' : '') + label
  return [
    { label: check(o.front_matter, t('Front matter')), action: () => toggleChapterOption(c.id, 'front_matter') },
    { label: check(o.no_header, t('No running header')), action: () => toggleChapterOption(c.id, 'no_header') },
    {
      label: t('Page numbers and layout…'),
      action: () => {
        panelChapter.value = c.id
        panel.value = 'book'
      },
    },
  ]
}

// The table of contents in the editor lists the chapters that aren't front matter.
const tocTitles = () => chapters.value.filter((c) => !isFrontMatter(c.id)).map((c) => ({ title: c.title }))
function nodeViews() {
  return {
    toc: tocNodeView(tocTitles, t('Page numbers appear in the page view.')),
    // Footnotes and endnotes (stage 5): the number, and the note's editor
    // while it is selected.
    footnote: noteNodeView({
      title: (kind, n) => (kind === 'endnote' ? t('Endnote {n}', { n }) : t('Footnote {n}', { n })),
      text: { toEndnote: t('Make it an endnote'), toFootnote: t('Make it a footnote'), remove: t('Delete note'), close: t('Back to the text (Esc)') },
      readOnly: () => readOnly.value,
    }),
  }
}
watch(
  () => chapters.value.map((c) => c.title).join('\n') + JSON.stringify(settings.value?.chapter_options || {}),
  () => view?.setProps({ nodeViews: nodeViews() }),
)

// From a character's Lines (?line=N): the chapter's dialogue N selected, in sight.
function goToLine() {
  const want = +route.query.line
  const { line: _drop, ...rest } = route.query
  router.replace({ query: rest })
  let found = null
  let n = 0
  view.state.doc.descendants((node, pos) => {
    if (found != null) return false
    if (node.type !== schema.nodes.dialogue) return true
    if (n++ === want) found = { node, pos }
    return false
  })
  if (!found) return
  const end = found.pos + 1 + found.node.content.size
  view.dispatch(view.state.tr.setSelection(TextSelection.create(view.state.doc, found.pos + 1, end)))
  const dom = view.nodeDOM(found.pos)
  if (dom?.scrollIntoView) dom.scrollIntoView({ block: 'center' })
}

// ---- A dialogue's speaker (Script Mode) ---------------------------------------------------
const speaker = ref(null) // { pos, top, left }: the dialogue whose options are open
const speakerBox = ref(null)
const speakerNode = computed(() => {
  const s = speaker.value
  const node = s && editorState.value?.doc.nodeAt(s.pos)
  return node?.type === schema.nodes.dialogue ? node : null
})
function openSpeaker(pos) {
  speaker.value = { pos, top: 0, left: 0 }
  nextTick(placeSpeaker)
}
function closeSpeaker() {
  speaker.value = null
  view?.focus()
}
function unwrapSpeaker() {
  const pos = speaker.value?.pos
  speaker.value = null
  if (pos != null) exec(dialogueToParagraph(pos))
}
// Under the speaker, kept in the paper area.
function placeSpeaker() {
  const area = paperArea.value
  const s = speaker.value
  if (!view || !area || !s) return
  const dom = view.nodeDOM(s.pos)
  const who = dom?.querySelector?.('.pm-who') || dom
  if (!who?.getBoundingClientRect) return
  const a = area.getBoundingClientRect()
  const r = who.getBoundingClientRect()
  const w = speakerBox.value?.root?.offsetWidth || 384
  speaker.value = {
    ...s,
    top: Math.round(r.bottom - a.top + area.scrollTop + 6),
    left: Math.round(Math.max(8, Math.min(r.left - a.left + area.scrollLeft, area.clientWidth - w - 8))),
  }
}
// The options close when the cursor leaves that dialogue (or it is gone).
watch(editorState, (st) => {
  const s = speaker.value
  if (!s || !st) return
  const { $from } = st.selection
  const inside = $from.depth >= 1 && $from.before($from.depth) === s.pos
  if (!speakerNode.value || !inside) speaker.value = null
})

// From the page view (?at=N): the cursor at the chapter's block N, in sight.
function goToBlock() {
  if (route.query.line != null && view) return goToLine()
  const at = route.query.at
  if (at == null || !view) return
  const want = +at
  let found = null
  view.state.doc.forEach((node, offset, i) => {
    if (i === want) found = offset
  })
  const { at: _drop, ...rest } = route.query
  router.replace({ query: rest })
  if (found == null) return
  const sel = TextSelection.near(view.state.doc.resolve(Math.min(found + 1, view.state.doc.content.size)))
  view.dispatch(view.state.tr.setSelection(sel))
  const dom = view.nodeDOM(found)
  if (dom?.scrollIntoView) dom.scrollIntoView({ block: 'center' })
}

// A paragraph style goes on the selected paragraphs, a character style on
// the selected text.
const applyStyle = (id) => exec(styleById(styles.value, id)?.kind === 'character' ? setCharStyle(id) : setStyle(id))

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

// ---- Pictures ---------------------------------------------------------------------------------
const pictureInput = ref(null)
const pictureBox = ref(null)
const uploading = ref(false)
const picturePlace = ref({ top: 0, left: 0 })
const pictureNode = computed(() => {
  const sel = editorState.value?.selection
  return sel instanceof NodeSelection && sel.node.type === schema.nodes.image ? sel.node : null
})
const pickPicture = () => pictureInput.value?.click()

function onPicture(e) {
  // Read the file before clearing the input (see CLAUDE.md).
  const file = e.target.files[0]
  e.target.value = ''
  if (file) addPictures([file])
}

// Uploads each picture, then puts it in the text at the cursor (a dropped
// picture has moved the cursor to where it was dropped).
async function addPictures(files) {
  for (const file of files) {
    if (!view || readOnly.value) return
    if (!PICTURE_TYPES.includes(file.type)) {
      error.value = t('this is not a picture the editor can use (PNG, JPEG, GIF or WebP)')
      return
    }
    if (file.size > MAX_PICTURE) {
      error.value = t('that picture is too large (over 32 MB)')
      return
    }
    const into = chapterId.value
    uploading.value = true
    error.value = ''
    try {
      const { src } = await booksApi.uploadPicture(volumeKey.value, file)
      // The writer went to another chapter meanwhile: the picture is kept, not placed.
      if (view && chapterId.value === into) exec(insertBlock('image', { src }, { select: true }))
    } catch (err) {
      error.value = err.message
      return
    } finally {
      uploading.value = false
    }
  }
}

// The options sit under the selected picture, kept in view.
function placePictureOptions() {
  const area = paperArea.value
  if (!view || !area || !pictureNode.value) return
  const dom = view.nodeDOM(view.state.selection.from)
  if (!dom?.getBoundingClientRect) return
  const a = area.getBoundingClientRect()
  const r = dom.getBoundingClientRect()
  const box = pictureBox.value?.root
  const h = box?.offsetHeight || 190
  const w = box?.offsetWidth || 368
  const below = r.bottom - a.top + area.scrollTop + 8
  const above = r.top - a.top + area.scrollTop - h - 8
  const lo = area.scrollTop + 8
  const hi = area.scrollTop + area.clientHeight - h - 8
  // Under the picture when it fits in view, else above it, else kept in view.
  const top = below <= hi ? below : above >= lo ? above : Math.max(lo, Math.min(below, hi))
  const left = r.left - a.left + area.scrollLeft
  picturePlace.value = {
    top: Math.round(top),
    left: Math.round(Math.max(8, Math.min(left, area.clientWidth - w - 8))),
  }
}
watch([pictureNode, zoom, areaWidth], () => nextTick(() => nextTick(placePictureOptions)), { flush: 'post' })

// ---- Tables (stage 5) -------------------------------------------------------------------------
// The table box's button sits at the table's top right corner, just
// outside it (in the page's margin) when there is room, kept in view.
const tableBoxEl = ref(null)
const tablePlace = ref({ top: 0, left: 0 })
const tableBox = computed(() => (editorState.value ? tableInfo(editorState.value) : null))
function placeTableOptions() {
  const area = paperArea.value
  const info = tableBox.value
  if (!view || !area || !info) return
  const dom = view.nodeDOM(info.pos)
  if (!dom?.getBoundingClientRect) return
  const a = area.getBoundingClientRect()
  const r = dom.getBoundingClientRect()
  const size = 28
  const lo = area.scrollTop + 4
  const hi = Math.max(lo, r.bottom - a.top + area.scrollTop - size)
  tablePlace.value = {
    top: Math.round(Math.max(lo, Math.min(r.top - a.top + area.scrollTop, hi))),
    left: Math.round(Math.max(4, Math.min(r.right - a.left + area.scrollLeft + 6, area.scrollLeft + area.clientWidth - size - 4))),
  }
}
watch([() => tableBox.value && editorState.value?.doc, () => tableBox.value?.pos, zoom, areaWidth], () => nextTick(() => nextTick(placeTableOptions)), { flush: 'post' })

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
    // The import marks the text before the first chapter as front matter
    // in the book's settings, so take them too.
    book.value = { ...book.value, chapters: res.chapters || [], words: res.words, settings: normaliseSettings(res.settings || book.value.settings) }
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
async function applyImport({ page, styles: takeStyles, layout: takeLayout, meta: takeMeta }) {
  const r = importReport.value
  const old = settings.value
  let next = { ...old }
  if (page && r.page_type_guess) {
    next.page_type = r.page_type_guess
    if (r.page && r.page_type_guess !== 'online') next.page = { ...old.page, ...r.page }
  }
  // The file's headers, footers and page numbers; its front matter is
  // marked as such (lower roman numbers, no header).
  if (takeLayout && r.layout) {
    next.layout = normaliseLayout({ ...old.layout, ...r.layout })
    for (const c of r.chapters || []) {
      if (c.front_matter) next = setChapterOption(next, c.id, { front_matter: true })
      if (c.number_restart != null) next = setChapterOption(next, c.id, { number_restart: c.number_restart })
    }
  }
  if (takeMeta && r.meta) next.meta = { ...old.meta, ...r.meta }
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
  if ((e.ctrlKey || e.metaKey) && e.altKey && e.code === 'Space') {
    e.preventDefault()
    reader.toggle()
  } else if (e.key === 'Escape' && painter.value) {
    painter.value = null
    view?.setProps({})
  } else if (e.key === 'Escape' && reader.status !== 'idle') {
    reader.stop()
    view?.focus()
  } else if (mod && key === 's') {
    e.preventDefault()
    flushText()
    if (settingsTimer) saveSettings()
  } else if (mod && (key === 'f' || key === 'h') && !e.shiftKey) {
    e.preventDefault()
    openFind(key === 'h')
  } else if ((e.ctrlKey || e.metaKey) && e.altKey && key === 'm' && !e.shiftKey) {
    // Word's "New Comment".
    e.preventDefault()
    newComment()
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
  document.head.appendChild(commentStyleEl)
  createView()
  loadChapters()
  loadBook()
  window.addEventListener('keydown', onKey)
  window.addEventListener('beforeunload', onUnload)
  resizer = new ResizeObserver(([entry]) => (areaWidth.value = entry.contentRect.width))
  if (paperArea.value) resizer.observe(paperArea.value)
})

onBeforeUnmount(() => {
  stopBack()
  reader.destroy()
  window.removeEventListener('keydown', onKey)
  window.removeEventListener('beforeunload', onUnload)
  resizer?.disconnect()
  saver?.dispose()
  saver = null
  view?.destroy()
  view = null
  styleEl.remove()
  commentStyleEl.remove()
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

/* The toolbar's pop-ups over the page, table cells (positioned) included. */
.write-tools {
  position: relative;
  z-index: 25;
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

.chapter-row {
  position: relative;
  display: flex;
  align-items: center;
}

.chapter-row .chapter-item {
  flex: 1;
  min-width: 0;
}

/* The chapter's layout menu shows on hover (and when it has the focus). */
.ch-menu {
  position: absolute;
  right: 0.1rem;
  opacity: 0;
  transition: opacity 0.15s;
}

.ch-menu :deep(.menu-btn) {
  min-height: 1.6rem;
  padding: 0 0.45rem;
  background: var(--surface);
}

.chapter-row:hover .ch-menu,
.ch-menu:focus-within,
.ch-menu.is-open {
  opacity: 1;
}

.pages-link {
  margin-left: 0.8rem;
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
  position: relative; /* a note's box (noteView.js) is placed in it */
  min-height: 100%;
}

.write-host :deep(.write-doc.is-painting) {
  cursor: copy;
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
  gap: 0.3rem 0.7rem;
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

/* The save state takes the room that is left and shortens with "…" rather
   than pushing the buttons onto a second line. */
.save-state {
  flex: 1 1 0;
  min-width: 4.5em;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.save-state.is-saved {
  color: var(--text-faint);
}

.save-state.is-local,
.save-state.is-conflict,
.save-state.is-device {
  color: var(--accent);
}

/* Offline writing (stage 5). */
.write-notice {
  margin: 0;
  padding: 0.5rem 0.9rem;
  border-radius: 8px;
  border: 1px solid color-mix(in srgb, var(--accent) 40%, transparent);
  background: var(--accent-soft);
  font-size: 0.85rem;
}

.offline-status {
  font-size: 0.76rem;
  color: var(--text-muted);
}

.offline-status.is-on {
  color: var(--text-primary);
}

.offline-status.is-away {
  color: var(--accent);
  font-weight: 600;
}

.spacer {
  flex: 1;
}

.spell-status {
  font-size: 0.76rem;
  color: var(--text-muted);
}

.spell-status.is-none,
.spell-status.is-error {
  color: var(--accent);
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
