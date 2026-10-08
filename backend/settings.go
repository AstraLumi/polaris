package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
)

// App-wide settings live in a small key/value table so a new setting
// never needs a schema change.
const appSettingsTableSQL = `CREATE TABLE app_settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
)`

const (
	settingMonthsPerYear = "calendar_months_per_year"
	settingDaysPerMonth  = "calendar_days_per_month"

	defaultMonthsPerYear = 12
	defaultDaysPerMonth  = 30

	// Generous ceilings — they exist to catch typos, not to limit worlds.
	maxMonthsPerYear = 100
	maxDaysPerMonth  = 400
)

// calendarSettings describes the story world's calendar. Quarters (Q1–Q4
// on the timeline) are derived from it: a year is split into four equal
// parts of however many months it has.
type calendarSettings struct {
	MonthsPerYear int `json:"months_per_year"`
	DaysPerMonth  int `json:"days_per_month"`
}

func readIntSetting(db *sql.DB, key string, fallback, min, max int) int {
	var raw string
	if err := db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, key).Scan(&raw); err != nil {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < min || v > max {
		return fallback
	}
	return v
}

func loadCalendar(db *sql.DB) calendarSettings {
	return calendarSettings{
		MonthsPerYear: readIntSetting(db, settingMonthsPerYear, defaultMonthsPerYear, 1, maxMonthsPerYear),
		DaysPerMonth:  readIntSetting(db, settingDaysPerMonth, defaultDaysPerMonth, 1, maxDaysPerMonth),
	}
}

func getSettingsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, loadCalendar(db))
	}
}

func updateSettingsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in calendarSettings
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if in.MonthsPerYear < 1 || in.MonthsPerYear > maxMonthsPerYear {
			http.Error(w, "months per year must be between 1 and "+strconv.Itoa(maxMonthsPerYear), http.StatusBadRequest)
			return
		}
		if in.DaysPerMonth < 1 || in.DaysPerMonth > maxDaysPerMonth {
			http.Error(w, "days per month must be between 1 and "+strconv.Itoa(maxDaysPerMonth), http.StatusBadRequest)
			return
		}

		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to start transaction", http.StatusInternalServerError)
			return
		}
		for key, val := range map[string]int{
			settingMonthsPerYear: in.MonthsPerYear,
			settingDaysPerMonth:  in.DaysPerMonth,
		} {
			if _, err := tx.Exec(
				`INSERT INTO app_settings (key, value) VALUES (?, ?)
				 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
				key, strconv.Itoa(val),
			); err != nil {
				tx.Rollback()
				log.Printf("save setting %s: %v", key, err)
				http.Error(w, "failed to save settings", http.StatusInternalServerError)
				return
			}
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to save settings", http.StatusInternalServerError)
			return
		}
		writeJSON(w, loadCalendar(db))
	}
}
