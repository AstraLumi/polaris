package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func wikiGet(t *testing.T, h http.Handler, story, typ string, id int64) wikiArticle {
	t.Helper()
	rec := do(t, h, "GET", "/api/s/"+story+"/wiki/"+typ+"/"+strconv.FormatInt(id, 10), nil, "")
	if rec.Code != 200 {
		t.Fatalf("get %s/%d: %d %s", typ, id, rec.Code, rec.Body.String())
	}
	var a wikiArticle
	json.Unmarshal(rec.Body.Bytes(), &a)
	return a
}

func wikiPut(t *testing.T, h http.Handler, story, typ string, id int64, body string) *wikiArticle {
	t.Helper()
	rec := do(t, h, "PUT", "/api/s/"+story+"/wiki/"+typ+"/"+strconv.FormatInt(id, 10), strings.NewReader(body), "application/json")
	if rec.Code != 200 {
		return nil
	}
	var a wikiArticle
	json.Unmarshal(rec.Body.Bytes(), &a)
	return &a
}

func wikiList(t *testing.T, h http.Handler, story string) []wikiListItem {
	t.Helper()
	rec := do(t, h, "GET", "/api/s/"+story+"/wiki", nil, "")
	var items []wikiListItem
	json.Unmarshal(rec.Body.Bytes(), &items)
	return items
}

func wikiCount(t *testing.T, mgr *storyManager, story string) int {
	t.Helper()
	app, _ := mgr.open(story)
	var n int
	app.db.QueryRow(`SELECT (SELECT COUNT(*) FROM wiki_entries) + (SELECT COUNT(*) FROM wiki_sections) + (SELECT COUNT(*) FROM wiki_infobox)`).Scan(&n)
	return n
}

func TestWikiPullsFromTheStoryAndDeletesWithIt(t *testing.T) {
	mgr, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Wikiworld"})
	app, _ := mgr.open(s.ID)

	mage := addClass(t, h, s.ID, "Mage")
	res, err := app.db.Exec(`INSERT INTO locations (name, description, kind) VALUES ('Aldoria', 'A city', 'minor')`)
	if err != nil {
		t.Fatal(err)
	}
	loc, _ := res.LastInsertId()
	body, ct := form(t, map[string]string{"name": "Aria", "level": "3", "class": "Mage"}, "", "", nil)
	rec := do(t, h, "POST", "/api/s/"+s.ID+"/characters", body, ct)
	var ch struct{ ID int64 }
	json.Unmarshal(rec.Body.Bytes(), &ch)
	res, _ = app.db.Exec(`INSERT INTO events (name, event_date, location_id) VALUES ('The Siege', '01-01-1000', ?)`, loc)
	ev, _ := res.LastInsertId()
	app.db.Exec(`INSERT INTO event_characters (event_id, character_id) VALUES (?, ?)`, ev, ch.ID)
	// A birth/founding detail record is not an article of its own.
	app.db.Exec(`INSERT INTO events (name, event_date, source_type, source_id) VALUES ('', '', 'character', ?)`, ch.ID)

	// Nothing was created by hand, yet everything is there.
	got := map[string]bool{}
	for _, it := range wikiList(t, h, s.ID) {
		got[it.Type+":"+it.Name] = true
		if it.Written {
			t.Errorf("%s should have no wiki text yet", it.Name)
		}
	}
	for _, want := range []string{"character:Aria", "location:Aldoria", "event:The Siege", "class:Mage"} {
		if !got[want] {
			t.Errorf("missing article %s in %v", want, got)
		}
	}
	if len(got) != 4 {
		t.Errorf("expected 4 articles, got %v", got)
	}

	// The character article shows its class and the events it was in.
	a := wikiGet(t, h, s.ID, "character", ch.ID)
	foundClass := false
	for _, f := range a.Facts {
		if f.Key == "class" && f.Link != nil && f.Link.ID == mage {
			foundClass = true
		}
	}
	if !foundClass {
		t.Errorf("character facts missing class link: %+v", a.Facts)
	}
	if len(a.Groups) != 1 || a.Groups[0].Key != "events" || a.Groups[0].Items[0].Name != "The Siege" {
		t.Errorf("groups: %+v", a.Groups)
	}

	// Add wiki text, including a link to the location.
	saved := wikiPut(t, h, s.ID, "character", ch.ID,
		`{"summary":"Born near [[Aldoria]].","fields":{"personality":"Calm","bogus":"x"},
		  "sections":[{"title":"Rumours","body":"Heard in [[location:aldoria|the city]]"},{"title":"","body":""}],
		  "infobox":[{"label":"Eyes","value":"Green"}]}`)
	if saved == nil {
		t.Fatal("save failed")
	}
	if saved.Fields["personality"] != "Calm" || len(saved.Fields) != 1 {
		t.Errorf("fields: %v", saved.Fields)
	}
	if len(saved.Sections) != 1 || len(saved.Infobox) != 1 {
		t.Errorf("sections/infobox: %+v %+v", saved.Sections, saved.Infobox)
	}
	if bl := wikiGet(t, h, s.ID, "location", loc).Backlinks; len(bl) != 1 || bl[0].Name != "Aria" {
		t.Errorf("backlinks: %+v", bl)
	}
	written := 0
	for _, it := range wikiList(t, h, s.ID) {
		if it.Written {
			written++
		}
	}
	if written != 1 {
		t.Errorf("written = %d", written)
	}

	// Clearing every field removes the entry; saving again then deleting the
	// source removes it too.
	if wikiPut(t, h, s.ID, "character", ch.ID, `{}`) == nil || wikiCount(t, mgr, s.ID) != 0 {
		t.Error("an empty save should remove the wiki entry")
	}
	wikiPut(t, h, s.ID, "character", ch.ID, `{"summary":"x","sections":[{"title":"A","body":"b"}],"infobox":[{"label":"l","value":"v"}]}`)
	wikiPut(t, h, s.ID, "location", loc, `{"summary":"y"}`)
	wikiPut(t, h, s.ID, "class", mage, `{"summary":"z"}`)
	if wikiCount(t, mgr, s.ID) != 5 {
		t.Fatalf("expected 5 rows, got %d", wikiCount(t, mgr, s.ID))
	}
	do(t, h, "DELETE", "/api/s/"+s.ID+"/characters", strings.NewReader(`{"ids":[`+strconv.FormatInt(ch.ID, 10)+`]}`), "application/json")
	if n := wikiCount(t, mgr, s.ID); n != 2 {
		t.Errorf("deleting the character should drop its 3 wiki rows, %d left", n)
	}
	do(t, h, "DELETE", "/api/s/"+s.ID+"/classes/"+strconv.FormatInt(mage, 10), nil, "")
	app.db.Exec(`DELETE FROM locations WHERE id = ?`, loc)
	if n := wikiCount(t, mgr, s.ID); n != 0 {
		t.Errorf("everything should be gone, %d left", n)
	}
}

func TestWikiRejectsBadInput(t *testing.T) {
	mgr, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "W"})
	app, _ := mgr.open(s.ID)
	res, _ := app.db.Exec(`INSERT INTO races (name) VALUES ('Elf')`)
	id, _ := res.LastInsertId()
	if wikiPut(t, h, s.ID, "race", id+99, `{"summary":"x"}`) != nil {
		t.Error("unknown source should 404")
	}
	if wikiPut(t, h, s.ID, "nonsense", id, `{"summary":"x"}`) != nil {
		t.Error("unknown type should 404")
	}
	if wikiPut(t, h, s.ID, "race", id, `{"sections":[{"title":"","body":"text"}]}`) != nil {
		t.Error("a section needs a title")
	}
	if wikiPut(t, h, s.ID, "race", id, `{"summary":"`+strings.Repeat("x", maxWikiText+1)+`"}`) != nil {
		t.Error("over-long text should be refused")
	}
	if wikiPut(t, h, s.ID, "race", id, `not json`) != nil {
		t.Error("bad json should be refused")
	}
}

func TestWikiLinkResolution(t *testing.T) {
	idx := wikiNameIndex([]wikiListItem{
		{Type: "class", ID: 1, Name: "Dragon"},
		{Type: "race", ID: 2, Name: "Dragon"},
		{Type: "gear", ID: 3, Name: "Iron  Helm"},
	})
	cases := map[string]string{
		"Dragon": "class:1", "race:dragon": "race:2", "Race: Dragon": "race:2",
		"iron helm": "gear:3", "body type:Dragon": "", "Nobody": "",
	}
	for in, want := range cases {
		got := ""
		if r := resolveWikiLink(in, idx); r != nil {
			got = r.Type + ":" + strconv.FormatInt(r.ID, 10)
		}
		if got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
}

func TestMigrationAddsWikiTables(t *testing.T) {
	mgr, _ := newTestServer(t)
	id := createStoryDirect(t, mgr, "Old")
	app, _ := mgr.open(id)
	app.db.Exec(`INSERT INTO classes (name) VALUES ('Keeper')`)
	drops := []string{`DROP TABLE wiki_infobox`, `DROP TABLE wiki_sections`, `DROP TABLE wiki_entries`}
	for _, wt := range wikiTypes {
		drops = append(drops, `DROP TRIGGER wiki_cleanup_`+wt.Table)
	}
	drops = append(drops, `ALTER TABLE locations DROP COLUMN is_city`, `ALTER TABLE locations DROP COLUMN is_capital`)
	drops = append(drops, `ALTER TABLE character_story DROP COLUMN status`, `UPDATE schema_meta SET value = '12' WHERE key = 'schema_version'`)
	for _, q := range drops {
		if _, err := app.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err := ensureSchema(app.db); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM wiki_entries`).Scan(&n); err != nil {
		t.Fatalf("wiki table missing after migration: %v", err)
	}
	app.db.QueryRow(`SELECT COUNT(*) FROM classes`).Scan(&n)
	if n != 1 {
		t.Fatal("migration must keep existing data")
	}
	// The cleanup triggers exist again.
	app.db.Exec(`INSERT INTO wiki_entries (entity_type, entity_id, summary) VALUES ('class', 1, 'x')`)
	app.db.Exec(`DELETE FROM classes`)
	app.db.QueryRow(`SELECT COUNT(*) FROM wiki_entries`).Scan(&n)
	if n != 0 {
		t.Error("delete trigger not recreated by the migration")
	}
}
