package main

// The fonts library (docs/EDITOR.md, "Fonts library (stage 5)"): the
// writer's own font files, one library for every story, each family with a
// record of its license. The files are read by fontparse.go.
//
// Storage: two tables in the registry (<DATA_DIR>/stories.db, beside the
// stories list; made and upgraded in place like it) and one folder,
// <UPLOADS_DIR>/_fonts/, which can't be a story's (story ids are 16 hex
// digits). A font file is named by its content (the first 32 hex digits of
// its SHA-256, then its format), so its address never changes what it
// serves and the browser may keep it forever; a license or receipt file
// attached to a family lives in _fonts/licenses/ under a random name.
//
// API (global, not under a story):
//
//	GET    /api/fonts                       every family with its files
//	POST   /api/fonts                       upload (multipart: file, one or
//	                                        more; family_id to add them to one)
//	PUT    /api/fonts/{id}                  the family's name and license record
//	DELETE /api/fonts/{id}                  the family, its files and license file
//	POST   /api/fonts/{id}/license-file     attach a receipt or license (multipart file)
//	GET    /api/fonts/{id}/license-file     download it
//	DELETE /api/fonts/{id}/license-file
//	DELETE /api/font-files/{fid}           one file of a family
//	GET    /api/fonts/used                  which stories, books and styles
//	                                        use each family (a scan, on demand)
//	GET    /uploads/fonts/{name}            a font file, for @font-face
//
// And under each story: GET /api/books/{volume}/fonts, the families a book
// names (styles, headers and footers, its text), for the export warnings.
// EPUB exports embed the families the library allows (exportepub.go); the
// library reaches the story's export handler through the request's context
// (withFontLibrary).

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	fontsDirName      = "_fonts"
	fontLicenseSubdir = "licenses"
	maxFontFiles      = 20 // files in one upload
)

// Variables only so tests can lower them.
var (
	maxFontFile        = int64(40 << 20)
	maxFontLicenseFile = int64(20 << 20)
	maxEmbedFontBytes  = int64(120 << 20) // every font put in one export
)

// License kinds ("" = not filled in yet).
var fontLicenseKinds = map[string]bool{"": true, "ofl": true, "apache": true, "commercial": true, "personal": true, "other": true}

var fontFileNamePattern = regexp.MustCompile(`^[0-9a-f]{32}\.(ttf|otf|woff|woff2)$`)

const fontTablesSQL = `
CREATE TABLE IF NOT EXISTS font_families (
	id                 INTEGER PRIMARY KEY AUTOINCREMENT,
	name               TEXT NOT NULL UNIQUE COLLATE NOCASE,
	license_kind       TEXT NOT NULL DEFAULT '',
	source             TEXT NOT NULL DEFAULT '',
	purchase_date      TEXT NOT NULL DEFAULT '',
	license_file       TEXT NOT NULL DEFAULT '',
	license_file_name  TEXT NOT NULL DEFAULT '',
	use_print          INTEGER NOT NULL DEFAULT 0,
	use_ebook          INTEGER NOT NULL DEFAULT 0,
	use_web            INTEGER NOT NULL DEFAULT 0,
	use_commercial     INTEGER NOT NULL DEFAULT 0,
	notes              TEXT NOT NULL DEFAULT '',
	created_at         TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE TABLE IF NOT EXISTS font_files (
	id             INTEGER PRIMARY KEY AUTOINCREMENT,
	family_id      INTEGER NOT NULL REFERENCES font_families(id) ON DELETE CASCADE,
	file           TEXT NOT NULL UNIQUE,
	original_name  TEXT NOT NULL DEFAULT '',
	format         TEXT NOT NULL,
	size           INTEGER NOT NULL DEFAULT 0,
	weight         INTEGER NOT NULL DEFAULT 400,
	italic         INTEGER NOT NULL DEFAULT 0,
	style_name     TEXT NOT NULL DEFAULT '',
	full_name      TEXT NOT NULL DEFAULT '',
	copyright      TEXT NOT NULL DEFAULT '',
	license_text   TEXT NOT NULL DEFAULT '',
	license_url    TEXT NOT NULL DEFAULT '',
	designer       TEXT NOT NULL DEFAULT '',
	vendor         TEXT NOT NULL DEFAULT '',
	details_read   INTEGER NOT NULL DEFAULT 1,
	fs_type        INTEGER,
	created_at     TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_font_files_family ON font_files(family_id);
`

type fontLibrary struct {
	db  *sql.DB // the registry
	dir string  // <UPLOADS_DIR>/_fonts
	mgr *storyManager

	metrics    fontMetricsCache // line spacing per Google family (fontmetrics.go)
	googleBase string           // Google's CSS address; "" for the real one (tests set it)
}

func newFontLibrary(reg *sql.DB, uploadsRoot string, mgr *storyManager) (*fontLibrary, error) {
	if _, err := reg.Exec(fontTablesSQL); err != nil {
		return nil, fmt.Errorf("create the fonts library: %w", err)
	}
	dir := filepath.Join(uploadsRoot, fontsDirName)
	if err := os.MkdirAll(filepath.Join(dir, fontLicenseSubdir), 0o755); err != nil {
		return nil, err
	}
	return &fontLibrary{db: reg, dir: dir, mgr: mgr}, nil
}

// ---- reading ----

type fontUsesOut struct {
	Print      bool `json:"print"`
	Ebook      bool `json:"ebook"`
	Web        bool `json:"web"`
	Commercial bool `json:"commercial"`
}

type fontFileOut struct {
	ID           int64  `json:"id"`
	URL          string `json:"url"`
	File         string `json:"-"`
	Format       string `json:"format"`
	Size         int64  `json:"size"`
	OriginalName string `json:"original_name"`
	Weight       int    `json:"weight"`
	Italic       bool   `json:"italic"`
	Style        string `json:"style"`
	FullName     string `json:"full_name"`
	Copyright    string `json:"copyright"`
	License      string `json:"license"`
	LicenseURL   string `json:"license_url"`
	Designer     string `json:"designer"`
	Vendor       string `json:"vendor"`
	DetailsRead  bool   `json:"details_read"`
	Embedding    string `json:"embedding"` // installable, editable, preview, restricted, unknown
	NoSubsetting bool   `json:"no_subsetting"`
	BitmapOnly   bool   `json:"bitmap_only"`
}

// forbidsEmbedding: this file's fsType keeps it out of documents.
func (f fontFileOut) forbidsEmbedding() bool { return f.Embedding == embedRestricted || f.BitmapOnly }

type fontFamilyOut struct {
	ID           int64         `json:"id"`
	Name         string        `json:"name"`
	LicenseKind  string        `json:"license_kind"`
	Source       string        `json:"source"`
	PurchaseDate string        `json:"purchase_date"`
	LicenseFile  string        `json:"license_file"` // the attached file's name, "" when none
	Uses         fontUsesOut   `json:"uses"`
	Notes        string        `json:"notes"`
	CreatedAt    string        `json:"created_at"`
	Files        []fontFileOut `json:"files"`
}

func fontFileURL(stored string) string { return "/uploads/fonts/" + stored }

func (l *fontLibrary) list() ([]fontFamilyOut, error) {
	rows, err := l.db.Query(`SELECT id, name, license_kind, source, purchase_date, license_file_name, use_print, use_ebook, use_web, use_commercial, notes, created_at
		FROM font_families ORDER BY name COLLATE NOCASE, id`)
	if err != nil {
		return nil, err
	}
	out := []fontFamilyOut{}
	index := map[int64]int{}
	for rows.Next() {
		f := fontFamilyOut{Files: []fontFileOut{}}
		if err := rows.Scan(&f.ID, &f.Name, &f.LicenseKind, &f.Source, &f.PurchaseDate, &f.LicenseFile,
			&f.Uses.Print, &f.Uses.Ebook, &f.Uses.Web, &f.Uses.Commercial, &f.Notes, &f.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		index[f.ID] = len(out)
		out = append(out, f)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	files, err := l.db.Query(`SELECT id, family_id, file, original_name, format, size, weight, italic, style_name, full_name,
		copyright, license_text, license_url, designer, vendor, details_read, fs_type
		FROM font_files ORDER BY italic, weight, id`)
	if err != nil {
		return nil, err
	}
	defer files.Close()
	for files.Next() {
		var f fontFileOut
		var fam int64
		var fs sql.NullInt64
		if err := files.Scan(&f.ID, &fam, &f.File, &f.OriginalName, &f.Format, &f.Size, &f.Weight, &f.Italic, &f.Style, &f.FullName,
			&f.Copyright, &f.License, &f.LicenseURL, &f.Designer, &f.Vendor, &f.DetailsRead, &fs); err != nil {
			return nil, err
		}
		f.URL = fontFileURL(f.File)
		f.Embedding = embedUnknown
		if fs.Valid {
			var fi fontInfo
			fi.setFSType(uint16(fs.Int64))
			f.Embedding, f.NoSubsetting, f.BitmapOnly = fi.Embedding, fi.NoSubsetting, fi.BitmapOnly
		}
		if i, ok := index[fam]; ok {
			out[i].Files = append(out[i].Files, f)
		}
	}
	return out, files.Err()
}

func (l *fontLibrary) family(id int64) (*fontFamilyOut, error) {
	all, err := l.list()
	if err != nil {
		return nil, err
	}
	for i := range all {
		if all[i].ID == id {
			return &all[i], nil
		}
	}
	return nil, errNoFontFamily
}

var errNoFontFamily = errors.New("font not found")

// ---- license guesses ----

var oflPattern = regexp.MustCompile(`\bofl\b`)

// guessLicense reads a license kind from a font's own words: the SIL Open
// Font License and the Apache License allow every use; a font that says
// it is for personal use only allows none of the four.
func guessLicense(info *fontInfo) (kind string, all bool) {
	text := strings.ToLower(info.License + " " + info.LicenseURL + " " + info.Copyright)
	switch {
	case strings.Contains(text, "open font license") || strings.Contains(text, "openfontlicense.org") ||
		strings.Contains(text, "scripts.sil.org/ofl") || oflPattern.MatchString(text):
		return "ofl", true
	case strings.Contains(text, "apache license") || strings.Contains(text, "apache.org/licenses"):
		return "apache", true
	case strings.Contains(text, "personal use") || strings.Contains(text, "non-commercial") || strings.Contains(text, "noncommercial"):
		return "personal", false
	}
	return "", false
}

// ---- uploading ----

type fontUploadProblem struct {
	File  string `json:"file"`
	Error string `json:"error"`
}

// addFont stores one font file in the library: in the family given, else
// in the family its names give (made, with a license guess, when the
// library doesn't have it yet). It answers the family's id.
func (l *fontLibrary) addFont(data []byte, originalName string, familyID int64) (int64, error) {
	info, err := parseFont(data)
	if err != nil {
		return 0, err
	}
	family, weight, italic := info.Family, info.Weight, info.Italic
	if !info.Read {
		family, weight, italic = guessFromFileName(originalName)
		if family == "" {
			family = "Untitled font"
		}
	}
	sum := sha256.Sum256(data)
	stored := hex.EncodeToString(sum[:16]) + "." + info.Format
	var dup int64
	if err := l.db.QueryRow(`SELECT family_id FROM font_files WHERE file = ?`, stored).Scan(&dup); err == nil {
		return 0, errors.New("this font file is already in the library")
	}

	tx, err := l.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if familyID != 0 {
		var one int
		if err := tx.QueryRow(`SELECT 1 FROM font_families WHERE id = ?`, familyID).Scan(&one); err != nil {
			return 0, errNoFontFamily
		}
	} else {
		err := tx.QueryRow(`SELECT id FROM font_families WHERE name = ? COLLATE NOCASE`, family).Scan(&familyID)
		if errors.Is(err, sql.ErrNoRows) {
			kind, all := guessLicense(info)
			res, err := tx.Exec(`INSERT INTO font_families (name, license_kind, source, use_print, use_ebook, use_web, use_commercial) VALUES (?, ?, ?, ?, ?, ?, ?)`,
				family, kind, info.LicenseURL, all, all, all, all)
			if err != nil {
				return 0, err
			}
			familyID, _ = res.LastInsertId()
		} else if err != nil {
			return 0, err
		} else if kind, all := guessLicense(info); kind != "" {
			// Often only one of a family's files names its license: fill it in
			// when the family has none yet, whichever file came first.
			if _, err := tx.Exec(`UPDATE font_families SET license_kind = ?,
				source = CASE WHEN source = '' THEN ? ELSE source END,
				use_print = use_print OR ?, use_ebook = use_ebook OR ?, use_web = use_web OR ?, use_commercial = use_commercial OR ?
				WHERE id = ? AND license_kind = ''`, kind, info.LicenseURL, all, all, all, all, familyID); err != nil {
				return 0, err
			}
		}
	}
	var fs sql.NullInt64
	if info.HasFSType {
		fs = sql.NullInt64{Int64: int64(info.FSType), Valid: true}
	}
	if _, err := tx.Exec(`INSERT INTO font_files (family_id, file, original_name, format, size, weight, italic, style_name, full_name,
		copyright, license_text, license_url, designer, vendor, details_read, fs_type) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		familyID, stored, truncateRunes(filepath.Base(originalName), maxShortNameLen), info.Format, len(data), weight, italic,
		info.Style, info.FullName, info.Copyright, info.License, info.LicenseURL, info.Designer, info.Vendor, info.Read, fs); err != nil {
		return 0, err
	}
	path := filepath.Join(l.dir, stored)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		os.Remove(path)
		return 0, err
	}
	return familyID, nil
}

func readFormFile(fh *multipart.FileHeader, limit int64) ([]byte, error) {
	f, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errTooLarge
	}
	return data, nil
}

var errTooLarge = errors.New("too large")

func (l *fontLibrary) uploadHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFontFiles*maxFontFile+(1<<20))
	if err := r.ParseMultipartForm(maxFormMemory); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			http.Error(w, "those font files are too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()
	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		http.Error(w, "choose a font file", http.StatusBadRequest)
		return
	}
	if len(files) > maxFontFiles {
		http.Error(w, "upload at most 20 font files at once", http.StatusBadRequest)
		return
	}
	var familyID int64
	if v := strings.TrimSpace(r.FormValue("family_id")); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			http.Error(w, "font not found", http.StatusNotFound)
			return
		}
		familyID = id
	}
	problems := []fontUploadProblem{}
	added := []int64{}
	tooLarge := false
	for _, fh := range files {
		data, err := readFormFile(fh, maxFontFile)
		if errors.Is(err, errTooLarge) {
			tooLarge = true
			problems = append(problems, fontUploadProblem{fh.Filename, "that font file is too large (over 40 MB)"})
			continue
		}
		if err != nil {
			problems = append(problems, fontUploadProblem{fh.Filename, "invalid form data"})
			continue
		}
		id, err := l.addFont(data, fh.Filename, familyID)
		if err != nil {
			msg := err.Error()
			if !errors.Is(err, errNotFont) && !errors.Is(err, errFontCollection) && !errors.Is(err, errBadFont) && !errors.Is(err, errNoFontFamily) &&
				msg != "this font file is already in the library" {
				log.Printf("add font %q: %v", fh.Filename, err)
				msg = "failed to save the font"
			}
			problems = append(problems, fontUploadProblem{fh.Filename, msg})
			continue
		}
		added = append(added, id)
	}
	if len(added) == 0 {
		code := http.StatusBadRequest
		if tooLarge && len(problems) == 1 {
			code = http.StatusRequestEntityTooLarge
		}
		http.Error(w, problems[0].Error, code)
		return
	}
	list, err := l.list()
	if err != nil {
		http.Error(w, "failed to load the fonts", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"families": list, "added": added, "problems": problems})
}

// ---- the license record ----

type fontFamilyIn struct {
	Name         *string      `json:"name"`
	LicenseKind  *string      `json:"license_kind"`
	Source       *string      `json:"source"`
	PurchaseDate *string      `json:"purchase_date"`
	Uses         *fontUsesOut `json:"uses"`
	Notes        *string      `json:"notes"`
	Files        []struct {
		ID     int64 `json:"id"`
		Weight int   `json:"weight"`
		Italic bool  `json:"italic"`
	} `json:"files"`
}

func (l *fontLibrary) update(id int64, in fontFamilyIn) error {
	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var one int
	if err := tx.QueryRow(`SELECT 1 FROM font_families WHERE id = ?`, id).Scan(&one); err != nil {
		return errNoFontFamily
	}
	set := func(col string, v any) error {
		_, err := tx.Exec(`UPDATE font_families SET `+col+` = ? WHERE id = ?`, v, id)
		return err
	}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" || utf8.RuneCountInString(name) > maxShortNameLen || strings.ContainsAny(name, "\"\\;{}<>") {
			return badRequest("a font needs a name (200 characters at most, without quotes or brackets)")
		}
		var other int64
		if err := tx.QueryRow(`SELECT id FROM font_families WHERE name = ? COLLATE NOCASE AND id <> ?`, name, id).Scan(&other); err == nil {
			return badRequest("another font in the library has that name")
		}
		if err := set("name", name); err != nil {
			return err
		}
	}
	if in.LicenseKind != nil {
		if !fontLicenseKinds[*in.LicenseKind] {
			return badRequest("unknown license kind")
		}
		if err := set("license_kind", *in.LicenseKind); err != nil {
			return err
		}
	}
	if in.Source != nil {
		if utf8.RuneCountInString(*in.Source) > 2000 {
			return badRequest("that text is too long")
		}
		if err := set("source", strings.TrimSpace(*in.Source)); err != nil {
			return err
		}
	}
	if in.PurchaseDate != nil {
		d := strings.TrimSpace(*in.PurchaseDate)
		if d != "" {
			if _, err := time.Parse("2006-01-02", d); err != nil {
				return badRequest("the purchase date is not valid")
			}
		}
		if err := set("purchase_date", d); err != nil {
			return err
		}
	}
	if in.Notes != nil {
		if utf8.RuneCountInString(*in.Notes) > 10000 {
			return badRequest("that text is too long")
		}
		if err := set("notes", *in.Notes); err != nil {
			return err
		}
	}
	if u := in.Uses; u != nil {
		if _, err := tx.Exec(`UPDATE font_families SET use_print = ?, use_ebook = ?, use_web = ?, use_commercial = ? WHERE id = ?`,
			u.Print, u.Ebook, u.Web, u.Commercial, id); err != nil {
			return err
		}
	}
	for _, f := range in.Files {
		if f.Weight < 100 || f.Weight > 900 || f.Weight%100 != 0 {
			return badRequest("a weight is one of 100, 200 … 900")
		}
		res, err := tx.Exec(`UPDATE font_files SET weight = ?, italic = ? WHERE id = ? AND family_id = ?`, f.Weight, f.Italic, f.ID, id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return badRequest("that file is not one of this font's")
		}
	}
	return tx.Commit()
}

type badRequest string

func (e badRequest) Error() string { return string(e) }

func (l *fontLibrary) fail(w http.ResponseWriter, err error, what string) {
	var br badRequest
	switch {
	case errors.Is(err, errNoFontFamily):
		http.Error(w, "font not found", http.StatusNotFound)
	case errors.As(err, &br):
		http.Error(w, br.Error(), http.StatusBadRequest)
	default:
		log.Printf("fonts library: %s: %v", what, err)
		http.Error(w, "failed to save the font", http.StatusInternalServerError)
	}
}

func pathID(r *http.Request, key string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(key), 10, 64)
	return id, err == nil && id > 0
}

func (l *fontLibrary) writeFamily(w http.ResponseWriter, id int64) {
	f, err := l.family(id)
	if err != nil {
		l.fail(w, err, "load")
		return
	}
	writeJSON(w, f)
}

func (l *fontLibrary) updateHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.Error(w, "font not found", http.StatusNotFound)
		return
	}
	var in fontFamilyIn
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if err := l.update(id, in); err != nil {
		l.fail(w, err, "update")
		return
	}
	l.writeFamily(w, id)
}

func (l *fontLibrary) remove(path string) {
	if clean, ok := safeRel(path); ok && path != "" {
		os.Remove(filepath.Join(l.dir, filepath.FromSlash(clean)))
	}
}

func (l *fontLibrary) deleteHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.Error(w, "font not found", http.StatusNotFound)
		return
	}
	var licenseFile string
	if err := l.db.QueryRow(`SELECT license_file FROM font_families WHERE id = ?`, id).Scan(&licenseFile); err != nil {
		http.Error(w, "font not found", http.StatusNotFound)
		return
	}
	var files []string
	rows, err := l.db.Query(`SELECT file FROM font_files WHERE family_id = ?`, id)
	if err == nil {
		for rows.Next() {
			var f string
			rows.Scan(&f)
			files = append(files, f)
		}
		rows.Close()
	}
	if _, err := l.db.Exec(`DELETE FROM font_files WHERE family_id = ?`, id); err != nil {
		l.fail(w, err, "delete")
		return
	}
	if _, err := l.db.Exec(`DELETE FROM font_families WHERE id = ?`, id); err != nil {
		l.fail(w, err, "delete")
		return
	}
	for _, f := range files {
		l.remove(f)
	}
	l.remove(licenseFile)
	writeJSON(w, map[string]bool{"deleted": true})
}

func (l *fontLibrary) deleteFileHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "fid")
	if !ok {
		http.Error(w, "font not found", http.StatusNotFound)
		return
	}
	var file string
	var fam int64
	if err := l.db.QueryRow(`SELECT file, family_id FROM font_files WHERE id = ?`, id).Scan(&file, &fam); err != nil {
		http.Error(w, "font not found", http.StatusNotFound)
		return
	}
	if _, err := l.db.Exec(`DELETE FROM font_files WHERE id = ?`, id); err != nil {
		l.fail(w, err, "delete a file")
		return
	}
	l.remove(file)
	l.writeFamily(w, fam)
}

// sniffLicenseFile: a PDF, a picture or plain text.
func sniffLicenseFile(data []byte) (ct, ext string, ok bool) {
	if ct, ext, ok := sniffPicture(data); ok {
		return ct, ext, true
	}
	switch ct := http.DetectContentType(data); {
	case ct == "application/pdf":
		return ct, ".pdf", true
	case strings.HasPrefix(ct, "text/plain"):
		return "text/plain; charset=utf-8", ".txt", true
	}
	return "", "", false
}

func (l *fontLibrary) licenseUploadHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.Error(w, "font not found", http.StatusNotFound)
		return
	}
	var old string
	if err := l.db.QueryRow(`SELECT license_file FROM font_families WHERE id = ?`, id).Scan(&old); err != nil {
		http.Error(w, "font not found", http.StatusNotFound)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxFontLicenseFile+(1<<20))
	if err := r.ParseMultipartForm(maxFormMemory); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			http.Error(w, "that file is too large (over 20 MB)", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()
	fhs := r.MultipartForm.File["file"]
	if len(fhs) == 0 {
		http.Error(w, "choose a file", http.StatusBadRequest)
		return
	}
	data, err := readFormFile(fhs[0], maxFontLicenseFile)
	if errors.Is(err, errTooLarge) {
		http.Error(w, "that file is too large (over 20 MB)", http.StatusRequestEntityTooLarge)
		return
	}
	if err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}
	_, ext, ok := sniffLicenseFile(data)
	if !ok {
		http.Error(w, "a license or receipt must be a PDF, a picture or a text file", http.StatusBadRequest)
		return
	}
	rnd := make([]byte, 12)
	rand.Read(rnd)
	stored := fontLicenseSubdir + "/" + hex.EncodeToString(rnd) + ext
	if err := os.WriteFile(filepath.Join(l.dir, filepath.FromSlash(stored)), data, 0o644); err != nil {
		l.fail(w, err, "save a license file")
		return
	}
	name := truncateRunes(filepath.Base(strings.ReplaceAll(fhs[0].Filename, `\`, "/")), maxShortNameLen)
	if _, err := l.db.Exec(`UPDATE font_families SET license_file = ?, license_file_name = ? WHERE id = ?`, stored, name, id); err != nil {
		l.remove(stored)
		l.fail(w, err, "save a license file")
		return
	}
	l.remove(old)
	l.writeFamily(w, id)
}

func (l *fontLibrary) licenseDeleteHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.Error(w, "font not found", http.StatusNotFound)
		return
	}
	var old string
	if err := l.db.QueryRow(`SELECT license_file FROM font_families WHERE id = ?`, id).Scan(&old); err != nil {
		http.Error(w, "font not found", http.StatusNotFound)
		return
	}
	if _, err := l.db.Exec(`UPDATE font_families SET license_file = '', license_file_name = '' WHERE id = ?`, id); err != nil {
		l.fail(w, err, "remove a license file")
		return
	}
	l.remove(old)
	l.writeFamily(w, id)
}

func (l *fontLibrary) licenseGetHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	var stored, name string
	if err := l.db.QueryRow(`SELECT license_file, license_file_name FROM font_families WHERE id = ?`, id).Scan(&stored, &name); err != nil || stored == "" {
		http.NotFound(w, r)
		return
	}
	clean, ok := safeRel(stored)
	if !ok {
		http.NotFound(w, r)
		return
	}
	data, err := os.ReadFile(filepath.Join(l.dir, filepath.FromSlash(clean)))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	ct, _, _ := sniffLicenseFile(data)
	if ct == "" {
		ct = "application/octet-stream"
	}
	if name == "" {
		name = "license" + filepath.Ext(stored)
	}
	h := w.Header()
	h.Set("Content-Type", ct)
	h.Set("Content-Disposition", contentDisposition(name))
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "sandbox")
	h.Set("Cache-Control", "no-store")
	w.Write(data)
}

// fontFileHandler serves a font file for @font-face. Its name is its
// content's, so it may be kept for good.
func (l *fontLibrary) fontFileHandler(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !fontFileNamePattern.MatchString(name) {
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
	h.Set("Content-Type", fontMIME(strings.TrimPrefix(filepath.Ext(name), ".")))
	h.Set("Cache-Control", "public, max-age=31536000, immutable")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "sandbox")
	http.ServeContent(w, r, "", st.ModTime(), f)
}

// ---- what uses a family ----

// bookFontUse is one family named by one book: by which of its styles,
// how many header/footer bands, and how many chapters' text.
type bookFontUse struct {
	Volume   int64    `json:"volume"`
	Book     string   `json:"book"`
	Family   string   `json:"family"`
	Styles   []string `json:"styles"`
	Bands    int      `json:"bands"`
	Chapters int      `json:"chapters"`
}

// bookFontUses lists the families every book of a story names (only the
// book vol when vol >= 0). A book counts when it was set up or has
// chapters; one never set up names its defaults.
func bookFontUses(db editorDB, vol int64) ([]bookFontUse, error) {
	type bookKey struct {
		vol    int64
		family string
	}
	books := map[int64]string{0: ""}
	rows, err := db.Query(`SELECT id, title FROM volumes`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		var title string
		if err := rows.Scan(&id, &title); err != nil {
			rows.Close()
			return nil, err
		}
		books[id] = title
	}
	rows.Close()
	counted := map[int64]bool{}
	rows, err = db.Query(`SELECT volume_id FROM book_settings`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		rows.Scan(&id)
		counted[id] = true
	}
	rows.Close()
	rows, err = db.Query(`SELECT DISTINCT COALESCE(volume_id, 0) FROM chapters`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		rows.Scan(&id)
		counted[id] = true
	}
	rows.Close()

	uses := map[bookKey]*bookFontUse{}
	var order []bookKey
	use := func(v int64, family string) *bookFontUse {
		family = strings.TrimSpace(family)
		k := bookKey{v, strings.ToLower(family)}
		u, ok := uses[k]
		if !ok {
			u = &bookFontUse{Volume: v, Book: books[v], Family: family, Styles: []string{}}
			uses[k] = u
			order = append(order, k)
		}
		return u
	}
	for v := range books {
		if !counted[v] || (vol >= 0 && v != vol) {
			continue
		}
		raw, _, err := loadBookSettings(db, v)
		if err != nil {
			return nil, err
		}
		var s struct {
			Styles []bookStyle `json:"styles"`
			Layout struct {
				Header map[string]struct {
					Font *string `json:"font"`
				} `json:"header"`
				Footer map[string]struct {
					Font *string `json:"font"`
				} `json:"footer"`
			} `json:"layout"`
		}
		decodeLoose(raw, &s)
		for _, st := range s.Styles {
			if f, ok := st.Text["font"].(string); ok && strings.TrimSpace(f) != "" {
				u := use(v, f)
				u.Styles = append(u.Styles, st.Name)
			}
		}
		for _, bands := range []map[string]struct {
			Font *string `json:"font"`
		}{s.Layout.Header, s.Layout.Footer} {
			for _, b := range bands {
				if b.Font != nil && strings.TrimSpace(*b.Font) != "" {
					use(v, *b.Font).Bands++
				}
			}
		}
	}
	// The text's own font marks: only docs that name a font are read.
	q := `SELECT COALESCE(c.volume_id, 0), t.doc FROM chapter_texts t JOIN chapters c ON c.id = t.chapter_id WHERE t.doc LIKE '%"font"%'`
	args := []any{}
	if vol >= 0 {
		q += ` AND c.volume_id IS ?`
		args = append(args, volumeNull(vol))
	}
	rows, err = db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var v int64
		var doc string
		if err := rows.Scan(&v, &doc); err != nil {
			return nil, err
		}
		var d pmNode
		if json.Unmarshal([]byte(doc), &d) != nil {
			continue
		}
		for f := range docFontFamilies(&d) {
			use(v, f).Chapters++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]bookFontUse, 0, len(order))
	for _, k := range order {
		out = append(out, *uses[k])
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Volume != out[j].Volume {
			return out[i].Volume < out[j].Volume
		}
		return strings.ToLower(out[i].Family) < strings.ToLower(out[j].Family)
	})
	return out, nil
}

// docFontFamilies: the families a doc's font marks name (as written; keyed
// by lower case).
func docFontFamilies(d *pmNode) map[string]bool {
	seen := map[string]bool{}
	out := map[string]bool{}
	pmWalk(d, func(n *pmNode) {
		for _, m := range n.Marks {
			if m.Type != "font" {
				continue
			}
			if f, ok := m.Attrs["family"].(string); ok && strings.TrimSpace(f) != "" && !seen[strings.ToLower(strings.TrimSpace(f))] {
				seen[strings.ToLower(strings.TrimSpace(f))] = true
				out[strings.TrimSpace(f)] = true
			}
		}
	})
	return out
}

// bookFontsHandler answers GET /api/books/{volume}/fonts (a story route).
func bookFontsHandler(db *sql.DB) http.HandlerFunc {
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
				http.Error(w, "failed to load the book", http.StatusInternalServerError)
			}
			return
		}
		uses, err := bookFontUses(db, vol)
		if err != nil {
			log.Printf("book %d fonts: %v", vol, err)
			http.Error(w, "failed to load the book", http.StatusInternalServerError)
			return
		}
		writeJSON(w, uses)
	}
}

// fontUseOut is one book, in one story, naming a library family.
type fontUseOut struct {
	StoryID string `json:"story_id"`
	Story   string `json:"story"`
	bookFontUse
}

// usedIn scans every story for the library's families: family id → uses.
func (l *fontLibrary) usedIn() (map[int64][]fontUseOut, error) {
	fams := map[string]int64{}
	rows, err := l.db.Query(`SELECT id, name FROM font_families`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		var name string
		rows.Scan(&id, &name)
		fams[strings.ToLower(name)] = id
	}
	rows.Close()
	out := map[int64][]fontUseOut{}
	if len(fams) == 0 || l.mgr == nil {
		return out, nil
	}
	type story struct{ id, name string }
	var stories []story
	rows, err = l.db.Query(`SELECT id, name FROM stories ORDER BY created_at, rowid`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var s story
		rows.Scan(&s.id, &s.name)
		stories = append(stories, s)
	}
	rows.Close()
	for _, s := range stories {
		app, err := l.mgr.open(s.id)
		if err != nil {
			continue // a story at a newer schema, say: its uses can't be read
		}
		uses, err := bookFontUses(app.db, -1)
		if err != nil {
			log.Printf("fonts used in story %s: %v", s.id, err)
			continue
		}
		for _, u := range uses {
			if id, ok := fams[strings.ToLower(u.Family)]; ok {
				out[id] = append(out[id], fontUseOut{StoryID: s.id, Story: s.name, bookFontUse: u})
			}
		}
	}
	return out, nil
}

func (l *fontLibrary) usedHandler(w http.ResponseWriter, r *http.Request) {
	used, err := l.usedIn()
	if err != nil {
		log.Printf("fonts used: %v", err)
		http.Error(w, "failed to load the fonts", http.StatusInternalServerError)
		return
	}
	out := map[string][]fontUseOut{}
	for id, u := range used {
		out[strconv.FormatInt(id, 10)] = u
	}
	writeJSON(w, out)
}

func (l *fontLibrary) listHandler(w http.ResponseWriter, r *http.Request) {
	list, err := l.list()
	if err != nil {
		log.Printf("list fonts: %v", err)
		http.Error(w, "failed to load the fonts", http.StatusInternalServerError)
		return
	}
	writeJSON(w, list)
}

func (l *fontLibrary) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/fonts", l.listHandler)
	mux.HandleFunc("POST /api/fonts", l.uploadHandler)
	mux.HandleFunc("GET /api/fonts/used", l.usedHandler)
	mux.HandleFunc("GET /api/fonts/line-metrics", l.lineMetricsHandler)
	mux.HandleFunc("PUT /api/fonts/{id}", l.updateHandler)
	mux.HandleFunc("DELETE /api/fonts/{id}", l.deleteHandler)
	mux.HandleFunc("POST /api/fonts/{id}/license-file", l.licenseUploadHandler)
	mux.HandleFunc("GET /api/fonts/{id}/license-file", l.licenseGetHandler)
	mux.HandleFunc("DELETE /api/fonts/{id}/license-file", l.licenseDeleteHandler)
	mux.HandleFunc("DELETE /api/font-files/{fid}", l.deleteFileHandler)
	mux.HandleFunc("GET /uploads/fonts/{name}", l.fontFileHandler)
}

// ---- exports ----

type fontLibraryKey struct{}

// withFontLibrary hands the library to a story's handlers (the EPUB export).
func withFontLibrary(ctx context.Context, l *fontLibrary) context.Context {
	if l == nil {
		return ctx
	}
	return context.WithValue(ctx, fontLibraryKey{}, l)
}

func fontLibraryFrom(ctx context.Context) *fontLibrary {
	l, _ := ctx.Value(fontLibraryKey{}).(*fontLibrary)
	return l
}

// exportFont is one font file put into an export.
type exportFont struct {
	Family string
	Weight int
	Italic bool
	Format string // ttf, otf, woff, woff2
	Data   []byte
}

// embeddable gives the files of the families named (any case) that may go
// into an e-book: the family's license allows e-books and the file's
// fsType allows embedding (an unread fsType, as a WOFF2's, is left to the
// license).
func (l *fontLibrary) embeddable(families []string) []exportFont {
	if len(families) == 0 {
		return nil
	}
	want := map[string]bool{}
	for _, f := range families {
		want[strings.ToLower(strings.TrimSpace(f))] = true
	}
	list, err := l.list()
	if err != nil {
		log.Printf("fonts for an export: %v", err)
		return nil
	}
	var out []exportFont
	var total int64
	for _, fam := range list {
		if !want[strings.ToLower(fam.Name)] || !fam.Uses.Ebook {
			continue
		}
		for _, f := range fam.Files {
			if f.forbidsEmbedding() || !fontFileNamePattern.MatchString(f.File) {
				continue
			}
			if total+f.Size > maxEmbedFontBytes {
				continue
			}
			data, err := os.ReadFile(filepath.Join(l.dir, f.File))
			if err != nil {
				continue
			}
			total += int64(len(data))
			out = append(out, exportFont{Family: fam.Name, Weight: f.Weight, Italic: f.Italic, Format: f.Format, Data: data})
		}
	}
	return out
}

// exportFamilies: the families an e-book's CSS names (its styles' and its
// text's), not the header and footer bands'.
func exportFamilies(b *exportBook) []string {
	seen := map[string]bool{}
	var out []string
	add := func(f string) {
		f = strings.TrimSpace(f)
		if f != "" && !seen[strings.ToLower(f)] {
			seen[strings.ToLower(f)] = true
			out = append(out, f)
		}
	}
	for _, s := range b.Styles {
		if f, ok := s.Text["font"].(string); ok {
			add(f)
		}
	}
	for _, c := range b.Chapters {
		for f := range docFontFamilies(c.Doc) {
			add(f)
		}
	}
	sort.Strings(out)
	return out
}
