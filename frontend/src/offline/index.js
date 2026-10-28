// Offline writing in the browser (docs/EDITOR.md, "Offline writing"): the
// one store of kept books, and the service worker's registration.
import { idbKV } from './kv.js'
import { createOfflineStore } from './store.js'

export const offlineStore = createOfflineStore(idbKV())

// The service worker keeps the app's files (built ones only: the dev server
// has none). It needs HTTPS or localhost; elsewhere the app works as before.
export function registerServiceWorker() {
  if (typeof window === 'undefined' || !window.isSecureContext || !('serviceWorker' in navigator)) return
  const go = () => navigator.serviceWorker.register('/sw.js').catch(() => {})
  if (document.readyState === 'complete') go()
  else window.addEventListener('load', go, { once: true })
}
