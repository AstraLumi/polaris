package main

// Stories: every story is a completely separate world — its own SQLite file
// and its own uploads folder — so nothing can leak between them and deleting
// one is just deleting two folders. A small registry database lists them
// (id, name, icon).
//
//	<DATA_DIR>/stories.db                 the registry
//	<DATA_DIR>/stories/<id>/story.db      one story's data
//	<UPLOADS_DIR>/<id>/...                that story's pictures (and its icon, under _story/)
//
// The existing handlers are reused untouched: each story gets its own copy of
// the route table bound to its own database (storyRoutes in main.go), and
// /api/s/{id}/... is dispatched to it with the prefix stripped.

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	storyIconDir  = "_story" // inside a story's uploads folder
	maxStoryName  = 60
	maxFormMemory = 10 << 20
)

// Story icons are drawn by the frontend like spell icons (builtin:<name>).
// Keep in sync with STORY_ICONS in frontend/src/builtinIcons.js.
var builtinStoryIcons = map[string]bool{
	"tome": true, "star": true, "compass": true, "castle": true, "crown": true,
	"sword": true, "shield": true, "mountain": true, "flame": true, "moon": true,
	"skull": true, "sparkles": true,
}

var storyIDPattern = regexp.MustCompile(`^[0-9a-f]{16}$`)

var errNoStory = errors.New("story not found")

type storyManager struct {
	dataDir     string
	uploadsRoot string
	reg         *sql.DB

	mu   sync.Mutex
	apps map[string]*storyApp
}

type storyApp struct {
	id         string
	db         *sql.DB
	dbPath     string
	uploadsDir string
	handler    http.Handler
}

// storyInfo is what the picker shows.
type storyInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Icon       string `json:"icon"` // "", "builtin:<name>" or an /uploads/s/<id>/... URL
	CreatedAt  string `json:"created_at"`
	Characters int    `json:"characters"`
}

func newStoryManager(dataDir, uploadsRoot string) (*storyManager, error) {
	for _, d := range []string{filepath.Join(dataDir, "stories"), uploadsRoot} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, fmt.Errorf("create %s: %w", d, err)
		}
	}
	reg, err := sql.Open("sqlite", filepath.Join(dataDir, "stories.db"))
	if err != nil {
		return nil, fmt.Errorf("open registry: %w", err)
	}
	if _, err := reg.Exec(`CREATE TABLE IF NOT EXISTS stories (
		id         TEXT PRIMARY KEY,
		name       TEXT NOT NULL,
		icon       TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`); err != nil {
		return nil, fmt.Errorf("create registry: %w", err)
	}
	return &storyManager{
		dataDir:     dataDir,
		uploadsRoot: uploadsRoot,
		reg:         reg,
		apps:        map[string]*storyApp{},
	}, nil
}

func (m *storyManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.apps {
		a.db.Close()
	}
	m.reg.Close()
}

func (m *storyManager) storyDir(id string) string   { return filepath.Join(m.dataDir, "stories", id) }
func (m *storyManager) dbPathFor(id string) string  { return filepath.Join(m.storyDir(id), "story.db") }
func (m *storyManager) uploadsFor(id string) string { return filepath.Join(m.uploadsRoot, id) }
func (m *storyManager) tmpDir() string              { return filepath.Join(m.dataDir, "tmp") }

func newStoryID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// open returns the story's running app, opening (and upgrading) its database
// on first use.
func (m *storyManager) open(id string) (*storyApp, error) {
	if !storyIDPattern.MatchString(id) {
		return nil, errNoStory
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if a, ok := m.apps[id]; ok {
		return a, nil
	}
	var one int
	if err := m.reg.QueryRow(`SELECT 1 FROM stories WHERE id = ?`, id).Scan(&one); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errNoStory
		}
		return nil, err
	}
	return m.openLocked(id)
}

func (m *storyManager) openLocked(id string) (*storyApp, error) {
	if err := os.MkdirAll(m.storyDir(id), 0o755); err != nil {
		return nil, err
	}
	uploads := m.uploadsFor(id)
	if err := os.MkdirAll(uploads, 0o755); err != nil {
		return nil, err
	}
	dbPath := m.dbPathFor(id)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, err
	}
	if err := ensureSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	a := &storyApp{id: id, db: db, dbPath: dbPath, uploadsDir: uploads, handler: storyRoutes(db, uploads)}
	m.apps[id] = a
	return a, nil
}

func validStoryName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", errors.New("a story needs a name")
	}
	if utf8.RuneCountInString(name) > maxStoryName {
		return "", errors.New("that story name is too long (60 characters at most)")
	}
	return name, nil
}

func isImageName(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".avif", ".bmp", ".ico":
		return true
	}
	return false
}

// safeRel cleans a relative path from the database or a zip and refuses
// anything that could point outside its folder.
func safeRel(rel string) (string, bool) {
	rel = strings.ReplaceAll(rel, "\\", "/")
	if rel == "" || strings.HasPrefix(rel, "/") {
		return "", false
	}
	clean := path.Clean(rel)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "\x00") {
		return "", false
	}
	return clean, true
}

// iconValue turns a story icon as stored ("", "builtin:x", "_story/icon.png")
// into what the API returns.
func iconValue(id, stored string) string {
	if stored == "" || isBuiltinIcon(stored) {
		return stored
	}
	return "/uploads/s/" + id + "/" + stored
}

func (m *storyManager) info(id, name, icon, created string) storyInfo {
	si := storyInfo{ID: id, Name: name, Icon: iconValue(id, icon), CreatedAt: created}
	if a, err := m.open(id); err == nil {
		a.db.QueryRow(`SELECT COUNT(*) FROM characters`).Scan(&si.Characters)
	}
	return si
}

func (m *storyManager) list() ([]storyInfo, error) {
	rows, err := m.reg.Query(`SELECT id, name, icon, created_at FROM stories ORDER BY created_at, rowid`)
	if err != nil {
		return nil, err
	}
	type row struct{ id, name, icon, created string }
	var all []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.name, &r.icon, &r.created); err != nil {
			rows.Close()
			return nil, err
		}
		all = append(all, r)
	}
	rows.Close()
	out := make([]storyInfo, 0, len(all))
	for _, r := range all {
		out = append(out, m.info(r.id, r.name, r.icon, r.created))
	}
	return out, nil
}

func (m *storyManager) get(id string) (storyInfo, string, error) {
	if !storyIDPattern.MatchString(id) {
		return storyInfo{}, "", errNoStory
	}
	var name, icon, created string
	err := m.reg.QueryRow(`SELECT name, icon, created_at FROM stories WHERE id = ?`, id).Scan(&name, &icon, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return storyInfo{}, "", errNoStory
	}
	if err != nil {
		return storyInfo{}, "", err
	}
	return m.info(id, name, icon, created), icon, nil
}

// iconUpload is an optional uploaded image for a story icon.
type iconUpload struct {
	filename string
	r        io.Reader
}

// resolveIcon applies the form's icon fields: an upload wins, otherwise the
// choice ("none" or builtin:<name>). changed is false when the form says
// nothing about the icon.
func resolveIcon(uploadsDir string, up *iconUpload, choice string) (stored string, changed bool, err error) {
	if up != nil {
		if !isImageName(up.filename) {
			return "", false, errors.New("the icon must be an image (png, jpg, gif, webp or svg)")
		}
		stem := fmt.Sprintf("icon-%d", time.Now().UnixNano())
		rel, err := saveUpload(uploadsDir, storyIconDir, stem, up.filename, up.r)
		if err != nil {
			return "", false, errors.New("failed to save the icon")
		}
		return rel, true, nil
	}
	choice = strings.TrimSpace(choice)
	switch {
	case choice == "none":
		return "", true, nil
	case isBuiltinIcon(choice) && builtinStoryIcons[strings.TrimPrefix(choice, builtinIconPrefix)]:
		return choice, true, nil
	}
	return "", false, nil
}

// create makes a new story. copyFrom, when set, copies that story's
// Character Assets library into it (see storyassets.go).
func (m *storyManager) create(name string, up *iconUpload, choice, copyFrom string) (storyInfo, error) {
	name, err := validStoryName(name)
	if err != nil {
		return storyInfo{}, err
	}
	var src *storyApp
	if copyFrom != "" {
		if src, err = m.open(copyFrom); err != nil {
			return storyInfo{}, errors.New("the story to copy from doesn't exist")
		}
	}
	id, err := newStoryID()
	if err != nil {
		return storyInfo{}, err
	}
	fail := func(err error) (storyInfo, error) {
		m.removeStory(id)
		return storyInfo{}, err
	}

	if err := os.MkdirAll(m.uploadsFor(id), 0o755); err != nil {
		return fail(err)
	}
	icon, _, err := resolveIcon(m.uploadsFor(id), up, choice)
	if err != nil {
		return fail(err)
	}
	if _, err := m.reg.Exec(`INSERT INTO stories (id, name, icon) VALUES (?, ?, ?)`, id, name, icon); err != nil {
		return fail(err)
	}
	m.mu.Lock()
	app, err := m.openLocked(id)
	m.mu.Unlock()
	if err != nil {
		return fail(err)
	}
	if src != nil {
		if err := copyAssets(app.db, src.dbPath, src.uploadsDir, app.uploadsDir); err != nil {
			log.Printf("copy assets %s -> %s: %v", src.id, id, err)
			return fail(errors.New("couldn't copy the Character Assets"))
		}
	}
	si, _, err := m.get(id)
	return si, err
}

func (m *storyManager) update(id string, name *string, up *iconUpload, choice string) (storyInfo, error) {
	_, oldIcon, err := m.get(id)
	if err != nil {
		return storyInfo{}, err
	}
	if name != nil {
		n, err := validStoryName(*name)
		if err != nil {
			return storyInfo{}, err
		}
		if _, err := m.reg.Exec(`UPDATE stories SET name = ? WHERE id = ?`, n, id); err != nil {
			return storyInfo{}, err
		}
	}
	icon, changed, err := resolveIcon(m.uploadsFor(id), up, choice)
	if err != nil {
		return storyInfo{}, err
	}
	if changed {
		if _, err := m.reg.Exec(`UPDATE stories SET icon = ? WHERE id = ?`, icon, id); err != nil {
			return storyInfo{}, err
		}
		if oldIcon != icon {
			removePicture(m.uploadsFor(id), oldIcon)
		}
	}
	si, _, err := m.get(id)
	return si, err
}

// removeStory deletes everything a story owns. Safe to call on a half-made
// story.
func (m *storyManager) removeStory(id string) {
	if !storyIDPattern.MatchString(id) {
		return
	}
	m.mu.Lock()
	if a, ok := m.apps[id]; ok {
		a.db.Close()
		delete(m.apps, id)
	}
	m.mu.Unlock()
	m.reg.Exec(`DELETE FROM stories WHERE id = ?`, id)
	os.RemoveAll(m.storyDir(id))
	os.RemoveAll(m.uploadsFor(id))
}

func (m *storyManager) delete(id, confirm string) error {
	si, _, err := m.get(id)
	if err != nil {
		return err
	}
	if strings.TrimSpace(confirm) != si.Name {
		return errors.New("type the story's name exactly to delete it")
	}
	m.removeStory(id)
	return nil
}

// ---- HTTP ----

func (m *storyManager) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		var n int
		status := "ok"
		if err := m.reg.QueryRow(`SELECT COUNT(*) FROM stories`).Scan(&n); err != nil {
			status = "db_error"
		}
		writeJSON(w, map[string]any{"status": status, "stories": n, "schema_version": currentSchemaVersion})
	})
	mux.HandleFunc("GET /api/stories", m.listHandler)
	mux.HandleFunc("POST /api/stories", m.createHandler)
	mux.HandleFunc("POST /api/stories/import", m.importHandler)
	mux.HandleFunc("PUT /api/stories/{id}", m.updateHandler)
	mux.HandleFunc("DELETE /api/stories/{id}", m.deleteHandler)
	mux.HandleFunc("GET /api/stories/{id}/export", m.exportHandler)
	mux.HandleFunc("/api/s/{id}/{rest...}", m.storyHandler)
	mux.HandleFunc("GET /uploads/s/{id}/{path...}", m.uploadsHandler)
}

func (m *storyManager) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errNoStory):
		http.Error(w, "story not found", http.StatusNotFound)
	default:
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}

func (m *storyManager) listHandler(w http.ResponseWriter, r *http.Request) {
	list, err := m.list()
	if err != nil {
		http.Error(w, "failed to load stories", http.StatusInternalServerError)
		log.Printf("list stories: %v", err)
		return
	}
	writeJSON(w, list)
}

// readStoryForm parses name/icon fields shared by create and update.
func readStoryForm(r *http.Request) (name *string, up *iconUpload, choice string, closeFn func(), err error) {
	closeFn = func() {}
	if err = r.ParseMultipartForm(maxFormMemory); err != nil {
		return nil, nil, "", closeFn, errors.New("invalid form data")
	}
	if vals, ok := r.MultipartForm.Value["name"]; ok && len(vals) > 0 {
		n := vals[0]
		name = &n
	}
	choice = r.FormValue("icon_choice")
	if f, h, ferr := r.FormFile("icon"); ferr == nil {
		up = &iconUpload{filename: h.Filename, r: f}
		closeFn = func() { f.Close() }
	}
	return
}

func (m *storyManager) createHandler(w http.ResponseWriter, r *http.Request) {
	name, up, choice, done, err := readStoryForm(r)
	defer done()
	if err != nil {
		m.fail(w, err)
		return
	}
	n := ""
	if name != nil {
		n = *name
	}
	si, err := m.create(n, up, choice, strings.TrimSpace(r.FormValue("copy_assets_from")))
	if err != nil {
		m.fail(w, err)
		return
	}
	writeJSON(w, si)
}

func (m *storyManager) updateHandler(w http.ResponseWriter, r *http.Request) {
	name, up, choice, done, err := readStoryForm(r)
	defer done()
	if err != nil {
		m.fail(w, err)
		return
	}
	si, err := m.update(r.PathValue("id"), name, up, choice)
	if err != nil {
		m.fail(w, err)
		return
	}
	writeJSON(w, si)
}

func (m *storyManager) deleteHandler(w http.ResponseWriter, r *http.Request) {
	if err := m.delete(r.PathValue("id"), r.URL.Query().Get("confirm")); err != nil {
		m.fail(w, err)
		return
	}
	writeJSON(w, map[string]bool{"deleted": true})
}

// storyHandler hands a request to the story's own routes.
func (m *storyManager) storyHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	app, err := m.open(id)
	if err != nil {
		if errors.Is(err, errNoStory) {
			http.Error(w, "story not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to open story", http.StatusInternalServerError)
			log.Printf("open story %s: %v", id, err)
		}
		return
	}
	r2 := r.Clone(r.Context())
	u := *r.URL
	u.Path = "/api/" + r.PathValue("rest")
	u.RawPath = ""
	r2.URL = &u
	rw := &uploadsRewriter{w: w, replacement: []byte(`"/uploads/s/` + id + `/`)}
	app.handler.ServeHTTP(rw, r2)
	rw.finish()
}

// The story's handlers write picture URLs as "/uploads/<file>". Here, where
// the story is known, they become "/uploads/s/<id>/<file>" so the browser asks
// for the right story's file. Only JSON string values that start with that
// prefix are touched.
type uploadsRewriter struct {
	w           http.ResponseWriter
	replacement []byte
	status      int
	buf         bytes.Buffer
}

func (u *uploadsRewriter) Header() http.Header { return u.w.Header() }
func (u *uploadsRewriter) WriteHeader(code int) {
	if u.status == 0 {
		u.status = code
	}
}
func (u *uploadsRewriter) Write(b []byte) (int, error) { return u.buf.Write(b) }

func (u *uploadsRewriter) finish() {
	body := u.buf.Bytes()
	if strings.HasPrefix(u.w.Header().Get("Content-Type"), "application/json") {
		body = bytes.ReplaceAll(body, []byte(`"/uploads/`), u.replacement)
	}
	if u.status == 0 {
		u.status = http.StatusOK
	}
	u.w.Header().Del("Content-Length")
	u.w.WriteHeader(u.status)
	u.w.Write(body)
}

func (m *storyManager) uploadsHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rel := r.PathValue("path")
	if !storyIDPattern.MatchString(id) || rel == "" || strings.HasSuffix(rel, "/") {
		http.NotFound(w, r)
		return
	}
	// Uploaded files are data, never pages: no sniffing, no scripts.
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox")
	http.StripPrefix("/uploads/s/"+id, http.FileServer(http.Dir(m.uploadsFor(id)))).ServeHTTP(w, r)
}
