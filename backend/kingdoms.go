package main

import (
	"database/sql"
	"log"
	"strings"
)

// Every kingdom (a major location with a colour) has a faction tied to it
// through factions.kingdom_id: same name and colour, headquartered in the
// kingdom. The map is where both are changed; the faction follows. A
// location that stops being a kingdom, or is deleted, leaves its faction in
// place but no longer tied (it may still matter to the story).

type kingdomInfo struct {
	ID    int64
	Name  string
	Color string
}

// kingdomOfFaction is the kingdom a faction is tied to, or nil.
func kingdomOfFaction(db *sql.DB, factionID int64) *kingdomInfo {
	var k kingdomInfo
	if db.QueryRow(`SELECT l.id, l.name, COALESCE(l.color, '') FROM factions f JOIN locations l ON l.id = f.kingdom_id
		WHERE f.id = ?`, factionID).Scan(&k.ID, &k.Name, &k.Color) != nil {
		return nil
	}
	return &k
}

// syncKingdomFaction brings the faction of location id in step with it:
// creating or tying one when it is a kingdom, untying it when it isn't.
func syncKingdomFaction(db *sql.DB, id int64) {
	var kind, name, color string
	if db.QueryRow(`SELECT kind, name, COALESCE(color, '') FROM locations WHERE id = ?`, id).Scan(&kind, &name, &color) != nil {
		return
	}
	if kind != "major" || color == "" {
		if _, err := db.Exec(`UPDATE factions SET kingdom_id = NULL WHERE kingdom_id = ?`, id); err != nil {
			log.Printf("untie kingdom faction: %v", err)
		}
		return
	}
	var fid int64
	err := db.QueryRow(`SELECT id FROM factions WHERE kingdom_id = ?`, id).Scan(&fid)
	if err == sql.ErrNoRows {
		// A faction of the same name that isn't tied to anything yet is
		// taken over rather than duplicated.
		err = db.QueryRow(`SELECT id FROM factions WHERE kingdom_id IS NULL AND name = ? COLLATE NOCASE ORDER BY id LIMIT 1`,
			strings.TrimSpace(name)).Scan(&fid)
		if err == sql.ErrNoRows {
			if _, e := db.Exec(`INSERT INTO factions (name, color, hq_location_id, kingdom_id) VALUES (?, ?, ?, ?)`, name, color, id, id); e != nil {
				log.Printf("create kingdom faction: %v", e)
			}
			return
		}
	}
	if err != nil {
		log.Printf("find kingdom faction: %v", err)
		return
	}
	if _, err := db.Exec(`UPDATE factions SET name = ?, color = ?, kingdom_id = ?,
		hq_location_id = COALESCE(hq_location_id, ?), updated_at = datetime('now') WHERE id = ?`,
		name, color, id, id, fid); err != nil {
		log.Printf("update kingdom faction: %v", err)
	}
}

// backfillKingdomFactions gives every existing kingdom its faction, once,
// when a story is upgraded to schema 20.
func backfillKingdomFactions(db *sql.DB) error {
	rows, err := db.Query(`SELECT id FROM locations WHERE kind = 'major' AND COALESCE(color, '') <> '' ORDER BY id`)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		syncKingdomFaction(db, id)
	}
	return nil
}
