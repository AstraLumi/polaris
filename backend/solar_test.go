package main

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func solarSave(t *testing.T, h http.Handler, method, url, body string) (*httptest.ResponseRecorder, solarSystem) {
	t.Helper()
	rec := do(t, h, method, url, strings.NewReader(body), "application/json")
	var s solarSystem
	json.Unmarshal(rec.Body.Bytes(), &s)
	return rec, s
}

// A star, planets round it and moons round them; one main planet, sized
// by the map; deleting a planet takes its moons.
func TestSolarSystem(t *testing.T) {
	_, h := newTestServer(t)
	st := createStory(t, h, map[string]string{"name": "Sky"})
	base := "/api/s/" + st.ID + "/solar"

	rec, s := solarSave(t, h, "POST", base+"/bodies", `{"kind":"star","name":"Sol","body_type":"star","radius_km":696000}`)
	if rec.Code != 200 || len(s.Bodies) != 1 {
		t.Fatalf("star: %d %s", rec.Code, rec.Body.String())
	}
	star := s.Bodies[0].ID
	if rec, _ := solarSave(t, h, "POST", base+"/bodies", `{"kind":"star","name":"Second","body_type":"star","radius_km":1000}`); rec.Code != 400 {
		t.Errorf("a second star: %d", rec.Code)
	}
	_, s = solarSave(t, h, "POST", base+"/bodies",
		`{"kind":"planet","name":"Home","body_type":"rocky","radius_km":1,"orbit_km":1.5e8,"parent_id":`+idStr(star)+`,"is_main":true,"color":"#3366AA"}`)
	home := s.Bodies[1]
	if !home.IsMain || home.Color != "#3366aa" || math.Abs(home.RadiusKm-s.Main.RadiusKm) > 1e-9 || s.Main.RadiusKm < 6000 {
		t.Errorf("main planet: %+v, main %+v", home, s.Main)
	}
	_, s = solarSave(t, h, "POST", base+"/bodies",
		`{"kind":"planet","name":"Giant","body_type":"gas","radius_km":70000,"orbit_km":7.8e8,"parent_id":`+idStr(star)+`,"is_main":true}`)
	mains := 0
	for _, b := range s.Bodies {
		if b.IsMain {
			mains++
		}
	}
	if mains != 1 || !s.Bodies[2].IsMain {
		t.Errorf("one main planet: %+v", s.Bodies)
	}
	giant := s.Bodies[2].ID

	for _, bad := range []string{
		`{"kind":"moon","name":"Lost","body_type":"rocky","radius_km":10,"orbit_km":1000,"parent_id":` + idStr(star) + `}`,
		`{"kind":"planet","name":"Hot","body_type":"star","radius_km":10,"orbit_km":1000,"parent_id":` + idStr(star) + `}`,
		`{"kind":"planet","name":"Nowhere","body_type":"rocky","radius_km":10,"parent_id":` + idStr(star) + `}`,
		`{"kind":"planet","name":"","body_type":"rocky","radius_km":10,"orbit_km":1000,"parent_id":` + idStr(star) + `}`,
	} {
		if rec, _ := solarSave(t, h, "POST", base+"/bodies", bad); rec.Code != 400 {
			t.Errorf("%s: %d", bad, rec.Code)
		}
	}

	_, s = solarSave(t, h, "POST", base+"/bodies", `{"kind":"moon","name":"Io","body_type":"rocky","radius_km":1800,"orbit_km":420000,"phase":400,"parent_id":`+idStr(giant)+`}`)
	if len(s.Bodies) != 4 || s.Bodies[3].Phase != 40 {
		t.Fatalf("moon: %+v", s.Bodies)
	}
	// Editing keeps what it orbits.
	rec, s = solarSave(t, h, "PUT", base+"/bodies/"+idStr(s.Bodies[3].ID), `{"kind":"star","name":"Io II","body_type":"ocean","radius_km":1800,"orbit_km":5e5,"parent_id":null}`)
	if rec.Code != 200 || s.Bodies[3].Kind != "moon" || *s.Bodies[3].ParentID != giant || s.Bodies[3].Name != "Io II" {
		t.Errorf("edit: %d %+v", rec.Code, s.Bodies[3])
	}
	_, s = solarSave(t, h, "DELETE", base+"/bodies/"+idStr(giant), ``)
	if len(s.Bodies) != 2 {
		t.Errorf("deleting the planet takes its moon: %+v", s.Bodies)
	}
}

// An older story gets the empty table.
func TestMigrationAddsSolarTable(t *testing.T) {
	mgr, _ := newTestServer(t)
	app, _ := mgr.open(createStoryDirect(t, mgr, "Old"))
	rollBackSchema30(t, app.db)
	if err := ensureSchema(app.db); err != nil {
		t.Fatal(err)
	}
	s, err := loadSolarSystem(app.db)
	if err != nil || len(s.Bodies) != 0 {
		t.Errorf("%v, %+v", err, s)
	}
}
