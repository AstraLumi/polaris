// Tab-to-link and Script Mode's speakers (docs/EDITOR.md, stage 4): which
// story items a word being typed could be the start of, and which character
// a speaker's name is. Pure, no dependencies (frontend/tests/nameMatch.test.mjs).
//
// Items are the wiki's article list (GET /wiki: type, id, name, a
// character's aliases (former names) and nicknames). Matching ignores case
// and accents; what is put in the text is the name (or nickname, or former
// name) as written in the story, replacing what was typed.

const MAX_WORDS = 6 // the longest name, in words, a typed start is looked for in
const MIN_LETTERS = 3

// Lower case without accents (é → e, ß → ss), for comparing.
export function fold(s) {
  return String(s ?? '')
    .normalize('NFKD')
    .replace(/\p{M}/gu, '')
    .toLowerCase()
    .replace(/ß/g, 'ss')
    .replace(/\s+/g, ' ')
}

const WORD_CHAR = /[\p{L}\p{N}]/u
const isWordChar = (c) => !!c && WORD_CHAR.test(c)
const letters = (s) => (String(s).match(/\p{L}/gu) || []).length

// Every name an item can be found by: { type, id, name, label, via, key }.
// via: 'name' | 'nickname' | 'alias'; label is the text a link gets.
export function buildNameIndex(items) {
  const out = []
  const seen = new Set()
  const add = (it, label, via) => {
    const text = String(label ?? '').trim().replace(/\s+/g, ' ')
    if (!text) return
    const key = fold(text)
    const k = `${it.type}#${it.id}#${key}`
    if (seen.has(k)) return
    seen.add(k)
    out.push({ type: it.type, id: Number(it.id), name: it.name, label: text, via, key })
  }
  for (const it of items || []) {
    if (!it || !it.type || it.id == null) continue
    add(it, it.name, 'name')
    for (const n of it.nicknames || []) add(it, n, 'nickname')
    for (const n of it.aliases || []) add(it, n, 'alias')
  }
  return out
}

// Best first: characters, then a name that starts with exactly what was
// typed (case and accents), then shorter names, then by name.
function rank(typed) {
  return (a, b) =>
    (a.type === 'character' ? 0 : 1) - (b.type === 'character' ? 0 : 1) ||
    (a.label.startsWith(typed) ? 0 : 1) - (b.label.startsWith(typed) ? 0 : 1) ||
    a.label.length - b.label.length ||
    a.label.localeCompare(b.label)
}

// What the text before the caret could be the start of.
//   before: the paragraph's text up to the caret; after: the text after it.
//   options.types: only these item types (Script Mode's speakers: characters).
// Gives { start, typed, matches } (start: where in `before` the typed name
// begins; each match { type, id, name, label, via, rest }) or null. A typed
// start needs three letters, must begin a word, and the caret must end one
// (not inside a word). The longest typed start that matches wins ("Mira V"
// for "Mira Vey" rather than "V").
export function findMatches(index, before, after = '', { max = 8, types = null, minLetters = MIN_LETTERS } = {}) {
  const text = String(before ?? '')
  if (!text || !isWordChar(text[text.length - 1]) || isWordChar(String(after ?? '')[0])) return null
  // Where words begin, from the caret back.
  const starts = []
  for (let i = text.length - 1; i >= 0 && starts.length < MAX_WORDS; i--) {
    if (isWordChar(text[i]) && !isWordChar(text[i - 1]) && text[i - 1] !== '’' && text[i - 1] !== "'") starts.push(i)
    if (text[i] === '\n' || text[i] === '￼') break
  }
  for (const start of starts.reverse()) {
    const typed = text.slice(start)
    if (letters(typed) < minLetters) continue
    const key = fold(typed)
    const found = index.filter((e) => e.key.startsWith(key) && (!types || types.includes(e.type)))
    if (!found.length) continue
    found.sort(rank(typed))
    const matches = found.slice(0, max).map((e) => ({ ...e, rest: restOf(e.label, typed) }))
    return { start, typed, matches }
  }
  return null
}

// The part of a name still to type, after what was typed (which may differ
// in case or accents: the rest is taken from the name by length).
function restOf(label, typed) {
  // Fold char by char to find where the typed text ends in the label.
  const want = fold(typed)
  let i = 0
  let got = ''
  while (i < label.length && got.length < want.length) {
    got += fold(label[i])
    i++
  }
  return label.slice(i)
}

// The character a speaker's name is (by name, nickname or former name; an
// exact match ignoring case and accents), or null.
export function resolveSpeaker(index, speaker) {
  const key = fold(String(speaker ?? '').trim())
  if (!key) return null
  const hit = index.filter((e) => e.type === 'character' && e.key === key)
  if (!hit.length) return null
  // The current name wins over a nickname, a nickname over a former name.
  const order = { name: 0, nickname: 1, alias: 2 }
  hit.sort((a, b) => order[a.via] - order[b.via] || a.id - b.id)
  return { id: hit[0].id, name: hit[0].name }
}

// A Script Mode line start: "Name:" or "Name, manner:" (what comes before
// the space that converts it). Gives { speaker, manner } or null. A speaker
// is up to six words and 60 characters; the manner up to 200.
export function parseSpeakerLine(text) {
  const m = /^\s*([^\s,:][^,:\n]{0,59}?)\s*(?:,\s*([^:\n]{1,200}?))?\s*:$/u.exec(String(text ?? ''))
  if (!m) return null
  const speaker = m[1].trim()
  if (!speaker || speaker.split(/\s+/).length > MAX_WORDS || !/\p{L}/u.test(speaker)) return null
  return { speaker, manner: (m[2] || '').trim() }
}

// Back to plain text, for Backspace at a dialogue's start.
export function speakerLead({ speaker, manner }) {
  const s = String(speaker || '').trim()
  const m = String(manner || '').trim()
  if (s && m) return `${s}, ${m}: `
  if (s) return `${s}: `
  return m ? `${m}: ` : ''
}
