package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

var hexColorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// maxHexBatch caps how many hexes one paint/erase request may touch.
const maxHexBatch = 100000

type mapLocation struct {
	ID           int64  `json:"id"`
	Kind         string `json:"kind"` // "major" | "minor"
	Name         string `json:"name"`
	Color        string `json:"color"` // "#rrggbb", or "" for a colorless major / any minor
	FoundingDate string `json:"founding_date"`
	Description  string `json:"description"`
	Q            *int64 `json:"q"` // minor locations only; null = not placed on a hex yet
	R            *int64 `json:"r"`
	BelongsToID  *int64 `json:"belongs_to_id"` // minor locations only; null = automatic
}

// mapPayload is the whole map in one response: every location, plus every
// hex a major location covers as [location_id, q, r] triples.
type mapPayload struct {
	Locations []mapLocation `json:"locations"`
	Hexes     [][3]int64    `json:"hexes"`
}

const mapLocationSelect = `
	SELECT id, kind, name, COALESCE(color, ''), COALESCE(founding_date, ''),
	       COALESCE(description, ''), q, r, belongs_to_id
	FROM locations
`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanMapLocation(s rowScanner) (mapLocation, error) {
	var l mapLocation
	var q, r, belongs sql.NullInt64
	if err := s.Scan(&l.ID, &l.Kind, &l.Name, &l.Color, &l.FoundingDate, &l.Description, &q, &r, &belongs); err != nil {
		return l, err
	}
	if q.Valid {
		v := q.Int64
		l.Q = &v
	}
	if r.Valid {
		v := r.Int64
		l.R = &v
	}
	if belongs.Valid {
		v := belongs.Int64
		l.BelongsToID = &v
	}
	return l, nil
}

func getMapHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		payload := mapPayload{Locations: []mapLocation{}, Hexes: [][3]int64{}}

		rows, err := db.Query(mapLocationSelect + ` ORDER BY name COLLATE NOCASE`)
		if err != nil {
			http.Error(w, "failed to load locations", http.StatusInternalServerError)
			log.Printf("map locations: %v", err)
			return
		}
		for rows.Next() {
			l, err := scanMapLocation(rows)
			if err != nil {
				rows.Close()
				http.Error(w, "failed to read locations", http.StatusInternalServerError)
				log.Printf("map locations scan: %v", err)
				return
			}
			payload.Locations = append(payload.Locations, l)
		}
		rows.Close()

		hexRows, err := db.Query(`SELECT location_id, q, r FROM location_hexes`)
		if err != nil {
			http.Error(w, "failed to load hexes", http.StatusInternalServerError)
			log.Printf("map hexes: %v", err)
			return
		}
		for hexRows.Next() {
			var h [3]int64
			if err := hexRows.Scan(&h[0], &h[1], &h[2]); err != nil {
				hexRows.Close()
				http.Error(w, "failed to read hexes", http.StatusInternalServerError)
				return
			}
			payload.Hexes = append(payload.Hexes, h)
		}
		hexRows.Close()

		writeJSON(w, payload)
	}
}

type mapLocationInput struct {
	Kind         string `json:"kind"`
	Name         string `json:"name"`
	Color        string `json:"color"`
	FoundingDate string `json:"founding_date"`
	Description  string `json:"description"`
	Q            *int64 `json:"q"`
	R            *int64 `json:"r"`
	BelongsToID  *int64 `json:"belongs_to_id"`
}

func optInt(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

// normalizeColor returns the lowercase #rrggbb, or "" for "no color".
// ok is false for anything malformed.
func normalizeColor(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", true
	}
	if !hexColorRe.MatchString(s) {
		return "", false
	}
	return strings.ToLower(s), true
}

func colorTaken(db *sql.DB, color string, exceptID int64) (bool, error) {
	var n int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM locations WHERE kind = 'major' AND color = ? AND id != ?`,
		color, exceptID,
	).Scan(&n)
	return n > 0, err
}

// validBelongsTo only accepts an existing major location; anything else
// quietly means "automatic".
func validBelongsTo(db *sql.DB, id *int64) sql.NullInt64 {
	if id == nil {
		return sql.NullInt64{}
	}
	var exists int
	if db.QueryRow(`SELECT 1 FROM locations WHERE id = ? AND kind = 'major'`, *id).Scan(&exists) != nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *id, Valid: true}
}

func writeMapLocation(w http.ResponseWriter, db *sql.DB, id int64) {
	l, err := scanMapLocation(db.QueryRow(mapLocationSelect+` WHERE id = ?`, id))
	if err != nil {
		writeJSON(w, map[string]any{"id": id})
		return
	}
	writeJSON(w, l)
}

func createMapLocationHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in mapLocationInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		in.Name = strings.TrimSpace(in.Name)
		if in.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		if msg := checkDateFits(loadCalendar(db), "Founding date", in.FoundingDate); msg != "" {
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		if in.Kind != "major" && in.Kind != "minor" {
			http.Error(w, "kind must be major or minor", http.StatusBadRequest)
			return
		}

		var color sql.NullString
		var q, rr, belongs sql.NullInt64
		if in.Kind == "major" {
			c, ok := normalizeColor(in.Color)
			if !ok {
				http.Error(w, "color must look like #rrggbb", http.StatusBadRequest)
				return
			}
			if c != "" {
				taken, err := colorTaken(db, c, 0)
				if err != nil {
					http.Error(w, "failed to check color", http.StatusInternalServerError)
					return
				}
				if taken {
					http.Error(w, "that color is already used by another kingdom", http.StatusBadRequest)
					return
				}
				color = sql.NullString{String: c, Valid: true}
			}
		} else {
			if in.Q == nil || in.R == nil {
				http.Error(w, "a location needs a hex", http.StatusBadRequest)
				return
			}
			q, rr = optInt(in.Q), optInt(in.R)
			belongs = validBelongsTo(db, in.BelongsToID)
		}

		res, err := db.Exec(
			`INSERT INTO locations (name, kind, color, founding_date, description, q, r, belongs_to_id)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			in.Name, in.Kind, color,
			nullableString(normalizeStoryDate(in.FoundingDate)),
			nullableString(strings.TrimSpace(in.Description)),
			q, rr, belongs,
		)
		if err != nil {
			log.Printf("insert location: %v", err)
			http.Error(w, "a location with that name already exists", http.StatusBadRequest)
			return
		}
		id, _ := res.LastInsertId()
		writeMapLocation(w, db, id)
	}
}

func updateMapLocationHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "location not found", http.StatusNotFound)
			return
		}
		var kind string
		var oldColor sql.NullString
		if err := db.QueryRow(`SELECT kind, color FROM locations WHERE id = ?`, id).Scan(&kind, &oldColor); err != nil {
			http.Error(w, "location not found", http.StatusNotFound)
			return
		}

		var in mapLocationInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		in.Name = strings.TrimSpace(in.Name)
		if in.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		if msg := checkDateFits(loadCalendar(db), "Founding date", in.FoundingDate); msg != "" {
			http.Error(w, msg, http.StatusBadRequest)
			return
		}

		// A location's kind never changes; everything else is replaced.
		var color sql.NullString
		var q, rr, belongs sql.NullInt64
		newColor := ""
		if kind == "major" {
			c, ok := normalizeColor(in.Color)
			if !ok {
				http.Error(w, "color must look like #rrggbb", http.StatusBadRequest)
				return
			}
			if c != "" {
				taken, err := colorTaken(db, c, id)
				if err != nil {
					http.Error(w, "failed to check color", http.StatusInternalServerError)
					return
				}
				if taken {
					http.Error(w, "that color is already used by another kingdom", http.StatusBadRequest)
					return
				}
				color = sql.NullString{String: c, Valid: true}
				newColor = c
			}
		} else {
			q, rr = optInt(in.Q), optInt(in.R)
			belongs = validBelongsTo(db, in.BelongsToID)
		}

		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to start transaction", http.StatusInternalServerError)
			return
		}
		if _, err := tx.Exec(
			`UPDATE locations
			 SET name = ?, color = ?, founding_date = ?, description = ?, q = ?, r = ?, belongs_to_id = ?
			 WHERE id = ?`,
			in.Name, color,
			nullableString(normalizeStoryDate(in.FoundingDate)),
			nullableString(strings.TrimSpace(in.Description)),
			q, rr, belongs, id,
		); err != nil {
			tx.Rollback()
			log.Printf("update location: %v", err)
			http.Error(w, "a location with that name already exists", http.StatusBadRequest)
			return
		}

		// A colorless major gaining a color becomes a kingdom: where its
		// hexes overlap another kingdom, it takes them over — the same
		// "newest paint wins" rule as painting.
		if kind == "major" && newColor != "" && !oldColor.Valid {
			if _, err := tx.Exec(
				`DELETE FROM location_hexes
				 WHERE location_id != ?
				   AND location_id IN (SELECT id FROM locations WHERE kind = 'major' AND color IS NOT NULL)
				   AND (q, r) IN (SELECT q, r FROM location_hexes WHERE location_id = ?)`,
				id, id,
			); err != nil {
				tx.Rollback()
				http.Error(w, "failed to resolve overlapping hexes", http.StatusInternalServerError)
				log.Printf("resolve overlaps: %v", err)
				return
			}
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to save", http.StatusInternalServerError)
			return
		}
		writeMapLocation(w, db, id)
	}
}

func deleteMapLocationHandler(db *sql.DB, uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		// A founding-details event attached to this location goes with it.
		// Plain events merely held at this location keep existing and just
		// lose the location (the UPDATE below).
		if locID, err := strconv.ParseInt(id, 10, 64); err == nil {
			deleteSourceEvents(db, uploadsDir, "location", []int64{locID})
		}

		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to start transaction", http.StatusInternalServerError)
			return
		}
		// Cleared explicitly rather than relying on foreign-key cascades.
		for _, stmt := range []string{
			`DELETE FROM location_hexes WHERE location_id = ?`,
			`UPDATE locations SET belongs_to_id = NULL WHERE belongs_to_id = ?`,
			`UPDATE spells SET origin_location_id = NULL WHERE origin_location_id = ?`,
			`UPDATE events SET location_id = NULL WHERE location_id = ?`,
			`DELETE FROM locations WHERE id = ?`,
		} {
			if _, err := tx.Exec(stmt, id); err != nil {
				tx.Rollback()
				log.Printf("delete location (%s): %v", stmt, err)
				http.Error(w, "failed to delete location", http.StatusInternalServerError)
				return
			}
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to delete location", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"deleted": id})
	}
}

type hexBatch struct {
	LocationID *int64     `json:"location_id"`
	Hexes      [][2]int64 `json:"hexes"`
}

// paintHexesHandler adds hexes to a major location. A colored location
// takes the hex over from any other colored one (a hex has one kingdom);
// a colorless one just joins in without touching the kingdom.
func paintHexesHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var b hexBatch
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil || b.LocationID == nil {
			http.Error(w, "location_id and hexes are required", http.StatusBadRequest)
			return
		}
		if len(b.Hexes) > maxHexBatch {
			http.Error(w, "too many hexes in one request", http.StatusBadRequest)
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
			http.Error(w, "failed to start transaction", http.StatusInternalServerError)
			return
		}
		for _, h := range b.Hexes {
			if color.Valid {
				if _, err := tx.Exec(
					`DELETE FROM location_hexes
					 WHERE q = ? AND r = ? AND location_id != ?
					   AND location_id IN (SELECT id FROM locations WHERE kind = 'major' AND color IS NOT NULL)`,
					h[0], h[1], *b.LocationID,
				); err != nil {
					tx.Rollback()
					log.Printf("paint (clear): %v", err)
					http.Error(w, "failed to paint", http.StatusInternalServerError)
					return
				}
			}
			if _, err := tx.Exec(
				`INSERT OR IGNORE INTO location_hexes (location_id, q, r) VALUES (?, ?, ?)`,
				*b.LocationID, h[0], h[1],
			); err != nil {
				tx.Rollback()
				log.Printf("paint (insert): %v", err)
				http.Error(w, "failed to paint", http.StatusInternalServerError)
				return
			}
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to paint", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"painted": len(b.Hexes)})
	}
}

// eraseHexesHandler removes one major location from the given hexes — or,
// when no location_id is sent, everything that covers them.
func eraseHexesHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var b hexBatch
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if len(b.Hexes) > maxHexBatch {
			http.Error(w, "too many hexes in one request", http.StatusBadRequest)
			return
		}

		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to start transaction", http.StatusInternalServerError)
			return
		}
		for _, h := range b.Hexes {
			var err error
			if b.LocationID != nil {
				_, err = tx.Exec(
					`DELETE FROM location_hexes WHERE location_id = ? AND q = ? AND r = ?`,
					*b.LocationID, h[0], h[1],
				)
			} else {
				_, err = tx.Exec(`DELETE FROM location_hexes WHERE q = ? AND r = ?`, h[0], h[1])
			}
			if err != nil {
				tx.Rollback()
				log.Printf("erase: %v", err)
				http.Error(w, "failed to erase", http.StatusInternalServerError)
				return
			}
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to erase", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"erased": len(b.Hexes)})
	}
}

// locationOption is the slim shape the Born in / Nation pickers need —
// no hexes, so it stays cheap to fetch on every character edit.
type locationOption struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Kind  string `json:"kind"`
	Color string `json:"color"`
}

func listLocationOptionsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(`SELECT id, name, kind, COALESCE(color, '') FROM locations ORDER BY name COLLATE NOCASE`)
		if err != nil {
			http.Error(w, "failed to load locations", http.StatusInternalServerError)
			log.Printf("location options: %v", err)
			return
		}
		defer rows.Close()
		out := []locationOption{}
		for rows.Next() {
			var o locationOption
			if err := rows.Scan(&o.ID, &o.Name, &o.Kind, &o.Color); err != nil {
				http.Error(w, "failed to read locations", http.StatusInternalServerError)
				return
			}
			out = append(out, o)
		}
		writeJSON(w, out)
	}
}
