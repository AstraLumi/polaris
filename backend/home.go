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

// looseEach is how many loose ends of one kind Home lists.
const looseEach = 3

// homeLooseEnd is a gentle nudge about data that will matter once the
// timeline exists: a thing that should have a date or a link and doesn't.
type homeLooseEnd struct {
	Kind  string `json:"kind"` // "character" | "location" | "event" | "chapter" | "lore"
	ID    int64  `json:"id"`   // the character's, location's, event's, chapter's or article's id
	Name  string `json:"name"`
	Issue string `json:"issue"`
	// For a character, the current version, when the fix is on its form.
	VersionID int64 `json:"version_id,omitempty"`
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
	// How many more there are beyond the few of each kind listed (at most
	// 20 counted per kind).
	LooseMore int         `json:"loose_more"`
	Events    []homeEvent `json:"events"`
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
			{&p.Counts.Kingdoms, `SELECT COUNT(*) FROM locations WHERE kind = 'major' AND color IS NOT NULL AND realm = 'kingdom'`},
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
		// in-story birth date, an unmatched birthplace/nation or no height
		// or body type, events with no date, chapters with nothing in them
		// and lore pages never written. A few of each, so one kind can't
		// crowd out the rest. A query with a third column gives the
		// character's current version.
		type spec struct {
			kind, issue, q string
		}
		const currentChar = `FROM characters c JOIN character_versions v ON v.character_id = c.id AND v.is_current = 1`
		for _, s := range []spec{
			{"location", "No founding date", `SELECT id, name FROM locations WHERE kind = 'major' AND color IS NOT NULL AND realm = 'kingdom' AND (founding_date IS NULL OR founding_date = '') ORDER BY name COLLATE NOCASE LIMIT 20`},
			{"character", "No in-story birth date", `SELECT c.id, v.name FROM characters c JOIN character_versions v ON v.character_id = c.id AND v.is_current = 1 WHERE c.birth_date IS NULL OR c.birth_date = '' ORDER BY v.name COLLATE NOCASE LIMIT 20`},
			{"character", "Birthplace needs a map location", `SELECT c.id, v.name FROM characters c JOIN character_versions v ON v.character_id = c.id AND v.is_current = 1 WHERE c.born_in_location_id IS NULL AND c.born_in IS NOT NULL AND c.born_in != '' ORDER BY v.name COLLATE NOCASE LIMIT 20`},
			{"character", "Nation needs a kingdom", `SELECT c.id, v.name FROM characters c JOIN character_versions v ON v.character_id = c.id AND v.is_current = 1 JOIN character_story st ON st.version_id = v.id WHERE st.nation_location_id IS NULL AND st.nation IS NOT NULL AND st.nation != '' ORDER BY v.name COLLATE NOCASE LIMIT 20`},
			{"character", "No height or body type", `SELECT c.id, v.name, v.id ` + currentChar + ` LEFT JOIN character_story st ON st.version_id = v.id WHERE st.height IS NULL OR st.body_type_id IS NULL ORDER BY v.name COLLATE NOCASE LIMIT 20`},
			{"event", "No date", `SELECT id, name FROM events WHERE event_date IS NULL OR event_date = '' ORDER BY name COLLATE NOCASE LIMIT 20`},
			{"chapter", "No events, versions or notes", `SELECT ch.id, ch.title FROM chapters ch WHERE TRIM(ch.notes) = '' AND NOT EXISTS (SELECT 1 FROM events e WHERE e.chapter_id = ch.id) AND NOT EXISTS (SELECT 1 FROM character_versions cv WHERE cv.chapter_id = ch.id) ORDER BY ch.position, ch.id LIMIT 20`},
			{"lore", "Not written yet", `SELECT l.id, l.name FROM lore_articles l WHERE NOT EXISTS (SELECT 1 FROM wiki_entries e WHERE e.entity_type = 'lore' AND e.entity_id = l.id AND (TRIM(COALESCE(e.summary, '')) != '' OR EXISTS (SELECT 1 FROM wiki_sections s WHERE s.entry_id = e.id AND TRIM(s.body) != ''))) ORDER BY l.name COLLATE NOCASE LIMIT 20`},
		} {
			lr, err := db.Query(s.q)
			if err != nil {
				log.Printf("home loose ends (%s): %v", s.issue, err)
				continue
			}
			cols, _ := lr.Columns()
			n := 0
			for lr.Next() {
				l := homeLooseEnd{Kind: s.kind, Issue: s.issue}
				dest := []any{&l.ID, &l.Name}
				if len(cols) > 2 {
					dest = append(dest, &l.VersionID)
				}
				if lr.Scan(dest...) == nil {
					if n < looseEach {
						p.Loose = append(p.Loose, l)
					}
					n++
				}
			}
			lr.Close()
			if n > looseEach {
				p.LooseMore += n - looseEach
			}
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
