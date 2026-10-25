// What the chapters' texts name and who speaks in them (the writing
// editor's index, backend/chapterindex.go): "Appears in" lists, a
// character's Lines and a chapter's "Who speaks".
import { t, tr } from './i18n'
import { STORY_API } from './stories'

void [tr('failed to load the chapters'), tr('failed to load the lines'), tr('failed to load the speakers')]

async function request(path, fallback) {
  const res = await fetch(`${STORY_API}${path}`)
  if (!res.ok) throw new Error(t((await res.text()).trim() || fallback))
  return res.json()
}

export const mentionsApi = {
  // [{ chapter_id, title, number, volume_id, volume_title, count }], in reading order.
  appearsIn: (type, id) => request(`/appears/${type}/${id}`, 'failed to load the chapters'),
  // [{ chapter_id, title, number, volume_id, volume_title, position, speaker, manner, text }].
  lines: (characterId) => request(`/characters/${characterId}/lines`, 'failed to load the lines'),
  // [{ character_id, name, lines, words, first }], most lines first.
  speakers: (chapterId) => request(`/chapters/${chapterId}/speakers`, 'failed to load the speakers'),
}

// The writing page at a chapter (and one of its lines).
export function writeLink(row, line) {
  const query = { chapter: String(row.chapter_id) }
  if (line != null) query.line = String(line)
  return { path: `/write/${row.volume_id || 0}`, query }
}
