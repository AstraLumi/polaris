package main

import (
	"math"
	"testing"
)

func formulaTestStats(t *testing.T) ComputedStats {
	t.Helper()
	mgr, _ := newTestServer(t)
	app, _ := mgr.open(createStoryDirect(t, mgr, "Formulas"))
	build := BuildStats{VIT: 2, DEF: 1, RES: 1, STR: 10, DEX: 3, Intel: 4, WIS: 5, AGL: 2}
	bases := map[string]float64{"sanity": 50, "resist_fire": 0.25, "cooking": 3}
	c, err := computeStats(app.db, 10, 18, 180, 70, build, bases, categoryRefs{})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// withFormulas runs a test with the given custom formulas (nil: the
// defaults), so these tests pass whether or not stats_custom.go exists.
func withFormulas(t *testing.T, f func(StatInputs) (map[string]float64, map[string]float64)) {
	saved := customFormulas
	customFormulas = f
	t.Cleanup(func() { customFormulas = saved })
}

// The default formulas, worked out by hand for one character, so moving or
// tidying them can't change anyone's numbers by accident.
func TestDefaultFormulas(t *testing.T) {
	withFormulas(t, nil)
	c := formulaTestStats(t)
	checks := []struct {
		name      string
		got, want float64
	}{
		{"hp", c.Base.HP, 10 + 2*10 + 10*2},
		{"mp", c.Base.MP, 10 + 5*5 + 4 + 10*2},
		{"attack", c.Base.Attack, 1 + 10*2 + 3},
		{"potency", c.Base.Potency, 1 + 4*2 + 5},
		{"defense", c.Base.Defense, 4},
		{"speed", c.Base.Speed, 5},
		{"luck at 18", c.Special.Luck, 1},
		{"carry limit", c.Special.CarryLimit, (70*0.13 + 0.8*2) * 2}, // STR 10 doubles it
		{"fall damage", c.Special.FallDamageThreshold, 2.1},
		{"sanity", c.Special.Sanity, 50},
		{"fire", c.Elemental.Fire, 1.25},
		{"cooking", c.Lifeskills.Cooking, 3},
	}
	for _, ch := range checks {
		if !near(ch.got, ch.want) {
			t.Errorf("%s: got %v, want %v", ch.name, ch.got, ch.want)
		}
	}
}

// stats_custom.go only has to name the stats it changes; the rest keep
// their defaults, and a name the app doesn't know is ignored.
func TestCustomFormulasOverrideOnlyWhatTheyName(t *testing.T) {
	withFormulas(t, func(in StatInputs) (map[string]float64, map[string]float64) {
		return map[string]float64{"hp": 100 + in.VIT, "made_up": 5},
			map[string]float64{"attack": 0.5}
	})

	c := formulaTestStats(t)
	if !near(c.Base.HP, 102) {
		t.Errorf("hp: got %v, want the custom 102", c.Base.HP)
	}
	if !near(c.Base.Attack, (1+10*2+3)*1.5) {
		t.Errorf("attack: got %v, want +50%% from the custom multiplier", c.Base.Attack)
	}
	if !near(c.Base.MP, 59) || !near(c.Special.CarryLimit, 21.4) {
		t.Errorf("stats the custom file didn't name must keep their defaults: mp %v, carry %v", c.Base.MP, c.Special.CarryLimit)
	}
}
