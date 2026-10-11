package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Lore articles are the wiki's free-form pages: gods, magic systems, history,
// languages, anything that isn't a character, place or other thing in the
// story. Unlike every other article they have no source of their own, so this
// table holds just a name and a picture; the text lives in wiki_entries like
// the rest (introduction, trivia, the user's sections and infobox rows), and
// the delete trigger from wikiTriggerSQL("lore") clears it.

const loreTableSQL = `CREATE TABLE lore_articles (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	name         TEXT NOT NULL,
	picture_path TEXT,
	created_at   TEXT NOT NULL DEFAULT (datetime('now'))
)`

// loreSchemaStatements create what schema 17 added. The fresh schema runs
// them before the wiki's, since the wiki's triggers include lore's.
var loreSchemaStatements = []string{loreTableSQL}

func loreExists(db *sql.DB, id int64) bool {
	var one int
	return db.QueryRow(`SELECT 1 FROM lore_articles WHERE id = ?`, id).Scan(&one) == nil
}

func saveLorePicture(db *sql.DB, r *http.Request, uploadsDir string, id int64) {
	var old sql.NullString
	db.QueryRow(`SELECT picture_path FROM lore_articles WHERE id = ?`, id).Scan(&old)
	if r.FormValue("remove_picture") == "1" && old.Valid {
		db.Exec(`UPDATE lore_articles SET picture_path = NULL WHERE id = ?`, id)
		removePicture(uploadsDir, old.String)
		old = sql.NullString{}
	}
	file, header, err := r.FormFile("picture")
	if err != nil {
		return
	}
	defer file.Close()
	rel, err := saveUpload(uploadsDir, "lore", fmt.Sprintf("%d-%d", id, time.Now().Unix()), header.Filename, file)
	if err != nil {
		log.Printf("save lore picture: %v", err)
		return
	}
	if _, err := db.Exec(`UPDATE lore_articles SET picture_path = ? WHERE id = ?`, rel, id); err != nil {
		log.Printf("update lore picture: %v", err)
		return
	}
	if old.Valid && old.String != "" && old.String != rel {
		removePicture(uploadsDir, old.String)
	}
}

// saveLoreHandler creates (no id in the path) or renames a lore article and
// sets its picture, in one multipart request. It answers with { id }; the
// article itself comes from the wiki endpoints.
func saveLoreHandler(db *sql.DB, uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var id int64
		if raw := r.PathValue("id"); raw != "" {
			var err error
			if id, err = strconv.ParseInt(raw, 10, 64); err != nil || !loreExists(db, id) {
				http.Error(w, "article not found", http.StatusNotFound)
				return
			}
		}
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, "invalid form data", http.StatusBadRequest)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" && id != 0 && r.Form["name"] == nil {
			// Only the picture is changing.
			db.QueryRow(`SELECT name FROM lore_articles WHERE id = ?`, id).Scan(&name)
		}
		if name == "" {
			http.Error(w, "the article needs a title", http.StatusBadRequest)
			return
		}
		if tooLong(name, maxWikiTitle) || strings.ContainsAny(name, "[]|\n") {
			http.Error(w, "that title can't be used", http.StatusBadRequest)
			return
		}
		if id == 0 {
			res, err := db.Exec(`INSERT INTO lore_articles (name) VALUES (?)`, name)
			if err != nil {
				log.Printf("create lore article: %v", err)
				http.Error(w, "failed to save the article", http.StatusInternalServerError)
				return
			}
			id, _ = res.LastInsertId()
		} else if _, err := db.Exec(`UPDATE lore_articles SET name = ? WHERE id = ?`, name, id); err != nil {
			log.Printf("rename lore article: %v", err)
			http.Error(w, "failed to save the article", http.StatusInternalServerError)
			return
		}
		saveLorePicture(db, r, uploadsDir, id)
		writeJSON(w, map[string]any{"id": id})
	}
}

func deleteLoreHandler(db *sql.DB, uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "article not found", http.StatusNotFound)
			return
		}
		var pic sql.NullString
		if err := db.QueryRow(`SELECT picture_path FROM lore_articles WHERE id = ?`, id).Scan(&pic); err != nil {
			http.Error(w, "article not found", http.StatusNotFound)
			return
		}
		if _, err := db.Exec(`DELETE FROM lore_articles WHERE id = ?`, id); err != nil {
			log.Printf("delete lore article: %v", err)
			http.Error(w, "failed to delete the article", http.StatusInternalServerError)
			return
		}
		if pic.Valid {
			removePicture(uploadsDir, pic.String)
		}
		writeJSON(w, map[string]any{"deleted": id})
	}
}
