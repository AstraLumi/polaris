import { createRouter, createWebHistory } from 'vue-router'
import Home from './views/Home.vue'
import StoryPicker from './views/StoryPicker.vue'
import NotFound from './views/NotFound.vue'
import { activeStoryId, forgetStory, loadStories, stories } from './stories'
import { tr } from './i18n'
import { pageTitle, rememberVisit, pageLoadStarted, pageLoadEnded } from './navigation'
import { fetchHome } from './api'
import { lastWriting, writePath } from './writeLast'

// Only the pages every visit starts on are part of the main script; each
// other page is fetched the first time it is opened.
// The sidebar's Write: the chapter written last, on any device; without a
// connection, the one this device was on; else the chapters with no volume.
async function resumeWriting() {
  try {
    const w = (await fetchHome()).writing
    if (w) return writePath({ volume: w.volume_id, chapter: w.chapter_id })
  } catch (e) {
    /* can't ask: go by what this device remembers */
  }
  const kept = lastWriting(activeStoryId)
  return kept ? writePath(kept) : '/write/0'
}

const routes = [
  // The picker is the one page that isn't inside a story (no sidebar).
  { path: '/stories', name: 'stories', component: StoryPicker, meta: { bare: true, title: tr('Stories') } },
  { path: '/', name: 'home', component: Home, meta: { title: tr('Home') } },
  { path: '/characters', name: 'characters', component: () => import('./views/Characters.vue'), meta: { title: tr('Characters') } },
  { path: '/characters/heights', name: 'height-chart', component: () => import('./views/HeightChart.vue'), meta: { title: tr('Height chart') } },
  { path: '/characters/:id', name: 'character-view', component: () => import('./views/CharacterView.vue'), meta: { title: tr('Characters') }, props: true },
  { path: '/characters/:id/family', name: 'family-tree', component: () => import('./views/FamilyTree.vue'), meta: { title: tr('Family tree') }, props: true },
  { path: '/characters/:id/versions', name: 'version-list', component: () => import('./views/VersionList.vue'), meta: { title: tr('Versions') }, props: true },
  {
    path: '/characters/:id/compare',
    name: 'version-compare',
    component: () => import('./views/VersionCompare.vue'),
    meta: { title: tr('Compare versions') },
    props: true,
  },
  {
    path: '/characters/:id/versions/:versionId/edit',
    name: 'version-edit',
    component: () => import('./views/VersionEdit.vue'),
    meta: { title: tr('Edit version') },
    props: true,
  },
  { path: '/assets', name: 'assets', component: () => import('./views/CharacterAssets.vue'), meta: { title: tr('Character Assets') } },
  { path: '/map', name: 'map', component: () => import('./views/MapPage.vue'), meta: { title: tr('Map') } },
  // Loaded on its own, with three.js, only when opened.
  { path: '/solar-system', name: 'solar-system', component: () => import('./views/SolarSystem.vue'), meta: { title: tr('Solar system') } },
  { path: '/settings', name: 'settings', component: () => import('./views/SettingsView.vue'), meta: { title: tr('Settings') } },
  { path: '/timeline', name: 'timeline', component: () => import('./views/TimelineView.vue'), meta: { title: tr('Timeline') } },
  { path: '/calendar', name: 'calendar', component: () => import('./views/CalendarView.vue'), meta: { title: tr('Calendar') } },
  { path: '/events', name: 'events', component: () => import('./views/Events.vue'), meta: { title: tr('Events') } },
  // 'new' must be declared before ':id' so it isn't read as an event id.
  { path: '/events/new', name: 'event-new', component: () => import('./views/EventEdit.vue'), meta: { title: tr('New event') } },
  { path: '/events/:id', name: 'event-view', component: () => import('./views/EventView.vue'), meta: { title: tr('Events') }, props: true },
  { path: '/events/:id/edit', name: 'event-edit', component: () => import('./views/EventEdit.vue'), meta: { title: tr('Edit event') }, props: true },
  { path: '/factions', name: 'factions', component: () => import('./views/Factions.vue'), meta: { title: tr('Factions') } },
  { path: '/bestiary', name: 'bestiary', component: () => import('./views/Bestiary.vue'), meta: { title: tr('Bestiary') } },
  { path: '/chapters', name: 'chapters', component: () => import('./views/Chapters.vue'), meta: { title: tr('Chapters') } },
  { path: '/chapters/:id', name: 'chapter', component: () => import('./views/ChapterView.vue'), meta: { title: tr('Chapters') }, props: true },
  // The writing editor: loaded on its own, with ProseMirror, only when opened.
  // :volume is a volume's id, or 0 for the chapters without one.
  { path: '/write', name: 'write-last', component: NotFound, beforeEnter: resumeWriting, meta: { title: tr('Write') } },
  { path: '/write/:volume', name: 'write', component: () => import('./views/WritePage.vue'), meta: { title: tr('Write') }, props: true },
  { path: '/write/:volume/pages', name: 'write-pages', component: () => import('./views/BookPagesPage.vue'), meta: { title: tr('Page view') }, props: true },
  // The fonts library: one for every story (fontLibrary.js), loaded on its own.
  { path: '/fonts', name: 'fonts', component: () => import('./views/FontsLibrary.vue'), meta: { title: tr('Fonts') } },
  { path: '/wiki', name: 'wiki', component: () => import('./views/WikiHome.vue'), meta: { title: tr('Wiki') } },
  { path: '/wiki/graph', name: 'wiki-graph', component: () => import('./views/GraphView.vue'), meta: { title: tr('Graph') } },
  { path: '/wiki/:type/:id', name: 'wiki-article', component: () => import('./views/WikiArticle.vue'), meta: { title: tr('Wiki') }, props: true },
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
  pageLoadStarted()
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

// A page's script can be gone: a tab left open across a release asks for the
// old build's file. Loading the address afresh gets the new build. Once per
// address, so being offline doesn't make it reload for ever.
router.onError((err, to) => {
  pageLoadEnded()
  if (!/dynamically imported module|module script failed|error loading dynamically/i.test(String(err?.message || err))) return
  const key = 'polaris-reloaded'
  try {
    if (sessionStorage.getItem(key) === to.fullPath) return
    sessionStorage.setItem(key, to.fullPath)
  } catch (e) {
    return
  }
  window.location.assign(to.fullPath)
})

// A detail page names what it shows once it has loaded it (see pageTitle).
router.afterEach((to, from, failure) => {
  pageLoadEnded()
  if (!failure) {
    try {
      sessionStorage.removeItem('polaris-reloaded')
    } catch (e) {
      /* nothing to forget */
    }
  }
  if (!failure) pageTitle.value = ''
  if (!failure) rememberVisit(to)
})

export default router
