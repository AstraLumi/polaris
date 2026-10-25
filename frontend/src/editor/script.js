// Script Mode (docs/EDITOR.md, stage 4). A chapter's doc root carries
// `mode: 'script'`; switching never changes the text. In Script Mode,
// "Name:" or "Name, manner:" typed at the start of a paragraph becomes a
// dialogue once a space follows (the speaker is found among the characters
// by name, nickname or former name; anyone else stays a plain speaker).
// In both modes: Enter at the end of a dialogue starts an empty paragraph
// (ready for the next "Name:"), Backspace at its start turns it back into
// the paragraph "Name, manner: line". Ctrl+Z right after a conversion
// undoes it. Tab-completing a speaker is nameSuggest.js's.
import { InputRule, inputRules } from 'prosemirror-inputrules'
import { keymap } from 'prosemirror-keymap'
import { Plugin, TextSelection } from 'prosemirror-state'
import { schema } from './schema.js'
import { parseSpeakerLine, resolveSpeaker, speakerLead } from './nameMatch.js'

const { dialogue, paragraph } = schema.nodes

export const isScript = (state) => state.doc.attrs.mode === 'script'

// Prose or script for the chapter.
export const setScriptMode = (on) => (state, dispatch) => {
  if (dispatch) dispatch(state.tr.setDocAttribute('mode', on ? 'script' : null))
  return true
}

// "Name, manner:" + space at a top-level paragraph's start.
function speakerRule(getIndex) {
  return new InputRule(/^([^\n:\ufffc]{1,262}:)\s$/u, (state, match, start, end) => {
    if (!isScript(state)) return null
    const $start = state.doc.resolve(start)
    if ($start.parent.type !== paragraph || $start.depth !== 1 || start !== $start.start()) return null
    const parsed = parseSpeakerLine(match[1])
    if (!parsed) return null
    const who = resolveSpeaker(getIndex() || [], parsed.speaker)
    const para = $start.parent
    const at = $start.before()
    const line = para.content.cut(end - $start.start())
    const node = dialogue.create({ character: who ? who.id : null, speaker: parsed.speaker, manner: parsed.manner }, line)
    const tr = state.tr.replaceWith(at, at + para.nodeSize, node)
    return tr.setSelection(TextSelection.create(tr.doc, at + 1))
  })
}

// Enter at the end of a dialogue: an empty paragraph after it.
export const newLineAfterDialogue = (state, dispatch) => {
  const { $from, empty } = state.selection
  if (!empty || $from.parent.type !== dialogue || $from.parentOffset !== $from.parent.content.size) return false
  if (dispatch) {
    const at = $from.after()
    const tr = state.tr.insert(at, paragraph.create())
    dispatch(tr.setSelection(TextSelection.create(tr.doc, at + 1)).scrollIntoView())
  }
  return true
}

// A dialogue back to a paragraph "Speaker, manner: line", the cursor where
// the line starts (so Backspace again goes on deleting as usual).
export const dialogueToParagraph = (pos) => (state, dispatch) => {
  const node = state.doc.nodeAt(pos)
  if (!node || node.type !== dialogue) return false
  if (dispatch) {
    const lead = speakerLead(node.attrs)
    const content = lead ? schema.text(lead) : null
    const para = paragraph.create(null, content ? [content, ...contentOf(node)] : contentOf(node))
    const tr = state.tr.replaceWith(pos, pos + node.nodeSize, para)
    dispatch(tr.setSelection(TextSelection.create(tr.doc, pos + 1 + lead.length)).scrollIntoView())
  }
  return true
}

const contentOf = (node) => {
  const out = []
  node.content.forEach((c) => out.push(c))
  return out
}

export const backspaceDialogue = (state, dispatch) => {
  const { $from, empty } = state.selection
  if (!empty || $from.parent.type !== dialogue || $from.parentOffset !== 0) return false
  return dialogueToParagraph($from.before())(state, dispatch)
}

// Changes a dialogue's speaker, character and manner.
export const setDialogueAttrs = (pos, patch) => (state, dispatch) => {
  const node = state.doc.nodeAt(pos)
  if (!node || node.type !== dialogue) return false
  if (dispatch) dispatch(state.tr.setNodeMarkup(pos, null, { ...node.attrs, ...patch }))
  return true
}

// The dialogue whose speaker was clicked, or null.
function dialoguePosOf(view, target) {
  const who = target.closest?.('.pm-who')
  if (!who || !view.dom.contains(who)) return null
  const line = who.parentElement?.querySelector('.pm-line')
  if (!line) return null
  try {
    const $pos = view.state.doc.resolve(view.posAtDOM(line, 0))
    return $pos.parent.type === dialogue ? $pos.before() : null
  } catch (e) {
    return null
  }
}

// options.getIndex(): the name index (nameMatch.js); options.onSpeaker(pos,
// element): a dialogue's speaker was clicked (the page opens its options).
export function scriptPlugins({ getIndex, onSpeaker }) {
  return [
    inputRules({ rules: [speakerRule(getIndex)] }),
    keymap({ Enter: newLineAfterDialogue, Backspace: backspaceDialogue }),
    new Plugin({
      props: {
        handleDOMEvents: {
          mousedown(view, e) {
            if (e.button !== 0) return false
            const pos = dialoguePosOf(view, e.target)
            if (pos == null) return false
            e.preventDefault()
            if (!view.editable) return true
            // The cursor goes to the line's start, the options open by it.
            const tr = view.state.tr.setSelection(TextSelection.create(view.state.doc, pos + 1))
            view.dispatch(tr)
            onSpeaker?.(pos, e.target.closest('.pm-who'))
            return true
          },
        },
      },
    }),
  ]
}
