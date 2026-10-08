package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// spellDetail is one spell as the frontend sees it. mp_cost / hp_cost are
// null when not set; the frontend shows "Free" when both are.
type spellDetail struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Level       int    `json:"level"`
	IconPath    string `json:"icon_path"`
	SourceType  string `json:"source_type"` // "class" | "subclass" | "specialization" | ""
	SourceID    int64  `json:"source_id"`
	SourceName  string `json:"source_name"`
	MPCost      *int64 `json:"mp_cost"`
	HPCost      *int64 `json:"hp_cost"`
	OriginID    *int64 `json:"origin_id"`
	Origin      string `json:"origin"`
	Description string `json:"description"`
}

const spellSelect = `
	SELECT
		s.id, s.name, s.level, COALESCE(s.icon_path, ''),
		COALESCE(s.source_type, ''), COALESCE(s.source_id, 0),
		CASE s.source_type
			WHEN 'class' THEN c.name
			WHEN 'subclass' THEN sc.name
			WHEN 'specialization' THEN sp.name
		END,
		s.mp_cost, s.hp_cost, s.origin_location_id, COALESCE(l.name, ''),
		COALESCE(s.description, '')
	FROM spells s
	LEFT JOIN classes c ON s.source_type = 'class' AND c.id = s.source_id
	LEFT JOIN subclasses sc ON s.source_type = 'subclass' AND sc.id = s.source_id
	LEFT JOIN specializations sp ON s.source_type = 'specialization' AND sp.id = s.source_id
	LEFT JOIN locations l ON l.id = s.origin_location_id
`

const spellOrder = `ORDER BY s.level, s.name COLLATE NOCASE`

// querySpells runs spellSelect with whatever WHERE/ORDER BY suffix the
// caller needs.
func querySpells(db *sql.DB, suffix string, args ...any) ([]spellDetail, error) {
	rows, err := db.Query(spellSelect+" "+suffix, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	spells := []spellDetail{}
	for rows.Next() {
		var s spellDetail
		var sourceName sql.NullString
		var mp, hp, originID sql.NullInt64
		if err := rows.Scan(
			&s.ID, &s.Name, &s.Level, &s.IconPath,
			&s.SourceType, &s.SourceID, &sourceName,
			&mp, &hp, &originID, &s.Origin, &s.Description,
		); err != nil {
			return nil, err
		}
		if s.IconPath != "" && !isBuiltinIcon(s.IconPath) {
			s.IconPath = "/uploads/" + s.IconPath
		}
		if sourceName.Valid {
			s.SourceName = sourceName.String
		} else {
			// The class/subclass/specialization it pointed at no longer
			// exists — treat as having no source.
			s.SourceType = ""
			s.SourceID = 0
		}
		if mp.Valid {
			v := mp.Int64
			s.MPCost = &v
		}
		if hp.Valid {
			v := hp.Int64
			s.HPCost = &v
		}
		if originID.Valid {
			v := originID.Int64
			s.OriginID = &v
		}
		spells = append(spells, s)
	}
	return spells, rows.Err()
}

// loadVersionSpells returns the spells a given version of a character knows.
func loadVersionSpells(db *sql.DB, versionID int64) ([]spellDetail, error) {
	return querySpells(db,
		`WHERE s.id IN (SELECT spell_id FROM character_spells WHERE version_id = ?) `+spellOrder,
		versionID,
	)
}

// replaceVersionSpells sets the exact list of spells a version knows.
// Unknown spell ids are skipped rather than failing the whole save.
func replaceVersionSpells(tx *sql.Tx, versionID int64, spellIDs []int64) error {
	if _, err := tx.Exec(`DELETE FROM character_spells WHERE version_id = ?`, versionID); err != nil {
		return err
	}
	for _, id := range spellIDs {
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO character_spells (version_id, spell_id)
			 SELECT ?, id FROM spells WHERE id = ?`,
			versionID, id,
		); err != nil {
			return err
		}
	}
	return nil
}

// ---- Handlers ---------------------------------------------------------------

func listSpellsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		spells, err := querySpells(db, spellOrder)
		if err != nil {
			http.Error(w, "failed to load spells", http.StatusInternalServerError)
			log.Printf("list spells: %v", err)
			return
		}
		writeJSON(w, spells)
	}
}

func getSpellHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		spells, err := querySpells(db, `WHERE s.id = ?`, r.PathValue("id"))
		if err != nil || len(spells) == 0 {
			http.Error(w, "spell not found", http.StatusNotFound)
			return
		}
		writeJSON(w, spells[0])
	}
}

type spellInput struct {
	name        string
	level       int
	sourceType  string
	sourceID    sql.NullInt64
	mp          sql.NullInt64
	hp          sql.NullInt64
	origin      sql.NullInt64
	description sql.NullString
}

var spellSourceTables = map[string]string{
	"class":          "classes",
	"subclass":       "subclasses",
	"specialization": "specializations",
}

// parseCost turns a blank, zero, or negative field into NULL — "no MP
// cost" and "0 MP cost" are the same thing here.
func parseCost(s string) sql.NullInt64 {
	v := parseIntDefault(s, 0)
	if v <= 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(v), Valid: true}
}

// parseSpellForm reads the multipart spell form. The Origin field is the
// name of an existing Map location.
func parseSpellForm(db *sql.DB, r *http.Request) (spellInput, error) {
	var in spellInput
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		return in, fmt.Errorf("invalid form data")
	}

	in.name = strings.TrimSpace(r.FormValue("name"))
	if in.name == "" {
		return in, fmt.Errorf("name is required")
	}
	in.level = parseIntDefault(r.FormValue("level"), 1)
	if in.level < 0 {
		in.level = 0
	}

	// Source arrives as "type:id". Anything unrecognized, or pointing at
	// something that no longer exists, simply means "no source".
	if parts := strings.SplitN(r.FormValue("source"), ":", 2); len(parts) == 2 {
		if table, ok := spellSourceTables[parts[0]]; ok {
			if id, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
				var exists int
				if db.QueryRow(`SELECT 1 FROM `+table+` WHERE id = ?`, id).Scan(&exists) == nil {
					in.sourceType = parts[0]
					in.sourceID = sql.NullInt64{Int64: id, Valid: true}
				}
			}
		}
	}

	in.mp = parseCost(r.FormValue("mp_cost"))
	in.hp = parseCost(r.FormValue("hp_cost"))

	// Origin is one of the locations on the Map. An unknown name is an
	// error rather than quietly creating a location with no place on the map.
	if origin := strings.TrimSpace(r.FormValue("origin")); origin != "" {
		id := lookupIDByName(db, "locations", origin)
		if !id.Valid {
			return in, fmt.Errorf("no location named %q — create it on the Map page first", origin)
		}
		in.origin = id
	}

	in.description = nullableString(strings.TrimSpace(r.FormValue("description")))
	return in, nil
}

// attachSpellIcon applies the form's icon choice: an uploaded file wins;
// otherwise a built-in pick ("icon_choice=builtin:flame") or an explicit
// "none" replaces the current icon; with neither, the icon is untouched.
// Whatever uploaded file the spell had before is removed when it's
// replaced or cleared. Uploaded filenames include a timestamp so a replaced
// icon gets a fresh URL instead of being masked by the browser's cache.
func attachSpellIcon(db *sql.DB, uploadsDir string, r *http.Request, spellID int64, oldPath string) {
	newPath := ""
	changed := false

	if file, header, err := r.FormFile("icon"); err == nil {
		defer file.Close()
		stem := fmt.Sprintf("%d-%d", spellID, time.Now().Unix())
		relPath, err := saveUpload(uploadsDir, "spells", stem, header.Filename, file)
		if err != nil {
			log.Printf("save spell icon: %v", err)
			return
		}
		newPath, changed = relPath, true
	} else if choice := parseIconChoice(r.FormValue("icon_choice")); choice.change {
		newPath, changed = choice.value, true
	}
	if !changed {
		return
	}

	var stored any = newPath
	if newPath == "" {
		stored = nil
	}
	if _, err := db.Exec(`UPDATE spells SET icon_path = ? WHERE id = ?`, stored, spellID); err != nil {
		log.Printf("update spell icon_path: %v", err)
		return
	}
	if oldPath != "" && oldPath != newPath {
		removePicture(uploadsDir, oldPath)
	}
}

func createSpellHandler(db *sql.DB, uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		in, err := parseSpellForm(db, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		res, err := db.Exec(
			`INSERT INTO spells
				(name, level, source_type, source_id, mp_cost, hp_cost, origin_location_id, description)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			in.name, in.level, nullableString(in.sourceType), in.sourceID,
			in.mp, in.hp, in.origin, in.description,
		)
		if err != nil {
			http.Error(w, "failed to create spell", http.StatusInternalServerError)
			log.Printf("insert spell: %v", err)
			return
		}
		id, _ := res.LastInsertId()

		attachSpellIcon(db, uploadsDir, r, id, "")

		spells, err := querySpells(db, `WHERE s.id = ?`, id)
		if err != nil || len(spells) == 0 {
			writeJSON(w, map[string]any{"id": id})
			return
		}
		writeJSON(w, spells[0])
	}
}

func updateSpellHandler(db *sql.DB, uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "spell not found", http.StatusNotFound)
			return
		}

		var oldIcon sql.NullString
		if err := db.QueryRow(`SELECT icon_path FROM spells WHERE id = ?`, id).Scan(&oldIcon); err != nil {
			http.Error(w, "spell not found", http.StatusNotFound)
			return
		}

		in, err := parseSpellForm(db, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if _, err := db.Exec(
			`UPDATE spells
			 SET name = ?, level = ?, source_type = ?, source_id = ?, mp_cost = ?, hp_cost = ?,
				origin_location_id = ?, description = ?
			 WHERE id = ?`,
			in.name, in.level, nullableString(in.sourceType), in.sourceID,
			in.mp, in.hp, in.origin, in.description, id,
		); err != nil {
			http.Error(w, "failed to update spell", http.StatusInternalServerError)
			log.Printf("update spell: %v", err)
			return
		}

		attachSpellIcon(db, uploadsDir, r, id, oldIcon.String)

		spells, err := querySpells(db, `WHERE s.id = ?`, id)
		if err != nil || len(spells) == 0 {
			writeJSON(w, map[string]any{"id": id})
			return
		}
		writeJSON(w, spells[0])
	}
}

func deleteSpellHandler(db *sql.DB, uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var icon sql.NullString
		db.QueryRow(`SELECT icon_path FROM spells WHERE id = ?`, id).Scan(&icon)

		// Removed explicitly rather than relying on the cascade, so no
		// character is ever left pointing at a spell that's gone.
		if _, err := db.Exec(`DELETE FROM character_spells WHERE spell_id = ?`, id); err != nil {
			http.Error(w, "failed to remove spell from characters", http.StatusInternalServerError)
			return
		}
		if _, err := db.Exec(`DELETE FROM spells WHERE id = ?`, id); err != nil {
			http.Error(w, "failed to delete spell", http.StatusInternalServerError)
			return
		}
		if icon.Valid {
			removePicture(uploadsDir, icon.String)
		}

		writeJSON(w, map[string]any{"deleted": id})
	}
}
