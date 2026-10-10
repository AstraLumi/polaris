package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMapSettlementFlags(t *testing.T) {
	_, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Maps"})
	put := func(method, url, body string) mapLocation {
		rec := do(t, h, method, url, strings.NewReader(body), "application/json")
		if rec.Code != 200 {
			t.Fatalf("%s %s: %d %s", method, url, rec.Code, rec.Body.String())
		}
		var l mapLocation
		json.Unmarshal(rec.Body.Bytes(), &l)
		return l
	}
	base := "/api/s/" + s.ID + "/map/locations"

	town := put("POST", base, `{"kind":"minor","name":"Aldor","q":1,"r":2,"is_city":true,"is_capital":true}`)
	if !town.IsCity || !town.IsCapital {
		t.Fatalf("flags not saved: %+v", town)
	}
	plain := put("POST", base, `{"kind":"minor","name":"Farm","q":3,"r":2}`)
	if plain.IsCity || plain.IsCapital {
		t.Fatalf("flags default off: %+v", plain)
	}
	// Editing replaces the flags.
	town = put("PUT", base+"/"+idStr(town.ID), `{"kind":"minor","name":"Aldor","q":1,"r":2,"is_city":true}`)
	if !town.IsCity || town.IsCapital {
		t.Fatalf("flags not updated: %+v", town)
	}
	// Only minor locations can be settlements.
	king := put("POST", base, `{"kind":"major","name":"Realm","color":"#aa0000","is_city":true,"is_capital":true}`)
	if king.IsCity || king.IsCapital {
		t.Fatalf("a major location must not be a city: %+v", king)
	}

	// The map payload and the wiki both carry it.
	rec := do(t, h, "GET", "/api/s/"+s.ID+"/map", nil, "")
	var payload mapPayload
	json.Unmarshal(rec.Body.Bytes(), &payload)
	cities := 0
	for _, l := range payload.Locations {
		if l.IsCity {
			cities++
		}
	}
	if cities != 1 {
		t.Errorf("expected one city in the payload, got %d", cities)
	}
	a := wikiGet(t, h, s.ID, "location", town.ID)
	found := false
	for _, f := range a.Facts {
		if f.Key == "settlement" && f.Value == "City" {
			found = true
		}
	}
	if !found {
		t.Errorf("wiki facts missing settlement: %+v", a.Facts)
	}
}

func TestMigrationAddsSettlementColumns(t *testing.T) {
	mgr, _ := newTestServer(t)
	id := createStoryDirect(t, mgr, "Old")
	app, _ := mgr.open(id)
	app.db.Exec(`INSERT INTO locations (name, kind) VALUES ('Greyhold', 'minor')`)
	rollBackSchema16(t, app.db)
	for _, q := range []string{
		`ALTER TABLE locations DROP COLUMN is_city`,
		`ALTER TABLE locations DROP COLUMN is_capital`,
		`ALTER TABLE character_story DROP COLUMN status`, `UPDATE schema_meta SET value = '13' WHERE key = 'schema_version'`,
	} {
		if _, err := app.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err := ensureSchema(app.db); err != nil {
		t.Fatal(err)
	}
	var name string
	var city, capital int
	if err := app.db.QueryRow(`SELECT name, is_city, is_capital FROM locations`).Scan(&name, &city, &capital); err != nil {
		t.Fatalf("columns missing after migration: %v", err)
	}
	if name != "Greyhold" || city != 0 || capital != 0 {
		t.Errorf("existing location changed: %s %d %d", name, city, capital)
	}
}
