package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// A character's tags belong to the character: saved with any version, shown
// on every version, in the list and on the wiki article.
func TestCharacterTags(t *testing.T) {
	h, s, vid := gearTestSetup(t)
	save := func(tags string) int {
		body, ct := form(t, map[string]string{"name": "Aria", "level": "10", "tags": tags}, "", "", nil)
		return do(t, h, "PUT", "/api/s/"+s.ID+"/versions/"+idStr(vid), body, ct).Code
	}
	if code := save(`["villain", " #Mage ", "VILLAIN", ""]`); code != 200 {
		t.Fatalf("save: %d", code)
	}
	rec := do(t, h, "GET", "/api/s/"+s.ID+"/versions/"+idStr(vid), nil, "")
	var v struct {
		CharacterID int64    `json:"character_id"`
		Tags        []string `json:"tags"`
	}
	json.Unmarshal(rec.Body.Bytes(), &v)
	if strings.Join(v.Tags, ",") != "Mage,villain" {
		t.Errorf("tags: %v", v.Tags)
	}

	// A new version shows the same tags without copying anything.
	rec = do(t, h, "POST", "/api/s/"+s.ID+"/characters/"+idStr(v.CharacterID)+"/versions",
		strings.NewReader(`{"version_date":"","version_reference":"","clone_from_version_id":`+idStr(vid)+`}`), "application/json")
	var nv struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(rec.Body.Bytes(), &nv)
	rec = do(t, h, "GET", "/api/s/"+s.ID+"/versions/"+idStr(nv.ID), nil, "")
	if !strings.Contains(rec.Body.String(), `"tags":["Mage","villain"]`) {
		t.Errorf("new version tags: %s", rec.Body.String())
	}
	if rec := do(t, h, "GET", "/api/s/"+s.ID+"/characters", nil, ""); !strings.Contains(rec.Body.String(), `"tags":["Mage","villain"]`) {
		t.Errorf("list tags: %s", rec.Body.String())
	}
	a := wikiGet(t, h, s.ID, "character", v.CharacterID)
	found := false
	for _, f := range a.Facts {
		found = found || (f.Key == "tags" && f.Value == "Mage, villain")
	}
	if !found {
		t.Errorf("wiki facts: %+v", a.Facts)
	}

	// Leaving tags out of a save keeps them; an empty list clears them.
	body, ct := form(t, map[string]string{"name": "Aria", "level": "10"}, "", "", nil)
	do(t, h, "PUT", "/api/s/"+s.ID+"/versions/"+idStr(vid), body, ct)
	if rec := do(t, h, "GET", "/api/s/"+s.ID+"/versions/"+idStr(vid), nil, ""); !strings.Contains(rec.Body.String(), `"tags":["Mage","villain"]`) {
		t.Error("a save without tags must keep them")
	}
	save(`[]`)
	if rec := do(t, h, "GET", "/api/s/"+s.ID+"/versions/"+idStr(vid), nil, ""); !strings.Contains(rec.Body.String(), `"tags":[]`) {
		t.Errorf("cleared tags: %s", rec.Body.String())
	}
	if code := save(`not json`); code != 400 {
		t.Errorf("bad tag list: %d", code)
	}
}

// A lore article's dated infobox rows put it on the timeline.
func TestLoreDatesOnTheTimeline(t *testing.T) {
	_, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Dates"})
	id, _ := createLore(t, h, s.ID, "The Cult")
	put := func(body string) int {
		return do(t, h, "PUT", "/api/s/"+s.ID+"/wiki/lore/"+strconv.FormatInt(id, 10), strings.NewReader(body), "application/json").Code
	}
	if code := put(`{"infobox":[{"label":"Founded","value":"nonsense","kind":"date"}]}`); code != 400 {
		t.Errorf("unreadable date: %d", code)
	}
	if code := put(`{"infobox":[{"label":"Founded","value":"0","kind":"date"}]}`); code != 400 {
		t.Errorf("year 0: %d", code)
	}
	if code := put(`{"infobox":[{"label":"Founded","value":"1200","kind":"date"},{"label":"Leader","value":"Bram"}]}`); code != 200 {
		t.Fatalf("save: %d", code)
	}
	a := wikiGet(t, h, s.ID, "lore", id)
	if len(a.Infobox) != 2 || a.Infobox[0].Kind != "date" || a.Infobox[0].Value != "01-01-1200" || a.Infobox[1].Kind != "" {
		t.Errorf("infobox: %+v", a.Infobox)
	}

	rec := do(t, h, "GET", "/api/s/"+s.ID+"/timeline", nil, "")
	var tl timelinePayload
	json.Unmarshal(rec.Body.Bytes(), &tl)
	if len(tl.Nodes) != 1 || tl.Nodes[0].Kind != "lore" || tl.Nodes[0].Name != "The Cult: Founded" || *tl.Nodes[0].SourceID != id {
		t.Errorf("timeline: %+v", tl.Nodes)
	}

	// Other articles can't have dated rows: the row is kept as plain text.
	res := createLoreLocation(t, h, s.ID)
	if a := wikiPut(t, h, s.ID, "location", res, `{"infobox":[{"label":"Founded","value":"1200","kind":"date"}]}`); a == nil || a.Infobox[0].Kind != "" {
		t.Errorf("dated row on a location: %+v", a)
	}
}

func createLoreLocation(t *testing.T, h http.Handler, story string) int64 {
	t.Helper()
	rec := do(t, h, "POST", "/api/s/"+story+"/map/locations", strings.NewReader(`{"kind":"major","name":"Vale"}`), "application/json")
	var out struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	return out.ID
}

func TestMigrationAddsTagsAndDatedRows(t *testing.T) {
	mgr, _ := newTestServer(t)
	id := createStoryDirect(t, mgr, "Old")
	app, _ := mgr.open(id)
	app.db.Exec(`INSERT INTO wiki_entries (id, entity_type, entity_id) VALUES (1, 'class', 1)`)
	app.db.Exec(`INSERT INTO characters (id) VALUES (1)`)
	rollBackSchema18(t, app.db)
	app.db.Exec(`INSERT INTO wiki_infobox (entry_id, position, label, value) VALUES (1, 0, 'Motto', 'Onward')`)
	if err := ensureSchema(app.db); err != nil {
		t.Fatal(err)
	}
	var kind string
	if err := app.db.QueryRow(`SELECT kind FROM wiki_infobox`).Scan(&kind); err != nil || kind != "" {
		t.Errorf("old rows should be plain rows: %q %v", kind, err)
	}
	if _, err := app.db.Exec(`INSERT INTO character_tags (character_id, tag) VALUES (1, 'hero')`); err != nil {
		t.Errorf("character_tags missing after the migration: %v", err)
	}
}
