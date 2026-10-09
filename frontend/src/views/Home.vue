<template>
  <div class="page">
    <header class="page-header">
      <div>
      <h1>{{ $t('Your world, at a glance.') }}</h1>
      <p class="page-sub">
        <template v-if="data">
          {{ $t('Calendar: {months} months × {days} days', { months: data.calendar.months_per_year, days: data.calendar.days_per_month }) }}
          · <RouterLink to="/settings">{{ $t('change') }}</RouterLink>
        </template>
        <template v-else>&nbsp;</template>
      </p>
      </div>
    </header>

    <p v-if="error" class="error-banner">{{ error }}</p>
    <p v-else-if="!data" class="loading-hint">{{ $t('Loading…') }}</p>

    <template v-else>
      <section class="tiles">
        <RouterLink v-for="t in tiles" :key="t.label" :to="t.to" class="tile glass-panel">
          <span class="tile-value">{{ t.value }}</span>
          <span class="tile-label">{{ $t(t.label) }}</span>
          <span v-if="t.note" class="tile-note">{{ $t(t.note) }}</span>
        </RouterLink>
      </section>

      <div class="columns">
        <section class="glass-panel block">
          <div class="block-head">
            <h2>{{ $t('Recently edited') }}</h2>
            <RouterLink to="/characters" class="more">{{ $t('All characters →') }}</RouterLink>
          </div>
          <ul v-if="data.recent.length" class="recent">
            <li v-for="c in data.recent" :key="c.character_id">
              <RouterLink :to="`/characters/${c.character_id}`" class="recent-row">
                <span class="avatar">
                  <img v-if="c.picture_path" :src="c.picture_path" :alt="c.name" />
                  <template v-else>{{ initials(c.name) }}</template>
                </span>
                <span class="recent-text">
                  <strong>{{ c.name }}</strong>
                  <small>{{ $t('Level {level}', { level: c.level }) }}<template v-if="c.class_name"> · {{ c.class_name }}</template></small>
                </span>
                <span class="when">{{ ago(c.updated_at) }}</span>
              </RouterLink>
            </li>
          </ul>
          <p v-else class="empty">{{ $t('No characters yet. Add one from the Characters page.') }}</p>
        </section>

        <section class="glass-panel block">
          <div class="block-head">
            <h2>{{ $t('Loose ends') }}</h2>
          </div>
          <ul v-if="data.loose_ends.length" class="loose">
            <li v-for="(l, i) in data.loose_ends" :key="i">
              <RouterLink :to="l.kind === 'character' ? `/characters/${l.id}` : { path: '/map', query: { edit: l.id } }" class="loose-row">
                <strong>{{ l.name }}</strong>
                <span>{{ $t(l.issue) }}</span>
              </RouterLink>
            </li>
          </ul>
          <p v-else class="empty">{{ $t('Nothing to tidy. Every kingdom and character has what the timeline needs.') }}</p>
          <p class="foot">{{ $t('Dates and map links the timeline will use, still missing.') }}</p>
        </section>
      </div>

      <section class="glass-panel block">
        <div class="block-head">
          <h2>{{ $t('Recently added events') }}</h2>
          <RouterLink to="/events" class="more">{{ $t('All events →') }}</RouterLink>
        </div>
        <ul v-if="data.events.length" class="recent">
          <li v-for="e in data.events" :key="e.id">
            <RouterLink :to="`/events/${e.id}`" class="recent-row">
              <span class="recent-text">
                <strong>{{ e.name }}</strong>
                <small>{{ fullDate(e.event_date) || $t('No date set') }}</small>
              </span>
              <span class="when">{{ ago(e.created_at) }}</span>
            </RouterLink>
          </li>
        </ul>
        <p v-else class="empty">{{ $t('No events yet.') }} <RouterLink to="/events/new">{{ $t('New event') }}</RouterLink></p>
      </section>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { fetchHome } from '../api'
import { t, tn, tr } from '../i18n'
import { fullDate } from '../calendar'

// The "loose ends" labels come from the backend (home.go); listing them here
// lets the language checker see them and keep every catalog complete.
void [tr('No founding date'), tr('No in-story birth date'), tr('Birthplace needs a map location'), tr('Nation needs a kingdom')]

const data = ref(null)
const error = ref('')

const tiles = computed(() => {
  const c = data.value.counts
  return [
    { label: tr('Characters'), value: c.characters, to: '/characters', note: c.versions > c.characters ? tn('{n} version', '{n} versions', c.versions) : '' },
    { label: tr('Kingdoms'), value: c.kingdoms, to: '/map' },
    { label: tr('Locations'), value: c.locations, to: '/map', note: tr('incl. regions') },
    { label: tr('Events'), value: c.events, to: '/events' },
    { label: tr('Spells'), value: c.spells, to: '/assets' },
  ]
})

function initials(name) {
  return (name || '')
    .split(' ')
    .filter(Boolean)
    .map((p) => p[0])
    .join('')
    .slice(0, 2)
    .toUpperCase()
}

// SQLite's datetime('now') is UTC without a zone marker.
function ago(stamp) {
  const ts = new Date(String(stamp).replace(' ', 'T') + 'Z').getTime()
  if (Number.isNaN(ts)) return ''
  const mins = Math.max(0, Math.round((Date.now() - ts) / 60000))
  if (mins < 1) return t('just now')
  if (mins < 60) return t('{n}m ago', { n: mins })
  const hrs = Math.round(mins / 60)
  if (hrs < 24) return t('{n}h ago', { n: hrs })
  const days = Math.round(hrs / 24)
  return days < 30 ? t('{n}d ago', { n: days }) : new Date(ts).toLocaleDateString()
}

onMounted(async () => {
  try {
    data.value = await fetchHome()
  } catch (err) {
    error.value = t("Couldn't load the dashboard. Try refreshing.")
  }
})
</script>

<style scoped>
.sub a {
  color: var(--accent);
}

.tiles {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
  gap: 1rem;
  margin-bottom: 1.25rem;
}

.tile {
  display: flex;
  flex-direction: column;
  padding: 1.1rem 1.25rem;
  text-decoration: none;
  transition: transform 0.12s ease;
}

.tile:hover {
  transform: translateY(-2px);
}

.tile-value {
  font-family: 'Fraunces', serif;
  font-size: 2.1rem;
  line-height: 1.1;
  color: var(--text-primary);
}

.tile-label {
  font-size: 0.85rem;
  color: var(--text-muted);
  margin-top: 0.15rem;
}

.tile-note {
  font-size: 0.72rem;
  color: var(--text-faint);
  margin-top: 0.15rem;
}

.columns {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 1rem;
  margin-bottom: 1rem;
}

.block {
  padding: 1.25rem 1.4rem;
}

.block h2 {
  font-family: 'Fraunces', serif;
  font-weight: 500;
  font-size: 1.1rem;
  margin: 0;
}

.block-head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 0.8rem;
}

.more {
  font-size: 0.78rem;
  color: var(--text-muted);
  text-decoration: none;
}

.recent,
.loose {
  list-style: none;
  margin: 0;
  padding: 0;
}

.recent-row,
.loose-row {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.5rem 0.4rem;
  border-radius: 8px;
  text-decoration: none;
}

.recent-row:hover,
.loose-row:hover {
  background: rgba(255, 255, 255, 0.05);
}

.avatar {
  width: 34px;
  height: 34px;
  border-radius: 50%;
  overflow: hidden;
  flex-shrink: 0;
  display: grid;
  place-items: center;
  background: var(--accent-soft);
  color: var(--accent);
  font-size: 0.75rem;
  font-weight: 700;
}

.avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.recent-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
  flex: 1;
}

.recent-text strong,
.loose-row strong {
  font-size: 0.9rem;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.recent-text small {
  font-size: 0.74rem;
  color: var(--text-faint);
}

.when {
  font-size: 0.72rem;
  color: var(--text-faint);
  flex-shrink: 0;
}

.loose-row {
  justify-content: space-between;
}

.loose-row span {
  font-size: 0.76rem;
  color: var(--accent);
  flex-shrink: 0;
}

.empty {
  margin: 0;
  font-size: 0.85rem;
  color: var(--text-faint);
  line-height: 1.5;
}

.foot {
  margin: 0.8rem 0 0;
  font-size: 0.72rem;
  color: var(--text-faint);
}
</style>
