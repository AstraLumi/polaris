// The filters of the culture pages and the reference library
// (src/cultureFilters.js), and the kinds matching the server's (read as
// text: the page's modules import others without a file extension).
// node frontend/tests/cultures.test.mjs
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { allTags, hasTags, filterItems, countByKind, pastedPictures } from '../src/cultureFilters.js'

const items = [
  { id: 1, name: 'Marshal coat', kind: 'clothing', description: 'Blue and white, worn at court', tags: ['Military', 'formal'] },
  { id: 2, name: 'Winter cloak', kind: 'clothing', description: 'Fur trim', tags: ['winter', 'military'] },
  { id: 3, name: 'Honey bread', kind: 'food', description: 'Eaten at the solstice', tags: ['Winter'] },
]

// Every tag once, the first spelling kept, sorted without regard to case.
assert.deepEqual(allTags(items), ['formal', 'Military', 'winter'])
assert.deepEqual(allTags([]), [])

assert.ok(hasTags(items[1], ['MILITARY', 'Winter']))
assert.ok(!hasTags(items[0], ['winter']))

const names = (list) => list.map((x) => x.id)
assert.deepEqual(names(filterItems(items, { tags: ['winter'] })), [2, 3])
assert.deepEqual(names(filterItems(items, { tags: ['winter'], kind: 'clothing' })), [2])
// Every word must be found, in any of the fields, tags included.
assert.deepEqual(names(filterItems(items, { search: 'court blue', fields: ['name', 'description'] })), [1])
assert.deepEqual(names(filterItems(items, { search: 'formal', fields: ['name', 'tags'] })), [1])
assert.deepEqual(names(filterItems(items, { search: 'formal' })), [])
assert.deepEqual(names(filterItems(items)), [1, 2, 3])

assert.deepEqual(countByKind(items), { clothing: 2, food: 1 })

// A paste holds pictures and text; only the pictures count.
const file = { name: 'image.png' }
const paste = {
  clipboardData: {
    items: [
      { kind: 'string', type: 'text/plain', getAsFile: () => null },
      { kind: 'file', type: 'image/png', getAsFile: () => file },
      { kind: 'file', type: 'application/pdf', getAsFile: () => ({}) },
    ],
  },
}
assert.deepEqual(pastedPictures(paste), [file])
assert.deepEqual(pastedPictures({}), [])

// The kinds are the server's (backend/cultures.go, backend/references.go).
const read = (path) => readFileSync(new URL(path, import.meta.url), 'utf8')
const keys = (text, start, re) => {
  const from = text.indexOf(start)
  assert.ok(from >= 0, `"${start}" not found`)
  const out = [...text.slice(from, text.indexOf(']', from) + 1).matchAll(re)].map((m) => m[1])
  assert.ok(out.length, 'an empty list: the source changed shape, update this test')
  return out
}
const goKeys = (file, start) => {
  const text = read(`../../backend/${file}`)
  const from = text.indexOf(start)
  assert.ok(from >= 0, `${file}: "${start}" not found`)
  return [...text.slice(from, text.indexOf('}', from)).matchAll(/"([a-z]+)"/g)].map((m) => m[1])
}
assert.deepEqual(goKeys('cultures.go', 'var cultureElementKinds'), keys(read('../src/cultures.js'), 'ELEMENT_KINDS', /id: '([a-z]+)'/g))
assert.deepEqual(
  goKeys('references.go', 'var referenceKinds').sort(),
  keys(read('../src/referenceLibrary.js'), 'REFERENCE_KINDS', /id: '([a-z]+)'/g).sort(),
)

console.log('cultures: ok')
