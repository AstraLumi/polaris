// Finds every UI string in src/ and checks the translation catalogs.
//   node frontend/tests/i18n.check.mjs          report problems (exit 1 if any)
//   node frontend/tests/i18n.check.mjs --keys   print every key as JSON
// A key is the English text inside $t('…'), t('…'), tn('…','…'), tr('…') or
// an apiError('…') fallback. Every language must have an entry for every key,
// with the same {placeholders}.

import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const here = path.dirname(fileURLToPath(import.meta.url))
const src = path.join(here, '..', 'src')

function walk(dir) {
  return fs.readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const p = path.join(dir, e.name)
    if (e.isDirectory()) return e.name === 'locales' ? [] : walk(p)
    return /\.(vue|js)$/.test(e.name) ? [p] : []
  })
}

const LIT = String.raw`('(?:[^'\\\n]|\\.)*'|"(?:[^"\\\n]|\\.)*"|` + '`(?:[^`\\\\$]|\\\\.)*`)'
function unquote(l) {
  const body = l.slice(1, -1)
  return body.replace(/\\(.)/g, (m, c) => (c === 'n' ? '\n' : c))
}

const keys = new Map() // key -> [files]
function add(k, file) {
  if (!/[A-Za-z]/.test(k)) return
  if (!keys.has(k)) keys.set(k, new Set())
  keys.get(k).add(path.relative(src, file))
}

for (const file of walk(src)) {
  const text = fs.readFileSync(file, 'utf8')
  // t('x') / $t('x') / tr('x') / tn('a', 'b', …) / $tn(...)
  for (const m of text.matchAll(new RegExp(String.raw`(?<![\w.])\$?(?:t|tr)\(\s*` + LIT, 'g'))) add(unquote(m[1]), file)
  for (const m of text.matchAll(new RegExp(String.raw`(?<![\w.])\$?tn\(\s*` + LIT + String.raw`\s*,\s*` + LIT, 'g'))) {
    add(unquote(m[1]), file)
    add(unquote(m[2]), file)
  }
  // apiError('x') and 'fallback' on the same call
  for (const line of text.split('\n')) {
    if (!line.includes('apiError(')) continue
    for (const m of line.slice(line.indexOf('apiError(')).matchAll(new RegExp(LIT, 'g'))) add(unquote(m[1]), file)
  }
}

if (process.argv.includes('--keys')) {
  console.log(JSON.stringify([...keys.keys()].sort(), null, 1))
  process.exit(0)
}

const placeholders = (s) => [...s.matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort().join(',')
let problems = 0
const langs = ['pt-BR', 'es-ES', 'es-419']
for (const lang of langs) {
  const cat = (await import(pathToFileURL(path.join(src, 'locales', lang + '.js')).href)).default
  const missing = [...keys.keys()].filter((k) => !(k in cat))
  const badPh = [...keys.keys()].filter((k) => k in cat && placeholders(k) !== placeholders(cat[k]))
  const empty = Object.keys(cat).filter((k) => !String(cat[k]).trim())
  const unused = Object.keys(cat).filter((k) => !keys.has(k))
  console.log(`${lang}: ${keys.size - missing.length}/${keys.size} translated` +
    (badPh.length ? `, ${badPh.length} placeholder mismatches` : '') +
    (unused.length ? `, ${unused.length} unused entries` : ''))
  for (const k of missing.slice(0, 40)) console.log(`  missing: ${JSON.stringify(k)}  (${[...keys.get(k)][0]})`)
  if (missing.length > 40) console.log(`  … and ${missing.length - 40} more`)
  for (const k of badPh) console.log(`  placeholders differ: ${JSON.stringify(k)} → ${JSON.stringify(cat[k])}`)
  for (const k of empty) console.log(`  empty translation: ${JSON.stringify(k)}`)
  problems += missing.length + badPh.length + empty.length
}
process.exit(problems ? 1 : 0)
