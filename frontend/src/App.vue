<template>
  <router-view v-if="$route.meta.bare" />
  <div v-else class="shell">
    <aside class="rail glass-panel">
      <div class="brand">
        <PolarisLogo class="brand-mark" />
        <span class="brand-name">{{ $t('Polaris') }}</span>
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
  </div>
</template>

<script setup>
import { onMounted } from 'vue'
import { loadCalendar } from './calendar'
import { tr } from './i18n'
import PolarisLogo from './components/PolarisLogo.vue'
import IconImage from './components/IconImage.vue'
import { currentStory } from './stories'

onMounted(loadCalendar)

const navItems = [
  { label: tr('Home'), to: '/' },
  { label: tr('Characters'), to: '/characters' },
  { label: tr('Character Assets'), to: '/assets' },
  { label: tr('Timeline'), to: '/timeline' },
  { label: tr('Map'), to: '/map' },
  { label: tr('Events'), to: '/events' },
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

@media (max-width: 720px) {
  .shell {
    flex-direction: column;
  }
  .rail {
    width: auto;
    position: static;
  }
}
</style>
