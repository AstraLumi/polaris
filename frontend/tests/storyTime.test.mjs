// When relations hold; mirrors TestHoldsAt in backend/schema20_test.go.
// No dependencies: node frontend/tests/storyTime.test.mjs
import assert from 'node:assert/strict'
import { chapterRanks, storyPoint, holdsAt } from '../src/storyTime.js'

const ranks = chapterRanks([{ id: 10 }, { id: 20 }, { id: 30 }])
const at = (chapterId, date) => storyPoint(chapterId, date, ranks)
const rel = (o) => ({ since_chapter_id: null, until_chapter_id: null, since: '', until: '', ...o })

assert.equal(holdsAt(at(20, ''), ranks, rel({})), true, 'no bounds')
assert.equal(holdsAt(at(10, ''), ranks, rel({ since_chapter_id: 20 })), false, 'before the start chapter')
assert.equal(holdsAt(at(20, ''), ranks, rel({ since_chapter_id: 20 })), true, 'at the start chapter')
assert.equal(holdsAt(at(20, ''), ranks, rel({ until_chapter_id: 20 })), false, 'at the end chapter')
assert.equal(holdsAt(at(10, ''), ranks, rel({ until_chapter_id: 20 })), true, 'before the end chapter')
assert.equal(holdsAt(at(null, '01-01-1200'), ranks, rel({ since_chapter_id: 20, since: '01-01-1250' })), false, 'no chapter: dates decide')
assert.equal(holdsAt(at(null, ''), ranks, rel({ since_chapter_id: 20, until_chapter_id: 30 })), true, 'nothing comparable: shown')
assert.equal(holdsAt(at(null, '1199'), ranks, rel({ since: '01-01-1200' })), false, 'date before start')
assert.equal(holdsAt(at(null, '01-01-1300'), ranks, rel({ until: '01-01-1300' })), false, 'date on the end')
assert.equal(holdsAt(at(null, '05-05-1250'), ranks, rel({ since: '1200', until: '1300' })), true, 'date inside')
assert.equal(holdsAt(null, ranks, rel({ since_chapter_id: 30 })), true, 'no point: everything')
assert.equal(holdsAt(at(99, ''), ranks, rel({ since_chapter_id: 20 })), true, 'unknown chapter on the version')

console.log('storyTime: ok')
