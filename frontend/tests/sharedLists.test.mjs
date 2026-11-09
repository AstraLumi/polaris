// Lists the server and the page each keep a copy of (CLAUDE.md: "exist
// twice"): the page offers a choice, the server checks it against its own
// list, and one added on a single side is either refused or never offered.
// Both sides are read as text (the page's modules import others without a
// file extension, which only the bundler follows) and compared.
// node frontend/tests/sharedLists.test.mjs
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const read = (path) => readFileSync(new URL(path, import.meta.url), 'utf8')

// The text between `opening` and the first `closing` after it.
function block(file, text, opening, closing) {
  const start = text.indexOf(opening)
  assert.ok(start >= 0, `${file}: "${opening}" not found`)
  const end = text.indexOf(closing, start + opening.length)
  assert.ok(end >= 0, `${file}: "${opening}" is never closed`)
  return text.slice(start + opening.length, end)
}
const quoted = (text, re) => {
  const out = [...text.matchAll(re)].map((m) => m[1]).filter(Boolean)
  assert.ok(out.length, 'an empty list: the source changed shape, update this test')
  return out
}

// Go: `name = map[string]bool{"a": true, …}` or `name = []string{"", "a", …}`
// (an empty string there means "from the character").
// gofmt lines the `=` signs up, so the spaces before them are squeezed first.
const goKeys = (file, name, type) =>
  quoted(block(file, read(`../../backend/${file}`).replace(/ +=/g, ' ='), `${name} = ${type}{`, '}'), /"([^"]*)"/g)
// JS: `const NAME = ['a', …]` on one line, every string in it…
const jsIds = (file, name) => quoted(block(file, read(`../src/${file}`), `const ${name} = [`, ']'), /'([^']*)'/g)
// …or `const NAME = [` up to the line that closes it, one property's values.
const jsProp = (file, name, prop) => quoted(block(file, read(`../src/${file}`), `const ${name} = [`, '\n]'), new RegExp(`\\b${prop}: '([^']*)'`, 'g'))

const same = (what, page, server) => assert.deepEqual([...page].sort(), [...server].sort(), `${what}: the page's list and the server's differ`)

same('spell icons', jsProp('builtinIcons.js', 'BUILTIN_ICONS', 'id'), goKeys('icons.go', 'builtinSpellIcons', 'map[string]bool'))
same('gear icons', jsIds('builtinIcons.js', 'GEAR_ICON_IDS'), goKeys('icons.go', 'builtinGearIcons', 'map[string]bool'))
same('story icons', jsIds('builtinIcons.js', 'STORY_ICON_IDS'), goKeys('stories.go', 'builtinStoryIcons', 'map[string]bool'))

same('biomes', jsProp('biomes.js', 'BIOME_GROUPS', 'key'), goKeys('planetlayers.go', 'biomeKeys', 'map[string]bool'))
same('terrain', jsProp('biomes.js', 'LANDFORMS', 'key'), goKeys('planetlayers.go', 'reliefKeys', 'map[string]bool'))

same('outline sexes', jsProp('bodyOutlines.js', 'OUTLINE_SEXES', 'key'), goKeys('bodyoutlines.go', 'outlineSexes', '[]string'))
same('outline ages', jsProp('bodyOutlines.js', 'OUTLINE_AGES', 'key'), goKeys('bodyoutlines.go', 'outlineAges', '[]string'))
same('outline builds', jsProp('bodyOutlines.js', 'OUTLINE_BUILDS', 'key'), goKeys('bodyoutlines.go', 'outlineBuilds', '[]string'))

console.log('sharedLists: ok')
