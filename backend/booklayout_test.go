package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// settingsOf reads a book answer's settings as generic JSON.
func settingsOf(t *testing.T, body string) map[string]any {
	t.Helper()
	var out struct {
		Settings map[string]any `json:"settings"`
	}
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatal(err)
	}
	return out.Settings
}

func jpath(m map[string]any, keys ...string) any {
	var cur any = m
	for _, k := range keys {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = mm[k]
	}
	return cur
}

// A book never set up has the layout's defaults; a saved book gets the
// missing keys filled in and keeps the ones the server doesn't know.
func TestBookLayoutDefaultsAndUnknownKeys(t *testing.T) {
	_, h, base, vol := editorSetup(t)
	rec := do(t, h, "GET", fmt.Sprintf("%s/books/%d", base, vol), nil, "")
	s := settingsOf(t, rec.Body.String())
	if jpath(s, "layout", "header", "odd", "center") != "{chapter}" || jpath(s, "layout", "chapter_start") != "next" ||
		jpath(s, "layout", "numbering", "format") != "arabic" || jpath(s, "meta", "language") != "en" {
		t.Fatalf("defaults: %v", s)
	}

	body := `{"settings":{"page_type":"a6",
		"meta":{"title":"The Book","series":"Kept"},
		"layout":{"different_first":false,"header":{"odd":{"right":"{book} · {page}","italic":true,"font":"Merriweather","size":7,"glow":1}},
		          "numbering":{"format":"upper_roman"},"future":[1,2]},
		"chapter_options":{"12":{"front_matter":true,"color":"red"},"13":{"number_restart":5}}}}`
	code, _, raw := putBook(t, h, base, vol, body)
	if code != 200 {
		t.Fatal(code, raw)
	}
	s = settingsOf(t, raw)
	checks := map[string][2]any{
		"meta title":           {jpath(s, "meta", "title"), "The Book"},
		"meta unknown":         {jpath(s, "meta", "series"), "Kept"},
		"meta default":         {jpath(s, "meta", "language"), "en"},
		"first off":            {jpath(s, "layout", "different_first"), false},
		"odd/even default":     {jpath(s, "layout", "different_odd_even"), true},
		"band text":            {jpath(s, "layout", "header", "odd", "right"), "{book} · {page}"},
		"band default text":    {jpath(s, "layout", "header", "odd", "center"), "{chapter}"},
		"band unknown":         {jpath(s, "layout", "header", "odd", "glow"), float64(1)},
		"other band default":   {jpath(s, "layout", "header", "even", "center"), "{book}"},
		"format":               {jpath(s, "layout", "numbering", "format"), "upper_roman"},
		"start default":        {jpath(s, "layout", "numbering", "start"), float64(1)},
		"chapter front matter": {jpath(s, "chapter_options", "12", "front_matter"), true},
		"chapter unknown":      {jpath(s, "chapter_options", "12", "color"), "red"},
		"chapter default":      {jpath(s, "chapter_options", "12", "no_header"), false},
		"restart":              {jpath(s, "chapter_options", "13", "number_restart"), float64(5)},
	}
	for name, c := range checks {
		if fmt.Sprint(c[0]) != fmt.Sprint(c[1]) {
			t.Errorf("%s: got %v, want %v", name, c[0], c[1])
		}
	}
	if fmt.Sprint(jpath(s, "layout", "future")) != "[1 2]" {
		t.Errorf("layout's unknown key: %v", jpath(s, "layout", "future"))
	}

	// bookLayoutOf reads the stored settings as Go values.
	got := getBook(t, h, base, vol)
	_ = got
	rec = do(t, h, "GET", fmt.Sprintf("%s/books/%d", base, vol), nil, "")
	var out struct {
		Settings json.RawMessage `json:"settings"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	ex := bookLayoutOf(out.Settings)
	if ex.Meta.Title != "The Book" || ex.Layout.DifferentFirst || !ex.Layout.Header.Odd.Italic || ex.Layout.Header.Odd.Size == nil ||
		*ex.Layout.Header.Odd.Size != 7 || ex.Layout.Numbering.Format != "upper_roman" || !ex.ChapterOptions[12].FrontMatter ||
		ex.ChapterOptions[13].NumberRestart == nil || *ex.ChapterOptions[13].NumberRestart != 5 || ex.Layout.Footer.First.Center != "{page}" {
		t.Errorf("bookLayoutOf: %+v", ex)
	}
	if d := bookLayoutOf(json.RawMessage(`{"layout":{"chapter_start":"sideways"}}`)); d.Layout.ChapterStart != "next" {
		t.Errorf("a bad stored layout reads as the defaults: %+v", d.Layout)
	}
}

func TestBookLayoutRejects(t *testing.T) {
	_, h, base, vol := editorSetup(t)
	long := strings.Repeat("x", maxBandText+1)
	for name, extra := range map[string]string{
		"layout not an object":  `"layout":[1]`,
		"band text not text":    `"layout":{"header":{"odd":{"left":5}}}`,
		"band text too long":    `"layout":{"footer":{"even":{"center":"` + long + `"}}}`,
		"band size":             `"layout":{"footer":{"even":{"size":200}}}`,
		"chapter start":         `"layout":{"chapter_start":"left"}`,
		"number format":         `"layout":{"numbering":{"format":"klingon"}}`,
		"start not a whole":     `"layout":{"numbering":{"start":1.5}}`,
		"start negative":        `"layout":{"numbering":{"start":-1}}`,
		"meta not text":         `"meta":{"title":["a"]}`,
		"meta too long":         `"meta":{"description":"` + strings.Repeat("y", maxMetaLong+1) + `"}`,
		"chapter key":           `"chapter_options":{"abc":{}}`,
		"chapter key zero":      `"chapter_options":{"0":{}}`,
		"chapter option type":   `"chapter_options":{"4":{"front_matter":"yes"}}`,
		"chapter restart range": `"chapter_options":{"4":{"number_restart":100000}}`,
	} {
		code, _, raw := putBook(t, h, base, vol, `{"settings":{"page_type":"a5",`+extra+`}}`)
		if code != 400 {
			t.Errorf("%s: %d %s", name, code, raw)
		}
	}
}

// GET /api/books/{volume}/docs gives every chapter's text in reading order.
func TestBookDocs(t *testing.T) {
	_, h, base, vol := editorSetup(t)
	a := addChapter(t, h, base, "First", vol)
	b := addChapter(t, h, base, "Second", vol)
	addChapter(t, h, base, "Elsewhere", 0)
	if code, _ := putText(t, h, base, a, docOf("Hello there."), 0); code != 200 {
		t.Fatal(code)
	}
	rec := do(t, h, "GET", fmt.Sprintf("%s/books/%d/docs", base, vol), nil, "")
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	var docs []struct {
		ID    int64
		Title string
		Doc   *pmNode
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &docs); err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 || docs[0].ID != a || docs[1].ID != b || docs[0].Title != "First" {
		t.Fatalf("docs: %+v", docs)
	}
	if docText(docs[0].Doc) != "Hello there." || docs[1].Doc == nil || validatePMDoc(docs[1].Doc) != nil {
		t.Errorf("texts: %q %+v", docText(docs[0].Doc), docs[1].Doc)
	}
	if rec := do(t, h, "GET", base+"/books/999/docs", nil, ""); rec.Code != 404 {
		t.Errorf("unknown book: %d", rec.Code)
	}
	rec = do(t, h, "GET", base+"/books/0/docs", nil, "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Elsewhere") {
		t.Errorf("book 0: %d %s", rec.Code, rec.Body.String())
	}
}

// The toc node: an atom with levels 1–3 and a title.
func TestPMDocTOC(t *testing.T) {
	for _, ok := range []string{
		`{"type":"doc","content":[{"type":"toc"}]}`,
		`{"type":"doc","content":[{"type":"toc","attrs":{"levels":3,"title":"Contents"}},{"type":"paragraph"}]}`,
	} {
		if _, err := parsePMDoc([]byte(ok)); err != nil {
			t.Errorf("%s: %v", ok, err)
		}
	}
	for _, bad := range []string{
		`{"type":"doc","content":[{"type":"toc","attrs":{"levels":4}}]}`,
		`{"type":"doc","content":[{"type":"toc","attrs":{"levels":"x"}}]}`,
		`{"type":"doc","content":[{"type":"toc","attrs":{"title":5}}]}`,
		`{"type":"doc","content":[{"type":"toc","content":[{"type":"paragraph"}]}]}`,
		`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"toc"}]}]}`,
	} {
		if _, err := parsePMDoc([]byte(bad)); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
}

// The import answer says which new chapter is the front matter, so taking
// the file's layout can mark it.
func TestImportMarksFrontMatter(t *testing.T) {
	_, h, base, vol := editorSetup(t)
	body, ct := form(t, map[string]string{"mode": "append"}, "file", "Rainy Book.docx", testDocx(t))
	rec := do(t, h, "POST", fmt.Sprintf("%s/books/%d/import-docx", base, vol), body, ct)
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	var out struct {
		Chapters []struct {
			ID          int64
			Title       string
			FrontMatter bool `json:"front_matter"`
		}
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out.Chapters) != 3 || !out.Chapters[0].FrontMatter || out.Chapters[1].FrontMatter || out.Chapters[2].FrontMatter {
		t.Fatalf("chapters: %+v", out.Chapters)
	}
	// The book marks it too, without the file's layout being taken, and a
	// book that wasn't set up stays so.
	rec = do(t, h, "GET", fmt.Sprintf("%s/books/%d", base, vol), nil, "")
	var book struct {
		IsSetUp  bool `json:"is_set_up"`
		Settings json.RawMessage
	}
	json.Unmarshal(rec.Body.Bytes(), &book)
	opts := bookLayoutOf(book.Settings).ChapterOptions
	if !opts[out.Chapters[0].ID].FrontMatter || opts[out.Chapters[1].ID].FrontMatter {
		t.Errorf("chapter options: %+v", opts)
	}
	if book.IsSetUp {
		t.Error("the import set the book up")
	}
}
