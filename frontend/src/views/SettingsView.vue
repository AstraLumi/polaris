<template>
  <div class="page" :class="{ 'is-narrow': tab !== 'stats' }">
    <header class="page-header">
      <div>
        <h1>{{ $t('Settings') }}</h1>
        <p class="page-sub">{{ $t('App-wide options that more than one page depends on.') }}</p>
      </div>
    </header>

    <div class="tab-bar">
      <button class="tab-button" :class="{ 'is-active': tab === 'general' }" @click="tab = 'general'">
        {{ $t('General') }}
      </button>
      <button class="tab-button" :class="{ 'is-active': tab === 'stats' }" :title="$t('Which stats characters have and how they are calculated')" @click="tab = 'stats'">
        {{ $t('Stats') }}
      </button>
    </div>

    <!-- Both stay mounted, so unsaved stat edits survive a look at General. -->
    <StatSystemEditor v-show="tab === 'stats'" />

    <div v-show="tab === 'general'">
    <section class="glass-panel card">
      <h2>{{ $t('Language') }}</h2>
      <p class="lede">
        {{ $t('The language of the app\'s menus, buttons and messages. What you write in your story is never translated. Remembered on this device only.') }}
      </p>
      <label class="field narrow">
        <span>{{ $t('Interface language') }}</span>
        <select :value="locale" @change="saveLocale($event.target.value)">
          <option v-for="l in LOCALES" :key="l.id" :value="l.id">{{ l.name }}</option>
        </select>
      </label>
    </section>

    <section class="glass-panel card">
      <h2>{{ $t('Appearance') }}</h2>
      <p class="lede">
        {{ $t('Pick a colour theme. It applies straight away and is remembered on this device only, so your phone and your desktop can look different.') }}
      </p>
      <div class="themes" role="radiogroup" :aria-label="$t('Colour theme')">
        <button
          v-for="t in THEMES"
          :key="t.id"
          type="button"
          class="theme-card"
          :class="{ 'is-selected': currentTheme === t.id }"
          :data-theme="t.id"
          role="radio"
          :aria-checked="currentTheme === t.id"
          @click="saveTheme(t.id)"
        >
          <span class="theme-preview">
            <span class="swatch" style="background: var(--accent)"></span>
            <span class="swatch" style="background: var(--logo-star)"></span>
            <span class="swatch" style="background: var(--line)"></span>
            <span class="swatch" style="background: var(--kind-birth)"></span>
            <span class="swatch" style="background: var(--kind-founding)"></span>
          </span>
          <span class="theme-name">{{ t.name }}<span v-if="currentTheme === t.id" class="theme-check">✓</span></span>
          <span class="theme-blurb">{{ $t(t.blurb) }}</span>
        </button>
      </div>
    </section>

    <section id="calendar" class="glass-panel card">
      <h2>{{ $t('Calendar') }}</h2>
      <p class="lede">
        {{ $t('How your world counts time. Date inputs, the Q1–Q4 labels and the timeline all follow this, so set it before entering many dates.') }}
      </p>

      <p v-if="loadError" class="error-banner">{{ loadError }}</p>
      <template v-else>
        <div class="field-row narrow">
          <label class="field">
            <span>{{ $t('Months per year') }}</span>
            <input v-model.number="months" type="number" min="1" :max="100" step="1" />
          </label>
          <label class="field">
            <span>{{ $t('Days per month') }}</span>
            <input v-model.number="days" type="number" min="1" :max="400" step="1" />
          </label>
        </div>

        <div class="quarters">
          <span class="quarters-title">{{ $t('Quarters') }}</span>
          <ul>
            <li v-for="q in quarterRows" :key="q.q">
              <strong>{{ $t('Q{n}', { n: q.q }) }}</strong>
              <span>{{ q.range }}</span>
            </li>
          </ul>
          <p v-if="uneven" class="note">
            {{ $t('{n} months don\'t split into four equal quarters, so some quarters get one more month than others.', { n: months }) }}
          </p>
        </div>

        <p class="note">
          {{ $t('Changing this doesn\'t rewrite dates you\'ve already saved. A saved date that no longer exists in the new calendar (say, day 31 after switching to 30-day months) is kept as is, and will be refused the next time you save that item until you fix it.') }}
        </p>

        <p v-if="saveError" class="error-banner">{{ saveError }}</p>
        <div class="actions">
          <button class="btn btn-primary" :disabled="saving || !valid" @click="save">
            {{ saving ? $t('Saving…') : $t('Save calendar') }}
          </button>
          <span v-if="saved" class="saved-hint">{{ $t('Saved.') }}</span>
        </div>
      </template>
    </section>

    <section id="seasons" class="glass-panel card">
      <h2>{{ $t('Seasons') }}</h2>
      <p class="lede">
        {{ $t('Name your world\'s seasons, put them in order and say how many months each lasts. They repeat from the day the first one starts, so a round of seasons can be longer than a year: a winter can last three years. The calendar shows them, and regions on the wiki can give temperatures for each.') }}
      </p>

      <template v-if="!loadError">
        <div v-if="seasonList.length" class="cycle-bar" :aria-label="$t('One round of seasons')">
          <span
            v-for="s in seasonList"
            :key="s.id"
            :style="{ flexGrow: s.months > 0 ? s.months : 0.01, background: s.color }"
            :title="`${s.name} · ${$tn('{n} month', '{n} months', s.months)}`"
          >{{ s.name }}</span>
        </div>
        <p v-if="seasonList.length && seasonsValid" class="note cycle-note">{{ cycleNote }}</p>

        <ul class="season-rows">
          <li v-for="(s, i) in seasonList" :key="s.id">
            <input v-model="s.color" type="color" class="season-color" :aria-label="$t('Colour')" />
            <input v-model="s.name" type="text" maxlength="60" class="season-name" :placeholder="$t('Season name')" :aria-label="$t('Season name')" />
            <label class="season-months">
              <input v-model.number="s.months" type="number" min="0.5" max="1200" step="0.5" :aria-label="$t('Months')" />
              <span>{{ $t('months') }}</span>
            </label>
            <button type="button" class="btn btn-ghost small" :disabled="i === 0" :aria-label="$t('Move up')" @click="moveSeason(i, -1)">↑</button>
            <button type="button" class="btn btn-ghost small" :disabled="i === seasonList.length - 1" :aria-label="$t('Move down')" @click="moveSeason(i, 1)">↓</button>
            <button type="button" class="btn btn-danger small" @click="seasonList.splice(i, 1)">{{ $t('Remove') }}</button>
          </li>
        </ul>
        <p v-if="!seasonList.length" class="note">{{ $t('No seasons: the calendar shows none.') }}</p>
        <div class="season-add">
          <button type="button" class="btn btn-ghost small" :disabled="seasonList.length >= 24" @click="addSeason">{{ $t('Add season') }}</button>
          <button type="button" class="btn btn-ghost small" @click="resetSeasons">{{ $t('Reset to the four seasons') }}</button>
        </div>

        <template v-if="seasonList.length">
          <div class="season-fields">
            <DateField v-model="seasonStart" :label="$t('The first season starts on')" />
            <label class="field">
              <span>{{ $t('These seasons are for the') }}</span>
              <select v-model="hemisphere">
                <option value="north">{{ $t('Northern hemisphere') }}</option>
                <option value="south">{{ $t('Southern hemisphere') }}</option>
              </select>
            </label>
            <label class="field">
              <span>{{ $t('Temperatures in') }}</span>
              <select v-model="unit">
                <option value="C">°C</option>
                <option value="F">°F</option>
              </select>
            </label>
          </div>
          <p class="note">
            {{ $t('The other hemisphere has the same seasons half a round later: its summer falls in your winter. The calendar shows both.') }}
          </p>
        </template>

        <p v-if="seasonError" class="error-banner">{{ seasonError }}</p>
        <div class="actions">
          <button class="btn btn-primary" :disabled="seasonSaving || !seasonsValid" @click="saveSeasonSettings">
            {{ seasonSaving ? $t('Saving…') : $t('Save seasons') }}
          </button>
          <span v-if="seasonSaved" class="saved-hint">{{ $t('Saved.') }}</span>
        </div>
      </template>
    </section>

    <section class="glass-panel card about">
      <h2>{{ $t('About Polaris') }}</h2>
      <dl class="about-list">
        <dt>{{ $t('Version') }}</dt>
        <dd>{{ appVersion }}</dd>
        <dt>{{ $t('Made by') }}</dt>
        <dd><a :href="REPO_OWNER_URL" target="_blank" rel="noopener noreferrer">AstraLumi</a></dd>
        <dt>{{ $t('License') }}</dt>
        <dd>
          <a :href="`${REPO_URL}/blob/main/LICENSE`" target="_blank" rel="noopener noreferrer">MIT</a>
          · {{ $t('free to use, change and share; provided as is, without warranty') }}
        </dd>
        <dt>{{ $t('Source code') }}</dt>
        <dd><a :href="REPO_URL" target="_blank" rel="noopener noreferrer">github.com/AstraLumi/polaris</a></dd>
      </dl>
    </section>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, nextTick } from 'vue'
import { fetchSettings, saveSettings, saveSeasons } from '../api'
import { loadCalendar, calendar } from '../calendar'
import { THEMES, currentTheme, saveTheme } from '../theme'
import { t, tn, LOCALES, locale, saveLocale } from '../i18n'
import pkg from '../../package.json'
import { useRoute } from 'vue-router'
import StatSystemEditor from '../components/StatSystemEditor.vue'
import DateField from '../components/DateField.vue'
import { newSeasonId } from '../seasons'

// ?tab=stats opens the Stats tab (the Build tab links there).
const route = useRoute()
const tab = ref(route.query.tab === 'stats' ? 'stats' : 'general')

// The version comes from package.json at build time, so it is always the
// one that was built.
const appVersion = `v${pkg.version}`
const REPO_URL = 'https://github.com/AstraLumi/polaris'
const REPO_OWNER_URL = 'https://github.com/AstraLumi'

const months = ref(calendar.months_per_year)
const days = ref(calendar.days_per_month)
const loadError = ref('')
const saveError = ref('')
const saving = ref(false)
const saved = ref(false)

const valid = computed(
  () =>
    Number.isInteger(months.value) && months.value >= 1 && months.value <= 100 &&
    Number.isInteger(days.value) && days.value >= 1 && days.value <= 400,
)
const uneven = computed(() => valid.value && months.value % 4 !== 0)

// Which months fall in each quarter, using the same rule as quarterOf().
const quarterRows = computed(() => {
  if (!valid.value) return []
  const rows = []
  for (let q = 1; q <= 4; q++) {
    // Same rule as quarterOf(): which months land in this quarter.
    let lo = null
    let hi = null
    for (let m = 1; m <= months.value; m++) {
      if (Math.floor(((m - 1) * 4) / months.value) + 1 === q) {
        if (lo === null) lo = m
        hi = m
      }
    }
    rows.push({
      q,
      range:
        lo === null
          ? t('(no months)')
          : lo === hi
            ? t('month {n}', { n: lo })
            : t('months {from}–{to}', { from: lo, to: hi }),
    })
  }
  return rows
})

// ---- Seasons ---------------------------------------------------------------------
const seasonList = ref([])
const seasonStart = ref('')
const hemisphere = ref('north')
const unit = ref('C')
const seasonError = ref('')
const seasonSaving = ref(false)
const seasonSaved = ref(false)

const SEASON_COLORS = ['#7fb2e5', '#86cf7f', '#f2c14e', '#e0864a', '#b48ce0', '#e57f9e']

function takeSeasons(s) {
  seasonList.value = (s.seasons || []).map((x, i) => ({ ...x, color: x.color || SEASON_COLORS[i % SEASON_COLORS.length] }))
  seasonStart.value = s.start || ''
  hemisphere.value = s.hemisphere || 'north'
  unit.value = s.unit || 'C'
}

const seasonsValid = computed(
  () =>
    seasonList.value.every((s) => s.name.trim() && s.months > 0 && s.months <= 1200) &&
    (!seasonList.value.length || /^(\d{1,3}-\d{1,3}-)?-?\d+$/.test(seasonStart.value || '')),
)

// How long one round of seasons is, against the year.
const cycleNote = computed(() => {
  const total = Math.round(seasonList.value.reduce((n, s) => n + s.months, 0) * 100) / 100
  const perYear = calendar.months_per_year
  if (total === perYear) return t('One round of seasons is one year.')
  if (total % perYear === 0) return t('One round of seasons is {n} years.', { n: total / perYear })
  if (total > perYear) {
    return t('One round of seasons is {n} months, about {years} years.', { n: total, years: Math.round((total / perYear) * 100) / 100 })
  }
  return tn('One round of seasons is {n} month, not a whole year.', 'One round of seasons is {n} months, not a whole year.', total)
})

function moveSeason(i, by) {
  const list = seasonList.value
  ;[list[i], list[i + by]] = [list[i + by], list[i]]
}

function addSeason() {
  seasonList.value.push({ id: newSeasonId(), name: '', months: 3, color: SEASON_COLORS[seasonList.value.length % SEASON_COLORS.length] })
}

// Winter, spring, summer and fall, a quarter of the year each, winter from
// the last month. The ids are the defaults', so temperatures typed for
// them come back.
function resetSeasons() {
  const quarter = Math.round((calendar.months_per_year / 4) * 100) / 100
  seasonList.value = [
    ['winter', t('Winter')],
    ['spring', t('Spring')],
    ['summer', t('Summer')],
    ['fall', t('Fall')],
  ].map(([id, name], i) => ({ id, name, months: quarter, color: SEASON_COLORS[i] }))
  seasonStart.value = `01-${String(calendar.months_per_year).padStart(2, '0')}-0001`
}

async function saveSeasonSettings() {
  seasonSaving.value = true
  seasonError.value = ''
  seasonSaved.value = false
  try {
    const s = await saveSeasons({
      seasons: seasonList.value.map((x) => ({ ...x, name: x.name.trim() })),
      start: seasonStart.value,
      hemisphere: hemisphere.value,
      unit: unit.value,
    })
    takeSeasons(s)
    await loadCalendar()
    seasonSaved.value = true
    setTimeout(() => (seasonSaved.value = false), 2500)
  } catch (err) {
    seasonError.value = err.message || t("Couldn't save.")
  } finally {
    seasonSaving.value = false
  }
}

async function save() {
  saving.value = true
  saveError.value = ''
  saved.value = false
  try {
    await saveSettings({ months_per_year: months.value, days_per_month: days.value })
    await loadCalendar()
    saved.value = true
    setTimeout(() => (saved.value = false), 2500)
  } catch (err) {
    saveError.value = err.message || t("Couldn't save.")
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  try {
    const s = await fetchSettings()
    months.value = s.months_per_year
    days.value = s.days_per_month
    if (s.seasons) takeSeasons(s.seasons)
  } catch (err) {
    loadError.value = "Couldn't load settings. Try refreshing."
  }
  // The calendar page's Settings button links to #calendar.
  const hash = route.hash.slice(1)
  if (hash) {
    await nextTick()
    document.getElementById(hash)?.scrollIntoView({ block: 'start' })
  }
})
</script>

<style scoped>
.about-list {
  display: grid;
  grid-template-columns: max-content 1fr;
  gap: 0.45rem 1.25rem;
  margin: 0.75rem 0 0;
  font-size: 0.88rem;
}

.about-list dt {
  color: var(--text-muted);
}

.about-list dd {
  margin: 0;
  color: var(--text-primary);
}

.about-list a {
  color: var(--accent);
}

@media (max-width: 720px) {
  .about-list {
    grid-template-columns: 1fr;
    gap: 0.15rem;
  }
  .about-list dd {
    margin-bottom: 0.5rem;
  }
}

.card {
  padding: 1.75rem;
  margin-bottom: 1.25rem;
}

.themes {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 0.75rem;
}

/* Each card carries its own data-theme, so it shows that theme's colours
   whatever theme is active. */
.theme-card {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
  align-items: flex-start;
  text-align: left;
  padding: 0.9rem;
  border-radius: 12px;
  border: 1px solid var(--glass-border);
  background: var(--bg);
  color: var(--text-primary);
  font-family: inherit;
  cursor: pointer;
  transition: transform 0.12s ease, border-color 0.12s ease;
}

.theme-card:hover {
  transform: translateY(-1px);
}

.theme-card.is-selected {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft);
}

.theme-preview {
  display: flex;
  gap: 0.3rem;
}

.swatch {
  width: 1.1rem;
  height: 1.1rem;
  border-radius: 50%;
  border: 1px solid rgba(255, 255, 255, 0.18);
}

.theme-name {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  font-weight: 700;
  font-size: 0.92rem;
}

.theme-check {
  color: var(--accent);
}

.theme-blurb {
  font-size: 0.76rem;
  line-height: 1.4;
  color: var(--text-muted);
}

.card h2 {
  font-family: 'Fraunces', serif;
  font-weight: 500;
  font-size: 1.2rem;
  margin: 0 0 0.5rem;
}

.lede {
  margin: 0 0 1.25rem;
  color: var(--text-muted);
  font-size: 0.9rem;
  line-height: 1.55;
}

.narrow {
  max-width: 420px;
}

.quarters {
  margin: 0.5rem 0 1.25rem;
}

.quarters-title {
  font-size: 0.85rem;
  color: var(--text-muted);
}

.quarters ul {
  list-style: none;
  margin: 0.5rem 0 0;
  padding: 0;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
  gap: 0.5rem;
}

.quarters li {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.5rem 0.75rem;
  border: 1px solid var(--glass-border);
  border-radius: 8px;
  font-size: 0.82rem;
  color: var(--text-muted);
}

.quarters li strong {
  color: var(--accent);
}

.cycle-bar {
  display: flex;
  overflow: hidden;
  margin-bottom: 0.4rem;
  border: 1px solid var(--glass-border);
  border-radius: 8px;
}

.cycle-bar span {
  flex-basis: 0;
  min-width: 0;
  padding: 0.35rem 0.5rem;
  overflow: hidden;
  color: #10131c;
  font-size: 0.76rem;
  font-weight: 700;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.cycle-note {
  margin-top: 0;
}

.season-rows {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
  margin: 0.75rem 0 0.6rem;
  padding: 0;
  list-style: none;
}

.season-rows li {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.4rem;
}

.season-color {
  width: 2.4rem;
  height: 2.2rem;
  padding: 0.15rem;
  cursor: pointer;
}

.season-name {
  flex: 1 1 10rem;
  min-width: 0;
}

.season-months {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.82rem;
  color: var(--text-muted);
}

.season-months input {
  width: 5.5rem;
}

.season-add {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin-bottom: 1.1rem;
}

.season-fields {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(190px, 1fr));
  gap: 0.75rem;
  align-items: end;
}

.note {
  margin: 0.75rem 0 1.25rem;
  font-size: 0.8rem;
  line-height: 1.5;
  color: var(--text-faint);
}

.actions {
  display: flex;
  align-items: center;
  gap: 0.9rem;
}

.saved-hint {
  font-size: 0.82rem;
  color: var(--text-muted);
}
</style>
