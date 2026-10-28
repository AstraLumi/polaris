// Comments in the editor (stage 5): the ProseMirror side of editor/comments.js.
//   - A new comment's text is held as a "pending" range (drawn like a
//     comment) while its row is made on the server; the range follows the
//     edits made meanwhile, and the mark goes on it once the row has an id
//     (so a mark never points at a comment that doesn't exist).
//   - Pasted text keeps only the comments this chapter knows and that are no
//     longer in its text (a cut and paste moves a comment; a copy doesn't
//     make a second one).
import { Plugin, PluginKey } from 'prosemirror-state'
import { Decoration, DecorationSet } from 'prosemirror-view'
import { Fragment, Slice } from 'prosemirror-model'
import { schema } from './schema.js'
import { keepOnPaste, commentRanges } from './comments.js'

export const commentKey = new PluginKey('comments')

export function commentPlugin({ known }) {
  return new Plugin({
    key: commentKey,
    state: {
      init: () => ({ pending: new Map() }),
      apply(tr, value) {
        const meta = tr.getMeta(commentKey)
        if (!tr.docChanged && !meta) return value
        const pending = new Map()
        for (const [k, r] of value.pending) {
          const from = tr.mapping.map(r.from, 1)
          const to = tr.mapping.map(r.to, -1)
          pending.set(k, { from, to: Math.max(from, to) })
        }
        if (meta?.add) pending.set(meta.add.key, { from: meta.add.from, to: meta.add.to })
        if (meta?.drop != null) pending.delete(meta.drop)
        return { pending }
      },
    },
    props: {
      decorations(state) {
        const { pending } = commentKey.getState(state)
        if (!pending.size) return null
        const decos = [...pending.values()].filter((r) => r.to > r.from).map((r) => Decoration.inline(r.from, r.to, { class: 'pm-comment is-pending' }))
        return DecorationSet.create(state.doc, decos)
      },
      transformPasted(slice, view) {
        const ids = known()
        const inDoc = new Set(commentRanges(view.state.doc).keys())
        return new Slice(stripFragment(slice.content, ids, inDoc), slice.openStart, slice.openEnd)
      },
    },
  })
}

function stripFragment(fragment, known, inDoc) {
  const out = []
  fragment.forEach((node) => {
    if (node.isText) {
      const marks = node.marks.filter((m) => m.type !== schema.marks.comment || keepOnPaste(+m.attrs.id, known, inDoc))
      out.push(marks.length === node.marks.length ? node : node.mark(marks))
    } else out.push(node.copy(stripFragment(node.content, known, inDoc)))
  })
  return Fragment.fromArray(out)
}

let nextKey = 1

// Starts a new comment on from–to: gives the key to finish it with.
export function startPending(view, from, to) {
  const key = nextKey++
  view.dispatch(view.state.tr.setMeta(commentKey, { add: { key, from, to } }).setMeta('addToHistory', false))
  return key
}

// The pending range now (null when its text is gone), dropping it.
export function takePending(view, key) {
  const r = commentKey.getState(view.state)?.pending.get(key) || null
  view.dispatch(view.state.tr.setMeta(commentKey, { drop: key }).setMeta('addToHistory', false))
  return r && r.to > r.from ? r : null
}

// Puts comment id's mark on from–to.
export function addCommentMark(state, from, to, id) {
  return state.tr.addMark(from, to, schema.marks.comment.create({ id }))
}

// Takes comment id's mark off the whole text.
export function removeCommentMark(state, id) {
  const tr = state.tr
  state.doc.descendants((node, pos) => {
    if (!node.isText) return true
    for (const m of node.marks) if (m.type === schema.marks.comment && +m.attrs.id === id) tr.removeMark(pos, pos + node.nodeSize, m)
    return false
  })
  return tr
}
