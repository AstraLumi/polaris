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
