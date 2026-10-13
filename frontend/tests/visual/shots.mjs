// Screenshots of the Solar system's 3D view, to see how it looks without a
// screen. It builds the backend and runs it on a throwaway data folder (never
// a real story), starts the Vite dev server against it, makes a test solar
// system through the API, opens it in headless Microsoft Edge (playwright-core
// drives the Edge installed with Windows; nothing else is downloaded) and
// saves a picture from each camera view.
//
//   node frontend/tests/visual/shots.mjs [options]
//     --views a,b,…    angle, above, side, or a body's name (Sun, Home, Luna,
//                      Giant, Comet, Oceanus); name:2 is twice as far out,
//                      name@sun looks at it from the star's side.
//                      Default: angle,above,side,Giant,Home
//     --days <n>       run the clock to day n first
//     --gravity        with live gravity
//     --star <class>   the star's class: yellow_dwarf (default), red_giant,
//                      white_dwarf, brown_dwarf, neutron_star, black_hole
//     --dyson          the star inside a Dyson sphere
//     --set <changes>  change the test system: Body.field=value, comma
//                      separated (Luna.phase=200 puts the Moon between Home
//                      and the Sun: an eclipse)
//     --panel          keep the side panel open (closed by default)
//     --full           the whole page, not just the 3D view
//     --size <WxH>     the window, default 1400x900
//     --software       draw with SwiftShader instead of the graphics card
//     --out <dir>      where the pictures go (default tests/visual/out,
//                      which git ignores)
//
// Needs Go and Microsoft Edge (or --channel chrome for Google Chrome).
import { spawn, execFileSync } from 'node:child_process'
import { mkdtempSync, mkdirSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, resolve, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium } from 'playwright-core'
import { createServer } from 'vite'
import { STAR_CLASSES, fitToClass } from '../../src/stars.js'

const here = dirname(fileURLToPath(import.meta.url))
const frontend = resolve(here, '../..')
const backend = resolve(frontend, '../backend')
const API_PORT = 18091
const WEB_PORT = 15173

// ---- Options ----------------------------------------------------------------------------
const args = process.argv.slice(2)
const flag = (name) => args.includes(`--${name}`)
const option = (name, fallback) => {
  const i = args.indexOf(`--${name}`)
  return i >= 0 && args[i + 1] ? args[i + 1] : fallback
}
const views = option('views', 'angle,above,side,Giant,Home').split(',').map((v) => v.trim()).filter(Boolean)
const days = Number(option('days', 0))
const starClass = option('star', 'yellow_dwarf')
if (!STAR_CLASSES[starClass]) throw new Error(`unknown star class ${starClass}`)
const [width, height] = option('size', '1400x900').split('x').map(Number)
const out = resolve(option('out', join(here, 'out')))

// ---- The test system ---------------------------------------------------------------------
// One of everything worth looking at: a main planet with a moon, a ringed
// gas giant, an ocean world, and an eccentric comet on a tilted orbit.
function testSystem() {
  const star = fitToClass({ kind: 'star', name: 'Sun', body_type: 'star', star_class: starClass, free_values: 0, dyson: flag('dyson') })
  const bodies = [
    star,
    { kind: 'planet', parent: 'Sun', name: 'Home', body_type: 'rocky', radius_km: 6371, orbit_km: 1.496e8, phase: 20, is_main: true },
    { kind: 'moon', parent: 'Home', name: 'Luna', body_type: 'rocky', radius_km: 1737, orbit_km: 384400, phase: 90 },
    { kind: 'planet', parent: 'Sun', name: 'Oceanus', body_type: 'ocean', radius_km: 9000, orbit_km: 2.6e8, phase: 250, axial_tilt: 15 },
    { kind: 'planet', parent: 'Sun', name: 'Comet', body_type: 'rocky', radius_km: 800, orbit_km: 4e8, phase: 330, eccentricity: 0.75, periapsis: 300, inclination: 35, ascending_node: 60 },
    { kind: 'planet', parent: 'Sun', name: 'Giant', body_type: 'gas', radius_km: 70000, orbit_km: 7.7e8, phase: 140, axial_tilt: 26, ring_inner: 1.2, ring_outer: 2.3 },
  ]
  for (const change of option('set', '').split(',').filter(Boolean)) {
    const [, name, field, value] = change.match(/^\s*(\w+)\.(\w+)=(.*)$/) ?? []
    const body = bodies.find((b) => b.name === name)
    if (!body) throw new Error(`--set: no body called ${name}`)
    body[field] = value === 'true' || value === 'false' ? value === 'true' : Number.isNaN(Number(value)) ? value : Number(value)
  }
  return bodies
}

// ---- Servers -------------------------------------------------------------------------------
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

async function waitFor(what, test, ms = 60000) {
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

async function startBackend() {
  const data = mkdtempSync(join(tmpdir(), 'polaris-shots-'))
  cleanups.push(() => rmSync(data, { recursive: true, force: true, maxRetries: 5, retryDelay: 200 }))
  const exe = join(data, process.platform === 'win32' ? 'polaris.exe' : 'polaris')
  execFileSync('go', ['build', '-o', exe, '.'], { cwd: backend, stdio: 'inherit' })
  const proc = spawn(exe, [], {
    cwd: backend,
    // Only this machine: no firewall prompt for the throwaway server.
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

// The test story and its solar system; returns the story's id.
async function makeStory() {
  const api = `http://127.0.0.1:${API_PORT}/api`
  const form = new FormData()
  form.set('name', 'Screenshots')
  const story = await (await fetch(`${api}/stories`, { method: 'POST', body: form })).json()
  const ids = new Map()
  for (const { parent, ...body } of testSystem()) {
    if (parent) body.parent_id = ids.get(parent)
    const res = await fetch(`${api}/s/${story.id}/solar/bodies`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) })
    if (!res.ok) throw new Error(`${body.name}: ${await res.text()}`)
    for (const b of (await res.json()).bodies) ids.set(b.name, b.id)
  }
  // Oceanus wears a map of its own (islands): the globe mesh, as the main
  // planet's would on a 3D map.
  const islands = Array.from({ length: 400 }, (_, k) => (k * 37) % 2000)
  const paint = await fetch(`${api}/s/${story.id}/solar/bodies/${ids.get('Oceanus')}/map/paint`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ color: '#3f8f4a', cells: islands }),
  })
  if (!paint.ok) throw new Error(`paint: ${await paint.text()}`)
  return story.id
}

// ---- Pictures --------------------------------------------------------------------------------
async function shoot(storyId) {
  const browser = await chromium.launch({
    channel: option('channel', 'msedge'),
    headless: true,
    args: flag('software') ? ['--use-angle=swiftshader', '--enable-unsafe-swiftshader'] : ['--enable-gpu', '--ignore-gpu-blocklist'],
  })
  cleanups.push(() => browser.close())
  const context = await browser.newContext({ viewport: { width, height }, locale: 'en-US' })
  await context.addInitScript((id) => localStorage.setItem('polaris-story', id), storyId)
  const page = await context.newPage()
  const errors = []
  page.on('pageerror', (e) => errors.push(e.message))
  page.on('console', (m) => m.type() === 'error' && errors.push(m.text()))

  await page.goto(`http://127.0.0.1:${WEB_PORT}/solar-system`)
  await page.getByRole('button', { name: 'Run', exact: true }).click()
  await page.waitForFunction(() => window.__solarScene?.skyLoaded() && window.__solarPage)
  if (!flag('panel')) await page.locator('.system-panel .fold').click()
  // The pointer out of the way: no tooltips in the pictures.
  await page.mouse.move(width - 2, height - 2)
  if (flag('gravity')) await page.evaluate(() => window.__solarPage.live(true))
  if (days > 0) {
    // Gravity gets there a little at a time: keep asking.
    await page.evaluate(async (target) => {
      for (let k = 0; k < 2000 && window.__solarPage.days() < target - 1e-9; k++) {
        window.__solarPage.moveTo(target)
        await new Promise((r) => setTimeout(r, 30))
      }
    }, days)
  }

  mkdirSync(out, { recursive: true })
  const saved = []
  for (const view of views) {
    const [where, fromSun] = view.split('@')
    const [name, zoom] = where.split(':')
    const ok = await page.evaluate(([n, z, f]) => window.__solarScene.view(n, { zoom: Number(z) || 1, fromSun: f }), [name, zoom, fromSun === 'sun'])
    if (!ok) {
      console.warn(`no view or body called ${name}`)
      continue
    }
    // Two more frames drawn after the camera moved: everything has caught up.
    for (let k = 0; k < 2; k++) {
      const from = await page.evaluate(() => (window.__solarScene.draw(), window.__solarScene.frames()))
      await page.waitForFunction((f) => window.__solarScene.frames() > f, from)
    }
    const file = join(out, `${view.replace(/[^\w.-]+/g, '_')}.png`)
    if (flag('full')) await page.screenshot({ path: file })
    else await page.locator('.solar-wrap').screenshot({ path: file })
    saved.push(file)
  }
  const day = await page.locator('.day-label').textContent()
  return { saved, errors, day }
}

try {
  await startBackend()
  await startFrontend()
  const { saved, errors, day } = await shoot(await makeStory())
  console.log(`${day}${flag('gravity') ? ', gravity' : ''}`)
  for (const f of saved) console.log(f)
  if (errors.length) console.log('page errors:\n' + errors.join('\n'))
} catch (err) {
  console.error(err)
  process.exitCode = 1
} finally {
  await cleanUp()
}
