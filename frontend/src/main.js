import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import './style.css'
import { loadSavedTheme } from './theme'
import { loadSavedLocale, i18nPlugin } from './i18n'
import { trackRecent } from './recentTracker'
import { registerServiceWorker } from './offline/index.js'

loadSavedTheme()
loadSavedLocale()

// Every dialog closes on a click on its backdrop (.overlay @click.self). A
// press that starts inside the dialog (selecting text, say) and is released
// over the backdrop still makes the browser fire that click, so it only
// counts when the press started on the backdrop too.
let pressedOn = null
document.addEventListener('mousedown', (e) => (pressedOn = e.target), true)
document.addEventListener(
  'click',
  (e) => {
    if (e.target.classList?.contains('overlay') && pressedOn !== e.target) e.stopPropagation()
  },
  true,
)

// Remember the items the writer opens (the search and the home page offer them).
trackRecent(router)

createApp(App).use(router).use(i18nPlugin).mount('#app')

// Offline writing (docs/EDITOR.md): the app's files kept by a service worker.
if (import.meta.env.PROD) registerServiceWorker()
