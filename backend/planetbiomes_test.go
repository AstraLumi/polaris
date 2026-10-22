package main

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
)

// Schema 38 turns the mountain ranges into Mountains hexes (keeping each
// range's height), deletes the ranges with their wiki entries, and lets the
// layers hold continents instead.
func TestMigrationTurnsMountainRangesIntoBiomes(t *testing.T) {
	mgr, _ := newTestServer(t)
	app, _ := mgr.open(createStoryDirect(t, mgr, "Old"))
	rollBackSchema38(t, app.db)
	for _, q := range []string{
		`INSERT INTO map_features (id, layer, name, color, value) VALUES (1, 'mountains', 'Alps', '#888888', 4200)`,
		`INSERT INTO map_features (id, layer, name, color, value) VALUES (2, 'mountains', 'Hills', '#777777', NULL)`,
		`INSERT INTO map_features (id, layer, name, color) VALUES (3, 'plates', 'Pacific', '#aa0000')`,
		`INSERT INTO feature_cells (layer, cell, feature_id) VALUES ('mountains', 10, 1), ('mountains', 11, 1), ('mountains', 20, 2), ('plates', 10, 3)`,
		`INSERT INTO wiki_entries (id, entity_type, entity_id, summary) VALUES (1, 'map_feature', 1, 'Tall'), (2, 'map_feature', 3, 'Big')`,
		`INSERT INTO wiki_sections (entry_id, position, title, body) VALUES (1, 0, 'Peaks', 'x')`,
		`INSERT INTO wiki_infobox (entry_id, position, label, value) VALUES (1, 0, 'Range', 'x')`,
	} {
		if _, err := app.db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if _, err := app.db.Exec(`INSERT INTO map_features (layer, name, color) VALUES ('continents', 'X', '#000000')`); err == nil {
		t.Fatal("schema 37 should not know continents")
	}

	if err := ensureSchema(app.db); err != nil {
		t.Fatal(err)
	}
	var v string
	app.db.QueryRow(`SELECT value FROM schema_meta WHERE key = 'schema_version'`).Scan(&v)
	if v != currentSchemaVersion {
		t.Fatalf("version %s", v)
	}

	// The ranges became Mountains hexes (schema 38), which schema 39 then
	// moved to the relief, leaving bare rock in the biomes.
	if got := cellKindsOf(t, app.db, "cell_biomes", "biome"); got != "10:barren 11:barren 20:barren" {
		t.Errorf("biomes: %v", got)
	}
	if got := cellKindsOf(t, app.db, "cell_relief", "relief"); got != "10:mountains@4200 11:mountains@4200 20:mountains" {
		t.Errorf("relief: %v", got)
	}

	count := func(q string) (n int) {
		app.db.QueryRow(q).Scan(&n)
		return
	}
	if n := count(`SELECT COUNT(*) FROM map_features WHERE layer = 'mountains'`); n != 0 {
		t.Errorf("%d ranges left", n)
	}
	if n := count(`SELECT COUNT(*) FROM feature_cells WHERE layer = 'mountains'`); n != 0 {
		t.Errorf("%d range cells left", n)
	}
	if n := count(`SELECT COUNT(*) FROM wiki_entries WHERE entity_type = 'map_feature' AND entity_id IN (1, 2)`); n != 0 {
		t.Errorf("%d range articles left", n)
	}
	if n := count(`SELECT COUNT(*) FROM wiki_sections`) + count(`SELECT COUNT(*) FROM wiki_infobox`); n != 0 {
		t.Errorf("%d section and infobox rows of a deleted article left", n)
	}
	if n := count(`SELECT COUNT(*) FROM wiki_entries WHERE entity_id = 3`) + count(`SELECT COUNT(*) FROM map_features WHERE id = 3`) + count(`SELECT COUNT(*) FROM feature_cells WHERE feature_id = 3`); n != 3 {
		t.Errorf("the plate was touched: %d", n)
	}

	if _, err := app.db.Exec(`INSERT INTO map_features (layer, name, color) VALUES ('continents', 'Laurasia', '#000000')`); err != nil {
		t.Errorf("continents: %v", err)
	}
	if _, err := app.db.Exec(`INSERT INTO map_features (layer, name, color) VALUES ('mountains', 'Alps', '#000000')`); err == nil {
		t.Error("mountains should be refused now")
	}

	// A story that is already at 38 is left alone by the rewrite.
	if err := ensureSchema(app.db); err != nil {
		t.Fatal(err)
	}
}

// cellKindsOf lists a per-cell table as "cell:key@height", ordered by cell.
func cellKindsOf(t *testing.T, db *sql.DB, table, col string) string {
	t.Helper()
	rows, err := db.Query(`SELECT cell, ` + col + `, height FROM ` + table + ` ORDER BY cell`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var cell int
		var key string
		var h *float64
		rows.Scan(&cell, &key, &h)
		s := fmt.Sprintf("%d:%s", cell, key)
		if h != nil {
			s += fmt.Sprintf("@%g", *h)
		}
		got = append(got, s)
	}
	return strings.Join(got, " ")
}

// Schema 39 moves hills, plateaus and mountains from the biomes to the
// relief, with their heights; the mountains stay as bare rock.
func TestMigrationMovesLandformsToRelief(t *testing.T) {
	mgr, _ := newTestServer(t)
	app, _ := mgr.open(createStoryDirect(t, mgr, "Old"))
	rollBackSchema39(t, app.db)
	if _, err := app.db.Exec(`INSERT INTO cell_relief (cell, relief) VALUES (1, 'hills')`); err == nil {
		t.Fatal("schema 38 should not know the relief")
	}
	if _, err := app.db.Exec(`INSERT INTO cell_biomes (cell, biome, height) VALUES
		(1, 'hills', 700), (2, 'plateau', NULL), (3, 'mountains', 4200), (4, 'desert', NULL), (5, 'tundra', 30)`); err != nil {
		t.Fatal(err)
	}
	if err := ensureSchema(app.db); err != nil {
		t.Fatal(err)
	}
	var v string
	app.db.QueryRow(`SELECT value FROM schema_meta WHERE key = 'schema_version'`).Scan(&v)
	if v != currentSchemaVersion {
		t.Fatalf("version %s", v)
	}
	if got := cellKindsOf(t, app.db, "cell_relief", "relief"); got != "1:hills@700 2:plateau 3:mountains@4200" {
		t.Errorf("relief: %v", got)
	}
	if got := cellKindsOf(t, app.db, "cell_biomes", "biome"); got != "3:barren 4:desert 5:tundra@30" {
		t.Errorf("biomes: %v", got)
	}
}

// Erasing clears the biomes of the cells it is given and no others.
func TestEraseBiomes(t *testing.T) {
	app, h, base := planetSetup(t)
	putMap(t, h, base, nil)
	post := func(path, body string) int {
		return do(t, h, "POST", base+"/planet/biomes/"+path, strings.NewReader(body), "application/json").Code
	}
	if c := post("paint", `{"biome":"tundra","cells":[1,2,3]}`); c != 200 {
		t.Fatal(c)
	}
	if c := post("erase", `{"cells":[2,3,4]}`); c != 200 {
		t.Fatal(c)
	}
	if c := post("erase", `{"cells":[-5]}`); c != 400 {
		t.Errorf("erase off the planet: %d", c)
	}
	var cells []int
	rows, _ := app.db.Query(`SELECT cell FROM cell_biomes`)
	defer rows.Close()
	for rows.Next() {
		var c int
		rows.Scan(&c)
		cells = append(cells, c)
	}
	if len(cells) != 1 || cells[0] != 1 {
		t.Errorf("cells left: %v", cells)
	}
}

// The relief is painted, replaced and erased like the biomes, but only the
// three landforms are accepted.
func TestPaintRelief(t *testing.T) {
	app, h, base := planetSetup(t)
	putMap(t, h, base, nil)
	post := func(path, body string) int {
		return do(t, h, "POST", base+"/planet/relief/"+path, strings.NewReader(body), "application/json").Code
	}
	if c := post("paint", `{"relief":"plateau","height":1800,"cells":[1,2,3]}`); c != 200 {
		t.Fatal(c)
	}
	if c := post("paint", `{"relief":"hills","height":null,"cells":[3]}`); c != 200 {
		t.Fatal(c)
	}
	for _, bad := range []string{
		`{"relief":"desert","cells":[1]}`, `{"relief":"","cells":[1]}`, `{"biome":"hills","cells":[1]}`,
		`{"relief":"hills","height":20001,"cells":[1]}`, `{"relief":"hills","cells":[-1]}`,
	} {
		if c := post("paint", bad); c != 400 {
			t.Errorf("%s: %d", bad, c)
		}
	}
	if rec := do(t, h, "POST", base+"/planet/relief/paint", strings.NewReader(`{"relief":"swamp","cells":[1]}`), "application/json"); !strings.Contains(rec.Body.String(), "unknown terrain") {
		t.Errorf("answer: %s", rec.Body.String())
	}
	if c := post("erase", `{"cells":[2]}`); c != 200 {
		t.Fatal(c)
	}
	if got := cellKindsOf(t, app.db, "cell_relief", "relief"); got != "1:plateau@1800 3:hills" {
		t.Errorf("relief: %v", got)
	}
	// The biomes are a separate layer: they know nothing of relief.
	if n := cellKindsOf(t, app.db, "cell_biomes", "biome"); n != "" {
		t.Errorf("biomes: %v", n)
	}
}

func TestFormatKm2(t *testing.T) {
	for in, want := range map[float64]string{12345678: "12,300,000", 1234: "1,230", 999.6: "1,000", 950: "950", 12.34: "12.3", 0: "0"} {
		if got := formatKm2(in); got != want {
			t.Errorf("formatKm2(%v) = %q, want %q", in, got, want)
		}
	}
}
