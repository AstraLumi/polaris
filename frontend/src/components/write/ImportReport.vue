<template>
  <div class="overlay import-overlay" @click.self="$emit('close')">
    <div class="modal glass-panel panel-solid is-wide import-report" role="dialog" aria-modal="true" :aria-label="$t('Word file imported')">
      <h2>{{ $t('Word file imported') }}</h2>

      <p class="lead">{{ $tn('{n} chapter was added at the end of the book:', '{n} chapters were added at the end of the book:', report.chapters.length) }}</p>
      <ol class="new-chapters">
        <li v-for="c in report.chapters" :key="c.id">
          <span class="title">{{ c.title }}</span>
          <span class="words">{{ $tn('{n} word', '{n} words', c.words || 0) }}</span>
        </li>
      </ol>

      <template v-if="leftOut.length">
        <h3>{{ $t('Left out or simplified') }}</h3>
        <ul class="left-out">
          <li v-for="l in leftOut" :key="l.key">{{ $t('{what}: {n}', { what: l.label, n: l.n }) }}</li>
        </ul>
      </template>
      <p v-else class="hint">{{ $t('Nothing in the file was left out.') }}</p>

      <section v-if="canApply" class="apply">
        <h3>{{ $t('The file\'s layout') }}</h3>
        <label v-if="report.page_type_guess" class="check">
          <input v-model="usePage" type="checkbox" />
          <span>
            {{ $t('Page type: {type}', { type: guessName }) }}
            <span v-if="report.page_type_guess !== settings.page_type" class="faint">{{ $t('(the book is {type} now)', { type: currentName }) }}</span>
          </span>
        </label>
        <label v-if="report.styles?.length" class="check">
          <input v-model="useStyles" type="checkbox" />
          <span>{{ $tn('Its {n} style (Normal, headings… as the file has them)', 'Its {n} styles (Normal, headings… as the file has them)', report.styles.length) }}</span>
        </label>
        <label v-if="report.layout" class="check">
          <input v-model="useLayout" type="checkbox" />
          <span>
            {{ $t('Its headers, footers and page numbers') }}
            <span v-if="layoutSummary" class="faint">{{ layoutSummary }}</span>
          </span>
        </label>
        <label v-if="metaSummary" class="check">
          <input v-model="useMeta" type="checkbox" />
          <span>{{ $t('Its details: {what}', { what: metaSummary }) }}</span>
        </label>
        <p v-if="report.fonts?.length" class="hint">
          {{ $t('Fonts it uses: {fonts}. Those Google Fonts has are loaded; the others show only if this computer has them.', { fonts: report.fonts.join(', ') }) }}
        </p>
        <p v-if="usePage && settings.page_type && report.page_type_guess !== settings.page_type && setUp" class="hint">
          {{ $t('Every chapter is saved as a version before the page type changes.') }}
        </p>
      </section>

      <div class="modal-actions">
        <button type="button" class="btn btn-ghost" :disabled="busy" @click="$emit('close')">{{ canApply ? $t('Keep the book as it is') : $t('Close') }}</button>
        <button v-if="canApply" type="button" class="btn btn-primary" :disabled="busy || (!usePage && !useStyles && !useLayout && !useMeta)" @click="$emit('apply', { page: usePage, styles: useStyles, layout: useLayout, meta: useMeta })">
          {{ $t('Apply to the book') }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
// What a .docx upload did (POST /api/books/{volume}/import-docx): the
// chapters it made, what it left out, and the offer to take the file's
// page type, page and styles for the book.
import { ref, computed } from 'vue'
import { t, tr } from '../../i18n'
import { pageTypeName } from '../../editor/pageTypes.js'

// What an import can leave out (docxLeftOutKeys in backend/docx.go).
const LEFT_OUT = {
  headers_footers: tr('headers and footers (they come with the book layout)'),
  toc: tr('tables of contents'),
  fields: tr('page numbers, page references and index entries'),
  fields_as_text: tr('other fields, kept as their plain text'),
  footnotes: tr('footnotes (their text is not imported yet)'),
  endnotes: tr('endnotes'),
  comments: tr('comments'),
  tables: tr('tables (their text is kept, cell by cell)'),
  text_boxes: tr('text boxes (their text is kept after their paragraph)'),
  equations: tr('equations'),
  tracked_changes: tr('tracked changes (accepted)'),
  drawings: tr('shapes, charts and SmartArt'),
  objects: tr('embedded objects and files'),
  pictures: tr('pictures a browser can\'t show, or too large'),
  hidden_text: tr('hidden text'),
  columns: tr('sections in more than one column'),
  sections: tr('extra sections (each became a page break)'),
}

const props = defineProps({
  report: { type: Object, required: true },
  settings: { type: Object, required: true },
  setUp: { type: Boolean, default: true },
  busy: { type: Boolean, default: false },
})
defineEmits(['apply', 'close'])

const usePage = ref(!!props.report.page_type_guess)
const useStyles = ref(!!props.report.styles?.length)
const useLayout = ref(!!props.report.layout)
const useMeta = ref(!!props.report.meta && Object.keys(props.report.meta).length > 0)

// What the file's layout says, in a few words: where the page number and
// the chapter's title go, and how pages are numbered.
const NUMBERS = { arabic: '1, 2, 3', roman: 'i, ii, iii', upper_roman: 'I, II, III', alpha: 'a, b, c', upper_alpha: 'A, B, C' }
const layoutSummary = computed(() => {
  const l = props.report.layout
  if (!l) return ''
  const bands = ['header', 'footer'].flatMap((p) => ['odd', 'even', 'first'].map((s) => l[p]?.[s] || {}))
  const has = (f) => bands.some((b) => [b.left, b.center, b.right].some((x) => (x || '').includes(f)))
  const parts = []
  if (has('{chapter}')) parts.push(t('chapter titles'))
  if (has('{page}')) parts.push(t('page numbers {format}', { format: NUMBERS[l.numbering?.format] || '1, 2, 3' }))
  if (l.different_first) parts.push(t('own first pages'))
  if (l.different_odd_even) parts.push(t('left and right pages'))
  return parts.length ? `(${parts.join(', ')})` : ''
})
const metaSummary = computed(() => {
  const m = props.report.meta || {}
  return [m.title, m.author].filter(Boolean).join(', ')
})

const leftOut = computed(() =>
  Object.entries(props.report.left_out || {})
    .filter(([, n]) => n > 0)
    .map(([key, n]) => ({ key, n, label: LEFT_OUT[key] ? t(LEFT_OUT[key]) : key.replace(/_/g, ' ') })),
)
const canApply = computed(() => !!props.report.page_type_guess || !!props.report.styles?.length || !!props.report.layout || !!metaSummary.value)
const nameOf = (type, page) => (type === 'online' ? t('Online Post') : type === 'custom' ? `${t('Custom size')} · ${pageTypeName({ page_type: 'custom', page })}` : pageTypeName({ page_type: type }))
const guessName = computed(() => nameOf(props.report.page_type_guess, props.report.page))
const currentName = computed(() => nameOf(props.settings.page_type, props.settings.page))
</script>

<style scoped>
.import-overlay {
  z-index: 80;
}

.lead {
  margin: -0.5rem 0 0.5rem;
  color: var(--text-muted);
}

.new-chapters {
  margin: 0 0 1rem;
  padding-left: 1.4rem;
  max-height: 12rem;
  overflow-y: auto;
}

.new-chapters li {
  padding: 0.15rem 0;
}

.new-chapters .words,
.faint {
  margin-left: 0.5rem;
  font-size: 0.78rem;
  color: var(--text-faint);
}

h3 {
  margin: 0.8rem 0 0.4rem;
  font-size: 1rem;
}

.left-out {
  margin: 0;
  padding-left: 1.2rem;
  font-size: 0.88rem;
  color: var(--text-muted);
  columns: 2 16rem;
}

.left-out li::first-letter {
  text-transform: uppercase;
}

.apply {
  margin-top: 1rem;
  padding: 0.7rem 0.9rem;
  border-radius: 10px;
  border: 1px solid var(--glass-border);
  background: var(--glass);
}

.apply h3 {
  margin-top: 0;
}

.check {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  margin: 0.35rem 0;
  font-size: 0.9rem;
  cursor: pointer;
}

.check input {
  margin-top: 0.2rem;
}

.hint {
  margin: 0.5rem 0 0;
  font-size: 0.8rem;
  color: var(--text-faint);
  line-height: 1.45;
}
</style>
