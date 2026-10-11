package main

import (
	"database/sql"
	"strconv"
	"strings"
)

// Schema 21 fixes a bug: specializations had no unique (class, name), so
// "INSERT OR IGNORE" in upsertClassScopedLookup never ignored anything and
// every save of a version with a specialization made another copy. This
// merges the copies into the oldest one before the unique index goes on:
// versions, spells, stat bonuses and wiki text that pointed at a copy move
// to the one that stays.

var schema21Statements = []string{
	`CREATE UNIQUE INDEX idx_specializations_class_name ON specializations(class_id, name COLLATE NOCASE)`,
}

type sqlStep struct {
	q    string
	args []any
}

func dedupeSpecializations(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT id, class_id, name FROM specializations ORDER BY id`)
	if err != nil {
		return err
	}
	keeper := map[string]int64{} // "class:name" -> oldest id
	type dup struct{ id, keep int64 }
	var dups []dup
	for rows.Next() {
		var id, class int64
		var name string
		if err := rows.Scan(&id, &class, &name); err != nil {
			rows.Close()
			return err
		}
		k := strconv.FormatInt(class, 10) + ":" + strings.ToLower(strings.TrimSpace(name))
		if keep, ok := keeper[k]; ok {
			dups = append(dups, dup{id, keep})
		} else {
			keeper[k] = id
		}
	}
	rows.Close()

	for _, d := range dups {
		var keeperBonuses, keeperEntry int
		tx.QueryRow(`SELECT COUNT(*) FROM stat_modifiers WHERE source_type = 'specialization' AND source_id = ?`, d.keep).Scan(&keeperBonuses)
		tx.QueryRow(`SELECT COUNT(*) FROM wiki_entries WHERE entity_type = 'specialization' AND entity_id = ?`, d.keep).Scan(&keeperEntry)
		steps := []sqlStep{
			{`UPDATE character_versions SET specialization_id = ? WHERE specialization_id = ?`, []any{d.keep, d.id}},
			{`UPDATE spells SET source_id = ? WHERE source_type = 'specialization' AND source_id = ?`, []any{d.keep, d.id}},
		}
		// A copy's own bonuses or wiki text survive only when the one that
		// stays has none.
		if keeperBonuses == 0 {
			steps = append(steps, sqlStep{`UPDATE stat_modifiers SET source_id = ? WHERE source_type = 'specialization' AND source_id = ?`, []any{d.keep, d.id}})
		}
		if keeperEntry == 0 {
			steps = append(steps, sqlStep{`UPDATE wiki_entries SET entity_id = ? WHERE entity_type = 'specialization' AND entity_id = ?`, []any{d.keep, d.id}})
		}
		steps = append(steps,
			sqlStep{`DELETE FROM stat_modifiers WHERE source_type = 'specialization' AND source_id = ?`, []any{d.id}},
			sqlStep{`DELETE FROM specializations WHERE id = ?`, []any{d.id}},
		)
		for _, s := range steps {
			if _, err := tx.Exec(s.q, s.args...); err != nil {
				return err
			}
		}
	}
	return nil
}
