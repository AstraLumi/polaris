package main

import (
	"encoding/json"
	"strconv"
	"testing"
)

// The assets summary counts the characters (current versions) using each
// asset and lists its bonuses.
func TestAssetSummary(t *testing.T) {
	app, h, base := planetSetup(t)
	db := app.db
	exec := func(q string, args ...any) int64 {
		t.Helper()
		res, err := db.Exec(q, args...)
		if err != nil {
			t.Fatal(err)
		}
		id, _ := res.LastInsertId()
		return id
	}
	blade := exec(`INSERT INTO classes (name) VALUES ('Blade')`)
	elf := exec(`INSERT INTO races (name) VALUES ('Elf')`)
	exec(`INSERT INTO stat_modifiers (source_type, source_id, target_stat, value) VALUES ('class', ?, 'str', 3)`, blade)
	for _, current := range []int{1, 0} {
		char := exec(`INSERT INTO characters DEFAULT VALUES`)
		v := exec(`INSERT INTO character_versions (character_id, name, is_current, class_id) VALUES (?, 'X', ?, ?)`, char, current, blade)
		exec(`INSERT INTO character_story (version_id, race_id) VALUES (?, ?)`, v, elf)
	}
	rec := do(t, h, "GET", base+"/assets/summary", nil, "")
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	var out struct {
		Uses      map[string]map[string]int                `json:"uses"`
		Modifiers map[string]map[string]map[string]float64 `json:"modifiers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	id := strconv.FormatInt(blade, 10)
	if out.Uses["class"][id] != 1 || out.Uses["race"][strconv.FormatInt(elf, 10)] != 1 {
		t.Errorf("uses (current versions only): %+v", out.Uses)
	}
	if out.Modifiers["class"][id]["str"] != 3 {
		t.Errorf("modifiers: %+v", out.Modifiers)
	}
}
