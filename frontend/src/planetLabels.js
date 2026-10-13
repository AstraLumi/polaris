// What the planet's views (PlanetGlobe.vue, PlanetFlatMap.vue) label, by how
// big a hex is on screen (cellPx): far out only the names of kingdoms,
// regions and oceans; closer, capitals, then cities, then every place; and
// close in, the big names step aside.

export const TIERS = {
  areaUntil: 16, // territory names, while a hex is smaller than this
  capital: 3, // capitals from here
  city: 6, // cities from here
  place: 11, // every other place from here
}

// How big a marker's picture is, by rank (place, city, capital); the
// capital's includes its glow.
export const ICON_PX = [10, 22, 34]

// candidates: [{ kind: 'area' | 'marker', key, name, x, y, rank, size, color, ocean }],
// already on screen (x, y in pixels) and facing the viewer. Answers the ones
// to show, most important first (kingdoms and regions by size, then oceans
// by size, then places), dropping names that
// would land on one already placed; a territory whose name doesn't fit is
// left out until zooming in makes room.
export function pickLabels(candidates, cellPx) {
  const order = (c) => (c.kind === 'area' ? (c.ocean ? 2.5 : 3) + c.size / 100 : c.rank)
  const shown = candidates
    .filter((c) => {
      if (c.kind === 'area') return cellPx < TIERS.areaUntil
      return cellPx >= (c.rank === 2 ? TIERS.capital : c.rank === 1 ? TIERS.city : TIERS.place)
    })
    .sort((a, b) => order(b) - order(a))
  const placed = []
  const free = (r) => !placed.some((p) => r.x0 < p.x1 && r.x1 > p.x0 && r.y0 < p.y1 && r.y1 > p.y0)
  const out = []
  for (const c of shown) {
    const px = c.kind === 'area' ? c.size : c.rank === 2 ? 12 : 11
    const w = c.name.length * px * 0.62 + 12
    const icon = c.kind === 'marker' ? ICON_PX[c.rank] : 0
    const top = c.kind === 'area' ? c.y - px / 2 - 3 : c.y + icon / 2 - 2
    const rect = { x0: c.x - w / 2, x1: c.x + w / 2, y0: top, y1: top + px + 8 }
    const named = free(rect)
    if (named) placed.push(rect)
    else if (c.kind === 'area') continue
    out.push({ ...c, named, icon })
    if (out.length >= 400) break
  }
  return out
}

// The size of a territory's name, from how many hexes it covers.
export const areaFontPx = (cells) => Math.round(Math.max(12, Math.min(24, 10 + Math.sqrt(cells) * 0.13)))
