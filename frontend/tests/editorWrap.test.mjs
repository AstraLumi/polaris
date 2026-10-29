// Pictures with text wrapping (image attr wrap: inline | left | right):
// the default for old docs, JSON and DOM round trips.
// node frontend/tests/editorWrap.test.mjs
import assert from 'node:assert/strict'
import { schema, docFromJSON, docToJSON } from '../src/editor/schema.js'

const img = (attrs) => ({ type: 'doc', content: [{ type: 'image', attrs: { src: '/uploads/a.png', alt: '', width: 120, align: 'left', fullPage: false, ...attrs } }, { type: 'paragraph' }] })

// An old doc without the attr reads as inline and stays so.
const old = docFromJSON({ type: 'doc', content: [{ type: 'image', attrs: { src: '/uploads/a.png' } }] })
assert.equal(old.firstChild.attrs.wrap, 'inline')

for (const wrap of ['inline', 'left', 'right']) {
  const out = docToJSON(docFromJSON(img({ wrap })))
  assert.equal(out.content[0].attrs.wrap, wrap)
}

// DOM: a wrapped picture carries data-wrap; inline carries none.
const node = (wrap) => schema.nodes.image.create({ src: '/uploads/a.png', width: 100, wrap })
assert.equal(schema.nodes.image.spec.toDOM(node('left'))[1]['data-wrap'], 'left')
assert.equal(schema.nodes.image.spec.toDOM(node('right'))[1]['data-wrap'], 'right')
assert.equal(schema.nodes.image.spec.toDOM(node('inline'))[1]['data-wrap'], undefined)

// DOM in: data-wrap is read back; unknown values fall to inline.
const rule = schema.nodes.image.spec.parseDOM[0]
const fig = (w) => {
  const im = { getAttribute: (k) => ({ src: '/uploads/a.png', alt: '' })[k] || null }
  return { querySelector: () => im, getAttribute: (k) => (k === 'data-wrap' ? w : null), hasAttribute: () => false }
}
assert.equal(rule.getAttrs(fig('right')).wrap, 'right')
assert.equal(rule.getAttrs(fig('bogus')).wrap, 'inline')
assert.equal(rule.getAttrs(fig(null)).wrap, 'inline')
console.log('editorWrap ok')
