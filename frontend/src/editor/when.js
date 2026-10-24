// Times from the writing routes ("2026-10-24 10:00:00" in UTC as SQLite
// writes them, an RFC 3339 stamp, or milliseconds) as "5m ago" and as the
// full local date and time.
import { timeAgo, exactTime } from '../timeAgo'

function toDate(stamp) {
  if (stamp == null || stamp === '') return null
  if (typeof stamp === 'number') return new Date(stamp)
  const s = String(stamp)
  const d = /[zZ]|[+-]\d\d:?\d\d$/.test(s) ? new Date(s) : new Date(s.replace(' ', 'T') + 'Z')
  return Number.isNaN(d.getTime()) ? null : d
}

const sqlite = (d) => d.toISOString().slice(0, 19).replace('T', ' ')

export const ago = (stamp) => (toDate(stamp) ? timeAgo(sqlite(toDate(stamp))) : '')
export const exact = (stamp) => (toDate(stamp) ? exactTime(sqlite(toDate(stamp))) : '')
