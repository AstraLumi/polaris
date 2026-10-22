package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// Plates, continents and depths are drawn like major locations: one feature
// per layer per cell; a kingdom's article names the plate under most of it;
// the layers follow a change of hex size; deleting a feature clears it.
func TestPlanetLayers(t *testing.T) {
	app, h, base := planetSetup(t)
	putMap(t, h, base, nil)
	newFeature := func(body string) mapFeature {
		t.Helper()
		rec := do(t, h, "POST", base+"/planet/features", strings.NewReader(body), "application/json")
		if rec.Code != 200 {
			t.Fatalf("feature %s: %d %s", body, rec.Code, rec.Body.String())
		}
		var f mapFeature
		json.Unmarshal(rec.Body.Bytes(), &f)
		return f
	}
	pacific := newFeature(`{"layer":"plates","name":"Pacific Plate","color":"#AA0000","value":5}`)
	eurasia := newFeature(`{"layer":"plates","name":"Eurasian Plate","color":"#00aa00"}`)
	laurasia := newFeature(`{"layer":"continents","name":"Laurasia","color":"#888888","value":4808}`)
	if pacific.Value != nil || pacific.Color != "#aa0000" || laurasia.Value != nil {
		t.Errorf("features: %+v %+v", pacific, laurasia)
	}
	for _, bad := range []string{`{"layer":"mountains","name":"X","color":"#000000"}`, `{"layer":"winds","name":"X","color":"#000000"}`, `{"layer":"plates","name":"","color":"#000000"}`, `{"layer":"plates","name":"X","color":""}`} {
		if rec := do(t, h, "POST", base+"/planet/features", strings.NewReader(bad), "application/json"); rec.Code != 400 {
			t.Errorf("%s: %d", bad, rec.Code)
		}
	}

	paint := func(path, body string) {
		t.Helper()
		if rec := do(t, h, "POST", base+"/planet/features/"+path, strings.NewReader(body), "application/json"); rec.Code != 200 {
			t.Fatalf("%s %s: %d %s", path, body, rec.Code, rec.Body.String())
		}
	}
	paint("paint", `{"feature_id":`+idStr(pacific.ID)+`,"cells":[10,11,12,13]}`)
	paint("paint", `{"feature_id":`+idStr(eurasia.ID)+`,"cells":[13,14]}`)  // takes 13 from the Pacific
	paint("paint", `{"feature_id":`+idStr(laurasia.ID)+`,"cells":[12,13]}`) // another layer: both stay
	paint("erase", `{"layer":"plates","cells":[10]}`)
	var l layersPayload
	json.Unmarshal(do(t, h, "GET", base+"/planet/layers", nil, "").Body.Bytes(), &l)
	got := map[[2]int64]bool{}
	for _, c := range l.Cells {
		got[c] = true
	}
	want := [][2]int64{{pacific.ID, 11}, {pacific.ID, 12}, {eurasia.ID, 13}, {eurasia.ID, 14}, {laurasia.ID, 12}, {laurasia.ID, 13}}
	if len(l.Cells) != len(want) {
		t.Errorf("cells: %v", l.Cells)
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing %v in %v", w, l.Cells)
		}
	}

	// A kingdom on cells 11–13 is on the Pacific Plate (two of its three
	// hexes) and also the Eurasian, with the continent in it.
	app.db.Exec(`INSERT INTO locations (id, name, kind, color, realm) VALUES (1, 'Aldoria', 'major', '#123456', 'kingdom')`)
	app.db.Exec(`INSERT INTO planet_cells (location_id, cell) VALUES (1, 11), (1, 12), (1, 13)`)
	story := strings.TrimPrefix(base, "/api/s/")
	a := wikiGet(t, h, story, "location", 1)
	var plate, continent *wikiRef
	groups := map[string]int{}
	for _, f := range a.Facts {
		if f.Key == "plate" {
			plate = f.Link
		}
		if f.Key == "continent" {
			continent = f.Link
		}
	}
	for _, g := range a.Groups {
		groups[g.Key] = len(g.Items)
	}
	if plate == nil || plate.ID != pacific.ID || groups["plates"] != 2 || groups["continents"] != 0 || continent == nil || continent.ID != laurasia.ID {
		t.Errorf("kingdom: plate %+v, groups %v", plate, groups)
	}
	p := wikiGet(t, h, story, "map_feature", pacific.ID)
	pg := map[string]int{}
	for _, g := range p.Groups {
		pg[g.Key] = len(g.Items)
	}
	if pg["lands_on"] != 1 || pg["continents_on"] != 1 {
		t.Errorf("plate article groups: %v", pg)
	}

	// Biomes: painted per hex, shown as shares in the articles.
	biomePost := func(path, body string) *httptest.ResponseRecorder {
		return do(t, h, "POST", base+"/planet/biomes/"+path, strings.NewReader(body), "application/json")
	}
	if rec := biomePost("paint", `{"biome":"desert","height":null,"cells":[11,12]}`); rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"changed":2}` {
		t.Fatalf("paint biomes: %d %s", rec.Code, rec.Body.String())
	}
	if rec := biomePost("paint", `{"biome":"barren","height":4200,"cells":[13]}`); rec.Code != 200 {
		t.Fatalf("paint biomes: %d %s", rec.Code, rec.Body.String())
	}
	if rec := biomePost("paint", `{"biome":"mountains","cells":[13]}`); rec.Code != 400 {
		t.Errorf("mountains are relief now, not a biome: %d", rec.Code)
	}
	reliefPost := func(path, body string) *httptest.ResponseRecorder {
		return do(t, h, "POST", base+"/planet/relief/"+path, strings.NewReader(body), "application/json")
	}
	if rec := reliefPost("paint", `{"relief":"plateau","height":1800,"cells":[11,12]}`); rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"changed":2}` {
		t.Fatalf("paint relief: %d %s", rec.Code, rec.Body.String())
	}
	if rec := reliefPost("paint", `{"relief":"mountains","height":4200,"cells":[13]}`); rec.Code != 200 {
		t.Fatalf("paint relief: %d %s", rec.Code, rec.Body.String())
	}
	biomePost("paint", `{"biome":"savanna","cells":[11]}`) // paints over: one biome per hex
	for _, bad := range []string{
		`{"biome":"swamp","cells":[1]}`, `{"biome":"desert","height":20001,"cells":[1]}`,
		`{"biome":"desert","height":-20001,"cells":[1]}`, `{"biome":"desert","cells":[-1]}`,
		`{"biome":"desert","cells":[99999999]}`, `{"biome":"desert","cells":`,
	} {
		if rec := biomePost("paint", bad); rec.Code != 400 {
			t.Errorf("%s: %d", bad, rec.Code)
		}
	}
	var bl layersPayload
	json.Unmarshal(do(t, h, "GET", base+"/planet/layers", nil, "").Body.Bytes(), &bl)
	if raw, _ := json.Marshal(bl.Biomes); string(raw) != `[[11,"savanna",null],[12,"desert",null],[13,"barren",4200]]` {
		t.Errorf("biomes: %s", raw)
	}
	if raw, _ := json.Marshal(bl.Relief); string(raw) != `[[11,"plateau",1800],[12,"plateau",1800],[13,"mountains",4200]]` {
		t.Errorf("relief: %s", raw)
	}
	factOf := func(a wikiArticle, key string) string {
		for _, f := range a.Facts {
			if f.Key == key {
				return f.Value
			}
		}
		return ""
	}
	// The kingdom (11–13) has a third each; the continent (12, 13) half and half.
	if v := factOf(wikiGet(t, h, story, "location", 1), "biomes"); v != "barren:33,desert:33,savanna:33" {
		t.Errorf("kingdom biomes: %q", v)
	}
	c := wikiGet(t, h, story, "map_feature", laurasia.ID)
	if v := factOf(c, "biomes"); v != "barren:50,desert:50" {
		t.Errorf("continent biomes: %q", v)
	}
	if v := factOf(wikiGet(t, h, story, "location", 1), "terrain"); v != "plateau:67,mountains:33" {
		t.Errorf("kingdom terrain: %q", v)
	}
	if v := factOf(c, "terrain"); v != "mountains:50,plateau:50" {
		t.Errorf("continent terrain: %q", v)
	}
	for _, f := range c.Facts {
		if f.Key == "terrain" && f.Kind != "terrain" {
			t.Errorf("terrain fact kind %q", f.Kind)
		}
	}
	want2 := "≈ " + formatKm2(2*hexAreaKm2(loadMapSettings(app.db))) + " km²"
	if v := factOf(c, "area"); v != want2 || factOf(c, "type") != "Continent" {
		t.Errorf("continent area %q, want %q", v, want2)
	}

	// A new hex size carries the layers over.
	putMap(t, h, base, map[string]any{"hex_km": 250.0})
	var n int
	app.db.QueryRow(`SELECT COUNT(*) FROM feature_cells`).Scan(&n)
	if n < len(want) {
		t.Errorf("after resampling: %d cells", n)
	}
	app.db.QueryRow(`SELECT COUNT(*) FROM cell_biomes`).Scan(&n)
	if n < 3 {
		t.Errorf("after resampling: %d biome hexes", n)
	}
	var mtn float64
	if app.db.QueryRow(`SELECT height FROM cell_biomes WHERE biome = 'barren' LIMIT 1`).Scan(&mtn) != nil || mtn != 4200 {
		t.Errorf("the barren hex's height after resampling: %v", mtn)
	}
	app.db.QueryRow(`SELECT COUNT(*) FROM cell_relief`).Scan(&n)
	if n < 3 {
		t.Errorf("after resampling: %d relief hexes", n)
	}
	if app.db.QueryRow(`SELECT height FROM cell_relief WHERE relief = 'mountains' LIMIT 1`).Scan(&mtn) != nil || mtn != 4200 {
		t.Errorf("the mountains' height after resampling: %v", mtn)
	}
	if rec := biomePost("erase", `{"cells":[0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20]}`); rec.Code != 200 {
		t.Errorf("erase biomes: %d", rec.Code)
	}
	if rec := reliefPost("erase", `{"cells":[0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20]}`); rec.Code != 200 {
		t.Errorf("erase relief: %d", rec.Code)
	}
	if rec := do(t, h, "DELETE", base+"/planet/features/"+idStr(laurasia.ID), nil, ""); rec.Code != 204 {
		t.Fatalf("delete: %d", rec.Code)
	}
	app.db.QueryRow(`SELECT COUNT(*) FROM feature_cells WHERE layer = 'continents'`).Scan(&n)
	if n != 0 {
		t.Errorf("deleting the continent leaves %d cells", n)
	}
}
