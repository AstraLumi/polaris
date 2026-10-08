package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func nullableString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// parseNullableFloat reads a form field that may be blank (left empty on
// purpose) or absent, returning SQL NULL in either case instead of 0.
func parseNullableFloat(s string) sql.NullFloat64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return sql.NullFloat64{}
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: f, Valid: true}
}

func parseIntDefault(s string, fallback int) int {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return fallback
	}
	return v
}

func parseFloatDefault(s string, fallback float64) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return fallback
	}
	return v
}

// uploadURL turns a stored upload path into the URL it is served at ("" stays "").
func uploadURL(stored string) string {
	if stored == "" {
		return ""
	}
	return "/uploads/" + stored
}
