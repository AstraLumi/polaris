// A footnote or endnote on the writing page (a ProseMirror node view, after
// ProseMirror's footnote example): the note's number in the text (drawn by
// CSS counters, editor.css), and while the note is selected a small box
// under it with the note's own editor. That editor's document is the
// footnote node itself; its steps are passed on to the page's editor
// (shifted to where the note is), so undo, autosave and the word counts
// see them as any other change, and changes made outside (an undo) are
// passed back in. The box sits beside the page's editor, not in it, so
// ProseMirror never sees its DOM. The rules (numbers, insert, kinds) are
// in footnotes.js.
import { EditorState, TextSelection } from 'prosemirror-state'
import { EditorView } from 'prosemirror-view'
import { StepMap } from 'prosemirror-transform'
import { keymap } from 'prosemirror-keymap'
import { baseKeymap, toggleMark } from 'prosemirror-commands'
import { undo, redo } from 'prosemirror-history'
import { inputRules, undoInputRule } from 'prosemirror-inputrules'
import { schema } from './schema.js'
import { typingRules } from './keymap.js'
import { insertHardBreak, insertTab } from './commands.js'
import { NOTE_EDIT, noteAt, noteText, justInserted, setNoteKind, removeNote, pasteIntoNote, noteKind } from './footnotes.js'

// opts: {
//   title(kind, label): the box's title ("Footnote 3"),
//   text: { toEndnote, toFootnote, remove, close } (button labels),
//   readOnly(): true while the text can't be changed,
// }
export function noteNodeView(opts) {
  return (node, view, getPos) => new NoteView(node, view, getPos, opts)
}

class NoteView {
  constructor(node, outer, getPos, opts) {
    this.node = node
    this.outer = outer
    this.getPos = getPos
    this.opts = opts
    this.dom = document.createElement('sup')
    this.dom.className = 'pm-fn-ref'
    this.clicked = false
    this.dom.addEventListener('mousedown', () => (this.clicked = true))
    this.pop = null
    this.inner = null
    this.drawKind()
  }

  drawKind() {
    this.dom.dataset.fn = noteKind(this.node.attrs.kind)
    const text = noteText(this.node).replace(/\s+/g, ' ').trim()
    this.dom.title = text.length > 160 ? `${text.slice(0, 159)}…` : text
  }

  selectNode() {
    this.dom.classList.add('ProseMirror-selectednode')
    const focus = this.clicked || justInserted(this.outer.state) === this.getPos()
    this.clicked = false
    if (!this.inner) this.open()
    if (focus) {
      const inner = this.inner
      requestAnimationFrame(() => {
        if (this.inner !== inner || !inner) return
        inner.focus()
        inner.dispatch(inner.state.tr.setSelection(TextSelection.atEnd(inner.state.doc)))
      })
    }
  }

  deselectNode() {
    this.dom.classList.remove('ProseMirror-selectednode')
    this.close()
  }

  // The box: the note's title, its kind and delete buttons, its editor.
  open() {
    const host = this.outer.dom.parentNode
    if (!host) return
    const o = this.opts
    const pop = document.createElement('div')
    pop.className = 'pm-fn-pop'
    const head = pop.appendChild(document.createElement('div'))
    head.className = 'pm-fn-head'
    this.titleEl = head.appendChild(document.createElement('span'))
    this.titleEl.className = 'pm-fn-title'
    const button = (cls, onClick) => {
      const b = head.appendChild(document.createElement('button'))
      b.type = 'button'
      b.className = cls
      b.addEventListener('mousedown', (e) => e.preventDefault())
      b.addEventListener('click', onClick)
      return b
    }
    this.kindBtn = button('pm-fn-btn', () => this.run(setNoteKind(this.getPos(), this.kind() === 'endnote' ? 'footnote' : 'endnote')))
    this.removeBtn = button('pm-fn-btn', () => {
      this.run(removeNote(this.getPos()))
      this.outer.focus()
    })
    const closeBtn = button('pm-fn-btn pm-fn-close', () => this.leave())
    closeBtn.textContent = '×'
    closeBtn.title = o.text?.close || ''
    closeBtn.setAttribute('aria-label', o.text?.close || '')
    const mount = pop.appendChild(document.createElement('div'))
    host.appendChild(pop)
    this.pop = pop
    this.inner = new EditorView(mount, {
      state: EditorState.create({
        doc: this.node,
        plugins: [
          inputRules({ rules: typingRules }),
          keymap({
            'Mod-z': (st, d) => undoInputRule(st, d) || undo(this.outer.state, this.outer.dispatch),
            'Mod-y': () => redo(this.outer.state, this.outer.dispatch),
            'Shift-Mod-z': () => redo(this.outer.state, this.outer.dispatch),
            'Mod-b': toggleMark(schema.marks.bold),
            'Mod-i': toggleMark(schema.marks.italic),
            'Mod-u': toggleMark(schema.marks.underline),
            'Shift-Enter': insertHardBreak,
            Tab: insertTab,
            'Mod-Enter': () => this.leave(),
            Escape: () => this.leave(),
          }),
          keymap(baseKeymap),
        ],
      }),
      dispatchTransaction: (tr) => this.dispatchInner(tr),
      editable: () => !o.readOnly?.(),
      attributes: { class: 'write-doc pm-fn-body', spellcheck: 'true' },
      transformPasted: pasteIntoNote,
    })
    this.drawBox()
    this.place()
  }

  kind() {
    return noteKind(this.node.attrs.kind)
  }

  drawBox() {
    if (!this.pop) return
    const o = this.opts
    const at = noteAt(this.outer.state.doc, this.getPos())
    this.pop.dataset.fn = this.kind()
    this.titleEl.textContent = o.title ? o.title(this.kind(), at?.label ?? '') : at?.label ?? ''
    this.kindBtn.textContent = this.kind() === 'endnote' ? o.text?.toFootnote || 'Footnote' : o.text?.toEndnote || 'Endnote'
    this.removeBtn.textContent = o.text?.remove || 'Delete'
    const ro = !!o.readOnly?.()
    this.kindBtn.disabled = ro
    this.removeBtn.disabled = ro
  }

  // Under the number, inside the page's width (in the page's own units:
  // the sheet may be zoomed, offsets aren't).
  place() {
    const host = this.pop?.parentNode
    if (!host) return
    let x = 0
    let y = 0
    for (let el = this.dom; el && el !== host; el = el.offsetParent) {
      x += el.offsetLeft
      y += el.offsetTop
    }
    const width = Math.min(this.pop.offsetWidth || 0, host.clientWidth)
    this.pop.style.top = `${y + this.dom.offsetHeight + 4}px`
    this.pop.style.left = `${Math.max(0, Math.min(x - 24, host.clientWidth - width))}px`
  }

  run(cmd) {
    if (!this.opts.readOnly?.()) cmd(this.outer.state, this.outer.dispatch)
  }

  // Back to the text, after the note's number.
  leave() {
    const pos = this.getPos() + this.node.nodeSize
    this.outer.dispatch(this.outer.state.tr.setSelection(TextSelection.create(this.outer.state.doc, pos)))
    this.outer.focus()
    return true
  }

  close() {
    if (this.inner) {
      this.inner.destroy()
      this.inner = null
    }
    if (this.pop) {
      this.pop.remove()
      this.pop = null
    }
  }

  // The note's editor changed its text: the same steps in the page's
  // editor, shifted to where the note is.
  dispatchInner(tr) {
    const { state, transactions } = this.inner.state.applyTransaction(tr)
    this.inner.updateState(state)
    if (tr.getMeta('fromOutside')) return
    const outerTr = this.outer.state.tr
    const offset = StepMap.offset(this.getPos() + 1)
    for (const t of transactions) for (const step of t.steps) outerTr.step(step.map(offset))
    if (outerTr.docChanged) this.outer.dispatch(outerTr.setMeta(NOTE_EDIT, true))
  }

  update(node) {
    if (node.type !== this.node.type) return false
    this.node = node
    this.drawKind()
    if (this.inner) {
      const state = this.inner.state
      const start = node.content.findDiffStart(state.doc.content)
      if (start != null) {
        let { a: endA, b: endB } = node.content.findDiffEnd(state.doc.content)
        const overlap = start - Math.min(endA, endB)
        if (overlap > 0) {
          endA += overlap
          endB += overlap
        }
        this.inner.dispatch(state.tr.replace(start, endB, node.slice(start, endA)).setMeta('fromOutside', true))
      }
      this.drawBox()
    }
    return true
  }

  destroy() {
    this.close()
  }

  stopEvent() {
    return false
  }

  ignoreMutation() {
    return true
  }
}
