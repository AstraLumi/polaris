package main

import (
	"strings"
	"testing"
)

// Saving versions with a specialization over and over used to make a new
// copy each time.
func TestSpecializationIsNotCopiedOnSave(t *testing.T) {
	mgr, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Specs"})
	body, ct := form(t, map[string]string{"name": "Aria", "level": "1", "class": "Mage", "specialization": "Spellslinger"}, "", "", nil)
	rec := do(t, h, "POST", "/api/s/"+s.ID+"/characters", body, ct)
	if rec.Code != 200 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	app, _ := mgr.open(s.ID)
	var vid int64
	app.db.QueryRow(`SELECT id FROM character_versions`).Scan(&vid)
	for _, spelled := range []string{"Spellslinger", "spellslinger ", "Spellslinger"} {
		body, ct := form(t, map[string]string{"name": "Aria", "level": "1", "class": "Mage", "specialization": spelled}, "", "", nil)
		if rec := do(t, h, "PUT", "/api/s/"+s.ID+"/versions/"+idStr(vid), body, ct); rec.Code != 200 {
			t.Fatalf("save: %d", rec.Code)
		}
	}
	var n int
	app.db.QueryRow(`SELECT COUNT(*) FROM specializations`).Scan(&n)
	if n != 1 {
		t.Errorf("%d specializations after saving three times, want 1", n)
	}
}

func TestMigrationMergesSpecializationCopies(t *testing.T) {
	mgr, _ := newTestServer(t)
	id := createStoryDirect(t, mgr, "Old")
	app, _ := mgr.open(id)
	rollBackSchema21(t, app.db)
	for _, q := range []string{
		`INSERT INTO classes (id, name) VALUES (1, 'Mage'), (2, 'Rogue')`,
		`INSERT INTO specializations (id, class_id, name) VALUES (1, 1, 'Spellslinger'), (2, 1, 'Spellslinger'), (3, 1, 'spellslinger'), (4, 2, 'Spellslinger')`,
		`INSERT INTO characters (id) VALUES (1)`,
		`INSERT INTO character_versions (id, character_id, is_current, name, specialization_id) VALUES (1, 1, 1, 'Aria', 3)`,
		`INSERT INTO spells (id, name, source_type, source_id) VALUES (1, 'Bolt', 'specialization', 2)`,
		`INSERT INTO stat_modifiers (source_type, source_id, target_stat, value) VALUES ('specialization', 2, 'int', 3), ('specialization', 3, 'int', 9)`,
		`INSERT INTO wiki_entries (entity_type, entity_id, summary) VALUES ('specialization', 3, 'Written on a copy')`,
	} {
		if _, err := app.db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if err := ensureSchema(app.db); err != nil {
		t.Fatal(err)
	}
	var specs, version, spell int64
	var bonuses []string
	app.db.QueryRow(`SELECT COUNT(*) FROM specializations`).Scan(&specs)
	app.db.QueryRow(`SELECT specialization_id FROM character_versions WHERE id = 1`).Scan(&version)
	app.db.QueryRow(`SELECT source_id FROM spells WHERE id = 1`).Scan(&spell)
	rows, _ := app.db.Query(`SELECT source_id || ':' || CAST(value AS INTEGER) FROM stat_modifiers WHERE source_type = 'specialization' ORDER BY id`)
	for rows.Next() {
		var s string
		rows.Scan(&s)
		bonuses = append(bonuses, s)
	}
	rows.Close()
	if specs != 2 || version != 1 || spell != 1 {
		t.Errorf("after merging: %d specializations, version on %d, spell on %d", specs, version, spell)
	}
	// The first copy's bonuses move over; the second copy's are dropped, the
	// one that stays having some by then. Its wiki text moves over.
	if strings.Join(bonuses, ",") != "1:3" {
		t.Errorf("bonuses: %v", bonuses)
	}
	var summary string
	app.db.QueryRow(`SELECT summary FROM wiki_entries WHERE entity_type = 'specialization' AND entity_id = 1`).Scan(&summary)
	if summary != "Written on a copy" {
		t.Errorf("wiki text: %q", summary)
	}
	if _, err := app.db.Exec(`INSERT INTO specializations (class_id, name) VALUES (1, 'SPELLSLINGER')`); err == nil {
		t.Error("the unique index should refuse another copy")
	}
}

// The infobox shows age, blood type and the human birth date when filled.
func TestWikiCharacterExtraFacts(t *testing.T) {
	_, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Facts"})
	body, ct := form(t, map[string]string{"name": "Aria", "level": "1"}, "", "", nil)
	do(t, h, "POST", "/api/s/"+s.ID+"/characters", body, ct)
	body, ct = form(t, map[string]string{"name": "Aria", "level": "1", "age": "27", "blood_type": "O-", "human_birth_date": "14-03-1996"}, "", "", nil)
	if rec := do(t, h, "PUT", "/api/s/"+s.ID+"/versions/1", body, ct); rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	a := wikiGet(t, h, s.ID, "character", 1)
	got := map[string]string{}
	for _, f := range a.Facts {
		got[f.Key] = f.Value
	}
	if got["age"] != "27" || got["blood_type"] != "O-" || got["human_birth_date"] != "14-03-1996" {
		t.Errorf("facts: %v", got)
	}
}
