// Page-level behaviour shared by the views: the browser tab title, the
// "unsaved changes" guard and the Ctrl+S save shortcut.

import { ref, onMounted, onBeforeUnmount } from 'vue'
import { confirmDiscard } from './confirm'
import { onBeforeRouteLeave, onBeforeRouteUpdate } from 'vue-router'
import { t } from './i18n'

// The name of whatever a detail page shows (a character, an event, an
// article). The router clears it on every navigation; App.vue combines it
// with the route's own title and the story name into document.title.
export const pageTitle = ref('')

// A link to the map that opens centred and zoomed on one location (see
// focusFromQuery in MapView.vue). Without an id it's the plain map.
export const mapPath = (locationId) => (locationId ? `/map?focus=${locationId}` : '/map')

// Asks before leaving a page whose form has unsaved edits: in-app navigation
// (including to the same view with other params, such as a wiki link to
// another article) and closing or reloading the tab. isDirty is a function
// so it is only evaluated when someone actually tries to leave.
export function useUnsavedGuard(isDirty) {
  const ask = async () => {
    if (isDirty() && !(await confirmDiscard())) return false
  }
  onBeforeRouteLeave(ask)
  onBeforeRouteUpdate(ask)

  const warnUnload = (e) => {
    if (isDirty()) e.preventDefault()
  }
  onMounted(() => window.addEventListener('beforeunload', warnUnload))
  onBeforeUnmount(() => window.removeEventListener('beforeunload', warnUnload))
}

// A pop-up form asks before a click outside it or Esc (App.vue clicks the
// backdrop for Esc) throws away what was typed. state() gathers the form's
// values (a picked file counts by its name). They are noted at the first
// touch inside the form (`touched`, on its pointerdown, keydown and input,
// caught before the field changes), so values that load after the form
// opens count as unchanged. requestClose is the backdrop's click handler.
export function useModalGuard(state, close) {
  const snapshot = () => JSON.stringify(state(), (k, v) => (typeof File !== 'undefined' && v instanceof File ? `file:${v.name}` : v))
  let start = null
  const touched = () => {
    if (start === null) start = snapshot()
  }
  const requestClose = async () => {
    if (start !== null && snapshot() !== start && !(await confirmDiscard())) return
    close()
  }
  return { requestClose, touched }
}

// Ctrl+S (Cmd+S on a Mac) calls save instead of the browser's "save page".
export function useSaveShortcut(save) {
  const onKey = (e) => {
    if ((e.ctrlKey || e.metaKey) && !e.altKey && e.key.toLowerCase() === 's') {
      e.preventDefault()
      save()
    }
  }
  onMounted(() => window.addEventListener('keydown', onKey))
  onBeforeUnmount(() => window.removeEventListener('keydown', onKey))
}

// The full-size picture viewer (PictureViewer.vue, shown by App.vue).
export const viewedPicture = ref(null) // { src, alt } or null
export function viewPicture(src, alt = '') {
  if (src) viewedPicture.value = { src, alt }
}

// Several pictures, viewed one at a time from `index` (a wiki gallery).
export function viewGallery(list, index = 0) {
  if (list.length) viewedPicture.value = { ...list[index], list, index }
}

// The last address each page was seen at, query included (a list's filters
// and sort), so a breadcrumb back to a list opens it as it was left even
// after a few pages in between. router.js records every navigation.
const lastSeenAt = new Map()
export function rememberVisit(route) {
  lastSeenAt.set(route.path, route.fullPath)
}
export const lastVisitOf = (path) => lastSeenAt.get(path) || path

// Opens the Ctrl+K search. App.vue owns it and listens for the shortcut, so
// this presses it.
export function openSearch() {
  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', ctrlKey: true, bubbles: true, cancelable: true }))
}

// "Try again" beside a page's load error: the page is made anew, so it
// loads from the start, without reloading the whole app.
export const pageKey = ref(0)
export const retryPage = () => pageKey.value++

// True while the router is on its way to a page (its script may still be
// on the wire), after a moment so a quick change shows nothing.
export const pageLoading = ref(false)
let loadingTimer = null
export function pageLoadStarted() {
  clearTimeout(loadingTimer)
  loadingTimer = setTimeout(() => (pageLoading.value = true), 200)
}
export function pageLoadEnded() {
  clearTimeout(loadingTimer)
  pageLoading.value = false
}
