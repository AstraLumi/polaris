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

// The solar system's name, and a map for every planet and moon but the main
// one (whose map is the story's map, planetmap.go). Those maps are painting
// only: cells in colours, no places or borders. Each keeps the grid it was
// first painted on (map_frequency), so resizing the body later doesn't move
// what was painted.

// schema32Statements create what schema 32 added. The fresh schema runs them
// too, after schema 30's table exists; the two wiki kinds get their delete
// triggers here (wikiTypeDef.OwnTrigger).
var schema32Statements = []string{
	`CREATE TABLE solar_systems (
		id   INTEGER PRIMARY KEY,
		name TEXT NOT NULL
	)`,
	`ALTER TABLE celestial_bodies ADD COLUMN map_frequency INTEGER`,
	`CREATE TABLE body_cells (
		body_id INTEGER NOT NULL REFERENCES celestial_bodies(id) ON DELETE CASCADE,
		cell    INTEGER NOT NULL,
		color   TEXT NOT NULL,
		PRIMARY KEY (body_id, cell)
	)`,
	wikiTriggerSQL("celestial_body"),
	wikiTriggerSQL("solar_system"),
}

const maxSystemName = 80

// ---- The system's name -----------------------------------------------------------

// The one solar system is row 1; it exists once it has a name.
func systemName(db *sql.DB) string {
	var name string
	db.QueryRow(`SELECT name FROM solar_systems WHERE id = 1`).Scan(&name)
	return name
}

func updateSystemHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&in); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		name := strings.TrimSpace(in.Name)
		if len([]rune(name)) > maxSystemName {
			http.Error(w, "that name is too long", http.StatusBadRequest)
			return
		}
		var err error
		if name == "" {
			// Unnamed again: the system's wiki article goes with it.
			_, err = db.Exec(`DELETE FROM solar_systems WHERE id = 1`)
		} else {
			_, err = db.Exec(`INSERT INTO solar_systems (id, name) VALUES (1, ?)
				ON CONFLICT(id) DO UPDATE SET name = excluded.name`, name)
		}
		if err != nil {
			log.Printf("save system name: %v", err)
			http.Error(w, "failed to save the name", http.StatusInternalServerError)
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

// ---- The shell round the system ------------------------------------------------------

// The whole system can sit inside a shell whose inside is the sky picture:
// it lights everything evenly on the main planet's day and night (one
// global cycle, no time zones), and by night gives only the light of its
// painted stars. frontend/src/lighting.js works out what that lights; this
// only keeps whether it is there.
const settingEnclosed = "solar_enclosed"

func systemEnclosed(db *sql.DB) bool {
	var raw string
	return db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, settingEnclosed).Scan(&raw) == nil && raw == "1"
}

// updateEnclosureHandler turns the shell on or off and answers with the
// solar system.
func updateEnclosureHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Enclosed *bool `json:"enclosed"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&in); err != nil || in.Enclosed == nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		value := "0"
		if *in.Enclosed {
			value = "1"
		}
		if _, err := db.Exec(`INSERT INTO app_settings (key, value) VALUES (?, ?)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value`, settingEnclosed, value); err != nil {
			log.Printf("save enclosure: %v", err)
			http.Error(w, "failed to save the shell", http.StatusInternalServerError)
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

// ---- Body maps ----------------------------------------------------------------------

// bodyGrid is the grid a body's map is on: the one it was painted on, else
// one with the story's hex size (Settings → Map) round the body's size.
func bodyGrid(db *sql.DB, id int64) (planetGrid, bool, error) {
	var kind string
	var radius float64
	var isMain bool
	var freq sql.NullInt64
	if err := db.QueryRow(`SELECT kind, radius_km, is_main, map_frequency FROM celestial_bodies WHERE id = ?`, id).
		Scan(&kind, &radius, &isMain, &freq); err != nil {
		return planetGrid{}, false, err
	}
	if kind == "star" || isMain {
		return planetGrid{}, false, nil
	}
	if freq.Valid {
		return planetGrid{int(freq.Int64)}, true, nil
	}
	return planetGrid{frequencyFor(2*math.Pi*radius, loadMapSettings(db).HexKm)}, true, nil
}

type bodyMap struct {
	BodyID    int64    `json:"body_id"`
	Frequency int      `json:"frequency"`
	Cells     [][2]any `json:"cells"`  // [cell, '#rrggbb']
	Colors    []string `json:"colors"` // the colours used, most first
}

func loadBodyMap(db *sql.DB, id int64, g planetGrid) (bodyMap, error) {
	out := bodyMap{BodyID: id, Frequency: g.n, Cells: [][2]any{}, Colors: []string{}}
	rows, err := db.Query(`SELECT cell, color FROM body_cells WHERE body_id = ? ORDER BY cell`, id)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	count := map[string]int{}
	for rows.Next() {
		var cell int64
		var color string
		if err := rows.Scan(&cell, &color); err != nil {
			return out, err
		}
		out.Cells = append(out.Cells, [2]any{cell, color})
		count[color]++
	}
	for c := range count {
		out.Colors = append(out.Colors, c)
	}
	sortByCount(out.Colors, count)
	return out, rows.Err()
}

func sortByCount(list []string, count map[string]int) {
	for i := 1; i < len(list); i++ {
		for j := i; j > 0 && (count[list[j]] > count[list[j-1]] || count[list[j]] == count[list[j-1]] && list[j] < list[j-1]); j-- {
			list[j], list[j-1] = list[j-1], list[j]
		}
	}
}

// bodyMapRequest reads the body of a map request and its grid; ok is false
// once an error was written.
func bodyMapRequest(db *sql.DB, w http.ResponseWriter, r *http.Request) (int64, planetGrid, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "body not found", http.StatusNotFound)
		return 0, planetGrid{}, false
	}
	g, has, err := bodyGrid(db, id)
	if err != nil {
		http.Error(w, "body not found", http.StatusNotFound)
		return 0, planetGrid{}, false
	}
	if !has {
		http.Error(w, "this body has no map of its own", http.StatusBadRequest)
		return 0, planetGrid{}, false
	}
	return id, g, true
}

func getBodyMapHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, g, ok := bodyMapRequest(db, w, r)
		if !ok {
			return
		}
		m, err := loadBodyMap(db, id, g)
		if err != nil {
			log.Printf("load body map: %v", err)
			http.Error(w, "failed to load the map", http.StatusInternalServerError)
			return
		}
		writeJSON(w, m)
	}
}

// paintBodyHandler paints cells in a colour; erase (color "") clears them.
// The first stroke fixes the body's grid.
func paintBodyHandler(db *sql.DB, erase bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, g, ok := bodyMapRequest(db, w, r)
		if !ok {
			return
		}
		var in struct {
			Color string  `json:"color"`
			Cells []int64 `json:"cells"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<22)).Decode(&in); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if len(in.Cells) > maxHexBatch {
			http.Error(w, "too many hexes in one request", http.StatusBadRequest)
			return
		}
		count := int64(g.cellCount())
		for _, c := range in.Cells {
			if c < 0 || c >= count {
				http.Error(w, "that hex isn't on the planet", http.StatusBadRequest)
				return
			}
		}
		color, valid := normalizeColor(in.Color)
		if !erase && (!valid || color == "") {
			http.Error(w, "invalid color", http.StatusBadRequest)
			return
		}
		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to paint", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()
		if _, err := tx.Exec(`UPDATE celestial_bodies SET map_frequency = ? WHERE id = ? AND map_frequency IS NULL`, g.n, id); err != nil {
			http.Error(w, "failed to paint", http.StatusInternalServerError)
			return
		}
		for _, c := range in.Cells {
			if erase {
				_, err = tx.Exec(`DELETE FROM body_cells WHERE body_id = ? AND cell = ?`, id, c)
			} else {
				_, err = tx.Exec(`INSERT INTO body_cells (body_id, cell, color) VALUES (?, ?, ?)
					ON CONFLICT(body_id, cell) DO UPDATE SET color = excluded.color`, id, c, color)
			}
			if err != nil {
				log.Printf("paint body: %v", err)
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

// ---- The wiki -----------------------------------------------------------------------

var bodyKindWord = map[string]string{"star": "Star", "planet": "Planet", "moon": "Moon"}
var bodyTypeWord = map[string]string{"rocky": "Rocky", "gas": "Gas giant", "ocean": "Ocean world"}
var starClassWord = map[string]string{
	"yellow_dwarf": "Yellow dwarf (like the Sun)", "red_giant": "Red giant", "white_dwarf": "White dwarf",
	"brown_dwarf": "Brown dwarf", "neutron_star": "Neutron star", "black_hole": "Black hole",
}

// km as a whole number, and AU for long distances.
func kmText(km float64) string {
	s := strconv.FormatFloat(math.Round(km), 'f', 0, 64) + " km"
	if km >= 1e7 {
		s += " (" + strconv.FormatFloat(km/1.496e8, 'g', 3, 64) + " AU)"
	}
	return s
}

func celestialFacts(db *sql.DB, a *wikiArticle) {
	s, err := loadSolarSystem(db)
	if err != nil {
		return
	}
	byID := map[int64]celestialBody{}
	for _, b := range s.Bodies {
		byID[b.ID] = b
	}
	b, ok := byID[a.ID]
	if !ok {
		return
	}
	a.addFact("type", bodyKindWord[b.Kind], "word", nil)
	if b.Kind == "star" {
		a.addFact("star_class", starClassWord[b.StarClass], "word", nil)
		if b.TemperatureK != nil && *b.TemperatureK > 0 {
			a.addFact("temperature", strconv.FormatFloat(math.Round(*b.TemperatureK), 'f', 0, 64)+" K", "", nil)
		}
	} else {
		a.addFact("body_kind", bodyTypeWord[b.BodyType], "word", nil)
	}
	if b.IsMain {
		a.addFact("world", "The world of the map", "word", nil)
	}
	if b.Dyson {
		a.addFact("dyson", "Inside a Dyson sphere", "word", nil)
	}
	if b.RingOuter > 0 {
		a.addFact("rings", strconv.FormatFloat(b.RingInner, 'g', 3, 64)+"–"+strconv.FormatFloat(b.RingOuter, 'g', 3, 64)+" ×", "", nil)
	}
	if b.Kind != "star" {
		a.addFact("axial_tilt", strconv.FormatFloat(b.AxialTilt, 'g', 4, 64)+"°", "", nil)
		a.addFact("day_length", strconv.FormatFloat(b.DayHours, 'g', 4, 64)+" h", "", nil)
	}
	a.addFact("radius", kmText(b.RadiusKm), "", nil)
	if b.ParentID != nil {
		if p, ok := byID[*b.ParentID]; ok {
			a.addFact("orbits", "", "", &wikiRef{Type: "celestial_body", ID: p.ID, Name: p.Name})
		}
	}
	if b.OrbitKm != nil {
		a.addFact("orbit_radius", kmText(*b.OrbitKm), "", nil)
		if b.Eccentricity > 0 {
			a.addFact("eccentricity", strconv.FormatFloat(b.Eccentricity, 'g', 3, 64), "", nil)
			a.addFact("closest", kmText(*b.OrbitKm*(1-b.Eccentricity)), "", nil)
			a.addFact("farthest", kmText(*b.OrbitKm*(1+b.Eccentricity)), "", nil)
		}
	}
	if name := systemName(db); name != "" {
		a.addFact("system", "", "", &wikiRef{Type: "solar_system", ID: 1, Name: name})
	}
	orbiting := []wikiRef{}
	for _, c := range s.Bodies {
		if c.ParentID != nil && *c.ParentID == b.ID {
			orbiting = append(orbiting, wikiRef{Type: "celestial_body", ID: c.ID, Name: c.Name})
		}
	}
	a.addGroup("orbited_by", orbiting)
}

func solarSystemFacts(db *sql.DB, a *wikiArticle) {
	s, err := loadSolarSystem(db)
	if err != nil {
		return
	}
	var stars, planets, moons []wikiRef
	for _, b := range s.Bodies {
		ref := wikiRef{Type: "celestial_body", ID: b.ID, Name: b.Name}
		switch b.Kind {
		case "star":
			stars = append(stars, ref)
		case "planet":
			planets = append(planets, ref)
		default:
			moons = append(moons, ref)
		}
	}
	if len(stars) > 0 {
		a.addFact("star", "", "", &stars[0])
	}
	a.addFact("planet_count", countText(len(planets)), "", nil)
	a.addGroup("planets", planets)
	a.addGroup("moons", moons)
}

func countText(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}
