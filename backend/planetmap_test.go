package main

import (
	"encoding/json"
	"math"
	"net/http"
	"slices"
	"testing"
)

type planetJSON struct {
	Settings  mapSettings   `json:"settings"`
	Locations []mapLocation `json:"locations"`
	Cells     [][2]int64    `json:"cells"`
}

func planetSetup(t *testing.T) (*storyApp, http.Handler, string) {
	t.Helper()
	mgr, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Planet"})
	app, _ := mgr.open(s.ID)
	return app, h, "/api/s/" + s.ID
}

func putMap(t *testing.T, h http.Handler, base string, v map[string]any) mapSettings {
	t.Helper()
	in := map[string]any{"mode": "3d", "planet_km": 40075.0, "hex_km": 500.0, "tilt": 23.44, "bestiary_projection": "mollweide"}
	for k, x := range v {
		in[k] = x
	}
	rec := do(t, h, "PUT", base+"/settings/map", jsonBody(in), "application/json")
	if rec.Code != 200 {
		t.Fatalf("map settings: %d %s", rec.Code, rec.Body.String())
	}
	var out mapSettings
	json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

func getPlanet(t *testing.T, h http.Handler, base string) planetJSON {
	t.Helper()
	var p planetJSON
	json.Unmarshal(do(t, h, "GET", base+"/planet", nil, "").Body.Bytes(), &p)
	return p
}

func cellsOfLoc(p planetJSON, id int64) []int64 {
	var out []int64
	for _, c := range p.Cells {
		if c[0] == id {
			out = append(out, c[1])
		}
	}
	slices.Sort(out)
	return out
}

func TestMapSettings(t *testing.T) {
	_, h, base := planetSetup(t)
	var s struct {
		Map mapSettings `json:"map"`
	}
	json.Unmarshal(do(t, h, "GET", base+"/settings", nil, "").Body.Bytes(), &s)
	if s.Map.Mode != "2d" || s.Map.Planet.Frequency != 141 {
		t.Fatalf("a story starts 2D only, its planet scaled like the 2D map: %+v", s.Map)
	}
	got := putMap(t, h, base, nil)
	if got.Mode != "3d" || got.Planet.Frequency != 14 || got.Cells != 10*14*14+2 || got.Planet.BestiaryProjection != "mollweide" {
		t.Errorf("saved: %+v", got)
	}
	if math.Abs(got.HexKm-500) > 30 {
		t.Errorf("cells are about as wide as asked: %v km", got.HexKm)
	}
	if rec := do(t, h, "PUT", base+"/settings/map", jsonBody(map[string]any{"mode": "3d", "planet_km": 0, "hex_km": 5}), "application/json"); rec.Code != 400 {
		t.Errorf("a planet of size 0: %d", rec.Code)
	}
}

// Painting the planet: a coloured location takes cells over, a colourless
// one joins in; erasing; places on cells.
func TestPaintPlanet(t *testing.T) {
	app, h, base := planetSetup(t)
	putMap(t, h, base, nil)
	app.db.Exec(`INSERT INTO locations (id, name, kind, color) VALUES (1, 'Aldoria', 'major', '#aa0000'),
		(2, 'Borea', 'major', '#00aa00'), (3, 'Highlands', 'major', NULL)`)
	paint := func(id int64, cells ...int64) {
		t.Helper()
		if rec := do(t, h, "POST", base+"/planet/paint", jsonBody(cellBatch{LocationID: &id, Cells: cells}), "application/json"); rec.Code != 200 {
			t.Fatalf("paint: %d %s", rec.Code, rec.Body.String())
		}
	}
	paint(1, 10, 11, 12)
	paint(2, 12, 13)
	paint(3, 11, 12)
	p := getPlanet(t, h, base)
	if !slices.Equal(cellsOfLoc(p, 1), []int64{10, 11}) || !slices.Equal(cellsOfLoc(p, 2), []int64{12, 13}) || !slices.Equal(cellsOfLoc(p, 3), []int64{11, 12}) {
		t.Errorf("cells: %v", p.Cells)
	}
	one := int64(1)
	do(t, h, "POST", base+"/planet/erase", jsonBody(cellBatch{LocationID: &one, Cells: []int64{10}}), "application/json")
	do(t, h, "POST", base+"/planet/erase", jsonBody(cellBatch{Cells: []int64{12}}), "application/json")
	p = getPlanet(t, h, base)
	if !slices.Equal(cellsOfLoc(p, 1), []int64{11}) || !slices.Equal(cellsOfLoc(p, 2), []int64{13}) || !slices.Equal(cellsOfLoc(p, 3), []int64{11}) {
		t.Errorf("after erasing: %v", p.Cells)
	}
	if rec := do(t, h, "POST", base+"/planet/paint", jsonBody(cellBatch{LocationID: &one, Cells: []int64{1e6}}), "application/json"); rec.Code != 400 {
		t.Errorf("a cell off the planet: %d", rec.Code)
	}

	// A town on a cell, then moved; its 2D hex stays.
	rec := do(t, h, "POST", base+"/map/locations", jsonBody(map[string]any{"kind": "minor", "name": "Thornby", "cell": 11}), "application/json")
	var town mapLocation
	json.Unmarshal(rec.Body.Bytes(), &town)
	if town.Cell == nil || *town.Cell != 11 || town.Q != nil {
		t.Fatalf("town: %d %+v", rec.Code, town)
	}
	if rec := do(t, h, "POST", base+"/map/locations", jsonBody(map[string]any{"kind": "minor", "name": "Nowhere"}), "application/json"); rec.Code != 400 {
		t.Errorf("a town with no hex: %d", rec.Code)
	}
	facts := wikiFacts(app, "location", town.ID)
	if facts["part_of"] != "Aldoria" {
		t.Errorf("on the planet, a town is part of the kingdom on its cell: %v", facts)
	}
}

func wikiFacts(app *storyApp, typ string, id int64) map[string]string {
	a := &wikiArticle{Type: typ, ID: id}
	loadSourceFacts(app.db, a)
	out := map[string]string{}
	for _, f := range a.Facts {
		out[f.Key] = f.Value
	}
	for _, g := range a.Groups {
		var names []string
		for _, r := range g.Items {
			names = append(names, r.Name)
		}
		b, _ := json.Marshal(names)
		out["group:"+g.Key] = string(b)
	}
	return out
}

// Porting the 2D map: the 2D equator becomes latitude 0, and each cell,
// place and creature territory takes what the 2D hex under it had.
func TestPortToPlanet(t *testing.T) {
	app, h, base := planetSetup(t)
	// 500 km hexes on both maps; the 2D equator on row 0, South from it down.
	do(t, h, "PUT", base+"/map/equator", jsonBody(map[string]any{"show": true, "row": 0, "center": 0, "hex_km": 500}), "application/json")
	putMap(t, h, base, nil)
	for _, q := range []string{
		`INSERT INTO locations (id, name, kind, color) VALUES (1, 'North', 'major', '#aa0000'), (2, 'South', 'major', '#00aa00')`,
		`INSERT INTO locations (id, name, kind, q, r) VALUES (3, 'Capital', 'minor', 2, -6)`,
		`INSERT INTO creatures (id, name, kind, custom_territory) VALUES (1, 'Wolf', 'animal', 1)`,
		`INSERT INTO creature_hexes (creature_id, q, r) VALUES (1, 0, 4)`,
	} {
		if _, err := app.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	for q := -5; q <= 5; q++ {
		for r := -8; r <= -1; r++ {
			app.db.Exec(`INSERT INTO location_hexes (location_id, q, r) VALUES (1, ?, ?)`, q-r/2, r)
		}
		for r := 0; r <= 8; r++ {
			app.db.Exec(`INSERT INTO location_hexes (location_id, q, r) VALUES (2, ?, ?)`, q-r/2, r)
		}
	}
	if rec := do(t, h, "POST", base+"/planet/port", jsonBody(map[string]any{"projection": "equirectangular"}), "application/json"); rec.Code != 200 {
		t.Fatalf("port: %d %s", rec.Code, rec.Body.String())
	}
	p := getPlanet(t, h, base)
	g := p.Settings.Planet.grid()
	north, south := cellsOfLoc(p, 1), cellsOfLoc(p, 2)
	if len(north) < 30 || len(south) < 30 {
		t.Fatalf("ported cells: %d north, %d south", len(north), len(south))
	}
	for _, c := range north {
		if lat, _ := latLon(g.center(int(c))); lat <= 0 {
			t.Errorf("north's cell %d at %.1f°", c, lat*180/math.Pi)
		}
	}
	for _, c := range south {
		if lat, _ := latLon(g.center(int(c))); lat > 1e-9 {
			t.Errorf("south's cell %d at %.1f°", c, lat*180/math.Pi)
		}
	}
	var town mapLocation
	for _, l := range p.Locations {
		if l.ID == 3 {
			town = l
		}
	}
	if town.Cell == nil || !slices.Contains(north, *town.Cell) {
		t.Errorf("the capital lands in North: %+v", town.Cell)
	}
	var wolf creatureOut
	json.Unmarshal(do(t, h, "GET", base+"/creatures/1", nil, "").Body.Bytes(), &wolf)
	if len(wolf.Cells) == 0 || !slices.Contains(south, wolf.Cells[0]) {
		t.Errorf("the wolf's territory comes along: %v", wolf.Cells)
	}

	facts := wikiFacts(app, "location", 1)
	if facts["hemisphere"] != "Northern hemisphere" {
		t.Errorf("on the planet, facts come from the cells: %v", facts)
	}
	if facts["group:borders"] != `["South","Unnamed ocean"]` {
		t.Errorf("borders: %v", facts["group:borders"])
	}

	// Smaller cells: everything moves onto the new grid.
	putMap(t, h, base, map[string]any{"hex_km": 250.0})
	p2 := getPlanet(t, h, base)
	if n := len(cellsOfLoc(p2, 1)); n < 3*len(north) || n > 5*len(north) {
		t.Errorf("halving the cells' width about quadruples them: %d -> %d", len(north), n)
	}
	for _, l := range p2.Locations {
		if l.ID == 3 && (l.Cell == nil || !slices.Contains(cellsOfLoc(p2, 1), *l.Cell)) {
			t.Errorf("the capital stays in North: %v", l.Cell)
		}
	}
}

// Borders on the 2D map, and what a coloured location is.
func TestBordersAndRealms(t *testing.T) {
	app, h, base := planetSetup(t)
	create := func(name, color, realm string) int64 {
		rec := do(t, h, "POST", base+"/map/locations", jsonBody(map[string]any{"kind": "major", "name": name, "color": color, "realm": realm}), "application/json")
		var l mapLocation
		json.Unmarshal(rec.Body.Bytes(), &l)
		return l.ID
	}
	k := create("Aldoria", "#aa0000", "kingdom")
	sea := create("Grey Sea", "#0000aa", "ocean")
	waste := create("Ashen Wastes", "#999999", "region")
	for _, q := range []string{
		`INSERT INTO location_hexes (location_id, q, r) VALUES (?1, 0, 0), (?1, 1, 0)`,
	} {
		app.db.Exec(q, k)
	}
	app.db.Exec(`INSERT INTO location_hexes (location_id, q, r) VALUES (?, 2, 0)`, sea)
	app.db.Exec(`INSERT INTO location_hexes (location_id, q, r) VALUES (?, -1, 0)`, waste)
	var n int
	app.db.QueryRow(`SELECT COUNT(*) FROM factions`).Scan(&n)
	if n != 1 {
		t.Errorf("only the kingdom gets a faction: %d", n)
	}
	f := wikiFacts(app, "location", k)
	if f["type"] != "Kingdom" || f["group:borders"] != `["Ashen Wastes","Grey Sea","Unnamed ocean"]` {
		t.Errorf("kingdom: %v", f)
	}
	if f := wikiFacts(app, "location", sea); f["type"] != "Ocean" || f["group:borders"] != `["Aldoria","Unnamed ocean"]` {
		t.Errorf("sea: %v", f)
	}
	// A kingdom becoming a region lets its faction go.
	do(t, h, "PUT", base+"/map/locations/"+idStr(k), jsonBody(map[string]any{"name": "Aldoria", "color": "#aa0000", "realm": "region"}), "application/json")
	app.db.QueryRow(`SELECT COUNT(*) FROM factions WHERE kingdom_id IS NOT NULL`).Scan(&n)
	if n != 0 {
		t.Errorf("a region has no faction: %d tied", n)
	}
}

// A creature's territory painted on the planet keeps its 2D hexes.
func TestCreatureCells(t *testing.T) {
	app, h, base := planetSetup(t)
	putMap(t, h, base, nil)
	app.db.Exec(`INSERT INTO creatures (id, name, kind, custom_territory) VALUES (1, 'Wolf', 'animal', 1)`)
	app.db.Exec(`INSERT INTO creature_hexes (creature_id, q, r) VALUES (1, 3, 3)`)
	body, ct := form(t, map[string]string{"name": "Wolf", "kind": "animal", "territory_cells": "[5, 6]"}, "", "", nil)
	if rec := do(t, h, "PUT", base+"/creatures/1", body, ct); rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	var c creatureOut
	json.Unmarshal(do(t, h, "GET", base+"/creatures/1", nil, "").Body.Bytes(), &c)
	if !slices.Equal(c.Cells, []int64{5, 6}) || len(c.Territory) != 1 {
		t.Errorf("cells %v, hexes %v", c.Cells, c.Territory)
	}
	body, ct = form(t, map[string]string{"name": "Wolf", "kind": "animal", "territory_cells": "[99999]"}, "", "", nil)
	if rec := do(t, h, "PUT", base+"/creatures/1", body, ct); rec.Code != 400 {
		t.Errorf("a cell off the planet: %d", rec.Code)
	}
}

func TestMigrationAddsThePlanet(t *testing.T) {
	mgr, _ := newTestServer(t)
	app, _ := mgr.open(createStoryDirect(t, mgr, "Old"))
	rollBackSchema27(t, app.db)
	app.db.Exec(`INSERT INTO locations (id, name, kind, color) VALUES (1, 'Aldoria', 'major', '#aa0000')`)
	if err := ensureSchema(app.db); err != nil {
		t.Fatal(err)
	}
	var realm string
	if err := app.db.QueryRow(`SELECT realm FROM locations WHERE id = 1`).Scan(&realm); err != nil || realm != "kingdom" {
		t.Errorf("an existing kingdom stays one: %q %v", realm, err)
	}
	if _, err := app.db.Exec(`INSERT INTO planet_cells (location_id, cell) VALUES (1, 5)`); err != nil {
		t.Fatal(err)
	}
}

// Saving a location without its other map's position keeps it.
func TestLocationKeepsItsOtherPlace(t *testing.T) {
	app, h, base := planetSetup(t)
	app.db.Exec(`INSERT INTO locations (id, name, kind, q, r, cell) VALUES (1, 'Thornby', 'minor', 3, 4, 77)`)
	app.db.Exec(`INSERT INTO locations (id, name, kind, color, realm) VALUES (2, 'Grey Sea', 'major', '#0000aa', 'ocean')`)
	do(t, h, "PUT", base+"/map/locations/1", jsonBody(map[string]any{"name": "Thornby", "q": 5, "r": 6}), "application/json")
	do(t, h, "PUT", base+"/map/locations/2", jsonBody(map[string]any{"name": "Grey Sea", "color": "#0000aa"}), "application/json")
	var q, r, cell int
	var realm string
	app.db.QueryRow(`SELECT q, r, cell FROM locations WHERE id = 1`).Scan(&q, &r, &cell)
	app.db.QueryRow(`SELECT realm FROM locations WHERE id = 2`).Scan(&realm)
	if q != 5 || r != 6 || cell != 77 || realm != "ocean" {
		t.Errorf("q %d r %d cell %d realm %s", q, r, cell, realm)
	}
}
