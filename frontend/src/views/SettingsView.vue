<template>
  <div class="page is-narrow">
    <header class="page-header">
      <div>
        <h1>{{ $t('Settings') }}</h1>
        <p class="page-sub">{{ $t('App-wide options that more than one page depends on.') }}</p>
      </div>
    </header>

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

    <section class="glass-panel card">
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
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { fetchSettings, saveSettings } from '../api'
import { loadCalendar, calendar } from '../calendar'
import { THEMES, currentTheme, saveTheme } from '../theme'
import { t, LOCALES, locale, saveLocale } from '../i18n'

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
  } catch (err) {
    loadError.value = "Couldn't load settings. Try refreshing."
  }
})
</script>

<style scoped>
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
