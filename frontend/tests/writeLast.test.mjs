// Where the writing was left. node frontend/tests/writeLast.test.mjs
import assert from 'node:assert/strict'
import { rememberWriting, lastWriting, writePath } from '../src/writeLast.js'

const box = () => {
  const m = new Map()
  return { getItem: (k) => (m.has(k) ? m.get(k) : null), setItem: (k, v) => m.set(k, String(v)), m }
}

// One place per story.
let s = box()
assert.equal(lastWriting('a', s), null)
rememberWriting('a', 3, 12, s)
rememberWriting('b', '0', 5, s)
assert.deepEqual(lastWriting('a', s), { volume: '3', chapter: 12 })
assert.deepEqual(lastWriting('b', s), { volume: '0', chapter: 5 })
rememberWriting('a', 4, null, s)
assert.deepEqual(lastWriting('a', s), { volume: '4', chapter: 0 })

// Nonsense is neither kept nor given back.
rememberWriting('a', 'x', 1, s)
rememberWriting('', 1, 1, s)
assert.deepEqual(lastWriting('a', s), { volume: '4', chapter: 0 })
s.setItem('polaris-write-last:c', '{"volume":"../1"}')
assert.equal(lastWriting('c', s), null)
s.setItem('polaris-write-last:c', 'not json')
assert.equal(lastWriting('c', s), null)

// Storage that refuses (private windows) never throws.
const broken = { getItem() { throw new Error('no') }, setItem() { throw new Error('no') } }
rememberWriting('a', 1, 1, broken)
assert.equal(lastWriting('a', broken), null)

assert.deepEqual(writePath({ volume: '3', chapter: 12 }), { path: '/write/3', query: { chapter: '12' } })
assert.deepEqual(writePath({ volume: 0, chapter: 0 }), { path: '/write/0', query: {} })

console.log('writeLast: ok')
