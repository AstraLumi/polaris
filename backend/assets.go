package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

// scalingFor decides how a modifier saved through the asset editor scales:
// Base Stats (hp/attack/defense/...) scale with the character's level by
// default — "+10 Attack per level" — everything else is a flat one-time
// bonus. See perLevelModifierStats in stats.go.
//
// Gear ("item" modifiers) is always flat, Base Stats included: "+5 Attack"
// from a sword is +5 however high the level is.
func scalingFor(sourceType, targetStat string) string {
	if sourceType != "item" && perLevelModifierStats[targetStat] {
		return "per_level"
	}
	return "flat"
}

// loadModifiers reads every stat_modifiers row for one source (e.g. one
// race) into a flat map, ready to prefill an edit form. Scaling isn't
// part of the map — scalingFor derives it deterministically from the key,
// so there's nothing to lose by not round-tripping it.
func loadModifiers(db *sql.DB, sourceType string, sourceID int64) (map[string]float64, error) {
	rows, err := db.Query(
		`SELECT target_stat, value FROM stat_modifiers WHERE source_type = ? AND source_id = ?`,
		sourceType, sourceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	modifiers := map[string]float64{}
	for rows.Next() {
		var key string
		var value float64
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		modifiers[key] = value
	}
	return modifiers, rows.Err()
}

// saveModifiers replaces every modifier row for one source with whatever's
// in values, applying scalingFor per key. Zero/absent entries simply
// aren't stored — a blank field on the form means "this asset doesn't
// touch that stat".
func saveModifiers(tx *sql.Tx, sourceType string, sourceID int64, values map[string]float64) error {
	if _, err := tx.Exec(
		`DELETE FROM stat_modifiers WHERE source_type = ? AND source_id = ?`,
		sourceType, sourceID,
	); err != nil {
		return err
	}
	for key, value := range values {
		if value == 0 {
			continue
		}
		if _, err := tx.Exec(
			`INSERT INTO stat_modifiers (source_type, source_id, target_stat, scaling, value) VALUES (?, ?, ?, ?, ?)`,
			sourceType, sourceID, key, scalingFor(sourceType, key), value,
		); err != nil {
			return err
		}
	}
	return nil
}

type assetPayload struct {
	Name      string             `json:"name"`
	Modifiers map[string]float64 `json:"modifiers"`
}

// ---- Simple assets: Classes, Races, Body Types ----------------------------
// All three are "a name plus modifiers", no class-scoping involved, so one
// set of handlers (parameterized by table + stat_modifiers source_type)
// covers all three.

type simpleAssetDetail struct {
	ID        int64              `json:"id"`
	Name      string             `json:"name"`
	IconPath  string             `json:"icon_path,omitempty"` // classes only
	Modifiers map[string]float64 `json:"modifiers"`
}

func getSimpleAssetHandler(db *sql.DB, table, sourceType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var d simpleAssetDetail
		if err := db.QueryRow(`SELECT id, name FROM `+table+` WHERE id = ?`, id).Scan(&d.ID, &d.Name); err != nil {
			http.Error(w, sourceType+" not found", http.StatusNotFound)
			return
		}
		modifiers, err := loadModifiers(db, sourceType, d.ID)
		if err != nil {
			http.Error(w, "failed to load modifiers", http.StatusInternalServerError)
			log.Printf("loadModifiers %s %s: %v", sourceType, id, err)
			return
		}
		d.Modifiers = modifiers
		if table == "classes" {
			var icon sql.NullString
			db.QueryRow(`SELECT icon_path FROM classes WHERE id = ?`, d.ID).Scan(&icon)
			d.IconPath = uploadURL(icon.String)
		}
		writeJSON(w, d)
	}
}

func createSimpleAssetHandler(db *sql.DB, table, sourceType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload assetPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		name := strings.TrimSpace(payload.Name)
		if name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}

		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to start transaction", http.StatusInternalServerError)
			return
		}
		res, err := tx.Exec(`INSERT INTO `+table+` (name) VALUES (?)`, name)
		if err != nil {
			tx.Rollback()
			http.Error(w, "a "+sourceType+" with that name may already exist", http.StatusBadRequest)
			return
		}
		id, _ := res.LastInsertId()
		if err := saveModifiers(tx, sourceType, id, payload.Modifiers); err != nil {
			tx.Rollback()
			http.Error(w, "failed to save modifiers", http.StatusInternalServerError)
			log.Printf("saveModifiers %s %d: %v", sourceType, id, err)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to save", http.StatusInternalServerError)
			return
		}

		writeJSON(w, map[string]any{"id": id})
	}
}

func updateSimpleAssetHandler(db *sql.DB, table, sourceType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var payload assetPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		name := strings.TrimSpace(payload.Name)
		if name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}

		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to start transaction", http.StatusInternalServerError)
			return
		}
		if _, err := tx.Exec(`UPDATE `+table+` SET name = ? WHERE id = ?`, name, id); err != nil {
			tx.Rollback()
			http.Error(w, "a "+sourceType+" with that name may already exist", http.StatusBadRequest)
			return
		}
		var assetID int64
		tx.QueryRow(`SELECT id FROM `+table+` WHERE id = ?`, id).Scan(&assetID)
		if err := saveModifiers(tx, sourceType, assetID, payload.Modifiers); err != nil {
			tx.Rollback()
			http.Error(w, "failed to save modifiers", http.StatusInternalServerError)
			log.Printf("saveModifiers %s %s: %v", sourceType, id, err)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to save", http.StatusInternalServerError)
			return
		}

		writeJSON(w, map[string]any{"id": assetID})
	}
}

func deleteSimpleAssetHandler(db *sql.DB, uploadsDir, table, sourceType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var oldIcon sql.NullString
		if table == "classes" {
			db.QueryRow(`SELECT icon_path FROM classes WHERE id = ?`, id).Scan(&oldIcon)
		}
		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to start transaction", http.StatusInternalServerError)
			return
		}
		// stat_modifiers has no FK here — source_id is polymorphic, so its
		// rows need to be cleaned up explicitly rather than cascading.
		if _, err := tx.Exec(`DELETE FROM stat_modifiers WHERE source_type = ? AND source_id = ?`, sourceType, id); err != nil {
			tx.Rollback()
			http.Error(w, "failed to delete modifiers", http.StatusInternalServerError)
			return
		}
		// Same story for spells that name this as their source (a no-op for
		// races and body types, which spells can't point at).
		if _, err := tx.Exec(`UPDATE spells SET source_type = NULL, source_id = NULL WHERE source_type = ? AND source_id = ?`, sourceType, id); err != nil {
			tx.Rollback()
			http.Error(w, "failed to clear spell sources", http.StatusInternalServerError)
			return
		}
		// Deleting a class also cascades away its specializations, so their
		// modifiers and spell references need clearing too.
		if table == "classes" {
			if _, err := tx.Exec(`DELETE FROM stat_modifiers WHERE source_type = 'specialization' AND source_id IN (SELECT id FROM specializations WHERE class_id = ?)`, id); err != nil {
				tx.Rollback()
				http.Error(w, "failed to delete specialization modifiers", http.StatusInternalServerError)
				return
			}
			if _, err := tx.Exec(`UPDATE spells SET source_type = NULL, source_id = NULL WHERE source_type = 'specialization' AND source_id IN (SELECT id FROM specializations WHERE class_id = ?)`, id); err != nil {
				tx.Rollback()
				http.Error(w, "failed to clear spell sources", http.StatusInternalServerError)
				return
			}
		}
		if _, err := tx.Exec(`DELETE FROM `+table+` WHERE id = ?`, id); err != nil {
			tx.Rollback()
			http.Error(w, "failed to delete "+sourceType, http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to delete", http.StatusInternalServerError)
			return
		}
		if oldIcon.Valid {
			removePicture(uploadsDir, oldIcon.String)
		}
		writeJSON(w, map[string]any{"deleted": id})
	}
}

// ---- Specializations: name + exactly one required class -------------------

type specializationPayload struct {
	Name      string             `json:"name"`
	ClassID   int64              `json:"class_id"`
	Modifiers map[string]float64 `json:"modifiers"`
}

type specializationDetail struct {
	ID        int64              `json:"id"`
	Name      string             `json:"name"`
	ClassID   int64              `json:"class_id"`
	ClassName string             `json:"class_name"`
	Modifiers map[string]float64 `json:"modifiers"`
}

func getSpecializationHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var d specializationDetail
		err := db.QueryRow(`
			SELECT sp.id, sp.name, sp.class_id, COALESCE(c.name, '')
			FROM specializations sp
			LEFT JOIN classes c ON c.id = sp.class_id
			WHERE sp.id = ?
		`, id).Scan(&d.ID, &d.Name, &d.ClassID, &d.ClassName)
		if err != nil {
			http.Error(w, "specialization not found", http.StatusNotFound)
			return
		}
		modifiers, err := loadModifiers(db, "specialization", d.ID)
		if err != nil {
			http.Error(w, "failed to load modifiers", http.StatusInternalServerError)
			return
		}
		d.Modifiers = modifiers
		writeJSON(w, d)
	}
}

func createSpecializationHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload specializationPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		name := strings.TrimSpace(payload.Name)
		if name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		if payload.ClassID == 0 {
			http.Error(w, "a class is required", http.StatusBadRequest)
			return
		}

		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to start transaction", http.StatusInternalServerError)
			return
		}
		res, err := tx.Exec(`INSERT INTO specializations (class_id, name) VALUES (?, ?)`, payload.ClassID, name)
		if err != nil {
			tx.Rollback()
			http.Error(w, "failed to create specialization", http.StatusBadRequest)
			return
		}
		id, _ := res.LastInsertId()
		if err := saveModifiers(tx, "specialization", id, payload.Modifiers); err != nil {
			tx.Rollback()
			http.Error(w, "failed to save modifiers", http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to save", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"id": id})
	}
}

func updateSpecializationHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var payload specializationPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		name := strings.TrimSpace(payload.Name)
		if name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		if payload.ClassID == 0 {
			http.Error(w, "a class is required", http.StatusBadRequest)
			return
		}

		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to start transaction", http.StatusInternalServerError)
			return
		}
		if _, err := tx.Exec(
			`UPDATE specializations SET name = ?, class_id = ? WHERE id = ?`,
			name, payload.ClassID, id,
		); err != nil {
			tx.Rollback()
			http.Error(w, "failed to update specialization", http.StatusBadRequest)
			return
		}
		var specID int64
		tx.QueryRow(`SELECT id FROM specializations WHERE id = ?`, id).Scan(&specID)
		if err := saveModifiers(tx, "specialization", specID, payload.Modifiers); err != nil {
			tx.Rollback()
			http.Error(w, "failed to save modifiers", http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to save", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"id": specID})
	}
}

func deleteSpecializationHandler(db *sql.DB, uploadsDir string) http.HandlerFunc {
	return deleteSimpleAssetHandler(db, uploadsDir, "specializations", "specialization")
}

// ---- Subclasses: name + zero or more classes -------------------------------

type subclassPayload struct {
	Name      string             `json:"name"`
	ClassIDs  []int64            `json:"class_ids"`
	Modifiers map[string]float64 `json:"modifiers"`
}

type subclassDetail struct {
	ID        int64              `json:"id"`
	Name      string             `json:"name"`
	ClassIDs  []int64            `json:"class_ids"`
	Modifiers map[string]float64 `json:"modifiers"`
}

func getSubclassHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var d subclassDetail
		if err := db.QueryRow(`SELECT id, name FROM subclasses WHERE id = ?`, id).Scan(&d.ID, &d.Name); err != nil {
			http.Error(w, "subclass not found", http.StatusNotFound)
			return
		}

		rows, err := db.Query(`SELECT class_id FROM subclass_classes WHERE subclass_id = ?`, d.ID)
		if err != nil {
			http.Error(w, "failed to load classes", http.StatusInternalServerError)
			return
		}
		d.ClassIDs = []int64{}
		for rows.Next() {
			var cid int64
			if err := rows.Scan(&cid); err == nil {
				d.ClassIDs = append(d.ClassIDs, cid)
			}
		}
		rows.Close()

		modifiers, err := loadModifiers(db, "subclass", d.ID)
		if err != nil {
			http.Error(w, "failed to load modifiers", http.StatusInternalServerError)
			return
		}
		d.Modifiers = modifiers
		writeJSON(w, d)
	}
}

func replaceSubclassClasses(tx *sql.Tx, subclassID int64, classIDs []int64) error {
	if _, err := tx.Exec(`DELETE FROM subclass_classes WHERE subclass_id = ?`, subclassID); err != nil {
		return err
	}
	for _, cid := range classIDs {
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO subclass_classes (subclass_id, class_id) VALUES (?, ?)`,
			subclassID, cid,
		); err != nil {
			return err
		}
	}
	return nil
}

func createSubclassHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload subclassPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		name := strings.TrimSpace(payload.Name)
		if name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}

		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to start transaction", http.StatusInternalServerError)
			return
		}
		res, err := tx.Exec(`INSERT INTO subclasses (name) VALUES (?)`, name)
		if err != nil {
			tx.Rollback()
			http.Error(w, "a subclass with that name may already exist", http.StatusBadRequest)
			return
		}
		id, _ := res.LastInsertId()
		if err := replaceSubclassClasses(tx, id, payload.ClassIDs); err != nil {
			tx.Rollback()
			http.Error(w, "failed to save classes", http.StatusInternalServerError)
			return
		}
		if err := saveModifiers(tx, "subclass", id, payload.Modifiers); err != nil {
			tx.Rollback()
			http.Error(w, "failed to save modifiers", http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to save", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"id": id})
	}
}

func updateSubclassHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var payload subclassPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		name := strings.TrimSpace(payload.Name)
		if name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}

		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to start transaction", http.StatusInternalServerError)
			return
		}
		if _, err := tx.Exec(`UPDATE subclasses SET name = ? WHERE id = ?`, name, id); err != nil {
			tx.Rollback()
			http.Error(w, "a subclass with that name may already exist", http.StatusBadRequest)
			return
		}
		var subID int64
		tx.QueryRow(`SELECT id FROM subclasses WHERE id = ?`, id).Scan(&subID)
		if err := replaceSubclassClasses(tx, subID, payload.ClassIDs); err != nil {
			tx.Rollback()
			http.Error(w, "failed to save classes", http.StatusInternalServerError)
			return
		}
		if err := saveModifiers(tx, "subclass", subID, payload.Modifiers); err != nil {
			tx.Rollback()
			http.Error(w, "failed to save modifiers", http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to save", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"id": subID})
	}
}

func deleteSubclassHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to start transaction", http.StatusInternalServerError)
			return
		}
		if _, err := tx.Exec(`DELETE FROM stat_modifiers WHERE source_type = 'subclass' AND source_id = ?`, id); err != nil {
			tx.Rollback()
			http.Error(w, "failed to delete modifiers", http.StatusInternalServerError)
			return
		}
		if _, err := tx.Exec(`UPDATE spells SET source_type = NULL, source_id = NULL WHERE source_type = 'subclass' AND source_id = ?`, id); err != nil {
			tx.Rollback()
			http.Error(w, "failed to clear spell sources", http.StatusInternalServerError)
			return
		}
		if _, err := tx.Exec(`DELETE FROM subclass_classes WHERE subclass_id = ?`, id); err != nil {
			tx.Rollback()
			http.Error(w, "failed to delete class links", http.StatusInternalServerError)
			return
		}
		if _, err := tx.Exec(`DELETE FROM subclasses WHERE id = ?`, id); err != nil {
			tx.Rollback()
			http.Error(w, "failed to delete subclass", http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to delete", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"deleted": id})
	}
}
