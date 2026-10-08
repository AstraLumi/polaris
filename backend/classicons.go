package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

// Class icons are uploaded separately from the class form's JSON body:
// the form saves the class, then PUTs the picture here. That keeps the
// create/update handlers shared with races and body types unchanged.

func setClassIconHandler(db *sql.DB, uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "class not found", http.StatusNotFound)
			return
		}
		var old sql.NullString
		if err := db.QueryRow(`SELECT icon_path FROM classes WHERE id = ?`, id).Scan(&old); err != nil {
			http.Error(w, "class not found", http.StatusNotFound)
			return
		}
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, "invalid form data", http.StatusBadRequest)
			return
		}
		file, header, err := r.FormFile("icon")
		if err != nil {
			http.Error(w, "an icon file is required", http.StatusBadRequest)
			return
		}
		defer file.Close()

		// The timestamp gives a replaced icon a fresh URL, so the browser
		// can't keep showing the old one from its cache.
		stem := fmt.Sprintf("%d-%d", id, time.Now().Unix())
		relPath, err := saveUpload(uploadsDir, "classes", stem, header.Filename, file)
		if err != nil {
			http.Error(w, "failed to save icon", http.StatusInternalServerError)
			log.Printf("save class icon: %v", err)
			return
		}
		if _, err := db.Exec(`UPDATE classes SET icon_path = ? WHERE id = ?`, relPath, id); err != nil {
			http.Error(w, "failed to save icon", http.StatusInternalServerError)
			log.Printf("update class icon_path: %v", err)
			return
		}
		if old.Valid && old.String != relPath {
			removePicture(uploadsDir, old.String)
		}
		writeJSON(w, map[string]any{"id": id, "icon_path": uploadURL(relPath)})
	}
}

func clearClassIconHandler(db *sql.DB, uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "class not found", http.StatusNotFound)
			return
		}
		var old sql.NullString
		if err := db.QueryRow(`SELECT icon_path FROM classes WHERE id = ?`, id).Scan(&old); err != nil {
			http.Error(w, "class not found", http.StatusNotFound)
			return
		}
		if _, err := db.Exec(`UPDATE classes SET icon_path = NULL WHERE id = ?`, id); err != nil {
			http.Error(w, "failed to clear icon", http.StatusInternalServerError)
			return
		}
		if old.Valid {
			removePicture(uploadsDir, old.String)
		}
		writeJSON(w, map[string]any{"id": id, "icon_path": ""})
	}
}
