package main

import "database/sql"

// When something holds in the story. A character version sits at a point:
// its chapter (by the chapters' story order) and its story date. A relation
// can start and end at a chapter or a date; it holds at a point when both
// ends allow it. An end that can't be compared with the point (no chapter on
// one side, no date on the other) never hides anything. "Until" is the point
// where it stops: friends until chapter 5 and adopted from chapter 5 don't
// overlap. frontend/src/storyTime.js does the same; keep them in step.

type storyPoint struct {
	Chapter    int // story order of the chapter, when HasChapter
	HasChapter bool
	Date       string // canonical story date, or ""
}

// chapterRanks is every chapter's place in the story's order.
func chapterRanks(db *sql.DB) map[int64]int {
	out := map[int64]int{}
	idx, err := loadChapterIndex(db)
	if err != nil {
		return out
	}
	for i, c := range idx.Chapters {
		out[c.ID] = i
	}
	return out
}

func pointOf(chapterID sql.NullInt64, date string, ranks map[int64]int) storyPoint {
	p := storyPoint{}
	if chapterID.Valid {
		if r, ok := ranks[chapterID.Int64]; ok {
			p.Chapter, p.HasChapter = r, true
		}
	}
	if _, ok := parseStoryDate(date); ok {
		p.Date = normalizeStoryDate(date)
	}
	return p
}

// holdsAt reports whether a span (chapters and/or dates at either end) holds
// at point p.
func holdsAt(p storyPoint, ranks map[int64]int, sinceCh, untilCh *int64, since, until string) bool {
	if rank, ok := chapterRank(sinceCh, ranks); ok && p.HasChapter {
		if p.Chapter < rank {
			return false
		}
	} else if since != "" && p.Date != "" && dateBefore(p.Date, since) {
		return false
	}
	if rank, ok := chapterRank(untilCh, ranks); ok && p.HasChapter {
		if p.Chapter >= rank {
			return false
		}
	} else if until != "" && p.Date != "" && !dateBefore(p.Date, until) {
		return false
	}
	return true
}

func chapterRank(id *int64, ranks map[int64]int) (int, bool) {
	if id == nil {
		return 0, false
	}
	r, ok := ranks[*id]
	return r, ok
}
