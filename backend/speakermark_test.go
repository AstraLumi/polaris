package main

import (
	"fmt"
	"strings"
	"testing"
)

// "Tag character": speaker-marked runs in prose are lines of that character,
// like Script Mode's dialogues; the text stays plain everywhere else.
func TestSpeakerMarksAreLines(t *testing.T) {
	const src = `{"type":"doc","content":[
 {"type":"paragraph","content":[
  {"type":"text","text":"He shouted, "},
  {"type":"text","text":"\"Obby! ","marks":[{"type":"speaker","attrs":{"character":5,"speaker":"Valiente"}}]},
  {"type":"text","text":"I'm in position!\"","marks":[{"type":"speaker","attrs":{"character":5,"speaker":"Valiente"}},{"type":"bold"}]},
  {"type":"text","text":" and left."}]},
 {"type":"dialogue","attrs":{"character":6,"speaker":"Obby"},"content":[{"type":"text","text":"Fine."}]},
 {"type":"paragraph","content":[{"type":"text","text":"\"Who?\"","marks":[{"type":"speaker","attrs":{"character":null,"speaker":"A guard"}}]}]}
]}`
	doc, err := parsePMDoc([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	lines := docDialogues(doc)
	if len(lines) != 3 || lines[0] != (pmDialogue{5, "Valiente", "", `"Obby! I'm in position!"`}) ||
		lines[1].Character != 6 || lines[2] != (pmDialogue{0, "A guard", "", `"Who?"`}) {
		t.Errorf("docDialogues = %+v", lines)
	}
	if got := docLinkCounts(doc); fmt.Sprint(got) != fmt.Sprint([]pmLinkCount{{pmStoryLink{"character", 5}, 1}, {pmStoryLink{"character", 6}, 1}}) {
		t.Errorf("docLinkCounts = %v", got)
	}
	if text := docText(doc); !strings.HasPrefix(text, `He shouted, "Obby! I'm in position!" and left.`) {
		t.Errorf("docText = %q", text)
	}
	for _, bad := range []string{
		`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"x","marks":[{"type":"speaker","attrs":{"character":0}}]}]}]}`,
		`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"x","marks":[{"type":"speaker","attrs":{"character":"Mira"}}]}]}]}`,
		`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"x","marks":[{"type":"speaker","attrs":{"speaker":"` + strings.Repeat("x", 201) + `"}}]}]}]}`,
	} {
		if _, err := parsePMDoc([]byte(bad)); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
}
