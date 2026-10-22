package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// The planet's layers: tectonic plates, continents and ocean depths, drawn
// on the planet's cells like major locations, each named and coloured. A
// cell belongs to at most one feature per layer. Only depths carry a number
// (metres). The wind and ocean currents layers are for a later system and
// have no features.
//
// Next to the layers, every cell can carry a biome (cell_biomes), with the
// height of its ground when it was painted with one. Biomes are independent
// of the feature layers.
//
// Only the planet (3D) has layers; the 2D map doesn't.

var schema34Statements = []string{
	`CREATE TABLE map_features (
		id    INTEGER PRIMARY KEY AUTOINCREMENT,
		layer TEXT NOT NULL CHECK (layer IN ('plates', 'mountains', 'depth')),
		name  TEXT NOT NULL,
		color TEXT NOT NULL,
		value REAL
	)`,
	`CREATE TABLE feature_cells (
		layer      TEXT NOT NULL,
		cell       INTEGER NOT NULL,
		feature_id INTEGER NOT NULL REFERENCES map_features(id) ON DELETE CASCADE,
		PRIMARY KEY (layer, cell)
	)`,
	`CREATE INDEX idx_feature_cells_feature ON feature_cells(feature_id)`,
	wikiTriggerSQL("map_feature"),
}

// schema38Statements upgrade a database to schema 38: the biomes. The fresh
// schema runs them too. (widenFeatureLayers swaps the mountain ranges layer
// for the continents one; moveMountainRangesStatements turn the old ranges
// into Mountains hexes. Only the migration needs those.)
var schema38Statements = []string{
	`CREATE TABLE cell_biomes (
		cell   INTEGER PRIMARY KEY,
		biome  TEXT NOT NULL,
		height REAL
	)`,
}

// moveMountainRangesStatements turn the mountain ranges into Mountains hexes
// (with the range's height), then delete the ranges; the wiki trigger takes
// their articles with them.
var moveMountainRangesStatements = []string{
	`INSERT OR IGNORE INTO cell_biomes (cell, biome, height)
		SELECT fc.cell, 'mountains', f.value FROM feature_cells fc
		JOIN map_features f ON f.id = fc.feature_id
		WHERE fc.layer = 'mountains' ORDER BY fc.cell`,
	`DELETE FROM feature_cells WHERE layer = 'mountains'`,
	`DELETE FROM map_features WHERE layer = 'mountains'`,
}

const (
	featureLayersOld = `CHECK (layer IN ('plates', 'mountains', 'depth'))`
	featureLayersNew = `CHECK (layer IN ('plates', 'continents', 'depth'))`
)

// widenFeatureLayers lets map_features.layer hold 'continents' instead of
// 'mountains' by rewriting its CHECK in the table's stored definition, the
// way widenRealms does for the locations. Runs on one connection (a
// transaction); a database already rewritten is left.
func widenFeatureLayers(tx *sql.Tx) error {
	return rewriteFeatureLayersCheck(tx, featureLayersOld, featureLayersNew)
}

func rewriteFeatureLayersCheck(tx *sql.Tx, from, to string) error {
	var def string
	if err := tx.QueryRow(`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'map_features'`).Scan(&def); err != nil {
		return err
	}
	if !strings.Contains(def, from) {
		return nil
	}
	var version int
	if err := tx.QueryRow(`PRAGMA schema_version`).Scan(&version); err != nil {
		return err
	}
	for _, q := range []string{
		`PRAGMA writable_schema = ON`,
		`UPDATE sqlite_master SET sql = ` + quoteSQL(strings.Replace(def, from, to, 1)) + ` WHERE type = 'table' AND name = 'map_features'`,
		fmt.Sprintf(`PRAGMA schema_version = %d`, version+1),
		`PRAGMA writable_schema = OFF`,
	} {
		if _, err := tx.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

var mapLayers = map[string]bool{"plates": true, "continents": true, "depth": true}

// The words the wiki shows for each layer's features.
var layerWord = map[string]string{"plates": "Tectonic plate", "continents": "Continent", "depth": "Ocean depth"}

// The biomes a cell can have: the same keys as frontend/src/biomes.js.
var biomeKeys = map[string]bool{
	"rainforest": true, "dry_forest": true, "temperate_rainforest": true, "temperate_forest": true,
	"boreal_forest": true, "mangrove": true, "savanna": true, "grassland": true, "steppe": true,
	"shrubland": true, "farmland": true, "desert": true, "cold_desert": true, "salt_flat": true,
	"barren": true, "tundra": true, "ice": true, "wetland": true, "lake": true, "mountains": true,
	"hills": true, "plateau": true, "volcanic": true,
}

// The highest or deepest ground a hex can be painted with, in metres.
const maxBiomeHeight = 20000

const (
	maxFeatureName = 80
	maxFeatures    = 1000
)

type mapFeature struct {
	ID    int64    `json:"id"`
	Layer string   `json:"layer"`
	Name  string   `json:"name"`
	Color string   `json:"color"`
	Value *float64 `json:"value"`
}

type layersPayload struct {
	Features []mapFeature `json:"features"`
	// Every painted cell, as [feature id, cell].
	Cells [][2]int64 `json:"cells"`
	// Every cell with a biome, as [cell, biome, height or null].
	Biomes [][3]any `json:"biomes"`
}

func loadLayers(db *sql.DB) (layersPayload, error) {
	out := layersPayload{Features: []mapFeature{}, Cells: [][2]int64{}, Biomes: [][3]any{}}
	rows, err := db.Query(`SELECT id, layer, name, color, value FROM map_features ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var f mapFeature
		var v sql.NullFloat64
		if err := rows.Scan(&f.ID, &f.Layer, &f.Name, &f.Color, &v); err != nil {
			rows.Close()
			return out, err
		}
		if v.Valid {
			f.Value = &v.Float64
		}
		out.Features = append(out.Features, f)
	}
	rows.Close()
	cells, err := db.Query(`SELECT feature_id, cell FROM feature_cells ORDER BY cell`)
	if err != nil {
		return out, err
	}
	defer cells.Close()
	for cells.Next() {
		var c [2]int64
		if err := cells.Scan(&c[0], &c[1]); err != nil {
			return out, err
		}
		out.Cells = append(out.Cells, c)
	}
	if err := cells.Err(); err != nil {
		return out, err
	}
	biomes, err := db.Query(`SELECT cell, biome, height FROM cell_biomes ORDER BY cell`)
	if err != nil {
		return out, err
	}
	defer biomes.Close()
	for biomes.Next() {
		var cell int64
		var biome string
		var h sql.NullFloat64
		if err := biomes.Scan(&cell, &biome, &h); err != nil {
			return out, err
		}
		var height any
		if h.Valid {
			height = h.Float64
		}
		out.Biomes = append(out.Biomes, [3]any{cell, biome, height})
	}
	return out, biomes.Err()
}

func getLayersHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l, err := loadLayers(db)
		if err != nil {
			log.Printf("load layers: %v", err)
			http.Error(w, "failed to load the layers", http.StatusInternalServerError)
			return
		}
		writeJSON(w, l)
	}
}

// saveFeatureHandler creates (POST) or updates (PUT /{id}) a feature and
// answers with it.
func saveFeatureHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var f mapFeature
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<14)).Decode(&f); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		var id int64
		if raw := r.PathValue("id"); raw != "" {
			var err error
			id, err = strconv.ParseInt(raw, 10, 64)
			if err != nil || db.QueryRow(`SELECT layer FROM map_features WHERE id = ?`, id).Scan(&f.Layer) != nil {
				http.Error(w, "feature not found", http.StatusNotFound)
				return
			}
		} else if !mapLayers[f.Layer] {
			http.Error(w, "unknown layer", http.StatusBadRequest)
			return
		}
		f.Name = strings.TrimSpace(f.Name)
		switch {
		case f.Name == "":
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		case len([]rune(f.Name)) > maxFeatureName:
			http.Error(w, "that name is too long", http.StatusBadRequest)
			return
		}
		color, ok := normalizeColor(f.Color)
		if !ok || color == "" {
			http.Error(w, "invalid color", http.StatusBadRequest)
			return
		}
		f.Color = color
		// Only the depths carry a number.
		if f.Layer != "depth" || (f.Value != nil && (math.IsNaN(*f.Value) || math.Abs(*f.Value) > 1e6)) {
			f.Value = nil
		}
		var err error
		if id == 0 {
			var n int
			db.QueryRow(`SELECT COUNT(*) FROM map_features`).Scan(&n)
			if n >= maxFeatures {
				http.Error(w, "too many features", http.StatusBadRequest)
				return
			}
			var res sql.Result
			res, err = db.Exec(`INSERT INTO map_features (layer, name, color, value) VALUES (?, ?, ?, ?)`, f.Layer, f.Name, f.Color, f.Value)
			if err == nil {
				f.ID, _ = res.LastInsertId()
			}
		} else {
			f.ID = id
			_, err = db.Exec(`UPDATE map_features SET name = ?, color = ?, value = ? WHERE id = ?`, f.Name, f.Color, f.Value, id)
		}
		if err != nil {
			log.Printf("save feature: %v", err)
			http.Error(w, "failed to save the feature", http.StatusInternalServerError)
			return
		}
		writeJSON(w, f)
	}
}

func deleteFeatureHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "feature not found", http.StatusNotFound)
			return
		}
		res, err := db.Exec(`DELETE FROM map_features WHERE id = ?`, id)
		if err != nil {
			http.Error(w, "failed to delete the feature", http.StatusInternalServerError)
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			http.Error(w, "feature not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// paintFeatureHandler paints cells with a feature (taking them from any
// other feature of its layer); with erase, it clears a layer's cells (only
// the given feature's, when one is given).
func paintFeatureHandler(db *sql.DB, erase bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			FeatureID *int64  `json:"feature_id"`
			Layer     string  `json:"layer"`
			Cells     []int64 `json:"cells"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<22)).Decode(&in); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if !checkCells(w, db, in.Cells) {
			return
		}
		layer := in.Layer
		if in.FeatureID != nil {
			if db.QueryRow(`SELECT layer FROM map_features WHERE id = ?`, *in.FeatureID).Scan(&layer) != nil {
				http.Error(w, "feature not found", http.StatusNotFound)
				return
			}
		} else if !erase {
			http.Error(w, "feature_id is required", http.StatusBadRequest)
			return
		}
		if !mapLayers[layer] {
			http.Error(w, "unknown layer", http.StatusBadRequest)
			return
		}
		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to paint", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()
		for _, c := range in.Cells {
			switch {
			case !erase:
				_, err = tx.Exec(`INSERT INTO feature_cells (layer, cell, feature_id) VALUES (?, ?, ?)
					ON CONFLICT(layer, cell) DO UPDATE SET feature_id = excluded.feature_id`, layer, c, *in.FeatureID)
			case in.FeatureID != nil:
				_, err = tx.Exec(`DELETE FROM feature_cells WHERE layer = ? AND cell = ? AND feature_id = ?`, layer, c, *in.FeatureID)
			default:
				_, err = tx.Exec(`DELETE FROM feature_cells WHERE layer = ? AND cell = ?`, layer, c)
			}
			if err != nil {
				log.Printf("paint feature: %v", err)
				http.Error(w, "failed to paint", http.StatusInternalServerError)
				return
			}
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to paint", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]int{"changed": len(in.Cells)})
	}
}

// checkCells answers 400 (and reports false) unless there aren't too many
// cells and every one is on the planet.
func checkCells(w http.ResponseWriter, db *sql.DB, cells []int64) bool {
	if len(cells) > maxHexBatch {
		http.Error(w, "too many hexes in one request", http.StatusBadRequest)
		return false
	}
	count := int64(loadMapSettings(db).Cells)
	for _, c := range cells {
		if c < 0 || c >= count {
			http.Error(w, "that hex isn't on the planet", http.StatusBadRequest)
			return false
		}
	}
	return true
}

// paintBiomesHandler sets the biome (and the ground's height, when one is
// given) of cells; with erase, it clears their biomes.
func paintBiomesHandler(db *sql.DB, erase bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Biome  string   `json:"biome"`
			Height *float64 `json:"height"`
			Cells  []int64  `json:"cells"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<22)).Decode(&in); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if !erase {
			if !biomeKeys[in.Biome] {
				http.Error(w, "unknown biome", http.StatusBadRequest)
				return
			}
			if in.Height != nil && (math.IsNaN(*in.Height) || math.Abs(*in.Height) > maxBiomeHeight) {
				http.Error(w, "that height is out of range", http.StatusBadRequest)
				return
			}
		}
		if !checkCells(w, db, in.Cells) {
			return
		}
		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to paint", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()
		for _, c := range in.Cells {
			if erase {
				_, err = tx.Exec(`DELETE FROM cell_biomes WHERE cell = ?`, c)
			} else {
				_, err = tx.Exec(`INSERT INTO cell_biomes (cell, biome, height) VALUES (?, ?, ?)
					ON CONFLICT(cell) DO UPDATE SET biome = excluded.biome, height = excluded.height`, c, in.Biome, in.Height)
			}
			if err != nil {
				log.Printf("paint biomes: %v", err)
				http.Error(w, "failed to paint", http.StatusInternalServerError)
				return
			}
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to paint", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]int{"changed": len(in.Cells)})
	}
}

// resampleLayers moves the layers (and the biomes) onto a new grid, as
// resamplePlanet does for everything else.
func resampleLayers(tx *sql.Tx, a, b planetGrid) error {
	if err := resampleBiomes(tx, a, b); err != nil {
		return err
	}
	type row struct {
		layer   string
		feature int64
	}
	rows, err := tx.Query(`SELECT layer, cell, feature_id FROM feature_cells`)
	if err != nil {
		return err
	}
	old := map[int64][]row{}
	for rows.Next() {
		var r row
		var cell int64
		if err := rows.Scan(&r.layer, &cell, &r.feature); err != nil {
			rows.Close()
			return err
		}
		old[cell] = append(old[cell], r)
	}
	rows.Close()
	if _, err := tx.Exec(`DELETE FROM feature_cells`); err != nil {
		return err
	}
	if len(old) == 0 {
		return nil
	}
	ins, err := tx.Prepare(`INSERT OR IGNORE INTO feature_cells (layer, cell, feature_id) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer ins.Close()
	for c := range b.cellCount() {
		for _, r := range old[int64(a.cellAt(b.center(c)))] {
			if _, err := ins.Exec(r.layer, c, r.feature); err != nil {
				return err
			}
		}
	}
	return nil
}

// resampleBiomes gives every new cell the biome (and height) of the old cell
// under its centre.
func resampleBiomes(tx *sql.Tx, a, b planetGrid) error {
	type biome struct {
		key    string
		height sql.NullFloat64
	}
	rows, err := tx.Query(`SELECT cell, biome, height FROM cell_biomes`)
	if err != nil {
		return err
	}
	old := map[int64]biome{}
	for rows.Next() {
		var cell int64
		var bm biome
		if err := rows.Scan(&cell, &bm.key, &bm.height); err != nil {
			rows.Close()
			return err
		}
		old[cell] = bm
	}
	rows.Close()
	if _, err := tx.Exec(`DELETE FROM cell_biomes`); err != nil {
		return err
	}
	if len(old) == 0 {
		return nil
	}
	ins, err := tx.Prepare(`INSERT OR IGNORE INTO cell_biomes (cell, biome, height) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer ins.Close()
	for c := range b.cellCount() {
		if bm, ok := old[int64(a.cellAt(b.center(c)))]; ok {
			if _, err := ins.Exec(c, bm.key, bm.height); err != nil {
				return err
			}
		}
	}
	return nil
}

// ---- The wiki -------------------------------------------------------------------------

// A feature's article: its layer, a continent's area, a depth's number, the
// lands on it and its biomes.
func featureFacts(db *sql.DB, a *wikiArticle) {
	var layer string
	var value sql.NullFloat64
	if db.QueryRow(`SELECT layer, value FROM map_features WHERE id = ?`, a.ID).Scan(&layer, &value) != nil {
		return
	}
	a.addFact("type", layerWord[layer], "word", nil)
	if layer == "depth" && value.Valid {
		a.addFact("depth", strconv.FormatFloat(math.Round(value.Float64), 'f', 0, 64)+" m", "", nil)
	}
	cellsOf := `SELECT cell FROM feature_cells WHERE feature_id = ?1`
	if layer == "continents" {
		var n int
		db.QueryRow(`SELECT COUNT(*) FROM feature_cells WHERE feature_id = ?`, a.ID).Scan(&n)
		if n > 0 {
			a.addFact("area", "≈ "+formatKm2(float64(n)*hexAreaKm2(loadMapSettings(db)))+" km²", "", nil)
		}
	}
	a.addGroup("lands_on", queryRefs(db, "location", `SELECT l.id, l.name FROM locations l
		WHERE l.kind = 'major' AND l.color IS NOT NULL AND l.realm IN ('kingdom', 'region')
		AND EXISTS (SELECT 1 FROM planet_cells pc JOIN feature_cells fc ON fc.cell = pc.cell
			WHERE pc.location_id = l.id AND fc.feature_id = ?)
		ORDER BY l.name COLLATE NOCASE`, a.ID))
	if layer == "plates" {
		a.addGroup("continents_on", queryRefs(db, "map_feature", `SELECT DISTINCT f.id, f.name FROM map_features f
			JOIN feature_cells m ON m.feature_id = f.id
			JOIN feature_cells p ON p.cell = m.cell AND p.layer = 'plates'
			WHERE f.layer = 'continents' AND p.feature_id = ? ORDER BY f.name COLLATE NOCASE`, a.ID))
	}
	if layer == "continents" {
		addBiomesFact(db, a, cellsOf, a.ID)
	}
}

// What a place stands on, by its hexes (planet only): the plate under most
// of it, any other plates it reaches, its continent(s) and its biomes.
func locationLayerFacts(db *sql.DB, a *wikiArticle, kind string) {
	if mapMode(db) != "3d" {
		return
	}
	cellsOf := `SELECT cell FROM planet_cells WHERE location_id = ?1`
	if kind == "minor" {
		cellsOf = `SELECT cell FROM locations WHERE id = ?1 AND cell IS NOT NULL`
	}
	plates := queryRefs(db, "map_feature", `SELECT f.id, f.name FROM feature_cells fc JOIN map_features f ON f.id = fc.feature_id
		WHERE fc.layer = 'plates' AND fc.cell IN (`+cellsOf+`)
		GROUP BY f.id ORDER BY COUNT(*) DESC, f.name COLLATE NOCASE`, a.ID)
	if len(plates) > 0 {
		a.addFact("plate", "", "", &plates[0])
	}
	if len(plates) > 1 {
		a.addGroup("plates", plates)
	}
	continents := queryRefs(db, "map_feature", `SELECT f.id, f.name FROM feature_cells fc JOIN map_features f ON f.id = fc.feature_id
		WHERE fc.layer = 'continents' AND fc.cell IN (`+cellsOf+`)
		GROUP BY f.id ORDER BY COUNT(*) DESC, f.name COLLATE NOCASE`, a.ID)
	if len(continents) > 0 {
		a.addFact("continent", "", "", &continents[0])
	}
	if len(continents) > 1 {
		a.addGroup("continents", continents)
	}
	addBiomesFact(db, a, cellsOf, a.ID)
}

// addBiomesFact adds the "biomes" fact: each biome's share of the cells that
// cellsOf selects (its one argument is ?1), in whole percent, largest first,
// at most 6, those under 1% left out. Written as "desert:40,savanna:25"; the
// frontend turns each key into its name. Nothing is added when no cell has a
// biome.
func addBiomesFact(db *sql.DB, a *wikiArticle, cellsOf string, arg any) {
	var total int
	db.QueryRow(`SELECT COUNT(*) FROM (`+cellsOf+`)`, arg).Scan(&total)
	if total == 0 {
		return
	}
	rows, err := db.Query(`SELECT biome, COUNT(*) FROM cell_biomes WHERE cell IN (`+cellsOf+`) GROUP BY biome`, arg)
	if err != nil {
		log.Printf("wiki biomes: %v", err)
		return
	}
	defer rows.Close()
	type share struct {
		key string
		n   int
	}
	var shares []share
	for rows.Next() {
		var s share
		if rows.Scan(&s.key, &s.n) == nil && biomeKeys[s.key] {
			shares = append(shares, s)
		}
	}
	sort.Slice(shares, func(i, j int) bool {
		if shares[i].n != shares[j].n {
			return shares[i].n > shares[j].n
		}
		return shares[i].key < shares[j].key
	})
	var parts []string
	for _, s := range shares {
		pct := 100 * float64(s.n) / float64(total)
		if pct < 1 || len(parts) == 6 {
			continue
		}
		parts = append(parts, s.key+":"+strconv.Itoa(int(math.Round(pct))))
	}
	a.addFact("biomes", strings.Join(parts, ","), "biomes", nil)
}

// hexAreaKm2 is the area of one hex: the planet's surface (a sphere with the
// circumference planet_km) shared among its cells.
func hexAreaKm2(m mapSettings) float64 {
	if m.Cells <= 0 {
		return 0
	}
	r := m.Planet.PlanetKm / (2 * math.Pi)
	return 4 * math.Pi * r * r / float64(m.Cells)
}

// formatKm2 writes an area with three significant figures and commas
// between the thousands: 12345678 becomes "12,300,000".
func formatKm2(v float64) string {
	if v <= 0 {
		return "0"
	}
	scale := math.Pow(10, math.Floor(math.Log10(v))-2)
	v = math.Round(v/scale) * scale
	if v < 1000 {
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	digits := strconv.FormatFloat(v, 'f', 0, 64)
	var b strings.Builder
	for i, c := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return b.String()
}
