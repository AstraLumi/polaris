package main

// Drafts: a scratch space kept apart from the story. A draft page is
// either a note (a free document in the writing editor's format, pmdoc.go)
// or a board of cards placed by hand: each card a title, a category (a
// named colour, shared by every board of the story) and a Markdown body,
// joined to any number of other cards of its board by lines.
//
// Wiki links go one way: a card's body may link to the story's articles
// (stored by id, schema49LinkColumns), but nothing of the wiki reads the
// drafts: no backlinks, no graph edges, no search.
//
//	GET    /api/drafts                     the pages and the categories
//	POST   /api/drafts                     add a page {kind, title}
//	PUT    /api/drafts/order               {ids} in their new order
//	PUT    /api/drafts/{id}                rename {title}
//	DELETE /api/drafts/{id}                the page and its cards
//	GET    /api/drafts/{id}/note           {doc, version, updated_at}
//	PUT    /api/drafts/{id}/note           {doc, base_version}; 409 with the stored note
//	GET    /api/drafts/{id}/board          {cards, links}
//	POST   /api/drafts/{id}/cards          add a card
//	PUT    /api/draft-cards/{id}           change a card (only the fields sent)
//	DELETE /api/draft-cards/{id}           it and its lines
//	POST   /api/drafts/{id}/links          join two cards {a, b, label}
//	PUT    /api/draft-links/{id}           {label}
//	DELETE /api/draft-links/{id}
//	POST   /api/draft-categories           {name, color}
//	PUT    /api/draft-categories/{id}      {name, color}
//	DELETE /api/draft-categories/{id}      its cards keep no category

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"math"
	"net/http"
	"strings"
)

const (
	maxDraftCardBody = 100_000
	maxDraftCards    = 2000 // per board
	maxDraftLabel    = 80
	// A card's size on the board, in board units (pixels at zoom 1).
	minDraftCardW, maxDraftCardW = 160, 1600
	minDraftCardH, maxDraftCardH = 80, 1600
	maxDraftCoord                = 1e6
)

var schema49Statements = []string{
	`CREATE TABLE draft_pages (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		kind       TEXT NOT NULL CHECK (kind IN ('note', 'board')),
		title      TEXT NOT NULL,
		position   INTEGER NOT NULL DEFAULT 0,
		doc        TEXT NOT NULL DEFAULT '',
		version    INTEGER NOT NULL DEFAULT 0,
		words      INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE TABLE draft_categories (
		id       INTEGER PRIMARY KEY AUTOINCREMENT,
		name     TEXT NOT NULL,
		color    TEXT NOT NULL,
		position INTEGER NOT NULL DEFAULT 0
	)`,
	`CREATE TABLE draft_cards (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		page_id     INTEGER NOT NULL REFERENCES draft_pages(id) ON DELETE CASCADE,
		title       TEXT NOT NULL DEFAULT '',
		category_id INTEGER REFERENCES draft_categories(id) ON DELETE SET NULL,
		body        TEXT NOT NULL DEFAULT '',
		x           REAL NOT NULL DEFAULT 0,
		y           REAL NOT NULL DEFAULT 0,
		w           REAL NOT NULL DEFAULT 320,
		h           REAL NOT NULL DEFAULT 240,
		created_at  TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE INDEX idx_draft_cards_page ON draft_cards(page_id)`,
	// A line between two cards of one board, stored once with a < b.
	`CREATE TABLE draft_links (
		id      INTEGER PRIMARY KEY AUTOINCREMENT,
		page_id INTEGER NOT NULL REFERENCES draft_pages(id) ON DELETE CASCADE,
		a_id    INTEGER NOT NULL REFERENCES draft_cards(id) ON DELETE CASCADE,
		b_id    INTEGER NOT NULL REFERENCES draft_cards(id) ON DELETE CASCADE,
		label   TEXT NOT NULL DEFAULT '',
		CHECK (a_id < b_id),
		UNIQUE (a_id, b_id)
	)`,
	`CREATE INDEX idx_draft_links_page ON draft_links(page_id)`,
	`CREATE INDEX idx_draft_links_b ON draft_links(b_id)`,
}

// schema49LinkColumns: a card's body holds wiki links (by id). The wiki
// never reads them back (wikiTexts covers the wiki's own text only).
var schema49LinkColumns = []linkColumn{
	{"draft_cards", "body", "", ""},
}

type draftPage struct {
	ID        int64  `json:"id"`
	Kind      string `json:"kind"`
	Title     string `json:"title"`
	Cards     int    `json:"cards"`
	Words     int    `json:"words"`
	UpdatedAt string `json:"updated_at"`
}

type draftCategory struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type draftCard struct {
	ID         int64   `json:"id"`
	Title      string  `json:"title"`
	CategoryID *int64  `json:"category_id"`
	Body       string  `json:"body"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	W          float64 `json:"w"`
	H          float64 `json:"h"`
}

type draftLink struct {
	ID    int64  `json:"id"`
	A     int64  `json:"a"`
	B     int64  `json:"b"`
	Label string `json:"label"`
}

func draftRoutes(mux routeMux, db *sql.DB) {
	mux.HandleFunc("GET /api/drafts", listDraftsHandler(db))
	mux.HandleFunc("POST /api/drafts", createDraftHandler(db))
	mux.HandleFunc("PUT /api/drafts/order", reorderDraftsHandler(db))
	mux.HandleFunc("PUT /api/drafts/{id}", renameDraftHandler(db))
	mux.HandleFunc("DELETE /api/drafts/{id}", deleteDraftHandler(db))
	mux.HandleFunc("GET /api/drafts/{id}/note", getDraftNoteHandler(db))
	mux.HandleFunc("PUT /api/drafts/{id}/note", putDraftNoteHandler(db))
	mux.HandleFunc("GET /api/drafts/{id}/board", getDraftBoardHandler(db))
	mux.HandleFunc("POST /api/drafts/{id}/cards", createDraftCardHandler(db))
	mux.HandleFunc("PUT /api/draft-cards/{id}", updateDraftCardHandler(db))
	mux.HandleFunc("DELETE /api/draft-cards/{id}", deleteDraftRowHandler(db, "draft_cards", "card"))
	mux.HandleFunc("POST /api/drafts/{id}/links", createDraftLinkHandler(db))
	mux.HandleFunc("PUT /api/draft-links/{id}", updateDraftLinkHandler(db))
	mux.HandleFunc("DELETE /api/draft-links/{id}", deleteDraftRowHandler(db, "draft_links", "line"))
	mux.HandleFunc("POST /api/draft-categories", saveDraftCategoryHandler(db, false))
	mux.HandleFunc("PUT /api/draft-categories/{id}", saveDraftCategoryHandler(db, true))
	mux.HandleFunc("DELETE /api/draft-categories/{id}", deleteDraftRowHandler(db, "draft_categories", "category"))
}

func draftFail(w http.ResponseWriter, what string, err error) {
	log.Printf("drafts: %s: %v", what, err)
	http.Error(w, "failed to "+what, http.StatusInternalServerError)
}

// draftPageKind gives the kind of page id ("" when there is none).
func draftPageKind(db *sql.DB, id int64) string {
	var kind string
	db.QueryRow(`SELECT kind FROM draft_pages WHERE id = ?`, id).Scan(&kind)
	return kind
}

func loadDraftCategories(db *sql.DB) ([]draftCategory, error) {
	rows, err := db.Query(`SELECT id, name, color FROM draft_categories ORDER BY position, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []draftCategory{}
	for rows.Next() {
		var c draftCategory
		if err := rows.Scan(&c.ID, &c.Name, &c.Color); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func listDraftsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(`SELECT p.id, p.kind, p.title, p.words, p.updated_at,
				(SELECT COUNT(*) FROM draft_cards c WHERE c.page_id = p.id)
			FROM draft_pages p ORDER BY p.position, p.id`)
		if err != nil {
			draftFail(w, "load the drafts", err)
			return
		}
		defer rows.Close()
		pages := []draftPage{}
		for rows.Next() {
			var p draftPage
			if err := rows.Scan(&p.ID, &p.Kind, &p.Title, &p.Words, &p.UpdatedAt, &p.Cards); err != nil {
				draftFail(w, "load the drafts", err)
				return
			}
			pages = append(pages, p)
		}
		if err := rows.Err(); err != nil {
			draftFail(w, "load the drafts", err)
			return
		}
		cats, err := loadDraftCategories(db)
		if err != nil {
			draftFail(w, "load the drafts", err)
			return
		}
		writeJSON(w, map[string]any{"pages": pages, "categories": cats})
	}
}

func createDraftHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Kind  string `json:"kind"`
			Title string `json:"title"`
		}
		if !decodeJSON(w, r, &in) {
			return
		}
		if in.Kind != "note" && in.Kind != "board" {
			http.Error(w, "a draft is a note or a board", http.StatusBadRequest)
			return
		}
		title, ok := cleanTitle(w, in.Title)
		if !ok {
			return
		}
		res, err := db.Exec(`INSERT INTO draft_pages (kind, title, position)
			VALUES (?, ?, (SELECT COALESCE(MAX(position), -1) + 1 FROM draft_pages))`, in.Kind, title)
		if err != nil {
			draftFail(w, "add the draft", err)
			return
		}
		id, _ := res.LastInsertId()
		var p draftPage
		db.QueryRow(`SELECT id, kind, title, words, updated_at FROM draft_pages WHERE id = ?`, id).
			Scan(&p.ID, &p.Kind, &p.Title, &p.Words, &p.UpdatedAt)
		writeJSON(w, p)
	}
}

func reorderDraftsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			IDs []int64 `json:"ids"`
		}
		if !decodeJSON(w, r, &in) {
			return
		}
		tx, err := db.Begin()
		if err != nil {
			draftFail(w, "save the order", err)
			return
		}
		defer tx.Rollback()
		for i, id := range in.IDs {
			if _, err := tx.Exec(`UPDATE draft_pages SET position = ? WHERE id = ?`, i, id); err != nil {
				draftFail(w, "save the order", err)
				return
			}
		}
		if err := tx.Commit(); err != nil {
			draftFail(w, "save the order", err)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	}
}

func renameDraftHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(r, "id")
		var in struct {
			Title string `json:"title"`
		}
		if !ok || draftPageKind(db, id) == "" {
			http.Error(w, "draft not found", http.StatusNotFound)
			return
		}
		if !decodeJSON(w, r, &in) {
			return
		}
		title, ok := cleanTitle(w, in.Title)
		if !ok {
			return
		}
		if _, err := db.Exec(`UPDATE draft_pages SET title = ?, updated_at = datetime('now') WHERE id = ?`, title, id); err != nil {
			draftFail(w, "rename the draft", err)
			return
		}
		writeJSON(w, map[string]any{"id": id, "title": title})
	}
}

func deleteDraftHandler(db *sql.DB) http.HandlerFunc {
	return deleteDraftRowHandler(db, "draft_pages", "draft")
}

// deleteDraftRowHandler deletes one row of a drafts table by its id; the
// foreign keys take what hangs on it.
func deleteDraftRowHandler(db *sql.DB, table, what string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(r, "id")
		if !ok {
			http.Error(w, what+" not found", http.StatusNotFound)
			return
		}
		res, err := db.Exec(`DELETE FROM `+table+` WHERE id = ?`, id)
		if err != nil {
			draftFail(w, "delete the "+what, err)
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			http.Error(w, what+" not found", http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]any{"deleted": id})
	}
}

// ---- Notes -------------------------------------------------------------------

type draftNote struct {
	Doc       json.RawMessage `json:"doc"`
	Version   int64           `json:"version"`
	Words     int             `json:"words"`
	UpdatedAt string          `json:"updated_at"`
}

var errNoDraftNote = errors.New("note not found")

func loadDraftNote(q wikiQuerier, id int64) (draftNote, error) {
	var n draftNote
	var kind, doc string
	err := q.QueryRow(`SELECT kind, doc, version, words, updated_at FROM draft_pages WHERE id = ?`, id).
		Scan(&kind, &doc, &n.Version, &n.Words, &n.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && kind != "note") {
		return n, errNoDraftNote
	}
	if err != nil {
		return n, err
	}
	if doc == "" {
		empty, _ := json.Marshal(emptyPMDoc())
		n.Doc = empty
	} else {
		n.Doc = json.RawMessage(doc)
	}
	return n, nil
}

func getDraftNoteHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		n, err := loadDraftNote(db, id)
		if errors.Is(err, errNoDraftNote) {
			http.Error(w, "note not found", http.StatusNotFound)
			return
		}
		if err != nil {
			draftFail(w, "load the note", err)
			return
		}
		writeJSON(w, n)
	}
}

// putDraftNoteHandler saves a note written on the stored version
// (base_version); otherwise it answers 409 with the stored note, as the
// chapters' texts do.
func putDraftNoteHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		var in struct {
			Doc         json.RawMessage `json:"doc"`
			BaseVersion *int64          `json:"base_version"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, chapterTextBodyAt)).Decode(&in); err != nil || in.BaseVersion == nil {
			var tooBig *http.MaxBytesError
			if errors.As(err, &tooBig) {
				http.Error(w, "the note is too long to save in one piece", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		doc, words, msg := cleanChapterDoc(in.Doc)
		if msg != "" {
			http.Error(w, strings.ReplaceAll(msg, "chapter", "note"), http.StatusBadRequest)
			return
		}
		res, err := db.Exec(`UPDATE draft_pages SET doc = ?, words = ?, version = version + 1, updated_at = datetime('now')
			WHERE id = ? AND kind = 'note' AND version = ?`, string(doc), words, id, *in.BaseVersion)
		if err != nil {
			draftFail(w, "save the note", err)
			return
		}
		n, err := loadDraftNote(db, id)
		if errors.Is(err, errNoDraftNote) {
			http.Error(w, "note not found", http.StatusNotFound)
			return
		}
		if err != nil {
			draftFail(w, "save the note", err)
			return
		}
		if changed, _ := res.RowsAffected(); changed == 0 {
			writeJSONStatus(w, http.StatusConflict, n)
			return
		}
		writeJSON(w, map[string]any{"version": n.Version, "words": n.Words, "updated_at": n.UpdatedAt})
	}
}

// ---- Boards ------------------------------------------------------------------

func getDraftBoardHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		if draftPageKind(db, id) != "board" {
			http.Error(w, "board not found", http.StatusNotFound)
			return
		}
		cards := []draftCard{}
		rows, err := db.Query(`SELECT id, title, category_id, body, x, y, w, h FROM draft_cards WHERE page_id = ? ORDER BY id`, id)
		if err != nil {
			draftFail(w, "load the board", err)
			return
		}
		for rows.Next() {
			var c draftCard
			var cat sql.NullInt64
			if err := rows.Scan(&c.ID, &c.Title, &cat, &c.Body, &c.X, &c.Y, &c.W, &c.H); err != nil {
				rows.Close()
				draftFail(w, "load the board", err)
				return
			}
			if cat.Valid {
				c.CategoryID = &cat.Int64
			}
			cards = append(cards, c)
		}
		rows.Close()
		links := []draftLink{}
		rows, err = db.Query(`SELECT id, a_id, b_id, label FROM draft_links WHERE page_id = ? ORDER BY id`, id)
		if err != nil {
			draftFail(w, "load the board", err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var l draftLink
			if err := rows.Scan(&l.ID, &l.A, &l.B, &l.Label); err != nil {
				draftFail(w, "load the board", err)
				return
			}
			links = append(links, l)
		}
		if err := rows.Err(); err != nil {
			draftFail(w, "load the board", err)
			return
		}
		writeJSON(w, map[string]any{"cards": cards, "links": links})
	}
}

// draftCardIn is a card's fields as sent; nil leaves a field as it is.
type draftCardIn struct {
	Title      *string  `json:"title"`
	CategoryID *int64   `json:"category_id"`
	NoCategory bool     `json:"no_category"`
	Body       *string  `json:"body"`
	X          *float64 `json:"x"`
	Y          *float64 `json:"y"`
	W          *float64 `json:"w"`
	H          *float64 `json:"h"`
}

// apply checks the fields sent and writes them into c; "" when they're fine.
func (in draftCardIn) apply(db *sql.DB, c *draftCard) string {
	if in.Title != nil {
		t := strings.TrimSpace(*in.Title)
		if len(t) > 200 {
			return "that title is too long"
		}
		c.Title = t
	}
	if in.Body != nil {
		if len(*in.Body) > maxDraftCardBody {
			return "that text is too long"
		}
		c.Body = *in.Body
	}
	if in.NoCategory {
		c.CategoryID = nil
	} else if in.CategoryID != nil {
		var n int
		db.QueryRow(`SELECT COUNT(*) FROM draft_categories WHERE id = ?`, *in.CategoryID).Scan(&n)
		if n == 0 {
			return "that category doesn't exist"
		}
		c.CategoryID = in.CategoryID
	}
	num := func(p *float64, dst *float64, lo, hi float64) bool {
		if p == nil {
			return true
		}
		if math.IsNaN(*p) || math.IsInf(*p, 0) {
			return false
		}
		*dst = math.Max(lo, math.Min(hi, math.Round(*p)))
		return true
	}
	if !num(in.X, &c.X, -maxDraftCoord, maxDraftCoord) || !num(in.Y, &c.Y, -maxDraftCoord, maxDraftCoord) ||
		!num(in.W, &c.W, minDraftCardW, maxDraftCardW) || !num(in.H, &c.H, minDraftCardH, maxDraftCardH) {
		return "a card's place and size must be numbers"
	}
	return ""
}

func writeDraftCard(w http.ResponseWriter, db *sql.DB, id int64) {
	var c draftCard
	var cat sql.NullInt64
	if err := db.QueryRow(`SELECT id, title, category_id, body, x, y, w, h FROM draft_cards WHERE id = ?`, id).
		Scan(&c.ID, &c.Title, &cat, &c.Body, &c.X, &c.Y, &c.W, &c.H); err != nil {
		draftFail(w, "load the card", err)
		return
	}
	if cat.Valid {
		c.CategoryID = &cat.Int64
	}
	writeJSON(w, c)
}

func createDraftCardHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, _ := pathID(r, "id")
		if draftPageKind(db, page) != "board" {
			http.Error(w, "board not found", http.StatusNotFound)
			return
		}
		var in draftCardIn
		if !decodeJSON(w, r, &in) {
			return
		}
		var n int
		db.QueryRow(`SELECT COUNT(*) FROM draft_cards WHERE page_id = ?`, page).Scan(&n)
		if n >= maxDraftCards {
			http.Error(w, "this board is full", http.StatusBadRequest)
			return
		}
		c := draftCard{W: 320, H: 240}
		if msg := in.apply(db, &c); msg != "" {
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		if err := storeLinksByID(db, &c.Body); err != nil {
			draftFail(w, "add the card", err)
			return
		}
		res, err := db.Exec(`INSERT INTO draft_cards (page_id, title, category_id, body, x, y, w, h) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			page, c.Title, c.CategoryID, c.Body, c.X, c.Y, c.W, c.H)
		if err != nil {
			draftFail(w, "add the card", err)
			return
		}
		id, _ := res.LastInsertId()
		db.Exec(`UPDATE draft_pages SET updated_at = datetime('now') WHERE id = ?`, page)
		writeDraftCard(w, db, id)
	}
}

func updateDraftCardHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		var c draftCard
		var cat sql.NullInt64
		var page int64
		if err := db.QueryRow(`SELECT page_id, title, category_id, body, x, y, w, h FROM draft_cards WHERE id = ?`, id).
			Scan(&page, &c.Title, &cat, &c.Body, &c.X, &c.Y, &c.W, &c.H); err != nil {
			http.Error(w, "card not found", http.StatusNotFound)
			return
		}
		if cat.Valid {
			c.CategoryID = &cat.Int64
		}
		var in draftCardIn
		if !decodeJSON(w, r, &in) {
			return
		}
		if msg := in.apply(db, &c); msg != "" {
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		if in.Body != nil {
			if err := storeLinksByID(db, &c.Body); err != nil {
				draftFail(w, "save the card", err)
				return
			}
		}
		if _, err := db.Exec(`UPDATE draft_cards SET title = ?, category_id = ?, body = ?, x = ?, y = ?, w = ?, h = ?,
				updated_at = datetime('now') WHERE id = ?`, c.Title, c.CategoryID, c.Body, c.X, c.Y, c.W, c.H, id); err != nil {
			draftFail(w, "save the card", err)
			return
		}
		db.Exec(`UPDATE draft_pages SET updated_at = datetime('now') WHERE id = ?`, page)
		writeDraftCard(w, db, id)
	}
}

func cleanDraftLabel(s string) (string, bool) {
	s = strings.TrimSpace(s)
	return s, len(s) <= maxDraftLabel
}

func createDraftLinkHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, _ := pathID(r, "id")
		if draftPageKind(db, page) != "board" {
			http.Error(w, "board not found", http.StatusNotFound)
			return
		}
		var in struct {
			A     int64  `json:"a"`
			B     int64  `json:"b"`
			Label string `json:"label"`
		}
		if !decodeJSON(w, r, &in) {
			return
		}
		a, b := min(in.A, in.B), max(in.A, in.B)
		if a == b {
			http.Error(w, "a card can't be joined to itself", http.StatusBadRequest)
			return
		}
		label, ok := cleanDraftLabel(in.Label)
		if !ok {
			http.Error(w, "that label is too long", http.StatusBadRequest)
			return
		}
		var n int
		db.QueryRow(`SELECT COUNT(*) FROM draft_cards WHERE page_id = ? AND id IN (?, ?)`, page, a, b).Scan(&n)
		if n != 2 {
			http.Error(w, "both cards must be on this board", http.StatusBadRequest)
			return
		}
		var l draftLink
		err := db.QueryRow(`SELECT id, a_id, b_id, label FROM draft_links WHERE a_id = ? AND b_id = ?`, a, b).
			Scan(&l.ID, &l.A, &l.B, &l.Label)
		if err == nil {
			writeJSON(w, l) // already joined
			return
		}
		res, err := db.Exec(`INSERT INTO draft_links (page_id, a_id, b_id, label) VALUES (?, ?, ?, ?)`, page, a, b, label)
		if err != nil {
			draftFail(w, "join the cards", err)
			return
		}
		l.ID, _ = res.LastInsertId()
		l.A, l.B, l.Label = a, b, label
		writeJSON(w, l)
	}
}

func updateDraftLinkHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := pathID(r, "id")
		var in struct {
			Label string `json:"label"`
		}
		if !decodeJSON(w, r, &in) {
			return
		}
		label, ok := cleanDraftLabel(in.Label)
		if !ok {
			http.Error(w, "that label is too long", http.StatusBadRequest)
			return
		}
		res, err := db.Exec(`UPDATE draft_links SET label = ? WHERE id = ?`, label, id)
		if err != nil {
			draftFail(w, "save the line", err)
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			http.Error(w, "line not found", http.StatusNotFound)
			return
		}
		var l draftLink
		db.QueryRow(`SELECT id, a_id, b_id, label FROM draft_links WHERE id = ?`, id).Scan(&l.ID, &l.A, &l.B, &l.Label)
		writeJSON(w, l)
	}
}

func saveDraftCategoryHandler(db *sql.DB, update bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var id int64
		if update {
			var ok bool
			if id, ok = pathID(r, "id"); !ok {
				http.Error(w, "category not found", http.StatusNotFound)
				return
			}
		}
		var in struct {
			Name  string `json:"name"`
			Color string `json:"color"`
		}
		if !decodeJSON(w, r, &in) {
			return
		}
		name := strings.TrimSpace(in.Name)
		if name == "" || len(name) > 60 {
			http.Error(w, "a category needs a short name", http.StatusBadRequest)
			return
		}
		color, ok := normalizeColor(in.Color)
		if !ok || color == "" {
			http.Error(w, "a category needs a colour like #aa3344", http.StatusBadRequest)
			return
		}
		if update {
			res, err := db.Exec(`UPDATE draft_categories SET name = ?, color = ? WHERE id = ?`, name, color, id)
			if err != nil {
				draftFail(w, "save the category", err)
				return
			}
			if n, _ := res.RowsAffected(); n == 0 {
				http.Error(w, "category not found", http.StatusNotFound)
				return
			}
		} else {
			res, err := db.Exec(`INSERT INTO draft_categories (name, color, position)
				VALUES (?, ?, (SELECT COALESCE(MAX(position), -1) + 1 FROM draft_categories))`, name, color)
			if err != nil {
				draftFail(w, "add the category", err)
				return
			}
			id, _ = res.LastInsertId()
		}
		writeJSON(w, draftCategory{ID: id, Name: name, Color: color})
	}
}
