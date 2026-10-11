// Stat formulas as blocks (no dependencies, so node tests can load it).
//
// A formula is stored as text ("10 + [vit] * 10 + [level] * 2", the syntax of
// backend/statformula.go) and edited as a row of blocks in Settings → Stats.
// These read the text into blocks and write it back.

// Formula inputs (stats.js FORMULA_INPUTS); no stat can take these keys.
export const RESERVED_KEYS = ['level', 'age', 'height', 'weight', 'base']

// Text → blocks: { type: 'num', text } | { type: 'ref', key } |
// { type: 'fn', name } (the function with its opening bracket) | { type: 'op', op }.
// Anything it doesn't understand becomes a { type: 'bad', text } block, so
// nothing typed is ever lost.
export function parseBlocks(text) {
  const out = []
  const s = text || ''
  let i = 0
  while (i < s.length) {
    const c = s[i]
    if (/\s/.test(c)) {
      i++
    } else if (/[0-9.]/.test(c)) {
      let j = i
      while (j < s.length && /[0-9.]/.test(s[j])) j++
      out.push({ type: 'num', text: s.slice(i, j) })
      i = j
    } else if (c === '[') {
      const j = s.indexOf(']', i)
      if (j < 0) {
        out.push({ type: 'bad', text: s.slice(i) })
        break
      }
      out.push({ type: 'ref', key: s.slice(i + 1, j).trim() })
      i = j + 1
    } else if (/[a-z]/.test(c)) {
      let j = i
      while (j < s.length && /[a-z0-9_]/.test(s[j])) j++
      const name = s.slice(i, j)
      let k = j
      while (k < s.length && /\s/.test(s[k])) k++
      if (s[k] === '(') {
        out.push({ type: 'fn', name })
        i = k + 1
      } else {
        out.push({ type: 'bad', text: name })
        i = j
      }
    } else if ('+-*/^(),'.includes(c)) {
      out.push({ type: 'op', op: c })
      i++
    } else {
      out.push({ type: 'bad', text: c })
      i++
    }
  }
  return out
}

export function blocksToText(blocks) {
  let out = ''
  blocks.forEach((b, i) => {
    const prev = blocks[i - 1]
    const glued =
      !prev ||
      prev.type === 'fn' ||
      (prev.type === 'op' && prev.op === '(') ||
      (b.type === 'op' && (b.op === ')' || b.op === ',')) ||
      // a minus sign that starts a value (-0.75), not one that subtracts
      (prev.type === 'op' && prev.op === '-' && isSign(blocks, i - 1))
    if (!glued) out += ' '
    if (b.type === 'num') out += b.text
    else if (b.type === 'ref') out += `[${b.key}]`
    else if (b.type === 'fn') out += `${b.name}(`
    else if (b.type === 'op') out += b.op
    else out += b.text
  })
  return out
}

// Whether the minus at i is a sign: nothing before it, or an operator other
// than a closing bracket.
function isSign(blocks, i) {
  const prev = blocks[i - 1]
  return !prev || prev.type === 'fn' || (prev.type === 'op' && prev.op !== ')')
}

// Every [key] a formula reads, renamed: used while a stat that isn't saved
// yet still changes its key with its name.
export function renameRef(text, from, to) {
  return (text || '').replace(new RegExp(`\\[\\s*${from}\\s*\\]`, 'g'), `[${to}]`)
}

// A key for a new stat, from its name: "Max Stamina" → "max_stamina". Keys
// never change once saved; the reserved ones are the formula inputs.
export function keyFromName(name, taken) {
  const base =
    (name || '')
      .normalize('NFD')
      .replace(/[̀-ͯ]/g, '')
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, '_')
      .replace(/^[^a-z]+/, '')
      .replace(/_+$/, '')
      .slice(0, 32) || 'stat'
  const reserved = new Set([...RESERVED_KEYS, ...taken])
  let key = base
  for (let n = 2; reserved.has(key); n++) key = `${base}_${n}`
  return key
}
