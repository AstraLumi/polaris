package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// A version's status is saved, copied into new versions, and anything
// unknown falls back to alive.
func TestCharacterStatus(t *testing.T) {
	h, s, vid := gearTestSetup(t)
	status := func(id int64) string {
		rec := do(t, h, "GET", "/api/s/"+s.ID+"/versions/"+idStr(id), nil, "")
		var v struct {
			Story struct {
				Status string `json:"status"`
			} `json:"story"`
		}
		json.Unmarshal(rec.Body.Bytes(), &v)
		return v.Story.Status
	}
	save := func(value string) {
		body, ct := form(t, map[string]string{"name": "Aria", "level": "10", "status": value}, "", "", nil)
		if rec := do(t, h, "PUT", "/api/s/"+s.ID+"/versions/"+idStr(vid), body, ct); rec.Code != 200 {
			t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
		}
	}

	if got := status(vid); got != "alive" {
		t.Fatalf("new character should be alive, got %q", got)
	}
	save("missing")
	if got := status(vid); got != "missing" {
		t.Fatalf("got %q, want missing", got)
	}
	save("undead")
	if got := status(vid); got != "alive" {
		t.Fatalf("unknown status should save as alive, got %q", got)
	}

	save("dead")
	rec := do(t, h, "GET", "/api/s/"+s.ID+"/versions/"+idStr(vid), nil, "")
	var cur struct {
		CharacterID int64 `json:"character_id"`
	}
	json.Unmarshal(rec.Body.Bytes(), &cur)
	rec = do(t, h, "POST", "/api/s/"+s.ID+"/characters/"+idStr(cur.CharacterID)+"/versions",
		strings.NewReader(`{"version_date":"","version_reference":"","clone_from_version_id":`+idStr(vid)+`}`), "application/json")
	var nv struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(rec.Body.Bytes(), &nv)
	if got := status(nv.ID); got != "dead" {
		t.Errorf("a new version should start with the same status, got %q", got)
	}

	rec = do(t, h, "GET", "/api/s/"+s.ID+"/characters", nil, "")
	if !strings.Contains(rec.Body.String(), `"status":"dead"`) {
		t.Errorf("character list should carry the status: %s", rec.Body.String())
	}
}

func TestMigrationAddsCharacterStatus(t *testing.T) {
	mgr, _ := newTestServer(t)
	id := createStoryDirect(t, mgr, "Old")
	app, _ := mgr.open(id)
	rollBackSchema16(t, app.db)
	for _, q := range []string{
		`INSERT INTO characters (id) VALUES (1)`,
		`INSERT INTO character_versions (id, character_id, is_current, name) VALUES (1, 1, 1, 'Bram')`,
		`INSERT INTO character_story (version_id, deaths) VALUES (1, 2)`,
		`ALTER TABLE character_story DROP COLUMN status`,
		`UPDATE schema_meta SET value = '14' WHERE key = 'schema_version'`,
	} {
		if _, err := app.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err := ensureSchema(app.db); err != nil {
		t.Fatal(err)
	}
	var status string
	var deaths int
	if err := app.db.QueryRow(`SELECT status, deaths FROM character_story WHERE version_id = 1`).Scan(&status, &deaths); err != nil {
		t.Fatalf("status column missing after migration: %v", err)
	}
	if status != "alive" || deaths != 2 {
		t.Errorf("existing character should become alive and keep its deaths: %s %d", status, deaths)
	}
	var v string
	app.db.QueryRow(`SELECT value FROM schema_meta WHERE key = 'schema_version'`).Scan(&v)
	if v != currentSchemaVersion {
		t.Fatalf("version %s", v)
	}
}
