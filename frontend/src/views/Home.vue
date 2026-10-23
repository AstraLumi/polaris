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

      <section v-if="resume.length" class="glass-panel block resume">
        <div class="block-head">
          <h2>{{ $t('Continue where you left off') }}</h2>
        </div>
        <ul class="resume-list">
          <li v-for="r in resume" :key="r.key">
            <RouterLink :to="r.path" class="resume-row">
              <span class="resume-icon"><IconImage :src="r.picture" :name="r.name" /></span>
              <span class="recent-text">
                <strong>{{ r.name }}</strong>
                <small>{{ r.label }}</small>
              </span>
            </RouterLink>
          </li>
        </ul>
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
              <RouterLink :to="looseLink(l)" class="loose-row">
                <strong data-tip-overflow>{{ l.name }}</strong>
                <span>{{ $t(l.issue) }}</span>
              </RouterLink>
            </li>
          </ul>
          <p v-else class="empty">{{ $t('Nothing to tidy: every kingdom, character, event, chapter and lore page has what it needs.') }}</p>
          <p v-if="data.loose_more" class="foot">{{ $tn('…and {n} more like these.', '…and {n} more like these.', data.loose_more) }}</p>
          <p class="foot">{{ $t('Missing dates, map links and heights; empty chapters and lore pages.') }}</p>
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
import { timeAgo } from '../timeAgo'
import { TYPE_LABELS } from '../wiki'
import { recentPath, recentKey } from '../recent'
import { recentItems, recentPictures, checkRecent } from '../recentTracker'
import IconImage from '../components/IconImage.vue'

// The "loose ends" labels come from the backend (home.go); listing them here
// lets the language checker see them and keep every catalog complete.
void [
  tr('No founding date'), tr('No in-story birth date'), tr('Birthplace needs a map location'), tr('Nation needs a kingdom'),
  tr('No height or body type'), tr('No date'), tr('No events, versions or notes'), tr('Not written yet'),
]

// Where a loose end is fixed: the version's form for a height, the event's
// form for a date, the page itself otherwise.
function looseLink(l) {
  if (l.kind === 'character') return l.version_id ? `/characters/${l.id}/versions/${l.version_id}/edit` : `/characters/${l.id}`
  if (l.kind === 'event') return `/events/${l.id}/edit`
  if (l.kind === 'chapter') return `/chapters/${l.id}`
  if (l.kind === 'lore') return `/wiki/lore/${l.id}`
  return { path: '/map', query: { edit: l.id } }
}

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

const ago = timeAgo

// The items opened last (see recent.js), newest first.
const resume = computed(() =>
  recentItems.value.map((r) => ({
    key: recentKey(r),
    name: r.name,
    label: r.type === 'chapter' ? t('Chapter') : t(TYPE_LABELS[r.type] || r.type),
    picture: recentPictures.value[recentKey(r)] || '',
    path: recentPath(r),
  })),
)

onMounted(async () => {
  checkRecent()
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

.resume {
  margin-bottom: 1rem;
  padding-bottom: 0.9rem;
}

.resume-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(210px, 100%), 1fr));
  gap: 0.15rem 0.5rem;
}

.resume-row {
  display: flex;
  align-items: center;
  gap: 0.65rem;
  padding: 0.4rem 0.4rem;
  border-radius: 8px;
  text-decoration: none;
  min-width: 0;
}

.resume-row:hover {
  background: rgba(255, 255, 255, 0.05);
}

.resume-icon {
  width: 30px;
  height: 30px;
  border-radius: 8px;
  overflow: hidden;
  flex-shrink: 0;
  display: grid;
  place-items: center;
  background: var(--accent-soft);
  font-size: 0.8rem;
}

.columns {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(320px, 100%), 1fr));
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

/* Phones: two tiles per row, and only the last few items to resume. */
@media (max-width: 720px) {
  .resume-list li:nth-child(n + 5) {
    display: none;
  }

  .tiles {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
