// Copying a story for reading offline (src/offline/storyCopy.js): what is
// asked, on a fake server, and that every read the server offers is either
// part of the copy or left out on purpose.
// node frontend/tests/storyCopy.test.mjs
import assert from 'node:assert/strict'
import { readdirSync, readFileSync } from 'node:fs'
import { copyStory, uploadsIn, COVERED, SKIPPED, SINGLES, LISTS } from '../src/offline/storyCopy.js'

// ---- Every GET route of the server is accounted for ----
const backend = new URL('../../backend/', import.meta.url)
const routes = new Set()
for (const f of readdirSync(backend)) {
  if (!f.endsWith('.go') || f.endsWith('_test.go')) continue
  for (const m of readFileSync(new URL(f, backend), 'utf8').matchAll(/"GET \/api\/([^"]*)"/g)) routes.add(m[1])
}
assert.ok(routes.size > 50, 'the routes were found')
const known = new Set([...COVERED, ...Object.keys(SKIPPED)])
assert.deepEqual(
  [...routes].filter((r) => !known.has(r)),
  [],
  'a new GET route: add it to storyCopy.js (asked for the offline copy, or SKIPPED with the reason)',
)
assert.deepEqual(
  [...known].filter((r) => !routes.has(r)),
  [],
  'storyCopy.js names a route the server no longer has',
)

// ---- Files named in answers ----
assert.deepEqual(
  [...uploadsIn('{"picture":"/uploads/s/ab12/hero.png","text":"![map](/uploads/s/ab12/maps/world%20map.jpg) and /uploads/references/r1.webp\\n"}')],
  ['/uploads/s/ab12/hero.png', '/uploads/s/ab12/maps/world%20map.jpg', '/uploads/references/r1.webp'],
)

// ---- A copy ----
const server = {
  characters: [{ id: 1, name: 'Ayla', picture: '/uploads/s/ab12/ayla.png' }, { id: 2, name: 'Bren' }],
  'characters/1/versions': [{ id: 11 }, { id: 12 }],
  'characters/2/versions': [{ id: 21 }],
  wiki: [
    { type: 'character', id: 1 },
    { type: 'location', id: 7 },
  ],
  solar: { bodies: [{ id: 3 }, { id: 4 }] },
  events: [{ id: 5 }],
  'events/5': { id: 5, description: 'See ![](/uploads/s/ab12/battle.jpg)' },
}
const refused = new Set(['appears/location/7']) // the server says no: nothing to keep, not a failure
const down = new Set(['creatures'])
const asked = []
let most = 0
let running = 0
const get = async (path) => {
  asked.push(path)
  most = Math.max(most, ++running)
  await new Promise((r) => setTimeout(r, 1))
  running--
  if (down.has(path)) return { reached: false }
  if (refused.has(path)) return { reached: true, text: null }
  return { reached: true, text: JSON.stringify(server[path] ?? (path in LISTS ? [] : {})) }
}
let last = null
const out = await copyStory({ get, onProgress: (done, total) => (last = [done, total]), parallel: 4 })

assert.equal(new Set(asked).size, asked.length, 'nothing is asked twice')
assert.ok(most <= 4, 'no more at once than allowed')
for (const path of [...SINGLES, ...Object.keys(LISTS)]) assert.ok(asked.includes(path), path)
for (const path of [
  'characters/1/current', 'characters/1/family', 'characters/1/connections', 'characters/1/lines', 'characters/2/versions',
  'versions/11', 'versions/12', 'versions/21',
  'wiki/character/1', 'appears/character/1', 'wiki/location/7', 'appears/location/7',
  'solar/bodies/3/map', 'solar/bodies/4/map', 'events/5',
]) {
  assert.ok(asked.includes(path), path)
}
assert.deepEqual(out.failed, ['creatures'])
assert.deepEqual(out.files.sort(), ['/uploads/s/ab12/ayla.png', '/uploads/s/ab12/battle.jpg'])
assert.deepEqual(last, [asked.length, asked.length])
assert.equal(out.asked.length, asked.length)

console.log('storyCopy: ok')
