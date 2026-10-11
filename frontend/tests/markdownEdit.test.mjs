// The Markdown toolbar's text edits. No dependencies: node frontend/tests/markdownEdit.test.mjs
import assert from 'node:assert/strict'
import { wrapInline, toggleLines, codeBlock, table, rule, webLink, searchArticles, wikiLinkMarkup } from '../src/markdownEdit.js'
import { renderMarkdown } from '../src/markdown.js'

// Apply an edit the way the editor does; returns [text, selected text].
function run(text, from, to, fn, ...args) {
  const e = fn(text, from, to, ...args)
  const out = text.slice(0, e.from) + e.insert + text.slice(e.to)
  return [out, out.slice(e.selStart, e.selEnd)]
}
const all = (text, fn, ...args) => run(text, 0, text.length, fn, ...args)

// Inline: wrap, unwrap, placeholder, edge spaces stay outside.
assert.deepEqual(run('a word here', 2, 6, wrapInline, '**', 'bold'), ['a **word** here', 'word'])
assert.deepEqual(run('a **word** here', 4, 8, wrapInline, '**', 'bold'), ['a word here', 'word'])
assert.deepEqual(run('a **word** here', 2, 10, wrapInline, '**', 'bold'), ['a word here', 'word'])
assert.deepEqual(run('ab', 1, 1, wrapInline, '*', 'italic'), ['a*italic*b', 'italic'])
assert.deepEqual(run('a word here', 1, 7, wrapInline, '~~', 'x'), ['a ~~word~~ here', 'word'])

// Lines: add to each line, take off again, numbered counts up.
assert.equal(all('one\ntwo', toggleLines, 'bullet')[0], '- one\n- two')
assert.equal(all('- one\n- two', toggleLines, 'bullet')[0], 'one\ntwo')
assert.equal(all('one\n\ntwo', toggleLines, 'numbered')[0], '1. one\n\n2. two')
assert.equal(all('- one', toggleLines, 'numbered')[0], '1. one')
assert.equal(run('intro\ntitle\nmore', 8, 8, toggleLines, 'heading')[0], 'intro\n# title\nmore')
assert.equal(run('# title', 3, 3, toggleLines, 'heading')[0], 'title')
assert.equal(all('a\nb', toggleLines, 'quote')[0], '> a\n> b')
assert.equal(run('a\nb\n', 0, 2, toggleLines, 'quote')[0], '> a\nb\n')
assert.equal(run('', 0, 0, toggleLines, 'bullet')[0], '- ')

// Blocks get a blank line around them so they render as blocks.
assert.deepEqual(run('text', 4, 4, codeBlock, 'code'), ['text\n\n```\ncode\n```', 'code'])
assert.equal(run('a\nb', 2, 3, codeBlock, 'code')[0], 'a\n\n```\nb\n```')
assert.ok(renderMarkdown(run('Intro', 5, 5, table, 'Column', 'Cell')[0]).includes('<table>'))
assert.equal(run('Intro', 5, 5, table, 'Column', 'Cell')[1], 'Column')
assert.equal(run('a\n\nb', 2, 2, rule)[0], 'a\n\n---\n\nb')
assert.ok(renderMarkdown(run('a', 1, 1, rule)[0]).includes('<hr />'))

// Web links select the address.
assert.deepEqual(run('see here', 4, 8, webLink, 'link'), ['see [here](https://)', 'https://'])

// Searching the wiki.
const items = [
  { type: 'character', id: 1, name: 'Aldo' },
  { type: 'location', id: 7, name: 'Aldoria' },
  { type: 'race', id: 2, name: 'Old Aldorian' },
  { type: 'event', id: 3, name: 'Fall of Aldoria' },
  { type: 'race', id: 4, name: 'Élan' },
  { type: 'character', id: 5, name: 'Dragon' },
  { type: 'race', id: 6, name: 'Dragon' },
]
assert.deepEqual(searchArticles(items, 'aldo').map((x) => x.id), [1, 7, 3, 2])
assert.deepEqual(searchArticles(items, 'ALDORIA').map((x) => x.id), [7, 3, 2])
assert.deepEqual(searchArticles(items, 'elan').map((x) => x.id), [4])
assert.deepEqual(searchArticles(items, '  '), [])
assert.equal(searchArticles(items, 'a', 2).length, 2)

// The inserted link leads to the article that was picked.
const resolve = (inner) => {
  const [a, b] = inner.includes(':') ? inner.split(':') : ['', inner]
  return items.find((x) => x.name.toLowerCase() === b.trim().toLowerCase() && (!a || x.type === a)) || null
}
assert.equal(wikiLinkMarkup(items[1], '', resolve), '[[Aldoria]]')
assert.equal(wikiLinkMarkup(items[1], 'aldoria', resolve), '[[Aldoria]]')
assert.equal(wikiLinkMarkup(items[1], 'the city', resolve), '[[Aldoria|the city]]')
assert.equal(wikiLinkMarkup(items[1], 'a [b]\nc', resolve), '[[Aldoria|a b c]]')
assert.equal(wikiLinkMarkup(items[5], '', resolve), '[[Dragon]]')
assert.equal(wikiLinkMarkup(items[6], '', resolve), '[[race:Dragon]]')
assert.ok(renderMarkdown(wikiLinkMarkup(items[6], 'wyrms', resolve), resolve).includes('href="/wiki/race/6"'))
assert.ok(renderMarkdown(wikiLinkMarkup(items[6], 'wyrms', resolve), resolve).includes('>wyrms</a>'))

console.log('markdownEdit: ok')
