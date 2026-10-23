// The items the writer opened last, per story, kept on this device only.
// Pure list logic plus localStorage; the router hook that fills it is in
// recentTracker.js. No imports, so node can test it.

export const MAX_RECENT = 8
const KEY = (storyId) => `polaris-recent-${storyId}`

// An entry is { type, id, name, at }. type is 'character', 'event',
// 'chapter' or any other wiki article type (location, faction, lore…).
export const recentKey = (e) => `${e.type}:${e.id}`

// The page that opens the item.
export function recentPath(e) {
  if (e.type === 'character') return `/characters/${e.id}`
  if (e.type === 'event') return `/events/${e.id}`
  if (e.type === 'chapter') return `/chapters/${e.id}`
  return `/wiki/${e.type}/${e.id}`
}

// Keeps what looks like an entry; anything else in storage is dropped.
export function cleanRecent(list, max = MAX_RECENT) {
  if (!Array.isArray(list)) return []
  const seen = new Set()
  const out = []
  for (const e of list) {
    if (!e || typeof e.type !== 'string' || !e.type || (typeof e.id !== 'number' && typeof e.id !== 'string') || e.id === '') continue
    const entry = { type: e.type, id: e.id, name: typeof e.name === 'string' ? e.name : '', at: Number(e.at) || 0 }
    if (seen.has(recentKey(entry))) continue
    seen.add(recentKey(entry))
    out.push(entry)
    if (out.length >= max) break
  }
  return out
}

// The list with `entry` on top: newest first, no duplicates, at most `max`.
export function pushRecent(list, entry, max = MAX_RECENT) {
  const key = recentKey(entry)
  return cleanRecent([entry, ...list.filter((e) => recentKey(e) !== key)], max)
}

// The list without one item (it was deleted).
export function dropRecent(list, type, id) {
  return list.filter((e) => !(e.type === type && String(e.id) === String(id)))
}

export function loadRecent(storyId) {
  if (!storyId) return []
  try {
    return cleanRecent(JSON.parse(localStorage.getItem(KEY(storyId)) || '[]'))
  } catch (e) {
    return []
  }
}

export function saveRecent(storyId, list) {
  if (!storyId) return
  try {
    localStorage.setItem(KEY(storyId), JSON.stringify(list))
  } catch (e) {
    /* can't remember it; the list just starts again next time */
  }
}
