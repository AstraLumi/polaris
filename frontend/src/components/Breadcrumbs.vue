<template>
  <div class="page-trail no-print">
    <nav class="crumbs" :aria-label="$t('Breadcrumbs')">
      <ol>
        <li v-for="(c, i) in shown" :key="i" :class="{ 'is-first': i === 0, 'is-last': i === shown.length - 1 }">
          <a v-if="c.to && !c.here" :href="hrefOf(c)" :title="c.label" @click="go($event, c)">{{ c.label }}</a>
          <span v-else :title="c.label" :aria-current="c.here ? 'page' : undefined">{{ c.label }}</span>
        </li>
      </ol>
    </nav>
    <div v-if="$slots.default" class="trail-side"><slot /></div>
  </div>
</template>

<script setup>
// Where a page sits: "Characters / Aria Vale / Versions / Edit". Every crumb
// but the last is a link. A crumb that leads to the page you came from goes
// back to it (as the browser's Back does, keeping its scroll and filters);
// otherwise a list opens with the filters it last had (lastVisitOf). The
// default slot sits at the other end of the row (the page's ItemLinks).
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { t } from '../i18n'
import { lastVisitOf } from '../navigation'

const props = defineProps({
  // [{ label, to }]: labels already translated; `to` a path or a route
  // object, or nothing for a crumb that isn't a page (a chapter's volume).
  crumbs: { type: Array, required: true },
  // The page's item doesn't exist: its own crumb reads "Not found".
  missing: { type: Boolean, default: false },
})

const route = useRoute()
const router = useRouter()

// The last crumb is the page itself, not a link. A crumb whose name hasn't
// loaded yet is left out until it has.
const shown = computed(() => {
  const list = props.crumbs.map((c, i) => ({ ...c, here: i === props.crumbs.length - 1 }))
  if (props.missing) list[list.length - 1] = { label: t('Not found'), here: true }
  return list.filter((c) => c.label)
})

// vue-router keeps the previous in-app location in history.state.back (null
// on a fresh load); reading route makes this recompute when the same view
// is reused for another item.
const previous = computed(() => {
  void route.fullPath
  return router.options.history.state.back || ''
})

// A plain path matches whatever filters the page had; a route object with a
// query (the wiki's "Characters") only that same query.
function isPrevious(c) {
  if (!previous.value) return false
  const back = router.resolve(previous.value)
  const to = router.resolve(c.to)
  return typeof c.to === 'string' ? back.path === to.path : back.fullPath === to.fullPath
}
// A plain path opens as it was last seen; a route object is used as given.
const target = (c) => (typeof c.to === 'string' ? lastVisitOf(c.to) : c.to)
const hrefOf = (c) => router.resolve(isPrevious(c) ? previous.value : target(c)).href

function go(e, c) {
  if (e.button !== 0 || e.ctrlKey || e.metaKey || e.shiftKey || e.altKey) return // new tab and friends
  e.preventDefault()
  if (isPrevious(c)) router.back()
  else router.push(target(c))
}
</script>

<style scoped>
.page-trail {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 0.5rem 1rem;
  margin-bottom: 1rem;
  min-height: 1.6rem;
}

.crumbs {
  flex: 1 1 14rem;
  min-width: 0;
}

.crumbs ol {
  display: flex;
  align-items: center;
  min-width: 0;
  margin: 0;
  padding: 0;
  list-style: none;
  font-size: 0.85rem;
  color: var(--text-faint);
}

.crumbs li {
  display: flex;
  align-items: center;
  min-width: 0;
  white-space: nowrap;
}

.crumbs li + li::before {
  content: '/';
  flex-shrink: 0;
  margin: 0 0.45rem;
  opacity: 0.6;
}

.crumbs a,
.crumbs span {
  display: block;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}

.crumbs a {
  color: var(--text-faint);
  text-decoration: none;
}

.crumbs a:hover,
.crumbs a:focus-visible {
  color: var(--text-primary);
  text-decoration: underline;
}

/* The list keeps its name; the middle gives way first, then the page's own. */
.crumbs li.is-first {
  flex-shrink: 0;
}

.crumbs li:not(.is-first):not(.is-last) {
  flex-shrink: 2;
  max-width: 16rem;
}

.crumbs li.is-last {
  flex-shrink: 1;
  color: var(--text-muted);
}

.trail-side {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

@media (max-width: 600px) {
  .crumbs li:not(.is-first):not(.is-last) {
    max-width: 7.5rem;
  }
}
</style>
