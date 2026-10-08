package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	maxTagsPerEvent = 30
	maxTagLength    = 40
)

type eventPerson struct {
	ID          int64  `json:"id"` // character id
	Name        string `json:"name"`
	PicturePath string `json:"picture_path"`
}

// eventSource says what a source-linked event is attached to. The name is
// the display name of the source ("Aldoria"), not the event's.
type eventSource struct {
	Type string `json:"type"` // "character" | "location"
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type eventDetail struct {
	ID           int64         `json:"id"`
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	EventDate    string        `json:"event_date"`
	LocationID   *int64        `json:"location_id"`
	LocationName string        `json:"location_name"`
	PicturePath  string        `json:"picture_path"`
	Tags         []string      `json:"tags"`
	People       []eventPerson `json:"people"`
	Source       *eventSource  `json:"source"`
	CreatedAt    string        `json:"created_at"`
	UpdatedAt    string        `json:"updated_at"`
}

type eventSummary struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	EventDate    string   `json:"event_date"`
	LocationName string   `json:"location_name"`
	PicturePath  string   `json:"picture_path"`
	Tags         []string `json:"tags"`
	PeopleCount  int      `json:"people_count"`
	SourceType   string   `json:"source_type"`
}

// ---- Sources (character births, location foundings) -------------------------

// resolveSource reads the live name and date of whatever a source-linked
// event is attached to. ok is false when the source no longer exists.
func resolveSource(db *sql.DB, sourceType string, sourceID int64) (src eventSource, date string, ok bool) {
	src = eventSource{Type: sourceType, ID: sourceID}
	switch sourceType {
	case "character":
		var name, birth string
		err := db.QueryRow(`
			SELECT v.name, COALESCE(st.birth_date, '')
			FROM character_versions v
			LEFT JOIN character_story st ON st.version_id = v.id
			WHERE v.character_id = ? AND v.is_current = 1`, sourceID).Scan(&name, &birth)
		if err != nil {
			return src, "", false
		}
		src.Name = name
		return src, normalizeStoryDate(birth), true
	case "location":
		var name, founded string
		err := db.QueryRow(
			`SELECT name, COALESCE(founding_date, '') FROM locations WHERE id = ?`, sourceID,
		).Scan(&name, &founded)
		if err != nil {
			return src, "", false
		}
		src.Name = name
		return src, normalizeStoryDate(founded), true
	}
	return src, "", false
}

func sourceEventName(src eventSource) string {
	if src.Type == "character" {
		return "Birth of " + src.Name
	}
	return "Founding of " + src.Name
}

// deleteSourceEvents removes the details records attached to deleted
// characters or locations, plus their pictures. Called before the source
// rows themselves go, so nothing is left pointing at a ghost.
func deleteSourceEvents(db *sql.DB, uploadsDir, sourceType string, ids []int64) {
	for _, id := range ids {
		var eventID int64
		var pic sql.NullString
		err := db.QueryRow(
			`SELECT id, picture_path FROM events WHERE source_type = ? AND source_id = ?`, sourceType, id,
		).Scan(&eventID, &pic)
		if err != nil {
			continue
		}
		if pic.Valid {
			removePicture(uploadsDir, pic.String)
		}
		deleteEventRows(db, eventID)
	}
}

// deleteEventRows clears an event and its tags/people explicitly rather
// than leaning on foreign-key cascades (which are per-connection in SQLite).
func deleteEventRows(db *sql.DB, eventID int64) error {
	for _, stmt := range []string{
		`DELETE FROM event_tags WHERE event_id = ?`,
		`DELETE FROM event_characters WHERE event_id = ?`,
		`DELETE FROM events WHERE id = ?`,
	} {
		if _, err := db.Exec(stmt, eventID); err != nil {
			return err
		}
	}
	return nil
}

// ---- Loading ----------------------------------------------------------------

func loadEventTags(db *sql.DB, eventID int64) []string {
	tags := []string{}
	rows, err := db.Query(`SELECT tag FROM event_tags WHERE event_id = ? ORDER BY tag COLLATE NOCASE`, eventID)
	if err != nil {
		return tags
	}
	defer rows.Close()
	for rows.Next() {
		var t string
		if rows.Scan(&t) == nil {
			tags = append(tags, t)
		}
	}
	return tags
}

func loadEventPeople(db *sql.DB, eventID int64) []eventPerson {
	people := []eventPerson{}
	rows, err := db.Query(`
		SELECT c.id, v.name, COALESCE(v.picture_path, '')
		FROM event_characters ec
		JOIN characters c ON c.id = ec.character_id
		JOIN character_versions v ON v.character_id = c.id AND v.is_current = 1
		WHERE ec.event_id = ?
		ORDER BY v.name COLLATE NOCASE`, eventID)
	if err != nil {
		return people
	}
	defer rows.Close()
	for rows.Next() {
		var p eventPerson
		if rows.Scan(&p.ID, &p.Name, &p.PicturePath) == nil {
			if p.PicturePath != "" {
				p.PicturePath = "/uploads/" + p.PicturePath
			}
			people = append(people, p)
		}
	}
	return people
}

const eventSelect = `
	SELECT e.id, e.name, COALESCE(e.description, ''), COALESCE(e.event_date, ''),
	       e.location_id, COALESCE(l.name, ''), COALESCE(e.picture_path, ''),
	       COALESCE(e.source_type, ''), COALESCE(e.source_id, 0), e.created_at, e.updated_at
	FROM events e
	LEFT JOIN locations l ON l.id = e.location_id
`

type eventRow struct {
	d          eventDetail
	sourceType string
	sourceID   int64
}

func scanEventRow(s rowScanner) (eventRow, error) {
	var r eventRow
	var loc sql.NullInt64
	if err := s.Scan(
		&r.d.ID, &r.d.Name, &r.d.Description, &r.d.EventDate, &loc, &r.d.LocationName,
		&r.d.PicturePath, &r.sourceType, &r.sourceID, &r.d.CreatedAt, &r.d.UpdatedAt,
	); err != nil {
		return r, err
	}
	if loc.Valid {
		v := loc.Int64
		r.d.LocationID = &v
	}
	if r.d.PicturePath != "" {
		r.d.PicturePath = "/uploads/" + r.d.PicturePath
	}
	r.d.EventDate = normalizeStoryDate(r.d.EventDate)
	r.d.Tags = []string{}
	r.d.People = []eventPerson{}
	return r, nil
}

// applySource overwrites a linked event's name and date with the live
// values from its source. Returns false when the source is gone.
func (r *eventRow) applySource(db *sql.DB) bool {
	if r.sourceType == "" {
		return true
	}
	src, date, ok := resolveSource(db, r.sourceType, r.sourceID)
	if !ok {
		return false
	}
	r.d.Source = &src
	r.d.Name = sourceEventName(src)
	r.d.EventDate = date
	return true
}

func loadEvent(db *sql.DB, id int64) (*eventDetail, error) {
	r, err := scanEventRow(db.QueryRow(eventSelect+` WHERE e.id = ?`, id))
	if err != nil {
		return nil, err
	}
	if !r.applySource(db) {
		return nil, sql.ErrNoRows
	}
	r.d.Tags = loadEventTags(db, id)
	r.d.People = loadEventPeople(db, id)
	return &r.d, nil
}

// dateKey orders story dates chronologically. Unreadable or missing dates
// sort last.
func dateKey(text string) (year, month, day int, ok bool) {
	d, parsed := parseStoryDate(text)
	if !parsed {
		return 0, 0, 0, false
	}
	return d.Year, d.Month, d.Day, true
}

func dateBefore(a, b string) bool {
	ay, am, ad, aok := dateKey(a)
	by, bm, bd, bok := dateKey(b)
	if aok != bok {
		return aok // readable dates come first
	}
	if !aok {
		return false
	}
	if ay != by {
		return ay < by
	}
	if am != bm {
		return am < bm
	}
	return ad < bd
}

// listEventsHandler returns events in chronological order. Optional
// filters: tag (exact, case-insensitive), character_id, location_id, and q
// (matches name or description).
func listEventsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := eventSelect + ` WHERE 1 = 1`
		var args []any
		if tag := strings.TrimSpace(r.URL.Query().Get("tag")); tag != "" {
			query += ` AND EXISTS (SELECT 1 FROM event_tags t WHERE t.event_id = e.id AND t.tag = ? COLLATE NOCASE)`
			args = append(args, tag)
		}
		if cid := strings.TrimSpace(r.URL.Query().Get("character_id")); cid != "" {
			query += ` AND EXISTS (SELECT 1 FROM event_characters ec WHERE ec.event_id = e.id AND ec.character_id = ?)`
			args = append(args, cid)
		}
		if lid := strings.TrimSpace(r.URL.Query().Get("location_id")); lid != "" {
			query += ` AND e.location_id = ?`
			args = append(args, lid)
		}

		rows, err := db.Query(query, args...)
		if err != nil {
			http.Error(w, "failed to load events", http.StatusInternalServerError)
			log.Printf("list events: %v", err)
			return
		}
		var all []eventRow
		for rows.Next() {
			er, err := scanEventRow(rows)
			if err != nil {
				rows.Close()
				http.Error(w, "failed to read events", http.StatusInternalServerError)
				log.Printf("list events scan: %v", err)
				return
			}
			all = append(all, er)
		}
		rows.Close()

		needle := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
		var kept []eventRow
		for _, er := range all {
			if !er.applySource(db) {
				continue
			}
			if needle != "" &&
				!strings.Contains(strings.ToLower(er.d.Name), needle) &&
				!strings.Contains(strings.ToLower(er.d.Description), needle) {
				continue
			}
			kept = append(kept, er)
		}
		sort.SliceStable(kept, func(i, j int) bool {
			if dateBefore(kept[i].d.EventDate, kept[j].d.EventDate) {
				return true
			}
			if dateBefore(kept[j].d.EventDate, kept[i].d.EventDate) {
				return false
			}
			return kept[i].d.ID < kept[j].d.ID
		})

		out := make([]eventSummary, 0, len(kept))
		for _, er := range kept {
			people := loadEventPeople(db, er.d.ID)
			out = append(out, eventSummary{
				ID: er.d.ID, Name: er.d.Name, EventDate: er.d.EventDate,
				LocationName: er.d.LocationName, PicturePath: er.d.PicturePath,
				Tags: loadEventTags(db, er.d.ID), PeopleCount: len(people), SourceType: er.sourceType,
			})
		}
		writeJSON(w, out)
	}
}

func getEventHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "event not found", http.StatusNotFound)
			return
		}
		d, err := loadEvent(db, id)
		if err != nil {
			http.Error(w, "event not found", http.StatusNotFound)
			return
		}
		writeJSON(w, d)
	}
}

// ---- Writing ----------------------------------------------------------------

// cleanTags trims, strips a leading #, collapses inner whitespace, drops
// blanks and case-insensitive duplicates, and enforces the length limits.
func cleanTags(raw []string) ([]string, string) {
	seen := map[string]bool{}
	out := []string{}
	for _, t := range raw {
		t = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(t), "#"))
		t = strings.Join(strings.Fields(t), " ")
		if t == "" {
			continue
		}
		if len([]rune(t)) > maxTagLength {
			return nil, fmt.Sprintf("tags can be at most %d characters", maxTagLength)
		}
		key := strings.ToLower(t)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, t)
	}
	if len(out) > maxTagsPerEvent {
		return nil, fmt.Sprintf("an event can have at most %d tags", maxTagsPerEvent)
	}
	return out, ""
}

// canonicalTag reuses the spelling an existing tag already has anywhere in
// the app, so "WarOfAsh" and "warofash" never become two tags.
func canonicalTag(tx *sql.Tx, tag string) string {
	var existing string
	if tx.QueryRow(`SELECT tag FROM event_tags WHERE tag = ? COLLATE NOCASE LIMIT 1`, tag).Scan(&existing) == nil {
		return existing
	}
	return tag
}

type eventForm struct {
	Name         string
	Description  string
	EventDate    string
	LocationID   sql.NullInt64
	Tags         []string
	CharacterIDs []int64
	HasTags      bool
	HasPeople    bool
}

// parseEventForm reads the multipart body shared by create and update.
// tags and character_ids are JSON arrays; when absent on an update the
// existing tags/people are left alone.
func parseEventForm(db *sql.DB, r *http.Request) (eventForm, string) {
	var f eventForm
	f.Name = strings.TrimSpace(r.FormValue("name"))
	f.Description = strings.TrimSpace(r.FormValue("description"))
	f.EventDate = normalizeStoryDate(r.FormValue("event_date"))
	f.LocationID = parseLocationID(db, r.FormValue("location_id"), false)

	if raw := r.Form["tags"]; len(raw) > 0 {
		var tags []string
		if err := json.Unmarshal([]byte(raw[0]), &tags); err != nil {
			return f, "invalid tag list"
		}
		cleaned, msg := cleanTags(tags)
		if msg != "" {
			return f, msg
		}
		f.Tags, f.HasTags = cleaned, true
	}
	if raw := r.Form["character_ids"]; len(raw) > 0 {
		if err := json.Unmarshal([]byte(raw[0]), &f.CharacterIDs); err != nil {
			return f, "invalid people list"
		}
		f.HasPeople = true
	}
	return f, ""
}

// validateEventDate: a new or edited free-standing event must sit on a
// real date, since the timeline has nowhere to put one that doesn't.
func validateEventDate(db *sql.DB, date string) string {
	if date == "" {
		return "a date is required"
	}
	if _, ok := parseStoryDate(date); !ok {
		return "the date must look like DD-MM-YYYY (or just a year)"
	}
	return checkDateFits(loadCalendar(db), "Date", date)
}

func replaceEventTags(tx *sql.Tx, eventID int64, tags []string) error {
	if _, err := tx.Exec(`DELETE FROM event_tags WHERE event_id = ?`, eventID); err != nil {
		return err
	}
	for _, t := range tags {
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO event_tags (event_id, tag) VALUES (?, ?)`, eventID, canonicalTag(tx, t),
		); err != nil {
			return err
		}
	}
	return nil
}

func replaceEventPeople(tx *sql.Tx, eventID int64, ids []int64) error {
	if _, err := tx.Exec(`DELETE FROM event_characters WHERE event_id = ?`, eventID); err != nil {
		return err
	}
	for _, id := range ids {
		// Only existing characters; unknown ids quietly drop out.
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO event_characters (event_id, character_id)
			 SELECT ?, id FROM characters WHERE id = ?`, eventID, id,
		); err != nil {
			return err
		}
	}
	return nil
}

// saveEventPicture replaces an event's picture. The file name carries a
// timestamp so a replaced image is never served stale from the browser
// cache, and the previous file is removed.
func saveEventPicture(db *sql.DB, r *http.Request, uploadsDir string, eventID int64) {
	file, header, err := r.FormFile("picture")
	if err != nil {
		return
	}
	defer file.Close()

	var old sql.NullString
	db.QueryRow(`SELECT picture_path FROM events WHERE id = ?`, eventID).Scan(&old)

	stem := fmt.Sprintf("%d-%d", eventID, time.Now().Unix())
	rel, err := saveUpload(uploadsDir, "events", stem, header.Filename, file)
	if err != nil {
		log.Printf("save event picture: %v", err)
		return
	}
	if _, err := db.Exec(`UPDATE events SET picture_path = ? WHERE id = ?`, rel, eventID); err != nil {
		log.Printf("update event picture_path: %v", err)
		return
	}
	if old.Valid && old.String != "" && old.String != rel {
		removePicture(uploadsDir, old.String)
	}
}

func createEventHandler(db *sql.DB, uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, "invalid form data", http.StatusBadRequest)
			return
		}
		f, msg := parseEventForm(db, r)
		if msg != "" {
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		if f.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		if msg := validateEventDate(db, f.EventDate); msg != "" {
			http.Error(w, msg, http.StatusBadRequest)
			return
		}

		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to start transaction", http.StatusInternalServerError)
			return
		}
		res, err := tx.Exec(
			`INSERT INTO events (name, description, event_date, location_id) VALUES (?, ?, ?, ?)`,
			f.Name, nullableString(f.Description), f.EventDate, f.LocationID,
		)
		if err != nil {
			tx.Rollback()
			log.Printf("insert event: %v", err)
			http.Error(w, "failed to create event", http.StatusInternalServerError)
			return
		}
		id, _ := res.LastInsertId()
		if err := replaceEventTags(tx, id, f.Tags); err != nil {
			tx.Rollback()
			log.Printf("event tags: %v", err)
			http.Error(w, "failed to save tags", http.StatusInternalServerError)
			return
		}
		if err := replaceEventPeople(tx, id, f.CharacterIDs); err != nil {
			tx.Rollback()
			log.Printf("event people: %v", err)
			http.Error(w, "failed to save people", http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to save event", http.StatusInternalServerError)
			return
		}

		saveEventPicture(db, r, uploadsDir, id)

		d, err := loadEvent(db, id)
		if err != nil {
			http.Error(w, "saved, but failed to reload event", http.StatusInternalServerError)
			return
		}
		writeJSON(w, d)
	}
}

func updateEventHandler(db *sql.DB, uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "event not found", http.StatusNotFound)
			return
		}
		var sourceType sql.NullString
		var oldPic sql.NullString
		if err := db.QueryRow(
			`SELECT source_type, picture_path FROM events WHERE id = ?`, id,
		).Scan(&sourceType, &oldPic); err != nil {
			http.Error(w, "event not found", http.StatusNotFound)
			return
		}
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, "invalid form data", http.StatusBadRequest)
			return
		}
		f, msg := parseEventForm(db, r)
		if msg != "" {
			http.Error(w, msg, http.StatusBadRequest)
			return
		}

		linked := sourceType.Valid
		if !linked {
			if f.Name == "" {
				http.Error(w, "name is required", http.StatusBadRequest)
				return
			}
			if msg := validateEventDate(db, f.EventDate); msg != "" {
				http.Error(w, msg, http.StatusBadRequest)
				return
			}
		}

		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to start transaction", http.StatusInternalServerError)
			return
		}
		// A linked event's name and date belong to its source, so only the
		// details are written; a free-standing event takes everything.
		if linked {
			_, err = tx.Exec(
				`UPDATE events SET description = ?, location_id = ?, updated_at = datetime('now') WHERE id = ?`,
				nullableString(f.Description), f.LocationID, id,
			)
		} else {
			_, err = tx.Exec(
				`UPDATE events SET name = ?, description = ?, event_date = ?, location_id = ?, updated_at = datetime('now') WHERE id = ?`,
				f.Name, nullableString(f.Description), f.EventDate, f.LocationID, id,
			)
		}
		if err != nil {
			tx.Rollback()
			log.Printf("update event: %v", err)
			http.Error(w, "failed to update event", http.StatusInternalServerError)
			return
		}
		if f.HasTags {
			if err := replaceEventTags(tx, id, f.Tags); err != nil {
				tx.Rollback()
				http.Error(w, "failed to save tags", http.StatusInternalServerError)
				return
			}
		}
		if f.HasPeople {
			if err := replaceEventPeople(tx, id, f.CharacterIDs); err != nil {
				tx.Rollback()
				http.Error(w, "failed to save people", http.StatusInternalServerError)
				return
			}
		}
		if r.FormValue("remove_picture") == "1" && oldPic.Valid {
			if _, err := tx.Exec(`UPDATE events SET picture_path = NULL WHERE id = ?`, id); err != nil {
				tx.Rollback()
				http.Error(w, "failed to remove picture", http.StatusInternalServerError)
				return
			}
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to save event", http.StatusInternalServerError)
			return
		}
		if r.FormValue("remove_picture") == "1" && oldPic.Valid {
			removePicture(uploadsDir, oldPic.String)
		}

		saveEventPicture(db, r, uploadsDir, id)

		d, err := loadEvent(db, id)
		if err != nil {
			http.Error(w, "saved, but failed to reload event", http.StatusInternalServerError)
			return
		}
		writeJSON(w, d)
	}
}

func deleteEventHandler(db *sql.DB, uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "event not found", http.StatusNotFound)
			return
		}
		var pic sql.NullString
		if err := db.QueryRow(`SELECT picture_path FROM events WHERE id = ?`, id).Scan(&pic); err != nil {
			http.Error(w, "event not found", http.StatusNotFound)
			return
		}
		if pic.Valid {
			removePicture(uploadsDir, pic.String)
		}
		if err := deleteEventRows(db, id); err != nil {
			log.Printf("delete event: %v", err)
			http.Error(w, "failed to delete event", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"deleted": id})
	}
}

// createEventFromSourceHandler is what "add event details" on an imported
// timeline node calls: it finds the details record for a character's birth
// or a location's founding, or creates an empty one. Idempotent.
func createEventFromSourceHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			SourceType string `json:"source_type"`
			SourceID   int64  `json:"source_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		src, date, ok := resolveSource(db, in.SourceType, in.SourceID)
		if !ok {
			http.Error(w, "that character or location no longer exists", http.StatusNotFound)
			return
		}

		var id int64
		err := db.QueryRow(
			`SELECT id FROM events WHERE source_type = ? AND source_id = ?`, in.SourceType, in.SourceID,
		).Scan(&id)
		if err == sql.ErrNoRows {
			res, err := db.Exec(
				`INSERT INTO events (name, event_date, source_type, source_id) VALUES (?, ?, ?, ?)`,
				sourceEventName(src), date, in.SourceType, in.SourceID,
			)
			if err != nil {
				log.Printf("insert source event: %v", err)
				http.Error(w, "failed to create event", http.StatusInternalServerError)
				return
			}
			id, _ = res.LastInsertId()
		} else if err != nil {
			http.Error(w, "failed to look up event", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"id": id})
	}
}

// ---- Pickers & tags -----------------------------------------------------------

type tagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

func listEventTagsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(`SELECT tag, COUNT(*) FROM event_tags GROUP BY tag COLLATE NOCASE ORDER BY tag COLLATE NOCASE`)
		if err != nil {
			http.Error(w, "failed to load tags", http.StatusInternalServerError)
			log.Printf("event tags: %v", err)
			return
		}
		defer rows.Close()
		out := []tagCount{}
		for rows.Next() {
			var t tagCount
			if rows.Scan(&t.Tag, &t.Count) == nil {
				out = append(out, t)
			}
		}
		writeJSON(w, out)
	}
}

// characterOption is the slim character list for the "involved people"
// picker — names only, without the stat computation the grid does.
type characterOption struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	PicturePath string `json:"picture_path"`
}

func listCharacterOptionsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(`
			SELECT c.id, v.name, COALESCE(v.picture_path, '')
			FROM characters c
			JOIN character_versions v ON v.character_id = c.id AND v.is_current = 1
			ORDER BY v.name COLLATE NOCASE`)
		if err != nil {
			http.Error(w, "failed to load characters", http.StatusInternalServerError)
			log.Printf("character options: %v", err)
			return
		}
		defer rows.Close()
		out := []characterOption{}
		for rows.Next() {
			var o characterOption
			if rows.Scan(&o.ID, &o.Name, &o.PicturePath) == nil {
				if o.PicturePath != "" {
					o.PicturePath = "/uploads/" + o.PicturePath
				}
				out = append(out, o)
			}
		}
		writeJSON(w, out)
	}
}
