// Nested lists and pictures in the writing editor (src/editor/schema.js,
// keymap.js, commands.js): Tab and Shift+Tab move a list item one level in
// and out, Enter on an empty item takes it out one level, nine levels is the
// most; a picture inserted at the cursor is selected, its options change its
// attrs, and it can be removed. node frontend/tests/editorLists.test.mjs
import assert from 'node:assert/strict'
import { EditorState, TextSelection, NodeSelection } from 'prosemirror-state'
import { liftListItem, splitListItem } from 'prosemirror-schema-list'
import { chainCommands } from 'prosemirror-commands'
import { schema, docFromJSON, docToJSON } from '../src/editor/schema.js'
import { indentListItem, outdentEmptyListItem, listLevels, MAX_LIST_LEVELS, wordKeymap } from '../src/editor/keymap.js'
import { insertBlock, setImageAttrs, removeImage, selectionFormat } from '../src/editor/commands.js'
import { builtinStyles } from '../src/editor/styles.js'
import { countNode } from '../src/editor/wordCount.js'

const li = schema.nodes.list_item
const styles = builtinStyles(10)
const p = (text) => ({ type: 'paragraph', content: text ? [{ type: 'text', text }] : undefined })
const item = (text, ...more) => ({ type: 'list_item', content: [p(text), ...more] })
const bullets = (...items) => ({ type: 'bullet_list', content: items })
const numbers = (...items) => ({ type: 'ordered_list', content: items })
const stateOf = (...blocks) => EditorState.create({ schema, doc: docFromJSON({ type: 'doc', content: blocks }) })

// Where the text is, and the cursor there.
function at(state, text, offset = 0) {
  let pos = null
  state.doc.descendants((node, nodePos) => {
    if (pos == null && node.isTextblock && node.textContent === text) pos = nodePos + 1 + offset
  })
  assert.notEqual(pos, null, `no paragraph "${text}"`)
  return state.apply(state.tr.setSelection(TextSelection.create(state.doc, pos)))
}
function run(state, cmd) {
  let next = null
  const handled = cmd(state, (tr) => (next = state.apply(tr)))
  return handled && next ? next : handled ? state : null
}
// A list as text: [item [nested] item], # for numbered.
function shape(node) {
  if (node.type.name === 'bullet_list' || node.type.name === 'ordered_list') {
    return (node.type.name === 'ordered_list' ? '#' : '') + '[' + node.children.map(shape).join(' ') + ']'
  }
  if (node.type.name === 'list_item') return node.children.map(shape).join(' ')
  return node.textContent || '_'
}
const shapeOf = (state) => state.doc.children.map(shape).join(' ')

// ---- The schema -----------------------------------------------------------------
assert.equal(li.spec.content, 'paragraph block*')
const nested = docFromJSON({ type: 'doc', content: [bullets(item('one', numbers(item('one-a'), item('one-b'))), item('two'))] })
nested.check()
assert.deepEqual(docToJSON(nested).content[0].content[0].content[1].type, 'ordered_list', 'a list inside an item survives a round trip')
assert.equal(countNode(nested), 4, 'words are counted in nested items')
// An old document (items of paragraphs only) still reads.
docFromJSON({ type: 'doc', content: [bullets({ type: 'list_item', content: [p('a'), p('b')] })] }).check()

// ---- Tab and Shift+Tab ------------------------------------------------------------
const flat = stateOf(bullets(item('a'), item('b'), item('c')))
let s = run(at(flat, 'b'), indentListItem(li))
assert.equal(shapeOf(s), '[a [b] c]', 'Tab nests the item under the one before it')
s = run(at(s, 'c'), indentListItem(li))
assert.equal(shapeOf(s), '[a [b] [c]]'.replace('[b] [c]', '[b c]'), 'c joins b one level in')
s = run(at(s, 'c'), indentListItem(li))
assert.equal(shapeOf(s), '[a [b [c]]]')
assert.equal(listLevels(at(s, 'c').selection.$from), 3)
s = run(at(s, 'c'), liftListItem(li))
assert.equal(shapeOf(s), '[a [b c]]', 'Shift+Tab takes it a level out')
s = run(at(s, 'c'), liftListItem(li))
assert.equal(shapeOf(s), '[a [b] c]')

// The first item can't go in; Tab at its start does nothing, in the text it types a tab.
const first = at(flat, 'a')
assert.equal(shapeOf(run(first, indentListItem(li))), '[a b c]')
assert.equal(run(at(flat, 'a', 1), indentListItem(li)), null, 'not at the start: left to the tab key')
// Out of a list, Tab isn't ours.
assert.equal(run(at(stateOf(p('plain')), 'plain'), indentListItem(li)), null)

// Nine levels at most: sink one item as far as it goes, giving it a sibling above each time.
let stairs = stateOf(bullets(item('a'), item('b')))
for (let i = 0; i < 20; i++) {
  // after each Tab the item that was b is still the last one; give it a sibling above to sink under
  const before = stairs
  stairs = run(at(stairs, 'b'), indentListItem(li)) || stairs
  if (stairs === before) break
  if (listLevels(at(stairs, 'b').selection.$from) >= MAX_LIST_LEVELS) break
  // add a new line above b at its level, so b can sink again
  const { $from } = at(stairs, 'b').selection
  const tr = stairs.tr.insert($from.before($from.depth - 1), li.create(null, schema.nodes.paragraph.create(null, schema.text('pad'))))
  stairs = stairs.apply(tr)
}
assert.equal(listLevels(at(stairs, 'b').selection.$from), MAX_LIST_LEVELS)
const capped = run(at(stairs, 'b'), indentListItem(li))
assert.ok(capped, 'past the last level Tab is swallowed (no tab typed)')
assert.ok(capped.doc.eq(stairs.doc), 'and changes nothing')

// ---- Enter on an empty item ------------------------------------------------------------
const enter = chainCommands(outdentEmptyListItem(li), splitListItem(li))
const withEmpty = stateOf(bullets(item('a', bullets(item('b'), item(''))), item('c')))
s = run(at(withEmpty, ''), enter)
assert.equal(shapeOf(s), '[a [b] _ c]', 'an empty nested item comes out one level')
s = run(at(s, ''), enter)
assert.equal(shapeOf(s), '[a [b]] _ [c]', 'and out of the list from the first level')
// A non-empty item splits as usual.
s = run(at(withEmpty, 'b', 1), enter)
assert.equal(shapeOf(s), '[a [b _ _] c]', 'splitting keeps the nesting')
// An empty item that has a list under it is split, not lifted.
const parent = stateOf(bullets(item('', bullets(item('kid')))))
assert.equal(outdentEmptyListItem(li)(at(parent, ''), null), false)

// The keymap wires them to Tab, Shift-Tab and Enter.
const keys = wordKeymap(() => styles)
const tabbed = run(at(flat, 'b'), keys.Tab)
assert.equal(shapeOf(tabbed), '[a [b] c]')
assert.equal(shapeOf(run(at(tabbed, 'b'), keys['Shift-Tab'])), '[a b c]')

// ---- Pictures ----------------------------------------------------------------------------
const base = stateOf(p('before'), p('after'))
let pic = run(at(base, 'before', 6), insertBlock('image', { src: '/uploads/editor/a.png' }, { select: true }))
assert.ok(pic.selection instanceof NodeSelection && pic.selection.node.type.name === 'image', 'the new picture is selected')
assert.deepEqual(pic.doc.children.map((n) => n.type.name), ['paragraph', 'image', 'paragraph'])
assert.equal(pic.selection.node.attrs.align, 'center')
assert.equal(pic.selection.node.attrs.width, null)
assert.equal(selectionFormat(pic, styles).atom, 'image')

// In the middle of a paragraph it is split; in a list it goes after the list.
pic = run(at(base, 'before', 3), insertBlock('image', { src: '/uploads/editor/a.png' }, { select: true }))
assert.deepEqual(pic.doc.children.map((n) => n.type.name + ':' + (n.textContent || '')), ['paragraph:bef', 'image:', 'paragraph:ore', 'paragraph:after'])
pic = run(at(stateOf(bullets(item('a'), item('b'))), 'b', 1), insertBlock('image', { src: '/uploads/editor/a.png' }, { select: true }))
assert.deepEqual(pic.doc.children.map((n) => n.type.name), ['bullet_list', 'image', 'paragraph'])
assert.ok(pic.selection instanceof NodeSelection)

// Its options.
let opt = run(pic, setImageAttrs({ width: 180, align: 'right' }))
assert.equal(opt.selection.node.attrs.width, 180)
assert.equal(opt.selection.node.attrs.align, 'right')
assert.equal(opt.selection.node.attrs.src, '/uploads/editor/a.png', 'the rest stays')
assert.ok(opt.selection instanceof NodeSelection, 'still selected')
opt = run(opt, setImageAttrs({ width: null, fullPage: true, alt: 'A map' }))
assert.deepEqual(
  [opt.selection.node.attrs.width, opt.selection.node.attrs.fullPage, opt.selection.node.attrs.alt],
  [null, true, 'A map'],
)
// Stored without the empty ones.
assert.deepEqual(docToJSON(opt.doc).content[1], { type: 'image', attrs: { src: '/uploads/editor/a.png', alt: 'A map', align: 'right', fullPage: true, wrap: 'inline' } })
// Only on a selected picture.
assert.equal(setImageAttrs({ width: 5 })(at(base, 'before'), null), false)
assert.equal(removeImage(at(base, 'before'), null), false)
// Remove.
const gone = run(opt, removeImage)
assert.deepEqual(gone.doc.children.map((n) => n.type.name), ['bullet_list', 'paragraph'])

console.log('editorLists: ok')
