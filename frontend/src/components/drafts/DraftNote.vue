<template>
  <div class="write-page draft-note" :class="{ 'is-dark-paper': darkPaper }">
    <div v-if="editorState" class="note-bar">
      <WriteToolbar :state="editorState" :styles="styles" :exec="exec" lite online />
    </div>
    <p v-if="error" class="error-text note-error">{{ error }}</p>
    <div class="note-scroll">
      <div class="sheet is-online note-sheet">
        <div ref="editorEl" class="note-host"></div>
      </div>
    </div>
    <p class="note-status muted">
      <span>{{ statusText }}</span>
      <span v-if="words">· {{ tn('{n} word', '{n} words', words, { n: words }) }}</span>
    </p>
  </div>
</template>

<script setup>
// A draft note (views/DraftsPage.vue): the writing editor's document with
// the tools that work outside a book (WriteToolbar's `lite`), saved a
// moment after typing stops like a chapter (editor/autosave.js).
import 'prosemirror-view/style/prosemirror.css'
import 'prosemirror-gapcursor/style/gapcursor.css'
import '../../editor/editor.css'
import { ref, shallowRef, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { EditorState } from 'prosemirror-state'
import { EditorView } from 'prosemirror-view'
import { t, tn } from '../../i18n'
import { schema, docFromJSON, docToJSON, emptyDocJSON } from '../../editor/schema.js'
import { editorPlugins } from '../../editor/keymap.js'
import { tablePlugins } from '../../editor/tables.js'
import { pasteIntoEmpty } from '../../editor/commands.js'
import { builtinStyles, styleSheet, styleFonts } from '../../editor/styles.js'
import { loadFonts } from '../../editor/fonts.js'
import { countNode } from '../../editor/wordCount.js'
import { createSaver } from '../../editor/autosave.js'
import { draftNoteApi } from '../../editor/api.js'
import { askConfirm } from '../../confirm'
import WriteToolbar from '../write/WriteToolbar.vue'

const props = defineProps({
  id: { type: Number, required: true },
  // Put the cursor in the text once it is loaded (not for a note just
  // made, whose name is being typed).
  autofocus: { type: Boolean, default: true },
})
const emit = defineEmits(['words'])

// Notes read like a post: left-aligned, a blank line between paragraphs.
const styles = builtinStyles(11, { print: false })
const darkPaper = document.documentElement.dataset.theme !== 'daylight'

const editorEl = ref(null)
const editorState = shallowRef(null)
const error = ref('')
const saveState = ref('saved')
const words = ref(0)
let view = null
let saver = null

const statusText = computed(
  () =>
    ({
      saved: t('Saved'),
      local: t('Not saved yet'),
      saving: t('Saving…'),
      failed: t('Not saved: {why}', { why: saver?.error?.message || '' }),
      conflict: t('Changed somewhere else'),
    })[saveState.value] || '',
)

const styleEl = document.createElement('style')
styleEl.textContent = styleSheet(styles, '.draft-doc')
document.head.appendChild(styleEl)
loadFonts(styleFonts(styles))

const plugins = [...tablePlugins(), ...editorPlugins({ getStyles: () => styles })]
const makeState = (json) => EditorState.create({ schema, doc: docFromJSON(json), plugins })

function exec(cmd) {
  if (!view) return
  cmd(view.state, view.dispatch, view)
  view.focus()
}

function setDoc(json) {
  const st = makeState(json)
  view.updateState(st)
  editorState.value = st
  words.value = countNode(st.doc)
}

let countTimer = 0
function createView() {
  view = new EditorView(editorEl.value, {
    state: makeState(emptyDocJSON()),
    attributes: { class: 'write-doc draft-doc is-online' },
    dispatchTransaction(tr) {
      const st = view.state.apply(tr)
      view.updateState(st)
      editorState.value = st
      if (tr.docChanged) {
        saver?.changed()
        clearTimeout(countTimer)
        countTimer = setTimeout(() => (words.value = countNode(view.state.doc)), 250)
      }
    },
    handlePaste(v, e, slice) {
      const tr = pasteIntoEmpty(v.state, slice)
      if (!tr) return false
      v.dispatch(tr)
      return true
    },
  })
  editorState.value = view.state
}

function startSaver(id, version) {
  saver?.dispose()
  saveState.value = 'saved'
  saver = createSaver({
    getDoc: () => docToJSON(view.state.doc),
    save: (doc, base) => draftNoteApi.save(id, doc, base),
    keep: () => {},
    drop: () => {},
    base: version,
    onState: (s) => {
      if (props.id === id) saveState.value = s
    },
    onSaved: (res) => {
      if (res?.words != null) emit('words', id, res.words)
    },
    onConflict: async (stored) => {
      if (props.id !== id) return
      const loadTheirs = await askConfirm(t('This note was changed somewhere else.'), {
        message: t('Load that version? Cancel keeps what is on this screen and saves it over the other.'),
        confirmLabel: t('Load the other'),
        danger: false,
      })
      if (props.id !== id) return
      if (!loadTheirs) saver.resolved(stored?.version ?? 0, true)
      else {
        setDoc(stored?.doc || emptyDocJSON())
        saver.resolved(stored?.version ?? 0, false)
      }
    },
  })
}

let opening = 0
async function open(id) {
  const token = ++opening
  error.value = ''
  try {
    const note = await draftNoteApi.get(id)
    if (token !== opening) return
    setDoc(note.doc)
    startSaver(id, note.version)
    if (props.autofocus) view.focus()
  } catch (e) {
    if (token === opening) error.value = e.message
  }
}

// Saves what is waiting (leaving the note, the page or the story).
async function flush() {
  if (saver) await saver.flush()
}

watch(
  () => props.id,
  async (id, old) => {
    if (old != null) await flush()
    if (view) open(id)
  },
)

onMounted(() => {
  createView()
  open(props.id)
})

onBeforeUnmount(() => {
  flush()
  saver?.dispose()
  view?.destroy()
  styleEl.remove()
  clearTimeout(countTimer)
})

defineExpose({ flush })
</script>

<style scoped>
.draft-note {
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
  gap: 0.6rem;
}

.note-bar {
  flex: none;
}

.note-scroll {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 0.25rem 0.25rem 2rem;
}

.note-sheet {
  box-sizing: border-box;
  margin: 0 auto;
  background: var(--paper);
  color: var(--ink);
  box-shadow: var(--paper-shadow);
  border-radius: 10px;
  max-width: 46rem;
  padding: 2rem 2.4rem;
  min-height: 60vh;
}

.note-host :deep(.write-doc) {
  min-height: 50vh;
}

.note-status {
  flex: none;
  margin: 0;
  font-size: 0.8rem;
  display: flex;
  gap: 0.4rem;
}

.note-error {
  margin: 0;
}

@media (max-width: 640px) {
  .note-sheet {
    padding: 1.2rem 1rem;
  }
}
</style>
