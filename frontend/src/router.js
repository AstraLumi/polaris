import { createRouter, createWebHistory } from 'vue-router'
import Home from './views/Home.vue'
import Characters from './views/Characters.vue'
import CharacterView from './views/CharacterView.vue'
import VersionList from './views/VersionList.vue'
import FamilyTree from './views/FamilyTree.vue'
import VersionEdit from './views/VersionEdit.vue'
import VersionCompare from './views/VersionCompare.vue'
import CharacterAssets from './views/CharacterAssets.vue'
import MapView from './views/MapView.vue'
import SettingsView from './views/SettingsView.vue'
import Events from './views/Events.vue'
import EventView from './views/EventView.vue'
import EventEdit from './views/EventEdit.vue'
import TimelineView from './views/TimelineView.vue'
import WikiHome from './views/WikiHome.vue'
import WikiArticle from './views/WikiArticle.vue'
import GraphView from './views/GraphView.vue'
import StoryPicker from './views/StoryPicker.vue'
import Factions from './views/Factions.vue'
import Chapters from './views/Chapters.vue'
import ChapterView from './views/ChapterView.vue'
import { activeStoryId, forgetStory, loadStories, stories } from './stories'
import { tr } from './i18n'
import { pageTitle } from './navigation'

const routes = [
  // The picker is the one page that isn't inside a story (no sidebar).
  { path: '/stories', name: 'stories', component: StoryPicker, meta: { bare: true, title: tr('Stories') } },
  { path: '/', name: 'home', component: Home, meta: { title: tr('Home') } },
  { path: '/characters', name: 'characters', component: Characters, meta: { title: tr('Characters') } },
  { path: '/characters/:id', name: 'character-view', component: CharacterView, meta: { title: tr('Characters') }, props: true },
  { path: '/characters/:id/family', name: 'family-tree', component: FamilyTree, meta: { title: tr('Family tree') }, props: true },
  { path: '/characters/:id/versions', name: 'version-list', component: VersionList, meta: { title: tr('Versions') }, props: true },
  {
    path: '/characters/:id/compare',
    name: 'version-compare',
    component: VersionCompare,
    meta: { title: tr('Compare versions') },
    props: true,
  },
  {
    path: '/characters/:id/versions/:versionId/edit',
    name: 'version-edit',
    component: VersionEdit,
    meta: { title: tr('Edit version') },
    props: true,
  },
  { path: '/assets', name: 'assets', component: CharacterAssets, meta: { title: tr('Character Assets') } },
  { path: '/map', name: 'map', component: MapView, meta: { title: tr('Map') } },
  { path: '/settings', name: 'settings', component: SettingsView, meta: { title: tr('Settings') } },
  { path: '/timeline', name: 'timeline', component: TimelineView, meta: { title: tr('Timeline') } },
  { path: '/events', name: 'events', component: Events, meta: { title: tr('Events') } },
  // 'new' must be declared before ':id' so it isn't read as an event id.
  { path: '/events/new', name: 'event-new', component: EventEdit, meta: { title: tr('New event') } },
  { path: '/events/:id', name: 'event-view', component: EventView, meta: { title: tr('Events') }, props: true },
  { path: '/events/:id/edit', name: 'event-edit', component: EventEdit, meta: { title: tr('Edit event') }, props: true },
  { path: '/factions', name: 'factions', component: Factions, meta: { title: tr('Factions') } },
  { path: '/chapters', name: 'chapters', component: Chapters, meta: { title: tr('Chapters') } },
  { path: '/chapters/:id', name: 'chapter', component: ChapterView, meta: { title: tr('Chapters') }, props: true },
  { path: '/wiki', name: 'wiki', component: WikiHome, meta: { title: tr('Wiki') } },
  { path: '/wiki/graph', name: 'wiki-graph', component: GraphView, meta: { title: tr('Graph') } },
  { path: '/wiki/:type/:id', name: 'wiki-article', component: WikiArticle, meta: { title: tr('Wiki') }, props: true },
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

// A detail page names what it shows once it has loaded it (see pageTitle).
router.afterEach((to, from, failure) => {
  if (!failure) pageTitle.value = ''
})

export default router
