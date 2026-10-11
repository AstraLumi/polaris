// The family tree's layout. No dependencies: node frontend/tests/familyLayout.test.mjs
import assert from 'node:assert/strict'
import { layoutFamily, CARD_W } from '../src/familyLayout.js'

const people = (...ids) => ids.map((id) => ({ id }))
const L = (from, to, kind, until) => ({ from, to, kind, until })
const center = (t, id) => t.pos.get(id).x + CARD_W / 2

// Grandparents 1+2 have 3 and 4; 3 marries 5; 3+5 have 6, 7 and 8; 9 is 4's partner.
const tree = layoutFamily(
  people(1, 2, 3, 4, 5, 6, 7, 8, 9),
  [
    L(1, 2, 'spouse'),
    L(1, 3, 'parent'), L(2, 3, 'parent'), L(1, 4, 'parent'), L(2, 4, 'parent'),
    L(3, 5, 'spouse', '01-01-1250'),
    L(3, 6, 'parent'), L(5, 6, 'parent'), L(3, 7, 'parent'), L(5, 7, 'parent'), L(3, 8, 'parent'), L(5, 8, 'parent'),
    L(4, 9, 'partner'),
  ],
  6,
)
const gen = (id) => tree.pos.get(id).gen
assert.deepEqual([1, 2, 3, 4, 5, 6, 7, 8, 9].map(gen), [0, 0, 1, 1, 1, 2, 2, 2, 1], 'generations')
for (const id of [1, 2, 3]) assert.ok(tree.pos.get(id).y < tree.pos.get(6).y, 'parents above children')

// Partners sit side by side; the ended marriage is marked.
for (const [a, b] of [[1, 2], [3, 5], [4, 9]]) {
  assert.equal(Math.abs(tree.pos.get(a).x - tree.pos.get(b).x) <= CARD_W + 40, true, `${a} and ${b} side by side`)
}
const ended = tree.couples.find((c) => (c.a === 3 && c.b === 5) || (c.a === 5 && c.b === 3))
assert.ok(ended.ended && ended.adjacent)

// No two cards in a row overlap.
for (let g = 0; g <= 2; g++) {
  const xs = [...tree.pos.values()].filter((p) => p.gen === g).map((p) => p.x).sort((a, b) => a - b)
  for (let i = 1; i < xs.length; i++) assert.ok(xs[i] - xs[i - 1] >= CARD_W, `row ${g} overlaps`)
}
assert.ok(Math.min(...[...tree.pos.values()].map((p) => p.x)) === 0, 'starts at x = 0')

// Children sit under the middle of their parents.
const kids = [6, 7, 8].map((id) => center(tree, id))
const kidsMid = (Math.min(...kids) + Math.max(...kids)) / 2
const parentsMid = (center(tree, 3) + center(tree, 5)) / 2
assert.ok(Math.abs(kidsMid - parentsMid) < CARD_W, `children centred (${kidsMid} vs ${parentsMid})`)

// Families group children by their parents.
const fam = tree.families.find((f) => f.parents.join() === '3,5')
assert.deepEqual([...fam.children].sort(), [6, 7, 8])
assert.equal(tree.families.find((f) => f.parents.join() === '1,2').children.length, 2)

// Siblings without parents in the tree get a bracket of their own.
const sibs = layoutFamily(people(1, 2), [L(1, 2, 'sibling')], 1)
assert.deepEqual(sibs.siblingPairs, [{ a: 1, b: 2 }])
assert.equal(sibs.pos.get(1).gen, sibs.pos.get(2).gen)

// Siblings without parents end up next to the sibling who has a family.
const side = layoutFamily(
  people(1, 2, 3, 4, 5, 6, 7),
  [L(1, 2, 'spouse'), L(1, 3, 'parent'), L(2, 3, 'parent'), L(4, 5, 'spouse'), L(4, 6, 'parent'), L(5, 6, 'parent'), L(7, 5, 'sibling')],
  3,
)
const gap = Math.abs(side.pos.get(7).x - side.pos.get(5).x)
assert.ok(gap <= 2 * CARD_W + 80, `sibling placed ${gap}px from her sister`)

// Impossible data (each the other's parent) still lays out.
const loop = layoutFamily(people(1, 2), [L(1, 2, 'parent'), L(2, 1, 'parent')], 1)
assert.ok(Number.isFinite(loop.pos.get(2).x))

// One person alone.
const alone = layoutFamily(people(7), [], 7)
assert.deepEqual(alone.pos.get(7), { x: 0, y: 0, gen: 0 })

console.log('familyLayout: ok')
