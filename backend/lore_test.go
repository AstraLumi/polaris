package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
)

func createLore(t *testing.T, h http.Handler, story, name string) (int64, int) {
	t.Helper()
	body, ct := form(t, map[string]string{"name": name}, "", "", nil)
	rec := do(t, h, "POST", "/api/s/"+story+"/lore", body, ct)
	var out struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	return out.ID, rec.Code
}

func TestLoreArticles(t *testing.T) {
	mgr, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Lore"})
	app, _ := mgr.open(s.ID)
	res, _ := app.db.Exec(`INSERT INTO locations (name, kind) VALUES ('Aldoria', 'minor')`)
	loc, _ := res.LastInsertId()

	for _, bad := range []string{"", "  ", "a [b]", "x|y"} {
		if _, code := createLore(t, h, s.ID, bad); code != 400 {
			t.Errorf("title %q: got %d, want 400", bad, code)
		}
	}
	id, code := createLore(t, h, s.ID, "The Old Gods")
	if code != 200 || id == 0 {
		t.Fatalf("create: %d", code)
	}

	// It's an article like any other: listed, linkable, with trivia and
	// the user's sections, and its related list is what it links to.
	found := false
	for _, it := range wikiList(t, h, s.ID) {
		if it.Type == "lore" && it.ID == id && it.Name == "The Old Gods" {
			found = true
		}
	}
	if !found {
		t.Fatal("lore article missing from the wiki list")
	}
	a := wikiPut(t, h, s.ID, "lore", id, `{"summary":"Worshipped in [[Aldoria]].","fields":{"trivia":"Old.","history":"dropped"},"sections":[{"title":"Rites","body":"x"}]}`)
	if a == nil {
		t.Fatal("save failed")
	}
	if a.Fields["trivia"] != "Old." || a.Fields["history"] != "" || len(a.Sections) != 1 {
		t.Errorf("saved text: %+v", a)
	}
	if len(a.Groups) != 1 || a.Groups[0].Key != "links_to" || a.Groups[0].Items[0].ID != loc {
		t.Errorf("links_to group: %+v", a.Groups)
	}
	place := wikiGet(t, h, s.ID, "location", loc)
	if len(place.Backlinks) != 1 || place.Backlinks[0].Type != "lore" {
		t.Errorf("backlink from the lore page: %+v", place.Backlinks)
	}

	// Renaming keeps the text; deleting takes it along.
	body, ct := form(t, map[string]string{"name": "The Elder Gods"}, "", "", nil)
	if rec := do(t, h, "PUT", "/api/s/"+s.ID+"/lore/"+strconv.FormatInt(id, 10), body, ct); rec.Code != 200 {
		t.Fatalf("rename: %d %s", rec.Code, rec.Body.String())
	}
	if a := wikiGet(t, h, s.ID, "lore", id); a.Name != "The Elder Gods" || a.Summary == "" {
		t.Errorf("after rename: %q %q", a.Name, a.Summary)
	}
	if rec := do(t, h, "DELETE", "/api/s/"+s.ID+"/lore/"+strconv.FormatInt(id, 10), nil, ""); rec.Code != 200 {
		t.Fatalf("delete: %d", rec.Code)
	}
	if n := wikiCount(t, mgr, s.ID); n != 0 {
		t.Errorf("%d wiki rows left after deleting the lore article", n)
	}
	if rec := do(t, h, "GET", "/api/s/"+s.ID+"/wiki/lore/"+strconv.FormatInt(id, 10), nil, ""); rec.Code != 404 {
		t.Errorf("deleted article: %d", rec.Code)
	}
}

func TestMigrationAddsLoreArticles(t *testing.T) {
	mgr, _ := newTestServer(t)
	id := createStoryDirect(t, mgr, "Old")
	app, _ := mgr.open(id)
	app.db.Exec(`INSERT INTO classes (name) VALUES ('Keeper')`)
	rollBackSchema17(t, app.db)
	if err := ensureSchema(app.db); err != nil {
		t.Fatal(err)
	}
	var n int
	app.db.QueryRow(`SELECT COUNT(*) FROM classes`).Scan(&n)
	if n != 1 {
		t.Fatal("migration must keep existing data")
	}
	if _, err := app.db.Exec(`INSERT INTO lore_articles (id, name) VALUES (1, 'Gods')`); err != nil {
		t.Fatalf("lore table missing after migration: %v", err)
	}
	app.db.Exec(`INSERT INTO wiki_entries (entity_type, entity_id, summary) VALUES ('lore', 1, 'x')`)
	app.db.Exec(`DELETE FROM lore_articles`)
	app.db.QueryRow(`SELECT COUNT(*) FROM wiki_entries`).Scan(&n)
	if n != 0 {
		t.Error("lore delete trigger missing after the migration")
	}
}
