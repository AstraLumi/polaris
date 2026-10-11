// When something holds in the story; the same rules as backend/storytime.go.
// No dependencies: node frontend/tests/storyTime.test.mjs
//
// A character version sits at a point: its chapter (by the chapters' story
// order) and its story date. A relation can start and end at a chapter or a
// date; it holds at a point when both ends allow it. An end that can't be
// compared with the point never hides anything, and "until" is where it
// stops (friends until chapter 5 and adopted from chapter 5 don't overlap).

const DATE_RE = /^(?:(\d{1,3})-(\d{1,3})-)?(-?\d{1,9})$/
function dateKey(text) {
  const m = DATE_RE.exec(String(text || '').trim())
  if (!m) return null
  return [parseInt(m[3], 10), m[2] ? parseInt(m[2], 10) : 1, m[1] ? parseInt(m[1], 10) : 1]
}
function before(a, b) {
  for (let i = 0; i < 3; i++) if (a[i] !== b[i]) return a[i] < b[i]
  return false
}

// chapters: the chapter index's list, already in story order.
export const chapterRanks = (chapters) => new Map((chapters || []).map((c, i) => [c.id, i]))

export function storyPoint(chapterId, date, ranks) {
  const rank = chapterId != null ? ranks.get(chapterId) : undefined
  return { chapter: rank ?? null, date: dateKey(date) }
}

export function holdsAt(point, ranks, { since_chapter_id: sinceCh, until_chapter_id: untilCh, since, until }) {
  if (!point) return true
  const s = sinceCh != null ? ranks.get(sinceCh) : undefined
  const u = untilCh != null ? ranks.get(untilCh) : undefined
  const sinceDate = dateKey(since)
  const untilDate = dateKey(until)
  if (s !== undefined && point.chapter !== null) {
    if (point.chapter < s) return false
  } else if (sinceDate && point.date && before(point.date, sinceDate)) return false
  if (u !== undefined && point.chapter !== null) {
    if (point.chapter >= u) return false
  } else if (untilDate && point.date && !before(point.date, untilDate)) return false
  return true
}
