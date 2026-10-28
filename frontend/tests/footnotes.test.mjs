// Footnotes and endnotes in the writing editor (src/editor/footnotes.js,
// the footnote node in schema.js): the stored JSON in and out, numbering,
// inserting and changing notes, formatting around a note leaving its text
// alone (and undo), pastes, word counts, and the page view's CSS and
// contents.
// node frontend/tests/footnotes.test.mjs
import assert from 'node:assert/strict'
import { EditorState, TextSelection, NodeSelection } from 'prosemirror-state'
import { undo, history } from 'prosemirror-history'
import { toggleMark } from 'prosemirror-commands'
import { DOMSerializer, Slice, Fragment } from 'prosemirror-model'
import { schema, docFromJSON, docToJSON } from '../src/editor/schema.js'
import { noteList, noteAt, notesInJSON, noteLabel, roman, noteText, insertNote, canInsertNote, setNoteKind, removeNote, notesPlugin, justInserted, cleanNoteContent, pasteIntoNote, NOTE_EDIT } from '../src/editor/footnotes.js'
import { countDocJSON, countNode } from '../src/editor/wordCount.js'
import { wordKeymap } from '../src/editor/keymap.js'
import { pagedCSS, tocEntries, notesCSS } from '../src/editor/bookLayout.js'
import { noteSize } from '../src/editor/css.js'
import { styleSheet, builtinStyles } from '../src/editor/styles.js'
import { collectSentences } from '../src/editor/readAloud.js'
import { findMatches } from '../src/editor/find.js'

const txt = (text, ...marks) => (marks.length ? { type: 'text', text, marks: marks.map((m) => ({ type: m })) } : { type: 'text', text })
const para = (...content) => ({ type: 'paragraph', ...(content.length ? { content } : {}) })
const note = (kind, ...paras) => ({ type: 'footnote', attrs: { kind }, content: paras.length ? paras : [para()] })
const docJSON = (...blocks) => ({ type: 'doc', content: blocks })

// ---- Roman numerals and labels -----------------------------------------------------
assert.equal(roman(1), 'i')
assert.equal(roman(4), 'iv')
assert.equal(roman(9), 'ix')
assert.equal(roman(14), 'xiv')
assert.equal(roman(1990), 'mcmxc')
assert.equal(roman(0), '0')
assert.equal(noteLabel('footnote', 3), '3')
assert.equal(noteLabel('endnote', 3), 'iii')
assert.equal(noteLabel('nonsense', 2), '2', 'an unknown kind is a footnote')

// ---- The schema and the stored JSON -----------------------------------------------
const fn = schema.nodes.footnote
assert.ok(fn, 'the footnote node')
assert.ok(fn.isInline && fn.isAtom, 'an inline atom')
assert.ok(schema.nodes.paragraph.contentMatch.matchType(fn), 'in paragraphs')
assert.ok(schema.nodes.heading.contentMatch.matchType(fn), 'in headings')
assert.ok(schema.nodes.dialogue.contentMatch.matchType(fn), 'in dialogue')
assert.ok(fn.contentMatch.matchType(schema.nodes.paragraph), 'holds paragraphs')
assert.ok(!fn.contentMatch.matchType(schema.nodes.heading), 'no headings in a note')
assert.ok(!fn.contentMatch.matchType(schema.nodes.bullet_list), 'no lists in a note')

const stored = docJSON(
  para(txt('He crossed the river'), note('footnote', para(txt('The Vel, in spring.'))), txt(' at dawn.')),
  para(txt('Then '), note('endnote', para(txt('See the map.')), para(txt('Drawn later.', 'italic'))), txt('rest'), note('footnote', para(txt('Second.')))),
)
const doc = docFromJSON(stored)
assert.deepEqual(docToJSON(doc), stored, 'stored JSON comes back the same (kind always written)')
assert.deepEqual(docToJSON(docFromJSON(docJSON(para(txt('a'), { type: 'footnote', content: [para(txt('x'))] })))).content[0].content[1].attrs, { kind: 'footnote' }, 'a missing kind reads as a footnote and is written')
// Unknown attrs are kept.
const odd = docJSON(para(txt('a'), { type: 'footnote', attrs: { kind: 'endnote', custom: 7 }, content: [para(txt('x'))] }))
assert.deepEqual(docToJSON(docFromJSON(odd)), odd)

// Marks never stay on a note's number (or a line break): the server only
// takes marks on text.
{
  const st = EditorState.create({ schema, doc })
  const tr = st.tr.addMark(1, st.doc.content.size - 1, schema.marks.bold.create())
  const out = docToJSON(tr.doc)
  const walk = (n) => {
    if (n.type !== 'text') assert.equal(n.marks, undefined, `${n.type} has no marks`)
    ;(n.content || []).forEach(walk)
  }
  walk(out)
}
{
  const hb = docFromJSON(docJSON(para(txt('ab'), { type: 'hard_break' }, txt('cd'))))
  const st = EditorState.create({ schema, doc: hb })
  const out = docToJSON(st.tr.addMark(1, hb.content.size - 1, schema.marks.bold.create()).doc)
  assert.equal(out.content[0].content[1].marks, undefined, 'a bold line break is stored without marks')
}

// ---- Numbering ---------------------------------------------------------------------------
const list = noteList(doc)
assert.deepEqual(
  list.map((x) => [x.kind, x.n, x.label]),
  [
    ['footnote', 1, '1'],
    ['endnote', 1, 'i'],
    ['footnote', 2, '2'],
  ],
  'footnotes and endnotes counted apart, in the order of the text',
)
assert.equal(noteAt(doc, list[2].pos).label, '2')
assert.equal(noteAt(doc, 0), null)
assert.deepEqual(
  notesInJSON(stored).map((x) => x.label),
  ['1', 'i', '2'],
)
assert.equal(notesInJSON(stored)[1].content.length, 2)
assert.equal(noteText(list[1].node), 'See the map.\nDrawn later.')
// In a table cell and a list too, in reading order.
{
  const cellDoc = docFromJSON(
    docJSON(
      para(txt('a'), note('footnote', para(txt('one')))),
      { type: 'bullet_list', content: [{ type: 'list_item', content: [para(txt('b'), note('footnote', para(txt('two'))))] }] },
      { type: 'table', content: [{ type: 'table_row', content: [{ type: 'table_cell', content: [para(txt('c'), note('footnote', para(txt('three'))))] }] }] },
    ),
  )
  assert.deepEqual(noteList(cellDoc).map((x) => noteText(x.node)), ['one', 'two', 'three'])
}

// ---- Inserting --------------------------------------------------------------------------
const plugins = [notesPlugin(), history()]
const at = (json, pos) => {
  const d = docFromJSON(json)
  return EditorState.create({ schema, doc: d, plugins, selection: TextSelection.create(d, pos) })
}
const run = (state, cmd) => {
  let next = state
  const ok = cmd(state, (tr) => (next = state.apply(tr)))
  return { ok, state: next }
}
{
  // After "Hello" (pos 6).
  const st = at(docJSON(para(txt('Hello world'))), 6)
  assert.ok(canInsertNote(st))
  const { ok, state } = run(st, insertNote('footnote'))
  assert.ok(ok)
  const out = docToJSON(state.doc)
  assert.deepEqual(out.content[0].content, [txt('Hello'), { type: 'footnote', attrs: { kind: 'footnote' }, content: [{ type: 'paragraph' }] }, txt(' world')])
  assert.ok(state.selection instanceof NodeSelection && state.selection.node.type === fn, 'the new note is selected')
  assert.equal(justInserted(state), 6, 'remembered for its editor to open')
  // The next transaction forgets it.
  assert.equal(justInserted(state.apply(state.tr)), null)
}
{
  // A selection: the note goes after it, the text stays.
  const d = docFromJSON(docJSON(para(txt('Hello world'))))
  const st = EditorState.create({ schema, doc: d, plugins, selection: TextSelection.create(d, 1, 6) })
  const { state } = run(st, insertNote('endnote'))
  const out = docToJSON(state.doc)
  assert.equal(out.content[0].content[0].text, 'Hello')
  assert.equal(out.content[0].content[1].attrs.kind, 'endnote')
}
{
  // Not on a picture or a scene break selected.
  const d = docFromJSON(docJSON(para(txt('a')), { type: 'scene_break', attrs: { glyph: '* * *' } }, para(txt('b'))))
  const st = EditorState.create({ schema, doc: d, plugins, selection: NodeSelection.create(d, 3) })
  assert.ok(!canInsertNote(st))
  // A note selected: the new one goes right after it.
  const d2 = docFromJSON(docJSON(para(txt('a'), note('footnote', para(txt('x'))), txt('b'))))
  const st2 = EditorState.create({ schema, doc: d2, plugins, selection: NodeSelection.create(d2, 2) })
  const { state } = run(st2, insertNote('footnote'))
  assert.deepEqual(noteList(state.doc).map((x) => noteText(x.node)), ['x', ''])
}
// Ctrl+Alt+F / Ctrl+Alt+D (Word's).
{
  const keys = wordKeymap(() => builtinStyles())
  assert.ok(keys['Mod-Alt-f'] && keys['Mod-Alt-d'])
  const { state } = run(at(docJSON(para(txt('Hi'))), 3), keys['Mod-Alt-d'])
  assert.equal(noteList(state.doc)[0].kind, 'endnote')
}

// ---- Kind and removal ----------------------------------------------------------------
{
  const st = EditorState.create({ schema, doc, plugins })
  const pos = noteList(doc)[0].pos
  const { state } = run(st, setNoteKind(pos, 'endnote'))
  assert.deepEqual(noteList(state.doc).map((x) => x.label), ['i', 'ii', '1'], 'numbers follow the kinds')
  const { state: s2 } = run(state, removeNote(pos))
  assert.deepEqual(noteList(s2.doc).map((x) => x.label), ['i', '1'])
  assert.equal(s2.doc.textContent.includes('The Vel'), false, 'its text goes with it')
  assert.ok(!run(st, removeNote(1)).ok, 'not a note there')
}

// ---- Formatting around a note leaves its text alone ----------------------------------
{
  const st = EditorState.create({ schema, doc, plugins })
  const all = TextSelection.create(st.doc, 1, st.doc.content.size - 1)
  const { state } = run(st.apply(st.tr.setSelection(all)), toggleMark(schema.marks.bold))
  const notes = noteList(state.doc)
  for (const n of notes) {
    n.node.descendants((c) => {
      if (c.isText) assert.ok(!schema.marks.bold.isInSet(c.marks), `note text "${c.text}" not bold`)
    })
  }
  // The text around it is.
  assert.ok(schema.marks.bold.isInSet(state.doc.firstChild.firstChild.marks))
  // The italics the note had are still there.
  assert.ok(notes[1].node.lastChild.firstChild.marks.some((m) => m.type === schema.marks.italic))
  // One undo takes the whole thing back.
  const back = run(state, undo).state
  assert.deepEqual(docToJSON(back.doc), stored)
}
{
  // A change inside a note (its own editor's steps) is kept.
  const st = EditorState.create({ schema, doc, plugins })
  const n = noteList(st.doc)[0]
  const from = n.pos + 2 // inside the note's paragraph
  const tr = st.tr.addMark(from, from + 3, schema.marks.bold.create()).setMeta(NOTE_EDIT, true)
  const next = st.apply(tr)
  const marked = noteList(next.doc)[0].node.firstChild.firstChild
  assert.ok(schema.marks.bold.isInSet(marked.marks), 'bold inside the note stays')
  // Its undo too (no NOTE_EDIT meta on the undo, and the step is inside the note).
  const undone = run(next, undo).state
  assert.ok(!schema.marks.bold.isInSet(noteList(undone.doc)[0].node.firstChild.firstChild.marks), 'and undoing it works')
}

// ---- Pastes ---------------------------------------------------------------------------
{
  const comment = schema.marks.comment.create({ id: 4 })
  const inner = schema.nodes.footnote.create({ kind: 'footnote' }, schema.nodes.paragraph.create(null, schema.text('deep')))
  const content = Fragment.from(schema.nodes.paragraph.create(null, [schema.text('note', [comment]), inner]))
  const clean = cleanNoteContent(content)
  assert.equal(clean.firstChild.childCount, 1, 'no note inside a note')
  assert.equal(clean.firstChild.firstChild.marks.length, 0, 'no comments in a note')
  const pasted = pasteIntoNote(new Slice(content, 1, 1))
  assert.equal(pasted.content.firstChild.textContent, 'note')
  // Pasted into the text: notes kept, their insides cleaned.
  const outer = Fragment.from(schema.nodes.paragraph.create(null, [schema.text('a'), schema.nodes.footnote.create({ kind: 'footnote' }, content)]))
  const out = notesPlugin().props.transformPasted(new Slice(outer, 1, 1))
  const n = out.content.firstChild.child(1)
  assert.equal(n.type, schema.nodes.footnote)
  assert.equal(n.firstChild.childCount, 1)
}
// Our own HTML (a copy in the editor) carries the note.
{
  const ser = DOMSerializer.fromSchema(schema)
  const spec = schema.nodes.footnote.spec.toDOM(noteList(doc)[1].node)
  assert.equal(spec[0], 'sup')
  assert.equal(spec[1]['data-fn'], 'endnote')
  const content = JSON.parse(spec[1]['data-note'])
  assert.equal(content.length, 2)
  const rule = schema.nodes.footnote.spec.parseDOM[0]
  const fake = { getAttribute: (k) => (k === 'data-note' ? spec[1]['data-note'] : k === 'data-fn' ? 'endnote' : null) }
  assert.deepEqual(rule.getAttrs(fake), { kind: 'endnote' })
  const back = rule.getContent(fake, schema)
  assert.ok(back.eq(noteList(doc)[1].node.content), 'the note comes back from data-note')
  const broken = { getAttribute: () => '{not json' }
  assert.equal(rule.getContent(broken, schema).childCount, 1, 'anything else is an empty note')
  assert.ok(typeof ser.serializeNode === 'function')
}

// ---- Word counts --------------------------------------------------------------------
{
  // "He crossed the river" (4) "The Vel, in spring." (4) "at dawn." (2) |
  // "Then" (1) "See the map." (3) "Drawn later." (2) "rest" (1) "Second." (1)
  assert.equal(countDocJSON(stored), 18, 'notes count')
  assert.equal(countNode(doc), 18, 'the same in the editor')
  // A note between two words keeps them apart, and alone in a paragraph.
  assert.equal(countDocJSON(docJSON(para(txt('one'), note('footnote', para(txt('two'))), txt('three')))), 3)
  assert.equal(countDocJSON(docJSON(para(note('footnote', para(txt('two words')))))), 2)
  const d = docFromJSON(docJSON(para(txt('one'), note('footnote', para(txt('two'))), txt('three'))))
  assert.equal(countNode(d), 3)
  // A selection holding the note counts its words.
  assert.equal(countNode(d, 1, d.content.size - 1), 3)
  assert.equal(countNode(d, 1, 4), 1, 'just "one"')
}

// ---- Find and read aloud skip the notes ---------------------------------------------
assert.equal(findMatches(doc, 'map').length, 0, 'find looks in the text, not in notes')
assert.equal(findMatches(doc, 'river').length, 1)
{
  const said = collectSentences(doc).map((s) => s.text).join(' | ')
  assert.ok(!said.includes('Vel'), 'read aloud skips notes')
  assert.ok(said.includes('He crossed the river at dawn.'), said)
}

// ---- Page view -------------------------------------------------------------------------
assert.equal(noteSize(12), 10)
assert.equal(noteSize(10.5), 9)
assert.equal(noteSize(8), 6.5)
{
  const css = notesCSS({ font: 'Merriweather', size: 12, color: '#000' })
  assert.ok(css.some((r) => r.includes('.pv-fn.pv-footnote { float: footnote; }')))
  assert.ok(css.some((r) => r.includes('font-size: 10pt') && r.includes('"Merriweather"')))
  assert.ok(css.some((r) => r.includes('@footnote')))
  const all = pagedCSS({ page: { width_mm: 148, height_mm: 210 } }, { font: 'Lora', size: 10.5 })
  assert.ok(all.includes('float: footnote'), 'pagedCSS has the notes')
}
// A note in a heading isn't in its title in the contents.
{
  const chapters = [{ id: 1, title: 'One', doc: docJSON({ type: 'heading', attrs: { level: 1 }, content: [txt('The Ford'), note('footnote', para(txt('A real place.')))] }) }]
  assert.deepEqual(tocEntries(chapters, [{}], 1).map((e) => e.text), ['The Ford'])
}
// The book's stylesheet sizes notes.
{
  const css = styleSheet(builtinStyles(), '.write-doc')
  assert.ok(/\.write-doc\.pm-fn-body p, \.write-doc \.pv-note \{ font-size: [\d.]+pt; text-indent: 0/.test(css), css)
}

console.log('footnotes: all tests passed')
