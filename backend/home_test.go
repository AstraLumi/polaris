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
