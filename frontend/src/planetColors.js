// What colour a cell of the planet is, shared by the map (PlanetMap.vue)
// and the solar system's main planet: its kingdom's or region's, land when
// something without a colour is there, or '' for the sea. An ocean leaves
// its water as it is (the map outlines it instead), or tints it.

export const LAND = '#7c7f74' // land with no kingdom, region or ocean on it
export const SEA = '#1b2d4a' // open sea, as the globe and the flat map draw it

// The sea with some of an ocean's colour in it.
export function oceanTint(color, amount = 0.3) {
  const a = parseInt(SEA.slice(1), 16)
  const b = parseInt(color.slice(1), 16)
  const ch = (s) => Math.round(((a >> s) & 255) * (1 - amount) + ((b >> s) & 255) * amount)
  return '#' + ((ch(16) << 16) | (ch(8) << 8) | ch(0)).toString(16).padStart(6, '0')
}

// ids: the major locations on the cell; locById: id -> location; hasMinor:
// whether a place stands on it; tint: whether oceans tint their water.
export function cellColor(ids, locById, hasMinor = false, tint = false) {
  if (ids && ids.length) {
    for (const id of ids) {
      const l = locById.get(id)
      if (l && l.color) return l.realm !== 'ocean' ? l.color : tint ? oceanTint(l.color) : ''
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
