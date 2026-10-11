// The graph view's force layout. No dependencies: node frontend/tests/graphLayout.test.mjs
import assert from 'node:assert/strict'
import { createSimulation, bounds, nodeRadius } from '../src/graphLayout.js'

const run = (nodes, edges, ticks = 300) => {
  const sim = createSimulation(nodes, edges)
  for (let i = 0; i < ticks; i++) sim.tick()
  return sim
}
const dist = (a, b) => Math.hypot(a.x - b.x, a.y - b.y)

// Two triangles joined by one edge, and a node on its own.
const nodes = Array.from({ length: 7 }, (_, i) => ({ id: i }))
const edges = [
  [0, 1], [1, 2], [2, 0],
  [3, 4], [4, 5], [5, 3],
  [2, 3],
].map(([source, target]) => ({ source, target }))
const sim = run(nodes, edges)

for (const n of nodes) assert.ok(Number.isFinite(n.x) && Number.isFinite(n.y), 'positions stay finite')
assert.ok(sim.cool, 'the layout cools down')

// Linked nodes end up near each other; the triangles stay apart from each other.
const within = (dist(nodes[0], nodes[1]) + dist(nodes[3], nodes[4])) / 2
const across = (dist(nodes[0], nodes[4]) + dist(nodes[1], nodes[5])) / 2
assert.ok(within < across, `linked ${within.toFixed(1)} should be closer than unlinked ${across.toFixed(1)}`)
assert.ok(within > 20 && within < 120, `link length ${within.toFixed(1)}`)
// Nothing sits on top of anything else.
for (let i = 0; i < nodes.length; i++) for (let j = i + 1; j < nodes.length; j++) assert.ok(dist(nodes[i], nodes[j]) > 10)

// A dragged (fixed) node stays where it's put.
const pinned = [{ fx: 300, fy: -200 }, {}]
run(pinned, [{ source: 0, target: 1 }], 50)
assert.equal(pinned[0].x, 300)
assert.equal(pinned[0].y, -200)

// Rebuilding keeps positions that already exist.
const kept = [{ x: 5, y: 6 }]
createSimulation(kept, [])
assert.equal(kept[0].x, 5)

// Many nodes on the same spot get pushed apart instead of producing NaN.
const stacked = Array.from({ length: 5 }, () => ({ x: 0, y: 0 }))
run(stacked, [], 100)
for (const n of stacked) assert.ok(Number.isFinite(n.x))
assert.ok(new Set(stacked.map((n) => Math.round(n.x * 100))).size > 1)

assert.deepEqual(bounds([{ x: 1, y: 2 }, { x: -3, y: 5 }]), { minX: -3, maxX: 1, minY: 2, maxY: 5 })
assert.equal(bounds([]), null)
assert.ok(nodeRadius(0) < nodeRadius(9) && nodeRadius(10000) === 18)

console.log('graphLayout: ok')
