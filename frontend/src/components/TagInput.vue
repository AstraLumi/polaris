<template>
  <div class="field tag-field">
    <span>{{ $t(label) }}</span>
    <div class="tag-box" @click="inputEl && inputEl.focus()">
      <span v-for="t in modelValue" :key="t" class="tag-chip">
        #{{ t }}
        <button type="button" class="chip-x" :aria-label="$t('Remove {name}', { name: t })" @click.stop="remove(t)">×</button>
      </span>
      <input
        ref="inputEl"
        v-model="draft"
        type="text"
        :list="listId"
        :placeholder="modelValue.length ? '' : $t(placeholder)"
        @keydown.enter.prevent="commit"
        @keydown="onKey"
        @blur="commit"
      />
      <datalist :id="listId">
        <option v-for="s in suggestionList" :key="s" :value="s" />
      </datalist>
    </div>
    <p v-if="hint" class="tag-hint">{{ $t(hint) }}</p>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { tr } from '../i18n'

// Free-form tags as chips. Enter, comma or leaving the box adds one;
// Backspace on an empty box removes the last. Duplicates (ignoring case)
// are dropped. `suggestions` are tags already used elsewhere, offered so
// the same spelling gets reused.
const props = defineProps({
  modelValue: { type: Array, default: () => [] },
  label: { type: String, default: tr('Tags') },
  placeholder: { type: String, default: tr('Type a tag, press Enter') },
  hint: { type: String, default: '' },
  suggestions: { type: Array, default: () => [] },
  listId: { type: String, default: 'tag-suggestions' },
})
const emit = defineEmits(['update:modelValue'])

const draft = ref('')
const inputEl = ref(null)

const suggestionList = computed(() => {
  const have = new Set(props.modelValue.map((t) => t.toLowerCase()))
  return props.suggestions.filter((s) => !have.has(s.toLowerCase()))
})

function clean(raw) {
  return raw.replace(/^#+/, '').trim().replace(/\s+/g, ' ')
}

function add(raw) {
  const tag = clean(raw)
  if (!tag) return
  const existing = props.suggestions.find((s) => s.toLowerCase() === tag.toLowerCase())
  const final = existing || tag
  if (props.modelValue.some((t) => t.toLowerCase() === final.toLowerCase())) return
  emit('update:modelValue', [...props.modelValue, final])
}

function commit() {
  if (draft.value.trim()) add(draft.value)
  draft.value = ''
}

function remove(tag) {
  emit('update:modelValue', props.modelValue.filter((t) => t !== tag))
}

function onKey(e) {
  if (e.key === ',') {
    e.preventDefault()
    commit()
  } else if (e.key === 'Backspace' && !draft.value && props.modelValue.length) {
    remove(props.modelValue[props.modelValue.length - 1])
  }
}
</script>

<style scoped>
.tag-box {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
  align-items: center;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--glass-border);
  border-radius: 8px;
  padding: 0.4rem 0.5rem;
  min-height: 2.5rem;
  cursor: text;
}

.tag-box:focus-within {
  border-color: var(--accent);
}

.tag-box input {
  flex: 1;
  min-width: 8rem;
  width: auto;
  background: transparent;
  border: none;
  padding: 0.15rem 0.2rem;
}

.tag-box input:focus {
  outline: none;
}

/* The suggestion list still works; just hide the browser's dropdown arrow. */
.tag-box input::-webkit-calendar-picker-indicator {
  display: none !important;
}

.tag-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  background: var(--accent-soft);
  color: var(--accent);
  border: 1px solid color-mix(in srgb, var(--accent) 30%, transparent);
  border-radius: 999px;
  padding: 0.15rem 0.35rem 0.15rem 0.6rem;
  font-size: 0.78rem;
  font-weight: 600;
}

.chip-x {
  background: transparent;
  border: none;
  color: inherit;
  cursor: pointer;
  font-size: 0.95rem;
  line-height: 1;
  padding: 0 0.15rem;
  opacity: 0.7;
}

.chip-x:hover {
  opacity: 1;
}

.tag-hint {
  margin: 0.2rem 0 0;
  font-size: 0.74rem;
  color: var(--text-faint);
}
</style>
