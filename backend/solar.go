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

// The solar system: a star, the planets round it and the moons round them.
// This file only keeps the bodies; their physics (mass from size and
// density, gravity, orbits) is worked out by the Solar system page
// (frontend/src/orbits.js), which is where it is drawn.
//
// One planet can be the main one: the planet of the map. Its size is the
// map's (Settings → Map), so the two never disagree.

// schema30Statements create what schema 30 added. The fresh schema runs them
// too.
var schema30Statements = []string{
	`CREATE TABLE celestial_bodies (
		id        INTEGER PRIMARY KEY AUTOINCREMENT,
		parent_id INTEGER REFERENCES celestial_bodies(id) ON DELETE CASCADE,
		kind      TEXT NOT NULL CHECK (kind IN ('star', 'planet', 'moon')),
		name      TEXT NOT NULL,
		body_type TEXT NOT NULL CHECK (body_type IN ('star', 'rocky', 'gas', 'ocean')),
		radius_km REAL NOT NULL,
		orbit_km  REAL,
		phase     REAL NOT NULL DEFAULT 0,
		color     TEXT,
		is_main   INTEGER NOT NULL DEFAULT 0,
		sort      INTEGER NOT NULL DEFAULT 0
	)`,
	`CREATE INDEX idx_celestial_parent ON celestial_bodies(parent_id)`,
}

// schema31Statements create what schema 31 added: a star's class (white
// dwarf, black hole…), its surface temperature and its own mass (a star's
// mass isn't its size times one density), and which of temperature, size
// and mass are typed freely instead of kept within what is real for the
// class (freeValues). Stars made before are Sun-like stars.
var schema31Statements = []string{
	`ALTER TABLE celestial_bodies ADD COLUMN star_class TEXT`,
	`ALTER TABLE celestial_bodies ADD COLUMN temperature_k REAL`,
	`ALTER TABLE celestial_bodies ADD COLUMN mass_kg REAL`,
	`ALTER TABLE celestial_bodies ADD COLUMN free_values INTEGER NOT NULL DEFAULT 0`,
	`UPDATE celestial_bodies SET star_class = 'yellow_dwarf', temperature_k = 5772,
		mass_kg = 1410 * 4.0 / 3.0 * 3.141592653589793 * (radius_km * 1000) * (radius_km * 1000) * (radius_km * 1000)
		WHERE kind = 'star'`,
}

// The classes a star can be. Their realistic ranges live with the physics
// (frontend/src/stars.js); here a star only has to be one of them.
var starClasses = map[string]bool{
	"yellow_dwarf": true, "red_giant": true, "white_dwarf": true,
	"brown_dwarf": true, "neutron_star": true, "black_hole": true,
}

// freeValues bits: set when the value is typed freely ("Reality
// Following" unticked).
const (
	freeTemperature = 1 << iota
	freeRadius
	freeMass
	freeAll = freeTemperature | freeRadius | freeMass
)

const (
	maxBodyName  = 80
	maxRadiusKm  = 1e8  // a thousand suns
	maxOrbitKm   = 1e13 // ~70 000 times Earth's distance from the Sun
	maxBodyCount = 500
)

type celestialBody struct {
	ID       int64    `json:"id"`
	ParentID *int64   `json:"parent_id"`
	Kind     string   `json:"kind"`
	Name     string   `json:"name"`
	BodyType string   `json:"body_type"`
	RadiusKm float64  `json:"radius_km"`
	OrbitKm  *float64 `json:"orbit_km"`
	Phase    float64  `json:"phase"`
	Color    string   `json:"color"`
	IsMain   bool     `json:"is_main"`
	Sort     int      `json:"sort"`
	// Stars only.
	StarClass    string   `json:"star_class"`
	TemperatureK *float64 `json:"temperature_k"`
	MassKg       *float64 `json:"mass_kg"`
	FreeValues   int      `json:"free_values"`
}

// solarSystem is what GET /api/solar returns. The main planet's radius is
// already the map's; main says so, with the map's axial tilt and whether the
// map is a planet at all (a 2D map has nothing to wrap round it).
type solarSystem struct {
	Bodies []celestialBody `json:"bodies"`
	Main   struct {
		RadiusKm float64 `json:"radius_km"`
		Tilt     float64 `json:"tilt"`
		Globe    bool    `json:"globe"`
	} `json:"main"`
}

func loadSolarSystem(db *sql.DB) (solarSystem, error) {
	var out solarSystem
	m := loadMapSettings(db)
	out.Main.RadiusKm = m.Planet.PlanetKm / (2 * math.Pi)
	out.Main.Tilt = m.Planet.Tilt
	out.Main.Globe = m.Mode == "3d"
	out.Bodies = []celestialBody{}
	rows, err := db.Query(`SELECT id, parent_id, kind, name, body_type, radius_km, orbit_km, phase, COALESCE(color, ''), is_main, sort,
		COALESCE(star_class, ''), temperature_k, mass_kg, free_values
		FROM celestial_bodies ORDER BY sort, id`)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var b celestialBody
		var parent sql.NullInt64
		var orbit, temp, mass sql.NullFloat64
		if err := rows.Scan(&b.ID, &parent, &b.Kind, &b.Name, &b.BodyType, &b.RadiusKm, &orbit, &b.Phase, &b.Color, &b.IsMain, &b.Sort,
			&b.StarClass, &temp, &mass, &b.FreeValues); err != nil {
			return out, err
		}
		if temp.Valid {
			b.TemperatureK = &temp.Float64
		}
		if mass.Valid {
			b.MassKg = &mass.Float64
		}
		if parent.Valid {
			b.ParentID = &parent.Int64
		}
		if orbit.Valid {
			b.OrbitKm = &orbit.Float64
		}
		if b.IsMain {
			b.RadiusKm = out.Main.RadiusKm
		}
		out.Bodies = append(out.Bodies, b)
	}
	return out, rows.Err()
}

func getSolarHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := loadSolarSystem(db)
		if err != nil {
			log.Printf("load solar system: %v", err)
			http.Error(w, "failed to load the solar system", http.StatusInternalServerError)
			return
		}
		writeJSON(w, s)
	}
}

// What each kind of body may be, and what it must orbit.
var (
	bodyTypesOf = map[string][]string{
		"star":   {"star"},
		"planet": {"rocky", "gas", "ocean"},
		"moon":   {"rocky", "gas", "ocean"},
	}
	parentKindOf = map[string]string{"planet": "star", "moon": "planet"}
)

// checkBody cleans a body before it is saved and says what is wrong with it,
// if anything. id is 0 for a new body.
func checkBody(db *sql.DB, b *celestialBody, id int64) string {
	b.Name = strings.TrimSpace(b.Name)
	switch {
	case b.Name == "":
		return "name is required"
	case len([]rune(b.Name)) > maxBodyName:
		return "that name is too long"
	}
	types, ok := bodyTypesOf[b.Kind]
	if !ok {
		return "unknown kind of body"
	}
	okType := false
	for _, t := range types {
		okType = okType || t == b.BodyType
	}
	if !okType {
		return "that kind of body can't be of that type"
	}
	if !(b.RadiusKm > 0 && b.RadiusKm <= maxRadiusKm) {
		return "the radius must be more than 0"
	}
	color, ok := normalizeColor(b.Color)
	if !ok {
		return "invalid color"
	}
	b.Color = color
	b.Phase = math.Mod(math.Mod(b.Phase, 360)+360, 360)
	if b.Kind != "planet" {
		b.IsMain = false
	}

	if b.Kind == "star" {
		b.ParentID, b.OrbitKm = nil, nil
		if b.StarClass == "" {
			b.StarClass = "yellow_dwarf"
		}
		if !starClasses[b.StarClass] {
			return "unknown kind of star"
		}
		if b.TemperatureK == nil || !(*b.TemperatureK >= 0 && *b.TemperatureK <= 1e9) {
			return "the temperature must be between 0 and 1 000 000 000 K"
		}
		if b.MassKg == nil || !(*b.MassKg > 0 && *b.MassKg <= 1e34) {
			return "the mass must be more than 0"
		}
		b.FreeValues &= freeAll
		var other int64
		if db.QueryRow(`SELECT id FROM celestial_bodies WHERE kind = 'star' AND id != ?`, id).Scan(&other) == nil {
			return "a solar system has one star"
		}
		return ""
	}
	// Only stars have these, for now; planets weigh what their size and
	// type say.
	b.StarClass, b.TemperatureK, b.MassKg, b.FreeValues = "", nil, nil, 0
	if b.ParentID == nil {
		return "pick what it orbits"
	}
	var parentKind string
	if db.QueryRow(`SELECT kind FROM celestial_bodies WHERE id = ?`, *b.ParentID).Scan(&parentKind) != nil || parentKind != parentKindOf[b.Kind] {
		if b.Kind == "planet" {
			return "a planet orbits the star"
		}
		return "a moon orbits a planet"
	}
	if b.OrbitKm == nil || !(*b.OrbitKm > 0 && *b.OrbitKm <= maxOrbitKm) {
		return "the orbit's distance must be more than 0"
	}
	return ""
}

func saveBodyHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var id int64
		if raw := r.PathValue("id"); raw != "" {
			var err error
			if id, err = strconv.ParseInt(raw, 10, 64); err != nil || !bodyExists(db, id) {
				http.Error(w, "body not found", http.StatusNotFound)
				return
			}
		}
		var b celestialBody
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&b); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if id != 0 {
			// What a body is and what it orbits stay as they were made.
			var kind string
			var parent sql.NullInt64
			db.QueryRow(`SELECT kind, parent_id FROM celestial_bodies WHERE id = ?`, id).Scan(&kind, &parent)
			b.Kind = kind
			b.ParentID = nil
			if parent.Valid {
				b.ParentID = &parent.Int64
			}
		} else {
			var n int
			db.QueryRow(`SELECT COUNT(*) FROM celestial_bodies`).Scan(&n)
			if n >= maxBodyCount {
				http.Error(w, "a solar system can have at most 500 bodies", http.StatusBadRequest)
				return
			}
		}
		if msg := checkBody(db, &b, id); msg != "" {
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		if err := writeBody(db, &b, id); err != nil {
			log.Printf("save body: %v", err)
			http.Error(w, "failed to save the body", http.StatusInternalServerError)
			return
		}
		s, err := loadSolarSystem(db)
		if err != nil {
			http.Error(w, "saved, but failed to reload the solar system", http.StatusInternalServerError)
			return
		}
		writeJSON(w, s)
	}
}

func bodyExists(db *sql.DB, id int64) bool {
	var x int
	return db.QueryRow(`SELECT 1 FROM celestial_bodies WHERE id = ?`, id).Scan(&x) == nil
}

// writeBody inserts (id 0) or updates a checked body. Only one planet is
// the main one: marking one unmarks the rest.
func writeBody(db *sql.DB, b *celestialBody, id int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if b.IsMain {
		if _, err := tx.Exec(`UPDATE celestial_bodies SET is_main = 0 WHERE id != ?`, id); err != nil {
			return err
		}
	}
	color := nullableString(b.Color)
	if id == 0 {
		res, err := tx.Exec(`INSERT INTO celestial_bodies (parent_id, kind, name, body_type, radius_km, orbit_km, phase, color, is_main,
				star_class, temperature_k, mass_kg, free_values, sort)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, (SELECT COALESCE(MAX(sort), 0) + 1 FROM celestial_bodies))`,
			b.ParentID, b.Kind, b.Name, b.BodyType, b.RadiusKm, b.OrbitKm, b.Phase, color, b.IsMain,
			nullableString(b.StarClass), b.TemperatureK, b.MassKg, b.FreeValues)
		if err != nil {
			return err
		}
		b.ID, _ = res.LastInsertId()
	} else if _, err := tx.Exec(`UPDATE celestial_bodies SET name = ?, body_type = ?, radius_km = ?, orbit_km = ?, phase = ?, color = ?, is_main = ?,
			star_class = ?, temperature_k = ?, mass_kg = ?, free_values = ?
		WHERE id = ?`, b.Name, b.BodyType, b.RadiusKm, b.OrbitKm, b.Phase, color, b.IsMain,
		nullableString(b.StarClass), b.TemperatureK, b.MassKg, b.FreeValues, id); err != nil {
		return err
	}
	return tx.Commit()
}

// Deleting a body takes what orbits it along (the foreign key cascades).
func deleteBodyHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || !bodyExists(db, id) {
			http.Error(w, "body not found", http.StatusNotFound)
			return
		}
		if _, err := db.Exec(`DELETE FROM celestial_bodies WHERE id = ?`, id); err != nil {
			log.Printf("delete body: %v", err)
			http.Error(w, "failed to delete the body", http.StatusInternalServerError)
			return
		}
		s, err := loadSolarSystem(db)
		if err != nil {
			http.Error(w, "deleted, but failed to reload the solar system", http.StatusInternalServerError)
			return
		}
		writeJSON(w, s)
	}
}
