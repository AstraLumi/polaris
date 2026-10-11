<template>
  <label class="field date-field-wrap">
    <span>{{ $t(label) }}</span>
    <input
      ref="inputEl"
      class="date-input"
      :class="{ 'is-invalid': showProblem }"
      type="text"
      inputmode="numeric"
      autocomplete="off"
      spellcheck="false"
      :value="shown"
      :placeholder="$t('DD-MM-YYYY')"
      :title="$t('Digits fill in from the right: the last four are the year, then the month, then the day. Type - for a year before 1.')"
      @beforeinput="onBeforeInput"
      @focus="focused = true"
      @blur="onBlur"
    />
    <p v-if="showProblem" class="date-note is-error">{{ problem }}</p>
    <p v-else-if="legacyText" class="date-note">
      {{ $t('Saved as “{text}”, which can\'t be placed on the timeline. Enter a year to replace it.', { text: legacyText }) }}
    </p>
    <p v-else-if="hint" class="date-note">{{ $t(hint) }}</p>
  </label>
</template>

<script setup>
import { ref, computed, watch, nextTick } from 'vue'
import { calendar, parseStoryDate, formatStoryDate } from '../calendar'
import { digitsToParts, partsToDigits, displayDigits, parsePasted, MAX_DIGITS } from '../dateDigits'
import { t, tr } from '../i18n'

// One box for a story date, typed as digits that fill in from the right
// (dateDigits.js): 1456 is the year 1456, 21456 the 2nd month of it, 3021456
// the 3rd of that month. Backspace takes the last digit off, "-" switches to
// a year before 1, and a whole date can be pasted.
//
// v-model is the canonical text ("DD-MM-YYYY", year may be negative) or ''.
// Day and month are optional: a year alone is saved as the 1st of its 1st
// month.
const props = defineProps({
  modelValue: { type: String, default: '' },
  label: { type: String, default: tr('Date') },
  hint: { type: String, default: '' },
})
const emit = defineEmits(['update:modelValue'])

const inputEl = ref(null)
const digits = ref('')
const negative = ref(false)
const legacyText = ref('')
const focused = ref(false)
let lastEmitted = null

function load(text) {
  legacyText.value = ''
  negative.value = false
  digits.value = ''
  if (!text) return
  const d = parseStoryDate(text)
  if (!d) {
    // Old free text: keep it untouched until a real date is typed.
    legacyText.value = text
    return
  }
  // A year typed on its own is shown that way again.
  const bare = !/^\s*\d{1,3}-\d{1,3}-/.test(text)
  digits.value = partsToDigits(d, bare)
  negative.value = d.year < 0
}

watch(
  () => props.modelValue,
  (text) => {
    if (text !== lastEmitted) load(text)
  },
  { immediate: true },
)

const shown = computed(() => displayDigits(digits.value, negative.value, { day: t('DD'), month: t('MM') }))

const problem = computed(() => {
  const p = digitsToParts(digits.value, negative.value)
  if (!p) return negative.value ? t('Enter a year.') : ''
  if (p.year === 0) return t('There is no year 0 — the year before 1 is -1.')
  if (p.month !== null && (p.month < 1 || p.month > calendar.months_per_year))
    return t('Month must be 1–{max}.', { max: calendar.months_per_year })
  if (p.day !== null && (p.day < 1 || p.day > calendar.days_per_month))
    return t('Day must be 1–{max}.', { max: calendar.days_per_month })
  return ''
})
// Half-typed dates are often "wrong" on the way (21 as a month), so problems
// only show once the box is left.
const showProblem = computed(() => !focused.value && !!problem.value)

function commit() {
  const p = digitsToParts(digits.value, negative.value)
  let out = ''
  if (p && !problem.value) {
    out = formatStoryDate({ day: p.day ?? 1, month: p.month ?? 1, year: p.year })
    // A year alone keeps that shape, so it shows up the same way again.
    if (p.day === null && p.month === null) out = String(p.year)
  }
  if (digits.value || negative.value) legacyText.value = ''
  lastEmitted = out
  emit('update:modelValue', out)
}

async function refresh() {
  commit()
  await nextTick()
  const el = inputEl.value
  if (!el) return
  el.value = shown.value // the browser's own edit was cancelled
  el.setSelectionRange(el.value.length, el.value.length)
}

function onBeforeInput(e) {
  const el = e.target
  e.preventDefault()
  const everything = el.value.length > 0 && el.selectionStart === 0 && el.selectionEnd === el.value.length
  const type = e.inputType

  if (type.startsWith('insert')) {
    const text = e.data ?? e.dataTransfer?.getData('text/plain') ?? ''
    if (type !== 'insertText') {
      const pasted = parsePasted(text)
      if (pasted) {
        digits.value = pasted.digits
        negative.value = pasted.negative
        return refresh()
      }
    }
    if (everything) {
      digits.value = ''
      negative.value = false
    }
    for (const ch of text) {
      if (ch >= '0' && ch <= '9') {
        if (digits.value.length < MAX_DIGITS && !(digits.value === '' && ch === '0')) digits.value += ch
      } else if (ch === '-' && type === 'insertText') {
        negative.value = !negative.value
      }
    }
    return refresh()
  }

  if (type === 'deleteContentBackward' && !everything) {
    digits.value = digits.value.slice(0, -1)
    if (!digits.value) negative.value = false
    return refresh()
  }
  if (type.startsWith('delete')) {
    // Delete, cutting, deleting a word or everything selected: start over.
    digits.value = ''
    negative.value = false
    return refresh()
  }
}

function onBlur() {
  focused.value = false
}
</script>

<style scoped>
.date-input {
  font-variant-numeric: tabular-nums;
  letter-spacing: 0.02em;
}

.date-input.is-invalid {
  border-color: var(--danger-text);
}

.date-note {
  margin: 0.15rem 0 0;
  font-size: 0.74rem;
  color: var(--text-faint);
}

.date-note.is-error {
  color: var(--danger-text);
}
</style>
