// The spellchecker's Hunspell reader (src/editor/hunspell.js) on a tiny
// dictionary written here, then on the four bundled ones
// (public/dict/, gzipped) with a few words each.
// node frontend/tests/spellcheck.test.mjs
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { gunzipSync } from 'node:zlib'
import { createSpeller, capType } from '../src/editor/hunspell.js'

const AFF = `SET UTF-8
TRY esianrtolcdugmphbyfvkwz'
ICONV 1
ICONV ’ '
FORBIDDENWORD !
NOSUGGEST ?
NEEDAFFIX %
KEEPCASE K
BREAK 2
BREAK -
BREAK ^ex-
REP 1
REP f ph
MAP 1
MAP cç

# plural: -s, or -es after s/x, y → ies after a consonant
SFX S Y 4
SFX S 0 s [^sxy]
SFX S 0 es [sx]
SFX S y ies [^aeiou]y
SFX S 0 s [aeiou]y

# -ing, the e dropped
SFX G Y 2
SFX G e ing e
SFX G 0 ing [^e]

# -tion, which can take the plural (two suffixes)
SFX T Y 1
SFX T e ion/S te

PFX U Y 1
PFX U 0 un .

PFX R N 1
PFX R 0 re .
`
const DIC = `11
cat/S
fox/S
city/S
make/GU
lock/USR
create/T
Paris
iPhone/K
teh/!
damn/?
bake/%G
`

const sp = createSpeller(AFF, DIC)
const yes = (...ws) => ws.forEach((w) => assert.ok(sp.check(w), `${w} should be a word`))
const no = (...ws) => ws.forEach((w) => assert.ok(!sp.check(w), `${w} should not be a word`))

yes('cat', 'cats', 'foxes', 'cities', 'make', 'making', 'unmake', 'unmaking')
no('cate', 'foxs', 'citys', 'makeing', 'remake')
// A cross product: un- with -s where both allow it; re- doesn't cross.
yes('locks', 'unlocks', 'relock', 'unlock')
no('relocks')
// Two suffixes: create → creation → creations.
yes('creation', 'creations')
no('creates', 'creationss')
// Case: a lower-case word in any case, a capitalised one only capitalised.
yes('Cat', 'CATS', 'Paris', 'PARIS', 'Making')
no('paris', 'pARIS')
// KEEPCASE: only as written. FORBIDDENWORD: never. NEEDAFFIX: only with one.
yes('iPhone')
no('IPHONE', 'Iphone', 'teh', 'bake')
yes('baking')
// Curly apostrophes (ICONV), hyphens and BREAK.
yes('cat-fox', 'ex-cats')
no('cat-foo', 'ex-')
assert.ok(sp.check(''))

// Suggestions: the case, REP, MAP, one edit, two words; never a NOSUGGEST word.
assert.deepEqual(sp.suggest('paris'), ['Paris'])
assert.equal(sp.suggest('ctas')[0], 'cats')
assert.equal(sp.suggest('Ctas')[0], 'Cats')
assert.equal(sp.suggest('CTAS')[0], 'CATS')
assert.ok(sp.suggest('catfox').includes('cat fox'))
assert.ok(!sp.suggest('dam').includes('damn'))
assert.ok(sp.suggest('mking').includes('making'))
assert.equal(capType('McDonald'), 3)

// FLAG long and num, and AF aliases.
const long = createSpeller('FLAG long\nSFX Aa Y 1\nSFX Aa 0 s .\n', '1\nbird/AaZz\n')
assert.ok(long.check('birds') && !long.check('birdz'))
const num = createSpeller('FLAG num\nSFX 101 Y 1\nSFX 101 0 ed .\n', '1\njump/7,101\n')
assert.ok(num.check('jumped') && !num.check('jumps'))
// With AF, a stem's flags are an alias's number.
const alias = createSpeller('FLAG num\nAF 1\nAF 101,7\nSFX 101 Y 1\nSFX 101 0 ed .\n', '1\nwalk/1\n')
assert.ok(alias.check('walked') && !alias.check('walks'))

// ---- The bundled dictionaries ---------------------------------------------------------
const dict = (code) => {
  const read = (ext) => gunzipSync(readFileSync(new URL(`../public/dict/${code}/${code}.${ext}.gz`, import.meta.url))).toString('utf8')
  const t0 = performance.now()
  const s = createSpeller(read('aff'), read('dic'))
  return { s, ms: Math.round(performance.now() - t0) }
}
const cases = {
  en_US: { yes: ['walked', 'houses', 'color', 'don’t', 'unhappiness', 'Paris', 'PARIS', 'well-known'], no: ['recieve', 'colour', 'teh', 'paris'], sug: ['recieve', 'receive'] },
  en_GB: { yes: ['colour', 'organisation', 'don’t', 'centre'], no: ['recieve', 'color'], sug: ['color', 'colour'] },
  pt_BR: { yes: ['casas', 'cantávamos', 'aplicações', 'fazê-lo', 'ex-presidente', 'bem-te-vi', 'coração', 'Brasil'], no: ['aplicacao', 'coracao', 'descubrimos'], sug: ['aplicacao', 'aplicação'] },
  es_ES: { yes: ['casas', 'cantábamos', 'aplicaciones', 'dígamelo', 'Madrid', 'naciones'], no: ['aplicacion', 'perrro', 'madrid'], sug: ['perrro', 'perro'] },
}
for (const [code, c] of Object.entries(cases)) {
  const { s, ms } = dict(code)
  for (const w of c.yes) assert.ok(s.check(w), `${code}: ${w}`)
  for (const w of c.no) assert.ok(!s.check(w), `${code}: not ${w}`)
  const [typo, want] = c.sug
  assert.ok(s.suggest(typo).slice(0, 3).includes(want), `${code}: ${typo} → ${want}: ${s.suggest(typo)}`)
  console.log(`  ${code}: ${s.size} stems, loaded in ${ms} ms`)
}

console.log('spellcheck: ok')
