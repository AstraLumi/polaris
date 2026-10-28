// Screenshots of the writing page (/write/:volume) without a screen: the
// page type chooser, an A6 book, the styles panel, find and replace, the
// saved versions, a save conflict, an Online Post book and a phone. The
// server is faked in memory with page.route (no backend, no real data), so
// it runs while the server side of the editor is still being built. Also
// checks a few things the screenshots can't show: clicking a word puts the
// cursor in it at any zoom, pasting Word HTML keeps its formatting, typing
// helpers and autosave.
//
//   node frontend/tests/visual/writeShots.mjs [--out <dir>] [--size 1400x900]
//
// Needs Microsoft Edge (or --channel chrome).
import { mkdirSync } from 'node:fs'
import { join, resolve, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'
import assert from 'node:assert/strict'
import { chromium } from 'playwright-core'
import { createServer } from 'vite'
import { defaultSettings } from '../../src/editor/pageTypes.js'
import { countDocJSON } from '../../src/editor/wordCount.js'

const here = dirname(fileURLToPath(import.meta.url))
const frontend = resolve(here, '../..')
const WEB_PORT = 15177
const BASE = `http://127.0.0.1:${WEB_PORT}`

const args = process.argv.slice(2)
const option = (name, fallback) => {
  const i = args.indexOf(`--${name}`)
  return i >= 0 && args[i + 1] ? args[i + 1] : fallback
}
const [width, height] = option('size', '1400x900').split('x').map(Number)
const out = resolve(option('out', join(here, 'out')))

// ---- The fake story -------------------------------------------------------------
const STORY = 'shots'
const txt = (text, ...marks) => (marks.length ? { type: 'text', text, marks: marks.map((m) => (typeof m === 'string' ? { type: m } : m)) } : { type: 'text', text })
const para = (attrs, ...content) => ({ type: 'paragraph', ...(attrs ? { attrs } : {}), content })

const a6 = defaultSettings('a6')
a6.styles.find((s) => s.id === 'normal').text.font = 'Merriweather'
a6.styles.push({
  id: 'signpost',
  name: 'Signpost',
  kind: 'paragraph',
  based_on: 'normal',
  builtin: false,
  para: { align: 'center', indentFirst: 0, spaceBefore: 6, spaceAfter: 6 },
  text: { font: 'Turret Road', size: 9 },
})

const chapterOne = {
  type: 'doc',
  content: [
    { type: 'heading', attrs: { level: 1 }, content: [txt('Chapter One')] },
    para({ style: 'signpost' }, txt('MILESTONES')),
    para({ indentFirst: 0 }, txt('The', 'smallcaps'), txt(' lantern swung from the cart’s hook, and Mira counted the milestones the way her mother had taught her: one for the road, one for the rain, one for whatever waited at the end.')),
    para(null, txt('“We’re late,” said Oren. He did not look up from the reins. '), txt('Late', 'italic'), txt(' was the only word he had spoken since the ford, and he spoke it the way other men cursed.')),
    para(null, txt('She pulled the map from her coat. Somewhere past the third hill the ink had run, and the town of '), txt('Hollowmere', 'bold'), txt(' was only a blot with a name beside it.')),
    { type: 'scene_break' },
    para({ indentFirst: 0 }, txt('By morning the rain had stopped. The road ran on—narrower now, the hedges leaning in—and the lantern had burned down to a coal…')),
    para({ style: 'signpost' }, txt('Here the old road ends')),
    para({ style: 'quote' }, txt('Whoever finds this, keep walking. The town is not where the map says it is.')),
    para(null, txt('Mira read the sign twice, then folded the map into a square small enough to '), txt('forget', { type: 'color', attrs: { color: '#9c2a1e' } }), txt('.')),
  ],
}

const onlineDoc = {
  type: 'doc',
  content: [
    { type: 'heading', attrs: { level: 1 }, content: [txt('Episode 1: The Ford')] },
    para(null, txt('Posting this one a day early because the next chapter is a monster. Thanks for reading!', 'italic')),
    para(null, txt('The ford was higher than Oren had promised. Mira watched the water climb the cart’s wheels and said nothing, because saying something would make it real.')),
    para(null, txt('“Hold the lantern,” he said.')),
    para(null, txt('She held the lantern. The far bank was a smear of reeds and one crooked post, and on the post someone had nailed a board with a single word burned into it: '), txt('TURN', 'bold'), txt('.')),
    { type: 'scene_break' },
    para(null, txt('They did not turn.')),
  ],
}

// A chapter with comments (stage 5): two overlapping, one across two
// paragraphs with a reply, one resolved.
const cmk = (id) => ({ type: 'comment', attrs: { id } })
const fordDoc = {
  type: 'doc',
  content: [
    { type: 'heading', attrs: { level: 1 }, content: [txt('The Ford')] },
    para(null, txt('The ford was higher than Oren had promised. '), txt('Mira watched the water climb', cmk(501)), txt(' the cart’s wheels', cmk(501), cmk(502)), txt(' and said nothing, because saying something would make it real.')),
    para(null, txt('“Hold the lantern,” he said. ', cmk(503))),
    para(null, txt('She held the lantern.', cmk(503)), txt(' The far bank was a smear of reeds and one crooked post, and on the post someone had nailed a board with a single word burned into it: '), txt('TURN', 'bold', cmk(504)), txt('.')),
    para(null, txt('They did not turn.')),
  ],
}

const now = () => new Date().toISOString()
const db = {
  volumes: [
    { id: 1, title: 'The Grey Road' },
    { id: 2, title: 'Book Two' },
    { id: 3, title: 'Serial' },
  ],
  chapters: [
    { id: 11, title: 'Milestones', volume_id: 1 },
    { id: 12, title: 'Hollowmere', volume_id: 1 },
    { id: 13, title: 'The Ford', volume_id: 1 },
    { id: 21, title: 'Opening', volume_id: 2 },
    { id: 31, title: 'Episode 1', volume_id: 3 },
  ],
  books: {
    1: { settings: a6, is_set_up: true },
    2: { settings: defaultSettings('a5'), is_set_up: false },
    3: { settings: defaultSettings('online'), is_set_up: true },
  },
  texts: {
    11: { doc: chapterOne, version: 4, updated_at: now() },
    12: { doc: { type: 'doc', content: [para(null, txt('Hollowmere was smaller than its name.'))] }, version: 1, updated_at: now() },
    31: { doc: onlineDoc, version: 2, updated_at: now() },
    13: { doc: fordDoc, version: 3, updated_at: now() },
  },
  comments: [
    { id: 501, chapter_id: 13, parent_id: null, text: 'Too slow here? Cut to the water.', resolved: false, resolved_at: null, created_at: '2026-10-26 20:11:00', updated_at: '2026-10-26 20:11:00' },
    { id: 502, chapter_id: 13, parent_id: null, text: 'Wheels: wooden or iron-rimmed? Check chapter 2.', resolved: false, resolved_at: null, created_at: '2026-10-27 09:02:00', updated_at: '2026-10-27 09:02:00' },
    { id: 503, chapter_id: 13, parent_id: null, text: 'Echo of the lantern in chapter 1.\nKeep both lines short.', resolved: false, resolved_at: null, created_at: '2026-10-27 10:30:00', updated_at: '2026-10-27 10:30:00' },
    { id: 505, chapter_id: 13, parent_id: 503, text: 'Done in the last pass.', resolved: false, resolved_at: null, created_at: '2026-10-27 18:45:00', updated_at: '2026-10-27 18:45:00' },
    { id: 504, chapter_id: 13, parent_id: null, text: 'Burned or painted?', resolved: true, resolved_at: '2026-10-27 19:00:00', created_at: '2026-10-25 08:00:00', updated_at: '2026-10-25 08:00:00' },
  ],
  versions: {
    11: [
      { id: 3, label: 'Before switching from A5 to A6', created_at: '2026-10-20 18:02:11', doc: { type: 'doc', content: chapterOne.content.slice(0, 5) } },
      { id: 2, label: 'Autosave', created_at: '2026-10-19 21:40:00', doc: { type: 'doc', content: chapterOne.content.slice(0, 3) } },
      { id: 1, label: 'First draft', created_at: '2026-10-12 09:15:42', doc: { type: 'doc', content: [chapterOne.content[0], para(null, txt('The lantern swung. Mira counted.'))] } },
    ],
  },
  conflictNext: false,
  puts: [],
}
let nextVersionId = 100
let nextCommentId = 600
const words = (doc) => countDocJSON(doc)
const textOf = (id) => {
  const t = db.texts[id] || { doc: { type: 'doc', content: [{ type: 'paragraph' }] }, version: 0, updated_at: null }
  return { doc: t.doc, version: t.version, words: words(t.doc), updated_at: t.updated_at }
}
const chapterIndex = () => ({
  volumes: db.volumes,
  chapters: db.chapters.map((c, i) => ({ ...c, number: db.chapters.filter((x) => x.volume_id === c.volume_id).indexOf(c) + 1, position: i, event_count: 0, version_count: 0 })),
})

async function fakeApi(route) {
  const req = route.request()
  const url = new URL(req.url())
  const method = req.method()
  const path = url.pathname.replace(`/api/s/${STORY}`, '')
  const body = () => JSON.parse(req.postData() || '{}')
  const json = (data, status = 200) => route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(data) })
  let m
  if (url.pathname === '/api/stories') return json([{ id: STORY, name: 'The Grey Road', icon: '' }])
  if (path === '/chapters' && method === 'GET') return json(chapterIndex())
  if (path === '/chapters' && method === 'POST') {
    const b = body()
    const c = { id: 100 + db.chapters.length, title: b.title, volume_id: b.volume_id ?? null }
    db.chapters.push(c)
    return json(c)
  }
  if ((m = /^\/books\/(\d+)$/.exec(path))) {
    const v = +m[1]
    if (method === 'PUT') {
      const b = body()
      db.books[v] = { settings: b.settings, is_set_up: true }
      if (b.switch_label) {
        for (const c of db.chapters.filter((x) => (x.volume_id ?? 0) === v)) {
          ;(db.versions[c.id] ||= []).unshift({ id: nextVersionId++, label: b.switch_label, created_at: now(), doc: textOf(c.id).doc })
        }
      }
    }
    const b = db.books[v] || { settings: defaultSettings('a5'), is_set_up: false }
    const chapters = db.chapters
      .filter((c) => (c.volume_id ?? 0) === v)
      .map((c, i) => ({ id: c.id, title: c.title, position: i, words: textOf(c.id).words, version: textOf(c.id).version }))
    const defaults = Object.fromEntries(['a4', 'a5', 'a6', 'custom', 'online'].map((k) => [k, defaultSettings(k)]))
    const title = db.volumes.find((x) => x.id === v)?.title || ''
    return json({ volume: v, title, settings: b.settings, chapters, is_set_up: b.is_set_up, words: chapters.reduce((n, c) => n + c.words, 0), page_types: Object.keys(defaults), defaults })
  }
  if ((m = /^\/chapters\/(\d+)\/text$/.exec(path))) {
    const id = +m[1]
    if (method === 'PUT') {
      const b = body()
      db.puts.push(b)
      const cur = textOf(id)
      if (db.conflictNext || b.base_version !== cur.version) {
        db.conflictNext = false
        return json(cur, 409)
      }
      db.texts[id] = { doc: b.doc, version: cur.version + 1, updated_at: now() }
    }
    return json(textOf(id))
  }
  if ((m = /^\/chapters\/(\d+)\/text\/versions$/.exec(path))) {
    const id = +m[1]
    const list = (db.versions[id] ||= [])
    if (method === 'POST') {
      const b = body()
      const v = { id: nextVersionId++, label: b.label || '', created_at: now(), doc: b.doc || textOf(id).doc }
      list.unshift(v)
      return json({ id: v.id, label: v.label, words: words(v.doc), created_at: v.created_at })
    }
    return json(list.map((v) => ({ id: v.id, label: v.label, words: words(v.doc), created_at: v.created_at })))
  }
  if ((m = /^\/chapters\/(\d+)\/text\/versions\/(\d+)(\/restore)?$/.exec(path))) {
    const id = +m[1]
    const v = (db.versions[id] || []).find((x) => x.id === +m[2])
    if (!v) return route.fulfill({ status: 404, body: 'version not found' })
    if (m[3]) {
      db.versions[id].unshift({ id: nextVersionId++, label: 'Before restoring', created_at: now(), doc: textOf(id).doc })
      db.texts[id] = { doc: v.doc, version: textOf(id).version + 1, updated_at: now() }
      return json(textOf(id))
    }
    return json({ id: v.id, label: v.label, words: words(v.doc), created_at: v.created_at, doc: v.doc })
  }
  if ((m = /^\/chapters\/(\d+)\/comments$/.exec(path))) {
    const id = +m[1]
    if (method === 'POST') {
      const b = body()
      const c = { id: nextCommentId++, chapter_id: id, parent_id: b.parent_id ?? null, text: b.text || '', resolved: false, resolved_at: null, created_at: now(), updated_at: now() }
      db.comments.push(c)
      return json(c)
    }
    return json(db.comments.filter((c) => c.chapter_id === id))
  }
  if ((m = /^\/comments\/(\d+)$/.exec(path))) {
    const c = db.comments.find((x) => x.id === +m[1])
    if (!c) return route.fulfill({ status: 404, body: 'comment not found' })
    if (method === 'DELETE') {
      db.comments = db.comments.filter((x) => x.id !== c.id && x.parent_id !== c.id)
      return route.fulfill({ status: 204 })
    }
    const b = body()
    if (b.text != null) c.text = b.text
    if (b.resolved != null) {
      c.resolved = b.resolved
      c.resolved_at = b.resolved ? now() : null
    }
    return json(c)
  }
  if (path === '/stat-system') return json({ defs: [], settings: {} })
  return route.fulfill({ status: 404, contentType: 'text/plain', body: 'not faked' })
}

// ---- Running ----------------------------------------------------------------------
const cleanups = []
async function cleanUp() {
  for (const f of cleanups.reverse()) await f()
}

async function startFrontend() {
  const server = await createServer({ root: frontend, logLevel: 'error', server: { host: '127.0.0.1', port: WEB_PORT, strictPort: true } })
  await server.listen()
  cleanups.push(() => server.close())
}

async function newPage(browser, viewport, errors) {
  const context = await browser.newContext({ viewport, locale: 'en-US', deviceScaleFactor: 1 })
  cleanups.push(() => context.close())
  await context.addInitScript((story) => {
    localStorage.setItem('polaris-story', story)
    localStorage.setItem('polaris-nav-collapsed', '0')
  }, STORY)
  await context.route('**/api/**', fakeApi)
  const page = await context.newPage()
  page.on('pageerror', (e) => errors.push(e.message))
  page.on('console', (m) => m.type() === 'error' && !/404|Failed to load resource/.test(m.text()) && errors.push(m.text()))
  return page
}

const settle = async (page, ms = 500) => {
  await page.evaluate(() => document.fonts.ready)
  await page.waitForTimeout(ms)
}

async function shoot() {
  const browser = await chromium.launch({ channel: option('channel', 'msedge'), headless: true })
  cleanups.push(() => browser.close())
  const errors = []
  const checks = []
  checks.push = (...c) => (c.forEach((x) => console.log('check: ' + x)), Array.prototype.push.apply(checks, c))
  mkdirSync(out, { recursive: true })
  const saved = []
  let page = await newPage(browser, { width, height }, errors)
  const snap = async (name, opts = {}) => {
    const file = join(out, name)
    await page.screenshot({ path: file, ...opts })
    saved.push(file)
  }
  const doc = () => page.evaluate(() => window.__writeView.state.doc.toJSON())

  // 1. A book never set up asks for its page type.
  await page.goto(`${BASE}/write/2`)
  await page.waitForSelector('.page-type-dialog')
  await page.getByRole('radio', { name: /A6/ }).click()
  await settle(page, 300)
  await snap('01_page_type_chooser.png')
  await page.getByRole('radio', { name: /Custom size/ }).click()
  await settle(page, 200)
  await snap('01b_page_type_custom.png')

  // 2. An A6 book: Merriweather 8 pt, justified, first-line indents, a
  // centred Turret Road style, a scene break.
  await page.goto(`${BASE}/write/1?chapter=11`)
  await page.waitForSelector('.write-doc h1')
  await page.waitForFunction(() => document.fonts.check('8pt Merriweather') && document.fonts.check('9pt "Turret Road"'), null, { timeout: 15000 }).catch(() => checks.push('fonts did not load (offline?)'))
  await settle(page, 800)
  await snap('02_a6_book.png')

  // Clicking a word puts the cursor in that word, at this zoom.
  const clickWord = async (word) => {
    const box = await page.evaluate((w) => {
      const walker = document.createTreeWalker(document.querySelector('.write-doc'), NodeFilter.SHOW_TEXT)
      for (let n = walker.nextNode(); n; n = walker.nextNode()) {
        const i = n.data.indexOf(w)
        if (i < 0) continue
        n.parentElement.scrollIntoView({ block: 'center' })
        const r = document.createRange()
        r.setStart(n, i + 1)
        r.setEnd(n, i + w.length - 1)
        const b = r.getBoundingClientRect()
        return { x: b.left + b.width / 2, y: b.top + b.height / 2 }
      }
      return null
    }, word)
    await page.mouse.click(box.x, box.y)
    await page.waitForTimeout(150)
    return page.evaluate(() => {
      const v = window.__writeView
      const { $from } = v.state.selection
      const text = $from.parent.textContent
      let a = $from.parentOffset
      let b = a
      while (a > 0 && /\w/.test(text[a - 1])) a--
      while (b < text.length && /\w/.test(text[b])) b++
      return text.slice(a, b)
    })
  }
  const zoomNow = await page.evaluate(() => getComputedStyle(document.querySelector('.sheet')).zoom)
  checks.push(`zoom ${zoomNow}: click on "Hollowmere" → "${await clickWord('Hollowmere')}"`)
  await page.selectOption('.zoom select', '2')
  await settle(page, 300)
  checks.push(`zoom 2: click on "milestones" → "${await clickWord('milestones')}"`)
  await page.selectOption('.zoom select', 'auto')
  await settle(page, 300)

  // Typing helpers: smart quotes, -- and ..., and Ctrl+Z right after.
  await clickWord('forget')
  await page.keyboard.press('End')
  await page.keyboard.press('Enter')
  await page.keyboard.type('"Wait--what..." she said. \'Fine\'')
  let typed = await page.evaluate(() => window.__writeView.state.selection.$from.parent.textContent)
  assert.equal(typed, '“Wait—what…” she said. ‘Fine’')
  await page.keyboard.type(' --')
  await page.keyboard.press('Control+z')
  typed = await page.evaluate(() => window.__writeView.state.selection.$from.parent.textContent)
  assert.equal(typed, '“Wait—what…” she said. ‘Fine’ --', 'Ctrl+Z right after undoes the em dash')
  checks.push('typing helpers and their undo: ok')

  // Autosave: sent about 1.5 s after typing stops, on the stored version.
  await page.waitForTimeout(2300)
  const last = db.puts.at(-1)
  assert.ok(last && last.base_version === 4, 'autosaved on version 4')
  assert.equal(await page.locator('.save-state').textContent(), 'Saved')
  checks.push(`autosave: ${db.puts.length} save(s), now version ${db.texts[11].version}`)

  // Pasting from Word keeps what the schema knows.
  await page.keyboard.press('Enter')
  await page.evaluate(() => {
    const html = `<html><body><!--StartFragment--><p class=MsoNormal align=center style='text-align:center;text-indent:0cm;margin-bottom:6.0pt;line-height:150%'><span style='font-family:"Turret Road";font-size:11.0pt;color:#1F4E79'>Pasted </span><b><span style='font-size:11.0pt'>from Word</span></b><span style='font-variant:small-caps'> small caps</span><span style='mso-tab-count:1'>   </span><span style='background:yellow'>marked</span><sup>2</sup></p><p class=MsoNormal style='margin-left:36pt;text-indent:-18pt'><i>Hanging</i> indent</p><!--EndFragment--></body></html>`
    const dt = new DataTransfer()
    dt.setData('text/html', html)
    dt.setData('text/plain', 'Pasted from Word small caps marked2\nHanging indent')
    window.__writeView.dom.dispatchEvent(new ClipboardEvent('paste', { clipboardData: dt, bubbles: true, cancelable: true }))
  })
  await page.waitForTimeout(200)
  const pasted = (await doc()).content.filter((n) => /Pasted|Hanging/.test(JSON.stringify(n)))
  const p1 = pasted[0]
  assert.equal(p1.attrs.align, 'center')
  assert.equal(p1.attrs.indentFirst, 0)
  assert.equal(p1.attrs.spaceAfter, 6)
  assert.equal(p1.attrs.lineHeight, 1.5)
  const markTypes = (node, word) => node.content.find((c) => c.text?.includes(word))?.marks?.map((m) => m.type + (m.attrs ? JSON.stringify(m.attrs) : '')) || []
  const pastedMarks = markTypes(p1, 'Pasted').join(' ')
  assert.ok(/font\{"family":"Turret Road"/.test(pastedMarks) && /size\{"pt":11/.test(pastedMarks) && /color\{"color":"#1f4e79"/.test(pastedMarks), 'font, size, colour: ' + pastedMarks)
  assert.ok(markTypes(p1, 'from Word').some((m) => m.startsWith('bold')))
  assert.ok(markTypes(p1, 'small caps').some((m) => m.startsWith('smallcaps')))
  assert.ok(markTypes(p1, 'marked').some((m) => m.startsWith('highlight')))
  assert.ok(p1.content.some((c) => c.type === 'tab'), 'a Word tab')
  assert.ok(markTypes(p1, '2').some((m) => m.startsWith('superscript')))
  assert.equal(pasted[1].attrs.indentLeft, 36)
  assert.equal(pasted[1].attrs.indentFirst, -18)
  checks.push('paste from Word: ok')
  await settle(page, 300)
  await page.evaluate(() => document.querySelector('.paper-area').scrollTo(0, 99999))
  await snap('02b_a6_after_typing_and_paste.png')
  await page.evaluate(() => document.querySelector('.paper-area').scrollTo(0, 0))

  // 3. The styles panel, on the Signpost paragraph.
  await clickWord('MILESTONES')
  await page.getByRole('button', { name: 'Styles', exact: true }).click()
  await page.waitForSelector('.styles-panel')
  await settle(page, 300)
  await snap('03_styles_panel.png')
  await page.getByRole('button', { name: 'Styles', exact: true }).click()

  // 4. Find and replace.
  await clickWord('lantern')
  await page.keyboard.press('Control+h')
  await page.waitForSelector('.find-panel')
  await page.locator('.find-panel input[type=search]').fill('the')
  await page.locator('.find-panel input[type=text]').fill('THE')
  await settle(page, 300)
  await snap('04_find_replace.png')
  const count = await page.locator('.find-panel .count').textContent()
  await page.getByRole('checkbox', { name: 'Whole word' }).check()
  await page.getByRole('checkbox', { name: 'Match case' }).check()
  const strict = await page.locator('.find-panel .count').textContent()
  checks.push(`find "the": ${count}; whole word + match case: ${strict}`)
  await page.getByRole('button', { name: 'Replace all' }).click()
  const after = (await doc()).content.map((n) => JSON.stringify(n)).join('')
  assert.ok(!/\bthe\b/.test(after.replace(/"type":"[a-z_]+"/g, '')), 'every lower-case "the" replaced')
  await page.keyboard.press('Escape')
  await page.waitForTimeout(100)
  checks.push('after Escape the focus is in ' + (await page.evaluate(() => document.activeElement.className)))
  await page.keyboard.press('Control+z')
  await page.waitForTimeout(100)
  checks.push('replace all, then one Ctrl+Z: ' + ((await doc()).content.some((n) => JSON.stringify(n).includes(' the ')) ? 'undone' : 'NOT undone'))

  // 5. Saved versions, one opened.
  await page.getByRole('button', { name: 'Saved versions', exact: true }).click()
  await page.waitForSelector('.versions-panel .version-row')
  await page.locator('.versions-panel .version-row').nth(2).click()
  await page.waitForSelector('.versions-panel .preview .write-doc p')
  await settle(page, 300)
  await snap('05_versions_panel.png')
  await page.getByRole('button', { name: 'Saved versions', exact: true }).click()

  // 6. A conflict: the next save finds the chapter changed elsewhere.
  db.conflictNext = true
  db.texts[11] = { doc: { type: 'doc', content: [...chapterOne.content.slice(0, 4), para(null, txt('(Changed in another tab: Mira never found the map.)', 'italic'))] }, version: db.texts[11].version + 1, updated_at: now() }
  await clickWord('Oren')
  await page.keyboard.type(' (edited here)')
  await page.waitForSelector('.conflict', { timeout: 6000 })
  await settle(page, 300)
  await snap('06_conflict.png')
  await page.getByRole('button', { name: 'Keep mine' }).click()
  await page.waitForTimeout(2500)
  assert.equal(await page.locator('.save-state').textContent(), 'Saved')
  checks.push(`conflict kept mine: stored text has "(edited here)": ${JSON.stringify(db.texts[11].doc).includes('(edited here)')}; versions: ${db.versions[11].map((v) => v.label).slice(0, 2).join(' / ')}`)

  // Dark paper.
  await page.getByRole('button', { name: 'Dark paper' }).click()
  await settle(page, 200)
  await snap('07_dark_paper.png')
  await page.getByRole('button', { name: 'Light paper' }).click()

  // 7b. Comments (stage 5): the panel beside the page, threads in the order
  // of the text; picking one selects its text.
  await page.goto(`${BASE}/write/1?chapter=13`)
  await page.waitForSelector('.write-doc h1')
  await page.getByRole('button', { name: 'Comments', exact: true }).click()
  await page.waitForSelector('.comments-panel .thread')
  const threads = await page.locator('.comments-panel .thread > .entry .cm-text').allTextContents()
  assert.equal(threads.length, 3, 'the resolved comment is hidden: ' + threads.join(' / '))
  assert.ok(threads[0].startsWith('Too slow') && threads[1].startsWith('Wheels'), 'in the order of the text')
  await page.locator('.comments-panel .thread').nth(1).locator('.quote').click()
  const picked = await page.evaluate(() => {
    const { from, to } = window.__writeView.state.selection
    return window.__writeView.state.doc.textBetween(from, to)
  })
  assert.equal(picked, ' the cart’s wheels')
  await settle(page, 300)
  await snap('07b_comments_panel.png')
  // Clicking commented text makes its comment the active one.
  await clickWord('Hold')
  await page.waitForTimeout(150)
  assert.match(await page.locator('.comments-panel .thread.is-active .cm-text').first().textContent(), /Echo of the lantern/)
  // Ctrl+Alt+M with nothing selected comments the word at the cursor.
  await clickWord('reeds')
  await page.keyboard.press('Control+Alt+m')
  await page.waitForSelector('.comments-panel .thread textarea')
  await page.keyboard.type('Which reeds? Bulrushes?')
  await page.keyboard.press('Control+Enter')
  await page.waitForTimeout(300)
  const made = db.comments.find((c) => c.text === 'Which reeds? Bulrushes?')
  assert.ok(made, 'the new comment is saved')
  const marked = await page.evaluate((id) => {
    let out = ''
    window.__writeView.state.doc.descendants((n) => {
      if (n.isText && n.marks.some((m) => m.type.name === 'comment' && m.attrs.id === id)) out += n.text
    })
    return out
  }, made.id)
  assert.equal(marked, 'reeds')
  // Resolving takes the highlight away.
  await page.locator('.comments-panel .thread').first().getByRole('button', { name: 'Resolve', exact: true }).click()
  await page.waitForTimeout(300)
  const css = await page.evaluate(() => document.querySelector('style[data-write-comments]').textContent)
  assert.ok(!css.includes('"501"') && css.includes(`"${made.id}"`), 'resolved comments are not highlighted')
  await page.waitForTimeout(2000) // the mark goes in with the chapter's autosave
  assert.ok(JSON.stringify(db.texts[13].doc).includes(`"id":${made.id}`), "the mark is saved with the chapter's text")
  checks.push(`comments: ${db.comments.length} rows; new one on "${marked}"; 501 resolved: ${db.comments.find((c) => c.id === 501).resolved}`)
  await page.getByRole('checkbox', { name: /Show resolved/ }).check()
  await settle(page, 300)
  await snap('07c_comments_new_and_resolved.png')
  await page.getByRole('button', { name: 'Dark paper' }).click()
  await settle(page, 200)
  await snap('07d_comments_dark_paper.png')
  await page.getByRole('button', { name: 'Light paper' }).click()

  // 8. An Online Post book.
  await page.goto(`${BASE}/write/3`)
  await page.waitForSelector('.write-doc h1')
  await settle(page, 800)
  await snap('08_online_post.png')

  // Switching page type warns and labels the versions.
  await page.getByRole('button', { name: 'Book and page', exact: true }).click()
  await page.getByRole('button', { name: 'Change…' }).click()
  await page.getByRole('radio', { name: /A5/ }).click()
  await settle(page, 200)
  await snap('08b_switch_page_type.png')
  await page.getByRole('button', { name: 'Switch page type' }).click()
  await page.waitForTimeout(500)
  checks.push(`switch online → A5: book is ${db.books[3].settings.page_type}; version label "${db.versions[31]?.[0]?.label}"`)

  // 9. A phone.
  page = await newPage(browser, { width: 390, height: 844 }, errors)
  await page.goto(`${BASE}/write/1?chapter=11`)
  await page.waitForSelector('.write-doc h1')
  await settle(page, 800)
  await snap('09_phone.png')
  await page.getByRole('button', { name: 'More tools' }).click()
  await settle(page, 200)
  await snap('09b_phone_toolbar_open.png')
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth)
  checks.push(`phone: page wider than the screen by ${overflow}px`)

  return { saved, errors, checks }
}

try {
  await startFrontend()
  const { saved, errors, checks } = await shoot()
  for (const f of saved) console.log(f)
  if (errors.length) console.log('page errors:\n' + errors.join('\n'))
} catch (err) {
  console.error(err)
  process.exitCode = 1
} finally {
  await cleanUp()
}
