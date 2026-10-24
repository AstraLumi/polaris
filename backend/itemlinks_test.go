package main

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
)

// The items' own texts (itemLinkColumns) store their links by id when saved
// from their pages, and saving what the form shows keeps the bytes.
func TestItemTextsStoreLinksByID(t *testing.T) {
	mgr, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Items"})
	app, _ := mgr.open(s.ID)
	api := "/api/s/" + s.ID
	res, _ := app.db.Exec(`INSERT INTO locations (name, kind) VALUES ('Aldoria', 'minor')`)
	loc, _ := res.LastInsertId()
	L := idStr(loc)
	typed := "Seen in [[Aldoria]] and [[aldoria|the city]]; [[Nowhere]]."
	stored := "Seen in [[location#" + L + "]] and [[location#" + L + "|the city]]; [[Nowhere]]."

	send := func(method, url string, fields map[string]string) []byte {
		t.Helper()
		body, ct := form(t, fields, "", "", nil)
		rec := do(t, h, method, api+url, body, ct)
		if rec.Code != 200 {
			t.Fatalf("%s %s: %d %s", method, url, rec.Code, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	newID := func(b []byte) int64 {
		var v struct {
			ID int64 `json:"id"`
		}
		json.Unmarshal(b, &v)
		if v.ID == 0 {
			t.Fatalf("no id in %s", b)
		}
		return v.ID
	}
	text := func(table, col, key string, id int64) string {
		t.Helper()
		var v string
		if err := app.db.QueryRow(`SELECT COALESCE(`+col+`, '') FROM `+table+` WHERE `+key+` = ?`, id).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}

	ch := newID(send("POST", "/characters", map[string]string{"name": "Aria"}))
	var cur struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(do(t, h, "GET", api+"/characters/"+idStr(ch)+"/current", nil, "").Body.Bytes(), &cur)
	ev := newID(send("POST", "/events", map[string]string{"name": "Siege", "event_date": "1000", "description": typed}))
	fac := newID(send("POST", "/factions", map[string]string{"name": "Guild", "description": typed}))
	cre := newID(send("POST", "/creatures", map[string]string{"name": "Wolf", "kind": "animal", "description": typed}))
	gear := newID(send("POST", "/gear", map[string]string{"name": "Cloak", "slot": "cape", "description": typed}))
	spell := newID(send("POST", "/spells", map[string]string{"name": "Spark", "description": typed}))
	rec := do(t, h, "POST", api+"/map/locations", jsonBody(map[string]any{"kind": "minor", "name": "Port", "q": 1, "r": 1, "description": typed}), "application/json")
	if rec.Code != 200 {
		t.Fatalf("location: %d %s", rec.Code, rec.Body.String())
	}
	port := newID(rec.Body.Bytes())
	send("PUT", "/versions/"+idStr(cur.ID), map[string]string{
		"name": "Aria", "level": "1", "description": typed, "bio": "Born in [[Aldoria]].", "speech_mannerisms": "Says [[Port]] a lot.",
	})

	checks := []struct{ got, want string }{
		{text("events", "description", "id", ev), stored},
		{text("factions", "description", "id", fac), stored},
		{text("creatures", "description", "id", cre), stored},
		{text("gear", "description", "id", gear), stored},
		{text("spells", "description", "id", spell), stored},
		{text("locations", "description", "id", port), stored},
		{text("character_story", "description", "version_id", cur.ID), stored},
		{text("character_story", "bio", "version_id", cur.ID), "Born in [[location#" + L + "]]."},
		{text("character_story", "speech_mannerisms", "version_id", cur.ID), "Says [[location#" + idStr(port) + "]] a lot."},
	}
	for i, c := range checks {
		if c.got != c.want {
			t.Errorf("%d: got %q, want %q", i, c.got, c.want)
		}
	}

	// Saved again as the form shows it (by name), nothing changes.
	send("PUT", "/events/"+idStr(ev), map[string]string{"name": "Siege", "event_date": "1000", "description": "Seen in [[Aldoria]] and [[Aldoria|the city]]; [[Nowhere]]."})
	if got := text("events", "description", "id", ev); got != stored {
		t.Errorf("an unchanged save: %q", got)
	}
	// A rename carries through to the wiki's sheet.
	app.db.Exec(`UPDATE locations SET name = 'New Aldoria' WHERE id = ?`, loc)
	if a := wikiGet(t, h, s.ID, "event", ev); a.LinkNames["location#"+L] != "New Aldoria" {
		t.Errorf("sheet link names: %v", a.LinkNames)
	}
	// The events' search reads the names the links show.
	var found []struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(do(t, h, "GET", api+"/events?q=new+aldoria", nil, "").Body.Bytes(), &found)
	if len(found) != 1 || found[0].ID != ev {
		t.Errorf("search by a linked name: %+v", found)
	}
}

// Schema 43 turns the links already written in the items' texts into the
// stored form.
func TestMigrationStoresItemLinksByID(t *testing.T) {
	mgr, _ := newTestServer(t)
	app, _ := mgr.open(createStoryDirect(t, mgr, "Old"))
	db := app.db
	rollBackSchema43(t, db)
	exec := func(q string, args ...any) int64 {
		t.Helper()
		res, err := db.Exec(q, args...)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		id, _ := res.LastInsertId()
		return id
	}
	typed := "In [[Aldoria]] with [[Ash|her]]. [[Nowhere]]. `[[Aldoria]]`"
	loc := exec(`INSERT INTO locations (name, kind, description) VALUES ('Aldoria', 'minor', ?)`, "Home of [[Aria Vale]].")
	ch := exec(`INSERT INTO characters DEFAULT VALUES`)
	exec(`INSERT INTO character_versions (character_id, name, is_current) VALUES (?, 'Ash', 0)`, ch)
	v := exec(`INSERT INTO character_versions (character_id, name, is_current) VALUES (?, 'Aria Vale', 1)`, ch)
	exec(`INSERT INTO character_story (version_id, description, bio, speech_mannerisms) VALUES (?, ?, ?, ?)`, v, typed, "[[Aldoria]]", "plain")
	ev := exec(`INSERT INTO events (name, event_date, description) VALUES ('Siege', '1000', ?)`, typed)
	fac := exec(`INSERT INTO factions (name, description) VALUES ('Guild', ?)`, typed)
	sp := exec(`INSERT INTO spells (name, description) VALUES ('Spark', ?)`, typed)
	g := exec(`INSERT INTO gear (name, slot, description) VALUES ('Cloak', 'cape', ?)`, typed)
	cr := exec(`INSERT INTO creatures (name, kind, description) VALUES ('Wolf', 'animal', ?)`, typed)
	bare := exec(`INSERT INTO gear (name, slot) VALUES ('Plain', 'cape')`)

	if err := ensureSchema(db); err != nil {
		t.Fatal(err)
	}
	if v := storedSchema(db); v != currentSchemaVersion {
		t.Fatalf("schema %s", v)
	}
	L, C := idStr(loc), idStr(ch)
	stored := "In [[location#" + L + "]] with [[character#" + C + "|her]]. [[Nowhere]]. `[[Aldoria]]`"
	text := func(q string, id int64) string {
		t.Helper()
		var s sql.NullString
		if err := db.QueryRow(q, id).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s.String
	}
	checks := []struct{ got, want string }{
		{text(`SELECT description FROM locations WHERE id = ?`, loc), "Home of [[character#" + C + "]]."},
		{text(`SELECT description FROM character_story WHERE version_id = ?`, v), stored},
		{text(`SELECT bio FROM character_story WHERE version_id = ?`, v), "[[location#" + L + "]]"},
		{text(`SELECT speech_mannerisms FROM character_story WHERE version_id = ?`, v), "plain"},
		{text(`SELECT description FROM events WHERE id = ?`, ev), stored},
		{text(`SELECT description FROM factions WHERE id = ?`, fac), stored},
		{text(`SELECT description FROM spells WHERE id = ?`, sp), stored},
		{text(`SELECT description FROM gear WHERE id = ?`, g), stored},
		{text(`SELECT description FROM creatures WHERE id = ?`, cr), stored},
		{text(`SELECT description FROM gear WHERE id = ?`, bare), ""},
	}
	for i, c := range checks {
		if c.got != c.want {
			t.Errorf("%d: got %q, want %q", i, c.got, c.want)
		}
	}
}

// Copying the Character Assets keeps links to copied rows (same ids) and
// turns links to anything else back into names.
func TestCopyAssetsRelinksDescriptions(t *testing.T) {
	mgr, h := newTestServer(t)
	a := createStory(t, h, map[string]string{"name": "Alpha"})
	app, _ := mgr.open(a.ID)
	exec := func(q string, args ...any) int64 {
		t.Helper()
		res, err := app.db.Exec(q, args...)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		id, _ := res.LastInsertId()
		return id
	}
	// Spare rows first, so the ids are not the ones a new story would give.
	exec(`INSERT INTO classes (name) VALUES ('Spare')`)
	mage := exec(`INSERT INTO classes (name) VALUES ('Mage')`)
	exec(`INSERT INTO gear (name, slot) VALUES ('Spare', 'cape')`)
	cloak := exec(`INSERT INTO gear (name, slot) VALUES ('Cloak', 'cape')`)
	exec(`INSERT INTO locations (name, kind) VALUES ('Spare', 'minor')`)
	loc := exec(`INSERT INTO locations (name, kind) VALUES ('Greyhold', 'minor')`)
	ch := exec(`INSERT INTO characters DEFAULT VALUES`)
	exec(`INSERT INTO character_versions (character_id, name, is_current) VALUES (?, 'Aria', 1)`, ch)
	spell := exec(`INSERT INTO spells (name, description) VALUES ('Spark', ?)`,
		"Taught to [[class#"+idStr(mage)+"|mages]] in [[location#"+idStr(loc)+"]] by [[character#"+idStr(ch)+"|her]]; see [[gear#"+idStr(cloak)+"]]. Once [[location#999|elsewhere]]. [[Nowhere]].")
	exec(`INSERT INTO gear (name, slot, description) VALUES ('Hat', 'head', ?)`, "Worn by [[character#"+idStr(ch)+"]].")

	b := createStory(t, h, map[string]string{"name": "Beta", "copy_assets_from": a.ID})
	bapp, _ := mgr.open(b.ID)
	var got, hat string
	bapp.db.QueryRow(`SELECT description FROM spells WHERE id = ?`, spell).Scan(&got)
	bapp.db.QueryRow(`SELECT description FROM gear WHERE name = 'Hat'`).Scan(&hat)
	want := "Taught to [[class#" + idStr(mage) + "|mages]] in [[location:Greyhold]] by [[character:Aria|her]]; see [[gear#" + idStr(cloak) + "]]. Once elsewhere. [[Nowhere]]."
	if got != want {
		t.Errorf("copied spell:\n got %q\nwant %q", got, want)
	}
	if hat != "Worn by [[character:Aria]]." {
		t.Errorf("copied gear: %q", hat)
	}
	// In the new story the kept ids find the copied rows.
	x, err := loadWikiLinkIndex(bapp.db)
	if err != nil {
		t.Fatal(err)
	}
	if r := x.resolve("class#" + idStr(mage)); r == nil || r.Name != "Mage" {
		t.Errorf("the copied class: %+v", r)
	}
	if r := x.resolve("gear#" + idStr(cloak)); r == nil || r.Name != "Cloak" {
		t.Errorf("the copied gear: %+v", r)
	}
	// The source keeps its ids.
	var src string
	app.db.QueryRow(`SELECT description FROM spells WHERE id = ?`, spell).Scan(&src)
	if !strings.Contains(src, "[[location#"+idStr(loc)+"]]") {
		t.Errorf("the source changed: %q", src)
	}
}
