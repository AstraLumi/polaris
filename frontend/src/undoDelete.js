// Deleting with a way back. A list that deletes through here hides the item
// at once and shows "Deleted · Undo" (UndoBar.vue, in App.vue) for a few
// seconds; the server is only asked to delete when they pass. Undo before
// then and nothing was ever sent.
//
//   deleteSoon({
//     kind: 'event', ids: [e.id],
//     label: t('Deleted “{name}”.', { name: e.name }),
//     run: () => eventsApi.remove(e.id),   // the real delete
//     done: load,                           // then the list, read again
//   })
//
// and the list shows notGone('event', events). Nothing in the list itself is
// touched, so Undo is just the item no longer being hidden.
//
// One deletion waits at a time: a second one, leaving the page (router.js)
// or closing the tab carries the first out at once, so no other page ever
// meets an item that is about to go.
import { ref, reactive } from 'vue'

export const UNDO_MS = 8000

// What the bar shows: { label } while a deletion waits, { error } for a
// while after one failed.
export const pendingDelete = ref(null)

const gone = reactive(new Set())
const keyOf = (kind, id) => `${kind}:${id}`

export const isGone = (kind, id) => gone.has(keyOf(kind, id))
export const notGone = (kind, list) => (gone.size ? list.filter((x) => !gone.has(keyOf(kind, x.id))) : list)

let current = null
let errorTimer = 0

// Set while the tab is closing: request.js then asks for what is sent to
// outlive the page.
export const leaving = { now: false }

export function deleteSoon({ kind, ids, label, run, done }) {
  flushDeletes()
  clearTimeout(errorTimer)
  const keys = ids.map((id) => keyOf(kind, id))
  for (const k of keys) gone.add(k)
  current = { keys, run, done, timer: setTimeout(flushDeletes, UNDO_MS) }
  pendingDelete.value = { label }
}

export function undoDelete() {
  const c = current
  if (!c) return
  current = null
  clearTimeout(c.timer)
  for (const k of c.keys) gone.delete(k)
  pendingDelete.value = null
}

// Carries the waiting deletion out now. reload is false when the page that
// asked is being left: its list need not be read again.
export async function flushDeletes({ reload = true } = {}) {
  const c = current
  if (!c) return
  current = null
  clearTimeout(c.timer)
  pendingDelete.value = null
  let failed = null
  try {
    await c.run()
  } catch (e) {
    failed = e
  }
  if (reload) {
    try {
      await c.done?.()
    } catch (e) {
      // The list says so itself.
    }
  }
  // Only now, with the list read again: the item never flickers back.
  for (const k of c.keys) gone.delete(k)
  if (failed && !current) {
    pendingDelete.value = { error: failed.message || '' }
    errorTimer = setTimeout(() => {
      if (pendingDelete.value && 'error' in pendingDelete.value) pendingDelete.value = null
    }, 7000)
  }
}

export function dismissDeleteError() {
  if (pendingDelete.value && 'error' in pendingDelete.value) pendingDelete.value = null
}

if (typeof window !== 'undefined') {
  // Closing the tab: the request is marked to outlive the page.
  window.addEventListener('pagehide', () => {
    leaving.now = true
    flushDeletes({ reload: false })
  })
  window.addEventListener('pageshow', () => (leaving.now = false))
}
