package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func listComments(t *testing.T, h http.Handler, base string, ch int64) []commentOut {
	t.Helper()
	rec := do(t, h, "GET", fmt.Sprintf("%s/chapters/%d/comments", base, ch), nil, "")
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	var out []commentOut
	json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

func addComment(t *testing.T, h http.Handler, base string, ch int64, body string) commentOut {
	t.Helper()
	rec := do(t, h, "POST", fmt.Sprintf("%s/chapters/%d/comments", base, ch), strings.NewReader(body), "application/json")
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	var c commentOut
	json.Unmarshal(rec.Body.Bytes(), &c)
	return c
}

// commentedDoc is a doc whose second word carries the comments given.
func commentedDoc(ids ...int64) string {
	var marks []string
	for _, id := range ids {
		marks = append(marks, fmt.Sprintf(`{"type":"comment","attrs":{"id":%d}}`, id))
	}
	mid := `{"type":"text","text":"middle"}`
	if len(marks) > 0 {
		mid = `{"type":"text","text":"middle","marks":[` + strings.Join(marks, ",") + `]}`
	}
	return `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"The "},` + mid + `,{"type":"text","text":" end."}]}]}`
}

// Schema 46 adds the comments' table; it goes with its chapter.
func TestMigrationAddsChapterComments(t *testing.T) {
	app, h, base, vol := editorSetup(t)
	one := addChapter(t, h, base, "One", vol)
	rollBackSchema46(t, app.db)
	if err := ensureSchema(app.db); err != nil {
		t.Fatal(err)
	}
	var v string
	app.db.QueryRow(`SELECT value FROM schema_meta WHERE key = 'schema_version'`).Scan(&v)
	if v != currentSchemaVersion {
		t.Fatalf("schema %s", v)
	}
	c := addComment(t, h, base, one, `{"text":"Check this"}`)
	addComment(t, h, base, one, fmt.Sprintf(`{"text":"Done","parent_id":%d}`, c.ID))
	if rec := do(t, h, "DELETE", fmt.Sprintf("%s/chapters/%d", base, one), nil, ""); rec.Code != 200 && rec.Code != 204 {
		t.Fatal("delete chapter", rec.Code)
	}
	var n int
	app.db.QueryRow(`SELECT COUNT(*) FROM chapter_comments`).Scan(&n)
	if n != 0 {
		t.Fatalf("%d comments left after their chapter went", n)
	}
}

func TestCommentRoutes(t *testing.T) {
	_, h, base, vol := editorSetup(t)
	one := addChapter(t, h, base, "One", vol)
	two := addChapter(t, h, base, "Two", vol)

	c := addComment(t, h, base, one, `{"text":"Is this clear?  \n"}`)
	if c.Text != "Is this clear?" || c.ChapterID != one || c.ParentID != nil || c.Resolved {
		t.Fatalf("new comment: %+v", c)
	}
	// An empty comment is allowed (the editor makes one, then the writer types).
	empty := addComment(t, h, base, one, `{"text":""}`)
	// A reply to a reply joins the thread.
	r1 := addComment(t, h, base, one, fmt.Sprintf(`{"text":"Yes","parent_id":%d}`, c.ID))
	r2 := addComment(t, h, base, one, fmt.Sprintf(`{"text":"Sure?","parent_id":%d}`, r1.ID))
	if r1.ParentID == nil || *r1.ParentID != c.ID || r2.ParentID == nil || *r2.ParentID != c.ID {
		t.Fatalf("replies: %+v %+v", r1, r2)
	}
	for _, bad := range []struct {
		method, url, body string
		want              int
	}{
		{"POST", fmt.Sprintf("/chapters/%d/comments", two), fmt.Sprintf(`{"text":"x","parent_id":%d}`, c.ID), 404}, // another chapter's
		{"POST", fmt.Sprintf("/chapters/%d/comments", one), fmt.Sprintf(`{"text":" ","parent_id":%d}`, c.ID), 400}, // an empty reply
		{"POST", fmt.Sprintf("/chapters/%d/comments", one), `{"text":"` + strings.Repeat("x", maxCommentText+1) + `"}`, 400},
		{"POST", "/chapters/99999/comments", `{"text":"x"}`, 404},
		{"GET", "/chapters/99999/comments", ``, 404},
		{"PUT", fmt.Sprintf("/comments/%d", r1.ID), `{"resolved":true}`, 400}, // a reply is resolved with its thread
		{"PUT", "/comments/99999", `{"text":"x"}`, 404},
		{"DELETE", "/comments/99999", ``, 404},
		{"POST", fmt.Sprintf("/chapters/%d/comments", one), `not json`, 400},
	} {
		var body *strings.Reader
		if bad.body != "" {
			body = strings.NewReader(bad.body)
		}
		var rec interface{ Result() *http.Response }
		if body != nil {
			rec = do(t, h, bad.method, base+bad.url, body, "application/json")
		} else {
			rec = do(t, h, bad.method, base+bad.url, nil, "")
		}
		if code := rec.Result().StatusCode; code != bad.want {
			t.Errorf("%s %s: %d, want %d", bad.method, bad.url, code, bad.want)
		}
	}

	// Edit, resolve and reopen.
	put := func(id int64, body string) commentOut {
		t.Helper()
		rec := do(t, h, "PUT", fmt.Sprintf("%s/comments/%d", base, id), strings.NewReader(body), "application/json")
		if rec.Code != 200 {
			t.Fatal(rec.Code, rec.Body.String())
		}
		var out commentOut
		json.Unmarshal(rec.Body.Bytes(), &out)
		return out
	}
	if got := put(empty.ID, `{"text":"Now written"}`); got.Text != "Now written" {
		t.Errorf("edit: %+v", got)
	}
	if got := put(c.ID, `{"resolved":true}`); !got.Resolved || got.ResolvedAt == nil || got.Text != "Is this clear?" {
		t.Errorf("resolve: %+v", got)
	}
	if got := put(c.ID, `{"resolved":false}`); got.Resolved || got.ResolvedAt != nil {
		t.Errorf("reopen: %+v", got)
	}
	if list := listComments(t, h, base, one); len(list) != 4 || list[0].ID != c.ID {
		t.Fatalf("list: %+v", list)
	}
	if list := listComments(t, h, base, two); len(list) != 0 {
		t.Fatalf("chapter two: %+v", list)
	}
	// Deleting a comment takes its replies.
	if rec := do(t, h, "DELETE", fmt.Sprintf("%s/comments/%d", base, c.ID), nil, ""); rec.Code != 204 {
		t.Fatal("delete", rec.Code)
	}
	if list := listComments(t, h, base, one); len(list) != 1 || list[0].ID != empty.ID {
		t.Fatalf("after delete: %+v", list)
	}
}

// Comment marks: several on one text with different ids; never the same
// id twice; an id is a positive number.
func TestCommentMarksInDocs(t *testing.T) {
	if _, err := parsePMDoc([]byte(commentedDoc(3, 4))); err != nil {
		t.Errorf("two comments on one text: %v", err)
	}
	for name, doc := range map[string]string{
		"same id twice": commentedDoc(3, 3),
		"id 0":          commentedDoc(0),
		"id text":       strings.Replace(commentedDoc(3), `"id":3`, `"id":"three"`, 1),
		"no id":         strings.Replace(commentedDoc(3), `{"id":3}`, `{}`, 1),
	} {
		if _, err := parsePMDoc([]byte(doc)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// Comments don't count as words or links.
	d, _ := parsePMDoc([]byte(commentedDoc(3, 4)))
	if docWords(d) != 3 || len(docStoryLinks(d)) != 0 {
		t.Errorf("words %d", docWords(d))
	}
}

func commentRow(t *testing.T, app *storyApp, id int64) (exists, anchored bool, detached string) {
	t.Helper()
	var det *string
	err := app.db.QueryRow(`SELECT anchored, detached_at FROM chapter_comments WHERE id = ?`, id).Scan(&anchored, &det)
	if err != nil {
		return false, false, ""
	}
	if det != nil {
		detached = *det
	}
	return true, anchored, detached
}

// The rows follow the text: anchored when saved with their mark, detached
// when the mark goes, back when it comes back (an undo), deleted a day
// after; kept while a saved version holds the mark.
func TestCommentLifecycle(t *testing.T) {
	app, h, base, vol := editorSetup(t)
	clock := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	editorNow = func() time.Time { return clock }
	t.Cleanup(func() { editorNow = time.Now })
	one := addChapter(t, h, base, "One", vol)
	version := int64(0)
	save := func(doc string) {
		t.Helper()
		code, out := putText(t, h, base, one, doc, version)
		if code != 200 {
			t.Fatalf("save: %d", code)
		}
		version = out.Version
	}
	save(docOf("Start."))

	a := addComment(t, h, base, one, `{"text":"a"}`)
	never := addComment(t, h, base, one, `{"text":"never placed"}`)
	reply := addComment(t, h, base, one, fmt.Sprintf(`{"text":"r","parent_id":%d}`, a.ID))

	// Saved with its mark: anchored. The one never placed is kept for now.
	save(commentedDoc(a.ID))
	if ok, anchored, det := commentRow(t, app, a.ID); !ok || !anchored || det != "" {
		t.Fatalf("after its mark was saved: %v %v %q", ok, anchored, det)
	}
	if ok, anchored, _ := commentRow(t, app, never.ID); !ok || anchored {
		t.Fatal("a comment not placed yet should stay")
	}

	// The text goes: detached, not deleted.
	clock = clock.Add(time.Minute)
	save(commentedDoc())
	if ok, _, det := commentRow(t, app, a.ID); !ok || det == "" {
		t.Fatalf("after its text went: %v %q", ok, det)
	}
	// Undo brings the mark back: attached again.
	clock = clock.Add(time.Minute)
	save(commentedDoc(a.ID))
	if _, _, det := commentRow(t, app, a.ID); det != "" {
		t.Fatalf("after an undo: detached %q", det)
	}
	// Gone for good: deleted (with its reply) at a save a day later; the
	// one never placed too.
	save(commentedDoc())
	clock = clock.Add(25 * time.Hour)
	save(docOf("Later."))
	for _, id := range []int64{a.ID, reply.ID, never.ID} {
		if ok, _, _ := commentRow(t, app, id); ok {
			t.Errorf("comment %d should be gone", id)
		}
	}

	// A comment whose mark a saved version holds is kept for a restore.
	b := addComment(t, h, base, one, `{"text":"b"}`)
	save(commentedDoc(b.ID))
	rec := do(t, h, "POST", fmt.Sprintf("%s/chapters/%d/text/versions", base, one), strings.NewReader(`{"label":"kept"}`), "application/json")
	if rec.Code != 200 {
		t.Fatal("version", rec.Code, rec.Body.String())
	}
	var v textVersionOut
	json.Unmarshal(rec.Body.Bytes(), &v)
	save(docOf("Rewritten."))
	clock = clock.Add(48 * time.Hour)
	save(docOf("Rewritten again."))
	if ok, _, det := commentRow(t, app, b.ID); !ok || det != "" {
		t.Fatalf("a comment in a version: %v %q", ok, det)
	}
	if rec := do(t, h, "POST", fmt.Sprintf("%s/chapters/%d/text/versions/%d/restore", base, one, v.ID), nil, ""); rec.Code != 200 {
		t.Fatal("restore", rec.Code)
	}
	if list := listComments(t, h, base, one); len(list) != 1 || list[0].ID != b.ID {
		t.Fatalf("after restore: %+v", list)
	}
	// A mark pointing at a deleted comment is harmless.
	version = getText(t, h, base, one).Version
	save(commentedDoc(b.ID, 987654))
}
