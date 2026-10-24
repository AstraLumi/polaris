// Find and replace in the editor: a plugin that keeps the matches of the
// current search (worked out again whenever the text changes) and paints
// them, plus the commands the find panel calls.
import { Plugin, PluginKey, TextSelection } from 'prosemirror-state'
import { Decoration, DecorationSet } from 'prosemirror-view'
import { findMatches, replaceMatches, nextMatchIndex } from './find.js'

export const findKey = new PluginKey('find')

const EMPTY = { query: '', opts: {}, matches: [], current: -1, deco: DecorationSet.empty }

function decorate(doc, matches, current) {
  return DecorationSet.create(
    doc,
    matches.map((m, i) => Decoration.inline(m.from, m.to, { class: i === current ? 'find-match is-current' : 'find-match' })),
  )
}

function build(doc, query, opts, current) {
  const matches = findMatches(doc, query, opts)
  const cur = matches.length ? Math.min(Math.max(current, 0), matches.length - 1) : -1
  return { query, opts, matches, current: cur, deco: decorate(doc, matches, cur) }
}

export function findPlugin() {
  return new Plugin({
    key: findKey,
    state: {
      init: () => EMPTY,
      apply(tr, value, oldState, newState) {
        const meta = tr.getMeta(findKey)
        if (meta) {
          if (meta.clear) return EMPTY
          const query = meta.query ?? value.query
          const opts = meta.opts ?? value.opts
          const matches = findMatches(newState.doc, query, opts)
          let current = meta.from != null ? nextMatchIndex(matches, meta.from) : meta.current ?? value.current
          current = matches.length ? Math.min(Math.max(current, 0), matches.length - 1) : -1
          return { query, opts, matches, current, deco: decorate(newState.doc, matches, current) }
        }
        if (tr.docChanged && value.query) return build(newState.doc, value.query, value.opts, value.current)
        return value
      },
    },
    props: {
      decorations: (state) => findKey.getState(state).deco,
    },
  })
}

export const findState = (state) => findKey.getState(state) || EMPTY

// A new search (or new options), starting from the cursor.
export function setSearch(view, query, opts) {
  view.dispatch(view.state.tr.setMeta(findKey, { query, opts, from: view.state.selection.from }))
  revealCurrent(view)
}

export function clearSearch(view) {
  view.dispatch(view.state.tr.setMeta(findKey, { clear: true }))
}

function revealCurrent(view, select = false) {
  const { matches, current } = findState(view.state)
  const m = matches[current]
  if (!m) return
  const tr = view.state.tr
  if (select) tr.setSelection(TextSelection.create(tr.doc, m.from, m.to))
  view.dispatch(tr.scrollIntoView())
  // Scroll the match itself into view (scrollIntoView follows the selection).
  const dom = view.domAtPos(m.from)
  const el = dom.node.nodeType === 1 ? dom.node : dom.node.parentElement
  el?.scrollIntoView?.({ block: 'center', inline: 'nearest' })
}

export function findStep(view, dir = 1) {
  const { matches, current } = findState(view.state)
  if (!matches.length) return
  const next = (current + dir + matches.length) % matches.length
  view.dispatch(view.state.tr.setMeta(findKey, { current: next }))
  revealCurrent(view, true)
}

export function replaceCurrent(view, text) {
  const { matches, current } = findState(view.state)
  const m = matches[current]
  if (!m) return
  view.dispatch(replaceMatches(view.state.tr, [m], text).scrollIntoView())
  revealCurrent(view)
}

// Replaces every match in one step (one Ctrl+Z undoes it). Returns how many.
export function replaceAll(view, text) {
  const { matches } = findState(view.state)
  if (!matches.length) return 0
  view.dispatch(replaceMatches(view.state.tr, matches, text))
  return matches.length
}
