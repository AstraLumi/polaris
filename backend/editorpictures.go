package main

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// Pictures for the writing editor (docs/EDITOR.md, "Server"):
// POST /api/books/{volume}/pictures takes one picture (multipart `file`),
// checks what it really is (the same sniffing as the .docx import: PNG,
// JPEG, GIF or WebP, whatever the file is called) and keeps it under
// uploads/editor/. The answer's src goes in an `image` node. Pictures are
// not removed when the text stops using them (the text may come back from a
// saved version).

// A variable only so tests can lower it.
var maxEditorPicture = int64(32 << 20)

func uploadPictureHandler(db *sql.DB, uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vol, ok := bookVolume(r)
		if !ok {
			http.Error(w, "book not found", http.StatusNotFound)
			return
		}
		if _, err := bookTitle(db, vol); err != nil {
			if errors.Is(err, errNoBook) {
				http.Error(w, "book not found", http.StatusNotFound)
			} else {
				http.Error(w, "failed to save the picture", http.StatusInternalServerError)
			}
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxEditorPicture+(1<<20))
		if err := r.ParseMultipartForm(maxFormMemory); err != nil {
			var tooBig *http.MaxBytesError
			if errors.As(err, &tooBig) {
				http.Error(w, "that picture is too large (over 32 MB)", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "invalid form data", http.StatusBadRequest)
			return
		}
		defer r.MultipartForm.RemoveAll()
		file, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "choose a picture", http.StatusBadRequest)
			return
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, maxEditorPicture+1))
		if err != nil {
			http.Error(w, "invalid form data", http.StatusBadRequest)
			return
		}
		if int64(len(data)) > maxEditorPicture {
			http.Error(w, "that picture is too large (over 32 MB)", http.StatusRequestEntityTooLarge)
			return
		}
		_, ext, ok := sniffPicture(data)
		if !ok {
			http.Error(w, "this is not a picture the editor can use (PNG, JPEG, GIF or WebP)", http.StatusBadRequest)
			return
		}
		// Named by the book and the time, with the extension of what it is.
		rel, err := saveUpload(uploadsDir, "editor", fmt.Sprintf("%d-%d", vol, time.Now().UnixNano()), "picture"+ext, bytes.NewReader(data))
		if err != nil {
			log.Printf("save editor picture: %v", err)
			http.Error(w, "failed to save the picture", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"src": uploadURL(rel)})
	}
}
