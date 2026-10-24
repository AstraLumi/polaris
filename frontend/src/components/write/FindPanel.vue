<template>
  <div class="find-panel glass-panel panel-solid" role="search" :aria-label="$t('Find and replace')" @keydown.esc.stop.prevent="$emit('close')">
    <div class="find-row">
      <input
        ref="queryEl"
        v-model="query"
        type="search"
        :placeholder="$t('Find')"
        :aria-label="$t('Find')"
        @keydown.enter.prevent="step($event.shiftKey ? -1 : 1)"
      />
      <span class="count" aria-live="polite">{{ countLabel }}</span>
      <button type="button" class="btn btn-ghost small icon" :title="$t('Previous (Shift+Enter)')" :aria-label="$t('Previous')" :disabled="!total" @click="step(-1)">↑</button>
      <button type="button" class="btn btn-ghost small icon" :title="$t('Next (Enter)')" :aria-label="$t('Next')" :disabled="!total" @click="step(1)">↓</button>
      <button type="button" class="btn btn-ghost small icon" :aria-label="$t('Close')" @click="$emit('close')">✕</button>
    </div>
    <div class="find-row">
      <input
        ref="replaceEl"
        v-model="replacement"
        type="text"
        :placeholder="$t('Replace with')"
        :aria-label="$t('Replace with')"
        @keydown.enter.prevent="replaceOne"
      />
      <button type="button" class="btn btn-ghost small" :disabled="!total" @click="replaceOne">{{ $t('Replace') }}</button>
      <button type="button" class="btn btn-ghost small" :disabled="!total" @click="replaceEvery">{{ $t('Replace all') }}</button>
    </div>
    <div class="find-row opts">
      <label><input v-model="matchCase" type="checkbox" /> {{ $t('Match case') }}</label>
      <label><input v-model="wholeWord" type="checkbox" /> {{ $t('Whole word') }}</label>
      <span v-if="note" class="note">{{ note }}</span>
    </div>
  </div>
</template>

<script setup>
// Find and replace in the open chapter (Ctrl+F, Ctrl+H). Matches are
// painted in the text; Enter goes to the next one.
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { t, tn } from '../../i18n'
import { setSearch, clearSearch, findState, findStep, replaceCurrent, replaceAll } from '../../editor/findPlugin.js'

const props = defineProps({
  view: { type: Object, required: true },
  state: { type: Object, required: true }, // the editor state, so counts follow it
  replace: { type: Boolean, default: false }, // opened with Ctrl+H
  initial: { type: String, default: '' },
})
defineEmits(['close'])

const query = ref(props.initial)
const replacement = ref('')
const matchCase = ref(false)
const wholeWord = ref(false)
const note = ref('')
const queryEl = ref(null)
const replaceEl = ref(null)

const found = computed(() => findState(props.state))
const total = computed(() => found.value.matches.length)
const countLabel = computed(() => {
  if (!query.value) return ''
  if (!total.value) return t('No matches')
  return t('{i} of {n}', { i: found.value.current + 1, n: total.value })
})

watch([query, matchCase, wholeWord], () => {
  note.value = ''
  setSearch(props.view, query.value, { matchCase: matchCase.value, wholeWord: wholeWord.value })
})

const step = (dir) => findStep(props.view, dir)
function replaceOne() {
  replaceCurrent(props.view, replacement.value)
}
function replaceEvery() {
  const n = replaceAll(props.view, replacement.value)
  note.value = tn('Replaced {n} time.', 'Replaced {n} times.', n)
  // The buttons are disabled now that nothing matches: keep the focus here.
  focus('query')
}

function focus(which) {
  nextTick(() => {
    const el = which === 'replace' ? replaceEl.value : queryEl.value
    el?.focus()
    el?.select?.()
  })
}
defineExpose({ focus })

onMounted(() => {
  if (query.value) setSearch(props.view, query.value, { matchCase: matchCase.value, wholeWord: wholeWord.value })
  focus(props.replace && query.value ? 'replace' : 'query')
})
onBeforeUnmount(() => clearSearch(props.view))
</script>

<style scoped>
.find-panel {
  position: absolute;
  top: 0.6rem;
  right: 1rem;
  z-index: 5;
  width: min(25rem, calc(100% - 2rem));
  padding: 0.55rem 0.6rem;
  display: grid;
  gap: 0.4rem;
}

.find-row {
  display: flex;
  align-items: center;
  gap: 0.3rem;
}

.find-row input[type='search'],
.find-row input[type='text'] {
  flex: 1;
  min-width: 0;
  min-height: 2.1rem;
  padding: 0.3rem 0.6rem;
}

.count {
  font-size: 0.75rem;
  color: var(--text-faint);
  white-space: nowrap;
  min-width: 3.5rem;
  text-align: right;
}

.icon {
  min-width: 2rem;
  padding-inline: 0.4rem;
}

.opts {
  gap: 0.9rem;
  font-size: 0.8rem;
  color: var(--text-muted);
}

.opts label {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  cursor: pointer;
}

.note {
  margin-left: auto;
  color: var(--accent);
}
</style>
