package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// Seasons are the story's own: named spans, each so many months long, that
// repeat from a start date. The cycle needn't be a year long, so a winter
// can last three years. They describe one hemisphere; the other is half a
// cycle apart (frontend/src/seasons.js works that out for the calendar).
// They live in app_settings as JSON, like the rest of the calendar.
//
// A region's temperatures per season live in location_climate, keyed by the
// season's id, so renaming a season keeps them.

const settingSeasons = "calendar_seasons"

const (
	maxSeasons       = 24   // the error messages say so
	maxSeasonMonths  = 1200 // the error messages say so
	maxSeasonNameLen = 60
)

type season struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Months float64 `json:"months"`
	Color  string  `json:"color"`
}

type seasonSettings struct {
	Seasons []season `json:"seasons"`
	// The day the first season starts, DD-MM-YYYY. The cycle repeats from
	// there both ways.
	Start      string `json:"start"`
	Hemisphere string `json:"hemisphere"` // the one these seasons describe: "north" | "south"
	Unit       string `json:"unit"`       // temperatures: "C" | "F"
}

var seasonIDRe = regexp.MustCompile(`^[a-z0-9-]{1,40}$`)

// defaultSeasons: winter, spring, summer and fall, each a quarter of the
// year, winter starting with the last month (Q4 into Q1, and so on).
func defaultSeasons(cal calendarSettings) seasonSettings {
	quarter := math.Round(float64(cal.MonthsPerYear)/4*100) / 100
	return seasonSettings{
		Seasons: []season{
			{ID: "winter", Name: "Winter", Months: quarter, Color: "#7fb2e5"},
			{ID: "spring", Name: "Spring", Months: quarter, Color: "#86cf7f"},
			{ID: "summer", Name: "Summer", Months: quarter, Color: "#f2c14e"},
			{ID: "fall", Name: "Fall", Months: quarter, Color: "#e0864a"},
		},
		Start:      storyDate{Day: 1, Month: cal.MonthsPerYear, Year: 1}.String(),
		Hemisphere: "north",
		Unit:       "C",
	}
}

func loadSeasons(db *sql.DB, cal calendarSettings) seasonSettings {
	var raw string
	if db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, settingSeasons).Scan(&raw) == nil {
		var s seasonSettings
		if json.Unmarshal([]byte(raw), &s) == nil && s.Seasons != nil {
			return s
		}
	}
	return defaultSeasons(cal)
}

// clean checks a save against the calendar and fills in what's missing.
func (s *seasonSettings) clean(cal calendarSettings) string {
	if len(s.Seasons) > maxSeasons {
		return "a story can have at most 24 seasons"
	}
	if s.Hemisphere != "south" {
		s.Hemisphere = "north"
	}
	if s.Unit != "F" {
		s.Unit = "C"
	}
	// Seasons keep their ids (a region's temperatures are keyed by them); a
	// new or repeated one gets an id no other season has.
	taken := map[string]bool{}
	keep := make([]bool, len(s.Seasons))
	for i, x := range s.Seasons {
		if seasonIDRe.MatchString(x.ID) && !taken[x.ID] {
			taken[x.ID], keep[i] = true, true
		}
	}
	next := 1
	for i := range s.Seasons {
		x := &s.Seasons[i]
		if !keep[i] {
			for x.ID = "s" + strconv.Itoa(next); taken[x.ID]; x.ID = "s" + strconv.Itoa(next) {
				next++
			}
			taken[x.ID] = true
		}
		x.Name = strings.TrimSpace(x.Name)
		if x.Name == "" {
			return "every season needs a name"
		}
		if tooLong(x.Name, maxSeasonNameLen) {
			return "that season name is too long"
		}
		if !(x.Months > 0) || x.Months > maxSeasonMonths {
			return "a season lasts more than 0 and at most 1200 months"
		}
		x.Months = math.Round(x.Months*100) / 100
		if x.Color != "" && !hexColorRe.MatchString(x.Color) {
			x.Color = ""
		}
	}
	if len(s.Seasons) == 0 {
		s.Start = ""
		return ""
	}
	// A year alone means its first day.
	d, ok := parseStoryDate(s.Start)
	if !ok {
		return "the first season needs a start date"
	}
	if msg := checkDateFits(cal, "Start of the first season", d.String()); msg != "" {
		return msg
	}
	s.Start = d.String()
	return ""
}

func updateSeasonsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in seasonSettings
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if in.Seasons == nil {
			in.Seasons = []season{}
		}
		cal := loadCalendar(db)
		if msg := in.clean(cal); msg != "" {
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		raw, _ := json.Marshal(in)
		if _, err := db.Exec(
			`INSERT INTO app_settings (key, value) VALUES (?, ?)
			 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
			settingSeasons, string(raw),
		); err != nil {
			log.Printf("save seasons: %v", err)
			http.Error(w, "failed to save the seasons", http.StatusInternalServerError)
			return
		}
		writeJSON(w, loadSeasons(db, cal))
	}
}

// ---- Region climate ------------------------------------------------------------

const locationClimateTableSQL = `CREATE TABLE location_climate (
	location_id INTEGER NOT NULL REFERENCES locations(id) ON DELETE CASCADE,
	season_id   TEXT NOT NULL,
	low         REAL,
	high        REAL,
	PRIMARY KEY (location_id, season_id)
)`

// schema26Statements upgrade a database to schema 26: temperatures per
// season for regions. The fresh schema runs them too.
var schema26Statements = []string{locationClimateTableSQL}

// A season's temperature range in one region; either end may be unknown.
type climateRow struct {
	SeasonID string   `json:"season_id"`
	Low      *float64 `json:"low"`
	High     *float64 `json:"high"`
}

// Temperatures beyond this are typos, whatever the unit.
const maxTemperature = 10000

func loadClimate(db *sql.DB, locationID int64) []climateRow {
	out := []climateRow{}
	rows, err := db.Query(`SELECT season_id, low, high FROM location_climate WHERE location_id = ? ORDER BY season_id`, locationID)
	if err != nil {
		log.Printf("load climate: %v", err)
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var c climateRow
		var lo, hi sql.NullFloat64
		if rows.Scan(&c.SeasonID, &lo, &hi) != nil {
			continue
		}
		if lo.Valid {
			c.Low = &lo.Float64
		}
		if hi.Valid {
			c.High = &hi.Float64
		}
		out = append(out, c)
	}
	return out
}

// cleanClimate drops rows with nothing in them and puts each range the
// right way round.
func cleanClimate(in []climateRow) ([]climateRow, string) {
	var out []climateRow
	seen := map[string]bool{}
	for _, c := range in {
		if c.Low == nil && c.High == nil {
			continue
		}
		if !seasonIDRe.MatchString(c.SeasonID) || seen[c.SeasonID] {
			return nil, "unknown season"
		}
		seen[c.SeasonID] = true
		for _, v := range []*float64{c.Low, c.High} {
			if v != nil && (math.IsNaN(*v) || math.Abs(*v) > maxTemperature) {
				return nil, "that temperature is out of range"
			}
		}
		if c.Low != nil && c.High != nil && *c.Low > *c.High {
			c.Low, c.High = c.High, c.Low
		}
		out = append(out, c)
	}
	if len(out) > maxSeasons*4 {
		return nil, "too many seasons"
	}
	return out, ""
}

func saveClimate(tx *sql.Tx, locationID int64, rows []climateRow) error {
	if _, err := tx.Exec(`DELETE FROM location_climate WHERE location_id = ?`, locationID); err != nil {
		return err
	}
	for _, c := range rows {
		if _, err := tx.Exec(`INSERT INTO location_climate (location_id, season_id, low, high) VALUES (?, ?, ?, ?)`,
			locationID, c.SeasonID, c.Low, c.High); err != nil {
			return err
		}
	}
	return nil
}

// ---- The map's equator -----------------------------------------------------------

const settingEquator = "map_equator"

// mapEquator is a line across the hex map. Row is the hex row (r) it runs
// along at column Center (fractions fall between rows); Bend curves it,
// in rows per 100 columns squared away from Center, for when the map is a
// projection of a globe rather than flat.
type mapEquator struct {
	Show   bool    `json:"show"`
	Row    float64 `json:"row"`
	Center float64 `json:"center"`
	Bend   float64 `json:"bend"`
}

const maxEquatorCoord = 1e6

func loadEquator(db *sql.DB) mapEquator {
	var raw string
	var e mapEquator
	if db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, settingEquator).Scan(&raw) == nil {
		json.Unmarshal([]byte(raw), &e)
	}
	return e
}

func updateEquatorHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in mapEquator
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		for _, v := range []float64{in.Row, in.Center} {
			if math.IsNaN(v) || math.Abs(v) > maxEquatorCoord {
				http.Error(w, "the equator is off the map", http.StatusBadRequest)
				return
			}
		}
		if math.IsNaN(in.Bend) || math.Abs(in.Bend) > 1000 {
			http.Error(w, "that bend is out of range", http.StatusBadRequest)
			return
		}
		raw, _ := json.Marshal(in)
		if _, err := db.Exec(
			`INSERT INTO app_settings (key, value) VALUES (?, ?)
			 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
			settingEquator, string(raw),
		); err != nil {
			log.Printf("save equator: %v", err)
			http.Error(w, "failed to save the equator", http.StatusInternalServerError)
			return
		}
		writeJSON(w, loadEquator(db))
	}
}
