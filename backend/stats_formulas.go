package main

import "math"

// The stat formulas. They are tuned for the author's own setting; to use
// your own without touching this file, copy stats_custom.go.example to
// stats_custom.go (git ignores it, so updates never overwrite it) and
// change only the stats you want. See "Stat formulas" in the README.

// StatInputs is everything a formula can read: the 8 primary stats after
// class/race/gear bonuses, the character's level, age and body, and the
// numbers typed on the sheet (Bases, by stat key; see specialBaseKeys).
type StatInputs struct {
	VIT, DEF, RES, STR, DEX, INT, WIS, AGL float64

	Level    float64
	Age      float64 // years
	HeightM  float64 // metres
	WeightKG float64

	Bases map[string]float64
}

// Base is the number typed on the sheet for key, or 0.
func (in StatInputs) Base(key string) float64 { return in.Bases[key] }

// customFormulas is set by stats_custom.go when there is one. It returns
// the same two maps as defaultFormulas, but only for the stats it changes;
// every stat it leaves out keeps its default.
var customFormulas func(in StatInputs) (values, multipliers map[string]float64)

// defaultFormulas gives every derived stat its value before bonuses
// (values) and, for a few stats, a percentage that bonuses are multiplied
// by (multipliers: 0.25 means +25%). Asset and gear bonuses are applied on
// top of both afterwards, in computeStats.
func defaultFormulas(in StatInputs) (values, multipliers map[string]float64) {
	values = map[string]float64{
		// Base Stats.
		"hp":         10 + in.VIT*10 + in.Level*2,
		"mp":         10 + in.WIS*5 + in.INT + in.Level*2,
		"attack":     1 + in.STR*2 + in.DEX,
		"potency":    1 + in.INT*2 + in.WIS,
		"defense":    1 + in.DEF*3,
		"resistance": 1 + in.RES*3,
		"speed":      1 + in.AGL*2,

		// Special Stats.
		"luck": 1 + luckInitialFactor(in.Age),

		// Weight-led baseline with a small height nudge (clamped so short
		// characters don't get a negative contribution). STR's effect is the
		// multiplier below, not part of this.
		"carry_limit": in.WeightKG*0.13 + math.Max(0, in.HeightM-1)*2,

		// Taller/lighter characters have more give on landing; heavier
		// ones take more damage from the same fall.
		"fall_damage_threshold": 2.0 + (in.HeightM-1.7)*1.0 - (in.WeightKG-defaultWeightKG)*0.02,

		// Sanity has no formula: exactly the typed base, plus bonuses.
		"sanity": in.Base("sanity"),

		// More body mass = harder to shed heat, but better insulated
		// against cold. Both start from their own typed base.
		"heat_threshold": in.Base("heat_threshold") - (in.WeightKG-defaultWeightKG)*0.05,
		"cold_threshold": in.Base("cold_threshold") - (in.WeightKG-defaultWeightKG)*0.05,

		// Special Defenses: 1.00x baseline plus whatever's been typed in.
		"resist_water":   1 + in.Base("resist_water"),
		"resist_fire":    1 + in.Base("resist_fire"),
		"resist_wind":    1 + in.Base("resist_wind"),
		"resist_earth":   1 + in.Base("resist_earth"),
		"resist_ice":     1 + in.Base("resist_ice"),
		"resist_thunder": 1 + in.Base("resist_thunder"),
		"resist_impact":  1 + in.Base("resist_impact"),

		// Lifeskills: no baseline, just the typed level plus bonuses.
		"cooking":    in.Base("cooking"),
		"crafting":   in.Base("crafting"),
		"alchemy":    in.Base("alchemy"),
		"hunting":    in.Base("hunting"),
		"gathering":  in.Base("gathering"),
		"farming":    in.Base("farming"),
		"blessing":   in.Base("blessing"),
		"enchanting": in.Base("enchanting"),
		"smithing":   in.Base("smithing"),
	}

	multipliers = map[string]float64{
		// STR's carry-limit contribution is quadratic: (STR/10)^2 barely
		// moves the needle below 10 STR (a realistic range) and gets much
		// stronger above it: STR 5 → +25%, STR 10 → +100% (2x), STR 20 →
		// +400% (5x).
		"carry_limit": math.Pow(in.STR/10, 2),
	}
	return values, multipliers
}

// luckAgeBreakpoints are (age, factor) pairs Luck's InitialFactor is
// linearly interpolated between. Flat before 0 and after 200.
var luckAgeBreakpoints = [][2]float64{
	{0, 0.75},
	{18, 0},
	{100, -0.75},
	{200, -1},
}

func luckInitialFactor(age float64) float64 {
	if age <= luckAgeBreakpoints[0][0] {
		return luckAgeBreakpoints[0][1]
	}
	last := len(luckAgeBreakpoints) - 1
	if age >= luckAgeBreakpoints[last][0] {
		return luckAgeBreakpoints[last][1]
	}
	for i := 0; i < last; i++ {
		x0, y0 := luckAgeBreakpoints[i][0], luckAgeBreakpoints[i][1]
		x1, y1 := luckAgeBreakpoints[i+1][0], luckAgeBreakpoints[i+1][1]
		if age >= x0 && age <= x1 {
			t := (age - x0) / (x1 - x0)
			return roundTo2(y0 + t*(y1-y0))
		}
	}
	return 0 // unreachable given the clamps above
}

// derivedFormulas is defaultFormulas with stats_custom.go's changes laid
// on top. A custom stat name the defaults don't have is ignored: it would
// have nowhere to show.
func derivedFormulas(in StatInputs) (values, multipliers map[string]float64) {
	values, multipliers = defaultFormulas(in)
	if customFormulas == nil {
		return values, multipliers
	}
	cv, cm := customFormulas(in)
	for k, v := range cv {
		if _, known := values[k]; known {
			values[k] = v
		}
	}
	for k, v := range cm {
		if _, known := values[k]; known {
			multipliers[k] = v
		}
	}
	return values, multipliers
}
