package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

// Uploading a Word file into a book (docs/EDITOR.md, "The server"):
// parseDocx (docx.go) splits it into chapters; here its pictures go into
// the story's uploads and the chapters are added at the end of the book,
// each with its text. The book's settings are left as they are; the answer
// carries what the file used (page, guessed page type, styles, fonts) and
// what was left out, for the client to offer.

const maxDocxUpload = 64 << 20

// insertImportedComments makes the rows of one imported chapter's comments
// (chapter: its index in the import) and gives each comment's new id by its
// Local: the comments first, then the replies.
func insertImportedComments(tx editorDB, chapterID int64, chapter int, list []docxComment, now time.Time) (map[int64]int64, error) {
	ids := map[int64]int64{}
	ordered := make([]docxComment, 0, len(list))
	for _, reply := range []bool{false, true} {
		for _, c := range list {
			if c.Chapter == chapter && (c.Parent != 0) == reply {
				ordered = append(ordered, c)
			}
		}
	}
	for _, c := range ordered {
		var parent any
		if c.Parent != 0 {
			p, ok := ids[c.Parent]
			if !ok {
				continue
			}
			parent = p
		}
		at := c.Created
		if at.IsZero() || at.After(now) {
			at = now
		}
		var resolvedAt any
		if c.Resolved && c.Parent == 0 {
			resolvedAt = sqlTime(now)
		}
		res, err := tx.Exec(`INSERT INTO chapter_comments (chapter_id, parent_id, text, resolved, resolved_at, anchored, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, 1, ?, ?)`, chapterID, parent, c.Text, c.Resolved && c.Parent == 0, resolvedAt, sqlTime(at), sqlTime(at))
		if err != nil {
			return nil, err
		}
		ids[c.Local], _ = res.LastInsertId()
	}
	return ids, nil
}

// docxPictureExts are the picture types an import keeps (the ones parseDocx
// sniffs; anything else is never written to the uploads).
var docxPictureExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true}

type importedChapter struct {
	ID            int64  `json:"id"`
	Title         string `json:"title"`
	Words         int    `json:"words"`
	FrontMatter   bool   `json:"front_matter,omitempty"`   // the text before the first Heading 1
	NumberRestart *int   `json:"number_restart,omitempty"` // a page number restart the file has (docxlayout.go)
}

func importDocxHandler(db *sql.DB, uploadsDir string) http.HandlerFunc {
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
				http.Error(w, "failed to import the file", http.StatusInternalServerError)
			}
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxDocxUpload+(1<<20))
		if err := r.ParseMultipartForm(maxFormMemory); err != nil {
			var tooBig *http.MaxBytesError
			if errors.As(err, &tooBig) {
				http.Error(w, errDocxTooBig.Error(), http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "invalid form data", http.StatusBadRequest)
			return
		}
		defer r.MultipartForm.RemoveAll()
		if mode := r.FormValue("mode"); mode != "" && mode != "append" {
			http.Error(w, "unknown import mode", http.StatusBadRequest)
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "choose a .docx file", http.StatusBadRequest)
			return
		}
		data, err := io.ReadAll(io.LimitReader(file, maxDocxUpload+1))
		file.Close()
		if err != nil {
			http.Error(w, "invalid form data", http.StatusBadRequest)
			return
		}
		if len(data) > maxDocxUpload {
			http.Error(w, errDocxTooBig.Error(), http.StatusRequestEntityTooLarge)
			return
		}

		imp, err := parseDocx(data)
		switch {
		case errors.Is(err, errDocxTooBig):
			http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
			return
		case errors.Is(err, errDocxNotZip), errors.Is(err, errDocxNoDocument):
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		case err != nil:
			log.Printf("read .docx %q: %v", header.Filename, err)
			http.Error(w, "this .docx file is damaged and can't be read", http.StatusBadRequest)
			return
		}
		if len(imp.Chapters) == 0 {
			http.Error(w, "this .docx file has no text to import", http.StatusBadRequest)
			return
		}

		// The pictures first: the docs only hold their addresses.
		var saved []string
		dropPictures := func() {
			for _, rel := range saved {
				removePicture(uploadsDir, rel)
			}
		}
		urls := make([]string, len(imp.Pictures))
		stamp := time.Now().UnixNano()
		for i, p := range imp.Pictures {
			if !docxPictureExts[strings.ToLower(filepath.Ext(p.Name))] {
				continue // its images are dropped
			}
			rel, err := saveUpload(uploadsDir, "editor", fmt.Sprintf("%d-%d-%d", vol, stamp, i), p.Name, bytes.NewReader(p.Data))
			if err != nil {
				dropPictures()
				log.Printf("save imported picture: %v", err)
				http.Error(w, "failed to save the file's pictures", http.StatusInternalServerError)
				return
			}
			saved = append(saved, rel)
			urls[i] = uploadURL(rel)
		}
		imp.usePictureURLs(urls)

		type newChapter struct {
			title   string
			doc     []byte
			words   int
			front   bool
			restart *int
			node    *pmNode
			index   int
		}
		chapters := make([]newChapter, 0, len(imp.Chapters))
		for i, c := range imp.Chapters {
			if err := validatePMDoc(c.Doc); err != nil {
				dropPictures()
				log.Printf("imported chapter %d of %q: %v", i+1, header.Filename, err)
				http.Error(w, "this .docx file could not be turned into chapters", http.StatusBadRequest)
				return
			}
			doc, err := json.Marshal(c.Doc)
			if err != nil {
				dropPictures()
				http.Error(w, "this .docx file could not be turned into chapters", http.StatusBadRequest)
				return
			}
			if len(doc) > maxDocBytes {
				dropPictures()
				http.Error(w, fmt.Sprintf("chapter %d of this file is too long for one chapter; split it with a Heading 1", i+1), http.StatusRequestEntityTooLarge)
				return
			}
			title := strings.TrimSpace(c.Title)
			if c.FrontMatter {
				title = docxFrontMatterTitle
			}
			if title == "" {
				title = fmt.Sprintf("Chapter %d", i+1)
			}
			if r := []rune(title); len(r) > 200 {
				title = string(r[:200])
			}
			chapters = append(chapters, newChapter{title, doc, docWords(c.Doc), c.FrontMatter, nil, c.Doc, i})
			if n, ok := imp.Restarts[i]; ok {
				chapters[len(chapters)-1].restart = &n
			}
		}

		now := sqlTime(editorNow())
		out := []importedChapter{}
		tx, err := db.Begin()
		if err != nil {
			dropPictures()
			http.Error(w, "failed to import the file", http.StatusInternalServerError)
			return
		}
		fail := func(err error) {
			tx.Rollback()
			dropPictures()
			log.Printf("import .docx into book %d: %v", vol, err)
			http.Error(w, "failed to import the file", http.StatusInternalServerError)
		}
		for _, c := range chapters {
			res, err := tx.Exec(`INSERT INTO chapters (volume_id, title, position)
				SELECT ?, ?, COALESCE(MAX(position), -1) + 1 FROM chapters WHERE volume_id IS ?`,
				volumeNull(vol), c.title, volumeNull(vol))
			if err != nil {
				fail(err)
				return
			}
			id, _ := res.LastInsertId()
			// The chapter's comments (docxcomments.go): their rows first, then
			// their ids in the marks.
			if ids, err := insertImportedComments(tx, id, c.index, imp.Comments, editorNow()); err != nil {
				fail(err)
				return
			} else if len(ids) > 0 {
				mapCommentMarks(c.node, ids)
				if c.doc, err = json.Marshal(c.node); err != nil {
					fail(err)
					return
				}
			}
			if _, err := tx.Exec(`INSERT INTO chapter_texts (chapter_id, doc, version, words, updated_at, auto_since)
				VALUES (?, ?, 1, ?, ?, ?)`, id, string(c.doc), c.words, now, now); err != nil {
				fail(err)
				return
			}
			if err := indexChapterText(tx, id, string(c.doc)); err != nil { // chapterindex.go
				fail(err)
				return
			}
			out = append(out, importedChapter{ID: id, Title: c.title, Words: c.words, FrontMatter: c.front, NumberRestart: c.restart})
		}
		if err := markFrontMatter(tx, vol, out); err != nil {
			fail(err)
			return
		}
		if err := tx.Commit(); err != nil {
			fail(err)
			return
		}
		// imp gives page, page_type_guess, styles, fonts and left_out.
		writeJSON(w, struct {
			Chapters []importedChapter `json:"chapters"`
			*docxImport
		}{out, imp})
	}
}

// markFrontMatter marks the chapters made of the text before the file's
// first Heading 1 as front matter in the book's chapter options, whether or
// not the file's layout is taken afterwards, so they are never counted as
// chapters. A book that was never set up stays so.
func markFrontMatter(db editorDB, vol int64, chapters []importedChapter) error {
	var ids []int64
	for _, c := range chapters {
		if c.FrontMatter {
			ids = append(ids, c.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	raw, setUp, err := loadBookSettings(db, vol)
	if err != nil {
		return err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	opts := map[string]map[string]any{}
	if r := m["chapter_options"]; len(r) > 0 && string(r) != "null" {
		if err := json.Unmarshal(r, &opts); err != nil {
			return err
		}
	}
	for _, id := range ids {
		key := fmt.Sprint(id)
		if opts[key] == nil {
			opts[key] = map[string]any{}
		}
		opts[key]["front_matter"] = true
	}
	if m["chapter_options"], err = json.Marshal(opts); err != nil {
		return err
	}
	settings, err := json.Marshal(m)
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO book_settings (volume_id, settings, is_set_up, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(volume_id) DO UPDATE SET settings = excluded.settings, updated_at = excluded.updated_at`,
		vol, string(settings), setUp, sqlTime(editorNow()))
	return err
}
