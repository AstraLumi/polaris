package main

// Cultures: the ways of life of the story's peoples. A culture can belong
// to any number of kingdoms (coloured major locations), and a kingdom can
// hold several cultures. Each culture has elements, sorted by kind
// (clothing, food, festivals…): a name, a description, pictures, tags, and
// for a design the references from the shared library it is based on
// (references.go, linked by id, since the library lives outside the story).
// Every culture is a wiki article (wikiTypes "culture").
//
//	GET    /api/cultures                    every culture, with its kingdoms
//	POST   /api/cultures                    add one (multipart)
//	GET    /api/cultures/{id}               one, with its elements
//	PUT    /api/cultures/{id}               change one (multipart)
//	DELETE /api/cultures/{id}               it, its elements and pictures
//	POST   /api/cultures/{id}/elements      add an element (multipart)
//	PUT    /api/culture-elements/{id}       change one (multipart)
//	DELETE /api/culture-elements/{id}
//	GET    /api/culture-elements            every element of every culture
//	                                        (browsing by tag)

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// The kinds of element, in the order the culture page shows them; the
// same list is in frontend/src/cultures.js.
var cultureElementKinds = []string{
	"clothing", "food", "festival", "custom", "belief", "art", "architecture", "language", "other",
}

const (
	maxCulturePictures = 40
	maxCultureTags     = 40
	maxCultureRefs     = 100
)

var schema48Statements = []string{
	`CREATE TABLE cultures (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		name         TEXT NOT NULL,
		description  TEXT NOT NULL DEFAULT '',
		picture_path TEXT,
		created_at   TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at   TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE TABLE culture_kingdoms (
		culture_id  INTEGER NOT NULL REFERENCES cultures(id) ON DELETE CASCADE,
		location_id INTEGER NOT NULL REFERENCES locations(id) ON DELETE CASCADE,
		PRIMARY KEY (culture_id, location_id)
	)`,
	`CREATE INDEX idx_culture_kingdoms_location ON culture_kingdoms(location_id)`,
	`CREATE TABLE culture_elements (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		culture_id  INTEGER NOT NULL REFERENCES cultures(id) ON DELETE CASCADE,
		kind        TEXT NOT NULL DEFAULT 'clothing' CHECK (kind IN ('clothing', 'food', 'festival', 'custom',
			'belief', 'art', 'architecture', 'language', 'other')),
		name        TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		created_at  TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE INDEX idx_culture_elements_culture ON culture_elements(culture_id)`,
	`CREATE TABLE culture_element_pictures (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		element_id INTEGER NOT NULL REFERENCES culture_elements(id) ON DELETE CASCADE,
		position   INTEGER NOT NULL DEFAULT 0,
		file_path  TEXT NOT NULL
	)`,
	`CREATE INDEX idx_culture_element_pictures_element ON culture_element_pictures(element_id)`,
	`CREATE TABLE culture_element_tags (
		element_id INTEGER NOT NULL REFERENCES culture_elements(id) ON DELETE CASCADE,
		tag        TEXT NOT NULL COLLATE NOCASE,
		PRIMARY KEY (element_id, tag)
	)`,
	// reference_id points into the shared library (the registry), so it
	// has no foreign key: a reference deleted there just stops showing.
	`CREATE TABLE culture_element_refs (
		element_id   INTEGER NOT NULL REFERENCES culture_elements(id) ON DELETE CASCADE,
		reference_id INTEGER NOT NULL,
		PRIMARY KEY (element_id, reference_id)
	)`,
	wikiTriggerSQL("culture"),
}

// schema48LinkColumns: the cultures' texts hold wiki links (by id).
var schema48LinkColumns = []linkColumn{
	{"cultures", "description", "", ""},
	{"culture_elements", "description", "", ""},
}

type cultureKingdom struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type cultureElementPicture struct {
	ID  int64  `json:"id"`
	URL string `json:"url"`
}

type cultureElementOut struct {
	ID           int64                   `json:"id"`
	CultureID    int64                   `json:"culture_id"`
	CultureName  string                  `json:"culture_name"`
	Kind         string                  `json:"kind"`
	Name         string                  `json:"name"`
	Description  string                  `json:"description"`
	Pictures     []cultureElementPicture `json:"pictures"`
	Tags         []string                `json:"tags"`
	ReferenceIDs []int64                 `json:"reference_ids"`
}

type cultureOut struct {
	ID           int64               `json:"id"`
	Name         string              `json:"name"`
	Description  string              `json:"description"`
	PicturePath  string              `json:"picture_path"`
	Kingdoms     []cultureKingdom    `json:"kingdoms"`
	ElementCount int                 `json:"element_count"`
	Elements     []cultureElementOut `json:"elements,omitempty"`
	UpdatedAt    string              `json:"updated_at"`
}

// loadCultures reads every culture (or one, id > 0) with its kingdoms, in
// two queries.
func loadCultures(db *sql.DB, id int64) ([]cultureOut, error) {
	where, args := "", []any{}
	if id > 0 {
		where, args = " WHERE c.id = ?", append(args, id)
	}
	rows, err := db.Query(`SELECT c.id, c.name, c.description, COALESCE(c.picture_path, ''), c.updated_at,
		(SELECT COUNT(*) FROM culture_elements e WHERE e.culture_id = c.id)
		FROM cultures c`+where+` ORDER BY c.name COLLATE NOCASE`, args...)
	if err != nil {
		return nil, err
	}
	out := []cultureOut{}
	at := map[int64]int{}
	for rows.Next() {
		var c cultureOut
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.PicturePath, &c.UpdatedAt, &c.ElementCount); err != nil {
			rows.Close()
			return nil, err
		}
		c.PicturePath = uploadURL(c.PicturePath)
		c.Kingdoms = []cultureKingdom{}
		at[c.ID] = len(out)
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	kq := `SELECT k.culture_id, l.id, l.name, COALESCE(l.color, '') FROM culture_kingdoms k
		JOIN locations l ON l.id = k.location_id`
	if id > 0 {
		kq += ` WHERE k.culture_id = ?`
	}
	rows, err = db.Query(kq+` ORDER BY l.name COLLATE NOCASE`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int64
		var k cultureKingdom
		if rows.Scan(&cid, &k.ID, &k.Name, &k.Color) == nil {
			if i, ok := at[cid]; ok {
				out[i].Kingdoms = append(out[i].Kingdoms, k)
			}
		}
	}
	return out, rows.Err()
}

// loadCultureElements reads the elements of one culture (cultureID > 0),
// one element (elementID > 0) or all of them, with their pictures, tags
// and references, in four queries.
func loadCultureElements(db *sql.DB, cultureID, elementID int64) ([]cultureElementOut, error) {
	where, args := "", []any{}
	switch {
	case elementID > 0:
		where, args = " WHERE e.id = ?", []any{elementID}
	case cultureID > 0:
		where, args = " WHERE e.culture_id = ?", []any{cultureID}
	}
	rows, err := db.Query(`SELECT e.id, e.culture_id, c.name, e.kind, e.name, e.description
		FROM culture_elements e JOIN cultures c ON c.id = e.culture_id`+where+`
		ORDER BY c.name COLLATE NOCASE, e.name COLLATE NOCASE, e.id`, args...)
	if err != nil {
		return nil, err
	}
	out := []cultureElementOut{}
	at := map[int64]int{}
	for rows.Next() {
		var e cultureElementOut
		if err := rows.Scan(&e.ID, &e.CultureID, &e.CultureName, &e.Kind, &e.Name, &e.Description); err != nil {
			rows.Close()
			return nil, err
		}
		e.Pictures, e.Tags, e.ReferenceIDs = []cultureElementPicture{}, []string{}, []int64{}
		at[e.ID] = len(out)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	if len(out) == 0 {
		return out, nil
	}
	// The children, read for the same elements.
	sub := `SELECT e.id FROM culture_elements e` + where
	each := func(q string, scan func(rows *sql.Rows) error) error {
		rows, err := db.Query(q, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			if err := scan(rows); err != nil {
				return err
			}
		}
		return rows.Err()
	}
	err = each(`SELECT element_id, id, file_path FROM culture_element_pictures WHERE element_id IN (`+sub+`)
		ORDER BY element_id, position, id`, func(rows *sql.Rows) error {
		var eid int64
		var p cultureElementPicture
		if err := rows.Scan(&eid, &p.ID, &p.URL); err != nil {
			return err
		}
		if i, ok := at[eid]; ok {
			p.URL = uploadURL(p.URL)
			out[i].Pictures = append(out[i].Pictures, p)
		}
		return nil
	})
	if err == nil {
		err = each(`SELECT element_id, tag FROM culture_element_tags WHERE element_id IN (`+sub+`)
			ORDER BY element_id, tag COLLATE NOCASE`, func(rows *sql.Rows) error {
			var eid int64
			var t string
			if err := rows.Scan(&eid, &t); err != nil {
				return err
			}
			if i, ok := at[eid]; ok {
				out[i].Tags = append(out[i].Tags, t)
			}
			return nil
		})
	}
	if err == nil {
		err = each(`SELECT element_id, reference_id FROM culture_element_refs WHERE element_id IN (`+sub+`)
			ORDER BY element_id, reference_id`, func(rows *sql.Rows) error {
			var eid, rid int64
			if err := rows.Scan(&eid, &rid); err != nil {
				return err
			}
			if i, ok := at[eid]; ok {
				out[i].ReferenceIDs = append(out[i].ReferenceIDs, rid)
			}
			return nil
		})
	}
	return out, err
}

func culturePathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

func listCulturesHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := loadCultures(db, 0)
		if err != nil {
			log.Printf("list cultures: %v", err)
			http.Error(w, "failed to load the cultures", http.StatusInternalServerError)
			return
		}
		writeJSON(w, list)
	}
}

func loadCulture(db *sql.DB, id int64) (cultureOut, bool, error) {
	list, err := loadCultures(db, id)
	if err != nil || len(list) == 0 {
		return cultureOut{}, false, err
	}
	c := list[0]
	if c.Elements, err = loadCultureElements(db, id, 0); err != nil {
		return c, true, err
	}
	return c, true, nil
}

func getCultureHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := culturePathID(r)
		if !ok {
			http.Error(w, "culture not found", http.StatusNotFound)
			return
		}
		c, found, err := loadCulture(db, id)
		if err != nil {
			log.Printf("load culture: %v", err)
			http.Error(w, "failed to load the culture", http.StatusInternalServerError)
			return
		}
		if !found {
			http.Error(w, "culture not found", http.StatusNotFound)
			return
		}
		writeJSON(w, c)
	}
}

func listCultureElementsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := loadCultureElements(db, 0, 0)
		if err != nil {
			log.Printf("list culture elements: %v", err)
			http.Error(w, "failed to load the cultures", http.StatusInternalServerError)
			return
		}
		writeJSON(w, list)
	}
}

// saveCultureHandler adds (no id) or changes a culture: name, description,
// kingdom_ids (JSON list of locations; only coloured major ones count),
// picture, remove_picture.
func saveCultureHandler(db *sql.DB, uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var id int64
		if r.PathValue("id") != "" {
			var ok bool
			if id, ok = culturePathID(r); !ok || db.QueryRow(`SELECT id FROM cultures WHERE id = ?`, id).Scan(&id) != nil {
				http.Error(w, "culture not found", http.StatusNotFound)
				return
			}
		}
		if err := parsePictureForm(w, r); err != nil {
			http.Error(w, "invalid form data", http.StatusBadRequest)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		desc := strings.TrimSpace(r.FormValue("description"))
		if name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		if len(name) > 120 || len(desc) > 20000 {
			http.Error(w, "that text is too long", http.StatusBadRequest)
			return
		}
		var taken int
		db.QueryRow(`SELECT COUNT(*) FROM cultures WHERE name = ? COLLATE NOCASE AND id != ?`, name, id).Scan(&taken)
		if taken > 0 {
			http.Error(w, "a culture with that name already exists", http.StatusBadRequest)
			return
		}
		kingdoms, ok := idList(r.FormValue("kingdom_ids"))
		if !ok || len(kingdoms) > 500 {
			http.Error(w, "invalid kingdom list", http.StatusBadRequest)
			return
		}
		if !storeLinksOrFail(w, db, &desc) {
			return
		}
		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to save the culture", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()
		if id == 0 {
			res, err := tx.Exec(`INSERT INTO cultures (name, description) VALUES (?, ?)`, name, desc)
			if err != nil {
				log.Printf("create culture: %v", err)
				http.Error(w, "failed to save the culture", http.StatusInternalServerError)
				return
			}
			id, _ = res.LastInsertId()
		} else if _, err := tx.Exec(`UPDATE cultures SET name = ?, description = ?, updated_at = datetime('now') WHERE id = ?`,
			name, desc, id); err != nil {
			log.Printf("update culture: %v", err)
			http.Error(w, "failed to save the culture", http.StatusInternalServerError)
			return
		}
		if r.Form["kingdom_ids"] != nil {
			if _, err := tx.Exec(`DELETE FROM culture_kingdoms WHERE culture_id = ?`, id); err != nil {
				http.Error(w, "failed to save the culture", http.StatusInternalServerError)
				return
			}
			for _, k := range kingdoms {
				// Unknown places and places that aren't kingdoms quietly drop out.
				if _, err := tx.Exec(`INSERT OR IGNORE INTO culture_kingdoms (culture_id, location_id)
					SELECT ?, id FROM locations WHERE id = ? AND kind = 'major' AND COALESCE(color, '') <> ''`, id, k); err != nil {
					http.Error(w, "failed to save the culture", http.StatusInternalServerError)
					return
				}
			}
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to save the culture", http.StatusInternalServerError)
			return
		}
		saveCulturePicture(db, r, uploadsDir, id)
		c, _, err := loadCulture(db, id)
		if err != nil {
			http.Error(w, "failed to save the culture", http.StatusInternalServerError)
			return
		}
		writeJSON(w, c)
	}
}

func saveCulturePicture(db *sql.DB, r *http.Request, uploadsDir string, id int64) {
	var old sql.NullString
	db.QueryRow(`SELECT picture_path FROM cultures WHERE id = ?`, id).Scan(&old)
	if r.FormValue("remove_picture") == "1" && old.Valid {
		db.Exec(`UPDATE cultures SET picture_path = NULL WHERE id = ?`, id)
		removePicture(uploadsDir, old.String)
		old = sql.NullString{}
	}
	file, header, err := r.FormFile("picture")
	if err != nil {
		return
	}
	defer file.Close()
	rel, err := saveUpload(uploadsDir, "cultures", fmt.Sprintf("%d-%d", id, time.Now().Unix()), header.Filename, file)
	if err != nil {
		log.Printf("save culture picture: %v", err)
		return
	}
	if _, err := db.Exec(`UPDATE cultures SET picture_path = ? WHERE id = ?`, rel, id); err != nil {
		log.Printf("update culture picture: %v", err)
		return
	}
	if old.Valid && old.String != "" && old.String != rel {
		removePicture(uploadsDir, old.String)
	}
}

// elementFiles lists the pictures of the elements a query picks.
func elementFiles(db *sql.DB, where string, arg int64) []string {
	var out []string
	rows, err := db.Query(`SELECT p.file_path FROM culture_element_pictures p
		JOIN culture_elements e ON e.id = p.element_id WHERE `+where, arg)
	if err != nil {
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var f string
		if rows.Scan(&f) == nil {
			out = append(out, f)
		}
	}
	return out
}

func deleteCultureHandler(db *sql.DB, uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := culturePathID(r)
		var pic sql.NullString
		if !ok || db.QueryRow(`SELECT picture_path FROM cultures WHERE id = ?`, id).Scan(&pic) != nil {
			http.Error(w, "culture not found", http.StatusNotFound)
			return
		}
		files := elementFiles(db, `e.culture_id = ?`, id)
		if _, err := db.Exec(`DELETE FROM cultures WHERE id = ?`, id); err != nil {
			log.Printf("delete culture: %v", err)
			http.Error(w, "failed to delete the culture", http.StatusInternalServerError)
			return
		}
		if pic.Valid {
			removePicture(uploadsDir, pic.String)
		}
		for _, f := range files {
			removePicture(uploadsDir, f)
		}
		writeJSON(w, map[string]any{"deleted": id})
	}
}

// saveCultureElementHandler adds an element to a culture (POST under the
// culture) or changes one (PUT /api/culture-elements/{id}):
//
//	kind, name, description     the fields
//	tags                        JSON list
//	reference_ids               JSON list of the library's reference ids
//	pictures                    files to add, at the end
//	remove_pictures, order      JSON lists of picture ids
func saveCultureElementHandler(db *sql.DB, uploadsDir string, isNew bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := culturePathID(r)
		var cultureID, elementID int64
		if isNew {
			cultureID = id
			if !ok || db.QueryRow(`SELECT id FROM cultures WHERE id = ?`, id).Scan(&cultureID) != nil {
				http.Error(w, "culture not found", http.StatusNotFound)
				return
			}
		} else {
			elementID = id
			if !ok || db.QueryRow(`SELECT culture_id FROM culture_elements WHERE id = ?`, id).Scan(&cultureID) != nil {
				http.Error(w, "element not found", http.StatusNotFound)
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

		kind := strings.TrimSpace(r.FormValue("kind"))
		if kind == "" {
			kind = "clothing"
		}
		known := false
		for _, k := range cultureElementKinds {
			known = known || k == kind
		}
		name := strings.TrimSpace(r.FormValue("name"))
		desc := strings.TrimSpace(r.FormValue("description"))
		switch {
		case !known:
			http.Error(w, "unknown kind", http.StatusBadRequest)
			return
		case name == "":
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		case len(name) > 200 || len(desc) > 20000:
			http.Error(w, "that text is too long", http.StatusBadRequest)
			return
		}
		tags, msg := tagsFromJSON(r.FormValue("tags"))
		if msg != "" {
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		refs, ok1 := idList(r.FormValue("reference_ids"))
		remove, ok2 := idList(r.FormValue("remove_pictures"))
		order, ok3 := idList(r.FormValue("order"))
		if !ok1 || !ok2 || !ok3 || len(refs) > maxCultureRefs {
			http.Error(w, "invalid list", http.StatusBadRequest)
			return
		}
		files := r.MultipartForm.File["pictures"]
		var have int
		if elementID != 0 {
			db.QueryRow(`SELECT COUNT(*) FROM culture_element_pictures WHERE element_id = ?`, elementID).Scan(&have)
		}
		if have-len(remove)+len(files) > maxCulturePictures {
			http.Error(w, fmt.Sprintf("an element holds at most %d pictures", maxCulturePictures), http.StatusBadRequest)
			return
		}
		// Check the pictures before anything is written.
		type upload struct {
			data []byte
			ext  string
		}
		var uploads []upload
		for _, fh := range files {
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
		if !storeLinksOrFail(w, db, &desc) {
			return
		}
		var written []string
		for i, u := range uploads {
			rel, err := saveUpload(uploadsDir, "cultures", fmt.Sprintf("e%d-%d-%d", cultureID, time.Now().UnixNano(), i),
				"x"+u.ext, bytes.NewReader(u.data))
			if err != nil {
				log.Printf("save element picture: %v", err)
				for _, f := range written {
					removePicture(uploadsDir, f)
				}
				http.Error(w, "failed to save the element", http.StatusInternalServerError)
				return
			}
			written = append(written, rel)
		}
		gone, err := writeCultureElement(db, cultureID, &elementID, kind, name, desc, tags, refs, written, remove, order)
		if err != nil {
			log.Printf("save culture element: %v", err)
			for _, f := range written {
				removePicture(uploadsDir, f)
			}
			http.Error(w, "failed to save the element", http.StatusInternalServerError)
			return
		}
		for _, f := range gone {
			removePicture(uploadsDir, f)
		}
		list, err := loadCultureElements(db, 0, elementID)
		if err != nil || len(list) == 0 {
			http.Error(w, "failed to save the element", http.StatusInternalServerError)
			return
		}
		writeJSON(w, list[0])
	}
}

func writeCultureElement(db *sql.DB, cultureID int64, elementID *int64, kind, name, desc string,
	tags []string, refs []int64, files []string, remove, order []int64) ([]string, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	id := *elementID
	if id == 0 {
		res, err := tx.Exec(`INSERT INTO culture_elements (culture_id, kind, name, description) VALUES (?, ?, ?, ?)`,
			cultureID, kind, name, desc)
		if err != nil {
			return nil, err
		}
		id, _ = res.LastInsertId()
	} else if _, err := tx.Exec(`UPDATE culture_elements SET kind = ?, name = ?, description = ?,
		updated_at = datetime('now') WHERE id = ?`, kind, name, desc, id); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`DELETE FROM culture_element_tags WHERE element_id = ?`, id); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`DELETE FROM culture_element_refs WHERE element_id = ?`, id); err != nil {
		return nil, err
	}
	for _, t := range tags {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO culture_element_tags (element_id, tag) VALUES (?, ?)`, id, t); err != nil {
			return nil, err
		}
	}
	for _, ref := range refs {
		if ref <= 0 {
			continue
		}
		if _, err := tx.Exec(`INSERT OR IGNORE INTO culture_element_refs (element_id, reference_id) VALUES (?, ?)`, id, ref); err != nil {
			return nil, err
		}
	}
	var gone []string
	for _, pid := range remove {
		var f string
		if tx.QueryRow(`SELECT file_path FROM culture_element_pictures WHERE id = ? AND element_id = ?`, pid, id).Scan(&f) != nil {
			continue
		}
		if _, err := tx.Exec(`DELETE FROM culture_element_pictures WHERE id = ?`, pid); err != nil {
			return nil, err
		}
		gone = append(gone, f)
	}
	for i, pid := range order {
		if _, err := tx.Exec(`UPDATE culture_element_pictures SET position = ? WHERE id = ? AND element_id = ?`, i, pid, id); err != nil {
			return nil, err
		}
	}
	var last int
	tx.QueryRow(`SELECT COALESCE(MAX(position), -1) FROM culture_element_pictures WHERE element_id = ?`, id).Scan(&last)
	for _, f := range files {
		last++
		if _, err := tx.Exec(`INSERT INTO culture_element_pictures (element_id, position, file_path) VALUES (?, ?, ?)`, id, last, f); err != nil {
			return nil, err
		}
	}
	if _, err := tx.Exec(`UPDATE cultures SET updated_at = datetime('now') WHERE id = ?`, cultureID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	*elementID = id
	return gone, nil
}

func deleteCultureElementHandler(db *sql.DB, uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := culturePathID(r)
		if !ok {
			http.Error(w, "element not found", http.StatusNotFound)
			return
		}
		files := elementFiles(db, `e.id = ?`, id)
		res, err := db.Exec(`DELETE FROM culture_elements WHERE id = ?`, id)
		if err != nil {
			log.Printf("delete culture element: %v", err)
			http.Error(w, "failed to delete the element", http.StatusInternalServerError)
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			http.Error(w, "element not found", http.StatusNotFound)
			return
		}
		for _, f := range files {
			removePicture(uploadsDir, f)
		}
		writeJSON(w, map[string]any{"deleted": id})
	}
}

// cultureRoutes adds the cultures' API to a story's routes.
func cultureRoutes(mux *http.ServeMux, db *sql.DB, uploadsDir string) {
	mux.HandleFunc("GET /api/cultures", listCulturesHandler(db))
	mux.HandleFunc("POST /api/cultures", saveCultureHandler(db, uploadsDir))
	mux.HandleFunc("GET /api/cultures/{id}", getCultureHandler(db))
	mux.HandleFunc("PUT /api/cultures/{id}", saveCultureHandler(db, uploadsDir))
	mux.HandleFunc("DELETE /api/cultures/{id}", deleteCultureHandler(db, uploadsDir))
	mux.HandleFunc("POST /api/cultures/{id}/elements", saveCultureElementHandler(db, uploadsDir, true))
	mux.HandleFunc("GET /api/culture-elements", listCultureElementsHandler(db))
	mux.HandleFunc("PUT /api/culture-elements/{id}", saveCultureElementHandler(db, uploadsDir, false))
	mux.HandleFunc("DELETE /api/culture-elements/{id}", deleteCultureElementHandler(db, uploadsDir))
}
