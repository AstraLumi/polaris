// The copy of each chapter's text kept in this browser (IndexedDB) until the
// server has it, so a closed tab or a lost connection loses nothing. One
// entry per story and chapter: { doc, base_version, saved_at }. When the
// browser won't store anything, these quietly do nothing.

const DB_NAME = 'polaris-writing'
const STORE = 'drafts'
let opening = null

function db() {
  if (!opening) {
    opening = new Promise((resolve, reject) => {
      if (typeof indexedDB === 'undefined') return reject(new Error('no IndexedDB'))
      const req = indexedDB.open(DB_NAME, 1)
      req.onupgradeneeded = () => req.result.createObjectStore(STORE)
      req.onsuccess = () => resolve(req.result)
      req.onerror = () => reject(req.error)
    }).catch((e) => {
      opening = null
      throw e
    })
  }
  return opening
}

async function run(mode, fn) {
  try {
    const d = await db()
    return await new Promise((resolve, reject) => {
      const tx = d.transaction(STORE, mode)
      const req = fn(tx.objectStore(STORE))
      tx.oncomplete = () => resolve(req?.result ?? null)
      tx.onerror = () => reject(tx.error)
      tx.onabort = () => reject(tx.error)
    })
  } catch (e) {
    return null
  }
}

const key = (story, chapter) => `${story}:${chapter}`

export const keepCopy = (story, chapter, entry) => run('readwrite', (s) => s.put(entry, key(story, chapter)))
export const readCopy = (story, chapter) => run('readonly', (s) => s.get(key(story, chapter)))
export const dropCopy = (story, chapter) => run('readwrite', (s) => s.delete(key(story, chapter)))
