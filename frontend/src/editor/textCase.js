// Shift+F3 (Word's "change case"): lower → UPPER → Title → lower… Pure.

const LETTER = /\p{L}/u
const WORD_INSIDE = /[\p{L}\p{N}'’]/u

// Text in a case, character by character. `before`: the character just
// before it (for Title case of text that starts mid-word).
export function toCase(text, mode, before = '') {
  if (mode === 'upper') return text.toUpperCase()
  if (mode === 'lower') return text.toLowerCase()
  let prev = before
  let out = ''
  for (const ch of text) {
    out += WORD_INSIDE.test(prev) ? ch.toLowerCase() : ch.toUpperCase()
    prev = ch
  }
  return out
}

// The case a piece of text is in now: 'lower', 'upper', 'title' or 'mixed'.
export function caseOf(text, before = '') {
  if (!LETTER.test(text)) return 'mixed'
  if (text === text.toLowerCase()) return 'lower'
  if (text === text.toUpperCase()) return 'upper'
  if (text === toCase(text, 'title', before)) return 'title'
  return 'mixed'
}

// The next case in Word's cycle.
export function nextCase(text, before = '') {
  const c = caseOf(text, before)
  return c === 'lower' ? 'upper' : c === 'upper' ? 'title' : 'lower'
}
