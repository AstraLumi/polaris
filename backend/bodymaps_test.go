package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// The system gets a name; it and its bodies are wiki articles that link to
// each other; deleting a body takes its article.
func TestSolarSystemWiki(t *testing.T) {
	_, h := newTestServer(t)
	st := createStory(t, h, map[string]string{"name": "Wiki sky"})
	base := "/api/s/" + st.ID + "/solar"
	_, s := solarSave(t, h, "POST", base+"/bodies", `{"kind":"star","name":"Sol","body_type":"star","radius_km":696000,"temperature_k":5772,"mass_kg":1.989e30}`)
	star := s.Bodies[0].ID
	_, s = solarSave(t, h, "POST", base+"/bodies", `{"kind":"planet","name":"Terra","body_type":"rocky","radius_km":6371,"orbit_km":1.496e8,"parent_id":`+idStr(star)+`}`)
	terra := s.Bodies[1].ID
	_, s = solarSave(t, h, "PUT", base+"/system", `{"name":"  Helios  "}`)
	if s.Name != "Helios" {
		t.Fatalf("name: %q", s.Name)
	}

	a := wikiGet(t, h, st.ID, "celestial_body", terra)
	facts := map[string]wikiFact{}
	for _, f := range a.Facts {
		facts[f.Key] = f
	}
	if facts["type"].Value != "Planet" || facts["orbits"].Link == nil || facts["orbits"].Link.ID != star ||
		facts["system"].Link == nil || !strings.Contains(facts["orbit_radius"].Value, "1 AU") {
		t.Errorf("planet facts: %+v", a.Facts)
	}
	sys := wikiGet(t, h, st.ID, "solar_system", 1)
	if sys.Name != "Helios" || len(sys.Groups) != 1 || sys.Groups[0].Items[0].Name != "Terra" {
		t.Errorf("system: %+v", sys)
	}

	wikiPut(t, h, st.ID, "celestial_body", terra, `{"summary":"Home.","fields":{},"sections":[],"infobox":[]}`)
	solarSave(t, h, "DELETE", base+"/bodies/"+idStr(terra), ``)
	if rec := do(t, h, "GET", "/api/s/"+st.ID+"/wiki/celestial_body/"+idStr(terra), nil, ""); rec.Code != 404 {
		t.Errorf("deleted planet's article: %d", rec.Code)
	}
}

// The shell round the system is a switch kept with the story.
func TestSolarEnclosure(t *testing.T) {
	_, h := newTestServer(t)
	st := createStory(t, h, map[string]string{"name": "Shelled"})
	base := "/api/s/" + st.ID + "/solar"
	_, s := solarSave(t, h, "GET", base, ``)
	if s.Enclosed {
		t.Fatal("a new system is open")
	}
	if _, s = solarSave(t, h, "PUT", base+"/enclosure", `{"enclosed":true}`); !s.Enclosed {
		t.Fatal("not enclosed after turning it on")
	}
	// Naming the system leaves it as it is.
	if _, s = solarSave(t, h, "PUT", base+"/system", `{"name":"Kept"}`); !s.Enclosed {
		t.Fatal("naming opened the shell")
	}
	if _, s = solarSave(t, h, "PUT", base+"/enclosure", `{"enclosed":false}`); s.Enclosed {
		t.Fatal("still enclosed after turning it off")
	}
	if rec := do(t, h, "PUT", base+"/enclosure", strings.NewReader(`{}`), "application/json"); rec.Code != 400 {
		t.Errorf("no value: %d", rec.Code)
	}
}

// Every planet but the main one has a map to paint; it keeps the grid it
// was first painted on.
func TestBodyMaps(t *testing.T) {
	mgr, h := newTestServer(t)
	st := createStory(t, h, map[string]string{"name": "Maps"})
	app, _ := mgr.open(st.ID)
	base := "/api/s/" + st.ID + "/solar"
	_, s := solarSave(t, h, "POST", base+"/bodies", `{"kind":"star","name":"Sol","body_type":"star","radius_km":696000,"temperature_k":5772,"mass_kg":1.989e30}`)
	star := s.Bodies[0].ID
	_, s = solarSave(t, h, "POST", base+"/bodies", `{"kind":"planet","name":"Home","body_type":"rocky","radius_km":6371,"orbit_km":1.5e8,"is_main":true,"parent_id":`+idStr(star)+`}`)
	home := s.Bodies[1].ID
	_, s = solarSave(t, h, "POST", base+"/bodies", `{"kind":"planet","name":"Mars","body_type":"rocky","radius_km":3390,"orbit_km":2.3e8,"parent_id":`+idStr(star)+`}`)
	mars := s.Bodies[2].ID

	for _, id := range []int64{star, home} {
		if rec := do(t, h, "GET", base+"/bodies/"+idStr(id)+"/map", nil, ""); rec.Code != 400 {
			t.Errorf("body %d has no map of its own: %d", id, rec.Code)
		}
	}
	var m bodyMap
	json.Unmarshal(do(t, h, "GET", base+"/bodies/"+idStr(mars)+"/map", nil, "").Body.Bytes(), &m)
	if m.Frequency < 2 || len(m.Cells) != 0 {
		t.Fatalf("empty map: %+v", m)
	}
	freq := m.Frequency
	paint := func(path, body string, want int) {
		t.Helper()
		if rec := do(t, h, "POST", base+"/bodies/"+idStr(mars)+"/map/"+path, strings.NewReader(body), "application/json"); rec.Code != want {
			t.Errorf("%s %s: %d %s", path, body, rec.Code, rec.Body.String())
		}
	}
	paint("paint", `{"color":"#AA3300","cells":[0,1,2]}`, 200)
	paint("paint", `{"color":"#00aa00","cells":[2]}`, 200)
	paint("erase", `{"cells":[0]}`, 200)
	paint("paint", `{"color":"red","cells":[3]}`, 400)
	paint("paint", `{"color":"#aa3300","cells":[999999999]}`, 400)

	// Growing the planet doesn't move what was painted.
	solarSave(t, h, "PUT", base+"/bodies/"+idStr(mars), `{"name":"Mars","body_type":"rocky","radius_km":30000,"orbit_km":2.3e8}`)
	json.Unmarshal(do(t, h, "GET", base+"/bodies/"+idStr(mars)+"/map", nil, "").Body.Bytes(), &m)
	if m.Frequency != freq || len(m.Cells) != 2 || m.Colors[0] != "#00aa00" && m.Colors[0] != "#aa3300" {
		t.Errorf("after resizing: %+v", m)
	}
	solarSave(t, h, "DELETE", base+"/bodies/"+idStr(mars), ``)
	var n int
	app.db.QueryRow(`SELECT COUNT(*) FROM body_cells`).Scan(&n)
	if n != 0 {
		t.Errorf("deleting the planet leaves %d cells", n)
	}
}
