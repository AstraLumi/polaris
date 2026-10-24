package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
)

type versionSummary struct {
	ID               int64  `json:"id"`
	IsCurrent        bool   `json:"is_current"`
	VersionDate      string `json:"version_date"`
	VersionReference string `json:"version_reference"`
	Name             string `json:"name"`
	Nickname         string `json:"nickname"`
	Level            int    `json:"level"`
	ClassName        string `json:"class_name"`
	SubclassName     string `json:"subclass_name"`
	ChapterID        *int64 `json:"chapter_id"`
	CreatedAt        string `json:"created_at"`
}

type storyDetail struct {
	Gender       string   `json:"gender"`
	RaceName     string   `json:"race_name"`
	Height       *float64 `json:"height"`
	Weight       *float64 `json:"weight"`
	BodyTypeName string   `json:"body_type_name"`
	Age          *float64 `json:"age"`
	// Blood type, Born in, Birth date, the Earth birth date and the Earth
	// name are the character's own (the same in every version), shown with
	// each version's story for convenience.
	HumanBirthDate string `json:"human_birth_date"`
	EarthName      string `json:"earth_name"`
	BloodType      string `json:"blood_type"`
	// Born in / Nation link to Map locations. born_in / nation carry the
	// old free text only while it hasn't been matched to a location.
	BornIn           string `json:"born_in"`
	BornInLocationID *int64 `json:"born_in_location_id"`
	BornInName       string `json:"born_in_name"`
	Nation           string `json:"nation"`
	NationLocationID *int64 `json:"nation_location_id"`
	NationName       string `json:"nation_name"`
	BirthDate        string `json:"birth_date"`
	Deaths           int    `json:"deaths"`
	Description      string `json:"description"`
	Bio              string `json:"bio"`
	SpeechMannerisms string `json:"speech_mannerisms"`
	Status           string `json:"status"` // "alive" | "missing" | "dead"
}

type versionDetail struct {
	ID                 int64                 `json:"id"`
	CharacterID        int64                 `json:"character_id"`
	Tags               []string              `json:"tags"` // the character's, shared by all versions
	IsCurrent          bool                  `json:"is_current"`
	VersionDate        string                `json:"version_date"`
	VersionReference   string                `json:"version_reference"`
	ChapterID          *int64                `json:"chapter_id"`
	Name               string                `json:"name"`
	Nickname           string                `json:"nickname"`
	Level              int                   `json:"level"`
	ClassName          string                `json:"class_name"`
	ClassIcon          string                `json:"class_icon"`
	SubclassName       string                `json:"subclass_name"`
	SpecializationName string                `json:"specialization_name"`
	PicturePath        string                `json:"picture_path"`
	Story              storyDetail           `json:"story"`
	Build              map[string]int        `json:"build"`         // points spent on each primary stat
	SpecialBases       map[string]float64    `json:"special_bases"` // [base] typed for calculated stats
	Spells             []spellDetail         `json:"spells"`
	Gear               map[string]gearDetail `json:"gear"`     // equipped pieces by equipment slot
	Computed           map[string]float64    `json:"computed"` // every stat by key, bonuses included
	Points             int                   `json:"points"`   // points this version has to spend on primary stats
}

// listVersionsHandler orders the current version first, then the rest by
// their in-story date where that parses cleanly as DD-MM-YYYY, falling
// back to creation order for anything that doesn't.
func listVersionsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		characterID := r.PathValue("id")

		rows, err := db.Query(`
			SELECT
				v.id, v.is_current, COALESCE(v.version_date, ''), COALESCE(v.version_reference, ''),
				v.name, COALESCE(v.nickname, ''), v.level,
				COALESCE(cl.name, ''), COALESCE(sc.name, ''), v.created_at, v.chapter_id
			FROM character_versions v
			LEFT JOIN classes cl ON cl.id = v.class_id
			LEFT JOIN subclasses sc ON sc.id = v.subclass_id
			WHERE v.character_id = ?
			ORDER BY
				v.is_current DESC,
				CASE WHEN length(v.version_date) = 10
					THEN substr(v.version_date, 7, 4) || substr(v.version_date, 4, 2) || substr(v.version_date, 1, 2)
					ELSE ''
				END DESC,
				v.created_at DESC
		`, characterID)
		if err != nil {
			http.Error(w, "failed to load versions", http.StatusInternalServerError)
			log.Printf("listVersions query: %v", err)
			return
		}
		defer rows.Close()

		results := []versionSummary{}
		for rows.Next() {
			var v versionSummary
			var chapter sql.NullInt64
			if err := rows.Scan(
				&v.ID, &v.IsCurrent, &v.VersionDate, &v.VersionReference,
				&v.Name, &v.Nickname, &v.Level, &v.ClassName, &v.SubclassName, &v.CreatedAt, &chapter,
			); err != nil {
				http.Error(w, "failed to read versions", http.StatusInternalServerError)
				log.Printf("listVersions scan: %v", err)
				return
			}
			if chapter.Valid {
				v.ChapterID = &chapter.Int64
			}
			results = append(results, v)
		}

		writeJSON(w, results)
	}
}

// createVersionHandler clones an existing version (defaulting to the
// current one) into a new, non-current version tagged with its own
// story-timeline date and reference. The picture is deliberately not
// copied — two versions sharing one image file would make deleting either
// of them risk breaking the other.
func createVersionHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		characterID := r.PathValue("id")

		var payload struct {
			VersionDate      string `json:"version_date"`
			VersionReference string `json:"version_reference"`
			CloneFromID      *int64 `json:"clone_from_version_id"`
			ChapterID        *int64 `json:"chapter_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		var sourceID int64
		if payload.CloneFromID != nil {
			if err := db.QueryRow(
				`SELECT id FROM character_versions WHERE id = ? AND character_id = ?`,
				*payload.CloneFromID, characterID,
			).Scan(&sourceID); err != nil {
				http.Error(w, "source version not found for this character", http.StatusBadRequest)
				return
			}
		} else {
			if err := db.QueryRow(
				`SELECT id FROM character_versions WHERE character_id = ? AND is_current = 1`,
				characterID,
			).Scan(&sourceID); err != nil {
				http.Error(w, "this character has no current version to clone", http.StatusBadRequest)
				return
			}
		}

		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to start transaction", http.StatusInternalServerError)
			return
		}

		res, err := tx.Exec(`
			INSERT INTO character_versions
				(character_id, is_current, version_date, version_reference, chapter_id,
				 name, nickname, level, class_id, subclass_id, specialization_id, picture_path)
			SELECT character_id, 0, ?, ?, ?, name, nickname, level, class_id, subclass_id, specialization_id, NULL
			FROM character_versions WHERE id = ?
		`, nullableString(normalizeStoryDate(payload.VersionDate)), nullableString(strings.TrimSpace(payload.VersionReference)),
			validChapter(db, chapterParam(payload.ChapterID)), sourceID)
		if err != nil {
			tx.Rollback()
			http.Error(w, "failed to create version", http.StatusInternalServerError)
			log.Printf("clone version: %v", err)
			return
		}
		newVersionID, _ := res.LastInsertId()

		if _, err := tx.Exec(`
			INSERT INTO character_story
				(version_id, gender, race_id, height, weight, body_type_id,
				 nation, nation_location_id,
				 deaths, description, bio, speech_mannerisms, status)
			SELECT ?, gender, race_id, height, weight, body_type_id,
				nation, nation_location_id,
				deaths, description, bio, speech_mannerisms, status
			FROM character_story WHERE version_id = ?
		`, newVersionID, sourceID); err != nil {
			tx.Rollback()
			http.Error(w, "failed to clone story data", http.StatusInternalServerError)
			log.Printf("clone story: %v", err)
			return
		}

		if _, err := tx.Exec(`
			INSERT INTO character_special_bases (version_id, stat_key, value)
			SELECT ?, stat_key, value FROM character_special_bases WHERE version_id = ?
		`, newVersionID, sourceID); err != nil {
			tx.Rollback()
			http.Error(w, "failed to clone special stat bases", http.StatusInternalServerError)
			log.Printf("clone special bases: %v", err)
			return
		}

		if _, err := tx.Exec(`
			INSERT INTO character_spells (version_id, spell_id)
			SELECT ?, spell_id FROM character_spells WHERE version_id = ?
		`, newVersionID, sourceID); err != nil {
			tx.Rollback()
			http.Error(w, "failed to clone spells", http.StatusInternalServerError)
			log.Printf("clone spells: %v", err)
			return
		}

		if _, err := tx.Exec(`
			INSERT INTO character_gear (version_id, slot, gear_id)
			SELECT ?, slot, gear_id FROM character_gear WHERE version_id = ?
		`, newVersionID, sourceID); err != nil {
			tx.Rollback()
			http.Error(w, "failed to clone gear", http.StatusInternalServerError)
			log.Printf("clone gear: %v", err)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to save new version", http.StatusInternalServerError)
			return
		}

		writeJSON(w, map[string]any{"id": newVersionID})
	}
}

// buildVersionDetail assembles everything the View/Edit pages need for a
// single version: identity, story, raw build stats, and the full computed
// stat breakdown.
func buildVersionDetail(db *sql.DB, versionID string) (*versionDetail, error) {
	var d versionDetail
	var classID, subclassID, specializationID, raceID, bodyTypeID, chapterID sql.NullInt64
	var height, weight, age sql.NullFloat64
	var bornInLoc, nationLoc sql.NullInt64

	err := db.QueryRow(`
		SELECT
			v.id, v.character_id, v.is_current,
			COALESCE(v.version_date, ''), COALESCE(v.version_reference, ''),
			v.name, COALESCE(v.nickname, ''), v.level,
			COALESCE(cl.name, ''), COALESCE(sc.name, ''), COALESCE(sp.name, ''),
			COALESCE(v.picture_path, ''), COALESCE(cl.icon_path, ''),
			v.class_id, v.subclass_id, v.specialization_id,
			COALESCE(st.gender, ''), COALESCE(r.name, ''), st.height, st.weight, COALESCE(bt.name, ''),
			st.age,
			COALESCE(ch.human_birth_date, ''), COALESCE(ch.earth_name, ''), COALESCE(ch.blood_type, ''),
			COALESCE(ch.born_in, ''), COALESCE(st.nation, ''), COALESCE(ch.birth_date, ''),
			ch.born_in_location_id, COALESCE(bl.name, ''), st.nation_location_id, COALESCE(nl.name, ''),
			COALESCE(st.deaths, 0), COALESCE(st.description, ''), COALESCE(st.bio, ''),
			COALESCE(st.speech_mannerisms, ''), COALESCE(st.status, 'alive'), st.race_id, st.body_type_id, v.chapter_id
		FROM character_versions v
		JOIN characters ch ON ch.id = v.character_id
		LEFT JOIN classes cl ON cl.id = v.class_id
		LEFT JOIN subclasses sc ON sc.id = v.subclass_id
		LEFT JOIN specializations sp ON sp.id = v.specialization_id
		LEFT JOIN character_story st ON st.version_id = v.id
		LEFT JOIN races r ON r.id = st.race_id
		LEFT JOIN body_types bt ON bt.id = st.body_type_id
		LEFT JOIN locations bl ON bl.id = ch.born_in_location_id
		LEFT JOIN locations nl ON nl.id = st.nation_location_id
		WHERE v.id = ?
	`, versionID).Scan(
		&d.ID, &d.CharacterID, &d.IsCurrent, &d.VersionDate, &d.VersionReference,
		&d.Name, &d.Nickname, &d.Level,
		&d.ClassName, &d.SubclassName, &d.SpecializationName, &d.PicturePath, &d.ClassIcon,
		&classID, &subclassID, &specializationID,
		&d.Story.Gender, &d.Story.RaceName, &height, &weight, &d.Story.BodyTypeName,
		&age,
		&d.Story.HumanBirthDate, &d.Story.EarthName, &d.Story.BloodType, &d.Story.BornIn, &d.Story.Nation,
		&d.Story.BirthDate,
		&bornInLoc, &d.Story.BornInName, &nationLoc, &d.Story.NationName,
		&d.Story.Deaths, &d.Story.Description, &d.Story.Bio,
		&d.Story.SpeechMannerisms, &d.Story.Status, &raceID, &bodyTypeID, &chapterID,
	)
	if err != nil {
		return nil, err
	}

	if d.PicturePath != "" {
		d.PicturePath = "/uploads/" + d.PicturePath
	}
	d.ClassIcon = uploadURL(d.ClassIcon)
	if chapterID.Valid {
		d.ChapterID = &chapterID.Int64
	}
	if bornInLoc.Valid {
		d.Story.BornInLocationID = &bornInLoc.Int64
	}
	if nationLoc.Valid {
		d.Story.NationLocationID = &nationLoc.Int64
	}
	if height.Valid {
		d.Story.Height = &height.Float64
	}
	if weight.Valid {
		d.Story.Weight = &weight.Float64
	}
	if age.Valid {
		d.Story.Age = &age.Float64
	}

	specialBases, err := loadSpecialBases(db, d.ID)
	if err != nil {
		return nil, err
	}
	sys, err := loadStatSystem(db)
	if err != nil {
		return nil, err
	}
	d.Build, d.SpecialBases = sys.split(specialBases)

	spells, err := loadVersionSpells(db, d.ID)
	if err != nil {
		return nil, err
	}
	d.Spells = spells

	gear, err := loadVersionGear(db, d.ID)
	if err != nil {
		return nil, err
	}
	d.Gear = gear
	var itemIDs []int64
	for _, g := range gear {
		itemIDs = append(itemIDs, g.ID)
	}

	in := statInputs{
		Level:    float64(d.Level),
		Age:      resolveOrDefault(age, defaultAge),
		HeightCM: resolveOrDefault(height, defaultHeightCM),
		WeightKG: resolveOrDefault(weight, defaultWeightKG),
		Values:   specialBases,
	}
	d.Points = sys.pointsFor(in)
	computed, err := computeStats(db, sys, in,
		categoryRefs{
			ClassID:          classID,
			SubclassID:       subclassID,
			SpecializationID: specializationID,
			RaceID:           raceID,
			BodyTypeID:       bodyTypeID,
			ItemIDs:          itemIDs,
		},
	)
	if err != nil {
		return nil, err
	}
	d.Computed = computed

	d.Tags = characterTags(db, d.CharacterID)
	return &d, nil
}

// characterTags lists a character's tags. They belong to the character, not
// a version, so every version shows and edits the same ones.
func characterTags(db *sql.DB, characterID int64) []string {
	out := []string{}
	rows, err := db.Query(`SELECT tag FROM character_tags WHERE character_id = ? ORDER BY tag COLLATE NOCASE`, characterID)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var t string
		if rows.Scan(&t) == nil {
			out = append(out, t)
		}
	}
	return out
}

func replaceCharacterTags(tx *sql.Tx, characterID int64, tags []string) error {
	if _, err := tx.Exec(`DELETE FROM character_tags WHERE character_id = ?`, characterID); err != nil {
		return err
	}
	for _, t := range tags {
		if _, err := tx.Exec(`INSERT INTO character_tags (character_id, tag) VALUES (?, ?)`, characterID, t); err != nil {
			return err
		}
	}
	return nil
}

// loadSpecialBases reads every character_special_bases row for a version
// into a plain map: the points spent on primary stats and the [base] typed
// for calculated ones, by stat key. Missing keys are simply absent — Go's
// zero value for a missing map key (0) is exactly the right default.
func loadSpecialBases(db *sql.DB, versionID int64) (map[string]float64, error) {
	rows, err := db.Query(`SELECT stat_key, value FROM character_special_bases WHERE version_id = ?`, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bases := map[string]float64{}
	for rows.Next() {
		var key string
		var value float64
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		bases[key] = value
	}
	return bases, rows.Err()
}

// split sorts a version's typed numbers into the points spent on primary
// stats and the [base] of calculated stats. Every primary stat is listed, at 0
// when nothing is spent on it.
func (s *statSystem) split(values map[string]float64) (map[string]int, map[string]float64) {
	points, bases := map[string]int{}, map[string]float64{}
	for _, k := range s.keys("primary") {
		points[k] = int(values[k])
	}
	for k, v := range values {
		if d := s.def(k); d != nil && d.Category != "primary" {
			bases[k] = v
		}
	}
	return points, bases
}

// saveSpecialBases replaces every typed number of a version (points and
// bases) with whatever's in values, keeping only the story's own stats.
// Deleting and re-inserting is simpler than an upsert-per-key loop and the
// table holds a few dozen rows per version.
func saveSpecialBases(tx *sql.Tx, versionID string, sys *statSystem, values map[string]float64) error {
	if _, err := tx.Exec(`DELETE FROM character_special_bases WHERE version_id = ?`, versionID); err != nil {
		return err
	}
	for key, v := range values {
		if v == 0 || sys.def(key) == nil {
			continue // don't bother storing zeros — a missing key already reads as 0
		}
		if _, err := tx.Exec(
			`INSERT INTO character_special_bases (version_id, stat_key, value) VALUES (?, ?, ?)`,
			versionID, key, v,
		); err != nil {
			return err
		}
	}
	return nil
}

func getVersionHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		detail, err := buildVersionDetail(db, r.PathValue("id"))
		if err != nil {
			http.Error(w, "version not found", http.StatusNotFound)
			return
		}
		writeJSON(w, detail)
	}
}

func getCurrentVersionHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var versionID string
		err := db.QueryRow(
			`SELECT id FROM character_versions WHERE character_id = ? AND is_current = 1`,
			r.PathValue("id"),
		).Scan(&versionID)
		if err != nil {
			http.Error(w, "character has no current version", http.StatusNotFound)
			return
		}
		detail, err := buildVersionDetail(db, versionID)
		if err != nil {
			http.Error(w, "version not found", http.StatusNotFound)
			return
		}
		writeJSON(w, detail)
	}
}

// updateVersionHandler saves identity, story, and the typed stats in one
// request. Points spent on primary stats can't exceed what the stat system's
// points formula gives — checked here as a safety net even though the UI
// already enforces it while editing.
func updateVersionHandler(db *sql.DB, uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		versionID := r.PathValue("id")

		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, "invalid form data", http.StatusBadRequest)
			return
		}

		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		level := parseIntDefault(r.FormValue("level"), 1)
		if level < 1 {
			level = 1
		}

		cal := loadCalendar(db)
		for label, field := range map[string]string{
			"Story date": "version_date", "Birth date": "birth_date",
		} {
			if msg := checkDateFits(cal, label, r.FormValue(field)); msg != "" {
				http.Error(w, msg, http.StatusBadRequest)
				return
			}
		}
		if msg := checkDateFits(earthCalendar, "Earth birthdate", r.FormValue("human_birth_date")); msg != "" {
			http.Error(w, msg, http.StatusBadRequest)
			return
		}

		// build: { primary stat: points }; special_bases: { stat: [base] }.
		// Either can be left out, keeping what's saved.
		sys, err := loadStatSystem(db)
		if err != nil {
			http.Error(w, "failed to load stats", http.StatusInternalServerError)
			return
		}
		vid, _ := strconv.ParseInt(versionID, 10, 64)
		saved, err := loadSpecialBases(db, vid)
		if err != nil {
			http.Error(w, "failed to load stats", http.StatusInternalServerError)
			return
		}
		build, specialBaseValues := sys.split(saved)
		if raw := r.FormValue("build"); raw != "" && json.Unmarshal([]byte(raw), &build) != nil {
			http.Error(w, "invalid stats", http.StatusBadRequest)
			return
		}
		if raw := r.FormValue("special_bases"); raw != "" && json.Unmarshal([]byte(raw), &specialBaseValues) != nil {
			http.Error(w, "invalid stats", http.StatusBadRequest)
			return
		}
		typed := map[string]float64{}
		for k, v := range specialBaseValues {
			if d := sys.def(k); d != nil && d.Category != "primary" {
				typed[k] = v
			}
		}
		spent := 0
		for _, k := range sys.keys("primary") {
			build[k] = max(build[k], 0)
			spent += build[k]
			typed[k] = float64(build[k])
		}
		budget := sys.pointsFor(statInputs{
			Level:    float64(level),
			Age:      parseFloatDefault(r.FormValue("age"), defaultAge),
			HeightCM: parseFloatDefault(r.FormValue("height"), defaultHeightCM),
			WeightKG: parseFloatDefault(r.FormValue("weight"), defaultWeightKG),
		})
		if spent > budget {
			http.Error(w, fmt.Sprintf("stat points spent (%d) exceed the points available (%d)", spent, budget), http.StatusBadRequest)
			return
		}

		bornInLoc := parseLocationID(db, r.FormValue("born_in_location_id"), false)
		nationLoc := parseLocationID(db, r.FormValue("nation_location_id"), true)

		classID, err := upsertClass(db, r.FormValue("class"))
		if err != nil {
			http.Error(w, "failed to save class", http.StatusInternalServerError)
			return
		}
		subclassID, err := upsertSubclass(db, classID, r.FormValue("subclass"))
		if err != nil {
			http.Error(w, "failed to save subclass", http.StatusInternalServerError)
			return
		}
		specializationID, err := upsertSpecialization(db, classID, r.FormValue("specialization"))
		if err != nil {
			http.Error(w, "failed to save specialization", http.StatusInternalServerError)
			return
		}
		raceID, err := upsertRace(db, r.FormValue("race"))
		if err != nil {
			http.Error(w, "failed to save race", http.StatusInternalServerError)
			return
		}
		bodyTypeID, err := upsertBodyType(db, r.FormValue("body_type"))
		if err != nil {
			http.Error(w, "failed to save body type", http.StatusInternalServerError)
			return
		}
		desc, bio, speech := r.FormValue("description"), r.FormValue("bio"), r.FormValue("speech_mannerisms")
		if !storeLinksOrFail(w, db, &desc, &bio, &speech) {
			return
		}

		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to start transaction", http.StatusInternalServerError)
			return
		}

		_, err = tx.Exec(`
			UPDATE character_versions
			SET version_date = ?, version_reference = ?, chapter_id = ?, name = ?, nickname = ?, level = ?,
				class_id = ?, subclass_id = ?, specialization_id = ?, updated_at = datetime('now')
			WHERE id = ?
		`,
			nullableString(normalizeStoryDate(r.FormValue("version_date"))),
			nullableString(strings.TrimSpace(r.FormValue("version_reference"))),
			validChapter(db, r.FormValue("chapter_id")),
			name, nullableString(strings.TrimSpace(r.FormValue("nickname"))), level,
			classID, subclassID, specializationID, versionID,
		)
		if err != nil {
			tx.Rollback()
			http.Error(w, "failed to update character version", http.StatusInternalServerError)
			log.Printf("update character_version: %v", err)
			return
		}

		_, err = tx.Exec(`
			UPDATE character_story
			SET gender = ?, race_id = ?, height = ?, weight = ?, body_type_id = ?, age = ?,
				nation = ?, nation_location_id = ?,
				deaths = ?, description = ?, bio = ?, speech_mannerisms = ?, status = ?
			WHERE version_id = ?
		`,
			nullableString(strings.TrimSpace(r.FormValue("gender"))), raceID,
			parseNullableFloat(r.FormValue("height")), parseNullableFloat(r.FormValue("weight")), bodyTypeID,
			parseNullableFloat(r.FormValue("age")),
			// Typed text only survives while nothing is linked.
			nullableString(unlinkedText(r.FormValue("nation"), nationLoc)), nationLoc,
			parseIntDefault(r.FormValue("deaths"), 0),
			nullableString(desc),
			nullableString(bio),
			nullableString(speech),
			normalizeStatus(r.FormValue("status")),
			versionID,
		)
		if err != nil {
			tx.Rollback()
			http.Error(w, "failed to update story", http.StatusInternalServerError)
			log.Printf("update character_story: %v", err)
			return
		}

		// Birth date, blood type and birthplace belong to the character, so
		// saving them from any version changes them for all. The version
		// form no longer sends them (the version list edits them, see
		// charactershared.go): only those sent are changed.
		sent := func(k string) bool { _, ok := r.Form[k]; return ok }
		var sets []string
		var args []any
		if sent("birth_date") {
			sets = append(sets, "birth_date = ?")
			args = append(args, nullableString(normalizeStoryDate(r.FormValue("birth_date"))))
		}
		if sent("blood_type") {
			sets = append(sets, "blood_type = ?")
			args = append(args, nullableString(strings.TrimSpace(r.FormValue("blood_type"))))
		}
		if sent("human_birth_date") {
			sets = append(sets, "human_birth_date = ?")
			args = append(args, nullableString(normalizeStoryDate(r.FormValue("human_birth_date"))))
		}
		if sent("born_in") || sent("born_in_location_id") {
			sets = append(sets, "born_in = ?", "born_in_location_id = ?")
			args = append(args, nullableString(unlinkedText(r.FormValue("born_in"), bornInLoc)), bornInLoc)
		}
		if len(sets) > 0 {
			if _, err := tx.Exec(`UPDATE characters SET `+strings.Join(sets, ", ")+`
				WHERE id = (SELECT character_id FROM character_versions WHERE id = ?)`, append(args, versionID)...); err != nil {
				tx.Rollback()
				http.Error(w, "failed to update character", http.StatusInternalServerError)
				log.Printf("update characters: %v", err)
				return
			}
		}

		if err := saveSpecialBases(tx, versionID, sys, typed); err != nil {
			tx.Rollback()
			http.Error(w, "failed to update special stat bases", http.StatusInternalServerError)
			log.Printf("save special bases: %v", err)
			return
		}

		// spell_ids is a JSON array. If the field isn't in the request at
		// all, the spell list is left exactly as it was.
		if raw := r.Form["spell_ids"]; len(raw) > 0 {
			var spellIDs []int64
			if err := json.Unmarshal([]byte(raw[0]), &spellIDs); err != nil {
				tx.Rollback()
				http.Error(w, "invalid spell list", http.StatusBadRequest)
				return
			}
			vid, _ := strconv.ParseInt(versionID, 10, 64)
			if err := replaceVersionSpells(tx, vid, spellIDs); err != nil {
				tx.Rollback()
				http.Error(w, "failed to update spells", http.StatusInternalServerError)
				log.Printf("replace version spells: %v", err)
				return
			}
		}

		// gear is a JSON object of equipment slot -> gear id. Like spell_ids,
		// leaving it out leaves the outfit exactly as it was.
		if raw := r.Form["gear"]; len(raw) > 0 {
			var equipped map[string]int64
			if err := json.Unmarshal([]byte(raw[0]), &equipped); err != nil {
				tx.Rollback()
				http.Error(w, "invalid gear list", http.StatusBadRequest)
				return
			}
			vid, _ := strconv.ParseInt(versionID, 10, 64)
			if err := replaceVersionGear(tx, vid, equipped); err != nil {
				tx.Rollback()
				http.Error(w, "failed to update gear", http.StatusInternalServerError)
				log.Printf("replace version gear: %v", err)
				return
			}
		}

		// tags is a JSON list; they are the character's (see characterTags).
		if raw := r.Form["tags"]; len(raw) > 0 {
			var list []string
			if err := json.Unmarshal([]byte(raw[0]), &list); err != nil {
				tx.Rollback()
				http.Error(w, "invalid tag list", http.StatusBadRequest)
				return
			}
			tags, msg := cleanTags(list)
			if msg != "" {
				tx.Rollback()
				http.Error(w, msg, http.StatusBadRequest)
				return
			}
			var characterID int64
			if err := tx.QueryRow(`SELECT character_id FROM character_versions WHERE id = ?`, versionID).Scan(&characterID); err != nil ||
				replaceCharacterTags(tx, characterID, tags) != nil {
				tx.Rollback()
				http.Error(w, "failed to save changes", http.StatusInternalServerError)
				return
			}
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to save changes", http.StatusInternalServerError)
			return
		}

		if file, header, err := r.FormFile("picture"); err == nil {
			defer file.Close()
			var oldPath sql.NullString
			db.QueryRow(`SELECT picture_path FROM character_versions WHERE id = ?`, versionID).Scan(&oldPath)
			relPath, saveErr := savePicture(uploadsDir, versionID, header.Filename, file)
			if saveErr != nil {
				log.Printf("savePicture: %v", saveErr)
			} else if _, err := db.Exec(
				`UPDATE character_versions SET picture_path = ? WHERE id = ?`,
				relPath, versionID,
			); err != nil {
				log.Printf("update picture_path: %v", err)
			} else if oldPath.Valid && oldPath.String != relPath {
				removePicture(uploadsDir, oldPath.String)
			}
		}

		detail, err := buildVersionDetail(db, versionID)
		if err != nil {
			http.Error(w, "saved, but failed to reload version", http.StatusInternalServerError)
			return
		}
		writeJSON(w, detail)
	}
}

// deleteVersionHandler refuses to delete a character's only version — that
// operation is "delete the character" instead, from the Characters grid.
// Deleting the current version promotes the most recently created
// remaining version to current automatically.
func deleteVersionHandler(db *sql.DB, uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		versionID := r.PathValue("id")

		var characterID int64
		var isCurrent bool
		var picturePath sql.NullString
		if err := db.QueryRow(
			`SELECT character_id, is_current, picture_path FROM character_versions WHERE id = ?`,
			versionID,
		).Scan(&characterID, &isCurrent, &picturePath); err != nil {
			http.Error(w, "version not found", http.StatusNotFound)
			return
		}

		var versionCount int
		if err := db.QueryRow(
			`SELECT COUNT(*) FROM character_versions WHERE character_id = ?`, characterID,
		).Scan(&versionCount); err != nil {
			http.Error(w, "failed to check version count", http.StatusInternalServerError)
			return
		}
		if versionCount <= 1 {
			http.Error(w, "can't delete a character's only version — delete the character instead", http.StatusBadRequest)
			return
		}

		if _, err := db.Exec(`DELETE FROM character_versions WHERE id = ?`, versionID); err != nil {
			http.Error(w, "failed to delete version", http.StatusInternalServerError)
			log.Printf("delete version: %v", err)
			return
		}
		if picturePath.Valid {
			removePicture(uploadsDir, picturePath.String)
		}

		if isCurrent {
			var promoteID int64
			err := db.QueryRow(
				`SELECT id FROM character_versions WHERE character_id = ? ORDER BY created_at DESC LIMIT 1`,
				characterID,
			).Scan(&promoteID)
			if err == nil {
				db.Exec(`UPDATE character_versions SET is_current = 1 WHERE id = ?`, promoteID)
			}
		}

		writeJSON(w, map[string]any{"deleted": versionID})
	}
}

func setCurrentVersionHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		versionID := r.PathValue("id")

		var characterID int64
		if err := db.QueryRow(
			`SELECT character_id FROM character_versions WHERE id = ?`, versionID,
		).Scan(&characterID); err != nil {
			http.Error(w, "version not found", http.StatusNotFound)
			return
		}

		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to start transaction", http.StatusInternalServerError)
			return
		}
		if _, err := tx.Exec(
			`UPDATE character_versions SET is_current = 0 WHERE character_id = ?`, characterID,
		); err != nil {
			tx.Rollback()
			http.Error(w, "failed to update current version", http.StatusInternalServerError)
			return
		}
		if _, err := tx.Exec(
			`UPDATE character_versions SET is_current = 1 WHERE id = ?`, versionID,
		); err != nil {
			tx.Rollback()
			http.Error(w, "failed to update current version", http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to save changes", http.StatusInternalServerError)
			return
		}

		writeJSON(w, map[string]any{"is_current": versionID})
	}
}

// parseLocationID reads a posted location id. Blank, unparsable or
// unknown ids all mean "no link". When kingdomOnly is set the location
// must be a kingdom (a colored major location) — that's what Nation is.
func parseLocationID(db *sql.DB, raw string, kingdomOnly bool) sql.NullInt64 {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return sql.NullInt64{}
	}
	q := `SELECT 1 FROM locations WHERE id = ?`
	if kingdomOnly {
		q += ` AND kind = 'major' AND color IS NOT NULL`
	}
	var ok int
	if db.QueryRow(q, id).Scan(&ok) != nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: id, Valid: true}
}

// unlinkedText keeps legacy free text only while no location is linked.
func unlinkedText(text string, link sql.NullInt64) string {
	if link.Valid {
		return ""
	}
	return strings.TrimSpace(text)
}

// normalizeStatus keeps a version's status to the three the schema allows;
// anything else (including nothing, from an older form) means alive.
func normalizeStatus(s string) string {
	switch s = strings.TrimSpace(strings.ToLower(s)); s {
	case "missing", "dead":
		return s
	}
	return "alive"
}

// chapterParam turns an optional JSON chapter id into the form value
// validChapter reads.
func chapterParam(id *int64) string {
	if id == nil {
		return ""
	}
	return strconv.FormatInt(*id, 10)
}
