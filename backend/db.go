package main

import (
	"database/sql"
	"fmt"
	"log"
)

// Bump this whenever the schema changes. If there's an entry in
// `migrations` for the version a database is currently at, initSchema
// upgrades it in place and keeps the data. Only a database older than the
// oldest migration (or a brand new one) gets wiped and recreated from
// freshSchema — so every schema change from here on should ship with a
// migration instead of relying on that reset.
const currentSchemaVersion = "20"

const freshSchema = `
CREATE TABLE classes (
	id        INTEGER PRIMARY KEY AUTOINCREMENT,
	name      TEXT NOT NULL UNIQUE,
	icon_path TEXT
);

CREATE TABLE subclasses (
	id   INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE
);

-- A subclass can be tied to zero, one, or several classes. Zero means
-- "not restricted" (shows up as an option regardless of the character's
-- class); one or more means it only shows up when the character's class
-- matches one of these.
CREATE TABLE subclass_classes (
	subclass_id INTEGER NOT NULL REFERENCES subclasses(id) ON DELETE CASCADE,
	class_id    INTEGER NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
	PRIMARY KEY (subclass_id, class_id)
);

CREATE TABLE specializations (
	id       INTEGER PRIMARY KEY AUTOINCREMENT,
	class_id INTEGER NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
	name     TEXT NOT NULL
);

CREATE TABLE races (
	id   INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE
);

CREATE TABLE body_types (
	id   INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE
);

-- A character is just an anchor: an identity that its versions hang off of.
CREATE TABLE characters (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Everything that can change over the course of the story — name, level,
-- class, picture, and (via character_story/character_build) their full
-- sheet — lives on a version, not the character. Exactly one version per
-- character has is_current = 1; that's what the Characters grid shows.
CREATE TABLE character_versions (
	id                 INTEGER PRIMARY KEY AUTOINCREMENT,
	character_id       INTEGER NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
	is_current         INTEGER NOT NULL DEFAULT 0,
	version_date       TEXT,  -- free-text story-timeline date, e.g. "15-03-1024"
	version_reference  TEXT,  -- free-text, e.g. "Vol. 2 - Ch. 5 - Pg. 12"
	name               TEXT NOT NULL,
	nickname           TEXT,
	level              INTEGER NOT NULL DEFAULT 1,
	class_id           INTEGER REFERENCES classes(id) ON DELETE SET NULL,
	subclass_id        INTEGER REFERENCES subclasses(id) ON DELETE SET NULL,
	specialization_id  INTEGER REFERENCES specializations(id) ON DELETE SET NULL,
	picture_path       TEXT,
	created_at         TEXT NOT NULL DEFAULT (datetime('now')),
	updated_at         TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_character_versions_character_id ON character_versions(character_id);

-- Only one current version per character, enforced at the database level.
CREATE UNIQUE INDEX idx_one_current_version_per_character
	ON character_versions(character_id)
	WHERE is_current = 1;

CREATE TABLE character_story (
	version_id        INTEGER PRIMARY KEY REFERENCES character_versions(id) ON DELETE CASCADE,
	gender            TEXT,
	race_id           INTEGER REFERENCES races(id) ON DELETE SET NULL,
	height            REAL,  -- cm
	weight            REAL,  -- kg
	body_type_id      INTEGER REFERENCES body_types(id) ON DELETE SET NULL,
	age               REAL,  -- drives the Luck special stat
	human_birth_date  TEXT,
	blood_type        TEXT,
	-- born_in / nation hold the pre-9 free text, and only while it could
	-- not be matched to a map location; once linked the text is cleared
	-- and the *_location_id link wins.
	born_in           TEXT,
	nation            TEXT,
	born_in_location_id INTEGER REFERENCES locations(id) ON DELETE SET NULL,
	nation_location_id  INTEGER REFERENCES locations(id) ON DELETE SET NULL,
	birth_date        TEXT,
	deaths            INTEGER NOT NULL DEFAULT 0,
	description       TEXT,
	bio               TEXT,
	speech_mannerisms TEXT,
	-- Alive, missing or dead at this point in the story. Separate from
	-- deaths, which counts lives lost (a character can die and come back).
	status            TEXT NOT NULL DEFAULT 'alive' CHECK (status IN ('alive', 'missing', 'dead'))
);

-- The only editable combat data: the 8 primary stats. Everything else
-- (Base Stats, Special Defenses) is calculated from these plus
-- stat_modifiers, never stored directly.
CREATE TABLE character_build (
	version_id INTEGER PRIMARY KEY REFERENCES character_versions(id) ON DELETE CASCADE,
	vit   INTEGER NOT NULL DEFAULT 0,
	def   INTEGER NOT NULL DEFAULT 0,
	res   INTEGER NOT NULL DEFAULT 0,
	str   INTEGER NOT NULL DEFAULT 0,
	dex   INTEGER NOT NULL DEFAULT 0,
	intel INTEGER NOT NULL DEFAULT 0,
	wis   INTEGER NOT NULL DEFAULT 0,
	agl   INTEGER NOT NULL DEFAULT 0
);

-- Every "type a number, see it combined with bonuses" special stat
-- (Sanity, Heat/Cold Threshold, each Special Defense, each Lifeskill, and
-- whatever gets added after those) stores its typed base here instead of
-- as a dedicated column. stat_key matches the target_stat name used in
-- stat_modifiers, so a new special stat is just a new key — no schema
-- change needed to add one.
CREATE TABLE character_special_bases (
	version_id INTEGER NOT NULL REFERENCES character_versions(id) ON DELETE CASCADE,
	stat_key   TEXT NOT NULL,
	value      REAL NOT NULL DEFAULT 0,
	PRIMARY KEY (version_id, stat_key)
);

-- Groundwork for "Class X gives +10 Attack per level" style effects.
-- Empty for now — every computed stat just falls back to its 1:1 base
-- value until rows get added here (eventually from a future Character
-- Assets admin UI, or — for 'item' — an equipment system that doesn't
-- exist yet). target_stat is free text on purpose: it'll eventually cover
-- luck, lifeskills, fall damage, carry weight, etc., not just the combat
-- stats that exist today. 'multiplier' modifiers are summed as a percentage
-- delta and applied once at the end (two +10% rows = +20%, not compounded).
CREATE TABLE stat_modifiers (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	source_type TEXT NOT NULL CHECK (source_type IN ('class', 'subclass', 'specialization', 'race', 'body_type', 'item')),
	source_id   INTEGER NOT NULL,
	target_stat TEXT NOT NULL,
	scaling     TEXT NOT NULL DEFAULT 'flat' CHECK (scaling IN ('flat', 'per_level', 'multiplier')),
	value       REAL NOT NULL DEFAULT 0,
	created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Locations are the Map page's data. A 'major' location (a kingdom, or a
-- colorless region) claims hexes via location_hexes; a 'minor' location
-- sits on one hex (q, r). belongs_to_id is the manual "belongs to" override
-- for a minor location — when NULL, it belongs to whichever colored major
-- location owns its hex.
CREATE TABLE locations (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	name          TEXT NOT NULL UNIQUE,
	description   TEXT,
	picture_path  TEXT,
	kind          TEXT NOT NULL DEFAULT 'minor',
	color         TEXT,
	founding_date TEXT,
	q             INTEGER,
	r             INTEGER,
	belongs_to_id INTEGER REFERENCES locations(id) ON DELETE SET NULL,
	is_city       INTEGER NOT NULL DEFAULT 0,
	is_capital    INTEGER NOT NULL DEFAULT 0
);

` + eventsTableSQL + `;

` + eventsSourceIndexSQL + `;

` + eventTagsTableSQL + `;

` + eventCharactersTableSQL + `;

` + spellsTableSQL + `;

` + characterSpellsTableSQL + `;

` + locationHexesTableSQL + `;

` + locationHexesIndexSQL + `;

` + locationColorIndexSQL + `;

` + appSettingsTableSQL + `;

` + gearTableSQL + `;

` + characterGearTableSQL + `;
`

// Which hexes a major location covers. A hex can belong to any number of
// colorless major locations but at most one colored one (the kingdom) —
// that rule lives in the paint handler, since SQL can't express it here.
const locationHexesTableSQL = `CREATE TABLE location_hexes (
	location_id INTEGER NOT NULL REFERENCES locations(id) ON DELETE CASCADE,
	q           INTEGER NOT NULL,
	r           INTEGER NOT NULL,
	PRIMARY KEY (location_id, q, r)
)`

const locationHexesIndexSQL = `CREATE INDEX idx_location_hexes_qr ON location_hexes(q, r)`

// No two kingdoms share a color, or the map couldn't tell them apart.
const locationColorIndexSQL = `CREATE UNIQUE INDEX idx_locations_major_color ON locations(color) WHERE kind = 'major' AND color IS NOT NULL`

// Spells and the per-version spell list are defined once here so a fresh
// install and the 6 -> 7 migration can never drift apart.
//
// source_type/source_id point at a class, subclass, or specialization —
// polymorphic, so there's no foreign key; deleting one of those clears
// the reference explicitly (see assets.go). mp_cost/hp_cost are NULL when
// not set; a spell with neither is displayed as "Free".
const spellsTableSQL = `CREATE TABLE spells (
	id                 INTEGER PRIMARY KEY AUTOINCREMENT,
	name               TEXT NOT NULL,
	level              INTEGER NOT NULL DEFAULT 1,
	icon_path          TEXT,
	source_type        TEXT CHECK (source_type IN ('class', 'subclass', 'specialization')),
	source_id          INTEGER,
	mp_cost            INTEGER,
	hp_cost            INTEGER,
	origin_location_id INTEGER REFERENCES locations(id) ON DELETE SET NULL,
	description        TEXT,
	created_at         TEXT NOT NULL DEFAULT (datetime('now'))
)`

// Which spells a given version of a character knows. Per-version like the
// rest of the sheet, so a character can learn spells over the story.
const characterSpellsTableSQL = `CREATE TABLE character_spells (
	version_id INTEGER NOT NULL REFERENCES character_versions(id) ON DELETE CASCADE,
	spell_id   INTEGER NOT NULL REFERENCES spells(id) ON DELETE CASCADE,
	PRIMARY KEY (version_id, spell_id)
)`

// Events: single-date happenings that populate the timeline. An event
// whose source_type/source_id are set is the "details" record for a node
// the timeline generates on its own (a character's birth, a location's
// founding); its name and date are read live from that source rather than
// stored, so renaming or redating the source can never leave it stale.
// Tags are the one linking mechanism between events (a war's start and
// end share a tag); tag uniqueness is case-insensitive.
const eventsTableSQL = `CREATE TABLE events (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	name         TEXT NOT NULL,
	description  TEXT,
	event_date   TEXT NOT NULL DEFAULT '',
	location_id  INTEGER REFERENCES locations(id) ON DELETE SET NULL,
	picture_path TEXT,
	source_type  TEXT CHECK (source_type IN ('character', 'location')),
	source_id    INTEGER,
	created_at   TEXT NOT NULL DEFAULT (datetime('now')),
	updated_at   TEXT NOT NULL DEFAULT (datetime('now'))
)`

const eventsSourceIndexSQL = `CREATE UNIQUE INDEX idx_events_source ON events(source_type, source_id) WHERE source_type IS NOT NULL`

const eventTagsTableSQL = `CREATE TABLE event_tags (
	event_id INTEGER NOT NULL REFERENCES events(id) ON DELETE CASCADE,
	tag      TEXT NOT NULL COLLATE NOCASE,
	PRIMARY KEY (event_id, tag)
)`

const eventCharactersTableSQL = `CREATE TABLE event_characters (
	event_id     INTEGER NOT NULL REFERENCES events(id) ON DELETE CASCADE,
	character_id INTEGER NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
	PRIMARY KEY (event_id, character_id)
)`

type migration struct {
	to         string
	statements []string
	// after runs once the statements are in, for data changes SQL alone
	// can't make (optional).
	after func(*sql.DB) error
}

// migrations upgrade a database in place, one version at a time, keeping
// its data. The key is the version being upgraded FROM.
var migrations = map[string]migration{
	"6": {
		to: "7",
		statements: []string{
			// The old spells table was a placeholder nothing ever wrote
			// to, so it's safe to replace outright.
			`DROP TABLE IF EXISTS spells`,
			spellsTableSQL,
			characterSpellsTableSQL,
			// Locations weren't unique by name before; the Origin combo
			// box needs find-or-create-by-name to work.
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_locations_name ON locations(name)`,
		},
	},
	"7": {
		to: "8",
		statements: []string{
			// Existing locations (created through a spell's Origin) keep
			// their rows and become minor locations that aren't on a hex
			// yet — the 'minor' default is what makes that happen.
			`ALTER TABLE locations ADD COLUMN kind TEXT NOT NULL DEFAULT 'minor'`,
			`ALTER TABLE locations ADD COLUMN color TEXT`,
			`ALTER TABLE locations ADD COLUMN founding_date TEXT`,
			`ALTER TABLE locations ADD COLUMN q INTEGER`,
			`ALTER TABLE locations ADD COLUMN r INTEGER`,
			`ALTER TABLE locations ADD COLUMN belongs_to_id INTEGER REFERENCES locations(id) ON DELETE SET NULL`,
			locationHexesTableSQL,
			locationHexesIndexSQL,
			locationColorIndexSQL,
		},
	},
	"8": {
		to: "9",
		statements: []string{
			appSettingsTableSQL,
			// Born in / Nation become links to Map locations.
			`ALTER TABLE character_story ADD COLUMN born_in_location_id INTEGER REFERENCES locations(id) ON DELETE SET NULL`,
			`ALTER TABLE character_story ADD COLUMN nation_location_id INTEGER REFERENCES locations(id) ON DELETE SET NULL`,
			// Link whatever old text already names a location (case-
			// insensitive; location names are unique). Nation can only be
			// a kingdom, i.e. a colored major location.
			`UPDATE character_story SET born_in_location_id = (
				SELECT l.id FROM locations l WHERE lower(l.name) = lower(trim(character_story.born_in))
			) WHERE born_in IS NOT NULL AND trim(born_in) != ''`,
			`UPDATE character_story SET nation_location_id = (
				SELECT l.id FROM locations l
				WHERE lower(l.name) = lower(trim(character_story.nation))
				  AND l.kind = 'major' AND l.color IS NOT NULL
			) WHERE nation IS NOT NULL AND trim(nation) != ''`,
			// Matched text is now redundant; unmatched text stays so the
			// editor can flag it for a manual pick.
			`UPDATE character_story SET born_in = NULL WHERE born_in_location_id IS NOT NULL`,
			`UPDATE character_story SET nation = NULL WHERE nation_location_id IS NOT NULL`,
		},
	},
	"9": {
		to: "10",
		statements: []string{
			// The two placeholder tables were never wired to anything, but
			// carry over whatever rows they hold rather than assume.
			`ALTER TABLE events RENAME TO events_old`,
			eventsTableSQL,
			eventsSourceIndexSQL,
			eventTagsTableSQL,
			eventCharactersTableSQL,
			`INSERT INTO events (name, description, location_id, event_date)
			 SELECT name, description, location_id, COALESCE(event_date, '') FROM events_old`,
			`INSERT INTO events (name, description, event_date)
			 SELECT title, description, COALESCE(event_date, '') FROM timeline_events`,
			`DROP TABLE events_old`,
			`DROP TABLE timeline_events`,
		},
	},
	"10": {
		to: "11",
		statements: []string{
			// Classes can have an uploaded icon.
			`ALTER TABLE classes ADD COLUMN icon_path TEXT`,
		},
	},
	"11": {
		to: "12",
		statements: []string{
			// Gear and what each character version has equipped.
			gearTableSQL,
			characterGearTableSQL,
		},
	},
	"12": {
		to: "13",
		// The wiki's own tables and the triggers that clean them up when a
		// source (character, location, ...) is deleted. Only the article
		// kinds that existed at schema 13: later ones add their own trigger.
		statements: wikiSchemaFor(wikiTypesAtSchema13...),
	},
	"13": {
		to: "14",
		statements: []string{
			// Minor locations can be tagged as a city and/or a capital, which
			// changes how the map draws them.
			`ALTER TABLE locations ADD COLUMN is_city INTEGER NOT NULL DEFAULT 0`,
			`ALTER TABLE locations ADD COLUMN is_capital INTEGER NOT NULL DEFAULT 0`,
		},
	},
	"14": {
		to: "15",
		statements: []string{
			// Each version says whether the character is alive, missing or dead.
			`ALTER TABLE character_story ADD COLUMN status TEXT NOT NULL DEFAULT 'alive' CHECK (status IN ('alive', 'missing', 'dead'))`,
		},
	},
	"15": {
		to: "16",
		// Relations between characters, factions, and chapters (in volumes)
		// that events and character versions can point to.
		statements: append(append([]string{}, worldSchemaStatements...), wikiTriggerSQL("faction")),
	},
	"16": {
		to: "17",
		// Free-form lore articles in the wiki (lore.go).
		statements: append(append([]string{}, loreSchemaStatements...), wikiTriggerSQL("lore")),
	},
	"17": {
		to: "18",
		// Tags on characters, and dated infobox rows on lore articles.
		statements: schema18Statements,
	},
	"18": {
		to: "19",
		// Picture galleries on wiki articles (wikigallery.go).
		statements: schema19Statements,
	},
	"19": {
		to: "20",
		// Relations can start and end at a chapter; kingdoms get a faction.
		statements: schema20Statements,
		after:      backfillKingdomFactions,
	},
}

// schema20Statements create what schema 20 added. The fresh schema runs them
// after every other table exists.
var schema20Statements = []string{
	`ALTER TABLE character_relations ADD COLUMN since_chapter_id INTEGER`,
	`ALTER TABLE character_relations ADD COLUMN until_chapter_id INTEGER`,
	`CREATE TRIGGER relations_chapter_unlink AFTER DELETE ON chapters BEGIN
		UPDATE character_relations SET since_chapter_id = NULL WHERE since_chapter_id = OLD.id;
		UPDATE character_relations SET until_chapter_id = NULL WHERE until_chapter_id = OLD.id;
	END`,
	// The kingdom (a coloured major location) a faction stands for; see kingdoms.go.
	`ALTER TABLE factions ADD COLUMN kingdom_id INTEGER`,
	`CREATE UNIQUE INDEX idx_factions_kingdom ON factions(kingdom_id) WHERE kingdom_id IS NOT NULL`,
	`CREATE TRIGGER factions_kingdom_unlink AFTER DELETE ON locations BEGIN
		UPDATE factions SET kingdom_id = NULL WHERE kingdom_id = OLD.id;
	END`,
}

const characterTagsTableSQL = `CREATE TABLE character_tags (
	character_id INTEGER NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
	tag          TEXT NOT NULL COLLATE NOCASE,
	PRIMARY KEY (character_id, tag)
)`

// schema18Statements create what schema 18 added. The fresh schema runs them
// after the wiki's tables, which the second one changes.
var schema18Statements = []string{
	characterTagsTableSQL,
	// "" for a plain row, "date" for a lore article's dated row (it shows on
	// the timeline).
	`ALTER TABLE wiki_infobox ADD COLUMN kind TEXT NOT NULL DEFAULT ''`,
}

// worldSchemaStatements create what schema 16 added. They run after the
// fresh schema too, so both paths end up with the same tables.
var worldSchemaStatements = []string{
	`ALTER TABLE events ADD COLUMN chapter_id INTEGER`,
	`ALTER TABLE character_versions ADD COLUMN chapter_id INTEGER`,
	volumesTableSQL,
	chaptersTableSQL,
	chaptersUnlinkTriggerSQL,
	characterRelationsTableSQL,
	`CREATE INDEX idx_relations_from ON character_relations(from_id)`,
	`CREATE INDEX idx_relations_to ON character_relations(to_id)`,
	factionsTableSQL,
	factionMembersTableSQL,
	`CREATE INDEX idx_faction_members_faction ON faction_members(faction_id)`,
	`CREATE INDEX idx_faction_members_character ON faction_members(character_id)`,
}

func applyMigration(db *sql.DB, m migration) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	for _, stmt := range m.statements {
		if _, err := tx.Exec(stmt); err != nil {
			tx.Rollback()
			return err
		}
	}
	if _, err := tx.Exec(
		`INSERT INTO schema_meta (key, value) VALUES ('schema_version', ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		m.to,
	); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// tablesInDependencyOrder lists every app-owned table, children before
// parents, so dropping them in this order never trips a foreign key.
var tablesInDependencyOrder = []string{
	"character_tags",
	"lore_articles",
	"faction_members",
	"factions",
	"character_relations",
	"chapters",
	"volumes",
	"wiki_gallery",
	"wiki_infobox",
	"wiki_sections",
	"wiki_entries",
	"event_characters",
	"event_tags",
	"stat_modifiers",
	"character_gear",
	"character_spells",
	"character_special_bases",
	"character_build",
	"character_story",
	"character_versions",
	"characters",
	"subclass_classes",
	"specializations",
	"subclasses",
	"classes",
	"races",
	"body_types",
	"spells",
	"gear",
	"events",
	"timeline_events",
	"location_hexes",
	"locations",
	"app_settings",
}

// initSchema is the startup entry point for a database that must work: any
// failure is fatal. Story databases are opened through ensureSchema instead,
// so one broken story can't take the whole server down.
func initSchema(db *sql.DB) {
	if err := ensureSchema(db); err != nil {
		log.Fatalf("schema setup failed: %v", err)
	}
}

// canMigrate reports whether a database at `version` can be upgraded in
// place to the current schema without losing data.
func canMigrate(version string) bool {
	for i := 0; i < 100; i++ {
		if version == currentSchemaVersion {
			return true
		}
		m, ok := migrations[version]
		if !ok {
			return false
		}
		version = m.to
	}
	return false
}

// ensureSchema brings db to currentSchemaVersion: migrating in place when it
// can, creating the fresh schema when the database is empty or too old to
// migrate.
func ensureSchema(db *sql.DB) error {
	if _, err := db.Exec(
		`CREATE TABLE IF NOT EXISTS schema_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
	); err != nil {
		return fmt.Errorf("create schema_meta: %w", err)
	}

	var version string
	err := db.QueryRow(
		`SELECT value FROM schema_meta WHERE key = 'schema_version'`,
	).Scan(&version)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("read schema version: %w", err)
	}

	// Walk any available migrations forward — these keep existing data.
	for version != currentSchemaVersion {
		m, ok := migrations[version]
		if !ok {
			break
		}
		log.Printf("migrating schema %s -> %s", version, m.to)
		if err := applyMigration(db, m); err != nil {
			return fmt.Errorf("migration %s -> %s: %w", version, m.to, err)
		}
		if m.after != nil {
			if err := m.after(db); err != nil {
				return fmt.Errorf("migration %s -> %s: %w", version, m.to, err)
			}
		}
		version = m.to
	}

	if version == currentSchemaVersion {
		return nil
	}

	// No migration path from here (brand new database, or one older than
	// the oldest migration) — start fresh.
	log.Printf("no migration path from schema %q to %q — creating fresh tables", version, currentSchemaVersion)

	for _, table := range tablesInDependencyOrder {
		if _, err := db.Exec("DROP TABLE IF EXISTS " + table); err != nil {
			return fmt.Errorf("drop table %s: %w", table, err)
		}
	}

	if _, err := db.Exec(freshSchema); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	for _, stmt := range append(append([]string{}, worldSchemaStatements...), loreSchemaStatements...) {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("create schema: %w", err)
		}
	}
	for _, stmt := range append(append(append(append([]string{}, wikiSchemaStatements...), schema18Statements...), schema19Statements...), schema20Statements...) {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("create wiki schema: %w", err)
		}
	}

	if _, err := db.Exec(
		`INSERT INTO schema_meta (key, value) VALUES ('schema_version', ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		currentSchemaVersion,
	); err != nil {
		return fmt.Errorf("record schema version: %w", err)
	}

	log.Printf("schema reset complete, now at version %s", currentSchemaVersion)
	return nil
}
