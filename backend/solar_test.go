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

	rec, s := solarSave(t, h, "POST", base+"/bodies", `{"kind":"star","name":"Sol","body_type":"star","radius_km":696000,"temperature_k":5772,"mass_kg":1.989e30}`)
	if rec.Code != 200 || len(s.Bodies) != 1 {
		t.Fatalf("star: %d %s", rec.Code, rec.Body.String())
	}
	star := s.Bodies[0].ID
	if rec, _ := solarSave(t, h, "POST", base+"/bodies", `{"kind":"star","name":"Second","body_type":"star","radius_km":1000,"temperature_k":5772,"mass_kg":1e30}`); rec.Code != 400 {
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

// A star has a class, a temperature and a mass; planets have none.
func TestStarClasses(t *testing.T) {
	_, h := newTestServer(t)
	st := createStory(t, h, map[string]string{"name": "Stars"})
	base := "/api/s/" + st.ID + "/solar"
	rec, s := solarSave(t, h, "POST", base+"/bodies",
		`{"kind":"star","name":"Remnant","body_type":"star","star_class":"black_hole","radius_km":29.5,"temperature_k":0,"mass_kg":1.989e31,"free_values":15}`)
	if rec.Code != 200 {
		t.Fatalf("black hole: %d %s", rec.Code, rec.Body.String())
	}
	b := s.Bodies[0]
	if b.StarClass != "black_hole" || *b.TemperatureK != 0 || *b.MassKg != 1.989e31 || b.FreeValues != freeAll {
		t.Errorf("star: %+v", b)
	}
	_, s = solarSave(t, h, "POST", base+"/bodies",
		`{"kind":"planet","name":"P","body_type":"rocky","radius_km":6000,"orbit_km":1e8,"parent_id":`+idStr(b.ID)+`,"star_class":"red_giant","mass_kg":5}`)
	if p := s.Bodies[1]; p.StarClass != "" || p.MassKg != nil {
		t.Errorf("a planet keeps no star values: %+v", p)
	}
	for _, bad := range []string{
		`{"kind":"star","name":"X","body_type":"star","star_class":"quasar","radius_km":1,"temperature_k":1,"mass_kg":1}`,
		`{"kind":"star","name":"X","body_type":"star","radius_km":1,"temperature_k":1}`,
	} {
		if rec, _ := solarSave(t, h, "PUT", base+"/bodies/"+idStr(b.ID), bad); rec.Code != 400 {
			t.Errorf("%s: %d", bad, rec.Code)
		}
	}
}

// Stars made before classes are Sun-like, with the mass their size gave.
func TestMigrationGivesStarsAClass(t *testing.T) {
	mgr, _ := newTestServer(t)
	app, _ := mgr.open(createStoryDirect(t, mgr, "Old"))
	rollBackSchema31(t, app.db)
	if _, err := app.db.Exec(`INSERT INTO celestial_bodies (kind, name, body_type, radius_km) VALUES ('star', 'Sol', 'star', 696000)`); err != nil {
		t.Fatal(err)
	}
	if err := ensureSchema(app.db); err != nil {
		t.Fatal(err)
	}
	s, _ := loadSolarSystem(app.db)
	b := s.Bodies[0]
	if b.StarClass != "yellow_dwarf" || *b.TemperatureK != 5772 || math.Abs(*b.MassKg/1.989e30-1) > 0.01 {
		t.Errorf("%+v, mass %v", b, *b.MassKg)
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
