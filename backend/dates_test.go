package main

import "testing"

func TestNormalizeStoryDate(t *testing.T) {
	cases := map[string]string{
		"1054": "01-01-1054", "5-3-1054": "05-03-1054", "15-03-1024": "15-03-1024",
		"1": "01-01-0001", "-54": "01-01--0054", "3-4--120": "03-04--0120", "0": "01-01-0000",
		"": "", "  1033 ": "01-01-1033",
		// unreadable text is returned untouched, never discarded
		"spring of 1033": "spring of 1033", "00-01-1033": "00-01-1033", "1-2-3-4": "1-2-3-4",
	}
	for in, want := range cases {
		if got := normalizeStoryDate(in); got != want {
			t.Errorf("normalize(%q)=%q want %q", in, got, want)
		}
	}
	for _, in := range []string{"-54", "1", "3-4--120", "31-12-9999"} {
		c := normalizeStoryDate(in)
		d, ok := parseStoryDate(c)
		if !ok || d.String() != c {
			t.Errorf("round trip failed for %q -> %q", in, c)
		}
	}
}

func TestCheckDateFits(t *testing.T) {
	cal := calendarSettings{MonthsPerYear: 10, DaysPerMonth: 40}
	if msg := checkDateFits(cal, "X", "40-10-1000"); msg != "" {
		t.Errorf("should fit: %s", msg)
	}
	if msg := checkDateFits(cal, "X", "41-1-1000"); msg == "" {
		t.Error("day 41 should not fit")
	}
	if msg := checkDateFits(cal, "X", "1-11-1000"); msg == "" {
		t.Error("month 11 should not fit")
	}
	if msg := checkDateFits(cal, "X", "garbage"); msg != "" {
		t.Error("unreadable text must pass through")
	}
	if msg := checkDateFits(cal, "X", "5-5-0"); msg == "" {
		t.Error("year 0 must be refused")
	}
}

func TestTimeValueHasNoYearZero(t *testing.T) {
	cal := calendarSettings{MonthsPerYear: 12, DaysPerMonth: 30}
	at := func(s string) float64 {
		d, ok := parseStoryDate(s)
		if !ok {
			t.Fatalf("unreadable %q", s)
		}
		return timeValue(d, cal)
	}
	year := float64(12 * 30)
	if got := at("1-1-1") - at("1-1--1"); got != year {
		t.Errorf("-1 to 1 should be one year (%v days), got %v", year, got)
	}
	if got := at("1-1-1055") - at("1-1-1054"); got != year {
		t.Errorf("1054 to 1055 should be one year, got %v", got)
	}
	if got := at("1-2-1054") - at("1-1-1054"); got != 30 {
		t.Errorf("one month should be 30 days, got %v", got)
	}
	// a custom calendar changes the scale, not the logic
	cal = calendarSettings{MonthsPerYear: 10, DaysPerMonth: 40}
	d1, _ := parseStoryDate("1-1-1055")
	d0, _ := parseStoryDate("1-1-1054")
	if got := timeValue(d1, cal) - timeValue(d0, cal); got != 400 {
		t.Errorf("custom calendar year should be 400 days, got %v", got)
	}
}
