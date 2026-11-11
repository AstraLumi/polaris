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
	"context"
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
	fonts       *fontLibrary // the fonts library, one for every story (fontlib.go)

	mu      sync.Mutex
	apps    map[string]*storyApp
	opening map[string]*opening // stories being opened (openStory)
}

type storyApp struct {
	id         string
	db         *sql.DB
	dbPath     string
	uploadsDir string
	handler    http.Handler
	graph      *graphCache // the wiki graph, kept ready (graphcache.go)

	// Under the manager's lock: how many are using the story right now, and
	// when it was last used (closeIdle).
	users    int
	lastUsed time.Time
}

// storyInfo is what the picker shows.
type storyInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Icon       string `json:"icon"` // "", "builtin:<name>" or an /uploads/s/<id>/... URL
	CreatedAt  string `json:"created_at"`
	Characters int    `json:"characters"`
	// LastExportedAt is when the story was last exported (UTC, SQLite
	// format), or "" if never: the picker nudges toward a fresh backup.
	LastExportedAt string `json:"last_exported_at"`
}

func newStoryManager(dataDir, uploadsRoot string) (*storyManager, error) {
	for _, d := range []string{filepath.Join(dataDir, "stories"), uploadsRoot} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, fmt.Errorf("create %s: %w", d, err)
		}
	}
	reg, err := sql.Open("sqlite", sqliteDSN(filepath.Join(dataDir, "stories.db")))
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
	// Registries made before exports were tracked lack the column.
	if err := addColumnIfMissing(reg, "stories", "last_exported_at", "TEXT"); err != nil {
		return nil, fmt.Errorf("upgrade registry: %w", err)
	}
	// Half-made exports and imports a stopped server left behind.
	os.RemoveAll(filepath.Join(dataDir, "tmp"))
	if err := os.MkdirAll(filepath.Join(dataDir, "tmp"), 0o755); err != nil {
		return nil, fmt.Errorf("create tmp: %w", err)
	}
	m := &storyManager{
		dataDir:     dataDir,
		uploadsRoot: uploadsRoot,
		reg:         reg,
		apps:        map[string]*storyApp{},
		opening:     map[string]*opening{},
	}
	if m.fonts, err = newFontLibrary(reg, uploadsRoot, m); err != nil {
		reg.Close()
		return nil, err
	}
	return m, nil
}

func (m *storyManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.apps {
		a.graph.stop()
		a.db.Close()
	}
	m.reg.Close()
}

func (m *storyManager) storyDir(id string) string   { return filepath.Join(m.dataDir, "stories", id) }
func (m *storyManager) dbPathFor(id string) string  { return filepath.Join(m.storyDir(id), "story.db") }
func (m *storyManager) uploadsFor(id string) string { return filepath.Join(m.uploadsRoot, id) }
func (m *storyManager) tmpDir() string              { return filepath.Join(m.dataDir, "tmp") }

// sqliteDSN is how every database is opened. The settings are in the
// address so each pooled connection gets them (a PRAGMA run once only
// reaches one):
//   - foreign keys on, so deletes cascade whichever connection makes them;
//   - the schema isn't trusted: a story can be imported from a file, and
//     its triggers and views must not reach SQLite's unsafe functions;
//   - a busy timeout, so a request that meets another's write waits for it
//     instead of failing with "database is locked";
//   - transactions take the write lock when they begin. One that read first
//     and wrote later could not wait: two of them each hold a read lock the
//     other's write needs, and SQLite fails one at once whatever the timeout.
func sqliteDSN(path string) string {
	return path + "?_pragma=foreign_keys(1)&_pragma=trusted_schema(0)&_pragma=busy_timeout(5000)&_txlock=immediate"
}

func newStoryID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// open returns the story's running app, opening (and upgrading) its database
// on first use. For code that uses the story at once and is done: something
// that holds on to it (a request, an export) takes it with use instead, so
// it isn't closed for being idle in the meantime.
func (m *storyManager) open(id string) (*storyApp, error) {
	return m.openStory(id, true)
}

// opening is a story on its way to being open: whoever else asks for it
// meanwhile waits for this one attempt instead of starting another.
type opening struct {
	done chan struct{}
	app  *storyApp
	err  error
}

// openStory opens a story without holding up the others: the manager's lock
// is only held to look the story up, never while its database is opened,
// backed up or upgraded (which can take a while). listed is false for a
// story being made, whose row the caller has just written.
func (m *storyManager) openStory(id string, listed bool) (*storyApp, error) {
	if !storyIDPattern.MatchString(id) {
		return nil, errNoStory
	}
	m.mu.Lock()
	if a, ok := m.apps[id]; ok {
		a.lastUsed = time.Now()
		m.mu.Unlock()
		return a, nil
	}
	if o := m.opening[id]; o != nil {
		m.mu.Unlock()
		<-o.done
		return o.app, o.err
	}
	o := &opening{done: make(chan struct{})}
	m.opening[id] = o
	m.mu.Unlock()

	if listed {
		var one int
		if err := m.reg.QueryRow(`SELECT 1 FROM stories WHERE id = ?`, id).Scan(&one); err != nil {
			o.err = err
			if errors.Is(err, sql.ErrNoRows) {
				o.err = errNoStory
			}
		}
	}
	if o.err == nil {
		o.app, o.err = m.build(id)
	}

	m.mu.Lock()
	delete(m.opening, id)
	if o.err == nil {
		o.app.lastUsed = time.Now()
		m.apps[id] = o.app
	}
	m.mu.Unlock()
	close(o.done)
	return o.app, o.err
}

// use is open for as long as the story is needed: it is not closed for
// being idle until done is called.
func (m *storyManager) use(id string) (app *storyApp, done func(), err error) {
	for {
		a, err := m.open(id)
		if err != nil {
			return nil, func() {}, err
		}
		m.mu.Lock()
		if m.apps[id] == a {
			a.users++
			m.mu.Unlock()
			return a, func() { m.release(a) }, nil
		}
		m.mu.Unlock() // closed between the two looks: open it again
	}
}

// peek is use for a story only if it is open already.
func (m *storyManager) peek(id string) (app *storyApp, done func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a := m.apps[id]
	if a == nil {
		return nil, func() {}
	}
	a.users++
	return a, func() { m.release(a) }
}

func (m *storyManager) release(a *storyApp) {
	m.mu.Lock()
	a.users--
	a.lastUsed = time.Now()
	m.mu.Unlock()
}

// Variables only so the tests can shorten them.
var (
	storyIdleAfter = 30 * time.Minute // close a story nobody used for this long
	storyIdleLook  = 5 * time.Minute
)

// closeIdle closes the stories nobody has used lately: their connections,
// and the wiki graph kept for them, are given back. The next request opens
// the story again in a moment.
func (m *storyManager) closeIdle(now time.Time) (closed int) {
	m.mu.Lock()
	var idle []*storyApp
	for id, a := range m.apps {
		if a.users == 0 && now.Sub(a.lastUsed) >= storyIdleAfter {
			idle = append(idle, a)
			delete(m.apps, id)
		}
	}
	m.mu.Unlock()
	for _, a := range idle {
		a.graph.stop()
		a.db.Close()
	}
	return len(idle)
}

func (m *storyManager) idleLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(storyIdleLook):
		}
		if n := m.closeIdle(time.Now()); n > 0 {
			log.Printf("closed %d idle stories", n)
		}
	}
}

// build opens one story's database (backing it up and upgrading it when it
// is at an older schema) and makes its routes. It doesn't touch the
// manager's list: openStory does.
func (m *storyManager) build(id string) (*storyApp, error) {
	if err := os.MkdirAll(m.storyDir(id), 0o755); err != nil {
		return nil, err
	}
	uploads := m.uploadsFor(id)
	if err := os.MkdirAll(uploads, 0o755); err != nil {
		return nil, err
	}
	dbPath := m.dbPathFor(id)
	// Foreign keys on every pooled connection, not just the first: a PRAGMA
	// run once only reaches one connection, so deletes elsewhere didn't
	// cascade and left rows behind.
	db, err := sql.Open("sqlite", sqliteDSN(dbPath))
	if err != nil {
		return nil, err
	}
	// A story at another schema is backed up first (storybackup.go).
	backedUp, err := m.backupBeforeOpen(id, db, uploads)
	if err != nil {
		log.Printf("backup %s: %v", id, err)
		// An upgrade can't be undone, so it doesn't start without its
		// backup. (A story that will be refused anyway goes on to be.)
		if version := storedSchema(db); !newerSchema(version) && canMigrate(version) {
			db.Close()
			return nil, errNoBackup
		}
	}
	if err := ensureSchema(db); err != nil {
		db.Close()
		if errors.Is(err, errNewerSchema) && backedUp {
			return nil, errNewerSchemaBackedUp
		}
		return nil, err
	}
	graph := newGraphCache(db, dbPath)
	return &storyApp{id: id, db: db, dbPath: dbPath, uploadsDir: uploads, handler: storyRoutes(db, uploads, graph), graph: graph}, nil
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
	m.reg.QueryRow(`SELECT COALESCE(last_exported_at, '') FROM stories WHERE id = ?`, id).Scan(&si.LastExportedAt)
	si.Characters = m.characterCount(id)
	return si
}

// characterCount is for the picker. A story that isn't open is only looked
// into, on a connection of its own that can't write: listing the stories
// must not open them all (each would be upgraded and backed up there and
// then, and kept open for good).
func (m *storyManager) characterCount(id string) (n int) {
	if a, done := m.peek(id); a != nil {
		defer done()
		a.db.QueryRow(`SELECT COUNT(*) FROM characters`).Scan(&n)
		return n
	}
	if _, err := os.Stat(m.dbPathFor(id)); err != nil {
		return 0
	}
	db, err := sql.Open("sqlite", m.dbPathFor(id)+"?_pragma=query_only(1)&_pragma=trusted_schema(0)&_pragma=busy_timeout(2000)")
	if err != nil {
		return 0
	}
	defer db.Close()
	db.QueryRow(`SELECT COUNT(*) FROM characters`).Scan(&n)
	return n
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
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
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
		var done func()
		if src, done, err = m.use(copyFrom); err != nil {
			return storyInfo{}, errors.New("the story to copy from doesn't exist")
		}
		defer done()
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
	app, err := m.openStory(id, false)
	if err != nil {
		return fail(err)
	}
	if src != nil {
		if err := copyAssets(app.db, src.db, src.dbPath, src.uploadsDir, app.uploadsDir); err != nil {
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
		a.graph.stop()
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
	// The server's own daily copies (snapshots.go).
	mux.HandleFunc("GET /api/stories/{id}/snapshots", m.snapshotsHandler)
	mux.HandleFunc("POST /api/stories/{id}/snapshots/{name}/restore", m.restoreSnapshotHandler)
	mux.HandleFunc("/api/s/{id}/{rest...}", m.storyHandler)
	mux.HandleFunc("GET /uploads/s/{id}/{path...}", m.uploadsHandler)
	m.fonts.registerRoutes(mux)
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
func readStoryForm(w http.ResponseWriter, r *http.Request) (name *string, up *iconUpload, choice string, closeFn func(), err error) {
	closeFn = func() {}
	if err = parsePictureForm(w, r); err != nil {
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
	name, up, choice, done, err := readStoryForm(w, r)
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
	name, up, choice, done, err := readStoryForm(w, r)
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
	app, done, err := m.use(id)
	defer done()
	if err != nil {
		if errors.Is(err, errNoStory) {
			http.Error(w, "story not found", http.StatusNotFound)
		} else if errors.Is(err, errNewerSchema) || errors.Is(err, errNewerSchemaBackedUp) ||
			errors.Is(err, errUnknownSchema) || errors.Is(err, errNoBackup) {
			if !errors.Is(err, errNewerSchema) && !errors.Is(err, errNewerSchemaBackedUp) {
				log.Printf("open story %s: %v", id, err)
			}
			http.Error(w, err.Error(), http.StatusConflict)
		} else {
			http.Error(w, "failed to open story", http.StatusInternalServerError)
			log.Printf("open story %s: %v", id, err)
		}
		return
	}
	// The library goes with the request: the EPUB export embeds its fonts.
	r2 := r.Clone(withFontLibrary(r.Context(), m.fonts))
	u := *r.URL
	u.Path = "/api/" + r.PathValue("rest")
	u.RawPath = ""
	r2.URL = &u
	rw := &uploadsRewriter{w: w, replacement: []byte(`"/uploads/s/` + id + `/`)}
	reads := readErrors.Load()
	app.handler.ServeHTTP(rw, r2)
	// A list cut short by a broken read is not sent as if it were whole
	// (noteReadError, utils.go). Only what was asked for: something saved
	// stays saved.
	if r.Method == http.MethodGet && readErrors.Load() != reads {
		rw.failInstead("failed to load: the story could not be read just now")
	}
	rw.finish()
	// The server only clears the temporary files of the request it was
	// given, and the form was read on the copy.
	if r2.MultipartForm != nil {
		r2.MultipartForm.RemoveAll()
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		app.graph.changed(r.PathValue("rest"))
	}
}

// The story's handlers write picture URLs as "/uploads/<file>". Here, where
// the story is known, they become "/uploads/s/<id>/<file>" so the browser asks
// for the right story's file. Only JSON string values that start with that
// prefix are touched: a JSON answer is held until it is whole, anything else
// (a book's export, an error's text) goes straight out as it is written.
type uploadsRewriter struct {
	w           http.ResponseWriter
	replacement []byte
	status      int
	decided     bool
	direct      bool // not JSON: written through
	buf         bytes.Buffer
}

func (u *uploadsRewriter) Header() http.Header { return u.w.Header() }

func (u *uploadsRewriter) decide() {
	u.decided = true
	if u.status == 0 {
		u.status = http.StatusOK
	}
	if !strings.HasPrefix(u.w.Header().Get("Content-Type"), "application/json") {
		u.direct = true
		u.w.WriteHeader(u.status)
	}
}

func (u *uploadsRewriter) WriteHeader(code int) {
	if u.decided {
		return
	}
	u.status = code
	u.decide()
}

func (u *uploadsRewriter) Write(b []byte) (int, error) {
	if !u.decided {
		u.decide()
	}
	if u.direct {
		return u.w.Write(b)
	}
	return u.buf.Write(b)
}

func (u *uploadsRewriter) Flush() {
	if f, ok := u.w.(http.Flusher); ok && u.direct {
		f.Flush()
	}
}

// failInstead drops a JSON answer still held back and sends an error in its
// place. An answer already on its way (not JSON) is left as it is.
func (u *uploadsRewriter) failInstead(msg string) {
	if u.direct || (u.status != 0 && u.status != http.StatusOK) {
		return
	}
	u.buf.Reset()
	u.w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	u.w.Header().Del("ETag")
	u.status = http.StatusInternalServerError
	u.decided = true
	u.buf.WriteString(msg)
}

func (u *uploadsRewriter) finish() {
	if !u.decided {
		u.decide()
	}
	if u.direct {
		return
	}
	body := bytes.ReplaceAll(u.buf.Bytes(), []byte(`"/uploads/`), u.replacement)
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

// addColumnIfMissing adds a column to a registry table that predates it.
// (Story databases use the migrations in db.go; the registry is just one
// small table, so it is upgraded in place like this.)
func addColumnIfMissing(db *sql.DB, table, column, decl string) error {
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()
	_, err = db.Exec(`ALTER TABLE ` + quoteIdent(table) + ` ADD COLUMN ` + quoteIdent(column) + ` ` + decl)
	return err
}
