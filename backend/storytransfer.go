package main

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// A story exports as one zip:
//
//	polaris-story.json   what it is (name, icon, schema version)
//	story.db             a consistent snapshot of the database
//	uploads/...          every picture and icon, same relative paths
//
// Importing creates a NEW story from such a zip (never overwrites one), so it
// doubles as a per-story backup/restore.

const (
	storyFormat        = "polaris-story"
	storyFormatVersion = 1
	maxImportBytes     = 1 << 30 // compressed upload
	maxImportUnpacked  = 2 << 30
	maxImportFiles     = 50000
)

type storyManifest struct {
	Format        string `json:"format"`
	FormatVersion int    `json:"format_version"`
	Name          string `json:"name"`
	Icon          string `json:"icon"` // as stored: "", builtin:x, or _story/...
	SchemaVersion string `json:"schema_version"`
	ExportedAt    string `json:"exported_at"`
}

var slugStrip = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.Trim(slugStrip.ReplaceAllString(strings.ToLower(s), "-"), "-")
	if s == "" {
		return "story"
	}
	if len(s) > 40 {
		s = strings.Trim(s[:40], "-")
	}
	return s
}

func (m *storyManager) exportHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	si, icon, err := m.get(id)
	if err != nil {
		m.fail(w, err)
		return
	}
	app, err := m.open(id)
	if err != nil {
		m.fail(w, err)
		return
	}

	if err := os.MkdirAll(m.tmpDir(), 0o755); err != nil {
		http.Error(w, "export failed", http.StatusInternalServerError)
		return
	}
	snap, err := os.CreateTemp(m.tmpDir(), "export-*.db")
	if err != nil {
		http.Error(w, "export failed", http.StatusInternalServerError)
		return
	}
	snapPath := snap.Name()
	snap.Close()
	os.Remove(snapPath) // VACUUM INTO wants a path that doesn't exist yet
	defer os.Remove(snapPath)
	if _, err := app.db.Exec(`VACUUM INTO ?`, snapPath); err != nil {
		http.Error(w, "export failed", http.StatusInternalServerError)
		log.Printf("export snapshot %s: %v", id, err)
		return
	}

	man := storyManifest{
		Format: storyFormat, FormatVersion: storyFormatVersion, Name: si.Name, Icon: icon,
		SchemaVersion: currentSchemaVersion, ExportedAt: time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="polaris-%s-%s.zip"`, slugify(si.Name), time.Now().Format("2006-01-02")))
	zw := zip.NewWriter(w)
	defer zw.Close()

	mf, _ := zw.Create("polaris-story.json")
	enc := json.NewEncoder(mf)
	enc.SetIndent("", "  ")
	enc.Encode(man)

	if err := addFileToZip(zw, "story.db", snapPath); err != nil {
		log.Printf("export %s: %v", id, err)
		return
	}
	filepath.WalkDir(app.uploadsDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(app.uploadsDir, p)
		if err != nil {
			return nil
		}
		if err := addFileToZip(zw, "uploads/"+filepath.ToSlash(rel), p); err != nil {
			log.Printf("export %s: %v", id, err)
		}
		return nil
	})
	if err := zw.Close(); err != nil {
		log.Printf("export %s: %v", id, err)
		return
	}
	if _, err := m.reg.Exec(`UPDATE stories SET last_exported_at = datetime('now') WHERE id = ?`, id); err != nil {
		log.Printf("export %s: remember export time: %v", id, err)
	}
}

func addFileToZip(zw *zip.Writer, name, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	hdr := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: time.Now()}
	if isImageName(name) && !strings.HasSuffix(strings.ToLower(name), ".svg") {
		hdr.Method = zip.Store // already compressed
	}
	out, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	return err
}

func (m *storyManager) importHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImportBytes)
	if err := r.ParseMultipartForm(maxFormMemory); err != nil {
		http.Error(w, "couldn't read the upload", http.StatusBadRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()
	f, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "choose a story .zip to import", http.StatusBadRequest)
		return
	}
	defer f.Close()

	if err := os.MkdirAll(m.tmpDir(), 0o755); err != nil {
		http.Error(w, "import failed", http.StatusInternalServerError)
		return
	}
	tmp, err := os.CreateTemp(m.tmpDir(), "import-*.zip")
	if err != nil {
		http.Error(w, "import failed", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmp.Name())
	size, err := io.Copy(tmp, f)
	tmp.Close()
	if err != nil {
		http.Error(w, "couldn't read the upload", http.StatusBadRequest)
		return
	}

	si, err := m.importZip(tmp.Name(), size)
	if err != nil {
		var ue userError
		if errors.As(err, &ue) {
			http.Error(w, ue.Error(), http.StatusBadRequest)
		} else {
			log.Printf("import: %v", err)
			http.Error(w, "import failed", http.StatusInternalServerError)
		}
		return
	}
	writeJSON(w, si)
}

// userError is a problem with what the user uploaded (shown to them as is).
type userError string

func (e userError) Error() string { return string(e) }

func (m *storyManager) importZip(zipPath string, size int64) (storyInfo, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return storyInfo{}, userError("that file isn't a valid story export")
	}
	defer zr.Close()
	if len(zr.File) > maxImportFiles {
		return storyInfo{}, userError("that story export has too many files")
	}

	var man storyManifest
	var dbEntry *zip.File
	for _, f := range zr.File {
		switch f.Name {
		case "polaris-story.json":
			rc, err := f.Open()
			if err != nil {
				return storyInfo{}, userError("that file isn't a valid story export")
			}
			err = json.NewDecoder(io.LimitReader(rc, 1<<20)).Decode(&man)
			rc.Close()
			if err != nil {
				return storyInfo{}, userError("that file isn't a valid story export")
			}
		case "story.db":
			dbEntry = f
		}
	}
	if man.Format != storyFormat || dbEntry == nil {
		return storyInfo{}, userError("that file isn't a Polaris story export")
	}
	if man.FormatVersion > storyFormatVersion {
		return storyInfo{}, userError("that export was made by a newer version of Polaris")
	}
	name, err := validStoryName(man.Name)
	if err != nil {
		name = "Imported story"
	}

	// Importing never replaces a story; if the name is taken (say, importing a
	// backup of a story that still exists) mark the newcomer so the two can be
	// told apart.
	var taken int
	if m.reg.QueryRow(`SELECT 1 FROM stories WHERE name = ?`, name).Scan(&taken) == nil {
		name = name + " (imported)"
	}

	id, err := newStoryID()
	if err != nil {
		return storyInfo{}, err
	}
	ok := false
	defer func() {
		if !ok {
			m.removeStory(id)
		}
	}()
	if err := os.MkdirAll(m.storyDir(id), 0o755); err != nil {
		return storyInfo{}, err
	}
	uploads := m.uploadsFor(id)
	if err := os.MkdirAll(uploads, 0o755); err != nil {
		return storyInfo{}, err
	}

	var written int64
	extract := func(f *zip.File, dst string) error {
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		out, err := os.Create(dst)
		if err != nil {
			return err
		}
		n, err := io.Copy(out, io.LimitReader(rc, maxImportUnpacked-written+1))
		out.Close()
		written += n
		if err != nil {
			return err
		}
		if written > maxImportUnpacked {
			return userError("that story export is too large")
		}
		return nil
	}

	if err := extract(dbEntry, m.dbPathFor(id)); err != nil {
		return storyInfo{}, err
	}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || !strings.HasPrefix(f.Name, "uploads/") {
			continue
		}
		rel, good := safeRel(strings.TrimPrefix(f.Name, "uploads/"))
		if !good || !isImageName(rel) {
			continue // anything that isn't a plain image is ignored
		}
		if err := extract(f, filepath.Join(uploads, filepath.FromSlash(rel))); err != nil {
			return storyInfo{}, err
		}
	}

	if err := checkImportedDB(m.dbPathFor(id)); err != nil {
		return storyInfo{}, err
	}

	icon := ""
	switch {
	case isBuiltinIcon(man.Icon) && builtinStoryIcons[strings.TrimPrefix(man.Icon, builtinIconPrefix)]:
		icon = man.Icon
	case man.Icon != "":
		if rel, good := safeRel(man.Icon); good && strings.HasPrefix(rel, storyIconDir+"/") {
			if _, err := os.Stat(filepath.Join(uploads, filepath.FromSlash(rel))); err == nil {
				icon = rel
			}
		}
	}
	if _, err := m.reg.Exec(`INSERT INTO stories (id, name, icon) VALUES (?, ?, ?)`, id, name, icon); err != nil {
		return storyInfo{}, err
	}
	m.mu.Lock()
	_, err = m.openLocked(id) // migrates an older export up to the current schema
	m.mu.Unlock()
	if err != nil {
		return storyInfo{}, err
	}
	ok = true
	si, _, err := m.get(id)
	return si, err
}

// checkImportedDB makes sure an uploaded database is a healthy Polaris
// database this version can open, before it's allowed anywhere near a story.
func checkImportedDB(dbPath string) error {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return userError("the story database is damaged")
	}
	defer db.Close()
	var check string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&check); err != nil || check != "ok" {
		return userError("the story database is damaged")
	}
	var version string
	if err := db.QueryRow(`SELECT value FROM schema_meta WHERE key = 'schema_version'`).Scan(&version); err != nil {
		return userError("that file isn't a Polaris story database")
	}
	if !canMigrate(version) {
		return userError("that export was made by a different version of Polaris and can't be opened")
	}
	return nil
}
