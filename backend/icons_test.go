package main

import "testing"

func TestParseIconChoice(t *testing.T) {
	cases := []struct {
		in   string
		want iconChoice
	}{
		{"", iconChoice{}},
		{"keep", iconChoice{}},
		{"none", iconChoice{change: true}},
		{"builtin:flame", iconChoice{change: true, value: "builtin:flame"}},
		{" builtin:ice ", iconChoice{change: true, value: "builtin:ice"}},
		{"builtin:nonsense", iconChoice{}},
		{"builtin:", iconChoice{}},
		{"../etc/passwd", iconChoice{}},
	}
	for _, c := range cases {
		if got := parseIconChoice(c.in); got != c.want {
			t.Errorf("parseIconChoice(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestIsBuiltinIcon(t *testing.T) {
	if !isBuiltinIcon("builtin:flame") || isBuiltinIcon("spells/3-1.png") || isBuiltinIcon("") {
		t.Fatal("isBuiltinIcon misclassifies")
	}
}
