package main

import (
	"database/sql"
	"log"
	"net/http"
	"slices"
)

// Body outlines are the plain figures the height chart draws
// (frontend/src/bodyOutlines.js, HeightChart.vue). A body type picks a build
// and, if it wants, a sex and an age; what it leaves empty comes from the
// character (gender, age). The keys exist twice, here and in
// bodyOutlines.js: change both.
var (
	outlineSexes  = []string{"", "male", "female"}
	outlineAges   = []string{"", "child", "teen", "adult", "elderly"}
	outlineBuilds = []string{"", "slim", "average", "athletic", "chubby", "heavy"}
)

type bodyOutline struct {
	Sex   string `json:"sex"`
	Age   string `json:"age"`
	Build string `json:"build"`
}

func (o bodyOutline) valid() bool {
	return slices.Contains(outlineSexes, o.Sex) && slices.Contains(outlineAges, o.Age) && slices.Contains(outlineBuilds, o.Build)
}

// schema40Statements give every body type an outline, all of it taken from
// the character until the writer picks one.
var schema40Statements = []string{
	`ALTER TABLE body_types ADD COLUMN outline_sex TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE body_types ADD COLUMN outline_age TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE body_types ADD COLUMN outline_build TEXT NOT NULL DEFAULT ''`,
}

func loadBodyOutline(q interface {
	QueryRow(string, ...any) *sql.Row
}, id int64) bodyOutline {
	var o bodyOutline
	q.QueryRow(`SELECT outline_sex, outline_age, outline_build FROM body_types WHERE id = ?`, id).Scan(&o.Sex, &o.Age, &o.Build)
	return o
}

func saveBodyOutline(tx *sql.Tx, id int64, o bodyOutline) error {
	_, err := tx.Exec(`UPDATE body_types SET outline_sex = ?, outline_age = ?, outline_build = ? WHERE id = ?`, o.Sex, o.Age, o.Build, id)
	return err
}

type bodyTypeOption struct {
	ID      int64       `json:"id"`
	Name    string      `json:"name"`
	Outline bodyOutline `json:"outline"`
}

func listBodyTypes(db *sql.DB) ([]bodyTypeOption, error) {
	rows, err := db.Query(`SELECT id, name, outline_sex, outline_age, outline_build FROM body_types ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []bodyTypeOption{}
	for rows.Next() {
		var b bodyTypeOption
		if err := rows.Scan(&b.ID, &b.Name, &b.Outline.Sex, &b.Outline.Age, &b.Outline.Build); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// listBodyTypesHandler is the body types' lookup list, with their outlines.
func listBodyTypesHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := listBodyTypes(db)
		if err != nil {
			http.Error(w, "failed to load body types", http.StatusInternalServerError)
			log.Printf("list body types: %v", err)
			return
		}
		writeJSON(w, list)
	}
}

type heightVersion struct {
	ID         int64    `json:"id"`
	Name       string   `json:"name"`
	Nickname   string   `json:"nickname"`
	IsCurrent  bool     `json:"is_current"`
	Date       string   `json:"version_date"`
	Reference  string   `json:"version_reference"`
	Picture    string   `json:"picture_path"`
	Height     *float64 `json:"height"`
	Gender     string   `json:"gender"`
	Age        *float64 `json:"age"`
	BodyTypeID *int64   `json:"body_type_id"`
}

type heightCharacter struct {
	ID       int64           `json:"id"`
	Versions []heightVersion `json:"versions"`
}

// characterHeightsHandler gives the height chart every character's versions
// (height, gender, age, body type) and the body types' outlines.
func characterHeightsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bodyTypes, err := listBodyTypes(db)
		if err != nil {
			http.Error(w, "failed to load body types", http.StatusInternalServerError)
			log.Printf("heights body types: %v", err)
			return
		}
		rows, err := db.Query(`
			SELECT v.character_id, v.id, v.name, COALESCE(v.nickname, ''), v.is_current,
				COALESCE(v.version_date, ''), COALESCE(v.version_reference, ''), COALESCE(v.picture_path, ''),
				st.height, COALESCE(st.gender, ''), st.age, st.body_type_id
			FROM character_versions v
			JOIN characters c ON c.id = v.character_id
			LEFT JOIN character_story st ON st.version_id = v.id
			ORDER BY (SELECT name FROM character_versions WHERE character_id = v.character_id AND is_current = 1) COLLATE NOCASE,
				v.character_id, v.id`)
		if err != nil {
			http.Error(w, "failed to load characters", http.StatusInternalServerError)
			log.Printf("heights: %v", err)
			return
		}
		defer rows.Close()
		characters := []heightCharacter{}
		for rows.Next() {
			var charID int64
			var v heightVersion
			var height, age sql.NullFloat64
			var bodyType sql.NullInt64
			if err := rows.Scan(&charID, &v.ID, &v.Name, &v.Nickname, &v.IsCurrent, &v.Date, &v.Reference, &v.Picture,
				&height, &v.Gender, &age, &bodyType); err != nil {
				http.Error(w, "failed to read characters", http.StatusInternalServerError)
				log.Printf("heights scan: %v", err)
				return
			}
			v.Picture = uploadURL(v.Picture)
			if height.Valid {
				v.Height = &height.Float64
			}
			if age.Valid {
				v.Age = &age.Float64
			}
			if bodyType.Valid {
				v.BodyTypeID = &bodyType.Int64
			}
			if n := len(characters); n == 0 || characters[n-1].ID != charID {
				characters = append(characters, heightCharacter{ID: charID})
			}
			last := &characters[len(characters)-1]
			last.Versions = append(last.Versions, v)
		}
		writeJSON(w, map[string]any{"characters": characters, "body_types": bodyTypes})
	}
}
