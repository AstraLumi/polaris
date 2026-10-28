// Read aloud (stage 5): the browser half. A decoration plugin that paints the
// sentence being read, and a reader that speaks the chapter one sentence
// (long ones in pieces) at a time with speechSynthesis.
//
// Why a sentence per utterance: the highlight follows without needing the
// voice's `boundary` events (many voices give none), and Chrome's habit of
// silencing an utterance after about 15 seconds never bites. "Pause" cancels
// the current sentence and says it again from its start on resume, because
// speechSynthesis.pause() is unreliable across browsers. Typing, a click in
// the text, or any other change of the document stops the reading (the
// positions of the sentences would be stale); Esc stops it too.
import { Plugin, PluginKey } from 'prosemirror-state'
import { Decoration, DecorationSet } from 'prosemirror-view'
import { collectSentences, chunkText, clampRate, pickVoice, loadPrefs, savePrefs } from './readAloud.js'

export const readKey = new PluginKey('readAloud')

export const speechSupported = () => typeof window !== 'undefined' && 'speechSynthesis' in window && typeof SpeechSynthesisUtterance !== 'undefined'

// onStop: called when the text is clicked in or changed while reading.
export function readAloudPlugin({ onInterrupt } = {}) {
  return new Plugin({
    key: readKey,
    state: {
      init: () => DecorationSet.empty,
      apply(tr, deco, oldState, newState) {
        const meta = tr.getMeta(readKey)
        if (meta) return meta.range ? DecorationSet.create(newState.doc, [Decoration.inline(meta.range.from, meta.range.to, { class: 'read-now' })]) : DecorationSet.empty
        return tr.docChanged ? deco.map(tr.mapping, tr.doc) : deco
      },
    },
    props: {
      decorations: (state) => readKey.getState(state),
      handleDOMEvents: {
        mousedown: () => {
          queueMicrotask(() => onInterrupt?.())
          return false
        },
      },
    },
    view: () => ({
      update(view, prev) {
        if (view.state.doc !== prev.doc) queueMicrotask(() => onInterrupt?.())
      },
    }),
  })
}

// getView: the EditorView. getLang: the book's language. onChange(state):
// { status: 'idle' | 'playing' | 'paused', index, total, rate, voice }.
export function createReader({ getView, getLang, onChange }) {
  const synth = speechSupported() ? window.speechSynthesis : null
  const prefs = loadPrefs()
  let voices = []
  let items = []
  let index = 0
  let status = 'idle'
  let doc0 = null
  let token = 0
  let timer = null

  const emit = () => onChange?.({ status, index, total: items.length, rate: prefs.rate, voice: prefs.voice, voices })

  function paint(range) {
    const view = getView()
    if (!view) return
    view.dispatch(view.state.tr.setMeta(readKey, { range }))
  }

  // Keeps the sentence in view, scrolling the page gently only when it has
  // drifted out of the middle part of the area.
  function reveal(item) {
    const view = getView()
    const area = view?.dom.closest('.paper-area')
    if (!area) return
    try {
      const c = view.coordsAtPos(item.from)
      const r = area.getBoundingClientRect()
      const top = r.top + r.height * 0.2
      const bottom = r.top + r.height * 0.75
      if (c.top < top || c.bottom > bottom) area.scrollBy({ top: c.top - (r.top + r.height * 0.35), behavior: 'smooth' })
    } catch {
      // the position is gone; nothing to show
    }
  }

  function loadVoices() {
    if (!synth) return
    voices = synth.getVoices()
    emit()
  }
  if (synth) {
    loadVoices()
    synth.addEventListener?.('voiceschanged', loadVoices)
  }

  function currentVoice() {
    return pickVoice(voices, prefs.voice, getLang())
  }

  function finish() {
    token++
    clearTimeout(timer)
    timer = null
    synth?.cancel()
    status = 'idle'
    items = []
    index = 0
    doc0 = null
    paint(null)
    emit()
  }

  function speak(i) {
    clearTimeout(timer)
    const view = getView()
    if (!synth || !view || view.state.doc !== doc0 || i < 0) return finish()
    if (i >= items.length) return finish()
    index = i
    const item = items[i]
    const my = ++token
    synth.cancel()
    status = 'playing'
    paint({ from: item.from, to: item.to })
    reveal(item)
    emit()
    const pieces = chunkText(item.spoken)
    const voice = currentVoice()
    const lang = voice?.lang || getLang() || 'en'
    let p = 0
    const next = () => {
      if (my !== token) return
      if (p >= pieces.length) {
        timer = setTimeout(() => my === token && speak(i + 1), item.after)
        return
      }
      const u = new SpeechSynthesisUtterance(pieces[p++])
      try {
        if (voice) u.voice = voice
      } catch {
        // not a voice of this browser (a stand-in in tests)
      }
      u.lang = lang
      u.rate = prefs.rate
      u.onend = next
      u.onerror = (e) => {
        if (my !== token) return
        if (e.error === 'interrupted' || e.error === 'canceled') return
        finish()
      }
      synth.speak(u)
    }
    next()
  }

  return {
    supported: !!synth,
    get status() {
      return status
    },
    // Reads the selection, else from the caret to the end of the chapter.
    start() {
      const view = getView()
      if (!synth || !view) return
      finish()
      const { from, to, empty } = view.state.selection
      items = collectSentences(view.state.doc, { from, to: empty ? null : to, lang: getLang() })
      if (!items.length) return emit()
      doc0 = view.state.doc
      speak(0)
    },
    pause() {
      if (status !== 'playing') return
      token++
      clearTimeout(timer)
      synth.cancel()
      status = 'paused'
      emit()
    },
    resume() {
      if (status === 'paused') speak(index)
    },
    toggle() {
      if (status === 'playing') this.pause()
      else if (status === 'paused') this.resume()
      else this.start()
    },
    stop() {
      if (status !== 'idle') finish()
    },
    step(by) {
      if (status === 'idle') return
      const to = index + by
      if (to < 0) return speak(0)
      if (status === 'paused') {
        index = Math.min(to, items.length - 1)
        paint({ from: items[index].from, to: items[index].to })
        reveal(items[index])
        return emit()
      }
      speak(to)
    },
    setRate(r) {
      prefs.rate = clampRate(r)
      savePrefs(prefs)
      if (status === 'playing') speak(index)
      else emit()
    },
    setVoice(uri) {
      prefs.voice = uri || ''
      savePrefs(prefs)
      if (status === 'playing') speak(index)
      else emit()
    },
    refresh: emit,
    destroy() {
      if (status !== 'idle') finish()
      synth?.removeEventListener?.('voiceschanged', loadVoices)
    },
  }
}
