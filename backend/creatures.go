package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

// The bestiary: the monsters, animals and constructs of the world. Each has a
// level range, the region it is native to (any map location), a description,
// a picture, and other creatures it is related to. Relations go both ways and
// are stored once, smaller id first. Every creature is a wiki article
// (wikiTypes "creature").
//
// Its territory is the map hexes it lives on: by default the area of its
// region (every hex of a kingdom or region, the one hex of a town), or, once
// painted by hand (custom_territory), the hexes in creature_hexes. On the
// planet (planetmap.go) the same, in cells: creature_cells.

var creatureKinds = []string{"monster", "animal", "construct"}

// creatureKindWord is how a kind reads in the wiki infobox (translated there).
var creatureKindWord = map[string]string{"monster": "Monster", "animal": "Animal", "construct": "Construct"}

const creaturesTableSQL = `CREATE TABLE creatures (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	name         TEXT NOT NULL,
	kind         TEXT NOT NULL DEFAULT 'monster' CHECK (kind IN ('monster', 'animal', 'construct')),
	level_min    INTEGER,
	level_max    INTEGER,
	region_id    INTEGER REFERENCES locations(id) ON DELETE SET NULL,
	description  TEXT,
	picture_path TEXT,
	-- 1: the territory is the painted creature_hexes, not the region's area.
	custom_territory INTEGER NOT NULL DEFAULT 0,
	created_at   TEXT NOT NULL DEFAULT (datetime('now')),
	updated_at   TEXT NOT NULL DEFAULT (datetime('now'))
)`

const creatureHexesTableSQL = `CREATE TABLE creature_hexes (
	creature_id INTEGER NOT NULL REFERENCES creatures(id) ON DELETE CASCADE,
	q           INTEGER NOT NULL,
	r           INTEGER NOT NULL,
	PRIMARY KEY (creature_id, q, r)
)`

const creatureLinksTableSQL = `CREATE TABLE creature_links (
	a_id INTEGER NOT NULL REFERENCES creatures(id) ON DELETE CASCADE,
	b_id INTEGER NOT NULL REFERENCES creatures(id) ON DELETE CASCADE,
	PRIMARY KEY (a_id, b_id),
	CHECK (a_id < b_id)
)`

type creatureRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type creatureOut struct {
	ID          int64         `json:"id"`
	Name        string        `json:"name"`
	Kind        string        `json:"kind"`
	LevelMin    *int64        `json:"level_min"`
	LevelMax    *int64        `json:"level_max"`
	RegionID    *int64        `json:"region_id"`
	RegionName  string        `json:"region_name"`
	Description string        `json:"description"`
	PicturePath string        `json:"picture_path"`
	Related     []creatureRef `json:"related"`
	// Territory is the hexes it lives on ([q, r]); CustomTerritory says they
	// were painted rather than taken from the region.
	Territory       [][2]int64 `json:"territory"`
	CustomTerritory bool       `json:"custom_territory"`
	// Cells is the territory on the planet.
	Cells []int64 `json:"cells"`
}

const creatureSelect = `SELECT c.id, c.name, c.kind, c.level_min, c.level_max, c.region_id, COALESCE(l.name, ''),
	COALESCE(c.description, ''), COALESCE(c.picture_path, ''), c.custom_territory
	FROM creatures c LEFT JOIN locations l ON l.id = c.region_id`

func scanCreature(s rowScanner) (creatureOut, error) {
	var c creatureOut
	var lo, hi, region sql.NullInt64
	if err := s.Scan(&c.ID, &c.Name, &c.Kind, &lo, &hi, &region, &c.RegionName, &c.Description, &c.PicturePath, &c.CustomTerritory); err != nil {
		return c, err
	}
	if lo.Valid {
		c.LevelMin = &lo.Int64
	}
	if hi.Valid {
		c.LevelMax = &hi.Int64
	}
	if region.Valid {
		c.RegionID = &region.Int64
	}
	c.PicturePath = uploadURL(c.PicturePath)
	c.Related = []creatureRef{}
	c.Territory = [][2]int64{}
	c.Cells = []int64{}
	return c, nil
}

// cellsOf is a creature's territory on the planet: its painted cells, or
// else its region's.
func cellsOf(db *sql.DB, c creatureOut) []int64 {
	out := []int64{}
	var rows *sql.Rows
	var err error
	switch {
	case c.CustomTerritory:
		rows, err = db.Query(`SELECT cell FROM creature_cells WHERE creature_id = ? ORDER BY cell`, c.ID)
	case c.RegionID != nil:
		rows, err = db.Query(`SELECT cell FROM planet_cells WHERE location_id = ?1
			UNION SELECT cell FROM locations WHERE id = ?1 AND kind = 'minor' AND cell IS NOT NULL ORDER BY 1`, *c.RegionID)
	default:
		return out
	}
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var x int64
		if rows.Scan(&x) == nil {
			out = append(out, x)
		}
	}
	return out
}

// territoryOf is the hexes a creature lives on: its painted hexes, or else
// its region's area.
func territoryOf(db *sql.DB, c creatureOut) [][2]int64 {
	out := [][2]int64{}
	var rows *sql.Rows
	var err error
	switch {
	case c.CustomTerritory:
		rows, err = db.Query(`SELECT q, r FROM creature_hexes WHERE creature_id = ? ORDER BY r, q`, c.ID)
	case c.RegionID != nil:
		// A kingdom's or region's hexes, or the one hex a town stands on.
		rows, err = db.Query(`SELECT q, r FROM location_hexes WHERE location_id = ?1
			UNION SELECT q, r FROM locations WHERE id = ?1 AND kind = 'minor' AND q IS NOT NULL AND r IS NOT NULL
			ORDER BY 2, 1`, *c.RegionID)
	default:
		return out
	}
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var h [2]int64
		if rows.Scan(&h[0], &h[1]) == nil {
			out = append(out, h)
		}
	}
	return out
}

// creatureLinks lists every creature related to each creature, by id.
func creatureLinks(db *sql.DB) map[int64][]creatureRef {
	out := map[int64][]creatureRef{}
	rows, err := db.Query(`SELECT k.a_id, a.name, k.b_id, b.name FROM creature_links k
		JOIN creatures a ON a.id = k.a_id JOIN creatures b ON b.id = k.b_id
		ORDER BY a.name COLLATE NOCASE, b.name COLLATE NOCASE`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var a, b creatureRef
		if rows.Scan(&a.ID, &a.Name, &b.ID, &b.Name) == nil {
			out[a.ID] = append(out[a.ID], b)
			out[b.ID] = append(out[b.ID], a)
		}
	}
	return out
}

func listCreaturesHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(creatureSelect + ` ORDER BY c.name COLLATE NOCASE`)
		if err != nil {
			log.Printf("list creatures: %v", err)
			http.Error(w, "failed to load the bestiary", http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		links := creatureLinks(db)
		out := []creatureOut{}
		for rows.Next() {
			c, err := scanCreature(rows)
			if err != nil {
				continue
			}
			if rel := links[c.ID]; rel != nil {
				c.Related = rel
			}
			c.Territory = territoryOf(db, c)
			c.Cells = cellsOf(db, c)
			out = append(out, c)
		}
		writeJSON(w, out)
	}
}

func loadCreature(db *sql.DB, id int64) (creatureOut, error) {
	c, err := scanCreature(db.QueryRow(creatureSelect+` WHERE c.id = ?`, id))
	if err != nil {
		return c, err
	}
	if rel := creatureLinks(db)[id]; rel != nil {
		c.Related = rel
	}
	c.Territory = territoryOf(db, c)
	c.Cells = cellsOf(db, c)
	return c, nil
}

func getCreatureHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
		c, err := loadCreature(db, id)
		if err != nil {
			http.Error(w, "creature not found", http.StatusNotFound)
			return
		}
		writeJSON(w, c)
	}
}

type creatureForm struct {
	Name, Kind, Description string
	LevelMin, LevelMax      sql.NullInt64
	Region                  sql.NullInt64
	Related                 []int64
	// Territory: nil leaves it as it is; Follow goes back to the region's
	// area; otherwise Hexes (2D) or Cells (the planet) are painted by hand.
	Territory *territoryForm
}

type territoryForm struct {
	Follow   bool
	Hexes    [][2]int64
	Cells    []int64
	OnPlanet bool
}

func parseLevel(raw string) (sql.NullInt64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return sql.NullInt64{}, true
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < 0 || n > 1_000_000 {
		return sql.NullInt64{}, false
	}
	return sql.NullInt64{Int64: n, Valid: true}, true
}

// parseCreatureForm reads and checks a posted creature; id is the one being
// edited (0 for a new one).
func parseCreatureForm(db *sql.DB, r *http.Request, id int64) (creatureForm, string) {
	var f creatureForm
	f.Name = strings.TrimSpace(r.FormValue("name"))
	f.Description = strings.TrimSpace(r.FormValue("description"))
	if f.Name == "" {
		return f, "name is required"
	}
	if len(f.Name) > 120 || len(f.Description) > 20000 {
		return f, "that text is too long"
	}
	var taken int
	db.QueryRow(`SELECT COUNT(*) FROM creatures WHERE name = ? COLLATE NOCASE AND id != ?`, f.Name, id).Scan(&taken)
	if taken > 0 {
		return f, "a creature with that name already exists"
	}
	f.Kind = strings.TrimSpace(r.FormValue("kind"))
	if _, ok := creatureKindWord[f.Kind]; !ok {
		return f, "choose monster, animal or construct"
	}
	var ok1, ok2 bool
	f.LevelMin, ok1 = parseLevel(r.FormValue("level_min"))
	f.LevelMax, ok2 = parseLevel(r.FormValue("level_max"))
	if !ok1 || !ok2 {
		return f, "levels must be whole numbers"
	}
	if f.LevelMin.Valid && f.LevelMax.Valid && f.LevelMin.Int64 > f.LevelMax.Int64 {
		f.LevelMin, f.LevelMax = f.LevelMax, f.LevelMin
	}
	f.Region = parseLocationID(db, r.FormValue("region_id"), false)
	if raw := r.FormValue("related"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &f.Related); err != nil || len(f.Related) > 500 {
			return f, "invalid list of related creatures"
		}
	}
	// territory: "region" to follow the region, or a JSON list of [q, r].
	if raw := strings.TrimSpace(r.FormValue("territory")); raw == "region" {
		f.Territory = &territoryForm{Follow: true}
	} else if raw != "" {
		var hexes [][2]int64
		if err := json.Unmarshal([]byte(raw), &hexes); err != nil || len(hexes) > 20000 {
			return f, "invalid territory"
		}
		f.Territory = &territoryForm{Hexes: hexes}
	}
	// territory_cells: a JSON list of planet cells.
	if raw := strings.TrimSpace(r.FormValue("territory_cells")); raw != "" && (f.Territory == nil || !f.Territory.Follow) {
		var cells []int64
		count := int64(loadMapSettings(db).Cells)
		if err := json.Unmarshal([]byte(raw), &cells); err != nil || len(cells) > maxHexBatch {
			return f, "invalid territory"
		}
		for _, c := range cells {
			if c < 0 || c >= count {
				return f, "invalid territory"
			}
		}
		f.Territory = &territoryForm{Cells: cells, OnPlanet: true}
	}
	return f, ""
}

// saveCreatureLinks replaces the creature's relations. Unknown creatures and
// the creature itself quietly drop out.
func saveCreatureLinks(tx *sql.Tx, id int64, related []int64) error {
	if _, err := tx.Exec(`DELETE FROM creature_links WHERE a_id = ? OR b_id = ?`, id, id); err != nil {
		return err
	}
	for _, other := range related {
		if other == id {
			continue
		}
		a, b := min(id, other), max(id, other)
		if _, err := tx.Exec(`INSERT OR IGNORE INTO creature_links (a_id, b_id)
			SELECT ?, ? WHERE EXISTS (SELECT 1 FROM creatures WHERE id = ?)`, a, b, other); err != nil {
			return err
		}
	}
	return nil
}

// saveTerritory replaces a creature's painted hexes (or, on the planet, its
// cells; the other map's are kept), or drops both so the territory follows
// its region again.
func saveTerritory(tx *sql.Tx, id int64, t territoryForm) error {
	if t.Follow || !t.OnPlanet {
		if _, err := tx.Exec(`DELETE FROM creature_hexes WHERE creature_id = ?`, id); err != nil {
			return err
		}
	}
	if t.Follow || t.OnPlanet {
		if _, err := tx.Exec(`DELETE FROM creature_cells WHERE creature_id = ?`, id); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`UPDATE creatures SET custom_territory = ? WHERE id = ?`, !t.Follow, id); err != nil {
		return err
	}
	if t.Follow {
		return nil
	}
	for _, c := range t.Cells {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO creature_cells (creature_id, cell) VALUES (?, ?)`, id, c); err != nil {
			return err
		}
	}
	for _, h := range t.Hexes {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO creature_hexes (creature_id, q, r) VALUES (?, ?, ?)`, id, h[0], h[1]); err != nil {
			return err
		}
	}
	return nil
}

func saveCreaturePicture(db *sql.DB, r *http.Request, uploadsDir string, id int64) {
	var old sql.NullString
	db.QueryRow(`SELECT picture_path FROM creatures WHERE id = ?`, id).Scan(&old)
	if r.FormValue("remove_picture") == "1" && old.Valid {
		db.Exec(`UPDATE creatures SET picture_path = NULL WHERE id = ?`, id)
		removePicture(uploadsDir, old.String)
		old = sql.NullString{}
	}
	file, header, err := r.FormFile("picture")
	if err != nil {
		return
	}
	defer file.Close()
	rel, err := saveUpload(uploadsDir, "creatures", fmt.Sprintf("%d-%d", id, time.Now().UnixNano()), header.Filename, file)
	if err != nil {
		log.Printf("save creature picture: %v", err)
		return
	}
	if _, err := db.Exec(`UPDATE creatures SET picture_path = ? WHERE id = ?`, rel, id); err != nil {
		log.Printf("update creature picture: %v", err)
		return
	}
	if old.Valid && old.String != "" && old.String != rel {
		removePicture(uploadsDir, old.String)
	}
}

// saveCreatureHandler creates (no id in the path) or updates a creature, its
// relations and picture, in one multipart request.
func saveCreatureHandler(db *sql.DB, uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var id int64
		if raw := r.PathValue("id"); raw != "" {
			var one int
			id, _ = strconv.ParseInt(raw, 10, 64)
			if db.QueryRow(`SELECT 1 FROM creatures WHERE id = ?`, id).Scan(&one) != nil {
				http.Error(w, "creature not found", http.StatusNotFound)
				return
			}
		}
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, "invalid form data", http.StatusBadRequest)
			return
		}
		f, msg := parseCreatureForm(db, r, id)
		if msg != "" {
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to save the creature", http.StatusInternalServerError)
			return
		}
		if id == 0 {
			res, err := tx.Exec(`INSERT INTO creatures (name, kind, level_min, level_max, region_id, description)
				VALUES (?, ?, ?, ?, ?, ?)`, f.Name, f.Kind, f.LevelMin, f.LevelMax, f.Region, nullableString(f.Description))
			if err != nil {
				tx.Rollback()
				log.Printf("create creature: %v", err)
				http.Error(w, "failed to save the creature", http.StatusInternalServerError)
				return
			}
			id, _ = res.LastInsertId()
		} else if _, err := tx.Exec(`UPDATE creatures SET name = ?, kind = ?, level_min = ?, level_max = ?, region_id = ?,
				description = ?, updated_at = datetime('now') WHERE id = ?`,
			f.Name, f.Kind, f.LevelMin, f.LevelMax, f.Region, nullableString(f.Description), id); err != nil {
			tx.Rollback()
			log.Printf("update creature: %v", err)
			http.Error(w, "failed to save the creature", http.StatusInternalServerError)
			return
		}
		if f.Territory != nil {
			if err := saveTerritory(tx, id, *f.Territory); err != nil {
				tx.Rollback()
				log.Printf("creature territory: %v", err)
				http.Error(w, "failed to save the creature", http.StatusInternalServerError)
				return
			}
		}
		if r.Form["related"] != nil {
			if err := saveCreatureLinks(tx, id, f.Related); err != nil {
				tx.Rollback()
				log.Printf("creature links: %v", err)
				http.Error(w, "failed to save the creature", http.StatusInternalServerError)
				return
			}
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to save the creature", http.StatusInternalServerError)
			return
		}
		saveCreaturePicture(db, r, uploadsDir, id)
		out, err := loadCreature(db, id)
		if err != nil {
			http.Error(w, "failed to save the creature", http.StatusInternalServerError)
			return
		}
		writeJSON(w, out)
	}
}

func deleteCreatureHandler(db *sql.DB, uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
		var pic sql.NullString
		if err := db.QueryRow(`SELECT picture_path FROM creatures WHERE id = ?`, id).Scan(&pic); err != nil {
			http.Error(w, "creature not found", http.StatusNotFound)
			return
		}
		if _, err := db.Exec(`DELETE FROM creatures WHERE id = ?`, id); err != nil {
			log.Printf("delete creature: %v", err)
			http.Error(w, "failed to delete the creature", http.StatusInternalServerError)
			return
		}
		if pic.Valid {
			removePicture(uploadsDir, pic.String)
		}
		writeJSON(w, map[string]any{"deleted": id})
	}
}

// levelRange reads a creature's levels: "5–12", "5+", "up to 12" or "".
func levelRange(lo, hi sql.NullInt64) string {
	switch {
	case lo.Valid && hi.Valid && lo.Int64 == hi.Int64:
		return strconv.FormatInt(lo.Int64, 10)
	case lo.Valid && hi.Valid:
		return fmt.Sprintf("%d–%d", lo.Int64, hi.Int64)
	case lo.Valid:
		return fmt.Sprintf("%d+", lo.Int64)
	case hi.Valid:
		return fmt.Sprintf("≤ %d", hi.Int64)
	}
	return ""
}

// creatureSchemaStatements create the bestiary's tables. The fresh schema
// runs them before the wiki's, since the wiki's triggers include theirs (as
// with lore articles).
var creatureSchemaStatements = []string{
	creaturesTableSQL,
	creatureLinksTableSQL,
	creatureHexesTableSQL,
	`CREATE INDEX idx_creature_links_b ON creature_links(b_id)`,
}

// gearDescriptionSQL is the rest of schema 25, run by the fresh schema
// after the gear table exists.
const gearDescriptionSQL = `ALTER TABLE gear ADD COLUMN description TEXT`

// schema25Statements upgrade a database to schema 25: gear descriptions and
// the bestiary with its wiki trigger.
var schema25Statements = slices.Concat(
	[]string{gearDescriptionSQL},
	creatureSchemaStatements,
	[]string{wikiTriggerSQL("creature")},
)
