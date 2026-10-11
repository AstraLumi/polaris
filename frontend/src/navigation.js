// Page-level behaviour shared by the views: the browser tab title, the
// "unsaved changes" guard and the Ctrl+S save shortcut.

import { ref, onMounted, onBeforeUnmount } from 'vue'
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
  const ask = () => {
    if (isDirty() && !window.confirm(t('Discard your changes?'))) return false
  }
  onBeforeRouteLeave(ask)
  onBeforeRouteUpdate(ask)

  const warnUnload = (e) => {
    if (isDirty()) e.preventDefault()
  }
  onMounted(() => window.addEventListener('beforeunload', warnUnload))
  onBeforeUnmount(() => window.removeEventListener('beforeunload', warnUnload))
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
