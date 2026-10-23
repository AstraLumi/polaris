package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// The calendar keeps the era names it is given, trims them, keeps them when
// an older page leaves them out, and refuses one too long or on two lines.
func TestCalendarEraNames(t *testing.T) {
	_, h, base := planetSetup(t)
	put := func(body string) (int, calendarSettings) {
		rec := do(t, h, "PUT", base+"/settings", strings.NewReader(body), "application/json")
		var out calendarSettings
		json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}
	if c, s := put(`{"months_per_year":12,"days_per_month":30,"hours_per_day":24,"year_before":"  BC ","year_after":"AD"}`); c != 200 || s.YearBefore != "BC" || s.YearAfter != "AD" {
		t.Fatalf("%d %+v", c, s)
	}
	if c, s := put(`{"months_per_year":10,"days_per_month":30}`); c != 200 || s.YearBefore != "BC" || s.MonthsPerYear != 10 {
		t.Errorf("left out, kept: %d %+v", c, s)
	}
	if c, s := put(`{"months_per_year":10,"days_per_month":30,"year_before":""}`); c != 200 || s.YearBefore != "" || s.YearAfter != "AD" {
		t.Errorf("cleared: %d %+v", c, s)
	}
	for _, bad := range []string{`"` + strings.Repeat("x", 25) + `"`, `"two\nlines"`} {
		if c, _ := put(`{"months_per_year":12,"days_per_month":30,"year_after":` + bad + `}`); c != 400 {
			t.Errorf("%s: %d", bad, c)
		}
	}
}
