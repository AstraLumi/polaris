<template>
  <label class="field date-field-wrap">
    <span>{{ $t(label) }}</span>
    <span class="date-box">
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
      :title="$t('Type the date as digits, day first: 13021456 is 13-02-1456. A year alone is fine; the last four digits are always the year. Type - for a year before 1.')"
      @beforeinput="onBeforeInput"
      @focus="focused = true"
      @blur="onBlur"
    />
      <button
        ref="buttonEl"
        type="button"
        class="date-pick"
        :class="{ 'is-open': open }"
        :aria-label="$t('Pick on the calendar')"
        :title="$t('Pick on the calendar')"
        :aria-expanded="open"
        @click.prevent="toggle"
      >
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <rect x="3.5" y="5" width="17" height="15.5" rx="2.5" />
          <path d="M3.5 10h17M8 3v4M16 3v4" />
        </svg>
      </button>
    </span>
    <Teleport to="body">
      <div
        v-if="open"
        ref="popEl"
        class="date-pop"
        role="dialog"
        :aria-label="$t('Pick a date')"
        :style="popStyle"
        @keydown.esc.stop.prevent="close(true)"
      >
        <div class="pop-head">
          <button type="button" class="pop-step" :aria-label="$t('Previous month')" @click="stepMonth(-1)">‹</button>
          <div class="pop-title">
            <span class="pop-month">{{ monthLabel(view.month) }}</span>
            <input
              class="pop-year"
              type="number"
              :value="view.year"
              :aria-label="$t('Year')"
              @change="setYear($event.target.value)"
            />
          </div>
          <button type="button" class="pop-step" :aria-label="$t('Next month')" @click="stepMonth(1)">›</button>
        </div>
        <div class="pop-months" role="radiogroup" :aria-label="$t('Month')" :style="{ gridTemplateColumns: `repeat(${monthColumns}, 1fr)` }">
          <button
            v-for="m in monthCount"
            :key="m"
            type="button"
            role="radio"
            class="pop-month-pick"
            :class="{ 'is-on': m === view.month }"
            :aria-checked="m === view.month"
            :title="monthLabel(m)"
            @click="view.month = m"
          >
            {{ earth ? monthShort(m) : m }}
          </button>
        </div>
        <div class="pop-days" :style="{ gridTemplateColumns: `repeat(${dayColumns}, 1fr)` }">
          <button
            v-for="d in dayCount"
            :key="d"
            type="button"
            class="pop-day"
            :class="{ 'is-on': isPicked(d), 'is-new-season': seasonStarts.has(d) && d > 1 }"
            :style="seasonStyle(d)"
            :title="seasonTitle(d)"
            @click="pick(d)"
          >
            {{ d }}
          </button>
        </div>
        <p v-if="seasonLine" class="pop-season">{{ seasonLine }}</p>
        <div class="pop-foot">
          <button type="button" class="btn btn-ghost small" @click="pickYear">{{ $t('Just the year') }}</button>
          <button v-if="digits || negative" type="button" class="btn btn-ghost small" @click="clear">{{ $t('Clear') }}</button>
        </div>
      </div>
    </Teleport>
    <p v-if="showProblem" class="date-note is-error">{{ problem }}</p>
    <p v-else-if="legacyText" class="date-note">
      {{ $t('Saved as “{text}”, which can\'t be placed on the timeline. Enter a year to replace it.', { text: legacyText }) }}
    </p>
    <p v-else-if="hint" class="date-note">{{ $t(hint) }}</p>
  </label>
</template>

<script setup>
import { ref, reactive, computed, watch, nextTick, onBeforeUnmount } from 'vue'
import { calendar, parseStoryDate, formatStoryDate, dayColumns as columnsFor } from '../calendar'
import { seasonCycle, monthSeasons } from '../seasons'
import { digitsToParts, partsToDigits, displayDigits, parsePasted, MAX_DIGITS } from '../dateDigits'
import { t, tr, locale } from '../i18n'

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
  // A date on Earth: 12 months of up to 31 days, whatever the story's
  // calendar is.
  earth: { type: Boolean, default: false },
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

const shown = computed(() => displayDigits(digits.value, negative.value))

const problem = computed(() => {
  const p = digitsToParts(digits.value, negative.value)
  if (!p) return negative.value ? t('Enter a year.') : ''
  if (p.year === 0) return t('There is no year 0 — the year before 1 is -1.')
  const months = props.earth ? 12 : calendar.months_per_year
  const days = props.earth ? 31 : calendar.days_per_month
  if (p.month !== null && (p.month < 1 || p.month > months)) return t('Month must be 1–{max}.', { max: months })
  if (p.day !== null && (p.day < 1 || p.day > days)) return t('Day must be 1–{max}.', { max: days })
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
        if (digits.value.length < MAX_DIGITS) digits.value += ch
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

// ---- The calendar popover: a month of the story's calendar at a time,
// its seasons tinted; a click picks the day.
const buttonEl = ref(null)
const popEl = ref(null)
const open = ref(false)
const view = reactive({ month: 1, year: 1 })
const popStyle = ref({})

const monthCount = computed(() => (props.earth ? 12 : calendar.months_per_year))
// Earth's months have their own lengths (February given its 29th).
const EARTH_DAYS = [31, 29, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31]
const dayCount = computed(() => (props.earth ? EARTH_DAYS[view.month - 1] : calendar.days_per_month))
// Months in even rows of at most eight (12 in two rows of 6).
const monthColumns = computed(() => Math.ceil(monthCount.value / Math.ceil(monthCount.value / 8)))
const dayColumns = computed(() => columnsFor(dayCount.value))

function earthMonth(m, style) {
  return new Intl.DateTimeFormat(locale.value, { month: style, timeZone: 'UTC' }).format(new Date(Date.UTC(2000, m - 1, 1)))
}
function monthShort(m) {
  return earthMonth(m, 'short')
}
function monthLabel(m) {
  return props.earth ? earthMonth(m, 'long') : t('Month {n}', { n: m })
}

const picked = computed(() => digitsToParts(digits.value, negative.value))
function isPicked(d) {
  const p = picked.value
  return !!p && p.year === view.year && p.month === view.month && p.day === d
}

function toggle() {
  if (open.value) return close()
  const p = picked.value
  view.year = p && p.year ? p.year : 1
  view.month = p && p.month >= 1 && p.month <= monthCount.value ? p.month : 1
  open.value = true
  nextTick(place)
}

function close(refocus = false) {
  open.value = false
  if (refocus) inputEl.value?.focus()
}

// Under the box, or above it when there's no room below; kept on screen.
function place() {
  const box = inputEl.value?.getBoundingClientRect()
  const pop = popEl.value
  if (!box || !pop) return
  const w = pop.offsetWidth
  const h = pop.offsetHeight
  const below = box.bottom + 6
  const top = below + h > window.innerHeight - 8 && box.top - h - 6 > 8 ? box.top - h - 6 : below
  const left = Math.min(Math.max(8, box.left), window.innerWidth - w - 8)
  popStyle.value = { top: `${Math.max(8, top)}px`, left: `${left}px` }
}

// There is no year 0: stepping back from 1 lands on -1.
function stepMonth(by) {
  let m = view.month + by
  let y = view.year
  if (m < 1) {
    m = monthCount.value
    y = y === 1 ? -1 : y - 1
  } else if (m > monthCount.value) {
    m = 1
    y = y === -1 ? 1 : y + 1
  }
  view.month = m
  view.year = y
}

function setYear(text) {
  const y = parseInt(text, 10)
  if (Number.isFinite(y)) view.year = y === 0 ? 1 : y
}

function emitText(text) {
  load(text)
  lastEmitted = text
  emit('update:modelValue', text)
}
function pick(d) {
  emitText(formatStoryDate({ day: d, month: view.month, year: view.year }))
  close(true)
}
function pickYear() {
  emitText(String(view.year))
  close(true)
}
function clear() {
  emitText('')
  close(true)
}

// The story's seasons (not on Earth dates) tint the days they cover, in
// their own colours.
const cycle = computed(() => (props.earth ? null : seasonCycle(calendar.seasons, calendar)))
const runs = computed(() => (cycle.value ? monthSeasons(view.year, view.month, cycle.value, calendar) : []))
const seasonStarts = computed(() => new Set(runs.value.map((r) => r.from)))
const SEASON_HUES = [205, 130, 45, 15, 280, 170, 330, 95]
function runOf(d) {
  return runs.value.find((r) => d >= r.from && d <= r.to)
}
function seasonStyle(d) {
  const r = runOf(d)
  if (!r) return null
  if (r.season.color) return { '--season': `color-mix(in srgb, ${r.season.color} 22%, transparent)` }
  const i = (calendar.seasons.seasons || []).indexOf(r.season)
  return { '--season': `hsl(${SEASON_HUES[Math.max(0, i) % SEASON_HUES.length]} 55% 60% / 0.16)` }
}
function seasonTitle(d) {
  return runOf(d)?.season.name || ''
}
const seasonLine = computed(() =>
  runs.value.map((r) => (runs.value.length > 1 ? `${r.season.name} ${r.from}–${r.to}` : r.season.name)).join(' · '),
)

// Closes on a click elsewhere, and follows the box when the page moves.
function onPointer(e) {
  if (popEl.value?.contains(e.target) || buttonEl.value?.contains(e.target)) return
  close()
}
function onMove() {
  place()
}
function listen(on) {
  const act = on ? 'addEventListener' : 'removeEventListener'
  document[act]('pointerdown', onPointer, true)
  window[act]('scroll', onMove, true)
  window[act]('resize', onMove)
}
watch(open, listen)
watch([() => view.month, () => view.year], () => nextTick(place))
onBeforeUnmount(() => listen(false))
</script>

<style scoped>
.date-box {
  position: relative;
  display: block;
}

.date-input {
  width: 100%;
  font-variant-numeric: tabular-nums;
  letter-spacing: 0.02em;
  padding-right: 2.6rem;
}

.date-pick {
  position: absolute;
  right: 0.3rem;
  top: 50%;
  transform: translateY(-50%);
  display: grid;
  place-items: center;
  width: 2rem;
  height: 2rem;
  border-radius: 7px;
  border: none;
  background: transparent;
  color: var(--text-faint);
  cursor: pointer;
}

.date-pick svg {
  width: 1.05rem;
  height: 1.05rem;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.8;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.date-pick:hover,
.date-pick.is-open {
  color: var(--accent);
  background: var(--accent-soft);
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

<!-- The popover is teleported into <body>, so its styles aren't scoped. -->
<style>
.date-pop {
  position: fixed;
  z-index: 3000;
  width: min(20rem, calc(100vw - 16px));
  padding: 0.75rem;
  background: var(--surface);
  border: 1px solid var(--glass-border);
  border-radius: 14px;
  box-shadow: 0 18px 40px rgba(0, 0, 0, 0.45);
  font-family: 'Manrope', sans-serif;
  color: var(--text-primary);
}

.date-pop .pop-head {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  margin-bottom: 0.55rem;
}

.date-pop .pop-step {
  width: 2rem;
  height: 2rem;
  border-radius: 8px;
  border: 1px solid var(--glass-border);
  background: transparent;
  color: var(--text-primary);
  font-size: 1.1rem;
  cursor: pointer;
}

.date-pop .pop-step:hover {
  background: var(--accent-soft);
}

.date-pop .pop-title {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.45rem;
  font-family: 'Fraunces', serif;
  font-size: 1rem;
}

.date-pop .pop-year {
  width: 5.2rem;
  min-height: 2rem;
  padding: 0 0.4rem;
  text-align: center;
  font-variant-numeric: tabular-nums;
  font-family: 'Manrope', sans-serif;
}

.date-pop .pop-months {
  display: grid;
  gap: 0.2rem;
  margin-bottom: 0.55rem;
  padding-bottom: 0.55rem;
  border-bottom: 1px solid var(--glass-border);
}

.date-pop .pop-month-pick {
  min-width: 1.85rem;
  height: 1.6rem;
  padding: 0 0.3rem;
  border-radius: 6px;
  border: 1px solid transparent;
  background: transparent;
  color: var(--text-muted);
  font: inherit;
  font-size: 0.72rem;
  font-variant-numeric: tabular-nums;
  cursor: pointer;
}

.date-pop .pop-month-pick:hover {
  color: var(--text-primary);
  background: rgba(255, 255, 255, 0.05);
}

.date-pop .pop-month-pick.is-on {
  color: var(--text-primary);
  border-color: var(--accent);
  background: var(--accent-soft);
}

.date-pop .pop-days {
  display: grid;
  gap: 0.2rem;
}

.date-pop .pop-day {
  aspect-ratio: 1;
  min-height: 1.6rem;
  border-radius: 6px;
  border: 1px solid transparent;
  background: var(--season, rgba(255, 255, 255, 0.025));
  color: var(--text-primary);
  font: inherit;
  font-size: 0.75rem;
  font-variant-numeric: tabular-nums;
  cursor: pointer;
}

.date-pop .pop-day.is-new-season {
  box-shadow: inset 2px 0 0 color-mix(in srgb, var(--text-primary) 40%, transparent);
}

.date-pop .pop-day:hover {
  border-color: var(--glass-border);
  background: rgba(255, 255, 255, 0.08);
}

.date-pop .pop-day.is-on {
  background: var(--accent);
  color: var(--accent-text);
  font-weight: 700;
}

.date-pop .pop-season {
  margin: 0.5rem 0 0;
  font-size: 0.74rem;
  color: var(--text-muted);
}

.date-pop .pop-foot {
  display: flex;
  justify-content: space-between;
  gap: 0.4rem;
  margin-top: 0.6rem;
}
</style>
