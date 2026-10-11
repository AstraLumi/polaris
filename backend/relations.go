package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// Relations link two characters: "Aria is Bram's sister", "Cora is Aria's
// mentor". A relation belongs to the characters, not to a version, and can
// start and end at a chapter and/or an in-story date (an ally who became a
// rival is two relations); each version shows the ones that hold at its own
// chapter and date (storytime.go). It is stored once and read from both sides: from_id's side
// reads the kind's first wording ("Parent of"), to_id's side the second
// ("Child of").

const characterRelationsTableSQL = `CREATE TABLE character_relations (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	from_id       INTEGER NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
	to_id         INTEGER NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
	kind          TEXT NOT NULL,
	label         TEXT NOT NULL DEFAULT '', -- custom wording, from from_id's side
	reverse_label TEXT NOT NULL DEFAULT '', -- custom wording, from to_id's side
	since         TEXT NOT NULL DEFAULT '',
	until         TEXT NOT NULL DEFAULT '',
	notes         TEXT NOT NULL DEFAULT '',
	CHECK (from_id <> to_id)
)`

// relationWording is how each kind reads from each side, in English (the
// frontend translates it; frontend/src/relations.js has the same list).
// "custom" uses the relation's own label and reverse_label instead.
var relationWording = map[string][2]string{
	"parent":  {"Parent of", "Child of"},
	"sibling": {"Sibling of", "Sibling of"},
	"spouse":  {"Spouse of", "Spouse of"},
	"partner": {"Partner of", "Partner of"},
	"friend":  {"Friend of", "Friend of"},
	"ally":    {"Ally of", "Ally of"},
	"rival":   {"Rival of", "Rival of"},
	"enemy":   {"Enemy of", "Enemy of"},
	"mentor":  {"Mentor of", "Student of"},
}

type relationOut struct {
	ID           int64  `json:"id"`
	FromID       int64  `json:"from_id"`
	FromName     string `json:"from_name"`
	FromPicture  string `json:"from_picture"`
	ToID         int64  `json:"to_id"`
	ToName       string `json:"to_name"`
	ToPicture    string `json:"to_picture"`
	Kind         string `json:"kind"`
	Label        string `json:"label"`
	ReverseLabel string `json:"reverse_label"`
	Since        string `json:"since"`
	Until        string `json:"until"`
	SinceChapter *int64 `json:"since_chapter_id"`
	UntilChapter *int64 `json:"until_chapter_id"`
	Notes        string `json:"notes"`
}

// holdsAt reports whether the relation holds at story point p.
func (r relationOut) holdsAt(p storyPoint, ranks map[int64]int) bool {
	return holdsAt(p, ranks, r.SinceChapter, r.UntilChapter, r.Since, r.Until)
}

// wording is the relation as read from character id's side, and whether it
// is one of the built-in (translatable) phrases.
func (r relationOut) wording(id int64) (text string, builtin bool) {
	side := 0
	if r.ToID == id {
		side = 1
	}
	if w, ok := relationWording[r.Kind]; ok {
		return w[side], true
	}
	if side == 1 && r.ReverseLabel != "" {
		return r.ReverseLabel, false
	}
	return r.Label, false
}

const relationSelect = `
	SELECT r.id, r.from_id, fv.name, COALESCE(fv.picture_path, ''), r.to_id, tv.name, COALESCE(tv.picture_path, ''),
	       r.kind, r.label, r.reverse_label, r.since, r.until, r.notes, r.since_chapter_id, r.until_chapter_id
	FROM character_relations r
	JOIN character_versions fv ON fv.character_id = r.from_id AND fv.is_current = 1
	JOIN character_versions tv ON tv.character_id = r.to_id AND tv.is_current = 1`

func scanRelations(rows *sql.Rows) []relationOut {
	defer rows.Close()
	out := []relationOut{}
	for rows.Next() {
		var r relationOut
		var since, until sql.NullInt64
		if err := rows.Scan(&r.ID, &r.FromID, &r.FromName, &r.FromPicture, &r.ToID, &r.ToName, &r.ToPicture,
			&r.Kind, &r.Label, &r.ReverseLabel, &r.Since, &r.Until, &r.Notes, &since, &until); err != nil {
			log.Printf("scan relation: %v", err)
			continue
		}
		if since.Valid {
			r.SinceChapter = &since.Int64
		}
		if until.Valid {
			r.UntilChapter = &until.Int64
		}
		r.FromPicture = uploadURL(r.FromPicture)
		r.ToPicture = uploadURL(r.ToPicture)
		out = append(out, r)
	}
	return out
}

// relationsOf lists every relation a character is on either side of, in the
// order they were added.
func relationsOf(db *sql.DB, characterID int64) []relationOut {
	rows, err := db.Query(relationSelect+`
		WHERE r.from_id = ? OR r.to_id = ?
		ORDER BY r.id`, characterID, characterID)
	if err != nil {
		log.Printf("relations of %d: %v", characterID, err)
		return []relationOut{}
	}
	return scanRelations(rows)
}

type relationIn struct {
	FromID       int64  `json:"from_id"`
	ToID         int64  `json:"to_id"`
	Kind         string `json:"kind"`
	Label        string `json:"label"`
	ReverseLabel string `json:"reverse_label"`
	Since        string `json:"since"`
	Until        string `json:"until"`
	SinceChapter *int64 `json:"since_chapter_id"`
	UntilChapter *int64 `json:"until_chapter_id"`
	Notes        string `json:"notes"`
}

// validate cleans the input in place and returns a user-facing message
// when it can't be saved.
func (in *relationIn) validate(db *sql.DB) string {
	in.Kind = strings.TrimSpace(in.Kind)
	in.Label = strings.TrimSpace(in.Label)
	in.ReverseLabel = strings.TrimSpace(in.ReverseLabel)
	in.Notes = strings.TrimSpace(in.Notes)
	if in.FromID == in.ToID {
		return "a character can't be related to themselves"
	}
	for _, id := range []int64{in.FromID, in.ToID} {
		var one int
		if db.QueryRow(`SELECT 1 FROM characters WHERE id = ?`, id).Scan(&one) != nil {
			return "pick two characters"
		}
	}
	if _, ok := relationWording[in.Kind]; ok {
		in.Label, in.ReverseLabel = "", ""
	} else if in.Kind == "custom" {
		if in.Label == "" {
			return "a custom relation needs its wording"
		}
		if len(in.Label) > 80 || len(in.ReverseLabel) > 80 {
			return "that wording is too long"
		}
	} else {
		return "unknown kind of relation"
	}
	if len(in.Notes) > 4000 {
		return "that text is too long"
	}
	// A chapter that no longer exists is simply dropped.
	ranks := chapterRanks(db)
	for _, c := range []**int64{&in.SinceChapter, &in.UntilChapter} {
		if *c != nil {
			if _, ok := ranks[**c]; !ok {
				*c = nil
			}
		}
	}
	if in.SinceChapter != nil && in.UntilChapter != nil && ranks[*in.UntilChapter] <= ranks[*in.SinceChapter] {
		return "the end chapter must come after the start chapter"
	}
	return validateSpan(db, &in.Since, &in.Until)
}

// validateSpan normalizes an optional since/until pair of story dates.
func validateSpan(db *sql.DB, since, until *string) string {
	cal := loadCalendar(db)
	for _, d := range []struct {
		label string
		v     *string
	}{{"Since", since}, {"Until", until}} {
		*d.v = normalizeStoryDate(*d.v)
		if *d.v == "" {
			continue
		}
		if _, ok := parseStoryDate(*d.v); !ok {
			return "dates must look like DD-MM-YYYY (or just a year)"
		}
		if msg := checkDateFits(cal, d.label, *d.v); msg != "" {
			return msg
		}
	}
	if *since != "" && *until != "" && dateBefore(*until, *since) {
		return "the end date is before the start date"
	}
	return ""
}

func decodeRelation(w http.ResponseWriter, r *http.Request, db *sql.DB) (relationIn, bool) {
	var in relationIn
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return in, false
	}
	if msg := in.validate(db); msg != "" {
		http.Error(w, msg, http.StatusBadRequest)
		return in, false
	}
	return in, true
}

func loadRelation(db *sql.DB, id int64) (relationOut, error) {
	rows, err := db.Query(relationSelect+` WHERE r.id = ?`, id)
	if err != nil {
		return relationOut{}, err
	}
	list := scanRelations(rows)
	if len(list) == 0 {
		return relationOut{}, sql.ErrNoRows
	}
	return list[0], nil
}

func createRelationHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		in, ok := decodeRelation(w, r, db)
		if !ok {
			return
		}
		res, err := db.Exec(`INSERT INTO character_relations
			(from_id, to_id, kind, label, reverse_label, since, until, notes, since_chapter_id, until_chapter_id)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			in.FromID, in.ToID, in.Kind, in.Label, in.ReverseLabel, in.Since, in.Until, in.Notes, in.SinceChapter, in.UntilChapter)
		if err != nil {
			log.Printf("create relation: %v", err)
			http.Error(w, "failed to save the relation", http.StatusInternalServerError)
			return
		}
		id, _ := res.LastInsertId()
		out, err := loadRelation(db, id)
		if err != nil {
			http.Error(w, "failed to save the relation", http.StatusInternalServerError)
			return
		}
		writeJSON(w, out)
	}
}

func updateRelationHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "relation not found", http.StatusNotFound)
			return
		}
		in, ok := decodeRelation(w, r, db)
		if !ok {
			return
		}
		res, err := db.Exec(`UPDATE character_relations
			SET from_id = ?, to_id = ?, kind = ?, label = ?, reverse_label = ?, since = ?, until = ?, notes = ?,
				since_chapter_id = ?, until_chapter_id = ?
			WHERE id = ?`,
			in.FromID, in.ToID, in.Kind, in.Label, in.ReverseLabel, in.Since, in.Until, in.Notes, in.SinceChapter, in.UntilChapter, id)
		if err != nil {
			log.Printf("update relation: %v", err)
			http.Error(w, "failed to save the relation", http.StatusInternalServerError)
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			http.Error(w, "relation not found", http.StatusNotFound)
			return
		}
		out, err := loadRelation(db, id)
		if err != nil {
			http.Error(w, "failed to save the relation", http.StatusInternalServerError)
			return
		}
		writeJSON(w, out)
	}
}

func deleteRelationHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "relation not found", http.StatusNotFound)
			return
		}
		if _, err := db.Exec(`DELETE FROM character_relations WHERE id = ?`, id); err != nil {
			http.Error(w, "failed to delete the relation", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"deleted": id})
	}
}

// characterConnectionsHandler returns a character's relations and faction
// memberships, for the Relations tab of their sheet.
func characterConnectionsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "character not found", http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]any{
			"relations": relationsOf(db, id),
			"factions":  membershipsOf(db, id),
		})
	}
}
