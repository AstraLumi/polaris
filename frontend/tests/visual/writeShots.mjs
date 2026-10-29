// Screenshots of the writing page (/write/:volume) without a screen: the
// page type chooser, an A6 book, the styles panel, find and replace, the
// saved versions, a save conflict, an Online Post book and a phone. The
// server is faked in memory with page.route (no backend, no real data), so
// it runs while the server side of the editor is still being built. Also
// checks a few things the screenshots can't show: clicking a word puts the
// cursor in it at any zoom, pasting Word HTML keeps its formatting, typing
// helpers and autosave. Tables (stage 5): a stat block and a long
// inventory in the editor (the table box, inserting one from the toolbar,
// Tab through the cells, a table pasted from Word) and in the page view
// (rows breaking across pages, the header row repeated).
//
//   node frontend/tests/visual/writeShots.mjs [--out <dir>] [--size 1400x900] [--only spell|tables|footnotes|boxes|wrap|import]
//
// --only spell: just the spellchecker (underlines, the menu on a word, Add
// to dictionary, Ignore, typing, the spelling panel, dark paper, off).
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
const WRAP_TEXT = 'The road bent north after the ford, and nobody spoke for a long while. Rain came in thin lines across the hills, the cart wheels sang, and Mira counted the lanterns one more time before the light went.'
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

// A LitRPG stat block and an inventory long enough to run over pages.
const cellP = (attrs, ...content) => ({ type: 'table_cell', ...(attrs ? { attrs } : {}), content: [para({ indentFirst: 0 }, ...content)] })
const headP = (attrs, ...content) => ({ type: 'table_header', ...(attrs ? { attrs } : {}), content: [para({ indentFirst: 0 }, ...content)] })
const tRow = (...cells) => ({ type: 'table_row', content: cells })
const statBlock = {
  type: 'table',
  attrs: { borders: 'outer', align: 'center', width: 'full' },
  content: [
    tRow(headP({ colspan: 4, background: '#1f3864' }, txt('STATUS', 'bold', { type: 'color', attrs: { color: '#ffffff' } }))),
    tRow(cellP({ colwidth: [60] }, txt('Name', 'bold')), cellP({ colspan: 3, colwidth: [50, 60, 50] }, txt('Mira Vey'))),
    tRow(cellP({ colwidth: [60], background: '#deeaf6' }, txt('STR', 'smallcaps')), cellP({ colwidth: [50] }, txt('12')), cellP({ colwidth: [60], background: '#deeaf6' }, txt('AGI', 'smallcaps')), cellP({ colwidth: [50] }, txt('17'))),
    tRow(cellP({ colwidth: [60], background: '#deeaf6' }, txt('INT', 'smallcaps')), cellP({ colwidth: [50] }, txt('9')), cellP({ colwidth: [60], background: '#deeaf6' }, txt('LCK', 'smallcaps')), cellP({ colwidth: [50] }, txt('3'))),
    tRow(cellP({ colwidth: [60], valign: 'middle' }, txt('Skills', 'bold')), { type: 'table_cell', attrs: { colspan: 3, colwidth: [50, 60, 50] }, content: [{ type: 'bullet_list', content: [{ type: 'list_item', content: [para(null, txt('Lantern-reading (Lv 2)'))] }, { type: 'list_item', content: [para(null, txt('Road sense (Lv 1)'))] }] }] }),
  ],
}
const inventory = {
  type: 'table',
  attrs: { borders: 'all' },
  content: [
    tRow(headP({ colwidth: [30] }, txt('#', 'bold')), headP({ colwidth: [140] }, txt('Item', 'bold')), headP({ colwidth: [50] }, txt('Weight', 'bold'))),
    ...Array.from({ length: 34 }, (_, i) =>
      tRow(cellP({ colwidth: [30] }, txt(String(i + 1))), cellP({ colwidth: [140] }, txt(['Tallow candle', 'Waxed map', 'Copper ring', 'Ford-stone', 'Dried apples', 'Rope (10 m)'][i % 6])), cellP({ colwidth: [50] }, txt(`${(i % 5) + 1}.${i % 10} kg`))),
    ),
  ],
}
const fordDoc = {
  type: 'doc',
  content: [
    { type: 'heading', attrs: { level: 1 }, content: [txt('The Ford')] },
    para(null, txt('The window opened the moment her boots touched the water, blue and patient as a held breath.')),
    statBlock,
    para(null, txt('She closed it with a blink. The cart creaked on. Later, by the fire, she counted what they still carried.')),
    inventory,
    para(null, txt('It was not much, but it was theirs.')),
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
const crossingDoc = {
  type: 'doc',
  content: [
    { type: 'heading', attrs: { level: 1 }, content: [txt('The Crossing')] },
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
    { id: 14, title: 'Typos', volume_id: 1 },
    { id: 15, title: 'The Crossing', volume_id: 1 },
    { id: 16, title: 'Notes', volume_id: 1 },
    { id: 17, title: 'Boxes', volume_id: 1 },
    { id: 18, title: 'Wrapping', volume_id: 1 },
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
    14: {
      doc: {
        type: 'doc',
        content: [
          { type: 'heading', attrs: { level: 1 }, content: [txt('Typos')] },
          para(null, txt('Mira walkd past '), txt('Hollowmer', { type: 'story_link', attrs: { type: 'location', id: 7 } }), txt(' with Zarathul, carrying Oren’s lantren. The Grey Ordr waited by the ford, and '), txt('nobdy', 'italic'), txt(' spoke.')),
          para(null, txt('LEVEL UP. See https://example.com/wrng-paht for the 3rd map, e.g. the old one.')),
          { type: 'dialogue', attrs: { character: 1, speaker: 'Mira', manner: 'quitely' }, content: [txt('We shuold turn back.')] },
        ],
      },
      version: 1,
      updated_at: now(),
    },
    16: {
      doc: {
        type: 'doc',
        content: [
          { type: 'heading', attrs: { level: 1 }, content: [txt('Notes')] },
          para(null, txt('The ford was lower than the map said'), { type: 'footnote', attrs: { kind: 'footnote' }, content: [para(null, txt('Marked at one and a half feet on the old survey.'))] }, txt(', and Oren cursed it anyway.')),
          para(null, txt('Nobody had crossed since the flood'), { type: 'footnote', attrs: { kind: 'endnote' }, content: [para(null, txt('The flood of 1051, see the almanac.'))] }, txt('.')),
        ],
      },
      version: 1,
      updated_at: now(),
    },
    17: {
      doc: {
        type: 'doc',
        content: [
          { type: 'heading', attrs: { level: 1 }, content: [txt('Boxes')] },
          para({ dropCap: 3 }, txt('The road bent north after the ford, and nobody spoke for a long while. Rain came in thin lines across the hills, and the cart wheels sang.')),
          para({ boxWidth: 1.5, boxColor: '#3366cc', shade: '#e8f0ff' }, txt('[System] Quest updated: Cross the ford.')),
          para({ boxWidth: 1.5, boxColor: '#3366cc', shade: '#e8f0ff' }, txt('Reward: 120 xp, one wet pair of boots.')),
          para(null, txt('Mira read it twice, then closed her eyes.')),
        ],
      },
      version: 1,
      updated_at: now(),
    },
    18: {
      doc: {
        type: 'doc',
        content: [
          { type: 'heading', attrs: { level: 1 }, content: [txt('Wrapping')] },
          { type: 'image', attrs: { src: '/uploads/editor/wrap.svg', alt: 'A left picture', width: 110, align: 'left', wrap: 'left' } },
          ...[1, 2, 3].map((i) => para(null, txt(WRAP_TEXT + ' ' + i))),
          { type: 'image', attrs: { src: '/uploads/editor/wrap.svg', alt: 'A right picture', width: 110, align: 'right', wrap: 'right' } },
          ...[4, 5, 6].map((i) => para(null, txt(WRAP_TEXT + ' ' + i))),
          { type: 'image', attrs: { src: '/uploads/editor/wrap.svg', alt: 'At the end', width: 90, align: 'left', wrap: 'left' } },
        ],
      },
      version: 1,
      updated_at: now(),
    },
    13: { doc: fordDoc, version: 1, updated_at: now() },
    31: { doc: onlineDoc, version: 2, updated_at: now() },
    15: { doc: crossingDoc, version: 3, updated_at: now() },
  },
  comments: [
    { id: 501, chapter_id: 15, parent_id: null, text: 'Too slow here? Cut to the water.', resolved: false, resolved_at: null, created_at: '2026-10-26 20:11:00', updated_at: '2026-10-26 20:11:00' },
    { id: 502, chapter_id: 15, parent_id: null, text: 'Wheels: wooden or iron-rimmed? Check chapter 2.', resolved: false, resolved_at: null, created_at: '2026-10-27 09:02:00', updated_at: '2026-10-27 09:02:00' },
    { id: 503, chapter_id: 15, parent_id: null, text: 'Echo of the lantern in chapter 1.\nKeep both lines short.', resolved: false, resolved_at: null, created_at: '2026-10-27 10:30:00', updated_at: '2026-10-27 10:30:00' },
    { id: 505, chapter_id: 15, parent_id: 503, text: 'Done in the last pass.', resolved: false, resolved_at: null, created_at: '2026-10-27 18:45:00', updated_at: '2026-10-27 18:45:00' },
    { id: 504, chapter_id: 15, parent_id: null, text: 'Burned or painted?', resolved: true, resolved_at: '2026-10-27 19:00:00', created_at: '2026-10-25 08:00:00', updated_at: '2026-10-25 08:00:00' },
  ],
  versions: {
    11: [
      { id: 3, label: 'Before switching from A5 to A6', created_at: '2026-10-20 18:02:11', doc: { type: 'doc', content: chapterOne.content.slice(0, 5) } },
      { id: 2, label: 'Autosave', created_at: '2026-10-19 21:40:00', doc: { type: 'doc', content: chapterOne.content.slice(0, 3) } },
      { id: 1, label: 'First draft', created_at: '2026-10-12 09:15:42', doc: { type: 'doc', content: [chapterOne.content[0], para(null, txt('The lantern swung. Mira counted.'))] } },
    ],
  },
  conflictNext: false,
  wiki: [
    { type: 'character', id: 1, name: 'Mira Vey', nicknames: ['Mimi'], aliases: [] },
    { type: 'character', id: 2, name: 'Oren', nicknames: [], aliases: [] },
    { type: 'location', id: 7, name: 'Hollowmere' },
    { type: 'faction', id: 3, name: 'The Grey Order' },
  ],
  spelling: ['Turret'],
  puts: [],
}
let nextVersionId = 100
let nextCommentId = 600
const words = (doc) => countDocJSON(doc)
const textOf = (id) => {
  const t = db.texts[id] || { doc: { type: 'doc', content: [{ type: 'paragraph' }] }, version: 0, updated_at: null }
  return { doc: t.doc, version: t.version, words: words(t.doc), updated_at: t.updated_at }
}
// As the server: front matter (the book's chapter options) isn't numbered.
const isFront = (c) => !!db.books[c.volume_id ?? 0]?.settings?.chapter_options?.[c.id]?.front_matter
const chapterIndex = () => ({
  volumes: db.volumes,
  chapters: db.chapters.map((c, i) => ({ ...c, front_matter: isFront(c), number: isFront(c) ? 0 : db.chapters.filter((x) => x.volume_id === c.volume_id && !isFront(x)).indexOf(c) + 1, position: i, event_count: 0, version_count: 0 })),
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
  if ((m = /^\/books\/(\d+)\/import-docx$/.exec(path))) {
    // As the server does: the text before the first Heading 1 is front
    // matter, marked in the book's chapter options too.
    const v = +m[1]
    const para = (t) => ({ type: 'doc', content: [{ type: 'paragraph', content: [{ type: 'text', text: t }] }] })
    const made = [['Front matter', 'For everyone who waited at the ford.', true], ['1. RECODED', 'The road bent north.', false], ['2. WARM CHAINS', 'Rain came in thin lines.', false]].map(([title, text, front]) => {
      const c = { id: 100 + db.chapters.length, title, volume_id: v || null }
      db.chapters.push(c)
      db.texts[c.id] = { doc: para(text), version: 1, updated_at: now() }
      return { id: c.id, title, words: text.split(' ').length, front_matter: front }
    })
    const b = (db.books[v] ||= { settings: defaultSettings('a5'), is_set_up: false })
    b.settings = { ...b.settings, chapter_options: { ...(b.settings.chapter_options || {}), [made[0].id]: { front_matter: true } } }
    return json({ chapters: made, left_out: { headers_footers: 2 }, fonts: [] })
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
  if ((m = /^\/books\/(\d+)\/docs$/.exec(path))) {
    const v = +m[1]
    return json(db.chapters.filter((c) => (c.volume_id ?? 0) === v).map((c) => ({ id: c.id, title: c.title, doc: textOf(c.id).doc })))
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
  if (path === '/wiki') return json(db.wiki)
  if (path === '/spelling/words') {
    if (method === 'POST') {
      const b = body()
      db.spelling = [...new Set([...db.spelling, ...(b.add || [])])].filter((w) => !(b.remove || []).includes(w)).sort()
    }
    return json({ words: db.spelling })
  }
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
  // A speech synthesis that says nothing and never finishes a sentence, so
  // read aloud can be looked at (the real voices can't be heard headless).
  await context.addInitScript(() => {
    const spoken = (window.__spoken = [])
    const fake = {
      getVoices: () => [
        { name: 'Fake English', lang: 'en-US', voiceURI: 'fake-en', default: true },
        { name: 'Fake Portuguese', lang: 'pt-BR', voiceURI: 'fake-pt' },
      ],
      speak: (u) => spoken.push(u.text),
      cancel: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
    }
    Object.defineProperty(window, 'speechSynthesis', { value: fake, configurable: true })
  })
  await context.route('**/api/**', fakeApi)
  await context.route('**/uploads/**', (route) => route.fulfill({ contentType: 'image/svg+xml', body: '<svg xmlns="http://www.w3.org/2000/svg" width="220" height="160"><rect width="220" height="160" fill="#d9a441"/><circle cx="110" cy="80" r="50" fill="#3366cc"/></svg>' }))
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

  if (option('only', '') === 'footnotes') {
    await footnoteShots(page, snap, checks)
    return { saved, errors, checks }
  }
  if (option('only', '') === 'import') {
    await importShots(page, snap, checks)
    return { saved, errors, checks }
  }
  if (option('only', '') === 'wrap') {
    await wrapShots(page, snap, checks)
    return { saved, errors, checks }
  }
  if (option('only', '') === 'boxes') {
    await boxShots(page, snap, checks)
    return { saved, errors, checks }
  }
  if (option('only', '') === 'tables') {
    await tableShots(page, snap, checks)
    return { saved, errors, checks }
  }

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

  // 5b. Read aloud: Ctrl+Alt+Space from the caret highlights the sentence and
  // shows the control; a click in the text stops it.
  await clickWord('lantern')
  await page.keyboard.press('Control+Alt+Space')
  await page.waitForSelector('.read-bar')
  await page.waitForSelector('.write-doc .read-now')
  await settle(page, 300)
  await snap('05b_read_aloud.png')
  const sentence = await page.locator('.write-doc .read-now').first().textContent()
  assert.ok((await page.evaluate(() => window.__spoken.length)) >= 1, 'something was said: ' + (await page.evaluate(() => JSON.stringify([window.__spoken, String(window.speechSynthesis.speak)]))))
  await page.getByRole('button', { name: 'Next sentence' }).click()
  await page.waitForTimeout(100)
  assert.notEqual(await page.locator('.write-doc .read-now').first().textContent(), sentence, 'next sentence moves the highlight')
  await page.keyboard.press('Escape')
  await page.waitForTimeout(100)
  assert.equal(await page.locator('.read-bar').count(), 0, 'Esc stops reading')
  await clickWord('lantern')
  await page.keyboard.press('Control+Alt+Space')
  await page.waitForSelector('.read-bar')
  await clickWord('lantern')
  await page.waitForTimeout(100)
  assert.equal(await page.locator('.read-bar').count(), 0, 'a click in the text stops reading')
  checks.push('read aloud: highlight "' + sentence.slice(0, 40) + '", next, Esc and click stop: ok')

  // 6. A conflict: the next save finds the same paragraph changed elsewhere
  // (edits to different paragraphs are merged without asking).
  db.conflictNext = true
  db.texts[11] = { doc: { type: 'doc', content: chapterOne.content.map((blk) => (JSON.stringify(blk).includes('said Oren') ? para(null, txt('(Changed in another tab: Oren never spoke.)', 'italic')) : blk)) }, version: db.texts[11].version + 1, updated_at: now() }
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
  await page.goto(`${BASE}/write/1?chapter=15`)
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
  assert.ok(JSON.stringify(db.texts[15].doc).includes(`"id":${made.id}`), "the mark is saved with the chapter's text")
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

  page = await newPage(browser, { width, height }, errors)
  await tableShots(page, snap, checks)
  return { saved, errors, checks }
}

// The spellchecker on the "Typos" chapter (English, the book's language).
async function spellShots() {
  const browser = await chromium.launch({ channel: option('channel', 'msedge'), headless: true })
  cleanups.push(() => browser.close())
  const errors = []
  const checks = []
  checks.push = (...c) => (c.forEach((x) => console.log('check: ' + x)), Array.prototype.push.apply(checks, c))
  mkdirSync(out, { recursive: true })
  const saved = []
  const page = await newPage(browser, { width, height }, errors)
  const snap = async (name) => {
    const file = join(out, name)
    await page.screenshot({ path: file })
    saved.push(file)
  }
  const underlined = () => page.evaluate(() => [...document.querySelectorAll('.write-doc .spell-wrong')].map((e) => e.textContent))
  const rightClick = async (word) => {
    await page.locator('.write-doc .spell-wrong', { hasText: word }).first().click({ button: 'right' })
    await page.waitForSelector('.spell-menu')
    await page.waitForFunction(() => !document.querySelector('.spell-menu .spell-note') || /No suggestions/.test(document.querySelector('.spell-menu').textContent))
  }

  const t0 = Date.now()
  await page.goto(`${BASE}/write/1?chapter=14`)
  await page.waitForSelector('.write-doc .spell-wrong', { timeout: 20000 })
  await settle(page, 600)
  checks.push(`dictionary loaded and the chapter checked ${Date.now() - t0} ms after opening the page`)
  const first = await underlined()
  checks.push('underlined: ' + first.join(', '))
  assert.deepEqual([...first].sort(), ['Ordr', 'Zarathul', 'lantren', 'nobdy', 'shuold', 'walkd'], 'names, links, capitals, addresses, numbers and the speaker and manner are not underlined')
  assert.equal(await page.evaluate(() => document.querySelector('.write-doc').getAttribute('spellcheck')), 'false', "the browser's own spellcheck is off")
  checks.push('status: ' + (await page.locator('.spell-status').textContent()))
  await snap('20_spell_underlines.png')

  await rightClick('lantren')
  const sugs = await page.locator('.spell-menu .spell-sug').allTextContents()
  checks.push('suggestions for "lantren": ' + sugs.join(', '))
  assert.equal(sugs[0], 'lantern')
  await snap('21_spell_menu.png')
  await page.locator('.spell-menu .spell-sug').first().click()
  await settle(page, 600)
  assert.ok(JSON.stringify(await page.evaluate(() => window.__writeView.state.doc.toJSON())).includes('Oren’s lantern.'), 'the suggestion replaced the word')

  await rightClick('Zarathul')
  await page.getByRole('menuitem', { name: 'Add to dictionary' }).click()
  await page.waitForFunction(() => ![...document.querySelectorAll('.write-doc .spell-wrong')].some((e) => e.textContent === 'Zarathul'), null, { timeout: 5000 })
  assert.ok(db.spelling.includes('Zarathul'), 'stored in the story')
  checks.push("Add to dictionary: stored; the story's words: " + db.spelling.join(', '))

  await rightClick('walkd')
  await page.getByRole('menuitem', { name: 'Ignore' }).click()
  await page.waitForFunction(() => ![...document.querySelectorAll('.write-doc .spell-wrong')].some((e) => e.textContent === 'walkd'), null, { timeout: 5000 })
  assert.ok(!db.spelling.includes('walkd'), 'Ignore is not stored')

  // Typing a wrong word: no underline while the caret is in it, one after.
  await page.locator('.write-doc h1').click()
  await page.keyboard.press('End')
  await page.keyboard.type(' Agian')
  await page.waitForTimeout(900)
  const typing = await underlined()
  await page.keyboard.type(' ')
  await page.waitForTimeout(900)
  const after = await underlined()
  checks.push(`typed "Agian": underlined while typing ${typing.includes('Agian')}, after a space ${after.includes('Agian')}`)
  assert.ok(!typing.includes('Agian') && after.includes('Agian'))

  await page.locator('.spell-status').click()
  await page.waitForSelector('.spell-panel')
  await settle(page, 300)
  await snap('22_spell_panel.png')

  await page.getByRole('button', { name: 'Dark paper' }).click()
  await settle(page, 300)
  await snap('23_spell_dark_paper.png')
  await page.getByRole('button', { name: 'Light paper' }).click()

  // Off: no underlines, the browser's own spellcheck back; on again.
  await page.getByRole('checkbox', { name: 'Check spelling as I write' }).uncheck()
  await page.waitForTimeout(300)
  assert.equal((await underlined()).length, 0)
  assert.equal(await page.evaluate(() => document.querySelector('.write-doc').getAttribute('spellcheck')), 'true')
  await page.getByRole('checkbox', { name: 'Check spelling as I write' }).check()
  await page.waitForSelector('.write-doc .spell-wrong', { timeout: 10000 })
  checks.push('off and on again: ok')

  // Another dictionary (it sets the book's language).
  const t1 = Date.now()
  await page.getByRole('combobox', { name: /Dictionary/ }).selectOption('pt_BR')
  await page.waitForFunction(() => /Spelling: Português/.test(document.querySelector('.spell-status')?.textContent || ''), null, { timeout: 30000 })
  checks.push(`pt_BR loaded in ${Date.now() - t1} ms; book language now ${db.books[1]?.settings?.meta?.language ?? '(not saved yet)'}`)
  return { saved, errors, checks }
}

// Footnotes and endnotes (stage 5): a note open in the editor, the page view.
async function footnoteShots(page, snap, checks) {
  await page.goto(`${BASE}/write/1?chapter=16`)
  await page.waitForSelector('.write-doc .pm-fn-ref')
  await settle(page, 800)
  assert.equal(await page.locator('.write-doc .pm-fn-ref').count(), 2, 'two note marks')
  await page.locator('.write-doc .pm-fn-ref').first().click()
  await page.waitForSelector('.pm-fn-pop')
  await settle(page, 300)
  await snap('11a_footnote_open.png')
  const popText = await page.locator('.pm-fn-pop').first().textContent()
  assert.ok(/Marked at one and a half/.test(popText), 'the note text is in its pop-up: ' + popText)
  checks.push('footnote opens in place with its text')
  await page.keyboard.press('Escape')
  await page.goto(`${BASE}/write/1/pages`)
  await page.waitForSelector('.pv-pages:not(.is-hidden) .pagedjs_page', { timeout: 60000 })
  await settle(page, 600)
  const idx = await page.evaluate(() => [...document.querySelectorAll('.pagedjs_page')].findIndex((p) => /ford was lower/.test(p.textContent)))
  assert.ok(idx >= 0, 'the Notes chapter is in the page view')
  await page.evaluate((i) => document.querySelectorAll('.pagedjs_page')[i].scrollIntoView({ block: 'start' }), idx)
  await settle(page, 400)
  const foot = await page.evaluate((i) => document.querySelectorAll('.pagedjs_page')[i].textContent.includes('Marked at one and a half'), idx)
  assert.ok(foot, 'the footnote text is on the page that holds its mark')
  checks.push('page view: footnote at the page foot')
  await snap('11b_footnote_page_view.png')
}

// Boxes and drop caps (stage 5): a two-paragraph system window and a drop
// cap, in the editor and in the page view.
async function boxShots(page, snap, checks) {
  await page.goto(`${BASE}/write/1?chapter=17`)
  await page.waitForSelector('.write-doc [data-box]')
  await settle(page, 800)
  const boxes = await page.evaluate(() => [...document.querySelectorAll('.write-doc [data-box]')].map((p) => {
    const c = getComputedStyle(p)
    return { top: c.borderTopWidth, bottom: c.borderBottomWidth, mt: c.marginTop }
  }))
  assert.equal(boxes.length, 2, 'two boxed paragraphs')
  assert.equal(boxes[0].bottom, '0px', 'the first box has no bottom border, it joins the next')
  assert.equal(boxes[1].top, '0px', 'the second box has no top border')
  checks.push('editor: two consecutive boxed paragraphs draw as one box')
  await snap('12a_boxes_editor.png')
  await page.goto(`${BASE}/write/1/pages`)
  await page.waitForSelector('.pv-pages:not(.is-hidden) .pagedjs_page', { timeout: 60000 })
  await settle(page, 600)
  const idx = await page.evaluate(() => [...document.querySelectorAll('.pagedjs_page')].findIndex((p) => /Quest updated/.test(p.textContent)))
  assert.ok(idx >= 0, 'the Boxes chapter is in the page view')
  await page.evaluate((i) => document.querySelectorAll('.pagedjs_page')[i].scrollIntoView({ block: 'start' }), idx)
  await settle(page, 400)
  checks.push('page view: box and drop cap')
  await snap('12b_boxes_page_view.png')
}

// Pictures with text wrapping (stage 5): a left and a right float, one at
// the chapter's end, in the editor and in the page view.
// A Word file's front matter: marked as such by the import, without
// taking the file's layout.
async function importShots(page, snap, checks) {
  await page.goto(`${BASE}/write/1?chapter=11`)
  await page.waitForSelector('.write-doc h1')
  await page.setInputFiles('input[type=file][accept^=".docx"]', { name: 'Book.docx', mimeType: 'application/octet-stream', buffer: Buffer.from('PK') })
  await page.waitForSelector('.import-report')
  await settle(page, 400)
  assert.equal(await page.locator('.new-chapters .front .tag').count(), 1, 'the front matter is tagged in the report')
  checks.push('import report: front matter tagged, not counted')
  await snap('14a_import_report.png')
  await page.locator('.import-report .btn-ghost').click()
  await settle(page, 400)
  const nums = await page.locator('.chapter-list .ch-num').allTextContents()
  assert.deepEqual(nums.slice(-3), ['·', '9', '10'], 'front matter is not counted in the chapter numbers')
  checks.push('chapter list: front matter not counted')
  await snap('14b_import_chapter_list.png')
  // The story's Chapters page numbers them the same way.
  await page.goto(`${BASE}/chapters`)
  await page.waitForSelector('.num')
  await settle(page, 300)
  const listed = await page.evaluate(() => Object.fromEntries([...document.querySelectorAll('.chapter-title')].map((e) => [e.textContent.trim(), e.parentElement.querySelector('.num')?.textContent])))
  assert.deepEqual([listed['Front matter'], listed['1. RECODED'], listed['2. WARM CHAINS']], ['·', '9', '10'], 'the Chapters page does not number front matter')
  checks.push('Chapters page: front matter not numbered')
  await snap('14c_chapters_page.png')
}

async function wrapShots(page, snap, checks) {
  await page.goto(`${BASE}/write/1?chapter=18`)
  await page.waitForSelector('.write-doc figure.pm-image[data-wrap]')
  await settle(page, 800)
  const f = await page.evaluate(() => [...document.querySelectorAll('.write-doc figure.pm-image')].map((e) => getComputedStyle(e).float))
  assert.deepEqual(f, ['left', 'right', 'left'], 'pictures float')
  checks.push('editor: pictures float left and right')
  await snap('13a_wrap_editor.png')
  await page.goto(`${BASE}/write/1/pages`)
  await page.waitForSelector('.pv-pages:not(.is-hidden) .pagedjs_page', { timeout: 60000 })
  await settle(page, 800)
  const idx = await page.evaluate(() => [...document.querySelectorAll('.pagedjs_page')].findIndex((p) => p.querySelector('figure.pm-image[data-wrap]')))
  assert.ok(idx >= 0, 'the Wrapping chapter is in the page view')
  await page.evaluate((i) => document.querySelectorAll('.pagedjs_page')[i].scrollIntoView({ block: 'start' }), idx)
  await settle(page, 400)
  checks.push('page view: floated pictures')
  await snap('13b_wrap_page_view.png')
}

// Tables (stage 5): the editor and the page view.
async function tableShots(page, snapAt, checks) {
  const snap = (name, opts) => snapAt(name, opts, page)
  let s0
  const state = () => page.evaluate(() => {
    const v = window.__writeView
    const { $from } = v.state.selection
    let cell = null
    for (let d = $from.depth; d > 0; d--) if (/cell/.test($from.node(d).type.spec.tableRole || '')) cell = $from.node(d).textContent
    return { cell, doc: v.state.doc.toJSON() }
  })
  await page.goto(`${BASE}/write/1?chapter=13`)
  await page.waitForSelector('.write-doc table.pm-table')
  await settle(page, 800)
  // The cursor in the stat block's "Mira Vey": the table box shows.
  await page.locator('.write-doc td', { hasText: 'Mira Vey' }).click()
  await page.waitForSelector('.table-options .chip')
  await settle(page, 300)
  await snap('10_table_editor.png')
  await page.locator('.table-options .chip').click()
  await page.waitForSelector('.table-options .panel')
  await settle(page, 200)
  await snap('10a_table_options.png')
  // Shade the cell from the box; Esc closes the box, the cursor stays.
  await page.getByRole('button', { name: 'Cell shading #fff2cc' }).click()
  await page.keyboard.press('Escape')
  await page.waitForTimeout(100)
  assert.equal(await page.locator('.table-options .panel').count(), 0, 'Esc closes the table box')
  s0 = await state()
  assert.equal(JSON.stringify(s0.doc).includes('"background":"#fff2cc"'), true, 'the cell was shaded')
  checks.push('table box: opens from its button, shades a cell, closes on Esc')
  await page.locator('.write-doc p', { hasText: 'She closed it' }).click()
  await page.waitForTimeout(150)
  assert.equal(await page.locator('.table-options').count(), 0, 'the table box closes outside a table')

  // Insert a 2 × 3 table from the toolbar's grid after the last paragraph,
  // Tab through it; Tab in the last cell adds a row.
  await page.locator('.write-doc p', { hasText: 'It was not much' }).click()
  await page.keyboard.press('End')
  await page.getByRole('button', { name: 'Insert table', exact: true }).click()
  await page.getByRole('button', { name: '2 × 3 table' }).click()
  await page.waitForTimeout(150)
  for (const w of ['HP', '40', '/ 40', 'MP', '12', '/ 15']) {
    await page.keyboard.type(w)
    await page.keyboard.press('Tab')
  }
  let s = await state()
  const made = s.doc.content.filter((n) => n.type === 'table').at(-1)
  assert.equal(made.content.length, 3, 'Tab in the last cell added a row')
  assert.equal(s.cell, '', 'the cursor is in the new row')
  assert.equal(made.content[0].content.map((c) => c.content[0].content?.[0]?.text).join('|'), 'HP|40|/ 40')
  await page.keyboard.press('Shift+Tab')
  s = await state()
  assert.equal(s.cell, '/ 15', 'Shift+Tab goes back a cell')
  checks.push('insert table, Tab, Tab adds a row, Shift+Tab: ok')
  await settle(page, 300)
  await snap('10b_table_inserted.png')

  // A table pasted from Word (in the empty paragraph after the new table)
  // keeps its cells, spans, shading and borders; the shading isn't also
  // taken for a highlight on the text.
  // Let the editor settle first: right after Shift+Tab in a table, headless
  // Edge sometimes leaves the caret where it was (only in the merged stage 5
  // build; not understood yet, see docs/EDITOR.md).
  await settle(page, 400)
  await page.keyboard.press('Control+End')
  await page.evaluate(() => {
    const html = `<html><body><!--StartFragment--><table class=MsoTableGrid border=1 cellspacing=0 cellpadding=0 style='border-collapse:collapse;border:none'>
<tr><td width=200 colspan=2 style='width:150pt;border:solid windowtext 1.0pt;background:#FFF2CC;padding:0cm 5.4pt'><p class=MsoNormal><b>Quest</b></p></td></tr>
<tr><td width=100 valign=middle style='width:75pt;border:solid windowtext 1.0pt;padding:0cm 5.4pt'><p class=MsoNormal>Reward</p></td>
<td width=100 style='width:75pt;border:solid windowtext 1.0pt;padding:0cm 5.4pt'><p class=MsoNormal>50 silver</p></td></tr></table><!--EndFragment--></body></html>`
    const dt = new DataTransfer()
    dt.setData('text/html', html)
    dt.setData('text/plain', 'Quest\nReward\t50 silver')
    window.__writeView.dom.dispatchEvent(new ClipboardEvent('paste', { clipboardData: dt, bubbles: true, cancelable: true }))
  })
  await page.waitForTimeout(200)
  s = await state()
  const pasted = s.doc.content.find((n) => n.type === 'table' && JSON.stringify(n).includes('Quest'))
  assert.ok(pasted, 'the pasted table is a table')
  const q = pasted.content[0].content[0]
  assert.equal(q.attrs.colspan, 2, JSON.stringify(pasted))
  assert.ok(!JSON.stringify(q).includes('highlight'), 'no highlight from the cell’s shading')
  assert.equal(q.attrs.background, '#fff2cc')
  assert.equal(pasted.attrs.borders, 'all')
  assert.equal(pasted.content[1].content[0].attrs.valign, 'middle')
  assert.ok(Math.abs(pasted.content[1].content[1].attrs.colwidth[0] - 100) < 0.5, 'a Word width of 75 pt (100 px)')
  checks.push('paste a table from Word: ok')

  // Dark paper.
  await page.locator('.write-doc td', { hasText: 'Mira Vey' }).click()
  await page.getByRole('button', { name: 'Dark paper' }).click()
  await settle(page, 200)
  await snap('10c_table_dark_paper.png')
  await page.getByRole('button', { name: 'Light paper' }).click()

  // The page view: the inventory breaks between rows, its header repeated.
  await page.goto(`${BASE}/write/1/pages`)
  await page.waitForSelector('.pv-pages:not(.is-hidden) .pagedjs_page', { timeout: 60000 })
  await settle(page, 600)
  const pv = await page.evaluate(() => {
    const pages = [...document.querySelectorAll('.pagedjs_page')]
    const withInventory = pages.filter((p) => [...p.querySelectorAll('td')].some((td) => /kg$/.test(td.textContent.trim())))
    return {
      pages: withInventory.length,
      heads: withInventory.filter((p) => [...p.querySelectorAll('thead th')].some((th) => th.textContent.trim() === '#')).length,
      colgroups: withInventory.filter((p) => [...p.querySelectorAll('table')].some((t) => /kg/.test(t.textContent) && t.querySelector('colgroup col')?.style.width)).length,
      first: pages.indexOf(withInventory[0]),
    }
  })
  checks.push(`page view: the inventory runs over ${pv.pages} pages, the header row on ${pv.heads}, its column widths on ${pv.colgroups}`)
  assert.ok(pv.pages > 1 && pv.heads === pv.pages && pv.colgroups === pv.pages, 'the header row and widths repeat on every page')
  const shotPages = async (from, name) => {
    const box = await page.evaluate((i) => {
      const p = document.querySelectorAll('.pagedjs_page')[i]
      p.scrollIntoView({ block: 'start' })
      return null
    }, from)
    void box
    await settle(page, 300)
    await snap(name)
  }
  const statPage = await page.evaluate(() => [...document.querySelectorAll('.pagedjs_page')].findIndex((p) => /STATUS/.test(p.textContent)))
  await shotPages(statPage, '10d_table_page_view.png')
  await shotPages(pv.first + 1, '10e_table_page_view_continued.png')
}

try {
  await startFrontend()
  const { saved, errors, checks } = option('only', '') === 'spell' ? await spellShots() : await shoot()
  for (const f of saved) console.log(f)
  if (errors.length) console.log('page errors:\n' + errors.join('\n'))
} catch (err) {
  console.error(err)
  process.exitCode = 1
} finally {
  await cleanUp()
}
