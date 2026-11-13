// Reading offline (src/offline/kept.js): which reads are kept, and what a
// read answers when the server can and can't be reached, on a fake network
// and fake caches.
// node frontend/tests/kept.test.mjs
import assert from 'node:assert/strict'
import { cacheFor, fetchKept, dataCache, SHARED_CACHE } from '../src/offline/kept.js'

const STORY = '0123456789abcdef'
const keptOnly = (id) => id === STORY

// ---- Which reads are kept ----
assert.equal(cacheFor(`/api/s/${STORY}/characters`, keptOnly), dataCache(STORY))
assert.equal(cacheFor(`http://127.0.0.1:18092/api/s/${STORY}/events?tag=war`, keptOnly), dataCache(STORY))
assert.equal(cacheFor('/api/s/ffffffffffffffff/characters', keptOnly), '', 'a story not kept keeps nothing')
assert.equal(cacheFor('/api/stories', keptOnly), SHARED_CACHE)
assert.equal(cacheFor('/api/references', keptOnly), SHARED_CACHE)
assert.equal(cacheFor('/api/health', keptOnly), '', 'whether the server is there is never answered from a copy')
assert.equal(cacheFor(`/api/stories/${STORY}/snapshots`, keptOnly), '')

// ---- The world ----
function world() {
  const stores = new Map()
  const cacheOf = (name) => {
    if (!stores.has(name)) stores.set(name, new Map())
    const m = stores.get(name)
    return {
      match: async (url) => m.get(url)?.clone(),
      put: async (url, res) => void m.set(url, res),
      delete: async (url) => m.delete(url),
    }
  }
  const w = { stores, online: true, connected: true, answers: new Map(), asked: [], reports: [] }
  w.env = {
    kept: keptOnly,
    caches: { open: async (name) => cacheOf(name) },
    connected: () => w.connected,
    report: (ok) => w.reports.push(ok),
    fetch: async (url, options = {}) => {
      w.asked.push(`${options.method || 'GET'} ${url}`)
      if (options.signal?.aborted) throw Object.assign(new Error('aborted'), { name: 'AbortError' })
      if (!w.online) throw new TypeError('Failed to fetch')
      const a = w.answers.get(url) || { status: 404, body: 'not found', type: 'text/plain' }
      return new Response(a.body, { status: a.status || 200, headers: { 'Content-Type': a.type || 'application/json' } })
    },
  }
  return w
}
const settle = () => new Promise((r) => setTimeout(r, 0))
const list = `/api/s/${STORY}/characters`
const keptText = (w, url) => w.stores.get(dataCache(STORY)).get(url).clone().text()

const w = world()
w.answers.set(list, { body: '[{"id":1}]' })
// With the server there: its answer, kept for later.
let res = await fetchKept(list, {}, w.env)
assert.equal(await res.text(), '[{"id":1}]')
await settle()
assert.ok(w.stores.get(dataCache(STORY)).has(list))
assert.deepEqual(w.reports, [true])

// A newer answer replaces the kept one.
w.answers.set(list, { body: '[{"id":1},{"id":2}]' })
await fetchKept(list, {}, w.env)
await settle()

// The server gone: the kept copy, and the app is told.
w.online = false
w.reports.length = 0
res = await fetchKept(list, {}, w.env)
assert.equal(await res.text(), '[{"id":1},{"id":2}]')
assert.deepEqual(w.reports, [false])
// ...and it can be read again (each read gets its own copy).
assert.equal(await (await fetchKept(list, {}, w.env)).text(), '[{"id":1},{"id":2}]')

// Known to be out of reach: the copy at once, the network isn't asked.
w.connected = false
w.asked.length = 0
await fetchKept(list, {}, w.env)
assert.deepEqual(w.asked, [])
// Something never kept is still asked for, and fails as it always did.
await assert.rejects(fetchKept(`/api/s/${STORY}/characters/9/current`, {}, w.env), /Failed to fetch/)
w.connected = true

// Saves are never answered from a copy, and never kept.
await assert.rejects(fetchKept(list, { method: 'POST', body: '{}' }, w.env), /Failed to fetch/)
w.online = true
w.answers.set(list, { body: '{"id":3}' })
await fetchKept(list, { method: 'POST', body: '{}' }, w.env)
await settle()
assert.equal(await keptText(w, list), '[{"id":1},{"id":2}]')

// A proxy answering for a server that is down counts as none.
w.answers.set(list, { status: 502, body: 'Bad gateway', type: 'text/html' })
res = await fetchKept(list, {}, w.env)
assert.equal(res.status, 200)
assert.equal(await res.text(), '[{"id":1},{"id":2}]')
// ...with nothing kept, the proxy's answer goes through.
w.answers.set(`/api/s/${STORY}/gear`, { status: 503, body: 'Unavailable', type: 'text/html' })
assert.equal((await fetchKept(`/api/s/${STORY}/gear`, {}, w.env)).status, 503)

// Not JSON (the app's page for an unknown address), or an error: not kept.
w.answers.set(`/api/s/${STORY}/odd`, { body: '<html>', type: 'text/html' })
await fetchKept(`/api/s/${STORY}/odd`, {}, w.env)
w.answers.set(`/api/s/${STORY}/bad`, { status: 500, body: '{"error":"x"}' })
await fetchKept(`/api/s/${STORY}/bad`, {}, w.env)
await settle()
assert.ok(!w.stores.get(dataCache(STORY)).has(`/api/s/${STORY}/odd`))
assert.ok(!w.stores.get(dataCache(STORY)).has(`/api/s/${STORY}/bad`))

// Deleted on the server: its copy goes too.
w.answers.delete(list)
assert.equal((await fetchKept(list, {}, w.env)).status, 404)
await settle()
assert.ok(!w.stores.get(dataCache(STORY)).has(list))

// The caller giving up is not "no connection".
const gone = new AbortController()
gone.abort()
await assert.rejects(fetchKept(list, { signal: gone.signal }, w.env), { name: 'AbortError' })

// A story that isn't kept: plain fetch, nothing stored.
const other = '/api/s/ffffffffffffffff/characters'
w.answers.set(other, { body: '[]' })
await fetchKept(other, {}, w.env)
await settle()
assert.ok(!w.stores.has(dataCache('ffffffffffffffff')))

console.log('kept: ok')
