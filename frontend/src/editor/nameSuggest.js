// Tab-to-link (docs/EDITOR.md, stage 4): while a word is typed that starts
// a story item's name (or a character's nickname or former name), the rest
// shows in faint text after the caret; Tab puts the whole name in as a
// story_link. With several matches a small list opens under the caret
// (↑/↓ to pick, Tab or Enter to take one, a click too). Esc, moving the
// caret or typing anything that isn't the name closes it. In Script Mode a
// name typed at the start of a line is a speaker: only characters are
// offered and the name goes in as plain text, ready for "Name:".
//
// What matches is worked out by nameMatch.js (pure); this is the editor
// part: the text before the caret, the ghost and list (widget decorations,
// so they scroll and zoom with the page) and the keys.
import { Plugin, PluginKey, TextSelection } from 'prosemirror-state'
import { Decoration, DecorationSet } from 'prosemirror-view'
import { schema } from './schema.js'
import { findMatches } from './nameMatch.js'

export const suggestKey = new PluginKey('nameSuggest')

const OBJ = '￼'
const { link, story_link: storyLink } = schema.marks

// The textblock's text before and after the caret, one character per
// position: linked text and inline atoms are OBJ, so no name is looked for
// across or inside a link.
function textAround($pos) {
  const parent = $pos.parent
  let all = ''
  parent.forEach((child) => {
    if (child.isText) all += child.marks.some((m) => m.type === link || m.type === storyLink) ? OBJ.repeat(child.text.length) : child.text
    else all += OBJ.repeat(child.nodeSize)
  })
  return { before: all.slice(0, $pos.parentOffset), after: all.slice($pos.parentOffset) }
}

// The suggestion for the caret now, or null.
function suggestionAt(state, getIndex) {
  const { selection } = state
  if (!selection.empty || !(selection instanceof TextSelection)) return null
  const $pos = selection.$from
  if (!$pos.parent.isTextblock || $pos.parent.type.spec.code) return null
  if ($pos.marks().some((m) => m.type === link || m.type === storyLink)) return null
  const index = getIndex()
  if (!index || !index.length) return null
  const { before, after } = textAround($pos)
  // Script Mode: a name at the start of a paragraph is a speaker.
  const scriptLine = state.doc.attrs.mode === 'script' && $pos.parent.type === schema.nodes.paragraph && $pos.depth === 1
  let found = null
  let speaker = false
  if (scriptLine) {
    found = findMatches(index, before, after, { types: ['character'] })
    speaker = !!found && found.start === 0
    if (!speaker) found = null
  }
  if (!found) found = findMatches(index, before, after)
  if (!found) return null
  const from = $pos.start() + found.start
  return { from, to: $pos.pos, typed: found.typed, matches: found.matches, selected: 0, speaker }
}

// options.getIndex(): the name index (buildNameIndex) or null.
// options.typeLabel(type): the item type's name, for the list.
export function nameSuggest({ getIndex, typeLabel = (t) => t, hint = '' }) {
  return new Plugin({
    key: suggestKey,
    state: {
      init: () => ({ active: null, dismissed: null }),
      apply(tr, prev, oldState, newState) {
        const meta = tr.getMeta(suggestKey)
        let dismissed = prev.dismissed == null ? null : tr.mapping.map(prev.dismissed)
        if (meta?.select != null && prev.active) return { ...prev, active: { ...prev.active, selected: meta.select } }
        if (meta?.dismiss) return { active: null, dismissed: prev.active ? tr.mapping.map(prev.active.from) : dismissed }
        if (!tr.docChanged) return tr.selectionSet ? { active: null, dismissed: null } : prev
        if (meta?.accepted != null) dismissed = meta.accepted
        // Pasted, dropped or undone text gets no suggestion.
        if (tr.getMeta('paste') || tr.getMeta('uiEvent') === 'drop' || tr.getMeta('history$')) return { active: null, dismissed: null }
        const active = suggestionAt(newState, getIndex)
        if (!active || active.from === dismissed) return { active: null, dismissed: active ? dismissed : null }
        // Keep the picked one while it still matches.
        const was = prev.active && prev.active.matches[prev.active.selected]
        const keep = was ? active.matches.findIndex((m) => m.type === was.type && m.id === was.id && m.label === was.label) : -1
        return { active: { ...active, selected: Math.max(0, keep) }, dismissed: null }
      },
    },
    props: {
      decorations(state) {
        const { active } = suggestKey.getState(state)
        if (!active) return null
        const pick = active.matches[active.selected]
        const widget = Decoration.widget(active.to, (view) => suggestionDOM(view, active, typeLabel, hint), {
          side: 1,
          ignoreSelection: true,
          key: `suggest:${active.from}:${active.selected}:${active.matches.map((m) => `${m.type}${m.id}${m.label}`).join('|')}:${pick.rest}`,
        })
        return DecorationSet.create(state.doc, [widget])
      },
      handleKeyDown(view, e) {
        const { active } = suggestKey.getState(view.state)
        if (!active || e.ctrlKey || e.metaKey || e.altKey) return false
        const many = active.matches.length > 1
        if (e.key === 'Tab' && !e.shiftKey) return acceptSuggestion(view)
        if (e.key === 'Enter' && many && !e.shiftKey) return acceptSuggestion(view)
        if ((e.key === 'ArrowDown' || e.key === 'ArrowUp') && many) {
          const n = active.matches.length
          const next = (active.selected + (e.key === 'ArrowDown' ? 1 : n - 1)) % n
          view.dispatch(view.state.tr.setMeta(suggestKey, { select: next }))
          return true
        }
        if (e.key === 'Escape') {
          // Not also the page's Esc (leaving focus mode, closing find).
          e.stopPropagation()
          view.dispatch(view.state.tr.setMeta(suggestKey, { dismiss: true }))
          return true
        }
        return false
      },
    },
  })
}

// Puts the picked name in place of what was typed: a story_link over it, or
// plain text for a Script Mode speaker. The text keeps the typed text's
// formatting.
export function acceptSuggestion(view, i) {
  const { active } = suggestKey.getState(view.state) || {}
  if (!active) return false
  const m = active.matches[i ?? active.selected]
  if (!m) return false
  const { state } = view
  const typedNode = state.doc.nodeAt(active.from)
  let marks = (typedNode?.isText ? typedNode.marks : state.doc.resolve(active.from).marks()).filter((mk) => mk.type !== storyLink && mk.type !== link)
  if (!active.speaker) marks = storyLink.create({ type: m.type, id: m.id }).addToSet(marks)
  const tr = state.tr.replaceWith(active.from, active.to, schema.text(m.label, marks))
  tr.setSelection(TextSelection.create(tr.doc, active.from + m.label.length))
  tr.setMeta(suggestKey, { accepted: active.from })
  view.dispatch(tr.scrollIntoView())
  view.focus()
  return true
}

// The ghost text, and the list when there are several names.
function suggestionDOM(view, active, typeLabel, hint) {
  const wrap = document.createElement('span')
  wrap.className = 'pm-suggest'
  wrap.contentEditable = 'false'
  const pick = active.matches[active.selected]
  const ghost = document.createElement('span')
  ghost.className = pick.rest ? 'pm-ghost' : 'pm-ghost is-hint'
  ghost.textContent = pick.rest || '⇥'
  ghost.title = hint
  // A tap on the ghost takes it (phones have no Tab key).
  ghost.addEventListener('mousedown', (e) => {
    e.preventDefault()
    acceptSuggestion(view)
  })
  wrap.appendChild(ghost)
  if (active.matches.length > 1) {
    const list = document.createElement('span')
    list.className = 'pm-suggest-list'
    list.setAttribute('role', 'listbox')
    active.matches.forEach((m, i) => {
      const row = document.createElement('span')
      row.className = i === active.selected ? 'pm-suggest-item is-picked' : 'pm-suggest-item'
      row.setAttribute('role', 'option')
      row.setAttribute('aria-selected', String(i === active.selected))
      const name = document.createElement('span')
      name.className = 'pm-suggest-name'
      name.textContent = m.label
      const kind = document.createElement('span')
      kind.className = 'pm-suggest-type'
      kind.textContent = m.via === 'name' || m.label === m.name ? typeLabel(m.type) : `${typeLabel(m.type)} · ${m.name}`
      row.append(name, kind)
      row.addEventListener('mousedown', (e) => {
        e.preventDefault()
        acceptSuggestion(view, i)
      })
      list.appendChild(row)
    })
    wrap.appendChild(list)
  }
  return wrap
}
