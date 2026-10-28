// The page's side of the spellchecker's worker (spell.worker.js): one
// dictionary at a time, words checked in batches and remembered (a word is
// asked about once per dictionary), suggestions asked for on demand and
// remembered too. No DOM, no Vue: the worker is made by `makeWorker`.

const BATCH = 400

export function createSpellClient(makeWorker = () => new Worker(new URL('./spell.worker.js', import.meta.url), { type: 'module' })) {
  let worker = null
  let nextId = 1
  const waiting = new Map()
  let code = null
  let ready = null // the load of `code`
  let answers = new Map() // word → true (a word) / false (not one)
  let suggestions = new Map()

  function send(msg) {
    if (!worker) {
      worker = makeWorker()
      worker.onmessage = (e) => {
        const w = waiting.get(e.data.id)
        if (!w) return
        waiting.delete(e.data.id)
        if (e.data.error) w.reject(new Error(e.data.error))
        else w.resolve(e.data)
      }
      worker.onerror = (e) => {
        for (const w of waiting.values()) w.reject(new Error(e.message || 'worker failed'))
        waiting.clear()
      }
    }
    const id = nextId++
    return new Promise((resolve, reject) => {
      waiting.set(id, { resolve, reject })
      worker.postMessage({ ...msg, id })
    })
  }

  return {
    get code() {
      return code
    },
    // Loads a dictionary (forgets what the last one said). Resolves with
    // { size, ms }.
    use(next) {
      if (next === code && ready) return ready
      code = next
      answers = new Map()
      suggestions = new Map()
      ready = send({ type: 'load', code: next })
      ready.catch(() => {
        if (code === next) ready = null // asked again next time
      })
      return ready
    },
    // What it already knows about a word: true, false or undefined.
    known: (word) => answers.get(word),
    // The words of the list that aren't words, as a Set.
    async wrong(words) {
      const want = code
      const ask = [...new Set(words)].filter((w) => !answers.has(w))
      for (let i = 0; i < ask.length; i += BATCH) {
        const part = ask.slice(i, i + BATCH)
        const res = await send({ type: 'check', code: want, words: part })
        if (code !== want) break
        const bad = new Set(res.wrong)
        for (const w of part) answers.set(w, !bad.has(w))
      }
      return new Set(words.filter((w) => answers.get(w) === false))
    },
    async suggest(word) {
      if (suggestions.has(word)) return suggestions.get(word)
      const want = code
      const res = await send({ type: 'suggest', code: want, word })
      if (code === want) suggestions.set(word, res.suggestions)
      return res.suggestions
    },
    dispose() {
      worker?.terminate()
      worker = null
      for (const w of waiting.values()) w.reject(new Error('closed'))
      waiting.clear()
      code = null
      ready = null
    },
  }
}
