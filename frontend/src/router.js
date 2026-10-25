import { createRouter, createWebHistory } from 'vue-router'
import Home from './views/Home.vue'
import Characters from './views/Characters.vue'
import CharacterView from './views/CharacterView.vue'
import VersionList from './views/VersionList.vue'
import FamilyTree from './views/FamilyTree.vue'
import VersionEdit from './views/VersionEdit.vue'
import VersionCompare from './views/VersionCompare.vue'
import CharacterAssets from './views/CharacterAssets.vue'
import MapPage from './views/MapPage.vue'
import SettingsView from './views/SettingsView.vue'
import Events from './views/Events.vue'
import EventView from './views/EventView.vue'
import EventEdit from './views/EventEdit.vue'
import TimelineView from './views/TimelineView.vue'
import CalendarView from './views/CalendarView.vue'
import WikiHome from './views/WikiHome.vue'
import WikiArticle from './views/WikiArticle.vue'
import GraphView from './views/GraphView.vue'
import StoryPicker from './views/StoryPicker.vue'
import Factions from './views/Factions.vue'
import Bestiary from './views/Bestiary.vue'
import Chapters from './views/Chapters.vue'
import ChapterView from './views/ChapterView.vue'
import NotFound from './views/NotFound.vue'
import { activeStoryId, forgetStory, loadStories, stories } from './stories'
import { tr } from './i18n'
import { pageTitle, rememberVisit } from './navigation'

const routes = [
  // The picker is the one page that isn't inside a story (no sidebar).
  { path: '/stories', name: 'stories', component: StoryPicker, meta: { bare: true, title: tr('Stories') } },
  { path: '/', name: 'home', component: Home, meta: { title: tr('Home') } },
  { path: '/characters', name: 'characters', component: Characters, meta: { title: tr('Characters') } },
  { path: '/characters/heights', name: 'height-chart', component: () => import('./views/HeightChart.vue'), meta: { title: tr('Height chart') } },
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
  { path: '/map', name: 'map', component: MapPage, meta: { title: tr('Map') } },
  // Loaded on its own, with three.js, only when opened.
  { path: '/solar-system', name: 'solar-system', component: () => import('./views/SolarSystem.vue'), meta: { title: tr('Solar system') } },
  { path: '/settings', name: 'settings', component: SettingsView, meta: { title: tr('Settings') } },
  { path: '/timeline', name: 'timeline', component: TimelineView, meta: { title: tr('Timeline') } },
  { path: '/calendar', name: 'calendar', component: CalendarView, meta: { title: tr('Calendar') } },
  { path: '/events', name: 'events', component: Events, meta: { title: tr('Events') } },
  // 'new' must be declared before ':id' so it isn't read as an event id.
  { path: '/events/new', name: 'event-new', component: EventEdit, meta: { title: tr('New event') } },
  { path: '/events/:id', name: 'event-view', component: EventView, meta: { title: tr('Events') }, props: true },
  { path: '/events/:id/edit', name: 'event-edit', component: EventEdit, meta: { title: tr('Edit event') }, props: true },
  { path: '/factions', name: 'factions', component: Factions, meta: { title: tr('Factions') } },
  { path: '/bestiary', name: 'bestiary', component: Bestiary, meta: { title: tr('Bestiary') } },
  { path: '/chapters', name: 'chapters', component: Chapters, meta: { title: tr('Chapters') } },
  { path: '/chapters/:id', name: 'chapter', component: ChapterView, meta: { title: tr('Chapters') }, props: true },
  // The writing editor: loaded on its own, with ProseMirror, only when opened.
  // :volume is a volume's id, or 0 for the chapters without one.
  { path: '/write/:volume', name: 'write', component: () => import('./views/WritePage.vue'), meta: { title: tr('Write') }, props: true },
  { path: '/write/:volume/pages', name: 'write-pages', component: () => import('./views/BookPagesPage.vue'), meta: { title: tr('Page view') }, props: true },
  { path: '/wiki', name: 'wiki', component: WikiHome, meta: { title: tr('Wiki') } },
  { path: '/wiki/graph', name: 'wiki-graph', component: GraphView, meta: { title: tr('Graph') } },
  { path: '/wiki/:type/:id', name: 'wiki-article', component: WikiArticle, meta: { title: tr('Wiki') }, props: true },
  // Anything else: a mistyped address or an old link to something removed.
  { path: '/:pathMatch(.*)*', name: 'not-found', component: NotFound, meta: { title: tr('Not found') } },
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
  if (!failure) rememberVisit(to)
})

export default router
