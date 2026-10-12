package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"
)

// The map as part of a planet: an equator across the hexes, and from it every
// hex's latitude, hemisphere and climate zone. frontend/src/planet.js does
// the same sums for the map's lines; keep them in step.

const settingEquator = "map_equator"

// mapEquator is a line across the hex map. Row is the hex row (r) it runs
// along at column Center (fractions fall between rows); Bend curves it,
// in rows per 100 columns squared away from Center, for when the map is a
// projection of a globe rather than flat. The lines of latitude run
// alongside it, bent the same way.
//
// HexKm and PlanetKm fix the scale (how many rows from the equator to a
// pole); Tilt is the planet's axial tilt in degrees, which puts the tropics
// at that latitude and the polar circles at 90 minus it.
type mapEquator struct {
	Show      bool    `json:"show"`
	Row       float64 `json:"row"`
	Center    float64 `json:"center"`
	Bend      float64 `json:"bend"`
	ShowZones bool    `json:"show_zones"`
	HexKm     float64 `json:"hex_km"`    // a hex's width, side to side
	PlanetKm  float64 `json:"planet_km"` // the circumference at the equator
	Tilt      float64 `json:"tilt"`
	// Placed: the equator has been saved, so places have a latitude. Set
	// when loading; never stored.
	Placed bool `json:"placed"`
}

const maxEquatorCoord = 1e6

func defaultEquator() mapEquator {
	return mapEquator{HexKm: 50, PlanetKm: 40075, Tilt: 23.44}
}

func loadEquator(db *sql.DB) mapEquator {
	var raw string
	e := defaultEquator()
	if db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, settingEquator).Scan(&raw) == nil {
		json.Unmarshal([]byte(raw), &e)
		e.Placed = true
	}
	return e
}

// rowsToPole is how many hex rows lie between the equator and a pole. Rows
// are 3/4 of a hex's height apart, which is √3/2 of its width.
func (e mapEquator) rowsToPole() float64 {
	return e.PlanetKm / 4 / (e.HexKm * math.Sqrt(3) / 2)
}

// latitude of hex (q, r) in degrees, north positive.
func (e mapEquator) latitude(q, r int64) float64 {
	off := (float64(q) + float64(r)/2 - e.Center) / 100
	equatorRow := e.Row + e.Bend*off*off
	lat := (equatorRow - float64(r)) / e.rowsToPole() * 90
	return math.Max(-90, math.Min(90, lat))
}

// zone names the climate zone at a latitude: inside the tropics, beyond a
// polar circle, or between them.
func (e mapEquator) zone(lat float64) string {
	a := math.Abs(lat)
	switch {
	case a >= 90-e.Tilt:
		return "Polar"
	case a < e.Tilt:
		return "Tropical"
	}
	return "Temperate"
}

func formatLatitude(lat float64) string {
	v := math.Round(math.Abs(lat))
	switch {
	case v == 0:
		return "0°"
	case lat > 0:
		return fmt.Sprintf("%.0f°N", v)
	}
	return fmt.Sprintf("%.0f°S", v)
}

// addPlanetFacts gives a place on the map its hemisphere, latitude (a range
// for a region) and climate zones: on the planet in 3D, or from the 2D
// equator once it has been placed.
func (a *wikiArticle) addPlanetFacts(db *sql.DB, kind string, id int64) {
	var lats []float64
	var e mapEquator
	if mapMode(db) == "3d" {
		var p planetSettings
		lats, p = planetLatitudes(db, kind, id)
		e.Tilt = p.Tilt
	} else {
		e = loadEquator(db)
		if !e.Placed || !(e.rowsToPole() > 0) {
			return
		}
		lats = e.hexLatitudes(db, kind, id)
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	zones := map[string]bool{}
	for _, lat := range lats {
		lo, hi = math.Min(lo, lat), math.Max(hi, lat)
		zones[e.zone(lat)] = true
	}
	if math.IsInf(lo, 1) {
		return // not on the map
	}

	switch {
	case lo >= 0 && hi <= 0:
		a.addFact("hemisphere", "On the equator", "word", nil)
	case lo >= 0:
		a.addFact("hemisphere", "Northern hemisphere", "word", nil)
	case hi <= 0:
		a.addFact("hemisphere", "Southern hemisphere", "word", nil)
	default:
		a.addFact("hemisphere", "Both hemispheres", "word", nil)
	}
	if from, to := formatLatitude(lo), formatLatitude(hi); from == to {
		a.addFact("latitude", from, "", nil)
	} else {
		a.addFact("latitude", from+" – "+to, "", nil)
	}
	var names []string
	for _, z := range []string{"Tropical", "Temperate", "Polar"} {
		if zones[z] {
			names = append(names, z)
		}
	}
	a.addFact("climate_zone", strings.Join(names, "|"), "words", nil)
}

// hexLatitudes are the latitudes of a location's 2D hexes.
func (e mapEquator) hexLatitudes(db *sql.DB, kind string, id int64) []float64 {
	q := `SELECT q, r FROM location_hexes WHERE location_id = ?`
	if kind != "major" {
		q = `SELECT q, r FROM locations WHERE id = ? AND q IS NOT NULL AND r IS NOT NULL`
	}
	rows, err := db.Query(q, id)
	if err != nil {
		log.Printf("planet facts: %v", err)
		return nil
	}
	defer rows.Close()
	var out []float64
	for rows.Next() {
		var hq, hr int64
		if rows.Scan(&hq, &hr) == nil {
			out = append(out, e.latitude(hq, hr))
		}
	}
	return out
}

func updateEquatorHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		in := defaultEquator()
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		for _, v := range []float64{in.Row, in.Center} {
			if math.IsNaN(v) || math.Abs(v) > maxEquatorCoord {
				http.Error(w, "the equator is off the map", http.StatusBadRequest)
				return
			}
		}
		if math.IsNaN(in.Bend) || math.Abs(in.Bend) > 1000 {
			http.Error(w, "that bend is out of range", http.StatusBadRequest)
			return
		}
		if !(in.HexKm > 0 && in.HexKm <= 1e6 && in.PlanetKm > 0 && in.PlanetKm <= 1e9) {
			http.Error(w, "the hex size and the planet's size must be more than 0", http.StatusBadRequest)
			return
		}
		if !(in.Tilt >= 0 && in.Tilt <= 90) {
			http.Error(w, "the axial tilt is between 0 and 90 degrees", http.StatusBadRequest)
			return
		}
		in.Placed = false
		raw, _ := json.Marshal(in)
		if _, err := db.Exec(
			`INSERT INTO app_settings (key, value) VALUES (?, ?)
			 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
			settingEquator, string(raw),
		); err != nil {
			log.Printf("save equator: %v", err)
			http.Error(w, "failed to save the equator", http.StatusInternalServerError)
			return
		}
		writeJSON(w, loadEquator(db))
	}
}
