package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
)

// The writing editor's books (docs/EDITOR.md is the contract). A volume is a
// book; the chapters without a volume are book 0. Each book keeps its
// settings (page type, page, styles) as JSON in book_settings; a book never
// set up has no row and gets the a5 defaults. Chapter texts and their
// versions are in chaptertext.go, the .docx upload in editorimport.go.

const bookSettingsTableSQL = `CREATE TABLE book_settings (
	volume_id  INTEGER PRIMARY KEY, -- a volume's id, or 0 for the chapters without one
	settings   TEXT NOT NULL,
	is_set_up  INTEGER NOT NULL DEFAULT 0,
	updated_at TEXT NOT NULL DEFAULT (datetime('now'))
)`

// chapter_texts holds each chapter's current text (a ProseMirror document,
// see pmdoc.go). version goes up by one on every save, so a save made from
// an older copy is refused (409). auto_since is when the writing since the
// last automatic version started.
const chapterTextsTableSQL = `CREATE TABLE chapter_texts (
	chapter_id INTEGER PRIMARY KEY REFERENCES chapters(id) ON DELETE CASCADE,
	doc        TEXT NOT NULL,
	version    INTEGER NOT NULL DEFAULT 1,
	words      INTEGER NOT NULL DEFAULT 0,
	updated_at TEXT NOT NULL DEFAULT (datetime('now')),
	auto_since TEXT NOT NULL DEFAULT (datetime('now'))
)`

const chapterTextVersionsTableSQL = `CREATE TABLE chapter_text_versions (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	chapter_id INTEGER NOT NULL REFERENCES chapters(id) ON DELETE CASCADE,
	doc        TEXT NOT NULL,
	words      INTEGER NOT NULL DEFAULT 0,
	label      TEXT NOT NULL DEFAULT '',
	automatic  INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL DEFAULT (datetime('now'))
)`

// schema44Statements create what schema 44 added. The fresh schema runs them
// too, after the chapters' tables.
var schema44Statements = []string{
	bookSettingsTableSQL,
	// book_settings.volume_id can't be a foreign key (0 is no volume), so
	// this does what ON DELETE CASCADE would. The volume's chapters fall
	// back to book 0, which has its own settings.
	`CREATE TRIGGER book_settings_volume_delete AFTER DELETE ON volumes BEGIN
		DELETE FROM book_settings WHERE volume_id = OLD.id;
	END`,
	chapterTextsTableSQL,
	chapterTextVersionsTableSQL,
	`CREATE INDEX idx_chapter_text_versions_chapter ON chapter_text_versions(chapter_id, automatic)`,
}

// editorDB is what the editor's helpers need from a database or a
// transaction.
type editorDB interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

const (
	maxBookSettingsBytes = 512 << 10
	maxBookStyles        = 300
	maxStyleField        = 200 // runes in a style's id or name
)

// pageTypeDef is one page type's paper and body text size.
type pageTypeDef struct {
	widthMM, heightMM float64
	bodyPt            float64
	margins           [4]float64 // top, bottom, inner, outer
	gutterMM          float64
}

var pageTypeDefs = map[string]pageTypeDef{
	"a4":     {210, 297, 12, [4]float64{25, 25, 25, 25}, 5},
	"a5":     {148, 210, 10.5, [4]float64{18, 18, 15, 15}, 5},
	"a6":     {105, 148, 8, [4]float64{12, 12, 12, 12}, 5},
	"custom": {127, 203.2, 10.5, [4]float64{15, 15, 15, 15}, 5}, // 5 × 8 in until changed
	"online": {148, 210, 12, [4]float64{18, 18, 15, 15}, 5},     // the page is ignored
}

// pageTypeOrder lists the page types for the client's choice.
var pageTypeOrder = []string{"a4", "a5", "a6", "custom", "online"}

// bookPage and bookStyle (the settings' "page" and styles) are defined in
// docx.go, which fills them from a Word file.

func defaultPage(pageType string) bookPage {
	d := pageTypeDefs[pageType]
	return bookPage{
		WidthMM: d.widthMM, HeightMM: d.heightMM,
		MarginTopMM: d.margins[0], MarginBottomMM: d.margins[1],
		MarginInnerMM: d.margins[2], MarginOuterMM: d.margins[3],
		GutterMM: d.gutterMM, Mirror: true,
	}
}

// pageProblem says what is wrong with a page, or "".
func pageProblem(p bookPage) string {
	for _, v := range []float64{p.WidthMM, p.HeightMM, p.MarginTopMM, p.MarginBottomMM, p.MarginInnerMM, p.MarginOuterMM, p.GutterMM} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return "the page size is not valid"
		}
	}
	if p.WidthMM < 50 || p.WidthMM > 1000 || p.HeightMM < 50 || p.HeightMM > 1000 {
		return "a page must be between 50 and 1000 mm on each side"
	}
	for _, m := range []float64{p.MarginTopMM, p.MarginBottomMM, p.MarginInnerMM, p.MarginOuterMM} {
		if m < 0 || m > 200 {
			return "the margins must be between 0 and 200 mm"
		}
	}
	if p.GutterMM < 0 || p.GutterMM > 100 {
		return "the gutter must be between 0 and 100 mm"
	}
	if p.WidthMM-p.MarginInnerMM-p.MarginOuterMM-p.GutterMM < 20 || p.HeightMM-p.MarginTopMM-p.MarginBottomMM < 20 {
		return "the margins leave no room for the text"
	}
	return ""
}

// builtinStyleIDs are always present: renamable, never deletable.
var builtinStyleIDs = []string{"normal", "heading1", "heading2", "heading3", "title", "subtitle", "quote"}

const defaultBookFont = "Georgia"

func roundPt(v float64) float64 { return math.Round(v*10) / 10 }

// defaultStyles are the built-in styles for a page type, sized from its
// body text.
func defaultStyles(pageType string) []bookStyle {
	b := pageTypeDefs[pageType].bodyPt
	online := pageType == "online"
	normal := "normal"
	align, center := "justify", "center"
	indent, after, line := roundPt(14.2), 0.0, 1.15
	if online {
		align, center, indent, after = "left", "left", 0, b
	}
	para := func(kv ...any) map[string]any {
		m := map[string]any{}
		for i := 0; i+1 < len(kv); i += 2 {
			m[kv[i].(string)] = kv[i+1]
		}
		return m
	}
	style := func(id, name string, p, t map[string]any) bookStyle {
		s := bookStyle{ID: id, Name: name, Kind: "paragraph", Builtin: true, Para: p, Text: t}
		if id != "normal" {
			s.BasedOn = &normal
		}
		return s
	}
	return []bookStyle{
		style("normal", "Normal",
			para("align", align, "indentFirst", indent, "indentLeft", 0.0, "indentRight", 0.0,
				"spaceBefore", 0.0, "spaceAfter", after, "lineHeight", line),
			para("font", defaultBookFont, "size", b, "color", nil)),
		style("heading1", "Heading 1",
			para("align", center, "indentFirst", 0.0, "spaceBefore", roundPt(b*3), "spaceAfter", roundPt(b*1.5), "keepNext", true),
			para("size", roundPt(b*1.6), "bold", true)),
		style("heading2", "Heading 2",
			para("align", center, "indentFirst", 0.0, "spaceBefore", roundPt(b*1.5), "spaceAfter", roundPt(b*0.75), "keepNext", true),
			para("size", roundPt(b*1.3), "bold", true)),
		style("heading3", "Heading 3",
			para("align", "left", "indentFirst", 0.0, "spaceBefore", roundPt(b), "spaceAfter", roundPt(b*0.5), "keepNext", true),
			para("size", roundPt(b*1.15), "bold", true, "italic", true)),
		style("title", "Title",
			para("align", "center", "indentFirst", 0.0, "spaceBefore", 0.0, "spaceAfter", roundPt(b), "keepNext", true),
			para("size", roundPt(b*2.4))),
		style("subtitle", "Subtitle",
			para("align", "center", "indentFirst", 0.0, "spaceAfter", roundPt(b*2), "keepNext", true),
			para("size", roundPt(b*1.4), "italic", true)),
		style("quote", "Quote",
			para("align", align, "indentFirst", 0.0, "indentLeft", 28.0, "indentRight", 28.0,
				"spaceBefore", roundPt(b*0.5), "spaceAfter", roundPt(b*0.5)),
			para("italic", true)),
	}
}

// defaultBookSettings is a page type's settings, as JSON.
func defaultBookSettings(pageType string) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{
		"page_type": pageType,
		"page":      defaultPage(pageType),
		"styles":    defaultStyles(pageType),
	})
	return raw
}

// cleanBookSettings checks posted settings and gives them back as they will
// be stored: a known page type, a page that fits (the standard sizes'
// paper is theirs), and styles that include every built-in one. Keys it
// doesn't know (stage 3's headers and footers…) are kept.
func cleanBookSettings(raw json.RawMessage) (json.RawMessage, string) {
	if len(raw) > maxBookSettingsBytes {
		return nil, "the book's settings are too large"
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return nil, "the book's settings are not valid"
	}
	var pageType string
	if err := json.Unmarshal(m["page_type"], &pageType); err != nil {
		return nil, "pick a page type"
	}
	def, ok := pageTypeDefs[pageType]
	if !ok {
		return nil, "unknown page type"
	}
	page := defaultPage(pageType)
	if p, ok := m["page"]; ok && string(p) != "null" {
		if err := json.Unmarshal(p, &page); err != nil {
			return nil, "the page size is not valid"
		}
	}
	if pageType != "custom" && pageType != "online" {
		page.WidthMM, page.HeightMM = def.widthMM, def.heightMM
	}
	if msg := pageProblem(page); msg != "" {
		if pageType != "online" {
			return nil, msg
		}
		page = defaultPage(pageType) // ignored for online anyway
	}
	m["page"], _ = json.Marshal(page)

	styles, msg := cleanStyles(m["styles"], pageType)
	if msg != "" {
		return nil, msg
	}
	m["styles"] = styles
	out, err := json.Marshal(m)
	if err != nil {
		return nil, "the book's settings are not valid"
	}
	return out, ""
}

// cleanStyles checks a book's styles: each one an object with an id, a name
// and a kind, ids unique, every built-in present (a missing one comes back
// from the defaults). Everything else in a style is kept as sent.
func cleanStyles(raw json.RawMessage, pageType string) (json.RawMessage, string) {
	var list []json.RawMessage
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, "the styles are not valid"
		}
	}
	if len(list) > maxBookStyles {
		return nil, fmt.Sprintf("a book holds at most %d styles", maxBookStyles)
	}
	seen := map[string]bool{}
	for _, s := range list {
		var head struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Kind string `json:"kind"`
		}
		var obj map[string]json.RawMessage
		if json.Unmarshal(s, &obj) != nil || obj == nil || json.Unmarshal(s, &head) != nil {
			return nil, "the styles are not valid"
		}
		if strings.TrimSpace(head.ID) == "" || strings.TrimSpace(head.Name) == "" {
			return nil, "every style needs a name"
		}
		if tooLong(head.ID, maxStyleField) || tooLong(head.Name, maxStyleField) {
			return nil, "that text is too long"
		}
		if head.Kind != "paragraph" && head.Kind != "character" {
			return nil, "a style is either a paragraph style or a character style"
		}
		if seen[head.ID] {
			return nil, "two styles have the same id"
		}
		seen[head.ID] = true
	}
	for _, d := range defaultStyles(pageType) {
		if !seen[d.ID] {
			b, _ := json.Marshal(d)
			list = append(list, b)
		}
	}
	out, err := json.Marshal(list)
	if err != nil {
		return nil, "the styles are not valid"
	}
	return out, ""
}

// bookChapter is a chapter as a book lists it.
type bookChapter struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Position int    `json:"position"`
	Words    int    `json:"words"`
	Version  int64  `json:"version"`
}

type bookOut struct {
	Volume   int64                      `json:"volume"`
	Title    string                     `json:"title"` // the volume's; "" for book 0
	Settings json.RawMessage            `json:"settings"`
	IsSetUp  bool                       `json:"is_set_up"`
	Chapters []bookChapter              `json:"chapters"`
	Words    int                        `json:"words"`
	Defaults map[string]json.RawMessage `json:"defaults"` // every page type's settings
	Types    []string                   `json:"page_types"`
}

// volumeNull is a book number as the chapters' volume_id: 0 is NULL.
func volumeNull(vol int64) sql.NullInt64 {
	return sql.NullInt64{Int64: vol, Valid: vol != 0}
}

var errNoBook = errors.New("book not found")

// bookTitle checks that a book exists (0 always does) and gives its title.
func bookTitle(db editorDB, vol int64) (string, error) {
	if vol == 0 {
		return "", nil
	}
	if vol < 0 {
		return "", errNoBook
	}
	var title string
	err := db.QueryRow(`SELECT title FROM volumes WHERE id = ?`, vol).Scan(&title)
	if err == sql.ErrNoRows {
		return "", errNoBook
	}
	return title, err
}

func bookChapters(db editorDB, vol int64) ([]bookChapter, error) {
	rows, err := db.Query(`
		SELECT c.id, c.title, c.position, COALESCE(t.words, 0), COALESCE(t.version, 0)
		FROM chapters c LEFT JOIN chapter_texts t ON t.chapter_id = c.id
		WHERE c.volume_id IS ?
		ORDER BY c.position, c.id`, volumeNull(vol))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []bookChapter{}
	for rows.Next() {
		var c bookChapter
		if err := rows.Scan(&c.ID, &c.Title, &c.Position, &c.Words, &c.Version); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// loadBookSettings gives a book's settings, or the a5 defaults when it was
// never set up.
func loadBookSettings(db editorDB, vol int64) (json.RawMessage, bool, error) {
	var s string
	var setUp bool
	err := db.QueryRow(`SELECT settings, is_set_up FROM book_settings WHERE volume_id = ?`, vol).Scan(&s, &setUp)
	if err == sql.ErrNoRows {
		return defaultBookSettings("a5"), false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return json.RawMessage(s), setUp, nil
}

func loadBook(db editorDB, vol int64) (*bookOut, error) {
	title, err := bookTitle(db, vol)
	if err != nil {
		return nil, err
	}
	b := &bookOut{Volume: vol, Title: title, Types: pageTypeOrder, Defaults: map[string]json.RawMessage{}}
	if b.Settings, b.IsSetUp, err = loadBookSettings(db, vol); err != nil {
		return nil, err
	}
	if b.Chapters, err = bookChapters(db, vol); err != nil {
		return nil, err
	}
	for _, c := range b.Chapters {
		b.Words += c.Words
	}
	for _, t := range pageTypeOrder {
		b.Defaults[t] = defaultBookSettings(t)
	}
	return b, nil
}

// bookVolume reads {volume} from the path.
func bookVolume(r *http.Request) (int64, bool) {
	v, err := strconv.ParseInt(r.PathValue("volume"), 10, 64)
	return v, err == nil && v >= 0
}

func writeBook(w http.ResponseWriter, db editorDB, vol int64) {
	b, err := loadBook(db, vol)
	if errors.Is(err, errNoBook) {
		http.Error(w, "book not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("load book %d: %v", vol, err)
		http.Error(w, "failed to load the book", http.StatusInternalServerError)
		return
	}
	writeJSON(w, b)
}

func getBookHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vol, ok := bookVolume(r)
		if !ok {
			http.Error(w, "book not found", http.StatusNotFound)
			return
		}
		writeBook(w, db, vol)
	}
}

// saveBookHandler stores a book's settings. With switch_label (the page type
// changes), every written chapter of the book is first kept as a version
// with that label, in the same transaction.
func saveBookHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vol, ok := bookVolume(r)
		if !ok {
			http.Error(w, "book not found", http.StatusNotFound)
			return
		}
		var in struct {
			Settings    json.RawMessage `json:"settings"`
			SwitchLabel string          `json:"switch_label"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBookSettingsBytes+(64<<10))).Decode(&in); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		settings, msg := cleanBookSettings(in.Settings)
		if msg != "" {
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		label := strings.TrimSpace(in.SwitchLabel)
		if tooLong(label, maxVersionLabel) {
			http.Error(w, "that text is too long", http.StatusBadRequest)
			return
		}
		if _, err := bookTitle(db, vol); err != nil {
			if errors.Is(err, errNoBook) {
				http.Error(w, "book not found", http.StatusNotFound)
			} else {
				http.Error(w, "failed to save the book", http.StatusInternalServerError)
			}
			return
		}
		now := sqlTime(editorNow())
		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to save the book", http.StatusInternalServerError)
			return
		}
		fail := func(err error) {
			tx.Rollback()
			log.Printf("save book %d: %v", vol, err)
			http.Error(w, "failed to save the book", http.StatusInternalServerError)
		}
		if label != "" {
			if _, err := tx.Exec(`INSERT INTO chapter_text_versions (chapter_id, doc, words, label, automatic, created_at)
				SELECT t.chapter_id, t.doc, t.words, ?, 0, ?
				FROM chapter_texts t JOIN chapters c ON c.id = t.chapter_id
				WHERE c.volume_id IS ?
				ORDER BY c.position, c.id`, label, now, volumeNull(vol)); err != nil {
				fail(err)
				return
			}
		}
		if _, err := tx.Exec(`INSERT INTO book_settings (volume_id, settings, is_set_up, updated_at) VALUES (?, ?, 1, ?)
			ON CONFLICT(volume_id) DO UPDATE SET settings = excluded.settings, is_set_up = 1, updated_at = excluded.updated_at`,
			vol, string(settings), now); err != nil {
			fail(err)
			return
		}
		if err := tx.Commit(); err != nil {
			fail(err)
			return
		}
		writeBook(w, db, vol)
	}
}

// writeJSONStatus is writeJSON with a status other than 200.
func writeJSONStatus(w http.ResponseWriter, status int, v any) {
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(v)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(buf.Bytes())
}
