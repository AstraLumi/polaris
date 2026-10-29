package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestSwapDocFont(t *testing.T) {
	doc := `{"type":"doc","content":[{"type":"paragraph","content":[
		{"type":"text","text":"Go "},
		{"type":"text","marks":[{"type":"bold"},{"type":"font","attrs":{"family":"Cambria"}}],"text":"north ★ "},
		{"type":"text","marks":[{"type":"font","attrs":{"family":"cambria"}}],"text":"→ now"}]},
		{"type":"paragraph","content":[{"type":"text","marks":[{"type":"font","attrs":{"family":"Other"}}],"text":"a b"}]}]}`
	load := func() *pmNode {
		var d pmNode
		if err := json.Unmarshal([]byte(doc), &d); err != nil {
			t.Fatal(err)
		}
		return &d
	}
	text := func(d *pmNode) string {
		b, _ := json.Marshal(d)
		return string(b)
	}

	// Spaces to the body font: they lose the mark and join the plain text.
	d := load()
	if n := swapDocFont(d, "CAMBRIA", isFontSpace, ""); n != 3 {
		t.Errorf("spaces swapped: %d", n)
	}
	got := text(d)
	for _, want := range []string{
		`{"type":"text","text":"Go "}`,
		`"marks":[{"type":"bold"}],"text":" "}`,
		`{"type":"text","text":" "}`,
		`"text":"a b"`, // another font's spaces stay
	} {
		if !strings.Contains(got, want) {
			t.Errorf("spaces: no %s in %s", want, got)
		}
	}
	if err := validatePMDoc(d); err != nil {
		t.Error(err)
	}
	if c := docFontChars(d); c["Cambria"].spaces != 0 || c["Other"].spaces != 1 {
		t.Errorf("after the swap: %+v %+v", c["Cambria"], c["Other"])
	}

	// Two symbols to another font; the rest of the text stays in Cambria.
	d = load()
	c := docFontChars(d)["Cambria"]
	if c.spaces != 3 || c.symbols["★"] != 1 || c.symbols["→"] != 1 || len(c.symbols) != 2 {
		t.Errorf("counts: %+v", c)
	}
	set := map[rune]bool{'★': true, '→': true}
	if n := swapDocFont(d, "Cambria", func(r rune) bool { return set[r] }, "Noto Sans Symbols 2"); n != 2 {
		t.Errorf("symbols swapped: %d", n)
	}
	got = text(d)
	if !strings.Contains(got, `{"type":"bold"},{"type":"font","attrs":{"family":"Noto Sans Symbols 2"}}],"text":"★"}`) ||
		!strings.Contains(got, `"family":"Noto Sans Symbols 2"}}],"text":"→"}`) || !strings.Contains(got, `"text":"north "`) {
		t.Errorf("symbols: %s", got)
	}

	// All of it: the font is gone from the doc, its pieces joined.
	d = load()
	swapDocFont(d, "cambria", func(rune) bool { return true }, "")
	got = text(d)
	if strings.Contains(strings.ToLower(got), "cambria") || !strings.Contains(got, `"marks":[{"type":"bold"}],"text":"north ★ "}`) || !strings.Contains(got, `{"type":"text","text":"→ now"}`) {
		t.Errorf("all: %s", got)
	}
}

func TestSwapBookFont(t *testing.T) {
	_, h := newTestServer(t)
	id, vol, ch := fontStory(t, h, "Swap Story")
	base := "/api/s/" + id
	doc := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Plain"},` +
		`{"type":"text","marks":[{"type":"font","attrs":{"family":"Cambria Math"}}],"text":" ∞ "},{"type":"text","text":"end"}]}]}`
	before := getText(t, h, base, ch)
	if code, _ := putText(t, h, base, ch, doc, before.Version); code != 200 {
		t.Fatal(code)
	}
	fonts := func() map[string]bookFontOut {
		rec := do(t, h, "GET", fmt.Sprintf("%s/books/%d/fonts", base, vol), nil, "")
		var out []bookFontOut
		json.Unmarshal(rec.Body.Bytes(), &out)
		m := map[string]bookFontOut{}
		for _, o := range out {
			m[strings.ToLower(o.Family)] = o
		}
		return m
	}
	o := fonts()["cambria math"]
	if o.Source != "system" || o.Spaces != 2 || len(o.Symbols) != 1 || o.Symbols[0] != (fontSymbol{"∞", 1}) {
		t.Fatalf("cambria math: %+v", o)
	}

	swap := func(body string) (int, string) {
		rec := do(t, h, "POST", fmt.Sprintf("%s/books/%d/fonts/swap", base, vol), strings.NewReader(body), "application/json")
		return rec.Code, rec.Body.String()
	}
	for _, bad := range []string{`{"family":"","what":"all"}`, `{"family":"Cambria Math","what":"chars"}`, `{"family":"Cambria Math","what":"later"}`, `{"family":"Cambria Math","what":"all","to":"cambria math"}`} {
		if code, _ := swap(bad); code != 400 {
			t.Errorf("%s: %d", bad, code)
		}
	}
	v0 := getText(t, h, base, ch).Version
	if code, body := swap(`{"family":"Cambria Math","what":"spaces","label":"Before the spaces"}`); code != 200 || !strings.Contains(body, `"chapters":1`) || !strings.Contains(body, `"pieces":2`) {
		t.Fatal(code, body)
	}
	after := getText(t, h, base, ch)
	if after.Version != v0+1 || !strings.Contains(string(after.Doc), `{"type":"text","text":"Plain "}`) || !strings.Contains(string(after.Doc), `{"type":"text","text":" end"}`) {
		t.Errorf("after the spaces: v%d %s", after.Version, after.Doc)
	}
	if vs := listTextVersions(t, h, base, ch); len(vs) == 0 || vs[0].Label != "Before the spaces" {
		t.Errorf("versions: %+v", vs)
	}
	if o := fonts()["cambria math"]; o.Spaces != 0 || o.Chapters != 1 {
		t.Errorf("after the spaces: %+v", o)
	}
	// Nothing left to change: no chapter is touched, no version kept.
	if code, body := swap(`{"family":"Cambria Math","what":"spaces"}`); code != 200 || !strings.Contains(body, `"chapters":0`) {
		t.Error(code, body)
	}
	if code, body := swap(`{"family":"Cambria Math","what":"chars","chars":"∞","to":"Noto Sans Math"}`); code != 200 || !strings.Contains(body, `"chapters":1`) {
		t.Fatal(code, body)
	}
	m := fonts()
	if _, ok := m["cambria math"]; ok {
		t.Errorf("Cambria Math is still listed: %+v", m["cambria math"])
	}
	if o := m["noto sans math"]; o.Source != "bundled" || len(o.Symbols) != 1 {
		t.Errorf("noto sans math: %+v", o)
	}
}
