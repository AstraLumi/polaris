package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Story dates are stored as text in one canonical shape: DD-MM-YYYY, with
// the year zero-padded to four digits and an optional leading minus for
// years before year 1 ("01-01--0054" is year -54). Day and month are
// always present once a date is saved — a bare year becomes the first day
// of the first month.
//
// The calendar itself (how many months a year has, how many days a month
// has) is a user setting, so nothing in here assumes 12 or 30/31 — range
// checks go through calendarSettings (see settings.go).

type storyDate struct {
	Day, Month, Year int
}

// Accepts "YYYY" (year only) or "D-M-YYYY" / "DD-MM-YYYY"; the year may be
// negative. Anything else is not a date this app can place on a timeline.
var storyDateRe = regexp.MustCompile(`^(?:(\d{1,3})-(\d{1,3})-)?(-?\d{1,9})$`)

// parseStoryDate reads a stored or typed date. ok is false for blank or
// unreadable text, and for day/month 0 (which no calendar has).
func parseStoryDate(s string) (storyDate, bool) {
	m := storyDateRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return storyDate{}, false
	}
	year, err := strconv.Atoi(m[3])
	if err != nil {
		return storyDate{}, false
	}
	d := storyDate{Day: 1, Month: 1, Year: year}
	if m[1] != "" {
		d.Day, _ = strconv.Atoi(m[1])
		d.Month, _ = strconv.Atoi(m[2])
		if d.Day < 1 || d.Month < 1 {
			return storyDate{}, false
		}
	}
	return d, true
}

func (d storyDate) String() string {
	year := fmt.Sprintf("%04d", d.Year)
	if d.Year < 0 {
		year = fmt.Sprintf("-%04d", -d.Year)
	}
	return fmt.Sprintf("%02d-%02d-%s", d.Day, d.Month, year)
}

// fits reports whether the date exists in the configured calendar.
func (d storyDate) fits(cal calendarSettings) bool {
	return d.Day >= 1 && d.Day <= cal.DaysPerMonth && d.Month >= 1 && d.Month <= cal.MonthsPerYear
}

// normalizeStoryDate trims a typed date and rewrites it into the canonical
// shape when it can be read ("1054" -> "01-01-1054"). Text that can't be
// read is returned as typed, never discarded: an old free-text value
// shouldn't vanish just because someone saved an unrelated field.
func normalizeStoryDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if d, ok := parseStoryDate(s); ok {
		return d.String()
	}
	return s
}

// checkDateFits returns a user-facing message when s is a readable date
// that doesn't exist in the configured calendar (e.g. day 31 in a world
// with 30-day months). Blank and unreadable text pass — unreadable legacy
// text is kept and flagged in the UI rather than blocking a save.
func checkDateFits(cal calendarSettings, label, s string) string {
	d, ok := parseStoryDate(s)
	if !ok {
		return ""
	}
	if d.Year == 0 {
		return label + ": there is no year 0 — the year before 1 is -1"
	}
	if d.fits(cal) {
		return ""
	}
	return fmt.Sprintf("%s: day %d / month %d doesn't exist — this calendar has %d months of %d days",
		label, d.Day, d.Month, cal.MonthsPerYear, cal.DaysPerMonth)
}

// timeValue places a date on a single number line, in days, so the
// timeline can space events in proportion to the time between them. There
// is no year 0 (the year before 1 is -1), so a gap from -1 to 1 counts as
// one year, not two. A year typed as 0 — which validation refuses — is
// treated as year 1.
func timeValue(d storyDate, cal calendarSettings) float64 {
	yearIndex := d.Year
	if yearIndex > 0 {
		yearIndex--
	}
	yearDays := cal.MonthsPerYear * cal.DaysPerMonth
	return float64(yearIndex)*float64(yearDays) +
		float64((d.Month-1)*cal.DaysPerMonth) + float64(d.Day-1)
}
