// Reading a story offline end to end (docs/DEVELOPMENT.md, "Reading
// offline"), without a screen: builds the app, embeds it in the real Go
// server, runs that on 127.0.0.1 with a throwaway data folder and story
// (never real data), and in headless Edge: keeps the story from the story
// picker, cuts the network and stops the server, then opens pages that were
// never opened before (the characters, one character's sheet, the wiki, the
// timeline, the map) and checks they show the kept story with its picture,
// that a save says it can't reach the server, and that removing the copy
// removes it.
//
//   node frontend/tests/visual/readOfflineShots.mjs [--out <dir>] [--no-build] [--channel chrome]
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
const PORT = 18094
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
  const data = mkdtempSync(join(tmpdir(), 'polaris-read-offline-'))
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

async function makeStory() {
  const form = new FormData()
  form.set('name', 'Read offline test')
  const story = await (await fetch(`${BASE}/api/stories`, { method: 'POST', body: form })).json()
  const api = `${BASE}/api/s/${story.id}`
  const people = []
  for (const name of ['Ayla Venn', 'Bren Oakhand']) {
    const c = new FormData()
    c.set('name', name)
    c.set('level', '3')
    c.set('picture', new Blob([png(160, 200)], { type: 'image/png' }), 'face.png')
    const res = await fetch(`${api}/characters`, { method: 'POST', body: c })
    if (!res.ok) throw new Error(`character: ${res.status} ${await res.text()}`)
    people.push(await res.json())
  }
  return { story: story.id, people }
}

// ---- In the browser --------------------------------------------------------------------------
async function run() {
  const exeData = build()
  const stopServer = await startServer(exeData)
  const t = await makeStory()
  const browser = await chromium.launch({ channel: option('channel', 'msedge'), headless: true })
  cleanups.push(() => browser.close())
  // A phone's screen: this is what the Android app shows.
  const context = await browser.newContext({ viewport: { width: 412, height: 880 }, locale: 'en-US', deviceScaleFactor: 2 })
  await context.addInitScript((story) => localStorage.setItem('polaris-story', story), t.story)
  const page = await context.newPage()
  const errors = []
  let expectFailures = false
  page.on('pageerror', (e) => errors.push(e.message))
  page.on('console', (m) => {
    if (m.type() !== 'error') return
    // With no connection, failed loads are expected.
    if (expectFailures && /Failed to load resource|ERR_INTERNET_DISCONNECTED|ERR_CONNECTION_REFUSED|Failed to fetch|NetworkError/.test(m.text())) return
    // An article nothing appears in, a body with no map of its own.
    if (/status of 40[04]/.test(m.text())) return
    errors.push(m.text())
  })
  mkdirSync(out, { recursive: true })
  const saved = []
  const snap = async (name) => {
    await page.evaluate(() => document.fonts.ready)
    await page.waitForTimeout(400)
    const file = join(out, name)
    await page.screenshot({ path: file })
    saved.push(file)
  }
  const check = (what) => console.log('check: ' + what)
  const body = () => page.locator('body').innerText()
  const banner = () => page.locator('.offline-banner').innerText()

  // 1. The story picker, with the service worker in charge of the page.
  await page.goto(`${BASE}/stories`)
  await page.waitForSelector('.card-name')
  await page.evaluate(() => navigator.serviceWorker.ready)
  await waitFor('the service worker to control the page', () => page.evaluate(() => !!navigator.serviceWorker.controller))

  // 2. Keep the story for reading offline.
  await page.locator('.card', { hasText: 'Read offline test' }).locator('.row-menu button').first().click()
  await page.getByRole('menuitem', { name: 'Keep for reading offline' }).click()
  await waitFor('the copy to be made', async () => (await body()).includes('can now be read on this device'), 120000)
  assert.match(await body(), /Kept for reading offline, just now/)
  const kept = await page.evaluate(async (story) => {
    const paths = async (name) => (await (await caches.open(name)).keys()).map((r) => new URL(r.url).pathname)
    const names = await caches.keys()
    const app = names.find((n) => n.startsWith('polaris-app-'))
    return { names, data: await paths(`polaris-data-${story}`), shared: await paths('polaris-data'), app: await paths(app), mark: JSON.parse(localStorage.getItem('polaris-kept-stories'))[story] }
  }, t.story)
  const api = `/api/s/${t.story}`
  for (const path of ['/characters', '/home', '/settings', '/wiki', '/timeline', '/map', `/characters/${t.people[0].id}/current`, `/characters/${t.people[1].id}/versions`, `/wiki/character/${t.people[0].id}`]) {
    assert.ok(kept.data.includes(api + path), `${path} is kept`)
  }
  assert.ok(kept.data.some((p) => p.startsWith('/uploads/')), `the pictures are kept (${kept.data.filter((p) => !p.startsWith('/api/')).join(', ')})`)
  assert.ok(kept.shared.includes('/api/stories'))
  assert.ok(kept.app.some((p) => /\/assets\/CharacterView-/.test(p)), 'pages never opened are kept too')
  assert.ok(kept.app.some((p) => /\/assets\/MapPage-/.test(p)))
  assert.equal(kept.mark.missing, 0, `nothing is missing (${JSON.stringify(kept.mark)})`)
  check(`the copy holds ${kept.data.length} answers and pictures, and all ${kept.app.length} of the app's files`)
  await snap('read_offline_01_kept.png')

  // 3. No connection: the network cut and the server stopped.
  expectFailures = true
  await context.setOffline(true)
  await stopServer()

  // 4. Pages never opened before, loaded from nothing.
  await page.goto(`${BASE}/characters`)
  await waitFor('the characters', async () => (await body()).includes('Ayla Venn'))
  assert.ok((await body()).includes('Bren Oakhand'))
  await waitFor('the banner', async () => (await banner()).includes('copy of the story kept on this device'))
  await waitFor('a kept picture to draw', () => page.evaluate(() => [...document.images].some((i) => i.src.includes('/uploads/') && i.complete && i.naturalWidth > 0)))
  check('the characters list opens offline, with its pictures and the banner')
  await snap('read_offline_02_characters.png')

  await page.goto(`${BASE}/characters/${t.people[0].id}`)
  await waitFor("a character's sheet", async () => (await body()).includes('Ayla Venn'))
  assert.ok(!(await body()).includes("can't reach the server"), 'the sheet loads whole')
  check("a character's sheet opens offline")
  await snap('read_offline_03_sheet.png')

  for (const [path, name] of [['/wiki', 'wiki'], ['/timeline', 'timeline'], ['/map', 'map'], ['/', 'home'], ['/wiki/character/' + t.people[1].id, 'article']]) {
    await page.goto(BASE + path)
    await waitFor(`the ${name} page`, async () => (await banner()).includes('kept on this device'))
    await page.waitForTimeout(1200)
    const text = await body()
    assert.ok(!/can't reach the server|Try again/.test(text), `the ${name} page loads from the copy:\n${text.slice(0, 600)}`)
    await snap(`read_offline_04_${name}.png`)
  }
  check('the wiki, an article, the timeline, the map and home open offline')

  // 5. Nothing can be saved; the story picker still lists the story.
  await page.goto(`${BASE}/stories`)
  await waitFor('the story picker', async () => (await body()).includes('Read offline test'))
  assert.match(await body(), /Kept for reading offline/)
  const save = await page.evaluate(async (story) => {
    try {
      const res = await fetch(`/api/s/${story}/chapters`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{"title":"x"}' })
      return String(res.status)
    } catch (e) {
      return 'failed'
    }
  }, t.story)
  assert.equal(save, 'failed', 'a save is never answered from the copy')
  check('saving offline fails as before')

  // 6. Removing the copy.
  await page.locator('.card', { hasText: 'Read offline test' }).locator('.row-menu button').first().click()
  await page.getByRole('menuitem', { name: 'Remove the offline copy' }).click()
  await waitFor('the copy to go', () => page.evaluate(async (story) => !(await caches.keys()).includes(`polaris-data-${story}`), t.story))
  assert.ok(!(await body()).includes('Kept for reading offline'))
  check('removing the copy deletes it')

  assert.deepEqual(errors, [], 'no errors in the page')
  console.log(`\nall good; pictures:\n  ${saved.join('\n  ')}`)
}

run()
  .catch((err) => {
    console.error(err)
    process.exitCode = 1
  })
  .finally(cleanUp)
