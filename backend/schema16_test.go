package main

import (
	"database/sql"
	"testing"
)

// rollBackSchema37 turns a fresh database back into schema 36.
func rollBackSchema37(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, q := range []string{
		`ALTER TABLE celestial_bodies DROP COLUMN inclination`,
		`ALTER TABLE celestial_bodies DROP COLUMN ascending_node`,
		`UPDATE schema_meta SET value = '36' WHERE key = 'schema_version'`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
}

// rollBackSchema36 turns a fresh database back into schema 35.
func rollBackSchema36(t *testing.T, db *sql.DB) {
	t.Helper()
	rollBackSchema37(t, db)
	for _, q := range []string{
		`ALTER TABLE celestial_bodies DROP COLUMN ring_inner`,
		`ALTER TABLE celestial_bodies DROP COLUMN ring_outer`,
		`ALTER TABLE celestial_bodies DROP COLUMN ring_color`,
		`ALTER TABLE celestial_bodies DROP COLUMN dyson`,
		`UPDATE schema_meta SET value = '35' WHERE key = 'schema_version'`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
}

// rollBackSchema35 turns a fresh database back into schema 34.
func rollBackSchema35(t *testing.T, db *sql.DB) {
	t.Helper()
	rollBackSchema36(t, db)
	for _, q := range []string{
		`ALTER TABLE celestial_bodies DROP COLUMN axial_tilt`,
		`ALTER TABLE celestial_bodies DROP COLUMN day_hours`,
		`UPDATE schema_meta SET value = '34' WHERE key = 'schema_version'`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
}

// rollBackSchema34 turns a fresh database back into schema 33.
func rollBackSchema34(t *testing.T, db *sql.DB) {
	t.Helper()
	rollBackSchema35(t, db)
	for _, q := range []string{
		`DROP TRIGGER wiki_cleanup_map_features`,
		`DROP TABLE feature_cells`,
		`DROP TABLE map_features`,
		`UPDATE schema_meta SET value = '33' WHERE key = 'schema_version'`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
}

// rollBackSchema33 turns a fresh database back into schema 32.
func rollBackSchema33(t *testing.T, db *sql.DB) {
	t.Helper()
	rollBackSchema34(t, db)
	for _, q := range []string{
		`ALTER TABLE celestial_bodies DROP COLUMN eccentricity`,
		`ALTER TABLE celestial_bodies DROP COLUMN periapsis`,
		`UPDATE schema_meta SET value = '32' WHERE key = 'schema_version'`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
}

// rollBackSchema32 turns a fresh database back into schema 31.
func rollBackSchema32(t *testing.T, db *sql.DB) {
	t.Helper()
	rollBackSchema33(t, db)
	for _, q := range []string{
		`DROP TRIGGER wiki_cleanup_celestial_bodies`,
		`DROP TRIGGER wiki_cleanup_solar_systems`,
		`DROP TABLE body_cells`,
		`DROP TABLE solar_systems`,
		`ALTER TABLE celestial_bodies DROP COLUMN map_frequency`,
		`UPDATE schema_meta SET value = '31' WHERE key = 'schema_version'`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
}

// rollBackSchema31 turns a fresh database back into schema 30.
func rollBackSchema31(t *testing.T, db *sql.DB) {
	t.Helper()
	rollBackSchema32(t, db)
	for _, q := range []string{
		`ALTER TABLE celestial_bodies DROP COLUMN star_class`,
		`ALTER TABLE celestial_bodies DROP COLUMN temperature_k`,
		`ALTER TABLE celestial_bodies DROP COLUMN mass_kg`,
		`ALTER TABLE celestial_bodies DROP COLUMN free_values`,
		`UPDATE schema_meta SET value = '30' WHERE key = 'schema_version'`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
}

// rollBackSchema30 turns a fresh database back into schema 29.
func rollBackSchema30(t *testing.T, db *sql.DB) {
	t.Helper()
	rollBackSchema31(t, db)
	for _, q := range []string{
		`DROP TABLE celestial_bodies`,
		`UPDATE schema_meta SET value = '29' WHERE key = 'schema_version'`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
}

// rollBackSchema29 turns a fresh database back into schema 28.
func rollBackSchema29(t *testing.T, db *sql.DB) {
	t.Helper()
	rollBackSchema30(t, db)
	for _, q := range []string{
		`ALTER TABLE character_story ADD COLUMN human_birth_date TEXT`,
		`ALTER TABLE characters DROP COLUMN human_birth_date`,
		`ALTER TABLE characters DROP COLUMN earth_name`,
		`UPDATE schema_meta SET value = '28' WHERE key = 'schema_version'`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
}

// rollBackSchema28 turns a fresh database back into schema 27. The realm
// check stays wide; widenRealms leaves it so.
func rollBackSchema28(t *testing.T, db *sql.DB) {
	t.Helper()
	rollBackSchema29(t, db)
	for _, q := range []string{
		`DROP INDEX idx_locations_major_color`,
		`CREATE UNIQUE INDEX idx_locations_major_color ON locations(color) WHERE kind = 'major' AND color IS NOT NULL`,
		`UPDATE schema_meta SET value = '27' WHERE key = 'schema_version'`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
}

// rollBackSchema27 turns a fresh database back into schema 26.
func rollBackSchema27(t *testing.T, db *sql.DB) {
	t.Helper()
	rollBackSchema28(t, db)
	for _, q := range []string{
		`DROP TABLE creature_cells`,
		`DROP TABLE planet_cells`,
		`ALTER TABLE locations DROP COLUMN cell`,
		`ALTER TABLE locations DROP COLUMN realm`,
		`UPDATE schema_meta SET value = '26' WHERE key = 'schema_version'`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
}

// rollBackSchema26 turns a fresh database back into schema 25.
func rollBackSchema26(t *testing.T, db *sql.DB) {
	t.Helper()
	rollBackSchema27(t, db)
	for _, q := range []string{
		`DROP TABLE location_climate`,
		`UPDATE schema_meta SET value = '25' WHERE key = 'schema_version'`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
}

// rollBackSchema25 turns a fresh database back into schema 24.
func rollBackSchema25(t *testing.T, db *sql.DB) {
	t.Helper()
	rollBackSchema26(t, db)
	for _, q := range []string{
		`DROP TRIGGER wiki_cleanup_creatures`,
		`DROP TABLE creature_links`,
		`DROP TABLE creature_hexes`,
		`DROP TABLE creatures`,
		`ALTER TABLE gear DROP COLUMN description`,
		`UPDATE schema_meta SET value = '24' WHERE key = 'schema_version'`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
}

// rollBackSchema24 turns a fresh database back into schema 23.
func rollBackSchema24(t *testing.T, db *sql.DB) {
	t.Helper()
	rollBackSchema25(t, db)
	for _, q := range []string{
		`ALTER TABLE stat_defs DROP COLUMN lower_is_better`,
		`UPDATE schema_meta SET value = '23' WHERE key = 'schema_version'`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
}

// rollBackSchema23 turns a fresh database back into schema 22.
func rollBackSchema23(t *testing.T, db *sql.DB) {
	t.Helper()
	rollBackSchema24(t, db)
	for _, q := range []string{
		`ALTER TABLE character_story ADD COLUMN blood_type TEXT`,
		`ALTER TABLE character_story ADD COLUMN born_in TEXT`,
		`ALTER TABLE character_story ADD COLUMN born_in_location_id INTEGER REFERENCES locations(id) ON DELETE SET NULL`,
		`ALTER TABLE character_story ADD COLUMN birth_date TEXT`,
		`ALTER TABLE characters DROP COLUMN birth_date`,
		`ALTER TABLE characters DROP COLUMN blood_type`,
		`ALTER TABLE characters DROP COLUMN born_in`,
		`ALTER TABLE characters DROP COLUMN born_in_location_id`,
		`ALTER TABLE races DROP COLUMN kingdom_id`,
		`ALTER TABLE specializations DROP COLUMN kingdom_id`,
		`UPDATE schema_meta SET value = '22' WHERE key = 'schema_version'`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
}

// rollBackSchema22 turns a fresh database back into schema 21.
func rollBackSchema22(t *testing.T, db *sql.DB) {
	t.Helper()
	rollBackSchema23(t, db)
	for _, q := range []string{
		`CREATE TABLE character_build (
			version_id INTEGER PRIMARY KEY REFERENCES character_versions(id) ON DELETE CASCADE,
			vit   INTEGER NOT NULL DEFAULT 0,
			def   INTEGER NOT NULL DEFAULT 0,
			res   INTEGER NOT NULL DEFAULT 0,
			str   INTEGER NOT NULL DEFAULT 0,
			dex   INTEGER NOT NULL DEFAULT 0,
			intel INTEGER NOT NULL DEFAULT 0,
			wis   INTEGER NOT NULL DEFAULT 0,
			agl   INTEGER NOT NULL DEFAULT 0
		)`,
		`INSERT INTO character_build (version_id) SELECT id FROM character_versions`,
		`DELETE FROM character_special_bases WHERE stat_key IN ('vit', 'def', 'res', 'str', 'dex', 'intel', 'wis', 'agl')`,
		`DROP TABLE stat_defs`,
		`DROP TABLE stat_settings`,
		`UPDATE schema_meta SET value = '21' WHERE key = 'schema_version'`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
}

// rollBackSchema21 turns a fresh database back into schema 20.
func rollBackSchema21(t *testing.T, db *sql.DB) {
	t.Helper()
	rollBackSchema22(t, db)
	for _, q := range []string{
		`DROP INDEX idx_specializations_class_name`,
		`UPDATE schema_meta SET value = '20' WHERE key = 'schema_version'`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
}

// rollBackSchema20 turns a fresh database back into schema 19.
func rollBackSchema20(t *testing.T, db *sql.DB) {
	t.Helper()
	rollBackSchema21(t, db)
	for _, q := range []string{
		`DROP TRIGGER relations_chapter_unlink`,
		`DROP TRIGGER factions_kingdom_unlink`,
		`DROP INDEX idx_factions_kingdom`,
		`ALTER TABLE character_relations DROP COLUMN since_chapter_id`,
		`ALTER TABLE character_relations DROP COLUMN until_chapter_id`,
		`ALTER TABLE factions DROP COLUMN kingdom_id`,
		`UPDATE schema_meta SET value = '19' WHERE key = 'schema_version'`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
}

// rollBackSchema19 turns a fresh database back into schema 18.
func rollBackSchema19(t *testing.T, db *sql.DB) {
	t.Helper()
	rollBackSchema20(t, db)
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
