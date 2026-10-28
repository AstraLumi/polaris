// Offline writing end to end (docs/EDITOR.md, "Offline writing"), without a
// screen: builds the app, embeds it in the real Go server (as the Dockerfile
// does), runs that on 127.0.0.1 with a throwaway data folder and story
// (never real data), and in headless Edge: opens a book, marks it
// "Available offline", cuts the network and stops the server, writes in
// three chapters, reloads with no connection (the service worker's page,
// the kept texts and picture), changes two chapters "elsewhere" through the
// API, brings everything back and checks what landed: a chapter sent as
// is, one merged (different paragraphs), one in conflict (the same
// paragraph) settled through the conflict view. Also the rest of the app
// with no connection, and forgetting the book.
//
//   node frontend/tests/visual/offlineShots.mjs [--out <dir>] [--no-build] [--channel chrome]
//
// Pictures land in frontend/tests/visual/out/ (ignored by git). Needs Go and
// Microsoft Edge (or --channel chrome). Writes the build into
// backend/static/ (build output, ignored by git) like the Dockerfile.
import { spawn, execFileSync } from 'node:child_process'
import { mkdtempSync, mkdirSync, rmSync, readdirSync, cpSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, resolve, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'
import { deflateSync } from 'node:zlib'
import assert from 'node:assert/strict'
import { chromium } from 'playwright-core'

const here = dirname(fileURLToPath(import.meta.url))
const frontend = resolve(here, '../..')
const backend = resolve(frontend, '../backend')
const PORT = 18093
const BASE = `http://127.0.0.1:${PORT}`

const args = process.argv.slice(2)
const flag = (name) => args.includes(`--${name}`)
const option = (name, fallback) => {
  const i = args.indexOf(`--${name}`)
  return i >= 0 && args[i + 1] ? args[i + 1] : fallback
}
const out = resolve(option('out', join(here, 'out')))

const cleanups = []
async function cleanUp() {
  while (cleanups.length) {
    try {
      await cleanups.pop()()
    } catch {
      // Best effort.
    }
  }
}

async function waitFor(what, test, ms = 30000) {
  const until = Date.now() + ms
  for (;;) {
    try {
      if (await test()) return
    } catch {
      // Not yet.
    }
    if (Date.now() > until) throw new Error(`timed out waiting for ${what}`)
    await new Promise((r) => setTimeout(r, 200))
  }
}

// ---- The app and the server ------------------------------------------------------------
function build() {
  if (!flag('no-build')) execFileSync(process.execPath, [join(frontend, 'node_modules/vite/bin/vite.js'), 'build', '--logLevel', 'warn'], { cwd: frontend, stdio: 'inherit' })
  const statics = join(backend, 'static')
  for (const f of readdirSync(statics)) if (f !== '.gitkeep') rmSync(join(statics, f), { recursive: true, force: true })
  cpSync(join(frontend, 'dist'), statics, { recursive: true })
  const data = mkdtempSync(join(tmpdir(), 'polaris-offline-'))
  cleanups.push(() => rmSync(data, { recursive: true, force: true, maxRetries: 5, retryDelay: 200 }))
  const exe = join(data, process.platform === 'win32' ? 'polaris.exe' : 'polaris')
  execFileSync('go', ['build', '-o', exe, '.'], { cwd: backend, stdio: 'inherit' })
  return { exe, data }
}

// Starts the server; answers a function that stops it.
async function startServer({ exe, data }) {
  const proc = spawn(exe, [], {
    cwd: backend,
    // Only this machine: no firewall prompt for the throwaway server.
    env: { ...process.env, LISTEN_HOST: '127.0.0.1', PORT: String(PORT), DATA_DIR: join(data, 'data'), DB_PATH: join(data, 'data', 'polaris.db'), UPLOADS_DIR: join(data, 'uploads') },
    stdio: ['ignore', 'ignore', 'pipe'],
  })
  let log = ''
  proc.stderr.on('data', (d) => (log += d))
  const exited = new Promise((r) => proc.once('exit', r))
  const stop = async () => {
    proc.kill()
    await exited
  }
  cleanups.push(stop)
  await waitFor('the server', async () => (await fetch(`${BASE}/api/health`)).ok).catch((err) => {
    throw new Error(`${err.message}\n${log}`)
  })
  return stop
}

// ---- The throwaway story ---------------------------------------------------------------------
// A small PNG (a coloured band), written here so nothing is downloaded.
function png(w, h) {
  const table = Array.from({ length: 256 }, (_, n) => {
    let c = n
    for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1
    return c >>> 0
  })
  const crc = (buf) => {
    let c = 0xffffffff
    for (const b of buf) c = table[(c ^ b) & 0xff] ^ (c >>> 8)
    return (c ^ 0xffffffff) >>> 0
  }
  const chunk = (type, data) => {
    const len = Buffer.alloc(4)
    len.writeUInt32BE(data.length)
    const body = Buffer.concat([Buffer.from(type), data])
    const sum = Buffer.alloc(4)
    sum.writeUInt32BE(crc(body))
    return Buffer.concat([len, body, sum])
  }
  const ihdr = Buffer.alloc(13)
  ihdr.writeUInt32BE(w, 0)
  ihdr.writeUInt32BE(h, 4)
  ihdr.set([8, 2, 0, 0, 0], 8)
  const raw = Buffer.alloc((w * 3 + 1) * h)
  for (let y = 0; y < h; y++) {
    for (let x = 0; x < w; x++) {
      const i = y * (w * 3 + 1) + 1 + x * 3
      raw[i] = 40 + (x * 200) / w
      raw[i + 1] = 90
      raw[i + 2] = 160 + (y * 80) / h
    }
  }
  return Buffer.concat([Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]), chunk('IHDR', ihdr), chunk('IDAT', deflateSync(raw)), chunk('IEND', Buffer.alloc(0))])
}

const para = (text) => ({ type: 'paragraph', content: [{ type: 'text', text }] })
const textsOf = (doc) => (doc.content || []).map((n) => (n.content || []).map((c) => c.text || '').join(''))

function storyApi(story) {
  const api = `${BASE}/api/s/${story}`
  const call = async (path, method = 'GET', body) => {
    const res = await fetch(api + path, body instanceof FormData ? { method, body } : { method, headers: { 'Content-Type': 'application/json' }, body: body && JSON.stringify(body) })
    if (!res.ok) throw new Error(`${method} ${path}: ${res.status} ${await res.text()}`)
    return res.status === 204 ? null : res.json()
  }
  return {
    call,
    text: (id) => call(`/chapters/${id}/text`),
    async setText(id, doc) {
      const now = await call(`/chapters/${id}/text`)
      return call(`/chapters/${id}/text`, 'PUT', { doc, base_version: now.version })
    },
    versions: (id) => call(`/chapters/${id}/text/versions`),
  }
}

async function makeStory() {
  const form = new FormData()
  form.set('name', 'Offline test')
  const story = await (await fetch(`${BASE}/api/stories`, { method: 'POST', body: form })).json()
  const api = storyApi(story.id)
  const vol = await api.call('/volumes', 'POST', { title: 'The Kept Book' })
  // The book set up with its defaults (no page type dialog).
  const book = await api.call(`/books/${vol.id}`)
  await api.call(`/books/${vol.id}`, 'PUT', { settings: book.settings })
  const pic = new FormData()
  pic.set('file', new Blob([png(240, 90)], { type: 'image/png' }), 'band.png')
  const { src } = await api.call(`/books/${vol.id}/pictures`, 'POST', pic)
  const ids = []
  for (const title of ['One', 'Two', 'Three']) ids.push((await api.call('/chapters', 'POST', { title: `Chapter ${title}`, volume_id: vol.id })).id)
  await api.setText(ids[0], { type: 'doc', content: [para('The first paragraph of chapter one.'), { type: 'image', attrs: { src } }, para('The last paragraph of chapter one.')] })
  await api.setText(ids[1], { type: 'doc', content: [para('Chapter two begins here.'), para('The middle of chapter two.'), para('Chapter two ends here.')] })
  await api.setText(ids[2], { type: 'doc', content: [para('Chapter three, alone.')] })
  return { story: story.id, volume: vol.id, ids, api, picture: src }
}

// ---- In the browser --------------------------------------------------------------------------
async function run() {
  const exeData = build()
  let stopServer = await startServer(exeData)
  const t = await makeStory()
  const browser = await chromium.launch({ channel: option('channel', 'msedge'), headless: true })
  cleanups.push(() => browser.close())
  const context = await browser.newContext({ viewport: { width: 1400, height: 900 }, locale: 'en-US', deviceScaleFactor: 1 })
  await context.addInitScript((story) => {
    localStorage.setItem('polaris-story', story)
    localStorage.setItem('polaris-write-spell', '0')
  }, t.story)
  const page = await context.newPage()
  const errors = []
  let expectFailures = false
  page.on('pageerror', (e) => errors.push(e.message))
  page.on('console', (m) => {
    if (m.type() !== 'error') return
    // With no connection, failed loads are expected.
    if (expectFailures && /Failed to load resource|ERR_INTERNET_DISCONNECTED|Failed to fetch|NetworkError|dynamically imported module/.test(m.text())) return
    // The version check answering 409 is how a conflict is found.
    if (/status of 409/.test(m.text())) return
    errors.push(m.text())
  })
  page.on('dialog', (d) => d.accept())
  mkdirSync(out, { recursive: true })
  const saved = []
  const snap = async (name) => {
    await page.evaluate(() => document.fonts.ready)
    await page.waitForTimeout(300)
    const file = join(out, name)
    await page.screenshot({ path: file })
    saved.push(file)
  }
  const checks = []
  const check = (what) => {
    checks.push(what)
    console.log('check: ' + what)
  }
  const docText = () => page.locator('.write-doc').innerText()
  const saveState = () => page.locator('.save-state').innerText()
  const typeAtEndOf = async (n, text) => {
    const p = page.locator('.write-doc > p').nth(n)
    await p.click()
    await page.keyboard.press('End')
    await page.keyboard.type(text)
  }
  const openChapter = async (title) => {
    await page.locator('.chapter-item', { hasText: title }).click()
    await waitFor(`${title} open`, async () => (await page.locator('.chapter-item.is-here').innerText()).includes(title))
    await page.waitForTimeout(300)
  }
  const waitSaved = (what) => waitFor(what, async () => /Saved on this device|^Saved$/.test((await saveState()).trim()), 15000)

  // 1. The book, with the service worker in charge of the page.
  await page.goto(`${BASE}/write/${t.volume}?chapter=${t.ids[0]}`)
  await page.waitForSelector('.write-doc p')
  await page.evaluate(() => navigator.serviceWorker.ready)
  await waitFor('the service worker to control the page', () => page.evaluate(() => !!navigator.serviceWorker.controller))
  check('the service worker is installed and controls the page')

  // 2. Available offline.
  await page.locator('.offline-status').click()
  await page.locator('.offline-panel input[type="checkbox"]').check()
  await waitFor('the book to be kept', async () => (await page.locator('.offline-panel').innerText()).includes('Kept on this device'))
  const kept = await page.evaluate(async (story) => {
    const names = await caches.keys()
    const pics = await (await caches.open(`polaris-offline-${story}`)).keys()
    return { names, pics: pics.map((r) => new URL(r.url).pathname) }
  }, t.story)
  assert.ok(kept.pics.includes(t.picture), `the picture is kept (${JSON.stringify(kept)})`)
  assert.ok(kept.names.some((n) => n.startsWith('polaris-app-')))
  check('marking the book offline keeps its picture in the story cache')
  assert.equal((await page.locator('.offline-status').innerText()).trim(), 'Available offline')
  await snap('offline_01_kept.png')

  // 3. No connection: the network cut and the server stopped.
  expectFailures = true
  await context.setOffline(true)
  await stopServer()
  await waitFor('the page to see it is offline', async () => (await page.locator('.offline-status').innerText()).includes('No connection'))
  await typeAtEndOf(1, ' Written offline in one.')
  await waitSaved('chapter one kept on the device')
  assert.equal((await saveState()).trim(), 'Saved on this device')
  check('a save with no connection is kept on the device')
  await snap('offline_02_writing_offline.png')

  // 4. Reload with no connection: the service worker's page, the kept text and picture.
  await page.reload()
  await page.waitForSelector('.write-doc p')
  await waitFor('the kept text', async () => (await docText()).includes('Written offline in one.'))
  await waitFor('the kept picture', () => page.evaluate(() => [...document.querySelectorAll('.write-doc img')].every((i) => i.complete && i.naturalWidth > 0) && document.querySelectorAll('.write-doc img').length > 0))
  assert.equal((await page.locator('.chapter-item').count()), 3)
  await waitFor("the story's name as kept", async () => (await page.locator('.story-name').innerText()).trim() === 'Offline test')
  check('reloaded offline: the app, the kept chapter list, text and picture')
  await snap('offline_03_reloaded_offline.png')

  // 5. More chapters written offline.
  await openChapter('Chapter Two')
  await typeAtEndOf(1, ' (edited offline)')
  await waitSaved('chapter two kept')
  await openChapter('Chapter Three')
  await typeAtEndOf(0, ' Offline three.')
  await waitSaved('chapter three kept')
  await openChapter('Chapter One')
  assert.ok((await docText()).includes('Written offline in one.'))
  assert.match(await page.locator('.offline-status').innerText(), /No connection/)
  await page.locator('.offline-status').click()
  await waitFor('the panel lists three unsent chapters', async () => (await page.locator('.offline-panel .list li').count()) === 3)
  check('three chapters wait to be sent')
  await snap('offline_04_three_waiting.png')
  await page.locator('.offline-panel .side-head button').click()

  // 6. Meanwhile, elsewhere: chapter one changed in another paragraph,
  // chapter two in the same one.
  stopServer = await startServer(exeData)
  const one = await t.api.text(t.ids[0])
  one.doc.content[0] = para('The first paragraph, changed elsewhere.')
  await t.api.setText(t.ids[0], one.doc)
  const two = await t.api.text(t.ids[1])
  two.doc.content[1] = para('The middle, changed elsewhere.')
  await t.api.setText(t.ids[1], two.doc)

  // 7. Back online: sent, merged, or left for the writer.
  await context.setOffline(false)
  expectFailures = false
  await waitFor('chapter three sent', async () => textsOf((await t.api.text(t.ids[2])).doc)[0].includes('Offline three.'), 40000)
  check('chapter three (changed only here) sent as it was')
  await waitFor('chapter one merged', async () => {
    const s = textsOf((await t.api.text(t.ids[0])).doc)
    return s[0] === 'The first paragraph, changed elsewhere.' && s[2] === 'The last paragraph of chapter one. Written offline in one.'
  }, 20000)
  await waitFor('the editor shows the merge', async () => (await docText()).includes('changed elsewhere'))
  const v1 = await t.api.versions(t.ids[0])
  assert.ok(v1.some((v) => v.label === 'Before merging with changes made elsewhere'), 'the text before the merge is a version')
  check('chapter one (open, other paragraph changed elsewhere) merged; the text before is a version')
  const twoNow = textsOf((await t.api.text(t.ids[1])).doc)
  assert.equal(twoNow[1], 'The middle, changed elsewhere.', 'a conflict leaves the server text alone')
  await waitFor('the notice', async () => (await page.locator('.write-notice').count()) > 0)
  await page.locator('.offline-status').click()
  await waitFor('chapter two listed as a conflict', async () => (await page.locator('.offline-panel').innerText()).includes('changed elsewhere too'))
  check('chapter two (same paragraph changed elsewhere) waits for the writer')
  await snap('offline_05_back_online.png')
  await page.locator('.offline-panel .side-head button').click()

  // 8. The conflict, settled in the conflict view.
  await openChapter('Chapter Two')
  await page.waitForSelector('.offer')
  await page.locator('.offer').getByRole('button', { name: 'Compare' }).click()
  await page.waitForSelector('.conflict-overlay')
  await snap('offline_06_conflict.png')
  await page.getByRole('button', { name: "Use this browser's copy" }).click()
  await waitFor('chapter two takes the offline text', async () => textsOf((await t.api.text(t.ids[1])).doc)[1] === 'The middle of chapter two. (edited offline)', 15000)
  const v2 = await t.api.versions(t.ids[1])
  assert.ok(v2.some((v) => v.label === 'Before using the copy kept in this browser'))
  await waitFor('nothing left to send', async () => (await page.locator('.offline-status').innerText()).trim() === 'Available offline')
  check("the conflict settled with the browser's copy; the other text is a version")

  // 9. The rest of the app with no connection.
  expectFailures = true
  await context.setOffline(true)
  await page.goto(`${BASE}/`)
  await page.waitForSelector('.offline-banner')
  assert.ok((await page.locator('.offline-banner').innerText()).includes('The Kept Book'))
  check('other pages say there is no connection, with a link to the kept book')
  await snap('offline_07_other_pages.png')
  await page.locator('.offline-banner a', { hasText: 'The Kept Book' }).click()
  await waitFor('the kept book from the link', async () => (await page.locator('.write-doc').count()) > 0 && (await docText()).length > 0)
  await context.setOffline(false)
  expectFailures = false

  // 10. Forgetting the book.
  await waitFor('connected again', async () => (await page.locator('.offline-status').innerText()).trim() === 'Available offline', 30000)
  await page.locator('.offline-status').click()
  await page.locator('.offline-panel input[type="checkbox"]').uncheck()
  await waitFor('forgotten', async () => (await page.locator('.offline-status').innerText()).trim() === 'Offline: off')
  const left = await page.evaluate(async (story) => (await (await caches.open(`polaris-offline-${story}`)).keys()).length, t.story)
  assert.equal(left, 0)
  check("forgetting the book empties the story's cache")

  return { saved, errors, checks }
}

try {
  const { saved, errors, checks } = await run()
  console.log(`${checks.length} checks passed`)
  for (const f of saved) console.log(f)
  if (errors.length) {
    console.error('page errors:\n' + errors.join('\n'))
    process.exitCode = 1
  }
} catch (e) {
  console.error(e)
  process.exitCode = 1
} finally {
  await cleanUp()
}
