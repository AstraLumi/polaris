// Pure layout for the zigzag timeline. No Vue, no DOM: give it the dated
// nodes (sorted by t) and a width, get back every position and the SVG
// path. Kept separate so the geometry can be tested on its own.
//
// The line runs left to right along a row, makes a U-turn down at the row
// end, runs right to left on the next row, and so on. Positions along the
// line are proportional to time, except:
//  - nodes are never closer than `minGapPx`, so same-year events stay
//    separate dots (this is the only place proportionality gives way);
//  - a gap longer than `collapseYears` is not drawn to scale. It becomes a
//    short zigzag break (the "coil") with a "≈ N years" caption, so a century of nothing
//    doesn't cost pages of scrolling. (collapseYears of 0 turns this off.)
// U-turns are not counted as time: only straight runs are to scale.

export const DEFAULTS = {
  pxPerYear: 40,
  minGapPx: 48, // closest two nodes may sit
  collapseYears: 25,
  rowHeight: 300, // distance between rows; the U-turn radius is half of it
  pad: 40, // outer margin around the whole drawing
  coilLoops: 2, // up-and-down strokes in a coil
  coilAdvance: 48, // how far along the line a coil carries you
  coilAmplitude: 28, // height of the tallest spike above the line
  coilLeadPx: 80, // straight line kept on each side of a coil
  topPad: 170, // first row's distance from the top (room for labels above)
  bottomPad: 190,
  labelMin: 80,
  labelMax: 170,
}

// Label lanes, tried in order. up1/down1 sit close to the line; up2/down2
// are the second tier for crowded stretches.
export const LANES = ['up1', 'down1', 'up2', 'down2']

export function estimateLabelWidth(name, opts = DEFAULTS) {
  const w = String(name || '').length * 6.4 + 14
  return Math.max(opts.labelMin, Math.min(opts.labelMax, w))
}

// The smallest width the drawing is useful at: the turns need room and the
// straight run between them can't shrink to nothing.
export function minimumWidth(opts = {}) {
  const o = { ...DEFAULTS, ...opts }
  return 2 * (o.pad + o.rowHeight / 2) + 320
}

// A coil: a tight zigzag, like the break mark on a chart axis, that
// carries you `advance` px forward. `loops` up-and-down strokes, each a
// little smaller than the one before, then back onto the line. Returns the
// corner points (the last one is back on the line), how far it rises
// above the line (height) and how far it dips below it (depth).
export function coilPoints(x0, y0, dir, advance, loops, amplitude = DEFAULTS.coilAmplitude) {
  const peaks = []
  for (let i = 0; i < loops; i++) {
    const fade = 1 - (0.2 * i) / Math.max(1, loops - 1)
    peaks.push(amplitude * fade, -0.7 * amplitude * fade)
  }
  // Corners are evenly spaced; the climb in and the drop out are a half step.
  const step = advance / peaks.length
  const pts = peaks.map((h, i) => [x0 + dir * step * (i + 0.5), y0 - h])
  pts.push([x0 + dir * advance, y0])
  return {
    points: pts,
    height: Math.max(...peaks),
    depth: -Math.min(...peaks),
  }
}

export function layoutTimeline(nodes, yearDays, width, userOpts = {}) {
  const o = { ...DEFAULTS, ...userOpts }
  const r = o.rowHeight / 2
  const xmin = o.pad + r
  const xmax = Math.max(width - o.pad - r, xmin + 320)

  const d = []
  const coils = []
  const placed = []

  let x = xmin
  let y = o.topPad
  let dir = 1
  let row = 0
  const f = (n) => Math.round(n * 100) / 100

  d.push(`M ${f(x)} ${f(y)}`)

  function turn() {
    const sweep = dir > 0 ? 1 : 0
    d.push(`A ${r} ${r} 0 0 ${sweep} ${f(x)} ${f(y + 2 * r)}`)
    y += 2 * r
    dir = -dir
    row++
  }

  // Walk `len` px along the line, turning at row ends as needed.
  function advance(len) {
    while (len > 0.001) {
      const room = dir > 0 ? xmax - x : x - xmin
      if (len <= room + 0.001) {
        x += dir * len
        d.push(`L ${f(x)} ${f(y)}`)
        return
      }
      if (room > 0.001) {
        x = dir > 0 ? xmax : xmin
        d.push(`L ${f(x)} ${f(y)}`)
        len -= room
      }
      turn()
    }
  }

  // A coil is drawn whole on one row: if it doesn't fit before the row
  // ends, run out the row, turn, and coil on the next.
  function coil(years) {
    advance(o.coilLeadPx)
    const room = dir > 0 ? xmax - x : x - xmin
    if (room < o.coilAdvance) {
      if (room > 0.001) {
        x = dir > 0 ? xmax : xmin
        d.push(`L ${f(x)} ${f(y)}`)
      }
      turn()
    }
    const { points, height, depth } = coilPoints(x, y, dir, o.coilAdvance, o.coilLoops, o.coilAmplitude)
    for (const [px, py] of points) d.push(`L ${f(px)} ${f(py)}`)
    const startX = x
    x += dir * o.coilAdvance
    coils.push({
      x: (startX + x) / 2,
      y,
      row,
      dir,
      years,
      height,
      depth,
    })
    advance(o.coilLeadPx)
  }

  for (let i = 0; i < nodes.length; i++) {
    const n = nodes[i]
    if (i > 0) {
      const gapYears = (n.t - nodes[i - 1].t) / yearDays
      if (o.collapseYears > 0 && gapYears > o.collapseYears) {
        coil(gapYears)
      } else {
        advance(Math.max(o.minGapPx, gapYears * o.pxPerYear))
      }
    }
    placed.push({ ...n, x, y, row, dir })
  }

  // ---- Label lanes ----------------------------------------------------------
  // Per row, give each label the first lane where it doesn't overlap one
  // already there. A label (at most labelMax wide, plus 12px of breathing
  // room) can only be blocked by earlier nodes within that distance; at
  // minGapPx spacing that is at most floor((labelMax + 12) / minGapPx) = 3
  // of them, so one of the four lanes is always free, however crowded.
  const taken = new Map() // `${row}:${lane}` -> [[left, right], ...]
  for (const p of placed) {
    const w = estimateLabelWidth(p.name, o)
    const left = p.x - w / 2 - 6
    const right = p.x + w / 2 + 6
    let chosen = null
    for (const lane of LANES) {
      const key = `${p.row}:${lane}`
      const list = taken.get(key) || []
      if (!list.some(([l, rr]) => left < rr && right > l)) {
        chosen = lane
        taken.set(key, [...list, [left, right]])
        break
      }
    }
    if (!chosen) {
      chosen = LANES[0] // unreachable at default spacing; keep going rather than drop a node
    }
    p.lane = chosen
    p.labelWidth = w
  }

  const height = y + o.bottomPad
  return {
    width,
    height,
    pathD: d.join(' '),
    nodes: placed,
    coils,
    rowCount: row + 1,
    xmin,
    xmax,
    radius: r,
    opts: o,
  }
}

// A starting zoom that puts the whole timeline in roughly three rows, so a
// first visit shows the shape of the story rather than a speck or a mile of
// scrolling. Gaps that will be coiled don't count toward the length.
export function autoPxPerYear(nodes, yearDays, width, userOpts = {}) {
  const o = { ...DEFAULTS, ...userOpts }
  const r = o.rowHeight / 2
  const run = Math.max(width - 2 * (o.pad + r), 320)
  let years = 0
  for (let i = 1; i < nodes.length; i++) {
    const g = (nodes[i].t - nodes[i - 1].t) / yearDays
    if (!(o.collapseYears > 0 && g > o.collapseYears)) years += g
  }
  if (years <= 0) return o.pxPerYear
  const px = (run * 3) / years
  return Math.max(2, Math.min(400, px))
}
