package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestExportSmoke imports a real .docx, exports it in every format and
// imports the .docx export again, checking the chapters and word counts
// come back (no text is printed, only titles and counts). Skipped unless
// POLARIS_DOCX_SMOKE names a file; POLARIS_EXPORT_OUT, when set, is a
// folder the exports are written to:
//
//	POLARIS_DOCX_SMOKE=/path/book.docx POLARIS_EXPORT_OUT=/tmp/out go test -run TestExportSmoke -v .
func TestExportSmoke(t *testing.T) {
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
	up := t.TempDir()
	urls := make([]string, len(imp.Pictures))
	for i, p := range imp.Pictures {
		rel, err := saveUpload(up, "editor", fmt.Sprintf("pic-%d", i), p.Name, bytes.NewReader(p.Data))
		if err != nil {
			t.Fatal(err)
		}
		urls[i] = uploadURL(rel)
	}
	imp.usePictureURLs(urls)
	settings, _ := json.Marshal(map[string]any{"page_type": imp.PageTypeGuess, "page": imp.Page, "styles": imp.Styles,
		"meta": map[string]any{"title": "Smoke Test"}})
	b := &exportBook{Volume: 1, uploadsDir: up}
	for i, c := range imp.Chapters {
		title := c.Title
		if c.FrontMatter {
			title = docxFrontMatterTitle
		}
		b.Chapters = append(b.Chapters, exportChapter{ID: int64(i + 1), Title: title, Doc: c.Doc})
	}
	parseExportSettings(settings, b)

	out := os.Getenv("POLARIS_EXPORT_OUT")
	if out != "" {
		os.MkdirAll(out, 0o755)
	}
	var docx []byte
	for _, f := range []string{"docx", "epub", "html", "md", "txt"} {
		d, err := exportFormats[f](b)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		t.Logf("%s: %d bytes", f, len(d))
		if f == "docx" {
			docx = d
		}
		if out != "" {
			if err := os.WriteFile(filepath.Join(out, "export."+f), d, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if b.Missing > 0 {
		t.Errorf("%d pictures could not be read back from the uploads", b.Missing)
	}

	back, rb := reimport(t, docx, up)
	if len(back.Chapters) != len(imp.Chapters) {
		t.Fatalf("%d chapters back, want %d", len(back.Chapters), len(imp.Chapters))
	}
	total, totalBack := 0, 0
	diffs := map[string]int{}
	for i, c := range imp.Chapters {
		bc := back.Chapters[i]
		total += c.Words
		totalBack += bc.Words
		if bc.Title != c.Title || bc.FrontMatter != c.FrontMatter {
			t.Errorf("chapter %d: %q (front %v) back as %q (front %v)", i+1, c.Title, c.FrontMatter, bc.Title, bc.FrontMatter)
		}
		if bc.Words != c.Words {
			t.Errorf("chapter %d %q: %d words back, want %d", i+1, c.Title, bc.Words, c.Words)
		}
		// Block by block, counted by what differs (no text printed).
		want := linearize(b, normalizeForDocx(c.Doc))
		got := linearize(rb, bc.Doc)
		if len(want) != len(got) {
			diffs["block count"]++
			j := 0
			for j < len(want) && j < len(got) && want[j].kind == got[j].kind && want[j].text == got[j].text {
				j++
			}
			kinds := func(ls []linBlock) string {
				var s []string
				for k := max(0, j-5); k < min(len(ls), j+4); k++ {
					s = append(s, fmt.Sprintf("%s(%d chars)", ls[k].kind, len([]rune(ls[k].text))))
				}
				return strings.Join(s, " ")
			}
			t.Logf("chapter %d: %d blocks back, want %d; from block %d want %s, got %s", i+1, len(got), len(want), j, kinds(want), kinds(got))
			continue
		}
		for j := range want {
			w, g := want[j], got[j]
			switch {
			case w.kind != g.kind:
				diffs["kind "+w.kind+"→"+g.kind]++
			case w.text != g.text:
				diffs["text"]++
			case w.kind == "image" || w.kind == "full" || w.kind == "scene" || w.kind == "pb":
				if w.src != g.src {
					diffs["picture"]++
				}
			default:
				if g.style != expectedStyleID(w.style) {
					diffs["style "+w.style+"→"+g.style]++
				}
				if !w.skipFmt && !g.skipFmt {
					wp, gp := w.p, g.p
					if wp.Align != gp.Align {
						diffs["align"]++
					}
					if !nearPt(wp.IndentFirst, gp.IndentFirst, 0.051) || !nearPt(wp.IndentLeft, gp.IndentLeft, 0.051) || !nearPt(wp.IndentRight, gp.IndentRight, 0.051) {
						diffs["indent"]++
					}
					if !nearPt(wp.SpaceBefore, gp.SpaceBefore, 0.051) || !nearPt(wp.SpaceAfter, gp.SpaceAfter, 0.051) {
						diffs["spacing"]++
					}
					if !nearPt(wp.LineHeight, gp.LineHeight, 0.005) && wp.LineExact == 0 || !nearPt(wp.LineExact, gp.LineExact, 0.051) {
						diffs["line height"]++
					}
					if wp.KeepNext != gp.KeepNext {
						diffs["keepNext"]++
					}
				}
				if len(w.runs) != len(g.runs) {
					diffs["runs"]++
				} else {
					for k := range w.runs {
						if w.runs[k] != g.runs[k] {
							diffs["run format"]++
							break
						}
					}
				}
			}
		}
	}
	t.Logf("chapters: %d; words: %d imported, %d after export and import again", len(imp.Chapters), total, totalBack)
	keys := make([]string, 0, len(diffs))
	for k := range diffs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s: %d", k, diffs[k]))
	}
	if len(parts) == 0 {
		t.Logf("blocks: no differences")
	} else {
		t.Logf("blocks that differ: %s", strings.Join(parts, ", "))
	}
	t.Logf("left out on the second import: %v", back.LeftOut)
}
