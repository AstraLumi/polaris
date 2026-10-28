package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// A synthetic book built from pmNode values: every node and mark
// EDITOR.md lists, for the exports and the .docx round trip.

func pt(text string, marks ...pmMark) *pmNode { return &pmNode{Type: "text", Text: text, Marks: marks} }
func mk(typ string, kv ...any) pmMark {
	m := pmMark{Type: typ}
	if len(kv) > 0 {
		m.Attrs = map[string]any{}
		for i := 0; i+1 < len(kv); i += 2 {
			m.Attrs[kv[i].(string)] = kv[i+1]
		}
	}
	return m
}
func blk(typ string, attrs map[string]any, content ...*pmNode) *pmNode {
	return &pmNode{Type: typ, Attrs: attrs, Content: content}
}
func kv(pairs ...any) map[string]any {
	m := map[string]any{}
	for i := 0; i+1 < len(pairs); i += 2 {
		m[pairs[i].(string)] = pairs[i+1]
	}
	return m
}
func para(content ...*pmNode) *pmNode { return blk("paragraph", nil, content...) }
func item(content ...*pmNode) *pmNode { return blk("list_item", nil, content...) }

func writePNG(t *testing.T, path string, w, h int, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{c.R, c.G, uint8(x % 256), 255})
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

const testBookSettings = `{
  "page_type": "custom",
  "page": {"width_mm": 127, "height_mm": 203.2, "margin_top_mm": 15, "margin_bottom_mm": 16,
           "margin_inner_mm": 18, "margin_outer_mm": 14, "gutter_mm": 4, "mirror": true},
  "styles": [
    {"id":"normal","name":"Normal","kind":"paragraph","based_on":null,"builtin":true,
     "para":{"align":"justify","indentFirst":14.2,"indentLeft":0,"indentRight":0,"spaceBefore":0,"spaceAfter":2,"lineHeight":1.15},
     "text":{"font":"Merriweather","size":10.5,"color":null}},
    {"id":"heading1","name":"Heading 1","kind":"paragraph","based_on":"normal","builtin":true,
     "para":{"align":"center","indentFirst":0,"spaceBefore":30,"spaceAfter":15,"keepNext":true},"text":{"size":17,"bold":true}},
    {"id":"heading2","name":"Heading 2","kind":"paragraph","based_on":"normal","builtin":true,
     "para":{"align":"center","indentFirst":0,"spaceBefore":15,"spaceAfter":8,"keepNext":true},"text":{"size":13.5,"bold":true}},
    {"id":"letter","name":"Letter","kind":"paragraph","based_on":"normal",
     "para":{"indentLeft":20,"indentFirst":0,"lineExact":13},"text":{"font":"Courier New","italic":true}},
    {"id":"word-shout","name":"Shout","kind":"character","based_on":null,"text":{"bold":true,"color":"#aa0000"}},
    {"id":"whisper","name":"Whisper","kind":"character","based_on":null,"text":{"smallcaps":true,"size":9}}
  ],
  "meta": {"title":"The Test Book","subtitle":"A Subtitle","author":"Ana Autora","language":"pt-BR",
           "publisher":"Small Press","isbn":"978-3-16-148410-0","rights":"All rights reserved","description":"A book & a test."},
  "layout": {"different_first": true, "different_odd_even": true,
             "header": {"odd": {"left":"","center":"{chapter}","right":"","italic":true},
                        "even": {"left":"{book}","center":"","right":"{author}","smallcaps":true},
                        "first": {"left":"","center":"","right":""}},
             "footer": {"odd": {"center":"{page}"}, "even": {"center":"— {page} —"}, "first": {"center":"{page}"}},
             "chapter_start": "odd", "numbering": {"format": "arabic", "start": 1}, "toc_dots": true},
  "chapter_options": {"1": {"front_matter": true}, "3": {"number_restart": 5}}
}`

// testBook: four chapters; pictures in a temp uploads folder.
func testBook(t *testing.T) (*exportBook, string) {
	t.Helper()
	up := t.TempDir()
	writePNG(t, filepath.Join(up, "editor", "cover.png"), 400, 640, color.RGBA{200, 10, 10, 255})
	writePNG(t, filepath.Join(up, "editor", "pic.png"), 200, 100, color.RGBA{10, 200, 10, 255})
	writePNG(t, filepath.Join(up, "editor", "orn.png"), 60, 20, color.RGBA{10, 10, 200, 255})

	front := &pmNode{Type: "doc", Content: []*pmNode{
		blk("image", kv("src", "/uploads/editor/cover.png", "alt", "Cover", "fullPage", true, "align", "center")),
		blk("paragraph", kv("style", "title"), pt("The Test Book")),
		blk("paragraph", kv("style", "subtitle"), pt("A Subtitle")),
		blk("page_break", nil),
		blk("toc", kv("levels", 2, "title", "Contents")),
	}}
	one := &pmNode{Type: "doc", Content: []*pmNode{
		blk("heading", kv("level", 1, "style", "heading1", "bookmark", "ch1"), pt("Chapter One")),
		para(pt("Plain, "), pt("bold", mk("bold")), pt(" "), pt("italic", mk("italic")), pt(" "),
			pt("under", mk("underline")), pt(" "), pt("struck", mk("strike")), pt(" x"), pt("2", mk("superscript")),
			pt(" H"), pt("2", mk("subscript")), pt("O "), pt("Small Caps", mk("smallcaps")), pt(" "),
			pt("Arial", mk("font", "family", "Arial")), pt(" "), pt("big", mk("size", "pt", 14.5)), pt(" "),
			pt("red", mk("color", "color", "#cc0000")), pt(" "), pt("yellow", mk("highlight", "color", "#ffff00")), pt(" "),
			pt("peach", mk("highlight", "color", "#ffeeaa")), pt(" "),
			pt("web", mk("link", "href", "https://example.com/a b?x=1&y=ç")), pt(" "),
			pt("back", mk("link", "href", "#ch1")), pt(" "),
			pt("Mira", mk("story_link", "type", "character", "id", 3)),
			blk("hard_break", nil), pt("after break"), blk("tab", nil), pt("after tab")),
		blk("paragraph", kv("align", "center", "indentFirst", -10, "indentLeft", 30, "indentRight", 5,
			"spaceBefore", 6, "spaceAfter", 3, "lineHeight", 1.5, "keepNext", true), pt("A formatted paragraph.")),
		blk("paragraph", kv("lineHeight", 1.03, "lineExact", 13), pt("Exactly spaced.")),
		blk("heading", kv("level", 2, "style", "heading2"), pt("A Section")),
		blk("scene_break", kv("glyph", "* * *")),
		blk("scene_break", kv("glyph", "~ ❦ ~")),
		para(pt("Shouted", mk("char_style", "id", "word-shout")), pt(" and "),
			pt("whispered italics", mk("char_style", "id", "whisper"), mk("italic")), pt(".")),
		blk("paragraph", kv("style", "letter"), pt("Dear reader, a letter.")),
		blk("paragraph", kv("style", "quote"), pt("A quoted line.")),
		blk("image", kv("src", "/uploads/editor/pic.png", "alt", "A picture", "width", 100.0, "align", "right")),
		blk("image", kv("src", "/uploads/editor/pic.png", "alt", "", "width", nil, "align", "center")),
		blk("scene_break", kv("glyph", "", "image", "/uploads/editor/orn.png")),
		blk("dialogue", kv("character", 3, "speaker", "Mira", "manner", "softly"), pt("Come "), pt("here", mk("italic")), pt(".")),
		para(),
		para(pt("The end of one.")),
	}}
	two := &pmNode{Type: "doc", Content: []*pmNode{
		blk("heading", kv("level", 1), pt("Chapter Two")),
		blk("bullet_list", nil,
			item(para(pt("first bullet"))),
			item(para(pt("second bullet")),
				blk("ordered_list", kv("start", 1), item(para(pt("nested one"))), item(para(pt("nested two"))))),
			item(para(pt("third bullet")), para(pt("its second paragraph")))),
		blk("ordered_list", kv("start", 3), item(para(pt("three"))), item(para(pt("four")))),
		para(pt("Before the break.")),
		blk("page_break", nil),
		para(pt("After the break, a co\u00adop\u2011era\u00adtive word.")),
		blk("image", kv("src", "/uploads/editor/cover.png", "alt", "", "fullPage", true)),
		para(pt("After the full page.")),
		blk("paragraph", kv("style", "heading2"), pt("A paragraph in a heading style.")),
		blk("heading", kv("level", 3, "style", "letter"), pt("A heading in the letter style")),
	}}
	three := &pmNode{Type: "doc", Content: []*pmNode{
		blk("heading", kv("level", 1), pt("Chapter Three: "), pt("Restarted", mk("italic"))),
		para(pt("Short.")),
	}}
	b := &exportBook{Volume: 1, VolumeTitle: "Book One", uploadsDir: up, Chapters: []exportChapter{
		{ID: 1, Title: "Front matter", Doc: front},
		{ID: 2, Title: "Chapter One", Doc: one},
		{ID: 3, Title: "Chapter Two", Doc: two},
		{ID: 4, Title: "Chapter Three: Restarted", Doc: three},
	}}
	parseExportSettings(json.RawMessage(testBookSettings), b)
	return b, up
}

// --- linear form for comparing docs ---

type linRun struct {
	text string
	f    exText
	link string
}

type linBlock struct {
	kind    string // p, h1…h6, li-b, li-o, scene, image, full, pb
	style   string
	text    string
	p       exPara
	skipFmt bool
	runs    []linRun
	src     string
	width   float64
	align   string
}

func (l linBlock) String() string {
	return fmt.Sprintf("%s[%s] %q", l.kind, l.style, l.text)
}

// normalizeForDocx turns the original into what a .docx can give back:
// no toc, dialogue as a paragraph, an ornament as a picture.
func normalizeForDocx(n *pmNode) *pmNode {
	c := *n
	c.Content = nil
	for _, k := range n.Content {
		if k == nil {
			continue
		}
		switch k.Type {
		case "toc":
			continue
		case "dialogue":
			speaker, lead := dialogueParts(k)
			p := &pmNode{Type: "paragraph", Attrs: map[string]any{}}
			for key, v := range k.Attrs {
				if key != "character" && key != "speaker" && key != "manner" {
					p.Attrs[key] = v
				}
			}
			p.Content = append(p.Content, pt(speaker, mk("smallcaps")), pt(lead))
			p.Content = append(p.Content, k.Content...)
			p.Content = append(p.Content, pt("”"))
			c.Content = append(c.Content, p)
			continue
		case "scene_break":
			if src := attrString(k, "image"); src != "" {
				c.Content = append(c.Content, blk("image", kv("src", src, "align", "center")))
				continue
			}
		}
		c.Content = append(c.Content, normalizeForDocx(k))
	}
	return &c
}

func linearize(b *exportBook, doc *pmNode) []linBlock {
	var out []linBlock
	var walk func(n *pmNode, listKind string, first bool)
	walk = func(n *pmNode, listKind string, first bool) {
		switch n.Type {
		case "paragraph", "heading":
			style := b.paraStyleOf(n)
			l := linBlock{kind: "p", style: style, p: b.resolvedPara(n)}
			if n.Type == "heading" {
				l.kind = fmt.Sprintf("h%d", headingLevel(n))
			}
			if listKind != "" {
				if first {
					l.kind = listKind
				}
				l.skipFmt = true
			}
			for _, c := range n.Content {
				switch c.Type {
				case "text":
					// A link comes back percent-encoded (the same address).
					r := linRun{text: c.Text, f: b.resolvedText(style, c.Marks), link: uriEscape(linkHrefOf(c.Marks))}
					r.f.Size = math.Round(r.f.Size*2) / 2
					if k := len(l.runs); k > 0 && l.runs[k-1].f == r.f && l.runs[k-1].link == r.link {
						l.runs[k-1].text += r.text
					} else {
						l.runs = append(l.runs, r)
					}
				case "hard_break":
					l.runs = append(l.runs, linRun{text: "\n"})
				case "tab":
					l.runs = append(l.runs, linRun{text: "\t"})
				}
			}
			l.text = pmTextblockText(n, "\n", "\t")
			out = append(out, l)
		case "scene_break":
			out = append(out, linBlock{kind: "scene", text: sceneGlyphOf(n)})
		case "page_break":
			out = append(out, linBlock{kind: "pb"})
		case "image":
			l := linBlock{kind: "image", src: attrString(n, "src"), align: attrString(n, "align")}
			if attrBool(n, "fullPage") {
				l.kind, l.align = "full", ""
			}
			l.width, _ = pmNum(n.Attrs["width"])
			out = append(out, l)
		case "bullet_list", "ordered_list":
			kind := "li-b"
			if n.Type == "ordered_list" {
				kind = "li-o"
			}
			for _, it := range n.Content {
				for i, c := range it.Content {
					if c.Type == "bullet_list" || c.Type == "ordered_list" {
						walk(c, "", false)
					} else {
						walk(c, kind, i == 0)
					}
				}
			}
		default:
			for _, c := range n.Content {
				walk(c, listKind, first)
			}
		}
	}
	walk(doc, "", false)
	// Page breaks next to a full-page picture say nothing; nor do a
	// chapter's first and last.
	var clean []linBlock
	for i, l := range out {
		if l.kind == "pb" && (i > 0 && out[i-1].kind == "full" || i+1 < len(out) && out[i+1].kind == "full") {
			continue
		}
		clean = append(clean, l)
	}
	for len(clean) > 0 && clean[len(clean)-1].kind == "pb" {
		clean = clean[:len(clean)-1]
	}
	for len(clean) > 0 && clean[0].kind == "pb" {
		clean = clean[1:]
	}
	return clean
}

// expectedStyleID: what a style id comes back as from Word.
func expectedStyleID(id string) string {
	if _, ok := wordBuiltinIDs[id]; ok || strings.HasPrefix(id, "word-") {
		return id
	}
	return "word-" + styleSlug(id)
}

func nearPt(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func compareLinear(t *testing.T, chapter string, want, got []linBlock) {
	t.Helper()
	if len(want) != len(got) {
		t.Errorf("%s: %d blocks, want %d\n got: %v\nwant: %v", chapter, len(got), len(want), got, want)
		return
	}
	for i := range want {
		w, g := want[i], got[i]
		where := fmt.Sprintf("%s block %d (%s)", chapter, i, w)
		if w.kind != g.kind {
			t.Errorf("%s: kind %s, want %s", where, g.kind, w.kind)
			continue
		}
		if w.text != g.text {
			t.Errorf("%s: text %q, want %q", where, g.text, w.text)
		}
		switch w.kind {
		case "image", "full":
			if w.src != g.src {
				t.Errorf("%s: src %q, want %q", where, g.src, w.src)
			}
			if w.align != "" && w.align != g.align {
				t.Errorf("%s: align %q, want %q", where, g.align, w.align)
			}
			if w.width > 0 && !nearPt(w.width, g.width, 0.5) {
				t.Errorf("%s: width %v, want %v", where, g.width, w.width)
			}
			continue
		case "scene", "pb":
			continue
		}
		if g.style != expectedStyleID(w.style) {
			t.Errorf("%s: style %q, want %q", where, g.style, expectedStyleID(w.style))
		}
		if !w.skipFmt && !g.skipFmt {
			wp, gp := w.p, g.p
			ok := wp.Align == gp.Align && wp.KeepNext == gp.KeepNext
			for _, x := range [][2]float64{{wp.IndentFirst, gp.IndentFirst}, {wp.IndentLeft, gp.IndentLeft}, {wp.IndentRight, gp.IndentRight},
				{wp.SpaceBefore, gp.SpaceBefore}, {wp.SpaceAfter, gp.SpaceAfter}} {
				ok = ok && nearPt(x[0], x[1], 0.051)
			}
			if wp.LineExact > 0 || gp.LineExact > 0 {
				ok = ok && nearPt(wp.LineExact, gp.LineExact, 0.051)
			} else {
				ok = ok && nearPt(wp.LineHeight, gp.LineHeight, 0.005)
			}
			if !ok {
				t.Errorf("%s: paragraph %+v, want %+v", where, gp, wp)
			}
		}
		if len(w.runs) != len(g.runs) {
			t.Errorf("%s: runs\n got %+v\nwant %+v", where, g.runs, w.runs)
			continue
		}
		for j := range w.runs {
			if w.runs[j] != g.runs[j] {
				t.Errorf("%s: run %d %+v, want %+v", where, j, g.runs[j], w.runs[j])
			}
		}
	}
}

// reimport parses an exported .docx and gives its chapters' docs with the
// pictures put back at the addresses they came from (matched by bytes),
// and the imported styles as an exportBook to resolve them.
func reimport(t *testing.T, data []byte, uploads string) (*docxImport, *exportBook) {
	t.Helper()
	imp, err := parseDocx(data)
	if err != nil {
		t.Fatal("re-import:", err)
	}
	byBytes := map[string]string{}
	filepath.Walk(uploads, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			d, _ := os.ReadFile(p)
			rel, _ := filepath.Rel(uploads, p)
			byBytes[string(d)] = "/uploads/" + filepath.ToSlash(rel)
		}
		return nil
	})
	urls := make([]string, len(imp.Pictures))
	for i, p := range imp.Pictures {
		urls[i] = byBytes[string(p.Data)]
		if urls[i] == "" {
			t.Errorf("re-imported picture %d (%s) matches no upload", i, p.Name)
		}
	}
	imp.usePictureURLs(urls)
	for i, c := range imp.Chapters {
		if err := validatePMDoc(c.Doc); err != nil {
			t.Errorf("re-imported chapter %d is not a valid doc: %v", i, err)
		}
	}
	settings, _ := json.Marshal(map[string]any{"page_type": imp.PageTypeGuess, "page": imp.Page, "styles": imp.Styles})
	rb := &exportBook{}
	parseExportSettings(settings, rb)
	return imp, rb
}

func TestExportDocxRoundTrip(t *testing.T) {
	b, up := testBook(t)
	addTestComments(b) // docxcomments_test.go
	data, err := exportDocx(b)
	if err != nil {
		t.Fatal(err)
	}
	imp, rb := reimport(t, data, up)
	if len(imp.Chapters) != len(b.Chapters) {
		for _, c := range imp.Chapters {
			t.Logf("imported chapter %q front=%v", c.Title, c.FrontMatter)
		}
		t.Fatalf("%d chapters back, want %d", len(imp.Chapters), len(b.Chapters))
	}
	for i, c := range b.Chapters {
		ic := imp.Chapters[i]
		if i == 0 {
			if !ic.FrontMatter {
				t.Errorf("chapter 1 should come back as front matter")
			}
		} else if ic.Title != c.Title {
			t.Errorf("chapter %d title %q, want %q", i+1, ic.Title, c.Title)
		}
		want := linearize(b, normalizeForDocx(c.Doc))
		got := linearize(rb, ic.Doc)
		compareLinear(t, c.Title, want, got)
		if w, g := docWords(normalizeForDocx(c.Doc)), ic.Words; w != g {
			t.Errorf("%s: %d words back, want %d", c.Title, g, w)
		}
	}

	// Page setup.
	pg := imp.Page
	for _, x := range [][2]float64{{pg.WidthMM, b.Page.WidthMM}, {pg.HeightMM, b.Page.HeightMM}, {pg.MarginTopMM, b.Page.MarginTopMM},
		{pg.MarginBottomMM, b.Page.MarginBottomMM}, {pg.MarginInnerMM, b.Page.MarginInnerMM}, {pg.MarginOuterMM, b.Page.MarginOuterMM}, {pg.GutterMM, b.Page.GutterMM}} {
		if !nearPt(x[0], x[1], 0.11) {
			t.Errorf("page %+v, want %+v", pg, b.Page)
			break
		}
	}
	if !pg.Mirror {
		t.Error("mirrored margins were lost")
	}

	// Styles in use come back with their names, kinds and bases.
	got := map[string]bookStyle{}
	for _, s := range imp.Styles {
		got[s.ID] = s
	}
	for _, id := range []string{"normal", "heading1", "heading2", "title", "subtitle", "quote", "letter", "word-shout", "whisper"} {
		src := b.style(id)
		s, ok := got[expectedStyleID(id)]
		if !ok {
			t.Errorf("style %s did not come back (got %v)", id, sortedKeys(func() map[string]bool {
				m := map[string]bool{}
				for k := range got {
					m[k] = true
				}
				return m
			}()))
			continue
		}
		if s.Kind != src.Kind {
			t.Errorf("style %s: kind %s, want %s", id, s.Kind, src.Kind)
		}
		if !src.Builtin && s.Name != src.Name {
			t.Errorf("style %s: name %q, want %q", id, s.Name, src.Name)
		}
		if id == "letter" && (s.BasedOn == nil || *s.BasedOn != "normal") {
			t.Errorf("style letter: based_on %v, want normal", s.BasedOn)
		}
	}
	// The toc was dropped by the import (counted).
	if imp.LeftOut["toc"] != 1 {
		t.Errorf("left_out %v: the table of contents should be counted once", imp.LeftOut)
	}
	// Comments, their ranges, replies and resolved state.
	checkCommentsCameBack(t, imp)
}

// --- the package's structure ---

func unzip(t *testing.T, data []byte) (map[string][]byte, []*zip.File) {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		files[f.Name], _ = io.ReadAll(rc)
		rc.Close()
	}
	return files, zr.File
}

func wellFormed(t *testing.T, name string, data []byte) {
	t.Helper()
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = true
	dec.Entity = map[string]string{} // XHTML: only XML's own entities
	for {
		_, err := dec.Token()
		if err == io.EOF {
			return
		}
		if err != nil {
			t.Fatalf("%s is not well-formed XML: %v", name, err)
		}
	}
}

func TestExportDocxPackage(t *testing.T) {
	b, _ := testBook(t)
	data, err := exportDocx(b)
	if err != nil {
		t.Fatal(err)
	}
	if out := os.Getenv("POLARIS_EXPORT_OUT"); out != "" {
		// For a look in Word or a reader.
		os.MkdirAll(out, 0o755)
		os.WriteFile(filepath.Join(out, "synthetic.docx"), data, 0o644)
		if epub, err := exportEPUB(b); err == nil {
			os.WriteFile(filepath.Join(out, "synthetic.epub"), epub, 0o644)
		}
	}
	files, _ := unzip(t, data)
	for name, d := range files {
		if strings.HasSuffix(name, ".xml") || strings.HasSuffix(name, ".rels") {
			wellFormed(t, name, d)
		}
	}
	ct := string(files["[Content_Types].xml"])
	// Every part has a content type; every relationship target exists.
	for name := range files {
		if name == "[Content_Types].xml" || strings.HasSuffix(name, ".rels") {
			continue
		}
		ext := name[strings.LastIndex(name, ".")+1:]
		if !strings.Contains(ct, `PartName="/`+name+`"`) && !strings.Contains(ct, `Extension="`+ext+`"`) {
			t.Errorf("%s has no content type", name)
		}
	}
	relRe := regexp.MustCompile(`Target="([^"]+)"( TargetMode="External")?`)
	for _, m := range relRe.FindAllStringSubmatch(string(files["word/_rels/document.xml.rels"]), -1) {
		if m[2] == "" && files["word/"+m[1]] == nil {
			t.Errorf("relationship to a missing part %s", m[1])
		}
	}
	doc := string(files["word/document.xml"])
	settings := string(files["word/settings.xml"])
	for _, want := range []string{"<w:mirrorMargins/>", "<w:evenAndOddHeaders/>", `<w:updateFields w:val="true"/>`} {
		if !strings.Contains(settings, want) {
			t.Errorf("settings.xml lacks %s", want)
		}
	}
	if n := strings.Count(doc, "<w:sectPr>"); n != len(b.Chapters) {
		t.Errorf("%d sections, want one per chapter (%d)", n, len(b.Chapters))
	}
	for _, want := range []string{
		`<w:pgNumType w:fmt="lowerRoman" w:start="1"/>`, // front matter
		`<w:pgNumType w:fmt="decimal" w:start="1"/>`,    // the first chapter
		`<w:pgNumType w:fmt="decimal" w:start="5"/>`,    // number_restart
		`<w:type w:val="oddPage"/>`, `<w:titlePg/>`, `w:gutter="227"`,
		` TOC \o "1-2" \h \z \u `, `<w:docPartGallery w:val="Table of Contents"/>`,
		`<w:numPr><w:ilvl w:val="1"/>`, `<w:pageBreakBefore/>`, `relativeFrom="page"`, `<w:softHyphen/>`, `<w:noBreakHyphen/>`,
		`<w:hyperlink w:anchor="ch1"`, `<w:bookmarkStart w:id="`, `<w:smallCaps/>`, `<w:highlight w:val="yellow"/>`, `w:fill="FFEEAA"`,
		`<w:rStyle w:val="shout"/>`, `<w:outlineLvl w:val="9"/>`, `<w:outlineLvl w:val="2"/>`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("document.xml lacks %s", want)
		}
	}
	if !strings.Contains(string(files["word/_rels/document.xml.rels"]), `Target="https://example.com/a%20b?x=1&amp;y=%C3%A7" TargetMode="External"`) {
		t.Errorf("web link relationship not escaped right:\n%s", files["word/_rels/document.xml.rels"])
	}
	styles := string(files["word/styles.xml"])
	for _, want := range []string{`w:styleId="Heading1"><w:name w:val="heading 1"/>`, `<w:outlineLvl w:val="0"/>`,
		`w:styleId="letter"><w:name w:val="Letter"/><w:basedOn w:val="Normal"/>`, `w:type="character" w:customStyle="1" w:styleId="shout"`,
		`w:line="260" w:lineRule="exact"`, `<w:rFonts w:ascii="Merriweather"`, `w:styleId="TOC1"`, `w:leader="dot"`} {
		if !strings.Contains(styles, want) {
			t.Errorf("styles.xml lacks %s", want)
		}
	}
	// Headers and footers: the chapter's title, PAGE fields, the book and
	// author on even pages; none over the front matter.
	var hf strings.Builder
	for name, d := range files {
		if strings.HasPrefix(name, "word/header") || strings.HasPrefix(name, "word/footer") {
			hf.Write(d)
		}
	}
	for _, want := range []string{`>Chapter One<`, `>Chapter Two<`, `w:instr=" PAGE "`, `>The Test Book<`, `>Ana Autora<`, `>— <`} {
		if !strings.Contains(hf.String(), want) {
			t.Errorf("headers and footers lack %s", want)
		}
	}
	core := string(files["docProps/core.xml"])
	for _, want := range []string{"<dc:title>The Test Book</dc:title>", "<dc:creator>Ana Autora</dc:creator>", "<dc:language>pt-BR</dc:language>", "A book &amp; a test."} {
		if !strings.Contains(core, want) {
			t.Errorf("core.xml lacks %s", want)
		}
	}
	if !strings.Contains(string(files["docProps/app.xml"]), "<Company>Small Press</Company>") {
		t.Error("app.xml lacks the publisher")
	}
	// Each picture is in the package once.
	media := 0
	for name := range files {
		if strings.HasPrefix(name, "word/media/") {
			media++
		}
	}
	if media != 3 {
		t.Errorf("%d pictures in the package, want 3", media)
	}
}

// --- EPUB ---

func TestExportEPUB(t *testing.T) {
	b, _ := testBook(t)
	data, err := exportEPUB(b)
	if err != nil {
		t.Fatal(err)
	}
	files, list := unzip(t, data)
	if list[0].Name != "mimetype" || list[0].Method != zip.Store || string(files["mimetype"]) != "application/epub+zip" || len(list[0].Extra) != 0 {
		t.Fatal("mimetype must come first, stored, without extra fields")
	}
	if !strings.Contains(string(files["META-INF/container.xml"]), `full-path="OEBPS/content.opf"`) {
		t.Fatal("container.xml doesn't point at the OPF")
	}
	for name, d := range files {
		if strings.HasSuffix(name, ".xml") || strings.HasSuffix(name, ".opf") || strings.HasSuffix(name, ".xhtml") || strings.HasSuffix(name, ".ncx") {
			wellFormed(t, name, d)
		}
	}
	// The manifest lists every file and nothing else; the spine its pages.
	var opf struct {
		UID      string `xml:"unique-identifier,attr"`
		Metadata struct {
			Identifiers []struct {
				ID    string `xml:"id,attr"`
				Value string `xml:",chardata"`
			} `xml:"identifier"`
			Titles   []string `xml:"title"`
			Creator  string   `xml:"creator"`
			Language string   `xml:"language"`
			Metas    []struct {
				Property string `xml:"property,attr"`
				Name     string `xml:"name,attr"`
				Content  string `xml:"content,attr"`
				Value    string `xml:",chardata"`
			} `xml:"meta"`
		} `xml:"metadata"`
		Items []struct {
			ID    string `xml:"id,attr"`
			Href  string `xml:"href,attr"`
			Type  string `xml:"media-type,attr"`
			Props string `xml:"properties,attr"`
		} `xml:"manifest>item"`
		Spine []struct {
			IDRef string `xml:"idref,attr"`
		} `xml:"spine>itemref"`
	}
	if err := xml.Unmarshal(files["OEBPS/content.opf"], &opf); err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}
	listed := map[string]bool{}
	cover, nav := "", ""
	for _, it := range opf.Items {
		if files["OEBPS/"+it.Href] == nil {
			t.Errorf("manifest item %s: no file %s", it.ID, it.Href)
		}
		ids[it.ID] = it.Href
		listed["OEBPS/"+it.Href] = true
		if strings.Contains(it.Props, "cover-image") {
			cover = it.Href
		}
		if strings.Contains(it.Props, "nav") {
			nav = it.Href
		}
	}
	for name := range files {
		if strings.HasPrefix(name, "OEBPS/") && name != "OEBPS/content.opf" && !listed[name] {
			t.Errorf("%s is not in the manifest", name)
		}
	}
	if len(opf.Spine) != len(b.Chapters) {
		t.Errorf("spine has %d pages, want %d", len(opf.Spine), len(b.Chapters))
	}
	for _, s := range opf.Spine {
		if ids[s.IDRef] == "" {
			t.Errorf("spine names %s, not in the manifest", s.IDRef)
		}
	}
	if cover == "" || nav != "nav.xhtml" {
		t.Errorf("cover %q nav %q", cover, nav)
	}
	if opf.Metadata.Identifiers[0].ID != opf.UID || opf.Metadata.Identifiers[0].Value != "urn:isbn:9783161484100" {
		t.Errorf("identifier %+v", opf.Metadata.Identifiers)
	}
	if opf.Metadata.Creator != "Ana Autora" || opf.Metadata.Language != "pt-BR" || len(opf.Metadata.Titles) != 2 {
		t.Errorf("metadata %+v", opf.Metadata)
	}
	modified := false
	for _, m := range opf.Metadata.Metas {
		if m.Property == "dcterms:modified" && regexp.MustCompile(`^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ$`).MatchString(m.Value) {
			modified = true
		}
	}
	if !modified {
		t.Error("no dcterms:modified")
	}
	// Every link inside the book points at a file and an id there.
	hrefRe := regexp.MustCompile(`(?:href|src)="([^"#:]+)(?:#([^"]+))?"`)
	for name, d := range files {
		if !strings.HasSuffix(name, ".xhtml") {
			continue
		}
		for _, m := range hrefRe.FindAllStringSubmatch(string(d), -1) {
			target := "OEBPS/" + m[1]
			if files[target] == nil {
				t.Errorf("%s links to a missing %s", name, m[1])
				continue
			}
			if m[2] != "" && !strings.Contains(string(files[target]), `id="`+m[2]+`"`) {
				t.Errorf("%s links to a missing id %s#%s", name, m[1], m[2])
			}
		}
	}
	navx := string(files["OEBPS/nav.xhtml"])
	for _, want := range []string{`epub:type="toc"`, `>Chapter One</a>`, `>A Section</a>`, `>Chapter Three: Restarted</a>`} {
		if !strings.Contains(navx, want) {
			t.Errorf("nav lacks %s", want)
		}
	}
	ch1 := string(files["OEBPS/ch002.xhtml"])
	for _, want := range []string{`<h1 id="h1" class="s-heading1">`, `<strong>bold</strong>`, `<em>italic</em>`, `<u>under</u>`, `<s>struck</s>`,
		`<sup>2</sup>`, `<sub>2</sub>`, `font-variant: small-caps`, `background-color: #ffeeaa`, `href="https://example.com/a%20b?x=1&amp;y=%C3%A7"`,
		`href="ch002.xhtml#ch1"`, `<p class="scene-break" role="separator">* * *</p>`, `<span class="speaker">Mira</span>, softly: “Come <em>here</em>.”`,
		`class="s-letter"`, `<br/>`, `class="tab"`, `<figure class="image" style="text-align: right">`, `style="width: `} {
		if !strings.Contains(ch1, want) {
			t.Errorf("chapter one's XHTML lacks %s", want)
		}
	}
	front := string(files["OEBPS/ch001.xhtml"])
	for _, want := range []string{`epub:type="frontmatter"`, `class="full-page"`, `<section class="toc">`, `href="ch002.xhtml#h1">Chapter One</a>`, `href="ch002.xhtml#h2">A Section</a>`} {
		if !strings.Contains(front, want) {
			t.Errorf("front matter lacks %s", want)
		}
	}
	css := string(files["OEBPS/style.css"])
	for _, want := range []string{`p, .s-normal {`, `font-family: "Merriweather", serif`, `.s-letter {`, `text-indent: 1.352em`} {
		if !strings.Contains(css, want) {
			t.Errorf("style.css lacks %s", want)
		}
	}
	if strings.Contains(css, "@font-face") {
		t.Error("fonts must not be embedded")
	}
}

// --- Online Post ---

func TestExportOnlinePost(t *testing.T) {
	b, _ := testBook(t)
	h, _ := exportHTML(b)
	html := string(h)
	body := html[strings.Index(html, "<body>")+6 : strings.Index(html, "</body>")]
	allowed := map[string]bool{"p": true, "em": true, "strong": true, "u": true, "s": true, "sup": true, "sub": true, "h1": true, "h2": true,
		"h3": true, "hr": true, "blockquote": true, "br": true, "a": true, "ul": true, "ol": true, "li": true}
	for _, m := range regexp.MustCompile(`<(/?)([a-z0-9]+)([^>]*)>`).FindAllStringSubmatch(body, -1) {
		if !allowed[m[2]] {
			t.Errorf("tag %s isn't allowed", m[2])
		}
		attrs := strings.TrimSpace(m[3])
		if attrs != "" && attrs != `style="text-align: center;"` && !strings.HasPrefix(attrs, `href="`) && !strings.HasPrefix(attrs, `start="`) {
			t.Errorf("tag %s has attributes %s", m[2], attrs)
		}
	}
	for _, want := range []string{`<h1 style="text-align: center;">Chapter One</h1>`, `<strong>bold</strong>`, `<em>italic</em>`,
		`<a href="https://example.com/a b?x=1&amp;y=ç">web</a>`, `back`, `<br>after break`, `<p style="text-align: center;">* * *</p>`,
		`<blockquote>`, `<p><em>A quoted line.</em></p>`, `<p>Mira, softly: “Come <em>here</em>.”</p>`, `<p>&nbsp;</p>`, `<hr>`,
		"<ul>\n<li>first bullet</li>", `<ol start="3">`, `<p><em>Dear reader, a letter.</em></p>`} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML lacks %s", want)
		}
	}
	if strings.Contains(body, "<img") || strings.Contains(body, `href="#`) || strings.Contains(body, "class=") {
		t.Error("HTML holds pictures, in-book links or classes")
	}

	md, _ := exportMarkdown(b)
	mds := string(md)
	for _, want := range []string{"# Chapter One", "## A Section", "**bold**", "*italic*", "~~struck~~", "<u>under</u>", "x<sup>2</sup>",
		"[web](https://example.com/a%20b?x=1&y=ç)", "\\\nafter break", "* * *", "\\~ ❦ \\~", "> *A quoted line.*", "- first bullet",
		"  1. nested one", "3. three", "  its second paragraph", "- second bullet\n  1. nested one", "Mira, softly: “Come *here*.”", "**Contents**"} {
		if !strings.Contains(mds, want) {
			t.Errorf("Markdown lacks %q", want)
		}
	}
	tx, _ := exportText(b)
	txt := string(tx)
	for _, want := range []string{"Chapter One\n\nPlain, bold italic", "after break\tafter tab", "* * *", "• first bullet",
		"  1. nested one", "3. three", "Mira, softly: “Come here.”", "Contents\nChapter One\n  A Section"} {
		if !strings.Contains(txt, want) {
			t.Errorf("text lacks %q", want)
		}
	}
	if strings.ContainsAny(txt, "<>*") && strings.Contains(txt, "<") {
		t.Error("text holds markup")
	}
}

// --- odd inputs ---

func TestExportOddInputs(t *testing.T) {
	formats := []string{"docx", "epub", "html", "md", "txt"}
	run := func(name string, b *exportBook) map[string][]byte {
		out := map[string][]byte{}
		for _, f := range formats {
			data, err := exportFormats[f](b)
			if err != nil {
				t.Fatalf("%s as %s: %v", name, f, err)
			}
			out[f] = data
		}
		if _, err := parseDocx(out["docx"]); err != nil {
			t.Errorf("%s: the .docx doesn't read back: %v", name, err)
		}
		files, _ := unzip(t, out["epub"])
		for n, d := range files {
			if strings.HasSuffix(n, ".xhtml") || strings.HasSuffix(n, ".opf") {
				wellFormed(t, name+" "+n, d)
			}
		}
		files, _ = unzip(t, out["docx"])
		for n, d := range files {
			if strings.HasSuffix(n, ".xml") {
				wellFormed(t, name+" "+n, d)
			}
		}
		return out
	}
	settings := json.RawMessage(`{"page_type":"a5"}`)

	empty := &exportBook{}
	parseExportSettings(settings, empty)
	run("empty book", empty)

	blank := &exportBook{Chapters: []exportChapter{{ID: 1, Title: "Blank", Doc: emptyPMDoc()}}}
	parseExportSettings(settings, blank)
	run("empty chapter", blank)

	missing := &exportBook{uploadsDir: t.TempDir(), Chapters: []exportChapter{{ID: 1, Title: "Gone", Doc: &pmNode{Type: "doc", Content: []*pmNode{
		blk("image", kv("src", "/uploads/editor/nothing.png", "fullPage", true)),
		blk("image", kv("src", "/uploads/../../etc/passwd")),
		blk("scene_break", kv("image", "/uploads/editor/none.png", "glyph", "#")),
		para(pt("Text stays.")),
	}}}}}
	parseExportSettings(settings, missing)
	out := run("missing pictures", missing)
	if missing.Missing != 3 {
		t.Errorf("%d pictures counted missing, want 3", missing.Missing)
	}
	if !strings.Contains(string(out["txt"]), "Text stays.") {
		t.Error("the text went with the pictures")
	}

	// Huge text: one long chapter, with odd characters XML can't hold.
	var paras []*pmNode
	for i := 0; i < 20000; i++ {
		paras = append(paras, para(pt(fmt.Sprintf("Paragraph %d with <markup> & \x01control\x0b chars and some words to pad it out.", i))))
	}
	huge := &exportBook{Chapters: []exportChapter{{ID: 1, Title: "Huge <&>", Doc: &pmNode{Type: "doc", Content: append([]*pmNode{
		blk("heading", kv("level", 1), pt("Huge <&>"))}, paras...)}}}}
	parseExportSettings(settings, huge)
	out = run("huge text", huge)
	imp, _ := parseDocx(out["docx"])
	if imp.Chapters[0].Words != docWords(huge.Chapters[0].Doc) {
		t.Errorf("huge: %d words back, want %d", imp.Chapters[0].Words, docWords(huge.Chapters[0].Doc))
	}
	if len(out["docx"]) > 4<<20 {
		t.Errorf("huge: the .docx is %d bytes", len(out["docx"]))
	}

	// Online page type: no headers, footers or page numbers.
	online := &exportBook{Chapters: []exportChapter{{ID: 1, Title: "Post", Doc: &pmNode{Type: "doc", Content: []*pmNode{
		blk("heading", kv("level", 1), pt("Post")), para(pt("One")), blk("page_break", nil), para(pt("Two"))}}}}}
	parseExportSettings(json.RawMessage(`{"page_type":"online"}`), online)
	out = run("online", online)
	files, _ := unzip(t, out["docx"])
	if strings.Contains(string(files["word/document.xml"]), "headerReference") || strings.Contains(string(files["word/document.xml"]), "pageBreakBefore") {
		t.Error("an online book's .docx has headers or page breaks")
	}
}

func TestExportFileNames(t *testing.T) {
	b := &exportBook{Meta: exportMeta{Title: `Ação: "Livro"/1?`}}
	if got := b.exportFileName("docx"); got != "Ação Livro 1.docx" {
		t.Errorf("file name %q", got)
	}
	cd := contentDisposition("Ação Livro 1.docx")
	if cd != `attachment; filename="Acao Livro 1.docx"; filename*=UTF-8''A%C3%A7%C3%A3o%20Livro%201.docx` {
		t.Errorf("Content-Disposition %s", cd)
	}
	b = &exportBook{VolumeTitle: "  ", Single: true, Chapters: []exportChapter{{Title: "Um"}}}
	if got := b.exportFileName("txt"); got != "Book - Um.txt" {
		t.Errorf("file name %q", got)
	}
}

// --- the routes ---

func TestExportRoutes(t *testing.T) {
	app, h, base, vol := editorSetup(t)
	ch := addChapter(t, h, base, "Opening", vol)
	other := addChapter(t, h, base, "Elsewhere", 0)
	doc := `{"type":"doc","content":[{"type":"heading","attrs":{"level":1},"content":[{"type":"text","text":"Opening"}]},` +
		`{"type":"paragraph","content":[{"type":"text","text":"It began."}]},` +
		`{"type":"image","attrs":{"src":"/uploads/editor/x.png","alt":"","width":null,"align":"center","fullPage":false}}]}`
	if code, _ := putText(t, h, base, ch, doc, 0); code != 200 {
		t.Fatal(code)
	}
	writePNG(t, filepath.Join(app.uploadsDir, "editor", "x.png"), 10, 10, color.RGBA{1, 2, 3, 255})
	set := `{"settings":{"page_type":"a5","meta":{"title":"Livro Ação"}}}`
	if rec := do(t, h, "PUT", fmt.Sprintf("%s/books/%d", base, vol), strings.NewReader(set), "application/json"); rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	for f, mime := range exportMIME {
		rec := do(t, h, "GET", fmt.Sprintf("%s/books/%d/export.%s", base, vol, f), nil, "")
		if rec.Code != 200 {
			t.Fatalf("%s: %d %s", f, rec.Code, rec.Body.String())
		}
		if rec.Header().Get("Content-Type") != mime || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s: headers %v", f, rec.Header())
		}
		if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, `filename="Livro Acao.`+f+`"`) || !strings.Contains(cd, "filename*=UTF-8''Livro%20A%C3%A7%C3%A3o.") {
			t.Errorf("%s: Content-Disposition %s", f, cd)
		}
		if f == "docx" {
			files, _ := unzip(t, rec.Body.Bytes())
			if files["word/media/image1.png"] == nil {
				t.Error("the picture didn't make it into the .docx")
			}
		}
		if f == "txt" && !strings.Contains(rec.Body.String(), "It began.") {
			t.Errorf("txt: %q", rec.Body.String())
		}
	}
	rec := do(t, h, "GET", fmt.Sprintf("%s/books/%d/export.md?chapter=%d", base, vol, ch), nil, "")
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Disposition"), "Livro Acao - Opening.md") {
		t.Errorf("one chapter: %d %v", rec.Code, rec.Header())
	}
	for _, url := range []string{
		fmt.Sprintf("%s/books/%d/export.docx", base, vol+99),
		fmt.Sprintf("%s/books/%d/export.docx?chapter=%d", base, vol, other), // not in this book
		fmt.Sprintf("%s/books/%d/export.epub?chapter=999", base, vol),
		fmt.Sprintf("%s/books/%d/export.txt?chapter=abc", base, vol),
	} {
		if rec := do(t, h, "GET", url, nil, ""); rec.Code != 404 {
			t.Errorf("%s: %d, want 404", url, rec.Code)
		}
	}
	// Book 0 (loose chapters) exports too.
	if rec := do(t, h, "GET", base+"/books/0/export.txt", nil, ""); rec.Code != 200 {
		t.Errorf("book 0: %d", rec.Code)
	}
}
