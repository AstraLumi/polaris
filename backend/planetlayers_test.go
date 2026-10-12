package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// Plates, ranges and depths are drawn like major locations: one feature
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
	alps := newFeature(`{"layer":"mountains","name":"Alps","color":"#888888","value":4808}`)
	if pacific.Value != nil || pacific.Color != "#aa0000" || alps.Value == nil || *alps.Value != 4808 {
		t.Errorf("features: %+v %+v", pacific, alps)
	}
	for _, bad := range []string{`{"layer":"winds","name":"X","color":"#000000"}`, `{"layer":"plates","name":"","color":"#000000"}`, `{"layer":"plates","name":"X","color":""}`} {
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
	paint("paint", `{"feature_id":`+idStr(eurasia.ID)+`,"cells":[13,14]}`) // takes 13 from the Pacific
	paint("paint", `{"feature_id":`+idStr(alps.ID)+`,"cells":[12,13]}`)    // another layer: both stay
	paint("erase", `{"layer":"plates","cells":[10]}`)
	var l layersPayload
	json.Unmarshal(do(t, h, "GET", base+"/planet/layers", nil, "").Body.Bytes(), &l)
	got := map[[2]int64]bool{}
	for _, c := range l.Cells {
		got[c] = true
	}
	want := [][2]int64{{pacific.ID, 11}, {pacific.ID, 12}, {eurasia.ID, 13}, {eurasia.ID, 14}, {alps.ID, 12}, {alps.ID, 13}}
	if len(l.Cells) != len(want) {
		t.Errorf("cells: %v", l.Cells)
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing %v in %v", w, l.Cells)
		}
	}

	// A kingdom on cells 11–13 is on the Pacific Plate (two of its three
	// hexes) and also the Eurasian, with the Alps in it.
	app.db.Exec(`INSERT INTO locations (id, name, kind, color, realm) VALUES (1, 'Aldoria', 'major', '#123456', 'kingdom')`)
	app.db.Exec(`INSERT INTO planet_cells (location_id, cell) VALUES (1, 11), (1, 12), (1, 13)`)
	story := strings.TrimPrefix(base, "/api/s/")
	a := wikiGet(t, h, story, "location", 1)
	var plate *wikiRef
	groups := map[string]int{}
	for _, f := range a.Facts {
		if f.Key == "plate" {
			plate = f.Link
		}
	}
	for _, g := range a.Groups {
		groups[g.Key] = len(g.Items)
	}
	if plate == nil || plate.ID != pacific.ID || groups["plates"] != 2 || groups["mountain_ranges"] != 1 {
		t.Errorf("kingdom: plate %+v, groups %v", plate, groups)
	}
	p := wikiGet(t, h, story, "map_feature", pacific.ID)
	pg := map[string]int{}
	for _, g := range p.Groups {
		pg[g.Key] = len(g.Items)
	}
	if pg["lands_on"] != 1 || pg["ranges_on"] != 1 {
		t.Errorf("plate article groups: %v", pg)
	}

	// A new hex size carries the layers over.
	putMap(t, h, base, map[string]any{"hex_km": 250.0})
	var n int
	app.db.QueryRow(`SELECT COUNT(*) FROM feature_cells`).Scan(&n)
	if n < len(want) {
		t.Errorf("after resampling: %d cells", n)
	}
	if rec := do(t, h, "DELETE", base+"/planet/features/"+idStr(alps.ID), nil, ""); rec.Code != 204 {
		t.Fatalf("delete: %d", rec.Code)
	}
	app.db.QueryRow(`SELECT COUNT(*) FROM feature_cells WHERE layer = 'mountains'`).Scan(&n)
	if n != 0 {
		t.Errorf("deleting the range leaves %d cells", n)
	}
}
