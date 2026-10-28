// A Hunspell reader for the spellchecker (docs/EDITOR.md, stage 5): reads a
// LibreOffice/Hunspell dictionary (.aff and .dic) and answers "is this a
// word" and "what could this word be". Pure, no dependencies; it runs in
// spell.worker.js and in node (frontend/tests/spellcheck.test.mjs).
//
// Words are not expanded at load (nspell does that, and takes minutes and
// gigabytes on pt_BR): the stems stay in one Map and a word is checked the
// way Hunspell does, by taking affixes off it and looking for a stem that
// allows them. What is read is the subset the bundled dictionaries use:
//
//   SET (UTF-8 only), FLAG (characters, UTF-8, long, num), AF aliases,
//   PFX/SFX with conditions, cross products and two suffixes (a suffix
//   whose continuation flags allow another one: es "-ción" + "-es"),
//   FULLSTRIP, NEEDAFFIX, FORBIDDENWORD, NOSUGGEST, KEEPCASE,
//   ONLYINCOMPOUND (such stems are never words on their own), BREAK,
//   ICONV, and for suggestions TRY, REP and MAP.
//
// Not read: compounding (COMPOUNDRULE, COMPOUNDFLAG…; the bundled ones use
// it only for ordinals such as "21st", and words with digits are never
// checked), morphology, PHONE and n-gram suggestions, other encodings.

const NOCAP = 0
const INITCAP = 1
const ALLCAP = 2
const HUHCAP = 3

export function capType(w) {
  let upper = 0
  let lower = 0
  for (const ch of w) {
    const l = ch.toLowerCase()
    const u = ch.toUpperCase()
    if (l === u) continue
    if (ch === u) upper++
    else lower++
  }
  if (!upper) return NOCAP
  if (!lower) return ALLCAP
  const first = String.fromCodePoint(w.codePointAt(0))
  if (upper === 1 && first !== first.toLowerCase()) return INITCAP
  return HUHCAP
}

const lower = (w) => w.toLowerCase()
export const capitalise = (w) => {
  const first = String.fromCodePoint(w.codePointAt(0))
  return first.toUpperCase() + w.slice(first.length)
}

// ---- Reading the .aff file ---------------------------------------------------------

// Hunspell's affix condition (characters, [set], [^set] and ".") as a
// regular expression on the end (suffix) or start (prefix) of a stem.
function conditionSource(cond) {
  let out = ''
  let inSet = false
  for (let i = 0; i < cond.length; i++) {
    const c = cond[i]
    if (inSet) {
      if (c === ']') {
        inSet = false
        out += c
      } else if (c === '^' && cond[i - 1] === '[') out += c
      else out += /[\\\]\[\-^]/.test(c) ? '\\' + c : c
    } else if (c === '[') {
      inSet = true
      out += c
    } else if (c === '.') out += '.'
    else out += /[\\^$*+?()[\]{}|/.-]/.test(c) ? '\\' + c : c
  }
  if (inSet) out += ']'
  return out
}

export function parseAff(text) {
  const aff = {
    flagMode: 'char',
    aliases: null,
    prefixes: [],
    suffixes: [],
    rep: [],
    map: [],
    iconv: [],
    breaks: null,
    try: '',
    wordchars: '',
    fullstrip: false,
    flags: {},
  }
  const lines = String(text).replace(/^﻿/, '').split(/\r?\n/)
  const pending = new Map() // PFX/SFX flag → entries still to read
  const headers = new Set() // REP, MAP, BREAK, ICONV, AF: header line seen
  const flagged = []
  for (const raw of lines) {
    const line = raw.trim()
    if (!line || line[0] === '#') continue
    const tk = line.split(/\s+/)
    const key = tk[0]
    switch (key) {
      case 'FLAG':
        aff.flagMode = tk[1] === 'long' ? 'long' : tk[1] === 'num' ? 'num' : 'char'
        break
      case 'TRY':
        aff.try = tk[1] || ''
        break
      case 'WORDCHARS':
        aff.wordchars = tk[1] || ''
        break
      case 'FULLSTRIP':
        aff.fullstrip = true
        break
      case 'NOSUGGEST':
      case 'FORBIDDENWORD':
      case 'NEEDAFFIX':
      case 'PSEUDOROOT':
      case 'KEEPCASE':
      case 'ONLYINCOMPOUND':
      case 'CIRCUMFIX':
        flagged.push([key === 'PSEUDOROOT' ? 'NEEDAFFIX' : key, tk[1]])
        break
      case 'AF':
        if (!headers.has('AF')) {
          headers.add('AF')
          aff.aliases = ['']
        } else aff.aliases.push(tk[1] || '')
        break
      case 'REP':
      case 'ICONV':
        if (tk.length >= 3) aff[key === 'REP' ? 'rep' : 'iconv'].push([tk[1], tk[2]])
        break
      case 'MAP':
        if (!headers.has('MAP')) headers.add('MAP')
        else if (tk[1]) aff.map.push(mapItems(tk[1]))
        break
      case 'BREAK':
        if (!headers.has('BREAK')) {
          headers.add('BREAK')
          aff.breaks = []
        } else if (tk[1]) aff.breaks.push(tk[1])
        break
      case 'PFX':
      case 'SFX': {
        const flag = tk[1]
        const id = key + flag
        if (!pending.get(id)) {
          pending.set(id, +tk[3] || 0)
          aff[key === 'PFX' ? 'prefixes' : 'suffixes'].push({ header: true, flag, cross: tk[2] === 'Y' })
          break
        }
        pending.set(id, pending.get(id) - 1)
        const list = aff[key === 'PFX' ? 'prefixes' : 'suffixes']
        let head = null
        for (let i = list.length - 1; i >= 0; i--) {
          if (list[i].header && list[i].flag === flag) {
            head = list[i]
            break
          }
        }
        const strip = tk[2] === '0' ? '' : tk[2] || ''
        let add = tk[3] || ''
        let cont = ''
        const slash = add.indexOf('/')
        if (slash >= 0) {
          cont = add.slice(slash + 1)
          add = add.slice(0, slash)
        }
        if (add === '0') add = ''
        list.push({ flag, cross: !!head?.cross, strip, add, cond: tk[4] || '.', cont })
        break
      }
      default:
        break
    }
  }
  aff.prefixes = aff.prefixes.filter((r) => !r.header)
  aff.suffixes = aff.suffixes.filter((r) => !r.header)
  aff._flagged = flagged
  return aff
}

// "aáàâ" or "(ss)ß" → ['a', 'á', 'à', 'â'] / ['ss', 'ß'].
function mapItems(s) {
  const out = []
  for (let i = 0; i < s.length; ) {
    if (s[i] === '(') {
      const j = s.indexOf(')', i)
      if (j > i) {
        out.push(s.slice(i + 1, j))
        i = j + 1
        continue
      }
    }
    const cp = String.fromCodePoint(s.codePointAt(i))
    out.push(cp)
    i += cp.length
  }
  return out
}

// ---- The dictionary ---------------------------------------------------------------------

export function createSpeller(affText, dicText) {
  const aff = parseAff(affText)

  // Flags become one UTF-16 unit each, whatever the dictionary's FLAG type,
  // so a stem's flags are a string and `includes` tells if it has one.
  const interned = new Map()
  const intern = (token) => {
    if (aff.flagMode === 'char' && token.length === 1) return token
    let c = interned.get(token)
    if (!c) {
      c = String.fromCharCode(0xe000 + interned.size)
      interned.set(token, c)
    }
    return c
  }
  const decodeFlags = (s, aliased = true) => {
    if (!s) return ''
    if (aliased && aff.aliases && /^\d+$/.test(s)) s = aff.aliases[+s] || ''
    if (aff.flagMode === 'long') {
      let out = ''
      for (let i = 0; i < s.length; i += 2) out += intern(s.slice(i, i + 2))
      return out
    }
    if (aff.flagMode === 'num') return s.split(',').filter(Boolean).map(intern).join('')
    let out = ''
    for (const ch of s) out += intern(ch)
    return out
  }
  const flag = (token) => (token == null ? null : decodeFlags(token, false)[0] || null)

  const F = {}
  for (const [k, v] of aff._flagged) F[k] = flag(v)

  // Affixes, indexed by what they add (suffixes by their ending).
  const prepare = (r, kind) => ({ ...r, flag: flag(r.flag), cont: decodeFlags(r.cont), kind, re: null })
  const sfxByAdd = new Map()
  const pfxByAdd = new Map()
  let maxSfx = 0
  let maxPfx = 0
  const contFlags = new Set() // flags some suffix lets follow it
  for (const r of aff.suffixes.map((r) => prepare(r, 'S'))) {
    if (!sfxByAdd.has(r.add)) sfxByAdd.set(r.add, [])
    sfxByAdd.get(r.add).push(r)
    maxSfx = Math.max(maxSfx, r.add.length)
    for (const c of r.cont) contFlags.add(c)
  }
  for (const r of aff.prefixes.map((r) => prepare(r, 'P'))) {
    if (!pfxByAdd.has(r.add)) pfxByAdd.set(r.add, [])
    pfxByAdd.get(r.add).push(r)
    maxPfx = Math.max(maxPfx, r.add.length)
  }

  const condCache = new Map()
  function condOk(r, stem) {
    if (r.cond === '.') return true
    if (!r.re) {
      const key = r.kind + r.cond
      let re = condCache.get(key)
      if (!re) {
        const src = conditionSource(r.cond)
        re = new RegExp(r.kind === 'S' ? `(?:${src})$` : `^(?:${src})`, 'u')
        condCache.set(key, re)
      }
      r.re = re
    }
    return r.re.test(stem)
  }

  // The stems: word → flags (a string), or a list of them for homonyms.
  const words = new Map()
  const addWord = (word, flags) => {
    const had = words.get(word)
    if (had === undefined) words.set(word, flags)
    else if (Array.isArray(had)) had.push(flags)
    else words.set(word, [had, flags])
  }
  const lines = String(dicText).replace(/^﻿/, '').split('\n')
  for (let i = 1; i < lines.length; i++) {
    let s = lines[i]
    const tab = s.indexOf('\t')
    if (tab >= 0) s = s.slice(0, tab)
    s = s.trim()
    if (!s) continue
    // Morphological fields ("po:noun") after a space.
    const sp = s.search(/\s\S+:/)
    if (sp > 0) s = s.slice(0, sp).trim()
    let slash = -1
    for (let j = s.indexOf('/'); j >= 0; j = s.indexOf('/', j + 1)) {
      if (j > 0 && s[j - 1] !== '\\') {
        slash = j
        break
      }
    }
    const word = (slash >= 0 ? s.slice(0, slash) : s).replace(/\\\//g, '/')
    if (word) addWord(word, slash >= 0 ? decodeFlags(s.slice(slash + 1)) : '')
  }

  const homonyms = (v) => (v === undefined ? [] : Array.isArray(v) ? v : [v])

  // A stem that may stand with an affix whose flag is `fl`: its flags, or
  // null. `also(flags)` adds a condition (a cross product's other affix).
  function stemWith(stem, fl, also, caseChanged) {
    for (const flags of homonyms(words.get(stem))) {
      if (!flags.includes(fl)) continue
      if (F.FORBIDDENWORD && flags.includes(F.FORBIDDENWORD)) continue
      if (F.ONLYINCOMPOUND && flags.includes(F.ONLYINCOMPOUND)) continue
      if (caseChanged && F.KEEPCASE && flags.includes(F.KEEPCASE)) continue
      if (also && !also(flags)) continue
      return flags
    }
    return null
  }

  // The word with one suffix (or two) taken off. `pfx`: a prefix already
  // taken off (cross product). `outer`: the flag of an outer suffix this
  // one must allow (the second of two suffixes).
  function bySuffix(w, pfx, outer, caseChanged) {
    const n = w.length
    for (let k = 0; k <= Math.min(n, maxSfx); k++) {
      const rules = sfxByAdd.get(k ? w.slice(n - k) : '')
      if (!rules) continue
      const base = n - k
      if (base < 0 || (base === 0 && !aff.fullstrip)) continue
      const head = w.slice(0, base)
      for (const r of rules) {
        if (pfx && !r.cross) continue
        if (outer && !r.cont.includes(outer)) continue
        if (!outer && F.NEEDAFFIX && r.cont.includes(F.NEEDAFFIX)) continue
        const stem = head + r.strip
        if (!stem || !condOk(r, stem)) continue
        const also = pfx ? (flags) => flags.includes(pfx.flag) || r.cont.includes(pfx.flag) : null
        const found = stemWith(stem, r.flag, also, caseChanged)
        if (found) return found
        if (!pfx && !outer && contFlags.has(r.flag)) {
          const inner = bySuffix(stem, null, r.flag, caseChanged)
          if (inner) return inner
        }
      }
    }
    return null
  }

  function byPrefix(w, caseChanged) {
    const n = w.length
    for (let k = 0; k <= Math.min(n, maxPfx); k++) {
      const rules = pfxByAdd.get(w.slice(0, k))
      if (!rules) continue
      const rest = n - k
      if (rest < 0 || (rest === 0 && !aff.fullstrip)) continue
      const tail = w.slice(k)
      for (const r of rules) {
        const stem = r.strip + tail
        if (!stem || !condOk(r, stem)) continue
        if (!(F.NEEDAFFIX && r.cont.includes(F.NEEDAFFIX))) {
          const found = stemWith(stem, r.flag, null, caseChanged)
          if (found) return found
        }
        if (r.cross) {
          const found = bySuffix(stem, r, null, caseChanged)
          if (found) return found
        }
      }
    }
    return null
  }

  // The flags of the stem that makes `w` a word (exactly as cased), null
  // when it isn't one, or false when it is a forbidden word.
  function lookup(w, caseChanged) {
    for (const flags of homonyms(words.get(w))) {
      if (F.FORBIDDENWORD && flags.includes(F.FORBIDDENWORD)) return false
      if (F.NEEDAFFIX && flags.includes(F.NEEDAFFIX)) continue
      if (F.ONLYINCOMPOUND && flags.includes(F.ONLYINCOMPOUND)) continue
      if (caseChanged && F.KEEPCASE && flags.includes(F.KEEPCASE)) continue
      return flags
    }
    return bySuffix(w, null, null, caseChanged) || byPrefix(w, caseChanged)
  }

  // A word in any of the cases Hunspell allows for it: "Paris" and "PARIS"
  // for Paris, "The" and "THE" for the.
  function found(w) {
    const type = capType(w)
    const tries = [[w, false]]
    if (type === INITCAP) tries.push([lower(w), true])
    if (type === ALLCAP) tries.push([capitalise(lower(w)), true], [lower(w), true])
    for (const [form, changed] of tries) {
      const f = lookup(form, changed)
      if (f === false) return false
      if (f != null) return f
    }
    return null
  }

  const iconv = [...aff.iconv].sort((a, b) => b[0].length - a[0].length)
  function convert(w) {
    let s = String(w).replace(/[’ʼ]/g, "'")
    if (!iconv.length) return s
    let out = ''
    for (let i = 0; i < s.length; ) {
      const hit = iconv.find(([from]) => s.startsWith(from, i))
      if (hit) {
        out += hit[1]
        i += hit[0].length
      } else out += s[i++]
    }
    return out
  }

  const breaks = aff.breaks || ['-', '^-', '-$']
  function correct(w, depth = 0) {
    const f = found(w)
    if (f === false) return false
    if (f != null) return true
    if (depth > 8) return false
    // Hyphenated words: each part a word (BREAK).
    for (const b of breaks) {
      if (b.length > 1 && b[0] === '^') {
        const p = b.slice(1)
        if (w.length > p.length && w.startsWith(p) && correct(w.slice(p.length), depth + 1)) return true
      } else if (b.length > 1 && b.endsWith('$')) {
        const p = b.slice(0, -1)
        if (w.length > p.length && w.endsWith(p) && correct(w.slice(0, w.length - p.length), depth + 1)) return true
      } else {
        const i = w.indexOf(b)
        if (i > 0 && i + b.length < w.length && correct(w.slice(0, i), depth + 1) && correct(w.slice(i + b.length), depth + 1)) return true
      }
    }
    return false
  }

  // ---- Suggestions -------------------------------------------------------------------
  // Hunspell's simple ways (REP, MAP, the case, one letter more, less,
  // swapped or different, two words), then two edits while time allows;
  // ranked by how close each is to the word (accents count half).
  const tryChars = (() => {
    const set = []
    for (const ch of aff.try) if (!set.includes(ch)) set.push(ch)
    return set
  })()
  const suggestable = (c) => {
    if (!c || c.length > 100) return false
    const f = found(c)
    return f != null && f !== false && !(F.NOSUGGEST && f.includes(F.NOSUGGEST))
  }

  function suggest(raw, { max = 6, budgetMs = 250 } = {}) {
    const typed = String(raw)
    const curly = /[’]/.test(typed)
    const w = convert(typed)
    if (!w) return []
    const type = capType(w)
    const base = type === ALLCAP || type === INITCAP ? lower(w) : w
    const out = new Map() // candidate → rank
    const add = (c, rank) => {
      if (c === w || out.has(c)) return
      if (c.includes(' ')) {
        const parts = c.split(' ')
        if (parts.some((p) => !p || !suggestable(p))) return
      } else if (!suggestable(c)) return
      out.set(c, rank)
    }
    const start = Date.now()
    const late = () => Date.now() - start > budgetMs

    // The case.
    add(lower(w), 0)
    add(capitalise(lower(w)), 0)

    for (const src of base === w ? [w] : [w, base]) {
      // REP: common mistakes.
      for (const [from0, to0] of aff.rep) {
        let from = from0
        const atStart = from.startsWith('^')
        const atEnd = from.endsWith('$')
        if (atStart) from = from.slice(1)
        if (atEnd) from = from.slice(0, -1)
        if (!from) continue
        const to = to0.replace(/_/g, ' ')
        for (let i = src.indexOf(from); i >= 0; i = src.indexOf(from, i + 1)) {
          if (atStart && i !== 0) break
          if (atEnd && i + from.length !== src.length) continue
          add(src.slice(0, i) + to + src.slice(i + from.length), 0)
        }
      }
      // MAP: related letters (accents), any number of them.
      if (aff.map.length) mapVariants(src, (c) => add(c, 0), late)
    }

    // One edit.
    const one = edits(base)
    for (const c of one) add(c, 1)
    // Two words.
    for (let i = 1; i < base.length; i++) {
      const a = base.slice(0, i)
      const b = base.slice(i)
      if ((a.length > 1 || /^[ae]$/i.test(a)) && b.length > 1) add(`${a} ${b}`, 2.5)
    }
    // Two edits, while there is time.
    if (out.size < max && base.length > 3) {
      for (const c1 of one) {
        if (late()) break
        for (const c2 of edits(c1)) add(c2, 2)
      }
    }

    const fw = fold(base)
    const ranked = [...out.entries()]
      .map(([c, rank], i) => ({
        c,
        // Close first; a capital the writer didn't type counts a little.
        score: rank * 10 + distance(base, lower(c), fw) - (lower(c)[0] === base[0] ? 0.3 : 0) + (type === NOCAP && capType(c) !== NOCAP ? 0.25 : 0),
        i,
      }))
      .sort((a, b) => a.score - b.score || a.i - b.i)
      .map((x) => x.c)
    const seen = new Set()
    const res = []
    for (let c of ranked) {
      if (type === ALLCAP) c = c.toUpperCase()
      else if (type === INITCAP && !c.includes(' ')) c = capitalise(c)
      if (curly) c = c.replace(/'/g, '’')
      // One of "lantern" / "Lantern": the first, the closer one.
      if (c === typed || seen.has(lower(c))) continue
      seen.add(lower(c))
      res.push(c)
      if (res.length >= max) break
    }
    return res
  }

  // Every string one edit away: a letter swapped with the next (or the one
  // after), left out, added or changed (with the TRY letters).
  function edits(w) {
    const out = new Set()
    const chars = Array.from(w)
    const n = chars.length
    const letters = tryChars.length ? tryChars : [...new Set(chars)]
    for (let i = 0; i < n - 1; i++) {
      const c = chars.slice()
      ;[c[i], c[i + 1]] = [c[i + 1], c[i]]
      out.add(c.join(''))
      if (i < n - 2) {
        const d = chars.slice()
        ;[d[i], d[i + 2]] = [d[i + 2], d[i]]
        out.add(d.join(''))
      }
    }
    for (let i = 0; i < n; i++) out.add(chars.slice(0, i).join('') + chars.slice(i + 1).join(''))
    for (let i = 0; i <= n; i++) {
      const head = chars.slice(0, i).join('')
      const tail = chars.slice(i).join('')
      for (const l of letters) out.add(head + l + tail)
    }
    for (let i = 0; i < n; i++) {
      const head = chars.slice(0, i).join('')
      const tail = chars.slice(i + 1).join('')
      for (const l of letters) if (l !== chars[i]) out.add(head + l + tail)
    }
    out.delete(w)
    return out
  }

  // MAP: each related letter tried in each place, in every combination.
  function mapVariants(w, emit, late) {
    let count = 0
    const walk = (s, from) => {
      if (count++ > 2000 || late()) return
      for (let i = from; i < s.length; i++) {
        for (const group of aff.map) {
          for (const item of group) {
            if (!s.startsWith(item, i)) continue
            for (const other of group) {
              if (other === item) continue
              const next = s.slice(0, i) + other + s.slice(i + item.length)
              emit(next)
              walk(next, i + other.length)
            }
          }
        }
      }
    }
    walk(w, 0)
  }

  return {
    size: words.size,
    // Is it a word? (Exactly as written: "paris" isn't Paris.)
    check: (word) => {
      const w = convert(word)
      return !w || correct(w)
    },
    suggest,
    // A word the dictionary takes from now on (with no affixes).
    add: (word) => addWord(convert(word), ''),
  }
}

// Lower case without accents, for comparing.
function fold(s) {
  return String(s).normalize('NFKD').replace(/\p{M}/gu, '').toLowerCase()
}

// Edit distance with swaps (Damerau), a change between two letters that
// differ only in accents costing half.
function distance(a, b, fa = fold(a)) {
  const x = Array.from(a)
  const y = Array.from(b)
  const fx = Array.from(fa)
  const fy = Array.from(fold(b))
  const sameLen = fx.length === x.length && fy.length === y.length
  const m = x.length
  const n = y.length
  const d = Array.from({ length: m + 1 }, (_, i) => [i, ...new Array(n).fill(0)])
  for (let j = 1; j <= n; j++) d[0][j] = j
  for (let i = 1; i <= m; i++) {
    for (let j = 1; j <= n; j++) {
      let cost = x[i - 1] === y[j - 1] ? 0 : 1
      if (cost && sameLen && fx[i - 1] === fy[j - 1]) cost = 0.5
      d[i][j] = Math.min(d[i - 1][j] + 1, d[i][j - 1] + 1, d[i - 1][j - 1] + cost)
      if (i > 1 && j > 1 && x[i - 1] === y[j - 2] && x[i - 2] === y[j - 1]) d[i][j] = Math.min(d[i][j], d[i - 2][j - 2] + 1)
    }
  }
  return d[m][n]
}
