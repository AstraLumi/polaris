<template>
  <div class="field date-field-wrap">
    <span>{{ $t(label) }}</span>
    <div class="date-inputs">
      <input
        v-model="day"
        type="number"
        inputmode="numeric"
        step="1"
        :min="1"
        :max="calendar.days_per_month"
        :placeholder="$t('DD')"
        :aria-label="$t('Day')"
        @input="commit"
      />
      <input
        v-model="month"
        type="number"
        inputmode="numeric"
        step="1"
        :min="1"
        :max="calendar.months_per_year"
        :placeholder="$t('MM')"
        :aria-label="$t('Month')"
        @input="commit"
      />
      <input
        v-model="year"
        type="number"
        inputmode="numeric"
        step="1"
        class="year-input"
        :placeholder="$t('Year')"
        :aria-label="$t('Year')"
        @input="commit"
      />
    </div>
    <p v-if="problem" class="date-note is-error">{{ problem }}</p>
    <p v-else-if="legacyText" class="date-note">
      {{ $t('Saved as “{text}”, which can\'t be placed on the timeline. Enter a year to replace it.', { text: legacyText }) }}
    </p>
    <p v-else-if="hint" class="date-note">{{ $t(hint) }}</p>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { calendar, parseStoryDate, formatStoryDate } from '../calendar'
import { t, tr } from '../i18n'

// v-model is the canonical text ("DD-MM-YYYY", year may be negative) or ''.
// Day and month are optional: leave them blank and the date is saved as the
// 1st of the 1st month of that year.
const props = defineProps({
  modelValue: { type: String, default: '' },
  label: { type: String, default: tr('Date') },
  hint: { type: String, default: '' },
})
const emit = defineEmits(['update:modelValue'])

const day = ref('')
const month = ref('')
const year = ref('')
const legacyText = ref('')
let lastEmitted = null

function load(text) {
  legacyText.value = ''
  if (!text) {
    day.value = month.value = year.value = ''
    return
  }
  const d = parseStoryDate(text)
  if (!d) {
    // Old free text: keep it untouched until a real year is typed.
    legacyText.value = text
    day.value = month.value = year.value = ''
    return
  }
  // A bare year was stored as 01-01-YYYY; show it the way it was typed.
  const bare = d.day === 1 && d.month === 1 && !/^\d{1,3}-\d{1,3}-/.test(text.trim())
  day.value = bare ? '' : d.day
  month.value = bare ? '' : d.month
  year.value = d.year
}

watch(
  () => props.modelValue,
  (text) => {
    if (text !== lastEmitted) load(text)
  },
  { immediate: true },
)

function toInt(v) {
  if (v === '' || v === null) return null
  const n = Number(v)
  return Number.isInteger(n) ? n : NaN
}

const problem = computed(() => {
  const d = toInt(day.value)
  const m = toInt(month.value)
  const y = toInt(year.value)
  if (Number.isNaN(d) || Number.isNaN(m) || Number.isNaN(y)) return t('Use whole numbers.')
  if (y === 0) return t('There is no year 0 — the year before 1 is -1.')
  if ((d !== null || m !== null) && y === null) return t('Enter a year.')
  if (m !== null && (m < 1 || m > calendar.months_per_year))
    return t('Month must be 1–{max}.', { max: calendar.months_per_year })
  if (d !== null && (d < 1 || d > calendar.days_per_month))
    return t('Day must be 1–{max}.', { max: calendar.days_per_month })
  return ''
})

function commit() {
  legacyText.value = ''
  const y = toInt(year.value)
  let out = ''
  if (!problem.value && y !== null) {
    out = formatStoryDate({
      day: toInt(day.value) ?? 1,
      month: toInt(month.value) ?? 1,
      year: y,
    })
  }
  lastEmitted = out
  emit('update:modelValue', out)
}
</script>

<style scoped>
.date-inputs {
  display: grid;
  grid-template-columns: 3.6rem 3.6rem minmax(4.6rem, 1fr);
  gap: 0.5rem;
}

.date-inputs input {
  min-width: 0;
  padding-left: 0.6rem;
  padding-right: 0.4rem;
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
