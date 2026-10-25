package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"strings"
	"testing"
)

// Small .docx files built in code: no real manuscripts in the repo.

const docxNS = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
	`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
	`xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" ` +
	`xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
	`xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture" ` +
	`xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006" ` +
	`xmlns:v="urn:schemas-microsoft-com:vml" ` +
	`xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape" ` +
	`xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math" ` +
	`xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart"`

const relNS = `xmlns="http://schemas.openxmlformats.org/package/2006/relationships"`

const testTheme = `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="T"><a:themeElements>` +
	`<a:fontScheme name="F"><a:majorFont><a:latin typeface="Calibri Light"/></a:majorFont>` +
	`<a:minorFont><a:latin typeface="Calibri"/></a:minorFont></a:fontScheme></a:themeElements></a:theme>`

type docxSpec struct {
	body      string            // inside w:body
	styles    string            // inside w:styles ("" = no styles part)
	numbering string            // inside w:numbering
	settings  string            // inside w:settings
	theme     bool              // add testTheme
	rels      string            // extra Relationship elements of the document
	files     map[string][]byte // extra parts
}

func buildDocx(t *testing.T, s docxSpec) []byte {
	t.Helper()
	files := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8"?><Relationships ` + relNS + `><Relationship Id="rId1" ` +
			`Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:document ` + docxNS + `><w:body>` + s.body + `</w:body></w:document>`,
	}
	rels := s.rels
	add := func(id, typ, target, content string) {
		rels += fmt.Sprintf(`<Relationship Id="%s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/%s" Target="%s"/>`, id, typ, target)
		files["word/"+target] = content
	}
	if s.styles != "" {
		add("rIdStyles", "styles", "styles.xml", `<w:styles `+docxNS+`>`+s.styles+`</w:styles>`)
	}
	if s.numbering != "" {
		add("rIdNum", "numbering", "numbering.xml", `<w:numbering `+docxNS+`>`+s.numbering+`</w:numbering>`)
	}
	if s.settings != "" {
		add("rIdSettings", "settings", "settings.xml", `<w:settings `+docxNS+`>`+s.settings+`</w:settings>`)
	}
	if s.theme {
		add("rIdTheme", "theme", "theme/theme1.xml", testTheme)
	}
	files["word/_rels/document.xml.rels"] = `<?xml version="1.0" encoding="UTF-8"?><Relationships ` + relNS + `>` + rels + `</Relationships>`
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(content))
	}
	for name, content := range s.files {
		w, _ := zw.Create(name)
		w.Write(content)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

var xmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")

// wp is a paragraph; wr a run with text; wrx a run with raw content.
func wp(pPr string, content ...string) string {
	if pPr != "" {
		pPr = "<w:pPr>" + pPr + "</w:pPr>"
	}
	return "<w:p>" + pPr + strings.Join(content, "") + "</w:p>"
}

func wr(rPr, text string) string {
	return wrx(rPr, `<w:t xml:space="preserve">`+xmlEscaper.Replace(text)+`</w:t>`)
}

func wrx(rPr, inner string) string {
	if rPr != "" {
		rPr = "<w:rPr>" + rPr + "</w:rPr>"
	}
	return "<w:r>" + rPr + inner + "</w:r>"
}

// fld is a complex field with its shown result.
func fld(instr, result string) string {
	return wrx("", `<w:fldChar w:fldCharType="begin"/>`) + wrx("", `<w:instrText xml:space="preserve"> `+xmlEscaper.Replace(instr)+` </w:instrText>`) +
		wrx("", `<w:fldChar w:fldCharType="separate"/>`) + result + wrx("", `<w:fldChar w:fldCharType="end"/>`)
}

func importDocx(t *testing.T, s docxSpec) *docxImport {
	t.Helper()
	imp, err := parseDocx(buildDocx(t, s))
	if err != nil {
		t.Fatal(err)
	}
	urls := make([]string, len(imp.Pictures))
	for i := range urls {
		urls[i] = fmt.Sprintf("/uploads/chapters/pic-%d.png", i)
	}
	imp.usePictureURLs(urls)
	for _, ch := range imp.Chapters {
		if err := validatePMDoc(ch.Doc); err != nil {
			js, _ := json.Marshal(ch.Doc)
			t.Fatalf("chapter %q: %v\n%s", ch.Title, err, js)
		}
	}
	return imp
}

// allBlocks is every chapter's blocks, in order.
func allBlocks(imp *docxImport) []*pmNode {
	var out []*pmNode
	for _, ch := range imp.Chapters {
		out = append(out, ch.Doc.Content...)
	}
	return out
}

func nodeText(n *pmNode) string { return pmTextblockText(n, "\n", "\t") }

// textNode finds the text node holding s.
func textNode(t *testing.T, imp *docxImport, s string) *pmNode {
	t.Helper()
	var found *pmNode
	for _, ch := range imp.Chapters {
		pmWalk(ch.Doc, func(n *pmNode) {
			if found == nil && n.Type == "text" && strings.Contains(n.Text, s) {
				found = n
			}
		})
	}
	if found == nil {
		t.Fatalf("no text %q", s)
	}
	return found
}

func markMap(n *pmNode) map[string]map[string]any {
	m := map[string]map[string]any{}
	for _, mk := range n.Marks {
		m[mk.Type] = mk.Attrs
		if mk.Attrs == nil {
			m[mk.Type] = map[string]any{}
		}
	}
	return m
}

func attrNum(n *pmNode, key string) (float64, bool) {
	v, ok := n.Attrs[key]
	if !ok {
		return 0, false
	}
	return pmNum(v)
}

func types(nodes []*pmNode) string {
	var s []string
	for _, n := range nodes {
		s = append(s, n.Type)
	}
	return strings.Join(s, ",")
}

func styleByID(imp *docxImport, id string) *bookStyle {
	for i := range imp.Styles {
		if imp.Styles[i].ID == id {
			return &imp.Styles[i]
		}
	}
	return nil
}

// --- styles and chapters ---

const testStyles = `<w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:asciiTheme="minorHAnsi" w:hAnsiTheme="minorHAnsi"/><w:sz w:val="24"/></w:rPr></w:rPrDefault>` +
	`<w:pPrDefault><w:pPr><w:spacing w:after="160" w:line="259" w:lineRule="auto"/></w:pPr></w:pPrDefault></w:docDefaults>` +
	`<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/></w:style>` +
	// A Portuguese Word's Heading 1: the styleId is translated, the name isn't.
	`<w:style w:type="paragraph" w:styleId="Ttulo1"><w:name w:val="heading 1"/><w:basedOn w:val="Normal"/><w:pPr><w:keepNext/>` +
	`<w:spacing w:before="480" w:after="240"/><w:jc w:val="center"/><w:outlineLvl w:val="0"/></w:pPr>` +
	`<w:rPr><w:rFonts w:asciiTheme="majorHAnsi" w:hAnsiTheme="majorHAnsi"/><w:b/><w:sz w:val="40"/><w:color w:val="2F5496"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Heading2"><w:name w:val="heading 2"/><w:basedOn w:val="Ttulo1"/><w:pPr><w:outlineLvl w:val="1"/></w:pPr><w:rPr><w:sz w:val="32"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Title"><w:name w:val="Title"/><w:basedOn w:val="Normal"/><w:rPr><w:sz w:val="56"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Letter"><w:name w:val="Letter Text"/><w:basedOn w:val="Normal"/><w:pPr><w:ind w:left="720" w:right="720"/></w:pPr>` +
	`<w:rPr><w:i/><w:rFonts w:ascii="Georgia" w:hAnsi="Georgia"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="LetterSig"><w:name w:val="Letter Signature"/><w:basedOn w:val="Letter"/><w:pPr><w:jc w:val="right"/></w:pPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Unused"><w:name w:val="Never Used"/><w:basedOn w:val="Normal"/></w:style>` +
	`<w:style w:type="character" w:default="1" w:styleId="DefaultParagraphFont"><w:name w:val="Default Paragraph Font"/></w:style>` +
	`<w:style w:type="character" w:styleId="Shout"><w:name w:val="Shout"/><w:basedOn w:val="DefaultParagraphFont"/><w:rPr><w:b/><w:color w:val="C00000"/></w:rPr></w:style>`

func TestDocxChaptersAndStyles(t *testing.T) {
	handFormatted := `<w:jc w:val="both"/><w:ind w:firstLine="284"/><w:spacing w:after="40" w:line="240" w:lineRule="auto"/>`
	merri := `<w:rFonts w:ascii="Merriweather" w:hAnsi="Merriweather"/><w:sz w:val="16"/>`
	body := wp(`<w:pStyle w:val="Title"/>`, wr("", "My Book")) +
		wp("") +
		wp(`<w:pStyle w:val="Ttulo1"/>`, wr("", "Chapter "), wr("", "One")) +
		strings.Repeat(wp(handFormatted, wr(merri, "It was a dark and stormy night; the rain fell in torrents.")), 4) +
		wp(`<w:pStyle w:val="Letter"/>`, wr("", "Dear friend, "), wr(`<w:rStyle w:val="Shout"/>`, "HEY")) +
		wp(`<w:pStyle w:val="LetterSig"/>`, wr("", "Yours")) +
		wp(`<w:pStyle w:val="Heading2"/>`, wr("", "A section")) +
		wp(`<w:pStyle w:val="Ttulo1"/><w:pageBreakBefore/>`, wr("", "Chapter Two")) +
		wp("", wr("", "More text."))
	imp := importDocx(t, docxSpec{body: body, styles: testStyles, theme: true})

	if len(imp.Chapters) != 3 {
		t.Fatalf("chapters: %d", len(imp.Chapters))
	}
	fm, one, two := imp.Chapters[0], imp.Chapters[1], imp.Chapters[2]
	if !fm.FrontMatter || fm.Title != "" || types(fm.Doc.Content) != "paragraph,paragraph" || fm.Words != 2 {
		t.Errorf("front matter: %+v %s", fm, types(fm.Doc.Content))
	}
	if fm.Doc.Content[0].Attrs["style"] != "title" {
		t.Errorf("title paragraph: %v", fm.Doc.Content[0].Attrs)
	}
	if one.Title != "Chapter One" || one.FrontMatter || two.Title != "Chapter Two" {
		t.Errorf("titles %q %q", one.Title, two.Title)
	}
	// The page break before Chapter Two isn't left at the end of Chapter One.
	if got := types(one.Doc.Content); got != "heading,paragraph,paragraph,paragraph,paragraph,paragraph,paragraph,heading" {
		t.Errorf("chapter one: %s", got)
	}
	h := one.Doc.Content[0]
	if lv, _ := pmInt(h.Attrs["level"]); lv != 1 || h.Attrs["style"] != "heading1" || len(h.Attrs) != 2 || nodeText(h) != "Chapter One" {
		t.Errorf("heading: %v %q", h.Attrs, nodeText(h))
	}
	if len(h.Content) != 1 || len(h.Content[0].Marks) != 0 {
		t.Errorf("heading text should carry no marks: %+v", h.Content)
	}
	if h2 := one.Doc.Content[7]; h2.Attrs["style"] != "heading2" || h2.Attrs["level"] != 2 {
		t.Errorf("heading 2: %v", h2.Attrs)
	}
	// Hand formatting is the Normal style's now: the paragraphs carry nothing.
	if p := one.Doc.Content[1]; p.Attrs != nil || len(p.Content[0].Marks) != 0 {
		t.Errorf("body paragraph: %v %v", p.Attrs, p.Content[0].Marks)
	}
	// The one paragraph left as Word made it says how it differs.
	more := two.Doc.Content[1]
	want := map[string]float64{"indentFirst": 0, "spaceAfter": 8, "lineHeight": 1.08}
	for k, v := range want {
		if got, ok := attrNum(more, k); !ok || got != v {
			t.Errorf("More text %s = %v, want %v (%v)", k, got, v, more.Attrs)
		}
	}
	if more.Attrs["align"] != "left" {
		t.Errorf("More text align: %v", more.Attrs)
	}
	if m := markMap(more.Content[0]); m["font"]["family"] != "Calibri" || m["size"]["pt"] != 12.0 {
		t.Errorf("More text marks: %v", m)
	}
	// The character style is named by a mark; its bold and colour are the style's.
	if m := markMap(textNode(t, imp, "HEY")); m["char_style"]["id"] != "word-shout" || len(m) != 1 {
		t.Errorf("HEY marks: %v", m)
	}
	if m := markMap(textNode(t, imp, "Dear friend")); len(m) != 0 {
		t.Errorf("letter text marks: %v", m)
	}

	// Styles.
	var ids []string
	for _, s := range imp.Styles {
		ids = append(ids, s.ID)
	}
	if got := strings.Join(ids, ","); got != "normal,heading1,heading2,title,word-lettersig,word-letter,word-shout" {
		t.Errorf("styles: %s", got)
	}
	js := func(v any) string { b, _ := json.Marshal(v); return string(b) }
	checks := map[string][2]string{
		"normal": {`{"align":"justify","indentFirst":14.2,"indentLeft":0,"indentRight":0,"keepNext":false,"lineHeight":1,"spaceAfter":2,"spaceBefore":0}`,
			`{"color":null,"font":"Merriweather","size":8}`},
		"heading1": {`{"align":"center","indentFirst":0,"keepNext":true,"lineHeight":1.08,"spaceAfter":12,"spaceBefore":24}`,
			`{"bold":true,"color":"#2f5496","font":"Calibri Light","size":20}`},
		"heading2":       {`null`, `{"size":16}`},
		"title":          {`{"align":"left","indentFirst":0,"lineHeight":1.08,"spaceAfter":8}`, `{"font":"Calibri","size":28}`},
		"word-letter":    {`{"align":"left","indentFirst":0,"indentLeft":36,"indentRight":36,"lineHeight":1.08,"spaceAfter":8}`, `{"font":"Georgia","italic":true,"size":12}`},
		"word-lettersig": {`{"align":"right"}`, `{}`},
	}
	for id, w := range checks {
		s := styleByID(imp, id)
		if s == nil {
			t.Errorf("no style %s", id)
			continue
		}
		para := js(s.Para)
		if len(s.Para) == 0 {
			para = "null"
		}
		if para != w[0] || js(s.Text) != w[1] {
			t.Errorf("%s: para %s text %s\nwant para %s text %s", id, para, js(s.Text), w[0], w[1])
		}
	}
	n, h1, sig, shout := styleByID(imp, "normal"), styleByID(imp, "heading1"), styleByID(imp, "word-lettersig"), styleByID(imp, "word-shout")
	if n.BasedOn != nil || !n.Builtin || n.Name != "Normal" || n.Kind != "paragraph" {
		t.Errorf("normal: %+v", n)
	}
	if h1.BasedOn == nil || *h1.BasedOn != "normal" || !h1.Builtin || h1.Name != "Heading 1" {
		t.Errorf("heading1: %+v", h1)
	}
	if sig.BasedOn == nil || *sig.BasedOn != "word-letter" || sig.Builtin || sig.Name != "Letter Signature" {
		t.Errorf("letter signature: %+v", sig)
	}
	if shout.Kind != "character" || shout.BasedOn != nil || js(shout.Text) != `{"bold":true,"color":"#c00000"}` {
		t.Errorf("shout: %+v %s", shout, js(shout.Text))
	}
	if len(imp.Fonts) < 3 || imp.Fonts[0] != "Merriweather" {
		t.Errorf("fonts: %v", imp.Fonts)
	}
	// The answer's JSON has exactly the contract's keys.
	var answer map[string]any
	json.Unmarshal([]byte(js(imp)), &answer)
	if len(answer) != 5 || answer["page_type_guess"] != "custom" || answer["left_out"] == nil {
		t.Errorf("answer: %s", js(imp))
	}
}

func TestDocxNoHeadingIsOneChapterAndEmptyFrontMatterIsDropped(t *testing.T) {
	imp := importDocx(t, docxSpec{body: wp("", wr("", "Just a story.")) + wp("")})
	if len(imp.Chapters) != 1 || !imp.Chapters[0].FrontMatter || imp.Chapters[0].Words != 3 {
		t.Fatalf("%+v", imp.Chapters)
	}
	// Empty paragraphs are kept.
	if types(imp.Chapters[0].Doc.Content) != "paragraph,paragraph" {
		t.Errorf("%s", types(imp.Chapters[0].Doc.Content))
	}
	imp = importDocx(t, docxSpec{body: wp("") + wp("", wrx("", `<w:br w:type="page"/>`)) +
		wp(`<w:outlineLvl w:val="0"/>`, wr("", "Prologue")) + wp("", wr("", "Text."))})
	if len(imp.Chapters) != 1 || imp.Chapters[0].Title != "Prologue" {
		t.Fatalf("%+v", imp.Chapters)
	}
	// An empty Heading 1 doesn't start a chapter.
	imp = importDocx(t, docxSpec{body: wp(`<w:outlineLvl w:val="0"/>`, wr("", "One")) + wp(`<w:outlineLvl w:val="0"/>`) + wp("", wr("", "x"))})
	if len(imp.Chapters) != 1 {
		t.Errorf("empty heading split: %d", len(imp.Chapters))
	}
}

// --- paragraphs ---

func TestDocxParagraphFormatting(t *testing.T) {
	long := strings.Repeat("Plain paragraph text that sets the defaults. ", 12)
	body := wp("", wr("", long)) +
		wp(`<w:jc w:val="both"/>`, wr("", "justified")) +
		wp(`<w:jc w:val="center"/>`, wr("", "centred")) +
		wp(`<w:jc w:val="end"/>`, wr("", "right")) +
		wp(`<w:ind w:firstLine="284"/>`, wr("", "first line")) +
		wp(`<w:ind w:left="720" w:hanging="360"/>`, wr("", "hanging")) +
		wp(`<w:ind w:start="720" w:end="1440"/>`, wr("", "start end")) +
		wp(`<w:spacing w:before="240" w:after="120"/>`, wr("", "spaced")) +
		wp(`<w:spacing w:beforeAutospacing="1" w:afterAutospacing="1"/>`, wr("", "auto spaced")) +
		wp(`<w:spacing w:line="360" w:lineRule="auto"/>`, wr("", "one and a half")) +
		wp(`<w:spacing w:line="480" w:lineRule="exact"/>`, wr(`<w:sz w:val="20"/>`, "exactly 24")) +
		wp(`<w:spacing w:line="200" w:lineRule="atLeast"/>`, wr(`<w:sz w:val="20"/>`, "at least 10")) +
		wp(`<w:keepNext/>`, wr("", "keep with next")) +
		wp("", wr("", "before"), wrx("", `<w:br w:type="page"/>`), wr("", "after")) +
		wp("", wrx("", `<w:br w:type="page"/>`)) +
		wp("", wr("", "a"), wrx("", `<w:br/>`), wr("", "b"), wrx("", `<w:tab/>`), wr("", "c"), wrx("", `<w:cr/>`), wr("", "d")) +
		wp("") +
		wp(`<w:pBdr><w:top w:val="single"/></w:pBdr>`, wr("", "boxed"))
	imp := importDocx(t, docxSpec{body: body})
	bs := allBlocks(imp)
	if got := types(bs); got != strings.Repeat("paragraph,", 14)+"page_break,paragraph,page_break,paragraph,paragraph,paragraph" {
		t.Fatalf("blocks: %s", got)
	}
	byText := map[string]*pmNode{}
	for _, b := range bs {
		byText[nodeText(b)] = b
	}
	check := func(text, key string, want any) {
		t.Helper()
		b := byText[text]
		if b == nil {
			t.Errorf("no paragraph %q", text)
			return
		}
		got := b.Attrs[key]
		if f, ok := pmNum(got); ok {
			got = f
		}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%q %s = %v, want %v (attrs %v)", text, key, got, want, b.Attrs)
		}
	}
	if bs[0].Attrs != nil {
		t.Errorf("plain paragraph attrs: %v", bs[0].Attrs)
	}
	check("justified", "align", "justify")
	check("centred", "align", "center")
	check("right", "align", "right")
	check("first line", "indentFirst", 14.2)
	check("hanging", "indentLeft", 36)
	check("hanging", "indentFirst", -18)
	check("start end", "indentLeft", 36)
	check("start end", "indentRight", 72)
	check("spaced", "spaceBefore", 12)
	check("spaced", "spaceAfter", 6)
	check("auto spaced", "spaceBefore", 14)
	check("auto spaced", "spaceAfter", 14)
	check("one and a half", "lineHeight", 1.5)
	check("exactly 24", "lineHeight", 2)
	check("exactly 24", "lineExact", 24)
	check("at least 10", "lineHeight", 1)
	check("at least 10", "lineExact", nil)
	check("keep with next", "keepNext", true)
	check("before", "align", nil)
	if b := byText["a\nb\tc\nd"]; b == nil || types(b.Content) != "text,hard_break,text,tab,text,hard_break,text" {
		t.Errorf("breaks and tabs: %v", b)
	}
	if imp.LeftOut["borders_shading"] != 1 {
		t.Errorf("left out: %v", imp.LeftOut)
	}
}

// --- runs ---

func TestDocxRunFormatting(t *testing.T) {
	long := strings.Repeat("Ordinary words in the default font. ", 10)
	runs := []string{
		wr("", long),
		wr(`<w:b/>`, "boldtext"),
		wr(`<w:b w:val="0"/>`, "notbold"),
		wr(`<w:i/>`, "italictext"),
		wr(`<w:u w:val="single"/>`, "under1"),
		wr(`<w:u w:val="double"/>`, "under2"),
		wr(`<w:u w:val="none"/>`, "notunder"),
		wr(`<w:strike/>`, "struck1"),
		wr(`<w:dstrike/>`, "struck2"),
		wr(`<w:vertAlign w:val="superscript"/>`, "sup"),
		wr(`<w:vertAlign w:val="subscript"/>`, "sub"),
		wr(`<w:smallCaps/>`, "smallcapstext"),
		wr(`<w:caps/>`, "loud words"),
		wr(`<w:rFonts w:ascii="Georgia" w:hAnsi="Georgia"/>`, "georgia"),
		wr(`<w:rFonts w:ascii="Arial" w:asciiTheme="majorHAnsi" w:hAnsi="Arial" w:hAnsiTheme="majorHAnsi"/>`, "themed"),
		wr(`<w:rFonts w:ascii="Arial" w:hAnsi="Symbola"/>`, "★☆"),
		wr(`<w:sz w:val="28"/>`, "fourteen"),
		wr(`<w:color w:val="FF0000"/>`, "redtext"),
		wr(`<w:color w:val="auto"/>`, "autocolour"),
		wr(`<w:color w:val="000000" w:themeColor="text1"/>`, "themeblack"),
		wr(`<w:highlight w:val="yellow"/>`, "highlighted"),
		wr(`<w:shd w:val="clear" w:color="auto" w:fill="FFF2CC"/>`, "shaded"),
		wr(`<w:shd w:val="clear" w:color="auto" w:fill="FFFFFF"/>`, "whiteshade"),
		wr(`<w:vanish/>`, "secret"),
		wrx(`<w:rFonts w:ascii="Calibri"/>`, `<w:sym w:font="Wingdings" w:char="F04A"/>`),
		wrx("", `<w:t>non</w:t><w:noBreakHyphen/><w:t>breaking</w:t>`),
		wr("", "日本語 😀 é ❤️ «quoted» — ok"),
	}
	body := wp("", runs...) + wp("", wr(`<w:vanish/>`, "all hidden"))
	imp := importDocx(t, docxSpec{body: body, theme: true})
	marks := func(s string) map[string]map[string]any { return markMap(textNode(t, imp, s)) }
	has := func(s string, want ...string) {
		t.Helper()
		m := marks(s)
		if len(m) != len(want) {
			t.Errorf("%q marks %v, want %v", s, m, want)
			return
		}
		for _, w := range want {
			if _, ok := m[w]; !ok {
				t.Errorf("%q marks %v, want %v", s, m, want)
			}
		}
	}
	has("boldtext", "bold")
	has("notbold")
	has("italictext", "italic")
	has("under1", "underline")
	has("under2", "underline")
	has("notunder")
	has("struck1", "strike")
	has("struck2", "strike")
	has("sup", "superscript")
	has("sub", "subscript")
	has("smallcapstext", "smallcaps")
	has("LOUD WORDS")
	has("georgia", "font")
	has("themed", "font")
	has("fourteen", "size")
	has("redtext", "color")
	has("autocolour")
	has("themeblack")
	has("highlighted", "highlight")
	has("shaded", "highlight")
	has("whiteshade")
	if f := marks("themed")["font"]["family"]; f != "Calibri Light" {
		t.Errorf("theme font: %v", f)
	}
	if f := marks("★☆")["font"]["family"]; f != "Symbola" {
		t.Errorf("symbols take the hAnsi font: %v", f)
	}
	if v := marks("fourteen")["size"]["pt"]; v != 14.0 {
		t.Errorf("size: %v", v)
	}
	if c := marks("redtext")["color"]["color"]; c != "#ff0000" {
		t.Errorf("color: %v", c)
	}
	if c := marks("highlighted")["highlight"]["color"]; c != "#ffff00" {
		t.Errorf("highlight: %v", c)
	}
	if c := marks("shaded")["highlight"]["color"]; c != "#fff2cc" {
		t.Errorf("shading: %v", c)
	}
	if m := marks("\uf04a"); m["font"]["family"] != "Wingdings" {
		t.Errorf("symbol: %v", m)
	}
	textNode(t, imp, "non\u2011breaking")
	textNode(t, imp, "日本語 😀 é ❤️ «quoted» — ok")
	for _, ch := range imp.Chapters {
		if strings.Contains(docText(ch.Doc), "secret") || strings.Contains(docText(ch.Doc), "hidden") {
			t.Error("hidden text was imported")
		}
	}
	if n := len(allBlocks(imp)); n != 1 {
		t.Errorf("a paragraph of hidden text should go: %d blocks", n)
	}
	if imp.LeftOut["hidden_text"] != 2 {
		t.Errorf("left out: %v", imp.LeftOut)
	}
	if imp.Fonts[0] != "Times New Roman" {
		t.Errorf("fonts: %v", imp.Fonts)
	}
}

// --- links and bookmarks ---

func TestDocxLinksAndBookmarks(t *testing.T) {
	rels := `<Relationship Id="rIdWeb" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://example.org/a?b=1" TargetMode="External"/>` +
		`<Relationship Id="rIdBad" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="javascript:alert(1)" TargetMode="External"/>` +
		`<Relationship Id="rIdFile" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="file:///C:/notes.docx" TargetMode="External"/>`
	body := `<w:bookmarkStart w:id="0" w:name="Intro"/>` +
		wp("", wr("", "Start "),
			`<w:hyperlink r:id="rIdWeb">`+wr("", "web")+`</w:hyperlink>`, wr("", " "),
			`<w:hyperlink w:anchor="Intro">`+wr("", "inside")+`</w:hyperlink>`, wr("", " "),
			`<w:hyperlink r:id="rIdBad">`+wr("", "bad")+`</w:hyperlink>`, wr("", " "),
			`<w:hyperlink r:id="rIdFile">`+wr("", "file")+`</w:hyperlink>`, wr("", " "),
			`<w:hyperlink w:anchor="_Toc1">`+wr("", "tocjump")+`</w:hyperlink>`) +
		wp("", `<w:bookmarkStart w:id="1" w:name="_Toc1"/>`, wr("", "target"), `<w:bookmarkEnd w:id="1"/>`) +
		wp("", `<w:bookmarkStart w:id="2" w:name="_GoBack"/>`, wr("", "cursor was here"))
	imp := importDocx(t, docxSpec{body: body, rels: rels})
	href := func(s string) any { return markMap(textNode(t, imp, s))["link"]["href"] }
	if h := href("web"); h != "https://example.org/a?b=1" {
		t.Errorf("web link %v", h)
	}
	if h := href("inside"); h != "#Intro" {
		t.Errorf("anchor link %v", h)
	}
	if h := href("bad"); h != nil {
		t.Errorf("script link kept: %v", h)
	}
	if h := href("file"); h != nil {
		t.Errorf("file link kept: %v", h)
	}
	bs := allBlocks(imp)
	if bs[0].Attrs["bookmark"] != "Intro" || bs[1].Attrs["bookmark"] != "_Toc1" || bs[2].Attrs != nil {
		t.Errorf("bookmarks: %v %v %v", bs[0].Attrs, bs[1].Attrs, bs[2].Attrs)
	}
}

// --- lists ---

const testNumbering = `<w:abstractNum w:abstractNumId="0"><w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="•"/></w:lvl></w:abstractNum>` +
	`<w:abstractNum w:abstractNumId="1"><w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%1."/></w:lvl>` +
	`<w:lvl w:ilvl="1"><w:start w:val="1"/><w:numFmt w:val="lowerLetter"/><w:lvlText w:val="%1.%2)"/></w:lvl></w:abstractNum>` +
	`<w:abstractNum w:abstractNumId="2"><w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="upperRoman"/><w:lvlText w:val="Chapter %1"/><w:suff w:val="space"/></w:lvl></w:abstractNum>` +
	`<w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>` +
	`<w:num w:numId="2"><w:abstractNumId w:val="1"/></w:num>` +
	`<w:num w:numId="3"><w:abstractNumId w:val="2"/></w:num>` +
	`<w:num w:numId="4"><w:abstractNumId w:val="1"/><w:lvlOverride w:ilvl="0"><w:startOverride w:val="5"/></w:lvlOverride></w:num>`

func numPr(id, lvl int) string {
	return fmt.Sprintf(`<w:numPr><w:ilvl w:val="%d"/><w:numId w:val="%d"/></w:numPr><w:ind w:left="720" w:hanging="360"/>`, lvl, id)
}

func TestDocxLists(t *testing.T) {
	styles := `<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/></w:style>` +
		`<w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:basedOn w:val="Normal"/>` +
		`<w:pPr><w:numPr><w:numId w:val="3"/></w:numPr><w:outlineLvl w:val="0"/></w:pPr></w:style>`
	body := wp(numPr(1, 0), wr("", "apple")) + wp(numPr(1, 0), wr("", "pear")) +
		wp(numPr(2, 0), wr("", "first")) + wp(numPr(2, 1), wr("", "sub")) + wp(numPr(2, 0), wr("", "second")) +
		wp("", wr("", "an interruption")) +
		wp(numPr(2, 0), wr("", "third")) +
		wp(numPr(4, 0), wr("", "fifth")) +
		wp(`<w:pStyle w:val="Heading1"/>`, wr("", "The Start")) +
		wp(`<w:pStyle w:val="Heading1"/>`, wr("", "Next"))
	imp := importDocx(t, docxSpec{body: body, styles: styles, numbering: testNumbering})
	if len(imp.Chapters) != 3 {
		t.Fatalf("chapters: %d", len(imp.Chapters))
	}
	bs := imp.Chapters[0].Doc.Content
	if got := types(bs); got != "bullet_list,ordered_list,paragraph,ordered_list,ordered_list" {
		t.Fatalf("blocks: %s", got)
	}
	items := func(n *pmNode) string {
		var s []string
		for _, it := range n.Content {
			s = append(s, nodeText(it.Content[0]))
			if it.Content[0].Attrs["indentLeft"] != nil || it.Content[0].Attrs["indentFirst"] != nil {
				t.Errorf("list item keeps the list's indent: %v", it.Content[0].Attrs)
			}
		}
		return strings.Join(s, "|")
	}
	if items(bs[0]) != "apple|pear" || items(bs[1]) != "first|second" || items(bs[3]) != "third" || items(bs[4]) != "fifth" {
		t.Errorf("items: %s / %s / %s / %s", items(bs[0]), items(bs[1]), items(bs[3]), items(bs[4]))
	}
	// Word's second level is a list inside the first item.
	first := bs[1].Content[0]
	if len(first.Content) != 2 || first.Content[1].Type != "ordered_list" || items(first.Content[1]) != "sub" {
		t.Errorf("nested level: %s", types(first.Content))
	}
	start := func(n *pmNode) int64 { v, _ := pmInt(n.Attrs["start"]); return v }
	if start(bs[1]) != 1 || start(bs[3]) != 3 || start(bs[4]) != 5 {
		t.Errorf("starts: %d %d %d", start(bs[1]), start(bs[3]), start(bs[4]))
	}
	if imp.Chapters[1].Title != "Chapter I The Start" || imp.Chapters[2].Title != "Chapter II Next" {
		t.Errorf("numbered headings: %q %q", imp.Chapters[1].Title, imp.Chapters[2].Title)
	}
	if imp.Chapters[1].Doc.Content[0].Type != "heading" {
		t.Error("a numbered heading stays a heading")
	}
}

// --- pictures ---

func testPNG(t *testing.T) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

const graphicPic = `<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic>` +
	`<pic:blipFill><a:blip r:embed="%s"/></pic:blipFill></pic:pic></a:graphicData></a:graphic>`

func inlinePic(rid string, cx, cy int) string {
	return wrx("", fmt.Sprintf(`<w:drawing><wp:inline><wp:extent cx="%d" cy="%d"/><wp:docPr id="1" name="Picture" descr="A map"/>`+graphicPic+`</wp:inline></w:drawing>`, cx, cy, rid))
}

func anchorPic(rid, relH, posH, relV string, cx, cy int) string {
	return wrx("", fmt.Sprintf(`<w:drawing><wp:anchor><wp:simplePos x="0" y="0"/><wp:positionH relativeFrom="%s">%s</wp:positionH>`+
		`<wp:positionV relativeFrom="%s"><wp:posOffset>0</wp:posOffset></wp:positionV><wp:extent cx="%d" cy="%d"/>`+
		`<wp:docPr id="2" name="Picture"/>`+graphicPic+`</wp:anchor></w:drawing>`, relH, posH, relV, cx, cy, rid))
}

func TestDocxPictures(t *testing.T) {
	rels := `<Relationship Id="rIdP" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/image1.png"/>` +
		`<Relationship Id="rIdSame" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="/word/media/image1.png"/>` +
		`<Relationship Id="rIdEmf" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/image2.emf"/>` +
		`<Relationship Id="rIdGone" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/missing.png"/>`
	page := `<w:sectPr><w:pgSz w:w="5954" w:h="8392"/><w:pgMar w:top="680" w:right="680" w:bottom="680" w:left="680" w:gutter="284"/></w:sectPr>`
	chart := `<w:drawing><wp:inline><wp:extent cx="100" cy="100"/><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart r:id="rIdChart"/></a:graphicData></a:graphic></wp:inline></w:drawing>`
	vml := `<w:pict><v:shape style="width:144pt;height:72pt"><v:imagedata r:id="rIdP"/></v:shape></w:pict>`
	body := wp(`<w:jc w:val="center"/>`, inlinePic("rIdP", 914400, 914400)) +
		wp("", wr("", "text before"), inlinePic("rIdP", 12700, 12700), wr("", "text after")) +
		wp("", anchorPic("rIdP", "page", `<wp:posOffset>0</wp:posOffset>`, "page", 5954*635, 8392*635)) +
		wp("", wr("", "with ornament"), anchorPic("rIdSame", "column", `<wp:align>center</wp:align>`, "paragraph", 1447138, 1447138)) +
		wp("", anchorPic("rIdP", "column", `<wp:posOffset>3725774</wp:posOffset>`, "paragraph", 1447138, 1447138)) +
		wp("", anchorPic("rIdP", "column", `<wp:posOffset>2000000</wp:posOffset>`, "paragraph", 600000, 600000)) +
		wp("", inlinePic("rIdEmf", 100, 100)) +
		wp("", inlinePic("rIdGone", 100, 100)) +
		wp("", wrx("", vml)) +
		wp("", wrx("", `<mc:AlternateContent><mc:Choice Requires="wps">`+fmt.Sprintf(`<w:drawing><wp:inline><wp:extent cx="254000" cy="254000"/>`+graphicPic+`</wp:inline></w:drawing>`, "rIdP")+
			`</mc:Choice><mc:Fallback>`+vml+`</mc:Fallback></mc:AlternateContent>`)) +
		wp("", wrx("", `<mc:AlternateContent><mc:Choice Requires="cx">`+chart+`</mc:Choice><mc:Fallback>`+vml+`</mc:Fallback></mc:AlternateContent>`)) +
		wp("", wrx("", chart)) +
		page
	spec := docxSpec{body: body, rels: rels, files: map[string][]byte{
		"word/media/image1.png": testPNG(t),
		"word/media/image2.emf": append([]byte{1, 0, 0, 0, 0x6c, 0, 0, 0}, bytes.Repeat([]byte(" EMF"), 20)...),
	}}
	imp, err := parseDocx(buildDocx(t, spec))
	if err != nil {
		t.Fatal(err)
	}
	if len(imp.Pictures) != 1 || imp.Pictures[0].Name != "image1.png" || imp.Pictures[0].ContentType != "image/png" {
		t.Fatalf("pictures: %+v", imp.Pictures)
	}
	bs := allBlocks(imp)
	// The page-sized picture goes to the start of its page (docxlayout.go):
	// first, as there is no page break before it.
	if got := types(bs); got != "image,image,paragraph,image,paragraph,paragraph,image,image,image,image,image" {
		t.Fatalf("blocks: %s", got)
	}
	img := func(i int) map[string]any { return bs[i].Attrs }
	if a := img(1); a["src"] != "docx-picture:0" || a["align"] != "center" || a["width"] != 72.0 || a["fullPage"] != false || a["alt"] != "A map" {
		t.Errorf("inline: %v", a)
	}
	if nodeText(bs[2]) != "text before" || nodeText(bs[4]) != "text after" || img(3)["width"] != 1.0 {
		t.Errorf("split paragraph: %q %q %v", nodeText(bs[2]), nodeText(bs[4]), img(3))
	}
	if a := img(0); a["fullPage"] != true || a["width"] != nil {
		t.Errorf("cover: %v", a)
	}
	if a := img(6); a["align"] != "center" || a["fullPage"] != false {
		t.Errorf("ornament: %v", a)
	}
	if a := img(7); a["align"] != "right" {
		t.Errorf("anchored at an offset: %v", a)
	}
	if a := img(8); a["width"] != 144.0 {
		t.Errorf("vml: %v", a)
	}
	if a := img(9); a["width"] != 20.0 {
		t.Errorf("alternate content should take the Choice: %v", a)
	}
	if a := img(10); a["width"] != 144.0 {
		t.Errorf("alternate content should take the Fallback: %v", a)
	}
	// Off the page, EMF, missing: left out. The chart too.
	if imp.LeftOut["pictures"] != 3 || imp.LeftOut["drawings"] != 1 {
		t.Errorf("left out: %v", imp.LeftOut)
	}
	if err := validatePMDoc(imp.Chapters[0].Doc); err == nil {
		t.Error("placeholders aren't valid until replaced")
	}
	imp.usePictureURLs([]string{"/uploads/chapters/x.png"})
	if err := validatePMDoc(imp.Chapters[0].Doc); err != nil || img(0)["src"] != "/uploads/chapters/x.png" {
		t.Errorf("after usePictureURLs: %v %v", err, img(0))
	}
	imp, _ = parseDocx(buildDocx(t, spec))
	imp.usePictureURLs([]string{""})
	if got := types(imp.Chapters[0].Doc.Content); got != "paragraph,paragraph,paragraph" {
		t.Errorf("dropped pictures: %s", got)
	}

	// A picture over the limit is left out, not an error.
	defer func(v int64) { maxDocxPicture = v }(maxDocxPicture)
	maxDocxPicture = 10
	imp, err = parseDocx(buildDocx(t, spec))
	if err != nil || len(imp.Pictures) != 0 || imp.LeftOut["pictures"] < 10 {
		t.Errorf("big picture: %v %d %v", err, len(imp.Pictures), imp.LeftOut)
	}
}

// --- scene breaks ---

func TestDocxSceneBreaks(t *testing.T) {
	var body string
	for _, s := range []string{"***", "*   *  *", "  ⁂  ", "#", "◇◇◇", "☆ ☆ ☆", "~", "— —", "●", "☾", "...", "* not one", "“…”"} {
		body += wp(`<w:jc w:val="center"/>`, wr("", s))
	}
	body += wp(`<w:outlineLvl w:val="1"/>`, wr("", "***"))
	imp := importDocx(t, docxSpec{body: body})
	var glyphs []string
	for _, b := range allBlocks(imp) {
		if b.Type == "scene_break" {
			glyphs = append(glyphs, b.Attrs["glyph"].(string))
		}
	}
	if got := strings.Join(glyphs, "|"); got != "***|* * *|⁂|#|◇◇◇|☆ ☆ ☆|~|— —" {
		t.Errorf("scene breaks: %s", got)
	}
}

// --- fields, tables of contents, tracked changes, other content ---

func TestDocxFieldsAndTOC(t *testing.T) {
	tocLines := wp("", fld(`TOC \o "1-3" \h \z \u`, `<w:hyperlink w:anchor="_Toc1">`+wr("", "Chapter One")+fld("PAGEREF _Toc1 \\h", wr("", "3"))+`</w:hyperlink>`)[:0],
		wrx("", `<w:fldChar w:fldCharType="begin"/>`), wrx("", `<w:instrText> TOC \o "1-3" \h </w:instrText>`), wrx("", `<w:fldChar w:fldCharType="separate"/>`),
		`<w:hyperlink w:anchor="_Toc1">`+wr("", "Chapter One")+fld(`PAGEREF _Toc1 \h`, wr("", "3"))+`</w:hyperlink>`) +
		wp("", `<w:hyperlink w:anchor="_Toc2">`+wr("", "Chapter Two")+fld(`PAGEREF _Toc2 \h`, wr("", "9"))+`</w:hyperlink>`) +
		wp("", wrx("", `<w:fldChar w:fldCharType="end"/>`))
	sdtTOC := `<w:sdt><w:sdtPr><w:docPartObj><w:docPartGallery w:val="Table of Contents"/><w:docPartUnique/></w:docPartObj></w:sdtPr><w:sdtContent>` +
		wp("", wr("", "Contents")) + tocLines + `</w:sdtContent></w:sdt>`
	body := wp("", wr("", "Title page")) +
		wp("", wrx("", `<w:br w:type="page"/>`)) +
		sdtTOC +
		wp("", wrx("", `<w:br w:type="page"/>`)) +
		wp("", wr("", "Intro")) +
		tocLines +
		wp("", wr("", "Page "), fld("PAGE", wr("", "7")), wr("", "of the book")) +
		wp("", wr("", "See "), fld(`REF _Ref123 \h`, wr("", "Chapter 2")), wr("", ".")) +
		wp("", wr("", "Now "), `<w:fldSimple w:instr=" PAGE "><w:r><w:t>4</w:t></w:r></w:fldSimple>`, wr("", "here")) +
		wp("", fld(`HYPERLINK "https://example.org/x"`, wr("", "site"))) +
		wp("", `<w:fldSimple w:instr="HYPERLINK \l &quot;Intro&quot;">`+wr("", "back")+`</w:fldSimple>`) +
		wp("", `<w:fldSimple w:instr=" DATE \@ &quot;d MMMM yyyy&quot; "><w:r><w:t>1 May 2026</w:t></w:r></w:fldSimple>`)
	imp := importDocx(t, docxSpec{body: body})
	var texts []string
	for _, b := range allBlocks(imp) {
		texts = append(texts, b.Type+":"+nodeText(b))
	}
	want := "paragraph:Title page|page_break:|paragraph:Intro|paragraph:Page of the book|paragraph:See Chapter 2.|paragraph:Now here|paragraph:site|paragraph:back|paragraph:1 May 2026"
	if got := strings.Join(texts, "|"); got != want {
		t.Errorf("blocks:\n%s\nwant\n%s", got, want)
	}
	if h := markMap(textNode(t, imp, "site"))["link"]["href"]; h != "https://example.org/x" {
		t.Errorf("hyperlink field: %v", h)
	}
	if h := markMap(textNode(t, imp, "back"))["link"]["href"]; h != "#Intro" {
		t.Errorf("anchor field: %v", h)
	}
	if imp.LeftOut["toc"] != 2 || imp.LeftOut["fields"] != 2 || imp.LeftOut["fields_as_text"] != 2 {
		t.Errorf("left out: %v", imp.LeftOut)
	}
}

func TestDocxOtherContent(t *testing.T) {
	textBox := `<mc:AlternateContent><mc:Choice Requires="wps"><w:drawing><wp:anchor><wp:positionH relativeFrom="column"><wp:posOffset>0</wp:posOffset></wp:positionH>` +
		`<wp:positionV relativeFrom="paragraph"><wp:posOffset>0</wp:posOffset></wp:positionV><wp:extent cx="100" cy="100"/><wp:docPr id="9" name="Text Box"/>` +
		`<a:graphic><a:graphicData uri="http://schemas.microsoft.com/office/word/2010/wordprocessingShape"><wps:wsp><wps:txbx><w:txbxContent>` +
		wp("", wr("", "boxed words")) + `</w:txbxContent></wps:txbx></wps:wsp></a:graphicData></a:graphic></wp:anchor></w:drawing></mc:Choice>` +
		`<mc:Fallback><w:pict><v:shape><v:textbox><w:txbxContent>` + wp("", wr("", "boxed words")) + `</w:txbxContent></v:textbox></v:shape></w:pict></mc:Fallback></mc:AlternateContent>`
	body := wp("", wr("", "keep "), `<w:ins w:id="1" w:author="A">`+wr("", "inserted")+`</w:ins>`,
		`<w:del w:id="2" w:author="A"><w:r><w:delText>deleted</w:delText></w:r></w:del>`,
		`<w:moveFrom w:id="3" w:author="A">`+wr("", "movedaway")+`</w:moveFrom>`, `<w:proofErr w:type="spellStart"/>`, wr("", " end")) +
		`<w:tbl><w:tblPr/><w:tr><w:tc><w:tcPr/>` + wp("", wr("", "cell one")) + `</w:tc><w:tc>` + wp("", wr("", "cell two")) + `</w:tc></w:tr></w:tbl>` +
		wp("", wr("", "anchor"), wrx("", textBox)) +
		wp("", wr("", "noted"), wrx("", `<w:footnoteReference w:id="1"/>`), wrx("", `<w:endnoteReference w:id="2"/>`), wrx("", `<w:commentReference w:id="0"/>`),
			`<m:oMath><m:r><m:t>x</m:t></m:r></m:oMath>`) +
		wp("", `<w:smartTag w:uri="u" w:element="place">`+wr("", "Lisbon")+`</w:smartTag>`, `<w:sdt><w:sdtContent>`+wr("", " control")+`</w:sdtContent></w:sdt>`,
			`<w:customXml w:element="x">`+wr("", " custom")+`</w:customXml>`) +
		`<w:ins w:id="5" w:author="A">` + wp("", wr("", "inserted paragraph")) + `</w:ins>`
	imp := importDocx(t, docxSpec{body: body})
	var texts []string
	for _, b := range allBlocks(imp) {
		texts = append(texts, nodeText(b))
	}
	want := "keep inserted end|cell one|cell two|anchor|boxed words|noted|Lisbon control custom|inserted paragraph"
	if got := strings.Join(texts, "|"); got != want {
		t.Errorf("blocks:\n%s\nwant\n%s", got, want)
	}
	for k, v := range map[string]int{"tracked_changes": 4, "tables": 1, "text_boxes": 1, "footnotes": 1, "endnotes": 1, "comments": 1, "equations": 1} {
		if imp.LeftOut[k] != v {
			t.Errorf("left out %s = %d, want %d (%v)", k, imp.LeftOut[k], v, imp.LeftOut)
		}
	}
}

// --- page setup and sections ---

func TestDocxPageSetupAndSections(t *testing.T) {
	rels := `<Relationship Id="rIdH1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/>` +
		`<Relationship Id="rIdH2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header2.xml"/>` +
		`<Relationship Id="rIdF1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer" Target="footer1.xml"/>`
	a5 := `<w:pgSz w:w="8391" w:h="11906" w:code="11"/><w:pgMar w:top="1134" w:right="851" w:bottom="1134" w:left="1134" w:header="567" w:footer="567" w:gutter="284"/>`
	a6 := `<w:pgSz w:w="5954" w:h="8392"/><w:pgMar w:top="680" w:right="567" w:bottom="-680" w:left="680" w:gutter="284"/>`
	body := wp(`<w:sectPr><w:headerReference w:type="default" r:id="rIdH1"/><w:footerReference w:type="default" r:id="rIdF1"/>`+a5+`</w:sectPr>`, wr("", "Short first section.")) +
		wp(`<w:sectPr><w:type w:val="continuous"/>`+a5+`<w:cols w:num="2"/></w:sectPr>`) +
		wp("", wr("", strings.Repeat("Most of the words are in the last section. ", 5))) +
		`<w:sectPr><w:headerReference w:type="default" r:id="rIdH1"/><w:headerReference w:type="even" r:id="rIdH2"/>` + a6 + `</w:sectPr>`
	imp := importDocx(t, docxSpec{body: body, rels: rels, settings: `<w:mirrorMargins/><w:evenAndOddHeaders/>`})
	// S1 -> S2 is continuous; S2 -> S3 a new page; S2's own empty paragraph goes.
	if got := types(allBlocks(imp)); got != "paragraph,page_break,paragraph" {
		t.Errorf("blocks: %s", got)
	}
	want := bookPage{WidthMM: 105, HeightMM: 148, MarginTopMM: 12, MarginBottomMM: 12, MarginInnerMM: 12, MarginOuterMM: 10, GutterMM: 5, Mirror: true}
	if imp.Page != want || imp.PageTypeGuess != "a6" {
		t.Errorf("page %+v %s", imp.Page, imp.PageTypeGuess)
	}
	for k, v := range map[string]int{"headers_footers": 3, "sections": 2, "columns": 1} {
		if imp.LeftOut[k] != v {
			t.Errorf("left out %s = %d, want %d (%v)", k, imp.LeftOut[k], v, imp.LeftOut)
		}
	}

	// The first section holding the most words: A5, not mirrored.
	body = wp(`<w:sectPr>`+a5+`</w:sectPr>`, wr("", strings.Repeat("words ", 50))) + wp("", wr("", "few")) + `<w:sectPr>` + a6 + `</w:sectPr>`
	imp = importDocx(t, docxSpec{body: body})
	want = bookPage{WidthMM: 148, HeightMM: 210, MarginTopMM: 20, MarginBottomMM: 20, MarginInnerMM: 20, MarginOuterMM: 15, GutterMM: 5}
	if imp.Page != want || imp.PageTypeGuess != "a5" {
		t.Errorf("page %+v %s", imp.Page, imp.PageTypeGuess)
	}

	// No section properties: Word's Letter page.
	imp = importDocx(t, docxSpec{body: wp("", wr("", "x"))})
	if imp.Page.WidthMM != 215.9 || imp.Page.HeightMM != 279.4 || imp.Page.MarginInnerMM != 25.4 || imp.PageTypeGuess != "custom" {
		t.Errorf("letter: %+v %s", imp.Page, imp.PageTypeGuess)
	}
	if g := guessPageType(bookPage{WidthMM: 210.2, HeightMM: 296.9}); g != "a4" {
		t.Errorf("a4 guess: %s", g)
	}
	if g := guessPageType(bookPage{WidthMM: 152.4, HeightMM: 228.6}); g != "custom" {
		t.Errorf("6x9 guess: %s", g)
	}
}

// --- malformed files ---

func zipOf(t *testing.T, files map[string]string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, _ := zw.Create(name)
		w.Write([]byte(content))
	}
	zw.Close()
	return buf.Bytes()
}

func TestDocxMalformed(t *testing.T) {
	if _, err := parseDocx([]byte("PK\x03\x04 not really")); !errors.Is(err, errDocxNotZip) {
		t.Errorf("not a zip: %v", err)
	}
	if _, err := parseDocx([]byte{0xD0, 0xCF, 0x11, 0xE0}); !errors.Is(err, errDocxNotZip) {
		t.Errorf("old .doc: %v", err)
	}
	if _, err := parseDocx(zipOf(t, map[string]string{"word/styles.xml": "<x/>"})); !errors.Is(err, errDocxNoDocument) {
		t.Errorf("no document: %v", err)
	}
	if _, err := parseDocx(zipOf(t, map[string]string{"word/document.xml": `<html><body/></html>`})); !errors.Is(err, errDocxNoDocument) {
		t.Errorf("not a Word document: %v", err)
	}
	if _, err := parseDocx(zipOf(t, map[string]string{"word/document.xml": `<w:document ` + docxNS + `><w:body><w:p>`})); err == nil {
		t.Error("broken XML accepted")
	}
	// The usual place is found without package relationships.
	imp, err := parseDocx(zipOf(t, map[string]string{"word/document.xml": `<w:document ` + docxNS + `><w:body>` + wp("", wr("", "bare")) + `</w:body></w:document>`}))
	if err != nil || len(imp.Chapters) != 1 || imp.Chapters[0].Words != 1 {
		t.Errorf("bare document: %v", err)
	}

	ok := buildDocx(t, docxSpec{body: wp("", wr("", strings.Repeat("text ", 400)))})
	restore := func(e int, total, x int64) func() {
		return func() { maxDocxEntries, maxDocxTotal, maxDocxXML = e, total, x }
	}(maxDocxEntries, maxDocxTotal, maxDocxXML)
	defer restore()

	maxDocxEntries = 3
	if _, err := parseDocx(ok); !errors.Is(err, errDocxTooBig) {
		t.Errorf("too many entries: %v", err)
	}
	restore()
	maxDocxTotal = 1000
	if _, err := parseDocx(ok); !errors.Is(err, errDocxTooBig) {
		t.Errorf("too much in all: %v", err)
	}
	restore()
	maxDocxXML = 500
	if _, err := parseDocx(ok); !errors.Is(err, errDocxTooBig) {
		t.Errorf("XML part too big: %v", err)
	}
	restore()

	// A zip bomb lying about its size: the read stops at the limit.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.CreateHeader(&zip.FileHeader{Name: "word/document.xml", Method: zip.Deflate})
	w.Write([]byte(`<w:document ` + docxNS + `><w:body>`))
	w.Write(bytes.Repeat([]byte("<w:p/>"), 2_000_000))
	w.Write([]byte(`</w:body></w:document>`))
	zw.Close()
	bomb := buf.Bytes()
	// Rewrite the declared size in the central directory to a small one.
	if i := bytes.LastIndex(bomb, []byte("PK\x01\x02")); i > 0 {
		copy(bomb[i+24:i+28], []byte{100, 0, 0, 0})
	}
	maxDocxXML = 1 << 20
	if _, err := parseDocx(bomb); err == nil {
		t.Error("zip bomb accepted")
	}
}
