package main

import (
	"encoding/json"
	"testing"
)

type creatureJSON struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	LevelMin   *int64 `json:"level_min"`
	LevelMax   *int64 `json:"level_max"`
	RegionName string `json:"region_name"`
	Related    []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"related"`
}

// The bestiary: creating, relating (both ways), the wiki article, and
// deleting a creature, which unrelates it.
func TestBestiary(t *testing.T) {
	mgr, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Beasts"})
	app, _ := mgr.open(s.ID)
	base := "/api/s/" + s.ID
	app.db.Exec(`INSERT INTO locations (id, name, kind) VALUES (1, 'Ashen Wastes', 'major')`)

	save := func(method, path string, fields map[string]string) creatureJSON {
		t.Helper()
		body, ct := form(t, fields, "", "", nil)
		rec := do(t, h, method, base+path, body, ct)
		if rec.Code != 200 {
			t.Fatalf("%s %s: %d %s", method, path, rec.Code, rec.Body.String())
		}
		var c creatureJSON
		json.Unmarshal(rec.Body.Bytes(), &c)
		return c
	}
	wolf := save("POST", "/creatures", map[string]string{"name": "Ash Wolf", "kind": "animal", "level_min": "12", "level_max": "5", "region_id": "1"})
	if wolf.LevelMin == nil || *wolf.LevelMin != 5 || *wolf.LevelMax != 12 || wolf.RegionName != "Ashen Wastes" {
		t.Fatalf("wolf: %+v (levels given the wrong way round are swapped)", wolf)
	}
	golem := save("POST", "/creatures", map[string]string{"name": "Ash Golem", "kind": "construct", "related": "[" + idStr(wolf.ID) + ",999]"})
	if len(golem.Related) != 1 || golem.Related[0].Name != "Ash Wolf" {
		t.Fatalf("golem related: %+v", golem.Related)
	}

	var got creatureJSON
	json.Unmarshal(do(t, h, "GET", base+"/creatures/"+idStr(wolf.ID), nil, "").Body.Bytes(), &got)
	if len(got.Related) != 1 || got.Related[0].Name != "Ash Golem" {
		t.Errorf("relations go both ways: wolf related %+v", got.Related)
	}

	body, ct := form(t, map[string]string{"name": "ash wolf", "kind": "monster"}, "", "", nil)
	if rec := do(t, h, "POST", base+"/creatures", body, ct); rec.Code != 400 {
		t.Errorf("a second Ash Wolf: %d", rec.Code)
	}
	body, ct = form(t, map[string]string{"name": "Wisp", "kind": "ghost"}, "", "", nil)
	if rec := do(t, h, "POST", base+"/creatures", body, ct); rec.Code != 400 {
		t.Errorf("an unknown kind: %d", rec.Code)
	}

	a := &wikiArticle{Type: "creature", ID: wolf.ID}
	loadSourceFacts(app.db, a)
	facts := map[string]string{}
	for _, f := range a.Facts {
		facts[f.Key] = f.Value
	}
	if facts["type"] != "Animal" || facts["level"] != "5–12" || facts["native_to"] != "Ashen Wastes" {
		t.Errorf("wolf facts: %v", facts)
	}
	region := &wikiArticle{Type: "location", ID: 1}
	loadSourceFacts(app.db, region)
	found := false
	for _, g := range region.Groups {
		found = found || g.Key == "creatures_here"
	}
	if !found {
		t.Error("the region should list the creatures native to it")
	}

	if rec := do(t, h, "DELETE", base+"/creatures/"+idStr(wolf.ID), nil, ""); rec.Code != 200 {
		t.Fatalf("delete: %d", rec.Code)
	}
	json.Unmarshal(do(t, h, "GET", base+"/creatures/"+idStr(golem.ID), nil, "").Body.Bytes(), &got)
	if len(got.Related) != 0 {
		t.Errorf("a deleted creature is still related: %+v", got.Related)
	}
}

func TestGearDescription(t *testing.T) {
	_, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Gear"})
	_, g := postGear(t, h, s.ID, map[string]string{"name": "Fur Cloak", "slot": "cape", "description": "Keeps the cold out."}, nil)
	if g.Description != "Keeps the cold out." {
		t.Errorf("description: %q", g.Description)
	}
}

func TestMigrationAddsTheBestiary(t *testing.T) {
	mgr, _ := newTestServer(t)
	app, _ := mgr.open(createStoryDirect(t, mgr, "Old"))
	rollBackSchema25(t, app.db)
	app.db.Exec(`INSERT INTO gear (name, slot) VALUES ('Helm', 'helmet')`)
	if err := ensureSchema(app.db); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`INSERT INTO creatures (name, kind) VALUES ('Wolf', 'animal')`); err != nil {
		t.Fatal(err)
	}
	var desc any
	if err := app.db.QueryRow(`SELECT description FROM gear`).Scan(&desc); err != nil {
		t.Fatal(err)
	}
}

// A creature's territory is its region's area until it is painted; going
// back to the region drops the painted hexes.
func TestCreatureTerritory(t *testing.T) {
	mgr, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Territory"})
	app, _ := mgr.open(s.ID)
	base := "/api/s/" + s.ID
	for _, q := range []string{
		`INSERT INTO locations (id, name, kind, color) VALUES (1, 'Aldoria', 'major', '#aa0000')`,
		`INSERT INTO location_hexes (location_id, q, r) VALUES (1, 0, 0), (1, 1, 0), (1, 0, 1)`,
		`INSERT INTO locations (id, name, kind, q, r) VALUES (2, 'Thornby', 'minor', 5, 5)`,
	} {
		if _, err := app.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	type out struct {
		ID              int64      `json:"id"`
		Territory       [][2]int64 `json:"territory"`
		CustomTerritory bool       `json:"custom_territory"`
	}
	save := func(method, path string, fields map[string]string) out {
		t.Helper()
		body, ct := form(t, fields, "", "", nil)
		rec := do(t, h, method, base+path, body, ct)
		if rec.Code != 200 {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
		var o out
		json.Unmarshal(rec.Body.Bytes(), &o)
		return o
	}
	c := save("POST", "/creatures", map[string]string{"name": "Wolf", "kind": "animal", "region_id": "1"})
	if len(c.Territory) != 3 || c.CustomTerritory {
		t.Fatalf("a kingdom's area: %+v", c)
	}
	path := "/creatures/" + idStr(c.ID)
	c = save("PUT", path, map[string]string{"name": "Wolf", "kind": "animal", "region_id": "2"})
	if len(c.Territory) != 1 || c.Territory[0] != [2]int64{5, 5} {
		t.Fatalf("a town's hex: %+v", c.Territory)
	}
	c = save("PUT", path, map[string]string{"name": "Wolf", "kind": "animal", "region_id": "1", "territory": "[[0,0],[9,9],[9,9]]"})
	if len(c.Territory) != 2 || !c.CustomTerritory {
		t.Fatalf("painted: %+v", c)
	}
	c = save("PUT", path, map[string]string{"name": "Wolf", "kind": "animal", "region_id": "1"})
	if len(c.Territory) != 2 {
		t.Errorf("leaving territory out must keep the painted hexes: %+v", c.Territory)
	}
	c = save("PUT", path, map[string]string{"name": "Wolf", "kind": "animal", "region_id": "1", "territory": "region"})
	if len(c.Territory) != 3 || c.CustomTerritory {
		t.Errorf("back to the region: %+v", c)
	}
}
