<template>
  <router-view v-if="$route.meta.bare" />
  <div v-else class="shell" :class="{ 'nav-collapsed': collapsed }">
    <aside class="rail glass-panel" :class="{ 'is-open': menuOpen, 'is-collapsed': collapsed }">
      <div class="rail-top">
        <router-link to="/stories" class="story-head" :title="collapsed ? currentStory?.name : $t('Switch story')">
          <span class="story-icon">
            <IconImage :src="currentStory?.icon || ''" :name="currentStory?.name || ''" />
          </span>
          <span class="story-meta">
            <span class="story-name" data-tip-overflow>{{ currentStory?.name || '…' }}</span>
            <span class="story-app">{{ $t('Polaris') }}</span>
          </span>
          <NavIcon name="switch" class="story-switch-icon" />
        </router-link>
        <button
          type="button"
          class="menu-toggle"
          :aria-expanded="menuOpen"
          :aria-label="menuOpen ? $t('Close menu') : $t('Menu')"
          @click="menuOpen = !menuOpen"
        >
          <NavIcon :name="menuOpen ? 'close' : 'menu'" />
        </button>
      </div>

      <div class="rail-body">
        <button
          type="button"
          class="search-btn"
          :aria-label="$t('Search')"
          :title="collapsed ? $t('Search') + ' (' + shortcutLabel + ')' : $t('Find any character, place, event or article')"
          @click="searchOpen = true"
        >
          <NavIcon name="search" />
          <span class="search-label">{{ $t('Search') }}</span>
          <kbd>{{ shortcutLabel }}</kbd>
        </button>

        <nav ref="navEl" class="nav" :aria-label="$t('Pages')">
          <template v-for="group in navGroups" :key="group.label || 'top'">
            <div v-if="group.label" class="nav-group" role="presentation">
              <span class="nav-group-label">{{ $t(group.label) }}</span>
            </div>
            <router-link
              v-for="item in group.items"
              :key="item.to"
              :to="item.to"
              class="nav-item"
              :class="{ 'is-active': activeTo === item.to }"
              :aria-current="activeTo === item.to ? 'page' : undefined"
              :title="collapsed ? $t(item.label) : $t(item.tip)"
            >
              <NavIcon :name="item.icon" />
              <span class="nav-label">{{ $t(item.label) }}</span>
            </router-link>
          </template>
        </nav>

        <div class="rail-bottom">
          <router-link
            to="/settings"
            class="nav-item"
            :class="{ 'is-active': activeTo === '/settings' }"
            :aria-current="activeTo === '/settings' ? 'page' : undefined"
            :title="collapsed ? $t('Settings') : $t('Language, theme, calendar and stats')"
          >
            <NavIcon name="settings" />
            <span class="nav-label">{{ $t('Settings') }}</span>
          </router-link>
          <button
            type="button"
            class="nav-item collapse-btn"
            :aria-label="collapsed ? $t('Expand the sidebar') : $t('Collapse the sidebar')"
            :title="collapsed ? $t('Expand the sidebar') : $t('Collapse the sidebar')"
            @click="toggleCollapsed"
          >
            <NavIcon :name="collapsed ? 'expand' : 'collapse'" />
            <span class="nav-label">{{ $t('Collapse') }}</span>
          </button>
        </div>
      </div>
    </aside>
    <div v-if="menuOpen" class="nav-backdrop" @click="menuOpen = false"></div>

    <main class="stage">
      <router-view />
    </main>

    <SearchPalette v-if="searchOpen" @close="searchOpen = false" />
    <PictureViewer v-if="viewedPicture" :picture="viewedPicture" />
  </div>
  <TooltipLayer />
  <LinkPreviewLayer />
</template>

<script setup>
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount, watchEffect } from 'vue'
import { useRoute } from 'vue-router'
import { loadCalendar } from './calendar'
import { loadStatSystem } from './stats'
import { t, tr } from './i18n'
import { pageTitle, viewedPicture } from './navigation'
import NavIcon from './components/NavIcon.vue'
import IconImage from './components/IconImage.vue'
import SearchPalette from './components/SearchPalette.vue'
import TooltipLayer from './components/TooltipLayer.vue'
import LinkPreviewLayer from './components/LinkPreviewLayer.vue'
import PictureViewer from './components/PictureViewer.vue'
import { currentStory } from './stories'

const route = useRoute()

// On a phone the sidebar folds into a bar with a menu button that slides
// the pages in; any navigation closes them again.
const menuOpen = ref(false)
watch(() => route.fullPath, () => {
  menuOpen.value = false
  viewedPicture.value = null
})

const searchOpen = ref(false)
const isMac = /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent)
const shortcutLabel = isMac ? '⌘K' : 'Ctrl K'

onMounted(() => {
  loadCalendar()
  loadStatSystem()
})

// "Aria · Characters · My Story · Polaris", most specific first.
watchEffect(() => {
  const parts = [pageTitle.value, route.meta.title ? t(route.meta.title) : '', route.meta.bare ? '' : currentStory.value?.name, t('Polaris')]
  document.title = [...new Set(parts.filter(Boolean))].join(' · ')
})

// Escape closes the topmost dialog the same way clicking its backdrop does.
// Capture phase, so pages with their own Escape handling (the map, the
// timeline) don't also react to the key that closed a dialog.
function onKeyDown(e) {
  // Ctrl+K (Cmd+K) opens the search from anywhere inside a story.
  if ((e.ctrlKey || e.metaKey) && !e.altKey && e.key.toLowerCase() === 'k' && !route.meta.bare) {
    e.preventDefault()
    searchOpen.value = !searchOpen.value
    return
  }
  if (e.key !== 'Escape' || e.defaultPrevented) return
  const overlays = document.querySelectorAll('.overlay')
  if (!overlays.length) return
  e.preventDefault()
  e.stopPropagation()
  overlays[overlays.length - 1].dispatchEvent(new MouseEvent('click', { bubbles: true }))
}
onMounted(() => window.addEventListener('keydown', onKeyDown, true))
onBeforeUnmount(() => window.removeEventListener('keydown', onKeyDown, true))

// The pages, in groups. The tip is the tooltip while the sidebar is open;
// collapsed, the tooltip is the page's name.
const navGroups = [
  { items: [{ label: tr('Home'), to: '/', icon: 'home', tip: tr('Recent edits and loose ends') }] },
  {
    label: tr('Story'),
    items: [
      { label: tr('Characters'), to: '/characters', icon: 'characters', tip: tr('Everyone in the story, with their sheets and versions') },
      { label: tr('Factions'), to: '/factions', icon: 'factions', tip: tr('Guilds, orders, houses and who belongs to them') },
      { label: tr('Chapters'), to: '/chapters', icon: 'chapters', tip: tr('Volumes and chapters, with your notes and what happens in each') },
    ],
  },
  {
    label: tr('World'),
    items: [
      { label: tr('Map'), to: '/map', icon: 'map', tip: tr('The hex map of kingdoms and places') },
      { label: tr('Solar system'), to: '/solar-system', icon: 'solar', tip: tr('The star, planets and moons round your world') },
      { label: tr('Bestiary'), to: '/bestiary', icon: 'bestiary', tip: tr('The monsters, animals and constructs of the world') },
      { label: tr('Character Assets'), to: '/assets', icon: 'assets', tip: tr('Classes, races, spells, gear and the rest a character is built from') },
    ],
  },
  {
    label: tr('Time'),
    items: [
      { label: tr('Timeline'), to: '/timeline', icon: 'timeline', tip: tr('Events, births and foundings, drawn to scale') },
      { label: tr('Calendar'), to: '/calendar', icon: 'calendar', tip: tr('What happens in each month, day by day') },
      { label: tr('Events'), to: '/events', icon: 'events', tip: tr('Everything that happens, with dates, places and people') },
    ],
  },
  {
    label: tr('Lore'),
    items: [
      { label: tr('Wiki'), to: '/wiki', icon: 'wiki', tip: tr('An article for everything in the story, with your lore on top') },
      { label: tr('Graph'), to: '/wiki/graph', icon: 'graph', tip: tr('Every wiki article and how they connect') },
    ],
  },
]

// The page being shown: the longest page path the address starts with, so
// a wiki article lights up Wiki but the graph lights up only Graph.
const allPaths = [...navGroups.flatMap((g) => g.items.map((i) => i.to)), '/settings']
// When the list has to scroll, the page being shown stays in view.
const navEl = ref(null)
watch(
  () => route.path,
  () => nextTick(() => navEl.value?.querySelector('.is-active')?.scrollIntoView({ block: 'nearest' })),
  { immediate: true },
)

const activeTo = computed(() => {
  const p = route.path
  return allPaths
    .filter((to) => (to === '/' ? p === '/' : p === to || p.startsWith(to + '/')))
    .sort((a, b) => b.length - a.length)[0]
})

// Icons only, remembered on this device.
const COLLAPSE_KEY = 'polaris-nav-collapsed'
const collapsed = ref(false)
try {
  collapsed.value = localStorage.getItem(COLLAPSE_KEY) === '1'
} catch (e) {
  /* storage unavailable: start open */
}
function toggleCollapsed() {
  collapsed.value = !collapsed.value
  try {
    localStorage.setItem(COLLAPSE_KEY, collapsed.value ? '1' : '0')
  } catch (e) {
    /* not remembered, still toggled */
  }
}
</script>

<style scoped>
.shell {
  display: flex;
  min-height: 100vh;
  gap: 1.5rem;
  padding: 1.5rem;
  align-items: stretch;
}

/* The sidebar fills the window's height and scrolls its page list on its
   own, so Settings stays reachable on short screens. */
.rail {
  width: 240px;
  flex-shrink: 0;
  height: calc(100vh - 3rem);
  position: sticky;
  top: 1.5rem;
  padding: 0.75rem 0.75rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  transition: width 0.18s ease;
}

.rail.is-collapsed {
  width: 64px;
  padding-inline: 0.55rem;
}

.rail-top {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.rail-body {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

/* ---- story header ---- */
.story-head {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 0.65rem;
  padding: 0.45rem;
  border-radius: 10px;
  text-decoration: none;
  color: inherit;
}

.story-head:hover {
  background: rgba(255, 255, 255, 0.05);
}

.story-icon {
  width: 2.1rem;
  height: 2.1rem;
  flex-shrink: 0;
  border-radius: 8px;
  background: var(--accent-soft);
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
}

.story-meta {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  line-height: 1.25;
}

.story-name {
  font-size: 0.92rem;
  font-weight: 600;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.story-app {
  font-size: 0.66rem;
  font-weight: 600;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: var(--accent);
}

.story-switch-icon {
  color: var(--text-faint);
}

.story-head:hover .story-switch-icon {
  color: var(--text-primary);
}

/* ---- search ---- */
.search-btn {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  padding: 0.5rem 0.65rem;
  border-radius: 8px;
  border: 1px solid var(--glass-border);
  background: rgba(255, 255, 255, 0.03);
  color: var(--text-faint);
  font: inherit;
  font-size: 0.85rem;
  cursor: pointer;
}

.search-label {
  flex: 1;
  text-align: left;
}

.search-btn:hover {
  color: var(--text-primary);
  border-color: color-mix(in srgb, var(--accent) 35%, transparent);
}

.search-btn kbd {
  font-family: inherit;
  font-size: 0.68rem;
  padding: 0.05rem 0.35rem;
  border: 1px solid var(--glass-border);
  border-radius: 4px;
  color: var(--text-faint);
}

/* ---- pages ---- */
.nav {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin-inline: -0.25rem;
  padding-inline: 0.25rem;
  scrollbar-width: thin;
  scrollbar-color: var(--glass-border) transparent;
}

.nav-group {
  padding: 0.55rem 0.65rem 0.2rem;
}

.nav-group-label {
  font-size: 0.68rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--text-faint);
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 0.7rem;
  width: 100%;
  padding: 0.38rem 0.65rem;
  border: none;
  border-radius: 8px;
  background: transparent;
  font: inherit;
  font-size: 0.9rem;
  color: var(--text-muted);
  text-decoration: none;
  text-align: left;
  cursor: pointer;
  transition: background 0.12s ease, color 0.12s ease;
}

.nav-item:hover {
  color: var(--text-primary);
  background: rgba(255, 255, 255, 0.05);
}

.nav-item.is-active {
  color: var(--text-primary);
  background: var(--accent-soft);
}

.nav-item.is-active .nav-icon {
  color: var(--accent);
}

.nav-item:focus-visible,
.search-btn:focus-visible,
.story-head:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 1px;
}

.nav-label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rail-bottom {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding-top: 0.6rem;
  border-top: 1px solid var(--glass-border);
}

/* ---- collapsed: icons only ---- */
.is-collapsed .story-head {
  justify-content: center;
  padding: 0.35rem 0;
}

.is-collapsed .story-meta,
.is-collapsed .story-switch-icon,
.is-collapsed .search-label,
.is-collapsed .search-btn kbd,
.is-collapsed .nav-label,
.is-collapsed .nav-group-label {
  display: none;
}

.is-collapsed .search-btn,
.is-collapsed .nav-item {
  justify-content: center;
  padding-inline: 0;
}

/* Group headings become thin dividers. */
.is-collapsed .nav-group {
  padding: 0.45rem 0.6rem;
}

.is-collapsed .nav-group::after {
  content: '';
  display: block;
  border-top: 1px solid var(--glass-border);
}

.stage {
  flex: 1;
  min-width: 0;
}

.menu-toggle,
.nav-backdrop {
  display: none;
}

/* Phones: a bar across the top with the story and a menu button; the menu
   slides in from the left over the page. */
@media (max-width: 720px) {
  .shell {
    flex-direction: column;
    gap: 0.75rem;
    padding: 0.75rem;
  }
  .rail,
  .rail.is-collapsed {
    width: auto;
    height: auto;
    top: 0.5rem;
    z-index: 40;
    padding: 0.4rem 0.5rem;
    /* A blur would trap the fixed menu inside the bar. */
    background: color-mix(in srgb, var(--surface) 94%, transparent);
    backdrop-filter: none;
    -webkit-backdrop-filter: none;
  }
  .is-collapsed .story-meta,
  .is-collapsed .nav-label,
  .is-collapsed .search-label,
  .is-collapsed .nav-group-label {
    display: revert;
  }
  .menu-toggle {
    display: grid;
    place-items: center;
    width: 2.4rem;
    height: 2.4rem;
    border-radius: 8px;
    border: 1px solid var(--glass-border);
    background: transparent;
    color: var(--text-primary);
    cursor: pointer;
  }
  .story-switch-icon,
  .collapse-btn,
  .search-btn kbd {
    display: none;
  }
  .rail-body {
    position: fixed;
    inset: 0 auto 0 0;
    width: min(290px, 85vw);
    padding: 1rem 0.75rem;
    background: var(--surface);
    border-right: 1px solid var(--glass-border);
    transform: translateX(-100%);
    transition: transform 0.2s ease;
    z-index: 61;
  }
  .rail.is-open .rail-body {
    transform: none;
  }
  .nav-backdrop {
    display: block;
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    z-index: 39;
  }
}
</style>
