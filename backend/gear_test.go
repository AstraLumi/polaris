package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func postGear(t *testing.T, h http.Handler, id string, fields map[string]string, file []byte) (*httptest.ResponseRecorder, gearDetail) {
	t.Helper()
	fn := ""
	ff := ""
	if file != nil {
		ff, fn = "icon", "g.png"
	}
	body, ct := form(t, fields, ff, fn, file)
	rec := do(t, h, "POST", "/api/s/"+id+"/gear", body, ct)
	var g gearDetail
	json.Unmarshal(rec.Body.Bytes(), &g)
	return rec, g
}

type versionOut struct {
	ID       int64                 `json:"id"`
	Gear     map[string]gearDetail `json:"gear"`
	Computed ComputedStats         `json:"computed"`
}

func gearTestSetup(t *testing.T) (http.Handler, storyInfo, int64) {
	t.Helper()
	_, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Gearworld"})
	body, ct := form(t, map[string]string{"name": "Aria", "level": "10"}, "", "", nil)
	rec := do(t, h, "POST", "/api/s/"+s.ID+"/characters", body, ct)
	if rec.Code != 200 {
		t.Fatalf("create character: %d %s", rec.Code, rec.Body.String())
	}
	var ch struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(rec.Body.Bytes(), &ch)
	rec = do(t, h, "GET", "/api/s/"+s.ID+"/characters/"+idStr(ch.ID)+"/current", nil, "")
	var v versionOut
	json.Unmarshal(rec.Body.Bytes(), &v)
	if v.ID == 0 {
		t.Fatalf("no current version: %s", rec.Body.String())
	}
	return h, s, v.ID
}

func putVersion(t *testing.T, h http.Handler, sid string, vid int64, gear string) versionOut {
	t.Helper()
	fields := map[string]string{"name": "Aria", "level": "10"}
	if gear != "" {
		fields["gear"] = gear
	}
	body, ct := form(t, fields, "", "", nil)
	rec := do(t, h, "PUT", "/api/s/"+sid+"/versions/"+idStr(vid), body, ct)
	if rec.Code != 200 {
		t.Fatalf("save version: %d %s", rec.Code, rec.Body.String())
	}
	var v versionOut
	json.Unmarshal(rec.Body.Bytes(), &v)
	return v
}

func TestGearValidation(t *testing.T) {
	_, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "G"})
	good := map[string]string{"name": "Iron Helm", "slot": "helmet", "weight": "3.5"}
	rec, g := postGear(t, h, s.ID, good, nil)
	if rec.Code != 200 || g.Slot != "helmet" || g.Weight != 3.5 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	for name, f := range map[string]map[string]string{
		"duplicate name": good,
		"no name":        {"slot": "helmet"},
		"no slot":        {"name": "A"},
		"bad slot":       {"name": "B", "slot": "glove1"},
		"negative":       {"name": "C", "slot": "ring", "weight": "-1"},
		"bad modifiers":  {"name": "D", "slot": "ring", "modifiers": "{nope"},
	} {
		if rec, _ := postGear(t, h, s.ID, f, nil); rec.Code != 400 {
			t.Errorf("%s should be refused, got %d", name, rec.Code)
		}
	}
	// A blank weight is allowed (0).
	if rec, g := postGear(t, h, s.ID, map[string]string{"name": "Feather", "slot": "cape"}, nil); rec.Code != 200 || g.Weight != 0 {
		t.Errorf("blank weight: %d %v", rec.Code, g.Weight)
	}
	// Built-in icons: allowed ones stick, unknown ones are ignored.
	_, g = postGear(t, h, s.ID, map[string]string{"name": "Pretty", "slot": "ring", "icon_choice": "builtin:ring"}, nil)
	if g.IconPath != "builtin:ring" {
		t.Errorf("builtin icon: %q", g.IconPath)
	}
	_, g = postGear(t, h, s.ID, map[string]string{"name": "Odd", "slot": "ring", "icon_choice": "builtin:flame"}, nil)
	if g.IconPath != "" {
		t.Errorf("spell-only icon must be ignored for gear: %q", g.IconPath)
	}
}

func TestGearStatsAreFlatAndStack(t *testing.T) {
	h, s, vid := gearTestSetup(t)
	_, helm := postGear(t, h, s.ID, map[string]string{"name": "Helm", "slot": "helmet", "modifiers": `{"attack":5,"vit":2,"resist_fire":0.1,"cooking":3}`}, nil)
	_, glove := postGear(t, h, s.ID, map[string]string{"name": "Glove", "slot": "glove", "modifiers": `{"str":1}`}, nil)
	_, ring := postGear(t, h, s.ID, map[string]string{"name": "Ring", "slot": "ring"}, nil)

	bare := putVersion(t, h, s.ID, vid, `{}`)

	// Stored as flat modifiers even for Base Stats.
	rec := do(t, h, "GET", "/api/s/"+s.ID+"/gear/"+idStr(helm.ID), nil, "")
	var got gearDetail
	json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Modifiers["attack"] != 5 || got.Modifiers["resist_fire"] != 0.1 {
		t.Fatalf("modifiers: %v", got.Modifiers)
	}

	// Attack is level 10; if "+5" were per-level it would be +50.
	dressed := putVersion(t, h, s.ID, vid, `{"helmet":`+idStr(helm.ID)+`,"glove1":`+idStr(glove.ID)+`,"glove2":`+idStr(glove.ID)+`,"ring1":`+idStr(helm.ID)+`,"ring2":`+idStr(ring.ID)+`,"bogus":1}`)
	if len(dressed.Gear) != 4 {
		t.Fatalf("expected helmet, 2 gloves, ring2 equipped (helm in a ring slot refused): %v", keys(dressed.Gear))
	}
	if _, bad := dressed.Gear["ring1"]; bad {
		t.Fatal("a helmet must not fit a ring slot")
	}
	// Derived Attack includes 2*STR; STR rose by 2 (glove worn twice), so +4 from that.
	if d := dressed.Computed.Base.Attack - bare.Computed.Base.Attack; d != 5+4 {
		t.Errorf("attack diff = %v, want 9 (flat 5 + 4 from the worn-twice glove's STR)", d)
	}
	if d := dressed.Computed.Primary.STR - bare.Computed.Primary.STR; d != 2 {
		t.Errorf("same glove in two slots should count twice: STR diff %v", d)
	}
	if d := dressed.Computed.Primary.VIT - bare.Computed.Primary.VIT; d != 2 {
		t.Errorf("vit diff %v", d)
	}
	if d := dressed.Computed.Elemental.Fire - bare.Computed.Elemental.Fire; d < 0.0999 || d > 0.1001 {
		t.Errorf("fire resistance diff %v", d)
	}
	if d := dressed.Computed.Lifeskills.Cooking - bare.Computed.Lifeskills.Cooking; d != 3 {
		t.Errorf("lifeskill diff %v", d)
	}

	// The live preview takes the same pieces.
	pv := do(t, h, "POST", "/api/s/"+s.ID+"/compute-preview", strings.NewReader(`{"level":10,"gear_ids":[`+idStr(helm.ID)+`]}`), "application/json")
	var pc ComputedStats
	json.Unmarshal(pv.Body.Bytes(), &pc)
	if pc.Primary.VIT != 2 {
		t.Errorf("preview vit %v", pc.Primary.VIT)
	}

	// Leaving `gear` out of a save leaves the outfit alone.
	again := putVersion(t, h, s.ID, vid, "")
	if len(again.Gear) != 4 {
		t.Errorf("omitting gear changed the outfit: %v", keys(again.Gear))
	}

	// A new version starts with the same outfit.
	rec = do(t, h, "GET", "/api/s/"+s.ID+"/versions/"+idStr(vid), nil, "")
	var cur struct {
		CharacterID int64 `json:"character_id"`
	}
	json.Unmarshal(rec.Body.Bytes(), &cur)
	if cur.CharacterID != 0 {
		rec = do(t, h, "POST", "/api/s/"+s.ID+"/characters/"+idStr(cur.CharacterID)+"/versions",
			strings.NewReader(`{"version_date":"","version_reference":"","clone_from_version_id":`+idStr(vid)+`}`), "application/json")
		var nv struct {
			ID int64 `json:"id"`
		}
		json.Unmarshal(rec.Body.Bytes(), &nv)
		r2 := do(t, h, "GET", "/api/s/"+s.ID+"/versions/"+idStr(nv.ID), nil, "")
		var v2 versionOut
		json.Unmarshal(r2.Body.Bytes(), &v2)
		if len(v2.Gear) != 4 {
			t.Errorf("new version should copy the outfit, got %v (%s)", keys(v2.Gear), rec.Body.String())
		}
	}

	// Changing a piece's type un-equips it; deleting removes it and its bonuses.
	body, ct := form(t, map[string]string{"name": "Glove", "slot": "ring", "modifiers": `{"str":1}`}, "", "", nil)
	if rec := do(t, h, "PUT", "/api/s/"+s.ID+"/gear/"+idStr(glove.ID), body, ct); rec.Code != 200 {
		t.Fatalf("retype: %d %s", rec.Code, rec.Body.String())
	}
	after := putVersion(t, h, s.ID, vid, "")
	if _, ok := after.Gear["glove1"]; ok {
		t.Error("retyped gear should leave the glove slots")
	}
	if rec := do(t, h, "DELETE", "/api/s/"+s.ID+"/gear/"+idStr(helm.ID), nil, ""); rec.Code != 200 {
		t.Fatalf("delete: %d", rec.Code)
	}
	gone := putVersion(t, h, s.ID, vid, "")
	if _, ok := gone.Gear["helmet"]; ok {
		t.Error("deleted gear still equipped")
	}
	if gone.Computed.Primary.VIT != bare.Computed.Primary.VIT {
		t.Error("deleted gear still gives stats")
	}
}

func keys(m map[string]gearDetail) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestGearIsCopiedWithAssets(t *testing.T) {
	mgr, h := newTestServer(t)
	a := createStory(t, h, map[string]string{"name": "A"})
	_, g := postGear(t, h, a.ID, map[string]string{"name": "Helm", "slot": "helmet", "weight": "2", "modifiers": `{"attack":5}`}, []byte("ICONBYTES"))
	if !strings.HasPrefix(g.IconPath, "/uploads/s/"+a.ID+"/gear/") {
		t.Fatalf("icon url: %s", g.IconPath)
	}
	b := createStory(t, h, map[string]string{"name": "B", "copy_assets_from": a.ID})
	rec := do(t, h, "GET", "/api/s/"+b.ID+"/gear", nil, "")
	var list []gearDetail
	json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 || list[0].Weight != 2 || list[0].Modifiers["attack"] != 5 || list[0].Slot != "helmet" {
		t.Fatalf("copied gear: %s", rec.Body.String())
	}
	if !strings.Contains(list[0].IconPath, b.ID) {
		t.Fatalf("copied icon should be under the new story: %s", list[0].IconPath)
	}
	app, _ := mgr.open(b.ID)
	var icon string
	app.db.QueryRow(`SELECT icon_path FROM gear`).Scan(&icon)
	if data, err := os.ReadFile(filepath.Join(app.uploadsDir, filepath.FromSlash(icon))); err != nil || string(data) != "ICONBYTES" {
		t.Fatalf("icon file not copied: %v", err)
	}
	// Deleting in B leaves A's gear and file alone.
	do(t, h, "DELETE", "/api/s/"+b.ID+"/gear/"+idStr(list[0].ID), nil, "")
	if l := listGearNames(t, h, a.ID); len(l) != 1 {
		t.Fatal("copy was not independent")
	}
}

func listGearNames(t *testing.T, h http.Handler, id string) []string {
	return listNames(t, h, id, "gear")
}

func TestSlotMapping(t *testing.T) {
	for _, s := range gearEquipSlots {
		if !isGearSlotType(s.typ) {
			t.Errorf("equipment slot %s accepts unknown type %s", s.key, s.typ)
		}
	}
	for _, typ := range gearSlotTypes {
		found := false
		for _, s := range gearEquipSlots {
			found = found || s.typ == typ
		}
		if !found {
			t.Errorf("no equipment slot takes %s gear", typ)
		}
	}
}

func TestMigrationAddsGearTables(t *testing.T) {
	mgr, _ := newTestServer(t)
	id := createStoryDirect(t, mgr, "Old")
	app, _ := mgr.open(id)
	app.db.Exec(`INSERT INTO classes (name) VALUES ('Keeper')`)
	for _, q := range []string{`ALTER TABLE character_story DROP COLUMN status`, `ALTER TABLE locations DROP COLUMN is_city`, `ALTER TABLE locations DROP COLUMN is_capital`, `DROP TABLE wiki_infobox`, `DROP TABLE wiki_sections`, `DROP TABLE wiki_entries`, `DROP TABLE character_gear`, `DROP TABLE gear`, `UPDATE schema_meta SET value = '11' WHERE key = 'schema_version'`} {
		if _, err := app.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err := ensureSchema(app.db); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM gear`).Scan(&n); err != nil {
		t.Fatalf("gear table missing after migration: %v", err)
	}
	app.db.QueryRow(`SELECT COUNT(*) FROM classes`).Scan(&n)
	if n != 1 {
		t.Fatal("migration must keep existing data")
	}
	var v string
	app.db.QueryRow(`SELECT value FROM schema_meta WHERE key = 'schema_version'`).Scan(&v)
	if v != currentSchemaVersion {
		t.Fatalf("version %s", v)
	}
}
