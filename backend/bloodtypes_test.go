package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// A story starts with the eight usual blood types, can save its own list,
// and refuses empty or repeated names.
func TestBloodTypes(t *testing.T) {
	_, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Blood"})
	base := "/api/s/" + s.ID

	var got settingsPayload
	json.Unmarshal(do(t, h, "GET", base+"/settings", nil, "").Body.Bytes(), &got)
	if len(got.BloodTypes) != 8 || got.BloodTypes[0].Name != "A+" || got.BloodTypes[7].Name != "O-" {
		t.Fatalf("defaults: %+v", got.BloodTypes)
	}

	rec := do(t, h, "PUT", base+"/settings/blood-types",
		strings.NewReader(`[{"name":" A+ ","note":""},{"name":"Starlight","note":"Glows in the dark."}]`), "application/json")
	if rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	json.Unmarshal(do(t, h, "GET", base+"/settings", nil, "").Body.Bytes(), &got)
	if len(got.BloodTypes) != 2 || got.BloodTypes[0].Name != "A+" || got.BloodTypes[1].Note != "Glows in the dark." {
		t.Errorf("saved: %+v", got.BloodTypes)
	}

	// An empty list is kept as empty, not read back as the defaults.
	do(t, h, "PUT", base+"/settings/blood-types", strings.NewReader(`[]`), "application/json")
	json.Unmarshal(do(t, h, "GET", base+"/settings", nil, "").Body.Bytes(), &got)
	if len(got.BloodTypes) != 0 {
		t.Errorf("empty list: %+v", got.BloodTypes)
	}

	for _, bad := range []string{`[{"name":" "}]`, `[{"name":"O+"},{"name":"o+"}]`, `[{"name":"` + strings.Repeat("x", 25) + `"}]`} {
		if rec := do(t, h, "PUT", base+"/settings/blood-types", strings.NewReader(bad), "application/json"); rec.Code != 400 {
			t.Errorf("%s: %d", bad, rec.Code)
		}
	}
}
