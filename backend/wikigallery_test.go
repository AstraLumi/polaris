package main

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func TestWikiGallery(t *testing.T) {
	mgr, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Gallery"})
	app, _ := mgr.open(s.ID)
	res, _ := app.db.Exec(`INSERT INTO locations (name, kind) VALUES ('Aldoria', 'major')`)
	loc, _ := res.LastInsertId()
	base := "/api/s/" + s.ID + "/wiki/location/" + strconv.FormatInt(loc, 10)
	png := []byte("\x89PNG\r\n\x1a\nfake")

	add := func(caption string) []galleryItem {
		body, ct := form(t, map[string]string{"caption": caption}, "pictures", "view.png", png)
		rec := do(t, h, "POST", base+"/gallery", body, ct)
		if rec.Code != 200 {
			t.Fatalf("add: %d %s", rec.Code, rec.Body.String())
		}
		var g []galleryItem
		json.Unmarshal(rec.Body.Bytes(), &g)
		return g
	}
	g := add("")
	g = add("The harbour at dawn")
	if len(g) != 2 || g[1].Caption != "The harbour at dawn" || !strings.HasPrefix(g[0].Picture, "/uploads/") {
		t.Fatalf("gallery: %+v", g)
	}

	// A gallery alone makes the article written, and the article carries it.
	a := wikiGet(t, h, s.ID, "location", loc)
	if len(a.Gallery) != 2 {
		t.Errorf("article gallery: %+v", a.Gallery)
	}

	// Reorder, recaption.
	rec := do(t, h, "PUT", base+"/gallery", jsonBody(map[string]any{"ids": []int64{g[1].ID, g[0].ID}}), "application/json")
	json.Unmarshal(rec.Body.Bytes(), &g)
	if g[0].Caption != "The harbour at dawn" {
		t.Errorf("order: %+v", g)
	}
	rec = do(t, h, "PUT", "/api/s/"+s.ID+"/wiki-gallery/"+strconv.FormatInt(g[1].ID, 10), jsonBody(map[string]any{"caption": " Walls "}), "application/json")
	json.Unmarshal(rec.Body.Bytes(), &g)
	if g[1].Caption != "Walls" {
		t.Errorf("caption: %+v", g)
	}
	long := strings.Repeat("x", maxCaption+1)
	if rec := do(t, h, "PUT", "/api/s/"+s.ID+"/wiki-gallery/"+strconv.FormatInt(g[1].ID, 10), jsonBody(map[string]any{"caption": long}), "application/json"); rec.Code != 400 {
		t.Errorf("long caption: %d", rec.Code)
	}

	// Clearing the text keeps the pictures; removing the last picture of an
	// article with no text leaves nothing behind.
	wikiPut(t, h, s.ID, "location", loc, `{"summary":"Some text"}`)
	if a := wikiPut(t, h, s.ID, "location", loc, `{"summary":""}`); a == nil || len(a.Gallery) != 2 {
		t.Fatalf("clearing the text lost the gallery: %+v", a)
	}
	for _, it := range g {
		do(t, h, "DELETE", "/api/s/"+s.ID+"/wiki-gallery/"+strconv.FormatInt(it.ID, 10), nil, "")
	}
	if n := wikiCount(t, mgr, s.ID); n != 0 {
		t.Errorf("%d wiki rows left after removing every picture", n)
	}

	// Deleting the place takes its pictures along.
	add("")
	app.db.Exec(`DELETE FROM locations WHERE id = ?`, loc)
	var left int
	app.db.QueryRow(`SELECT COUNT(*) FROM wiki_gallery`).Scan(&left)
	if left != 0 {
		t.Errorf("%d gallery rows left after deleting the place", left)
	}

	if rec := do(t, h, "POST", "/api/s/"+s.ID+"/wiki/location/999/gallery", nil, ""); rec.Code != 404 && rec.Code != 400 {
		t.Errorf("missing article: %d", rec.Code)
	}
}

func TestMigrationAddsGallery(t *testing.T) {
	mgr, _ := newTestServer(t)
	id := createStoryDirect(t, mgr, "Old")
	app, _ := mgr.open(id)
	app.db.Exec(`INSERT INTO wiki_entries (id, entity_type, entity_id, summary) VALUES (1, 'class', 1, 'kept')`)
	rollBackSchema19(t, app.db)
	if err := ensureSchema(app.db); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`INSERT INTO wiki_gallery (entry_id, position, file_path) VALUES (1, 0, 'gallery/x.png')`); err != nil {
		t.Errorf("wiki_gallery missing after the migration: %v", err)
	}
	var summary string
	app.db.QueryRow(`SELECT summary FROM wiki_entries WHERE id = 1`).Scan(&summary)
	if summary != "kept" {
		t.Error("migration must keep existing data")
	}
}
