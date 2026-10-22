package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// Schema 40 gives the body types an outline, empty (all from the character)
// for the ones already there.
func TestMigrationAddsBodyOutlines(t *testing.T) {
	mgr, _ := newTestServer(t)
	app, _ := mgr.open(createStoryDirect(t, mgr, "Old"))
	rollBackSchema40(t, app.db)
	if _, err := app.db.Exec(`INSERT INTO body_types (name) VALUES ('Stocky')`); err != nil {
		t.Fatal(err)
	}
	if err := ensureSchema(app.db); err != nil {
		t.Fatal(err)
	}
	list, err := listBodyTypes(app.db)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "Stocky" || list[0].Outline != (bodyOutline{}) {
		t.Fatalf("body types: %+v", list)
	}
}

// A body type keeps the outline it is given, keeps it when saved without
// one, and refuses keys it doesn't know.
func TestBodyTypeOutline(t *testing.T) {
	_, h, base := planetSetup(t)
	rec := do(t, h, "POST", base+"/body-types", strings.NewReader(`{"name":"Round","outline":{"sex":"","age":"elderly","build":"chubby"}}`), "application/json")
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	var made struct{ ID int64 }
	json.Unmarshal(rec.Body.Bytes(), &made)
	id := idStr(made.ID)
	get := func() simpleAssetDetail {
		var d simpleAssetDetail
		json.Unmarshal(do(t, h, "GET", base+"/body-types/"+id, nil, "").Body.Bytes(), &d)
		return d
	}
	if d := get(); d.Outline == nil || *d.Outline != (bodyOutline{Age: "elderly", Build: "chubby"}) {
		t.Fatalf("outline: %+v", d.Outline)
	}
	if c := do(t, h, "PUT", base+"/body-types/"+id, strings.NewReader(`{"name":"Rounder"}`), "application/json").Code; c != 200 {
		t.Fatal(c)
	}
	if d := get(); d.Name != "Rounder" || d.Outline.Build != "chubby" {
		t.Fatalf("after rename: %+v", d)
	}
	for _, bad := range []string{`{"sex":"robot","age":"","build":""}`, `{"sex":"","age":"","build":"huge"}`} {
		if c := do(t, h, "PUT", base+"/body-types/"+id, strings.NewReader(`{"name":"X","outline":`+bad+`}`), "application/json").Code; c != 400 {
			t.Errorf("%s: %d", bad, c)
		}
	}
	if c := do(t, h, "PUT", base+"/body-types/"+id, strings.NewReader(`{"name":"Rounder","outline":{"sex":"female","age":"","build":"slim"}}`), "application/json").Code; c != 200 {
		t.Fatal(c)
	}
	var list []bodyTypeOption
	json.Unmarshal(do(t, h, "GET", base+"/body-types", nil, "").Body.Bytes(), &list)
	if len(list) != 1 || list[0].Outline != (bodyOutline{Sex: "female", Build: "slim"}) {
		t.Fatalf("list: %+v", list)
	}
}

// The height chart gets every version of every character, with its height,
// gender, age and body type.
func TestCharacterHeights(t *testing.T) {
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
	tall := exec(`INSERT INTO body_types (name, outline_build) VALUES ('Tall', 'slim')`)
	for _, c := range []struct {
		name   string
		height any
		older  bool
	}{{"Zed", 182.5, true}, {"Ann", nil, false}} {
		char := exec(`INSERT INTO characters DEFAULT VALUES`)
		if c.older {
			v := exec(`INSERT INTO character_versions (character_id, name, version_date) VALUES (?, ?, '01-01-1000')`, char, c.name+" young")
			exec(`INSERT INTO character_story (version_id, height, gender, age) VALUES (?, 120, 'Male', 9)`, v)
		}
		v := exec(`INSERT INTO character_versions (character_id, name, is_current) VALUES (?, ?, 1)`, char, c.name)
		exec(`INSERT INTO character_story (version_id, height, gender, body_type_id) VALUES (?, ?, 'Female', ?)`, v, c.height, tall)
	}
	rec := do(t, h, "GET", base+"/characters/heights", nil, "")
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	var out struct {
		Characters []heightCharacter `json:"characters"`
		BodyTypes  []bodyTypeOption  `json:"body_types"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.BodyTypes) != 1 || out.BodyTypes[0].Outline.Build != "slim" {
		t.Fatalf("body types: %+v", out.BodyTypes)
	}
	if len(out.Characters) != 2 || out.Characters[0].Versions[0].Name != "Ann" {
		t.Fatalf("characters (by current name): %+v", out.Characters)
	}
	ann, zed := out.Characters[0].Versions, out.Characters[1].Versions
	if ann[0].Height != nil || *ann[0].BodyTypeID != tall {
		t.Errorf("Ann: %+v", ann[0])
	}
	if len(zed) != 2 || zed[0].IsCurrent || *zed[0].Height != 120 || *zed[0].Age != 9 || zed[0].Gender != "Male" ||
		!zed[1].IsCurrent || *zed[1].Height != 182.5 {
		t.Errorf("Zed: %+v", zed)
	}
}
