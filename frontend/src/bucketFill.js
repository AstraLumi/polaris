// The map's bucket tool: the connected empty hexes around the one clicked.
// Hexes with anything on them are never filled; they are the area's edge.
// Used by the 2D map (keys "q,r") and the planet (cell numbers) alike.

// Filling more hexes than this asks first.
export const BUCKET_WARN = 500
// One paint request can't hold more (backend maxHexBatch).
export const BUCKET_MAX = 100000

// Every empty hex reachable from `start` through empty neighbours. Stops
// early, with `tooBig`, past `max` hexes, and with `open` when a hex the
// area reaches is one `escapes` says is outside the map (an area of the
// endless 2D map that isn't closed in).
export function floodFill(start, { neighbors, empty, escapes = () => false, max = BUCKET_MAX }) {
  if (!empty(start)) return { cells: [], open: false, tooBig: false }
  const seen = new Set([start])
  const queue = [start]
  for (let i = 0; i < queue.length; i++) {
    const h = queue[i]
    if (escapes(h)) return { cells: [], open: true, tooBig: false }
    for (const n of neighbors(h)) {
      if (seen.has(n) || !empty(n)) continue
      seen.add(n)
      queue.push(n)
      if (queue.length > max) return { cells: [], open: false, tooBig: true }
    }
  }
  return { cells: queue, open: false, tooBig: false }
}
