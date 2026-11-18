package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// A story at schema 48 gets the drafts' tables; deleting a page takes its
// cards and lines, deleting a card its lines, deleting a category leaves
// its cards without one.
func TestMigrationAddsDrafts(t *testing.T) {
	mgr, _ := newTestServer(t)
	app, _ := mgr.open(createStoryDirect(t, mgr, "Old"))
	rollBackSchema49(t, app.db)
	if err := ensureSchema(app.db); err != nil {
		t.Fatal(err)
	}
	if v := storedSchema(app.db); v != currentSchemaVersion {
		t.Fatalf("version %s", v)
	}
	for _, q := range []string{
		`INSERT INTO draft_pages (id, kind, title) VALUES (1, 'board', 'Ideas')`,
		`INSERT INTO draft_categories (id, name, color) VALUES (1, 'Plot', '#aa3344')`,
		`INSERT INTO draft_cards (id, page_id, title, category_id) VALUES (1, 1, 'A', 1), (2, 1, 'B', NULL), (3, 1, 'C', NULL)`,
		`INSERT INTO draft_links (page_id, a_id, b_id) VALUES (1, 1, 2), (1, 2, 3)`,
	} {
		if _, err := app.db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if _, err := app.db.Exec(`INSERT INTO draft_pages (kind, title) VALUES ('sketch', 'x')`); err == nil {
		t.Error("an unknown kind was stored")
	}
	if _, err := app.db.Exec(`INSERT INTO draft_links (page_id, a_id, b_id) VALUES (1, 3, 1)`); err == nil {
		t.Error("a line was stored with a > b")
	}
	count := func(q string) (n int) { app.db.QueryRow(q).Scan(&n); return }
	app.db.Exec(`DELETE FROM draft_categories WHERE id = 1`)
	if n := count(`SELECT COUNT(*) FROM draft_cards WHERE category_id IS NULL`); n != 3 {
		t.Errorf("%d cards without a category after it went, want 3", n)
	}
	app.db.Exec(`DELETE FROM draft_cards WHERE id = 2`)
	if n := count(`SELECT COUNT(*) FROM draft_links`); n != 0 {
		t.Errorf("%d lines left after their card went", n)
	}
	app.db.Exec(`DELETE FROM draft_pages WHERE id = 1`)
	if n := count(`SELECT COUNT(*) FROM draft_cards`); n != 0 {
		t.Errorf("%d cards left after their board went", n)
	}
}

func TestDraftsAPI(t *testing.T) {
	mgr, h := newTestServer(t)
	id := createStoryDirect(t, mgr, "World")
	app, _ := mgr.open(id)
	app.db.Exec(`INSERT INTO locations (id, name, kind) VALUES (1, 'Aurel', 'major')`)
	base := "/api/s/" + id
	call := func(method, url string, in any, want int) []byte {
		t.Helper()
		rec := do(t, h, method, base+url, jsonBody(in), "application/json")
		if rec.Code != want {
			t.Fatalf("%s %s: %d %s", method, url, rec.Code, rec.Body.String())
		}
		return rec.Body.Bytes()
	}

	var board, note draftPage
	json.Unmarshal(call("POST", "/drafts", map[string]string{"kind": "board", "title": "Plot ideas"}, 200), &board)
	json.Unmarshal(call("POST", "/drafts", map[string]string{"kind": "note", "title": "Loose lines"}, 200), &note)
	call("POST", "/drafts", map[string]string{"kind": "sketch", "title": "x"}, 400)

	var cat draftCategory
	json.Unmarshal(call("POST", "/draft-categories", map[string]string{"name": "Plot", "color": "#AA3344"}, 200), &cat)
	if cat.Color != "#aa3344" {
		t.Errorf("colour stored as %q", cat.Color)
	}
	call("POST", "/draft-categories", map[string]string{"name": "Bad", "color": "red"}, 400)

	// Cards: links are stored by id, sizes kept in bounds.
	var a, b draftCard
	json.Unmarshal(call("POST", fmt.Sprintf("/drafts/%d/cards", board.ID), map[string]any{
		"title": "Siege", "category_id": cat.ID, "body": "Happens at [[Aurel]]", "x": 10, "y": 20, "w": 5, "h": 99999,
	}, 200), &a)
	if a.Body != "Happens at [[location#1]]" || a.W != minDraftCardW || a.H != maxDraftCardH || a.CategoryID == nil {
		t.Errorf("card stored as %+v", a)
	}
	json.Unmarshal(call("POST", fmt.Sprintf("/drafts/%d/cards", board.ID), map[string]any{"title": "Betrayal"}, 200), &b)
	call("POST", fmt.Sprintf("/drafts/%d/cards", note.ID), map[string]any{"title": "x"}, 404)

	// Only the fields sent change.
	var moved draftCard
	json.Unmarshal(call("PUT", fmt.Sprintf("/draft-cards/%d", a.ID), map[string]any{"x": 300.4}, 200), &moved)
	if moved.X != 300 || moved.Y != 20 || moved.Title != "Siege" || moved.Body != a.Body {
		t.Errorf("after a move: %+v", moved)
	}
	json.Unmarshal(call("PUT", fmt.Sprintf("/draft-cards/%d", a.ID), map[string]any{"no_category": true}, 200), &moved)
	if moved.CategoryID != nil {
		t.Error("the category stayed")
	}

	// Lines: stored once whichever way round, never to itself.
	var l draftLink
	json.Unmarshal(call("POST", fmt.Sprintf("/drafts/%d/links", board.ID), map[string]any{"a": b.ID, "b": a.ID, "label": "leads to"}, 200), &l)
	var again draftLink
	json.Unmarshal(call("POST", fmt.Sprintf("/drafts/%d/links", board.ID), map[string]any{"a": a.ID, "b": b.ID}, 200), &again)
	if again.ID != l.ID || l.A != min(a.ID, b.ID) {
		t.Errorf("lines %+v %+v", l, again)
	}
	call("POST", fmt.Sprintf("/drafts/%d/links", board.ID), map[string]any{"a": a.ID, "b": a.ID}, 400)

	var got struct {
		Cards []draftCard `json:"cards"`
		Links []draftLink `json:"links"`
	}
	json.Unmarshal(call("GET", fmt.Sprintf("/drafts/%d/board", board.ID), nil, 200), &got)
	if len(got.Cards) != 2 || len(got.Links) != 1 || got.Links[0].Label != "leads to" {
		t.Errorf("board %+v", got)
	}

	// One way: the wiki doesn't see the drafts.
	var art struct {
		Backlinks []wikiRef `json:"backlinks"`
	}
	json.Unmarshal(call("GET", "/wiki/location/1", nil, 200), &art)
	if len(art.Backlinks) != 0 {
		t.Errorf("the wiki sees the drafts: %+v", art.Backlinks)
	}

	// Notes: saved on their version, 409 with the stored note otherwise.
	var n draftNote
	json.Unmarshal(call("GET", fmt.Sprintf("/drafts/%d/note", note.ID), nil, 200), &n)
	if n.Version != 0 || !strings.Contains(string(n.Doc), `"doc"`) {
		t.Fatalf("new note %+v", n)
	}
	doc := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"two words"}]}]}`)
	call("PUT", fmt.Sprintf("/drafts/%d/note", note.ID), map[string]any{"doc": doc, "base_version": 0}, 200)
	call("PUT", fmt.Sprintf("/drafts/%d/note", note.ID), map[string]any{"doc": doc, "base_version": 0}, 409)
	call("PUT", fmt.Sprintf("/drafts/%d/note", note.ID), map[string]any{"doc": json.RawMessage(`{"type":"nope"}`), "base_version": 1}, 400)
	call("GET", fmt.Sprintf("/drafts/%d/note", board.ID), nil, 404)

	var list struct {
		Pages      []draftPage     `json:"pages"`
		Categories []draftCategory `json:"categories"`
	}
	call("PUT", "/drafts/order", map[string]any{"ids": []int64{note.ID, board.ID}}, 200)
	json.Unmarshal(call("GET", "/drafts", nil, 200), &list)
	if len(list.Pages) != 2 || list.Pages[0].ID != note.ID || list.Pages[0].Words != 2 || list.Pages[1].Cards != 2 || len(list.Categories) != 1 {
		t.Errorf("list %+v", list)
	}

	call("DELETE", fmt.Sprintf("/drafts/%d", board.ID), nil, 200)
	call("GET", fmt.Sprintf("/drafts/%d/board", board.ID), nil, 404)
}
