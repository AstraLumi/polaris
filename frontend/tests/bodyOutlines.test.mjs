// The height chart's body outlines (src/bodyOutlines.js).
// No dependencies: node frontend/tests/bodyOutlines.test.mjs
import assert from 'node:assert/strict'
import { ageStage, sexOf, outlineFor, figure, OUTLINE_AGES, OUTLINE_BUILDS } from '../src/bodyOutlines.js'

// Ages and genders read from the character.
assert.equal(ageStage(null), 'adult')
assert.equal(ageStage(7), 'child')
assert.equal(ageStage(15), 'teen')
assert.equal(ageStage(40), 'adult')
assert.equal(ageStage(81), 'elderly')
assert.equal(sexOf('Male'), 'male')
assert.equal(sexOf(' female '), 'female')
assert.equal(sexOf(''), 'neutral')
assert.equal(sexOf('Nonbinary'), 'neutral')

// The body type's choices win; what it leaves empty comes from the character.
assert.deepEqual(outlineFor(null, { gender: 'Female', age: 70 }), { sex: 'female', age: 'elderly', build: 'average' })
assert.deepEqual(outlineFor({ sex: '', age: 'child', build: 'chubby' }, { gender: 'Male', age: 30 }), { sex: 'male', age: 'child', build: 'chubby' })
assert.deepEqual(outlineFor({ sex: 'female', age: '', build: '' }, { gender: 'Male' }), { sex: 'female', age: 'adult', build: 'average' })

// Every outline is a set of closed paths from the top of the head to the soles,
// and gets wider from slim to heavy.
const numbers = (d) => [...d.matchAll(/-?\d+(?:\.\d+)?(?:e-?\d+)?/g)].map((m) => +m[0])
for (const sex of ['male', 'female', 'neutral']) {
  for (const { key: age } of OUTLINE_AGES) {
    let last = 0
    for (const { key: build } of OUTLINE_BUILDS) {
      const f = figure({ sex, age, build })
      assert.ok(f.shapes.length >= 4, `${sex} ${age} ${build}: shapes`)
      for (const d of f.shapes) {
        assert.ok(/^M.*Z$/.test(d), `${sex} ${age} ${build}: closed path`)
        assert.ok(numbers(d).every(Number.isFinite), `${sex} ${age} ${build}: numbers`)
      }
      const ys = f.shapes.flatMap((d) => numbers(d.replace(/A[^A-Z]*/g, ' ')).filter((_, i) => i % 2 === 1))
      assert.ok(Math.min(...ys) > -0.03 && Math.max(...ys) <= 1.0001, `${sex} ${age} ${build}: height ${Math.min(...ys)}..${Math.max(...ys)}`)
      assert.ok(Math.abs(Math.max(...ys) - 1) < 1e-4, `${sex} ${age} ${build}: stands on y = 1`)
      assert.ok(f.halfWidth > 0.1 && f.halfWidth < 0.35, `${sex} ${age} ${build}: width ${f.halfWidth}`)
      if (build !== 'athletic') {
        assert.ok(f.halfWidth >= last, `${sex} ${age}: ${build} wider than the build before`)
        last = f.halfWidth
      }
    }
  }
}

console.log('bodyOutlines: ok')
