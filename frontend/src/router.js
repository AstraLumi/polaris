import { createRouter, createWebHistory } from 'vue-router'
import Home from './views/Home.vue'
import Characters from './views/Characters.vue'
import CharacterView from './views/CharacterView.vue'
import VersionList from './views/VersionList.vue'
import VersionEdit from './views/VersionEdit.vue'
import CharacterAssets from './views/CharacterAssets.vue'
import MapView from './views/MapView.vue'
import SettingsView from './views/SettingsView.vue'
import Events from './views/Events.vue'
import EventView from './views/EventView.vue'
import EventEdit from './views/EventEdit.vue'
import TimelineView from './views/TimelineView.vue'
import WikiHome from './views/WikiHome.vue'
import WikiArticle from './views/WikiArticle.vue'
import StoryPicker from './views/StoryPicker.vue'
import { activeStoryId, forgetStory, loadStories, stories } from './stories'

const routes = [
  // The picker is the one page that isn't inside a story (no sidebar).
  { path: '/stories', name: 'stories', component: StoryPicker, meta: { bare: true } },
  { path: '/', name: 'home', component: Home },
  { path: '/characters', name: 'characters', component: Characters },
  { path: '/characters/:id', name: 'character-view', component: CharacterView, props: true },
  { path: '/characters/:id/versions', name: 'version-list', component: VersionList, props: true },
  {
    path: '/characters/:id/versions/:versionId/edit',
    name: 'version-edit',
    component: VersionEdit,
    props: true,
  },
  { path: '/assets', name: 'assets', component: CharacterAssets },
  { path: '/map', name: 'map', component: MapView },
  { path: '/settings', name: 'settings', component: SettingsView },
  { path: '/timeline', name: 'timeline', component: TimelineView },
  { path: '/events', name: 'events', component: Events },
  // 'new' must be declared before ':id' so it isn't read as an event id.
  { path: '/events/new', name: 'event-new', component: EventEdit },
  { path: '/events/:id', name: 'event-view', component: EventView, props: true },
  { path: '/events/:id/edit', name: 'event-edit', component: EventEdit, props: true },
  { path: '/wiki', name: 'wiki', component: WikiHome },
  { path: '/wiki/:type/:id', name: 'wiki-article', component: WikiArticle, props: true },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// Every page except the picker needs an open story. The remembered one is
// checked once per load: if it was deleted (say, from another device), forget
// it and show the picker instead of a page full of errors.
let storyChecked = false
router.beforeEach(async (to) => {
  if (to.name === 'stories') return true
  if (!activeStoryId) return { name: 'stories' }
  if (!storyChecked) {
    storyChecked = true
    try {
      await loadStories()
      if (!stories.value.some((s) => s.id === activeStoryId)) {
        forgetStory()
        window.location.replace('/stories')
        return false
      }
    } catch (e) {
      /* can't verify right now; let the page load and report its own errors */
    }
  }
  return true
})

export default router
