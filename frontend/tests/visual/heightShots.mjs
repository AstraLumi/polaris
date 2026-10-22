// Screenshots of the height chart and the body type form, to see them
// without a screen. Like climateShots.mjs: it builds the backend and runs it
// on a throwaway data folder (never a real story), starts the Vite dev
// server, makes a test story with body types and a small cast of different
// heights, ages and builds, and opens the pages in headless Microsoft Edge.
//
//   node frontend/tests/visual/heightShots.mjs [--size 1400x900] [--ft] [--out <dir>]
//
// Needs Go and Microsoft Edge.
import { spawn, execFileSync } from 'node:child_process'
import { mkdtempSync, mkdirSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, resolve, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium } from 'playwright-core'
import { createServer } from 'vite'

const here = dirname(fileURLToPath(import.meta.url))
const frontend = resolve(here, '../..')
const backend = resolve(frontend, '../backend')
const API_PORT = 18093
const WEB_PORT = 15175

const args = process.argv.slice(2)
const flag = (name) => args.includes(`--${name}`)
const option = (name, fallback) => {
  const i = args.indexOf(`--${name}`)
  return i >= 0 && args[i + 1] ? args[i + 1] : fallback
}
const [width, height] = option('size', '1400x900').split('x').map(Number)
const out = resolve(option('out', join(here, 'out')))

const cleanups = []
async function cleanUp() {
  for (const f of cleanups.reverse()) await f()
}

async function waitFor(what, test, ms = 60000) {
  const until = Date.now() + ms
  for (;;) {
    try {
      if (await test()) return
    } catch {
      // not yet
    }
    if (Date.now() > until) throw new Error(`timed out waiting for ${what}`)
    await new Promise((r) => setTimeout(r, 200))
  }
}

async function startBackend() {
  const data = mkdtempSync(join(tmpdir(), 'polaris-heights-'))
  cleanups.push(() => rmSync(data, { recursive: true, force: true, maxRetries: 5, retryDelay: 200 }))
  const exe = join(data, process.platform === 'win32' ? 'polaris.exe' : 'polaris')
  execFileSync('go', ['build', '-o', exe, '.'], { cwd: backend, stdio: 'inherit' })
  const proc = spawn(exe, [], {
    cwd: backend,
    env: { ...process.env, LISTEN_HOST: '127.0.0.1', PORT: String(API_PORT), DATA_DIR: join(data, 'data'), DB_PATH: join(data, 'data', 'polaris.db') },
    stdio: ['ignore', 'ignore', 'pipe'],
  })
  let log = ''
  proc.stderr.on('data', (d) => (log += d))
  const exited = new Promise((r) => proc.once('exit', r))
  cleanups.push(async () => {
    proc.kill()
    await exited
  })
  await waitFor('the backend', async () => (await fetch(`http://127.0.0.1:${API_PORT}/api/health`)).ok).catch((err) => {
    throw new Error(`${err.message}\n${log}`)
  })
}

async function startFrontend() {
  const server = await createServer({
    root: frontend,
    logLevel: 'error',
    server: {
      host: '127.0.0.1',
      port: WEB_PORT,
      strictPort: true,
      proxy: { '/api': `http://127.0.0.1:${API_PORT}`, '/uploads': `http://127.0.0.1:${API_PORT}` },
    },
  })
  await server.listen()
  cleanups.push(() => server.close())
}

// Body types with outlines, and a cast: [name, height cm, gender, age, body type].
const BODY_TYPES = [
  ['Lean', { sex: '', age: '', build: 'slim' }],
  ['Built', { sex: '', age: '', build: 'athletic' }],
  ['Round', { sex: '', age: '', build: 'chubby' }],
  ['Giant', { sex: '', age: '', build: 'heavy' }],
]
const CAST = [
  ['Aria', 168, 'Female', 24, 'Lean'],
  ['Borin', 142, 'Male', 140, 'Round'],
  ['Cael', 191, 'Male', 31, 'Built'],
  ['Dot', 118, 'Female', 7, ''],
  ['Edda', 158, 'Female', 78, 'Round'],
  ['Fenn', 172, '', 16, ''],
  ['Grum', 265, 'Male', 45, 'Giant'],
  ['Old Tam', 170, 'Male', 82, 'Lean'],
]

async function makeStory() {
  const api = `http://127.0.0.1:${API_PORT}/api`
  const form = new FormData()
  form.set('name', 'Height screenshots')
  const story = await (await fetch(`${api}/stories`, { method: 'POST', body: form })).json()
  const s = `${api}/s/${story.id}`
  const send = async (method, path, body) => {
    const res = await fetch(s + path, body instanceof FormData ? { method, body } : { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) })
    if (!res.ok) throw new Error(`${path}: ${await res.text()}`)
    return res.json()
  }
  for (const [name, outline] of BODY_TYPES) await send('POST', '/body-types', { name, modifiers: {}, outline })
  const ids = []
  for (const [name, cm, gender, age, bodyType] of CAST) {
    const f = new FormData()
    f.set('name', name)
    const made = await send('POST', '/characters', f)
    const v = new FormData()
    for (const [k, val] of Object.entries({ name, level: '1', gender, age: String(age), height: String(cm), weight: '', body_type: bodyType, status: 'alive' })) v.set(k, val)
    await send('PUT', `/versions/${made.version_id}`, v)
    ids.push(made.id)
  }
  return { id: story.id, ids }
}

async function shoot({ id, ids }) {
  const browser = await chromium.launch({ channel: option('channel', 'msedge'), headless: true })
  cleanups.push(() => browser.close())
  const context = await browser.newContext({ viewport: { width, height }, locale: 'en-US' })
  await context.addInitScript(([story, ft]) => {
    localStorage.setItem('polaris-story', story)
    if (ft) localStorage.setItem('polaris-height-unit', 'ft')
  }, [id, flag('ft')])
  const page = await context.newPage()
  const errors = []
  page.on('pageerror', (e) => errors.push(e.message))
  page.on('console', (m) => m.type() === 'error' && errors.push(m.text()))
  mkdirSync(out, { recursive: true })
  const saved = []
  const snap = async (name) => {
    const file = join(out, name)
    await page.screenshot({ path: file, fullPage: true })
    saved.push(file)
  }
  await page.goto(`http://127.0.0.1:${WEB_PORT}/characters/heights?c=${ids.join(',')}`)
  await page.waitForSelector('.chart-svg')
  await page.waitForTimeout(800)
  await snap(`heights${flag('ft') ? '_ft' : ''}.png`)
  await page.goto(`http://127.0.0.1:${WEB_PORT}/assets`)
  await page.getByRole('button', { name: 'Body Types', exact: true }).click()
  await page.waitForTimeout(500)
  await snap('body_types.png')
  await page.getByRole('button', { name: 'Add body type', exact: true }).click()
  await page.waitForSelector('.outline-picker')
  await page.getByRole('radio', { name: 'Chubby' }).click()
  await page.getByRole('radio', { name: 'Female', exact: true }).click()
  await page.getByRole('radio', { name: 'Elderly', exact: true }).click()
  await page.waitForTimeout(300)
  const file = join(out, 'body_type_form.png')
  await page.locator('.modal').screenshot({ path: file })
  saved.push(file)
  return { saved, errors }
}

try {
  await startBackend()
  await startFrontend()
  const { saved, errors } = await shoot(await makeStory())
  for (const f of saved) console.log(f)
  if (errors.length) console.log('page errors:\n' + errors.join('\n'))
} catch (err) {
  console.error(err)
  process.exitCode = 1
} finally {
  await cleanUp()
}
