package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// commentTexts is the text each comment mark covers in a doc, by id, in
// reading order ("|" between blocks).
func commentTexts(doc *pmNode) map[int64]string {
	out := map[int64]string{}
	last := map[int64]*pmNode{}
	var walk func(n, block *pmNode)
	walk = func(n, block *pmNode) {
		if n.Type == "paragraph" || n.Type == "heading" || n.Type == "dialogue" {
			block = n
		}
		for _, m := range n.Marks {
			if id := commentMarkID(m); id > 0 {
				if last[id] != nil && last[id] != block {
					out[id] += "|"
				}
				out[id] += n.Text
				last[id] = block
			}
		}
		for _, c := range n.Content {
			walk(c, block)
		}
	}
	walk(doc, nil)
	return out
}

const (
	commentsRelType     = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/comments"
	commentsExtRelType2 = "http://schemas.microsoft.com/office/2011/relationships/commentsExtended"
)

func cStart(id int) string { return fmt.Sprintf(`<w:commentRangeStart w:id="%d"/>`, id) }
func cEnd(id int) string   { return fmt.Sprintf(`<w:commentRangeEnd w:id="%d"/>`, id) }
func cRef(id int) string   { return wrx("", fmt.Sprintf(`<w:commentReference w:id="%d"/>`, id)) }

// A file with comments: ranges in a paragraph and overlapping, a reply, a
// resolved one, one across a Heading 1, points, and comments that can't be
// placed.
func commentsDocx(t *testing.T) []byte {
	t.Helper()
	h1 := func(text string) string { return wp(`<w:outlineLvl w:val="0"/>`, wr("", text)) }
	body := wp("", wr("", "Front")) +
		h1("Chapter One") +
		wp("", wr("", "The "), cStart(0), cStart(2), wr("", "quick brown"), cStart(1), wr("", " fox"), cEnd(0), cEnd(2),
			wr("", " jumps"), cEnd(1), cRef(0), cRef(1), cRef(2)) +
		wp("", wr("", "Last words "), cStart(3), wr("", "spanning")) +
		h1("Chapter Two") +
		wp("", wr("", "more"), cEnd(3), cRef(3), wr("", " text")) +
		wp("", wr("", "Point here"), cStart(4), cEnd(4), cRef(4)) +
		wp("", cStart(5), cEnd(5), cRef(5), wr("", " next word")) +
		wp("", wr("", "Unknown"), cRef(6))
	comment := func(id int, para string, texts ...string) string {
		var ps []string
		for i, tx := range texts {
			pid := ""
			if i == len(texts)-1 {
				pid = ` w14:paraId="` + para + `"`
			}
			ps = append(ps, `<w:p`+pid+`><w:r><w:annotationRef/></w:r>`+wr("", tx)+`</w:p>`)
		}
		return fmt.Sprintf(`<w:comment w:id="%d" w:author="Ana" w:date="2025-03-04T05:06:07Z" w:initials="A">%s</w:comment>`, id, strings.Join(ps, ""))
	}
	comments := `<?xml version="1.0" encoding="UTF-8"?><w:comments ` + docxNS + ` xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml">` +
		comment(0, "0000000A", "First line", "second line") + comment(1, "0000000B", "Overlaps") +
		comment(2, "0000000C", "A reply") + comment(3, "0000000D", "Across chapters") +
		comment(4, "0000000E", "A point") + comment(5, "0000000F", "Before the word") +
		comment(7, "00000010", "Never placed") + `</w:comments>`
	ext := `<?xml version="1.0" encoding="UTF-8"?><w15:commentsEx xmlns:w15="http://schemas.microsoft.com/office/word/2012/wordml">` +
		`<w15:commentEx w15:paraId="0000000A" w15:done="1"/><w15:commentEx w15:paraId="0000000B" w15:done="0"/>` +
		`<w15:commentEx w15:paraId="0000000C" w15:paraIdParent="0000000A" w15:done="1"/></w15:commentsEx>`
	return buildDocx(t, docxSpec{
		body: body,
		rels: `<Relationship Id="rIdCm" Type="` + commentsRelType + `" Target="comments.xml"/>` +
			`<Relationship Id="rIdCx" Type="` + commentsExtRelType2 + `" Target="commentsExtended.xml"/>`,
		files: map[string][]byte{"word/comments.xml": []byte(comments), "word/commentsExtended.xml": []byte(ext)},
	})
}

func TestDocxImportComments(t *testing.T) {
	imp, err := parseDocx(commentsDocx(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(imp.Chapters) != 3 {
		t.Fatalf("%d chapters", len(imp.Chapters))
	}
	for i, ch := range imp.Chapters {
		if err := validatePMDoc(ch.Doc); err != nil {
			js, _ := json.Marshal(ch.Doc)
			t.Fatalf("chapter %d: %v\n%s", i, err, js)
		}
	}
	byText := map[string]docxComment{}
	for _, c := range imp.Comments {
		byText[c.Text] = c
	}
	if len(imp.Comments) != 6 {
		t.Fatalf("comments: %+v", imp.Comments)
	}
	first, reply := byText["First line\nsecond line"], byText["A reply"]
	if first.Local == 0 || !first.Resolved || first.Chapter != 1 || !first.Created.Equal(parseWordDate("2025-03-04T05:06:07Z")) {
		t.Errorf("first comment: %+v", first)
	}
	if reply.Parent != first.Local || reply.Chapter != 1 {
		t.Errorf("reply: %+v (first %d)", reply, first.Local)
	}
	if byText["Overlaps"].Resolved || byText["Across chapters"].Chapter != 1 || byText["A point"].Chapter != 2 || byText["Before the word"].Chapter != 2 {
		t.Errorf("comments: %+v", imp.Comments)
	}
	// What each covers.
	one, two := commentTexts(imp.Chapters[1].Doc), commentTexts(imp.Chapters[2].Doc)
	for text, want := range map[string]string{"First line\nsecond line": "quick brown fox", "Overlaps": " fox jumps", "Across chapters": "spanning"} {
		if got := one[byText[text].Local]; got != want {
			t.Errorf("%q covers %q, want %q", text, got, want)
		}
	}
	if _, ok := one[reply.Local]; ok {
		t.Error("a reply's range should not be marked")
	}
	if _, ok := two[byText["Across chapters"].Local]; ok {
		t.Error("a comment's marks in a later chapter should be dropped")
	}
	if two[byText["A point"].Local] != "here" || two[byText["Before the word"].Local] != "next" {
		t.Errorf("points: %v", two)
	}
	// The text is all there.
	if got := nodeText(imp.Chapters[1].Doc.Content[1]); got != "The quick brown fox jumps" {
		t.Errorf("text %q", got)
	}
	// An unknown reference and a comment never placed.
	if imp.LeftOut["comments"] != 2 {
		t.Errorf("left out %v", imp.LeftOut)
	}
}

// The import route makes the rows in the same transaction and puts their ids
// in the marks.
func TestImportDocxCreatesComments(t *testing.T) {
	app, h, base, vol := editorSetup(t)
	body, ct := form(t, nil, "file", "Commented.docx", commentsDocx(t))
	rec := do(t, h, "POST", fmt.Sprintf("%s/books/%d/import-docx", base, vol), body, ct)
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	var out struct {
		Chapters []importedChapter `json:"chapters"`
		LeftOut  map[string]int    `json:"left_out"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out.Chapters) != 3 || out.LeftOut["comments"] != 2 {
		t.Fatalf("answer: %s", rec.Body.String())
	}
	one := out.Chapters[1].ID
	list := listComments(t, h, base, one)
	if len(list) != 4 {
		t.Fatalf("chapter one's comments: %+v", list)
	}
	byText := map[string]commentOut{}
	for _, c := range list {
		byText[c.Text] = c
	}
	first, reply := byText["First line\nsecond line"], byText["A reply"]
	if !first.Resolved || first.ResolvedAt == nil || reply.ParentID == nil || *reply.ParentID != first.ID || first.CreatedAt != "2025-03-04 05:06:07" {
		t.Errorf("rows: %+v", list)
	}
	doc, err := parsePMDoc(getText(t, h, base, one).Doc)
	if err != nil {
		t.Fatal(err)
	}
	if got := commentTexts(doc)[first.ID]; got != "quick brown fox" {
		t.Errorf("stored marks: %v", commentTexts(doc))
	}
	var anchored int
	app.db.QueryRow(`SELECT COUNT(*) FROM chapter_comments WHERE anchored = 1 AND parent_id IS NULL`).Scan(&anchored)
	if anchored != 5 {
		t.Errorf("%d comments anchored, want 5", anchored)
	}
}

// addTestComments puts comments on the test book (export_test.go):
// overlapping ones, one across two paragraphs (resolved, with two replies),
// one in a list with a new line and a tab, and marks that export nothing (a
// deleted comment's, another chapter's).
func addTestComments(b *exportBook) {
	cm := func(id int64) pmMark { return mk("comment", "id", id) }
	one := b.Chapters[1].Doc
	runs := one.Content[1].Content // "Plain, " "bold" " " "italic" " " "under"…
	runs[1].Marks = append(runs[1].Marks, cm(11))
	runs[2].Marks = append(runs[2].Marks, cm(11))
	runs[3].Marks = append(runs[3].Marks, cm(11), cm(12))
	runs[4].Marks = append(runs[4].Marks, cm(12))
	runs[5].Marks = append(runs[5].Marks, cm(12), cm(99))
	one.Content[2].Content[0].Marks = append(one.Content[2].Content[0].Marks, cm(13))
	one.Content[3].Content[0].Marks = append(one.Content[3].Content[0].Marks, cm(13))
	two := b.Chapters[2].Doc
	bullet := two.Content[1].Content[0].Content[0].Content[0] // "first bullet"
	bullet.Marks = append(bullet.Marks, cm(16))
	three := b.Chapters[3].Doc
	three.Content[1].Content[0].Marks = append(three.Content[1].Content[0].Marks, cm(17))
	at := parseWordDate("2026-01-02T03:04:05Z")
	b.Comments = map[int64]*exportComment{
		11: {ID: 11, ChapterID: 2, Text: "Too much formatting?", CreatedAt: at},
		12: {ID: 12, ChapterID: 2, Text: "Overlapping <note> & more", CreatedAt: at},
		13: {ID: 13, ChapterID: 2, Text: "Two paragraphs", Resolved: true, CreatedAt: at},
		14: {ID: 14, ChapterID: 2, ParentID: 13, Text: "First reply", CreatedAt: at},
		15: {ID: 15, ChapterID: 2, ParentID: 13, Text: "Second reply", CreatedAt: at},
		16: {ID: 16, ChapterID: 3, Text: "A line\nand another\twith a tab", CreatedAt: at},
		17: {ID: 17, ChapterID: 2, Text: "Belongs to chapter one, marked in three", CreatedAt: at},
	}
}

// checkCommentsCameBack: after an export and import, the comments are back
// with their ranges, replies and resolved state (TestExportDocxRoundTrip).
func checkCommentsCameBack(t *testing.T, imp *docxImport) {
	t.Helper()
	byText := map[string]docxComment{}
	for _, c := range imp.Comments {
		byText[c.Text] = c
	}
	if len(imp.Comments) != 6 {
		t.Fatalf("comments back: %+v", imp.Comments)
	}
	one, two, three := commentTexts(imp.Chapters[1].Doc), commentTexts(imp.Chapters[2].Doc), commentTexts(imp.Chapters[3].Doc)
	for text, want := range map[string]string{"Too much formatting?": "bold italic", "Overlapping <note> & more": "italic under", "Two paragraphs": "A formatted paragraph.|Exactly spaced."} {
		c := byText[text]
		if c.Chapter != 1 || one[c.Local] != want {
			t.Errorf("%q: chapter %d covers %q, want %q", text, c.Chapter, one[c.Local], want)
		}
	}
	if c := byText["A line\nand another\twith a tab"]; c.Chapter != 2 || two[c.Local] != "first bullet" {
		t.Errorf("list comment: %+v covers %q", c, two[c.Local])
	}
	if len(three) != 0 {
		t.Errorf("chapter three has comments %v", three)
	}
	root := byText["Two paragraphs"]
	if !root.Resolved || byText["Too much formatting?"].Resolved {
		t.Errorf("resolved: %+v", imp.Comments)
	}
	for _, r := range []string{"First reply", "Second reply"} {
		if c := byText[r]; c.Parent != root.Local || c.Chapter != 1 {
			t.Errorf("reply %q: %+v", r, c)
		}
	}
	if !byText["Too much formatting?"].Created.Equal(parseWordDate("2026-01-02T03:04:05Z")) {
		t.Errorf("date: %v", byText["Too much formatting?"].Created)
	}
	if imp.LeftOut["comments"] != 0 {
		t.Errorf("left out %v", imp.LeftOut)
	}
}

// The other exports leave comments out, and the text they split joins up.
func TestExportsLeaveCommentsOut(t *testing.T) {
	doc := &pmNode{Type: "doc", Content: []*pmNode{
		para(pt("Sh", mk("italic")), pt("ad", mk("italic"), mk("comment", "id", 3)), pt("ow", mk("italic"))),
	}}
	b := &exportBook{Chapters: []exportChapter{{ID: 1, Title: "One", Doc: doc}}, Comments: map[int64]*exportComment{3: {ID: 3, ChapterID: 1, Text: "note"}}}
	parseExportSettings(json.RawMessage(`{"page_type":"online"}`), b)
	b.withoutComments()
	if n := len(doc.Content[0].Content); n != 1 || doc.Content[0].Content[0].Text != "Shadow" {
		t.Fatalf("after stripping: %+v", doc.Content[0].Content)
	}
	html, err := exportHTML(b)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(html), "<em>Shadow</em>") || strings.Contains(string(html), "note") {
		t.Errorf("html: %s", html)
	}
}

// The comment parts are well-formed and listed in the package.
func TestExportDocxCommentParts(t *testing.T) {
	b, _ := testBook(t)
	addTestComments(b)
	data, err := exportDocx(b)
	if err != nil {
		t.Fatal(err)
	}
	files, _ := unzip(t, data)
	for _, name := range []string{"word/comments.xml", "word/commentsExtended.xml", "word/document.xml"} {
		if files[name] == nil {
			t.Fatalf("%s is missing", name)
		}
		wellFormed(t, name, files[name])
	}
	ct, rels := string(files["[Content_Types].xml"]), string(files["word/_rels/document.xml.rels"])
	for _, want := range []string{`PartName="/word/comments.xml"`, `PartName="/word/commentsExtended.xml"`} {
		if !strings.Contains(ct, want) {
			t.Errorf("content types lack %s", want)
		}
	}
	for _, want := range []string{`relationships/comments" Target="comments.xml"`, `commentsExtended" Target="commentsExtended.xml"`} {
		if !strings.Contains(rels, want) {
			t.Errorf("relationships lack %s", want)
		}
	}
	if n := strings.Count(string(files["word/document.xml"]), "<w:commentReference "); n != 6 {
		t.Errorf("%d comment references, want 6", n)
	}
	if !strings.Contains(string(files["word/comments.xml"]), `w:author="Ana Autora"`) {
		t.Error("the comments' author should be the book's")
	}
	// A book without comments has no comment parts.
	b2, _ := testBook(t)
	data, _ = exportDocx(b2)
	if files, _ := unzip(t, data); files["word/comments.xml"] != nil {
		t.Error("comments.xml in a book without comments")
	}
}
