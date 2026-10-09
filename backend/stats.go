package main

import (
	"database/sql"
	"math"
)

// BuildStats are the 8 primary point-buy stats — the only numbers spent
// against the level budget. Everything else in this file is derived from
// these (after modifiers are applied to them — see computeStats).
type BuildStats struct {
	VIT   int `json:"vit"`
	DEF   int `json:"def"`
	RES   int `json:"res"`
	STR   int `json:"str"`
	DEX   int `json:"dex"`
	Intel int `json:"intel"`
	WIS   int `json:"wis"`
	AGL   int `json:"agl"`
}

func (b BuildStats) Sum() int {
	return b.VIT + b.DEF + b.RES + b.STR + b.DEX + b.Intel + b.WIS + b.AGL
}

type BaseStats struct {
	HP         float64 `json:"hp"`
	MP         float64 `json:"mp"`
	Attack     float64 `json:"attack"`
	Potency    float64 `json:"potency"`
	Defense    float64 `json:"defense"`
	Resistance float64 `json:"resistance"`
	Speed      float64 `json:"speed"`
}

// SpecialStats sits in its own group underneath Base Stats. Sanity and the
// two thresholds each have a typed base (see specialBaseKeys) that this
// value is calculated from.
type SpecialStats struct {
	Luck                float64 `json:"luck"`
	CarryLimit          float64 `json:"carry_limit"`
	FallDamageThreshold float64 `json:"fall_damage_threshold"`
	Sanity              float64 `json:"sanity"`
	HeatThreshold       float64 `json:"heat_threshold"`
	ColdThreshold       float64 `json:"cold_threshold"`
}

type ElementalDefenses struct {
	Water   float64 `json:"resist_water"`
	Fire    float64 `json:"resist_fire"`
	Wind    float64 `json:"resist_wind"`
	Earth   float64 `json:"resist_earth"`
	Ice     float64 `json:"resist_ice"`
	Thunder float64 `json:"resist_thunder"`
	Impact  float64 `json:"resist_impact"`
}

type LifeskillStats struct {
	Cooking    float64 `json:"cooking"`
	Crafting   float64 `json:"crafting"`
	Alchemy    float64 `json:"alchemy"`
	Hunting    float64 `json:"hunting"`
	Gathering  float64 `json:"gathering"`
	Farming    float64 `json:"farming"`
	Blessing   float64 `json:"blessing"`
	Enchanting float64 `json:"enchanting"`
	Smithing   float64 `json:"smithing"`
}

// PrimaryStats are the 8 primary stats AFTER modifiers — what a
// character's STR etc. actually is once Class/Race/etc. bonuses are
// applied, as opposed to BuildStats which is just the raw spent points.
type PrimaryStats struct {
	VIT   float64 `json:"vit"`
	DEF   float64 `json:"def"`
	RES   float64 `json:"res"`
	STR   float64 `json:"str"`
	DEX   float64 `json:"dex"`
	Intel float64 `json:"intel"`
	WIS   float64 `json:"wis"`
	AGL   float64 `json:"agl"`
}

type ComputedStats struct {
	Primary    PrimaryStats      `json:"primary"`
	Base       BaseStats         `json:"base"`
	Special    SpecialStats      `json:"special"`
	Elemental  ElementalDefenses `json:"elemental"`
	Lifeskills LifeskillStats    `json:"lifeskills"`
}

// specialBaseKeys are every stat_key a person can type a base number for,
// stored in character_special_bases. Adding a new special stat later is
// just adding a key here (plus wiring it into computeStats) — no schema
// change needed.
var specialBaseKeys = []string{
	"sanity", "heat_threshold", "cold_threshold",
	"resist_water", "resist_fire", "resist_wind", "resist_earth", "resist_ice", "resist_thunder", "resist_impact",
	"cooking", "crafting", "alchemy", "hunting", "gathering", "farming", "blessing", "enchanting", "smithing",
}

// perLevelModifierStats are the stats where a modifier from an asset
// (Class, Subclass, Specialization, Race, Body Type) scales with the
// character's level by default — "+10 Attack per level" rather than a
// flat one-time +10. This matches Base Stats specifically; every other
// group (primary stats, Special Stats, Special Defenses, Lifeskill) is
// flat, since "a race gives +2 STR" or "+1 Smithing" isn't meant to
// compound with level the way combat power is.
var perLevelModifierStats = map[string]bool{
	"hp": true, "mp": true, "attack": true, "potency": true,
	"defense": true, "resistance": true, "speed": true,
}

// categoryRefs is everything about a version that a stat_modifiers row
// might key off of. Race and body type live on character_story, so
// they're passed in alongside the identity-level references. There's
// ItemIDs are the equipped gear pieces, one entry per occupied slot (the same
// piece in two slots appears twice, and counts twice).
type categoryRefs struct {
	ClassID          sql.NullInt64
	SubclassID       sql.NullInt64
	SpecializationID sql.NullInt64
	RaceID           sql.NullInt64
	BodyTypeID       sql.NullInt64
	ItemIDs          []int64
}

const (
	defaultHeightCM = 170.0
	defaultWeightKG = 70.0
)

// resolveOrDefault is used by callers (versions.go) to fall back to an
// average-human default when height/weight haven't been set yet, so the
// physical-stat formulas below don't produce nonsense for a brand new
// character.
func resolveOrDefault(v sql.NullFloat64, fallback float64) float64 {
	if v.Valid {
		return v.Float64
	}
	return fallback
}

func roundTo2(v float64) float64 {
	return math.Round(v*100) / 100
}

func roundToInt(v float64) float64 {
	return math.Round(v)
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

type modifierRow struct {
	sourceType string
	sourceID   int64
	targetStat string
	scaling    string
	value      float64
}

type modifierSource struct {
	kind string
	id   sql.NullInt64
}

func loadModifierRows(db *sql.DB) ([]modifierRow, error) {
	rows, err := db.Query(`SELECT source_type, source_id, target_stat, scaling, value FROM stat_modifiers`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []modifierRow
	for rows.Next() {
		var m modifierRow
		if err := rows.Scan(&m.sourceType, &m.sourceID, &m.targetStat, &m.scaling, &m.value); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

// applyModifiers adds every matching modifier row into additive/multiplierDelta,
// but ONLY for target_stat keys already present in additive — this is what
// scopes a single set of modifier rows to "just the primary stats" in one
// pass and "just the derived stats" in another, without needing two
// separate database queries or two different tables.
func applyModifiers(rows []modifierRow, sources []modifierSource, level float64, additive, multiplierDelta map[string]float64) {
	for _, row := range rows {
		// How many of the character's sources this row belongs to — normally
		// 0 or 1, but the same gear piece can be worn in two slots.
		times := 0.0
		for _, s := range sources {
			if s.kind == row.sourceType && s.id.Valid && s.id.Int64 == row.sourceID {
				times++
			}
		}
		if times == 0 {
			continue
		}
		if _, tracked := additive[row.targetStat]; !tracked {
			continue
		}
		switch row.scaling {
		case "multiplier":
			multiplierDelta[row.targetStat] += row.value * times
		case "per_level":
			additive[row.targetStat] += row.value * level * times
		default: // "flat"
			additive[row.targetStat] += row.value * times
		}
	}
}

func resolveFinal(additive, multiplierDelta map[string]float64) map[string]float64 {
	final := make(map[string]float64, len(additive))
	for stat, val := range additive {
		final[stat] = val * (1 + multiplierDelta[stat])
	}
	return final
}

// computeStats is a two-phase pipeline:
//
//  1. The 8 primary stats start from the character's raw point-buy values
//     and get their own modifier pass — this is what lets a race give
//     "+2 STR" directly, not just a bonus to something derived from STR.
//  2. Every derived stat (Base Stats, Special Stats, Special Defenses,
//     Lifeskills) is computed FROM those effective primaries, then gets
//     its own separate modifier pass — so a class can add flat Attack on
//     top of whatever STR/DEX already produced.
//
// bases holds whatever's been typed into character_special_bases, keyed
// by stat_key (see specialBaseKeys); a missing key reads as 0.
func computeStats(db *sql.DB, level int, age, heightCM, weightKG float64, build BuildStats, bases map[string]float64, refs categoryRefs) (ComputedStats, error) {
	lvl := float64(level)
	heightM := heightCM / 100

	rows, err := loadModifierRows(db)
	if err != nil {
		return ComputedStats{}, err
	}

	sources := []modifierSource{
		{"class", refs.ClassID},
		{"subclass", refs.SubclassID},
		{"specialization", refs.SpecializationID},
		{"race", refs.RaceID},
		{"body_type", refs.BodyTypeID},
	}
	for _, id := range refs.ItemIDs {
		sources = append(sources, modifierSource{"item", sql.NullInt64{Int64: id, Valid: true}})
	}

	// Phase 1: effective primary stats.
	primaryAdditive := map[string]float64{
		"vit": float64(build.VIT), "def": float64(build.DEF), "res": float64(build.RES),
		"str": float64(build.STR), "dex": float64(build.DEX), "intel": float64(build.Intel),
		"wis": float64(build.WIS), "agl": float64(build.AGL),
	}
	primaryMultiplier := map[string]float64{}
	applyModifiers(rows, sources, lvl, primaryAdditive, primaryMultiplier)
	primary := resolveFinal(primaryAdditive, primaryMultiplier)

	// Phase 2: everything derived from the effective primaries, by the
	// formulas in stats_formulas.go (or stats_custom.go).
	derivedAdditive, derivedMultiplier := derivedFormulas(StatInputs{
		VIT: primary["vit"], DEF: primary["def"], RES: primary["res"], STR: primary["str"],
		DEX: primary["dex"], INT: primary["intel"], WIS: primary["wis"], AGL: primary["agl"],
		Level: lvl, Age: age, HeightM: heightM, WeightKG: weightKG,
		Bases: bases,
	})

	applyModifiers(rows, sources, lvl, derivedAdditive, derivedMultiplier)
	final := resolveFinal(derivedAdditive, derivedMultiplier)

	return ComputedStats{
		Primary: PrimaryStats{
			VIT:   primary["vit"],
			DEF:   primary["def"],
			RES:   primary["res"],
			STR:   primary["str"],
			DEX:   primary["dex"],
			Intel: primary["intel"],
			WIS:   primary["wis"],
			AGL:   primary["agl"],
		},
		Base: BaseStats{
			HP:         final["hp"],
			MP:         final["mp"],
			Attack:     final["attack"],
			Potency:    final["potency"],
			Defense:    final["defense"],
			Resistance: final["resistance"],
			Speed:      final["speed"],
		},
		Special: SpecialStats{
			Luck:                roundTo2(clamp(final["luck"], 0, 2)),
			CarryLimit:          roundTo2(final["carry_limit"]),
			FallDamageThreshold: roundTo2(final["fall_damage_threshold"]),
			Sanity:              roundToInt(final["sanity"]),
			HeatThreshold:       roundToInt(final["heat_threshold"]),
			ColdThreshold:       roundToInt(final["cold_threshold"]),
		},
		Elemental: ElementalDefenses{
			Water:   final["resist_water"],
			Fire:    final["resist_fire"],
			Wind:    final["resist_wind"],
			Earth:   final["resist_earth"],
			Ice:     final["resist_ice"],
			Thunder: final["resist_thunder"],
			Impact:  final["resist_impact"],
		},
		Lifeskills: LifeskillStats{
			Cooking:    roundToInt(final["cooking"]),
			Crafting:   roundToInt(final["crafting"]),
			Alchemy:    roundToInt(final["alchemy"]),
			Hunting:    roundToInt(final["hunting"]),
			Gathering:  roundToInt(final["gathering"]),
			Farming:    roundToInt(final["farming"]),
			Blessing:   roundToInt(final["blessing"]),
			Enchanting: roundToInt(final["enchanting"]),
			Smithing:   roundToInt(final["smithing"]),
		},
	}, nil
}
