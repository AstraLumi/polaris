package main

import (
	"strings"
	"testing"
)

func boxPara(text string, attrs map[string]any) *pmNode {
	return blk("paragraph", attrs, pt(text))
}

func TestValidateBoxAttrs(t *testing.T) {
	ok := map[string]any{"boxWidth": 1.5, "boxColor": "#aa00ff", "shade": "#eeeeee", "dropCap": 3.0}
	doc := &pmNode{Type: "doc", Content: []*pmNode{boxPara("Hello", ok)}}
	if err := validatePMDoc(doc); err != nil {
		t.Fatalf("valid box attrs refused: %v", err)
	}
	for _, bad := range []map[string]any{{"boxWidth": 99.0}, {"boxColor": "red"}, {"shade": "#12345"}, {"dropCap": 7.0}, {"dropCap": 1.0}} {
		d := &pmNode{Type: "doc", Content: []*pmNode{boxPara("Hello", bad)}}
		if err := validatePMDoc(d); err == nil {
			t.Errorf("%v was accepted", bad)
		}
	}
}

// A box, shading and a drop cap go to .docx and come back.
func TestExportDocxRoundTripKeepLines(t *testing.T) {
	b, up := testBook(t)
	doc := b.Chapters[1].Doc
	doc.Content = append(doc.Content, boxPara("Kept whole.", map[string]any{"keepLines": true}), boxPara("Free to split.", nil),
		boxPara("Starts a page.", map[string]any{"pageBreakBefore": true}), boxPara("Stays with the next.", map[string]any{"keepNext": true}))
	data, err := exportDocx(b)
	if err != nil {
		t.Fatal(err)
	}
	imp, _ := reimport(t, data, up)
	found := map[string]*pmNode{}
	breakBefore := false
	for _, c := range imp.Chapters {
		for i, n := range c.Doc.Content {
			found[docPlain(n)] = n
			if docPlain(n) == "Starts a page." && i > 0 && c.Doc.Content[i-1].Type == "page_break" {
				breakBefore = true
			}
		}
	}
	if !breakBefore {
		t.Error("Page break before did not come back as a page break")
	}
	if n := found["Stays with the next."]; n == nil || n.Attrs["keepNext"] != true {
		t.Errorf("keep with next came back as %+v", n)
	}
	if n := found["Kept whole."]; n == nil || n.Attrs["keepLines"] != true {
		t.Errorf("kept paragraph came back as %+v", n)
	}
	if n := found["Free to split."]; n == nil || n.Attrs["keepLines"] != nil {
		t.Errorf("plain paragraph came back as %+v", n)
	}
}

func TestExportDocxRoundTripBoxes(t *testing.T) {
	b, up := testBook(t)
	box := map[string]any{"boxWidth": 1.5, "boxColor": "#3366cc", "shade": "#e8f0ff"}
	doc := b.Chapters[1].Doc
	doc.Content = append(doc.Content, boxPara("System window one.", box), boxPara("System window two.", box),
		boxPara("Dropped paragraph text.", map[string]any{"dropCap": 3.0}))
	data, err := exportDocx(b)
	if err != nil {
		t.Fatal(err)
	}
	imp, _ := reimport(t, data, up)
	found := map[string]*pmNode{}
	var walk func(n *pmNode)
	walk = func(n *pmNode) {
		if n.Type == "paragraph" {
			found[docPlain(n)] = n
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	for _, c := range imp.Chapters {
		walk(c.Doc)
	}
	for _, text := range []string{"System window one.", "System window two."} {
		n := found[text]
		if n == nil {
			t.Fatalf("%q did not come back", text)
		}
		if w, _ := pmNum(n.Attrs["boxWidth"]); w != 1.5 || n.Attrs["boxColor"] != "#3366cc" || n.Attrs["shade"] != "#e8f0ff" {
			t.Errorf("%q: box attrs %v", text, n.Attrs)
		}
	}
	d := found["Dropped paragraph text."]
	if d == nil {
		var all []string
		for k := range found {
			all = append(all, k)
		}
		t.Fatalf("drop cap paragraph not joined back: %s", strings.Join(all, " | "))
	}
	if n, _ := pmInt(d.Attrs["dropCap"]); n != 3 {
		t.Errorf("drop cap lines: %v", d.Attrs)
	}
	for _, k := range []string{"borders_shading", "drop_caps"} {
		if imp.LeftOut[k] != 0 {
			t.Errorf("left_out lists %s: %v", k, imp.LeftOut)
		}
	}
}
