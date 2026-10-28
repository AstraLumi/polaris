/* Polaris's service worker (docs/EDITOR.md, "Offline writing"). A template:
 * the build (swBuild.js, from vite.config.js) fills in BUILD, PRECACHE and
 * ENTRY and writes it as /sw.js. A plain script, no imports.
 *
 * - The app's files: the page, the main scripts and the writing page's
 *   (PRECACHE) are kept at install in `polaris-app-<BUILD>`; other built
 *   files (hashed names under /assets/) are kept the first time they load.
 *   A new release is a new BUILD: its cache is filled at install and the
 *   older ones are deleted when it takes over.
 * - Pages always come from the server while it answers (a new release is
 *   seen at once); the kept page only when it can't be reached.
 * - /api/ is never touched: the writing page keeps what offline books need
 *   in IndexedDB itself.
 * - Uploads (pictures, the fonts library's files) come from the server;
 *   the copies the writing page kept for offline books (caches named
 *   `polaris-offline-<story>`) only when it can't be reached. Kept as the
 *   server sent them, headers included (nosniff, sandbox).
 * - Spelling dictionaries (their address names their revision) and Google
 *   Fonts are kept the first time they load.
 */
const BUILD = 'dev'
const PRECACHE = ['/']
const ENTRY = ''

const APP = 'polaris-app-' + BUILD
const DICT = 'polaris-dict'
const FONTS = 'polaris-fonts'
const PAGE_WAIT = 6000

self.addEventListener('install', (event) => {
  event.waitUntil(
    (async () => {
      const cache = await caches.open(APP)
      await cache.addAll(PRECACHE.map((u) => new Request(new URL(u, self.location.href).href, { cache: 'reload' })))
      // The page must be this build's (a release going out while this
      // installs): otherwise try again on the next visit.
      if (ENTRY) {
        const page = await cache.match(new URL('/', self.location.href).href)
        const text = page ? await page.text() : ''
        if (!text.includes(ENTRY)) {
          await caches.delete(APP)
          throw new Error('the page is not from this build')
        }
      }
      await self.skipWaiting()
    })(),
  )
})

self.addEventListener('activate', (event) => {
  event.waitUntil(
    (async () => {
      for (const name of await caches.keys()) if (name.startsWith('polaris-app-') && name !== APP) await caches.delete(name)
      await self.clients.claim()
    })(),
  )
})

const isHtml = (res) => (res.headers.get('Content-Type') || '').startsWith('text/html')
// What a proxy in front of Polaris answers while Polaris is down.
const isGone = (res) => res.status === 502 || res.status === 503 || res.status === 504

// The server's answer, or (when it can't be reached) a kept copy.
async function network(req, fallback) {
  let res
  try {
    res = await fetch(req)
  } catch (e) {
    const kept = await fallback()
    if (kept) return kept
    throw e
  }
  return isGone(res) ? (await fallback()) || res : res
}

// A kept copy, else the server's (kept when it is a real file).
async function keptFirst(req, cacheName, key = req) {
  const cache = await caches.open(cacheName)
  const kept = await cache.match(key)
  if (kept) return kept
  const res = await fetch(req)
  if (res.ok && !isHtml(res)) await cache.put(key, res.clone()).catch(() => {})
  return res
}

async function appPage(req) {
  const kept = async () => (await (await caches.open(APP)).match(new URL('/', self.location.href).href)) || null
  // A connection that hangs is as good as none: the kept page after a while.
  let timer
  const slow = new Promise((resolve) => {
    timer = setTimeout(resolve, PAGE_WAIT)
  })
    .then(kept)
    .then((k) => k || new Promise(() => {}))
  const live = fetch(req).then(async (res) => (isGone(res) ? (await kept()) || res : res))
  try {
    return await Promise.race([live, slow])
  } catch (e) {
    const k = await kept()
    if (k) return k
    throw e
  } finally {
    clearTimeout(timer)
  }
}

// Google's stylesheets are asked for without CORS by <link>; asking with it
// gives a readable answer that can be kept (and used for the same request).
async function googleCss(req) {
  const cache = await caches.open(FONTS)
  try {
    const res = await fetch(req.url, { mode: 'cors', credentials: 'omit' })
    if (res.ok) await cache.put(req.url, res.clone()).catch(() => {})
    return res
  } catch (e) {
    const kept = await cache.match(req.url)
    if (kept) return kept
    throw e
  }
}

self.addEventListener('fetch', (event) => {
  const req = event.request
  if (req.method !== 'GET') return
  const url = new URL(req.url)
  if (url.origin === self.location.origin) {
    const path = url.pathname
    if (path.startsWith('/api/') || path === '/sw.js') return
    if (path.startsWith('/uploads/')) return event.respondWith(network(req, () => caches.match(req)))
    if (req.mode === 'navigate') return event.respondWith(appPage(req))
    if (path.startsWith('/assets/')) return event.respondWith(keptFirst(req, APP))
    if (path.startsWith('/dict/')) return event.respondWith(keptFirst(req, DICT, req.url))
    return event.respondWith(network(req, async () => (await (await caches.open(APP)).match(req)) || null))
  }
  if (url.hostname === 'fonts.googleapis.com') return event.respondWith(googleCss(req))
  if (url.hostname === 'fonts.gstatic.com') return event.respondWith(keptFirst(req, FONTS, req.url))
})
