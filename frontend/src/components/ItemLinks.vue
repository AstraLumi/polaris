<template>
  <nav v-if="links.length" class="item-links no-print" :aria-label="$t('Other pages about this')">
    <RouterLink v-for="l in links" :key="l.key" :to="l.to" class="item-link" :title="l.title">{{ l.label }}</RouterLink>
  </nav>
</template>

<script setup>
// The same links, in the same order, on every page about one item (the
// character sheet, its family tree, an event, any wiki article): Sheet ·
// Wiki · Map · Timeline · Family tree · Graph. Only those the item has, and
// never the page you're on (`here`).
import { computed } from 'vue'
import { t } from '../i18n'
import { mapPath } from '../navigation'
import { TYPE_ORDER, wikiPath } from '../wiki'

const props = defineProps({
  type: { type: String, required: true }, // a wiki type: character, location, event…
  id: { type: [String, Number], required: true },
  here: { type: String, default: '' }, // sheet, wiki, map, timeline, family or graph
  locationId: { type: Number, default: null }, // an event's place, for its map link
  locationName: { type: String, default: '' },
})

const links = computed(() => {
  const { type, here } = props
  const id = String(props.id)
  const out = []
  const add = (key, label, to, title = '') => {
    if (to && key !== here) out.push({ key, label, to, title })
  }
  if (type === 'character') add('sheet', t('Sheet'), `/characters/${id}`, t('Open the character sheet'))
  if (type === 'event') add('sheet', t('Event'), `/events/${id}`)
  if (TYPE_ORDER.includes(type)) add('wiki', t('Wiki'), wikiPath(type, id))
  if (type === 'location') add('map', t('Map'), mapPath(id))
  if (type === 'event' && props.locationId) add('map', t('Map'), mapPath(props.locationId), props.locationName)
  if (type === 'character') add('timeline', t('Timeline'), { path: '/timeline', query: { person: id, match: 'highlight' } })
  if (type === 'location') add('timeline', t('Timeline'), { path: '/timeline', query: { place: id, match: 'highlight' } })
  if (type === 'event') add('timeline', t('Timeline'), { path: '/timeline', query: { event: id } })
  if (type === 'character') add('family', t('Family tree'), `/characters/${id}/family`, t('Parents, children, siblings and partners, as a tree'))
  if (TYPE_ORDER.includes(type)) {
    add('graph', t('Graph'), { path: '/wiki/graph', query: { focus: `${type}:${id}` } }, t('This article and everything it connects to, on the graph'))
  }
  return out
})
</script>

<style scoped>
.item-links {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

.item-link {
  padding: 0.2rem 0.7rem;
  border: 1px solid var(--glass-border);
  border-radius: 999px;
  font-size: 0.78rem;
  color: var(--text-muted);
  text-decoration: none;
  white-space: nowrap;
}

.item-link:hover,
.item-link:focus-visible {
  color: var(--accent);
  border-color: color-mix(in srgb, var(--accent) 45%, transparent);
}

/* Phones: tighter, so the usual five fit on one line. */
@media (max-width: 600px) {
  .item-links {
    gap: 0.25rem;
  }
  .item-link {
    padding: 0.15rem 0.5rem;
    font-size: 0.74rem;
  }
}
</style>
