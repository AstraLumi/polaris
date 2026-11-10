package main

import (
	"encoding/json"
	"testing"
)

// Home's loose ends: a character with no height (pointing at its current
// version's form), an undated event, an empty chapter, an unwritten lore
// page; a few of each, the rest counted.
func TestHomeLooseEnds(t *testing.T) {
	app, h, base := planetSetup(t)
	db := app.db
	exec := func(q string, args ...any) int64 {
		t.Helper()
		res, err := db.Exec(q, args...)
		if err != nil {
			t.Fatal(err)
		}
		id, _ := res.LastInsertId()
		return id
	}
	char := exec(`INSERT INTO characters (birth_date) VALUES ('01-01-0001')`)
	v := exec(`INSERT INTO character_versions (character_id, name, is_current) VALUES (?, 'Ann', 1)`, char)
	exec(`INSERT INTO character_story (version_id) VALUES (?)`, v)
	for _, name := range []string{"A", "B", "C", "D", "E"} {
		exec(`INSERT INTO events (name, event_date) VALUES (?, '')`, name)
	}
	exec(`INSERT INTO events (name, event_date) VALUES ('Dated', '01-01-0001')`)
	empty := exec(`INSERT INTO chapters (title) VALUES ('Empty')`)
	exec(`INSERT INTO chapters (title, notes) VALUES ('Noted', 'Some notes')`)
	lore := exec(`INSERT INTO lore_articles (name) VALUES ('Myths')`)
	written := exec(`INSERT INTO lore_articles (name) VALUES ('Gods')`)
	exec(`INSERT INTO wiki_entries (entity_type, entity_id, summary) VALUES ('lore', ?, 'Written')`, written)

	rec := do(t, h, "GET", base+"/home", nil, "")
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	var out homePayload
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	byIssue := map[string][]homeLooseEnd{}
	for _, l := range out.Loose {
		byIssue[l.Issue] = append(byIssue[l.Issue], l)
	}
	if h := byIssue["No height or body type"]; len(h) != 1 || h[0].ID != char || h[0].VersionID != v {
		t.Errorf("height: %+v", h)
	}
	if e := byIssue["No date"]; len(e) != looseEach || out.LooseMore != 5-looseEach {
		t.Errorf("events: %d listed, %d more", len(e), out.LooseMore)
	}
	if c := byIssue["No events, versions or notes"]; len(c) != 1 || c[0].ID != empty {
		t.Errorf("chapters: %+v", c)
	}
	if l := byIssue["Not written yet"]; len(l) != 1 || l[0].ID != lore {
		t.Errorf("lore: %+v", l)
	}
}

// Home says where the writing stands: nothing without a chapter, the first
// chapter in reading order while nothing is written, then the chapter
// written last, with the story's word count.
func TestHomeWriting(t *testing.T) {
	app, h, base := planetSetup(t)
	home := func() homePayload {
		t.Helper()
		rec := do(t, h, "GET", base+"/home", nil, "")
		if rec.Code != 200 {
			t.Fatalf("home: %d %s", rec.Code, rec.Body.String())
		}
		var p homePayload
		if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if p := home(); p.Writing != nil || p.Counts.Chapters != 0 || p.Counts.Words != 0 {
		t.Fatalf("no chapters yet: %+v %+v", p.Writing, p.Counts)
	}

	exec := func(q string, args ...any) int64 {
		t.Helper()
		res, err := app.db.Exec(q, args...)
		if err != nil {
			t.Fatal(err)
		}
		id, _ := res.LastInsertId()
		return id
	}
	loose := exec(`INSERT INTO chapters (title, position) VALUES ('Loose', 0)`)
	vol := exec(`INSERT INTO volumes (title, position) VALUES ('Book one', 0)`)
	exec(`INSERT INTO chapters (title, volume_id, position) VALUES ('Second', ?, 1)`, vol)
	first := exec(`INSERT INTO chapters (title, volume_id, position) VALUES ('First', ?, 0)`, vol)
	if p := home(); p.Writing == nil || p.Writing.ChapterID != first || p.Writing.VolumeID != vol || p.Writing.UpdatedAt != "" || p.Counts.Chapters != 3 {
		t.Fatalf("nothing written: the first chapter of the first book: %+v", p.Writing)
	}

	exec(`INSERT INTO chapter_texts (chapter_id, doc, words, updated_at) VALUES (?, '{}', 120, '2026-01-02 10:00:00')`, first)
	exec(`INSERT INTO chapter_texts (chapter_id, doc, words, updated_at) VALUES (?, '{}', 30, '2026-01-03 10:00:00')`, loose)
	p := home()
	if p.Writing == nil || p.Writing.ChapterID != loose || p.Writing.VolumeID != 0 || p.Writing.Words != 30 || p.Writing.Title != "Loose" {
		t.Errorf("the chapter written last: %+v", p.Writing)
	}
	if p.Counts.Words != 150 {
		t.Errorf("words in the story: %d", p.Counts.Words)
	}
}
