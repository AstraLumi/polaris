package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// A chapter's text in the writing editor (docs/EDITOR.md, "The server"):
// one ProseMirror document per chapter, saved whole with a version number
// so a save from an older copy is refused, plus saved versions to go back
// to. The server keeps a version on its own every 30 minutes of writing
// ("Autosave", the last 20 kept per chapter); versions saved by the writer
// or before a restore or a page type switch are kept until the chapter goes.

const (
	maxDocBytes       = maxPMDocBytes // a chapter's document as JSON (pmdoc.go)
	maxVersionLabel   = 200
	autoVersionEvery  = 30 * time.Minute
	keepAutoVersions  = 20
	autoVersionLabel  = "Autosave"
	restoreLabel      = "Before restoring"
	emptyChapterDoc   = `{"type":"doc","content":[{"type":"paragraph"}]}`
	sqlTimeLayout     = "2006-01-02 15:04:05"
	chapterTextBodyAt = maxDocBytes + (64 << 10)
)

// editorNow is the editor's clock; tests move it to see the automatic
// versions.
var editorNow = time.Now

// sqlTime writes a time the way SQLite's datetime('now') does.
func sqlTime(t time.Time) string { return t.UTC().Format(sqlTimeLayout) }

type chapterText struct {
	Doc       json.RawMessage `json:"doc"`
	Version   int64           `json:"version"`
	Words     int             `json:"words"`
	UpdatedAt string          `json:"updated_at"`
}

var errNoChapter = errors.New("chapter not found")

// loadChapterText gives a chapter's text; a chapter never written has an
// empty doc at version 0.
func loadChapterText(db editorDB, id int64) (chapterText, error) {
	var one int
	if err := db.QueryRow(`SELECT 1 FROM chapters WHERE id = ?`, id).Scan(&one); err != nil {
		if err == sql.ErrNoRows {
			return chapterText{}, errNoChapter
		}
		return chapterText{}, err
	}
	var t chapterText
	var doc string
	err := db.QueryRow(`SELECT doc, version, words, updated_at FROM chapter_texts WHERE chapter_id = ?`, id).
		Scan(&doc, &t.Version, &t.Words, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return chapterText{Doc: json.RawMessage(emptyChapterDoc)}, nil
	}
	if err != nil {
		return chapterText{}, err
	}
	t.Doc = json.RawMessage(doc)
	return t, nil
}

// storyUploadPrefix matches a picture address as the browser was given it
// ("/uploads/s/<story>/…", see uploadsRewriter); docs store "/uploads/…" so
// they keep working when the story is exported and imported under a new id.
var storyUploadPrefix = regexp.MustCompile(`"/uploads/s/[0-9a-f]{16}/`)

// cleanChapterDoc checks a posted document and gives it back as it will be
// stored (compact, picture addresses story-free) with its word count.
func cleanChapterDoc(raw json.RawMessage) ([]byte, int, string) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, 0, "the chapter's text is missing"
	}
	if len(raw) > maxDocBytes {
		return nil, 0, "the chapter is too long to save in one piece; split it into two chapters"
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return nil, 0, "the chapter's text is not valid"
	}
	doc := storyUploadPrefix.ReplaceAll(buf.Bytes(), []byte(`"/uploads/`))
	d, err := parsePMDoc(doc) // pmdoc.go: the schema's checks
	if err != nil {
		return nil, 0, err.Error()
	}
	return doc, docWords(d), ""
}

func chapterIDOf(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil
}

func writeChapterText(w http.ResponseWriter, db editorDB, id int64) {
	t, err := loadChapterText(db, id)
	if errors.Is(err, errNoChapter) {
		http.Error(w, "chapter not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("load chapter text %d: %v", id, err)
		http.Error(w, "failed to load the chapter's text", http.StatusInternalServerError)
		return
	}
	writeJSON(w, t)
}

func getChapterTextHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := chapterIDOf(r)
		if !ok {
			http.Error(w, "chapter not found", http.StatusNotFound)
			return
		}
		writeChapterText(w, db, id)
	}
}

// keepVersion saves a doc as one of the chapter's versions; an automatic
// one also prunes the automatic versions beyond the last keepAutoVersions.
func keepVersion(tx editorDB, chapterID int64, doc string, words int, label string, automatic bool, at time.Time) (int64, error) {
	res, err := tx.Exec(`INSERT INTO chapter_text_versions (chapter_id, doc, words, label, automatic, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`, chapterID, doc, words, label, automatic, sqlTime(at))
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	if automatic {
		if _, err := tx.Exec(`DELETE FROM chapter_text_versions
			WHERE chapter_id = ? AND automatic = 1 AND id NOT IN (
				SELECT id FROM chapter_text_versions WHERE chapter_id = ? AND automatic = 1
				ORDER BY id DESC LIMIT ?)`, chapterID, chapterID, keepAutoVersions); err != nil {
			return 0, err
		}
	}
	return id, nil
}

// autoVersion keeps the stored text as an automatic version when the
// writing since the last one has gone on for autoVersionEvery, and says
// whether it did. Text already kept as the newest version isn't kept twice.
func autoVersion(tx editorDB, chapterID int64, now time.Time) (bool, error) {
	var doc, since string
	var words int
	if err := tx.QueryRow(`SELECT doc, words, auto_since FROM chapter_texts WHERE chapter_id = ?`, chapterID).
		Scan(&doc, &words, &since); err != nil {
		return false, err
	}
	start, err := time.Parse(sqlTimeLayout, since)
	if err == nil && now.Sub(start) < autoVersionEvery {
		return false, nil
	}
	var last string
	err = tx.QueryRow(`SELECT doc FROM chapter_text_versions WHERE chapter_id = ? ORDER BY id DESC LIMIT 1`, chapterID).Scan(&last)
	if err != nil && err != sql.ErrNoRows {
		return false, err
	}
	if err == sql.ErrNoRows || last != doc {
		if _, err := keepVersion(tx, chapterID, doc, words, autoVersionLabel, true, now); err != nil {
			return false, err
		}
	}
	return true, nil
}

// putChapterTextHandler saves a chapter's text when it was written on the
// stored version (base_version); otherwise it answers 409 with the stored
// text so the writer can pick.
func putChapterTextHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := chapterIDOf(r)
		if !ok {
			http.Error(w, "chapter not found", http.StatusNotFound)
			return
		}
		var in struct {
			Doc         json.RawMessage `json:"doc"`
			BaseVersion *int64          `json:"base_version"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, chapterTextBodyAt)).Decode(&in); err != nil {
			var tooBig *http.MaxBytesError
			if errors.As(err, &tooBig) {
				http.Error(w, "the chapter is too long to save in one piece; split it into two chapters", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		if in.BaseVersion == nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		doc, words, msg := cleanChapterDoc(in.Doc)
		if msg != "" {
			status := http.StatusBadRequest
			if len(in.Doc) > maxDocBytes {
				status = http.StatusRequestEntityTooLarge
			}
			http.Error(w, msg, status)
			return
		}
		saveChapterText(w, db, id, *in.BaseVersion, string(doc), words)
	}
}

func saveChapterText(w http.ResponseWriter, db *sql.DB, id, base int64, doc string, words int) {
	tx, err := db.Begin()
	if err != nil {
		http.Error(w, "failed to save the chapter's text", http.StatusInternalServerError)
		return
	}
	fail := func(err error) {
		tx.Rollback()
		log.Printf("save chapter text %d: %v", id, err)
		http.Error(w, "failed to save the chapter's text", http.StatusInternalServerError)
	}
	stored, err := loadChapterText(tx, id)
	if errors.Is(err, errNoChapter) {
		tx.Rollback()
		http.Error(w, "chapter not found", http.StatusNotFound)
		return
	}
	if err != nil {
		fail(err)
		return
	}
	conflict := func() {
		tx.Rollback()
		cur, err := loadChapterText(db, id)
		if err != nil {
			http.Error(w, "failed to save the chapter's text", http.StatusInternalServerError)
			return
		}
		writeJSONStatus(w, http.StatusConflict, cur)
	}
	if stored.Version != base {
		conflict()
		return
	}
	// Nothing changed: nothing to save.
	if stored.Version > 0 && string(stored.Doc) == doc {
		tx.Rollback()
		writeJSON(w, stored)
		return
	}
	now := editorNow()
	var res sql.Result
	if stored.Version == 0 {
		res, err = tx.Exec(`INSERT INTO chapter_texts (chapter_id, doc, version, words, updated_at, auto_since)
			VALUES (?, ?, 1, ?, ?, ?) ON CONFLICT(chapter_id) DO NOTHING`, id, doc, words, sqlTime(now), sqlTime(now))
	} else {
		kept, e := autoVersion(tx, id, now)
		if e != nil {
			fail(e)
			return
		}
		q := `UPDATE chapter_texts SET doc = ?, version = version + 1, words = ?, updated_at = ?`
		args := []any{doc, words, sqlTime(now)}
		if kept {
			q += `, auto_since = ?`
			args = append(args, sqlTime(now))
		}
		res, err = tx.Exec(q+` WHERE chapter_id = ? AND version = ?`, append(args, id, base)...)
	}
	if err != nil {
		fail(err)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		// Saved by someone else in the meantime.
		conflict()
		return
	}
	if err := tx.Commit(); err != nil {
		fail(err)
		return
	}
	writeChapterText(w, db, id)
}

type textVersionOut struct {
	ID        int64           `json:"id"`
	ChapterID int64           `json:"chapter_id"`
	Label     string          `json:"label"`
	Words     int             `json:"words"`
	Automatic bool            `json:"automatic"`
	CreatedAt string          `json:"created_at"`
	Doc       json.RawMessage `json:"doc,omitempty"`
}

func listTextVersionsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := chapterIDOf(r)
		if !ok {
			http.Error(w, "chapter not found", http.StatusNotFound)
			return
		}
		if _, err := loadChapterText(db, id); err != nil {
			http.Error(w, "chapter not found", http.StatusNotFound)
			return
		}
		rows, err := db.Query(`SELECT id, label, words, automatic, created_at FROM chapter_text_versions
			WHERE chapter_id = ? ORDER BY id DESC`, id)
		if err != nil {
			http.Error(w, "failed to load the versions", http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		out := []textVersionOut{}
		for rows.Next() {
			v := textVersionOut{ChapterID: id}
			if err := rows.Scan(&v.ID, &v.Label, &v.Words, &v.Automatic, &v.CreatedAt); err != nil {
				http.Error(w, "failed to load the versions", http.StatusInternalServerError)
				return
			}
			out = append(out, v)
		}
		writeJSON(w, out)
	}
}

func loadTextVersion(db editorDB, chapterID, vid int64) (textVersionOut, error) {
	v := textVersionOut{ID: vid, ChapterID: chapterID}
	var doc string
	err := db.QueryRow(`SELECT label, words, automatic, created_at, doc FROM chapter_text_versions
		WHERE id = ? AND chapter_id = ?`, vid, chapterID).Scan(&v.Label, &v.Words, &v.Automatic, &v.CreatedAt, &doc)
	v.Doc = json.RawMessage(doc)
	return v, err
}

func textVersionIDs(r *http.Request) (int64, int64, bool) {
	id, ok := chapterIDOf(r)
	vid, err := strconv.ParseInt(r.PathValue("vid"), 10, 64)
	return id, vid, ok && err == nil
}

func getTextVersionHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, vid, ok := textVersionIDs(r)
		if !ok {
			http.Error(w, "version not found", http.StatusNotFound)
			return
		}
		v, err := loadTextVersion(db, id, vid)
		if err != nil {
			http.Error(w, "version not found", http.StatusNotFound)
			return
		}
		writeJSON(w, v)
	}
}

// createTextVersionHandler saves the chapter's current text as a version
// with the writer's label. With a doc, that doc is saved instead (the copy
// the writer didn't keep after a 409), leaving the current text alone.
func createTextVersionHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := chapterIDOf(r)
		if !ok {
			http.Error(w, "chapter not found", http.StatusNotFound)
			return
		}
		var in struct {
			Label string          `json:"label"`
			Doc   json.RawMessage `json:"doc"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, chapterTextBodyAt)).Decode(&in); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		label := strings.TrimSpace(in.Label)
		if tooLong(label, maxVersionLabel) {
			http.Error(w, "that text is too long", http.StatusBadRequest)
			return
		}
		cur, err := loadChapterText(db, id)
		if errors.Is(err, errNoChapter) {
			http.Error(w, "chapter not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "failed to save the version", http.StatusInternalServerError)
			return
		}
		doc, words := string(cur.Doc), cur.Words
		if len(in.Doc) > 0 && string(in.Doc) != "null" {
			d, n, msg := cleanChapterDoc(in.Doc)
			if msg != "" {
				http.Error(w, msg, http.StatusBadRequest)
				return
			}
			doc, words = string(d), n
		} else if cur.Version == 0 {
			http.Error(w, "nothing is written in this chapter yet", http.StatusBadRequest)
			return
		}
		vid, err := keepVersion(db, id, doc, words, label, false, editorNow())
		if err != nil {
			log.Printf("save version of chapter %d: %v", id, err)
			http.Error(w, "failed to save the version", http.StatusInternalServerError)
			return
		}
		v, err := loadTextVersion(db, id, vid)
		if err != nil {
			http.Error(w, "failed to save the version", http.StatusInternalServerError)
			return
		}
		v.Doc = nil
		writeJSON(w, v)
	}
}

// restoreTextVersionHandler puts a version's text back, keeping the
// current text as a version "Before restoring" first.
func restoreTextVersionHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, vid, ok := textVersionIDs(r)
		if !ok {
			http.Error(w, "version not found", http.StatusNotFound)
			return
		}
		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to restore the version", http.StatusInternalServerError)
			return
		}
		fail := func(err error) {
			tx.Rollback()
			log.Printf("restore version %d of chapter %d: %v", vid, id, err)
			http.Error(w, "failed to restore the version", http.StatusInternalServerError)
		}
		v, err := loadTextVersion(tx, id, vid)
		if err != nil {
			tx.Rollback()
			http.Error(w, "version not found", http.StatusNotFound)
			return
		}
		cur, err := loadChapterText(tx, id)
		if err != nil {
			fail(err)
			return
		}
		now := editorNow()
		if cur.Version > 0 {
			if _, err := keepVersion(tx, id, string(cur.Doc), cur.Words, restoreLabel, false, now); err != nil {
				fail(err)
				return
			}
			_, err = tx.Exec(`UPDATE chapter_texts SET doc = ?, version = version + 1, words = ?, updated_at = ?, auto_since = ?
				WHERE chapter_id = ?`, string(v.Doc), v.Words, sqlTime(now), sqlTime(now), id)
		} else {
			_, err = tx.Exec(`INSERT INTO chapter_texts (chapter_id, doc, version, words, updated_at, auto_since)
				VALUES (?, ?, 1, ?, ?, ?)`, id, string(v.Doc), v.Words, sqlTime(now), sqlTime(now))
		}
		if err != nil {
			fail(err)
			return
		}
		if err := tx.Commit(); err != nil {
			fail(err)
			return
		}
		writeChapterText(w, db, id)
	}
}
