package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// Chapters are the story's own structure: an ordered list of chapters, each
// with notes, optionally grouped into volumes (books). Events and character
// versions can point to the chapter they happen in, so a chapter's page
// lists what happened in it and who changed. A chapter's number is its place
// within its volume; it isn't stored.

const volumesTableSQL = `CREATE TABLE volumes (
	id       INTEGER PRIMARY KEY AUTOINCREMENT,
	title    TEXT NOT NULL,
	position INTEGER NOT NULL DEFAULT 0
)`

const chaptersTableSQL = `CREATE TABLE chapters (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	volume_id  INTEGER REFERENCES volumes(id) ON DELETE SET NULL,
	title      TEXT NOT NULL,
	notes      TEXT NOT NULL DEFAULT '',
	position   INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	updated_at TEXT NOT NULL DEFAULT (datetime('now'))
)`

// events.chapter_id and character_versions.chapter_id are added to existing
// tables, which SQLite can only do without a foreign key, so this trigger
// does what ON DELETE SET NULL would.
const chaptersUnlinkTriggerSQL = `CREATE TRIGGER chapters_unlink AFTER DELETE ON chapters BEGIN
	UPDATE events SET chapter_id = NULL WHERE chapter_id = OLD.id;
	UPDATE character_versions SET chapter_id = NULL WHERE chapter_id = OLD.id;
END`

type volumeOut struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Position int    `json:"position"`
}

type chapterSummary struct {
	ID           int64  `json:"id"`
	VolumeID     *int64 `json:"volume_id"`
	Title        string `json:"title"`
	Position     int    `json:"position"`
	Number       int    `json:"number"` // 1-based, within its volume (or among those without one)
	EventCount   int    `json:"event_count"`
	VersionCount int    `json:"version_count"`
}

type chapterIndex struct {
	Volumes  []volumeOut      `json:"volumes"`
	Chapters []chapterSummary `json:"chapters"`
}

// loadChapterIndex returns every volume and chapter in reading order:
// volumes by position, chapters without a volume first, then each volume's
// chapters by position.
func loadChapterIndex(db *sql.DB) (chapterIndex, error) {
	idx := chapterIndex{Volumes: []volumeOut{}, Chapters: []chapterSummary{}}
	rows, err := db.Query(`SELECT id, title, position FROM volumes ORDER BY position, id`)
	if err != nil {
		return idx, err
	}
	order := map[int64]int{}
	for rows.Next() {
		var v volumeOut
		if err := rows.Scan(&v.ID, &v.Title, &v.Position); err != nil {
			rows.Close()
			return idx, err
		}
		order[v.ID] = len(idx.Volumes)
		idx.Volumes = append(idx.Volumes, v)
	}
	rows.Close()

	rows, err = db.Query(`
		SELECT c.id, c.volume_id, c.title, c.position,
		       (SELECT COUNT(*) FROM events e WHERE e.chapter_id = c.id),
		       (SELECT COUNT(*) FROM character_versions v WHERE v.chapter_id = c.id)
		FROM chapters c`)
	if err != nil {
		return idx, err
	}
	defer rows.Close()
	for rows.Next() {
		var c chapterSummary
		var vol sql.NullInt64
		if err := rows.Scan(&c.ID, &vol, &c.Title, &c.Position, &c.EventCount, &c.VersionCount); err != nil {
			return idx, err
		}
		if vol.Valid {
			c.VolumeID = &vol.Int64
		}
		idx.Chapters = append(idx.Chapters, c)
	}
	group := func(c chapterSummary) int {
		if c.VolumeID == nil {
			return -1
		}
		return order[*c.VolumeID]
	}
	sortChapters(idx.Chapters, group)
	counts := map[int]int{}
	for i := range idx.Chapters {
		g := group(idx.Chapters[i])
		counts[g]++
		idx.Chapters[i].Number = counts[g]
	}
	return idx, nil
}

func sortChapters(list []chapterSummary, group func(chapterSummary) int) {
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i], list[j]
		if ga, gb := group(a), group(b); ga != gb {
			return ga < gb
		}
		if a.Position != b.Position {
			return a.Position < b.Position
		}
		return a.ID < b.ID
	})
}

func listChaptersHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idx, err := loadChapterIndex(db)
		if err != nil {
			log.Printf("list chapters: %v", err)
			http.Error(w, "failed to load chapters", http.StatusInternalServerError)
			return
		}
		writeJSON(w, idx)
	}
}

type chapterEvent struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	EventDate string `json:"event_date"`
}

type chapterVersion struct {
	VersionID        int64  `json:"version_id"`
	CharacterID      int64  `json:"character_id"`
	Name             string `json:"name"`
	PicturePath      string `json:"picture_path"`
	VersionDate      string `json:"version_date"`
	VersionReference string `json:"version_reference"`
	IsCurrent        bool   `json:"is_current"`
}

type chapterDetail struct {
	chapterSummary
	Notes       string           `json:"notes"`
	VolumeTitle string           `json:"volume_title"`
	PrevID      *int64           `json:"prev_id"`
	NextID      *int64           `json:"next_id"`
	Events      []chapterEvent   `json:"events"`
	Versions    []chapterVersion `json:"versions"`
	UpdatedAt   string           `json:"updated_at"`
}

func loadChapter(db *sql.DB, id int64) (*chapterDetail, error) {
	idx, err := loadChapterIndex(db)
	if err != nil {
		return nil, err
	}
	var d *chapterDetail
	for i, c := range idx.Chapters {
		if c.ID != id {
			continue
		}
		d = &chapterDetail{chapterSummary: c, Events: []chapterEvent{}, Versions: []chapterVersion{}}
		if i > 0 {
			d.PrevID = &idx.Chapters[i-1].ID
		}
		if i+1 < len(idx.Chapters) {
			d.NextID = &idx.Chapters[i+1].ID
		}
		for _, v := range idx.Volumes {
			if c.VolumeID != nil && v.ID == *c.VolumeID {
				d.VolumeTitle = v.Title
			}
		}
	}
	if d == nil {
		return nil, sql.ErrNoRows
	}
	if err := db.QueryRow(`SELECT notes, updated_at FROM chapters WHERE id = ?`, id).Scan(&d.Notes, &d.UpdatedAt); err != nil {
		return nil, err
	}

	rows, err := db.Query(`SELECT id, name, event_date FROM events WHERE chapter_id = ?`, id)
	if err == nil {
		for rows.Next() {
			var e chapterEvent
			if rows.Scan(&e.ID, &e.Name, &e.EventDate) == nil {
				e.EventDate = normalizeStoryDate(e.EventDate)
				d.Events = append(d.Events, e)
			}
		}
		rows.Close()
	}
	// In story order, like the Events page.
	sort.SliceStable(d.Events, func(i, j int) bool { return dateBefore(d.Events[i].EventDate, d.Events[j].EventDate) })

	rows, err = db.Query(`
		SELECT v.id, v.character_id, cur.name, COALESCE(cur.picture_path, ''),
		       COALESCE(v.version_date, ''), COALESCE(v.version_reference, ''), v.is_current
		FROM character_versions v
		JOIN character_versions cur ON cur.character_id = v.character_id AND cur.is_current = 1
		WHERE v.chapter_id = ?
		ORDER BY cur.name COLLATE NOCASE`, id)
	if err == nil {
		for rows.Next() {
			var v chapterVersion
			if rows.Scan(&v.VersionID, &v.CharacterID, &v.Name, &v.PicturePath, &v.VersionDate, &v.VersionReference, &v.IsCurrent) == nil {
				v.PicturePath = uploadURL(v.PicturePath)
				d.Versions = append(d.Versions, v)
			}
		}
		rows.Close()
	}
	return d, nil
}

func getChapterHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "chapter not found", http.StatusNotFound)
			return
		}
		d, err := loadChapter(db, id)
		if err != nil {
			http.Error(w, "chapter not found", http.StatusNotFound)
			return
		}
		writeJSON(w, d)
	}
}

// validVolume reads an optional volume id: unknown ids mean none.
func validVolume(db *sql.DB, id *int64) sql.NullInt64 {
	if id == nil {
		return sql.NullInt64{}
	}
	var one int
	if db.QueryRow(`SELECT 1 FROM volumes WHERE id = ?`, *id).Scan(&one) != nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *id, Valid: true}
}

// validChapter reads an optional posted chapter id (events, versions):
// blank or unknown ids mean none.
func validChapter(db *sql.DB, raw string) sql.NullInt64 {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return sql.NullInt64{}
	}
	var one int
	if db.QueryRow(`SELECT 1 FROM chapters WHERE id = ?`, id).Scan(&one) != nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: id, Valid: true}
}

type chapterIn struct {
	Title    string  `json:"title"`
	VolumeID *int64  `json:"volume_id"`
	Notes    *string `json:"notes"` // nil: leave the notes as they are
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return false
	}
	return true
}

func cleanTitle(w http.ResponseWriter, s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		http.Error(w, "a title is required", http.StatusBadRequest)
		return "", false
	}
	if len(s) > 200 {
		http.Error(w, "that text is too long", http.StatusBadRequest)
		return "", false
	}
	return s, true
}

func createChapterHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in chapterIn
		if !decodeJSON(w, r, &in) {
			return
		}
		title, ok := cleanTitle(w, in.Title)
		if !ok {
			return
		}
		vol := validVolume(db, in.VolumeID)
		// New chapters go at the end of their volume.
		res, err := db.Exec(`INSERT INTO chapters (volume_id, title, position)
			SELECT ?, ?, COALESCE(MAX(position), -1) + 1 FROM chapters WHERE volume_id IS ?`, vol, title, vol)
		if err != nil {
			log.Printf("create chapter: %v", err)
			http.Error(w, "failed to save the chapter", http.StatusInternalServerError)
			return
		}
		id, _ := res.LastInsertId()
		d, err := loadChapter(db, id)
		if err != nil {
			http.Error(w, "failed to save the chapter", http.StatusInternalServerError)
			return
		}
		writeJSON(w, d)
	}
}

func updateChapterHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "chapter not found", http.StatusNotFound)
			return
		}
		var in chapterIn
		if !decodeJSON(w, r, &in) {
			return
		}
		title, ok := cleanTitle(w, in.Title)
		if !ok {
			return
		}
		if in.Notes != nil && len(*in.Notes) > maxWikiText {
			http.Error(w, "that text is too long", http.StatusBadRequest)
			return
		}
		var oldVol sql.NullInt64
		if err := db.QueryRow(`SELECT volume_id FROM chapters WHERE id = ?`, id).Scan(&oldVol); err != nil {
			http.Error(w, "chapter not found", http.StatusNotFound)
			return
		}
		vol := validVolume(db, in.VolumeID)
		q := `UPDATE chapters SET title = ?, volume_id = ?, updated_at = datetime('now')`
		args := []any{title, vol}
		if in.Notes != nil {
			q += `, notes = ?`
			args = append(args, *in.Notes)
		}
		if vol != oldVol {
			// Moved to another volume: it goes at the end there.
			q += `, position = (SELECT COALESCE(MAX(position), -1) + 1 FROM chapters WHERE volume_id IS ?)`
			args = append(args, vol)
		}
		if _, err := db.Exec(q+` WHERE id = ?`, append(args, id)...); err != nil {
			log.Printf("update chapter: %v", err)
			http.Error(w, "failed to save the chapter", http.StatusInternalServerError)
			return
		}
		d, err := loadChapter(db, id)
		if err != nil {
			http.Error(w, "failed to save the chapter", http.StatusInternalServerError)
			return
		}
		writeJSON(w, d)
	}
}

func deleteChapterHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "chapter not found", http.StatusNotFound)
			return
		}
		if _, err := db.Exec(`DELETE FROM chapters WHERE id = ?`, id); err != nil {
			log.Printf("delete chapter: %v", err)
			http.Error(w, "failed to delete the chapter", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"deleted": id})
	}
}

type volumeIn struct {
	Title string `json:"title"`
}

func createVolumeHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in volumeIn
		if !decodeJSON(w, r, &in) {
			return
		}
		title, ok := cleanTitle(w, in.Title)
		if !ok {
			return
		}
		res, err := db.Exec(`INSERT INTO volumes (title, position)
			SELECT ?, COALESCE(MAX(position), -1) + 1 FROM volumes`, title)
		if err != nil {
			log.Printf("create volume: %v", err)
			http.Error(w, "failed to save the volume", http.StatusInternalServerError)
			return
		}
		id, _ := res.LastInsertId()
		writeJSON(w, map[string]any{"id": id, "title": title})
	}
}

func updateVolumeHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "volume not found", http.StatusNotFound)
			return
		}
		var in volumeIn
		if !decodeJSON(w, r, &in) {
			return
		}
		title, ok := cleanTitle(w, in.Title)
		if !ok {
			return
		}
		res, err := db.Exec(`UPDATE volumes SET title = ? WHERE id = ?`, title, id)
		if err != nil {
			http.Error(w, "failed to save the volume", http.StatusInternalServerError)
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			http.Error(w, "volume not found", http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]any{"id": id, "title": title})
	}
}

// deleteVolumeHandler removes a volume; its chapters stay, without one.
func deleteVolumeHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "volume not found", http.StatusNotFound)
			return
		}
		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to delete the volume", http.StatusInternalServerError)
			return
		}
		// Its chapters join the end of the volume-less list, in their order.
		if _, err := tx.Exec(`UPDATE chapters SET volume_id = NULL,
				position = position + (SELECT COALESCE(MAX(position), -1) + 1 FROM chapters WHERE volume_id IS NULL)
			WHERE volume_id = ?`, id); err != nil {
			tx.Rollback()
			http.Error(w, "failed to delete the volume", http.StatusInternalServerError)
			return
		}
		if _, err := tx.Exec(`DELETE FROM volumes WHERE id = ?`, id); err != nil {
			tx.Rollback()
			http.Error(w, "failed to delete the volume", http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to delete the volume", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"deleted": id})
	}
}

// reorderChaptersHandler takes the whole order at once: volume ids in
// reading order, and every chapter with the volume it is in, in reading
// order. Ids it doesn't mention keep their place.
func reorderChaptersHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Volumes  []int64 `json:"volumes"`
			Chapters []struct {
				ID       int64  `json:"id"`
				VolumeID *int64 `json:"volume_id"`
			} `json:"chapters"`
		}
		if !decodeJSON(w, r, &in) {
			return
		}
		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to save the order", http.StatusInternalServerError)
			return
		}
		for i, id := range in.Volumes {
			if _, err := tx.Exec(`UPDATE volumes SET position = ? WHERE id = ?`, i, id); err != nil {
				tx.Rollback()
				http.Error(w, "failed to save the order", http.StatusInternalServerError)
				return
			}
		}
		for i, c := range in.Chapters {
			if _, err := tx.Exec(`UPDATE chapters SET position = ?,
					volume_id = (SELECT id FROM volumes WHERE id = ?) WHERE id = ?`, i, c.VolumeID, c.ID); err != nil {
				tx.Rollback()
				http.Error(w, "failed to save the order", http.StatusInternalServerError)
				return
			}
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to save the order", http.StatusInternalServerError)
			return
		}
		idx, err := loadChapterIndex(db)
		if err != nil {
			http.Error(w, "failed to load chapters", http.StatusInternalServerError)
			return
		}
		writeJSON(w, idx)
	}
}
