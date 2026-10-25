// Script Mode and Tab-to-link in the writing editor (src/editor/script.js,
// nameSuggest.js, schema.js): "Name, manner:" + space makes a dialogue,
// Enter and Backspace around one, the mode attr, the JSON; typing a name's
// start offers it, Tab links it, Esc dismisses. Runs the plugins on a
// stand-in view (no DOM). node frontend/tests/editorScript.test.mjs
import assert from 'node:assert/strict'
import { EditorState, TextSelection } from 'prosemirror-state'
import { schema, docFromJSON, docToJSON } from '../src/editor/schema.js'
import { buildNameIndex } from '../src/editor/nameMatch.js'
import { scriptPlugins, setScriptMode, backspaceDialogue, newLineAfterDialogue, setDialogueAttrs, dialogueToParagraph } from '../src/editor/script.js'
import { nameSuggest, suggestKey, acceptSuggestion } from '../src/editor/nameSuggest.js'
import { countNode, countDocJSON } from '../src/editor/wordCount.js'

const index = buildNameIndex([
  { type: 'character', id: 1, name: 'Aria Vale', nicknames: ['Ash'], aliases: ['Aria'] },
  { type: 'character', id: 2, name: 'Bram Vale' },
  { type: 'location', id: 7, name: 'Thornby' },
  { type: 'event', id: 9, name: 'Thorn Night' },
])
const plugins = [nameSuggest({ getIndex: () => index }), ...scriptPlugins({ getIndex: () => index })]
const p = (text) => ({ type: 'paragraph', content: text ? [{ type: 'text', text }] : undefined })

// A stand-in for EditorView: enough for input rules, key handlers, commands.
function viewOf(json) {
  const view = {
    state: EditorState.create({ schema, doc: docFromJSON(json), plugins }),
    composing: false,
    editable: true,
    dispatch(tr) {
      view.state = view.state.apply(tr)
    },
    focus() {},
  }
  return view
}
const endOf = (view) => {
  view.dispatch(view.state.tr.setSelection(TextSelection.atEnd(view.state.doc)))
  return view
}
// Types text one character at a time, as the browser would (input rules
// first, else the text goes in).
function type(view, text) {
  for (const ch of text) {
    const { from, to } = view.state.selection
    const handled = view.state.plugins.some((pl) => pl.props.handleTextInput?.call(pl, view, from, to, ch, () => view.state.tr.insertText(ch, from, to)))
    if (!handled) view.dispatch(view.state.tr.insertText(ch, from, to))
  }
}
function key(view, name) {
  const e = { key: name, shiftKey: false, ctrlKey: false, metaKey: false, altKey: false, stopPropagation() {}, preventDefault() {} }
  return view.state.plugins.some((pl) => pl.props.handleKeyDown?.call(pl, view, e))
}
const run = (view, cmd) => cmd(view.state, view.dispatch)
const blocks = (view) =>
  view.state.doc.content.content.map((n) => (n.type.name === 'dialogue' ? `D(${n.attrs.speaker}|${n.attrs.manner}|${n.attrs.character ?? '-'}):${n.textContent}` : `P:${n.textContent}`))

// ---- Script Mode ---------------------------------------------------------------
{
  const view = endOf(viewOf({ type: 'doc', content: [p('')] }))
  // Prose: "Name:" stays text.
  type(view, 'Aria: hello')
  assert.deepEqual(blocks(view), ['P:Aria: hello'])

  run(view, setScriptMode(true))
  assert.equal(view.state.doc.attrs.mode, 'script')
  assert.equal(docToJSON(view.state.doc).attrs.mode, 'script')
  // Switching never changes the text.
  assert.deepEqual(blocks(view), ['P:Aria: hello'])

  const fresh = endOf(viewOf({ type: 'doc', attrs: { mode: 'script' }, content: [p('')] }))
  type(fresh, 'Ash, softly: Come here.')
  assert.deepEqual(blocks(fresh), ['D(Ash|softly|1):Come here.'])
  // Enter at the end: an empty paragraph for the next speaker.
  assert.equal(run(fresh, newLineAfterDialogue), true)
  type(fresh, 'A guard: Halt!')
  assert.deepEqual(blocks(fresh), ['D(Ash|softly|1):Come here.', 'D(A guard||-):Halt!'])
  run(fresh, newLineAfterDialogue)
  type(fresh, 'The wind rose.')
  assert.deepEqual(blocks(fresh).at(-1), 'P:The wind rose.')
  // Not a speaker line: a sentence with a colon later on stays a paragraph.
  run(fresh, (s, d) => (d(s.tr.split(s.selection.from)), true))
  type(fresh, 'He said this and that and more again: ')
  assert.equal(blocks(fresh).at(-1), 'P:He said this and that and more again: ')

  // Backspace at a dialogue's start: back to "Speaker, manner: line".
  const first = 0
  fresh.dispatch(fresh.state.tr.setSelection(TextSelection.create(fresh.state.doc, first + 1)))
  assert.equal(run(fresh, backspaceDialogue), true)
  assert.equal(blocks(fresh)[0], 'P:Ash, softly: Come here.')
  assert.equal(fresh.state.selection.from, 1 + 'Ash, softly: '.length)

  // The speaker box's changes, and its "Back to plain text".
  const pos = fresh.state.doc.child(0).nodeSize // the guard's dialogue
  run(fresh, setDialogueAttrs(pos, { manner: 'from the wall', character: 2, speaker: 'Bram Vale' }))
  assert.equal(blocks(fresh)[1], 'D(Bram Vale|from the wall|2):Halt!')
  run(fresh, dialogueToParagraph(pos))
  assert.equal(blocks(fresh)[1], 'P:Bram Vale, from the wall: Halt!')

  // JSON: dialogue attrs written, null character left out; back in the same.
  const v2 = endOf(viewOf({ type: 'doc', attrs: { mode: 'script' }, content: [p('')] }))
  type(v2, 'Nobody: Who? ')
  const json = docToJSON(v2.state.doc)
  assert.deepEqual(json.content[0], { type: 'dialogue', attrs: { speaker: 'Nobody', manner: '' }, content: [{ type: 'text', text: 'Who? ' }] })
  assert.deepEqual(docToJSON(docFromJSON(json)), json)
  // Words: the line only.
  assert.equal(countNode(v2.state.doc), 1)
  assert.equal(countDocJSON(json), 1)
  // Back to prose: the attr goes.
  run(v2, setScriptMode(false))
  assert.equal(docToJSON(v2.state.doc).attrs, undefined)
  assert.deepEqual(blocks(v2), ['D(Nobody||-):Who? '])
}

// ---- Tab-to-link -----------------------------------------------------------------
const active = (view) => suggestKey.getState(view.state).active
{
  const view = endOf(viewOf({ type: 'doc', content: [p('')] }))
  type(view, 'Then Th')
  assert.equal(active(view), null)
  type(view, 'o')
  assert.deepEqual(active(view).matches.map((m) => m.label), ['Thornby', 'Thorn Night'])
  // ↓ picks the next, Tab takes it.
  assert.equal(key(view, 'ArrowDown'), true)
  assert.equal(active(view).selected, 1)
  assert.equal(key(view, 'Tab'), true)
  assert.equal(view.state.doc.textContent, 'Then Thorn Night')
  const linked = view.state.doc.child(0).lastChild
  assert.deepEqual(linked.marks.map((m) => [m.type.name, m.attrs.type, m.attrs.id]), [['story_link', 'event', 9]])
  // Typing on: no suggestion inside or right after the link; plain text.
  type(view, ' and ')
  assert.equal(active(view), null)
  assert.equal(view.state.doc.child(0).lastChild.marks.length, 0)

  // Case and accents don't matter; the name as written goes in.
  type(view, 'aria v')
  assert.deepEqual(active(view).matches.map((m) => m.label), ['Aria Vale'])
  assert.equal(key(view, 'Tab'), true)
  assert.equal(view.state.doc.textContent, 'Then Thorn Night and Aria Vale')

  // A nickname links as itself.
  type(view, ' or As')
  assert.equal(active(view), null)
  type(view, 'h')
  assert.equal(active(view).matches[0].rest, '')
  acceptSuggestion(view)
  const nick = view.state.doc.child(0).lastChild
  assert.equal(nick.text, 'Ash')
  assert.equal(nick.marks[0].attrs.id, 1)

  // Esc dismisses until the word ends.
  type(view, '. Bra')
  assert.ok(active(view))
  assert.equal(key(view, 'Escape'), true)
  assert.equal(active(view), null)
  type(view, 'm')
  assert.equal(active(view), null)
  type(view, ' saw Bra')
  assert.ok(active(view))
  // Moving the caret closes it; Tab without a suggestion isn't taken.
  view.dispatch(view.state.tr.setSelection(TextSelection.create(view.state.doc, 2)))
  assert.equal(active(view), null)
  assert.equal(key(view, 'Tab'), false)
}

// Script Mode: a name at a line's start is a speaker (characters only,
// plain text), then ": " makes the dialogue.
{
  const view = endOf(viewOf({ type: 'doc', attrs: { mode: 'script' }, content: [p('')] }))
  // A place at a line's start is not a speaker: it links as usual (action).
  type(view, 'Tho')
  assert.equal(active(view).speaker, false)
  view.dispatch(view.state.tr.delete(1, view.state.selection.from))
  type(view, 'Bra')
  assert.deepEqual(active(view).matches.map((m) => m.label), ['Bram Vale'])
  assert.equal(active(view).speaker, true)
  key(view, 'Tab')
  assert.equal(view.state.doc.child(0).firstChild.marks.length, 0)
  type(view, ', grimly: Go.')
  assert.deepEqual(blocks(view), ['D(Bram Vale|grimly|2):Go.'])
  // Further along the line a name links as usual.
  type(view, ' To Tho')
  assert.equal(active(view).speaker, false)
}

console.log('editorScript: ok')
