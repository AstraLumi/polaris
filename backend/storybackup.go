package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// Before a story is opened at a schema other than this version's, it is
// saved as an export zip in its folder's backups/ directory: ahead of an
// upgrade (the copy an older Polaris could still open), or when it was saved
// by a newer Polaris and is refused (errNewerSchema). One backup per schema
// version, so a refused story isn't copied again on every request. A backup
// is an ordinary export: Import Story restores it as a new story.

func (m *storyManager) backupsDir(id string) string { return filepath.Join(m.storyDir(id), "backups") }

// storedSchema is the schema version saved in db, "" for a new database.
func storedSchema(db *sql.DB) string {
	var v string
	db.QueryRow(`SELECT value FROM schema_meta WHERE key = 'schema_version'`).Scan(&v)
	return v
}

// backupBeforeOpen writes the backup when db is at another schema version and
// there isn't one for that version yet. It answers whether one exists now.
func (m *storyManager) backupBeforeOpen(id string, db *sql.DB, uploadsDir string) bool {
	version := storedSchema(db)
	if version == "" || version == currentSchemaVersion {
		return false
	}
	dir := m.backupsDir(id)
	if found, _ := filepath.Glob(filepath.Join(dir, fmt.Sprintf("*-schema%s-*.zip", version))); len(found) > 0 {
		return true
	}
	name, icon := id, ""
	m.reg.QueryRow(`SELECT name, COALESCE(icon, '') FROM stories WHERE id = ?`, id).Scan(&name, &icon)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("backup %s: %v", id, err)
		return false
	}
	path := filepath.Join(dir, fmt.Sprintf("polaris-%s-schema%s-%s.zip", slugify(name), version, time.Now().Format("2006-01-02-150405")))
	f, err := os.Create(path)
	if err != nil {
		log.Printf("backup %s: %v", id, err)
		return false
	}
	err = m.writeStoryZip(f, db, uploadsDir, name, icon, version)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(path)
		log.Printf("backup %s: %v", id, err)
		return false
	}
	log.Printf("story %s is at schema %s (this version: %s); backed up to %s", id, version, currentSchemaVersion, path)
	return true
}
