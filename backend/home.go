package main

import (
	"database/sql"
	"log"
	"net/http"
)

type homeCounts struct {
	Characters int `json:"characters"`
	Versions   int `json:"versions"`
	Kingdoms   int `json:"kingdoms"`
	Locations  int `json:"locations"` // every location, major and minor
	Spells     int `json:"spells"`
	Events     int `json:"events"`
}

type homeRecent struct {
	CharacterID int64  `json:"character_id"`
	Name        string `json:"name"`
	Level       int    `json:"level"`
	ClassName   string `json:"class_name"`
	PicturePath string `json:"picture_path"`
	UpdatedAt   string `json:"updated_at"`
}

// homeLooseEnd is a gentle nudge about data that will matter once the
// timeline exists: a thing that should have a date or a link and doesn't.
type homeLooseEnd struct {
	Kind  string `json:"kind"` // "character" | "location"
	ID    int64  `json:"id"`   // character id, or location id
	Name  string `json:"name"`
	Issue string `json:"issue"`
}

type homeEvent struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	EventDate string `json:"event_date"`
	CreatedAt string `json:"created_at"`
}

type homePayload struct {
	Counts   homeCounts       `json:"counts"`
	Calendar calendarSettings `json:"calendar"`
	Recent   []homeRecent     `json:"recent"`
	Loose    []homeLooseEnd   `json:"loose_ends"`
	Events   []homeEvent      `json:"events"`
}

func homeHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := homePayload{Calendar: loadCalendar(db), Recent: []homeRecent{}, Loose: []homeLooseEnd{}, Events: []homeEvent{}}

		for _, c := range []struct {
			dest *int
			q    string
		}{
			{&p.Counts.Characters, `SELECT COUNT(*) FROM characters`},
			{&p.Counts.Versions, `SELECT COUNT(*) FROM character_versions`},
			{&p.Counts.Kingdoms, `SELECT COUNT(*) FROM locations WHERE kind = 'major' AND color IS NOT NULL`},
			{&p.Counts.Locations, `SELECT COUNT(*) FROM locations`},
			{&p.Counts.Spells, `SELECT COUNT(*) FROM spells`},
			{&p.Counts.Events, `SELECT COUNT(*) FROM events`},
		} {
			if err := db.QueryRow(c.q).Scan(c.dest); err != nil {
				http.Error(w, "failed to load home", http.StatusInternalServerError)
				log.Printf("home count: %v", err)
				return
			}
		}

		// Most recently touched characters, by their current version.
		rows, err := db.Query(`
			SELECT c.id, v.name, v.level, COALESCE(cl.name, ''), COALESCE(v.picture_path, ''), v.updated_at
			FROM characters c
			JOIN character_versions v ON v.character_id = c.id AND v.is_current = 1
			LEFT JOIN classes cl ON cl.id = v.class_id
			ORDER BY v.updated_at DESC LIMIT 6`)
		if err != nil {
			http.Error(w, "failed to load home", http.StatusInternalServerError)
			log.Printf("home recent: %v", err)
			return
		}
		for rows.Next() {
			var h homeRecent
			if err := rows.Scan(&h.CharacterID, &h.Name, &h.Level, &h.ClassName, &h.PicturePath, &h.UpdatedAt); err != nil {
				rows.Close()
				http.Error(w, "failed to read home", http.StatusInternalServerError)
				return
			}
			if h.PicturePath != "" {
				h.PicturePath = "/uploads/" + h.PicturePath
			}
			p.Recent = append(p.Recent, h)
		}
		rows.Close()

		// Loose ends: kingdoms with no founding date, characters with no
		// in-story birth date or an unmatched birthplace/nation.
		type spec struct {
			kind, issue, q string
		}
		for _, s := range []spec{
			{"location", "No founding date", `SELECT id, name FROM locations WHERE kind = 'major' AND color IS NOT NULL AND (founding_date IS NULL OR founding_date = '') ORDER BY name COLLATE NOCASE LIMIT 20`},
			{"character", "No in-story birth date", `SELECT c.id, v.name FROM characters c JOIN character_versions v ON v.character_id = c.id AND v.is_current = 1 LEFT JOIN character_story st ON st.version_id = v.id WHERE st.birth_date IS NULL OR st.birth_date = '' ORDER BY v.name COLLATE NOCASE LIMIT 20`},
			{"character", "Birthplace needs a map location", `SELECT c.id, v.name FROM characters c JOIN character_versions v ON v.character_id = c.id AND v.is_current = 1 JOIN character_story st ON st.version_id = v.id WHERE st.born_in_location_id IS NULL AND st.born_in IS NOT NULL AND st.born_in != '' ORDER BY v.name COLLATE NOCASE LIMIT 20`},
			{"character", "Nation needs a kingdom", `SELECT c.id, v.name FROM characters c JOIN character_versions v ON v.character_id = c.id AND v.is_current = 1 JOIN character_story st ON st.version_id = v.id WHERE st.nation_location_id IS NULL AND st.nation IS NOT NULL AND st.nation != '' ORDER BY v.name COLLATE NOCASE LIMIT 20`},
		} {
			lr, err := db.Query(s.q)
			if err != nil {
				log.Printf("home loose ends (%s): %v", s.issue, err)
				continue
			}
			for lr.Next() {
				l := homeLooseEnd{Kind: s.kind, Issue: s.issue}
				if lr.Scan(&l.ID, &l.Name) == nil {
					p.Loose = append(p.Loose, l)
				}
			}
			lr.Close()
		}
		if len(p.Loose) > 12 {
			p.Loose = p.Loose[:12]
		}

		// The newest events, by when they were added (not their in-story date).
		er, err := db.Query(`SELECT id, name, event_date, created_at FROM events ORDER BY created_at DESC, id DESC LIMIT 6`)
		if err != nil {
			http.Error(w, "failed to load home", http.StatusInternalServerError)
			log.Printf("home events: %v", err)
			return
		}
		for er.Next() {
			var e homeEvent
			if er.Scan(&e.ID, &e.Name, &e.EventDate, &e.CreatedAt) == nil {
				p.Events = append(p.Events, e)
			}
		}
		er.Close()

		writeJSON(w, p)
	}
}
