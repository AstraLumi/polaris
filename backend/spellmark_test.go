package main

import (
	"fmt"
	"strings"
	"testing"
)

// "Tag spell": a spell-marked run is an appearance of that spell; the text
// stays plain in the exports.
func TestSpellMarksCountAndValidate(t *testing.T) {
	const src = `{"type":"doc","content":[
 {"type":"paragraph","content":[
  {"type":"text","text":"She cried "},
  {"type":"text","text":"Fireball! ","marks":[{"type":"spell","attrs":{"spell":4,"name":"Fireball"}}]},
  {"type":"text","text":"now","marks":[{"type":"spell","attrs":{"spell":4,"name":"Fireball"}},{"type":"bold"}]},
  {"type":"text","text":" and "},
  {"type":"text","text":"again","marks":[{"type":"spell","attrs":{"spell":4,"name":"Fireball"}}]}]}
]}`
	doc, err := parsePMDoc([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	want := []pmLinkCount{{pmStoryLink{"spell", 4}, 2}}
	if got := docLinkCounts(doc); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("docLinkCounts = %v, want %v", got, want)
	}
	if text := docText(doc); !strings.HasPrefix(text, "She cried Fireball! now and again") {
		t.Errorf("docText = %q", text)
	}
	for _, bad := range []string{
		`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"x","marks":[{"type":"spell","attrs":{}}]}]}]}`,
		`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"x","marks":[{"type":"spell","attrs":{"spell":0}}]}]}]}`,
		`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"x","marks":[{"type":"spell","attrs":{"spell":"a"}}]}]}]}`,
		`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"x","marks":[{"type":"spell","attrs":{"spell":1,"name":"` + strings.Repeat("x", 201) + `"}}]}]}]}`,
	} {
		if _, err := parsePMDoc([]byte(bad)); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
}

// Tags (speaker and spell) never reach an export: the words stay, plain.
func TestExportsDropTags(t *testing.T) {
	b, _ := testBook(t)
	tagged := blk("paragraph", nil,
		pt("Zorbles ", mk("speaker", "character", 3, "speaker", "Mira")),
		pt("cast Quuxfire", mk("spell", "spell", 9, "name", "Quuxfire")),
		pt(" at dawn."))
	b.Chapters[1].Doc.Content = append(b.Chapters[1].Doc.Content, tagged)
	for _, f := range []string{"docx", "epub", "html", "md", "txt"} {
		data, err := exportFormats[f](b)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		all := string(data)
		if f == "docx" || f == "epub" {
			files, _ := unzip(t, data)
			var sb strings.Builder
			for n, d := range files {
				if strings.HasSuffix(n, ".xml") || strings.HasSuffix(n, ".xhtml") {
					sb.Write(d)
				}
			}
			all = sb.String()
		}
		if !strings.Contains(all, "Zorbles") || !strings.Contains(all, "at dawn.") {
			t.Errorf("%s lost the tagged words or the text after them", f)
		}
		for _, bad := range []string{"pm-said", "data-said", "data-spell", "pm-spell"} {
			if strings.Contains(all, bad) {
				t.Errorf("%s holds tag markup %q", f, bad)
			}
		}
	}
}
