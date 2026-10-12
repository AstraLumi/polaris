package main

import (
	"database/sql"
	"fmt"
	"log"
	"math"
)

// Where things are, in whichever map counts (planetmap.go): the 2D hexes in
// 2D-only mode, the planet's cells in 3D.

// mapSpace names the table holding major locations' areas and how a place
// (a minor location) is matched to an area row.
type mapSpace struct {
	mode  string
	areas string // "location_hexes" | "planet_cells"
	match string // printf with the area alias then the place alias
}

func spaceOf(db *sql.DB) mapSpace {
	if mapMode(db) == "3d" {
		return mapSpace{"3d", "planet_cells", "%[1]s.cell = %[2]s.cell"}
	}
	return mapSpace{"2d", "location_hexes", "%[1]s.q = %[2]s.q AND %[1]s.r = %[2]s.r"}
}

// on is the join condition between area alias a and place alias p.
func (s mapSpace) on(a, p string) string { return fmt.Sprintf(s.match, a, p) }

// spot is one hex or cell: (q, r) in 2D, (cell, 0) on the planet.
type spot [2]int64

// areaSpots reads spots with an owner id: SQL selecting (id, spot…).
func (s mapSpace) read(db *sql.DB, q2d, q3d string, args ...any) map[spot][]int64 {
	out := map[spot][]int64{}
	q := q2d
	if s.mode == "3d" {
		q = q3d
	}
	rows, err := db.Query(q, args...)
	if err != nil {
		log.Printf("read spots: %v", err)
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var sp spot
		var err error
		if s.mode == "3d" {
			err = rows.Scan(&id, &sp[0])
		} else {
			err = rows.Scan(&id, &sp[0], &sp[1])
		}
		if err == nil {
			out[sp] = append(out[sp], id)
		}
	}
	return out
}

// neighbors of a spot: the six hexes round a 2D hex, or the cells round a
// planet cell.
func (s mapSpace) neighbors(g planetGrid, sp spot) []spot {
	if s.mode == "3d" {
		var out []spot
		for _, c := range g.neighbors(int(sp[0])) {
			out = append(out, spot{int64(c), 0})
		}
		return out
	}
	q, r := sp[0], sp[1]
	return []spot{{q + 1, r}, {q - 1, r}, {q, r + 1}, {q, r - 1}, {q + 1, r - 1}, {q - 1, r + 1}}
}

// addBorders lists what a kingdom, region or ocean touches: the other
// coloured major locations next to its hexes, and the unnamed ocean where
// it meets hexes nothing is on.
func (a *wikiArticle) addBorders(db *sql.DB, id int64) {
	s := spaceOf(db)
	g := loadMapSettings(db).Planet.grid()
	// Who rules each spot, and which spots have anything at all on them.
	owners := s.read(db,
		`SELECT h.location_id, h.q, h.r FROM location_hexes h JOIN locations l ON l.id = h.location_id
			WHERE l.kind = 'major' AND l.color IS NOT NULL`,
		`SELECT h.location_id, h.cell FROM planet_cells h JOIN locations l ON l.id = h.location_id
			WHERE l.kind = 'major' AND l.color IS NOT NULL`)
	land := s.read(db,
		`SELECT location_id, q, r FROM location_hexes UNION ALL
			SELECT id, q, r FROM locations WHERE kind = 'minor' AND q IS NOT NULL AND r IS NOT NULL`,
		`SELECT location_id, cell FROM planet_cells UNION ALL
			SELECT id, cell FROM locations WHERE kind = 'minor' AND cell IS NOT NULL`)

	next := map[int64]bool{}
	ocean := false
	for sp, ids := range owners {
		mine := false
		for _, x := range ids {
			mine = mine || x == id
		}
		if !mine {
			continue
		}
		for _, nb := range s.neighbors(g, sp) {
			if len(land[nb]) == 0 {
				ocean = true
				continue
			}
			for _, x := range owners[nb] {
				if x != id {
					next[x] = true
				}
			}
		}
	}
	var refs []wikiRef
	if len(next) > 0 {
		rows, err := db.Query(`SELECT id, name FROM locations WHERE kind = 'major' ORDER BY name COLLATE NOCASE`)
		if err == nil {
			for rows.Next() {
				var r wikiRef
				if rows.Scan(&r.ID, &r.Name) == nil && next[r.ID] {
					r.Type = "location"
					refs = append(refs, r)
				}
			}
			rows.Close()
		}
	}
	// The open sea has no article: a name alone, translated by the frontend.
	if ocean {
		refs = append(refs, wikiRef{Name: "Unnamed ocean"})
	}
	a.addGroup("borders", refs)
}

// planetLatitudes are the latitudes (degrees) of a location's cells on the
// planet, with the planet's settings.
func planetLatitudes(db *sql.DB, kind string, id int64) ([]float64, planetSettings) {
	p := loadMapSettings(db).Planet
	g := p.grid()
	q := `SELECT cell FROM planet_cells WHERE location_id = ?`
	if kind != "major" {
		q = `SELECT cell FROM locations WHERE id = ? AND cell IS NOT NULL`
	}
	rows, err := db.Query(q, id)
	if err != nil {
		return nil, p
	}
	defer rows.Close()
	var out []float64
	for rows.Next() {
		var c int
		if rows.Scan(&c) == nil && c >= 0 && c < g.cellCount() {
			lat, _ := latLon(g.center(c))
			out = append(out, lat*180/math.Pi)
		}
	}
	return out, p
}
