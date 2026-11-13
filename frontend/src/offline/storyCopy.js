// Keeping a whole story on this device for reading offline
// (docs/DEVELOPMENT.md, "Reading offline"): every read the pages make is
// asked once, through `send`, which keeps the answers (offline/kept.js);
// then the pictures those answers name, and the app's own files.
//
// The reads are listed here in the server's own notation. A new `GET` route
// must be added to one of these lists or to SKIPPED: storyCopy.test.mjs
// compares them with the routes in backend/*.go.

// Asked as they are.
export const SINGLES = [
  'home', 'settings', 'stat-system', 'stat-system/defaults', 'character-options', 'location-options',
  'assets/summary', 'characters/heights', 'event-tags', 'timeline', 'map', 'planet', 'planet/layers',
  'solar', 'locations', 'culture-elements', 'wiki', 'wiki/graph', 'spelling/words',
]

// A list, then each of its items by id (and what else an item's page asks).
export const LISTS = {
  characters: ['characters/{id}/current', 'characters/{id}/versions', 'characters/{id}/connections', 'characters/{id}/family', 'characters/{id}/lines'],
  classes: ['classes/{id}'],
  subclasses: ['subclasses/{id}'],
  specializations: ['specializations/{id}'],
  races: ['races/{id}'],
  'body-types': ['body-types/{id}'],
  creatures: ['creatures/{id}'],
  cultures: ['cultures/{id}'],
  events: ['events/{id}'],
  factions: ['factions/{id}'],
  gear: ['gear/{id}'],
  spells: ['spells/{id}'],
  chapters: ['chapters/{id}', 'chapters/{id}/speakers'],
}

// Found in other answers: each version of a character, each wiki article
// and where it appears, each other planet's or moon's map.
export const FOLLOWED = ['versions/{id}', 'wiki/{type}/{id}', 'appears/{type}/{id}', 'solar/bodies/{id}/map']

// Not part of the copy, and why.
export const SKIPPED = {
  health: 'asks whether the server is there',
  stories: 'the list of stories, kept by every visit (kept.js)',
  fonts: 'the fonts library, kept by every visit',
  references: 'the reference library, kept by every visit',
  'fonts/used': "the fonts library's page, kept by every visit",
  'references/used': "the reference library's page, kept by every visit",
  'fonts/line-metrics': 'measures a font on the server',
  'fonts/{id}/license-file': 'a download',
  'stories/{id}/export': 'a download',
  'stories/{id}/snapshots': "the server's own copies, to restore one",
  'books/{volume}': 'the writing page keeps its books itself (offline/store.js)',
  'books/{volume}/docs': 'the writing page',
  'books/{volume}/export.': 'a download',
  'books/{volume}/fonts': 'the writing page',
  'chapters/{id}/text': 'the writing page',
  'chapters/{id}/text/versions': 'the writing page',
  'chapters/{id}/text/versions/{vid}': 'the writing page',
  'chapters/{id}/comments': 'the writing page',
}

// Every route the copy asks, for the test.
export const COVERED = [...SINGLES, ...Object.keys(LISTS), ...Object.values(LISTS).flat(), ...FOLLOWED]

const itemsOf = (json) => (Array.isArray(json) ? json : Array.isArray(json?.items) ? json.items : [])
const idsOf = (json) => itemsOf(json).map((x) => x?.id).filter((id) => id !== undefined && id !== null && id !== '')

// Pictures and files an answer names ("/uploads/…" in any text of it).
export function uploadsIn(text) {
  return new Set(String(text).match(/\/uploads\/[A-Za-z0-9_\-./%~]+/g) || [])
}

// Asks everything. `get(path)` never throws and answers { reached, text }:
// whether the server answered at all, and the answer's text when it was a
// good one (an article nothing appears in may rightly be refused).
// Answers what was asked, what the server couldn't be reached for, and the
// files the answers name.
export async function copyStory({ get, onProgress = () => {}, parallel = 6 }) {
  const queue = []
  const asked = new Set()
  const failed = []
  const files = new Set()
  let done = 0

  // `then(json)` may add more to ask.
  const add = (path, then) => {
    if (asked.has(path)) return
    asked.add(path)
    queue.push({ path, then })
  }
  for (const path of SINGLES) {
    add(path, (json) => {
      if (path === 'wiki') {
        for (const a of itemsOf(json)) {
          if (!a?.type || a.id == null) continue
          add(`wiki/${a.type}/${a.id}`)
          add(`appears/${a.type}/${a.id}`)
        }
      }
      if (path === 'solar') for (const id of idsOf(json?.bodies)) add(`solar/bodies/${id}/map`)
    })
  }
  for (const [list, each] of Object.entries(LISTS)) {
    add(list, (json) => {
      for (const id of idsOf(json)) {
        for (const pattern of each) {
          const path = pattern.replace('{id}', id)
          add(path, pattern === 'characters/{id}/versions' ? (versions) => idsOf(versions).forEach((v) => add(`versions/${v}`)) : null)
        }
      }
    })
  }

  let active = 0
  await new Promise((resolve) => {
    const next = () => {
      if (!queue.length && !active) return resolve()
      while (active < parallel && queue.length) {
        const job = queue.shift()
        active++
        Promise.resolve(get(job.path))
          .then((res) => {
            if (!res?.reached) return void failed.push(job.path)
            if (typeof res.text !== 'string') return
            for (const f of uploadsIn(res.text)) files.add(f)
            if (job.then) {
              try {
                job.then(JSON.parse(res.text))
              } catch (e) {
                failed.push(job.path)
              }
            }
          })
          .catch(() => failed.push(job.path))
          .finally(() => {
            active--
            onProgress(++done, asked.size)
            next()
          })
      }
    }
    next()
  })
  return { asked: [...asked], failed, files: [...files] }
}
