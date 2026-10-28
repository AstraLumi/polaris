package main

import (
	"strings"
	"testing"
)

func testNote(kind, text string) *pmNode {
	return blk("footnote", kv("kind", kind), para(pt(text)))
}

func TestPMDocValidatesNotes(t *testing.T) {
	ok := &pmNode{Type: "doc", Content: []*pmNode{para(pt("Text"), testNote("footnote", "A note."))}}
	if err := validatePMDoc(ok); err != nil {
		t.Fatalf("a footnote with a paragraph should pass: %v", err)
	}
	nested := &pmNode{Type: "doc", Content: []*pmNode{para(pt("Text"),
		blk("footnote", kv("kind", "footnote"), para(pt("x"), testNote("endnote", "inner"))))}}
	if err := validatePMDoc(nested); err == nil {
		t.Fatal("a note inside a note should be rejected")
	}
}

func TestExportDocxRoundTripNotes(t *testing.T) {
	b, up := testBook(t)
	doc := b.Chapters[1].Doc
	doc.Content = append(doc.Content, para(pt("Body with "), testNote("footnote", "Footnote text here."),
		pt(" and "), testNote("endnote", "Endnote text here."), pt(" end.")))
	data, err := exportDocx(b)
	if err != nil {
		t.Fatal(err)
	}
	imp, _ := reimport(t, data, up)
	var notes []docNote
	for _, c := range imp.Chapters {
		notes = append(notes, docNotes(c.Doc)...)
	}
	if len(notes) != 2 {
		t.Fatalf("%d notes came back, want 2", len(notes))
	}
	for i, want := range []struct{ kind, text string }{{"footnote", "Footnote text here."}, {"endnote", "Endnote text here."}} {
		n := notes[i]
		if n.Kind != want.kind || !strings.Contains(docPlain(n.Node), want.text) {
			t.Errorf("note %d: kind %s text %q, want %s %q", i, n.Kind, docPlain(n.Node), want.kind, want.text)
		}
	}
	for _, k := range []string{"footnotes", "endnotes"} {
		if imp.LeftOut[k] != 0 {
			t.Errorf("left_out lists %s: %v", k, imp.LeftOut)
		}
	}
}

func docPlain(n *pmNode) string {
	if n == nil {
		return ""
	}
	s := n.Text
	for _, c := range n.Content {
		s += docPlain(c)
	}
	return s
}
