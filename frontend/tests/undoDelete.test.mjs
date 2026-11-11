// Deleting with a way back. node frontend/tests/undoDelete.test.mjs
import assert from 'node:assert/strict'
import { deleteSoon, undoDelete, flushDeletes, notGone, isGone, pendingDelete } from '../src/undoDelete.js'

const list = [{ id: 1 }, { id: 2 }, { id: 3 }]
const ids = (l) => l.map((x) => x.id)

// Hidden at once, nothing sent; Undo shows it again and never sends.
let sent = []
deleteSoon({ kind: 'event', ids: [2], label: 'two', run: async () => sent.push(2) })
assert.deepEqual(ids(notGone('event', list)), [1, 3])
assert.deepEqual(ids(notGone('chapter', list)), [1, 2, 3], 'another kind with the same id is not hidden')
assert.equal(pendingDelete.value.label, 'two')
undoDelete()
assert.deepEqual(ids(notGone('event', list)), [1, 2, 3])
assert.equal(pendingDelete.value, null)
await flushDeletes()
assert.deepEqual(sent, [], 'an undone deletion is never sent')

// Carried out: sent, the list read again, and only then no longer hidden.
let order = []
deleteSoon({
  kind: 'event',
  ids: [1, 3],
  label: 'both',
  run: async () => order.push('run'),
  done: async () => order.push(isGone('event', 1) ? 'reload while hidden' : 'reload too late'),
})
await flushDeletes()
assert.deepEqual(order, ['run', 'reload while hidden'])
assert.equal(isGone('event', 1), false)
assert.equal(pendingDelete.value, null)

// A second deletion carries the first out.
sent = []
deleteSoon({ kind: 'event', ids: [1], label: 'a', run: async () => sent.push(1) })
deleteSoon({ kind: 'event', ids: [2], label: 'b', run: async () => sent.push(2) })
assert.deepEqual(sent, [1])
assert.equal(pendingDelete.value.label, 'b')
undoDelete()
assert.deepEqual(sent, [1])

// One that fails comes back and says why.
deleteSoon({
  kind: 'event',
  ids: [3],
  label: 'c',
  run: async () => {
    throw new Error('no')
  },
})
await flushDeletes()
assert.equal(isGone('event', 3), false)
assert.equal(pendingDelete.value.error, 'no')

// Leaving the page: sent, but the list it came from is not read again.
let reloaded = false
deleteSoon({ kind: 'event', ids: [3], label: 'd', run: async () => {}, done: () => (reloaded = true) })
await flushDeletes({ reload: false })
assert.equal(reloaded, false)

console.log('undoDelete: ok')
process.exit(0) // the error note's timer would keep node waiting
