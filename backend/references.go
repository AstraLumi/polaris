package main

// The reference library: real-world pictures the writer collects for
// inspiration (an outfit seen online, say), each with a title, the link it
// came from, tags and a description. One library for every story, like the
// fonts library (fontlib.go): the story's own designs (culture elements,
// cultures.go) point at references by id.
//
// Storage: three tables in the registry (<DATA_DIR>/stories.db) and one
// folder, <UPLOADS_DIR>/_references/, which can't be a story's (story ids
// are 16 hex digits). A picture is named by 32 random hex digits and its
// extension. The source link is only stored and shown, never fetched.
//
// API (global, not under a story):
//
//	GET    /api/references              every reference with its pictures and tags
//	POST   /api/references              add one (multipart, see saveReference)
//	PUT    /api/references/{id}         change one (multipart)
//	DELETE /api/references/{id}         the reference and its pictures
//	GET    /api/references/used         which stories' designs use each one
//	GET    /uploads/references/{name}   a picture

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	referencesDirName     = "_references"
	maxReferencePictures  = 40
	maxReferenceTags      = 40
	maxReferenceForm      = 256 << 20
	maxReferencePicture   = int64(32 << 20)
	maxReferenceTitle     = 200
	maxReferenceText      = 20000
	maxReferenceSourceURL = 2000
)

// Kinds of reference; clothing first, the rest for later use.
var referenceKinds = map[string]bool{
	"clothing": true, "armor": true, "weapon": true, "hair": true,
	"jewelry": true, "architecture": true, "other": true,
}

// The picture types a reference takes, by extension, with the type the
// file is served as.
var referenceImageTypes = map[string]string{
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg",
	".gif": "image/gif", ".webp": "image/webp", ".avif": "image/avif",
}

var referenceFilePattern = regexp.MustCompile(`^[0-9a-f]{32}\.(png|jpg|jpeg|gif|webp|avif)$`)

const referenceTablesSQL = `
CREATE TABLE IF NOT EXISTS reference_items (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	title       TEXT NOT NULL,
	kind        TEXT NOT NULL DEFAULT 'clothing',
	source_url  TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT '',
	created_at  TEXT NOT NULL DEFAULT (datetime('now')),
	updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE TABLE IF NOT EXISTS reference_pictures (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	reference_id INTEGER NOT NULL REFERENCES reference_items(id) ON DELETE CASCADE,
	position     INTEGER NOT NULL DEFAULT 0,
	file         TEXT NOT NULL UNIQUE
);
CREATE INDEX IF NOT EXISTS idx_reference_pictures_ref ON reference_pictures(reference_id);
CREATE TABLE IF NOT EXISTS reference_tags (
	reference_id INTEGER NOT NULL REFERENCES reference_items(id) ON DELETE CASCADE,
	tag          TEXT NOT NULL COLLATE NOCASE,
	PRIMARY KEY (reference_id, tag)
);
`

type referenceLibrary struct {
	db  *sql.DB // the registry
	dir string  // <UPLOADS_DIR>/_references
	mgr *storyManager
}

func newReferenceLibrary(reg *sql.DB, uploadsRoot string, mgr *storyManager) (*referenceLibrary, error) {
	if _, err := reg.Exec(referenceTablesSQL); err != nil {
		return nil, fmt.Errorf("create the reference library: %w", err)
	}
	dir := filepath.Join(uploadsRoot, referencesDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &referenceLibrary{db: reg, dir: dir, mgr: mgr}, nil
}

type referencePicture struct {
	ID  int64  `json:"id"`
	URL string `json:"url"`
}

type referenceOut struct {
	ID          int64              `json:"id"`
	Title       string             `json:"title"`
	Kind        string             `json:"kind"`
	SourceURL   string             `json:"source_url"`
	Description string             `json:"description"`
	Pictures    []referencePicture `json:"pictures"`
	Tags        []string           `json:"tags"`
	CreatedAt   string             `json:"created_at"`
	UpdatedAt   string             `json:"updated_at"`
}

func referencePictureURL(file string) string { return "/uploads/references/" + file }

// list reads the whole library in three queries, newest first.
func (l *referenceLibrary) list(onlyID int64) ([]referenceOut, error) {
	where := ""
	args := []any{}
	if onlyID > 0 {
		where = " WHERE id = ?"
		args = append(args, onlyID)
	}
	rows, err := l.db.Query(`SELECT id, title, kind, source_url, description, created_at, updated_at
		FROM reference_items`+where+` ORDER BY created_at DESC, id DESC`, args...)
	if err != nil {
		return nil, err
	}
	out := []referenceOut{}
	at := map[int64]int{}
	for rows.Next() {
		var r referenceOut
		if err := rows.Scan(&r.ID, &r.Title, &r.Kind, &r.SourceURL, &r.Description, &r.CreatedAt, &r.UpdatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		r.Pictures, r.Tags = []referencePicture{}, []string{}
		at[r.ID] = len(out)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	if len(out) == 0 {
		return out, nil
	}

	rows, err = l.db.Query(`SELECT id, reference_id, file FROM reference_pictures ORDER BY reference_id, position, id`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var p referencePicture
		var ref int64
		var file string
		if rows.Scan(&p.ID, &ref, &file) != nil {
			continue
		}
		if i, ok := at[ref]; ok {
			p.URL = referencePictureURL(file)
			out[i].Pictures = append(out[i].Pictures, p)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	rows, err = l.db.Query(`SELECT reference_id, tag FROM reference_tags ORDER BY reference_id, tag COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var ref int64
		var tag string
		if rows.Scan(&ref, &tag) != nil {
			continue
		}
		if i, ok := at[ref]; ok {
			out[i].Tags = append(out[i].Tags, tag)
		}
	}
	return out, rows.Err()
}

func (l *referenceLibrary) listHandler(w http.ResponseWriter, r *http.Request) {
	list, err := l.list(0)
	if err != nil {
		log.Printf("list references: %v", err)
		http.Error(w, "failed to load the references", http.StatusInternalServerError)
		return
	}
	writeJSON(w, list)
}

// cleanSourceURL accepts an empty link or a web address (http or https),
// so a stored link can never run a script when clicked.
func cleanSourceURL(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", true
	}
	if len(raw) > maxReferenceSourceURL {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", false
	}
	return u.String(), true
}

// tagsFromJSON reads a JSON list of tags and cleans it as an event's are
// (cleanTags), with the library's own limit on how many.
func tagsFromJSON(raw string) ([]string, string) {
	if strings.TrimSpace(raw) == "" {
		return []string{}, ""
	}
	var in []string
	if json.Unmarshal([]byte(raw), &in) != nil {
		return nil, "invalid tag list"
	}
	if len(in) > maxReferenceTags {
		return nil, "too many tags"
	}
	return cleanTags(in)
}

// idList reads a JSON list of ids ("" is none).
func idList(raw string) ([]int64, bool) {
	if strings.TrimSpace(raw) == "" {
		return nil, true
	}
	var ids []int64
	return ids, json.Unmarshal([]byte(raw), &ids) == nil && len(ids) <= 10000
}

// newPictureName is 32 random hex digits and the extension.
func newPictureName(ext string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b) + ext, nil
}

// pictureExt checks an upload is a picture: a known extension and contents
// that look like one. It gives the extension the file is saved with.
func pictureExt(name string, data []byte) (string, bool) {
	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		// A picture pasted from the clipboard may come without a name.
		switch http.DetectContentType(data) {
		case "image/png":
			ext = ".png"
		case "image/jpeg":
			ext = ".jpg"
		case "image/gif":
			ext = ".gif"
		case "image/webp":
			ext = ".webp"
		}
	}
	if _, ok := referenceImageTypes[ext]; !ok {
		return "", false
	}
	// AVIF isn't known to DetectContentType; the rest must look like images.
	if ext != ".avif" && !strings.HasPrefix(http.DetectContentType(data), "image/") {
		return "", false
	}
	return ext, true
}

// saveHandler adds (no id) or changes a reference in one multipart request:
//
//	title, kind, source_url, description   the fields
//	tags                                   JSON list of tags
//	pictures                               files to add, at the end
//	remove_pictures                        JSON list of picture ids to drop
//	order                                  JSON list of picture ids, in order
func (l *referenceLibrary) saveHandler(w http.ResponseWriter, r *http.Request) {
	var id int64
	if raw := r.PathValue("id"); raw != "" {
		var err error
		if id, err = strconv.ParseInt(raw, 10, 64); err != nil ||
			l.db.QueryRow(`SELECT id FROM reference_items WHERE id = ?`, id).Scan(&id) != nil {
			http.Error(w, "reference not found", http.StatusNotFound)
			return
		}
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxReferenceForm)
	if err := r.ParseMultipartForm(maxFormMemory); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			http.Error(w, "those pictures are too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()

	title := strings.TrimSpace(r.FormValue("title"))
	desc := strings.TrimSpace(r.FormValue("description"))
	kind := strings.TrimSpace(r.FormValue("kind"))
	if kind == "" {
		kind = "clothing"
	}
	if title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	if len(title) > maxReferenceTitle || len(desc) > maxReferenceText {
		http.Error(w, "that text is too long", http.StatusBadRequest)
		return
	}
	if !referenceKinds[kind] {
		http.Error(w, "unknown kind", http.StatusBadRequest)
		return
	}
	source, ok := cleanSourceURL(r.FormValue("source_url"))
	if !ok {
		http.Error(w, "the source must be a web address (http or https)", http.StatusBadRequest)
		return
	}
	tags, msg := tagsFromJSON(r.FormValue("tags"))
	if msg != "" {
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	remove, ok1 := idList(r.FormValue("remove_pictures"))
	order, ok2 := idList(r.FormValue("order"))
	if !ok1 || !ok2 {
		http.Error(w, "invalid picture list", http.StatusBadRequest)
		return
	}

	// Read and check the new pictures before anything is written.
	type upload struct {
		data []byte
		ext  string
	}
	var uploads []upload
	for _, fh := range r.MultipartForm.File["pictures"] {
		data, err := readFormFile(fh, maxReferencePicture)
		if errors.Is(err, errTooLarge) {
			http.Error(w, "that picture is too large (over 32 MB)", http.StatusRequestEntityTooLarge)
			return
		}
		if err != nil {
			http.Error(w, "invalid form data", http.StatusBadRequest)
			return
		}
		ext, ok := pictureExt(fh.Filename, data)
		if !ok {
			http.Error(w, "pictures must be PNG, JPEG, GIF, WebP or AVIF", http.StatusBadRequest)
			return
		}
		uploads = append(uploads, upload{data, ext})
	}
	var have int
	if id != 0 {
		l.db.QueryRow(`SELECT COUNT(*) FROM reference_pictures WHERE reference_id = ?`, id).Scan(&have)
	}
	if have-len(remove)+len(uploads) > maxReferencePictures {
		http.Error(w, fmt.Sprintf("a reference holds at most %d pictures", maxReferencePictures), http.StatusBadRequest)
		return
	}

	// The files first: a failed save leaves at most unused files, removed
	// below.
	var written []string
	for _, u := range uploads {
		name, err := newPictureName(u.ext)
		if err == nil {
			err = os.WriteFile(filepath.Join(l.dir, name), u.data, 0o644)
		}
		if err != nil {
			log.Printf("save reference picture: %v", err)
			l.removeFiles(written)
			http.Error(w, "failed to save the reference", http.StatusInternalServerError)
			return
		}
		written = append(written, name)
	}

	gone, err := l.write(id, title, kind, source, desc, tags, written, remove, order, &id)
	if err != nil {
		log.Printf("save reference: %v", err)
		l.removeFiles(written)
		http.Error(w, "failed to save the reference", http.StatusInternalServerError)
		return
	}
	l.removeFiles(gone)
	list, err := l.list(id)
	if err != nil || len(list) == 0 {
		http.Error(w, "failed to save the reference", http.StatusInternalServerError)
		return
	}
	writeJSON(w, list[0])
}

// write saves a reference's rows in one transaction and gives the files of
// the pictures it dropped. newID receives the id of a new reference.
func (l *referenceLibrary) write(id int64, title, kind, source, desc string, tags, files []string,
	remove, order []int64, newID *int64) ([]string, error) {
	tx, err := l.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if id == 0 {
		res, err := tx.Exec(`INSERT INTO reference_items (title, kind, source_url, description) VALUES (?, ?, ?, ?)`,
			title, kind, source, desc)
		if err != nil {
			return nil, err
		}
		id, _ = res.LastInsertId()
	} else if _, err := tx.Exec(`UPDATE reference_items SET title = ?, kind = ?, source_url = ?, description = ?,
		updated_at = datetime('now') WHERE id = ?`, title, kind, source, desc, id); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`DELETE FROM reference_tags WHERE reference_id = ?`, id); err != nil {
		return nil, err
	}
	for _, t := range tags {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO reference_tags (reference_id, tag) VALUES (?, ?)`, id, t); err != nil {
			return nil, err
		}
	}
	var gone []string
	for _, pid := range remove {
		var file string
		if tx.QueryRow(`SELECT file FROM reference_pictures WHERE id = ? AND reference_id = ?`, pid, id).Scan(&file) != nil {
			continue
		}
		if _, err := tx.Exec(`DELETE FROM reference_pictures WHERE id = ?`, pid); err != nil {
			return nil, err
		}
		gone = append(gone, file)
	}
	for i, pid := range order {
		if _, err := tx.Exec(`UPDATE reference_pictures SET position = ? WHERE id = ? AND reference_id = ?`, i, pid, id); err != nil {
			return nil, err
		}
	}
	var last int
	tx.QueryRow(`SELECT COALESCE(MAX(position), -1) FROM reference_pictures WHERE reference_id = ?`, id).Scan(&last)
	for _, f := range files {
		last++
		if _, err := tx.Exec(`INSERT INTO reference_pictures (reference_id, position, file) VALUES (?, ?, ?)`, id, last, f); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	*newID = id
	return gone, nil
}

func (l *referenceLibrary) removeFiles(files []string) {
	for _, f := range files {
		if referenceFilePattern.MatchString(f) {
			os.Remove(filepath.Join(l.dir, f))
		}
	}
}

func (l *referenceLibrary) deleteHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "reference not found", http.StatusNotFound)
		return
	}
	var files []string
	rows, err := l.db.Query(`SELECT file FROM reference_pictures WHERE reference_id = ?`, id)
	if err == nil {
		for rows.Next() {
			var f string
			if rows.Scan(&f) == nil {
				files = append(files, f)
			}
		}
		rows.Close()
	}
	res, err := l.db.Exec(`DELETE FROM reference_items WHERE id = ?`, id)
	if err != nil {
		log.Printf("delete reference: %v", err)
		http.Error(w, "failed to delete the reference", http.StatusInternalServerError)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		http.Error(w, "reference not found", http.StatusNotFound)
		return
	}
	l.removeFiles(files)
	writeJSON(w, map[string]any{"deleted": id})
}

// referenceUse is one story design that points at a reference.
type referenceUse struct {
	StoryID   string `json:"story_id"`
	Story     string `json:"story"`
	CultureID int64  `json:"culture_id"`
	Culture   string `json:"culture"`
	ElementID int64  `json:"element_id"`
	Element   string `json:"element"`
}

// usedHandler scans every story for the designs that use each reference
// (by id), as the fonts library does for its families.
func (l *referenceLibrary) usedHandler(w http.ResponseWriter, r *http.Request) {
	out := map[string][]referenceUse{}
	if l.mgr == nil {
		writeJSON(w, out)
		return
	}
	type story struct{ id, name string }
	var stories []story
	rows, err := l.db.Query(`SELECT id, name FROM stories ORDER BY created_at, rowid`)
	if err != nil {
		http.Error(w, "failed to load the references", http.StatusInternalServerError)
		return
	}
	for rows.Next() {
		var s story
		rows.Scan(&s.id, &s.name)
		stories = append(stories, s)
	}
	rows.Close()
	for _, s := range stories {
		app, done, err := l.mgr.use(s.id)
		if err != nil {
			continue // a story at a newer schema, say
		}
		uses, err := referenceUses(app.db)
		done()
		if err != nil {
			log.Printf("references used in story %s: %v", s.id, err)
			continue
		}
		for _, u := range uses {
			key := strconv.FormatInt(u.ref, 10)
			u.use.StoryID, u.use.Story = s.id, s.name
			out[key] = append(out[key], u.use)
		}
	}
	writeJSON(w, out)
}

type storyReferenceUse struct {
	ref int64
	use referenceUse
}

func referenceUses(db *sql.DB) ([]storyReferenceUse, error) {
	rows, err := db.Query(`SELECT r.reference_id, c.id, c.name, e.id, e.name
		FROM culture_element_refs r
		JOIN culture_elements e ON e.id = r.element_id
		JOIN cultures c ON c.id = e.culture_id
		ORDER BY c.name COLLATE NOCASE, e.name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []storyReferenceUse
	for rows.Next() {
		var u storyReferenceUse
		if rows.Scan(&u.ref, &u.use.CultureID, &u.use.Culture, &u.use.ElementID, &u.use.Element) == nil {
			out = append(out, u)
		}
	}
	return out, rows.Err()
}

func (l *referenceLibrary) fileHandler(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !referenceFilePattern.MatchString(name) {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(filepath.Join(l.dir, name))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		http.NotFound(w, r)
		return
	}
	h := w.Header()
	h.Set("Content-Type", referenceImageTypes[strings.ToLower(filepath.Ext(name))])
	// A name is never reused, so the picture may be kept.
	h.Set("Cache-Control", "public, max-age=31536000, immutable")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "sandbox")
	http.ServeContent(w, r, "", st.ModTime(), f)
}

func (l *referenceLibrary) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/references", l.listHandler)
	mux.HandleFunc("POST /api/references", l.saveHandler)
	mux.HandleFunc("GET /api/references/used", l.usedHandler)
	mux.HandleFunc("PUT /api/references/{id}", l.saveHandler)
	mux.HandleFunc("DELETE /api/references/{id}", l.deleteHandler)
	mux.HandleFunc("GET /uploads/references/{name}", l.fileHandler)
}
