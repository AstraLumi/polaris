// The spellchecker's underlines (docs/EDITOR.md, "Spellchecker"): a plugin
// that keeps a wavy line under each word the dictionary and the story don't
// know. Only text that is on screen and has changed is checked: what a
// paragraph gave is remembered by its node (ProseMirror nodes don't change,
// a changed paragraph is a new node), so typing rechecks one paragraph,
// after a short pause. The word at the caret isn't underlined while it is
// being typed; linked text (story links, web links) and the speaker and
// manner of a dialogue (attrs, not text) are never checked.
//
// The checking itself is given: `getSpell()` returns null (off) or
//   { gen, skipCaps, known(word) → bool, wrong(words) → Promise<Set> }
// where `known` is the story's words (names, added, ignored) and `wrong`
// asks the dictionary. `gen` changes when any of it changes; refreshSpell
// then drops what was remembered.
import { Plugin, PluginKey } from 'prosemirror-state'
import { Decoration, DecorationSet } from 'prosemirror-view'
import { schema } from './schema.js'
import { tokenize } from './spellWords.js'

export const spellKey = new PluginKey('spell')
const { link, story_link: storyLink } = schema.marks
const DELAY = 350
const MAX_BLOCKS = 200 // per round; the rest in the next

// A textblock's text, one character per position (inline atoms and linked
// text are spaces, so no word is made across them).
export function blockText(node) {
  let s = ''
  node.forEach((child) => {
    if (child.isText) s += child.marks.some((m) => m.type === link || m.type === storyLink) ? ' '.repeat(child.text.length) : child.text
    else s += ' '.repeat(child.nodeSize)
  })
  return s
}

// The words of a block to look at: [{ word, from, to, dot }].
export function blockWords(node, spell) {
  return tokenize(blockText(node), { skipCaps: spell.skipCaps }).filter((t) => !spell.known(t.word))
}

// What the dictionary is asked for a word (an abbreviation: with its dot too).
const asks = (t) => (t.dot ? [t.word, t.word + '.'] : [t.word])

function build(doc, results, prev) {
  const decos = []
  doc.descendants((node, pos) => {
    if (!node.isTextblock) return true
    const r = results.get(node)
    if (r) for (const [a, b, word] of r) decos.push(Decoration.inline(pos + 1 + a, pos + 1 + b, { class: 'spell-wrong' }, { word }))
    else decos.push(...prev.find(pos, pos + node.nodeSize)) // until it is checked again
    return false
  })
  return DecorationSet.create(doc, decos)
}

// options.getSpell(): see above. options.onMenu({ word, from, to, x, y }):
// a right click (or, on a touch screen, a tap on the word the caret is
// already in) on an underlined word.
export function spellPlugin({ getSpell, onMenu = () => {}, isVisible = null }) {
  let results = new WeakMap() // textblock node → [[from, to, word]] in its text
  let gen = null
  return new Plugin({
    key: spellKey,
    state: {
      init: () => ({ deco: DecorationSet.empty, typingAt: null }),
      apply(tr, value) {
        const meta = tr.getMeta(spellKey)
        if (meta?.deco) return { ...value, deco: meta.deco }
        if (meta?.clear) return { deco: DecorationSet.empty, typingAt: null }
        let { deco, typingAt } = value
        if (tr.docChanged) {
          deco = deco.map(tr.mapping, tr.doc)
          const sel = tr.selection
          typingAt = sel.empty && !tr.getMeta('history$') && !tr.getMeta('paste') ? sel.head : null
        } else if (tr.selectionSet) typingAt = null
        return deco === value.deco && typingAt === value.typingAt ? value : { deco, typingAt }
      },
    },
    props: {
      decorations(state) {
        const { deco, typingAt } = spellKey.getState(state)
        const word = typingAt == null ? null : wordAround(state.doc, typingAt)
        if (!word) return deco
        const at = deco.find(word.from, word.to).filter((d) => d.from < word.to && d.to > word.from)
        return at.length ? deco.remove(at) : deco
      },
      handleDOMEvents: {
        contextmenu(view, e) {
          const hit = wordAt(view, e.clientX, e.clientY)
          if (!hit) return false
          e.preventDefault()
          onMenu({ ...hit, x: e.clientX, y: e.clientY })
          return true
        },
      },
      // Phones: the browser's long press is its own, so a tap on an
      // underlined word the caret is already in opens the menu.
      handleClick(view, pos, e) {
        if (typeof matchMedia === 'undefined' || !matchMedia('(pointer: coarse)').matches) return false
        const hit = decoAt(view.state, pos)
        const sel = view.state.selection
        if (hit && sel.empty && sel.from >= hit.from && sel.from <= hit.to) {
          const r = e.target?.getBoundingClientRect?.()
          setTimeout(() => onMenu({ ...hit, x: e.clientX || r?.left || 0, y: (r?.bottom ?? e.clientY) || 0 }), 0)
        }
        return false
      },
    },
    view(view) {
      let timer = null
      let running = false
      let again = false
      const schedule = (ms = DELAY) => {
        clearTimeout(timer)
        timer = setTimeout(run, ms)
      }
      const visible =
        isVisible ||
        ((v, offset) => {
          const dom = v.nodeDOM(offset)
          if (!dom?.getBoundingClientRect) return true
          const r = dom.getBoundingClientRect()
          const h = window.innerHeight || 800
          return r.bottom > -h && r.top < h * 2
        })

      async function run() {
        if (running) {
          again = true
          return
        }
        const spell = getSpell()
        if (!spell) return
        if (spell.gen !== gen) {
          gen = spell.gen
          results = new WeakMap()
        }
        // The textblocks on screen not checked yet.
        const todo = []
        const doc = view.state.doc
        doc.forEach((top, offset) => {
          if (todo.length >= MAX_BLOCKS || !visible(view, offset)) return
          const add = (node) => {
            if (!results.has(node) && !todo.includes(node)) todo.push(node)
          }
          if (top.isTextblock) add(top)
          else top.descendants((n) => (n.isTextblock ? (add(n), false) : true))
        })
        if (!todo.length) return
        running = true
        try {
          const words = todo.map((node) => [node, blockWords(node, spell)])
          const bad = await spell.wrong([...new Set(words.flatMap(([, ws]) => ws.flatMap(asks)))])
          if (getSpell()?.gen !== gen) return void (again = true)
          for (const [node, ws] of words) results.set(node, ws.filter((t) => asks(t).every((w) => bad.has(w))).map((t) => [t.from, t.to, t.word]))
          if (!view.isDestroyed) view.dispatch(view.state.tr.setMeta(spellKey, { deco: build(view.state.doc, results, spellKey.getState(view.state).deco) }).setMeta('addToHistory', false))
          if (todo.length >= MAX_BLOCKS) again = true
        } catch (e) {
          // The dictionary failed to load or the worker stopped: no underlines.
        } finally {
          running = false
          if (again) {
            again = false
            schedule(50)
          }
        }
      }
      const onScroll = () => schedule(150)
      window.addEventListener('scroll', onScroll, true)
      window.addEventListener('resize', onScroll)
      view.__spellRun = (ms) => schedule(ms)
      schedule(50)
      return {
        update(v, prev) {
          if (v.state.doc !== prev.doc) schedule()
        },
        destroy() {
          clearTimeout(timer)
          window.removeEventListener('scroll', onScroll, true)
          window.removeEventListener('resize', onScroll)
        },
      }
    },
  })
}

// The word the caret is in or at the end of (the one being typed): its
// { from, to } in the document, or null after a space.
const WORDISH = /[\p{L}\p{M}\p{N}'’ʼ\-‐]/u
function wordAround(doc, pos) {
  if (pos > doc.content.size) return null
  const $p = doc.resolve(pos)
  if (!$p.parent.isTextblock) return null
  const text = blockText($p.parent)
  let a = $p.parentOffset
  let b = a
  while (a > 0 && WORDISH.test(text[a - 1])) a--
  while (b < text.length && WORDISH.test(text[b])) b++
  return a === b ? null : { from: $p.start() + a, to: $p.start() + b }
}

// The underlined word at a document position: { word, from, to } or null.
function decoAt(state, pos) {
  const set = spellKey.getState(state)?.deco
  if (!set) return null
  const d = set.find(pos, pos).find((x) => x.from <= pos && x.to >= pos)
  return d ? { word: d.spec.word, from: d.from, to: d.to } : null
}

function wordAt(view, x, y) {
  const at = view.posAtCoords({ left: x, top: y })
  if (!at) return null
  const hit = decoAt(view.state, at.pos)
  // The word being typed isn't underlined, so it has no menu either.
  const typing = spellKey.getState(view.state).typingAt
  const busy = typing == null ? null : wordAround(view.state.doc, typing)
  if (!hit || (busy && busy.from < hit.to && busy.to > hit.from)) return null
  return hit
}

// Checks again from scratch (the language, the story's words or an option
// changed), or clears the underlines when the checker is off.
export function refreshSpell(view, on = true) {
  if (!view) return
  if (!on) view.dispatch(view.state.tr.setMeta(spellKey, { clear: true }).setMeta('addToHistory', false))
  view.__spellRun?.(on ? 20 : 0)
}

// Puts a suggestion in place of the word (if the word is still there),
// keeping its formatting.
export function replaceWord(view, { from, to, word }, text) {
  const { state } = view
  if (to > state.doc.content.size || state.doc.textBetween(from, to) !== word) return false
  view.dispatch(state.tr.insertText(text, from, to).scrollIntoView())
  view.focus()
  return true
}
