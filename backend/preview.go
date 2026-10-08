package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
)

type computePreviewRequest struct {
	Level          int                `json:"level"`
	Age            float64            `json:"age"`
	Height         float64            `json:"height"`
	Weight         float64            `json:"weight"`
	Class          string             `json:"class"`
	Subclass       string             `json:"subclass"`
	Specialization string             `json:"specialization"`
	Race           string             `json:"race"`
	BodyType       string             `json:"body_type"`
	VIT            int                `json:"vit"`
	DEF            int                `json:"def"`
	RES            int                `json:"res"`
	STR            int                `json:"str"`
	DEX            int                `json:"dex"`
	Intel          int                `json:"intel"`
	WIS            int                `json:"wis"`
	AGL            int                `json:"agl"`
	SpecialBases   map[string]float64 `json:"special_bases"`
	GearIDs        []int64            `json:"gear_ids"` // equipped pieces, one per occupied slot
}

// lookupIDByName finds an existing row by exact name — unlike the upsert
// helpers used when actually saving a character, this never creates
// anything. A name that doesn't exist yet (mid-typing, or a typo) simply
// contributes no modifiers, which is the correct "agnostic" behavior.
func lookupIDByName(db *sql.DB, table, name string) sql.NullInt64 {
	name = strings.TrimSpace(name)
	if name == "" {
		return sql.NullInt64{}
	}
	var id int64
	if err := db.QueryRow(`SELECT id FROM `+table+` WHERE name = ?`, name).Scan(&id); err != nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: id, Valid: true}
}

func computePreviewHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req computePreviewRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		level := req.Level
		if level < 1 {
			level = 1
		}
		height := req.Height
		if height <= 0 {
			height = defaultHeightCM
		}
		weight := req.Weight
		if weight <= 0 {
			weight = defaultWeightKG
		}

		refs := categoryRefs{
			ClassID:          lookupIDByName(db, "classes", req.Class),
			SubclassID:       lookupIDByName(db, "subclasses", req.Subclass),
			SpecializationID: lookupIDByName(db, "specializations", req.Specialization),
			RaceID:           lookupIDByName(db, "races", req.Race),
			BodyTypeID:       lookupIDByName(db, "body_types", req.BodyType),
			ItemIDs:          req.GearIDs,
		}

		build := BuildStats{
			VIT: req.VIT, DEF: req.DEF, RES: req.RES, STR: req.STR,
			DEX: req.DEX, Intel: req.Intel, WIS: req.WIS, AGL: req.AGL,
		}

		computed, err := computeStats(db, level, req.Age, height, weight, build, req.SpecialBases, refs)
		if err != nil {
			http.Error(w, "failed to compute stats", http.StatusInternalServerError)
			return
		}

		writeJSON(w, computed)
	}
}
