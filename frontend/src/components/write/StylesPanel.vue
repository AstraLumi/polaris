<template>
  <aside class="side-panel glass-panel panel-solid styles-panel" :aria-label="$t('Styles')">
    <header class="side-head">
      <h2>{{ $t('Styles') }}</h2>
      <button type="button" class="btn btn-ghost small" :aria-label="$t('Close')" @click="$emit('close')">✕</button>
    </header>

    <ul class="style-list">
      <li v-for="s in paragraphStyles" :key="s.id">
        <button
          type="button"
          class="style-row"
          :class="{ 'is-picked': s.id === pickedId, 'is-here': s.id === currentId }"
          :title="s.id === currentId ? $t('The style where the cursor is') : ''"
          @click="pickedId = s.id"
          @dblclick="$emit('apply', s.id)"
        >
          <span class="style-name" :style="previewStyle(s.id)">{{ nameOf(s) }}</span>
          <span v-if="s.id === currentId" class="here-dot" aria-hidden="true">●</span>
        </button>
      </li>
    </ul>
    <template v-if="characterStyles.length">
      <h3 class="list-head">{{ $t('Character styles') }}</h3>
      <ul class="style-list">
        <li v-for="s in characterStyles" :key="s.id">
          <button
            type="button"
            class="style-row"
            :class="{ 'is-picked': s.id === pickedId, 'is-here': s.id === currentCharId }"
            :title="s.id === currentCharId ? $t('The style where the cursor is') : ''"
            @click="pickedId = s.id"
            @dblclick="$emit('apply', s.id)"
          >
            <span class="style-name" :style="previewChar(s.id)">{{ nameOf(s) }}</span>
            <span v-if="s.id === currentCharId" class="here-dot" aria-hidden="true">●</span>
          </button>
        </li>
      </ul>
    </template>

    <div class="row-actions">
      <button type="button" class="btn btn-ghost small" @click="addStyle('paragraph')">{{ $t('New style') }}</button>
      <button type="button" class="btn btn-ghost small" @click="addStyle('character')">{{ $t('New character style') }}</button>
      <button type="button" class="btn btn-primary small" :disabled="!picked" @click="$emit('apply', pickedId)">{{ $t('Apply to the selection') }}</button>
    </div>
    <p v-if="isChar" class="hint">{{ $t('A character style is for words inside a paragraph. Pick "No character style" in the toolbar to take it off.') }}</p>

    <form v-if="picked" class="style-form" @submit.prevent>
      <label class="field">
        <span>{{ $t('Name') }}</span>
        <input :value="nameOf(picked)" type="text" maxlength="60" @change="set('name', $event.target.value.trim() || picked.name)" />
      </label>
      <label v-if="picked.id !== 'normal'" class="field">
        <span>{{ $t('Based on') }}</span>
        <select :value="picked.based_on || (isChar ? '' : 'normal')" @change="set('based_on', $event.target.value || (isChar ? null : 'normal'))">
          <option v-if="isChar" value="">{{ $t('None') }}</option>
          <option v-for="b in basedOn" :key="b.id" :value="b.id">{{ nameOf(b) }}</option>
        </select>
      </label>

      <h3>{{ $t('Text') }}</h3>
      <div class="grid">
        <label class="field span-2">
          <span>{{ $t('Font') }}</span>
          <input :value="picked.text?.font || ''" :placeholder="inherited.text.font || ''" type="text" list="write-fonts" @change="setText('font', $event.target.value.trim() || null)" />
        </label>
        <label class="field">
          <span>{{ $t('Size (pt)') }}</span>
          <input :value="picked.text?.size ?? ''" :placeholder="fmt(inherited.text.size)" type="number" min="1" max="400" step="0.5" @change="setText('size', numOrNull($event.target.value))" />
        </label>
        <label v-for="f in toggles" :key="f.key" class="field">
          <span>{{ f.label }}</span>
          <select :value="tri(picked.text?.[f.key])" @change="setText(f.key, untri($event.target.value))">
            <option value="">{{ inheritLabel(inherited.text[f.key] == null ? '' : inherited.text[f.key] ? $t('Yes') : $t('No')) }}</option>
            <option value="yes">{{ $t('Yes') }}</option>
            <option value="no">{{ $t('No') }}</option>
          </select>
        </label>
        <div class="field span-2">
          <span>{{ $t('Text colour') }}</span>
          <div class="color-row">
            <input type="color" :value="picked.text?.color || inherited.text.color || '#000000'" :aria-label="$t('Text colour')" @change="setText('color', $event.target.value)" />
            <button type="button" class="btn btn-ghost small" :disabled="!picked.text?.color" @click="setText('color', null)">{{ $t('Automatic') }}</button>
          </div>
        </div>
      </div>

      <template v-if="!isChar">
      <h3>{{ $t('Paragraph') }}</h3>
      <div class="grid">
        <label class="field span-2">
          <span>{{ $t('Alignment') }}</span>
          <select :value="picked.para?.align || ''" @change="setPara('align', $event.target.value || null)">
            <option value="">{{ inheritLabel(alignName(inherited.para.align)) }}</option>
            <option v-for="a in ALIGNS" :key="a" :value="a">{{ alignName(a) }}</option>
          </select>
        </label>
        <label class="field">
          <span>{{ $t('First line (mm)') }}</span>
          <input :value="mm(picked.para?.indentFirst)" :placeholder="mm(inherited.para.indentFirst)" type="number" step="0.5" min="-100" max="200" @change="setPara('indentFirst', ptOrNull($event.target.value))" />
        </label>
        <label class="field">
          <span>{{ $t('Left indent (mm)') }}</span>
          <input :value="mm(picked.para?.indentLeft)" :placeholder="mm(inherited.para.indentLeft)" type="number" step="0.5" min="-100" max="200" @change="setPara('indentLeft', ptOrNull($event.target.value))" />
        </label>
        <label class="field">
          <span>{{ $t('Right indent (mm)') }}</span>
          <input :value="mm(picked.para?.indentRight)" :placeholder="mm(inherited.para.indentRight)" type="number" step="0.5" min="-100" max="200" @change="setPara('indentRight', ptOrNull($event.target.value))" />
        </label>
        <label class="field">
          <span>{{ $t('Line height') }}</span>
          <input :value="picked.para?.lineHeight ?? ''" :placeholder="fmt(inherited.para.lineHeight)" type="number" step="0.05" min="0.5" max="5" @change="setPara('lineHeight', numOrNull($event.target.value))" />
        </label>
        <label class="field">
          <span>{{ $t('Space before (pt)') }}</span>
          <input :value="picked.para?.spaceBefore ?? ''" :placeholder="fmt(inherited.para.spaceBefore)" type="number" step="1" min="0" max="500" @change="setPara('spaceBefore', numOrNull($event.target.value))" />
        </label>
        <label class="field">
          <span>{{ $t('Space after (pt)') }}</span>
          <input :value="picked.para?.spaceAfter ?? ''" :placeholder="fmt(inherited.para.spaceAfter)" type="number" step="1" min="0" max="500" @change="setPara('spaceAfter', numOrNull($event.target.value))" />
        </label>
      </div>
      <p class="hint">{{ $t('An empty box takes the value of the style it is based on (shown faintly).') }}</p>
      </template>
      <p v-else class="hint">{{ $t('An empty box takes the look of the text around it, or of the style this one is based on.') }}</p>

      <div class="row-actions">
        <button v-if="!picked.builtin" type="button" class="btn btn-danger small" @click="removeStyle">{{ $t('Delete style') }}</button>
      </div>
    </form>
  </aside>
</template>

<script setup>
// A book's styles: pick one to change it (an empty box follows the style it
// is based on), make new ones, apply one to the selected paragraphs.
import { ref, computed, watch } from 'vue'
import { t, tr } from '../../i18n'
import { BUILTIN_NAMES, resolveStyle, resolveCharStyle, newStyleId, basedOnChoices, styleById } from '../../editor/styles.js'
import { mmToPt, ptToMm, num, textDeclarations, charDeclarations } from '../../editor/css.js'

void [tr('Normal'), tr('Heading 1'), tr('Heading 2'), tr('Heading 3'), tr('Title'), tr('Subtitle'), tr('Quote')]

const props = defineProps({
  styles: { type: Array, required: true },
  currentId: { type: String, default: null },
  currentCharId: { type: String, default: null },
})
const emit = defineEmits(['update', 'apply', 'close'])

const ALIGNS = ['left', 'center', 'right', 'justify']

const pickedId = ref(props.currentId || 'normal')
watch(
  () => props.currentId,
  (id) => {
    if (id) pickedId.value = id
  },
)

const paragraphStyles = computed(() => props.styles.filter((s) => (s.kind || 'paragraph') === 'paragraph'))
const characterStyles = computed(() => props.styles.filter((s) => s.kind === 'character'))
const picked = computed(() => styleById(props.styles, pickedId.value))
const isChar = computed(() => picked.value?.kind === 'character')
const basedOn = computed(() => basedOnChoices(props.styles, pickedId.value))
// What an empty box falls back to: the style this one is based on (a
// character style's: only what its base sets, else the text around it).
const inherited = computed(() => {
  if (isChar.value) return { text: resolveCharStyle(props.styles, picked.value.based_on), para: {} }
  return resolveStyle(props.styles, picked.value?.id === 'normal' ? '__none__' : picked.value?.based_on || 'normal')
})
const toggles = computed(() => [
  { key: 'bold', label: t('Bold') },
  { key: 'italic', label: t('Italic') },
  ...(isChar.value ? [{ key: 'underline', label: t('Underline') }, { key: 'strike', label: t('Strikethrough') }, { key: 'smallcaps', label: t('Small caps') }] : []),
])

// Built-in names are shown translated until the writer renames them.
const nameOf = (s) => (s.builtin && s.name === BUILTIN_NAMES[s.id] ? t(s.name) : s.name)
const alignName = (a) => ({ left: t('Left'), center: t('Centre'), right: t('Right'), justify: t('Justified') })[a] || ''
const inheritLabel = (v) => (v ? t('As based on ({value})', { value: v }) : isChar.value ? t('As the text around it') : t('As based on'))
const fmt = (n) => (n == null ? '' : num(n))
const mm = (pt) => (pt == null ? '' : String(ptToMm(pt)))
const numOrNull = (v) => (v === '' || !Number.isFinite(+v) ? null : +v)
const ptOrNull = (v) => (v === '' || !Number.isFinite(+v) ? null : mmToPt(+v))
const tri = (v) => (v == null ? '' : v ? 'yes' : 'no')
const untri = (v) => (v === '' ? null : v === 'yes')

function previewStyle(id) {
  const { text } = resolveStyle(props.styles, id)
  return textDeclarations({ ...text, size: null, color: null }).join('; ')
}
function previewChar(id) {
  return charDeclarations({ ...resolveCharStyle(props.styles, id), size: null, superscript: null, subscript: null }).join('; ')
}

function replace(next) {
  emit('update', props.styles.map((s) => (s.id === next.id ? next : s)))
}
function set(key, value) {
  replace({ ...picked.value, [key]: value })
}
function setText(key, value) {
  replace({ ...picked.value, text: { ...(picked.value.text || {}), [key]: value } })
}
function setPara(key, value) {
  replace({ ...picked.value, para: { ...(picked.value.para || {}), [key]: value } })
}

function addStyle(kind) {
  const name = kind === 'character' ? t('New character style') : t('New style')
  const id = newStyleId(props.styles, name)
  if (kind === 'character') {
    emit('update', [...props.styles, { id, name, kind, based_on: null, builtin: false, text: {} }])
  } else {
    const based = picked.value && (picked.value.kind || 'paragraph') === 'paragraph' ? picked.value.id : 'normal'
    emit('update', [...props.styles, { id, name, kind, based_on: based, builtin: false, para: {}, text: {} }])
  }
  pickedId.value = id
}

function removeStyle() {
  const gone = picked.value
  if (!gone || gone.builtin) return
  const sure = gone.kind === 'character'
    ? t('Delete the character style "{name}"? Text that uses it loses its look.', { name: gone.name })
    : t('Delete the style "{name}"? Paragraphs that use it look like Normal.', { name: gone.name })
  if (!window.confirm(sure)) return
  // Styles based on it are based on what it was based on.
  const next = props.styles.filter((s) => s.id !== gone.id).map((s) => (s.based_on === gone.id ? { ...s, based_on: gone.based_on || (gone.kind === 'character' ? null : 'normal') } : s))
  pickedId.value = 'normal'
  emit('update', next)
}
</script>

<style scoped>
.style-list {
  list-style: none;
  margin: 0 0 0.4rem;
  padding: 0;
  display: grid;
  gap: 0.15rem;
}

.style-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 0.35rem 0.6rem;
  border-radius: 8px;
  border: 1px solid transparent;
  background: none;
  color: var(--text-primary);
  font-size: 0.95rem;
  text-align: left;
  cursor: pointer;
}

.style-row:hover {
  background: var(--glass);
}

.style-row.is-picked {
  border-color: var(--accent);
  background: var(--accent-soft);
}

.style-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.list-head {
  margin: 0.6rem 0 0.3rem;
  font-size: 0.82rem;
  color: var(--text-muted);
  font-weight: 600;
}

.color-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.color-row input[type='color'] {
  width: 2.4rem;
  height: 1.8rem;
  min-height: 0;
  padding: 0;
}

.here-dot {
  color: var(--accent);
  font-size: 0.6rem;
}

.row-actions {
  display: flex;
  gap: 0.4rem;
  flex-wrap: wrap;
  margin: 0.4rem 0 0.8rem;
}

.style-form h3 {
  margin: 0.9rem 0 0.4rem;
  font-size: 0.95rem;
}

.grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.5rem 0.6rem;
}

.grid .span-2 {
  grid-column: span 2;
}

.style-form .field {
  margin: 0;
}

.style-form .field > span {
  font-size: 0.75rem;
}

.hint {
  margin: 0.4rem 0 0;
  font-size: 0.75rem;
  color: var(--text-faint);
  line-height: 1.4;
}
</style>
