package main

import (
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func TestHoldsAt(t *testing.T) {
	ranks := map[int64]int{10: 0, 20: 1, 30: 2}
	ch := func(id int64) *int64 { return &id }
	at := func(rank int, date string) storyPoint {
		p := storyPoint{Date: date}
		if rank >= 0 {
			p.Chapter, p.HasChapter = rank, true
		}
		return p
	}
	cases := []struct {
		name         string
		p            storyPoint
		sinceCh      *int64
		untilCh      *int64
		since, until string
		want         bool
	}{
		{"no bounds", at(1, ""), nil, nil, "", "", true},
		{"before the start chapter", at(0, ""), ch(20), nil, "", "", false},
		{"at the start chapter", at(1, ""), ch(20), nil, "", "", true},
		{"at the end chapter", at(1, ""), nil, ch(20), "", "", false},
		{"before the end chapter", at(0, ""), nil, ch(20), "", "", true},
		{"no chapter on the version: dates decide", at(-1, "01-01-1200"), ch(20), nil, "01-01-1250", "", false},
		{"nothing comparable: shown", at(-1, ""), ch(20), ch(30), "", "", true},
		{"date before start", at(-1, "01-01-1199"), nil, nil, "01-01-1200", "", false},
		{"date on the end", at(-1, "01-01-1300"), nil, nil, "", "01-01-1300", false},
		{"date inside", at(-1, "05-05-1250"), nil, nil, "01-01-1200", "01-01-1300", true},
	}
	for _, c := range cases {
		if got := holdsAt(c.p, ranks, c.sinceCh, c.untilCh, c.since, c.until); got != c.want {
			t.Errorf("%s: got %v", c.name, got)
		}
	}
}

// A character's relations change between versions; the wiki shows each
// version's, the graph all of them, and a former name still finds them.
func TestRelationsByVersionAndFormerNames(t *testing.T) {
	_, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Versions"})
	api := "/api/s/" + s.ID
	chars := map[string]int64{}
	versions := map[string]int64{}
	for _, name := range []string{"Lyra", "Bram"} {
		body, ct := form(t, map[string]string{"name": name}, "", "", nil)
		var ch struct {
			ID        int64 `json:"id"`
			VersionID int64 `json:"version_id"`
		}
		json.Unmarshal(do(t, h, "POST", api+"/characters", body, ct).Body.Bytes(), &ch)
		chars[name], versions[name] = ch.ID, ch.VersionID
	}
	chapter := func(title string) int64 {
		var c struct {
			ID int64 `json:"id"`
		}
		json.Unmarshal(do(t, h, "POST", api+"/chapters", jsonBody(map[string]any{"title": title}), "application/json").Body.Bytes(), &c)
		return c.ID
	}
	ch1, ch5 := chapter("One"), chapter("Five")
	setVersion := func(vid int64, name string, chapter int64) {
		body, ct := form(t, map[string]string{"name": name, "level": "1", "chapter_id": strconv.FormatInt(chapter, 10)}, "", "", nil)
		if rec := do(t, h, "PUT", api+"/versions/"+idStr(vid), body, ct); rec.Code != 200 {
			t.Fatalf("version: %d %s", rec.Code, rec.Body.String())
		}
	}
	setVersion(versions["Lyra"], "Lyra", ch1)

	// A later version, adopted and renamed.
	rec := do(t, h, "POST", api+"/characters/"+idStr(chars["Lyra"])+"/versions",
		strings.NewReader(`{"version_date":"","version_reference":"","clone_from_version_id":`+idStr(versions["Lyra"])+`}`), "application/json")
	var nv struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(rec.Body.Bytes(), &nv)
	setVersion(nv.ID, "Lyra Vane", ch5)
	do(t, h, "POST", api+"/versions/"+idStr(nv.ID)+"/set-current", nil, "")

	rel := func(kind string, since, until *int64) {
		rec := do(t, h, "POST", api+"/relations", jsonBody(map[string]any{
			"from_id": chars["Bram"], "to_id": chars["Lyra"], "kind": kind, "since_chapter_id": since, "until_chapter_id": until,
		}), "application/json")
		if rec.Code != 200 {
			t.Fatalf("relation: %d %s", rec.Code, rec.Body.String())
		}
	}
	rel("friend", nil, &ch5)
	rel("parent", &ch5, nil)
	if rec := do(t, h, "POST", api+"/relations", jsonBody(map[string]any{
		"from_id": chars["Bram"], "to_id": chars["Lyra"], "kind": "ally", "since_chapter_id": ch5, "until_chapter_id": ch1,
	}), "application/json"); rec.Code != 400 {
		t.Errorf("end chapter before start: %d", rec.Code)
	}

	notes := func(groups []wikiGroup) string {
		var out []string
		for _, g := range groups {
			if g.Key == "relations" {
				for _, r := range g.Items {
					out = append(out, r.Note)
				}
			}
		}
		return strings.Join(out, ",")
	}
	a := wikiGet(t, h, s.ID, "character", chars["Lyra"])
	if got := notes(a.Groups); got != "Child of" {
		t.Errorf("current version's relations: %q", got)
	}
	if len(a.Versions) != 2 || a.Versions[0].Name != "Lyra" || notes(a.Versions[0].Groups) != "Friend of" || notes(a.Versions[1].Groups) != "Child of" {
		t.Errorf("versions: %+v", a.Versions)
	}
	aka := ""
	for _, f := range a.Facts {
		if f.Key == "also_known_as" {
			aka = f.Value
		}
	}
	if aka != "Lyra" {
		t.Errorf("also known as: %q", aka)
	}

	// The graph keeps every relation, the former name still links.
	var g graphPayload
	json.Unmarshal(do(t, h, "GET", api+"/wiki/graph", nil, "").Body.Bytes(), &g)
	if len(g.Edges) != 1 || !g.Edges[0].Story {
		t.Errorf("graph: %+v", g.Edges)
	}
	b := wikiPut(t, h, s.ID, "character", chars["Bram"], `{"summary":"Raised [[Lyra]]."}`)
	if b == nil {
		t.Fatal("save failed")
	}
	if l := wikiGet(t, h, s.ID, "character", chars["Lyra"]); len(l.Backlinks) != 1 {
		t.Errorf("a link by the former name should count: %+v", l.Backlinks)
	}
	for _, it := range wikiList(t, h, s.ID) {
		if it.ID == chars["Lyra"] && it.Type == "character" && (len(it.Aliases) != 1 || it.Aliases[0] != "Lyra") {
			t.Errorf("aliases: %+v", it.Aliases)
		}
	}
}

// Kingdoms get a faction tied to them, which follows the map.
func TestKingdomFactions(t *testing.T) {
	mgr, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Realms"})
	api := "/api/s/" + s.ID
	app, _ := mgr.open(s.ID)
	app.db.Exec(`INSERT INTO factions (name) VALUES ('Velmar')`)

	newLoc := func(name, color string) int64 {
		var l struct {
			ID int64 `json:"id"`
		}
		json.Unmarshal(do(t, h, "POST", api+"/map/locations", jsonBody(map[string]any{"kind": "major", "name": name, "color": color}), "application/json").Body.Bytes(), &l)
		return l.ID
	}
	tied := func(loc int64) (name, color string, ok bool) {
		err := app.db.QueryRow(`SELECT name, COALESCE(color, '') FROM factions WHERE kingdom_id = ?`, loc).Scan(&name, &color)
		return name, color, err == nil
	}
	aldoria := newLoc("Aldoria", "#cc4444")
	if name, color, ok := tied(aldoria); !ok || name != "Aldoria" || color != "#cc4444" {
		t.Fatalf("kingdom faction: %q %q %v", name, color, ok)
	}
	newLoc("Plains", "")
	var n int
	app.db.QueryRow(`SELECT COUNT(*) FROM factions`).Scan(&n)
	if n != 2 {
		t.Errorf("a colourless place is no kingdom: %d factions", n)
	}
	// An untied faction of the same name is taken over, not duplicated.
	velmar := newLoc("Velmar", "#44cc44")
	app.db.QueryRow(`SELECT COUNT(*) FROM factions WHERE name = 'Velmar'`).Scan(&n)
	if _, _, ok := tied(velmar); !ok || n != 1 {
		t.Errorf("same-name faction should be tied: %d", n)
	}

	// Renaming on the map renames the faction; the faction can't rename it.
	do(t, h, "PUT", api+"/map/locations/"+idStr(aldoria), jsonBody(map[string]any{"kind": "major", "name": "Aldoria Prime", "color": "#cc4444"}), "application/json")
	if name, _, _ := tied(aldoria); name != "Aldoria Prime" {
		t.Errorf("rename: %q", name)
	}
	var fid int64
	app.db.QueryRow(`SELECT id FROM factions WHERE kingdom_id = ?`, aldoria).Scan(&fid)
	body, ct := form(t, map[string]string{"name": "Something else", "description": "Old realm"}, "", "", nil)
	do(t, h, "PUT", api+"/factions/"+idStr(fid), body, ct)
	if name, _, _ := tied(aldoria); name != "Aldoria Prime" {
		t.Errorf("a kingdom's faction keeps the kingdom's name: %q", name)
	}

	// Losing the colour unties; deleting the kingdom keeps the faction.
	do(t, h, "PUT", api+"/map/locations/"+idStr(aldoria), jsonBody(map[string]any{"kind": "major", "name": "Aldoria Prime", "color": ""}), "application/json")
	if _, _, ok := tied(aldoria); ok {
		t.Error("no longer a kingdom: should be untied")
	}
	do(t, h, "DELETE", api+"/map/locations/"+idStr(velmar), nil, "")
	var kingdom sql.NullInt64
	if err := app.db.QueryRow(`SELECT kingdom_id FROM factions WHERE name = 'Velmar'`).Scan(&kingdom); err != nil || kingdom.Valid {
		t.Errorf("deleted kingdom: faction gone or still tied (%v %v)", err, kingdom)
	}
}

func TestMigrationTiesExistingKingdoms(t *testing.T) {
	mgr, _ := newTestServer(t)
	id := createStoryDirect(t, mgr, "Old")
	app, _ := mgr.open(id)
	rollBackSchema20(t, app.db)
	for _, q := range []string{
		`INSERT INTO locations (id, name, kind, color) VALUES (1, 'Aldoria', 'major', '#cc4444'), (2, 'Plains', 'major', NULL)`,
		`INSERT INTO characters (id) VALUES (1), (2)`,
		`INSERT INTO character_relations (from_id, to_id, kind) VALUES (1, 2, 'friend')`,
	} {
		if _, err := app.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err := ensureSchema(app.db); err != nil {
		t.Fatal(err)
	}
	var n int
	app.db.QueryRow(`SELECT COUNT(*) FROM factions WHERE kingdom_id = 1 AND name = 'Aldoria'`).Scan(&n)
	if n != 1 {
		t.Error("the existing kingdom should get its faction")
	}
	var since sql.NullInt64
	if err := app.db.QueryRow(`SELECT since_chapter_id FROM character_relations`).Scan(&since); err != nil || since.Valid {
		t.Errorf("relations keep working, with no chapter: %v", err)
	}
}
