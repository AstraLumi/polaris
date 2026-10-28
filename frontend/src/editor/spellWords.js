// The spellchecker's words (docs/EDITOR.md, "Spellchecker"): what in a text
// counts as a word to check, the story's own words (names, nicknames,
// former names; the story's added words; the words ignored for the
// session) and which dictionary a book uses. Pure, no dependencies
// (frontend/tests/spellWords.test.mjs).

// The dictionaries in public/dict/<code>/ (frontend/scripts/dictionaries.mjs);
// the revision of the LibreOffice dictionaries they come from, added to the
// address so a browser never keeps an older copy.
export const DICT_REVISION = '4f1df64'
export const DICTIONARIES = {
  en_US: { name: 'English (US)' },
  en_GB: { name: 'English (UK)' },
  pt_BR: { name: 'Português (Brasil)' },
  es_ES: { name: 'Español' },
}
// What a book's language is set to when the writer picks a dictionary.
export const DICTIONARY_LANG = { en_US: 'en-US', en_GB: 'en-GB', pt_BR: 'pt-BR', es_ES: 'es' }

const COMMONWEALTH = ['gb', 'uk', 'au', 'nz', 'ie', 'za', 'in']

// The dictionary for a language tag ("en", "en-GB", "pt_BR", "es-419"), or
// null when there is none.
export function dictionaryFor(tag) {
  const parts = String(tag ?? '').trim().toLowerCase().replace(/_/g, '-').split('-')
  const [lang, region] = parts
  if (lang === 'en' || lang === 'eng') return COMMONWEALTH.includes(region) ? 'en_GB' : 'en_US'
  if (lang === 'pt' || lang === 'por') return 'pt_BR'
  if (lang === 'es' || lang === 'spa') return 'es_ES'
  return null
}

// The book's language picks the dictionary; an empty one falls back to the
// interface's. Gives { code, tag, from: 'book' | 'interface' }; code is
// null when that language has no dictionary (spelling is then off).
export function spellLanguage(bookLanguage, interfaceLocale) {
  const book = String(bookLanguage ?? '').trim()
  const tag = book || String(interfaceLocale ?? '').trim()
  return { code: dictionaryFor(tag), tag, from: book ? 'book' : 'interface' }
}

export const dictionaryUrl = (code, ext) => `/dict/${code}/${code}.${ext}.gz?v=${DICT_REVISION}`

// ---- What is a word -----------------------------------------------------------------

const LETTER = /[\p{L}\p{M}]/u
const ALNUM = /[\p{L}\p{M}\p{N}]/u
const DIGIT = /\p{N}/u
const JOINERS = new Set(["'", '’', 'ʼ', '-', '‐'])
// Web and mail addresses, and paths: never checked.
const ADDRESS = /(?:\b(?:https?|ftp):\/\/|\bwww\.|\bmailto:)[^\s<>"“”]*|[\p{L}\p{N}._%+-]+@[\p{L}\p{N}.-]+\.\p{L}{2,}/giu

// The words of a text to check: [{ word, from, to }] (offsets in the text).
// A word is letters (any script, with accents) and may hold an apostrophe
// or a hyphen between two letters ("don't", "Mira's", "well-known",
// "fazê-lo"). Left out: anything with a digit ("3rd", "B12"), one letter
// alone, web and mail addresses, words joined to others by a dot with no
// space ("e.g.", "example.com", "a.C.") and, with `skipCaps`, words in
// capitals ("NASA", "LEVEL UP"). `dot` is set when a full stop follows (an
// abbreviation may be in the dictionary with it).
export function tokenize(text, { skipCaps = false } = {}) {
  const s = String(text ?? '')
  const out = []
  const blocked = []
  for (const m of s.matchAll(ADDRESS)) blocked.push([m.index, m.index + m[0].length])
  let b = 0
  let i = 0
  const n = s.length
  while (i < n) {
    while (b < blocked.length && blocked[b][1] <= i) b++
    if (b < blocked.length && blocked[b][0] <= i) {
      i = blocked[b][1]
      continue
    }
    const cp = s.codePointAt(i)
    const ch = String.fromCodePoint(cp)
    if (!ALNUM.test(ch)) {
      i += ch.length
      continue
    }
    const from = i
    let j = i
    for (;;) {
      const c = String.fromCodePoint(s.codePointAt(j))
      if (ALNUM.test(c)) {
        j += c.length
        if (j >= n) break
        continue
      }
      // An apostrophe or hyphen between two letters (or digits) joins.
      if (JOINERS.has(c) && j > from && j + 1 < n && ALNUM.test(String.fromCodePoint(s.codePointAt(j + 1)))) {
        j += 1
        continue
      }
      break
    }
    i = j
    if (b < blocked.length && blocked[b][0] < j) continue
    const word = s.slice(from, j)
    if (DIGIT.test(word)) continue
    if (Array.from(word).filter((c) => LETTER.test(c)).length < 2) continue
    // Joined to the next or last word by a dot: an abbreviation or address.
    if ((s[j] === '.' && ALNUM.test(s[j + 1] || '')) || (s[from - 1] === '.' && ALNUM.test(s[from - 2] || ''))) continue
    // Joined by a slash, underscore or the like: a path or a code.
    if (/[/\\_@#]/.test(s[j] || '') || /[/\\_@#]/.test(s[from - 1] || '')) continue
    if (skipCaps && isCaps(word)) continue
    out.push({ word, from, to: j, dot: s[j] === '.' })
  }
  return out
}

// Every letter a capital (and at least two of them).
export function isCaps(word) {
  let caps = 0
  for (const c of word) {
    if (c.toLowerCase() === c.toUpperCase()) continue
    if (c !== c.toUpperCase()) return false
    caps++
  }
  return caps > 1
}

// ---- The story's words -------------------------------------------------------------------

const lower = (w) => w.toLowerCase()
const apostrophes = (w) => String(w).normalize('NFC').replace(/[’ʼ]/g, "'")

// A set of words that count as words: { has(word), size }. A word written
// in lower case in the list matches it in any case ("bravo", "Bravo",
// "BRAVO"); one with a capital matches only written with one, in any case
// ("Mira", "MIRA", "MiRA"; not "mira"). A possessive ("Mira's", "Mira’s",
// "Thomas'") matches when the word without it does.
export function wordSet(list) {
  const map = new Map() // lower case → true when the list has it in lower case
  for (const raw of list || []) {
    const w = apostrophes(raw).trim()
    if (!w) continue
    const key = lower(w)
    map.set(key, map.get(key) || w === key)
  }
  const one = (w) => {
    const key = lower(w)
    if (!map.has(key)) return false
    return map.get(key) || w !== key
  }
  return {
    size: map.size,
    has(raw) {
      const w = apostrophes(raw)
      if (one(w)) return true
      const poss = /^(.+?)(?:'s|'S|s')$/u.exec(w)
      return !!poss && poss[1].length > 0 && one(poss[1] + (w.endsWith("s'") ? 's' : ''))
    },
  }
}

// The words of the story's names: every name, nickname and former name in
// the wiki's list (GET /wiki: name, nicknames, aliases; or the names as
// plain strings, as nameMatch.js's index labels them), word by word,
// hyphenated names also in parts ("Ka-Ren": "Ka-Ren", "Ka", "Ren").
export function storyWords(items) {
  const out = new Set()
  const addName = (name) => {
    for (const { word } of tokenize(name)) {
      out.add(word)
      if (/[-‐]/.test(word)) for (const part of word.split(/[-‐]/)) if (part) out.add(part)
    }
  }
  for (const it of items || []) {
    if (!it) continue
    if (typeof it === 'string') {
      addName(it)
      continue
    }
    addName(it.name)
    for (const n of it.nicknames || []) addName(n)
    for (const n of it.aliases || []) addName(n)
  }
  return [...out]
}

// A word the writer can add to the story's dictionary: what the server
// takes (spelling.go): 1–64 characters, no spaces or control characters, a
// letter at least.
export function cleanWord(raw) {
  const w = String(raw ?? '').normalize('NFC').trim()
  if (!w || Array.from(w).length > 64 || /[\s\p{C}]/u.test(w) || !/\p{L}/u.test(w)) return ''
  return w
}
