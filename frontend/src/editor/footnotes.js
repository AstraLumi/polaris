// Footnotes and endnotes (stage 5; docs/EDITOR.md, "Footnotes and
// endnotes"): the `footnote` node's numbering, inserting, changing and
// removing a note, and the plugin that keeps a note's text out of what is
// done to the text around it. Pure (ProseMirror nodes and transactions, no
// view); the note's editor is noteView.js.
//
// Numbers are never stored: footnotes count 1, 2, 3… and endnotes i, ii,
// iii… (Word's defaults), each kind on its own, in the order of the
// chapter's text, starting again in each chapter.
import { Plugin, PluginKey, NodeSelection } from 'prosemirror-state'
import { AddMarkStep, RemoveMarkStep } from 'prosemirror-transform'
import { Fragment, Slice } from 'prosemirror-model'
import { schema } from './schema.js'

export const NOTE_KINDS = ['footnote', 'endnote']
export const noteKind = (k) => (k === 'endnote' ? 'endnote' : 'footnote')

// Lower-case roman numerals (endnotes); past 3999, the number.
export function roman(n) {
  if (!(n > 0 && n < 4000)) return String(n)
  const vals = [1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1]
  const syms = ['m', 'cm', 'd', 'cd', 'c', 'xc', 'l', 'xl', 'x', 'ix', 'v', 'iv', 'i']
  let out = ''
  vals.forEach((v, i) => {
    while (n >= v) {
      out += syms[i]
      n -= v
    }
  })
  return out
}

// How note n of a kind is shown: "3" or "iii".
export const noteLabel = (kind, n) => (noteKind(kind) === 'endnote' ? roman(n) : String(n))

// Every note of a document (a ProseMirror node), in the text's order:
// [{ pos, node, kind, n, label }].
export function noteList(doc) {
  const out = []
  const count = { footnote: 0, endnote: 0 }
  doc.descendants((node, pos) => {
    if (node.type !== schema.nodes.footnote) return true
    const kind = noteKind(node.attrs.kind)
    const n = ++count[kind]
    out.push({ pos, node, kind, n, label: noteLabel(kind, n) })
    return false
  })
  return out
}

// The note at pos ({ kind, n, label }), or null.
export const noteAt = (doc, pos) => noteList(doc).find((x) => x.pos === pos) || null

// The same for stored JSON: [{ kind, n, label, content }] (content: the
// note's paragraphs as JSON).
export function notesInJSON(json) {
  const out = []
  const count = { footnote: 0, endnote: 0 }
  const walk = (n) => {
    if (!n || typeof n !== 'object') return
    if (n.type === 'footnote') {
      const kind = noteKind(n.attrs?.kind)
      const i = ++count[kind]
      out.push({ kind, n: i, label: noteLabel(kind, i), content: Array.isArray(n.content) ? n.content : [] })
      return
    }
    if (Array.isArray(n.content)) n.content.forEach(walk)
  }
  walk(json)
  return out
}

// A note's text: its paragraphs, one per line (line breaks as new lines,
// tabs as spaces).
export function noteText(node) {
  const lines = []
  node.forEach((p) => {
    let s = ''
    p.forEach((c) => (s += c.isText ? c.text : c.type.name === 'hard_break' ? '\n' : ' '))
    lines.push(s)
  })
  return lines.join('\n')
}

// ---- Commands ------------------------------------------------------------------

// The meta a transaction carries when it comes from a note's own editor
// (its steps are inside the note).
export const NOTE_EDIT = 'polarisNoteEdit'
const openKey = new PluginKey('polarisOpenNote')

// Where a new note goes: after the selection (as Word puts the reference
// after the selected text), in text. Null where none can go (a picture or
// another block selected).
function insertPos(state) {
  const sel = state.selection
  if (sel instanceof NodeSelection && sel.node.type !== schema.nodes.footnote) return null
  const $to = sel.$to
  if (!$to.parent.inlineContent) return null
  if (!$to.parent.canReplaceWith($to.index(), $to.index(), schema.nodes.footnote)) return null
  return sel.to
}

export const canInsertNote = (state) => insertPos(state) != null

// Inserts an empty note of a kind after the selection and selects it (the
// writing page then opens its text for typing).
export const insertNote = (kind) => (state, dispatch) => {
  const at = insertPos(state)
  if (at == null) return false
  if (dispatch) {
    const node = schema.nodes.footnote.create({ kind: noteKind(kind) }, schema.nodes.paragraph.create())
    const tr = state.tr.insert(at, node)
    tr.setSelection(NodeSelection.create(tr.doc, at)).setMeta(openKey, at).scrollIntoView()
    dispatch(tr)
  }
  return true
}

// Makes the note at pos a footnote or an endnote.
export const setNoteKind = (pos, kind) => (state, dispatch) => {
  const node = state.doc.nodeAt(pos)
  if (!node || node.type !== schema.nodes.footnote) return false
  if (dispatch) dispatch(state.tr.setNodeMarkup(pos, null, { ...node.attrs, kind: noteKind(kind) }))
  return true
}

// Deletes the note at pos (its number and its text).
export const removeNote = (pos) => (state, dispatch) => {
  const node = state.doc.nodeAt(pos)
  if (!node || node.type !== schema.nodes.footnote) return false
  if (dispatch) dispatch(state.tr.delete(pos, pos + node.nodeSize).scrollIntoView())
  return true
}

// The position of the note just inserted (to open its editor), or null.
export const justInserted = (state) => openKey.getState(state) ?? null

// ---- Keeping notes apart ----------------------------------------------------------

// A note's content as a paste may hold it: no notes inside, no comments.
export function cleanNoteContent(fragment) {
  const out = []
  fragment.forEach((node) => {
    if (node.type === schema.nodes.footnote) return
    if (node.isText) {
      out.push(node.mark(node.marks.filter((m) => m.type !== schema.marks.comment)))
      return
    }
    out.push(node.copy(cleanNoteContent(node.content)))
  })
  return Fragment.from(out)
}

// Every note in a fragment cleaned (what is pasted into the text).
function cleanNotesIn(fragment) {
  const out = []
  fragment.forEach((node) => {
    if (node.type === schema.nodes.footnote) out.push(node.copy(cleanNoteContent(node.content)))
    else if (node.isLeaf) out.push(node)
    else out.push(node.copy(cleanNotesIn(node.content)))
  })
  return Fragment.from(out)
}

// What is pasted into a note: notes taken out, comments too.
export const pasteIntoNote = (slice) => new Slice(cleanNoteContent(slice.content), slice.openStart, slice.openEnd)

// The plugin on the writing page's editor:
//  - formatting or a comment put on a stretch of text that holds a note
//    reaches the note's number only, never its text (ProseMirror would mark
//    the text inside too): the note's text is put back as it was, in the
//    same undo step;
//  - pasted notes hold no notes and no comments;
//  - remembers the note just inserted (justInserted) for its editor.
export function notesPlugin() {
  return new Plugin({
    key: openKey,
    state: {
      init: () => null,
      apply: (tr, prev) => tr.getMeta(openKey) ?? (tr.getMeta('appendedTransaction') ? prev : null),
    },
    props: {
      transformPasted: (slice) => new Slice(cleanNotesIn(slice.content), slice.openStart, slice.openEnd),
    },
    appendTransaction(trs, _old, state) {
      const fixes = new Map() // the note's position now → its text before
      trs.forEach((tr, ti) => {
        if (tr.getMeta(NOTE_EDIT)) return
        tr.steps.forEach((step, si) => {
          if (!(step instanceof AddMarkStep || step instanceof RemoveMarkStep)) return
          tr.docs[si].nodesBetween(step.from, step.to, (node, pos) => {
            if (node.type !== schema.nodes.footnote) return true
            if (pos >= step.from && pos + node.nodeSize <= step.to) {
              let at = tr.mapping.slice(si).map(pos, 1)
              for (const later of trs.slice(ti + 1)) at = later.mapping.map(at, 1)
              if (!fixes.has(at)) fixes.set(at, node.content)
            }
            return false
          })
        })
      })
      if (!fixes.size) return null
      const tr = state.tr
      for (const [at, content] of fixes) {
        const node = tr.doc.nodeAt(at)
        if (!node || node.type !== schema.nodes.footnote || node.content.eq(content)) continue
        tr.replaceWith(at + 1, at + node.nodeSize - 1, content)
      }
      return tr.docChanged ? tr : null
    },
  })
}
