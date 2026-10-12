package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

// The blood types offered by the character's blood type picker. Each story
// keeps its own list (Settings → General), so a world can have types of its
// own. A note left empty on a built-in type shows the app's translated note
// about it (frontend/src/bloodTypes.js).
const settingBloodTypes = "blood_types"

const (
	maxBloodTypes    = 64
	maxBloodTypeName = 24
	maxBloodTypeNote = 600
)

type bloodType struct {
	Name string `json:"name"`
	Note string `json:"note"`
}

func defaultBloodTypes() []bloodType {
	out := []bloodType{}
	for _, n := range []string{"A+", "A-", "B+", "B-", "AB+", "AB-", "O+", "O-"} {
		out = append(out, bloodType{Name: n})
	}
	return out
}

func loadBloodTypes(db *sql.DB) []bloodType {
	var raw string
	if db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, settingBloodTypes).Scan(&raw) == nil {
		var list []bloodType
		if json.Unmarshal([]byte(raw), &list) == nil && list != nil {
			return list
		}
	}
	return defaultBloodTypes()
}

// cleanBloodTypes trims a list and refuses empty, repeated or overlong names.
func cleanBloodTypes(in []bloodType) ([]bloodType, string) {
	if len(in) > maxBloodTypes {
		return nil, "a story can have at most 64 blood types"
	}
	out := []bloodType{}
	seen := map[string]bool{}
	for _, b := range in {
		name := strings.TrimSpace(b.Name)
		note := strings.TrimSpace(b.Note)
		switch {
		case name == "":
			return nil, "every blood type needs a name"
		case len([]rune(name)) > maxBloodTypeName:
			return nil, "a blood type's name can be at most 24 characters"
		case len([]rune(note)) > maxBloodTypeNote:
			return nil, "a blood type's note can be at most 600 characters"
		case seen[strings.ToLower(name)]:
			return nil, "a blood type is listed twice"
		}
		seen[strings.ToLower(name)] = true
		out = append(out, bloodType{Name: name, Note: note})
	}
	return out, ""
}

func updateBloodTypesHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in []bloodType
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<17)).Decode(&in); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		list, msg := cleanBloodTypes(in)
		if msg != "" {
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		raw, _ := json.Marshal(list)
		if _, err := db.Exec(`INSERT INTO app_settings (key, value) VALUES (?, ?)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value`, settingBloodTypes, string(raw)); err != nil {
			log.Printf("save blood types: %v", err)
			http.Error(w, "failed to save the blood types", http.StatusInternalServerError)
			return
		}
		writeJSON(w, loadBloodTypes(db))
	}
}
