package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// A doc with links, a link split by formatting, and dialogue.
const scriptSample = `{"type":"doc","attrs":{"mode":"script"},"content":[
 {"type":"paragraph","content":[
   {"type":"text","text":"Mi","marks":[{"type":"story_link","attrs":{"type":"character","id":12}}]},
   {"type":"text","text":"ra","marks":[{"type":"story_link","attrs":{"type":"character","id":12}},{"type":"bold"}]},
   {"type":"text","text":" walked to "},
   {"type":"text","text":"Ilvar","marks":[{"type":"story_link","attrs":{"type":"location","id":7}}]},
   {"type":"text","text":" with "},
   {"type":"text","text":"Mira","marks":[{"type":"story_link","attrs":{"type":"character","id":12}}]}
 ]},
 {"type":"dialogue","attrs":{"character":12,"speaker":"Mira","manner":"softly"},"content":[{"type":"text","text":"Come here, now."}]},
 {"type":"dialogue","attrs":{"character":null,"speaker":"A guard","manner":""},"content":[{"type":"text","text":"Halt!"}]},
 {"type":"dialogue","attrs":{"character":"12","speaker":"Mira"},"content":[{"type":"text","text":"No. "},{"type":"text","text":"Ilvar","marks":[{"type":"story_link","attrs":{"type":"location","id":7}}]},{"type":"text","text":" waits."}]}
]}`

func TestDocLinkCountsAndDialogues(t *testing.T) {
	doc, err := parsePMDoc([]byte(scriptSample))
	if err != nil {
		t.Fatal(err)
	}
	got := docLinkCounts(doc)
	want := []pmLinkCount{{pmStoryLink{"character", 12}, 4}, {pmStoryLink{"location", 7}, 2}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("docLinkCounts = %v, want %v", got, want)
	}
	lines := docDialogues(doc)
	if len(lines) != 3 || lines[0] != (pmDialogue{12, "Mira", "softly", "Come here, now."}) ||
		lines[1] != (pmDialogue{0, "A guard", "", "Halt!"}) || lines[2].Character != 12 || lines[2].Text != "No. Ilvar waits." {
		t.Errorf("docDialogues = %+v", lines)
	}
	// Only the spoken lines count as words.
	if n := docWords(doc); n != 6+3+1+3 {
		t.Errorf("docWords = %d", n)
	}
	if text := docText(doc); !strings.Contains(text, "\nMira, softly: Come here, now.\nA guard: Halt!") {
		t.Errorf("docText = %q", text)
	}
}

func TestPMDocDialogueChecks(t *testing.T) {
	bad := map[string]string{
		"bad mode":       `{"type":"doc","attrs":{"mode":"poem"},"content":[{"type":"paragraph"}]}`,
		"character 0":    `{"type":"doc","content":[{"type":"dialogue","attrs":{"character":0}}]}`,
		"character text": `{"type":"doc","content":[{"type":"dialogue","attrs":{"character":"Mira"}}]}`,
		"long speaker":   `{"type":"doc","content":[{"type":"dialogue","attrs":{"speaker":"` + strings.Repeat("x", 201) + `"}}]}`,
		"long manner":    `{"type":"doc","content":[{"type":"dialogue","attrs":{"manner":"` + strings.Repeat("x", 201) + `"}}]}`,
		"block inside":   `{"type":"doc","content":[{"type":"dialogue","content":[{"type":"paragraph"}]}]}`,
	}
	for name, src := range bad {
		if _, err := parsePMDoc([]byte(src)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	for _, ok := range []string{
		`{"type":"doc","attrs":{"mode":"prose"},"content":[{"type":"dialogue"}]}`,
		`{"type":"doc","content":[{"type":"dialogue","attrs":{"character":null,"speaker":"","manner":""}}]}`,
		`{"type":"doc","content":[{"type":"bullet_list","content":[{"type":"list_item","content":[{"type":"paragraph"},{"type":"dialogue","attrs":{"speaker":"A"}}]}]}]}`,
	} {
		if _, err := parsePMDoc([]byte(ok)); err != nil {
			t.Errorf("%s: %v", ok, err)
		}
	}
}

type appearsJSON struct {
	ChapterID   int64  `json:"chapter_id"`
	Title       string `json:"title"`
	Number      int    `json:"number"`
	VolumeID    int64  `json:"volume_id"`
	VolumeTitle string `json:"volume_title"`
	Count       int    `json:"count"`
	Position    int    `json:"position"`
	Speaker     string `json:"speaker"`
	Manner      string `json:"manner"`
	Text        string `json:"text"`
}

// Saves, restores and deletes keep the index right; the readers give the
// chapters in reading order.
func TestChapterIndexFollowsTheTexts(t *testing.T) {
	app, h, base, vol := editorSetup(t)
	ch := app.db.Exec
	res, _ := ch(`INSERT INTO characters DEFAULT VALUES`)
	mira, _ := res.LastInsertId()
	if _, err := ch(`INSERT INTO character_versions (character_id, name, is_current) VALUES (?, 'Mira Vey', 1)`, mira); err != nil {
		t.Fatal(err)
	}
	doc := strings.ReplaceAll(scriptSample, `"id":12`, fmt.Sprintf(`"id":%d`, mira))
	doc = strings.ReplaceAll(doc, `"character":12`, fmt.Sprintf(`"character":%d`, mira))
	doc = strings.ReplaceAll(doc, `"character":"12"`, fmt.Sprintf(`"character":"%d"`, mira))

	two := addChapter(t, h, base, "Two", vol)
	one := addChapter(t, h, base, "One", vol)
	// "One" goes first.
	if rec := do(t, h, "PUT", base+"/chapters/order", strings.NewReader(fmt.Sprintf(`{"volumes":[%d],"chapters":[{"id":%d,"volume_id":%d},{"id":%d,"volume_id":%d}]}`, vol, one, vol, two, vol)), "application/json"); rec.Code != 200 {
		t.Fatalf("reorder: %d %s", rec.Code, rec.Body.String())
	}
	if code, _ := putText(t, h, base, two, doc, 0); code != 200 {
		t.Fatal("save two", code)
	}
	plain := fmt.Sprintf(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Mira","marks":[{"type":"story_link","attrs":{"type":"character","id":%d}}]}]}]}`, mira)
	if code, _ := putText(t, h, base, one, plain, 0); code != 200 {
		t.Fatal("save one", code)
	}

	list := func(url string) []appearsJSON {
		t.Helper()
		rec := do(t, h, "GET", base+url, nil, "")
		if rec.Code != 200 {
			t.Fatal(url, rec.Code, rec.Body.String())
		}
		var out []appearsJSON
		json.Unmarshal(rec.Body.Bytes(), &out)
		return out
	}
	appears := list(fmt.Sprintf("/appears/character/%d", mira))
	if len(appears) != 2 || appears[0].ChapterID != one || appears[0].Count != 1 || appears[1].ChapterID != two || appears[1].Count != 4 || appears[1].VolumeTitle != "Book One" {
		t.Fatalf("appears in: %+v", appears)
	}
	if loc := list("/appears/location/7"); len(loc) != 1 || loc[0].Count != 2 {
		t.Fatalf("location: %+v", loc)
	}
	lines := list(fmt.Sprintf("/characters/%d/lines", mira))
	if len(lines) != 2 || lines[0].Position != 0 || lines[0].Manner != "softly" || lines[0].Text != "Come here, now." || lines[1].Position != 2 || lines[1].ChapterID != two {
		t.Fatalf("lines: %+v", lines)
	}
	rec := do(t, h, "GET", fmt.Sprintf("%s/chapters/%d/speakers", base, two), nil, "")
	var speakers []speakerRow
	json.Unmarshal(rec.Body.Bytes(), &speakers)
	if len(speakers) != 2 || speakers[0].Name != "Mira Vey" || speakers[0].Lines != 2 || speakers[0].Words != 6 || speakers[1].Name != "A guard" || speakers[1].CharacterID != nil {
		t.Fatalf("speakers: %s", rec.Body.String())
	}

	// A new save replaces the chapter's rows; a restore brings them back.
	if code, _ := putText(t, h, base, two, docOf("Nobody here."), 1); code != 200 {
		t.Fatal("resave", code)
	}
	if got := list(fmt.Sprintf("/characters/%d/lines", mira)); len(got) != 0 {
		t.Fatalf("lines after resave: %+v", got)
	}
	vrec := do(t, h, "POST", fmt.Sprintf("%s/chapters/%d/text/versions", base, two), strings.NewReader(`{"label":"old","doc":`+doc+`}`), "application/json")
	var v textVersionOut
	json.Unmarshal(vrec.Body.Bytes(), &v)
	if rr := do(t, h, "POST", fmt.Sprintf("%s/chapters/%d/text/versions/%d/restore", base, two, v.ID), nil, ""); rr.Code != 200 {
		t.Fatal("restore", rr.Code, rr.Body.String())
	}
	if got := list(fmt.Sprintf("/characters/%d/lines", mira)); len(got) != 2 {
		t.Fatalf("lines after restore: %+v", got)
	}

	// A deleted chapter takes its rows with it.
	if rr := do(t, h, "DELETE", fmt.Sprintf("%s/chapters/%d", base, two), nil, ""); rr.Code != 200 && rr.Code != 204 {
		t.Fatal("delete", rr.Code)
	}
	var n int
	app.db.QueryRow(`SELECT (SELECT COUNT(*) FROM chapter_links WHERE chapter_id = ?) + (SELECT COUNT(*) FROM chapter_lines WHERE chapter_id = ?)`, two, two).Scan(&n)
	if n != 0 {
		t.Fatalf("%d rows left for a deleted chapter", n)
	}
	if got := list(fmt.Sprintf("/appears/character/%d", mira)); len(got) != 1 || got[0].ChapterID != one {
		t.Fatalf("appears after delete: %+v", got)
	}
}

// Schema 45 adds the index and fills it from the texts already written.
func TestMigrationIndexesChapterTexts(t *testing.T) {
	app, h, base, vol := editorSetup(t)
	one := addChapter(t, h, base, "One", vol)
	if code, _ := putText(t, h, base, one, scriptSample, 0); code != 200 {
		t.Fatal("save", code)
	}
	rollBackSchema45(t, app.db)
	if err := ensureSchema(app.db); err != nil {
		t.Fatal(err)
	}
	var v string
	app.db.QueryRow(`SELECT value FROM schema_meta WHERE key = 'schema_version'`).Scan(&v)
	if v != currentSchemaVersion {
		t.Fatalf("schema %s", v)
	}
	var links, lines int
	app.db.QueryRow(`SELECT COUNT(*) FROM chapter_links WHERE chapter_id = ?`, one).Scan(&links)
	app.db.QueryRow(`SELECT COUNT(*) FROM chapter_lines WHERE chapter_id = ?`, one).Scan(&lines)
	if links != 2 || lines != 3 {
		t.Fatalf("index after migration: %d links, %d lines", links, lines)
	}
}
