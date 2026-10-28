// The files an offline book needs besides its texts (docs/EDITOR.md,
// "Offline writing"), kept in the browser's Cache Storage where the service
// worker (sw.js) finds them when the server can't be reached: pictures and
// the fonts library's files per story (`polaris-offline-<story>`), Google
// Fonts and spelling dictionaries shared (`polaris-fonts`, `polaris-dict`).
// The names are the service worker's too.
import { googleFontFiles } from './docRefs.js'

export const storyCache = (story) => `polaris-offline-${story}`
export const FONT_CACHE = 'polaris-fonts'
export const DICT_CACHE = 'polaris-dict'

const hasCaches = () => typeof caches !== 'undefined'

// Offline writing needs a service worker (so the app opens with no
// connection), which needs HTTPS or localhost, and IndexedDB.
export function offlineSupported() {
  return typeof window !== 'undefined' && !!window.isSecureContext && 'serviceWorker' in navigator && typeof indexedDB !== 'undefined' && hasCaches()
}

// Keeps each address not kept yet. Answers the ones that failed.
export async function keepFiles(cacheName, urls, onProgress = () => {}) {
  if (!hasCaches()) return [...urls]
  const cache = await caches.open(cacheName)
  const failed = []
  let done = 0
  for (const url of urls) {
    try {
      if (!(await cache.match(url))) {
        const res = await fetch(url, { credentials: 'same-origin' })
        if (!res.ok) throw new Error(String(res.status))
        await cache.put(url, res)
      }
    } catch (e) {
      failed.push(url)
    }
    onProgress(++done, urls.length)
  }
  return failed
}

// Forgets what a cache holds that isn't in `keep` (same-origin paths or
// full addresses). `keep` null: the whole cache.
export async function pruneFiles(cacheName, keep) {
  if (!hasCaches()) return
  if (!keep) return void (await caches.delete(cacheName))
  const cache = await caches.open(cacheName)
  const wanted = new Set([...keep].map((u) => new URL(u, location.origin).href))
  for (const req of await cache.keys()) if (!wanted.has(req.url)) await cache.delete(req)
}

// A Google Fonts stylesheet and its Latin faces. Answers whether the sheet
// was found.
export async function keepGoogleFont(cssUrl) {
  if (!hasCaches()) return false
  const cache = await caches.open(FONT_CACHE)
  let res = await cache.match(cssUrl)
  if (!res) {
    try {
      res = await fetch(cssUrl, { mode: 'cors', credentials: 'omit' })
    } catch (e) {
      return false
    }
    if (!res.ok) return false
    await cache.put(cssUrl, res.clone())
  }
  const files = googleFontFiles(await res.text())
  for (const url of files) {
    try {
      if (await cache.match(url)) continue
      const f = await fetch(url, { mode: 'cors', credentials: 'omit' })
      if (f.ok) await cache.put(url, f)
    } catch (e) {
      /* the face is drawn in a fallback font */
    }
  }
  return true
}

// Dictionaries of an older revision (the address names it) are dropped.
export async function pruneDictionaries(revision) {
  if (!hasCaches()) return
  const cache = await caches.open(DICT_CACHE)
  for (const req of await cache.keys()) if (new URL(req.url).searchParams.get('v') !== revision) await cache.delete(req)
}

// Asks the browser not to clear what is kept when space runs low. Answers
// true (granted), false (refused) or null (can't ask).
export async function askPersistence() {
  try {
    if (!navigator.storage?.persist) return null
    if (await navigator.storage.persisted?.()) return true
    return await navigator.storage.persist()
  } catch (e) {
    return null
  }
}

export async function isPersisted() {
  try {
    return navigator.storage?.persisted ? await navigator.storage.persisted() : null
  } catch (e) {
    return null
  }
}
