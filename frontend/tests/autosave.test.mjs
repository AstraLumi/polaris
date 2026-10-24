// Saving a chapter's text (src/editor/autosave.js): kept in the browser at
// once, sent after a pause, a 409 stops and hands over both texts, a lost
// connection retries. Fake timers and calls. node frontend/tests/autosave.test.mjs
import assert from 'node:assert/strict'
import { createSaver } from '../src/editor/autosave.js'

// Fake timers: run() advances the clock and fires what is due, in order.
function fakeTimers() {
  let now = 0
  let id = 0
  const due = new Map()
  return {
    setTimeout: (fn, ms) => (due.set(++id, { fn, at: now + ms }), id),
    clearTimeout: (i) => due.delete(i),
    async advance(ms) {
      const until = now + ms
      for (;;) {
        const next = [...due.entries()].filter(([, t]) => t.at <= until).sort((a, b) => a[1].at - b[1].at)[0]
        if (!next) break
        due.delete(next[0])
        now = next[1].at
        await next[1].fn()
        await flushPromises()
      }
      now = until
      await flushPromises()
    },
  }
}
const flushPromises = async () => {
  for (let i = 0; i < 10; i++) await Promise.resolve()
}

function setup({ server = { version: 3 }, fail, wait } = {}) {
  const timers = fakeTimers()
  const log = []
  let doc = { type: 'doc', n: 0 }
  let kept = null
  const states = []
  const saver = createSaver({
    getDoc: () => doc,
    base: server.version,
    timers,
    save: async (d, base) => {
      log.push(['save', d.n, base])
      if (wait) await wait()
      const f = fail?.()
      if (f) throw f
      if (base !== server.version) {
        const e = new Error('conflict')
        e.status = 409
        e.stored = { doc: 'theirs', version: server.version }
        throw e
      }
      server.version++
      return { version: server.version, words: 1 }
    },
    keep: async (entry) => {
      kept = entry
      log.push(['keep', entry.doc.n, entry.base_version])
    },
    drop: async () => {
      kept = null
      log.push(['drop'])
    },
    onState: (s) => states.push(s),
    onConflict: (stored, mine) => log.push(['conflict', stored.version, mine.n]),
  })
  return {
    timers,
    log,
    saver,
    states,
    server,
    type(n) {
      doc = { type: 'doc', n }
      saver.changed()
    },
    get kept() {
      return kept
    },
  }
}

// Typing: kept in the browser right away, saved once typing stops.
{
  const t = setup()
  t.type(1)
  assert.equal(t.saver.state, 'local')
  await t.timers.advance(300)
  assert.deepEqual(t.kept.doc, { type: 'doc', n: 1 })
  assert.equal(t.kept.base_version, 3)
  t.type(2)
  await t.timers.advance(1000)
  t.type(3)
  await t.timers.advance(1000)
  assert.ok(!t.log.some((l) => l[0] === 'save'), 'not while typing')
  await t.timers.advance(600)
  assert.deepEqual(t.log.filter((l) => l[0] === 'save'), [['save', 3, 3]], 'one save, of the latest text')
  assert.equal(t.saver.state, 'saved')
  assert.equal(t.saver.base, 4)
  assert.equal(t.kept, null, 'the browser copy goes once the server has it')
  assert.deepEqual(t.states, ['local', 'saving', 'saved'])
  t.type(4)
  await t.timers.advance(2000)
  assert.deepEqual(t.log.filter((l) => l[0] === 'save').at(-1), ['save', 4, 4], 'the next save builds on the new version')
}

// Ctrl+S saves at once.
{
  const t = setup()
  t.type(1)
  assert.equal(await t.saver.flush(), true)
  assert.deepEqual(t.log.map((l) => l[0]), ['keep', 'save', 'drop'])
  assert.equal(await t.saver.flush(), true, 'nothing to do')
}

// Typing while a save is on its way: saved again after it.
{
  let release = null
  let calls = 0
  const t = setup({ wait: () => (calls++ ? null : new Promise((r) => (release = r))) })
  t.type(1)
  const p = t.saver.flush()
  await flushPromises()
  assert.equal(t.saver.state, 'saving')
  t.type(2)
  assert.equal(t.saver.state, 'saving')
  release()
  await p
  assert.equal(t.saver.state, 'local', 'the newer text is not saved yet')
  assert.ok(t.kept, 'so its browser copy stays')
  await t.timers.advance(2000)
  assert.deepEqual(t.log.filter((l) => l[0] === 'save'), [['save', 1, 3], ['save', 2, 4]])
  assert.equal(t.saver.state, 'saved')
  assert.equal(t.kept, null)
}

// A lost connection: still kept in the browser, tried again later.
{
  let down = true
  const t = setup({ fail: () => (down ? Object.assign(new Error('offline'), { status: 0 }) : null) })
  t.type(1)
  await t.timers.advance(1600)
  assert.equal(t.saver.state, 'local')
  assert.ok(t.kept, 'the browser copy stays')
  assert.equal(t.saver.error.status, 0)
  down = false
  await t.timers.advance(10000)
  assert.equal(t.saver.state, 'saved')
  assert.equal(t.kept, null)
}

// Changed somewhere else: a 409 stops saving and hands over both texts.
{
  const t = setup()
  t.server.version = 7 // another tab saved twice
  t.type(1)
  await t.timers.advance(1600)
  assert.equal(t.saver.state, 'conflict')
  assert.deepEqual(t.log.at(-1), ['conflict', 7, 1])
  t.type(2)
  await t.timers.advance(5000)
  assert.equal(t.log.filter((l) => l[0] === 'save').length, 1, 'no more saves until the writer chooses')
  assert.equal(t.saver.state, 'conflict')
  assert.ok(t.kept, 'what was typed stays in the browser meanwhile')
  // Keeping mine: send it on top of the stored version.
  t.saver.resolved(7, true)
  await t.timers.advance(1600)
  assert.deepEqual(t.log.filter((l) => l[0] === 'save').at(-1), ['save', 2, 7])
  assert.equal(t.saver.state, 'saved')
}

// Keeping theirs: nothing left to send.
{
  const t = setup()
  t.server.version = 5
  t.type(1)
  await t.timers.advance(1600)
  t.saver.resolved(5, false)
  await t.timers.advance(10)
  assert.equal(t.saver.state, 'saved')
  assert.equal(t.saver.dirty, false)
  assert.equal(t.kept, null)
}

// Leaving: a change not yet kept is kept before the saver stops.
{
  const t = setup()
  t.type(9)
  t.saver.dispose()
  await flushPromises()
  assert.equal(t.kept.doc.n, 9)
  await t.timers.advance(5000)
  assert.ok(!t.log.some((l) => l[0] === 'save'), 'stopped')
}

console.log('autosave: ok')
