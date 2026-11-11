package main

import (
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
)

// The index audit: a story far larger than anyone has written, and how long
// every read takes on it. Not part of the usual run (it takes a while and
// its numbers are for reading, not for passing):
//
//	POLARIS_AUDIT=1 go test -count=1 -run TestAudit -v .
//
// A read that stands out at the top of the list is either missing an index
// (indexes.go; TestAuditForeignKeys lists the foreign keys without one) or
// asks the database once per row of a list. POLARIS_AUDIT_INDEXES takes
// CREATE INDEX statements, separated by ";", to try before the measuring.
const (
	nChars     = 3000
	nVersions  = 3 // per character
	nLocs      = 1500
	nEvents    = 10000
	nChapters  = 1000
	nLore      = 2000
	nFactions  = 300
	nSpells    = 2000
	nGear      = 1500
	nCreatures = 800
)

func seedBig(t *testing.T, db *sql.DB) {
	t.Helper()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	ex := func(q string, args ...any) {
		if _, err := tx.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	ex(`PRAGMA defer_foreign_keys = ON`)
	for i := 1; i <= 20; i++ {
		ex(`INSERT INTO classes (name) VALUES (?)`, fmt.Sprintf("Class %d", i))
		ex(`INSERT INTO races (name) VALUES (?)`, fmt.Sprintf("Race %d", i))
		ex(`INSERT INTO subclasses (name) VALUES (?)`, fmt.Sprintf("Sub %d", i))
		ex(`INSERT INTO subclass_classes (subclass_id, class_id) VALUES (?, ?)`, i, i)
		ex(`INSERT INTO specializations (class_id, name) VALUES (?, ?)`, i, fmt.Sprintf("Spec %d", i))
		ex(`INSERT INTO body_types (name) VALUES (?)`, fmt.Sprintf("Body %d", i))
		ex(`INSERT INTO volumes (title, position) VALUES (?, ?)`, fmt.Sprintf("Volume %d", i), i)
	}
	for i := 1; i <= nLocs; i++ {
		kind := "minor"
		var parent any
		if i <= 40 {
			kind = "major"
		} else {
			parent = i%40 + 1
		}
		ex(`INSERT INTO locations (name, kind, belongs_to_id, description, q, r) VALUES (?, ?, ?, ?, ?, ?)`, fmt.Sprintf("Place %d", i), kind, parent, "A place of [[Person 5]] and [[Place 7]].", i%200, i/200)
	}
	for i := 1; i <= 40; i++ {
		for k := 0; k < 300; k++ {
			ex(`INSERT OR IGNORE INTO location_hexes (location_id, q, r) VALUES (?, ?, ?)`, i, (i*17+k)%400, (i*31+k/20)%400)
		}
	}
	for i := 1; i <= nChapters; i++ {
		ex(`INSERT INTO chapters (volume_id, title, notes, position) VALUES (?, ?, ?, ?)`, i%20+1, fmt.Sprintf("Chapter %d", i), "Notes about [[Person 3]].", i)
		ex(`INSERT INTO chapter_texts (chapter_id, doc, version, words) VALUES (?, ?, 1, 3000)`, i, `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"`+strings.Repeat("word ", 3000)+`"}]}]}`)
		for k := 0; k < 30; k++ {
			ex(`INSERT OR IGNORE INTO chapter_links (chapter_id, type, item_id, count) VALUES (?, 'character', ?, 2)`, i, (i*7+k*13)%nChars+1)
			ex(`INSERT OR IGNORE INTO chapter_lines (chapter_id, position, character_id, speaker, text) VALUES (?, ?, ?, 'x', 'A line said.')`, i, k, (i*7+k*13)%nChars+1)
		}
		for k := 0; k < 5; k++ {
			ex(`INSERT OR IGNORE INTO chapter_links (chapter_id, type, item_id, count) VALUES (?, 'location', ?, 1)`, i, (i*3+k)%nLocs+1)
		}
		ex(`INSERT INTO chapter_comments (chapter_id, text) VALUES (?, 'A comment')`, i)
	}
	v := 0
	for i := 1; i <= nChars; i++ {
		ex(`INSERT INTO characters (born_in_location_id, birth_date) VALUES (?, '01-01-1000')`, i%nLocs+1)
		ex(`INSERT INTO character_tags (character_id, tag) VALUES (?, ?)`, i, fmt.Sprintf("tag%d", i%30))
		for k := 0; k < nVersions; k++ {
			v++
			ex(`INSERT INTO character_versions (character_id, is_current, name, level, class_id, subclass_id, specialization_id, chapter_id, version_date) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				i, k == nVersions-1, fmt.Sprintf("Person %d", i), k*10, i%20+1, i%20+1, i%20+1, (i+k*50)%nChapters+1, fmt.Sprintf("01-01-%04d", 1000+k))
			ex(`INSERT INTO character_story (version_id, race_id, body_type_id, nation_location_id, description, bio, status) VALUES (?, ?, ?, ?, ?, ?, 'alive')`, v, i%20+1, i%20+1, i%40+1, "Knows [[Person 9]].", "Born in [[Place 4]].")
			for s := 0; s < 6; s++ {
				ex(`INSERT INTO character_spells (version_id, spell_id) VALUES (?, ?)`, v, (v+s*37)%nSpells+1)
			}
			for s := 0; s < 5; s++ {
				ex(`INSERT INTO character_gear (version_id, slot, gear_id) VALUES (?, ?, ?)`, v, fmt.Sprintf("slot%d", s), (v+s*41)%nGear+1)
			}
			for s := 0; s < 6; s++ {
				ex(`INSERT INTO character_special_bases (version_id, stat_key, value) VALUES (?, ?, 10)`, v, fmt.Sprintf("k%d", s))
			}
		}
		for k := 1; k <= 4; k++ {
			ex(`INSERT INTO character_relations (from_id, to_id, kind, label) VALUES (?, ?, ?, 'x')`, i, (i+k*97)%nChars+1, []string{"parent", "friend", "rival", "spouse"}[k-1])
		}
		ex(`INSERT INTO faction_members (faction_id, character_id, role) VALUES (?, ?, 'member')`, i%nFactions+1, i)
	}
	for i := 1; i <= nFactions; i++ {
		var parent any
		if i > 10 {
			parent = i%10 + 1
		}
		ex(`INSERT INTO factions (name, description, hq_location_id, parent_id) VALUES (?, 'Led by [[Person 2]].', ?, ?)`, fmt.Sprintf("Faction %d", i), i%nLocs+1, parent)
	}
	for i := 1; i <= nEvents; i++ {
		ex(`INSERT INTO events (name, description, event_date, location_id, chapter_id) VALUES (?, 'With [[Person 8]].', ?, ?, ?)`, fmt.Sprintf("Event %d", i), fmt.Sprintf("%02d-%02d-%04d", i%28+1, i%12+1, 1000+i%50), i%nLocs+1, i%nChapters+1)
		for k := 0; k < 4; k++ {
			ex(`INSERT OR IGNORE INTO event_characters (event_id, character_id) VALUES (?, ?)`, i, (i*11+k*101)%nChars+1)
		}
		ex(`INSERT INTO event_tags (event_id, tag) VALUES (?, ?)`, i, fmt.Sprintf("tag%d", i%40))
	}
	for i := 1; i <= nSpells; i++ {
		ex(`INSERT INTO spells (name, level, source_type, source_id, origin_location_id, description) VALUES (?, ?, 'class', ?, ?, 'Made by [[Person 1]].')`, fmt.Sprintf("Spell %d", i), i%50, i%20+1, i%nLocs+1)
		ex(`INSERT INTO stat_modifiers (source_type, source_id, target_stat, scaling, value) VALUES ('class', ?, 'str', 'flat', 2)`, i)
	}
	for i := 1; i <= nGear; i++ {
		ex(`INSERT INTO gear (name, slot, description) VALUES (?, ?, 'Worn by [[Person 6]].')`, fmt.Sprintf("Gear %d", i), fmt.Sprintf("slot%d", i%5))
		for k := 0; k < 3; k++ {
			ex(`INSERT INTO stat_modifiers (source_type, source_id, target_stat, scaling, value) VALUES ('item', ?, ?, 'flat', 2)`, i, []string{"str", "dex", "int"}[k])
		}
	}
	for i := 1; i <= nCreatures; i++ {
		ex(`INSERT INTO creatures (name, region_id, description) VALUES (?, ?, 'Hunted by [[Person 4]].')`, fmt.Sprintf("Beast %d", i), i%40+1)
		if i < nCreatures {
			ex(`INSERT INTO creature_links (a_id, b_id) VALUES (?, ?)`, i, i+1)
		}
	}
	for i := 1; i <= nLore; i++ {
		ex(`INSERT INTO lore_articles (name) VALUES (?)`, fmt.Sprintf("Lore %d", i))
	}
	entry := 0
	wiki := func(kind string, n int) {
		for i := 1; i <= n; i++ {
			entry++
			ex(`INSERT INTO wiki_entries (entity_type, entity_id, summary, fields) VALUES (?, ?, 'About [[Person 12]] and [[Place 3]].', '{}')`, kind, i)
			for k := 0; k < 3; k++ {
				ex(`INSERT INTO wiki_sections (entry_id, position, title, body) VALUES (?, ?, 'History', ?)`, entry, k, fmt.Sprintf("It began with [[Person %d]] at [[Place %d]] and [[Lore %d]]. %s", (i+k)%nChars+1, (i+k)%nLocs+1, (i+k)%nLore+1, strings.Repeat("More is told here. ", 40)))
			}
			ex(`INSERT INTO wiki_infobox (entry_id, position, label, value) VALUES (?, 0, 'Seat', '[[Place 2]]')`, entry)
			if i%5 == 0 {
				ex(`INSERT INTO wiki_gallery (entry_id, position, file_path) VALUES (?, 0, '/uploads/x.png')`, entry)
			}
		}
	}
	wiki("character", nChars)
	wiki("location", nLocs)
	wiki("lore", nLore)
	wiki("event", 2000)
	wiki("faction", nFactions)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	db.Exec(`ANALYZE`)
}

func TestAuditTimings(t *testing.T) {
	if os.Getenv("POLARIS_AUDIT") == "" {
		t.Skip()
	}
	mgr, h := newTestServer(t)
	id := createStoryDirect(t, mgr, "Big")
	app, err := mgr.open(id)
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("POLARIS_AUDIT_INDEXES") != "" {
		for _, q := range strings.Split(os.Getenv("POLARIS_AUDIT_INDEXES"), ";") {
			if strings.TrimSpace(q) == "" {
				continue
			}
			if _, err := app.db.Exec(q); err != nil {
				t.Fatalf("%s: %v", q, err)
			}
		}
	}
	start := time.Now()
	seedBig(t, app.db)
	fi, _ := os.Stat(app.dbPath)
	t.Logf("seeded in %v, %d MB", time.Since(start).Round(time.Millisecond), fi.Size()>>20)

	urls := []string{
		"/home", "/characters", "/character-options", "/characters/heights", "/characters/1500/versions", "/characters/1500/current",
		"/characters/1500/connections", "/characters/1500/family", "/characters/1500/lines", "/versions/4500",
		"/chapters", "/chapters/500", "/chapters/500/text", "/chapters/500/comments", "/chapters/500/speakers", "/chapters/500/text/versions",
		"/events", "/events/5000", "/event-tags", "/events?q=Event%2099", "/events?tag=tag3", "/events?chapter_id=500",
		"/timeline", "/factions", "/factions/150", "/creatures", "/creatures/400",
		"/locations", "/location-options", "/map", "/classes", "/classes/5", "/races", "/races/5", "/subclasses", "/specializations", "/body-types",
		"/spells", "/spells/1000", "/gear", "/gear/700", "/assets/summary",
		"/wiki", "/wiki/character/1500", "/wiki/location/700", "/wiki/lore/1000", "/wiki/event/1000", "/wiki/faction/150", "/wiki/graph",
		"/appears/character/1500", "/appears/location/700", "/stat-system", "/settings", "/books/5", "/books/5/docs", "/solar", "/planet",
	}
	type res struct {
		url  string
		ms   float64
		code int
		kb   int
	}
	var out []res
	for _, u := range urls {
		best := time.Hour
		var code, size int
		for range 3 {
			s := time.Now()
			rec := do(t, h, "GET", "/api/s/"+id+u, nil, "")
			if d := time.Since(s); d < best {
				best = d
			}
			code, size = rec.Code, rec.Body.Len()
		}
		out = append(out, res{u, float64(best.Microseconds()) / 1000, code, size >> 10})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ms > out[j].ms })
	var b strings.Builder
	for _, r := range out {
		fmt.Fprintf(&b, "%9.1f ms  %d  %6d KB  %s\n", r.ms, r.code, r.kb, r.url)
	}
	// Deletes: what a parent's removal costs.
	for _, d := range []string{"/map/locations/900", "/chapters/900", "/spells/1900", "/gear/1400", "/events/9000"} {
		s := time.Now()
		rec := do(t, h, "DELETE", "/api/s/"+id+d, nil, "")
		fmt.Fprintf(&b, "%9.1f ms  %d  DELETE %s\n", float64(time.Since(s).Microseconds())/1000, rec.Code, d)
	}
	t.Log("\n" + b.String())
}

// The foreign-key columns no index starts with: deleting the parent row
// reads the whole table. Worth an index on the tables that grow.
func TestAuditForeignKeys(t *testing.T) {
	if os.Getenv("POLARIS_AUDIT") == "" {
		t.Skip()
	}
	mgr, _ := newTestServer(t)
	app, err := mgr.open(createStoryDirect(t, mgr, "Audit"))
	if err != nil {
		t.Fatal(err)
	}
	db := app.db
	var tables []string
	rows, _ := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	for rows.Next() {
		var n string
		rows.Scan(&n)
		tables = append(tables, n)
	}
	rows.Close()
	var out []string
	for _, tb := range tables {
		// leading columns of every index (and the primary key)
		lead := map[string]bool{}
		ix, _ := db.Query(`SELECT name FROM pragma_index_list(?)`, tb)
		var names []string
		for ix.Next() {
			var n string
			ix.Scan(&n)
			names = append(names, n)
		}
		ix.Close()
		for _, n := range names {
			var col *string
			db.QueryRow(`SELECT name FROM pragma_index_info(?) WHERE seqno = 0`, n).Scan(&col)
			if col != nil {
				lead[*col] = true
			}
		}
		// rowid alias primary key
		var pkCols []string
		ti, _ := db.Query(`SELECT name, pk FROM pragma_table_info(?)`, tb)
		for ti.Next() {
			var n string
			var pk int
			ti.Scan(&n, &pk)
			if pk == 1 {
				pkCols = append(pkCols, n)
			}
		}
		ti.Close()
		for _, c := range pkCols {
			lead[c] = true
		}
		fk, _ := db.Query(`SELECT "from", "table", on_delete FROM pragma_foreign_key_list(?)`, tb)
		for fk.Next() {
			var from, to, od string
			fk.Scan(&from, &to, &od)
			if !lead[from] {
				out = append(out, fmt.Sprintf("%-28s %-22s -> %-20s %s", tb, from, to, od))
			}
		}
		fk.Close()
	}
	sort.Strings(out)
	t.Logf("\n%d foreign keys without an index:\n%s", len(out), strings.Join(out, "\n"))
}
