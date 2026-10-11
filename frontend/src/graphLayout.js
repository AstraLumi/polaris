// A small force-directed layout for the wiki's graph view (views/GraphView.vue).
// No dependencies: node frontend/tests/graphLayout.test.mjs
//
// It works like d3-force, simplified: every pair of nodes pushes apart, every
// edge pulls its two ends towards a set length, and a weak pull towards the
// middle keeps loose clusters from drifting away. Each tick moves the nodes a
// little; `alpha` is how hot the layout still is and cools towards zero.

const REPULSION = -110 // strength of the push between any two nodes
const MAX_REACH = 900 // pairs further apart than this ignore each other
const LINK_LENGTH = 70
const GRAVITY = 0.03
const VELOCITY_DECAY = 0.42
const ALPHA_MIN = 0.002
const ALPHA_DECAY = 1 - Math.pow(ALPHA_MIN, 1 / 300) // cools in about 300 ticks

// nodes: objects (kept, and given x, y, vx, vy; existing x/y are reused, so
// rebuilding after a filter change doesn't scramble the picture).
// edges: [{ source, target }] as indexes into nodes.
export function createSimulation(nodes, edges) {
  const golden = Math.PI * (3 - Math.sqrt(5))
  nodes.forEach((n, i) => {
    if (!Number.isFinite(n.x) || !Number.isFinite(n.y)) {
      const r = 12 * Math.sqrt(0.5 + i)
      n.x = r * Math.cos(i * golden)
      n.y = r * Math.sin(i * golden)
    }
    n.vx = 0
    n.vy = 0
  })

  const count = new Array(nodes.length).fill(0)
  for (const e of edges) {
    count[e.source]++
    count[e.target]++
  }
  const links = edges.map((e) => ({
    s: e.source,
    t: e.target,
    bias: count[e.source] / (count[e.source] + count[e.target]),
    strength: 1 / Math.min(count[e.source], count[e.target]),
  }))

  const sim = {
    nodes,
    alpha: 1,
    alphaTarget: 0,
    tick,
    // Warm the layout up again (after a drag or a change).
    reheat(a = 0.5) {
      sim.alpha = Math.max(sim.alpha, a)
    },
    get cool() {
      return sim.alpha < ALPHA_MIN && sim.alphaTarget === 0
    },
  }

  function tick() {
    sim.alpha += (sim.alphaTarget - sim.alpha) * ALPHA_DECAY
    const alpha = sim.alpha
    const n = nodes.length

    for (let i = 0; i < n; i++) {
      const a = nodes[i]
      for (let j = i + 1; j < n; j++) {
        const b = nodes[j]
        let dx = b.x - a.x
        let dy = b.y - a.y
        let d2 = dx * dx + dy * dy
        if (d2 > MAX_REACH * MAX_REACH) continue
        if (d2 < 1) {
          // Two nodes on the same spot: nudge them apart in a stable way.
          dx = (i - j) * 0.01 || 0.01
          dy = 0.01
          d2 = 1
        }
        const f = (REPULSION * alpha) / d2
        a.vx += dx * f
        a.vy += dy * f
        b.vx -= dx * f
        b.vy -= dy * f
      }
    }

    for (const l of links) {
      const s = nodes[l.s]
      const t = nodes[l.t]
      const dx = t.x + t.vx - s.x - s.vx || 0.01
      const dy = t.y + t.vy - s.y - s.vy || 0.01
      const d = Math.sqrt(dx * dx + dy * dy)
      const k = ((d - LINK_LENGTH) / d) * alpha * l.strength
      t.vx -= dx * k * l.bias
      t.vy -= dy * k * l.bias
      s.vx += dx * k * (1 - l.bias)
      s.vy += dy * k * (1 - l.bias)
    }

    for (let i = 0; i < n; i++) {
      const p = nodes[i]
      // An article with no connections is held closer, or the push from
      // everything else sends it far out to the edge.
      const g = GRAVITY * alpha * (count[i] ? 1 : 4)
      p.vx -= p.x * g
      p.vy -= p.y * g
      if (p.fx != null) {
        p.x = p.fx
        p.y = p.fy
        p.vx = 0
        p.vy = 0
      } else {
        p.vx *= 1 - VELOCITY_DECAY
        p.vy *= 1 - VELOCITY_DECAY
        p.x += p.vx
        p.y += p.vy
      }
    }
  }

  return sim
}

// The box around some nodes: { minX, maxX, minY, maxY }, or null.
export function bounds(nodes) {
  if (!nodes.length) return null
  let minX = Infinity
  let maxX = -Infinity
  let minY = Infinity
  let maxY = -Infinity
  for (const n of nodes) {
    minX = Math.min(minX, n.x)
    maxX = Math.max(maxX, n.x)
    minY = Math.min(minY, n.y)
    maxY = Math.max(maxY, n.y)
  }
  return { minX, maxX, minY, maxY }
}

// A node's size on the graph: bigger the more it connects to.
export const nodeRadius = (degree) => Math.min(18, 4 + Math.sqrt(degree) * 2.4)
