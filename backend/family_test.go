package main

import (
	"encoding/json"
	"strconv"
	"testing"
)

// A family is everyone reached through parent, sibling, spouse and partner
// relations; friends and rivals don't count.
func TestCharacterFamily(t *testing.T) {
	_, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Family"})
	api := "/api/s/" + s.ID
	ids := map[string]int64{}
	for _, name := range []string{"Mother", "Father", "Child", "Sister", "Friend", "Stranger"} {
		body, ct := form(t, map[string]string{"name": name}, "", "", nil)
		var ch struct {
			ID int64 `json:"id"`
		}
		json.Unmarshal(do(t, h, "POST", api+"/characters", body, ct).Body.Bytes(), &ch)
		ids[name] = ch.ID
	}
	rel := func(from, to, kind string) {
		rec := do(t, h, "POST", api+"/relations", jsonBody(map[string]any{"from_id": ids[from], "to_id": ids[to], "kind": kind}), "application/json")
		if rec.Code != 200 {
			t.Fatalf("relation: %d %s", rec.Code, rec.Body.String())
		}
	}
	rel("Mother", "Child", "parent")
	rel("Father", "Child", "parent")
	rel("Mother", "Father", "spouse")
	rel("Sister", "Child", "sibling")
	rel("Child", "Friend", "friend")

	rec := do(t, h, "GET", api+"/characters/"+strconv.FormatInt(ids["Sister"], 10)+"/family", nil, "")
	if rec.Code != 200 {
		t.Fatalf("family: %d %s", rec.Code, rec.Body.String())
	}
	var f familyPayload
	json.Unmarshal(rec.Body.Bytes(), &f)
	names := map[string]bool{}
	for _, p := range f.People {
		names[p.Name] = true
	}
	if len(f.People) != 4 || !names["Mother"] || !names["Father"] || !names["Child"] || !names["Sister"] {
		t.Errorf("people: %+v", f.People)
	}
	if len(f.Links) != 4 || f.Root != ids["Sister"] {
		t.Errorf("links: %+v", f.Links)
	}

	// Someone with no family is a family of one.
	rec = do(t, h, "GET", api+"/characters/"+strconv.FormatInt(ids["Stranger"], 10)+"/family", nil, "")
	json.Unmarshal(rec.Body.Bytes(), &f)
	if len(f.People) != 1 || len(f.Links) != 0 {
		t.Errorf("alone: %+v", f)
	}
	if rec := do(t, h, "GET", api+"/characters/9999/family", nil, ""); rec.Code != 404 {
		t.Errorf("missing character: %d", rec.Code)
	}
}
