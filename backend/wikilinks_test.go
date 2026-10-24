package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

// The cases frontend/tests/markdown.test.mjs checks too, so Go and JS read
// links the same way.
type wikiLinkFixtures struct {
	Types []string `json:"types"`
	Items []struct {
		Type    string   `json:"type"`
		ID      int64    `json:"id"`
		Name    string   `json:"name"`
		Aliases []string `json:"aliases"`
	} `json:"items"`
	Resolve   [][2]*string `json:"resolve"`
	Normalize [][2]string  `json:"normalize"`
	Editing   [][2]string  `json:"editing"`
	Plain     [][2]string  `json:"plain"`
	Links     []struct {
		Text    string
		Targets []string
	} `json:"-"`
	RawLinks [][2]json.RawMessage `json:"links"`
}

func loadWikiLinkFixtures(t *testing.T) (*wikiLinkFixtures, *wikiLinkIndex) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "frontend", "tests", "wikiLinks.fixtures.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f wikiLinkFixtures
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	for _, l := range f.RawLinks {
		var text string
		var targets []string
		json.Unmarshal(l[0], &text)
		json.Unmarshal(l[1], &targets)
		f.Links = append(f.Links, struct {
			Text    string
			Targets []string
		}{text, targets})
	}
	var items []wikiListItem
	for _, it := range f.Items {
		items = append(items, wikiListItem{Type: it.Type, ID: it.ID, Name: it.Name, Aliases: it.Aliases})
	}
	return &f, newWikiLinkIndex(items)
}

func TestWikiLinkFixtures(t *testing.T) {
	f, x := loadWikiLinkFixtures(t)

	var keys []string
	for _, wt := range wikiTypes {
		keys = append(keys, wt.Key)
	}
	sort.Strings(keys)
	want := slices.Clone(f.Types)
	sort.Strings(want)
	if !slices.Equal(keys, want) {
		t.Errorf("article kinds differ from the fixtures (and TYPE_ORDER): %v vs %v", keys, want)
	}

	for _, c := range f.Resolve {
		got := ""
		if r := x.resolve(*c[0]); r != nil {
			got = wikiIDLink(r.Type, r.ID)
		}
		w := ""
		if c[1] != nil {
			w = *c[1]
		}
		if got != w {
			t.Errorf("resolve %q = %q, want %q", *c[0], got, w)
		}
	}

	for _, c := range f.Normalize {
		got := normalizeWikiLinks(c[0], x)
		if got != c[1] {
			t.Errorf("normalize %q = %q, want %q", c[0], got, c[1])
		}
		if again := normalizeWikiLinks(got, x); again != got {
			t.Errorf("normalizing twice changes %q to %q", got, again)
		}
	}

	// What the editor shows (JS) saves back to the very same bytes.
	for _, c := range f.Editing {
		if got := normalizeWikiLinks(c[1], x); got != c[0] {
			t.Errorf("saving the editor's %q gives %q, want the stored %q", c[1], got, c[0])
		}
	}

	for _, c := range f.Plain {
		if got := wikiLinksAsText(c[0], x, ""); got != c[1] {
			t.Errorf("as text %q = %q, want %q", c[0], got, c[1])
		}
	}

	for _, c := range f.Links {
		var got []string
		for _, m := range findWikiLinks(c.Text) {
			got = append(got, m.Target)
		}
		if !slices.Equal(got, c.Targets) {
			t.Errorf("links in %q: %q, want %q", c.Text, got, c.Targets)
		}
	}
}

func TestWikiLinkNames(t *testing.T) {
	_, x := loadWikiLinkFixtures(t)
	got := wikiLinkNames(x, "[[character#12]] [[race#2|wyrms]] `[[class#1]]`", "[[character#99]] [[Bram]]")
	want := map[string]string{"character#12": "Aria Vale", "race#2": "Dragon"}
	if len(got) != len(want) || got["character#12"] != want["character#12"] || got["race#2"] != want["race#2"] {
		t.Errorf("link names: %v", got)
	}
}

// Saving wiki text stores its links by id: the article's name can change
// and the links still find it, show its new name, and count as backlinks.
func TestWikiSaveStoresLinksByID(t *testing.T) {
	mgr, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Linked"})
	app, _ := mgr.open(s.ID)
	res, _ := app.db.Exec(`INSERT INTO locations (name, kind) VALUES ('Aldoria', 'minor')`)
	loc, _ := res.LastInsertId()
	res, _ = app.db.Exec(`INSERT INTO races (name) VALUES ('Elf')`)
	race, _ := res.LastInsertId()
	res, _ = app.db.Exec(`INSERT INTO lore_articles (name) VALUES ('The Old Gods')`)
	gods, _ := res.LastInsertId()

	a := wikiPut(t, h, s.ID, "lore", gods, `{
		"summary":"Worshipped in [[Aldoria]] by [[race:elves|the elves]] and [[Elf]]. Not in [[Nowhere]]. `+"`[[Aldoria]]`"+`",
		"fields":{"trivia":"[[aldoria]]"},
		"sections":[{"title":"Rites in [[Aldoria]]","body":"See [[Elf|them]]."}],
		"infobox":[{"label":"Seat","value":"[[Aldoria]]"}]}`)
	if a == nil {
		t.Fatal("save failed")
	}
	lid, rid := idStr(loc), idStr(race)
	if want := "Worshipped in [[location#" + lid + "]] by [[race:elves|the elves]] and [[race#" + rid + "]]. Not in [[Nowhere]]. `[[Aldoria]]`"; a.Summary != want {
		t.Errorf("summary:\n got %q\nwant %q", a.Summary, want)
	}
	if a.Fields["trivia"] != "[[location#"+lid+"]]" || a.Sections[0].Body != "See [[race#"+rid+"|them]]." || a.Infobox[0].Value != "[[location#"+lid+"]]" {
		t.Errorf("saved text: %+v %+v %+v", a.Fields, a.Sections, a.Infobox)
	}
	if a.Sections[0].Title != "Rites in [[Aldoria]]" {
		t.Errorf("a section title is plain text and keeps what was typed: %q", a.Sections[0].Title)
	}
	if a.LinkNames["location#"+lid] != "Aldoria" || a.LinkNames["race#"+rid] != "Elf" {
		t.Errorf("link names: %v", a.LinkNames)
	}

	// Renamed: the links still lead there.
	app.db.Exec(`UPDATE locations SET name = 'New Aldoria' WHERE id = ?`, loc)
	place := wikiGet(t, h, s.ID, "location", loc)
	if len(place.Backlinks) != 1 || place.Backlinks[0].ID != gods {
		t.Errorf("backlinks after a rename: %+v", place.Backlinks)
	}
	a2 := wikiGet(t, h, s.ID, "lore", gods)
	if a2.LinkNames["location#"+lid] != "New Aldoria" || len(a2.Groups) != 1 || len(a2.Groups[0].Items) != 2 {
		t.Errorf("after the rename: %v %+v", a2.LinkNames, a2.Groups)
	}
	var g graphPayload
	json.Unmarshal(do(t, h, "GET", "/api/s/"+s.ID+"/wiki/graph", nil, "").Body.Bytes(), &g)
	links := 0
	for _, e := range g.Edges {
		if e.Link {
			links++
		}
	}
	if links != 2 {
		t.Errorf("graph link edges: %+v", g.Edges)
	}

	// Saving the text unchanged leaves it as it is.
	body, _ := json.Marshal(map[string]any{"summary": a2.Summary, "fields": a2.Fields, "sections": a2.Sections, "infobox": a2.Infobox})
	if a3 := wikiPut(t, h, s.ID, "lore", gods, string(body)); a3 == nil || a3.Summary != a2.Summary || a3.Fields["trivia"] != a2.Fields["trivia"] {
		t.Errorf("an unchanged save changed the text: %+v", a3)
	}

	// Deleted: the link stays, finding nothing.
	app.db.Exec(`DELETE FROM locations WHERE id = ?`, loc)
	a4 := wikiGet(t, h, s.ID, "lore", gods)
	if !strings.Contains(a4.Summary, "[[location#"+lid+"]]") || len(a4.Groups) != 1 || len(a4.Groups[0].Items) != 1 {
		t.Errorf("after the delete: %q %+v", a4.Summary, a4.Groups)
	}
	if _, ok := a4.LinkNames["location#"+lid]; ok {
		t.Errorf("a deleted article has no name: %v", a4.LinkNames)
	}
}

func TestChapterNotesStoreLinksByID(t *testing.T) {
	mgr, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Notes"})
	app, _ := mgr.open(s.ID)
	res, _ := app.db.Exec(`INSERT INTO locations (name, kind) VALUES ('Aldoria', 'minor')`)
	loc, _ := res.LastInsertId()
	api := "/api/s/" + s.ID
	rec := do(t, h, "POST", api+"/chapters", strings.NewReader(`{"title":"One"}`), "application/json")
	var ch chapterDetail
	json.Unmarshal(rec.Body.Bytes(), &ch)
	rec = do(t, h, "PUT", api+"/chapters/"+idStr(ch.ID), strings.NewReader(`{"title":"One","notes":"They reach [[Aldoria]]."}`), "application/json")
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	json.Unmarshal(rec.Body.Bytes(), &ch)
	if ch.Notes != "They reach [[location#"+idStr(loc)+"]]." {
		t.Errorf("notes: %q", ch.Notes)
	}
}

// Schema 42 turns the links already written into the stored form.
func TestMigrationStoresLinksByID(t *testing.T) {
	mgr, _ := newTestServer(t)
	app, _ := mgr.open(createStoryDirect(t, mgr, "Old"))
	db := app.db
	rollBackSchema42(t, db)
	exec := func(q string, args ...any) int64 {
		t.Helper()
		res, err := db.Exec(q, args...)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		id, _ := res.LastInsertId()
		return id
	}
	loc := exec(`INSERT INTO locations (name, kind) VALUES ('Aldoria', 'minor')`)
	ch := exec(`INSERT INTO characters DEFAULT VALUES`)
	exec(`INSERT INTO character_versions (character_id, name, is_current) VALUES (?, 'Ash', 0)`, ch)
	exec(`INSERT INTO character_versions (character_id, name, is_current) VALUES (?, 'Aria Vale', 1)`, ch)
	gods := exec(`INSERT INTO lore_articles (name) VALUES ('The Old Gods')`)
	entry := exec(`INSERT INTO wiki_entries (entity_type, entity_id, summary, fields) VALUES ('lore', ?, ?, ?)`,
		gods, "In [[Aldoria]], with [[Ash]]. [[Nowhere]].", `{"trivia":"[[aria vale|her]]","history":"plain"}`)
	sec := exec(`INSERT INTO wiki_sections (entry_id, position, title, body) VALUES (?, 0, '[[Aldoria]]', '[[Aldoria]]')`, entry)
	row := exec(`INSERT INTO wiki_infobox (entry_id, position, label, value) VALUES (?, 0, 'Seat', '[[Aldoria]]')`, entry)
	dated := exec(`INSERT INTO wiki_infobox (entry_id, position, label, value, kind) VALUES (?, 1, 'Founded', '01-01-1000', 'date')`, entry)
	chapter := exec(`INSERT INTO chapters (title, position, notes) VALUES ('One', 0, ?)`, "```\n[[Aldoria]]\n```\n[[Aldoria|home]]")
	// Another entry, linking a lore page.
	exec(`INSERT INTO wiki_entries (entity_type, entity_id, summary) VALUES ('location', ?, '[[The Old Gods]]')`, loc)

	if err := ensureSchema(db); err != nil {
		t.Fatal(err)
	}
	if v := storedSchema(db); v != currentSchemaVersion {
		t.Fatalf("schema %s", v)
	}
	L, C, G := idStr(loc), idStr(ch), idStr(gods)
	text := func(q string, id int64) string {
		t.Helper()
		var s string
		if err := db.QueryRow(q, id).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	checks := []struct{ got, want string }{
		{text(`SELECT summary FROM wiki_entries WHERE id = ?`, entry), "In [[location#" + L + "]], with [[character#" + C + "|Ash]]. [[Nowhere]]."},
		{text(`SELECT fields FROM wiki_entries WHERE id = ?`, entry), `{"history":"plain","trivia":"[[character#` + C + `|her]]"}`},
		{text(`SELECT title FROM wiki_sections WHERE id = ?`, sec), "[[Aldoria]]"},
		{text(`SELECT body FROM wiki_sections WHERE id = ?`, sec), "[[location#" + L + "]]"},
		{text(`SELECT value FROM wiki_infobox WHERE id = ?`, row), "[[location#" + L + "]]"},
		{text(`SELECT value FROM wiki_infobox WHERE id = ?`, dated), "01-01-1000"},
		{text(`SELECT notes FROM chapters WHERE id = ?`, chapter), "```\n[[Aldoria]]\n```\n[[location#" + L + "|home]]"},
		{text(`SELECT summary FROM wiki_entries WHERE entity_type = 'location' AND entity_id = ?`, loc), "[[lore#" + G + "]]"},
	}
	for i, c := range checks {
		if c.got != c.want {
			t.Errorf("%d: got %q, want %q", i, c.got, c.want)
		}
	}
}

// "Copy Character Assets" copies rows into another story, ids and all. A
// copied text that holds links by id must be relinked (linksForCopy), so
// every link column of a copied table is in its linkCols, and every other
// is a story-only table's.
func TestCopiedLinkColumnsAreRelinked(t *testing.T) {
	for _, c := range wikiLinkColumns {
		copied := false
		for _, a := range assetTables {
			if a.name == c.table {
				copied = true
				if !slices.Contains(a.linkCols, c.column) {
					t.Errorf("%s.%s holds links by id and is copied with the assets: add it to the table's linkCols", c.table, c.column)
				}
			}
		}
		if !copied && !slices.Contains(storyOnlyTables, c.table) {
			t.Errorf("%s should be a story-only table", c.table)
		}
	}
	for _, a := range assetTables {
		for _, col := range a.linkCols {
			if !slices.Contains(wikiLinkColumns, linkColumn{table: a.name, column: col}) {
				t.Errorf("%s.%s is in linkCols but not in wikiLinkColumns", a.name, col)
			}
		}
	}
}

func TestLinksForCopy(t *testing.T) {
	_, x := loadWikiLinkFixtures(t)
	copied := map[string]bool{"class": true, "race": true, "gear": true}
	cases := [][2]string{
		{"A [[class#1]] and [[race#2|wyrms]].", "A [[class#1]] and [[race#2|wyrms]]."},
		{"Made by [[character#12]] in [[location#7|the city]].", "Made by [[character:Aria Vale]] in [[location:Aldoria|the city]]."},
		{"[[character#12|]]", "[[character:Aria Vale|]]"},
		{"[[lore#5]] [[lore#6]]", "[[lore:Race: Origins]] [[lore:character#13]]"},
		{"Lost: [[character#99]], [[location#99|old home]].", "Lost: , old home."},
		{"[[class#99]]", ""},
		{"[[Aldoria]] `[[character#12]]`", "[[Aldoria]] `[[character#12]]`"},
	}
	for _, c := range cases {
		if got := linksForCopy(c[0], x, copied); got != c[1] {
			t.Errorf("copy %q = %q, want %q", c[0], got, c[1])
		}
	}
	// The names written for a copy find the same articles in the source.
	for _, m := range findWikiLinks(linksForCopy("[[character#12]] [[lore#6]] [[creature#9]]", x, copied)) {
		if r := x.resolve(m.Target); r == nil {
			t.Errorf("%q finds nothing", m.Target)
		}
	}
}

// An export keeps every row's id, so links by id still lead to the same
// articles in the imported story.
func TestExportImportKeepsLinks(t *testing.T) {
	mgr, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Linked"})
	app, _ := mgr.open(s.ID)
	app.db.Exec(`INSERT INTO locations (name, kind) VALUES ('Spare', 'minor')`)
	res, _ := app.db.Exec(`INSERT INTO locations (name, kind) VALUES ('Aldoria', 'minor')`)
	loc, _ := res.LastInsertId()
	app.db.Exec(`DELETE FROM locations WHERE name = 'Spare'`)
	res, _ = app.db.Exec(`INSERT INTO lore_articles (name) VALUES ('Gods')`)
	gods, _ := res.LastInsertId()
	if wikiPut(t, h, s.ID, "lore", gods, `{"summary":"In [[Aldoria]]."}`) == nil {
		t.Fatal("save failed")
	}
	body, ct := form(t, map[string]string{"name": "Siege", "event_date": "1000", "description": "At [[Aldoria]]."}, "", "", nil)
	var ev struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(do(t, h, "POST", "/api/s/"+s.ID+"/events", body, ct).Body.Bytes(), &ev)

	rec := importZipBytes(t, h, exportZip(t, h, s.ID))
	if rec.Code != 200 {
		t.Fatalf("import: %d %s", rec.Code, rec.Body.String())
	}
	var si storyInfo
	json.Unmarshal(rec.Body.Bytes(), &si)
	a := wikiGet(t, h, si.ID, "lore", gods)
	if a.Summary != "In [[location#"+idStr(loc)+"]]." || a.LinkNames["location#"+idStr(loc)] != "Aldoria" {
		t.Errorf("imported: %q %v", a.Summary, a.LinkNames)
	}
	if bl := wikiGet(t, h, si.ID, "location", loc).Backlinks; len(bl) != 1 || bl[0].ID != gods {
		t.Errorf("imported backlinks: %+v", bl)
	}
	// An item's own description too.
	e := wikiGet(t, h, si.ID, "event", ev.ID)
	if len(e.Sheet) != 1 || e.Sheet[0].Body != "At [[location#"+idStr(loc)+"]]." || e.LinkNames["location#"+idStr(loc)] != "Aldoria" {
		t.Errorf("imported description: %+v %v", e.Sheet, e.LinkNames)
	}
}
