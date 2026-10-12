package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"math"
	"net/http"
)

// A story's map is either flat (the 2D hex map, "2D only") or a planet (the
// 3D globe; its flat views are projections of it). Both are kept: the 2D
// tables (location_hexes, locations.q/r, creature_hexes) are the 2D-only
// version, and the planet has its own (planet_cells, locations.cell,
// creature_cells), filled by painting the globe or by porting the 2D map
// onto it. Whatever reads positions (the wiki, creature territories) asks
// mapMode which one counts.

const (
	settingMapMode = "map_mode"
	settingPlanet  = "planet"
)

const planetCellsTableSQL = `CREATE TABLE planet_cells (
	location_id INTEGER NOT NULL REFERENCES locations(id) ON DELETE CASCADE,
	cell        INTEGER NOT NULL,
	PRIMARY KEY (location_id, cell)
)`

const creatureCellsTableSQL = `CREATE TABLE creature_cells (
	creature_id INTEGER NOT NULL REFERENCES creatures(id) ON DELETE CASCADE,
	cell        INTEGER NOT NULL,
	PRIMARY KEY (creature_id, cell)
)`

// schema27Statements upgrade a database to schema 27: the planet, and what
// a coloured major location is (a kingdom, a region or an ocean). The fresh
// schema runs them too.
var schema27Statements = []string{
	`ALTER TABLE locations ADD COLUMN realm TEXT NOT NULL DEFAULT 'kingdom' CHECK (realm IN ('kingdom', 'region', 'ocean'))`,
	`ALTER TABLE locations ADD COLUMN cell INTEGER`,
	planetCellsTableSQL,
	`CREATE INDEX idx_planet_cells_cell ON planet_cells(cell)`,
	creatureCellsTableSQL,
}

// What a coloured major location is. Only a kingdom has a faction; a region
// is land too wild to rule, an ocean is water.
var realms = map[string]string{"kingdom": "Kingdom", "region": "Region", "ocean": "Ocean"}

type planetSettings struct {
	Frequency int     `json:"frequency"` // the grid's (planetgrid.go)
	PlanetKm  float64 `json:"planet_km"` // circumference at the equator
	Tilt      float64 `json:"tilt"`      // axial tilt, degrees
	// The projection the bestiary's territory pictures are drawn in.
	BestiaryProjection string `json:"bestiary_projection"`
	ShowEquator        bool   `json:"show_equator"`
	ShowZones          bool   `json:"show_zones"`
}

type mapSettings struct {
	Mode   string         `json:"mode"` // "2d" | "3d"
	Planet planetSettings `json:"planet"`
	// Derived, sent for show: a cell's width and how many cells there are.
	HexKm float64 `json:"hex_km"`
	Cells int     `json:"cells"`
}

// The planet starts from the 2D map's scale (planet.go).
func defaultPlanet(db *sql.DB) planetSettings {
	eq := loadEquator(db)
	return planetSettings{
		Frequency: frequencyFor(eq.PlanetKm, eq.HexKm), PlanetKm: eq.PlanetKm, Tilt: eq.Tilt,
		BestiaryProjection: "equirectangular", ShowEquator: true,
	}
}

func loadMapSettings(db *sql.DB) mapSettings {
	s := mapSettings{Mode: "2d", Planet: defaultPlanet(db)}
	var raw string
	if db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, settingMapMode).Scan(&raw) == nil && raw == "3d" {
		s.Mode = "3d"
	}
	if db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, settingPlanet).Scan(&raw) == nil {
		json.Unmarshal([]byte(raw), &s.Planet)
	}
	s.HexKm = hexKmOf(s.Planet.PlanetKm, s.Planet.Frequency)
	s.Cells = planetGrid{s.Planet.Frequency}.cellCount()
	return s
}

func mapMode(db *sql.DB) string { return loadMapSettings(db).Mode }

func (p planetSettings) grid() planetGrid { return planetGrid{p.Frequency} }

// ---- Settings ------------------------------------------------------------------

// The settings as typed: the planet's size and its cells' width, from which
// the grid's frequency follows.
type mapSettingsInput struct {
	Mode               string  `json:"mode"`
	PlanetKm           float64 `json:"planet_km"`
	HexKm              float64 `json:"hex_km"`
	Tilt               float64 `json:"tilt"`
	BestiaryProjection string  `json:"bestiary_projection"`
	ShowEquator        bool    `json:"show_equator"`
	ShowZones          bool    `json:"show_zones"`
}

func updateMapSettingsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in mapSettingsInput
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if in.Mode != "3d" {
			in.Mode = "2d"
		}
		if !(in.PlanetKm > 0 && in.PlanetKm <= 1e9 && in.HexKm > 0 && in.HexKm <= 1e9) {
			http.Error(w, "the hex size and the planet's size must be more than 0", http.StatusBadRequest)
			return
		}
		if !(in.Tilt >= 0 && in.Tilt <= 90) {
			http.Error(w, "the axial tilt is between 0 and 90 degrees", http.StatusBadRequest)
			return
		}
		if !validProjection(in.BestiaryProjection) {
			in.BestiaryProjection = "equirectangular"
		}
		old := loadMapSettings(db)
		next := planetSettings{
			Frequency: frequencyFor(in.PlanetKm, in.HexKm), PlanetKm: in.PlanetKm, Tilt: in.Tilt,
			BestiaryProjection: in.BestiaryProjection, ShowEquator: in.ShowEquator, ShowZones: in.ShowZones,
		}

		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to save the map settings", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()
		// New cell sizes: what was painted moves onto the new cells.
		if next.Frequency != old.Planet.Frequency {
			if err := resamplePlanet(tx, old.Planet.grid(), next.grid()); err != nil {
				log.Printf("resample planet: %v", err)
				http.Error(w, "failed to save the map settings", http.StatusInternalServerError)
				return
			}
		}
		raw, _ := json.Marshal(next)
		for key, val := range map[string]string{settingMapMode: in.Mode, settingPlanet: string(raw)} {
			if _, err := tx.Exec(`INSERT INTO app_settings (key, value) VALUES (?, ?)
				ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, val); err != nil {
				log.Printf("save map settings: %v", err)
				http.Error(w, "failed to save the map settings", http.StatusInternalServerError)
				return
			}
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to save the map settings", http.StatusInternalServerError)
			return
		}
		writeJSON(w, loadMapSettings(db))
	}
}

// resamplePlanet moves everything on the planet from grid a to grid b: each
// new cell takes what the old cell under its centre had, and each place
// moves to the new cell under its old one.
func resamplePlanet(tx *sql.Tx, a, b planetGrid) error {
	type cellRow struct{ owner, cell int64 }
	read := func(q string) (map[int64][]int64, error) {
		rows, err := tx.Query(q)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := map[int64][]int64{}
		for rows.Next() {
			var c cellRow
			if err := rows.Scan(&c.owner, &c.cell); err != nil {
				return nil, err
			}
			out[c.cell] = append(out[c.cell], c.owner)
		}
		return out, rows.Err()
	}
	areas, err := read(`SELECT location_id, cell FROM planet_cells`)
	if err != nil {
		return err
	}
	territories, err := read(`SELECT creature_id, cell FROM creature_cells`)
	if err != nil {
		return err
	}
	places, err := read(`SELECT id, cell FROM locations WHERE cell IS NOT NULL`)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM planet_cells`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM creature_cells`); err != nil {
		return err
	}
	if len(areas)+len(territories) > 0 {
		insArea, err := tx.Prepare(`INSERT OR IGNORE INTO planet_cells (location_id, cell) VALUES (?, ?)`)
		if err != nil {
			return err
		}
		defer insArea.Close()
		insTerr, err := tx.Prepare(`INSERT OR IGNORE INTO creature_cells (creature_id, cell) VALUES (?, ?)`)
		if err != nil {
			return err
		}
		defer insTerr.Close()
		for c := range b.cellCount() {
			old := int64(a.cellAt(b.center(c)))
			for _, id := range areas[old] {
				if _, err := insArea.Exec(id, c); err != nil {
					return err
				}
			}
			for _, id := range territories[old] {
				if _, err := insTerr.Exec(id, c); err != nil {
					return err
				}
			}
		}
	}
	for cell, ids := range places {
		for _, id := range ids {
			if _, err := tx.Exec(`UPDATE locations SET cell = ? WHERE id = ?`, b.cellAt(a.center(int(cell))), id); err != nil {
				return err
			}
		}
	}
	return nil
}

// ---- The planet's data ---------------------------------------------------------

type planetPayload struct {
	Settings  mapSettings   `json:"settings"`
	Locations []mapLocation `json:"locations"`
	// Every cell a major location covers, as [location id, cell].
	Cells [][2]int64 `json:"cells"`
}

func getPlanetHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := planetPayload{Settings: loadMapSettings(db), Locations: []mapLocation{}, Cells: [][2]int64{}}
		rows, err := db.Query(mapLocationSelect + ` ORDER BY name COLLATE NOCASE`)
		if err != nil {
			http.Error(w, "failed to load the planet", http.StatusInternalServerError)
			log.Printf("planet locations: %v", err)
			return
		}
		for rows.Next() {
			l, err := scanMapLocation(rows)
			if err != nil {
				rows.Close()
				http.Error(w, "failed to load the planet", http.StatusInternalServerError)
				return
			}
			p.Locations = append(p.Locations, l)
		}
		rows.Close()
		cells, err := db.Query(`SELECT location_id, cell FROM planet_cells`)
		if err != nil {
			http.Error(w, "failed to load the planet", http.StatusInternalServerError)
			return
		}
		defer cells.Close()
		for cells.Next() {
			var c [2]int64
			if cells.Scan(&c[0], &c[1]) == nil {
				p.Cells = append(p.Cells, c)
			}
		}
		writeJSON(w, p)
	}
}

type cellBatch struct {
	LocationID *int64  `json:"location_id"`
	Cells      []int64 `json:"cells"`
}

// readCellBatch reads a paint or erase and checks every cell is on the planet.
func readCellBatch(db *sql.DB, w http.ResponseWriter, r *http.Request) (cellBatch, bool) {
	var b cellBatch
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return b, false
	}
	if len(b.Cells) > maxHexBatch {
		http.Error(w, "too many hexes in one request", http.StatusBadRequest)
		return b, false
	}
	count := int64(loadMapSettings(db).Cells)
	for _, c := range b.Cells {
		if c < 0 || c >= count {
			http.Error(w, "that hex isn't on the planet", http.StatusBadRequest)
			return b, false
		}
	}
	return b, true
}

// paintCellsHandler is paintHexesHandler for the planet: a coloured location
// takes the cell over from any other coloured one.
func paintCellsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, ok := readCellBatch(db, w, r)
		if !ok {
			return
		}
		if b.LocationID == nil {
			http.Error(w, "location_id and hexes are required", http.StatusBadRequest)
			return
		}
		var kind string
		var color sql.NullString
		if err := db.QueryRow(`SELECT kind, color FROM locations WHERE id = ?`, *b.LocationID).Scan(&kind, &color); err != nil {
			http.Error(w, "location not found", http.StatusNotFound)
			return
		}
		if kind != "major" {
			http.Error(w, "only major locations can be painted onto hexes", http.StatusBadRequest)
			return
		}
		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to paint", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()
		for _, c := range b.Cells {
			if color.Valid {
				if _, err := tx.Exec(`DELETE FROM planet_cells WHERE cell = ? AND location_id != ?
					AND location_id IN (SELECT id FROM locations WHERE kind = 'major' AND color IS NOT NULL)`, c, *b.LocationID); err != nil {
					log.Printf("paint cells (clear): %v", err)
					http.Error(w, "failed to paint", http.StatusInternalServerError)
					return
				}
			}
			if _, err := tx.Exec(`INSERT OR IGNORE INTO planet_cells (location_id, cell) VALUES (?, ?)`, *b.LocationID, c); err != nil {
				log.Printf("paint cells: %v", err)
				http.Error(w, "failed to paint", http.StatusInternalServerError)
				return
			}
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to paint", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"painted": len(b.Cells)})
	}
}

func eraseCellsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, ok := readCellBatch(db, w, r)
		if !ok {
			return
		}
		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to erase", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()
		for _, c := range b.Cells {
			var err error
			if b.LocationID != nil {
				_, err = tx.Exec(`DELETE FROM planet_cells WHERE location_id = ? AND cell = ?`, *b.LocationID, c)
			} else {
				_, err = tx.Exec(`DELETE FROM planet_cells WHERE cell = ?`, c)
			}
			if err != nil {
				log.Printf("erase cells: %v", err)
				http.Error(w, "failed to erase", http.StatusInternalServerError)
				return
			}
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to erase", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"erased": len(b.Cells)})
	}
}

// ---- Porting the 2D map ----------------------------------------------------------

// portToPlanet replaces everything on the planet with the 2D map, read as
// drawn in projection proj: the 2D equator (planet.go) is latitude 0 and its
// centre column longitude 0, and the 2D hexes are the 2D equator's hex_km
// across. Each planet cell takes what the 2D hex under it had; each place
// goes to the cell under its hex.
func portToPlanet(tx *sql.Tx, eq mapEquator, planet planetSettings, proj string) error {
	g := planet.grid()
	p := projectionByName(proj)
	radius := planet.PlanetKm / (2 * math.Pi)
	rowKm := eq.HexKm * math.Sqrt(3) / 2
	equatorRow := func(col float64) float64 {
		off := (col - eq.Center) / 100
		return eq.Row + eq.Bend*off*off
	}

	read := func(q string) (map[[2]int64][]int64, error) {
		rows, err := tx.Query(q)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := map[[2]int64][]int64{}
		for rows.Next() {
			var id, q, r int64
			if err := rows.Scan(&id, &q, &r); err != nil {
				return nil, err
			}
			out[[2]int64{q, r}] = append(out[[2]int64{q, r}], id)
		}
		return out, rows.Err()
	}
	areas, err := read(`SELECT location_id, q, r FROM location_hexes`)
	if err != nil {
		return err
	}
	territories, err := read(`SELECT creature_id, q, r FROM creature_hexes`)
	if err != nil {
		return err
	}
	places, err := read(`SELECT id, q, r FROM locations WHERE kind = 'minor' AND q IS NOT NULL AND r IS NOT NULL`)
	if err != nil {
		return err
	}

	for _, q := range []string{`DELETE FROM planet_cells`, `DELETE FROM creature_cells`, `UPDATE locations SET cell = NULL`} {
		if _, err := tx.Exec(q); err != nil {
			return err
		}
	}
	insArea, err := tx.Prepare(`INSERT OR IGNORE INTO planet_cells (location_id, cell) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer insArea.Close()
	insTerr, err := tx.Prepare(`INSERT OR IGNORE INTO creature_cells (creature_id, cell) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer insTerr.Close()

	// The cell under a 2D hex's centre.
	cellUnder := func(hex [2]int64) (int, bool) {
		col := float64(hex[0]) + float64(hex[1])/2
		y := (equatorRow(col) - float64(hex[1])) * rowKm / radius
		x := (col - eq.Center) * eq.HexKm / radius
		lon, lat, ok := p.inverse(x, y)
		if !ok {
			return 0, false
		}
		return g.cellAt(fromLatLon(lat, lon)), true
	}
	insert := func(ins *sql.Stmt, ids []int64, cell int) error {
		for _, id := range ids {
			if _, err := ins.Exec(id, cell); err != nil {
				return err
			}
		}
		return nil
	}

	covered := map[[2]int64]bool{}
	if len(areas)+len(territories) > 0 {
		for c := range g.cellCount() {
			lat, lon := latLon(g.center(c))
			x, y := p.forward(lon, lat)
			col := eq.Center + x*radius/eq.HexKm
			row := equatorRow(col) - y*radius/rowKm
			q, r := axialRound(col-row/2, row)
			key := [2]int64{int64(q), int64(r)}
			if len(areas[key])+len(territories[key]) == 0 {
				continue
			}
			covered[key] = true
			if err := insert(insArea, areas[key], c); err != nil {
				return err
			}
			if err := insert(insTerr, territories[key], c); err != nil {
				return err
			}
		}
	}
	// Hexes smaller than the cells can fall between cell centres; each still
	// gets the cell under its own centre, so nothing small is lost.
	for _, set := range []struct {
		hexes map[[2]int64][]int64
		ins   *sql.Stmt
	}{{areas, insArea}, {territories, insTerr}} {
		for hex, ids := range set.hexes {
			if covered[hex] {
				continue
			}
			if cell, ok := cellUnder(hex); ok {
				if err := insert(set.ins, ids, cell); err != nil {
					return err
				}
			}
		}
	}
	for hex, ids := range places {
		cell, ok := cellUnder(hex)
		if !ok {
			continue
		}
		for _, id := range ids {
			if _, err := tx.Exec(`UPDATE locations SET cell = ? WHERE id = ?`, cell, id); err != nil {
				return err
			}
		}
	}
	return nil
}

func portPlanetHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Projection string `json:"projection"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || !validProjection(in.Projection) {
			http.Error(w, "pick a projection", http.StatusBadRequest)
			return
		}
		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to port the map", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()
		if err := portToPlanet(tx, loadEquator(db), loadMapSettings(db).Planet, in.Projection); err != nil {
			log.Printf("port to planet: %v", err)
			http.Error(w, "failed to port the map", http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to port the map", http.StatusInternalServerError)
			return
		}
		var n int
		db.QueryRow(`SELECT COUNT(*) FROM planet_cells`).Scan(&n)
		writeJSON(w, map[string]any{"cells": n})
	}
}
