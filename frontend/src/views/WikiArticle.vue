<template>
  <div class="page wiki-article">
    <BackLink to="/wiki" :label="$t('← Back to the wiki')" />

    <p v-if="loadError" class="error-banner">{{ loadError }}</p>
    <p v-else-if="!article" class="loading-hint">{{ $t('Loading…') }}</p>

    <article v-else class="paper glass-panel">
      <header class="title-row">
        <div class="title-text">
          <h1>{{ article.name }}</h1>
          <span class="kind">{{ $t(TYPE_LABELS[article.type]) }}</span>
        </div>
        <div class="actions">
          <template v-if="!editing">
            <button class="btn btn-primary small" @click="startEdit()">{{ $t('Edit') }}</button>
          </template>
          <template v-else>
            <button v-if="isLore" class="btn btn-danger small" :disabled="saving" @click="deleteOpen = true">{{ $t('Delete article') }}</button>
            <button class="btn btn-ghost small" :disabled="saving" @click="cancelEdit">{{ $t('Cancel') }}</button>
            <button class="btn btn-primary small" :disabled="saving" @click="save">
              {{ saving ? $t('Saving…') : $t('Save') }}
            </button>
          </template>
        </div>
      </header>

      <p v-if="saveError" class="error-banner">{{ saveError }}</p>

      <p v-if="isLore" class="hatnote">{{ $t('A free-form article: everything on it is written here.') }}</p>
      <p v-else class="hatnote">
        {{ hatParts[0] }}<RouterLink :to="sourcePath(article.type, article.id)">{{ sourceLabel(article.type) }}</RouterLink>{{ hatParts[1] }}
      </p>

      <div class="layout">
        <!-- ------------------------------------------------ the text -->
        <div class="main" @click="onContentClick">
          <template v-if="!editing">
            <div v-if="article.summary" class="md lead" v-html="md(article.summary)"></div>
            <p v-else-if="!hasText" class="stub-note">
              {{ $t('This article has no written text yet.') }}
              <a href="#" @click.prevent="startEdit()">{{ $t('Start writing it') }}</a>
            </p>

            <nav v-if="toc.length >= 2" class="toc">
              <div class="toc-head">
                <strong>{{ $t('Contents') }}</strong>
                <a href="#" class="toc-toggle" @click.prevent="tocOpen = !tocOpen">[{{ tocOpen ? $t('hide') : $t('show') }}]</a>
              </div>
              <ol v-if="tocOpen">
                <li v-for="(s, n) in toc" :key="s.id">
                  <a :href="`#${s.id}`" @click.prevent="jump(s.id)"><span class="num">{{ n + 1 }}</span> {{ s.title }}</a>
                </li>
              </ol>
            </nav>

            <section v-for="s in sheetSections" :id="s.id" :key="s.id" class="part">
              <h2>{{ s.title }}</h2>
              <p class="origin-note">{{ $t('Written on the original page; edit it there.') }}</p>
              <div class="md" v-html="md(s.body)"></div>
            </section>

            <section v-for="s in fieldSections" :id="s.id" :key="s.id" class="part">
              <h2>
                {{ s.title }}
                <a href="#" class="edit-link" :title="$t('Edit this section')" @click.prevent="startEdit(s.focus)">[ {{ $t('edit') }} ]</a>
              </h2>
              <div class="md" v-html="md(s.body)"></div>
            </section>

            <section v-for="s in customSections" :id="s.id" :key="s.id" class="part">
              <h2>
                {{ s.title }}
                <a href="#" class="edit-link" :title="$t('Edit this section')" @click.prevent="startEdit(s.focus)">[ {{ $t('edit') }} ]</a>
              </h2>
              <div class="md" v-html="md(s.body)"></div>
            </section>

            <section v-if="article.groups.length" id="sec-related" class="part">
              <h2>{{ $t('Related') }}</h2>
              <div v-for="g in article.groups" :key="g.key" class="group">
                <h3>{{ $t(GROUP_LABELS[g.key]) }}</h3>
                <ul class="links">
                  <li v-for="(r, i) in g.items" :key="i">
                    <template v-if="r.note && g.key === 'relations'">{{ (r.note_word ? $t(r.note) : r.note) + ' ' }}</template>
                    <RouterLink :to="wikiPath(r.type, r.id)">{{ r.name }}</RouterLink>
                    <span v-if="r.note && g.key !== 'relations'" class="faint"> ({{ r.note_word ? $t(r.note) : r.note }})</span>
                  </li>
                </ul>
              </div>
            </section>

            <section id="sec-backlinks" class="part">
              <h2>{{ $t('What links here') }}</h2>
              <ul v-if="article.backlinks.length" class="links">
                <li v-for="r in article.backlinks" :key="r.type + r.id">
                  <RouterLink :to="wikiPath(r.type, r.id)">{{ r.name }}</RouterLink>
                  <span class="faint"> ({{ $t(TYPE_LABELS[r.type]) }})</span>
                </li>
              </ul>
              <p v-else class="faint">{{ $t('No other article links here yet.') }}</p>
            </section>

            <p v-if="article.updated_at" class="foot">
              {{ $t('Last edited {when}', { when: editedAt }) }}
            </p>
          </template>

          <!-- ------------------------------------------- the editor -->
          <form v-else class="editor" @submit.prevent="save">
            <p class="hint">
              {{ $t('Format text with the buttons above each box, or type Markdown. Use “Link to article” to link to another page of the wiki.') }}
            </p>

            <div v-if="isLore" class="field">
              <label for="f-title">{{ $t('Title') }}</label>
              <input id="f-title" v-model="draft.name" type="text" maxlength="120" required />
            </div>

            <div class="field">
              <label for="f-summary">{{ $t('Introduction') }}</label>
              <MarkdownEditor id="f-summary" v-model="draft.summary" :items="items" :resolve="resolver" :placeholder="$t('A few sentences that open the article.')" />
            </div>

            <div v-for="k in article.field_keys" :key="k" class="field">
              <label :for="`f-${k}`">{{ $t(FIELD_LABELS[k]) }}</label>
              <MarkdownEditor :id="`f-${k}`" v-model="draft.fields[k]" :items="items" :resolve="resolver" />
            </div>

            <div class="custom-head">
              <h2>{{ $t('Extra sections') }}</h2>
              <button type="button" class="btn btn-ghost small" @click="addSection">{{ $t('Add section') }}</button>
            </div>
            <p v-if="!draft.sections.length" class="faint">{{ $t('Add your own titled sections, shown after the ones above.') }}</p>
            <div v-for="(s, i) in draft.sections" :key="s.key" class="custom-section">
              <div class="custom-row">
                <input :id="`s-${i}`" v-model="s.title" type="text" :placeholder="$t('Section title')" :aria-label="$t('Section title')" maxlength="120" />
                <button type="button" class="btn btn-ghost small" :disabled="i === 0" :aria-label="$t('Move up')" @click="move(draft.sections, i, -1)">↑</button>
                <button type="button" class="btn btn-ghost small" :disabled="i === draft.sections.length - 1" :aria-label="$t('Move down')" @click="move(draft.sections, i, 1)">↓</button>
                <button type="button" class="btn btn-danger small" @click="draft.sections.splice(i, 1)">{{ $t('Remove') }}</button>
              </div>
              <MarkdownEditor v-model="s.body" :items="items" :resolve="resolver" :aria-label="$t('Section text')" />
            </div>

            <div class="editor-actions">
              <button type="button" class="btn btn-ghost" :disabled="saving" @click="cancelEdit">{{ $t('Cancel') }}</button>
              <button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? $t('Saving…') : $t('Save') }}</button>
            </div>
          </form>
        </div>

        <!-- ------------------------------------------------ the infobox -->
        <aside class="infobox" :aria-label="$t('Summary box')">
          <div class="info-title">{{ article.name }}</div>
          <div v-if="editing && isLore" class="info-pic-edit">
            <div class="info-pic">
              <IconImage v-if="shownPicture" :src="shownPicture" :name="draft.name" :alt="draft.name" />
              <span v-else class="no-pic">{{ $t('No picture') }}</span>
            </div>
            <div class="pic-actions">
              <button type="button" class="btn btn-ghost small" @click="pictureInput.click()">
                {{ shownPicture ? $t('Change picture') : $t('Upload picture') }}
              </button>
              <button v-if="shownPicture" type="button" class="btn btn-ghost small" @click="dropPicture">{{ $t('Remove') }}</button>
              <input ref="pictureInput" type="file" accept="image/*" hidden @change="onPicture" />
            </div>
          </div>
          <div
            v-else-if="article.picture"
            class="info-pic"
            :class="{ zoomable: article.picture.startsWith('/uploads/') }"
            @click="article.picture.startsWith('/uploads/') && viewPicture(article.picture, article.name)"
          >
            <IconImage :src="article.picture" :name="article.name" :alt="article.name" />
          </div>
          <table>
            <tbody>
              <tr>
                <th>{{ $t('Kind') }}</th>
                <td>{{ $t(TYPE_LABELS[article.type]) }}</td>
              </tr>
              <tr v-for="f in article.facts" :key="f.key">
                <th>{{ $t(FACT_LABELS[f.key]) }}</th>
                <td>
                  <RouterLink v-if="f.link" :to="wikiPath(f.link.type, f.link.id)">{{ f.link.name }}</RouterLink>
                  <template v-else>{{ factText(f) }}</template>
                </td>
              </tr>
            </tbody>
          </table>

          <template v-if="!editing && article.infobox.length">
            <div class="info-sub">{{ $t('More details') }}</div>
            <table>
              <tbody>
                <tr v-for="(r, i) in article.infobox" :key="i">
                  <th>{{ r.label }}</th>
                  <td class="md inline" v-html="mdInline(r.value)"></td>
                </tr>
              </tbody>
            </table>
          </template>

          <template v-if="editing">
            <div class="info-sub">{{ $t('More details') }}</div>
            <div class="info-edit">
              <div v-for="(r, i) in draft.infobox" :key="r.key" class="info-row">
                <input v-model="r.label" type="text" :placeholder="$t('Label')" :aria-label="$t('Label')" maxlength="120" />
                <input :ref="(el) => (valueEls[r.key] = el)" v-model="r.value" type="text" :placeholder="$t('Value')" :aria-label="$t('Value')" maxlength="2000" />
                <button type="button" class="btn btn-ghost small row-link" :title="$t('Link to another wiki article')" @mousedown.prevent @click="openRowLink(r)">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                    <path d="M10 13a5 5 0 0 0 7.5.5l3-3a5 5 0 0 0-7-7l-1.7 1.7" />
                    <path d="M14 11a5 5 0 0 0-7.5-.5l-3 3a5 5 0 0 0 7 7l1.7-1.7" />
                  </svg>
                </button>
                <button type="button" class="btn btn-ghost small" :aria-label="$t('Remove')" @click="draft.infobox.splice(i, 1)">×</button>
              </div>
              <button type="button" class="btn btn-ghost small" @click="addRow">{{ $t('Add row') }}</button>
            </div>
          </template>
        </aside>
      </div>
    </article>

    <WikiLinkPanel v-if="rowLink" :items="items" :resolve="resolver" :text="rowLink.selected" @pick="insertRowLink" @close="closeRowLink" />
    <ConfirmDialog
      v-if="deleteOpen"
      :title="$t('Delete this article?')"
      :message="$t('Its text and picture are deleted. Links to it from other articles stay, shown as missing.')"
      :confirm-label="$t('Delete')"
      @cancel="deleteOpen = false"
      @confirm="deleteArticle"
    />
  </div>
</template>

<script setup>
import { ref, reactive, computed, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import IconImage from '../components/IconImage.vue'
import BackLink from '../components/BackLink.vue'
import MarkdownEditor from '../components/MarkdownEditor.vue'
import WikiLinkPanel from '../components/WikiLinkPanel.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import { pageTitle, useUnsavedGuard, useSaveShortcut, viewPicture } from '../navigation'
import { renderMarkdown } from '../markdown'
import { fullDate, dateParts } from '../calendar'
import { t } from '../i18n'
import {
  TYPE_LABELS, FIELD_LABELS, FACT_LABELS, GROUP_LABELS, SHEET_LABELS,
  wikiApi, wikiPath, sourcePath, sourceLabel, makeLinkResolver,
} from '../wiki'

const props = defineProps({
  type: { type: String, required: true },
  id: { type: String, required: true },
})
const router = useRouter()
const route = useRoute()

const article = ref(null)
const items = ref([])
const loadError = ref('')
const saveError = ref('')
const editing = ref(false)
const saving = ref(false)
const tocOpen = ref(true)
const draft = reactive({ name: '', summary: '', fields: {}, sections: [], infobox: [] })
const isLore = computed(() => article.value?.type === 'lore')
let snapshot = ''
let keySeq = 0

const resolver = computed(() => makeLinkResolver(items.value))
const md = (text) => renderMarkdown(text, resolver.value)
// Infobox values are one line: no paragraph wrapper.
const mdInline = (text) => md(text).replace(/^<p>([\s\S]*)<\/p>$/, '$1')

// "For the {link}, see …" is one translatable sentence with a link slot.
const hatParts = computed(() =>
  t('Facts shown here come from the {link}; this page adds the story around them.', { link: '\u0001' }).split('\u0001'),
)

const sheetSections = computed(() =>
  article.value.sheet.map((s) => ({ id: `sec-sheet-${s.key}`, title: t(SHEET_LABELS[s.key]), body: s.body })),
)
const fieldSections = computed(() =>
  article.value.field_keys
    .filter((k) => article.value.fields[k])
    .map((k) => ({ id: `sec-${k}`, title: t(FIELD_LABELS[k]), body: article.value.fields[k], focus: `f-${k}` })),
)
const customSections = computed(() =>
  article.value.sections.map((s, i) => ({ id: `sec-custom-${i}`, title: s.title, body: s.body, focus: `s-${i}` })),
)
const hasText = computed(
  () => fieldSections.value.length || customSections.value.length || article.value.infobox.length || sheetSections.value.length,
)

const toc = computed(() => {
  const rows = [...sheetSections.value, ...fieldSections.value, ...customSections.value]
  if (article.value.groups.length) rows.push({ id: 'sec-related', title: t('Related') })
  rows.push({ id: 'sec-backlinks', title: t('What links here') })
  return rows
})

// The server stores UTC ("2026-10-08 21:33:54").
const editedAt = computed(() => {
  const d = new Date(`${article.value.updated_at.replace(' ', 'T')}Z`)
  return Number.isNaN(d.getTime()) ? article.value.updated_at : d.toLocaleString()
})

function factText(f) {
  if (f.kind === 'date') return dateParts(f.value) ? fullDate(f.value) : f.value
  if (f.kind === 'word') return t(f.value)
  return f.value
}

function jump(id) {
  document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

// Links written as [[Name]] are plain anchors in the rendered text; send them
// through the router so the page doesn't reload.
function onContentClick(e) {
  const a = e.target.closest && e.target.closest('a[data-wiki]')
  if (!a) return
  e.preventDefault()
  router.push(a.getAttribute('href'))
}

const makeDraft = () => ({
  summary: draft.summary,
  fields: Object.fromEntries(Object.entries(draft.fields).filter(([, v]) => v && v.trim())),
  sections: draft.sections.map((s) => ({ title: s.title, body: s.body })),
  infobox: draft.infobox.map((r) => ({ label: r.label, value: r.value })),
})
// A lore article's title and picture are saved apart from its text.
const draftState = () => JSON.stringify({ ...makeDraft(), name: draft.name })
const pictureChanged = () => !!pictureFile.value || pictureRemoved.value
const metaChanged = () => draft.name.trim() !== article.value.name || pictureChanged()
const dirty = () => editing.value && (draftState() !== snapshot || pictureChanged())

// ---- a lore article's picture ----

const pictureInput = ref(null)
const pictureFile = ref(null)
const pictureRemoved = ref(false)
const previewUrl = ref('')
const shownPicture = computed(() => previewUrl.value || (pictureRemoved.value ? '' : article.value?.picture || ''))

function clearPreview() {
  if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
  previewUrl.value = ''
}

function onPicture(e) {
  const file = e.target.files[0] // read before clearing the input
  e.target.value = ''
  if (!file) return
  clearPreview()
  pictureFile.value = file
  pictureRemoved.value = false
  previewUrl.value = URL.createObjectURL(file)
}

function dropPicture() {
  clearPreview()
  pictureFile.value = null
  pictureRemoved.value = true
}

// ---- linking from an infobox value ----

const valueEls = {}
const rowLink = ref(null) // { key, from, to, selected }

function openRowLink(r) {
  const el = valueEls[r.key]
  const focused = el && document.activeElement === el
  const from = focused ? el.selectionStart : r.value.length
  const to = focused ? el.selectionEnd : r.value.length
  rowLink.value = { key: r.key, from, to, selected: r.value.slice(from, to) }
}

async function closeRowLink(caret) {
  const key = rowLink.value?.key
  rowLink.value = null
  await nextTick()
  const el = valueEls[key]
  if (!el) return
  el.focus()
  if (caret != null) el.setSelectionRange(caret, caret)
}

function insertRowLink(markup) {
  const { key, from, to } = rowLink.value
  const row = draft.infobox.find((r) => r.key === key)
  if (row) row.value = row.value.slice(0, from) + markup + row.value.slice(to)
  closeRowLink(from + markup.length)
}

function fillDraft() {
  const a = article.value
  draft.name = a.name
  clearPreview()
  pictureFile.value = null
  pictureRemoved.value = false
  draft.summary = a.summary
  draft.fields = Object.fromEntries(a.field_keys.map((k) => [k, a.fields[k] || '']))
  draft.sections = a.sections.map((s) => ({ key: ++keySeq, title: s.title, body: s.body }))
  draft.infobox = a.infobox.map((r) => ({ key: ++keySeq, label: r.label, value: r.value }))
  snapshot = draftState()
}

async function startEdit(focusId) {
  saveError.value = ''
  fillDraft()
  editing.value = true
  await nextTick()
  const el = focusId && document.getElementById(focusId)
  if (el) {
    el.scrollIntoView({ block: 'center' })
    el.focus()
  } else {
    window.scrollTo({ top: 0 })
  }
}

function cancelEdit() {
  if (dirty() && !window.confirm(t('Discard your changes?'))) return
  editing.value = false
  saveError.value = ''
}

const addSection = () => draft.sections.push({ key: ++keySeq, title: '', body: '' })
const addRow = () => draft.infobox.push({ key: ++keySeq, label: '', value: '' })

function move(list, i, by) {
  const j = i + by
  if (j < 0 || j >= list.length) return
  list.splice(j, 0, list.splice(i, 1)[0])
}

async function save() {
  saving.value = true
  saveError.value = ''
  try {
    if (isLore.value && metaChanged()) {
      const fd = new FormData()
      fd.append('name', draft.name.trim())
      if (pictureFile.value) fd.append('picture', pictureFile.value)
      else if (pictureRemoved.value) fd.append('remove_picture', '1')
      await wikiApi.saveLore(props.id, fd)
    }
    article.value = await wikiApi.save(props.type, props.id, makeDraft())
    pageTitle.value = article.value.name
    clearPreview()
    editing.value = false
    // A new written article should show up as such in the links' index too.
    items.value = await wikiApi.list()
    window.scrollTo({ top: 0 })
  } catch (e) {
    saveError.value = e.message
  } finally {
    saving.value = false
  }
}

async function load() {
  loadError.value = ''
  saveError.value = ''
  article.value = null
  editing.value = false
  try {
    const [a, list] = await Promise.all([wikiApi.get(props.type, props.id), wikiApi.list()])
    article.value = a
    items.value = list
    pageTitle.value = a.name
  } catch (e) {
    loadError.value = e.message
    return
  }
  // A new article opens straight in the editor (WikiHome's "New article").
  if (route.query.edit === '1') {
    router.replace({ query: {} })
    startEdit('f-title')
  }
}

const deleteOpen = ref(false)
async function deleteArticle() {
  deleteOpen.value = false
  saving.value = true
  saveError.value = ''
  try {
    await wikiApi.deleteLore(props.id)
    editing.value = false
    router.push('/wiki')
  } catch (e) {
    saveError.value = e.message
  } finally {
    saving.value = false
  }
}

watch(() => [props.type, props.id], load, { immediate: true })

useUnsavedGuard(dirty)
useSaveShortcut(() => editing.value && !saving.value && save())
</script>

<style scoped>
.paper {
  padding: 1.4rem 1.75rem 2rem;
}

.title-row {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  gap: 1rem;
  flex-wrap: wrap;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid var(--glass-border);
}

.title-text {
  display: flex;
  align-items: baseline;
  gap: 0.9rem;
  flex-wrap: wrap;
  min-width: 0;
}

.title-text h1 {
  margin: 0;
  font-size: 2.1rem;
  line-height: 1.15;
  overflow-wrap: anywhere;
}

.kind {
  font-size: 0.8rem;
  color: var(--text-faint);
  text-transform: uppercase;
  letter-spacing: 0.1em;
}

.actions {
  display: flex;
  gap: 0.5rem;
}

.hatnote {
  margin: 0.8rem 0 1.2rem 1rem;
  font-size: 0.86rem;
  font-style: italic;
  color: var(--text-muted);
}

.hatnote a,
.links a,
.toc a,
.stub-note a,
.infobox a,
.md :deep(a) {
  color: var(--line);
  text-decoration: none;
}

.hatnote a:hover,
.links a:hover,
.toc a:hover,
.stub-note a:hover,
.infobox a:hover,
.md :deep(a:hover) {
  text-decoration: underline;
}

.layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 19rem;
  gap: 1.75rem;
  align-items: start;
}

/* ---- the text ---- */

.lead {
  margin-bottom: 1.2rem;
}

.stub-note {
  margin: 0 0 1.2rem;
  padding: 0.7rem 0.9rem;
  border: 1px dashed var(--glass-border);
  border-radius: 8px;
  font-size: 0.9rem;
  color: var(--text-muted);
}

.toc {
  display: inline-block;
  min-width: 15rem;
  max-width: 100%;
  margin: 0 0 1.4rem;
  padding: 0.7rem 1.2rem 0.8rem;
  border: 1px solid var(--glass-border);
  background: rgba(255, 255, 255, 0.03);
  font-size: 0.88rem;
}

.toc-head {
  display: flex;
  justify-content: center;
  gap: 0.5rem;
  margin-bottom: 0.4rem;
}

.toc-toggle {
  font-size: 0.8rem;
}

.toc ol {
  list-style: none;
  margin: 0;
  padding: 0;
}

.toc li {
  padding: 0.12rem 0;
}

.toc .num {
  color: var(--text-faint);
  margin-right: 0.35rem;
}

.part {
  margin-bottom: 1.4rem;
}

.part h2 {
  display: flex;
  align-items: baseline;
  gap: 0.6rem;
  margin: 0 0 0.7rem;
  padding-bottom: 0.2rem;
  font-size: 1.6rem;
  border-bottom: 1px solid var(--glass-border);
}

.edit-link {
  font-family: 'Manrope', sans-serif;
  font-size: 0.75rem;
  font-weight: 500;
  color: var(--line);
  text-decoration: none;
}

.edit-link:hover {
  text-decoration: underline;
}

.origin-note {
  margin: -0.3rem 0 0.6rem;
  font-size: 0.78rem;
  font-style: italic;
  color: var(--text-faint);
}

.group h3 {
  margin: 0.8rem 0 0.3rem;
  font-size: 1.1rem;
}

.links {
  margin: 0;
  padding-left: 1.2rem;
  columns: 14rem;
  font-size: 0.92rem;
}

.faint {
  color: var(--text-faint);
  font-size: 0.88rem;
}

.foot {
  margin: 2rem 0 0;
  padding-top: 0.6rem;
  border-top: 1px solid var(--glass-border);
  font-size: 0.78rem;
  color: var(--text-faint);
}

/* Rendered Markdown itself is styled in style.css (.md). */

.md.inline :deep(p) {
  margin: 0;
}

/* ---- the infobox ---- */

.infobox {
  border: 1px solid var(--glass-border);
  background: rgba(255, 255, 255, 0.03);
  font-size: 0.86rem;
}

.info-title,
.info-sub {
  padding: 0.5rem 0.7rem;
  text-align: center;
  font-weight: 700;
  color: #fff;
  background: color-mix(in srgb, var(--line) 62%, var(--surface));
}

.info-title {
  font-size: 0.98rem;
  overflow-wrap: anywhere;
}

.info-sub {
  font-size: 0.8rem;
  background: color-mix(in srgb, var(--line) 38%, var(--surface));
}

.info-pic {
  display: flex;
  justify-content: center;
  padding: 0.8rem;
}

.info-pic > :deep(img) {
  max-width: 100%;
  max-height: 16rem;
  border-radius: 6px;
  object-fit: contain;
}

.info-pic:has(svg) {
  height: 6rem;
}

.infobox table {
  width: 100%;
  border-collapse: collapse;
}

.infobox th {
  width: 38%;
  padding: 0.35rem 0.6rem;
  text-align: right;
  vertical-align: top;
  font-weight: 700;
  color: var(--text-muted);
}

.infobox td {
  padding: 0.35rem 0.6rem;
  vertical-align: top;
  border-left: 1px solid var(--glass-border);
  overflow-wrap: anywhere;
}

.info-edit {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding: 0.6rem;
}

.info-row {
  display: grid;
  grid-template-columns: 1fr 1fr auto auto;
  gap: 0.35rem;
}

.row-link svg {
  width: 0.95rem;
  height: 0.95rem;
}

.info-row .btn.small {
  padding: 0 0.55rem;
}

.info-pic-edit .info-pic {
  min-height: 6rem;
  align-items: center;
}

.no-pic {
  font-size: 0.8rem;
  font-style: italic;
  color: var(--text-faint);
}

.pic-actions {
  display: flex;
  justify-content: center;
  gap: 0.4rem;
  padding: 0 0.6rem 0.7rem;
}

/* ---- the editor ---- */

.hint {
  margin: 0 0 1rem;
  font-size: 0.82rem;
  color: var(--text-muted);
}

.custom-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin: 1.5rem 0 0.6rem;
}

.custom-head h2 {
  margin: 0;
  font-size: 1.3rem;
}

.custom-section {
  margin-bottom: 1rem;
  padding: 0.75rem;
  border: 1px solid var(--glass-border);
  border-radius: 10px;
}

.custom-row {
  display: flex;
  gap: 0.4rem;
  margin-bottom: 0.5rem;
}

.editor-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
  margin-top: 1.25rem;
}

@media (max-width: 900px) {
  .layout {
    grid-template-columns: minmax(0, 1fr);
  }
  .infobox {
    order: -1;
  }
}
</style>
