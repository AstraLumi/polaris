// Screenshots of the planet map's continents and climate layers, to see
// them without a screen. Like shots.mjs: it builds the backend and runs it on
// a throwaway data folder (never a real story), starts the Vite dev server,
// makes a test story whose planet is a rough Earth (tests/earthLike.mjs:
// continents with forests, deserts, ice and mountains), opens the Map page
// in headless Microsoft Edge and saves a picture of each layer.
//
//   node frontend/tests/visual/climateShots.mjs [options]
//     --layers a,b,…   continents, wind, currents, temperature, rain, zones
//                      (default: all of them)
//     --month <n>      a month (1…12) instead of the whole year
//     --face <lon,lat> where the globe looks (default -30,25: the Atlantic)
//     --flat           the flat map instead of the globe
//     --hover <x,y>    rest the pointer there (to see a hex's readout)
//     --sky <lon,lat>  also open the sky view from that hex (the story gets
//                      a Sun, the planet and a Moon), with:
//     --at <date,hour> its date (DD-MM-YYYY) and local hour, default
//                      01-01-0001,0
//     --look <az,alt,fov>  where it looks, default 180,30,90
//     --dyson, --shell the Sun in a Dyson sphere; the system in its shell
//     --glow           the Moon shines on its own
//     --solar          then the planet in the Solar system's 3D view
//     --view <name>    the body it looks at there (default Terra)
//     --size <WxH>     the window, default 1400x900
//     --out <dir>      where the pictures go (default tests/visual/out)
//
// Needs Go and Microsoft Edge.
import { spawn, execFileSync } from 'node:child_process'
import { mkdtempSync, mkdirSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, resolve, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium } from 'playwright-core'
import { createServer } from 'vite'
import { earthLike } from '../earthLike.mjs'
import { frequencyFor } from '../../src/planetGrid.js'

const here = dirname(fileURLToPath(import.meta.url))
const frontend = resolve(here, '../..')
const backend = resolve(frontend, '../backend')
const API_PORT = 18092
const WEB_PORT = 15174

const args = process.argv.slice(2)
const flag = (name) => args.includes(`--${name}`)
const option = (name, fallback) => {
  const i = args.indexOf(`--${name}`)
  return i >= 0 && args[i + 1] ? args[i + 1] : fallback
}
const LABELS = { continents: 'Continents', wind: 'Winds', currents: 'Ocean currents', temperature: 'Temperature', rain: 'Rain', clouds: 'Clouds', zones: 'Climate types' }
const layers = option('layers', Object.keys(LABELS).join(',')).split(',').map((v) => v.trim()).filter(Boolean)
const month = Number(option('month', 0))
const [faceLon, faceLat] = option('face', '-30,25').split(',').map(Number)
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
  const data = mkdtempSync(join(tmpdir(), 'polaris-climate-'))
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

// The biomes of the rough Earth, from what earthLike() sets; its high
// ground (not the ice) is painted as mountains terrain at its height,
// covered in boreal forest.
function biomeOfCell(map, c) {
  if (map.albedo[c] >= 0.69) return 'ice'
  if (map.veg[c] <= 0.02) return 'desert'
  if (map.veg[c] >= 1) return 'rainforest'
  if (map.height[c] >= 2000) return 'boreal_forest'
  return null
}

async function makeStory() {
  const api = `http://127.0.0.1:${API_PORT}/api`
  const form = new FormData()
  form.set('name', 'Climate screenshots')
  const story = await (await fetch(`${api}/stories`, { method: 'POST', body: form })).json()
  const s = `${api}/s/${story.id}`
  const send = async (method, path, body) => {
    const res = await fetch(s + path, { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) })
    if (!res.ok) throw new Error(`${path}: ${await res.text()}`)
    return res.json()
  }
  const planetKm = 40075
  const hexKm = 200
  await send('PUT', '/settings/map', { mode: '3d', planet_km: planetKm, hex_km: hexKm, tilt: 23.44, bestiary_projection: 'equirectangular', show_equator: false, show_zones: false })
  const map = earthLike(frequencyFor(planetKm, hexKm))
  const land = []
  for (let c = 0; c < map.grid.count; c++) if (map.land[c]) land.push(c)
  const continent = await send('POST', '/planet/features', { layer: 'continents', name: 'Terra', color: '#8a9a5b' })
  await send('POST', '/planet/features/paint', { feature_id: continent.id, cells: land })
  const byBiome = new Map()
  const byHeight = new Map()
  for (const c of land) {
    const b = biomeOfCell(map, c)
    if (b) {
      if (!byBiome.has(b)) byBiome.set(b, [])
      byBiome.get(b).push(c)
    }
    if (map.height[c] >= 2000 && map.albedo[c] < 0.69) {
      if (!byHeight.has(map.height[c])) byHeight.set(map.height[c], [])
      byHeight.get(map.height[c]).push(c)
    }
  }
  for (const [biome, cells] of byBiome) await send('POST', '/planet/biomes/paint', { biome, height: null, cells })
  if (option('sky', '')) {
    // A Sun, the planet going round it, and a Moon on Earth's tilted orbit.
    const sun = await send('POST', '/solar/bodies', { kind: 'star', name: 'Sun', body_type: 'star', star_class: 'yellow_dwarf', radius_km: 696000, temperature_k: 5772, mass_kg: 1.989e30, free_values: 0, dyson: flag('dyson') })
    const sunId = sun.bodies.find((b) => b.name === 'Sun').id
    const home = await send('POST', '/solar/bodies', { kind: 'planet', parent_id: sunId, name: 'Terra', body_type: 'rocky', radius_km: 6371, orbit_km: 1.496e8, phase: 0, is_main: true })
    const homeId = home.bodies.find((b) => b.name === 'Terra').id
    await send('POST', '/solar/bodies', { kind: 'moon', parent_id: homeId, name: 'Luna', body_type: 'rocky', radius_km: 1737, orbit_km: 384400, phase: Number(option('moon', '0')), inclination: 5.14, ascending_node: 90, glows: flag('glow') })
    await send('POST', '/solar/bodies', { kind: 'planet', parent_id: sunId, name: 'Rubra', body_type: 'rocky', radius_km: 3390, orbit_km: 2.28e8, phase: 200 })
    await send('POST', '/solar/bodies', { kind: 'planet', parent_id: sunId, name: 'Vesper', body_type: 'rocky', radius_km: 6052, orbit_km: 1.08e8, phase: 330 })
    if (flag('shell')) await send('PUT', '/solar/enclosure', { enclosed: true })
  }
  for (const [height, cells] of byHeight) await send('POST', '/planet/relief/paint', { relief: 'mountains', height, cells })
  return story.id
}

async function shoot(storyId) {
  const browser = await chromium.launch({ channel: option('channel', 'msedge'), headless: true, args: ['--enable-gpu', '--ignore-gpu-blocklist'] })
  cleanups.push(() => browser.close())
  const context = await browser.newContext({ viewport: { width, height }, locale: 'en-US' })
  await context.addInitScript(([id, flat]) => {
    localStorage.setItem('polaris-story', id)
    localStorage.setItem('polaris-planet-view', flat ? 'flat' : 'globe')
  }, [storyId, flag('flat')])
  const page = await context.newPage()
  const errors = []
  page.on('pageerror', (e) => errors.push(e.message))
  page.on('console', (m) => m.type() === 'error' && errors.push(m.text()))
  await page.goto(`http://127.0.0.1:${WEB_PORT}/map`)
  await page.waitForSelector(flag('flat') ? '.flat-canvas' : '.globe-canvas')
  await page.waitForTimeout(1500)
  mkdirSync(out, { recursive: true })
  const saved = []
  for (const id of layers) {
    await page.getByRole('radio', { name: LABELS[id], exact: true }).click()
    if (id !== 'continents') {
      // The climate starts working a moment after the layer is picked.
      await page.waitForTimeout(500)
      await page.waitForFunction(() => !document.body.innerText.includes('Working out the climate'), null, { timeout: 60000 })
      if (month) await page.locator('.climate select').selectOption(String(month - 1))
    }
    if (!flag('flat')) {
      // Face the place asked for: the globe turns so that cell faces us.
      await page.evaluate(([lon, lat]) => {
        const d = Math.PI / 180
        const p = [Math.cos(lat * d) * Math.cos(lon * d), Math.cos(lat * d) * Math.sin(lon * d), Math.sin(lat * d)]
        window.__planetGlobe?.facePoint(p)
      }, [faceLon, faceLat])
    }
    const hover = option('hover', '')
    if (hover) {
      const [hx, hy] = hover.split(',').map(Number)
      await page.mouse.move(hx, hy)
    } else await page.mouse.move(width - 2, height - 2)
    await page.waitForTimeout(800)
    const file = join(out, `climate_${id}${month ? '_m' + month : ''}${flag('flat') ? '_flat' : ''}.png`)
    await page.screenshot({ path: file })
    saved.push(file)
  }
  const sky = option('sky', '')
  if (sky) {
    const [lon, lat] = sky.split(',').map(Number)
    const [date, hour] = option('at', '01-01-0001,0').split(',')
    const [az, alt, fov] = option('look', '180,30,90').split(',').map(Number)
    await page.evaluate(([lo, la]) => window.__planetMap.openSky(lo, la), [lon, lat])
    await page.waitForFunction(() => window.__skyView?.ready() && window.__planetMap.climateReady(), null, { timeout: 90000 })
    await page.evaluate((s) => window.__skyView.set(s), { date, hour: Number(hour), az, alt, fov })
    await page.mouse.move(width - 2, height - 2)
    await page.waitForTimeout(1200)
    const file = join(out, `sky_${date}_${hour}_${az}_${alt}${flag('shell') ? '_shell' : ''}.png`)
    await page.screenshot({ path: file })
    saved.push(file)
    const info = await page.evaluate(() => {
      const s = window.__skyView.sky()
      return { sun: [s.sun.alt, s.sun.az].map((x) => +(x * 57.2958).toFixed(1)), bodies: s.bodies.map((b) => [b.body.name, +(b.alt * 57.2958).toFixed(1), +(b.az * 57.2958).toFixed(1), +b.lit.toFixed(2), +b.mag.toFixed(1)]) }
    })
    console.log(JSON.stringify(info))
    await page.keyboard.press('Escape')
  }
  if (flag('solar') && sky) {
    // The planet in the solar system's 3D view, its clouds from its climate.
    await page.goto(`http://127.0.0.1:${WEB_PORT}/solar-system`)
    await page.getByRole('button', { name: 'Run', exact: true }).click()
    await page.waitForFunction(() => window.__solarScene?.skyLoaded() && window.__solarPage)
    await page.waitForTimeout(6000)
    await page.evaluate((who) => window.__solarScene.view(who, { zoom: 1 }), option('view', 'Terra'))
    await page.waitForTimeout(1500)
    const file = join(out, 'solar_terra.png')
    await page.screenshot({ path: file })
    saved.push(file)
  }
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
