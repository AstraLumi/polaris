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
    <p v-if="otherStyles.length" class="hint">{{ $tn('{n} character style from Word is kept but not used yet.', '{n} character styles from Word are kept but not used yet.', otherStyles.length) }}</p>

    <div class="row-actions">
      <button type="button" class="btn btn-ghost small" @click="addStyle">{{ $t('New style') }}</button>
      <button type="button" class="btn btn-primary small" :disabled="!picked" @click="$emit('apply', pickedId)">{{ $t('Apply to the selection') }}</button>
    </div>

    <form v-if="picked" class="style-form" @submit.prevent>
      <label class="field">
        <span>{{ $t('Name') }}</span>
        <input :value="nameOf(picked)" type="text" maxlength="60" @change="set('name', $event.target.value.trim() || picked.name)" />
      </label>
      <label v-if="picked.id !== 'normal'" class="field">
        <span>{{ $t('Based on') }}</span>
        <select :value="picked.based_on || 'normal'" @change="set('based_on', $event.target.value)">
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
        <label class="field">
          <span>{{ $t('Bold') }}</span>
          <select :value="tri(picked.text?.bold)" @change="setText('bold', untri($event.target.value))">
            <option value="">{{ inheritLabel(inherited.text.bold ? $t('Yes') : $t('No')) }}</option>
            <option value="yes">{{ $t('Yes') }}</option>
            <option value="no">{{ $t('No') }}</option>
          </select>
        </label>
        <label class="field">
          <span>{{ $t('Italic') }}</span>
          <select :value="tri(picked.text?.italic)" @change="setText('italic', untri($event.target.value))">
            <option value="">{{ inheritLabel(inherited.text.italic ? $t('Yes') : $t('No')) }}</option>
            <option value="yes">{{ $t('Yes') }}</option>
            <option value="no">{{ $t('No') }}</option>
          </select>
        </label>
      </div>

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
import { BUILTIN_NAMES, resolveStyle, newStyleId, basedOnChoices, styleById } from '../../editor/styles.js'
import { mmToPt, ptToMm, num, textDeclarations } from '../../editor/css.js'

void [tr('Normal'), tr('Heading 1'), tr('Heading 2'), tr('Heading 3'), tr('Title'), tr('Subtitle'), tr('Quote')]

const props = defineProps({
  styles: { type: Array, required: true },
  currentId: { type: String, default: null },
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
const otherStyles = computed(() => props.styles.filter((s) => s.kind && s.kind !== 'paragraph'))
const picked = computed(() => styleById(props.styles, pickedId.value))
const basedOn = computed(() => basedOnChoices(props.styles, pickedId.value))
// What an empty box falls back to: the style this one is based on.
const inherited = computed(() => resolveStyle(props.styles, picked.value?.id === 'normal' ? '__none__' : picked.value?.based_on || 'normal'))

// Built-in names are shown translated until the writer renames them.
const nameOf = (s) => (s.builtin && s.name === BUILTIN_NAMES[s.id] ? t(s.name) : s.name)
const alignName = (a) => ({ left: t('Left'), center: t('Centre'), right: t('Right'), justify: t('Justified') })[a] || ''
const inheritLabel = (v) => (v ? t('As based on ({value})', { value: v }) : t('As based on'))
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

function addStyle() {
  const name = t('New style')
  const id = newStyleId(props.styles, name)
  const based = picked.value && (picked.value.kind || 'paragraph') === 'paragraph' ? picked.value.id : 'normal'
  emit('update', [...props.styles, { id, name, kind: 'paragraph', based_on: based, builtin: false, para: {}, text: {} }])
  pickedId.value = id
}

function removeStyle() {
  const gone = picked.value
  if (!gone || gone.builtin) return
  if (!window.confirm(t('Delete the style "{name}"? Paragraphs that use it look like Normal.', { name: gone.name }))) return
  // Styles based on it are based on what it was based on.
  const next = props.styles.filter((s) => s.id !== gone.id).map((s) => (s.based_on === gone.id ? { ...s, based_on: gone.based_on || 'normal' } : s))
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
