package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// sharedCharacterInput is what belongs to the character rather than to one
// of its versions: birth date, blood type, birthplace and tags. The version
// list page edits them, above the versions.
type sharedCharacterInput struct {
	BirthDate        string   `json:"birth_date"`
	BloodType        string   `json:"blood_type"`
	BornIn           string   `json:"born_in"`
	BornInLocationID *int64   `json:"born_in_location_id"`
	Tags             []string `json:"tags"`
}

func updateSharedCharacterHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		characterID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "character not found", http.StatusNotFound)
			return
		}
		var exists int
		if db.QueryRow(`SELECT 1 FROM characters WHERE id = ?`, characterID).Scan(&exists) != nil {
			http.Error(w, "character not found", http.StatusNotFound)
			return
		}
		var in sharedCharacterInput
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if msg := checkDateFits(loadCalendar(db), "Birth date", in.BirthDate); msg != "" {
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		blood := strings.TrimSpace(in.BloodType)
		if len([]rune(blood)) > maxBloodTypeName {
			http.Error(w, "the blood type is too long", http.StatusBadRequest)
			return
		}
		tags, msg := cleanTags(in.Tags)
		if msg != "" {
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		rawLoc := ""
		if in.BornInLocationID != nil {
			rawLoc = strconv.FormatInt(*in.BornInLocationID, 10)
		}
		bornInLoc := parseLocationID(db, rawLoc, false)

		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to save changes", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()
		if _, err := tx.Exec(`
			UPDATE characters SET birth_date = ?, blood_type = ?, born_in = ?, born_in_location_id = ?
			WHERE id = ?
		`,
			nullableString(normalizeStoryDate(in.BirthDate)),
			nullableString(blood),
			nullableString(unlinkedText(in.BornIn, bornInLoc)), bornInLoc,
			characterID,
		); err != nil {
			log.Printf("update shared character: %v", err)
			http.Error(w, "failed to save changes", http.StatusInternalServerError)
			return
		}
		if err := replaceCharacterTags(tx, characterID, tags); err != nil {
			log.Printf("update character tags: %v", err)
			http.Error(w, "failed to save changes", http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to save changes", http.StatusInternalServerError)
			return
		}

		var currentID int64
		if err := db.QueryRow(`SELECT id FROM character_versions WHERE character_id = ? AND is_current = 1`, characterID).Scan(&currentID); err != nil {
			http.Error(w, "saved, but failed to reload the character", http.StatusInternalServerError)
			return
		}
		detail, err := buildVersionDetail(db, strconv.FormatInt(currentID, 10))
		if err != nil {
			http.Error(w, "saved, but failed to reload the character", http.StatusInternalServerError)
			return
		}
		writeJSON(w, detail)
	}
}
