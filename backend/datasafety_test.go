package main

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// closeStory forgets an open story, so the next request opens it afresh.
func closeStory(mgr *storyManager, id string) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	if a, ok := mgr.apps[id]; ok {
		a.graph.stop()
		a.db.Close()
		delete(mgr.apps, id)
	}
}

// Requests that write at the same moment wait for each other. Without the
// busy timeout and immediate transactions (sqliteDSN) most of these fail
// with "database is locked".
func TestConcurrentWritesWait(t *testing.T) {
	mgr, _ := newTestServer(t)
	app, err := mgr.open(createStoryDirect(t, mgr, "Busy"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`INSERT INTO classes (name) VALUES ('0')`); err != nil {
		t.Fatal(err)
	}
	const workers, rounds = 8, 25
	errs := make(chan error, workers*rounds)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range rounds {
				// Read, then write, as most saving handlers do.
				tx, err := app.db.Begin()
				if err != nil {
					errs <- err
					continue
				}
				var name string
				if err = tx.QueryRow(`SELECT name FROM classes`).Scan(&name); err == nil {
					n, _ := strconv.Atoi(name)
					_, err = tx.Exec(`UPDATE classes SET name = ?`, strconv.Itoa(n+1))
				}
				if err != nil {
					tx.Rollback()
					errs <- err
					continue
				}
				if err := tx.Commit(); err != nil {
					errs <- err
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("a write failed instead of waiting: %v", err)
	}
	var name string
	app.db.QueryRow(`SELECT name FROM classes`).Scan(&name)
	if name != strconv.Itoa(workers*rounds) {
		t.Errorf("every write counted once: %s", name)
	}
}

// A database that holds tables but has no version Polaris can upgrade from
// (here its version row is lost) is refused and left as it is. It used to
// be emptied and made anew.
func TestUnknownSchemaIsNotWiped(t *testing.T) {
	for _, lose := range []string{
		`DELETE FROM schema_meta`,
		`UPDATE schema_meta SET value = 'nonsense'`,
		`UPDATE schema_meta SET value = '1'`,
	} {
		mgr, h := newTestServer(t)
		id := createStoryDirect(t, mgr, "Lost")
		app, _ := mgr.open(id)
		app.db.Exec(`INSERT INTO classes (name) VALUES ('Keeper')`)
		if _, err := app.db.Exec(lose); err != nil {
			t.Fatal(err)
		}
		if err := ensureSchema(app.db); !errors.Is(err, errUnknownSchema) {
			t.Fatalf("%s: opening it: %v", lose, err)
		}
		closeStory(mgr, id)
		if rec := do(t, h, "GET", "/api/s/"+id+"/classes", nil, ""); rec.Code != 409 || !strings.Contains(rec.Body.String(), "untouched") {
			t.Errorf("%s: the API says why: %d %s", lose, rec.Code, rec.Body.String())
		}
		db := openRaw(t, mgr.dbPathFor(id))
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM classes`).Scan(&n); err != nil || n != 1 {
			t.Errorf("%s: the story's data must survive: %d %v", lose, n, err)
		}
		db.Close()
	}
}

// An upgrade does not start when its backup can't be saved; once it can,
// the story upgrades as usual.
func TestUpgradeNeedsItsBackup(t *testing.T) {
	mgr, h := newTestServer(t)
	id := createStoryDirect(t, mgr, "Old")
	app, _ := mgr.open(id)
	app.db.Exec(`INSERT INTO classes (name) VALUES ('Keeper')`)
	rollBackSchema28(t, app.db)
	closeStory(mgr, id)

	// A file where the backups folder goes: nothing can be saved in it.
	blocker := mgr.backupsDir(id)
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if rec := do(t, h, "GET", "/api/s/"+id+"/classes", nil, ""); rec.Code != 409 || !strings.Contains(rec.Body.String(), "backup") {
		t.Fatalf("no upgrade without a backup: %d %s", rec.Code, rec.Body.String())
	}
	db := openRaw(t, mgr.dbPathFor(id))
	if v := storedSchema(db); v != "27" {
		t.Errorf("the story stays at its schema: %s", v)
	}
	db.Close()

	if err := os.Remove(blocker); err != nil {
		t.Fatal(err)
	}
	if rec := do(t, h, "GET", "/api/s/"+id+"/classes", nil, ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), "Keeper") {
		t.Fatalf("it upgrades once the backup can be saved: %d %s", rec.Code, rec.Body.String())
	}
	if backups, _ := filepath.Glob(filepath.Join(mgr.backupsDir(id), "*-schema27-*.zip")); len(backups) != 1 {
		t.Errorf("backups: %v", backups)
	}
}

// An export arrives whole, with its length, and only then counts as one.
func TestExportIsSentWhole(t *testing.T) {
	mgr, h := newTestServer(t)
	id := createStoryDirect(t, mgr, "Whole")
	rec := do(t, h, "GET", "/api/stories/"+id+"/export", nil, "")
	if rec.Code != 200 || rec.Header().Get("Content-Length") != strconv.Itoa(rec.Body.Len()) || rec.Body.Len() == 0 {
		t.Fatalf("export: %d, length %q of %d", rec.Code, rec.Header().Get("Content-Length"), rec.Body.Len())
	}
	if si, _, _ := mgr.get(id); si.LastExportedAt == "" {
		t.Error("the export is remembered")
	}
	if left, _ := filepath.Glob(filepath.Join(mgr.tmpDir(), "export-*")); len(left) != 0 {
		t.Errorf("nothing is left behind: %v", left)
	}
}

// openRaw opens a story's database file as it is, upgrading nothing.
func openRaw(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	return db
}
