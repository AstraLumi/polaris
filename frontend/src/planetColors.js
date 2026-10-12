// What colour a cell of the planet is, shared by the map (PlanetMap.vue)
// and the solar system's main planet: its kingdom's or region's, land when
// something without a colour is there, or '' for the sea. An ocean leaves
// its water as it is (the map outlines it instead).

export const LAND = '#7c7f74' // land with no kingdom, region or ocean on it

// ids: the major locations on the cell; locById: id -> location; hasMinor:
// whether a place stands on it.
export function cellColor(ids, locById, hasMinor = false) {
  if (ids && ids.length) {
    for (const id of ids) {
      const l = locById.get(id)
      if (l && l.color) return l.realm === 'ocean' ? '' : l.color
    }
    return LAND
  }
  return hasMinor ? LAND : ''
}

// A colorOf(cell) for a whole planet as GET /api/planet sends it.
export function planetColorOf(data) {
  const locById = new Map(data.locations.map((l) => [l.id, l]))
  const members = new Map()
  for (const [id, c] of data.cells) {
    if (!members.has(c)) members.set(c, [])
    members.get(c).push(id)
  }
  const placed = new Set(data.locations.filter((l) => l.kind === 'minor' && l.cell != null).map((l) => l.cell))
  return (cell) => cellColor(members.get(cell), locById, placed.has(cell))
}
