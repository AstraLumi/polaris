// The service worker (src/offline/sw.js, written at build time by
// src/offline/swBuild.js): what it keeps at install, which requests it
// answers and how, on a fake network and fake caches.
// node frontend/tests/serviceWorker.test.mjs
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { precacheList, serviceWorkerSource, entryFile } from '../src/offline/swBuild.js'

const ORIGIN = 'http://127.0.0.1:18092'

// ---- The build's file list ----
const chunk = (fileName, more = {}) => ({ type: 'chunk', fileName, imports: [], viteMetadata: { importedCss: new Set() }, ...more })
const bundle = {
  'assets/index-A.js': chunk('assets/index-A.js', { isEntry: true, imports: ['assets/vendor-B.js'], viteMetadata: { importedCss: new Set(['assets/index-C.css']) } }),
  'assets/vendor-B.js': chunk('assets/vendor-B.js'),
  'assets/WritePage-D.js': chunk('assets/WritePage-D.js', {
    facadeModuleId: 'C:\\polaris\\frontend\\src\\views\\WritePage.vue',
    imports: ['assets/vendor-B.js', 'assets/Export-E.js'],
    viteMetadata: { importedCss: new Set(['assets/WritePage-F.css']) },
  }),
  'assets/Export-E.js': chunk('assets/Export-E.js'),
  'assets/MapPage-G.js': chunk('assets/MapPage-G.js', { facadeModuleId: '/polaris/frontend/src/views/MapPage.vue' }),
  'assets/spell.worker-H.js': { type: 'asset', fileName: 'assets/spell.worker-H.js' },
  'assets/climate.worker-I.js': { type: 'asset', fileName: 'assets/climate.worker-I.js' },
  'index.html': { type: 'asset', fileName: 'index.html' },
}
assert.deepEqual(precacheList(bundle), [
  '/assets/Export-E.js',
  '/assets/WritePage-D.js',
  '/assets/WritePage-F.css',
  '/assets/index-A.js',
  '/assets/index-C.css',
  '/assets/spell.worker-H.js',
  '/assets/vendor-B.js',
])
assert.equal(entryFile(bundle), 'assets/index-A.js')

const template = readFileSync(new URL('../src/offline/sw.js', import.meta.url), 'utf8')
const built = serviceWorkerSource(template, bundle, ['/', '/icon-192.png'])
assert.match(built.build, /^[0-9a-f]{12}$/)
assert.ok(built.source.includes(`const BUILD = "${built.build}"`))
assert.ok(built.source.includes('"/assets/index-A.js"'))
// Another build (a file changed, so its name did) is another BUILD.
const other = serviceWorkerSource(template, { ...bundle, 'assets/Export-Z.js': chunk('assets/Export-Z.js') }, ['/', '/icon-192.png'])
assert.notEqual(other.build, built.build)
assert.throws(() => serviceWorkerSource('nothing here', bundle), /not found/)

// ---- Running it ----
function world(sw) {
  const files = new Map() // path → { body, type, status }
  const serve = (path, body, type = 'text/plain', headers = {}, status = 200) => files.set(path, { body, type, headers, status })
  let online = true
  const log = []
  const fetchFn = async (req, init = {}) => {
    const url = new URL(typeof req === 'string' ? req : req.url, ORIGIN)
    log.push(url.pathname + (url.host === new URL(ORIGIN).host ? '' : `@${url.host}`) + (init.mode ? ` ${init.mode}` : ''))
    if (!online) throw new TypeError('Failed to fetch')
    const key = url.host === new URL(ORIGIN).host ? url.pathname + url.search : url.href
    const f = files.get(key) ?? files.get(url.pathname)
    // The server answers the app's page for any path it doesn't have.
    if (!f) return new Response(files.get('/')?.body ?? '', { status: 200, headers: { 'Content-Type': 'text/html' } })
    return new Response(f.body, { status: f.status || 200, headers: { 'Content-Type': f.type, ...f.headers } })
  }
  const stores = new Map()
  const keyOf = (r) => new URL(typeof r === 'string' ? r : r.url, ORIGIN).href
  const cacheOf = (name) => {
    if (!stores.has(name)) stores.set(name, new Map())
    const m = stores.get(name)
    return {
      match: async (r) => m.get(keyOf(r))?.clone(),
      put: async (r, res) => void m.set(keyOf(r), res),
      add: async (r) => {
        const res = await fetchFn(r)
        if (!res.ok) throw new Error('bad')
        m.set(keyOf(r), res)
      },
      addAll: async (list) => {
        for (const r of list) {
          const res = await fetchFn(r)
          if (!res.ok) throw new Error('bad')
          m.set(keyOf(r), res)
        }
      },
    }
  }
  const caches = {
    open: async (name) => cacheOf(name),
    keys: async () => [...stores.keys()],
    delete: async (name) => stores.delete(name),
    match: async (r) => {
      for (const m of stores.values()) if (m.has(keyOf(r))) return m.get(keyOf(r)).clone()
      return undefined
    },
  }
  const listeners = {}
  const self = {
    location: new URL('/sw.js', ORIGIN),
    addEventListener: (type, fn) => (listeners[type] = fn),
    skipWaiting: async () => {},
    clients: { claim: async () => {} },
  }
  vm.runInNewContext(sw, { self, caches, fetch: fetchFn, Request, Response, URL, setTimeout, clearTimeout })

  const lifecycle = async (type) => {
    let p = null
    listeners[type]({ waitUntil: (x) => (p = x) })
    await p
  }
  // { handled: false } when the worker leaves the request to the browser.
  const request = async (path, { mode = 'cors', host } = {}) => {
    let p = null
    const url = host ? `https://${host}${path}` : new URL(path, ORIGIN).href
    listeners.fetch({ request: { url, method: 'GET', mode }, respondWith: (x) => (p = x) })
    if (!p) return { handled: false }
    try {
      const res = await p
      return { handled: true, status: res.status, text: await res.text(), type: res.headers.get('Content-Type'), headers: res.headers }
    } catch (e) {
      return { handled: true, error: e.message }
    }
  }
  return { serve, files, stores, log, lifecycle, request, setOnline: (v) => (online = v) }
}

const page = (entry) => `<html><script type="module" src="/${entry}"></script></html>`
const w = world(built.source)
w.serve('/', page('assets/index-A.js'), 'text/html')
w.serve('/icon-192.png', 'PNG', 'image/png')
for (const f of precacheList(bundle)) w.serve(f, `code of ${f}`, f.endsWith('.css') ? 'text/css' : 'text/javascript')
w.serve('/assets/MapPage-G.js', 'map page', 'text/javascript')
w.serve('/uploads/s/0123456789abcdef/editor/pic.png', 'PIC', 'image/png', { 'X-Content-Type-Options': 'nosniff', 'Content-Security-Policy': 'sandbox' })
w.serve('/dict/en_US/en_US.dic.gz?v=4f1df64', 'DIC', 'application/gzip')

// Install keeps the page and the app's files; an older build's cache goes at activation.
w.stores.set('polaris-app-oldbuild', new Map())
w.stores.set('polaris-offline-0123456789abcdef', new Map())
await w.lifecycle('install')
const app = `polaris-app-${built.build}`
assert.ok(w.stores.get(app).has(`${ORIGIN}/`))
assert.ok(w.stores.get(app).has(`${ORIGIN}/assets/WritePage-D.js`))
assert.ok(!w.stores.get(app).has(`${ORIGIN}/assets/MapPage-G.js`), 'other pages load when used')
await w.lifecycle('activate')
assert.ok(!w.stores.has('polaris-app-oldbuild'))
assert.ok(w.stores.has('polaris-offline-0123456789abcdef'), "offline books' copies stay")

// The API is never touched; neither is the worker's own file.
assert.deepEqual(await w.request('/api/s/0123456789abcdef/books/1'), { handled: false })
assert.deepEqual(await w.request('/sw.js'), { handled: false })

// Pages: the server's while it answers (a new release shows at once)...
w.serve('/', page('assets/index-NEW.js'), 'text/html')
let r = await w.request('/write/1?chapter=3', { mode: 'navigate' })
assert.ok(r.text.includes('index-NEW'))
// ...the kept one when it can't be reached.
w.setOnline(false)
r = await w.request('/write/1?chapter=3', { mode: 'navigate' })
assert.ok(r.text.includes('index-A.js'), "the kept page is this build's")
r = await w.request('/characters', { mode: 'navigate' })
assert.equal(r.status, 200)
// A proxy answering for a server that is down counts as none.
w.setOnline(true)
w.serve('/write/9', 'Bad gateway', 'text/html', {}, 502)
r = await w.request('/write/9', { mode: 'navigate' })
assert.equal(r.status, 200)
assert.ok(r.text.includes('index-A.js'))
w.setOnline(false)

// Built files: the kept copy first; others kept when they first load.
r = await w.request('/assets/index-A.js')
assert.equal(r.text, 'code of /assets/index-A.js')
r = await w.request('/assets/MapPage-G.js')
assert.ok(r.error, 'never loaded online: not there offline')
w.setOnline(true)
w.log.length = 0
r = await w.request('/assets/MapPage-G.js')
assert.equal(r.text, 'map page')
w.setOnline(false)
r = await w.request('/assets/MapPage-G.js')
assert.equal(r.text, 'map page')
// A missing built file (the server answers its page) is never kept as one.
w.setOnline(true)
r = await w.request('/assets/gone-Q.js')
assert.equal(r.type, 'text/html')
assert.ok(!w.stores.get(app).has(`${ORIGIN}/assets/gone-Q.js`))

// Uploads: the server's; the copy kept for an offline book when it can't be
// reached, with the server's headers.
const pic = '/uploads/s/0123456789abcdef/editor/pic.png'
r = await w.request(pic)
assert.equal(r.text, 'PIC')
assert.ok(!w.stores.get(app).has(`${ORIGIN}${pic}`), 'the worker keeps no uploads itself')
w.setOnline(false)
r = await w.request(pic)
assert.ok(r.error, 'not kept: no copy')
// The writing page keeps an offline book's pictures (offline/caches.js).
{
  const res = new Response('PIC', { headers: { 'Content-Type': 'image/png', 'X-Content-Type-Options': 'nosniff', 'Content-Security-Policy': 'sandbox' } })
  w.stores.get('polaris-offline-0123456789abcdef').set(`${ORIGIN}${pic}`, res)
}
w.setOnline(false)
r = await w.request(pic)
assert.equal(r.text, 'PIC')
assert.equal(r.headers.get('X-Content-Type-Options'), 'nosniff')
assert.equal(r.headers.get('Content-Security-Policy'), 'sandbox')

// Dictionaries: kept the first time.
w.setOnline(true)
r = await w.request('/dict/en_US/en_US.dic.gz?v=4f1df64')
assert.equal(r.text, 'DIC')
w.setOnline(false)
r = await w.request('/dict/en_US/en_US.dic.gz?v=4f1df64')
assert.equal(r.text, 'DIC')

// Public files kept at install answer offline.
r = await w.request('/icon-192.png')
assert.equal(r.text, 'PNG')

// Google Fonts: the stylesheet asked for with CORS, kept; the files kept.
w.setOnline(true)
w.serve('https://fonts.googleapis.com/css2?family=Lora', '@font-face{}', 'text/css')
w.serve('https://fonts.gstatic.com/s/lora/a.woff2', 'FONT', 'font/woff2')
w.log.length = 0
r = await w.request('/css2?family=Lora', { host: 'fonts.googleapis.com', mode: 'no-cors' })
assert.equal(r.text, '@font-face{}')
assert.deepEqual(w.log, ['/css2@fonts.googleapis.com cors'])
r = await w.request('/s/lora/a.woff2', { host: 'fonts.gstatic.com' })
w.setOnline(false)
r = await w.request('/css2?family=Lora', { host: 'fonts.googleapis.com', mode: 'no-cors' })
assert.equal(r.text, '@font-face{}')
r = await w.request('/s/lora/a.woff2', { host: 'fonts.gstatic.com' })
assert.equal(r.text, 'FONT')
// Other sites are left alone.
assert.deepEqual(await w.request('/x.png', { host: 'example.com' }), { handled: false })

// An install that finds another build's page fails (and is tried again later).
{
  const w2 = world(built.source)
  w2.serve('/', page('assets/index-OTHER.js'), 'text/html')
  for (const f of ['/icon-192.png', ...precacheList(bundle)]) w2.serve(f, 'x', 'text/plain')
  await assert.rejects(w2.lifecycle('install'), /not from this build/)
  assert.ok(!w2.stores.has(app))
}

console.log('serviceWorker: ok')
