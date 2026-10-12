// The bestiary's tags and how a creature's levels read (backend/creatures.go).

import { t, tr } from './i18n'

export const CREATURE_KINDS = [
  { id: 'monster', label: tr('Monster') },
  { id: 'animal', label: tr('Animal') },
  { id: 'construct', label: tr('Construct') },
]

export const kindLabel = (id) => t(CREATURE_KINDS.find((k) => k.id === id)?.label || id)

// "5–12", "5+", "≤ 12" or "" (as the wiki infobox shows it).
export function levelText(c) {
  const lo = c.level_min
  const hi = c.level_max
  if (lo != null && hi != null) return lo === hi ? String(lo) : `${lo}–${hi}`
  if (lo != null) return `${lo}+`
  if (hi != null) return `≤ ${hi}`
  return ''
}

// Every hex something on the map covers, [[q, r]], from the map payload
// ({ locations, hexes: [[location id, q, r]] }): painted areas and the hexes
// towns stand on.
export function mapHexesOf(map) {
  const seen = new Set()
  const out = []
  const add = (q, r) => {
    const k = `${q},${r}`
    if (!seen.has(k)) {
      seen.add(k)
      out.push([q, r])
    }
  }
  for (const [, q, r] of map?.hexes || []) add(q, r)
  for (const l of map?.locations || []) if (l.kind === 'minor' && l.q != null && l.r != null) add(l.q, l.r)
  return out
}

// A place's area, [[q, r]]: every hex of a kingdom or region, or the hex a
// town stands on. What a creature's territory follows until it's painted
// (mirrors territoryOf in backend/creatures.go).
export function areaOf(map, locationId) {
  if (locationId == null) return []
  const hexes = (map?.hexes || []).filter(([id]) => id === locationId).map(([, q, r]) => [q, r])
  if (hexes.length) return hexes
  const l = (map?.locations || []).find((x) => x.id === locationId)
  return l && l.kind === 'minor' && l.q != null && l.r != null ? [[l.q, l.r]] : []
}

// On the planet (planetApi.get(): { locations, cells: [[location id, cell]] }):
// every cell something covers, and a place's area, as for the 2D map above.
export function planetLandOf(planet) {
  const out = new Set((planet?.cells || []).map(([, c]) => c))
  for (const l of planet?.locations || []) if (l.kind === 'minor' && l.cell != null) out.add(l.cell)
  return [...out]
}

export function planetAreaOf(planet, locationId) {
  if (locationId == null) return []
  const cells = (planet?.cells || []).filter(([id]) => id === locationId).map(([, c]) => c)
  if (cells.length) return cells
  const l = (planet?.locations || []).find((x) => x.id === locationId)
  return l && l.kind === 'minor' && l.cell != null ? [l.cell] : []
}
