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

// A wiki article's picture gallery: any number of captioned pictures, in an
// order the user picks. The rows hang off the article's wiki_entries row, so
// deleting the source (which deletes the entry, see wikiTriggerSQL) takes
// them along; an entry that only holds pictures is kept when its text is
// cleared. Changes save straight away, apart from the article's text.

const (
	maxGalleryPictures = 80
	maxCaption         = 300
)

const wikiGalleryTableSQL = `CREATE TABLE wiki_gallery (
	id        INTEGER PRIMARY KEY AUTOINCREMENT,
	entry_id  INTEGER NOT NULL REFERENCES wiki_entries(id) ON DELETE CASCADE,
	position  INTEGER NOT NULL,
	file_path TEXT NOT NULL,
	caption   TEXT NOT NULL DEFAULT ''
)`

// schema19Statements create what schema 19 added (after the wiki's tables).
var schema19Statements = []string{
	wikiGalleryTableSQL,
	`CREATE INDEX idx_wiki_gallery_entry ON wiki_gallery(entry_id)`,
}

type galleryItem struct {
	ID      int64  `json:"id"`
	Picture string `json:"picture"`
	Caption string `json:"caption"`
}

func loadGallery(db *sql.DB, entryID int64) []galleryItem {
	out := []galleryItem{}
	rows, err := db.Query(`SELECT id, file_path, caption FROM wiki_gallery WHERE entry_id = ? ORDER BY position, id`, entryID)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var g galleryItem
		if rows.Scan(&g.ID, &g.Picture, &g.Caption) == nil {
			g.Picture = uploadURL(g.Picture)
			out = append(out, g)
		}
	}
	return out
}

// articleEntry finds (or, with create, makes) the wiki_entries row of an
// article that exists. ok is false when there is no such article.
func articleEntry(db *sql.DB, typ string, id int64, create bool) (entryID int64, ok bool, err error) {
	t, known := wikiTypeFor(typ)
	if !known || db.QueryRow(t.oneSQL(), id).Scan(new(int64), new(string), new(string)) != nil {
		return 0, false, nil
	}
	err = db.QueryRow(`SELECT id FROM wiki_entries WHERE entity_type = ? AND entity_id = ?`, t.Key, id).Scan(&entryID)
	if err == sql.ErrNoRows && create {
		res, e := db.Exec(`INSERT INTO wiki_entries (entity_type, entity_id) VALUES (?, ?)`, t.Key, id)
		if e != nil {
			return 0, true, e
		}
		entryID, _ = res.LastInsertId()
		return entryID, true, nil
	}
	if err == sql.ErrNoRows {
		return 0, true, nil
	}
	return entryID, true, err
}

// pruneEntry deletes an entry that no longer holds anything at all.
func pruneEntry(db *sql.DB, entryID int64) {
	db.Exec(`DELETE FROM wiki_entries WHERE id = ? AND summary = '' AND fields IN ('', '{}')
		AND NOT EXISTS (SELECT 1 FROM wiki_sections WHERE entry_id = ?)
		AND NOT EXISTS (SELECT 1 FROM wiki_infobox WHERE entry_id = ?)
		AND NOT EXISTS (SELECT 1 FROM wiki_gallery WHERE entry_id = ?)`, entryID, entryID, entryID, entryID)
}

func touchEntry(db *sql.DB, entryID int64) {
	db.Exec(`UPDATE wiki_entries SET updated_at = datetime('now') WHERE id = ?`, entryID)
}

func galleryArticle(r *http.Request) (string, int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return r.PathValue("type"), id, err == nil
}

// addGalleryHandler adds the uploaded "pictures" (one or more) to the end of
// an article's gallery, each with the same optional "caption".
func addGalleryHandler(db *sql.DB, uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		typ, id, ok := galleryArticle(r)
		if !ok {
			http.Error(w, "article not found", http.StatusNotFound)
			return
		}
		if err := r.ParseMultipartForm(64 << 20); err != nil {
			http.Error(w, "invalid form data", http.StatusBadRequest)
			return
		}
		files := r.MultipartForm.File["pictures"]
		if len(files) == 0 {
			http.Error(w, "pick at least one picture", http.StatusBadRequest)
			return
		}
		caption := strings.TrimSpace(r.FormValue("caption"))
		if tooLong(caption, maxCaption) {
			http.Error(w, "that text is too long", http.StatusBadRequest)
			return
		}
		entryID, exists, err := articleEntry(db, typ, id, true)
		if !exists {
			http.Error(w, "article not found", http.StatusNotFound)
			return
		}
		if err != nil {
			log.Printf("gallery entry: %v", err)
			http.Error(w, "failed to save the picture", http.StatusInternalServerError)
			return
		}
		var count, last int
		db.QueryRow(`SELECT COUNT(*), COALESCE(MAX(position), -1) FROM wiki_gallery WHERE entry_id = ?`, entryID).Scan(&count, &last)
		if count+len(files) > maxGalleryPictures {
			pruneEntry(db, entryID)
			http.Error(w, fmt.Sprintf("a gallery holds at most %d pictures", maxGalleryPictures), http.StatusBadRequest)
			return
		}
		for i, fh := range files {
			f, err := fh.Open()
			if err != nil {
				continue
			}
			rel, err := saveUpload(uploadsDir, "gallery", fmt.Sprintf("%d-%d-%d", entryID, time.Now().UnixNano(), i), fh.Filename, f)
			f.Close()
			if err != nil {
				log.Printf("save gallery picture: %v", err)
				http.Error(w, "failed to save the picture", http.StatusInternalServerError)
				return
			}
			last++
			if _, err := db.Exec(`INSERT INTO wiki_gallery (entry_id, position, file_path, caption) VALUES (?, ?, ?, ?)`,
				entryID, last, rel, caption); err != nil {
				log.Printf("insert gallery picture: %v", err)
				http.Error(w, "failed to save the picture", http.StatusInternalServerError)
				return
			}
		}
		touchEntry(db, entryID)
		writeJSON(w, loadGallery(db, entryID))
	}
}

// orderGalleryHandler puts an article's pictures in the order of { ids }.
func orderGalleryHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		typ, id, ok := galleryArticle(r)
		entryID, exists, err := articleEntry(db, typ, id, false)
		if !ok || !exists || err != nil || entryID == 0 {
			http.Error(w, "article not found", http.StatusNotFound)
			return
		}
		var body struct {
			IDs []int64 `json:"ids"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body) != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to save the order", http.StatusInternalServerError)
			return
		}
		for i, gid := range body.IDs {
			tx.Exec(`UPDATE wiki_gallery SET position = ? WHERE id = ? AND entry_id = ?`, i, gid, entryID)
		}
		if tx.Commit() != nil {
			http.Error(w, "failed to save the order", http.StatusInternalServerError)
			return
		}
		writeJSON(w, loadGallery(db, entryID))
	}
}

func galleryRow(db *sql.DB, r *http.Request) (gid, entryID int64, file string, ok bool) {
	gid, err := strconv.ParseInt(r.PathValue("gid"), 10, 64)
	if err != nil {
		return 0, 0, "", false
	}
	if db.QueryRow(`SELECT entry_id, file_path FROM wiki_gallery WHERE id = ?`, gid).Scan(&entryID, &file) != nil {
		return 0, 0, "", false
	}
	return gid, entryID, file, true
}

// captionGalleryHandler changes one picture's caption.
func captionGalleryHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gid, entryID, _, ok := galleryRow(db, r)
		if !ok {
			http.Error(w, "picture not found", http.StatusNotFound)
			return
		}
		var body struct {
			Caption string `json:"caption"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body) != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		body.Caption = strings.TrimSpace(body.Caption)
		if tooLong(body.Caption, maxCaption) {
			http.Error(w, "that text is too long", http.StatusBadRequest)
			return
		}
		db.Exec(`UPDATE wiki_gallery SET caption = ? WHERE id = ?`, body.Caption, gid)
		touchEntry(db, entryID)
		writeJSON(w, loadGallery(db, entryID))
	}
}

func deleteGalleryHandler(db *sql.DB, uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gid, entryID, file, ok := galleryRow(db, r)
		if !ok {
			http.Error(w, "picture not found", http.StatusNotFound)
			return
		}
		if _, err := db.Exec(`DELETE FROM wiki_gallery WHERE id = ?`, gid); err != nil {
			http.Error(w, "failed to delete the picture", http.StatusInternalServerError)
			return
		}
		removePicture(uploadsDir, file)
		out := loadGallery(db, entryID)
		pruneEntry(db, entryID)
		writeJSON(w, out)
	}
}
