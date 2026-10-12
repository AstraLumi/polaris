package main

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// A story saved by a newer Polaris is refused, not wiped: without the guard
// it would fall through to the fresh schema, which drops every table.
func TestNewerSchemaIsRefused(t *testing.T) {
	mgr, h := newTestServer(t)
	id := createStoryDirect(t, mgr, "Future")
	app, _ := mgr.open(id)
	app.db.Exec(`INSERT INTO classes (name) VALUES ('Keeper')`)
	app.db.Exec(`UPDATE schema_meta SET value = '999' WHERE key = 'schema_version'`)

	if err := ensureSchema(app.db); !errors.Is(err, errNewerSchema) {
		t.Fatalf("opening a newer story: %v", err)
	}
	var n int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM classes`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("the story's data must survive: %d %v", n, err)
	}
	var v string
	app.db.QueryRow(`SELECT value FROM schema_meta WHERE key = 'schema_version'`).Scan(&v)
	if v != "999" {
		t.Errorf("its version is left as it was: %s", v)
	}

	// Through the API, once the story is opened afresh.
	mgr.mu.Lock()
	app.db.Close()
	delete(mgr.apps, id)
	mgr.mu.Unlock()
	if rec := do(t, h, "GET", "/api/s/"+id+"/classes", nil, ""); rec.Code != 409 || !strings.Contains(rec.Body.String(), "backup") {
		t.Errorf("the API says why: %d %s", rec.Code, rec.Body.String())
	}
	// It was backed up once, as an ordinary export at its own schema.
	do(t, h, "GET", "/api/s/"+id+"/classes", nil, "")
	backups, _ := filepath.Glob(filepath.Join(mgr.backupsDir(id), "*.zip"))
	if len(backups) != 1 || !strings.Contains(backups[0], "-schema999-") {
		t.Fatalf("backups: %v", backups)
	}
	zr, err := zip.OpenReader(backups[0])
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	var man storyManifest
	for _, f := range zr.File {
		if f.Name == "polaris-story.json" {
			rc, _ := f.Open()
			json.NewDecoder(rc).Decode(&man)
			rc.Close()
		}
	}
	if man.SchemaVersion != "999" || man.Name != "Future" {
		t.Errorf("manifest: %+v", man)
	}

	if !newerSchema("999") || newerSchema(currentSchemaVersion) || newerSchema("3") || newerSchema("") {
		t.Error("newerSchema")
	}
}

// An older story is backed up before it is upgraded, once.
func TestBackupBeforeUpgrade(t *testing.T) {
	mgr, h := newTestServer(t)
	id := createStoryDirect(t, mgr, "Old")
	app, _ := mgr.open(id)
	rollBackSchema28(t, app.db)
	mgr.mu.Lock()
	app.db.Close()
	delete(mgr.apps, id)
	mgr.mu.Unlock()
	if rec := do(t, h, "GET", "/api/s/"+id+"/classes", nil, ""); rec.Code != 200 {
		t.Fatalf("an older story still opens: %d %s", rec.Code, rec.Body.String())
	}
	backups, _ := filepath.Glob(filepath.Join(mgr.backupsDir(id), "*-schema27-*.zip"))
	if len(backups) != 1 {
		t.Errorf("backups: %v", backups)
	}
}
