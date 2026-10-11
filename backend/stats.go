package main

import (
	"database/sql"
)

// Which stats exist and how each is calculated is the story's stat system
// (statsystem.go). This file feeds it one character: their typed numbers,
// body and level, and the bonuses (stat_modifiers) from whatever they are.

// categoryRefs is everything about a version that a stat_modifiers row
// might key off of. Race and body type live on character_story, so
// they're passed in alongside the identity-level references.
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
	// Age a formula sees when none is set: the neutral point of the default
	// Luck curve, rather than 0, which would read as a newborn.
	defaultAge = 18.0
)

// resolveOrDefault is used by callers (versions.go) to fall back to an
// average-human default when height/weight haven't been set yet, so the
// physical-stat formulas don't produce nonsense for a brand new character.
func resolveOrDefault(v sql.NullFloat64, fallback float64) float64 {
	if v.Valid {
		return v.Float64
	}
	return fallback
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
// scopes a single set of modifier rows to the stats being worked out in one
// pass, without a query per stat. Flat and per-level rows from a source in
// taken are left out: the stat's formula reads them itself.
func applyModifiers(rows []modifierRow, sources []modifierSource, level float64, additive, multiplierDelta map[string]float64, taken map[string]bool) {
	for _, row := range rows {
		if taken[row.sourceType] && row.scaling != "multiplier" {
			continue
		}
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

// bonusesFor adds up, per source_type, the flat and per-level bonuses the
// character's sources give to stat key, as typed (per-level ones not yet
// multiplied by the level): what the formula bonus blocks read.
func bonusesFor(rows []modifierRow, sources []modifierSource, key string) map[string]float64 {
	out := map[string]float64{}
	for _, row := range rows {
		if row.targetStat != key || row.scaling == "multiplier" {
			continue
		}
		for _, s := range sources {
			if s.kind == row.sourceType && s.id.Valid && s.id.Int64 == row.sourceID {
				out[row.sourceType] += row.value
			}
		}
	}
	return out
}

func resolveFinal(additive, multiplierDelta map[string]float64) map[string]float64 {
	final := make(map[string]float64, len(additive))
	for stat, val := range additive {
		final[stat] = val * (1 + multiplierDelta[stat])
	}
	return final
}

// computeStats works out every stat of one character by the story's stat
// system, by stat key, with the bonuses of everything refs points at.
func computeStats(db *sql.DB, sys *statSystem, in statInputs, refs categoryRefs) (map[string]float64, error) {
	rows, err := loadModifierRows(db)
	if err != nil {
		return nil, err
	}
	if in.Values == nil {
		in.Values = map[string]float64{}
	}
	return sys.calculate(in, rows, refs.sources()), nil
}

func (refs categoryRefs) sources() []modifierSource {
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
	return sources
}
