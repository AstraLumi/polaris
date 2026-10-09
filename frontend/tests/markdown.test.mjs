// The wiki's Markdown renderer. No dependencies: node frontend/tests/markdown.test.mjs
import assert from 'node:assert/strict'
import { renderMarkdown } from '../src/markdown.js'

const articles = [
  { type: 'location', id: 7, name: 'Aldoria' },
  { type: 'race', id: 2, name: 'Dragon' },
]
const resolve = (inner) => {
  const [a, b] = inner.includes(':') ? inner.split(':') : ['', inner]
  return articles.find((x) => x.name.toLowerCase() === b.trim().toLowerCase() && (!a || x.type === a)) || null
}
const md = (s) => renderMarkdown(s, resolve)

// Nothing the user types becomes markup of its own.
assert.equal(md('<script>alert(1)</script>'), '<p>&lt;script&gt;alert(1)&lt;/script&gt;</p>')
assert.ok(!md('<img src=x onerror=alert(1)>').includes('<img'))
assert.ok(!md('[x](javascript:alert(1))').includes('<a'))
assert.ok(!md('[x](data:text/html,hi)').includes('<a'))
assert.ok(md('[ok](https://example.com)').includes('rel="noopener noreferrer"'))
assert.ok(md('[a "quoted" b](https://e.com/?q="x")').includes('&quot;'))

// Inline.
assert.equal(md('**bold** and *it* and ~~gone~~ and `a<b`'),
  '<p><strong>bold</strong> and <em>it</em> and <del>gone</del> and <code>a&lt;b</code></p>')
assert.equal(md('snake_case_name stays'), '<p>snake_case_name stays</p>')
assert.equal(md('one\ntwo'), '<p>one<br />two</p>')

// Blocks.
assert.equal(md('# Title'), '<h3>Title</h3>')
assert.equal(md('- a\n- b'), '<ul><li>a</li><li>b</li></ul>')
assert.equal(md('1. a\n2. b'), '<ol><li>a</li><li>b</li></ol>')
assert.equal(md('- a\n  - b\n- c'), '<ul><li>a<ul><li>b</li></ul></li><li>c</li></ul>')
assert.equal(md('> quoted'), '<blockquote><p>quoted</p></blockquote>')
assert.equal(md('---'), '<hr />')
assert.equal(md('```\n<b>\n```'), '<pre><code>&lt;b&gt;</code></pre>')
assert.ok(md('| a | b |\n|---|--:|\n| 1 | 2 |').includes('<th style="text-align:right">b</th>'))

// Wiki links.
assert.equal(md('See [[Aldoria]].'),
  '<p>See <a class="wikilink" href="/wiki/location/7" data-wiki="location/7">Aldoria</a>.</p>')
assert.ok(md('[[aldoria|the city]]').includes('>the city</a>'))
assert.ok(md('[[race:Dragon]]').includes('href="/wiki/race/2"'))
assert.ok(md('[[Nowhere]]').includes('wikilink is-missing'))
assert.ok(md('[[<b>]]').includes('&lt;b&gt;'))
assert.ok(md('`[[Aldoria]]`').includes('<code>[[Aldoria]]</code>'))

console.log('markdown: ok')
