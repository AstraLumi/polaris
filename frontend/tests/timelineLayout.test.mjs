import { layoutTimeline, autoPxPerYear, minimumWidth, estimateLabelWidth, LANES, DEFAULTS } from '../src/timelineLayout.js'
let failed = 0
const ok = (c, m) => { if (!c) failed++; console.log((c ? 'PASS ' : 'FAIL ') + m) }
const YD = 360
const node = (year, name = 'Event ' + year, month = 1) => ({ key: name + year, name, t: (year - 1) * YD + (month - 1) * 30 })
const W = 1000

// 1. proportional spacing on one row: 1033->1035 is double 1021->1022 (when above the min gap)
{
  const L = layoutTimeline([node(1021), node(1022), node(1033), node(1035)], YD, 1400, { pxPerYear: 60, collapseYears: 0 })
  const [a, b, c, d] = L.nodes
  ok(a.row === 0 && d.row === 0, 'all on first row at wide width')
  const g1 = b.x - a.x, g2 = d.x - c.x
  ok(Math.abs(g2 / g1 - 2) < 0.01, `2 years is double 1 year: ${g2} vs ${g1}`)
}
// 2. min gap: same-year nodes are never closer than minGapPx
{
  const L = layoutTimeline([node(1000), node(1000, 'B'), node(1000, 'C', 2)], YD, W, { pxPerYear: 10 })
  const gaps = L.nodes.slice(1).map((n, i) => Math.abs(n.x - L.nodes[i].x))
  ok(gaps.every((g) => g >= DEFAULTS.minGapPx - 0.01), 'same-date nodes keep min gap: ' + gaps.map((g) => g.toFixed(0)))
}
// 3. a 100-year gap coils instead of padding
{
  const L = layoutTimeline([node(1000), node(1100)], YD, W, { pxPerYear: 40 })
  ok(L.coils.length === 1 && Math.round(L.coils[0].years) === 100, 'one coil of ~100 years')
  const straight = layoutTimeline([node(1000), node(1100)], YD, W, { pxPerYear: 40, collapseYears: 0 })
  ok(L.height < straight.height, `coil keeps the page shorter (${L.height} < ${straight.height})`)
  ok(L.nodes[1].row <= 1, 'next event is close after the coil')
}
// 4. no coil at exactly the threshold, coil just beyond it
{
  ok(layoutTimeline([node(1000), node(1025)], YD, W).coils.length === 0, '25 years is not collapsed')
  ok(layoutTimeline([node(1000), node(1026)], YD, W).coils.length === 1, '26 years is collapsed')
}
// 5. zigzag: rows alternate direction, y grows, path turns at the edges
{
  const nodes = Array.from({ length: 40 }, (_, i) => node(1000 + i * 3))
  const L = layoutTimeline(nodes, YD, W, { pxPerYear: 30 })
  ok(L.rowCount >= 3, 'wraps onto several rows: ' + L.rowCount)
  ok(L.nodes.every((n) => (n.row % 2 === 0 ? n.dir === 1 : n.dir === -1)), 'even rows go right, odd rows go left')
  const monotone = L.nodes.every((n, i) => i === 0 || n.row > L.nodes[i - 1].row || (n.row % 2 === 0 ? n.x >= L.nodes[i - 1].x : n.x <= L.nodes[i - 1].x))
  ok(monotone, 'order along the line is preserved across turns')
  ok(L.nodes.every((n) => n.x >= L.xmin - 0.01 && n.x <= L.xmax + 0.01), 'nodes stay inside the straight run')
  ok(L.nodes.every((n) => n.y === 170 + n.row * 300), 'rows are evenly spaced')
  ok(/ A /.test(L.pathD), 'path contains arcs for the U-turns')
}
// 6. dense data: label lanes never overlap
{
  const names = ['The Founding of the Northern League', 'Short', 'A very long event name that goes on and on', 'War', 'Plague of the Seven Rivers']
  const nodes = Array.from({ length: 120 }, (_, i) => node(1000 + Math.floor(i / 6), names[i % names.length] + ' ' + i, 1 + (i % 6)))
  const L = layoutTimeline(nodes, YD, W, { pxPerYear: 20 })
  let overlaps = 0
  const byLane = {}
  for (const n of L.nodes) (byLane[`${n.row}:${n.lane}`] ||= []).push(n)
  for (const list of Object.values(byLane)) for (let i = 0; i < list.length; i++) for (let j = i + 1; j < list.length; j++) {
    const a = list[i], b = list[j]
    if (Math.abs(a.x - b.x) < (a.labelWidth + b.labelWidth) / 2) overlaps++
  }
  ok(overlaps === 0, `no overlapping labels in a dense run of ${nodes.length} nodes (${overlaps} overlaps)`)
  ok(Math.floor((DEFAULTS.labelMax + 12) / DEFAULTS.minGapPx) < LANES.length, `lane guarantee: at most ${Math.floor((DEFAULTS.labelMax + 12) / DEFAULTS.minGapPx)} blockers < ${LANES.length} lanes`)
}
// 7. coil never straddles a row end: it sits fully inside the run
{
  let bad = 0
  for (let w = 700; w <= 1600; w += 50) {
    const nodes = [node(1000), node(1003), node(1090), node(1093), node(1200), node(1204), node(1320)]
    const L = layoutTimeline(nodes, YD, w, { pxPerYear: 50 })
    for (const c of L.coils) if (c.x - 75 < L.xmin - 0.5 || c.x + 75 > L.xmax + 0.5) bad++
    const xs = L.pathD.match(/L ([\d.-]+) /g).map((m) => parseFloat(m.slice(2)))
    if (xs.some((v) => v < L.xmin - L.opts.coilAdvance || v > L.xmax + L.opts.coilAdvance)) bad++
  }
  ok(bad === 0, 'coils stay inside the row at every width from 700 to 1600')
}
// 8. single node, empty, auto zoom
{
  const one = layoutTimeline([node(1000)], YD, W)
  ok(one.nodes.length === 1 && one.rowCount === 1 && one.pathD.startsWith('M '), 'single node lays out')
  ok(layoutTimeline([], YD, W).nodes.length === 0, 'empty lays out')
  const nodes = Array.from({ length: 30 }, (_, i) => node(1000 + i * 10))
  const px = autoPxPerYear(nodes, YD, 1200)
  const L = layoutTimeline(nodes, YD, 1200, { pxPerYear: px })
  ok(L.rowCount >= 2 && L.rowCount <= 5, `auto zoom gives a few rows (${L.rowCount} rows at ${px.toFixed(1)} px/yr)`)
  ok(minimumWidth() > 600, 'minimum width is sane: ' + minimumWidth())
}
// 9. a custom calendar only changes the scale (year = 10 months * 40 days)
{
  const t = (y) => (y - 1) * 400
  const L = layoutTimeline([{ name: 'a', t: t(1000) }, { name: 'b', t: t(1002) }], 400, 1400, { pxPerYear: 60, collapseYears: 0 })
  ok(Math.abs((L.nodes[1].x - L.nodes[0].x) - 120) < 0.01, 'spacing uses the calendar year length')
}
process.exit(failed ? 1 : 0)
