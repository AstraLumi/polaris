package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestCleanTags(t *testing.T) {
	got, msg := cleanTags([]string{"  #WarOfAsh ", "warofash", "Plague  Year", "", "#", "WAROFASH", "plague year"})
	if msg != "" {
		t.Fatalf("unexpected error %q", msg)
	}
	want := []string{"WarOfAsh", "Plague Year"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
	if _, msg := cleanTags([]string{strings.Repeat("x", maxTagLength+1)}); msg == "" {
		t.Error("over-long tag should be rejected")
	}
	many := make([]string, maxTagsPerEvent+1)
	for i := range many {
		many[i] = strings.Repeat("a", 1) + string(rune('A'+i%26)) + string(rune('a'+i/26))
	}
	if _, msg := cleanTags(many); msg == "" {
		t.Error("too many tags should be rejected")
	}
}

func TestDateBeforeOrdersChronologically(t *testing.T) {
	// Text order would put "-0054" and "01-01-0999" in the wrong places.
	ordered := []string{"01-01--0200", "01-01--0054", "01-01-0001", "31-12-0999", "01-01-1000", "15-03-1054", "20-03-1054", "garbage", ""}
	for i := 0; i < len(ordered)-1; i++ {
		a, b := ordered[i], ordered[i+1]
		if !dateBefore(a, b) && !(i >= 7) { // the last two are both unreadable: neither is "before"
			t.Errorf("%q should sort before %q", a, b)
		}
		if dateBefore(b, a) {
			t.Errorf("%q must not sort before %q", b, a)
		}
	}
	if dateBefore("garbage", "") || dateBefore("", "garbage") {
		t.Error("two unreadable dates tie")
	}
}

func TestSourceEventName(t *testing.T) {
	if got := sourceEventName(eventSource{Type: "character", Name: "Aria"}); got != "Birth of Aria" {
		t.Errorf("got %q", got)
	}
	if got := sourceEventName(eventSource{Type: "location", Name: "Aldoria"}); got != "Founding of Aldoria" {
		t.Errorf("got %q", got)
	}
}
