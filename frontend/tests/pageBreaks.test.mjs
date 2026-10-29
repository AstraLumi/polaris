// Where pages break while writing (src/editor/pageBreaks.js): pure rules.
// node frontend/tests/pageBreaks.test.mjs
import assert from 'node:assert/strict'
import { paginate, gapHeight, pageTops } from '../src/editor/pageBreaks.js'

const LH = 10 // a line is 10 px tall, no space between lines
// blocks: { lines, keepNext, gapBefore } | { unit, height } | { hard } → items
function build(blocks) {
  const items = []
  let y = 0
  let pos = 1
  blocks.forEach((b, bi) => {
    y += b.gapBefore || 0
    if (b.unit || b.hard) {
      const h = b.hard ? 0 : b.height
      items.push({ pos, top: y, bottom: y + h, block: bi, first: true, last: true, unit: true, hard: !!b.hard, keepNext: !!b.keepNext, spaceBefore: b.gapBefore || 0 })
      y += h
      pos += 1
      return
    }
    for (let l = 0; l < b.lines; l++) {
      items.push({ pos: pos++, top: y, bottom: y + LH, block: bi, first: l === 0, last: l === b.lines - 1, keepNext: !!b.keepNext, spaceBefore: b.gapBefore || 0 })
      y += LH
    }
    pos += 2
  })
  return items
}
// Which block and which of its lines (0-based) start each new page.
const starts = (items, br) => br.map((b) => ({ block: items[b.index].block, line: items.slice(0, b.index).filter((x) => x.block === items[b.index].block).length }))

// ---- Nothing to do --------------------------------------------------------------
assert.deepEqual(paginate([], 100), [])
assert.deepEqual(paginate(build([{ lines: 5 }]), 100), [])
assert.deepEqual(paginate(build([{ lines: 5 }]), 0), [])

// ---- A line fits when its bottom is within the content height ---------------------
{
  const items = build([{ lines: 30 }]) // pages of 100 px: 10 lines
  const br = paginate(items, 100)
  assert.deepEqual(br.map((b) => b.index), [10, 20], 'ten lines a page')
  assert.equal(items[br[0].index].top, 100)
  assert.ok(!br[0].hard)
  // A hair over is tolerated (rounding), a whole pixel is not.
  assert.deepEqual(paginate(items, 99.7).map((b) => b.index), [10, 20])
  assert.deepEqual(paginate(items, 99).map((b) => b.index), [9, 18, 27])
}

// ---- Widows and orphans ---------------------------------------------------------
{
  // 9 lines, then 5: only one would fit the foot, so the paragraph moves.
  const items = build([{ lines: 9 }, { lines: 5 }])
  assert.deepEqual(starts(items, paginate(items, 100)), [{ block: 1, line: 0 }], 'orphan')
}
{
  // 8 lines, then 5: two fit, three are left: split.
  const items = build([{ lines: 8 }, { lines: 5 }])
  assert.deepEqual(starts(items, paginate(items, 100)), [{ block: 1, line: 2 }])
}
{
  // 8 lines, then 3 or 2: a paragraph of 2-3 lines moves whole.
  for (const [before, n] of [[8, 3], [9, 2], [9, 3]]) {
    const items = build([{ lines: before }, { lines: n }])
    assert.deepEqual(starts(items, paginate(items, 100)), [{ block: 1, line: 0 }], `${n} lines after ${before} move whole`)
  }
}
{
  // 8 lines, then 4: two and two.
  const items = build([{ lines: 8 }, { lines: 4 }])
  assert.deepEqual(starts(items, paginate(items, 100)), [{ block: 1, line: 2 }])
}
{
  // 11 lines on their own: the last would be alone on page 2, so two go.
  const items = build([{ lines: 11 }, { lines: 3 }])
  const br = paginate(items, 100)
  assert.deepEqual(starts(items, br)[0], { block: 0, line: 9 }, 'widow')
}
{
  // A long paragraph: no page of one line.
  const items = build([{ lines: 41 }])
  const br = paginate(items, 100)
  const cuts = [0, ...br.map((b) => b.index), items.length]
  for (let p = 0; p + 1 < cuts.length; p++) assert.ok(cuts[p + 1] - cuts[p] >= 2, 'no page of one line')
}

// ---- keepNext --------------------------------------------------------------------
{
  // A heading at the foot goes with the next paragraph.
  const items = build([{ lines: 8 }, { lines: 1, keepNext: true }, { lines: 5 }])
  assert.deepEqual(starts(items, paginate(items, 100)), [{ block: 1, line: 0 }])
}
{
  // With room for the paragraph's first two lines it stays.
  const items = build([{ lines: 6 }, { lines: 1, keepNext: true }, { lines: 6 }])
  assert.deepEqual(starts(items, paginate(items, 100)), [{ block: 2, line: 3 }])
}
{
  // Two headings in a row move together.
  const items = build([{ lines: 7 }, { lines: 1, keepNext: true }, { lines: 1, keepNext: true }, { lines: 4 }])
  assert.deepEqual(starts(items, paginate(items, 100)), [{ block: 1, line: 0 }])
}
{
  // keepNext never empties a page.
  const items = build([{ lines: 1, keepNext: true }, { lines: 12 }])
  const br = paginate(items, 100)
  assert.equal(items[br[0].index].block, 1)
  assert.equal(br[0].index, 10)
}

// ---- Hard page breaks ---------------------------------------------------------------
{
  const items = build([{ lines: 3 }, { hard: true }, { lines: 3 }, { hard: true }, { lines: 2 }])
  const br = paginate(items, 100)
  assert.deepEqual(br.map((b) => [b.index, b.hard]), [[4, true], [8, true]])
  assert.deepEqual(br.map((b) => b.pos), [items[4].pos, items[8].pos])
  assert.deepEqual(paginate(build([{ lines: 3 }, { hard: true }]), 100), [], 'a break at the very end starts nothing')
}
{
  // The space before stays after a hard break: 20 px, so eight lines fit.
  const items = build([{ lines: 3 }, { hard: true }, { lines: 12, gapBefore: 20 }])
  const br = paginate(items, 100)
  assert.equal(br[0].hard, true)
  assert.equal(br[1].index - br[0].index, 8)
}
{
  // A hard break right where a page ends makes no empty page.
  const items = build([{ lines: 10 }, { hard: true }, { lines: 2 }])
  assert.deepEqual(paginate(items, 100).map((b) => b.index), [11])
}

// ---- Space before is dropped after a natural break ----------------------------------
{
  const items = build([{ lines: 9 }, { lines: 5, gapBefore: 20 }, { lines: 4 }])
  const br = paginate(items, 100)
  assert.ok(br.length >= 1)
  assert.equal(pageTops(items, br, 0)[1], items[br[0].index].top, 'the page begins at the first line, not its space before')
  // Measured from there: a line fits if it is within 100 px of the first line's top.
  const items2 = build([{ lines: 9 }, { lines: 30, gapBefore: 20 }])
  const br2 = paginate(items2, 100)
  assert.equal(br2[1].index - br2[0].index, 10)
}

// ---- Units: pictures and tables -------------------------------------------------------
{
  const items = build([{ lines: 5 }, { unit: true, height: 70 }, { lines: 2 }])
  assert.deepEqual(paginate(items, 100).map((b) => b.index), [5], 'a picture that does not fit starts the next page')
}
{
  // A unit taller than a page starts its own page and overflows; the next item starts another.
  const items = build([{ lines: 3 }, { unit: true, height: 250 }, { lines: 3 }])
  assert.deepEqual(paginate(items, 100).map((b) => b.index), [3, 4])
  assert.deepEqual(paginate(build([{ unit: true, height: 300 }, { lines: 3 }]), 100).map((b) => b.index), [1])
}
{
  // A heading before a picture that does not fit: both move.
  const items = build([{ lines: 8 }, { lines: 1, keepNext: true }, { unit: true, height: 60 }])
  assert.deepEqual(paginate(items, 100).map((b) => b.index), [8])
}

// ---- Stability and the rules together --------------------------------------------------
{
  const blocks = []
  for (let i = 0; i < 80; i++) blocks.push({ lines: 1 + ((i * 7) % 9), keepNext: i % 11 === 0, gapBefore: i % 3 ? 4 : 0 })
  blocks.splice(30, 0, { hard: true })
  blocks.splice(50, 0, { unit: true, height: 45 })
  const items = build(blocks)
  const a = paginate(items, 233)
  assert.deepEqual(a, paginate(items, 233))
  const cuts = [0, ...a.map((x) => x.index), items.length]
  for (let p = 0; p + 1 < cuts.length; p++) {
    const page = items.slice(cuts[p], cuts[p + 1])
    const per = new Map()
    for (const it of page) per.set(it.block, (per.get(it.block) || 0) + 1)
    for (const [blk, n] of per) {
      const all = items.filter((x) => x.block === blk)
      if (n === 1 && all.length > 1) assert.fail(`block ${blk} has one line alone on page ${p}`)
    }
  }
}

// ---- The gap's height ------------------------------------------------------------------------
assert.equal(gapHeight({ w: 80, pageTop: 0, contentHeight: 100, extra: 30 }), 50, 'the rest of the page plus foot, space and head')
assert.equal(gapHeight({ w: 240, pageTop: 0, contentHeight: 100, extra: 30 }), 30, 'overflowing content: no rest of the page')
assert.equal(gapHeight({ w: 80, pageTop: 0, contentHeight: 100, extra: 30, keep: 6 }), 56, 'a space before kept after a hard break')

console.log('pageBreaks: ok')
