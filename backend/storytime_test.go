package main

import "testing"

// chapterRanks sorts the chapters itself (without the counts the chapter
// list reads); it must give them in the list's order.
func TestChapterRanksFollowTheChapterList(t *testing.T) {
	mgr, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Ranks"})
	app, _ := mgr.open(s.ID)
	for _, q := range []string{
		`INSERT INTO volumes (id, title, position) VALUES (1, 'Two', 2), (2, 'One', 1)`,
		`INSERT INTO chapters (id, volume_id, title, position) VALUES
			(1, 1, 'b', 1), (2, 1, 'a', 0), (3, NULL, 'loose', 5), (4, 2, 'c', 3), (5, 2, 'd', 3), (6, NULL, 'first', 0)`,
	} {
		if _, err := app.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	idx, err := loadChapterIndex(app.db)
	if err != nil {
		t.Fatal(err)
	}
	ranks := chapterRanks(app.db)
	if len(ranks) != len(idx.Chapters) {
		t.Fatalf("ranks %v for %d chapters", ranks, len(idx.Chapters))
	}
	for i, c := range idx.Chapters {
		if ranks[c.ID] != i {
			t.Errorf("chapter %d: rank %d, listed %d", c.ID, ranks[c.ID], i)
		}
	}
}
