package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type characterSummary struct {
	ID           int64  `json:"id"`
	VersionID    int64  `json:"version_id"`
	Name         string `json:"name"`
	Nickname     string `json:"nickname"`
	Level        int    `json:"level"`
	ClassName    string `json:"class_name"`
	ClassIcon    string `json:"class_icon"`
	SubclassName string `json:"subclass_name"`
	RaceName     string `json:"race_name"`
	PicturePath  string `json:"picture_path"`
	UpdatedAt    string `json:"updated_at"`
	HP           int    `json:"hp"`
	MP           int    `json:"mp"`
}

// listCharactersHandler powers the grid. HP/MP shown here go through the
// same computeStats pipeline as the View/Edit pages (via buildVersionDetail)
// so they reflect the real formula and any Class/Race/etc. modifiers —
// not the raw VIT/WIS point-buy values.
func listCharactersHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(`
			SELECT
				c.id, v.id, v.name, COALESCE(v.nickname, ''), v.level,
				COALESCE(cl.name, ''), COALESCE(sc.name, ''),
				COALESCE(v.picture_path, ''), COALESCE(cl.icon_path, ''),
				COALESCE(ra.name, ''), v.updated_at
			FROM characters c
			JOIN character_versions v ON v.character_id = c.id AND v.is_current = 1
			LEFT JOIN classes cl ON cl.id = v.class_id
			LEFT JOIN subclasses sc ON sc.id = v.subclass_id
			LEFT JOIN character_story st ON st.version_id = v.id
			LEFT JOIN races ra ON ra.id = st.race_id
			ORDER BY v.name COLLATE NOCASE
		`)
		if err != nil {
			http.Error(w, "failed to load characters", http.StatusInternalServerError)
			log.Printf("listCharacters query: %v", err)
			return
		}

		type row struct {
			characterID, versionID                                  int64
			name, nickname, className, subclassName, pic, classIcon string
			raceName, updatedAt                                     string
			level                                                   int
		}
		var raw []row
		for rows.Next() {
			var rr row
			if err := rows.Scan(
				&rr.characterID, &rr.versionID, &rr.name, &rr.nickname, &rr.level,
				&rr.className, &rr.subclassName, &rr.pic, &rr.classIcon,
				&rr.raceName, &rr.updatedAt,
			); err != nil {
				rows.Close()
				http.Error(w, "failed to read characters", http.StatusInternalServerError)
				log.Printf("listCharacters scan: %v", err)
				return
			}
			raw = append(raw, rr)
		}
		rows.Close()

		results := []characterSummary{}
		for _, rr := range raw {
			c := characterSummary{
				ID: rr.characterID, VersionID: rr.versionID, Name: rr.name, Nickname: rr.nickname,
				Level: rr.level, ClassName: rr.className, SubclassName: rr.subclassName,
				RaceName: rr.raceName, UpdatedAt: rr.updatedAt,
				ClassIcon: uploadURL(rr.classIcon),
			}
			if rr.pic != "" {
				c.PicturePath = "/uploads/" + rr.pic
			}
			if detail, err := buildVersionDetail(db, fmt.Sprintf("%d", rr.versionID)); err == nil {
				c.HP = int(math.Round(detail.Computed.Base.HP))
				c.MP = int(math.Round(detail.Computed.Base.MP))
			} else {
				log.Printf("listCharacters computeStats for version %d: %v", rr.versionID, err)
			}
			results = append(results, c)
		}

		writeJSON(w, results)
	}
}

// createCharacterHandler creates a character and its first version in one
// step, matching the existing Add Character form. version_date and
// version_reference are optional even for this first version — nothing
// stops someone from tagging where the character's introduction falls in
// the timeline right away.
func createCharacterHandler(db *sql.DB, uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(10 << 20); err != nil { // 10 MB cap
			http.Error(w, "invalid form data", http.StatusBadRequest)
			return
		}

		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		if msg := checkDateFits(loadCalendar(db), "Story date", r.FormValue("version_date")); msg != "" {
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		nickname := strings.TrimSpace(r.FormValue("nickname"))
		level := parseIntDefault(r.FormValue("level"), 1)
		if level < 1 {
			level = 1
		}

		classID, err := upsertClass(db, r.FormValue("class"))
		if err != nil {
			http.Error(w, "failed to save class", http.StatusInternalServerError)
			log.Printf("upsertClass: %v", err)
			return
		}
		subclassID, err := upsertSubclass(db, classID, r.FormValue("subclass"))
		if err != nil {
			http.Error(w, "failed to save subclass", http.StatusInternalServerError)
			log.Printf("upsertSubclass: %v", err)
			return
		}
		specializationID, err := upsertSpecialization(db, classID, r.FormValue("specialization"))
		if err != nil {
			http.Error(w, "failed to save specialization", http.StatusInternalServerError)
			log.Printf("upsertSpecialization: %v", err)
			return
		}

		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to start transaction", http.StatusInternalServerError)
			return
		}

		charRes, err := tx.Exec(`INSERT INTO characters DEFAULT VALUES`)
		if err != nil {
			tx.Rollback()
			http.Error(w, "failed to create character", http.StatusInternalServerError)
			log.Printf("insert character: %v", err)
			return
		}
		characterID, _ := charRes.LastInsertId()

		versionRes, err := tx.Exec(
			`INSERT INTO character_versions
				(character_id, is_current, version_date, version_reference,
				 name, nickname, level, class_id, subclass_id, specialization_id)
			 VALUES (?, 1, ?, ?, ?, ?, ?, ?, ?, ?)`,
			characterID,
			nullableString(normalizeStoryDate(r.FormValue("version_date"))),
			nullableString(strings.TrimSpace(r.FormValue("version_reference"))),
			name, nullableString(nickname), level, classID, subclassID, specializationID,
		)
		if err != nil {
			tx.Rollback()
			http.Error(w, "failed to create character version", http.StatusInternalServerError)
			log.Printf("insert character_version: %v", err)
			return
		}
		versionID, _ := versionRes.LastInsertId()

		if _, err := tx.Exec(`INSERT INTO character_story (version_id) VALUES (?)`, versionID); err != nil {
			tx.Rollback()
			http.Error(w, "failed to create character story row", http.StatusInternalServerError)
			return
		}
		if _, err := tx.Exec(`INSERT INTO character_build (version_id) VALUES (?)`, versionID); err != nil {
			tx.Rollback()
			http.Error(w, "failed to create character build row", http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to save character", http.StatusInternalServerError)
			return
		}

		if file, header, err := r.FormFile("picture"); err == nil {
			defer file.Close()
			relPath, saveErr := savePicture(uploadsDir, fmt.Sprintf("%d", versionID), header.Filename, file)
			if saveErr != nil {
				log.Printf("savePicture: %v", saveErr)
			} else if _, err := db.Exec(
				`UPDATE character_versions SET picture_path = ? WHERE id = ?`,
				relPath, versionID,
			); err != nil {
				log.Printf("update picture_path: %v", err)
			}
		}

		writeJSON(w, map[string]any{"id": characterID, "version_id": versionID})
	}
}

// savePicture names files after the version they belong to (not the
// character), since pictures live per-version.
func savePicture(uploadsDir string, versionID string, originalName string, src io.Reader) (string, error) {
	return saveUpload(uploadsDir, "characters", versionID, originalName, src)
}

// saveUpload writes an uploaded file to uploads/<subdir>/<stem><ext> and
// returns its path relative to the uploads directory (what gets stored in
// the database).
func saveUpload(uploadsDir, subdir, stem, originalName string, src io.Reader) (string, error) {
	ext := strings.ToLower(filepath.Ext(originalName))
	if ext == "" {
		ext = ".png"
	}
	if err := os.MkdirAll(filepath.Join(uploadsDir, subdir), 0o755); err != nil {
		return "", err
	}
	relPath := filepath.Join(subdir, stem+ext)
	fullPath := filepath.Join(uploadsDir, relPath)

	dst, err := os.Create(fullPath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", err
	}
	return filepath.ToSlash(relPath), nil
}

func removePicture(uploadsDir, relPath string) {
	if relPath == "" || isBuiltinIcon(relPath) {
		return
	}
	// A path from the database (or an imported story) must stay inside the
	// uploads folder.
	clean, ok := safeRel(relPath)
	if !ok {
		return
	}
	os.Remove(filepath.Join(uploadsDir, filepath.FromSlash(clean)))
}

// deleteCharactersHandler removes whole characters — every version, story
// row, build row, and uploaded picture that belongs to them.
func deleteCharactersHandler(db *sql.DB, uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			IDs []int64 `json:"ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || len(payload.IDs) == 0 {
			http.Error(w, "at least one id is required", http.StatusBadRequest)
			return
		}

		placeholders := make([]string, len(payload.IDs))
		args := make([]any, len(payload.IDs))
		for i, id := range payload.IDs {
			placeholders[i] = "?"
			args[i] = id
		}

		selectQuery := fmt.Sprintf(
			`SELECT picture_path FROM character_versions
			 WHERE character_id IN (%s) AND picture_path IS NOT NULL`,
			strings.Join(placeholders, ","),
		)
		if rows, err := db.Query(selectQuery, args...); err == nil {
			for rows.Next() {
				var p string
				if rows.Scan(&p) == nil {
					removePicture(uploadsDir, p)
				}
			}
			rows.Close()
		}

		// Birth-details events attached to these characters go with them,
		// and so do their appearances in other events' people lists.
		deleteSourceEvents(db, uploadsDir, "character", payload.IDs)
		if _, err := db.Exec(
			fmt.Sprintf(`DELETE FROM event_characters WHERE character_id IN (%s)`, strings.Join(placeholders, ",")), args...,
		); err != nil {
			log.Printf("delete event_characters: %v", err)
		}

		deleteQuery := fmt.Sprintf(`DELETE FROM characters WHERE id IN (%s)`, strings.Join(placeholders, ","))
		if _, err := db.Exec(deleteQuery, args...); err != nil {
			http.Error(w, "failed to delete characters", http.StatusInternalServerError)
			log.Printf("delete characters: %v", err)
			return
		}

		writeJSON(w, map[string]any{"deleted": len(payload.IDs)})
	}
}
