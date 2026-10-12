package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// The Earth birth date moves from the versions to the character, taken from
// the current version when it has one, else from another one.
func TestMigrationMovesEarthBirthDateToTheCharacter(t *testing.T) {
	mgr, _ := newTestServer(t)
	app, _ := mgr.open(createStoryDirect(t, mgr, "Old"))
	rollBackSchema29(t, app.db)
	for _, q := range []string{
		`INSERT INTO characters (id) VALUES (1)`,
		`INSERT INTO character_versions (id, character_id, is_current, name) VALUES (1, 1, 0, 'Young Aria'), (2, 1, 1, 'Aria')`,
		`INSERT INTO character_story (version_id, human_birth_date) VALUES (1, '14-03-1996'), (2, '')`,
	} {
		if _, err := app.db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if err := ensureSchema(app.db); err != nil {
		t.Fatal(err)
	}
	for _, vid := range []string{"1", "2"} {
		d, err := buildVersionDetail(app.db, vid)
		if err != nil {
			t.Fatal(err)
		}
		if d.Story.HumanBirthDate != "14-03-1996" || d.Story.EarthName != "" {
			t.Errorf("version %s: earth birth %q, earth name %q", vid, d.Story.HumanBirthDate, d.Story.EarthName)
		}
	}
}

// The Earth name and birth date are saved with the character, and the wiki
// shows the name.
func TestEarthNameAndBirthDate(t *testing.T) {
	mgr, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Isekai"})
	app, _ := mgr.open(s.ID)
	base := "/api/s/" + s.ID
	body, ct := form(t, map[string]string{"name": "Aria", "level": "1"}, "", "", nil)
	rec := do(t, h, "POST", base+"/characters", body, ct)
	var c struct {
		ID        int64 `json:"id"`
		VersionID int64 `json:"version_id"`
	}
	json.Unmarshal(rec.Body.Bytes(), &c)
	rec = do(t, h, "PUT", base+"/characters/"+idStr(c.ID)+"/shared",
		strings.NewReader(`{"human_birth_date":"14-03-1996","earth_name":" Aiko Tanaka "}`), "application/json")
	if rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	d, _ := buildVersionDetail(app.db, idStr(c.VersionID))
	if d.Story.HumanBirthDate != "14-03-1996" || d.Story.EarthName != "Aiko Tanaka" {
		t.Errorf("earth birth %q, earth name %q", d.Story.HumanBirthDate, d.Story.EarthName)
	}
	got := map[string]string{}
	for _, f := range wikiGet(t, h, s.ID, "character", c.ID).Facts {
		got[f.Key] = f.Value
	}
	if got["earth_name"] != "Aiko Tanaka" || got["human_birth_date"] != "14-03-1996" {
		t.Errorf("wiki facts: %v", got)
	}
}

// An Earth birthdate follows Earth's calendar, not the story's.
func TestEarthBirthDateUsesEarthsCalendar(t *testing.T) {
	_, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Short months"})
	base := "/api/s/" + s.ID
	body, ct := form(t, map[string]string{"name": "Aria", "level": "1"}, "", "", nil)
	do(t, h, "POST", base+"/characters", body, ct)
	if rec := do(t, h, "PUT", base+"/characters/1/shared", strings.NewReader(`{"human_birth_date":"31-12-1996"}`), "application/json"); rec.Code != 200 {
		t.Errorf("the 31st on Earth: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, h, "PUT", base+"/characters/1/shared", strings.NewReader(`{"human_birth_date":"01-13-1996"}`), "application/json"); rec.Code != 400 {
		t.Errorf("a 13th month on Earth: %d", rec.Code)
	}
}
