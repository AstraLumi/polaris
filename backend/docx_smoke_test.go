package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestDocxSmoke imports a real .docx and prints a summary (no text beyond
// chapter titles). Skipped unless POLARIS_DOCX_SMOKE names a file:
//
//	POLARIS_DOCX_SMOKE=/path/book.docx go test -run TestDocxSmoke -v .
func TestDocxSmoke(t *testing.T) {
	file := os.Getenv("POLARIS_DOCX_SMOKE")
	if file == "" {
		t.Skip("set POLARIS_DOCX_SMOKE to a .docx file")
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	imp, err := parseDocx(data)
	if err != nil {
		t.Fatal(err)
	}

	styles := map[string]bookStyle{}
	for _, s := range imp.Styles {
		styles[s.ID] = s
	}
	// lookup walks a style's chain for a para or text key.
	var lookup func(id, group, key string) any
	lookup = func(id, group, key string) any {
		s, ok := styles[id]
		if !ok {
			return nil
		}
		m := s.Para
		if group == "text" {
			m = s.Text
		}
		if v, ok := m[key]; ok {
			return v
		}
		if s.BasedOn != nil {
			return lookup(*s.BasedOn, group, key)
		}
		if id != "normal" {
			return lookup("normal", group, key)
		}
		return nil
	}

	total := 0
	t.Logf("chapters: %d", len(imp.Chapters))
	for i, ch := range imp.Chapters {
		title := ch.Title
		if ch.FrontMatter {
			title = "[" + docxFrontMatterTitle + "]"
		}
		js, _ := json.Marshal(ch.Doc)
		t.Logf("  %2d. %-40q %6d words  %7d bytes JSON", i+1, title, ch.Words, len(js))
		total += ch.Words
	}
	plain, plainTOC := plainDocxWords(t, data)
	t.Logf("words: imported %d; plain text of document.xml %d (of which in TOC-style paragraphs %d)", total, plain, plainTOC)

	t.Logf("styles:")
	for _, s := range imp.Styles {
		based := "-"
		if s.BasedOn != nil {
			based = *s.BasedOn
		}
		pj, _ := json.Marshal(s.Para)
		tj, _ := json.Marshal(s.Text)
		t.Logf("  %-18s %-14q %-9s based_on=%-10s builtin=%v para=%s text=%s", s.ID, s.Name, s.Kind, based, s.Builtin, pj, tj)
	}
	t.Logf("fonts: %v", imp.Fonts)
	pg, _ := json.Marshal(imp.Page)
	t.Logf("page: %s guess=%s", pg, imp.PageTypeGuess)
	lo, _ := json.Marshal(imp.LeftOut)
	t.Logf("left_out: %s", lo)
	for i, p := range imp.Pictures {
		t.Logf("picture %d: %s %s %d bytes", i, p.Name, p.ContentType, len(p.Data))
	}

	// Formatting statistics, from the docs and styles as a reader sees them.
	urls := make([]string, len(imp.Pictures))
	for i := range urls {
		urls[i] = fmt.Sprintf("/uploads/chapters/test-%d.png", i)
	}
	imp.usePictureURLs(urls)
	nodes := map[string]int{}
	aligns := map[string]int{}
	firstIndent, marksUsed := 0, map[string]int{}
	runs := map[string]int{}
	var images, scenes []string
	for _, ch := range imp.Chapters {
		if err := validatePMDoc(ch.Doc); err != nil {
			t.Errorf("chapter %q: invalid doc: %v", ch.Title, err)
		}
		var walk func(n *pmNode, style string)
		walk = func(n *pmNode, style string) {
			nodes[n.Type]++
			switch n.Type {
			case "paragraph", "heading":
				style = "normal"
				if s, ok := n.Attrs["style"].(string); ok {
					style = s
				}
				al, ok := n.Attrs["align"]
				if !ok {
					al = lookup(style, "para", "align")
				}
				aligns[fmt.Sprint(al)]++
				fi, ok := n.Attrs["indentFirst"]
				if !ok {
					fi = lookup(style, "para", "indentFirst")
				}
				if f, _ := pmNum(fi); f > 0 {
					firstIndent++
				}
			case "text":
				font, size := lookup(style, "text", "font"), lookup(style, "text", "size")
				for _, m := range n.Marks {
					marksUsed[m.Type]++
					switch m.Type {
					case "font":
						font = m.Attrs["family"]
					case "size":
						size = m.Attrs["pt"]
					}
				}
				runs[fmt.Sprintf("%v %vpt", font, size)]++
			case "image":
				images = append(images, fmt.Sprintf("fullPage=%v align=%v width=%v", n.Attrs["fullPage"], n.Attrs["align"], n.Attrs["width"]))
			case "scene_break":
				scenes = append(scenes, fmt.Sprint(n.Attrs["glyph"]))
			}
			for _, c := range n.Content {
				walk(c, style)
			}
		}
		walk(ch.Doc, "normal")
	}
	if os.Getenv("POLARIS_DOCX_SMOKE_OUTLINE") != "" {
		// The first blocks of the first two chapters, text as its length.
		for _, ch := range imp.Chapters[:min(2, len(imp.Chapters))] {
			t.Logf("outline of %q:", ch.Title)
			for i, b := range ch.Doc.Content[:min(25, len(ch.Doc.Content))] {
				t.Logf("  %2d %s", i, outlineNode(b))
			}
		}
	}
	t.Logf("nodes: %v", nodes)
	t.Logf("paragraph alignment: %v; with a first-line indent: %d", aligns, firstIndent)
	t.Logf("marks used (text nodes): %v", marksUsed)
	keys := make([]string, 0, len(runs))
	for k := range runs {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return runs[keys[i]] > runs[keys[j]] })
	for _, k := range keys {
		t.Logf("  text nodes in %-30s %d", k, runs[k])
	}
	t.Logf("images: %v", images)
	t.Logf("scene breaks: %d %v", len(scenes), uniqueStrings(scenes))
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// plainDocxWords counts words in document.xml's paragraphs the plain way
// (every w:t, deleted text aside; tabs and breaks part words), for
// comparison.
func plainDocxWords(t *testing.T, data []byte) (all, toc int) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var xmlText []byte
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, _ := f.Open()
			xmlText, _ = io.ReadAll(rc)
			rc.Close()
		}
	}
	paraRe := regexp.MustCompile(`(?s)<w:p[ >].*?</w:p>`)
	tRe := regexp.MustCompile(`<w:t(?: [^>]*)?>([^<]*)</w:t>`)
	breakRe := regexp.MustCompile(`<w:(?:tab|br|cr)(?: [^>]*)?/>`)
	for _, p := range paraRe.FindAll(xmlText, -1) {
		p = breakRe.ReplaceAll(p, []byte(`<w:t> </w:t>`))
		var b strings.Builder
		for _, m := range tRe.FindAllSubmatch(p, -1) {
			b.Write(m[1])
		}
		n := countWords(strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">").Replace(b.String()))
		all += n
		if bytes.Contains(p, []byte(`w:val="TOC`)) {
			toc += n
		}
	}
	return
}

// outlineNode shows a block with its attrs and marks, its text as lengths.
func outlineNode(n *pmNode) string {
	var b strings.Builder
	attrs, _ := json.Marshal(n.Attrs)
	fmt.Fprintf(&b, "%s %s", n.Type, attrs)
	for _, c := range n.Content {
		switch c.Type {
		case "text":
			var ms []string
			for _, m := range c.Marks {
				a, _ := json.Marshal(m.Attrs)
				ms = append(ms, m.Type+string(a))
			}
			fmt.Fprintf(&b, " [%d chars %s]", len([]rune(c.Text)), strings.Join(ms, ","))
		case "list_item", "paragraph":
			b.WriteString(" {" + outlineNode(c) + "}")
		default:
			b.WriteString(" <" + c.Type + ">")
		}
	}
	return b.String()
}
