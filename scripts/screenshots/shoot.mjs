// Takes the README screenshots (see README.md here): headless Edge or Chrome
// driven over the DevTools protocol with Node's own WebSocket, no packages.
// It first builds the demo story from seed-demo.js on the server it is given,
// so point it at a server with an empty, throwaway data folder.
//   node scripts/screenshots/shoot.mjs <base url> <out dir>
import { spawn } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import os from 'node:os'
import { fileURLToPath } from 'node:url'

const [base, outDir] = process.argv.slice(2)
const here = path.dirname(fileURLToPath(import.meta.url))
// Any Chromium browser works; set BROWSER to its path if it isn't Edge's usual one.
const EDGE = process.env.BROWSER || 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'
const PORT = 9333
const wait = (ms) => new Promise((r) => setTimeout(r, ms))

const edge = spawn(EDGE, [
  '--headless=new', '--lang=en-US', `--remote-debugging-port=${PORT}`, `--user-data-dir=${fs.mkdtempSync(path.join(os.tmpdir(), 'polaris-shots-'))}`,
  '--window-size=1440,900', '--hide-scrollbars', '--force-device-scale-factor=1', '--no-first-run', 'about:blank',
], { stdio: 'ignore' })

let target
for (let i = 0; i < 50 && !target; i++) {
  try {
    target = (await (await fetch(`http://127.0.0.1:${PORT}/json/list`)).json()).find((t) => t.type === 'page')
  } catch {
    await wait(200)
  }
}
const ws = new WebSocket(target.webSocketDebuggerUrl)
await new Promise((r) => ws.addEventListener('open', r))
let seq = 0
const pending = new Map()
const events = []
ws.addEventListener('message', (m) => {
  const msg = JSON.parse(m.data)
  if (msg.id && pending.has(msg.id)) {
    pending.get(msg.id)(msg)
    pending.delete(msg.id)
  } else if (msg.method) events.push(msg.method)
})
const send = (method, params = {}) =>
  new Promise((resolve, reject) => {
    const id = ++seq
    pending.set(id, (msg) => (msg.error ? reject(new Error(method + ': ' + msg.error.message)) : resolve(msg.result)))
    ws.send(JSON.stringify({ id, method, params }))
  })
const evaluate = async (expression) => {
  const r = await send('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true })
  if (r.exceptionDetails) throw new Error(r.exceptionDetails.exception?.description || r.exceptionDetails.text)
  return r.result.value
}
const size = (width, height) => send('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile: false })
const go = async (url, settle = 1200, width = 1440, height = 900) => {
  await size(width, height)
  await send('Page.navigate', { url: base + url })
  await wait(settle)
}
const shot = async (name, width = 1440, height = 900) => {
  await size(width, height)
  await wait(500)
  const { data } = await send('Page.captureScreenshot', { format: 'png' })
  fs.writeFileSync(path.join(outDir, name), Buffer.from(data, 'base64'))
  console.log('saved', name)
}

try {
  await send('Page.enable')
  await send('Runtime.enable')
  await size(1440, 900)
  await go('/stories')
  const { story, ids } = await evaluate(fs.readFileSync(path.join(here, 'seed-demo.js'), 'utf8'))
  console.log('seeded', story)
  await evaluate(`localStorage.setItem('polaris-story', ${JSON.stringify(story)}); localStorage.setItem('polaris-locale', 'en-US'); localStorage.setItem('timeline-settings', JSON.stringify({ pxPerYear: null, collapseYears: 3 })); 1`)

  await go(`/wiki/character/${ids.people.aria}`, 1800, 1440, 1000)
  await shot('wiki.png', 1440, 1000)

  await go('/wiki/graph', 3500)
  await evaluate(`[...document.querySelectorAll('button')].find((b) => b.textContent.trim() === 'Fit').click(); 1`)
  await wait(800)
  // Hover Aria (the only filled yellow dot: the one written character).
  const at = await evaluate(`(() => {
    const c = document.querySelector('.graph-canvas'), r = c.getBoundingClientRect(), d = c.getContext('2d').getImageData(0, 0, c.width, c.height).data
    for (let y = 0; y < c.height; y += 2) for (let x = 0; x < c.width; x += 2) {
      const i = (y * c.width + x) * 4
      const j = ((y + 8) * c.width + x) * 4 // filled, not just a ring: yellow 8px lower too
      const yellow = (k) => Math.abs(d[k] - 242) < 6 && Math.abs(d[k + 1] - 198) < 6 && Math.abs(d[k + 2] - 109) < 6
      if (yellow(i) && yellow(j) && yellow(j - 24) && yellow(j + 24)) return { x: r.left + x, y: r.top + y + 8 }
    }
    return null
  })()`)
  if (at) {
    await send('Input.dispatchMouseEvent', { type: 'mouseMoved', x: at.x, y: at.y })
    await wait(1200)
  }
  await shot('graph.png')

  await go(`/characters/${ids.people.aria}/family`, 1800)
  await shot('family.png')

  await go(`/timeline?person=${ids.people.aria}`, 1800)
  await shot('timeline.png')

  await go(`/map`, 1800)
  await shot('map.png')

  await go(`/characters/${ids.people.aria}`, 1500, 1440, 1000)
  await evaluate(`(async () => { for (let i = 0; i < 40; i++) { const b = [...document.querySelectorAll('.tab-button')].find((x) => x.textContent.trim() === 'Build'); if (b) { b.click(); return 1 } await new Promise((r) => setTimeout(r, 100)) } throw new Error('no Build tab') })()`)
  await wait(600)
  await shot('character.png', 1440, 1000)

} finally {
  ws.close()
  edge.kill()
}
