package main

import (
	"log"
	"net/http"
	"sort"
	"strconv"

	"database/sql"
)

// A character's family for the family tree page: everyone reachable through
// parent, sibling, spouse and partner relations, and those relations. The
// layout happens in the browser (frontend/src/familyLayout.js).

var familyKinds = map[string]bool{"parent": true, "sibling": true, "spouse": true, "partner": true}

const maxFamily = 400

type familyPerson struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
	Born    string `json:"born"` // story date, DD-MM-YYYY, or ""
	Status  string `json:"status"`
}

// familyLink is one relation. For "parent", From is the parent.
type familyLink struct {
	From  int64  `json:"from"`
	To    int64  `json:"to"`
	Kind  string `json:"kind"`
	Until string `json:"until,omitempty"` // a spouse or partner relation that ended
}

type familyPayload struct {
	Root   int64          `json:"root"`
	People []familyPerson `json:"people"`
	Links  []familyLink   `json:"links"`
}

func characterFamily(db *sql.DB, root int64) (*familyPayload, error) {
	rows, err := db.Query(`SELECT from_id, to_id, kind, until FROM character_relations ORDER BY id`)
	if err != nil {
		return nil, err
	}
	var all []familyLink
	adj := map[int64][]int{}
	for rows.Next() {
		var l familyLink
		if rows.Scan(&l.From, &l.To, &l.Kind, &l.Until) != nil || !familyKinds[l.Kind] {
			continue
		}
		adj[l.From] = append(adj[l.From], len(all))
		adj[l.To] = append(adj[l.To], len(all))
		all = append(all, l)
	}
	rows.Close()

	// Everyone connected to the root, nearest first, up to maxFamily.
	seen := map[int64]bool{root: true}
	queue := []int64{root}
	for len(queue) > 0 && len(seen) < maxFamily {
		id := queue[0]
		queue = queue[1:]
		for _, li := range adj[id] {
			for _, other := range []int64{all[li].From, all[li].To} {
				if !seen[other] && len(seen) < maxFamily {
					seen[other] = true
					queue = append(queue, other)
				}
			}
		}
	}

	out := &familyPayload{Root: root, People: []familyPerson{}, Links: []familyLink{}}
	prow, err := db.Query(`
		SELECT c.id, v.name, COALESCE(v.picture_path, ''), COALESCE(st.birth_date, ''), COALESCE(st.status, 'alive')
		FROM characters c
		JOIN character_versions v ON v.character_id = c.id AND v.is_current = 1
		LEFT JOIN character_story st ON st.version_id = v.id`)
	if err != nil {
		return nil, err
	}
	for prow.Next() {
		var p familyPerson
		if prow.Scan(&p.ID, &p.Name, &p.Picture, &p.Born, &p.Status) != nil || !seen[p.ID] {
			continue
		}
		p.Picture = uploadURL(p.Picture)
		p.Born = normalizeStoryDate(p.Born)
		if _, ok := parseStoryDate(p.Born); !ok {
			p.Born = ""
		}
		out.People = append(out.People, p)
	}
	prow.Close()
	sort.Slice(out.People, func(i, j int) bool { return out.People[i].ID < out.People[j].ID })

	found := false
	for _, p := range out.People {
		found = found || p.ID == root
	}
	if !found {
		return nil, sql.ErrNoRows
	}
	for _, l := range all {
		if seen[l.From] && seen[l.To] {
			out.Links = append(out.Links, l)
		}
	}
	return out, nil
}

func characterFamilyHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "character not found", http.StatusNotFound)
			return
		}
		f, err := characterFamily(db, id)
		if err == sql.ErrNoRows {
			http.Error(w, "character not found", http.StatusNotFound)
			return
		}
		if err != nil {
			log.Printf("character family: %v", err)
			http.Error(w, "failed to load the family", http.StatusInternalServerError)
			return
		}
		writeJSON(w, f)
	}
}
