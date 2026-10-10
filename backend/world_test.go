package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// worldSetup makes a story with three characters and returns their ids.
func worldSetup(t *testing.T) (http.Handler, storyInfo, []int64) {
	t.Helper()
	_, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "World"})
	var ids []int64
	for _, name := range []string{"Aria", "Bram", "Cora"} {
		body, ct := form(t, map[string]string{"name": name}, "", "", nil)
		rec := do(t, h, "POST", "/api/s/"+s.ID+"/characters", body, ct)
		var ch struct {
			ID int64 `json:"id"`
		}
		json.Unmarshal(rec.Body.Bytes(), &ch)
		if ch.ID == 0 {
			t.Fatalf("create %s: %s", name, rec.Body.String())
		}
		ids = append(ids, ch.ID)
	}
	return h, s, ids
}

func jsonBody(v any) *strings.Reader {
	b, _ := json.Marshal(v)
	return strings.NewReader(string(b))
}

func TestRelations(t *testing.T) {
	h, s, ids := worldSetup(t)
	api := "/api/s/" + s.ID
	aria, bram, cora := ids[0], ids[1], ids[2]

	for _, bad := range []map[string]any{
		{"from_id": aria, "to_id": aria, "kind": "friend"},
		{"from_id": aria, "to_id": bram, "kind": "custom"},
		{"from_id": aria, "to_id": bram, "kind": "best-buddy"},
		{"from_id": aria, "to_id": bram, "kind": "ally", "since": "01-01-1010", "until": "01-01-1000"},
	} {
		if rec := do(t, h, "POST", api+"/relations", jsonBody(bad), "application/json"); rec.Code != 400 {
			t.Errorf("%v should be refused, got %d", bad, rec.Code)
		}
	}

	rec := do(t, h, "POST", api+"/relations", jsonBody(map[string]any{
		"from_id": aria, "to_id": bram, "kind": "parent", "since": "1000",
	}), "application/json")
	if rec.Code != 200 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var rel relationOut
	json.Unmarshal(rec.Body.Bytes(), &rel)
	if rel.Since != "01-01-1000" {
		t.Errorf("since should be normalized, got %q", rel.Since)
	}
	do(t, h, "POST", api+"/relations", jsonBody(map[string]any{
		"from_id": cora, "to_id": aria, "kind": "custom", "label": "Sworn shield of", "reverse_label": "Protected by",
	}), "application/json")

	// Bram sees the relation from his side, in his wording.
	rec = do(t, h, "GET", api+"/characters/"+idStr(bram)+"/connections", nil, "")
	var conn struct {
		Relations []relationOut `json:"relations"`
	}
	json.Unmarshal(rec.Body.Bytes(), &conn)
	if len(conn.Relations) != 1 {
		t.Fatalf("bram's relations: %s", rec.Body.String())
	}
	if w, builtin := conn.Relations[0].wording(bram); w != "Child of" || !builtin {
		t.Errorf("bram should read 'Child of', got %q", w)
	}

	// Aria's wiki article lists both, with how each reads from her side.
	rec = do(t, h, "GET", api+"/wiki/character/"+idStr(aria), nil, "")
	body := rec.Body.String()
	for _, want := range []string{`"note":"Parent of","note_word":true`, `"note":"Protected by"`} {
		if !strings.Contains(body, want) {
			t.Errorf("aria's article should contain %s: %s", want, body)
		}
	}

	// Deleting a character removes their relations.
	do(t, h, "DELETE", api+"/characters", strings.NewReader(`{"ids":[`+idStr(bram)+`]}`), "application/json")
	rec = do(t, h, "GET", api+"/characters/"+idStr(aria)+"/connections", nil, "")
	json.Unmarshal(rec.Body.Bytes(), &conn)
	if len(conn.Relations) != 1 {
		t.Errorf("only the relation with Cora should remain: %s", rec.Body.String())
	}
}

func TestFactions(t *testing.T) {
	h, s, ids := worldSetup(t)
	api := "/api/s/" + s.ID
	save := func(method, url string, fields map[string]string) (int, factionOut) {
		body, ct := form(t, fields, "", "", nil)
		rec := do(t, h, method, url, body, ct)
		var f factionOut
		json.Unmarshal(rec.Body.Bytes(), &f)
		return rec.Code, f
	}

	code, church := save("POST", api+"/factions", map[string]string{"name": "The Church", "color": "#aa3344"})
	if code != 200 {
		t.Fatalf("create: %d", code)
	}
	members := `[{"character_id":` + idStr(ids[0]) + `,"role":"Inquisitor","since":"1000"},` +
		`{"character_id":` + idStr(ids[1]) + `,"role":"Novice","until":"01-01-1010"},{"character_id":9999}]`
	code, order := save("POST", api+"/factions", map[string]string{
		"name": "Order of Ash", "parent_id": idStr(church.ID), "members": members,
	})
	if code != 200 || order.ParentName != "The Church" || len(order.Members) != 2 {
		t.Fatalf("create with parent and members: %d %+v", code, order)
	}

	if code, _ := save("POST", api+"/factions", map[string]string{"name": "the church"}); code != 400 {
		t.Errorf("duplicate name (any case) should be refused, got %d", code)
	}
	if code, _ := save("PUT", api+"/factions/"+idStr(church.ID), map[string]string{
		"name": "The Church", "parent_id": idStr(order.ID),
	}); code != 400 {
		t.Errorf("a faction inside its own child should be refused, got %d", code)
	}

	// Saving without a members field leaves the members alone.
	if code, f := save("PUT", api+"/factions/"+idStr(order.ID), map[string]string{
		"name": "Order of Ash", "parent_id": idStr(church.ID),
	}); code != 200 || len(f.Members) != 2 {
		t.Errorf("members should survive a save without them: %d %+v", code, f.Members)
	}

	rec := do(t, h, "GET", api+"/wiki/faction/"+idStr(order.ID), nil, "")
	body := rec.Body.String()
	for _, want := range []string{`"key":"part_of"`, `"key":"members"`, `"key":"former_members"`, `"note":"Inquisitor"`} {
		if !strings.Contains(body, want) {
			t.Errorf("faction article should contain %s: %s", want, body)
		}
	}
	rec = do(t, h, "GET", api+"/characters", nil, "")
	if !strings.Contains(rec.Body.String(), `"faction_ids":[`+idStr(order.ID)+`]`) {
		t.Errorf("character list should carry faction ids: %s", rec.Body.String())
	}

	if rec := do(t, h, "DELETE", api+"/factions/"+idStr(church.ID), nil, ""); rec.Code != 200 {
		t.Fatalf("delete: %d", rec.Code)
	}
	if _, f := save("PUT", api+"/factions/"+idStr(order.ID), map[string]string{"name": "Order of Ash"}); f.ParentID != nil {
		t.Errorf("deleting the parent should leave the child without one")
	}
}

func TestChapters(t *testing.T) {
	h, s, ids := worldSetup(t)
	api := "/api/s/" + s.ID
	post := func(url string, v any) map[string]any {
		rec := do(t, h, "POST", url, jsonBody(v), "application/json")
		if rec.Code != 200 {
			t.Fatalf("%s: %d %s", url, rec.Code, rec.Body.String())
		}
		var out map[string]any
		json.Unmarshal(rec.Body.Bytes(), &out)
		return out
	}
	book := int64(post(api+"/volumes", map[string]any{"title": "Book One"})["id"].(float64))
	prologue := int64(post(api+"/chapters", map[string]any{"title": "Prologue"})["id"].(float64))
	c1 := int64(post(api+"/chapters", map[string]any{"title": "Ashes", "volume_id": book})["id"].(float64))
	c2 := post(api+"/chapters", map[string]any{"title": "Embers", "volume_id": book})
	if c2["number"].(float64) != 2 {
		t.Errorf("second chapter of a volume should be number 2: %v", c2)
	}

	// An event and a version point at chapter 1; its page lists both.
	body, ct := form(t, map[string]string{"name": "Battle", "event_date": "01-02-1000", "chapter_id": idStr(c1)}, "", "", nil)
	if rec := do(t, h, "POST", api+"/events", body, ct); !strings.Contains(rec.Body.String(), `"chapter_id":`+idStr(c1)) {
		t.Fatalf("event should keep its chapter: %s", rec.Body.String())
	}
	rec := do(t, h, "GET", api+"/characters/"+idStr(ids[0])+"/current", nil, "")
	var cur struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(rec.Body.Bytes(), &cur)
	body, ct = form(t, map[string]string{"name": "Aria", "level": "1", "chapter_id": idStr(c1)}, "", "", nil)
	do(t, h, "PUT", api+"/versions/"+idStr(cur.ID), body, ct)

	rec = do(t, h, "GET", api+"/chapters/"+idStr(c1), nil, "")
	var d chapterDetail
	json.Unmarshal(rec.Body.Bytes(), &d)
	if len(d.Events) != 1 || len(d.Versions) != 1 || d.VolumeTitle != "Book One" || d.PrevID == nil || *d.PrevID != prologue {
		t.Errorf("chapter page: %s", rec.Body.String())
	}

	// Reorder: Embers before Ashes.
	rec = do(t, h, "PUT", api+"/chapters/order", jsonBody(map[string]any{
		"volumes":  []int64{book},
		"chapters": []map[string]any{{"id": prologue}, {"id": int64(c2["id"].(float64)), "volume_id": book}, {"id": c1, "volume_id": book}},
	}), "application/json")
	var idx chapterIndex
	json.Unmarshal(rec.Body.Bytes(), &idx)
	if len(idx.Chapters) != 3 || idx.Chapters[2].ID != c1 || idx.Chapters[2].Number != 2 {
		t.Errorf("reorder: %s", rec.Body.String())
	}

	// Deleting the volume keeps its chapters; deleting a chapter unlinks.
	do(t, h, "DELETE", api+"/volumes/"+idStr(book), nil, "")
	do(t, h, "DELETE", api+"/chapters/"+idStr(c1), nil, "")
	rec = do(t, h, "GET", api+"/chapters", nil, "")
	json.Unmarshal(rec.Body.Bytes(), &idx)
	if len(idx.Volumes) != 0 || len(idx.Chapters) != 2 || idx.Chapters[1].VolumeID != nil {
		t.Errorf("after deletes: %s", rec.Body.String())
	}
	rec = do(t, h, "GET", api+"/events", nil, "")
	if strings.Contains(rec.Body.String(), `"chapter_id":`+idStr(c1)) {
		t.Errorf("event should lose its deleted chapter: %s", rec.Body.String())
	}
}

func TestLastExported(t *testing.T) {
	_, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Saga"})
	lastExported := func() string {
		rec := do(t, h, "GET", "/api/stories", nil, "")
		var list []storyInfo
		json.Unmarshal(rec.Body.Bytes(), &list)
		return list[0].LastExportedAt
	}
	if got := lastExported(); got != "" {
		t.Fatalf("a new story was never exported, got %q", got)
	}
	exportZip(t, h, s.ID)
	if got := lastExported(); got == "" {
		t.Errorf("export should be remembered")
	}
}
