package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// Birth date, blood type and birthplace move from the versions to the
// character, taken from the current version when it has them, else from
// another one.
func TestMigrationMovesBirthFactsToTheCharacter(t *testing.T) {
	mgr, _ := newTestServer(t)
	app, _ := mgr.open(createStoryDirect(t, mgr, "Old"))
	rollBackSchema23(t, app.db)
	for _, q := range []string{
		`INSERT INTO locations (id, name) VALUES (1, 'Aldor')`,
		`INSERT INTO characters (id) VALUES (1)`,
		`INSERT INTO character_versions (id, character_id, is_current, name) VALUES (1, 1, 0, 'Young Aria'), (2, 1, 1, 'Aria')`,
		`INSERT INTO character_story (version_id, birth_date, blood_type, born_in_location_id) VALUES (1, '15-03-1024', 'A+', 1), (2, '', 'B-', NULL)`,
	} {
		if _, err := app.db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if err := ensureSchema(app.db); err != nil {
		t.Fatal(err)
	}
	for _, vid := range []string{"1", "2"} {
		d, err := buildVersionDetail(app.db, vid)
		if err != nil {
			t.Fatal(err)
		}
		if d.Story.BirthDate != "15-03-1024" || d.Story.BloodType != "B-" || d.Story.BornInName != "Aldor" {
			t.Errorf("version %s: birth %q, blood %q, born in %q", vid, d.Story.BirthDate, d.Story.BloodType, d.Story.BornInName)
		}
	}
}

// Saving one version's birth date changes it for every version.
func TestBirthFactsAreSharedByVersions(t *testing.T) {
	mgr, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Shared"})
	app, _ := mgr.open(s.ID)
	base := "/api/s/" + s.ID
	body, ct := form(t, map[string]string{"name": "Aria", "level": "1"}, "", "", nil)
	rec := do(t, h, "POST", base+"/characters", body, ct)
	var c struct {
		ID        int64 `json:"id"`
		VersionID int64 `json:"version_id"`
	}
	json.Unmarshal(rec.Body.Bytes(), &c)
	rec = do(t, h, "POST", base+"/characters/"+idStr(c.ID)+"/versions",
		strings.NewReader(`{"version_date":"","version_reference":"","clone_from_version_id":`+idStr(c.VersionID)+`}`), "application/json")
	var nv struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(rec.Body.Bytes(), &nv)

	body, ct = form(t, map[string]string{"name": "Aria", "level": "1", "birth_date": "02-05-1590", "blood_type": "O+"}, "", "", nil)
	if rec := do(t, h, "PUT", base+"/versions/"+idStr(c.VersionID), body, ct); rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	d, _ := buildVersionDetail(app.db, idStr(nv.ID))
	if d.Story.BirthDate != "02-05-1590" || d.Story.BloodType != "O+" {
		t.Errorf("the other version: birth %q, blood %q", d.Story.BirthDate, d.Story.BloodType)
	}
}

// A race or specialization can be native to a kingdom (only a kingdom: a
// region or town is refused), and the wiki shows it both ways.
func TestNativeKingdom(t *testing.T) {
	mgr, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Native"})
	app, _ := mgr.open(s.ID)
	base := "/api/s/" + s.ID
	app.db.Exec(`INSERT INTO locations (id, name, kind, color) VALUES (1, 'Aldoria', 'major', '#aa0000'), (2, 'Thornby', 'minor', NULL)`)
	app.db.Exec(`INSERT INTO classes (id, name) VALUES (1, 'Mage')`)

	for _, c := range []struct{ path, body string }{
		{"/races", `{"name":"Elf","kingdom_id":1}`},
		{"/races", `{"name":"Dwarf","kingdom_id":2}`},
		{"/specializations", `{"name":"Spellslinger","class_id":1,"kingdom_id":1}`},
	} {
		if rec := do(t, h, "POST", base+c.path, strings.NewReader(c.body), "application/json"); rec.Code != 200 {
			t.Fatalf("%s: %d %s", c.body, rec.Code, rec.Body.String())
		}
	}
	var elf, dwarf, spec struct {
		KingdomID   *int64 `json:"kingdom_id"`
		KingdomName string `json:"kingdom_name"`
	}
	json.Unmarshal(do(t, h, "GET", base+"/races/1", nil, "").Body.Bytes(), &elf)
	json.Unmarshal(do(t, h, "GET", base+"/races/2", nil, "").Body.Bytes(), &dwarf)
	json.Unmarshal(do(t, h, "GET", base+"/specializations/1", nil, "").Body.Bytes(), &spec)
	if elf.KingdomName != "Aldoria" || spec.KingdomName != "Aldoria" || dwarf.KingdomID != nil {
		t.Errorf("elf %q, specialization %q, dwarf %v (a town isn't a kingdom)", elf.KingdomName, spec.KingdomName, dwarf.KingdomID)
	}

	a := &wikiArticle{Type: "location", ID: 1}
	loadSourceFacts(app.db, a)
	var groups []string
	for _, g := range a.Groups {
		groups = append(groups, g.Key)
	}
	if !strings.Contains(strings.Join(groups, ","), "native_races") || !strings.Contains(strings.Join(groups, ","), "native_specializations") {
		t.Errorf("kingdom groups: %v", groups)
	}
}

// The version list saves birth date, blood type, birthplace and tags on
// their own, and a version save that leaves them out keeps them.
func TestSharedCharacterFields(t *testing.T) {
	mgr, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Shared fields"})
	app, _ := mgr.open(s.ID)
	base := "/api/s/" + s.ID
	app.db.Exec(`INSERT INTO locations (id, name) VALUES (1, 'Aldor')`)
	body, ct := form(t, map[string]string{"name": "Aria", "level": "1"}, "", "", nil)
	rec := do(t, h, "POST", base+"/characters", body, ct)
	var c struct {
		ID        int64 `json:"id"`
		VersionID int64 `json:"version_id"`
	}
	json.Unmarshal(rec.Body.Bytes(), &c)

	rec = do(t, h, "PUT", base+"/characters/"+idStr(c.ID)+"/shared",
		strings.NewReader(`{"birth_date":"02-05-1590","blood_type":" AB- ","born_in":"","born_in_location_id":1,"tags":["hero"]}`), "application/json")
	if rec.Code != 200 {
		t.Fatalf("shared save: %d %s", rec.Code, rec.Body.String())
	}
	check := func(when string) {
		t.Helper()
		d, _ := buildVersionDetail(app.db, idStr(c.VersionID))
		if d.Story.BirthDate != "02-05-1590" || d.Story.BloodType != "AB-" || d.Story.BornInName != "Aldor" || len(d.Tags) != 1 || d.Tags[0] != "hero" {
			t.Errorf("%s: birth %q, blood %q, born in %q, tags %v", when, d.Story.BirthDate, d.Story.BloodType, d.Story.BornInName, d.Tags)
		}
	}
	check("after the shared save")

	body, ct = form(t, map[string]string{"name": "Aria", "level": "2"}, "", "", nil)
	if rec := do(t, h, "PUT", base+"/versions/"+idStr(c.VersionID), body, ct); rec.Code != 200 {
		t.Fatalf("version save: %d %s", rec.Code, rec.Body.String())
	}
	check("after a version save without them")

	if rec := do(t, h, "PUT", base+"/characters/"+idStr(c.ID)+"/shared",
		strings.NewReader(`{"birth_date":"99-99-1590"}`), "application/json"); rec.Code != 400 {
		t.Errorf("an impossible birth date: %d", rec.Code)
	}
	if rec := do(t, h, "PUT", base+"/characters/999/shared", strings.NewReader(`{}`), "application/json"); rec.Code != 404 {
		t.Errorf("a missing character: %d", rec.Code)
	}
}
