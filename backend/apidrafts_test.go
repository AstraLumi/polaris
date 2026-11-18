package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// The drafts' tools: list, read a board and a note, write a note as
// Markdown, add cards (category made by name, placed apart, joined); a
// read-only key sees none that write.
func TestMCPDraftTools(t *testing.T) {
	f := newAPIFixture(t)
	key, _ := f.makeKey(t, newAPIKeyIn{Name: "Claude", Write: true})
	base := "/api/s/" + f.story.ID
	body, ct := form(t, map[string]string{"name": "Mira"}, "", "", nil)
	if rec := do(t, f.app, "POST", base+"/characters", body, ct); rec.Code != 200 {
		t.Fatalf("character: %d %s", rec.Code, rec.Body.String())
	}
	var board, note draftPage
	rec := do(t, f.app, "POST", base+"/drafts", strings.NewReader(`{"kind":"board","title":"Plot"}`), "application/json")
	json.Unmarshal(rec.Body.Bytes(), &board)
	rec = do(t, f.app, "POST", base+"/drafts", strings.NewReader(`{"kind":"note","title":"Voices"}`), "application/json")
	json.Unmarshal(rec.Body.Bytes(), &note)

	run := func(name string, args map[string]any, out any) {
		t.Helper()
		args["story"] = f.story.ID
		text, isErr := f.tool(t, key, name, args)
		if isErr {
			t.Fatalf("%s: %s", name, text)
		}
		if out != nil {
			if err := json.Unmarshal([]byte(text), out); err != nil {
				t.Fatalf("%s: %v in %s", name, err, text)
			}
		}
	}

	var list draftsList
	run("list_drafts", map[string]any{}, &list)
	if len(list.Pages) != 2 {
		t.Fatalf("list %+v", list)
	}

	// Cards: the category is made by name once, links stored by id and read
	// back by name, the second card placed apart from the first and joined.
	var first, second struct {
		Card   draftCard `json:"card"`
		Joined []int64   `json:"joined"`
	}
	run("add_card", map[string]any{"draft_id": board.ID, "title": "Siege", "category": "Plot", "body": "**[[Mira]]** holds the gate. ||She dies.||"}, &first)
	run("add_card", map[string]any{"draft_id": board.ID, "title": "Betrayal", "category": "plot", "near": first.Card.ID, "join": []int64{first.Card.ID}, "join_label": "leads to"}, &second)
	if !strings.Contains(first.Card.Body, "[[character#") {
		t.Errorf("the body's link was not stored by id: %q", first.Card.Body)
	}
	if first.Card.CategoryID == nil || second.Card.CategoryID == nil || *first.Card.CategoryID != *second.Card.CategoryID {
		t.Errorf("the category was not shared: %v %v", first.Card.CategoryID, second.Card.CategoryID)
	}
	if second.Card.X < first.Card.X+draftTileW || len(second.Joined) != 1 {
		t.Errorf("second card %+v joined %v", second.Card, second.Joined)
	}

	var read struct {
		Kind  string           `json:"kind"`
		Cards []map[string]any `json:"cards"`
		Lines []map[string]any `json:"lines"`
	}
	run("read_draft", map[string]any{"draft_id": board.ID}, &read)
	if read.Kind != "board" || len(read.Cards) != 2 || len(read.Lines) != 1 || read.Lines[0]["label"] != "leads to" {
		t.Fatalf("board %+v", read)
	}
	if b, _ := read.Cards[0]["body"].(string); !strings.Contains(b, "[[character:Mira]]") || read.Cards[0]["category"] != "Plot" {
		t.Errorf("card read as %+v", read.Cards[0])
	}

	// A note: written and read as Markdown; a board isn't a note.
	run("write_note", map[string]any{"draft_id": note.ID, "markdown": "[[Mira]] never says please."}, nil)
	var n struct {
		Kind     string `json:"kind"`
		Markdown string `json:"markdown"`
		Version  int64  `json:"version"`
	}
	run("read_draft", map[string]any{"draft_id": note.ID}, &n)
	if n.Kind != "note" || !strings.Contains(n.Markdown, "never says please") || n.Version != 1 {
		t.Errorf("note %+v", n)
	}
	if text, isErr := f.tool(t, key, "write_note", map[string]any{"story": f.story.ID, "draft_id": board.ID, "markdown": "x"}); !isErr {
		t.Errorf("a board took a note's text: %s", text)
	}
	if text, isErr := f.tool(t, key, "add_card", map[string]any{"story": f.story.ID, "draft_id": note.ID, "title": "x"}); !isErr {
		t.Errorf("a note took a card: %s", text)
	}

	// The guide tells of them; a read-only key gets no writing tool.
	text, _ := f.tool(t, key, "api_guide", map[string]any{})
	if !strings.Contains(text, "Drafts are the writer's scratch space") || !strings.Contains(text, "POST drafts/{id}/cards  — json") {
		t.Error("the guide doesn't explain the drafts")
	}
	ro, _ := f.makeKey(t, newAPIKeyIn{Name: "Reader"})
	rec = f.call(t, ro, "POST", "/mcp", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, "application/json")
	if b := rec.Body.String(); strings.Contains(b, `"add_card"`) || strings.Contains(b, `"write_note"`) || !strings.Contains(b, `"read_draft"`) {
		t.Errorf("read-only tools: %s", b)
	}
}
