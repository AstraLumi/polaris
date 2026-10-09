<template>
  <router-view v-if="$route.meta.bare" />
  <div v-else class="shell">
    <aside class="rail glass-panel" :class="{ 'is-open': menuOpen }">
      <div class="brand">
        <PolarisLogo class="brand-mark" />
        <span class="brand-name">{{ $t('Polaris') }}</span>
        <button
          type="button"
          class="menu-toggle"
          :aria-expanded="menuOpen"
          :aria-label="$t('Menu')"
          @click="menuOpen = !menuOpen"
        >
          {{ menuOpen ? '✕' : '☰' }}
        </button>
      </div>

      <router-link to="/stories" class="story-switch" :title="$t('Switch story')">
        <span class="story-icon">
          <IconImage :src="currentStory?.icon || ''" :name="currentStory?.name || ''" />
        </span>
        <span class="story-meta">
          <span class="story-name">{{ currentStory?.name || '…' }}</span>
          <span class="story-action">{{ $t('Switch story') }}</span>
        </span>
      </router-link>

      <button type="button" class="search-btn" @click="searchOpen = true">
        <span>{{ $t('Search') }}</span>
        <kbd>{{ shortcutLabel }}</kbd>
      </button>

      <nav class="nav">
        <template v-for="item in navItems" :key="item.label">
          <router-link
            v-if="item.to"
            :to="item.to"
            class="nav-item"
            active-class="is-active"
          >
            <span class="nav-label">{{ $t(item.label) }}</span>
          </router-link>
          <span v-else class="nav-item is-disabled">
            <span class="nav-label">{{ $t(item.label) }}</span>
            <span class="nav-status">{{ $t('not built yet') }}</span>
          </span>
        </template>
      </nav>
    </aside>

    <main class="stage">
      <router-view />
    </main>

    <SearchPalette v-if="searchOpen" @close="searchOpen = false" />
    <PictureViewer v-if="viewedPicture" :picture="viewedPicture" />
  </div>
</template>

<script setup>
import { ref, watch, onMounted, onBeforeUnmount, watchEffect } from 'vue'
import { useRoute } from 'vue-router'
import { loadCalendar } from './calendar'
import { t, tr } from './i18n'
import { pageTitle, viewedPicture } from './navigation'
import PolarisLogo from './components/PolarisLogo.vue'
import IconImage from './components/IconImage.vue'
import SearchPalette from './components/SearchPalette.vue'
import PictureViewer from './components/PictureViewer.vue'
import { currentStory } from './stories'

const route = useRoute()

// On a phone the sidebar folds into a bar with a menu button; any
// navigation closes the menu again.
const menuOpen = ref(false)
watch(() => route.fullPath, () => {
  menuOpen.value = false
  viewedPicture.value = null
})

const searchOpen = ref(false)
const isMac = /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent)
const shortcutLabel = isMac ? '⌘K' : 'Ctrl K'

onMounted(loadCalendar)

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

const navItems = [
  { label: tr('Home'), to: '/' },
  { label: tr('Characters'), to: '/characters' },
  { label: tr('Character Assets'), to: '/assets' },
  { label: tr('Timeline'), to: '/timeline' },
  { label: tr('Map'), to: '/map' },
  { label: tr('Events'), to: '/events' },
  { label: tr('Wiki'), to: '/wiki' },
  { label: tr('Settings'), to: '/settings' },
]
</script>

<style scoped>
.shell {
  display: flex;
  min-height: 100vh;
  gap: 1.5rem;
  padding: 1.5rem;
  align-items: stretch;
}

.rail {
  width: 240px;
  flex-shrink: 0;
  padding: 1.75rem 1.25rem;
  display: flex;
  flex-direction: column;
  gap: 2.5rem;
  align-self: flex-start;
  position: sticky;
  top: 1.5rem;
}

.brand {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.brand-mark {
  height: 2.6rem;
  flex-shrink: 0;
}

.brand-name {
  font-size: 1.1rem;
  font-weight: 600;
  letter-spacing: 0.22em;
  text-transform: uppercase;
  color: var(--accent);
}

.story-switch {
  display: flex;
  align-items: center;
  gap: 0.7rem;
  padding: 0.55rem 0.6rem;
  margin: -1.4rem 0 -0.6rem;
  border-radius: 10px;
  border: 1px solid var(--glass-border);
  background: rgba(255, 255, 255, 0.03);
  text-decoration: none;
  color: inherit;
}

.story-switch:hover {
  background: var(--accent-soft);
  border-color: color-mix(in srgb, var(--accent) 35%, transparent);
}

.story-icon {
  width: 2.2rem;
  height: 2.2rem;
  flex-shrink: 0;
  border-radius: 8px;
  background: var(--accent-soft);
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
}

.story-meta {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.story-name {
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.story-action {
  font-size: 0.72rem;
  color: var(--text-faint);
}

.story-switch:hover .story-action {
  color: var(--accent);
}

.nav {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.nav-item {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  padding: 0.6rem 0.75rem;
  border-radius: 8px;
  border-left: 2px solid transparent;
  font-size: 0.92rem;
  color: var(--text-muted);
  text-decoration: none;
}

.nav-item.is-active {
  color: var(--text-primary);
  border-left-color: var(--accent);
  background: var(--accent-soft);
}

.nav-item.is-disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.nav-status {
  font-size: 0.7rem;
  color: var(--text-faint);
}

.stage {
  flex: 1;
  min-width: 0;
}

.search-btn {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 0.5rem;
  margin: -1.2rem 0 -1rem;
  padding: 0.5rem 0.75rem;
  border-radius: 8px;
  border: 1px solid var(--glass-border);
  background: rgba(255, 255, 255, 0.03);
  color: var(--text-muted);
  font: inherit;
  font-size: 0.85rem;
  cursor: pointer;
}

.search-btn:hover {
  color: var(--text-primary);
  border-color: color-mix(in srgb, var(--accent) 35%, transparent);
}

.search-btn kbd {
  font-family: inherit;
  font-size: 0.7rem;
  color: var(--text-faint);
}

.menu-toggle {
  display: none;
  margin-left: auto;
  width: 2.4rem;
  height: 2.4rem;
  border-radius: 8px;
  border: 1px solid var(--glass-border);
  background: transparent;
  color: var(--text-primary);
  font-size: 1.1rem;
  cursor: pointer;
}

/* Phones: the sidebar becomes a bar across the top; its menu button shows
   the story switch, search and pages. */
@media (max-width: 720px) {
  .shell {
    flex-direction: column;
    gap: 0.75rem;
    padding: 0.75rem;
  }
  .rail {
    width: auto;
    position: sticky;
    top: 0.5rem;
    z-index: 40;
    padding: 0.6rem 0.75rem;
    gap: 0.9rem;
    align-self: stretch;
  }
  .brand-mark {
    height: 2rem;
  }
  .menu-toggle {
    display: block;
  }
  .rail:not(.is-open) .story-switch,
  .rail:not(.is-open) .search-btn,
  .rail:not(.is-open) .nav {
    display: none;
  }
  .story-switch,
  .search-btn {
    margin: 0;
  }
  .search-btn kbd {
    display: none;
  }
}
</style>
