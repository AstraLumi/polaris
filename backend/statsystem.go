package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"slices"
	"strings"
)

// The stat system: which stats a story has and how each is calculated. It is
// the character sheet's ground truth: the sheet, the bonus forms of classes,
// races and gear, and version comparisons all list what is defined here, and
// Settings → Stats edits it.
//
// The categories are fixed and each keeps its behaviour; only the stats in
// them and their formulas change:
//
//   - primary   points are spent on them, up to the points formula (by
//     default one per level); their value is the points plus bonuses.
//   - base      calculated by formula; bonuses from classes, races and the
//     like count once per level ("+10 Attack per level"), gear's are flat.
//   - special, defense, lifeskill  calculated by formula, every bonus flat.
//
// A calculated stat whose formula reads [base] gets a box on the sheet to
// type that number in (Sanity, the thresholds, defenses, lifeskills).

var statCategories = []string{"primary", "base", "special", "defense", "lifeskill"}

func isStatCategory(c string) bool { return slices.Contains(statCategories, c) }

type statDef struct {
	Key       string `json:"key"`
	Category  string `json:"category"`
	Name      string `json:"name"`
	ShortName string `json:"short_name"`
	// Formula is the stat's value before bonuses; Multiplier, when set, is a
	// percentage the total is multiplied by (0.25: +25%). Primary stats have
	// neither.
	Formula    string `json:"formula"`
	Multiplier string `json:"multiplier"`
	Unit       string `json:"unit"`
	Decimals   int    `json:"decimals"`
	// Min and Max keep the final value in bounds, when set.
	Min *float64 `json:"min"`
	Max *float64 `json:"max"`
	// LowerIsBetter: a drop is good news (a cold threshold that goes down
	// means the character stands more cold), so a negative bonus or change
	// is shown as an improvement. Only how it looks; the maths is the same.
	LowerIsBetter bool `json:"lower_is_better"`
}

type statSystem struct {
	// PointsFormula is how many points a character has to spend on primary
	// stats.
	PointsFormula string    `json:"points_formula"`
	Stats         []statDef `json:"stats"`

	points   *formula
	formulas map[string]*formula
	mults    map[string]*formula
	// derived lists the calculated stats so that every stat comes after the
	// stats its formulas read.
	derived []*statDef
}

func (s *statSystem) def(key string) *statDef {
	for i := range s.Stats {
		if s.Stats[i].Key == key {
			return &s.Stats[i]
		}
	}
	return nil
}

func (s *statSystem) keys(category string) []string {
	var out []string
	for _, d := range s.Stats {
		if d.Category == category {
			out = append(out, d.Key)
		}
	}
	return out
}

// typed reports whether the sheet asks for a number for key: always for a
// primary stat (its points), and for a calculated one whose formula reads
// [base].
func (s *statSystem) typed(key string) bool {
	d := s.def(key)
	if d == nil {
		return false
	}
	return d.Category == "primary" || s.formulas[key].reads("base") || s.mults[key].reads("base")
}

// pointsFor is how many points a character can spend on primary stats.
func (s *statSystem) pointsFor(in statInputs) int {
	return int(math.Round(s.points.eval(in.env(nil, "", nil))))
}

// statIssue is one problem that stops a stat system from being saved.
type statIssue struct {
	Key    string `json:"key"`   // the stat, or "" for the points formula
	Field  string `json:"field"` // key, name, formula, multiplier, min, points
	Code   string `json:"code"`
	Detail string `json:"detail,omitempty"`
}

const maxStats = 200

// compile checks a stat system and gets it ready to calculate with. Every
// problem is returned; the system is only usable when there are none.
func (s *statSystem) compile() []statIssue {
	var issues []statIssue
	add := func(key, field, code, detail string) {
		issues = append(issues, statIssue{Key: key, Field: field, Code: code, Detail: detail})
	}
	if len(s.Stats) > maxStats {
		add("", "key", "too_many", fmt.Sprint(maxStats))
	}

	seen := map[string]bool{}
	for i := range s.Stats {
		d := &s.Stats[i]
		d.Name = strings.TrimSpace(d.Name)
		d.ShortName = strings.TrimSpace(d.ShortName)
		d.Unit = strings.TrimSpace(d.Unit)
		switch {
		case !validStatKey(d.Key):
			add(d.Key, "key", "bad_key", "")
		case isFormulaInput(d.Key):
			add(d.Key, "key", "reserved_key", "")
		case seen[d.Key]:
			add(d.Key, "key", "duplicate_key", "")
		}
		seen[d.Key] = true
		if !isStatCategory(d.Category) {
			add(d.Key, "key", "bad_category", d.Category)
		}
		if d.Name == "" {
			add(d.Key, "name", "no_name", "")
		}
		d.Decimals = int(clamp(float64(d.Decimals), 0, 4))
		if d.Min != nil && d.Max != nil && *d.Min > *d.Max {
			add(d.Key, "min", "min_over_max", "")
		}
		if d.Category == "primary" {
			d.Formula, d.Multiplier, d.Decimals = "", "", 0
		}
	}

	s.formulas = map[string]*formula{}
	s.mults = map[string]*formula{}
	check := func(key, field, src string, allowBase, allowStats bool) *formula {
		f, err := parseFormula(src)
		if err != nil {
			fe, _ := err.(*formulaError)
			if fe == nil {
				fe = &formulaError{Code: "syntax"}
			}
			add(key, field, fe.Code, fe.Detail)
			return nil
		}
		for _, r := range f.refs {
			switch {
			case perStatInput(r):
				if !allowBase {
					add(key, field, "unknown_value", r)
				}
			case isFormulaInput(r):
			case !allowStats || !seen[r]:
				add(key, field, "unknown_value", r)
			}
		}
		return f
	}

	s.points = check("", "points", s.PointsFormula, false, false)
	for i := range s.Stats {
		d := &s.Stats[i]
		if d.Category == "primary" {
			continue
		}
		s.formulas[d.Key] = check(d.Key, "formula", d.Formula, true, true)
		s.mults[d.Key] = check(d.Key, "multiplier", d.Multiplier, true, true)
	}

	// Order the calculated stats so each comes after what it reads, and
	// refuse loops (A reads B reads A).
	state := map[string]int{} // 1 visiting, 2 done
	s.derived = nil
	var visit func(d *statDef, path []string) bool
	visit = func(d *statDef, path []string) bool {
		switch state[d.Key] {
		case 2:
			return true
		case 1:
			add(d.Key, "formula", "loop", strings.Join(append(path, d.Key), " → "))
			return false
		}
		state[d.Key] = 1
		for _, f := range []*formula{s.formulas[d.Key], s.mults[d.Key]} {
			if f == nil {
				continue
			}
			for _, r := range f.refs {
				dep := s.def(r)
				if dep == nil || dep.Category == "primary" {
					continue
				}
				if !visit(dep, append(path, d.Key)) {
					state[d.Key] = 2
					return false
				}
			}
		}
		state[d.Key] = 2
		s.derived = append(s.derived, d)
		return true
	}
	for i := range s.Stats {
		if s.Stats[i].Category != "primary" {
			visit(&s.Stats[i], nil)
		}
	}
	return issues
}

// statInputs is one character's side of a calculation.
type statInputs struct {
	Level    float64
	Age      float64
	HeightCM float64
	WeightKG float64
	// Values are the numbers typed on the sheet, by stat key: primary stats'
	// points and calculated stats' [base].
	Values map[string]float64
}

// env is what a formula sees while calculating stat key: the inputs, [base]
// and the bonuses (by source_type) for that stat, and every stat in done.
func (in statInputs) env(done map[string]float64, key string, bonuses map[string]float64) func(string) float64 {
	return func(name string) float64 {
		if src, ok := bonusInputs[name]; ok {
			return bonuses[src]
		}
		switch name {
		case "level":
			return in.Level
		case "age":
			return in.Age
		case "height":
			return in.HeightCM
		case "weight":
			return in.WeightKG
		case "base":
			return in.Values[key]
		}
		return done[name]
	}
}

// calculate works out every stat. Bonuses (stat_modifiers rows matching
// sources) are added to a stat's own value, then the stat's multiplier and
// any multiplier bonuses apply to the total; the result is kept within
// min/max and rounded to the stat's decimals. A formula that reads a bonus
// block ([race_bonus]) takes that source's bonus over: it reads the number as
// typed on the asset, and the bonus isn't added on its own as well.
func (s *statSystem) calculate(in statInputs, rows []modifierRow, sources []modifierSource) map[string]float64 {
	out := map[string]float64{}

	// Primary stats first: points plus bonuses, so a race can give "+2 STR"
	// and everything calculated from STR follows.
	additive, multiplier := map[string]float64{}, map[string]float64{}
	for _, k := range s.keys("primary") {
		additive[k] = in.Values[k]
	}
	applyModifiers(rows, sources, in.Level, additive, multiplier, nil)
	for k, v := range resolveFinal(additive, multiplier) {
		out[k] = v
	}

	for _, d := range s.derived {
		taken := map[string]bool{}
		for name, src := range bonusInputs {
			if s.formulas[d.Key].reads(name) || s.mults[d.Key].reads(name) {
				taken[src] = true
			}
		}
		env := in.env(out, d.Key, bonusesFor(rows, sources, d.Key))
		additive := map[string]float64{d.Key: s.formulas[d.Key].eval(env)}
		multiplier := map[string]float64{d.Key: s.mults[d.Key].eval(env)}
		applyModifiers(rows, sources, in.Level, additive, multiplier, taken)
		v := finite(additive[d.Key] * (1 + multiplier[d.Key]))
		if d.Min != nil {
			v = math.Max(v, *d.Min)
		}
		if d.Max != nil {
			v = math.Min(v, *d.Max)
		}
		p := math.Pow(10, float64(d.Decimals))
		out[d.Key] = math.Round(v*p) / p
	}
	return out
}

// ---- Storage ---------------------------------------------------------------

const statDefsTableSQL = `CREATE TABLE stat_defs (
	key        TEXT PRIMARY KEY,
	category   TEXT NOT NULL CHECK (category IN ('primary', 'base', 'special', 'defense', 'lifeskill')),
	position   INTEGER NOT NULL DEFAULT 0,
	name       TEXT NOT NULL,
	short_name TEXT NOT NULL DEFAULT '',
	formula    TEXT NOT NULL DEFAULT '',
	multiplier TEXT NOT NULL DEFAULT '',
	unit       TEXT NOT NULL DEFAULT '',
	decimals   INTEGER NOT NULL DEFAULT 0,
	min_value  REAL,
	max_value  REAL
)`

// stat_settings holds the rest of the stat system (the points formula) as
// key/value pairs, so it travels with stat_defs when a story copies another's
// Character Assets.
const statSettingsTableSQL = `CREATE TABLE stat_settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
)`

func fp(v float64) *float64 { return &v }

// defaultStatSystem is the stat system every story starts with: the stats
// and formulas Polaris had before they could be edited.
func defaultStatSystem() statSystem {
	primary := func(key, name, short string) statDef {
		return statDef{Key: key, Category: "primary", Name: name, ShortName: short}
	}
	calc := func(cat, key, name, formula, unit string, decimals int) statDef {
		return statDef{Key: key, Category: cat, Name: name, Formula: formula, Unit: unit, Decimals: decimals}
	}
	defense := func(key, name string) statDef {
		return calc("defense", key, name, "1 + [base]", "x", 2)
	}
	lifeskill := func(key, name string) statDef {
		return calc("lifeskill", key, name, "[base]", "", 0)
	}
	luck := calc("special", "luck", "Luck", "1 + round(curve([age], 0, 0.75, 18, 0, 100, -0.75, 200, -1), 2)", "x", 2)
	luck.Min, luck.Max = fp(0), fp(2)
	carry := calc("special", "carry_limit", "Carry Limit", "[weight] * 0.13 + max(0, [height] / 100 - 1) * 2", "kg", 1)
	// STR multiplies the carry limit: (STR/10)^2 barely moves it below 10 STR
	// and doubles it at 10.
	carry.Multiplier = "([str] / 10) ^ 2"
	hp := calc("base", "hp", "HP", "10 + [vit] * 10 + [level] * 2", "", 0)
	cold := calc("special", "cold_threshold", "Cold Threshold", "[base] - ([weight] - 70) * 0.05", "°C", 0)
	cold.LowerIsBetter = true
	mp := calc("base", "mp", "MP", "10 + [wis] * 5 + [intel] + [level] * 2", "", 0)
	return statSystem{
		PointsFormula: "[level]",
		Stats: []statDef{
			primary("vit", "Vitality", "VIT"),
			primary("def", "Defense", "DEF"),
			primary("res", "Resistance", "RES"),
			primary("str", "Strength", "STR"),
			primary("dex", "Dexterity", "DEX"),
			primary("intel", "Intelligence", "INT"),
			primary("wis", "Wisdom", "WIS"),
			primary("agl", "Agility", "AGL"),

			hp, mp,
			calc("base", "attack", "Attack", "1 + [str] * 2 + [dex]", "", 0),
			calc("base", "potency", "Potency", "1 + [intel] * 2 + [wis]", "", 0),
			calc("base", "defense", "Defense", "1 + [def] * 3", "", 0),
			calc("base", "resistance", "Resistance", "1 + [res] * 3", "", 0),
			calc("base", "speed", "Speed", "1 + [agl] * 2", "", 0),

			luck, carry,
			calc("special", "fall_damage_threshold", "Fall Damage Threshold", "2 + ([height] / 100 - 1.7) - ([weight] - 70) * 0.02", "m", 1),
			calc("special", "sanity", "Sanity", "[base]", "γ", 0),
			calc("special", "heat_threshold", "Heat Threshold", "[base] - ([weight] - 70) * 0.05", "°C", 0),
			cold,

			defense("resist_water", "Water"),
			defense("resist_fire", "Fire"),
			defense("resist_wind", "Wind"),
			defense("resist_earth", "Earth"),
			defense("resist_ice", "Ice"),
			defense("resist_thunder", "Thunder"),
			defense("resist_impact", "Impact"),

			lifeskill("cooking", "Cooking"),
			lifeskill("crafting", "Crafting"),
			lifeskill("alchemy", "Alchemy"),
			lifeskill("hunting", "Hunting"),
			lifeskill("gathering", "Gathering"),
			lifeskill("farming", "Farming"),
			lifeskill("blessing", "Blessing"),
			lifeskill("enchanting", "Enchanting"),
			lifeskill("smithing", "Smithing"),
		},
	}
}

func nullableFloatPtr(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// writeStatSystem replaces the stored stat system with s.
func writeStatSystem(db execer, s statSystem) error {
	if _, err := db.Exec(`DELETE FROM stat_defs`); err != nil {
		return err
	}
	pos := map[string]int{}
	for _, d := range s.Stats {
		pos[d.Category]++
		if _, err := db.Exec(`INSERT INTO stat_defs
			(key, category, position, name, short_name, formula, multiplier, unit, decimals, min_value, max_value, lower_is_better)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			d.Key, d.Category, pos[d.Category], d.Name, d.ShortName, d.Formula, d.Multiplier, d.Unit, d.Decimals,
			nullableFloatPtr(d.Min), nullableFloatPtr(d.Max), d.LowerIsBetter); err != nil {
			return err
		}
	}
	_, err := db.Exec(`INSERT INTO stat_settings (key, value) VALUES ('points_formula', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, s.PointsFormula)
	return err
}

// readStatSystem loads the stored stat system as it is, without checking it.
func readStatSystem(db *sql.DB) (statSystem, error) {
	s := statSystem{Stats: []statDef{}}
	db.QueryRow(`SELECT value FROM stat_settings WHERE key = 'points_formula'`).Scan(&s.PointsFormula)
	rows, err := db.Query(`SELECT key, category, name, short_name, formula, multiplier, unit, decimals, min_value, max_value, lower_is_better
		FROM stat_defs ORDER BY CASE category
			WHEN 'primary' THEN 0 WHEN 'base' THEN 1 WHEN 'special' THEN 2 WHEN 'defense' THEN 3 ELSE 4 END,
			position, key`)
	if err != nil {
		return s, err
	}
	defer rows.Close()
	for rows.Next() {
		var d statDef
		var lo, hi sql.NullFloat64
		if err := rows.Scan(&d.Key, &d.Category, &d.Name, &d.ShortName, &d.Formula, &d.Multiplier, &d.Unit, &d.Decimals, &lo, &hi, &d.LowerIsBetter); err != nil {
			return s, err
		}
		if lo.Valid {
			d.Min = fp(lo.Float64)
		}
		if hi.Valid {
			d.Max = fp(hi.Float64)
		}
		s.Stats = append(s.Stats, d)
	}
	return s, rows.Err()
}

// loadStatSystem is the story's stat system, ready to calculate with. Saving
// checks every formula, so a broken one only gets here if the database was
// edited by hand; it then counts as empty rather than breaking every sheet.
func loadStatSystem(db *sql.DB) (*statSystem, error) {
	s, err := readStatSystem(db)
	if err != nil {
		return nil, err
	}
	if issues := s.compile(); len(issues) > 0 {
		log.Printf("stat system has %d problem(s), first: %+v", len(issues), issues[0])
		if s.points == nil {
			s.points = &formula{}
		}
		for _, d := range s.Stats {
			if d.Category != "primary" && s.formulas[d.Key] == nil {
				s.formulas[d.Key] = &formula{}
			}
		}
	}
	return &s, nil
}

// ---- Handlers --------------------------------------------------------------

type statUsage struct {
	Versions int `json:"versions"` // character versions with a number typed for it
	Bonuses  int `json:"bonuses"`  // classes, races, gear, ... giving a bonus to it
}

type statSystemPayload struct {
	statSystem
	Usage map[string]statUsage `json:"usage"`
}

func statSystemResponse(db *sql.DB) (statSystemPayload, error) {
	s, err := readStatSystem(db)
	if err != nil {
		return statSystemPayload{}, err
	}
	usage := map[string]statUsage{}
	for _, q := range []struct {
		sql   string
		count func(u *statUsage, n int)
	}{
		{`SELECT stat_key, COUNT(DISTINCT version_id) FROM character_special_bases WHERE value != 0 GROUP BY stat_key`,
			func(u *statUsage, n int) { u.Versions = n }},
		{`SELECT target_stat, COUNT(*) FROM stat_modifiers WHERE value != 0 GROUP BY target_stat`,
			func(u *statUsage, n int) { u.Bonuses = n }},
	} {
		rows, err := db.Query(q.sql)
		if err != nil {
			return statSystemPayload{}, err
		}
		for rows.Next() {
			var k string
			var n int
			if rows.Scan(&k, &n) == nil {
				u := usage[k]
				q.count(&u, n)
				usage[k] = u
			}
		}
		rows.Close()
	}
	return statSystemPayload{statSystem: s, Usage: usage}, nil
}

func getStatSystemHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := statSystemResponse(db)
		if err != nil {
			http.Error(w, "failed to load stats", http.StatusInternalServerError)
			log.Printf("load stat system: %v", err)
			return
		}
		writeJSON(w, p)
	}
}

// The stat system Polaris starts with, for Settings' "Back to the defaults".
func defaultStatSystemHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, defaultStatSystem())
}

func writeIssues(w http.ResponseWriter, issues []statIssue) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnprocessableEntity)
	json.NewEncoder(w).Encode(map[string]any{"issues": issues})
}

// updateStatSystemHandler saves a whole stat system. A stat that is no longer
// in it takes its typed numbers and its bonuses with it.
func updateStatSystemHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var s statSystem
		if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if issues := s.compile(); len(issues) > 0 {
			writeIssues(w, issues)
			return
		}
		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to start transaction", http.StatusInternalServerError)
			return
		}
		if err := writeStatSystem(tx, s); err == nil {
			_, err = tx.Exec(`DELETE FROM character_special_bases WHERE stat_key NOT IN (SELECT key FROM stat_defs)`)
			if err == nil {
				_, err = tx.Exec(`DELETE FROM stat_modifiers WHERE target_stat NOT IN (SELECT key FROM stat_defs)`)
			}
			if err == nil {
				// A stat that moved in or out of Base Stats changes how its
				// bonuses count; see scalingFor.
				err = rescaleModifiers(tx)
			}
			if err != nil {
				tx.Rollback()
				http.Error(w, "failed to save stats", http.StatusInternalServerError)
				log.Printf("save stat system: %v", err)
				return
			}
		} else {
			tx.Rollback()
			http.Error(w, "failed to save stats", http.StatusInternalServerError)
			log.Printf("save stat system: %v", err)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to save stats", http.StatusInternalServerError)
			return
		}
		p, err := statSystemResponse(db)
		if err != nil {
			http.Error(w, "failed to load stats", http.StatusInternalServerError)
			return
		}
		writeJSON(w, p)
	}
}

type statPreviewRequest struct {
	statSystem
	Sample struct {
		Level  float64            `json:"level"`
		Age    float64            `json:"age"`
		Height float64            `json:"height"`
		Weight float64            `json:"weight"`
		Values map[string]float64 `json:"values"`
		// Bonus the sample gets from each source (class, race, gear, ...) to
		// every calculated stat.
		Bonus float64 `json:"bonus"`
	} `json:"sample"`
}

// previewStatSystemHandler checks an unsaved stat system and works it out for
// a sample character (no bonuses), so Settings can show what each formula
// gives while it is being edited.
func previewStatSystemHandler(w http.ResponseWriter, r *http.Request) {
	var req statPreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	s := req.statSystem
	issues := s.compile()
	if issues == nil {
		issues = []statIssue{}
	}
	// A broken formula counts as empty here, so the rest still preview.
	if s.points == nil {
		s.points = &formula{}
	}
	for _, d := range s.Stats {
		if d.Category != "primary" && s.formulas[d.Key] == nil {
			s.formulas[d.Key] = &formula{}
		}
	}
	in := statInputs{
		Level: math.Max(1, req.Sample.Level), Age: req.Sample.Age,
		HeightCM: req.Sample.Height, WeightKG: req.Sample.Weight, Values: req.Sample.Values,
	}
	if in.Values == nil {
		in.Values = map[string]float64{}
	}
	// The sample bonus as if it came from one of each kind of asset, counted
	// the way real bonuses are (per level for Base Stats, but not from gear).
	var rows []modifierRow
	var sources []modifierSource
	if req.Sample.Bonus != 0 {
		for _, src := range bonusInputs {
			sources = append(sources, modifierSource{src, sql.NullInt64{Int64: 1, Valid: true}})
			for _, d := range s.Stats {
				if d.Category == "primary" {
					continue
				}
				scaling := "flat"
				if d.Category == "base" && src != "item" {
					scaling = "per_level"
				}
				rows = append(rows, modifierRow{sourceType: src, sourceID: 1, targetStat: d.Key, scaling: scaling, value: req.Sample.Bonus})
			}
		}
	}
	writeJSON(w, map[string]any{
		"issues": issues,
		"points": s.pointsFor(in),
		"values": s.calculate(in, rows, sources),
	})
}

// ---- Schema 22 -------------------------------------------------------------

// schema22Statements create what schema 22 added: the stat system, filled
// with the defaults, and the primary stats' points moved from the fixed
// character_build columns into character_special_bases (by stat key, like
// every other typed number). The fresh schema runs them too.
var schema22Statements = slices.Concat(
	[]string{statDefsTableSQL, statSettingsTableSQL},
	statSeedSQL(defaultStatSystem()),
	movePrimaryPointsSQL(),
	[]string{`DROP TABLE character_build`},
)

func sqlText(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func sqlNumber(p *float64) string {
	if p == nil {
		return "NULL"
	}
	return fmt.Sprint(*p)
}

func statSeedSQL(s statSystem) []string {
	out := []string{`INSERT INTO stat_settings (key, value) VALUES ('points_formula', ` + sqlText(s.PointsFormula) + `)`}
	pos := map[string]int{}
	for _, d := range s.Stats {
		pos[d.Category]++
		out = append(out, fmt.Sprintf(`INSERT INTO stat_defs
			(key, category, position, name, short_name, formula, multiplier, unit, decimals, min_value, max_value)
			VALUES (%s, %s, %d, %s, %s, %s, %s, %s, %d, %s, %s)`,
			sqlText(d.Key), sqlText(d.Category), pos[d.Category], sqlText(d.Name), sqlText(d.ShortName),
			sqlText(d.Formula), sqlText(d.Multiplier), sqlText(d.Unit), d.Decimals, sqlNumber(d.Min), sqlNumber(d.Max)))
	}
	return out
}

// Rows left behind by versions deleted long ago (foreign keys weren't always
// enforced) are skipped: copying them would break the new table's foreign key
// and fail the whole upgrade (2.0.0 did, so this filter was added after the
// release; it changes nothing for a database without such rows).
func movePrimaryPointsSQL() []string {
	var out []string
	for _, col := range []string{"vit", "def", "res", "str", "dex", "intel", "wis", "agl"} {
		out = append(out, fmt.Sprintf(`INSERT OR REPLACE INTO character_special_bases (version_id, stat_key, value)
			SELECT version_id, '%s', %s FROM character_build
			WHERE %s != 0 AND version_id IN (SELECT id FROM character_versions)`, col, col, col))
	}
	return out
}
