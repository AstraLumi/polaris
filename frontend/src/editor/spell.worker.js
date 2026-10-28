// The spellchecker's worker (docs/EDITOR.md, "Spellchecker"): loads one
// dictionary at a time (public/dict/<code>/, gzipped) into hunspell.js and
// answers, off the page's thread, which words of a batch are wrong and what
// a word could be. spellClient.js talks to it.
//
//   { id, type: 'load', code }          → { id, size, ms }
//   { id, type: 'check', code, words }  → { id, wrong: [...] }
//   { id, type: 'suggest', code, word } → { id, suggestions: [...] }
//   any failure                          → { id, error }
import { createSpeller } from './hunspell.js'
import { dictionaryUrl } from './spellWords.js'

let current = null // { code, speller }
let loading = null // { code, promise }

// A .gz file unpacked (unless the server already did, as some proxies do).
async function fetchText(url) {
  const res = await fetch(url, { cache: 'force-cache' })
  if (!res.ok) throw new Error(`${res.status}`)
  const bytes = new Uint8Array(await res.arrayBuffer())
  if (bytes[0] === 0x1f && bytes[1] === 0x8b) {
    const stream = new Blob([bytes]).stream().pipeThrough(new DecompressionStream('gzip'))
    return new Response(stream).text()
  }
  return new TextDecoder().decode(bytes)
}

function load(code) {
  if (current?.code === code) return Promise.resolve(current.speller)
  if (loading?.code === code) return loading.promise
  const promise = (async () => {
    const [aff, dic] = await Promise.all([fetchText(dictionaryUrl(code, 'aff')), fetchText(dictionaryUrl(code, 'dic'))])
    const speller = createSpeller(aff, dic)
    current = { code, speller } // the last one asked for; an older one is let go
    return speller
  })()
  loading = { code, promise }
  promise.finally(() => {
    if (loading?.promise === promise) loading = null
  })
  return promise
}

self.onmessage = async (e) => {
  const { id, type, code } = e.data
  try {
    const start = performance.now()
    const speller = await load(code)
    if (type === 'load') self.postMessage({ id, size: speller.size, ms: Math.round(performance.now() - start) })
    else if (type === 'check') self.postMessage({ id, wrong: e.data.words.filter((w) => !speller.check(w)) })
    else if (type === 'suggest') self.postMessage({ id, suggestions: speller.suggest(e.data.word, { max: 6 }) })
  } catch (err) {
    self.postMessage({ id, error: String(err?.message || err) })
  }
}
