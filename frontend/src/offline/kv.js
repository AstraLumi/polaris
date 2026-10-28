// The offline store's storage (docs/EDITOR.md, "Offline writing"): a few
// named tables of key → value, in IndexedDB in the browser and in memory for
// the node tests. Keys are strings ("<story>:<volume>", "<story>:<chapter>"),
// so `entries(table, prefix)` lists one story's rows. `update` reads and
// writes one row in a single transaction.

export const TABLES = ['books', 'chapters', 'stories']

export function memoryKV() {
  const tables = new Map(TABLES.map((t) => [t, new Map()]))
  const copy = (v) => (v === undefined ? null : structuredClone(v))
  const table = (name) => {
    const t = tables.get(name)
    if (!t) throw new Error(`no table ${name}`)
    return t
  }
  return {
    async get(name, key) {
      return copy(table(name).get(key))
    },
    async put(name, key, value) {
      table(name).set(key, copy(value))
    },
    async del(name, key) {
      table(name).delete(key)
    },
    async entries(name, prefix = '') {
      return [...table(name).entries()]
        .filter(([k]) => k.startsWith(prefix))
        .sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))
        .map(([k, v]) => [k, copy(v)])
    },
    // fn(old value or null) → the new value (undefined: leave it; null: delete).
    async update(name, key, fn) {
      const next = fn(copy(table(name).get(key)))
      if (next === null) table(name).delete(key)
      else if (next !== undefined) table(name).set(key, copy(next))
      return next === undefined ? copy(table(name).get(key)) : copy(next)
    },
  }
}

export function idbKV(dbName = 'polaris-offline', idb = globalThis.indexedDB) {
  let opening = null
  function open() {
    if (!opening) {
      opening = new Promise((resolve, reject) => {
        if (!idb) return reject(new Error('no IndexedDB'))
        const req = idb.open(dbName, 1)
        req.onupgradeneeded = () => {
          for (const t of TABLES) if (!req.result.objectStoreNames.contains(t)) req.result.createObjectStore(t)
        }
        req.onsuccess = () => {
          const db = req.result
          // Another tab upgrading the database: let it, and open again next time.
          db.onversionchange = () => {
            db.close()
            opening = null
          }
          resolve(db)
        }
        req.onerror = () => reject(req.error)
      }).catch((e) => {
        opening = null
        throw e
      })
    }
    return opening
  }

  async function run(name, mode, fn) {
    const db = await open()
    return new Promise((resolve, reject) => {
      const tx = db.transaction(name, mode)
      let result
      fn(tx.objectStore(name), (v) => (result = v))
      tx.oncomplete = () => resolve(result)
      tx.onerror = () => reject(tx.error)
      tx.onabort = () => reject(tx.error || new Error('aborted'))
    })
  }

  const range = (prefix) => (prefix ? IDBKeyRange.bound(prefix, prefix + '￿') : undefined)

  return {
    get: (name, key) =>
      run(name, 'readonly', (s, done) => {
        const r = s.get(key)
        r.onsuccess = () => done(r.result ?? null)
      }),
    put: (name, key, value) => run(name, 'readwrite', (s) => s.put(value, key)),
    del: (name, key) => run(name, 'readwrite', (s) => s.delete(key)),
    entries: (name, prefix = '') =>
      run(name, 'readonly', (s, done) => {
        const out = []
        const r = s.openCursor(range(prefix))
        r.onsuccess = () => {
          const c = r.result
          if (!c) return done(out)
          out.push([c.key, c.value])
          c.continue()
        }
      }),
    update: (name, key, fn) =>
      run(name, 'readwrite', (s, done) => {
        const r = s.get(key)
        r.onsuccess = () => {
          const old = r.result ?? null
          const next = fn(old === null ? null : structuredClone(old))
          if (next === null) s.delete(key)
          else if (next !== undefined) s.put(next, key)
          done(next === undefined ? old : next)
        }
      }),
  }
}
