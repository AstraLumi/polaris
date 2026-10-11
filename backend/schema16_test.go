package main

import (
	"database/sql"
	"testing"
)

// rollBackSchema19 turns a fresh database back into schema 18.
func rollBackSchema19(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, q := range []string{
		`DROP TABLE wiki_gallery`,
		`UPDATE schema_meta SET value = '18' WHERE key = 'schema_version'`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
}

// rollBackSchema18 turns a fresh database back into schema 17.
func rollBackSchema18(t *testing.T, db *sql.DB) {
	t.Helper()
	rollBackSchema19(t, db)
	for _, q := range []string{
		`DROP TABLE character_tags`,
		`ALTER TABLE wiki_infobox DROP COLUMN kind`,
		`UPDATE schema_meta SET value = '17' WHERE key = 'schema_version'`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
}

// rollBackSchema17 turns a fresh database back into schema 16.
func rollBackSchema17(t *testing.T, db *sql.DB) {
	t.Helper()
	rollBackSchema18(t, db)
	for _, q := range []string{
		`DROP TRIGGER wiki_cleanup_lore_articles`,
		`DROP TABLE lore_articles`,
		`UPDATE schema_meta SET value = '16' WHERE key = 'schema_version'`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
}

// rollBackSchema16 turns a fresh database back into schema 15, for the
// tests that start from an older schema and migrate up again. Triggers go
// first: SQLite won't drop a column a trigger mentions.
func rollBackSchema16(t *testing.T, db *sql.DB) {
	t.Helper()
	rollBackSchema17(t, db)
	for _, q := range []string{
		`DROP TRIGGER wiki_cleanup_factions`,
		`DROP TRIGGER chapters_unlink`,
		`DROP TABLE faction_members`,
		`DROP TABLE factions`,
		`DROP TABLE character_relations`,
		`DROP TABLE chapters`,
		`DROP TABLE volumes`,
		`ALTER TABLE events DROP COLUMN chapter_id`,
		`ALTER TABLE character_versions DROP COLUMN chapter_id`,
		`UPDATE schema_meta SET value = '15' WHERE key = 'schema_version'`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
}

func TestMigrationAddsRelationsFactionsAndChapters(t *testing.T) {
	mgr, _ := newTestServer(t)
	id := createStoryDirect(t, mgr, "Old")
	app, _ := mgr.open(id)
	rollBackSchema16(t, app.db)
	for _, q := range []string{
		`INSERT INTO characters (id) VALUES (1)`,
		`INSERT INTO character_versions (id, character_id, is_current, name) VALUES (1, 1, 1, 'Bram')`,
		`INSERT INTO events (id, name, event_date) VALUES (1, 'Battle', '01-01-1000')`,
	} {
		if _, err := app.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err := ensureSchema(app.db); err != nil {
		t.Fatal(err)
	}
	var v string
	app.db.QueryRow(`SELECT value FROM schema_meta WHERE key = 'schema_version'`).Scan(&v)
	if v != currentSchemaVersion {
		t.Fatalf("version %s", v)
	}

	// The new tables work, and old rows can point at a chapter.
	for _, q := range []string{
		`INSERT INTO characters (id) VALUES (2)`,
		`INSERT INTO character_relations (from_id, to_id, kind) VALUES (1, 2, 'sibling')`,
		`INSERT INTO factions (id, name) VALUES (1, 'The Order')`,
		`INSERT INTO faction_members (faction_id, character_id) VALUES (1, 1)`,
		`INSERT INTO volumes (id, title) VALUES (1, 'Book One')`,
		`INSERT INTO chapters (id, volume_id, title) VALUES (1, 1, 'Ashes')`,
		`UPDATE events SET chapter_id = 1 WHERE id = 1`,
		`UPDATE character_versions SET chapter_id = 1 WHERE id = 1`,
	} {
		if _, err := app.db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	// Deleting the chapter unlinks what pointed at it; deleting the faction
	// takes its wiki entry with it.
	app.db.Exec(`INSERT INTO wiki_entries (entity_type, entity_id, summary) VALUES ('faction', 1, 'x')`)
	app.db.Exec(`DELETE FROM chapters WHERE id = 1`)
	app.db.Exec(`DELETE FROM factions WHERE id = 1`)
	var linked, wiki, members int
	app.db.QueryRow(`SELECT COUNT(*) FROM events WHERE chapter_id IS NOT NULL`).Scan(&linked)
	app.db.QueryRow(`SELECT COUNT(*) FROM wiki_entries WHERE entity_type = 'faction'`).Scan(&wiki)
	app.db.QueryRow(`SELECT COUNT(*) FROM faction_members`).Scan(&members)
	if linked != 0 || wiki != 0 || members != 0 {
		t.Errorf("cleanup after delete: %d events still linked, %d wiki entries, %d members", linked, wiki, members)
	}
}
