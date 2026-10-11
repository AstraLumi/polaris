package main

import (
	"database/sql"
	"strings"
)

func upsertSimpleLookup(db *sql.DB, table, name string) (sql.NullInt64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return sql.NullInt64{}, nil
	}
	if _, err := db.Exec(`INSERT OR IGNORE INTO `+table+` (name) VALUES (?)`, name); err != nil {
		return sql.NullInt64{}, err
	}
	var id int64
	if err := db.QueryRow(`SELECT id FROM `+table+` WHERE name = ?`, name).Scan(&id); err != nil {
		return sql.NullInt64{}, err
	}
	return sql.NullInt64{Int64: id, Valid: true}, nil
}

func upsertClass(db *sql.DB, name string) (sql.NullInt64, error) {
	return upsertSimpleLookup(db, "classes", name)
}

func upsertRace(db *sql.DB, name string) (sql.NullInt64, error) {
	return upsertSimpleLookup(db, "races", name)
}

func upsertBodyType(db *sql.DB, name string) (sql.NullInt64, error) {
	return upsertSimpleLookup(db, "body_types", name)
}

// upsertClassScopedLookup handles specializations, which are "a name,
// scoped to exactly one class" — no class means no specialization to
// attach it to, so it's silently skipped rather than erroring. An existing
// one (any capitalisation) is reused: it looks first, rather than relying
// on INSERT OR IGNORE, which made a copy on every save before schema 21
// gave the table its unique index.
func upsertClassScopedLookup(db *sql.DB, table string, classID sql.NullInt64, name string) (sql.NullInt64, error) {
	name = strings.TrimSpace(name)
	if name == "" || !classID.Valid {
		return sql.NullInt64{}, nil
	}
	find := func() (int64, error) {
		var id int64
		err := db.QueryRow(
			`SELECT id FROM `+table+` WHERE class_id = ? AND name = ? COLLATE NOCASE ORDER BY id LIMIT 1`,
			classID.Int64, name,
		).Scan(&id)
		return id, err
	}
	id, err := find()
	if err == sql.ErrNoRows {
		if _, err := db.Exec(`INSERT OR IGNORE INTO `+table+` (class_id, name) VALUES (?, ?)`, classID.Int64, name); err != nil {
			return sql.NullInt64{}, err
		}
		id, err = find()
	}
	if err != nil {
		return sql.NullInt64{}, err
	}
	return sql.NullInt64{Int64: id, Valid: true}, nil
}

func upsertSpecialization(db *sql.DB, classID sql.NullInt64, name string) (sql.NullInt64, error) {
	return upsertClassScopedLookup(db, "specializations", classID, name)
}

// upsertSubclass finds-or-creates a subclass by name alone (subclasses are
// globally unique now, not scoped per class) and, if a class was also
// typed on the same form, links the two in subclass_classes — so a
// subclass typed fresh while a class is also set becomes restricted to
// that class going forward, without requiring a trip through the Character
// Assets page first.
func upsertSubclass(db *sql.DB, classID sql.NullInt64, name string) (sql.NullInt64, error) {
	id, err := upsertSimpleLookup(db, "subclasses", name)
	if err != nil || !id.Valid {
		return id, err
	}
	if classID.Valid {
		if _, err := db.Exec(
			`INSERT OR IGNORE INTO subclass_classes (subclass_id, class_id) VALUES (?, ?)`,
			id.Int64, classID.Int64,
		); err != nil {
			return id, err
		}
	}
	return id, nil
}
