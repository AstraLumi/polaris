package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Factions are the story's organisations: guilds, orders, churches, noble
// houses. A faction can have a headquarters on the map, sit inside a parent
// faction, and has members (characters) with a role and optional in-story
// since/until dates. Its wiki article is its page.

const factionsTableSQL = `CREATE TABLE factions (
	id             INTEGER PRIMARY KEY AUTOINCREMENT,
	name           TEXT NOT NULL,
	description    TEXT NOT NULL DEFAULT '',
	picture_path   TEXT,
	color          TEXT,
	hq_location_id INTEGER REFERENCES locations(id) ON DELETE SET NULL,
	parent_id      INTEGER REFERENCES factions(id) ON DELETE SET NULL,
	founding_date  TEXT NOT NULL DEFAULT '',
	created_at     TEXT NOT NULL DEFAULT (datetime('now')),
	updated_at     TEXT NOT NULL DEFAULT (datetime('now'))
)`

const factionMembersTableSQL = `CREATE TABLE faction_members (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	faction_id   INTEGER NOT NULL REFERENCES factions(id) ON DELETE CASCADE,
	character_id INTEGER NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
	role         TEXT NOT NULL DEFAULT '',
	since        TEXT NOT NULL DEFAULT '',
	until        TEXT NOT NULL DEFAULT ''
)`

type factionMember struct {
	ID          int64  `json:"id"`
	CharacterID int64  `json:"character_id"`
	Name        string `json:"name"`
	PicturePath string `json:"picture_path"`
	Role        string `json:"role"`
	Since       string `json:"since"`
	Until       string `json:"until"`
}

type factionOut struct {
	ID           int64           `json:"id"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	PicturePath  string          `json:"picture_path"`
	Color        string          `json:"color"`
	HQLocationID *int64          `json:"hq_location_id"`
	HQName       string          `json:"hq_name"`
	ParentID     *int64          `json:"parent_id"`
	ParentName   string          `json:"parent_name"`
	FoundingDate string          `json:"founding_date"`
	MemberCount  int             `json:"member_count"`
	Members      []factionMember `json:"members,omitempty"`
}

const factionSelect = `
	SELECT f.id, f.name, f.description, COALESCE(f.picture_path, ''), COALESCE(f.color, ''),
	       f.hq_location_id, COALESCE(l.name, ''), f.parent_id, COALESCE(p.name, ''), f.founding_date,
	       (SELECT COUNT(*) FROM faction_members m WHERE m.faction_id = f.id)
	FROM factions f
	LEFT JOIN locations l ON l.id = f.hq_location_id
	LEFT JOIN factions p ON p.id = f.parent_id`

func scanFaction(s rowScanner) (factionOut, error) {
	var f factionOut
	var hq, parent sql.NullInt64
	err := s.Scan(&f.ID, &f.Name, &f.Description, &f.PicturePath, &f.Color,
		&hq, &f.HQName, &parent, &f.ParentName, &f.FoundingDate, &f.MemberCount)
	if hq.Valid {
		f.HQLocationID = &hq.Int64
	}
	if parent.Valid {
		f.ParentID = &parent.Int64
	}
	f.PicturePath = uploadURL(f.PicturePath)
	return f, err
}

func factionMembers(db *sql.DB, factionID int64) []factionMember {
	out := []factionMember{}
	rows, err := db.Query(`
		SELECT m.id, m.character_id, v.name, COALESCE(v.picture_path, ''), m.role, m.since, m.until
		FROM faction_members m
		JOIN character_versions v ON v.character_id = m.character_id AND v.is_current = 1
		WHERE m.faction_id = ?
		ORDER BY m.until != '', v.name COLLATE NOCASE`, factionID)
	if err != nil {
		log.Printf("faction members: %v", err)
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var m factionMember
		if rows.Scan(&m.ID, &m.CharacterID, &m.Name, &m.PicturePath, &m.Role, &m.Since, &m.Until) == nil {
			m.PicturePath = uploadURL(m.PicturePath)
			out = append(out, m)
		}
	}
	return out
}

type membership struct {
	FactionID   int64  `json:"faction_id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	PicturePath string `json:"picture_path"`
	Role        string `json:"role"`
	Since       string `json:"since"`
	Until       string `json:"until"`
}

// membershipsOf lists the factions a character belongs (or belonged) to,
// current ones first.
func membershipsOf(db *sql.DB, characterID int64) []membership {
	out := []membership{}
	rows, err := db.Query(`
		SELECT f.id, f.name, COALESCE(f.color, ''), COALESCE(f.picture_path, ''), m.role, m.since, m.until
		FROM faction_members m JOIN factions f ON f.id = m.faction_id
		WHERE m.character_id = ?
		ORDER BY m.until != '', f.name COLLATE NOCASE`, characterID)
	if err != nil {
		log.Printf("memberships: %v", err)
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var m membership
		if rows.Scan(&m.FactionID, &m.Name, &m.Color, &m.PicturePath, &m.Role, &m.Since, &m.Until) == nil {
			m.PicturePath = uploadURL(m.PicturePath)
			out = append(out, m)
		}
	}
	return out
}

func listFactionsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(factionSelect + ` ORDER BY f.name COLLATE NOCASE`)
		if err != nil {
			log.Printf("list factions: %v", err)
			http.Error(w, "failed to load factions", http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		out := []factionOut{}
		for rows.Next() {
			f, err := scanFaction(rows)
			if err != nil {
				log.Printf("scan faction: %v", err)
				continue
			}
			out = append(out, f)
		}
		writeJSON(w, out)
	}
}

func loadFaction(db *sql.DB, id int64) (factionOut, error) {
	f, err := scanFaction(db.QueryRow(factionSelect+` WHERE f.id = ?`, id))
	if err != nil {
		return f, err
	}
	f.Members = factionMembers(db, id)
	return f, nil
}

func getFactionHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "faction not found", http.StatusNotFound)
			return
		}
		f, err := loadFaction(db, id)
		if err != nil {
			http.Error(w, "faction not found", http.StatusNotFound)
			return
		}
		writeJSON(w, f)
	}
}

type factionForm struct {
	Name, Description, Color, FoundingDate string
	HQ, Parent                             sql.NullInt64
	Members                                []factionMember
}

// parseFactionForm reads and checks a posted faction. id is the faction
// being edited (0 for a new one), so its own name and parent chain can be
// checked against itself.
func parseFactionForm(db *sql.DB, r *http.Request, id int64) (factionForm, string) {
	var f factionForm
	f.Name = strings.TrimSpace(r.FormValue("name"))
	f.Description = strings.TrimSpace(r.FormValue("description"))
	if f.Name == "" {
		return f, "name is required"
	}
	if len(f.Name) > 120 || len(f.Description) > 20000 {
		return f, "that text is too long"
	}
	var taken int
	db.QueryRow(`SELECT COUNT(*) FROM factions WHERE name = ? COLLATE NOCASE AND id != ?`, f.Name, id).Scan(&taken)
	if taken > 0 {
		return f, "a faction with that name already exists"
	}
	color, ok := normalizeColor(r.FormValue("color"))
	if !ok {
		return f, "invalid color"
	}
	f.Color = color
	f.HQ = parseLocationID(db, r.FormValue("hq_location_id"), false)

	// An unknown parent quietly means none. A known one must not have this
	// faction among its ancestors, or the faction would sit inside itself.
	pid, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("parent_id")), 10, 64)
	var one int
	if pid > 0 && db.QueryRow(`SELECT 1 FROM factions WHERE id = ?`, pid).Scan(&one) == nil {
		cur := pid
		for steps := 0; ; steps++ {
			if cur == id || steps > 100 {
				return f, "a faction can't sit inside itself"
			}
			var next sql.NullInt64
			if db.QueryRow(`SELECT parent_id FROM factions WHERE id = ?`, cur).Scan(&next) != nil || !next.Valid {
				break
			}
			cur = next.Int64
		}
		f.Parent = sql.NullInt64{Int64: pid, Valid: true}
	}

	f.FoundingDate = normalizeStoryDate(r.FormValue("founding_date"))
	if f.FoundingDate != "" {
		if _, ok := parseStoryDate(f.FoundingDate); !ok {
			return f, "dates must look like DD-MM-YYYY (or just a year)"
		}
		if msg := checkDateFits(loadCalendar(db), "Founded", f.FoundingDate); msg != "" {
			return f, msg
		}
	}

	if raw := r.FormValue("members"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &f.Members); err != nil {
			return f, "invalid member list"
		}
		if len(f.Members) > 1000 {
			return f, "too many members"
		}
		for i := range f.Members {
			m := &f.Members[i]
			m.Role = strings.TrimSpace(m.Role)
			if len(m.Role) > 120 {
				return f, "that text is too long"
			}
			if msg := validateSpan(db, &m.Since, &m.Until); msg != "" {
				return f, msg
			}
		}
	}
	return f, ""
}

func saveFactionMembers(tx *sql.Tx, factionID int64, members []factionMember) error {
	if _, err := tx.Exec(`DELETE FROM faction_members WHERE faction_id = ?`, factionID); err != nil {
		return err
	}
	for _, m := range members {
		// Unknown characters quietly drop out.
		if _, err := tx.Exec(`INSERT INTO faction_members (faction_id, character_id, role, since, until)
			SELECT ?, id, ?, ?, ? FROM characters WHERE id = ?`,
			factionID, m.Role, m.Since, m.Until, m.CharacterID); err != nil {
			return err
		}
	}
	return nil
}

func saveFactionPicture(db *sql.DB, r *http.Request, uploadsDir string, id int64) {
	var old sql.NullString
	db.QueryRow(`SELECT picture_path FROM factions WHERE id = ?`, id).Scan(&old)
	if r.FormValue("remove_picture") == "1" && old.Valid {
		db.Exec(`UPDATE factions SET picture_path = NULL WHERE id = ?`, id)
		removePicture(uploadsDir, old.String)
		old = sql.NullString{}
	}
	file, header, err := r.FormFile("picture")
	if err != nil {
		return
	}
	defer file.Close()
	rel, err := saveUpload(uploadsDir, "factions", fmt.Sprintf("%d-%d", id, time.Now().Unix()), header.Filename, file)
	if err != nil {
		log.Printf("save faction picture: %v", err)
		return
	}
	if _, err := db.Exec(`UPDATE factions SET picture_path = ? WHERE id = ?`, rel, id); err != nil {
		log.Printf("update faction picture: %v", err)
		return
	}
	if old.Valid && old.String != "" && old.String != rel {
		removePicture(uploadsDir, old.String)
	}
}

// saveFactionHandler creates (no id in the path) or updates a faction,
// with its member list, in one multipart request.
func saveFactionHandler(db *sql.DB, uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var id int64
		if raw := r.PathValue("id"); raw != "" {
			var err error
			if id, err = strconv.ParseInt(raw, 10, 64); err != nil {
				http.Error(w, "faction not found", http.StatusNotFound)
				return
			}
			var one int
			if db.QueryRow(`SELECT 1 FROM factions WHERE id = ?`, id).Scan(&one) != nil {
				http.Error(w, "faction not found", http.StatusNotFound)
				return
			}
		}
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, "invalid form data", http.StatusBadRequest)
			return
		}
		f, msg := parseFactionForm(db, r, id)
		if msg != "" {
			http.Error(w, msg, http.StatusBadRequest)
			return
		}

		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to save the faction", http.StatusInternalServerError)
			return
		}
		if id == 0 {
			res, err := tx.Exec(`INSERT INTO factions (name, description, color, hq_location_id, parent_id, founding_date)
				VALUES (?, ?, ?, ?, ?, ?)`, f.Name, f.Description, nullableString(f.Color), f.HQ, f.Parent, f.FoundingDate)
			if err != nil {
				tx.Rollback()
				log.Printf("create faction: %v", err)
				http.Error(w, "failed to save the faction", http.StatusInternalServerError)
				return
			}
			id, _ = res.LastInsertId()
		} else if _, err := tx.Exec(`UPDATE factions SET name = ?, description = ?, color = ?, hq_location_id = ?,
				parent_id = ?, founding_date = ?, updated_at = datetime('now') WHERE id = ?`,
			f.Name, f.Description, nullableString(f.Color), f.HQ, f.Parent, f.FoundingDate, id); err != nil {
			tx.Rollback()
			log.Printf("update faction: %v", err)
			http.Error(w, "failed to save the faction", http.StatusInternalServerError)
			return
		}
		if r.Form["members"] != nil {
			if err := saveFactionMembers(tx, id, f.Members); err != nil {
				tx.Rollback()
				log.Printf("faction members: %v", err)
				http.Error(w, "failed to save the faction", http.StatusInternalServerError)
				return
			}
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to save the faction", http.StatusInternalServerError)
			return
		}
		saveFactionPicture(db, r, uploadsDir, id)

		out, err := loadFaction(db, id)
		if err != nil {
			http.Error(w, "failed to save the faction", http.StatusInternalServerError)
			return
		}
		writeJSON(w, out)
	}
}

func deleteFactionHandler(db *sql.DB, uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "faction not found", http.StatusNotFound)
			return
		}
		var pic sql.NullString
		if err := db.QueryRow(`SELECT picture_path FROM factions WHERE id = ?`, id).Scan(&pic); err != nil {
			http.Error(w, "faction not found", http.StatusNotFound)
			return
		}
		if _, err := db.Exec(`DELETE FROM factions WHERE id = ?`, id); err != nil {
			log.Printf("delete faction: %v", err)
			http.Error(w, "failed to delete the faction", http.StatusInternalServerError)
			return
		}
		if pic.Valid {
			removePicture(uploadsDir, pic.String)
		}
		writeJSON(w, map[string]any{"deleted": id})
	}
}
