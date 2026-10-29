// "Tag character" (docs/EDITOR.md, stage 5): the speaker mark says who said
// a stretch of prose without Script Mode. Commands, the right-click menu and
// the faint drawing (with a "Said by" hover title) live here; the page shows
// the picker (components/write/SpeakerTag.vue).
import { Plugin } from 'prosemirror-state'
import { Decoration, DecorationSet } from 'prosemirror-view'
import { schema } from './schema.js'
import { markRange } from './commands.js'

const speakerType = schema.marks.speaker

// The selection without the spaces around it, or null when no text is left.
export function taggableRange(state) {
  const { from, to, empty } = state.selection
  if (empty) return null
  const isSpace = (pos) => /\s/.test(state.doc.textBetween(pos, pos + 1, '\n', '\n') || ' ')
  let a = from
  let b = to
  while (a < b && isSpace(a)) a++
  while (b > a && isSpace(b - 1)) b--
  return a < b ? { from: a, to: b } : null
}

// Tags the selection (or `range`) as said by { id, name }: a character, or
// a name that isn't one (id null). Replaces another tag in the range.
export const tagSpeaker = (who, range) => (state, dispatch) => {
  const r = range || taggableRange(state)
  if (!r || !who || (!who.name && who.id == null)) return false
  if (dispatch) {
    const tr = state.tr.removeMark(r.from, r.to, speakerType)
    tr.addMark(r.from, r.to, speakerType.create({ character: who.id ?? null, speaker: who.name || '' }))
    dispatch(tr.scrollIntoView())
  }
  return true
}

// The tag around a position: { from, to, attrs } or null.
export function speakerAt(state, pos) {
  const $pos = state.doc.resolve(pos)
  const has = (ms) => ms.find((m) => m.type === speakerType)
  const mark = has($pos.marks()) || ($pos.nodeAfter && has($pos.nodeAfter.marks))
  if (!mark) return null
  const r = markRange($pos, speakerType)
  return r ? { ...r, attrs: mark.attrs } : null
}

export const removeSpeaker = (range) => (state, dispatch) => {
  if (!range) return false
  if (dispatch) dispatch(state.tr.removeMark(range.from, range.to, speakerType))
  return true
}

// options.onMenu({ x, y, range, tag }): a right-click on the selection
// (range: what to tag, trimmed; tag: the existing one there) or inside a tag
// with nothing selected (range null). options.label(name): the hover title.
export function speakerPlugin({ onMenu, label = (name) => `Said by ${name}` }) {
  return new Plugin({
    props: {
      decorations(state) {
        const out = []
        state.doc.descendants((node, pos) => {
          if (!node.isText) return
          const m = node.marks.find((x) => x.type === speakerType)
          if (m) out.push(Decoration.inline(pos, pos + node.nodeSize, { title: label(m.attrs.speaker || '?') }))
        })
        return out.length ? DecorationSet.create(state.doc, out) : DecorationSet.empty
      },
      // Ctrl/Cmd+Alt+K: the same picker for the selection (or the tag the
      // caret is in).
      handleKeyDown(view, e) {
        if (!(e.ctrlKey || e.metaKey) || !e.altKey || e.shiftKey || e.key.toLowerCase() !== 'k') return false
        const range = taggableRange(view.state)
        const tag = range ? speakerAt(view.state, range.from) : speakerAt(view.state, view.state.selection.from)
        if (!range && !tag) return false
        const c = view.coordsAtPos((range || tag).to)
        e.preventDefault()
        onMenu({ x: c.left, y: c.bottom, range: range || null, tag, direct: true })
        return true
      },
      handleDOMEvents: {
        contextmenu(view, e) {
          const at = view.posAtCoords({ left: e.clientX, top: e.clientY })
          const sel = view.state.selection
          const range = taggableRange(view.state)
          if (range && at && at.pos >= sel.from && at.pos <= sel.to) {
            e.preventDefault()
            onMenu({ x: e.clientX, y: e.clientY, range, tag: speakerAt(view.state, range.from) })
            return true
          }
          // A misspelled word with nothing selected keeps the spelling menu.
          if (at && sel.empty && !e.target?.closest?.('.spell-wrong')) {
            const tag = speakerAt(view.state, at.pos)
            if (tag && at.pos > tag.from && at.pos < tag.to) {
              e.preventDefault()
              onMenu({ x: e.clientX, y: e.clientY, range: null, tag })
              return true
            }
          }
          return false
        },
      },
    },
  })
}
