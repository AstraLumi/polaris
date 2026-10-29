package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"strings"
	"testing"
	"time"
)

// editorSetup makes a story with one volume ("Book One") and returns the
// handler, the story's API base and the volume's id.
func editorSetup(t *testing.T) (*storyApp, http.Handler, string, int64) {
	t.Helper()
	app, h, base := planetSetup(t)
	rec := do(t, h, "POST", base+"/volumes", strings.NewReader(`{"title":"Book One"}`), "application/json")
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	var v struct{ ID int64 }
	json.Unmarshal(rec.Body.Bytes(), &v)
	return app, h, base, v.ID
}

func addChapter(t *testing.T, h http.Handler, base, title string, vol int64) int64 {
	t.Helper()
	body := fmt.Sprintf(`{"title":%q}`, title)
	if vol != 0 {
		body = fmt.Sprintf(`{"title":%q,"volume_id":%d}`, title, vol)
	}
	rec := do(t, h, "POST", base+"/chapters", strings.NewReader(body), "application/json")
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	var c struct{ ID int64 }
	json.Unmarshal(rec.Body.Bytes(), &c)
	return c.ID
}

// docOf is a doc of one paragraph per text.
func docOf(paras ...string) string {
	var parts []string
	for _, p := range paras {
		parts = append(parts, fmt.Sprintf(`{"type":"paragraph","content":[{"type":"text","text":%q}]}`, p))
	}
	return `{"type":"doc","content":[` + strings.Join(parts, ",") + `]}`
}

func putText(t *testing.T, h http.Handler, base string, ch int64, doc string, baseVersion int64) (int, chapterText) {
	t.Helper()
	body := fmt.Sprintf(`{"doc":%s,"base_version":%d}`, doc, baseVersion)
	rec := do(t, h, "PUT", fmt.Sprintf("%s/chapters/%d/text", base, ch), strings.NewReader(body), "application/json")
	var out chapterText
	json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func getText(t *testing.T, h http.Handler, base string, ch int64) chapterText {
	t.Helper()
	rec := do(t, h, "GET", fmt.Sprintf("%s/chapters/%d/text", base, ch), nil, "")
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	var out chapterText
	json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

func listTextVersions(t *testing.T, h http.Handler, base string, ch int64) []textVersionOut {
	t.Helper()
	rec := do(t, h, "GET", fmt.Sprintf("%s/chapters/%d/text/versions", base, ch), nil, "")
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	var out []textVersionOut
	json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

type bookJSON struct {
	Volume   int64 `json:"volume"`
	Title    string
	IsSetUp  bool `json:"is_set_up"`
	Words    int
	Chapters []bookChapter
	Settings struct {
		PageType string          `json:"page_type"`
		Page     bookPage        `json:"page"`
		Styles   []bookStyle     `json:"styles"`
		Extra    json.RawMessage `json:"extra"`
	}
	Defaults map[string]json.RawMessage
}

func getBook(t *testing.T, h http.Handler, base string, vol int64) bookJSON {
	t.Helper()
	rec := do(t, h, "GET", fmt.Sprintf("%s/books/%d", base, vol), nil, "")
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	var b bookJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	return b
}

func putBook(t *testing.T, h http.Handler, base string, vol int64, body string) (int, bookJSON, string) {
	t.Helper()
	rec := do(t, h, "PUT", fmt.Sprintf("%s/books/%d", base, vol), strings.NewReader(body), "application/json")
	var b bookJSON
	json.Unmarshal(rec.Body.Bytes(), &b)
	return rec.Code, b, rec.Body.String()
}

func styleIDs(list []bookStyle) string {
	var ids []string
	for _, s := range list {
		ids = append(ids, s.ID)
	}
	return strings.Join(ids, ",")
}

// Schema 44 adds the editor's tables to a story that has chapters already.
func TestMigrationAddsEditorTables(t *testing.T) {
	mgr, _ := newTestServer(t)
	app, _ := mgr.open(createStoryDirect(t, mgr, "Old"))
	rollBackSchema44(t, app.db)
	if _, err := app.db.Exec(`INSERT INTO chapters (title) VALUES ('Prologue')`); err != nil {
		t.Fatal(err)
	}
	if err := ensureSchema(app.db); err != nil {
		t.Fatal(err)
	}
	var v string
	app.db.QueryRow(`SELECT value FROM schema_meta WHERE key = 'schema_version'`).Scan(&v)
	if v != currentSchemaVersion {
		t.Fatalf("schema %s", v)
	}
	for _, table := range []string{"book_settings", "chapter_texts", "chapter_text_versions"} {
		var n int
		if err := app.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil || n != 0 {
			t.Fatalf("%s: %d %v", table, n, err)
		}
	}
	text, err := loadChapterText(app.db, 1)
	if err != nil || text.Version != 0 {
		t.Fatalf("old chapter's text: %+v %v", text, err)
	}
}

// A book never set up gets the a5 defaults with every built-in style; a
// book that doesn't exist is 404.
func TestBookDefaults(t *testing.T) {
	_, h, base, vol := editorSetup(t)
	addChapter(t, h, base, "One", vol)
	addChapter(t, h, base, "Loose", 0)
	b := getBook(t, h, base, vol)
	if b.IsSetUp || b.Settings.PageType != "a5" || b.Settings.Page.WidthMM != 148 || b.Title != "Book One" {
		t.Fatalf("defaults: %+v", b)
	}
	if got := styleIDs(b.Settings.Styles); got != strings.Join(builtinStyleIDs, ",") {
		t.Fatalf("styles: %s", got)
	}
	if b.Settings.Styles[0].Text["size"] != 10.5 {
		t.Fatalf("a5 body size: %v", b.Settings.Styles[0].Text)
	}
	if len(b.Chapters) != 1 || b.Chapters[0].Title != "One" || b.Chapters[0].Version != 0 {
		t.Fatalf("chapters: %+v", b.Chapters)
	}
	if len(b.Defaults) != len(pageTypeOrder) {
		t.Fatalf("defaults per type: %d", len(b.Defaults))
	}
	if loose := getBook(t, h, base, 0); len(loose.Chapters) != 1 || loose.Chapters[0].Title != "Loose" {
		t.Fatalf("book 0: %+v", loose.Chapters)
	}
	for _, bad := range []string{"999", "-1", "x"} {
		if c := do(t, h, "GET", base+"/books/"+bad, nil, "").Code; c != 404 {
			t.Errorf("book %s: %d", bad, c)
		}
	}
}

// Saving settings checks the page type and page, fixes the standard sizes'
// paper, brings back missing built-in styles and keeps unknown keys.
func TestSaveBookSettings(t *testing.T) {
	_, h, base, vol := editorSetup(t)
	code, b, body := putBook(t, h, base, vol, `{"settings":{"page_type":"a6",
		"page":{"width_mm":300,"height_mm":300,"margin_top_mm":10,"margin_bottom_mm":10,"margin_inner_mm":10,"margin_outer_mm":10,"gutter_mm":3,"mirror":true},
		"styles":[{"id":"normal","name":"Body","kind":"paragraph","builtin":true,"para":{"align":"left"},"text":{"font":"Merriweather","size":8},"future":1},
		          {"id":"chat","name":"Chat","kind":"character","text":{"italic":true}}],
		"extra":{"headers":true}}}`)
	if code != 200 {
		t.Fatal(code, body)
	}
	if !b.IsSetUp || b.Settings.PageType != "a6" || b.Settings.Page.WidthMM != 105 || b.Settings.Page.HeightMM != 148 || b.Settings.Page.GutterMM != 3 {
		t.Fatalf("saved: %+v", b.Settings)
	}
	if got := styleIDs(b.Settings.Styles); got != "normal,chat,heading1,heading2,heading3,title,subtitle,quote" {
		t.Fatalf("styles: %s", got)
	}
	if b.Settings.Styles[0].Name != "Body" || !strings.Contains(body, `"future":1`) || string(b.Settings.Extra) != `{"headers":true}` {
		t.Fatalf("kept as sent: %s", body)
	}
	// Book 0 is apart.
	if getBook(t, h, base, 0).IsSetUp {
		t.Fatal("book 0 should not be set up")
	}
	// A custom page keeps its size.
	code, b, body = putBook(t, h, base, vol, `{"settings":{"page_type":"custom","page":{"width_mm":152.4,"height_mm":228.6,"margin_top_mm":15,"margin_bottom_mm":15,"margin_inner_mm":15,"margin_outer_mm":12,"gutter_mm":5}}}`)
	if code != 200 || b.Settings.Page.WidthMM != 152.4 || len(b.Settings.Styles) != len(builtinStyleIDs) {
		t.Fatal(code, body)
	}
	// Online ignores a bad page.
	if code, _, body := putBook(t, h, base, vol, `{"settings":{"page_type":"online","page":{"width_mm":1}}}`); code != 200 {
		t.Fatal(code, body)
	}
	for _, bad := range []string{
		`{"settings":{"page_type":"letter"}}`,
		`{"settings":{}}`,
		`{"settings":[1]}`,
		`{}`,
		`{"settings":{"page_type":"custom","page":{"width_mm":20,"height_mm":30}}}`,
		`{"settings":{"page_type":"a6","page":{"margin_inner_mm":60,"margin_outer_mm":40}}}`,
		`{"settings":{"page_type":"a5","page":{"margin_top_mm":-1}}}`,
		`{"settings":{"page_type":"a5","styles":[{"id":"x","name":"X","kind":"table"}]}}`,
		`{"settings":{"page_type":"a5","styles":[{"id":"x","name":"X","kind":"paragraph"},{"id":"x","name":"Y","kind":"paragraph"}]}}`,
		`{"settings":{"page_type":"a5","styles":[{"id":"","name":"X","kind":"paragraph"}]}}`,
		`{"settings":{"page_type":"a5"},"switch_label":"` + strings.Repeat("x", 201) + `"}`,
	} {
		if code, _, body := putBook(t, h, base, vol, bad); code != 400 {
			t.Errorf("%s: %d %s", bad, code, body)
		}
	}
	if code, _, _ := putBook(t, h, base, 999, `{"settings":{"page_type":"a5"}}`); code != 404 {
		t.Errorf("unknown book: %d", code)
	}
}

// Switching the page type with a label keeps every written chapter of
// that book as a version first; other books are left alone.
func TestBookSwitchKeepsVersions(t *testing.T) {
	_, h, base, vol := editorSetup(t)
	a := addChapter(t, h, base, "A", vol)
	b := addChapter(t, h, base, "B", vol)
	addChapter(t, h, base, "Empty", vol)
	loose := addChapter(t, h, base, "Loose", 0)
	for _, ch := range []int64{a, b, loose} {
		if c, _ := putText(t, h, base, ch, docOf("Some words here"), 0); c != 200 {
			t.Fatal(c)
		}
	}
	code, book, body := putBook(t, h, base, vol, `{"settings":{"page_type":"a6"},"switch_label":"Before switching from A5 to A6"}`)
	if code != 200 || book.Settings.PageType != "a6" {
		t.Fatal(code, body)
	}
	for _, ch := range []int64{a, b} {
		vs := listTextVersions(t, h, base, ch)
		if len(vs) != 1 || vs[0].Label != "Before switching from A5 to A6" || vs[0].Automatic || vs[0].Words != 3 {
			t.Fatalf("chapter %d versions: %+v", ch, vs)
		}
	}
	if vs := listTextVersions(t, h, base, loose); len(vs) != 0 {
		t.Fatalf("the loose chapters are another book: %+v", vs)
	}
	if book.Words != 6 || len(book.Chapters) != 3 || book.Chapters[0].Words != 3 || book.Chapters[0].Version != 1 {
		t.Fatalf("book: %+v", book)
	}
}

// A chapter's text goes in and out; a save from an older copy gets 409
// with the stored text; bad documents are refused.
func TestChapterTextRoundTrip(t *testing.T) {
	app, h, base, vol := editorSetup(t)
	ch := addChapter(t, h, base, "One", vol)
	if got := getText(t, h, base, ch); got.Version != 0 || !strings.Contains(string(got.Doc), `"type":"doc"`) || got.Words != 0 {
		t.Fatalf("never written: %+v", got)
	}
	doc := `{"type":"doc","content":[{"type":"heading","attrs":{"level":1,"style":null,"mystery":7},"content":[{"type":"text","text":"Chapter One"}]},
		{"type":"paragraph","content":[{"type":"text","text":"She met "},{"type":"text","text":"Aria","marks":[{"type":"story_link","attrs":{"type":"character","id":3}}]},{"type":"text","text":" — at dawn, 3 times."}]}]}`
	code, saved := putText(t, h, base, ch, doc, 0)
	if code != 200 || saved.Version != 1 || saved.Words != 9 || saved.UpdatedAt == "" {
		t.Fatalf("save: %d %+v", code, saved)
	}
	got := getText(t, h, base, ch)
	if got.Version != 1 || !strings.Contains(string(got.Doc), `"mystery":7`) || !strings.Contains(string(got.Doc), `"story_link"`) {
		t.Fatalf("round trip: %s", got.Doc)
	}
	// Written on version 0 again: conflict, with the stored text.
	code, conflict := putText(t, h, base, ch, docOf("Other text"), 0)
	if code != 409 || conflict.Version != 1 || !strings.Contains(string(conflict.Doc), "Chapter One") {
		t.Fatalf("conflict: %d %+v", code, conflict)
	}
	// The same text again changes nothing.
	if code, same := putText(t, h, base, ch, string(got.Doc), 1); code != 200 || same.Version != 1 {
		t.Fatalf("unchanged: %d %+v", code, same)
	}
	if code, next := putText(t, h, base, ch, docOf("Second draft"), 1); code != 200 || next.Version != 2 || next.Words != 2 {
		t.Fatalf("second save: %d %+v", code, next)
	}
	// Pictures come back with the story's address and are stored without it.
	sid := strings.TrimPrefix(base, "/api/s/")
	pic := `{"type":"doc","content":[{"type":"image","attrs":{"src":"/uploads/s/` + sid + `/editor/a.png","alt":"","width":null,"align":"center","fullPage":false}}]}`
	if code, _ := putText(t, h, base, ch, pic, 2); code != 200 {
		t.Fatal(code)
	}
	var stored string
	app.db.QueryRow(`SELECT doc FROM chapter_texts WHERE chapter_id = ?`, ch).Scan(&stored)
	if !strings.Contains(stored, `"src":"/uploads/editor/a.png"`) {
		t.Fatalf("stored: %s", stored)
	}
	if got := getText(t, h, base, ch); !strings.Contains(string(got.Doc), `"/uploads/s/`+sid+`/editor/a.png"`) {
		t.Fatalf("served: %s", got.Doc)
	}

	url := fmt.Sprintf("%s/chapters/%d/text", base, ch)
	for _, bad := range []string{
		`{"doc":{"type":"paragraph"},"base_version":3}`,
		`{"doc":"text","base_version":3}`,
		`{"doc":null,"base_version":3}`,
		`{"doc":` + docOf("x") + `}`,
		`not json`,
	} {
		if c := do(t, h, "PUT", url, strings.NewReader(bad), "application/json").Code; c != 400 {
			t.Errorf("%s: %d", bad, c)
		}
	}
	huge := docOf(strings.Repeat("word ", maxDocBytes/5+10))
	if code, _ := putText(t, h, base, ch, huge, 3); code != 413 {
		t.Errorf("huge doc: %d", code)
	}
	if code, _ := putText(t, h, base, 999, docOf("x"), 0); code != 404 {
		t.Errorf("unknown chapter: %d", code)
	}
	if c := do(t, h, "GET", base+"/chapters/999/text", nil, "").Code; c != 404 {
		t.Errorf("unknown chapter: %d", c)
	}
}

// Versions: saved with a label (or from a posted doc), listed newest first,
// read with their doc, and restored after keeping the current text.
func TestTextVersionsAndRestore(t *testing.T) {
	_, h, base, vol := editorSetup(t)
	ch := addChapter(t, h, base, "One", vol)
	url := fmt.Sprintf("%s/chapters/%d/text/versions", base, ch)
	if c := do(t, h, "POST", url, strings.NewReader(`{"label":"Too soon"}`), "application/json").Code; c != 400 {
		t.Fatalf("nothing written yet: %d", c)
	}
	putText(t, h, base, ch, docOf("First draft"), 0)
	rec := do(t, h, "POST", url, strings.NewReader(`{"label":"  Draft one  "}`), "application/json")
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	var v1 textVersionOut
	json.Unmarshal(rec.Body.Bytes(), &v1)
	if v1.Label != "Draft one" || v1.Words != 2 || v1.Automatic || v1.Doc != nil {
		t.Fatalf("version: %+v", v1)
	}
	putText(t, h, base, ch, docOf("Second draft, longer"), 1)
	// The copy the writer didn't keep after a conflict.
	rec = do(t, h, "POST", url, strings.NewReader(`{"label":"Other copy","doc":`+docOf("Lost words kept")+`}`), "application/json")
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	if got := getText(t, h, base, ch); got.Version != 2 || !strings.Contains(string(got.Doc), "Second draft") {
		t.Fatalf("a posted doc leaves the text alone: %+v", got)
	}
	vs := listTextVersions(t, h, base, ch)
	if len(vs) != 2 || vs[0].Label != "Other copy" || vs[1].ID != v1.ID || vs[0].Doc != nil {
		t.Fatalf("list: %+v", vs)
	}
	rec = do(t, h, "GET", fmt.Sprintf("%s/%d", url, v1.ID), nil, "")
	var full textVersionOut
	json.Unmarshal(rec.Body.Bytes(), &full)
	if rec.Code != 200 || !strings.Contains(string(full.Doc), "First draft") {
		t.Fatalf("get version: %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, h, "POST", fmt.Sprintf("%s/%d/restore", url, v1.ID), nil, "")
	var restored chapterText
	json.Unmarshal(rec.Body.Bytes(), &restored)
	if rec.Code != 200 || restored.Version != 3 || restored.Words != 2 || !strings.Contains(string(restored.Doc), "First draft") {
		t.Fatalf("restore: %d %s", rec.Code, rec.Body.String())
	}
	vs = listTextVersions(t, h, base, ch)
	if len(vs) != 3 || vs[0].Label != restoreLabel || vs[0].Words != 3 {
		t.Fatalf("before restoring: %+v", vs)
	}
	// Another chapter's version isn't this one's.
	other := addChapter(t, h, base, "Two", vol)
	if c := do(t, h, "GET", fmt.Sprintf("%s/chapters/%d/text/versions/%d", base, other, v1.ID), nil, "").Code; c != 404 {
		t.Fatalf("other chapter's version: %d", c)
	}
	if c := do(t, h, "POST", fmt.Sprintf("%s/chapters/%d/text/versions/%d/restore", base, other, v1.ID), nil, "").Code; c != 404 {
		t.Fatalf("restore other chapter's version: %d", c)
	}
	if c := do(t, h, "POST", url, strings.NewReader(`{"label":"`+strings.Repeat("x", 201)+`"}`), "application/json").Code; c != 400 {
		t.Fatalf("long label: %d", c)
	}
	if c := do(t, h, "POST", url, strings.NewReader(`{"label":"x","doc":{"type":"nope"}}`), "application/json").Code; c != 400 {
		t.Fatalf("bad doc: %d", c)
	}
}

// The server keeps the text as an automatic version every 30 minutes of
// writing, keeps the last 20 of those, and never prunes the writer's own.
func TestAutomaticTextVersions(t *testing.T) {
	_, h, base, vol := editorSetup(t)
	clock := time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)
	editorNow = func() time.Time { return clock }
	t.Cleanup(func() { editorNow = time.Now })
	ch := addChapter(t, h, base, "One", vol)
	version := int64(0)
	save := func(text string, after time.Duration) {
		t.Helper()
		clock = clock.Add(after)
		code, out := putText(t, h, base, ch, docOf(text), version)
		if code != 200 {
			t.Fatalf("save %q: %d", text, code)
		}
		version = out.Version
	}
	save("one", 0)
	save("two", 10*time.Minute)
	save("three", 10*time.Minute)
	if vs := listTextVersions(t, h, base, ch); len(vs) != 0 {
		t.Fatalf("too soon: %+v", vs)
	}
	save("four", 11*time.Minute) // 31 minutes of writing: "three" is kept
	vs := listTextVersions(t, h, base, ch)
	if len(vs) != 1 || !vs[0].Automatic || vs[0].Label != autoVersionLabel || vs[0].CreatedAt != "2026-10-24 09:31:00" {
		t.Fatalf("automatic: %+v", vs)
	}
	rec := do(t, h, "GET", fmt.Sprintf("%s/chapters/%d/text/versions/%d", base, ch, vs[0].ID), nil, "")
	if !strings.Contains(rec.Body.String(), `"three"`) {
		t.Fatalf("kept text: %s", rec.Body.String())
	}
	save("five", 29*time.Minute)
	if vs := listTextVersions(t, h, base, ch); len(vs) != 1 {
		t.Fatalf("29 minutes later: %d", len(vs))
	}
	do(t, h, "POST", fmt.Sprintf("%s/chapters/%d/text/versions", base, ch), strings.NewReader(`{"label":"Mine"}`), "application/json")
	for i := 0; i < 25; i++ {
		save(fmt.Sprintf("draft %d", i), 31*time.Minute)
	}
	vs = listTextVersions(t, h, base, ch)
	auto, mine := 0, 0
	for _, v := range vs {
		if v.Automatic {
			auto++
		} else if v.Label == "Mine" {
			mine++
		}
	}
	if auto != keepAutoVersions || mine != 1 {
		t.Fatalf("pruned: %d automatic, %d mine", auto, mine)
	}
	// The newest automatic one is the text before the last save.
	rec = do(t, h, "GET", fmt.Sprintf("%s/chapters/%d/text/versions/%d", base, ch, vs[0].ID), nil, "")
	if !strings.Contains(rec.Body.String(), `"draft 23"`) {
		t.Fatalf("newest automatic: %s", rec.Body.String())
	}
}

// Deleting a chapter deletes its text and versions; deleting a volume
// deletes its book's settings, and its chapters (with their texts) join
// book 0.
func TestEditorDeletes(t *testing.T) {
	app, h, base, vol := editorSetup(t)
	a := addChapter(t, h, base, "A", vol)
	b := addChapter(t, h, base, "B", vol)
	for _, ch := range []int64{a, b} {
		putText(t, h, base, ch, docOf("Words"), 0)
		do(t, h, "POST", fmt.Sprintf("%s/chapters/%d/text/versions", base, ch), strings.NewReader(`{"label":"v"}`), "application/json")
	}
	putBook(t, h, base, vol, `{"settings":{"page_type":"a6"}}`)
	count := func(q string, args ...any) int {
		var n int
		app.db.QueryRow(q, args...).Scan(&n)
		return n
	}
	if c := do(t, h, "DELETE", fmt.Sprintf("%s/chapters/%d", base, a), nil, "").Code; c != 200 {
		t.Fatal(c)
	}
	if n := count(`SELECT COUNT(*) FROM chapter_texts WHERE chapter_id = ?`, a) + count(`SELECT COUNT(*) FROM chapter_text_versions WHERE chapter_id = ?`, a); n != 0 {
		t.Fatalf("deleted chapter's text left: %d", n)
	}
	if n := count(`SELECT COUNT(*) FROM chapter_text_versions WHERE chapter_id = ?`, b); n != 1 {
		t.Fatalf("other chapter's versions: %d", n)
	}
	if c := do(t, h, "DELETE", fmt.Sprintf("%s/volumes/%d", base, vol), nil, "").Code; c != 200 {
		t.Fatal(c)
	}
	if n := count(`SELECT COUNT(*) FROM book_settings`); n != 0 {
		t.Fatalf("book settings left: %d", n)
	}
	loose := getBook(t, h, base, 0)
	if loose.IsSetUp || len(loose.Chapters) != 1 || loose.Chapters[0].ID != b || loose.Chapters[0].Words != 1 {
		t.Fatalf("book 0: %+v", loose)
	}
}

// The chapter list and a chapter's page report the words in its text.
func TestChapterListsReportWords(t *testing.T) {
	_, h, base, vol := editorSetup(t)
	ch := addChapter(t, h, base, "One", vol)
	addChapter(t, h, base, "Two", vol)
	putText(t, h, base, ch, docOf("Four words right here"), 0)
	var idx chapterIndex
	json.Unmarshal(do(t, h, "GET", base+"/chapters", nil, "").Body.Bytes(), &idx)
	if len(idx.Chapters) != 2 || idx.Chapters[0].Words != 4 || idx.Chapters[1].Words != 0 {
		t.Fatalf("list: %+v", idx.Chapters)
	}
	var d chapterDetail
	json.Unmarshal(do(t, h, "GET", fmt.Sprintf("%s/chapters/%d", base, ch), nil, "").Body.Bytes(), &d)
	if d.Words != 4 {
		t.Fatalf("chapter page: %+v", d)
	}
}

// testDocx builds a small Word file: a line of front matter, then two
// chapters at Heading 1, the first with a picture.
func testDocx(t *testing.T) []byte {
	t.Helper()
	var pic bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(1, 1, color.RGBA{200, 10, 10, 255})
	png.Encode(&pic, img)
	const ns = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture"`
	para := func(style, text string) string {
		ppr := ""
		if style != "" {
			ppr = `<w:pPr><w:pStyle w:val="` + style + `"/></w:pPr>`
		}
		return `<w:p>` + ppr + `<w:r><w:t xml:space="preserve">` + text + `</w:t></w:r></w:p>`
	}
	drawing := `<w:p><w:r><w:drawing><wp:inline><wp:extent cx="952500" cy="952500"/><wp:docPr id="1" name="Picture 1"/>
		<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic>
		<pic:nvPicPr><pic:cNvPr id="1" name="map.png"/><pic:cNvPicPr/></pic:nvPicPr>
		<pic:blipFill><a:blip r:embed="rIdPic"/></pic:blipFill>
		<pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="952500" cy="952500"/></a:xfrm></pic:spPr>
		</pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing></w:r></w:p>`
	body := para("", "Dedicated to the rain.") +
		para("Heading1", "One") + para("", "It began on a grey morning.") + drawing +
		para("Heading1", "Two") + para("", "Then it ended.")
	files := map[string][]byte{
		"[Content_Types].xml": []byte(`<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
			<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
			<Default Extension="xml" ContentType="application/xml"/><Default Extension="png" ContentType="image/png"/>
			<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
			<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/></Types>`),
		"_rels/.rels": []byte(`<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
			<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`),
		"word/_rels/document.xml.rels": []byte(`<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
			<Relationship Id="rIdStyles" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
			<Relationship Id="rIdPic" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/image1.png"/></Relationships>`),
		"word/styles.xml": []byte(`<?xml version="1.0" encoding="UTF-8"?><w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
			<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/></w:style>
			<w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:basedOn w:val="Normal"/><w:pPr><w:outlineLvl w:val="0"/></w:pPr><w:rPr><w:b/></w:rPr></w:style></w:styles>`),
		"word/document.xml": []byte(`<?xml version="1.0" encoding="UTF-8"?><w:document ` + ns + `><w:body>` + body +
			`<w:sectPr><w:pgSz w:w="5954" w:h="8392"/><w:pgMar w:top="680" w:right="680" w:bottom="680" w:left="680" w:gutter="0"/></w:sectPr></w:body></w:document>`),
		"word/media/image1.png": pic.Bytes(),
	}
	return buildZip(t, files)
}

// Importing a .docx adds its chapters at the end of the book with their
// texts, saves its pictures in the uploads and leaves the book's settings.
func TestImportDocx(t *testing.T) {
	_, h, base, vol := editorSetup(t)
	first := addChapter(t, h, base, "Already here", vol)
	body, ct := form(t, map[string]string{"mode": "append"}, "file", "Rainy Book.docx", testDocx(t))
	rec := do(t, h, "POST", fmt.Sprintf("%s/books/%d/import-docx", base, vol), body, ct)
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	var out struct {
		Chapters      []importedChapter `json:"chapters"`
		Page          *bookPage         `json:"page"`
		PageTypeGuess string            `json:"page_type_guess"`
		Styles        []json.RawMessage `json:"styles"`
		Fonts         []string          `json:"fonts"`
		LeftOut       map[string]int    `json:"left_out"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, c := range out.Chapters {
		titles = append(titles, c.Title)
	}
	if strings.Join(titles, "|") != "Front matter|One|Two" || out.Chapters[1].Words < 6 {
		t.Fatalf("chapters: %+v", out.Chapters)
	}
	if out.Page == nil || out.PageTypeGuess == "" || out.Fonts == nil || out.LeftOut == nil {
		t.Fatalf("answer: %s", rec.Body.String())
	}
	book := getBook(t, h, base, vol)
	if len(book.Chapters) != 4 || book.Chapters[0].ID != first || book.Chapters[3].Title != "Two" || book.Chapters[2].Version != 1 {
		t.Fatalf("book: %+v", book.Chapters)
	}
	if book.IsSetUp {
		t.Fatal("an import leaves the settings alone")
	}
	text := getText(t, h, base, out.Chapters[1].ID)
	if !strings.Contains(string(text.Doc), "grey morning") || text.Words != out.Chapters[1].Words {
		t.Fatalf("text: %+v", text)
	}
	sid := strings.TrimPrefix(base, "/api/s/")
	i := strings.Index(string(text.Doc), `"/uploads/s/`+sid+`/editor/`)
	if i < 0 {
		t.Fatalf("picture: %s", text.Doc)
	}
	src := string(text.Doc[i+1:])
	src = src[:strings.IndexByte(src, '"')]
	pic := do(t, h, "GET", src, nil, "")
	if pic.Code != 200 || !bytes.HasPrefix(pic.Body.Bytes(), []byte("\x89PNG")) || pic.Header().Get("Content-Security-Policy") != "sandbox" {
		t.Fatalf("picture %s: %d", src, pic.Code)
	}
	// The imported text saves like any other.
	if code, _ := putText(t, h, base, out.Chapters[1].ID, string(text.Doc), 1); code != 200 {
		t.Fatalf("save imported text: %d", code)
	}

	// Into book 0, and bad uploads.
	body, ct = form(t, nil, "file", "x.docx", testDocx(t))
	if rec := do(t, h, "POST", base+"/books/0/import-docx", body, ct); rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	if n := len(getBook(t, h, base, 0).Chapters); n != 3 {
		t.Fatalf("book 0: %d chapters", n)
	}
	for name, c := range map[string]struct {
		fields map[string]string
		data   []byte
		want   int
	}{
		"not a zip": {nil, []byte("hello"), 400},
		"no doc":    {nil, buildZip(t, map[string][]byte{"a.txt": []byte("x")}), 400},
		"bad mode":  {map[string]string{"mode": "replace"}, testDocx(t), 400},
	} {
		body, ct := form(t, c.fields, "file", "x.docx", c.data)
		if rec := do(t, h, "POST", fmt.Sprintf("%s/books/%d/import-docx", base, vol), body, ct); rec.Code != c.want {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
	}
	body, ct = form(t, nil, "", "", nil)
	if rec := do(t, h, "POST", fmt.Sprintf("%s/books/%d/import-docx", base, vol), body, ct); rec.Code != 400 {
		t.Errorf("no file: %d", rec.Code)
	}
	body, ct = form(t, nil, "file", "x.docx", testDocx(t))
	if rec := do(t, h, "POST", base+"/books/999/import-docx", body, ct); rec.Code != 404 {
		t.Errorf("unknown book: %d", rec.Code)
	}
	if n := len(getBook(t, h, base, vol).Chapters); n != 4 {
		t.Fatalf("failed imports added chapters: %d", n)
	}
}

// A new volume can start with a copy of another's book settings, minus
// what belongs to the one book.
func TestCreateVolumeCopiesBookSettings(t *testing.T) {
	_, h, base, vol := editorSetup(t)
	ch := addChapter(t, h, base, "One", vol)
	code, _, body := putBook(t, h, base, vol, fmt.Sprintf(`{"settings":{"page_type":"a6",
		"meta":{"title":"Book One","subtitle":"Sub","author":"Me","isbn":"123","description":"d","publisher":"P"},
		"chapter_options":{"%d":{"front_matter":true}},"extra":{"k":1}}}`, ch))
	if code != 200 {
		t.Fatal(code, body)
	}
	create := func(payload string) (int, int64) {
		rec := do(t, h, "POST", base+"/volumes", strings.NewReader(payload), "application/json")
		var v struct{ ID int64 }
		json.Unmarshal(rec.Body.Bytes(), &v)
		return rec.Code, v.ID
	}
	settingsOf := func(id int64) (map[string]json.RawMessage, bool) {
		rec := do(t, h, "GET", fmt.Sprintf("%s/books/%d", base, id), nil, "")
		var b struct {
			Settings map[string]json.RawMessage
			IsSetUp  bool `json:"is_set_up"`
		}
		json.Unmarshal(rec.Body.Bytes(), &b)
		return b.Settings, b.IsSetUp
	}
	// Nothing is copied unless asked.
	code, plain := create(`{"title":"Two"}`)
	if _, set := settingsOf(plain); code != 200 || set {
		t.Fatal("no copy expected", code, set)
	}
	code, copy := create(fmt.Sprintf(`{"title":"Three","copy_settings_from":%d}`, vol))
	if code != 200 {
		t.Fatal(code)
	}
	m, set := settingsOf(copy)
	if !set || !strings.Contains(string(m["page_type"]), "a6") || string(m["extra"]) != `{"k":1}` {
		t.Fatalf("copy: %v %v", m, set)
	}
	if _, has := m["chapter_options"]; has {
		t.Fatal("chapter_options copied")
	}
	var meta map[string]string
	json.Unmarshal(m["meta"], &meta)
	if meta["author"] != "Me" || meta["publisher"] != "P" || meta["title"] != "" || meta["subtitle"] != "" || meta["isbn"] != "" || meta["description"] != "" {
		t.Fatalf("meta: %v", meta)
	}
	if code, _ := create(`{"title":"Four","copy_settings_from":9999}`); code != 400 {
		t.Fatalf("bad id: %d", code)
	}
	var idx chapterIndex
	json.Unmarshal(do(t, h, "GET", base+"/chapters", nil, "").Body.Bytes(), &idx)
	if n := len(idx.Volumes); n != 3 {
		t.Fatalf("a bad copy must not leave a volume: %d", n)
	}
}
