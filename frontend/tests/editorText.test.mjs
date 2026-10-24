// Word counts, find and replace, change case and the editor's commands
// (src/editor/wordCount.js, find.js, textCase.js, commands.js), on real
// ProseMirror states without a view. node frontend/tests/editorText.test.mjs
import assert from 'node:assert/strict'
import { EditorState, TextSelection } from 'prosemirror-state'
import { schema, docFromJSON, docToJSON } from '../src/editor/schema.js'
import { countText, countDocJSON, countNode } from '../src/editor/wordCount.js'
import { findInText, findMatches, replaceMatches, nextMatchIndex, searchRegExp } from '../src/editor/find.js'
import { toCase, caseOf, nextCase } from '../src/editor/textCase.js'
import {
  setParaAttrs,
  setStyle,
  insertBlock,
  changeCase,
  stepFontSize,
  setMark,
  selectionFormat,
  splitParagraph,
  toggleList,
  setLink,
  clearFormatting,
  pasteIntoEmpty,
} from '../src/editor/commands.js'
import { typingRules } from '../src/editor/keymap.js'
import { builtinStyles } from '../src/editor/styles.js'

// ---- Word counts (EDITOR.md: split on whitespace, runs with a letter or digit) ----
assert.equal(countText(''), 0)
assert.equal(countText('Hello, world!'), 2)
assert.equal(countText('— well — 42 … ok'), 3, 'dashes and dots alone are not words')
assert.equal(countText("it's a well-known fact"), 4)
assert.equal(countText('  São   Paulo\tñandú '), 3)
assert.equal(countText('日本語'), 1)
assert.equal(countText('² ½ x'), 1, 'digits as Go counts them (Nd)')

const json = {
  type: 'doc',
  content: [
    { type: 'heading', attrs: { level: 1 }, content: [{ type: 'text', text: 'Chapter One' }] },
    { type: 'paragraph', content: [{ type: 'text', text: 'one' }, { type: 'hard_break' }, { type: 'text', text: 'two' }, { type: 'tab' }, { type: 'text', text: 'three' }] },
    { type: 'scene_break' },
    { type: 'paragraph', content: [{ type: 'text', text: 'four ' }, { type: 'text', text: 'fi', marks: [{ type: 'bold' }] }, { type: 'text', text: 've' }] },
    { type: 'bullet_list', content: [{ type: 'list_item', content: [{ type: 'paragraph', content: [{ type: 'text', text: 'six seven' }] }] }] },
    { type: 'paragraph' },
  ],
}
assert.equal(countDocJSON(json), 9, 'a line break and a tab split words; marks do not')
const doc = docFromJSON(json)
assert.equal(countNode(doc), 9, 'the same on a ProseMirror node')
assert.equal(countDocJSON({ type: 'doc', content: [{ type: 'paragraph', content: [{ type: 'text', text: 'end' }] }, { type: 'paragraph', content: [{ type: 'text', text: 'start' }] }] }), 2, 'blocks never join')

// ---- Find -------------------------------------------------------------------------------
assert.deepEqual(findInText('The cat, the Cathedral.', 'the'), [[0, 3], [9, 12], [15, 18]])
assert.deepEqual(findInText('The cat, the Cathedral.', 'the', { matchCase: true }), [[9, 12], [15, 18]])
assert.deepEqual(findInText('The cat, the Cathedral.', 'the', { matchCase: true, wholeWord: true }), [[9, 12]])
assert.deepEqual(findInText('The cat, the Cathedral.', 'cat', { wholeWord: true }), [[4, 7]])
assert.deepEqual(findInText('ação e Ação', 'ação', { wholeWord: true }), [[0, 4], [7, 11]])
assert.deepEqual(findInText('a.b a+b', 'a.b'), [[0, 3]], 'no regular expressions')
assert.deepEqual(findInText('x', ''), [])
assert.equal(searchRegExp(''), null)

const fdoc = docFromJSON({
  type: 'doc',
  content: [
    { type: 'paragraph', content: [{ type: 'text', text: 'Ana met ' }, { type: 'text', text: 'Ana', marks: [{ type: 'italic' }] }, { type: 'text', text: 'belle.' }] },
    { type: 'paragraph', content: [{ type: 'text', text: 'ana' }, { type: 'hard_break' }, { type: 'text', text: 'Ana' }] },
  ],
})
const ms = findMatches(fdoc, 'ana')
assert.equal(ms.length, 4)
assert.deepEqual(ms.map((m) => fdoc.textBetween(m.from, m.to)), ['Ana', 'Ana', 'ana', 'Ana'], 'across marks, and after a line break')
assert.equal(findMatches(fdoc, 'ana', { wholeWord: true }).length, 3, '"Anabelle" is not the word Ana')
assert.equal(findMatches(fdoc, 'ana', { matchCase: true }).length, 1)
assert.equal(nextMatchIndex(ms, 0), 0)
assert.equal(nextMatchIndex(ms, ms[1].from), 1)
assert.equal(nextMatchIndex(ms, 1e9), 0, 'wraps round')
assert.equal(nextMatchIndex(ms, ms[1].from, true), 0)
assert.equal(nextMatchIndex([], 0), -1)

let st = EditorState.create({ schema, doc: fdoc })
const rtr = replaceMatches(st.tr, findMatches(fdoc, 'ana', { wholeWord: true }), 'Bea')
assert.equal(rtr.doc.textBetween(0, rtr.doc.content.size, '|', '/'), 'Bea met Anabelle.|Bea/Bea')
const all = replaceMatches(st.tr, findMatches(fdoc, 'ana'), 'Bea')
assert.deepEqual(docToJSON(all.doc).content[0].content[1], { type: 'text', text: 'Bea', marks: [{ type: 'italic' }] }, 'the replacement keeps the formatting')
assert.equal(replaceMatches(st.tr, findMatches(fdoc, 'Ana', { matchCase: true }), '').doc.textContent, ' met belle.ana', 'replacing with nothing deletes')

// ---- Change case ------------------------------------------------------------------------
assert.equal(toCase('hello wORLD', 'title'), 'Hello World')
assert.equal(toCase("it's o'neil", 'title'), "It's O'neil")
assert.equal(toCase('llo', 'title', 'e'), 'llo', 'mid-word stays lower')
assert.equal(caseOf('hello'), 'lower')
assert.equal(caseOf('HELLO'), 'upper')
assert.equal(caseOf('Hello There'), 'title')
assert.equal(caseOf('hELLo'), 'mixed')
assert.equal(nextCase('hello'), 'upper')
assert.equal(nextCase('HELLO'), 'title')
assert.equal(nextCase('Hello'), 'lower')
assert.equal(nextCase('hELLo'), 'lower')

// ---- Commands -------------------------------------------------------------------------------
const styles = builtinStyles(8)
const run = (state, cmd) => {
  let next = state
  const ok = cmd(state, (tr) => (next = state.apply(tr)))
  return { ok, state: next }
}
const make = (content, from, to = from) => {
  const d = docFromJSON({ type: 'doc', content })
  return EditorState.create({ schema, doc: d, selection: TextSelection.create(d, from, to) })
}
const para = (text, attrs) => ({ type: 'paragraph', attrs, content: text ? [{ type: 'text', text }] : undefined })

// Paragraph attrs on every selected paragraph; null goes back to the style.
st = make([para('one'), para('two'), para('three')], 2, 7)
st = run(st, setParaAttrs({ align: 'center', indentFirst: 0 })).state
assert.deepEqual(docToJSON(st.doc).content.map((p) => p.attrs), [{ align: 'center', indentFirst: 0 }, { align: 'center', indentFirst: 0 }, undefined])
st = run(st, setParaAttrs({ align: null })).state
assert.deepEqual(docToJSON(st.doc).content[0].attrs, { indentFirst: 0 })

// Styles: Heading 1 makes a heading and clears the paragraph's own formatting.
st = make([para('Title here', { align: 'right', spaceAfter: 3 })], 2)
st = run(st, setStyle('heading1')).state
assert.deepEqual(docToJSON(st.doc).content[0], { type: 'heading', attrs: { level: 1 }, content: [{ type: 'text', text: 'Title here' }] })
st = run(st, setStyle('quote')).state
assert.deepEqual(docToJSON(st.doc).content[0].attrs, { style: 'quote' })
assert.equal(docToJSON(st.doc).content[0].type, 'paragraph')
st = run(st, setStyle('normal')).state
assert.equal(docToJSON(st.doc).content[0].attrs, undefined)

// The toolbar's reading of the selection.
st = make([para('abc', { style: 'quote' })], 2)
let f = selectionFormat(st, styles)
assert.equal(f.styleId, 'quote')
assert.equal(f.italic, true, 'from the style')
assert.equal(f.size, 8)
assert.equal(f.align, 'justify')
assert.equal(f.indentLeft, 28)

// Sizes step through Word's list.
st = make([para('abc')], 1, 4)
st = run(st, stepFontSize(1, styles)).state
assert.equal(selectionFormat(st, styles).size, 9)
st = run(st, stepFontSize(1, styles)).state
st = run(st, stepFontSize(1, styles)).state
assert.equal(selectionFormat(st, styles).size, 10.5)
st = run(st, stepFontSize(-1, styles)).state
assert.equal(selectionFormat(st, styles).size, 10)

// Marks with attrs replace each other; null takes them off.
st = make([para('abc')], 1, 4)
st = run(st, setMark('font', { family: 'Lora' })).state
st = run(st, setMark('font', { family: 'Turret Road' })).state
assert.deepEqual(docToJSON(st.doc).content[0].content[0].marks, [{ type: 'font', attrs: { family: 'Turret Road' } }])
st = run(st, setMark('font', null)).state
assert.equal(docToJSON(st.doc).content[0].content[0].marks, undefined)

// A scene break splits the paragraph and puts the cursor after it.
st = make([para('before after')], 7)
st = run(st, insertBlock('scene_break')).state
assert.deepEqual(docToJSON(st.doc).content.map((n) => n.type), ['paragraph', 'scene_break', 'paragraph'])
assert.equal(st.doc.child(0).textContent, 'before')
assert.equal(st.doc.child(2).textContent, ' after')
assert.equal(st.selection.$from.parent, st.doc.child(2))
// At the end of the document a paragraph follows, with the same formatting.
st = make([para('last', { align: 'center' })], 5)
st = run(st, insertBlock('page_break')).state
assert.deepEqual(docToJSON(st.doc).content.map((n) => n.type), ['paragraph', 'page_break', 'paragraph'])
assert.deepEqual(docToJSON(st.doc).content[2].attrs, { align: 'center' })

// Enter keeps the paragraph's formatting; after a heading comes Normal.
st = make([para('one two', { align: 'center', style: 'quote' })], 4)
st = run(st, splitParagraph).state
assert.deepEqual(docToJSON(st.doc).content.map((p) => p.attrs), [{ style: 'quote', align: 'center' }, { style: 'quote', align: 'center' }])
st = make([{ type: 'heading', attrs: { level: 1 }, content: [{ type: 'text', text: 'Head' }] }], 5)
st = run(st, splitParagraph).state
assert.deepEqual(docToJSON(st.doc).content.map((p) => p.type), ['heading', 'paragraph'])
assert.equal(docToJSON(st.doc).content[1].attrs, undefined)

// Change case cycles and keeps marks; with no selection it takes the word.
st = make([{ type: 'paragraph', content: [{ type: 'text', text: 'big ' }, { type: 'text', text: 'red', marks: [{ type: 'bold' }] }, { type: 'text', text: ' dog' }] }], 1, 12)
st = run(st, changeCase).state
assert.equal(st.doc.textContent, 'BIG RED DOG')
st = run(st, changeCase).state
assert.equal(st.doc.textContent, 'Big Red Dog')
assert.deepEqual(docToJSON(st.doc).content[0].content[1], { type: 'text', text: 'Red', marks: [{ type: 'bold' }] })
st = run(st, changeCase).state
assert.equal(st.doc.textContent, 'big red dog')
st = make([para('one word here')], 7)
st = run(st, changeCase).state
assert.equal(st.doc.textContent, 'one WORD here')

// Lists on, switched, off.
st = make([para('item')], 2)
st = run(st, toggleList('bullet_list')).state
assert.equal(docToJSON(st.doc).content[0].type, 'bullet_list')
st = run(st, toggleList('ordered_list')).state
assert.equal(docToJSON(st.doc).content[0].type, 'ordered_list')
st = run(st, toggleList('ordered_list')).state
assert.equal(docToJSON(st.doc).content[0].type, 'paragraph')

// Links: on a selection, or typed in with nothing selected; empty takes it off.
st = make([para('see here')], 5, 9)
st = run(st, setLink('https://example.org')).state
assert.deepEqual(docToJSON(st.doc).content[0].content[1], { type: 'text', text: 'here', marks: [{ type: 'link', attrs: { href: 'https://example.org' } }] })
st = run(st, setLink('')).state
assert.equal(docToJSON(st.doc).content[0].content.length, 1)

// Clear formatting keeps the style and links.
st = make([{ type: 'paragraph', attrs: { style: 'quote', align: 'center' }, content: [{ type: 'text', text: 'x', marks: [{ type: 'bold' }, { type: 'link', attrs: { href: '#a' } }] }] }], 1, 2)
st = run(st, clearFormatting).state
assert.deepEqual(docToJSON(st.doc).content[0], { type: 'paragraph', attrs: { style: 'quote' }, content: [{ type: 'text', text: 'x', marks: [{ type: 'link', attrs: { href: '#a' } }] }] })

// Pasting into an empty paragraph takes the pasted paragraph's formatting.
{
  const src = docFromJSON({ type: 'doc', content: [para('Pasted', { align: 'center', indentFirst: 0 }), para('two', { style: 'quote' })] })
  const slice = src.slice(1, src.content.size - 1)
  st = make([para('before'), para(), para('after')], 9)
  const tr = pasteIntoEmpty(st, slice)
  const out = docToJSON(tr.doc).content
  assert.deepEqual(out.map((p) => p.attrs), [undefined, { align: 'center', indentFirst: 0 }, { style: 'quote' }, undefined])
  assert.deepEqual(out.map((p) => p.content?.[0].text), ['before', 'Pasted', 'two', 'after'])
  assert.equal(pasteIntoEmpty(make([para('full')], 2), slice), null, 'into text: as usual')
}

// ---- Typing helpers ---------------------------------------------------------------------------
const typed = (before, ch) => {
  const text = before + ch
  for (const rule of typingRules) {
    const m = rule.match.exec(text)
    if (m) {
      const s = make([para(before || undefined)], before.length + 1)
      const tr = rule.handler(s, m, before.length + 1 - (m[0].length - 1), before.length + 1)
      return tr ? tr.doc.textContent : text
    }
  }
  return text
}
assert.equal(typed('wait-', '-'), 'wait—')
assert.equal(typed('so..', '.'), 'so…')
assert.equal(typed('she said ', '"'), 'she said “')
assert.equal(typed('—', '"'), '—“', 'a quote after a dash opens')
assert.equal(typed('“hi', '"'), '“hi”')
assert.equal(typed('don', "'"), 'don’')
assert.equal(typed('', "'"), '‘')

console.log('editorText: ok')
