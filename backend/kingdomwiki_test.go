package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// A city on a kingdom's hexes belongs to it in the wiki and the graph, as on
// the map, and the kingdom and its faction are one node on the graph.
func TestPlacesOnAKingdomsHexes(t *testing.T) {
	mgr, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Realm"})
	api := "/api/s/" + s.ID
	app, _ := mgr.open(s.ID)

	var k struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(do(t, h, "POST", api+"/map/locations", jsonBody(map[string]any{"kind": "major", "name": "Stahl-Ur", "color": "#4488cc"}), "application/json").Body.Bytes(), &k)
	do(t, h, "POST", api+"/map/paint", jsonBody(map[string]any{"location_id": k.ID, "hexes": [][2]int{{0, 0}, {1, 0}}}), "application/json")
	place := func(name string, q int, capital bool) int64 {
		var l struct {
			ID int64 `json:"id"`
		}
		json.Unmarshal(do(t, h, "POST", api+"/map/locations", jsonBody(map[string]any{"kind": "minor", "name": name, "q": q, "r": 0, "is_city": true, "is_capital": capital}), "application/json").Body.Bytes(), &l)
		return l.ID
	}
	city := place("Ziggustahl", 0, false)
	capital := place("Ur", 1, true)
	place("Far Away", 9, false)

	a := wikiGet(t, h, s.ID, "location", k.ID)
	names := []string{}
	for _, g := range a.Groups {
		if g.Key == "places" {
			for _, r := range g.Items {
				names = append(names, r.Name)
			}
		}
	}
	if strings.Join(names, ",") != "Ur,Ziggustahl" {
		t.Errorf("places within: %v", names)
	}
	capFact := ""
	for _, f := range a.Facts {
		if f.Key == "capital" && f.Link != nil && f.Link.ID == capital {
			capFact = f.Value
		}
	}
	if capFact == "" {
		t.Errorf("capital fact missing: %+v", a.Facts)
	}
	c := wikiGet(t, h, s.ID, "location", city)
	partOf := false
	for _, f := range c.Facts {
		partOf = partOf || (f.Key == "part_of" && f.Link != nil && f.Link.ID == k.ID)
	}
	if !partOf {
		t.Errorf("city should be part of the kingdom: %+v", c.Facts)
	}

	var g graphPayload
	json.Unmarshal(do(t, h, "GET", api+"/wiki/graph", nil, "").Body.Bytes(), &g)
	var fid int64
	app.db.QueryRow(`SELECT id FROM factions WHERE kingdom_id = ?`, k.ID).Scan(&fid)
	kingdom := refKey("location", k.ID)
	if g.Merged[refKey("faction", fid)] != kingdom {
		t.Errorf("merged: %v", g.Merged)
	}
	for _, n := range g.Nodes {
		if n.Type == "faction" {
			t.Errorf("the kingdom's faction should not be a node of its own: %+v", n)
		}
	}
	linked := map[string]bool{}
	for _, e := range g.Edges {
		if e.A == kingdom || e.B == kingdom {
			linked[e.A+"-"+e.B] = true
		}
		if e.A == e.B {
			t.Errorf("self edge: %+v", e)
		}
	}
	if len(linked) != 2 {
		t.Errorf("kingdom edges: %v", linked)
	}
}
