// The wiki's Markdown renderer. No dependencies: node frontend/tests/markdown.test.mjs
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { renderMarkdown } from '../src/markdown.js'
import { TYPE_ORDER, makeLinkResolver, linksForEditing, linksAsText, findWikiLinks, parseIdLink } from '../src/wikiLinks.js'

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

// Links by id, read the same way as backend/wikilinks.go: the shared cases.
const fx = JSON.parse(readFileSync(new URL('./wikiLinks.fixtures.json', import.meta.url), 'utf8'))
assert.deepEqual([...TYPE_ORDER].sort(), [...fx.types].sort(), 'TYPE_ORDER and the article kinds of the server')
const linked = makeLinkResolver(fx.items)
linked.deletedLabel = 'gone'
const key = (it) => (it ? `${it.type}#${it.id}` : null)
for (const [target, want] of fx.resolve) assert.equal(key(linked(target)), want, `resolve ${JSON.stringify(target)}`)
for (const [stored, shown] of fx.editing) assert.equal(linksForEditing(stored, linked), shown, `editing ${JSON.stringify(stored)}`)
// Stored text is left alone by the editor's conversion once it is by name.
for (const [, shown] of fx.editing) assert.equal(linksForEditing(shown, linked), shown)
// As plain text (a description shown outside the wiki): the shared cases
// show a deleted article as nothing; here with no label, then with one.
const plainNames = makeLinkResolver(fx.items)
for (const [stored, shown] of fx.plain) assert.equal(linksAsText(stored, plainNames), shown, `as text ${JSON.stringify(stored)}`)
assert.equal(linksAsText('[[character#99]] [[character#12]]', linked), 'gone Aria Vale')
// Before the names load, a stored link without its own text shows nothing.
assert.equal(linksAsText('A [[character#12]], [[character#12|her]], [[Bram]].', null), 'A , her, Bram.')
assert.equal(linksAsText(null, linked), '')
for (const [text, targets] of fx.links) assert.deepEqual(findWikiLinks(text).map((m) => m.target), targets, `links in ${JSON.stringify(text)}`)
assert.deepEqual(parseIdLink(' character#12 '), { type: 'character', id: '12' })
assert.equal(parseIdLink('Character#12'), null)
assert.equal(parseIdLink('dragon#3'), null)

const md2 = (s) => renderMarkdown(s, linked)
// A link by id shows the current name, or its own text.
assert.equal(md2('[[character#12]]'), '<p><a class="wikilink" href="/wiki/character/12" data-wiki="character/12">Aria Vale</a></p>')
assert.ok(md2('[[character#12|her]]').includes('>her</a>'))
assert.ok(md2('[[character#12|]]').includes('>Aria Vale</a>'))
assert.ok(md2('[[race#2]]').includes('href="/wiki/race/2"'))
// Gone: a missing link, with its text or the "deleted" label.
assert.equal(md2('[[character#99]]'), '<p><span class="wikilink is-missing" title="gone">gone</span></p>')
assert.equal(md2('[[character#99|Old friend]]'), '<p><span class="wikilink is-missing" title="gone">Old friend</span></p>')
// Typed links still work, and the shown names are escaped.
assert.ok(md2('[[Aria Vale]] [[Ash]]').includes('href="/wiki/character/12"'))
assert.ok(md2('[[gear#4]]').includes('>Sword [old]</a>'))
assert.ok(renderMarkdown('[[location#1]]', makeLinkResolver([{ type: 'location', id: 1, name: '<b>' }])).includes('>&lt;b&gt;</a>'))
// Code holds text, not links.
assert.ok(md2('`[[character#12]]`').includes('<code>[[character#12]]</code>'))
// Without a deletedLabel there is still something to read.
assert.ok(renderMarkdown('[[character#99]]', null).includes('deleted article'))

console.log('markdown: ok')
