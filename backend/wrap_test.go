package main

import (
	"strings"
	"testing"
)

func wrapDoc(attrs map[string]any) *pmNode {
	return &pmNode{Type: "doc", Content: []*pmNode{blk("image", attrs)}}
}

func TestValidatePictureWrap(t *testing.T) {
	for _, w := range []string{"inline", "left", "right"} {
		if err := validatePMDoc(wrapDoc(kv("src", "/uploads/editor/pic.png", "wrap", w))); err != nil {
			t.Errorf("wrap %q refused: %v", w, err)
		}
	}
	// An old doc has no wrap at all.
	if err := validatePMDoc(wrapDoc(kv("src", "/uploads/editor/pic.png"))); err != nil {
		t.Errorf("no wrap refused: %v", err)
	}
	for _, bad := range []any{"center", "float", 3.0} {
		if err := validatePMDoc(wrapDoc(kv("src", "/uploads/editor/pic.png", "wrap", bad))); err == nil {
			t.Errorf("wrap %v was accepted", bad)
		}
	}
}

// A left and a right floated picture go to .docx as wrapSquare anchors,
// come back as wrapped pictures, and are not reported as left out.
func TestExportDocxRoundTripWrappedPictures(t *testing.T) {
	b, up := testBook(t)
	doc := b.Chapters[1].Doc
	doc.Content = append(doc.Content,
		blk("image", kv("src", "/uploads/editor/pic.png", "alt", "Left one", "width", 100.0, "align", "left", "wrap", "left")),
		blk("paragraph", nil, pt("Text beside the left picture.")),
		blk("image", kv("src", "/uploads/editor/pic.png", "alt", "Right one", "width", 100.0, "align", "right", "wrap", "right")),
		blk("paragraph", nil, pt("Text beside the right picture.")))
	data, err := exportDocx(b)
	if err != nil {
		t.Fatal(err)
	}
	imp, _ := reimport(t, data, up)
	got := map[string]string{}
	for _, c := range imp.Chapters {
		for _, n := range c.Doc.Content {
			if n.Type == "image" && attrString(n, "alt") != "" {
				got[attrString(n, "alt")] = attrString(n, "wrap") + "/" + attrString(n, "align")
			}
		}
	}
	if got["Left one"] != "left/left" || got["Right one"] != "right/right" {
		t.Errorf("wrap after round trip: %v", got)
	}
	if n := imp.LeftOut["pictures"]; n != 0 {
		t.Errorf("left_out lists pictures: %v", imp.LeftOut)
	}
}

// The XML a wrapped picture is written as.
func TestExportDocxWrappedPictureXML(t *testing.T) {
	b, _ := testBook(t)
	b.Chapters[1].Doc.Content = append(b.Chapters[1].Doc.Content,
		blk("image", kv("src", "/uploads/editor/pic.png", "width", 100.0, "wrap", "right")))
	data, err := exportDocx(b)
	if err != nil {
		t.Fatal(err)
	}
	files, _ := unzip(t, data)
	xml := string(files["word/document.xml"])
	for _, want := range []string{`<wp:wrapSquare wrapText="bothSides"/>`, `<wp:align>right</wp:align>`, `relativeFrom="column"`} {
		if !strings.Contains(xml, want) {
			t.Errorf("document.xml lacks %s", want)
		}
	}
}
