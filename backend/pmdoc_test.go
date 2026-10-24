package main

import (
	"encoding/json"
	"strings"
	"testing"
)

const pmSample = `{"type":"doc","attrs":{"future":1},"content":[
 {"type":"heading","attrs":{"level":1,"style":"heading1","shiny":true},"content":[{"type":"text","text":"1. Recoded"}]},
 {"type":"paragraph","attrs":{"align":"justify","indentFirst":14.2,"lineHeight":1,"keepNext":false,"unknownAttr":{"a":[1,2]}},"content":[
   {"type":"text","text":"Hello, "},
   {"type":"text","text":"Mira","marks":[{"type":"story_link","attrs":{"type":"character","id":12}},{"type":"bold"}]},
   {"type":"text","text":" — it's 3 o'clock"},{"type":"hard_break"},{"type":"text","text":"next line"},{"type":"tab"},
   {"type":"text","text":"Ilvar","marks":[{"type":"story_link","attrs":{"type":"location","id":"7"}}]},
   {"type":"text","text":" and Mira again.","marks":[{"type":"story_link","attrs":{"type":"character","id":12}},{"type":"link","attrs":{"href":"https://example.org/x"}}]}
 ],"futureKey":"kept"},
 {"type":"scene_break","attrs":{"glyph":"⁂"}},
 {"type":"page_break"},
 {"type":"image","attrs":{"src":"/uploads/chapters/a.png","alt":"","width":null,"align":"center","fullPage":true}},
 {"type":"bullet_list","content":[{"type":"list_item","content":[{"type":"paragraph","content":[{"type":"text","text":"one item"}]}]}]},
 {"type":"ordered_list","attrs":{"start":3},"content":[{"type":"list_item","content":[{"type":"paragraph"}]}]},
 {"type":"paragraph"}
]}`

func TestPMDocRoundTripKeepsUnknowns(t *testing.T) {
	doc, err := parsePMDoc([]byte(pmSample))
	if err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"future":1`, `"shiny":true`, `"unknownAttr":{"a":[1,2]}`, `"futureKey":"kept"`, `"indentFirst":14.2`, `"id":12`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("lost %s in %s", want, out)
		}
	}
	// A second round trip gives the same bytes.
	var again pmNode
	if err := json.Unmarshal(out, &again); err != nil {
		t.Fatal(err)
	}
	out2, _ := json.Marshal(&again)
	if string(out) != string(out2) {
		t.Errorf("round trip changed the doc:\n%s\n%s", out, out2)
	}
}

func TestPMDocWordsTextLinks(t *testing.T) {
	doc, err := parsePMDoc([]byte(pmSample))
	if err != nil {
		t.Fatal(err)
	}
	// "1. Recoded" (2) + "Hello, Mira — it's 3 o'clock / next line / Ilvar and Mira again." (11) + "one item" (2)
	if got := docWords(doc); got != 15 {
		t.Errorf("docWords = %d, want 15", got)
	}
	text := docText(doc)
	if !strings.Contains(text, "1. Recoded\nHello, Mira — it's 3 o'clock\nnext line\tIlvar") || !strings.Contains(text, "\n⁂\n") {
		t.Errorf("docText = %q", text)
	}
	links := docStoryLinks(doc)
	if len(links) != 2 || links[0] != (pmStoryLink{"character", 12}) || links[1] != (pmStoryLink{"location", 7}) {
		t.Errorf("docStoryLinks = %v", links)
	}
}

func TestCountWords(t *testing.T) {
	cases := map[string]int{
		"":                       0,
		"   ":                    0,
		"one two  three":         3,
		"— * * * —":              0,
		"well—maybe":             1,
		"3 o'clock, 1054.":       3,
		"日本語 テキスト":               2,
		"a b":                    2,
		"(…)":                    0,
		"Ünïcödé wörds… ⁂ fine!": 3,
	}
	for s, want := range cases {
		if got := countWords(s); got != want {
			t.Errorf("countWords(%q) = %d, want %d", s, got, want)
		}
	}
}

func TestPMDocRejects(t *testing.T) {
	bad := map[string]string{
		"not json":         `{"type":`,
		"root":             `{"type":"paragraph"}`,
		"empty doc":        `{"type":"doc","content":[]}`,
		"unknown node":     `{"type":"doc","content":[{"type":"table"}]}`,
		"inline at top":    `{"type":"doc","content":[{"type":"text","text":"x"}]}`,
		"block in para":    `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"paragraph"}]}]}`,
		"empty text":       `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":""}]}]}`,
		"unknown mark":     `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"x","marks":[{"type":"blink"}]}]}]}`,
		"double mark":      `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"x","marks":[{"type":"bold"},{"type":"bold"}]}]}]}`,
		"script link":      `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"x","marks":[{"type":"link","attrs":{"href":"javascript:alert(1)"}}]}]}]}`,
		"data link":        `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"x","marks":[{"type":"link","attrs":{"href":" DATA:text/html,x"}}]}]}]}`,
		"remote image":     `{"type":"doc","content":[{"type":"image","attrs":{"src":"https://evil.example/x.png"}}]}`,
		"dotdot image":     `{"type":"doc","content":[{"type":"image","attrs":{"src":"/uploads/../db.sqlite"}}]}`,
		"heading level":    `{"type":"doc","content":[{"type":"heading","attrs":{"level":7}}]}`,
		"bad align":        `{"type":"doc","content":[{"type":"paragraph","attrs":{"align":"middle"}}]}`,
		"bad color":        `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"x","marks":[{"type":"color","attrs":{"color":"red"}}]}]}]}`,
		"bad story link":   `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"x","marks":[{"type":"story_link","attrs":{"type":"Character","id":0}}]}]}]}`,
		"list of paras":    `{"type":"doc","content":[{"type":"bullet_list","content":[{"type":"paragraph"}]}]}`,
		"empty list item":  `{"type":"doc","content":[{"type":"bullet_list","content":[{"type":"list_item"}]}]}`,
		"atom with kids":   `{"type":"doc","content":[{"type":"page_break","content":[{"type":"paragraph"}]}]}`,
		"marks on a block": `{"type":"doc","content":[{"type":"paragraph","marks":[{"type":"bold"}]}]}`,
		"deep":             strings.Repeat(`{"type":"doc","content":[`, 40) + strings.Repeat(`]}`, 40),
	}
	for name, src := range bad {
		if _, err := parsePMDoc([]byte(src)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	big := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"` + strings.Repeat("x", maxPMDocBytes) + `"}]}]}`
	if _, err := parsePMDoc([]byte(big)); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Errorf("huge doc: %v", err)
	}
	if _, err := parsePMDoc([]byte(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"x","marks":[{"type":"link","attrs":{"href":"#_Toc12"}}]}]}]}`)); err != nil {
		t.Errorf("anchor link rejected: %v", err)
	}
}

func TestEmptyPMDocIsValid(t *testing.T) {
	if err := validatePMDoc(emptyPMDoc()); err != nil {
		t.Fatal(err)
	}
	if docWords(emptyPMDoc()) != 0 || docText(emptyPMDoc()) != "" {
		t.Error("empty doc has text")
	}
}
