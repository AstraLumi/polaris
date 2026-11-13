import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import './style.css'
import { loadSavedTheme } from './theme'
import { loadSavedLocale, i18nPlugin } from './i18n'
import { trackRecent } from './recentTracker'
import { retryPage } from './navigation'
import { registerServiceWorker } from './offline/index.js'
import { renewKeptLater } from './offline/kept.js'
import { activeStoryId } from './stories'

loadSavedTheme()

// Every dialog closes on a click on its backdrop (.overlay @click.self). A
// press that starts inside the dialog (selecting text, say) and is released
// over the backdrop still makes the browser fire that click, so it only
// counts when the press started on the backdrop too.
let pressedOn = null
document.addEventListener('mousedown', (e) => (pressedOn = e.target), true)
document.addEventListener(
  'click',
  (e) => {
    // Only a real click: Escape closes a dialog by sending its backdrop a
    // made-up one (App.vue), with no press before it, and must get through.
    if (e.isTrusted && e.target.classList?.contains('overlay') && pressedOn !== e.target) e.stopPropagation()
  },
  true,
)

// Remember the items the writer opens (the search and the home page offer them).
trackRecent(router)

// The page shows once its language is loaded (i18n.js), so it never
// appears in English first.
loadSavedLocale().finally(() => {
  const app = createApp(App).use(router).use(i18nPlugin)
  // "Try again" beside any page's load error (navigation.js).
  app.config.globalProperties.$retryPage = retryPage
  app.mount('#app')
})

// Offline writing (docs/EDITOR.md): the app's files kept by a service worker.
// A story kept for reading offline (offline/kept.js) has its copy renewed.
if (import.meta.env.PROD) {
  registerServiceWorker()
  renewKeptLater(activeStoryId)
}
