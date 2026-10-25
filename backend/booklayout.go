package main

// Stage 3's book settings (docs/EDITOR.md, "Stage 3 and 4 additions"): the
// book's details ("meta"), its layout (headers and footers, page numbers,
// where chapters start) and options per chapter ("chapter_options"), plus
// GET /api/books/{volume}/docs, every chapter's text for the page view and
// the exports.
//
// API:
//
//	cleanBookExtras(m map[string]json.RawMessage) string
//	    Called by cleanBookSettings: fills the missing keys of meta, layout
//	    and chapter_options with their defaults, checks the known ones and
//	    keeps keys it doesn't know. "" or a message for a 400.
//	bookLayoutOf(settings json.RawMessage) bookExtras
//	    The three read as Go values, defaults for anything missing or
//	    unreadable: for the exports.
//	bandFields                 the fields a band's text may hold
//
// A band's texts are kept as written: a {word} that isn't a field is
// literal text wherever the band is drawn.

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	maxBandText       = 300  // runes in a band's left, center or right
	maxMetaField      = 300  // runes in a meta field
	maxMetaLong       = 5000 // runes in meta.description
	maxChapterOptions = 10000
	maxPageStart      = 99999
)

// bandFields are the fields a header or footer text may hold.
var bandFields = []string{"{page}", "{chapter}", "{chapter_number}", "{book}", "{author}"}

// numberFormats are layout.numbering.format's values.
var numberFormats = map[string]bool{"arabic": true, "roman": true, "upper_roman": true, "alpha": true, "upper_alpha": true}

type bookBand struct {
	Left      string   `json:"left"`
	Center    string   `json:"center"`
	Right     string   `json:"right"`
	Font      *string  `json:"font"`
	Size      *float64 `json:"size"`
	Italic    bool     `json:"italic"`
	Smallcaps bool     `json:"smallcaps"`
	Uppercase bool     `json:"uppercase"`
}

// empty: nothing is drawn.
func (b bookBand) empty() bool {
	return strings.TrimSpace(b.Left+b.Center+b.Right) == ""
}

type bookBands struct {
	Odd   bookBand `json:"odd"`
	Even  bookBand `json:"even"`
	First bookBand `json:"first"`
}

type bookNumbering struct {
	Format string `json:"format"`
	Start  int    `json:"start"`
}

type bookLayout struct {
	DifferentFirst   bool          `json:"different_first"`
	DifferentOddEven bool          `json:"different_odd_even"`
	Header           bookBands     `json:"header"`
	Footer           bookBands     `json:"footer"`
	ChapterStart     string        `json:"chapter_start"` // "next" or "odd"
	Numbering        bookNumbering `json:"numbering"`
	TocDots          bool          `json:"toc_dots"`
}

type bookMeta struct {
	Title       string `json:"title"`
	Subtitle    string `json:"subtitle"`
	Author      string `json:"author"`
	Language    string `json:"language"`
	Publisher   string `json:"publisher"`
	ISBN        string `json:"isbn"`
	Rights      string `json:"rights"`
	Description string `json:"description"`
}

type chapterOption struct {
	FrontMatter   bool `json:"front_matter"`
	NumberRestart *int `json:"number_restart"`
	NoHeader      bool `json:"no_header"`
}

// bookExtras is what bookLayoutOf reads.
type bookExtras struct {
	Meta           bookMeta
	Layout         bookLayout
	ChapterOptions map[int64]chapterOption
}

func defaultBookMeta() bookMeta { return bookMeta{Language: "en"} }

// defaultBookLayout: the chapter's title over odd pages, the book's over
// even ones, nothing over a chapter's first page; the page number centred
// at the foot of every page.
func defaultBookLayout() bookLayout {
	page := bookBand{Center: "{page}"}
	return bookLayout{
		DifferentFirst:   true,
		DifferentOddEven: true,
		Header:           bookBands{Odd: bookBand{Center: "{chapter}"}, Even: bookBand{Center: "{book}"}},
		Footer:           bookBands{Odd: page, Even: page, First: page},
		ChapterStart:     "next",
		Numbering:        bookNumbering{Format: "arabic", Start: 1},
		TocDots:          true,
	}
}

func defaultChapterOption() chapterOption { return chapterOption{} }

// asMap gives a Go value as a JSON object decoded generically.
func asMap(v any) map[string]any {
	b, _ := json.Marshal(v)
	var m map[string]any
	decodeNumbers(b, &m)
	return m
}

func decodeNumbers(b []byte, into any) error {
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.UseNumber()
	return dec.Decode(into)
}

// withDefaults puts every key of def that have lacks (or holds as null)
// into have, going into objects both hold.
func withDefaults(have, def map[string]any) {
	for k, dv := range def {
		hv, ok := have[k]
		if !ok || hv == nil {
			have[k] = dv
			continue
		}
		hm, hIsMap := hv.(map[string]any)
		dm, dIsMap := dv.(map[string]any)
		if hIsMap && dIsMap {
			withDefaults(hm, dm)
		}
	}
}

// objectWithDefaults reads raw (missing or null: {}) as an object and
// fills in def. ok is false when raw isn't an object.
func objectWithDefaults(raw json.RawMessage, def any) (map[string]any, bool) {
	m := map[string]any{}
	if len(raw) > 0 && string(raw) != "null" {
		if err := decodeNumbers(raw, &m); err != nil || m == nil {
			return nil, false
		}
	}
	withDefaults(m, asMap(def))
	return m, true
}

// typed reads a generic object into a Go value (type check).
func typed(m map[string]any, into any) bool {
	b, err := json.Marshal(m)
	if err != nil {
		return false
	}
	return json.Unmarshal(b, into) == nil
}

func bandProblem(b bookBand) string {
	for _, s := range []string{b.Left, b.Center, b.Right} {
		if tooLong(s, maxBandText) || !utf8.ValidString(s) {
			return "a header or footer text is too long"
		}
	}
	if b.Font != nil && (tooLong(*b.Font, 100)) {
		return "the book's layout is not valid"
	}
	if b.Size != nil && (*b.Size < 4 || *b.Size > 72) {
		return "a header or footer's size must be between 4 and 72 pt"
	}
	return ""
}

func layoutProblem(l bookLayout) string {
	for _, b := range []bookBand{l.Header.Odd, l.Header.Even, l.Header.First, l.Footer.Odd, l.Footer.Even, l.Footer.First} {
		if msg := bandProblem(b); msg != "" {
			return msg
		}
	}
	if l.ChapterStart != "next" && l.ChapterStart != "odd" {
		return "a chapter starts on the next page or on a right-hand page"
	}
	if !numberFormats[l.Numbering.Format] {
		return "unknown page number format"
	}
	if l.Numbering.Start < 0 || l.Numbering.Start > maxPageStart {
		return "a page number must be between 0 and 99999"
	}
	return ""
}

func metaProblem(m bookMeta) string {
	for _, s := range []string{m.Title, m.Subtitle, m.Author, m.Language, m.Publisher, m.ISBN, m.Rights} {
		if tooLong(s, maxMetaField) {
			return "that text is too long"
		}
	}
	if tooLong(m.Description, maxMetaLong) {
		return "that text is too long"
	}
	return ""
}

// cleanBookExtras checks and completes meta, layout and chapter_options in
// a book's settings (decoded as raw keys), in place.
func cleanBookExtras(m map[string]json.RawMessage) string {
	meta, ok := objectWithDefaults(m["meta"], defaultBookMeta())
	var bm bookMeta
	if !ok || !typed(meta, &bm) {
		return "the book's details are not valid"
	}
	if msg := metaProblem(bm); msg != "" {
		return msg
	}

	layout, ok := objectWithDefaults(m["layout"], defaultBookLayout())
	var bl bookLayout
	if !ok || !typed(layout, &bl) {
		return "the book's layout is not valid"
	}
	if msg := layoutProblem(bl); msg != "" {
		return msg
	}

	opts := map[string]any{}
	if raw := m["chapter_options"]; len(raw) > 0 && string(raw) != "null" {
		var in map[string]json.RawMessage
		if json.Unmarshal(raw, &in) != nil || in == nil {
			return "the chapter options are not valid"
		}
		if len(in) > maxChapterOptions {
			return "the chapter options are not valid"
		}
		for k, v := range in {
			if id, err := strconv.ParseInt(k, 10, 64); err != nil || id < 1 || strconv.FormatInt(id, 10) != k {
				return "the chapter options are not valid"
			}
			o, ok := objectWithDefaults(v, defaultChapterOption())
			var co chapterOption
			if !ok || !typed(o, &co) {
				return "the chapter options are not valid"
			}
			if co.NumberRestart != nil && (*co.NumberRestart < 0 || *co.NumberRestart > maxPageStart) {
				return "a page number must be between 0 and 99999"
			}
			opts[k] = o
		}
	}

	for key, v := range map[string]any{"meta": meta, "layout": layout, "chapter_options": opts} {
		b, err := json.Marshal(v)
		if err != nil {
			return "the book's settings are not valid"
		}
		m[key] = b
	}
	return ""
}

// bookLayoutOf reads a book's stored settings' meta, layout and chapter
// options, with defaults for what is missing or unreadable.
func bookLayoutOf(settings json.RawMessage) bookExtras {
	out := bookExtras{Meta: defaultBookMeta(), Layout: defaultBookLayout(), ChapterOptions: map[int64]chapterOption{}}
	var m map[string]json.RawMessage
	if json.Unmarshal(settings, &m) != nil {
		return out
	}
	if meta, ok := objectWithDefaults(m["meta"], defaultBookMeta()); ok {
		var bm bookMeta
		if typed(meta, &bm) {
			out.Meta = bm
		}
	}
	if layout, ok := objectWithDefaults(m["layout"], defaultBookLayout()); ok {
		var bl bookLayout
		if typed(layout, &bl) && layoutProblem(bl) == "" {
			out.Layout = bl
		}
	}
	var opts map[string]json.RawMessage
	if json.Unmarshal(m["chapter_options"], &opts) == nil {
		for k, v := range opts {
			id, err := strconv.ParseInt(k, 10, 64)
			if err != nil || id < 1 {
				continue
			}
			o, ok := objectWithDefaults(v, defaultChapterOption())
			var co chapterOption
			if ok && typed(o, &co) {
				out.ChapterOptions[id] = co
			}
		}
	}
	return out
}

// --- GET /api/books/{volume}/docs ---

type bookDoc struct {
	ID    int64           `json:"id"`
	Title string          `json:"title"`
	Doc   json.RawMessage `json:"doc"`
}

// bookDocs gives every chapter of a book with its text, in reading order;
// a chapter never written has an empty doc.
func bookDocs(db editorDB, vol int64) ([]bookDoc, error) {
	if _, err := bookTitle(db, vol); err != nil {
		return nil, err
	}
	rows, err := db.Query(`
		SELECT c.id, c.title, COALESCE(t.doc, '')
		FROM chapters c LEFT JOIN chapter_texts t ON t.chapter_id = c.id
		WHERE c.volume_id IS ?
		ORDER BY c.position, c.id`, volumeNull(vol))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []bookDoc{}
	for rows.Next() {
		var d bookDoc
		var doc string
		if err := rows.Scan(&d.ID, &d.Title, &doc); err != nil {
			return nil, err
		}
		if doc == "" {
			doc = emptyChapterDoc
		}
		d.Doc = json.RawMessage(doc)
		out = append(out, d)
	}
	return out, rows.Err()
}

func getBookDocsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vol, ok := bookVolume(r)
		if !ok {
			http.Error(w, "book not found", http.StatusNotFound)
			return
		}
		docs, err := bookDocs(db, vol)
		if errors.Is(err, errNoBook) {
			http.Error(w, "book not found", http.StatusNotFound)
			return
		}
		if err != nil {
			log.Printf("load book %d's texts: %v", vol, err)
			http.Error(w, "failed to load the book", http.StatusInternalServerError)
			return
		}
		writeJSON(w, docs)
	}
}
