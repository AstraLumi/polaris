// Saving one chapter's text (docs/EDITOR.md, "The client"): every change is
// first kept in the browser, then sent a moment after typing stops. A save
// that finds the text changed somewhere else (409) stops and hands both
// texts to the page, which lets the writer keep one. Pure: the calls and the
// timers come in, so it runs under node tests.
//
// States: 'saved', 'local' (not saved yet: kept in this browser), 'saving',
// 'conflict'.

export function createSaver({
  getDoc, // () => the document's JSON now
  save, // (doc, baseVersion) => { version, words, updated_at }; throws { status: 409, stored } on a conflict
  keep, // (entry) => kept in the browser ({ doc, base_version, saved_at })
  drop, // () => the browser's copy removed
  base = 0,
  delay = 1500,
  keepDelay = 250,
  retryDelay = 10000,
  onState = () => {},
  onSaved = () => {},
  onConflict = () => {},
  timers = globalThis,
}) {
  let state = 'saved'
  let dirty = false
  let changes = 0
  let saving = null
  let conflict = false
  let disposed = false
  let saveTimer = null
  let keepTimer = null
  let lastError = null

  const setState = (s) => {
    if (state !== s) {
      state = s
      onState(s)
    }
  }
  const clear = (t) => t && timers.clearTimeout(t)

  function keepNow() {
    clear(keepTimer)
    keepTimer = null
    return Promise.resolve(keep({ doc: getDoc(), base_version: base, saved_at: Date.now() })).catch(() => {})
  }

  function schedule(ms) {
    clear(saveTimer)
    saveTimer = conflict || disposed ? null : timers.setTimeout(run, ms)
  }

  async function run() {
    saveTimer = null
    if (!dirty || conflict || disposed) return
    if (saving) return saving.then(run)
    const n = changes
    const doc = getDoc()
    setState('saving')
    saving = (async () => {
      try {
        const res = await save(doc, base)
        lastError = null
        if (res && res.version != null) base = res.version
        onSaved(res, doc)
        if (changes === n) {
          dirty = false
          clear(keepTimer)
          keepTimer = null
          await Promise.resolve(drop()).catch(() => {})
          if (changes === n) setState('saved')
        } else {
          setState('local')
          schedule(delay)
        }
      } catch (e) {
        lastError = e
        if (e && e.status === 409) {
          conflict = true
          setState('conflict')
          onConflict(e.stored, doc)
        } else {
          setState('local')
          schedule(retryDelay)
        }
      } finally {
        saving = null
      }
    })()
    return saving
  }

  return {
    get state() {
      return state
    },
    get dirty() {
      return dirty
    },
    get base() {
      return base
    },
    set base(v) {
      base = v
    },
    get error() {
      return lastError
    },
    // Something in the document changed.
    changed() {
      if (disposed) return
      dirty = true
      changes++
      if (!conflict) setState(saving ? 'saving' : 'local')
      clear(keepTimer)
      keepTimer = timers.setTimeout(keepNow, keepDelay)
      schedule(delay)
    },
    // Saves now (Ctrl+S, leaving the chapter). Resolves to whether
    // everything is saved.
    async flush() {
      clear(saveTimer)
      saveTimer = null
      if (keepTimer) await keepNow()
      if (saving) await saving
      if (dirty && !conflict) await run()
      return !dirty
    },
    // The writer chose a text after a conflict: `version` is the stored
    // version now; `unsaved` says whether what is in the editor still has
    // to be sent.
    resolved(version, unsaved = false) {
      conflict = false
      base = version
      if (unsaved) this.changed()
      else {
        dirty = false
        clear(keepTimer)
        keepTimer = null
        Promise.resolve(drop()).catch(() => {})
        setState('saved')
      }
    },
    // Stops; a change not yet kept in the browser is kept first.
    dispose() {
      if (keepTimer) keepNow()
      disposed = true
      clear(saveTimer)
    },
  }
}
