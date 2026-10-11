// Stat formulas as blocks (Settings → Stats). No dependencies:
//   node frontend/tests/formulaBlocks.test.mjs
import assert from 'node:assert/strict'
import { parseBlocks, blocksToText, renameRef, keyFromName } from '../src/formulaBlocks.js'

// Every default formula (backend/statsystem.go) survives a round trip.
for (const f of [
  '10 + [vit] * 10 + [level] * 2',
  '1 + round(curve([age], 0, 0.75, 18, 0, 100, -0.75, 200, -1), 2)',
  '[weight] * 0.13 + max(0, [height] / 100 - 1) * 2',
  '([str] / 10) ^ 2',
  '[base] - ([weight] - 70) * 0.05',
  '',
]) {
  assert.equal(blocksToText(parseBlocks(f)), f, f)
}

assert.deepEqual(parseBlocks('max( 1,[hp])'), [
  { type: 'fn', name: 'max' },
  { type: 'num', text: '1' },
  { type: 'op', op: ',' },
  { type: 'ref', key: 'hp' },
  { type: 'op', op: ')' },
])
// Nothing typed is lost, even what isn't a formula.
assert.equal(blocksToText(parseBlocks('2 $ foo')), '2 $ foo')

assert.equal(renameRef('[new_stat] + [new_stat_2] * [ new_stat ]', 'new_stat', 'stamina'), '[stamina] + [new_stat_2] * [stamina]')

assert.equal(keyFromName('Max Stamina', []), 'max_stamina')
assert.equal(keyFromName('Força Máxima', []), 'forca_maxima')
assert.equal(keyFromName('Level', []), 'level_2', 'formula inputs are reserved')
assert.equal(keyFromName('HP', ['hp', 'hp_2']), 'hp_3')
assert.equal(keyFromName('123', []), 'stat')
assert.equal(keyFromName('', ['stat']), 'stat_2')

console.log('formulaBlocks: ok')
