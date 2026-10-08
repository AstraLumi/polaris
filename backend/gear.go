package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Gear is a Character Assets type like spells: you define "an Iron Glove"
// once, then equip it on a character version.
//
//   - A gear piece has a slot TYPE (glove, ring, mainhand ...): what it is.
//   - A character version has equipment SLOTS (glove1, glove2, ring1 ...):
//     where things are worn. Two glove slots both take a glove.
//   - Its stat bonuses are ordinary stat_modifiers rows with source_type
//     'item'. They are always flat, and may target any stat.

// The slot types a piece of gear can be, in display order.
var gearSlotTypes = []string{
	"helmet", "face", "necklace", "cape", "torso", "glove", "ring", "pants", "boots", "mainhand", "offhand",
}

// The equipment slots on a character, with the slot type each accepts.
var gearEquipSlots = []struct{ key, typ string }{
	{"helmet", "helmet"}, {"face", "face"}, {"necklace", "necklace"}, {"cape", "cape"},
	{"torso", "torso"}, {"glove1", "glove"}, {"glove2", "glove"}, {"ring1", "ring"}, {"ring2", "ring"},
	{"pants", "pants"}, {"boots", "boots"}, {"mainhand", "mainhand"}, {"offhand", "offhand"},
}

func isGearSlotType(s string) bool {
	for _, t := range gearSlotTypes {
		if t == s {
			return true
		}
	}
	return false
}

func slotTypeFor(equipSlot string) (string, bool) {
	for _, s := range gearEquipSlots {
		if s.key == equipSlot {
			return s.typ, true
		}
	}
	return "", false
}

const gearTableSQL = `CREATE TABLE gear (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	name       TEXT NOT NULL UNIQUE,
	weight     REAL NOT NULL DEFAULT 0,
	slot       TEXT NOT NULL,
	icon_path  TEXT,
	created_at TEXT NOT NULL DEFAULT (datetime('now'))
)`

// Which gear a version has equipped, per slot. Per-version like spells, so a
// character's outfit can change over the story.
const characterGearTableSQL = `CREATE TABLE character_gear (
	version_id INTEGER NOT NULL REFERENCES character_versions(id) ON DELETE CASCADE,
	slot       TEXT NOT NULL,
	gear_id    INTEGER NOT NULL REFERENCES gear(id) ON DELETE CASCADE,
	PRIMARY KEY (version_id, slot)
)`

type gearDetail struct {
	ID        int64              `json:"id"`
	Name      string             `json:"name"`
	Weight    float64            `json:"weight"`
	Slot      string             `json:"slot"`
	IconPath  string             `json:"icon_path"`
	Modifiers map[string]float64 `json:"modifiers"`
}

// queryGear loads gear (optionally filtered by a WHERE suffix) together with
// each piece's modifiers.
func queryGear(db *sql.DB, suffix string, args ...any) ([]gearDetail, error) {
	rows, err := db.Query(
		`SELECT g.id, g.name, g.weight, g.slot, COALESCE(g.icon_path, '') FROM gear g `+suffix, args...)
	if err != nil {
		return nil, err
	}
	var list []gearDetail
	byID := map[int64]*gearDetail{}
	for rows.Next() {
		var g gearDetail
		if err := rows.Scan(&g.ID, &g.Name, &g.Weight, &g.Slot, &g.IconPath); err != nil {
			rows.Close()
			return nil, err
		}
		if !isBuiltinIcon(g.IconPath) {
			g.IconPath = uploadURL(g.IconPath)
		}
		g.Modifiers = map[string]float64{}
		list = append(list, g)
	}
	rows.Close()
	for i := range list {
		byID[list[i].ID] = &list[i]
	}

	mods, err := db.Query(`SELECT source_id, target_stat, value FROM stat_modifiers WHERE source_type = 'item'`)
	if err != nil {
		return nil, err
	}
	defer mods.Close()
	for mods.Next() {
		var id int64
		var key string
		var v float64
		if err := mods.Scan(&id, &key, &v); err != nil {
			return nil, err
		}
		if g, ok := byID[id]; ok {
			g.Modifiers[key] = v
		}
	}
	if list == nil {
		list = []gearDetail{}
	}
	return list, mods.Err()
}

const gearOrder = `ORDER BY g.name COLLATE NOCASE`

// ---- Equipped gear on a version -------------------------------------------

// loadVersionGear returns what a version has equipped, keyed by equipment slot.
func loadVersionGear(db *sql.DB, versionID int64) (map[string]gearDetail, error) {
	out := map[string]gearDetail{}
	rows, err := db.Query(`SELECT slot, gear_id FROM character_gear WHERE version_id = ?`, versionID)
	if err != nil {
		return nil, err
	}
	slots := map[string]int64{}
	for rows.Next() {
		var slot string
		var id int64
		if err := rows.Scan(&slot, &id); err != nil {
			rows.Close()
			return nil, err
		}
		slots[slot] = id
	}
	rows.Close()
	if len(slots) == 0 {
		return out, nil
	}
	all, err := queryGear(db, gearOrder)
	if err != nil {
		return nil, err
	}
	byID := map[int64]gearDetail{}
	for _, g := range all {
		byID[g.ID] = g
	}
	for slot, id := range slots {
		if g, ok := byID[id]; ok {
			out[slot] = g
		}
	}
	return out, nil
}

// replaceVersionGear sets exactly what a version has equipped. A slot that
// doesn't exist, a gear id that doesn't exist, or a piece of the wrong type
// for the slot is skipped rather than failing the whole save.
func replaceVersionGear(tx *sql.Tx, versionID int64, equipped map[string]int64) error {
	if _, err := tx.Exec(`DELETE FROM character_gear WHERE version_id = ?`, versionID); err != nil {
		return err
	}
	for slot, id := range equipped {
		typ, ok := slotTypeFor(slot)
		if !ok {
			continue
		}
		if _, err := tx.Exec(
			`INSERT INTO character_gear (version_id, slot, gear_id)
			 SELECT ?, ?, id FROM gear WHERE id = ? AND slot = ?`,
			versionID, slot, id, typ,
		); err != nil {
			return err
		}
	}
	return nil
}

// ---- Handlers ---------------------------------------------------------------

func listGearHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := queryGear(db, gearOrder)
		if err != nil {
			http.Error(w, "failed to load gear", http.StatusInternalServerError)
			log.Printf("list gear: %v", err)
			return
		}
		writeJSON(w, list)
	}
}

func getGearHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := queryGear(db, `WHERE g.id = ?`, r.PathValue("id"))
		if err != nil || len(list) == 0 {
			http.Error(w, "gear not found", http.StatusNotFound)
			return
		}
		writeJSON(w, list[0])
	}
}

type gearInput struct {
	name      string
	weight    float64
	slot      string
	modifiers map[string]float64
}

func parseGearForm(r *http.Request) (gearInput, error) {
	var in gearInput
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		return in, fmt.Errorf("invalid form data")
	}
	in.name = strings.TrimSpace(r.FormValue("name"))
	if in.name == "" {
		return in, fmt.Errorf("name is required")
	}
	in.slot = strings.TrimSpace(r.FormValue("slot"))
	if !isGearSlotType(in.slot) {
		return in, fmt.Errorf("choose a slot for this gear")
	}
	in.weight = roundTo2(parseFloatDefault(r.FormValue("weight"), 0))
	if in.weight < 0 {
		return in, fmt.Errorf("weight can't be negative")
	}
	if in.weight > 1e6 {
		return in, fmt.Errorf("weight is too large")
	}
	in.modifiers = map[string]float64{}
	if raw := strings.TrimSpace(r.FormValue("modifiers")); raw != "" {
		if err := json.Unmarshal([]byte(raw), &in.modifiers); err != nil {
			return in, fmt.Errorf("invalid stat bonuses")
		}
	}
	return in, nil
}

func attachGearIcon(db *sql.DB, uploadsDir string, r *http.Request, gearID int64, oldPath string) {
	newPath := ""
	changed := false
	if file, header, err := r.FormFile("icon"); err == nil {
		defer file.Close()
		stem := fmt.Sprintf("%d-%d", gearID, time.Now().Unix())
		relPath, err := saveUpload(uploadsDir, "gear", stem, header.Filename, file)
		if err != nil {
			log.Printf("save gear icon: %v", err)
			return
		}
		newPath, changed = relPath, true
	} else if choice := parseIconChoiceFrom(r.FormValue("icon_choice"), builtinGearIcons); choice.change {
		newPath, changed = choice.value, true
	}
	if !changed {
		return
	}
	var stored any = newPath
	if newPath == "" {
		stored = nil
	}
	if _, err := db.Exec(`UPDATE gear SET icon_path = ? WHERE id = ?`, stored, gearID); err != nil {
		log.Printf("update gear icon_path: %v", err)
		return
	}
	if oldPath != "" && oldPath != newPath {
		removePicture(uploadsDir, oldPath)
	}
}

func createGearHandler(db *sql.DB, uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		in, err := parseGearForm(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to start transaction", http.StatusInternalServerError)
			return
		}
		res, err := tx.Exec(`INSERT INTO gear (name, weight, slot) VALUES (?, ?, ?)`, in.name, in.weight, in.slot)
		if err != nil {
			tx.Rollback()
			http.Error(w, "a gear piece with that name may already exist", http.StatusBadRequest)
			return
		}
		id, _ := res.LastInsertId()
		if err := saveModifiers(tx, "item", id, in.modifiers); err != nil {
			tx.Rollback()
			http.Error(w, "failed to save stat bonuses", http.StatusInternalServerError)
			log.Printf("save gear modifiers: %v", err)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to save gear", http.StatusInternalServerError)
			return
		}
		attachGearIcon(db, uploadsDir, r, id, "")
		list, err := queryGear(db, `WHERE g.id = ?`, id)
		if err != nil || len(list) == 0 {
			writeJSON(w, map[string]any{"id": id})
			return
		}
		writeJSON(w, list[0])
	}
}

func updateGearHandler(db *sql.DB, uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "gear not found", http.StatusNotFound)
			return
		}
		var oldIcon sql.NullString
		var oldSlot string
		if err := db.QueryRow(`SELECT icon_path, slot FROM gear WHERE id = ?`, id).Scan(&oldIcon, &oldSlot); err != nil {
			http.Error(w, "gear not found", http.StatusNotFound)
			return
		}
		in, err := parseGearForm(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to start transaction", http.StatusInternalServerError)
			return
		}
		if _, err := tx.Exec(`UPDATE gear SET name = ?, weight = ?, slot = ? WHERE id = ?`, in.name, in.weight, in.slot, id); err != nil {
			tx.Rollback()
			http.Error(w, "a gear piece with that name may already exist", http.StatusBadRequest)
			return
		}
		// Changing what kind of gear it is un-equips it where it no longer fits.
		if in.slot != oldSlot {
			for _, s := range gearEquipSlots {
				if s.typ != in.slot {
					if _, err := tx.Exec(`DELETE FROM character_gear WHERE gear_id = ? AND slot = ?`, id, s.key); err != nil {
						tx.Rollback()
						http.Error(w, "failed to update gear", http.StatusInternalServerError)
						return
					}
				}
			}
		}
		if err := saveModifiers(tx, "item", id, in.modifiers); err != nil {
			tx.Rollback()
			http.Error(w, "failed to save stat bonuses", http.StatusInternalServerError)
			log.Printf("save gear modifiers: %v", err)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to save gear", http.StatusInternalServerError)
			return
		}
		attachGearIcon(db, uploadsDir, r, id, oldIcon.String)
		list, err := queryGear(db, `WHERE g.id = ?`, id)
		if err != nil || len(list) == 0 {
			writeJSON(w, map[string]any{"id": id})
			return
		}
		writeJSON(w, list[0])
	}
}

func deleteGearHandler(db *sql.DB, uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "gear not found", http.StatusNotFound)
			return
		}
		var icon sql.NullString
		db.QueryRow(`SELECT icon_path FROM gear WHERE id = ?`, id).Scan(&icon)

		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to start transaction", http.StatusInternalServerError)
			return
		}
		// Explicit, so no character keeps a link or a bonus from gear that's gone.
		for _, q := range []string{
			`DELETE FROM character_gear WHERE gear_id = ?`,
			`DELETE FROM stat_modifiers WHERE source_type = 'item' AND source_id = ?`,
			`DELETE FROM gear WHERE id = ?`,
		} {
			if _, err := tx.Exec(q, id); err != nil {
				tx.Rollback()
				http.Error(w, "failed to delete gear", http.StatusInternalServerError)
				log.Printf("delete gear: %v", err)
				return
			}
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to delete gear", http.StatusInternalServerError)
			return
		}
		if icon.Valid {
			removePicture(uploadsDir, icon.String)
		}
		writeJSON(w, map[string]any{"deleted": id})
	}
}
