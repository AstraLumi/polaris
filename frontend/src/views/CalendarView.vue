<template>
  <div class="page is-wide calendar-page">
    <header class="page-header">
      <div>
        <h1>{{ $t('Calendar') }}</h1>
        <p class="page-sub">
          {{ $t('{months} months of {days} days', { months: calendar.months_per_year, days: calendar.days_per_month }) }}
          <RouterLink to="/settings">{{ $t('change') }}</RouterLink>
        </p>
      </div>
      <div class="head-actions">
        <RouterLink to="/timeline" class="btn btn-ghost">{{ $t('Timeline') }}</RouterLink>
        <RouterLink :to="{ path: '/events/new', query: { date: newEventDate } }" class="btn btn-primary">{{ $t('New event') }}</RouterLink>
      </div>
    </header>

    <p v-if="loadError" class="error-banner">{{ loadError }}</p>
    <p v-else-if="!data" class="loading-hint">{{ $t('Loading…') }}</p>

    <template v-else>
      <section class="nav-bar glass-panel panel-solid">
        <div class="nav-group">
          <button type="button" class="btn btn-ghost small" :disabled="!prevPoint" :title="$t('The closest earlier month with something in it')" @click="goPoint(prevPoint)">
            ⇤ {{ $t('Previous') }}
          </button>
          <button type="button" class="btn btn-ghost small" :aria-label="$t('Previous month')" :title="$t('Previous month')" @click="shift(-1)">‹</button>
          <h2 class="month-title">{{ $t('Month {n}', { n: month }) }}, {{ yearLabel }}</h2>
          <button type="button" class="btn btn-ghost small" :aria-label="$t('Next month')" :title="$t('Next month')" @click="shift(1)">›</button>
          <button type="button" class="btn btn-ghost small" :disabled="!nextPoint" :title="$t('The closest later month with something in it')" @click="goPoint(nextPoint)">
            {{ $t('Next') }} ⇥
          </button>
        </div>
        <label class="year-jump">
          <span>{{ $t('Year') }}</span>
          <input :value="year" type="number" step="1" @change="setYear($event.target.value)" />
        </label>
      </section>

      <div class="months" role="tablist" :aria-label="$t('Months of {year}', { year: yearLabel })">
        <button
          v-for="m in calendar.months_per_year"
          :key="m"
          type="button"
          role="tab"
          class="month-chip"
          :class="{ 'is-on': m === month, 'has-points': monthCounts[m] }"
          :aria-selected="m === month"
          :title="$tn('{n} point', '{n} points', monthCounts[m] || 0)"
          @click="go(year, m)"
        >
          {{ m }}<span v-if="monthCounts[m]" class="dot-count">{{ monthCounts[m] }}</span>
        </button>
      </div>

      <div class="grid" :style="{ '--cols': COLS }">
        <div
          v-for="d in calendar.days_per_month"
          :key="d"
          class="day"
          :class="{ 'has-points': byDay[d], 'is-picked': picked === d }"
          @click="picked = picked === d ? null : d"
        >
          <span class="day-num">{{ d }}</span>
          <ul v-if="byDay[d]" class="chips">
            <li v-for="n in byDay[d].slice(0, MAX_CHIPS)" :key="n.key">
              <RouterLink :to="linkOf(n)" class="chip" :class="'kind-' + n.kind" :title="`${n.name} · ${fullDate(n.date)}`" @click.stop>
                {{ n.name }}
              </RouterLink>
            </li>
            <li v-if="byDay[d].length > MAX_CHIPS" class="more">{{ $t('+{n} more', { n: byDay[d].length - MAX_CHIPS }) }}</li>
          </ul>
        </div>
      </div>

      <div class="legend">
        <span v-for="k in KINDS" :key="k"><i class="dot" :class="'kind-' + k"></i>{{ kindLabel(k) }}</span>
      </div>

      <section class="glass-panel list-panel">
        <h3>
          {{ picked ? $t('Day {n}', { n: picked }) : $t('This month') }}
          <button v-if="picked" type="button" class="link-btn" @click="picked = null">{{ $t('Show the whole month') }}</button>
        </h3>
        <ul v-if="listed.length" class="list">
          <li v-for="n in listed" :key="n.key">
            <RouterLink :to="linkOf(n)" class="row">
              <i class="dot" :class="'kind-' + n.kind"></i>
              <span class="row-day">{{ parseStoryDate(n.date).day }}</span>
              <span class="row-name">{{ n.name }}</span>
              <span v-if="n.location_name" class="row-meta">{{ n.location_name }}</span>
              <span class="row-meta">{{ kindLabel(n.kind) }}</span>
            </RouterLink>
          </li>
        </ul>
        <p v-else class="empty">{{ picked ? $t('Nothing on this day.') : $t('Nothing happens this month.') }}</p>
      </section>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { timelineApi } from '../api'
import { calendar, parseStoryDate, formatStoryDate, fullDate } from '../calendar'
import { mapPath } from '../navigation'
import { wikiPath } from '../wiki'
import { t } from '../i18n'

// The story's calendar, a month at a time: everything the timeline shows
// (events, births, foundings, dated lore) on the day it happens. The custom
// calendar has no weeks, so the days simply run in rows. The month in view is
// in the URL (?y=&m=), so going back returns to it.

const COLS = 7
const MAX_CHIPS = 3
const KINDS = ['event', 'birth', 'founding', 'lore']

const route = useRoute()
const router = useRouter()
const data = ref(null)
const loadError = ref('')
const picked = ref(null)

const num = (v) => (typeof v === 'string' && /^-?\d+$/.test(v) ? Number(v) : null)
const year = computed(() => num(route.query.y) ?? fallback.value.year)
const month = computed(() => Math.min(calendar.months_per_year, Math.max(1, num(route.query.m) ?? fallback.value.month)))
const yearLabel = computed(() => (year.value < 0 ? `${year.value}` : String(year.value)))

// Points with a readable date, sorted by when.
const points = computed(() =>
  (data.value?.nodes || [])
    .map((n) => ({ ...n, d: parseStoryDate(n.date) }))
    .filter((n) => n.d)
    .sort((a, b) => a.t - b.t),
)

// With no month in the URL: the month of the latest point, or year 1.
const fallback = computed(() => {
  const last = points.value[points.value.length - 1]
  return last ? { year: last.d.year, month: last.d.month } : { year: 1, month: 1 }
})

const inMonth = computed(() => points.value.filter((n) => n.d.year === year.value && n.d.month === month.value))
const byDay = computed(() => {
  const out = {}
  for (const n of inMonth.value) (out[n.d.day] ||= []).push(n)
  return out
})
const monthCounts = computed(() => {
  const out = {}
  for (const n of points.value) if (n.d.year === year.value) out[n.d.month] = (out[n.d.month] || 0) + 1
  return out
})
const listed = computed(() => (picked.value ? byDay.value[picked.value] || [] : inMonth.value))

// Month order as one number, for finding the nearest month with something in it.
const monthIndex = (y, m) => y * calendar.months_per_year + (m - 1)
const prevPoint = computed(() => {
  const here = monthIndex(year.value, month.value)
  return [...points.value].reverse().find((n) => monthIndex(n.d.year, n.d.month) < here) || null
})
const nextPoint = computed(() => {
  const here = monthIndex(year.value, month.value)
  return points.value.find((n) => monthIndex(n.d.year, n.d.month) > here) || null
})

function go(y, m) {
  picked.value = null
  router.replace({ query: { ...route.query, y: String(y), m: String(m) } })
}

// One month back or forward; there is no year 0.
function shift(by) {
  let y = year.value
  let m = month.value + by
  if (m < 1) {
    m = calendar.months_per_year
    y = y === 1 ? -1 : y - 1
  } else if (m > calendar.months_per_year) {
    m = 1
    y = y === -1 ? 1 : y + 1
  }
  go(y, m)
}

function setYear(v) {
  const y = Number(v)
  if (Number.isInteger(y) && y !== 0) go(y, month.value)
}

const goPoint = (n) => n && go(n.d.year, n.d.month)

const newEventDate = computed(() => formatStoryDate({ day: picked.value || 1, month: month.value, year: year.value }))

const kindLabel = (k) => (k === 'birth' ? t('Birth') : k === 'founding' ? t('Founding') : k === 'lore' ? t('Lore') : t('Event'))

// Where each kind of point leads, as on the timeline.
function linkOf(n) {
  if (n.kind === 'event') return `/events/${n.event_id}`
  if (n.kind === 'birth') return `/characters/${n.source_id}`
  if (n.kind === 'lore') return wikiPath('lore', n.source_id)
  return n.event_id ? `/events/${n.event_id}` : mapPath(n.source_id)
}

watch(() => [route.query.y, route.query.m], () => (picked.value = null))

timelineApi
  .get()
  .then((d) => (data.value = d))
  .catch(() => (loadError.value = t("Couldn't load the timeline. Try refreshing.")))
</script>

<style scoped>
.calendar-page {
  --kind-event: var(--accent);
}

.head-actions {
  display: flex;
  gap: 0.5rem;
}

.page-sub a {
  color: var(--line);
  text-decoration: none;
}

.nav-bar {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  align-items: center;
  gap: 0.6rem 1rem;
  padding: 0.6rem 0.9rem;
  margin-bottom: 0.7rem;
}

.nav-group {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.35rem;
}

.month-title {
  min-width: 11rem;
  margin: 0 0.4rem;
  font-size: 1.25rem;
  text-align: center;
}

.year-jump {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  font-size: 0.8rem;
  color: var(--text-muted);
}

.year-jump input {
  width: 6.5rem;
  min-height: 2rem;
  padding: 0.25rem 0.5rem;
}

.months {
  display: flex;
  flex-wrap: wrap;
  gap: 0.3rem;
  margin-bottom: 0.8rem;
}

.month-chip {
  position: relative;
  min-width: 2.4rem;
  padding: 0.25rem 0.5rem;
  border: 1px solid var(--glass-border);
  border-radius: 8px;
  background: none;
  color: var(--text-faint);
  font-family: 'Manrope', sans-serif;
  font-size: 0.8rem;
  font-weight: 600;
  cursor: pointer;
}

.month-chip.has-points {
  color: var(--text-primary);
}

.month-chip.is-on {
  border-color: var(--accent);
  background: var(--accent-soft);
  color: var(--accent);
}

.dot-count {
  margin-left: 0.3rem;
  padding: 0 0.3rem;
  border-radius: 999px;
  background: color-mix(in srgb, var(--accent) 25%, transparent);
  font-size: 0.66rem;
}

.grid {
  display: grid;
  grid-template-columns: repeat(var(--cols), minmax(0, 1fr));
  gap: 0.35rem;
}

.day {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  min-height: 6.2rem;
  padding: 0.35rem 0.4rem;
  border: 1px solid var(--glass-border);
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.02);
  cursor: pointer;
}

.day.has-points {
  background: rgba(255, 255, 255, 0.045);
}

.day.is-picked {
  border-color: var(--accent);
}

.day-num {
  font-size: 0.74rem;
  font-weight: 700;
  color: var(--text-faint);
}

.chips {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
  margin: 0;
  padding: 0;
  list-style: none;
  min-width: 0;
}

.chip {
  display: block;
  padding: 0.1rem 0.35rem;
  overflow: hidden;
  border-radius: 5px;
  color: #10131c;
  font-size: 0.7rem;
  font-weight: 700;
  white-space: nowrap;
  text-overflow: ellipsis;
  text-decoration: none;
}

.more {
  font-size: 0.68rem;
  color: var(--text-faint);
}

.kind-event {
  background: var(--kind-event);
}
.kind-birth {
  background: var(--kind-birth);
}
.kind-founding {
  background: var(--kind-founding);
}
.kind-lore {
  background: var(--kind-lore);
}

.legend {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
  margin: 0.7rem 0 1rem;
  font-size: 0.76rem;
  color: var(--text-muted);
}

.legend span {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
}

.dot {
  display: inline-block;
  flex: none;
  width: 9px;
  height: 9px;
  border-radius: 50%;
}

.list-panel {
  padding: 0.9rem 1.1rem;
}

.list-panel h3 {
  display: flex;
  align-items: baseline;
  gap: 0.8rem;
  margin: 0 0 0.6rem;
}

.link-btn {
  border: none;
  background: none;
  padding: 0;
  color: var(--accent);
  font: inherit;
  font-size: 0.8rem;
  cursor: pointer;
}

.list {
  margin: 0;
  padding: 0;
  list-style: none;
}

.row {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  padding: 0.4rem 0.2rem;
  border-top: 1px solid var(--glass-border);
  color: var(--text-primary);
  font-size: 0.88rem;
  text-decoration: none;
}

.row:hover .row-name {
  color: var(--accent);
}

.row-day {
  width: 1.8rem;
  font-weight: 700;
  color: var(--text-faint);
  text-align: right;
}

.row-name {
  flex: 1;
  min-width: 0;
}

.row-meta {
  font-size: 0.76rem;
  color: var(--text-faint);
}

.empty {
  margin: 0;
  font-size: 0.86rem;
  color: var(--text-faint);
}

@media (max-width: 720px) {
  .grid {
    --cols: 4 !important;
  }
  .day {
    min-height: 4.5rem;
  }
}
</style>
