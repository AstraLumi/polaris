// Read aloud (stage 5): the pure part. Cuts the chapter into sentences the
// speech synthesis can say one at a time, each tied to its place in the
// document so the page can highlight it; also the voice list and the
// remembered voice and speed. No browser objects here (readAloudPlugin.js
// has them). Tested by tests/readAloud.test.mjs.

export const RATE_MIN = 0.5
export const RATE_MAX = 2
export const clampRate = (r) => {
  const n = Number(r)
  return Number.isFinite(n) ? Math.min(RATE_MAX, Math.max(RATE_MIN, n)) : 1
}

// Silence after a block, in ms: a paragraph, a heading, a scene break.
export const PAUSE = { paragraph: 350, heading: 900, scene: 1100 }

// Words that end in a dot without ending the sentence (lower case, no final dot).
const ABBR_EN = ['mr', 'mrs', 'ms', 'mx', 'dr', 'st', 'prof', 'sr', 'jr', 'mt', 'ft', 'vs', 'capt', 'col', 'gen', 'lt', 'sgt', 'rev', 'hon', 'gov', 'sen', 'rep', 'no', 'e.g', 'i.e', 'cf', 'approx', 'dept', 'inc', 'ltd', 'co', 'a.m', 'p.m']
const ABBR_ES = ['sr', 'sra', 'srta', 'dr', 'dra', 'don', 'doña', 'prof', 'profa', 'lic', 'ing', 'av', 'avda', 'pág', 'págs', 'núm', 'tel', 'vs', 'etc', 'p.ej', 'ej', 'cap', 'ud', 'uds', 'sto', 'sta', 'gral', 'cía', 'a.m', 'p.m', 'e.g', 'i.e', 'st', 'mr', 'mrs']
const ABBR = { en: new Set(ABBR_EN), es: new Set(ABBR_ES) }
const abbreviations = (lang) => (/^(pt|es)/i.test(lang || '') ? ABBR.es : ABBR.en)

const ENDERS = '.!?…'
const CLOSERS = '"\'”’»›)]}'
const hasWord = (s) => /[\p{L}\p{N}]/u.test(s)

// Sentences of one line of text as { start, end } offsets (end exclusive,
// no surrounding spaces). A full stop does not end a sentence after an
// abbreviation (Mr., Dr., e.g.; Sr., Sra. in Portuguese and Spanish), after
// a lone initial (J.), inside a number (3.5), or when the next word starts
// in lower case ("Stop!" she said. / wait... and then). Closing quotes and
// brackets after the mark stay with their sentence.
export function splitSentences(text, lang = 'en') {
  const abbr = abbreviations(lang)
  const out = []
  const n = text.length
  let start = 0
  while (start < n && /\s/.test(text[start])) start++
  let i = start
  while (i < n) {
    if (!ENDERS.includes(text[i])) {
      i++
      continue
    }
    let j = i
    while (j + 1 < n && ENDERS.includes(text[j + 1])) j++
    let k = j + 1
    while (k < n && CLOSERS.includes(text[k])) k++
    if (k < n && !/\s/.test(text[k])) {
      i = k
      continue
    }
    let next = k
    while (next < n && /\s/.test(text[next])) next++
    let ends = true
    if (next < n && /\p{Ll}/u.test(text[next])) ends = false
    if (ends && j === i && text[i] === '.') {
      const m = /([\p{L}\p{N}.]+)$/u.exec(text.slice(0, i))
      if (m) {
        const w = m[1].toLowerCase().replace(/^\.+/, '')
        if (abbr.has(w) || /^\p{Lu}$/u.test(m[1])) ends = false
      }
    }
    if (ends && !hasWord(text.slice(start, k))) ends = false
    if (ends) {
      out.push({ start, end: k })
      start = next
      i = next
    } else {
      i = k
    }
  }
  if (start < n && hasWord(text.slice(start))) out.push({ start, end: text.replace(/\s+$/, '').length })
  return out
}

// A long sentence in pieces of at most `max` characters, cut at a comma,
// semicolon, colon, dash or space, so no single utterance runs long (some
// browsers stop an utterance after about 15 seconds).
export function chunkText(text, max = 220) {
  const out = []
  let rest = text.trim()
  while (rest.length > max) {
    let cut = -1
    for (let p = max; p > max / 2; p--) {
      if (',;:—–'.includes(rest[p - 1])) {
        cut = p
        break
      }
    }
    if (cut < 0) cut = rest.lastIndexOf(' ', max) + 1
    if (cut <= 0) cut = max
    out.push(rest.slice(0, cut).trim())
    rest = rest.slice(cut).trim()
  }
  if (rest) out.push(rest)
  return out
}

// What to read: the sentences of `doc` as { from, to, text, spoken, after }.
// from/to are document positions (the highlight), text what is on the page,
// spoken what is said (a dialogue line first names its speaker when
// `speakers` is on: "Mira: Stop!"), after the silence that follows, in ms.
// With a selection (to > from) only the selected text is read (a sentence
// cut by the selection starts or ends at its edge); with a caret, from the
// sentence holding the caret to the end. Pictures, page breaks and the
// table of contents are skipped; a scene break lengthens the pause before it.
export function collectSentences(doc, { from = 0, to = null, lang = 'en', speakers = true } = {}) {
  const selection = to != null && to > from
  const hi = selection ? to : doc.content.size
  const items = []
  doc.nodesBetween(from, hi, (node, pos) => {
    if (node.type.name === 'scene_break') {
      if (items.length) items[items.length - 1].after = PAUSE.scene
      return false
    }
    if (!node.isTextblock) return true
    const kind = node.type.name
    let text = ''
    const map = []
    node.forEach((child, off) => {
      const at = pos + 1 + off
      if (child.isText) {
        for (let c = 0; c < child.text.length; c++) {
          map.push(at + c)
          text += child.text[c]
        }
      } else {
        map.push(at)
        text += ' '
      }
    })
    const first = items.length
    for (const s of splitSentences(text, lang)) {
      let a = s.start
      let b = s.end
      if (selection) {
        while (a < b && (map[a] < from || /\s/.test(text[a]))) a++
        while (b > a && (map[b - 1] >= hi || /\s/.test(text[b - 1]))) b--
        if (a >= b || !hasWord(text.slice(a, b))) continue
      } else if (map[b - 1] + 1 <= from) continue
      const said = text.slice(a, b).replace(/\s+/g, ' ')
      items.push({ from: map[a], to: map[b - 1] + 1, text: said, spoken: said, after: 0 })
    }
    if (items.length > first) {
      if (kind === 'dialogue' && speakers && node.attrs.speaker) items[first].spoken = `${node.attrs.speaker}: ${items[first].text}`
      items[items.length - 1].after = kind === 'heading' ? PAUSE.heading : PAUSE.paragraph
    }
    return false
  })
  return items
}

// ---- Voices --------------------------------------------------------------------------------

const base = (l) => String(l || '').toLowerCase().replace('_', '-')

// The voices for a book in `lang` ("en", "pt-BR"): those of the language
// first (the same region before the others), then the rest, each by name.
export function groupVoices(voices, lang) {
  const want = base(lang)
  const root = want.split('-')[0]
  const byName = (a, b) => a.name.localeCompare(b.name)
  const matching = []
  const others = []
  for (const v of voices || []) (root && base(v.lang).split('-')[0] === root ? matching : others).push(v)
  const score = (v) => (base(v.lang) === want ? 0 : 1)
  matching.sort((a, b) => score(a) - score(b) || byName(a, b))
  others.sort(byName)
  return { matching, others }
}

// The voice to use: the remembered one if it is still there, else the best
// of the book's language, else null (the browser's default).
export function pickVoice(voices, savedURI, lang) {
  const list = voices || []
  const saved = savedURI && list.find((v) => v.voiceURI === savedURI)
  if (saved) return saved
  const { matching } = groupVoices(list, lang)
  return matching.find((v) => v.default) || matching[0] || null
}

// ---- Remembered choices (per browser) -------------------------------------------------------

const KEY = 'polaris.readAloud'

export function loadPrefs(storage = globalThis.localStorage) {
  try {
    const p = JSON.parse(storage.getItem(KEY) || '{}')
    return { voice: typeof p.voice === 'string' ? p.voice : '', rate: clampRate(p.rate ?? 1) }
  } catch {
    return { voice: '', rate: 1 }
  }
}

export function savePrefs(prefs, storage = globalThis.localStorage) {
  try {
    storage.setItem(KEY, JSON.stringify({ voice: prefs.voice || '', rate: clampRate(prefs.rate) }))
  } catch {
    // private window, blocked storage: the choice just isn't kept
  }
}
