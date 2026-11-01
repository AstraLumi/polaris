// A small force-directed layout for the wiki's graph view (views/GraphView.vue).
// No dependencies: node frontend/tests/graphLayout.test.mjs
//
// It works like d3-force, simplified: every pair of nodes pushes apart, every
// edge pulls its two ends towards a set length, and a weak pull towards the
// middle keeps loose clusters from drifting away. Each tick moves the nodes a
// little; `alpha` is how hot the layout still is and cools towards zero.
//
// The push is worked out on a quadtree (Barnes–Hut, as d3 does): a far group
// of nodes pushes as one node at its centre, so a tick costs about n log n
// instead of n², which is what kept a big wiki's graph from showing.

const REPULSION = -110 // strength of the push between any two nodes
const MAX_REACH = 900 // pairs further apart than this ignore each other
const THETA2 = 0.81 // a group pushes as one when its width / distance < 0.9
const MAX_DEPTH = 32 // nodes this close together share one leaf
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

  const repel = pushApart()

  function tick() {
    sim.alpha += (sim.alphaTarget - sim.alpha) * ALPHA_DECAY
    const alpha = sim.alpha
    const n = nodes.length

    repel(nodes, alpha)

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

// The push between all the nodes, on a quadtree kept in flat arrays that
// are reused from tick to tick. Cell c is the square at (x0[c], y0[c]) with
// side side[c], holding count[c] nodes centred on (mx[c], my[c]). An inner
// cell's quarters are kid[4c] to kid[4c + 3] (0 for none: the root, cell 0,
// is nobody's quarter); a leaf has kid[4c] = -1, and its nodes are first[c],
// then nextIn[] from one to the next (-1 ends the list).
function pushApart() {
  let cap = 0
  let x0, y0, side, count, mx, my, kid, first
  let px = new Float64Array(0)
  let py = px
  let nextIn = new Int32Array(0)
  let used = 0
  const stack = new Int32Array(4 * MAX_DEPTH + 8)

  function newCell(x, y, s) {
    if (used === cap) {
      cap = Math.max(64, cap * 2)
      const more = (old, T, k = 1) => {
        const a = new T(cap * k)
        if (old) a.set(old)
        return a
      }
      x0 = more(x0, Float64Array)
      y0 = more(y0, Float64Array)
      side = more(side, Float64Array)
      count = more(count, Float64Array)
      mx = more(mx, Float64Array)
      my = more(my, Float64Array)
      kid = more(kid, Int32Array, 4)
      first = more(first, Int32Array)
    }
    const c = used++
    x0[c] = x
    y0[c] = y
    side[c] = s
    count[c] = mx[c] = my[c] = 0
    kid[4 * c] = -1
    kid[4 * c + 1] = kid[4 * c + 2] = kid[4 * c + 3] = 0
    first[c] = -1
    return c
  }

  // The quarter of cell c that node i is in, made if it isn't there yet.
  function quarter(c, i) {
    const half = side[c] / 2
    const right = px[i] >= x0[c] + half
    const down = py[i] >= y0[c] + half
    const q = 4 * c + (down ? 2 : 0) + (right ? 1 : 0)
    if (kid[q] <= 0) {
      const k = newCell(x0[c] + (right ? half : 0), y0[c] + (down ? half : 0), half)
      kid[q] = k // after newCell, which may have grown kid
    }
    return kid[q]
  }

  function add(i) {
    let c = 0
    for (let depth = 0; ; depth++) {
      count[c]++
      mx[c] += px[i]
      my[c] += py[i]
      if (kid[4 * c] !== -1) {
        c = quarter(c, i)
        continue
      }
      const f = first[c]
      if (f < 0 || depth >= MAX_DEPTH || (px[f] === px[i] && py[f] === py[i])) {
        nextIn[i] = f
        first[c] = i
        return
      }
      // A node somewhere else than the ones in this leaf: split it.
      kid[4 * c] = 0
      first[c] = -1
      for (let j = f; j >= 0; ) {
        const after = nextIn[j]
        const k = quarter(c, j)
        count[k]++
        mx[k] += px[j]
        my[k] += py[j]
        nextIn[j] = first[k]
        first[k] = j
        j = after
      }
      c = quarter(c, i)
    }
  }

  return function repelAll(nodes, alpha) {
    const n = nodes.length
    if (!n) return
    if (px.length < n) {
      px = new Float64Array(n)
      py = new Float64Array(n)
      nextIn = new Int32Array(n)
    }
    let minX = Infinity
    let minY = Infinity
    let maxX = -Infinity
    let maxY = -Infinity
    for (let i = 0; i < n; i++) {
      const x = (px[i] = nodes[i].x)
      const y = (py[i] = nodes[i].y)
      if (x < minX) minX = x
      if (x > maxX) maxX = x
      if (y < minY) minY = y
      if (y > maxY) maxY = y
    }
    used = 0
    newCell(minX, minY, Math.max(maxX - minX, maxY - minY, 1) * 1.0001)
    for (let i = 0; i < n; i++) add(i)
    for (let c = 0; c < used; c++) {
      if (count[c]) {
        mx[c] /= count[c]
        my[c] /= count[c]
      }
    }

    const reach2 = MAX_REACH * MAX_REACH
    for (let i = 0; i < n; i++) {
      const ax = px[i]
      const ay = py[i]
      let fx = 0
      let fy = 0
      let sp = 0
      stack[sp++] = 0
      while (sp) {
        const c = stack[--sp]
        // A cell entirely out of reach pushes nothing.
        const cx = x0[c]
        const cy = y0[c]
        const s = side[c]
        const ox = ax < cx ? cx - ax : ax > cx + s ? ax - cx - s : 0
        const oy = ay < cy ? cy - ay : ay > cy + s ? ay - cy - s : 0
        if (ox * ox + oy * oy > reach2) continue
        const k0 = 4 * c
        if (kid[k0] !== -1) {
          const dx = mx[c] - ax
          const dy = my[c] - ay
          const d2 = dx * dx + dy * dy
          // As one only from outside it: a group never holds the node it pushes.
          if ((ox || oy) && s * s < THETA2 * d2) {
            if (d2 <= reach2) {
              fx += (dx * count[c]) / d2
              fy += (dy * count[c]) / d2
            }
            continue
          }
          for (let q = k0; q < k0 + 4; q++) if (kid[q] > 0) stack[sp++] = kid[q]
          continue
        }
        for (let j = first[c]; j >= 0; j = nextIn[j]) {
          if (j === i) continue
          let dx = px[j] - ax
          let dy = py[j] - ay
          let d2 = dx * dx + dy * dy
          if (d2 > reach2) continue
          if (d2 < 1) {
            // Two nodes on the same spot: nudge them apart in a stable way.
            dx = (i - j) * 0.01
            dy = i < j ? 0.01 : -0.01
            d2 = 1
          }
          fx += dx / d2
          fy += dy / d2
        }
      }
      nodes[i].vx += fx * REPULSION * alpha
      nodes[i].vy += fy * REPULSION * alpha
    }
  }
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
