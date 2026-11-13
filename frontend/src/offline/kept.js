// Reading a story with no connection (docs/DEVELOPMENT.md, "Reading
// offline"): for a story kept on this device, every answer the server gives
// to a read is kept too (Cache Storage, `polaris-data-<story>`), and given
// back when the server can't be reached. Nothing is ever written offline
// here: saves fail as they always did, and the banner says why.
//
// Only what goes through `fetchKept` (request.js's `send`, and the fonts and
// reference libraries) is kept. The writing page's calls are not: it keeps
// its books itself, with what was written offline (offline/store.js).
import { ref } from 'vue'
import { connected, reportReach } from './connection.js'

export const dataCache = (story) => `polaris-data-${story}`
// What every story needs: the list of stories and the two shared libraries.
export const SHARED_CACHE = 'polaris-data'
export const SHARED = ['/api/stories', '/api/fonts', '/api/fonts/used', '/api/references', '/api/references/used']

// ---- Which stories are kept (localStorage: read at once, before any call) ----
const KEY = 'polaris-kept-stories'

function read() {
  try {
    const all = JSON.parse(localStorage.getItem(KEY) || '{}')
    return all && typeof all === 'object' ? all : {}
  } catch (e) {
    return {}
  }
}

// { <story id>: { at: <ms, the last full copy; 0 while the first is made>, name, missing } }
export const keptStories = ref(read())
export const isKept = (story) => !!keptStories.value[story]

export function markKept(story, info) {
  keptStories.value = { ...read(), [story]: info }
  try {
    localStorage.setItem(KEY, JSON.stringify(keptStories.value))
  } catch (e) {
    /* kept for this visit only */
  }
}

export async function forgetKept(story) {
  const all = read()
  delete all[story]
  keptStories.value = all
  try {
    localStorage.setItem(KEY, JSON.stringify(all))
  } catch (e) {
    /* nothing to forget */
  }
  try {
    await caches.delete(dataCache(story))
  } catch (e) {
    /* no cache storage */
  }
}

// A copy older than a day is made again by itself, a little after the app
// opens, so the story on the device follows the server without being asked.
const RENEW_AFTER = 24 * 3600 * 1000
export function renewKeptLater(story, wait = 8000) {
  const info = keptStories.value[story]
  if (!info || typeof window === 'undefined') return
  if (info.at && Date.now() - info.at < RENEW_AFTER) return
  setTimeout(() => {
    if (!connected.value) return
    import('./keepStory.js').then((m) => m.keepStory(story, info.name)).catch(() => {})
  }, wait)
}

// The cache a read's answer is kept in; '' for a read that isn't kept.
export function cacheFor(url, kept = isKept) {
  const path = String(url).replace(/^https?:\/\/[^/]+/, '').replace(/[?#].*$/, '')
  const m = /^\/api\/s\/([^/]+)\//.exec(path)
  if (m) return kept(m[1]) ? dataCache(m[1]) : ''
  return SHARED.includes(path) ? SHARED_CACHE : ''
}

const isJson = (res) => (res.headers.get('Content-Type') || '').startsWith('application/json')
// What a proxy in front of Polaris answers while Polaris is down.
const isGone = (res) => res.status === 502 || res.status === 503 || res.status === 504

// fetch, with the kept copy when the server can't be reached. `env` is for
// the tests (a fake network and fake caches).
export async function fetchKept(url, options = {}, env = {}) {
  const ask = env.fetch || fetch
  const store = env.caches || (typeof caches !== 'undefined' ? caches : null)
  const read = !options.method || options.method === 'GET'
  const name = read && store ? cacheFor(url, env.kept) : ''
  if (!name) return ask(url, options)

  const kept = async () => {
    try {
      return (await (await store.open(name)).match(url)) || null
    } catch (e) {
      return null
    }
  }
  // Already known to be out of reach: the kept copy at once, rather than
  // wait on a dead line for every call of a page.
  if (!(env.connected ? env.connected() : connected.value)) {
    const copy = await kept()
    if (copy) return copy
  }
  const report = env.report || reportReach
  let res
  try {
    res = await ask(url, options)
  } catch (e) {
    if (e?.name === 'AbortError') throw e // the caller's own signal
    const copy = await kept()
    if (!copy) throw e
    report(false)
    return copy
  }
  if (isGone(res)) {
    const copy = await kept()
    if (!copy) return res
    report(false)
    return copy
  }
  report(true)
  if (res.ok && isJson(res)) {
    const copy = res.clone()
    store
      .open(name)
      .then((cache) => cache.put(url, copy))
      .catch(() => {})
  } else if (res.status === 404) {
    // Deleted since it was kept.
    store
      .open(name)
      .then((cache) => cache.delete(url))
      .catch(() => {})
  }
  return res
}
