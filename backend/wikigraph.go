package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strconv"
)

// The wiki's graph view: every article as a node and every connection
// between two articles as an edge. Connections come from two places, kept
// apart so the page can show either: what the story itself says (the infobox
// links and related lists, from loadSourceFacts — the same ones the article
// pages show) and the [[links]] people wrote in the wiki text.

type graphNode struct {
	Type    string `json:"type"`
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
	Written bool   `json:"written"`
}

// graphEdge joins two nodes, named "type:id". The pair is unordered; Story
// and Link say where the connection comes from (both can be true).
type graphEdge struct {
	A     string `json:"a"`
	B     string `json:"b"`
	Story bool   `json:"story"`
	Link  bool   `json:"link"`
}

type graphPayload struct {
	Nodes []graphNode `json:"nodes"`
	Edges []graphEdge `json:"edges"`
}

func refKey(typ string, id int64) string { return typ + ":" + strconv.FormatInt(id, 10) }

// wikiTexts gathers everything written in the wiki, per entry, and which
// article ("type:id") each entry belongs to.
func wikiTexts(db *sql.DB) (texts map[int64][]string, owner map[int64]string) {
	texts = map[int64][]string{}
	owner = map[int64]string{}
	rows, err := db.Query(`SELECT id, entity_type, entity_id, summary, fields FROM wiki_entries`)
	if err != nil {
		return
	}
	for rows.Next() {
		var eid, ent int64
		var et, summary, fields string
		if rows.Scan(&eid, &et, &ent, &summary, &fields) != nil {
			continue
		}
		owner[eid] = refKey(et, ent)
		texts[eid] = append(texts[eid], summary)
		var m map[string]string
		if json.Unmarshal([]byte(fields), &m) == nil {
			for _, v := range m {
				texts[eid] = append(texts[eid], v)
			}
		}
	}
	rows.Close()
	for _, q := range []string{
		`SELECT entry_id, title || char(10) || body FROM wiki_sections`,
		`SELECT entry_id, value FROM wiki_infobox`,
	} {
		rows, err := db.Query(q)
		if err != nil {
			continue
		}
		for rows.Next() {
			var eid int64
			var s string
			if rows.Scan(&eid, &s) == nil {
				texts[eid] = append(texts[eid], s)
			}
		}
		rows.Close()
	}
	return
}

func buildWikiGraph(db *sql.DB) (*graphPayload, error) {
	items, err := listWikiItems(db)
	if err != nil {
		return nil, err
	}
	out := &graphPayload{Nodes: []graphNode{}, Edges: []graphEdge{}}
	exists := map[string]bool{}
	for _, it := range items {
		out.Nodes = append(out.Nodes, graphNode{Type: it.Type, ID: it.ID, Name: it.Name, Picture: it.Picture, Written: it.Written})
		exists[refKey(it.Type, it.ID)] = true
	}

	edges := map[[2]string]*graphEdge{}
	add := func(a, b string, story bool) {
		if a == b || !exists[a] || !exists[b] {
			return
		}
		if b < a {
			a, b = b, a
		}
		e := edges[[2]string{a, b}]
		if e == nil {
			e = &graphEdge{A: a, B: b}
			edges[[2]string{a, b}] = e
		}
		if story {
			e.Story = true
		} else {
			e.Link = true
		}
	}

	// What the story connects.
	for _, it := range items {
		a := &wikiArticle{Type: it.Type, ID: it.ID}
		loadSourceFacts(db, a)
		from := refKey(it.Type, it.ID)
		for _, f := range a.Facts {
			if f.Link != nil {
				add(from, refKey(f.Link.Type, f.Link.ID), true)
			}
		}
		for _, g := range a.Groups {
			for _, r := range g.Items {
				add(from, refKey(r.Type, r.ID), true)
			}
		}
	}

	// What the wiki text links.
	index := wikiNameIndex(items)
	texts, owner := wikiTexts(db)
	for eid, parts := range texts {
		from := owner[eid]
		for _, p := range parts {
			for _, m := range wikiLinkRe.FindAllStringSubmatch(p, -1) {
				if ref := resolveWikiLink(m[1], index); ref != nil {
					add(from, refKey(ref.Type, ref.ID), false)
				}
			}
		}
	}

	for _, e := range edges {
		out.Edges = append(out.Edges, *e)
	}
	sort.Slice(out.Edges, func(i, j int) bool {
		if out.Edges[i].A != out.Edges[j].A {
			return out.Edges[i].A < out.Edges[j].A
		}
		return out.Edges[i].B < out.Edges[j].B
	})
	return out, nil
}

func wikiGraphHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		g, err := buildWikiGraph(db)
		if err != nil {
			http.Error(w, "failed to load the graph", http.StatusInternalServerError)
			log.Printf("wiki graph: %v", err)
			return
		}
		writeJSON(w, g)
	}
}
