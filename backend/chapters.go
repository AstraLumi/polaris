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
	// BookSetUp: the volume's book settings were saved (offered as the
	// source when a new volume is added).
	BookSetUp bool `json:"book_set_up"`
}

type chapterSummary struct {
	ID           int64  `json:"id"`
	VolumeID     *int64 `json:"volume_id"`
	Title        string `json:"title"`
	Position     int    `json:"position"`
	Number       int    `json:"number"`                 // 1-based, within its volume (or among those without one); 0 for front matter
	FrontMatter  bool   `json:"front_matter,omitempty"` // marked so in its book (the writing editor's chapter options)
	EventCount   int    `json:"event_count"`
	VersionCount int    `json:"version_count"`
	Words        int    `json:"words"` // in its text (chaptertext.go)
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
	rows, err := db.Query(`SELECT id, title, position,
		EXISTS(SELECT 1 FROM book_settings b WHERE b.volume_id = volumes.id AND b.is_set_up = 1)
		FROM volumes ORDER BY position, id`)
	if err != nil {
		return idx, err
	}
	order := map[int64]int{}
	for rows.Next() {
		var v volumeOut
		if err := rows.Scan(&v.ID, &v.Title, &v.Position, &v.BookSetUp); err != nil {
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
		       (SELECT COUNT(*) FROM character_versions v WHERE v.chapter_id = c.id),
		       COALESCE((SELECT t.words FROM chapter_texts t WHERE t.chapter_id = c.id), 0)
		FROM chapters c`)
	if err != nil {
		return idx, err
	}
	defer rows.Close()
	for rows.Next() {
		var c chapterSummary
		var vol sql.NullInt64
		if err := rows.Scan(&c.ID, &vol, &c.Title, &c.Position, &c.EventCount, &c.VersionCount, &c.Words); err != nil {
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
	front, err := frontMatterChapters(db)
	if err != nil {
		return idx, err
	}
	counts := map[int]int{}
	for i := range idx.Chapters {
		c := &idx.Chapters[i]
		book := int64(0)
		if c.VolumeID != nil {
			book = *c.VolumeID
		}
		if front[book][c.ID] {
			c.FrontMatter = true
			continue
		}
		g := group(*c)
		counts[g]++
		c.Number = counts[g]
	}
	return idx, nil
}

// frontMatterChapters gives, per book (0: the chapters without a volume),
// the chapters its settings mark as front matter: they aren't numbered.
func frontMatterChapters(db *sql.DB) (map[int64]map[int64]bool, error) {
	rows, err := db.Query(`SELECT volume_id, settings FROM book_settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]map[int64]bool{}
	for rows.Next() {
		var vol int64
		var settings string
		if err := rows.Scan(&vol, &settings); err != nil {
			return nil, err
		}
		for id, o := range bookLayoutOf(json.RawMessage(settings)).ChapterOptions {
			if o.FrontMatter {
				if out[vol] == nil {
					out[vol] = map[int64]bool{}
				}
				out[vol][id] = true
			}
		}
	}
	return out, rows.Err()
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
			// Links are stored by id (wikilinks.go).
			notes := *in.Notes
			if strings.Contains(notes, "[[") {
				x, err := loadWikiLinkIndex(db)
				if err != nil {
					log.Printf("index the wiki: %v", err)
					http.Error(w, "failed to save the chapter", http.StatusInternalServerError)
					return
				}
				notes = normalizeWikiLinks(notes, x)
			}
			q += `, notes = ?`
			args = append(args, notes)
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
	// CopySettingsFrom: a volume whose book settings the new one starts
	// with (see copyBookSettings).
	CopySettingsFrom *int64 `json:"copy_settings_from"`
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
		if in.CopySettingsFrom != nil {
			var one int
			if db.QueryRow(`SELECT 1 FROM volumes WHERE id = ?`, *in.CopySettingsFrom).Scan(&one) != nil {
				http.Error(w, "the volume to copy the book settings from does not exist", http.StatusBadRequest)
				return
			}
		}
		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to save the volume", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()
		res, err := tx.Exec(`INSERT INTO volumes (title, position)
			SELECT ?, COALESCE(MAX(position), -1) + 1 FROM volumes`, title)
		if err != nil {
			log.Printf("create volume: %v", err)
			http.Error(w, "failed to save the volume", http.StatusInternalServerError)
			return
		}
		id, _ := res.LastInsertId()
		if in.CopySettingsFrom != nil {
			if err := copyBookSettings(tx, *in.CopySettingsFrom, id); err != nil {
				log.Printf("copy book settings: %v", err)
				http.Error(w, "failed to copy the book settings", http.StatusInternalServerError)
				return
			}
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to save the volume", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"id": id, "title": title})
	}
}

// copyBookSettings gives volume `to` the book settings of volume `from`
// (is_set_up too), except what belongs to the one book: meta title,
// subtitle, isbn and description, and `chapter_options` (keyed by the
// other volume's chapter ids). A source never set up copies nothing. What
// the server doesn't know is kept, as saves do.
func copyBookSettings(db editorDB, from, to int64) error {
	var s string
	var setUp bool
	err := db.QueryRow(`SELECT settings, is_set_up FROM book_settings WHERE volume_id = ?`, from).Scan(&s, &setUp)
	if err == sql.ErrNoRows || (err == nil && !setUp) {
		return nil
	}
	if err != nil {
		return err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return err
	}
	delete(m, "chapter_options")
	if raw, ok := m["meta"]; ok {
		var meta map[string]json.RawMessage
		if json.Unmarshal(raw, &meta) == nil {
			for _, k := range []string{"title", "subtitle", "isbn", "description"} {
				delete(meta, k)
			}
			m["meta"], _ = json.Marshal(meta)
		}
	}
	out, err := json.Marshal(m)
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO book_settings (volume_id, settings, is_set_up, updated_at) VALUES (?, ?, 1, ?)`,
		to, string(out), sqlTime(editorNow()))
	return err
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
