// Where the writing was left on this device, one place per story: the
// sidebar's Write opens it when the server can't say (no connection).
// Plain functions, no dependencies: node tests/writeLast.test.mjs

const key = (story) => `polaris-write-last:${story}`

export function rememberWriting(story, volume, chapter, storage = globalThis.localStorage) {
  if (!story || !/^\d+$/.test(String(volume))) return
  try {
    storage.setItem(key(story), JSON.stringify({ volume: String(volume), chapter: Number(chapter) || 0 }))
  } catch (e) {
    /* not remembered */
  }
}

export function lastWriting(story, storage = globalThis.localStorage) {
  try {
    const v = JSON.parse(storage.getItem(key(story)) || 'null')
    if (!v || !/^\d+$/.test(String(v.volume))) return null
    return { volume: String(v.volume), chapter: Number(v.chapter) > 0 ? Number(v.chapter) : 0 }
  } catch (e) {
    return null
  }
}

// The address of a book, at a chapter when one is known.
export function writePath({ volume, chapter }) {
  return { path: `/write/${volume || 0}`, query: chapter ? { chapter: String(chapter) } : {} }
}
