package main

import (
	"database/sql"
	"encoding/json"
	"math"
	"math/rand"
	"strings"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// legacyStats is how Polaris calculated stats before they could be edited
// (stats_formulas.go up to 1.6), kept so the default stat system can be
// checked against it.
func legacyStats(p map[string]float64, bases map[string]float64, level, age, heightM, weightKG float64) map[string]float64 {
	luckFactor := func(age float64) float64 {
		pts := [][2]float64{{0, 0.75}, {18, 0}, {100, -0.75}, {200, -1}}
		if age <= 0 {
			return 0.75
		}
		if age >= 200 {
			return -1
		}
		for i := 0; i < 3; i++ {
			x0, y0, x1, y1 := pts[i][0], pts[i][1], pts[i+1][0], pts[i+1][1]
			if age >= x0 && age <= x1 {
				return math.Round((y0+(age-x0)/(x1-x0)*(y1-y0))*100) / 100
			}
		}
		return 0
	}
	v := map[string]float64{
		"hp":                    10 + p["vit"]*10 + level*2,
		"mp":                    10 + p["wis"]*5 + p["intel"] + level*2,
		"attack":                1 + p["str"]*2 + p["dex"],
		"potency":               1 + p["intel"]*2 + p["wis"],
		"defense":               1 + p["def"]*3,
		"resistance":            1 + p["res"]*3,
		"speed":                 1 + p["agl"]*2,
		"luck":                  clamp(1+luckFactor(age), 0, 2),
		"carry_limit":           (weightKG*0.13 + math.Max(0, heightM-1)*2) * (1 + math.Pow(p["str"]/10, 2)),
		"fall_damage_threshold": 2.0 + (heightM-1.7)*1.0 - (weightKG-70)*0.02,
		"sanity":                bases["sanity"],
		"heat_threshold":        bases["heat_threshold"] - (weightKG-70)*0.05,
		"cold_threshold":        bases["cold_threshold"] - (weightKG-70)*0.05,
	}
	for _, k := range []string{"resist_water", "resist_fire", "resist_wind", "resist_earth", "resist_ice", "resist_thunder", "resist_impact"} {
		v[k] = 1 + bases[k]
	}
	for _, k := range []string{"cooking", "crafting", "alchemy", "hunting", "gathering", "farming", "blessing", "enchanting", "smithing"} {
		v[k] = bases[k]
	}
	return v
}

// The default stat system gives every character the same numbers the old
// built-in formulas did, rounded the way the sheet shows them.
func TestDefaultStatSystemMatchesTheOldFormulas(t *testing.T) {
	sys := defaultStatSystem()
	if issues := sys.compile(); len(issues) > 0 {
		t.Fatalf("defaults don't compile: %+v", issues)
	}
	rng := rand.New(rand.NewSource(1))
	for n := 0; n < 500; n++ {
		values := map[string]float64{}
		points := map[string]float64{}
		for _, k := range sys.keys("primary") {
			points[k] = float64(rng.Intn(40))
			values[k] = points[k]
		}
		bases := map[string]float64{}
		for _, d := range sys.Stats {
			if d.Category != "primary" && sys.typed(d.Key) {
				bases[d.Key] = math.Round(rng.Float64()*100) / 10
				values[d.Key] = bases[d.Key]
			}
		}
		level := float64(1 + rng.Intn(100))
		age := float64(rng.Intn(260)) - 10
		height := 100 + rng.Float64()*120
		weight := 30 + rng.Float64()*120

		got := sys.calculate(statInputs{Level: level, Age: age, HeightCM: height, WeightKG: weight, Values: values}, nil, nil)
		want := legacyStats(points, bases, level, age, height/100, weight)
		for _, d := range sys.Stats {
			if d.Category == "primary" {
				if got[d.Key] != points[d.Key] {
					t.Fatalf("%s: got %v, want the %v points spent", d.Key, got[d.Key], points[d.Key])
				}
				continue
			}
			tolerance := 0.5*math.Pow(10, -float64(d.Decimals)) + 1e-9
			if math.Abs(got[d.Key]-want[d.Key]) > tolerance {
				t.Fatalf("%s (level %v, age %v, %.1f cm, %.1f kg): got %v, old formula gave %v",
					d.Key, level, age, height, weight, got[d.Key], want[d.Key])
			}
		}
	}
}

func TestFormulaParsing(t *testing.T) {
	env := func(name string) float64 {
		return map[string]float64{"vit": 3, "level": 10, "str": 20}[name]
	}
	for src, want := range map[string]float64{
		"":                                    0,
		"10 + [vit] * 10 + [level] * 2":       60,
		"2 ^ 3 ^ 2":                           512, // right to left
		"-2 ^ 2":                              -4,
		"([str] / 10) ^ 2":                    4,
		"min(3, 1, 2) + max(1, 5)":            6,
		"round(2.345, 2)":                     2.35,
		"clamp(15, 0, 10)":                    10,
		"curve(9, 0, 0, 18, 1)":               0.5,
		"curve(-5, 0, 0.75, 18, 0)":           0.75,
		"1 / 0":                               0,
		"sqrt(0 - 4)":                         0,
		"floor(1.9) + ceil(1.1) + abs(0 - 2)": 5,
	} {
		f, err := parseFormula(src)
		if err != nil {
			t.Errorf("%q: %v", src, err)
			continue
		}
		if got := f.eval(env); !near(got, want) {
			t.Errorf("%q = %v, want %v", src, got, want)
		}
	}
	for src, code := range map[string]string{
		"1 +":       "incomplete",
		"(1 + 2":    "unclosed",
		"1 + 2)":    "unopened",
		"foo(1)":    "unknown_function",
		"min(1)":    "arg_count",
		"[Bad Key]": "unknown_value",
		"1 2":       "syntax",
		"1 $ 2":     "syntax",
		"1..2":      "bad_number",
	} {
		_, err := parseFormula(src)
		fe, ok := err.(*formulaError)
		if !ok || fe.Code != code {
			t.Errorf("%q: got %v, want %s", src, err, code)
		}
	}
}

func issueCodes(issues []statIssue) string {
	var out []string
	for _, i := range issues {
		out = append(out, i.Key+"/"+i.Field+"/"+i.Code)
	}
	return strings.Join(out, ",")
}

func TestStatSystemChecks(t *testing.T) {
	base := func(key, formula string) statDef {
		return statDef{Key: key, Category: "base", Name: key, Formula: formula}
	}
	cases := []struct {
		name  string
		sys   statSystem
		codes string
	}{
		{"unknown stat", statSystem{PointsFormula: "[level]", Stats: []statDef{base("hp", "[vit]")}}, "hp/formula/unknown_value"},
		{"loop", statSystem{PointsFormula: "[level]", Stats: []statDef{base("a", "[b]"), base("b", "[a] + 1")}}, "a/formula/loop"},
		{"points can't read stats", statSystem{PointsFormula: "[vit]", Stats: []statDef{{Key: "vit", Category: "primary", Name: "VIT"}}}, "/points/unknown_value"},
		{"reserved key", statSystem{PointsFormula: "[level]", Stats: []statDef{base("level", "1")}}, "level/key/reserved_key"},
		{"duplicate key", statSystem{PointsFormula: "[level]", Stats: []statDef{base("hp", "1"), base("hp", "2")}}, "hp/key/duplicate_key"},
		{"no name", statSystem{PointsFormula: "[level]", Stats: []statDef{{Key: "hp", Category: "base", Name: "  "}}}, "hp/name/no_name"},
		{"stats can read each other", statSystem{PointsFormula: "[level]", Stats: []statDef{base("stamina", "[hp] / 2"), base("hp", "10")}}, ""},
	}
	for _, c := range cases {
		if got := issueCodes(c.sys.compile()); got != c.codes {
			t.Errorf("%s: got %q, want %q", c.name, got, c.codes)
		}
	}

	// A stat reading another is worked out after it, whatever the order.
	sys := statSystem{PointsFormula: "[level]", Stats: []statDef{base("stamina", "[hp] / 2"), base("hp", "10 + [level]")}}
	sys.compile()
	if got := sys.calculate(statInputs{Level: 4, Values: map[string]float64{}}, nil, nil); got["stamina"] != 7 {
		t.Errorf("stamina = %v, want 7", got["stamina"])
	}
}

func statSystemOf(t *testing.T, db *sql.DB) statSystem {
	t.Helper()
	s, err := readStatSystem(db)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// Saving the stat system through the API: a removed stat takes its typed
// numbers and bonuses with it, a broken formula is refused with its problem,
// and characters follow the new formulas.
func TestStatSystemAPI(t *testing.T) {
	mgr, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Stats"})
	app, _ := mgr.open(s.ID)
	base := "/api/s/" + s.ID

	body, ct := form(t, map[string]string{"name": "Aria", "level": "5", "race": "Elf"}, "", "", nil)
	if rec := do(t, h, "POST", base+"/characters", body, ct); rec.Code != 200 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var vid int64
	app.db.QueryRow(`SELECT id FROM character_versions`).Scan(&vid)
	for _, q := range []string{
		`INSERT INTO character_special_bases (version_id, stat_key, value) VALUES (` + idStr(vid) + `, 'vit', 3), (` + idStr(vid) + `, 'cooking', 4)`,
		`INSERT INTO stat_modifiers (source_type, source_id, target_stat, value) SELECT 'race', id, 'cooking', 2 FROM races`,
	} {
		if _, err := app.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}

	sys := statSystemOf(t, app.db)
	var kept []statDef
	for _, d := range sys.Stats {
		if d.Key == "cooking" {
			continue
		}
		if d.Key == "hp" {
			d.Formula = "[vit] * 100"
		}
		kept = append(kept, d)
	}
	kept = append(kept, statDef{Key: "stamina", Category: "base", Name: "Stamina", Formula: "[hp] / 2"})
	sys.Stats = kept

	broken := sys
	broken.Stats = append(append([]statDef{}, kept...), statDef{Key: "oops", Category: "special", Name: "Oops", Formula: "[cooking] +"})
	raw, _ := json.Marshal(broken)
	rec := do(t, h, "PUT", base+"/stat-system", strings.NewReader(string(raw)), "application/json")
	if rec.Code != 422 || !strings.Contains(rec.Body.String(), `"incomplete"`) {
		t.Fatalf("broken formula: %d %s", rec.Code, rec.Body.String())
	}

	raw, _ = json.Marshal(sys)
	if rec := do(t, h, "PUT", base+"/stat-system", strings.NewReader(string(raw)), "application/json"); rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	var n int
	app.db.QueryRow(`SELECT COUNT(*) FROM character_special_bases WHERE stat_key = 'cooking'`).Scan(&n)
	app.db.QueryRow(`SELECT COUNT(*) + ? FROM stat_modifiers WHERE target_stat = 'cooking'`, n).Scan(&n)
	if n != 0 {
		t.Errorf("%d rows still refer to the removed Cooking", n)
	}

	rec = do(t, h, "GET", base+"/versions/"+idStr(vid), nil, "")
	var v struct {
		Build    map[string]int     `json:"build"`
		Computed map[string]float64 `json:"computed"`
	}
	json.Unmarshal(rec.Body.Bytes(), &v)
	if v.Build["vit"] != 3 || v.Computed["hp"] != 300 || v.Computed["stamina"] != 150 {
		t.Errorf("after the change: build %v, hp %v, stamina %v", v.Build, v.Computed["hp"], v.Computed["stamina"])
	}
	if _, ok := v.Computed["cooking"]; ok {
		t.Error("the removed Cooking is still on the sheet")
	}
}

// Moving a stat into Base Stats makes the bonuses of classes and races count
// per level; gear stays flat.
func TestBonusesFollowTheirStatsCategory(t *testing.T) {
	mgr, _ := newTestServer(t)
	app, _ := mgr.open(createStoryDirect(t, mgr, "Scaling"))
	for _, q := range []string{
		`INSERT INTO races (id, name) VALUES (1, 'Elf')`,
		`INSERT INTO stat_modifiers (source_type, source_id, target_stat, scaling, value) VALUES ('race', 1, 'cooking', 'flat', 2), ('item', 1, 'cooking', 'flat', 1)`,
		`UPDATE stat_defs SET category = 'base' WHERE key = 'cooking'`,
	} {
		if _, err := app.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	tx, _ := app.db.Begin()
	if err := rescaleModifiers(tx); err != nil {
		t.Fatal(err)
	}
	tx.Commit()
	rows, _ := app.db.Query(`SELECT source_type || ':' || scaling FROM stat_modifiers ORDER BY id`)
	var got []string
	for rows.Next() {
		var s string
		rows.Scan(&s)
		got = append(got, s)
	}
	rows.Close()
	if strings.Join(got, ",") != "race:per_level,item:flat" {
		t.Errorf("scaling: %v", got)
	}
}

// Upgrading to schema 22 gives a story the default stat system and moves the
// points spent on primary stats over, so every sheet reads as before.
func TestMigrationAddsTheStatSystem(t *testing.T) {
	mgr, _ := newTestServer(t)
	app, _ := mgr.open(createStoryDirect(t, mgr, "Old"))
	rollBackSchema22(t, app.db)
	for _, q := range []string{
		`INSERT INTO characters (id) VALUES (1)`,
		`INSERT INTO character_versions (id, character_id, is_current, name, level) VALUES (1, 1, 1, 'Aria', 20)`,
		`INSERT INTO character_story (version_id) VALUES (1)`,
		`INSERT INTO character_build (version_id, vit, str, agl) VALUES (1, 4, 10, 0)`,
		`INSERT INTO character_special_bases (version_id, stat_key, value) VALUES (1, 'sanity', 40)`,
	} {
		if _, err := app.db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if err := ensureSchema(app.db); err != nil {
		t.Fatal(err)
	}
	if got := statSystemOf(t, app.db); len(got.Stats) != len(defaultStatSystem().Stats) || got.PointsFormula != "[level]" {
		t.Errorf("stat system after the upgrade: %d stats, points %q", len(got.Stats), got.PointsFormula)
	}
	d, err := buildVersionDetail(app.db, "1")
	if err != nil {
		t.Fatal(err)
	}
	if d.Build["vit"] != 4 || d.Build["str"] != 10 || d.Build["agl"] != 0 || d.SpecialBases["sanity"] != 40 {
		t.Errorf("typed values after the upgrade: build %v, bases %v", d.Build, d.SpecialBases)
	}
	if d.Computed["hp"] != 10+4*10+20*2 || d.Computed["sanity"] != 40 || d.Points != 20 {
		t.Errorf("hp %v, sanity %v, points %v", d.Computed["hp"], d.Computed["sanity"], d.Points)
	}
	var n int
	app.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name = 'character_build'`).Scan(&n)
	if n != 0 {
		t.Error("character_build should be gone")
	}
}

// A formula that reads a bonus block takes that bonus over: it gets the
// number as typed, and the bonus isn't added on its own too. Bonuses from
// sources the formula doesn't read are added as usual.
func TestBonusBlocks(t *testing.T) {
	sys := statSystem{PointsFormula: "[level]", Stats: []statDef{
		{Key: "attack", Category: "base", Name: "Attack", Formula: "10 + [race_bonus] * 3"},
		{Key: "luck", Category: "special", Name: "Luck", Formula: "[class_bonus] / 2"},
	}}
	if issues := sys.compile(); len(issues) > 0 {
		t.Fatal(issueCodes(issues))
	}
	src := func(kind string) modifierSource { return modifierSource{kind, sql.NullInt64{Int64: 1, Valid: true}} }
	rows := []modifierRow{
		{sourceType: "race", sourceID: 1, targetStat: "attack", scaling: "per_level", value: 2},
		{sourceType: "class", sourceID: 1, targetStat: "attack", scaling: "per_level", value: 1},
		{sourceType: "class", sourceID: 1, targetStat: "luck", scaling: "flat", value: 4},
		{sourceType: "item", sourceID: 1, targetStat: "luck", scaling: "flat", value: 1},
	}
	got := sys.calculate(statInputs{Level: 5, Values: map[string]float64{}}, rows, []modifierSource{src("race"), src("class"), src("item")})
	// Attack: 10 + 2*3 from the formula, plus the class's 1 per level (5).
	if got["attack"] != 21 {
		t.Errorf("attack = %v, want 21", got["attack"])
	}
	// Luck: 4/2 from the formula, plus the gear's 1.
	if got["luck"] != 3 {
		t.Errorf("luck = %v, want 3", got["luck"])
	}

	bad := statSystem{PointsFormula: "[level] + [race_bonus]"}
	if codes := issueCodes(bad.compile()); codes != "/points/unknown_value" {
		t.Errorf("points formula reading a bonus: %q", codes)
	}
}
