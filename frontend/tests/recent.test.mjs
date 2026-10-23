// The recently opened list. No dependencies: node frontend/tests/recent.test.mjs
import assert from 'node:assert/strict'
import { pushRecent, dropRecent, cleanRecent, recentPath, recentKey, loadRecent, saveRecent, MAX_RECENT } from '../src/recent.js'

const e = (type, id, name = `${type} ${id}`) => ({ type, id, name, at: 1 })

// Newest first, no duplicates: opening an item again moves it to the top.
let list = []
list = pushRecent(list, e('character', 1))
list = pushRecent(list, e('event', 1))
list = pushRecent(list, e('character', 2))
assert.deepEqual(list.map(recentKey), ['character:2', 'event:1', 'character:1'])
list = pushRecent(list, e('character', 1, 'Renamed'))
assert.deepEqual(list.map(recentKey), ['character:1', 'character:2', 'event:1'])
assert.equal(list[0].name, 'Renamed')

// The same id of another type is another item.
assert.equal(pushRecent(list, e('chapter', 1)).length, 4)

// At most MAX_RECENT; the oldest falls off.
let many = []
for (let i = 1; i <= 12; i++) many = pushRecent(many, e('location', i))
assert.equal(many.length, MAX_RECENT)
assert.equal(many[0].id, 12)
assert.equal(many[many.length - 1].id, 12 - MAX_RECENT + 1)

// Deleted items go.
assert.deepEqual(dropRecent(list, 'character', 2).map(recentKey), ['character:1', 'event:1'])
assert.equal(dropRecent(list, 'character', '2').length, 2, 'ids compare as text')

// Rubbish from storage is dropped, not shown.
assert.deepEqual(cleanRecent('nope'), [])
assert.deepEqual(
  cleanRecent([null, 3, {}, { type: 'event' }, { type: 'event', id: 4, name: 'Ok' }, { type: 'event', id: 4 }]).map(recentKey),
  ['event:4'],
)

// Where each kind opens.
assert.equal(recentPath(e('character', 3)), '/characters/3')
assert.equal(recentPath(e('event', 3)), '/events/3')
assert.equal(recentPath(e('chapter', 3)), '/chapters/3')
assert.equal(recentPath(e('lore', 3)), '/wiki/lore/3')
assert.equal(recentPath(e('location', 3)), '/wiki/location/3')

// No storage to read from or write to (node has none) is fine.
assert.deepEqual(loadRecent('abc'), [])
assert.deepEqual(loadRecent(''), [])
saveRecent('abc', list)

// With a storage, each story has its own list.
const store = new Map()
globalThis.localStorage = { getItem: (k) => (store.has(k) ? store.get(k) : null), setItem: (k, v) => store.set(k, String(v)) }
saveRecent('one', list)
assert.deepEqual(loadRecent('one').map(recentKey), ['character:1', 'character:2', 'event:1'])
assert.deepEqual(loadRecent('two'), [])
store.set('polaris-recent-bad', '{not json')
assert.deepEqual(loadRecent('bad'), [])

console.log('recent: ok')
