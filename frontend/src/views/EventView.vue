<template>
  <div class="page event-page">
    <BackLink to="/events" :label="$t('← Back to events')" />

    <p v-if="loadError" class="error-banner">{{ loadError }}</p>
    <p v-else-if="!event" class="loading-hint">{{ $t('Loading…') }}</p>

    <template v-else>
      <article class="glass-panel hero">
        <img v-if="event.picture_path" :src="event.picture_path" :alt="event.name" class="hero-pic zoomable" @click="viewPicture(event.picture_path, event.name)" />
        <div class="hero-text">
          <div class="when">
            <template v-if="shown">
              <span class="when-short">{{ shown.short }}</span>
              <span class="when-full">{{ shown.full }}</span>
            </template>
            <span v-else class="when-full">{{ $t('No readable date') }}</span>
          </div>
          <h1>{{ event.name }}</h1>
          <p v-if="event.location_id" class="where">
            {{ $t('at') }} <RouterLink :to="mapPath(event.location_id)">{{ event.location_name }}</RouterLink>
          </p>
          <p v-if="event.chapter_id && chapterLabel(event.chapter_id)" class="where">
            {{ $t('in') }} <RouterLink :to="`/chapters/${event.chapter_id}`">{{ chapterLabel(event.chapter_id) }}</RouterLink>
          </p>
          <p v-if="event.source" class="linked-note">
            {{ importedParts[0] }}<RouterLink :to="event.source.type === 'character' ? `/characters/${event.source.id}` : mapPath(event.source.id)">{{ event.source.type === 'character' ? $t('{name}\'s birth date', { name: event.source.name }) : $t('{name}\'s founding date', { name: event.source.name }) }}</RouterLink>{{ importedParts[1] }}
          </p>
          <div class="actions">
            <RouterLink :to="`/events/${event.id}/edit`" class="btn btn-primary">{{ $t('Edit') }}</RouterLink>
            <button class="btn btn-danger" @click="confirmOpen = true">
              {{ event.source ? $t('Clear details') : $t('Delete') }}
            </button>
          </div>
        </div>
      </article>

      <section class="glass-panel block">
        <h2>{{ $t('Description') }}</h2>
        <p v-if="event.description" class="description">{{ event.description }}</p>
        <p v-else class="empty">{{ $t('No description yet.') }}</p>
      </section>

      <div class="two-col">
        <section class="glass-panel block">
          <h2>{{ $t('Tags') }}</h2>
          <div v-if="event.tags.length" class="chips">
            <RouterLink v-for="t in event.tags" :key="t" :to="{ path: '/events', query: { tag: t } }" class="tag-chip">
              #{{ t }}
            </RouterLink>
          </div>
          <p v-else class="empty">{{ $t('No tags.') }}</p>
          <p v-if="event.tags.length" class="foot">{{ $t('Click a tag to see every event that shares it.') }}</p>
        </section>

        <section class="glass-panel block">
          <h2>{{ $t('Involved people') }}</h2>
          <ul v-if="event.people.length" class="people">
            <li v-for="p in event.people" :key="p.id">
              <RouterLink :to="`/characters/${p.id}`" class="person">
                <span class="avatar">
                  <img v-if="p.picture_path" :src="p.picture_path" :alt="p.name" />
                  <template v-else>{{ initials(p.name) }}</template>
                </span>
                {{ p.name }}
              </RouterLink>
            </li>
          </ul>
          <p v-else class="empty">{{ $t('Nobody listed.') }}</p>
        </section>
      </div>

      <section v-if="related.length" class="glass-panel block">
        <div class="block-head">
          <h2>{{ $t('Shares a tag with') }}</h2>
          <RouterLink :to="newWithTags" class="more">{{ $t('New event with these tags →') }}</RouterLink>
        </div>
        <ul class="related">
          <li v-for="r in related" :key="r.id">
            <RouterLink :to="`/events/${r.id}`" class="related-row">
              <span class="related-date">{{ relatedDate(r) }}</span>
              <strong>{{ r.name }}</strong>
              <span class="related-tags">
                <template v-for="t in r.tags.filter((x) => hasTag(x))" :key="t">#{{ t }} </template>
              </span>
            </RouterLink>
          </li>
        </ul>
      </section>
      <p v-else-if="event.tags.length" class="foot standalone">
        {{ $t('Nothing else uses these tags yet.') }}
        <RouterLink :to="newWithTags">{{ $t('Create a related event →') }}</RouterLink>
      </p>
    </template>

    <Teleport to="body">
      <ConfirmDialog
        v-if="confirmOpen"
        :title="event.source ? $t('Clear these details?') : $t('Delete {name}?', { name: event.name })"
        :message="event.source
          ? $t('The description, picture, tags and people are removed. The date itself stays on the timeline.')
          : $t('This removes it from the timeline too.')"
        :confirm-label="event.source ? $t('Clear') : $t('Delete')"
        @cancel="confirmOpen = false"
        @confirm="remove"
      />
    </Teleport>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import { eventsApi } from '../api'
import { shortDate, fullDate, dateParts } from '../calendar'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import BackLink from '../components/BackLink.vue'
import { chapterLabel, loadChapters } from '../chapters'
import { pageTitle, viewPicture, mapPath } from '../navigation'
import { t as tt } from '../i18n'

const props = defineProps({ id: { type: String, required: true } })
const router = useRouter()
loadChapters()

const event = ref(null)
const related = ref([])
const loadError = ref('')
const confirmOpen = ref(false)

const shown = computed(() => {
  const d = event.value?.event_date
  if (!dateParts(d)) return null
  return { short: shortDate(d), full: fullDate(d) }
})

// One whole sentence with a {link} slot, split so the link can sit inside it.
const importedParts = computed(() => tt('Imported from {link}. Its name and date come from there; everything else here is yours to fill in.', { link: '\u0001' }).split('\u0001'))

const hasTag = (t) => event.value.tags.some((x) => x.toLowerCase() === t.toLowerCase())

const newWithTags = computed(() => ({ path: '/events/new', query: { tag: event.value?.tags || [] } }))

function relatedDate(r) {
  return shortDate(r.event_date) || tt('undated')
}

function initials(name) {
  return (name || '').split(' ').filter(Boolean).map((p) => p[0]).join('').slice(0, 2).toUpperCase()
}

async function load() {
  loadError.value = ''
  event.value = null
  related.value = []
  try {
    event.value = await eventsApi.get(props.id)
    pageTitle.value = event.value.name
  } catch (err) {
    loadError.value = tt("Couldn't find that event.")
    return
  }
  // Everything else that shares at least one tag, de-duplicated.
  try {
    const lists = await Promise.all(event.value.tags.slice(0, 8).map((t) => eventsApi.list({ tag: t })))
    const seen = new Map()
    for (const list of lists) for (const e of list) if (e.id !== event.value.id) seen.set(e.id, e)
    related.value = [...seen.values()]
  } catch (err) {
    // Related events are a bonus; the page works without them.
  }
}

async function remove() {
  confirmOpen.value = false
  try {
    await eventsApi.remove(event.value.id)
    // Back to where you came from, unless that was a page about this event
    // (its wiki article), which is gone now too.
    const previous = router.options.history.state.back
    const gone = [`/events/${event.value.id}`, `/wiki/event/${event.value.id}`]
    const path = previous ? router.resolve(previous).path : ''
    if (previous && !gone.some((p) => path === p || path.startsWith(`${p}/`))) router.back()
    else router.replace('/events')
  } catch (err) {
    loadError.value = err.message || tt("Couldn't delete.")
  }
}

watch(() => props.id, load, { immediate: true })
</script>

<style scoped>
.event-page {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.back-link {
  margin-bottom: 0;
}

.hero {
  display: flex;
  gap: 1.5rem;
  padding: 1.5rem;
  align-items: flex-start;
}

.hero-pic {
  width: 220px;
  max-height: 260px;
  border-radius: 12px;
  object-fit: cover;
  flex-shrink: 0;
}

.hero-text {
  flex: 1;
  min-width: 0;
}

.when {
  display: flex;
  align-items: baseline;
  gap: 0.7rem;
}

.when-short {
  font-size: 0.82rem;
  font-weight: 700;
  letter-spacing: 0.05em;
  color: var(--accent);
}

.when-full {
  font-size: 0.82rem;
  color: var(--text-faint);
}

.hero h1 {
  font-family: 'Fraunces', serif;
  font-weight: 500;
  font-size: 2rem;
  line-height: 1.2;
  margin: 0.3rem 0 0.4rem;
}

.where {
  margin: 0 0 0.5rem;
  font-size: 0.9rem;
  color: var(--text-muted);
}

.where a,
.linked-note a {
  color: var(--accent);
}

.linked-note {
  margin: 0.4rem 0 0;
  font-size: 0.8rem;
  color: var(--text-faint);
  line-height: 1.5;
  max-width: 52ch;
}

.actions {
  display: flex;
  gap: 0.6rem;
  margin-top: 1.1rem;
}

.actions .btn {
  text-decoration: none;
}

.block {
  padding: 1.25rem 1.4rem;
}

.block h2 {
  font-family: 'Fraunces', serif;
  font-weight: 500;
  font-size: 1.05rem;
  margin: 0 0 0.7rem;
}

.block-head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 0.7rem;
}

.block-head h2 {
  margin: 0;
}

.more {
  font-size: 0.78rem;
  color: var(--text-muted);
  text-decoration: none;
}

.description {
  margin: 0;
  font-size: 0.94rem;
  line-height: 1.65;
  color: var(--text-primary);
  white-space: pre-wrap;
}

.empty {
  margin: 0;
  font-size: 0.86rem;
  color: var(--text-faint);
  font-style: italic;
}

.foot {
  margin: 0.8rem 0 0;
  font-size: 0.74rem;
  color: var(--text-faint);
}

.foot.standalone {
  margin: 0;
}

.foot a {
  color: var(--accent);
}

.two-col {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 1rem;
}

.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.45rem;
}

.tag-chip {
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--accent);
  background: var(--accent-soft);
  border: 1px solid color-mix(in srgb, var(--accent) 30%, transparent);
  border-radius: 999px;
  padding: 0.2rem 0.75rem;
  text-decoration: none;
}

.people {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.person {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  border: 1px solid var(--glass-border);
  background: rgba(255, 255, 255, 0.04);
  border-radius: 999px;
  padding: 0.25rem 0.8rem 0.25rem 0.3rem;
  font-size: 0.85rem;
  text-decoration: none;
}

.person:hover {
  background: rgba(255, 255, 255, 0.08);
}

.avatar {
  width: 26px;
  height: 26px;
  border-radius: 50%;
  overflow: hidden;
  display: grid;
  place-items: center;
  background: var(--accent-soft);
  color: var(--accent);
  font-size: 0.62rem;
  font-weight: 700;
}

.avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.related {
  list-style: none;
  margin: 0;
  padding: 0;
}

.related-row {
  display: flex;
  align-items: baseline;
  gap: 0.9rem;
  padding: 0.5rem 0.4rem;
  border-radius: 8px;
  text-decoration: none;
}

.related-row:hover {
  background: rgba(255, 255, 255, 0.05);
}

.related-date {
  width: 5.5rem;
  flex-shrink: 0;
  font-size: 0.78rem;
  font-weight: 700;
  color: var(--accent);
}

.related-row strong {
  font-size: 0.92rem;
  font-weight: 600;
}

.related-tags {
  margin-left: auto;
  font-size: 0.74rem;
  color: var(--text-faint);
}

@media (max-width: 720px) {
  .hero {
    flex-direction: column;
  }
  .hero-pic {
    width: 100%;
  }
}
</style>
