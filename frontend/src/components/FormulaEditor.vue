<template>
  <div class="formula-editor" :class="{ 'is-readonly': readonly, 'is-invalid': invalid }">
    <div
      ref="box"
      class="formula-line"
      :tabindex="readonly ? -1 : 0"
      role="textbox"
      :aria-label="label"
      :aria-readonly="readonly"
      @keydown="onKey"
      @focus="onFocus"
      @blur="focused = false"
      @mousedown.self="placeCaret(blocks.length)"
    >
      <template v-for="(b, i) in blocks" :key="i">
        <span v-if="focused && caret === i" class="caret" aria-hidden="true"></span>
        <span
          class="block"
          :class="'is-' + blockKind(b)"
          :title="blockTitle(b)"
          @mousedown.prevent="placeCaret(i + 1)"
        >{{ blockText(b) }}</span>
      </template>
      <span v-if="focused && caret === blocks.length" class="caret" aria-hidden="true"></span>
      <span v-if="!blocks.length && !focused" class="empty">{{ placeholder || $t('Empty: always 0') }}</span>
    </div>

    <div v-if="!readonly && toolbar" class="toolbar" @mousedown.prevent>
      <div v-for="group in valueGroups" :key="group.label" class="tool-group">
        <span class="tool-label">{{ group.label }}</span>
        <button
          v-for="v in group.items"
          :key="v.key"
          type="button"
          class="tool is-ref"
          :class="{ 'is-input': v.input }"
          :title="v.title"
          @click="insert({ type: 'ref', key: v.key })"
        >
          {{ v.label }}
        </button>
      </div>
      <div class="tool-group">
        <span class="tool-label">{{ $t('Operators') }}</span>
        <button v-for="o in FORMULA_OPERATORS" :key="o.op" type="button" class="tool is-op" :title="$t(o.about)" @click="insert({ type: 'op', op: o.op })">
          {{ o.label }}
        </button>
        <span class="number-tool">
          <input
            v-model="numberText"
            type="number"
            step="any"
            :placeholder="$t('Number')"
            :aria-label="$t('Number')"
            @mousedown.stop
            @keydown.enter.prevent="addNumber"
          />
          <button type="button" class="tool is-num" :disabled="numberText === '' || numberText === null" @click="addNumber">{{ $t('Add') }}</button>
        </span>
      </div>
      <div class="tool-group">
        <span class="tool-label">{{ $t('Functions') }}</span>
        <button v-for="f in FORMULA_FUNCTIONS" :key="f.name" type="button" class="tool is-fn" :title="$t(f.about)" @click="insert({ type: 'fn', name: f.name })">
          {{ f.name }}(
        </button>
      </div>
      <div class="tool-actions">
        <span class="hint">{{ $t('Click a block to put the cursor after it. Numbers and + − × ÷ ( ) can be typed; Backspace removes.') }}</span>
        <button type="button" class="btn btn-ghost small" :disabled="!blocks.length" @click="clear">{{ $t('Clear') }}</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, nextTick } from 'vue'
import { t } from '../i18n'
import { parseBlocks, blocksToText, FORMULA_OPERATORS, FORMULA_FUNCTIONS } from '../stats'

// A formula as a row of blocks: [Level] × [VIT] + 2. Values, operators and
// functions are added from the toolbar at the cursor; numbers and operators
// can also be typed. The text it edits is what backend/statformula.go reads.
const props = defineProps({
  modelValue: { type: String, default: '' },
  // [{ label, items: [{ key, label, title, input }] }]: what [values] it can add.
  values: { type: Array, default: () => [] },
  // Names for [keys] in the line (anything in values is found on its own).
  names: { type: Object, default: () => ({}) },
  readonly: { type: Boolean, default: false },
  toolbar: { type: Boolean, default: true },
  invalid: { type: Boolean, default: false },
  label: { type: String, default: '' },
  placeholder: { type: String, default: '' },
})
const emit = defineEmits(['update:modelValue', 'focus'])

const box = ref(null)
const blocks = ref(parseBlocks(props.modelValue))
const caret = ref(blocks.value.length)
const focused = ref(false)
const numberText = ref('')

let lastText = props.modelValue
watch(
  () => props.modelValue,
  (text) => {
    if (text === lastText) return
    lastText = text
    blocks.value = parseBlocks(text)
    caret.value = blocks.value.length
  },
)

function commit() {
  lastText = blocksToText(blocks.value)
  emit('update:modelValue', lastText)
}

const valueGroups = computed(() => props.values.filter((g) => g.items.length))
const nameOf = computed(() => {
  const m = { ...props.names }
  for (const g of props.values) for (const v of g.items) m[v.key] = v
  return m
})

const OP_TEXT = Object.fromEntries(FORMULA_OPERATORS.map((o) => [o.op, o.label]))

function blockKind(b) {
  if (b.type === 'ref') return nameOf.value[b.key]?.input ? 'input' : nameOf.value[b.key] ? 'ref' : 'unknown'
  return b.type
}

function blockText(b) {
  if (b.type === 'ref') return nameOf.value[b.key]?.label || b.key
  if (b.type === 'fn') return `${b.name}(`
  if (b.type === 'op') return OP_TEXT[b.op] || b.op
  return b.text
}

function blockTitle(b) {
  if (b.type === 'ref') return nameOf.value[b.key]?.title || t('There is no stat called {name}.', { name: b.key })
  if (b.type === 'fn') return t(FORMULA_FUNCTIONS.find((f) => f.name === b.name)?.about || '')
  if (b.type === 'op') return t(FORMULA_OPERATORS.find((o) => o.op === b.op)?.about || '')
  return ''
}

function onFocus() {
  if (props.readonly) return
  focused.value = true
  emit('focus')
}

function placeCaret(i) {
  if (props.readonly) return
  caret.value = Math.max(0, Math.min(i, blocks.value.length))
  box.value?.focus()
}

function insert(block) {
  blocks.value.splice(caret.value, 0, block)
  caret.value++
  commit()
  nextTick(() => box.value?.focus())
}

function addNumber() {
  const n = Number(numberText.value)
  if (numberText.value === '' || numberText.value === null || !Number.isFinite(n)) return
  if (n < 0) insert({ type: 'op', op: '-' })
  insert({ type: 'num', text: String(Math.abs(n)) })
  numberText.value = ''
}

function clear() {
  blocks.value = []
  caret.value = 0
  commit()
  nextTick(() => box.value?.focus())
}

const TYPED_OPS = { '+': '+', '-': '-', '*': '*', x: '*', '/': '/', '^': '^', '(': '(', ')': ')', ',': ',' }

function onKey(e) {
  if (props.readonly || e.ctrlKey || e.metaKey || e.altKey) return
  const list = blocks.value
  const prev = list[caret.value - 1]
  if (e.key === 'ArrowLeft') caret.value = Math.max(0, caret.value - 1)
  else if (e.key === 'ArrowRight') caret.value = Math.min(list.length, caret.value + 1)
  else if (e.key === 'Home') caret.value = 0
  else if (e.key === 'End') caret.value = list.length
  else if (e.key === 'Backspace') {
    if (!prev) return
    if (prev.type === 'num' && prev.text.length > 1) prev.text = prev.text.slice(0, -1)
    else {
      list.splice(caret.value - 1, 1)
      caret.value--
    }
    commit()
  } else if (e.key === 'Delete') {
    if (caret.value >= list.length) return
    list.splice(caret.value, 1)
    commit()
  } else if (/^[0-9.]$/.test(e.key)) {
    if (prev?.type === 'num') {
      if (e.key === '.' && prev.text.includes('.')) return e.preventDefault()
      prev.text += e.key
      commit()
    } else insert({ type: 'num', text: e.key === '.' ? '0.' : e.key })
  } else if (TYPED_OPS[e.key]) {
    insert({ type: 'op', op: TYPED_OPS[e.key] })
  } else return
  e.preventDefault()
}
</script>

<style scoped>
.formula-line {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.3rem;
  min-height: var(--control-h);
  padding: 0.4rem 0.55rem;
  background: rgba(255, 255, 255, 0.045);
  border: 1px solid var(--glass-border);
  border-radius: 8px;
  cursor: text;
  transition: border-color 0.12s ease, box-shadow 0.12s ease;
}

.formula-line:focus {
  outline: none;
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft);
}

.is-invalid .formula-line {
  border-color: color-mix(in srgb, var(--danger) 60%, transparent);
}

.is-readonly .formula-line {
  background: transparent;
  border-color: transparent;
  padding: 0;
  min-height: 0;
  cursor: inherit;
}

.block {
  display: inline-flex;
  align-items: center;
  padding: 0.12rem 0.45rem;
  border-radius: 6px;
  font-size: 0.82rem;
  line-height: 1.4;
  color: var(--text-primary);
  white-space: nowrap;
  user-select: none;
}

.block.is-ref,
.tool.is-ref {
  background: var(--accent-soft);
  border: 1px solid color-mix(in srgb, var(--accent) 40%, transparent);
  color: var(--accent);
  font-weight: 700;
}

.block.is-input,
.tool.is-input {
  background: color-mix(in srgb, var(--text-primary) 8%, transparent);
  border: 1px solid var(--glass-border);
  color: var(--text-primary);
  font-weight: 700;
}

.block.is-unknown,
.block.is-bad {
  background: color-mix(in srgb, var(--danger) 18%, transparent);
  border: 1px solid color-mix(in srgb, var(--danger) 45%, transparent);
  color: var(--danger-text);
}

.block.is-fn {
  font-family: ui-monospace, 'Cascadia Mono', Consolas, monospace;
  color: var(--text-muted);
  padding-right: 0.15rem;
}

.block.is-op {
  color: var(--text-muted);
  font-weight: 700;
  padding: 0.12rem 0.15rem;
}

.block.is-num {
  font-variant-numeric: tabular-nums;
  padding: 0.12rem 0.2rem;
}

.caret {
  width: 2px;
  align-self: stretch;
  margin: 0.15rem -0.15rem;
  background: var(--accent);
  animation: blink 1s steps(1) infinite;
}

@keyframes blink {
  50% {
    opacity: 0;
  }
}

.empty {
  font-size: 0.82rem;
  color: var(--text-faint);
  font-style: italic;
}

.toolbar {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  margin-top: 0.6rem;
  padding: 0.75rem;
  border: 1px solid var(--glass-border);
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.02);
}

.tool-group {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.3rem;
}

.tool-label {
  flex: 0 0 100%;
  font-size: 0.68rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-faint);
}

.tool {
  min-width: 2rem;
  padding: 0.2rem 0.55rem;
  border-radius: 6px;
  border: 1px solid var(--glass-border);
  background: transparent;
  color: var(--text-primary);
  font-family: 'Manrope', sans-serif;
  font-size: 0.8rem;
  cursor: pointer;
}

.tool:not(:disabled):hover {
  border-color: var(--accent);
}

.tool:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.tool.is-op {
  font-weight: 700;
}

.tool.is-fn {
  font-family: ui-monospace, 'Cascadia Mono', Consolas, monospace;
}

.number-tool {
  display: inline-flex;
  gap: 0.3rem;
  margin-left: 0.4rem;
}

.number-tool input {
  width: 6.5rem;
  min-height: 0;
  padding: 0.2rem 0.5rem;
  font-size: 0.8rem;
}

.tool-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.hint {
  font-size: 0.74rem;
  color: var(--text-faint);
}
</style>
