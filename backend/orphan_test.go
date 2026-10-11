package main

import "testing"

// Older databases can hold rows whose version, character or location is long
// gone: foreign keys were only enforced on some connections, so deletes
// didn't always cascade. Upgrading must step over them instead of failing.
func TestMigrationsSkipLeftoverRows(t *testing.T) {
	mgr, _ := newTestServer(t)
	app, _ := mgr.open(createStoryDirect(t, mgr, "Old"))
	app.db.SetMaxOpenConns(1)
	rollBackSchema22(t, app.db)
	for _, q := range []string{
		`PRAGMA foreign_keys = OFF`,
		`INSERT INTO characters (id) VALUES (1)`,
		`INSERT INTO character_versions (id, character_id, is_current, name) VALUES (1, 1, 1, 'Aria')`,
		`INSERT INTO character_story (version_id, birth_date, born_in_location_id) VALUES (1, '01-02-1500', 99), (77, '01-01-1400', NULL)`,
		`INSERT INTO character_build (version_id, vit) VALUES (1, 3), (77, 5)`,
		`PRAGMA foreign_keys = ON`,
	} {
		if _, err := app.db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if err := ensureSchema(app.db); err != nil {
		t.Fatalf("upgrading: %v", err)
	}
	d, err := buildVersionDetail(app.db, "1")
	if err != nil {
		t.Fatal(err)
	}
	if d.Build["vit"] != 3 || d.Story.BirthDate != "01-02-1500" || d.Story.BornInLocationID != nil {
		t.Errorf("after upgrading: vit %v, birth %q, born in %v", d.Build["vit"], d.Story.BirthDate, d.Story.BornInLocationID)
	}
}

// Every connection to a story database enforces foreign keys, so deleting a
// version always takes its rows with it.
func TestForeignKeysOnEveryConnection(t *testing.T) {
	mgr, _ := newTestServer(t)
	app, _ := mgr.open(createStoryDirect(t, mgr, "Keys"))
	app.db.SetMaxOpenConns(4)
	conns := make([]interface{ Close() error }, 0, 4)
	for i := 0; i < 4; i++ {
		c, err := app.db.Conn(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		var on int
		c.QueryRowContext(t.Context(), `PRAGMA foreign_keys`).Scan(&on)
		if on != 1 {
			t.Errorf("connection %d has foreign keys off", i)
		}
		conns = append(conns, c)
	}
	for _, c := range conns {
		c.Close()
	}
}
