package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
)

// The planet's layers: tectonic plates, mountain ranges and ocean depths,
// drawn on the planet's cells like major locations, each named and
// coloured. A cell belongs to at most one feature per layer. Mountain ranges
// and depths can carry a number (height or depth, in metres). The wind and
// ocean currents layers are for a later system and have no features.
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

var mapLayers = map[string]bool{"plates": true, "mountains": true, "depth": true}

// The words the wiki shows for each layer's features.
var layerWord = map[string]string{"plates": "Tectonic plate", "mountains": "Mountain range", "depth": "Ocean depth"}

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
}

func loadLayers(db *sql.DB) (layersPayload, error) {
	out := layersPayload{Features: []mapFeature{}, Cells: [][2]int64{}}
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
	return out, cells.Err()
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
		if f.Layer == "plates" || (f.Value != nil && (math.IsNaN(*f.Value) || math.Abs(*f.Value) > 1e6)) {
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
		if len(in.Cells) > maxHexBatch {
			http.Error(w, "too many hexes in one request", http.StatusBadRequest)
			return
		}
		count := int64(loadMapSettings(db).Cells)
		for _, c := range in.Cells {
			if c < 0 || c >= count {
				http.Error(w, "that hex isn't on the planet", http.StatusBadRequest)
				return
			}
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

// resampleLayers moves the layers onto a new grid, as resamplePlanet does
// for everything else.
func resampleLayers(tx *sql.Tx, a, b planetGrid) error {
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

// ---- The wiki -------------------------------------------------------------------------

// A feature's article: its layer, its height or depth, and the lands on it.
func featureFacts(db *sql.DB, a *wikiArticle) {
	var layer string
	var value sql.NullFloat64
	if db.QueryRow(`SELECT layer, value FROM map_features WHERE id = ?`, a.ID).Scan(&layer, &value) != nil {
		return
	}
	a.addFact("type", layerWord[layer], "word", nil)
	if value.Valid {
		m := strconv.FormatFloat(math.Round(value.Float64), 'f', 0, 64) + " m"
		if layer == "mountains" {
			a.addFact("height", m, "", nil)
		} else {
			a.addFact("depth", m, "", nil)
		}
	}
	a.addGroup("lands_on", queryRefs(db, "location", `SELECT l.id, l.name FROM locations l
		WHERE l.kind = 'major' AND l.color IS NOT NULL AND l.realm IN ('kingdom', 'region')
		AND EXISTS (SELECT 1 FROM planet_cells pc JOIN feature_cells fc ON fc.cell = pc.cell
			WHERE pc.location_id = l.id AND fc.feature_id = ?)
		ORDER BY l.name COLLATE NOCASE`, a.ID))
	if layer == "plates" {
		a.addGroup("ranges_on", queryRefs(db, "map_feature", `SELECT DISTINCT f.id, f.name FROM map_features f
			JOIN feature_cells m ON m.feature_id = f.id
			JOIN feature_cells p ON p.cell = m.cell AND p.layer = 'plates'
			WHERE f.layer = 'mountains' AND p.feature_id = ? ORDER BY f.name COLLATE NOCASE`, a.ID))
	}
}

// What a place stands on, by its hexes (planet only): the plate under most
// of it, any other plates it reaches, and the mountain ranges in it.
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
	a.addGroup("mountain_ranges", queryRefs(db, "map_feature", `SELECT f.id, f.name FROM feature_cells fc JOIN map_features f ON f.id = fc.feature_id
		WHERE fc.layer = 'mountains' AND fc.cell IN (`+cellsOf+`)
		GROUP BY f.id ORDER BY COUNT(*) DESC, f.name COLLATE NOCASE`, a.ID))
}
