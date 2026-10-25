package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// layoutDocx builds a three-section book: front matter, then two chapters
// each in its own section, with headers and footers the way novelists set
// them up in Word.
func layoutDocx(t *testing.T) docxSpec {
	t.Helper()
	ref := func(kind, typ, id string) string {
		return fmt.Sprintf(`<w:%sReference w:type="%s" r:id="%s"/>`, kind, typ, id)
	}
	page := `<w:pgSz w:w="5954" w:h="8392"/><w:pgMar w:top="680" w:right="680" w:bottom="680" w:left="680" w:gutter="284"/>`
	body := wp("", wr("", "For the rain.")) +
		wp(`<w:sectPr>`+ref("footer", "default", "rF0")+page+`</w:sectPr>`) +
		wp(`<w:pStyle w:val="Heading1"/>`, wr("", "1. The Start")) + wp("", wr("", "Short.")) +
		wp(`<w:sectPr>`+ref("header", "default", "rH")+ref("header", "even", "rHE")+ref("header", "first", "rH1")+
			ref("footer", "default", "rF")+ref("footer", "even", "rFE")+ref("footer", "first", "rFF")+
			page+`<w:pgNumType w:fmt="upperRoman" w:start="1"/><w:titlePg/></w:sectPr>`) +
		wp(`<w:pStyle w:val="Heading1"/>`, wr("", "2. The Middle")) + wp("", wr("", strings.Repeat("Most words are here. ", 30))) +
		`<w:sectPr>` + ref("header", "first", "rH2") + `<w:type w:val="nextPage"/>` + page + `<w:pgNumType w:fmt="upperRoman" w:start="23"/><w:titlePg/></w:sectPr>`

	hdr := func(kind, inner string) []byte {
		tag := "hdr"
		if kind == "footer" {
			tag = "ftr"
		}
		return []byte(`<w:` + tag + ` ` + docxNS + `>` + inner + `</w:` + tag + `>`)
	}
	style := func(id string, extra string) string { return `<w:pStyle w:val="` + id + `"/>` + extra }
	files := map[string][]byte{
		// A header with a tab: the book's name on the left, the chapter (a STYLEREF) at the style's centre stop.
		"word/header1.xml": hdr("header", wp(style("Header", ""), wr("", "Rainbook"), wrx("", "<w:tab/>"), fld(`STYLEREF "Heading 1"`, wr("", "2. The Middle")))),
		// Even pages: the page number at the right margin (a positional tab).
		"word/header2.xml": hdr("header", wp(style("Header", ""), wrx("", `<w:ptab w:relativeTo="margin" w:alignment="right" w:leader="none"/>`), `<w:fldSimple w:instr=" PAGE "><w:r><w:t>4</w:t></w:r></w:fldSimple>`)),
		// First pages: each chapter's number in words, in capitals.
		"word/header3.xml": hdr("header", wp(style("Header", `<w:jc w:val="center"/>`), wr("", "CHAPTER ONE"))),
		"word/header4.xml": hdr("header", wp(style("Header", `<w:jc w:val="center"/>`), wr("", "CHAPTER TWO"))),
		"word/footer0.xml": hdr("footer", wp(style("Footer", `<w:jc w:val="right"/>`), wr("", "© 2026"))),
		"word/footer1.xml": hdr("footer", wp(style("Footer", `<w:jc w:val="right"/>`), wr("", "Rainbook: Part One – "), fld("PAGE \\* MERGEFORMAT", wr("", "2")), fld("NUMPAGES", wr("", "9")))),
		"word/footer2.xml": hdr("footer", wp(style("Footer", `<w:jc w:val="right"/>`), wr("<w:i/>", "Ann Writer – "), fld("PAGE", wr("", "3")))),
		"word/footer3.xml": hdr("footer", wp(style("Footer", `<w:jc w:val="center"/>`), fld("PAGE", wr("", "1")))),
		"docProps/core.xml": []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" ` +
			`xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Rainbook</dc:title><dc:subject></dc:subject><dc:creator>Ann Writer</dc:creator><dc:language>en-GB</dc:language></cp:coreProperties>`),
	}
	rel := func(id, kind, target string) string {
		return fmt.Sprintf(`<Relationship Id="%s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/%s" Target="%s"/>`, id, kind, target)
	}
	rels := rel("rH", "header", "header1.xml") + rel("rHE", "header", "header2.xml") + rel("rH1", "header", "header3.xml") +
		rel("rH2", "header", "header4.xml") + rel("rF0", "footer", "footer0.xml") + rel("rF", "footer", "footer1.xml") +
		rel("rFE", "footer", "footer2.xml") + rel("rFF", "footer", "footer3.xml")
	styles := `<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/><w:rPr><w:rFonts w:ascii="Merriweather" w:hAnsi="Merriweather"/><w:sz w:val="16"/></w:rPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:basedOn w:val="Normal"/><w:pPr><w:outlineLvl w:val="0"/></w:pPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="Header"><w:name w:val="header"/><w:basedOn w:val="Normal"/><w:pPr><w:tabs><w:tab w:val="right" w:pos="9026"/><w:tab w:val="center" w:pos="4513"/></w:tabs></w:pPr>` +
		`<w:rPr><w:rFonts w:ascii="Cinzel" w:hAnsi="Cinzel"/><w:sz w:val="18"/></w:rPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="Footer"><w:name w:val="footer"/><w:basedOn w:val="Normal"/></w:style>`
	return docxSpec{body: body, rels: rels, files: files, styles: styles, settings: `<w:mirrorMargins/><w:evenAndOddHeaders/>`}
}

func TestDocxLayoutGuess(t *testing.T) {
	imp := importDocx(t, layoutDocx(t))
	l := imp.Layout
	if l == nil {
		t.Fatal("no layout guessed")
	}
	if !l.DifferentFirst || !l.DifferentOddEven || l.ChapterStart != "next" || l.Numbering.Format != "upper_roman" || l.Numbering.Start != 1 {
		t.Errorf("layout: %+v", l)
	}
	band := func(b bookBand) string { return b.Left + "|" + b.Center + "|" + b.Right }
	for name, c := range map[string][2]string{
		"header odd":   {band(l.Header.Odd), "Rainbook|{chapter}|"},
		"header even":  {band(l.Header.Even), "||{page}"},
		"header first": {band(l.Header.First), "|{chapter}|"},
		"footer odd":   {band(l.Footer.Odd), "||Rainbook: Part One – {page}"},
		"footer even":  {band(l.Footer.Even), "||Ann Writer – {page}"},
		"footer first": {band(l.Footer.First), "|{page}|"},
	} {
		if c[0] != c[1] {
			t.Errorf("%s: %q, want %q", name, c[0], c[1])
		}
	}
	h := l.Header.Odd
	if h.Font == nil || *h.Font != "Cinzel" || h.Size == nil || *h.Size != 9 || h.Italic {
		t.Errorf("header's font: %+v", h)
	}
	if !l.Header.First.Uppercase || l.Header.Odd.Uppercase {
		t.Errorf("capitals: first %v odd %v", l.Header.First.Uppercase, l.Header.Odd.Uppercase)
	}
	if f := l.Footer.Odd; f.Font != nil || f.Size == nil || *f.Size != 8 {
		t.Errorf("the footer is in the body's font: %+v", f)
	}
	if !l.Footer.Even.Italic {
		t.Errorf("even footer italic: %+v", l.Footer.Even)
	}
	if imp.Meta["title"] != "Rainbook" || imp.Meta["author"] != "Ann Writer" || imp.Meta["language"] != "en-GB" {
		t.Errorf("meta: %v", imp.Meta)
	}
	if _, ok := imp.Meta["subtitle"]; ok {
		t.Errorf("an empty subject isn't a subtitle: %v", imp.Meta)
	}
	// The guess is a layout the server takes.
	raw, _ := json.Marshal(map[string]any{"page_type": "a6", "layout": l, "meta": imp.Meta})
	if _, msg := cleanBookSettings(raw); msg != "" {
		t.Errorf("guessed layout refused: %s", msg)
	}
	js, _ := json.Marshal(imp)
	if !strings.Contains(string(js), `"layout":{`) || !strings.Contains(string(js), `"meta":{`) {
		t.Errorf("import answer: %s", js)
	}
}

// Odd-page chapter starts, a title written into the header, no even pages
// and no first page of their own.
func TestDocxLayoutGuessSimple(t *testing.T) {
	ref := `<w:headerReference w:type="default" r:id="rH"/>`
	body := wp(`<w:pStyle w:val="Heading1"/>`, wr("", "Dawn")) + wp("", wr("", "Words words.")) +
		wp(`<w:sectPr>`+ref+`<w:type w:val="oddPage"/><w:pgNumType w:fmt="lowerLetter"/></w:sectPr>`) +
		wp(`<w:pStyle w:val="Heading1"/>`, wr("", "Dusk")) + wp("", wr("", "More words here, many more.")) +
		`<w:sectPr>` + ref + `<w:type w:val="oddPage"/><w:pgNumType w:fmt="lowerLetter"/></w:sectPr>`
	spec := docxSpec{body: body,
		rels:   `<Relationship Id="rH" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/>`,
		styles: `<w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:pPr><w:outlineLvl w:val="0"/></w:pPr></w:style>`,
		files:  map[string][]byte{"word/header1.xml": []byte(`<w:hdr ` + docxNS + `>` + wp(`<w:jc w:val="left"/>`, wr("<w:caps/>", "Part I: Dusk")) + `</w:hdr>`)},
	}
	imp := importDocx(t, spec)
	l := imp.Layout
	if l == nil {
		t.Fatal("no layout")
	}
	if l.DifferentFirst || l.DifferentOddEven || l.ChapterStart != "odd" || l.Numbering.Format != "alpha" {
		t.Errorf("layout: %+v", l)
	}
	if l.Header.Odd.Left != "Part I: {chapter}" || l.Header.Even.Left != "Part I: {chapter}" || !l.Header.Odd.Uppercase {
		t.Errorf("header: %+v", l.Header)
	}
	if imp.Meta != nil {
		t.Errorf("meta without core properties: %v", imp.Meta)
	}

	// Nothing about pages in the file: no guess.
	imp = importDocx(t, docxSpec{body: wp("", wr("", "x"))})
	if imp.Layout != nil {
		t.Errorf("layout from nothing: %+v", imp.Layout)
	}
}

func TestNormTitle(t *testing.T) {
	for in, want := range map[string]string{"1. RECODED": "recoded", "  Chapter   Two ": "chapter two", "12 - Sky": "sky", "1984": "1984"} {
		if got := normTitle(in); got != want {
			t.Errorf("normTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

// A cover anchored in the title page's paragraph comes first; a page-sized
// picture later goes to the start of its page. Section starts that carry
// the count on are not restarts; a start lower than the one before is.
func TestDocxCoverFirstAndRestarts(t *testing.T) {
	rels := `<Relationship Id="rIdP" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/image1.png"/>`
	full := anchorPic("rIdP", "page", `<wp:posOffset>0</wp:posOffset>`, "page", 5954*635, 8392*635)
	pg := `<w:pgSz w:w="5954" w:h="8392"/>`
	sect := func(start int) string {
		return fmt.Sprintf(`<w:sectPr>%s<w:pgNumType w:fmt="upperRoman" w:start="%d"/></w:sectPr>`, pg, start)
	}
	h1 := func(s string) string { return wp(`<w:pStyle w:val="Heading1"/>`, wr("", s)) }
	body := wp(`<w:jc w:val="center"/>`, wr("", "THE BOOK"), full) +
		wp(sect(1), wr("", "A line of front matter.")) +
		h1("One") + wp("", wr("", "Words of one.")) + wp("", wrx("", `<w:br w:type="page"/>`)) +
		wp("", wr("", "Map of the realm:"), full) + wp(sect(1), wr("", "After the map.")) +
		h1("Two") + wp(sect(23), wr("", "Words of two.")) +
		h1("Three") + wp("", wr("", strings.Repeat("Most words. ", 40))) + sect(1)
	imp := importDocx(t, docxSpec{body: body, rels: rels, files: map[string][]byte{"word/media/image1.png": testPNG(t)},
		styles: `<w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:pPr><w:outlineLvl w:val="0"/></w:pPr></w:style>`})
	if len(imp.Chapters) != 4 || !imp.Chapters[0].FrontMatter {
		t.Fatalf("chapters: %d", len(imp.Chapters))
	}
	if got := types(imp.Chapters[0].Doc.Content); !strings.HasPrefix(got, "image,paragraph") {
		t.Errorf("front matter: %s (the cover first)", got)
	}
	if got := types(imp.Chapters[1].Doc.Content); got != "heading,paragraph,page_break,image,paragraph,paragraph" {
		t.Errorf("chapter one: %s (the picture at the start of its page)", got)
	}
	if imp.Layout == nil || imp.Layout.Numbering.Format != "upper_roman" || imp.Layout.Numbering.Start != 1 {
		t.Fatalf("layout: %+v", imp.Layout)
	}
	if len(imp.Restarts) != 1 || imp.Restarts[3] != 1 {
		t.Errorf("restarts: %v (only chapter Three's)", imp.Restarts)
	}
}
