package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// The graph has every article as a node and joins two articles once, saying
// whether the story, the wiki text, or both connect them.
func TestWikiGraph(t *testing.T) {
	mgr, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Graph"})
	app, _ := mgr.open(s.ID)
	res, _ := app.db.Exec(`INSERT INTO locations (name, kind) VALUES ('Aldoria', 'major')`)
	loc, _ := res.LastInsertId()
	res, _ = app.db.Exec(`INSERT INTO factions (name, hq_location_id) VALUES ('The Order', ?)`, loc)
	fac, _ := res.LastInsertId()
	gods, _ := createLore(t, h, s.ID, "The Old Gods")
	app.db.Exec(`INSERT INTO lore_articles (name) VALUES ('Alone')`)

	// The faction's headquarters is a story connection; the lore page links
	// to both by name, and the faction links back to Aldoria in its text too.
	wikiPut(t, h, s.ID, "lore", gods, `{"summary":"Worshipped in [[Aldoria]] by [[The Order]]."}`)
	wikiPut(t, h, s.ID, "faction", fac, `{"summary":"Based in [[Aldoria]]. Also [[Nowhere]]."}`)

	rec := do(t, h, "GET", "/api/s/"+s.ID+"/wiki/graph", nil, "")
	if rec.Code != 200 {
		t.Fatalf("graph: %d %s", rec.Code, rec.Body.String())
	}
	var g graphPayload
	json.Unmarshal(rec.Body.Bytes(), &g)
	if len(g.Nodes) != 4 {
		t.Errorf("nodes: %+v", g.Nodes)
	}
	got := []string{}
	for _, e := range g.Edges {
		kind := ""
		if e.Story {
			kind += "S"
		}
		if e.Link {
			kind += "L"
		}
		got = append(got, e.A+"-"+e.B+"="+kind)
	}
	want := "faction:1-location:1=SL,faction:1-lore:1=L,location:1-lore:1=L"
	if strings.Join(got, ",") != want {
		t.Errorf("edges:\n got %s\nwant %s", strings.Join(got, ","), want)
	}
}
