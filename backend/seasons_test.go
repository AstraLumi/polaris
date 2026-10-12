package main

import (
	"encoding/json"
	"testing"
)

type settingsJSON struct {
	MonthsPerYear int            `json:"months_per_year"`
	Seasons       seasonSettings `json:"seasons"`
}

// With nothing saved, the seasons are winter to fall, a quarter each,
// winter starting with the last month.
func TestDefaultSeasons(t *testing.T) {
	_, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Seasons"})
	var got settingsJSON
	json.Unmarshal(do(t, h, "GET", "/api/s/"+s.ID+"/settings", nil, "").Body.Bytes(), &got)
	ss := got.Seasons
	if len(ss.Seasons) != 4 || ss.Seasons[0].Name != "Winter" || ss.Seasons[0].Months != 3 || ss.Start != "01-12-0001" || ss.Hemisphere != "north" {
		t.Fatalf("defaults: %+v", ss)
	}
}

func TestSaveSeasons(t *testing.T) {
	_, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Seasons"})
	url := "/api/s/" + s.ID + "/settings/seasons"
	put := func(v any) (int, seasonSettings) {
		rec := do(t, h, "PUT", url, jsonBody(v), "application/json")
		var out seasonSettings
		json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}

	code, out := put(seasonSettings{
		Seasons: []season{
			{ID: "winter", Name: " Long Winter ", Months: 36},
			{Name: "Thaw", Months: 2.5, Color: "red"},
			{ID: "winter", Name: "Echo", Months: 1},
		},
		Start: "15-03-0100", Hemisphere: "south", Unit: "F",
	})
	if code != 200 {
		t.Fatalf("save: %d", code)
	}
	if out.Seasons[0].Name != "Long Winter" || out.Seasons[1].ID == "" || out.Seasons[1].Color != "" || out.Seasons[2].ID == "winter" {
		t.Errorf("cleaned: %+v", out.Seasons)
	}
	// A new season never takes an id a later one already has.
	_, ids := put(seasonSettings{Seasons: []season{{Name: "New", Months: 1}, {ID: "s1", Name: "Old", Months: 1}}, Start: "01-01-0001"})
	if ids.Seasons[0].ID == "s1" || ids.Seasons[1].ID != "s1" {
		t.Errorf("ids: %+v", ids.Seasons)
	}
	if out.Hemisphere != "south" || out.Unit != "F" || out.Start != "15-03-0100" {
		t.Errorf("saved: %+v", out)
	}

	for name, bad := range map[string]seasonSettings{
		"no name":        {Seasons: []season{{Months: 3}}, Start: "01-01-0001"},
		"zero months":    {Seasons: []season{{Name: "A"}}, Start: "01-01-0001"},
		"no start":       {Seasons: []season{{Name: "A", Months: 1}}},
		"day 31":         {Seasons: []season{{Name: "A", Months: 1}}, Start: "31-01-0001"},
		"year 0":         {Seasons: []season{{Name: "A", Months: 1}}, Start: "01-01-0000"},
		"too many years": {Seasons: []season{{Name: "A", Months: 5000}}, Start: "01-01-0001"},
	} {
		if code, _ := put(bad); code != 400 {
			t.Errorf("%s: %d", name, code)
		}
	}

	// No seasons at all is allowed, and stays that way.
	if code, out := put(seasonSettings{Seasons: []season{}}); code != 200 || len(out.Seasons) != 0 {
		t.Errorf("no seasons: %d %+v", code, out)
	}
}

// A region's temperatures save with its article; a town has none.
func TestRegionClimate(t *testing.T) {
	mgr, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Climate"})
	app, _ := mgr.open(s.ID)
	app.db.Exec(`INSERT INTO locations (id, name, kind) VALUES (1, 'Kemet-Vib', 'major'), (2, 'Zalagnord', 'minor')`)
	base := "/api/s/" + s.ID + "/wiki/location/"

	type article struct {
		HasClimate bool         `json:"has_climate"`
		Climate    []climateRow `json:"climate"`
	}
	f := func(v float64) *float64 { return &v }
	rec := do(t, h, "PUT", base+"1", jsonBody(map[string]any{
		"climate": []climateRow{
			{SeasonID: "summer", Low: f(40), High: f(25)},
			{SeasonID: "winter", Low: f(10)},
			{SeasonID: "spring"},
		},
	}), "application/json")
	if rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	var a article
	json.Unmarshal(do(t, h, "GET", base+"1", nil, "").Body.Bytes(), &a)
	if !a.HasClimate || len(a.Climate) != 2 {
		t.Fatalf("climate: %+v", a)
	}
	for _, c := range a.Climate {
		if c.SeasonID == "summer" && (*c.Low != 25 || *c.High != 40) {
			t.Errorf("a range typed the wrong way round is swapped: %+v", c)
		}
		if c.SeasonID == "winter" && (c.High != nil || *c.Low != 10) {
			t.Errorf("one end alone: %+v", c)
		}
	}

	// Saving the text alone leaves the temperatures.
	do(t, h, "PUT", base+"1", jsonBody(map[string]any{"summary": "Hot."}), "application/json")
	json.Unmarshal(do(t, h, "GET", base+"1", nil, "").Body.Bytes(), &a)
	if len(a.Climate) != 2 {
		t.Errorf("a text save dropped the climate: %+v", a.Climate)
	}

	json.Unmarshal(do(t, h, "GET", base+"2", nil, "").Body.Bytes(), &a)
	if a.HasClimate {
		t.Error("a minor location has no climate")
	}
	if rec := do(t, h, "PUT", base+"2", jsonBody(map[string]any{"climate": []climateRow{{SeasonID: "summer", Low: f(1)}}}), "application/json"); rec.Code != 400 {
		t.Errorf("climate on a town: %d", rec.Code)
	}

	app.db.Exec(`DELETE FROM locations WHERE id = 1`)
	var n int
	app.db.QueryRow(`SELECT COUNT(*) FROM location_climate`).Scan(&n)
	if n != 0 {
		t.Errorf("a deleted region's climate stays behind: %d rows", n)
	}
}

func TestMapEquator(t *testing.T) {
	_, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Equator"})
	base := "/api/s/" + s.ID + "/map"
	var m struct {
		Equator mapEquator `json:"equator"`
	}
	json.Unmarshal(do(t, h, "GET", base, nil, "").Body.Bytes(), &m)
	if m.Equator.Show {
		t.Error("the equator starts hidden")
	}
	if rec := do(t, h, "PUT", base+"/equator", jsonBody(mapEquator{Show: true, Row: 40.5, Center: 12, Bend: -3}), "application/json"); rec.Code != 200 {
		t.Fatalf("save: %d", rec.Code)
	}
	json.Unmarshal(do(t, h, "GET", base, nil, "").Body.Bytes(), &m)
	if m.Equator != (mapEquator{Show: true, Row: 40.5, Center: 12, Bend: -3}) {
		t.Errorf("equator: %+v", m.Equator)
	}
	if rec := do(t, h, "PUT", base+"/equator", jsonBody(map[string]any{"show": true, "row": 1e9}), "application/json"); rec.Code != 400 {
		t.Errorf("an equator far off the map: %d", rec.Code)
	}
}

func TestMigrationAddsRegionClimate(t *testing.T) {
	mgr, _ := newTestServer(t)
	app, _ := mgr.open(createStoryDirect(t, mgr, "Old"))
	rollBackSchema26(t, app.db)
	app.db.Exec(`INSERT INTO locations (id, name, kind) VALUES (1, 'North', 'major')`)
	if err := ensureSchema(app.db); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`INSERT INTO location_climate (location_id, season_id, low) VALUES (1, 'winter', -5)`); err != nil {
		t.Fatal(err)
	}
	var n int
	app.db.QueryRow(`SELECT COUNT(*) FROM locations`).Scan(&n)
	if n != 1 {
		t.Fatal("migration must keep existing data")
	}
}
