package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func saveReference(t *testing.T, h http.Handler, method, url string, fields map[string]string, pic []byte) (referenceOut, int, string) {
	t.Helper()
	field := ""
	if pic != nil {
		field = "pictures"
	}
	body, ctype := form(t, fields, field, "look.png", pic)
	rec := do(t, h, method, url, body, ctype)
	var out referenceOut
	json.Unmarshal(rec.Body.Bytes(), &out)
	return out, rec.Code, rec.Body.String()
}

// The library is shared by every story: added, changed, its pictures
// served sandboxed and removed with it; links must be web addresses.
func TestReferenceLibrary(t *testing.T) {
	mgr, h := newTestServer(t)
	ref, code, body := saveReference(t, h, "POST", "/api/references", map[string]string{
		"title": "Oath of the Marshal", "source_url": "https://x.com/someone/status/1",
		"description": "White and blue coat", "tags": `["MilitaryLolita", "#ouji", "ouji", " "]`,
	}, tinyPNG(t))
	if code != 200 {
		t.Fatal(code, body)
	}
	if ref.Title != "Oath of the Marshal" || ref.Kind != "clothing" || len(ref.Pictures) != 1 ||
		strings.Join(ref.Tags, ",") != "MilitaryLolita,ouji" {
		t.Fatalf("saved %+v", ref)
	}

	rec := do(t, h, "GET", ref.Pictures[0].URL, nil, "")
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/png" ||
		rec.Header().Get("Content-Security-Policy") != "sandbox" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("picture served as %d %v", rec.Code, rec.Header())
	}

	for _, bad := range []string{"javascript:alert(1)", "ftp://x/y", "not a link"} {
		if _, code, _ := saveReference(t, h, "POST", "/api/references", map[string]string{"title": "x", "source_url": bad}, nil); code != 400 {
			t.Errorf("source %q: %d", bad, code)
		}
	}
	if _, code, _ := saveReference(t, h, "POST", "/api/references", map[string]string{"title": "x"}, []byte("<html>no picture</html>")); code != 400 {
		t.Errorf("a page passed as a picture: %d", code)
	}

	// A second picture, then the first one dropped.
	url := fmt.Sprintf("/api/references/%d", ref.ID)
	ref2, code, body := saveReference(t, h, "PUT", url, map[string]string{"title": "Marshal", "tags": `[]`}, tinyPNG(t))
	if code != 200 || len(ref2.Pictures) != 2 || len(ref2.Tags) != 0 || ref2.Title != "Marshal" {
		t.Fatal(code, body)
	}
	first := filepath.Join(mgr.refs.dir, strings.TrimPrefix(ref.Pictures[0].URL, "/uploads/references/"))
	ref3, code, _ := saveReference(t, h, "PUT", url, map[string]string{
		"title": "Marshal", "remove_pictures": fmt.Sprintf("[%d]", ref.Pictures[0].ID),
	}, nil)
	if code != 200 || len(ref3.Pictures) != 1 {
		t.Fatalf("after removing: %d %+v", code, ref3)
	}
	if _, err := os.Stat(first); !os.IsNotExist(err) {
		t.Error("the removed picture's file is still there")
	}

	// Every story sees the same library.
	createStoryDirect(t, mgr, "Another")
	rec = do(t, h, "GET", "/api/references", nil, "")
	var list []referenceOut
	json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("library holds %d", len(list))
	}

	if rec := do(t, h, "DELETE", url, nil, ""); rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	left, _ := os.ReadDir(mgr.refs.dir)
	if len(left) != 0 {
		t.Errorf("%d files left after the reference went", len(left))
	}
}

// A culture with kingdoms and a clothing design based on a reference; the
// library says which designs use it; deleting the culture cleans up.
func TestCulturesAPI(t *testing.T) {
	mgr, h := newTestServer(t)
	id := createStoryDirect(t, mgr, "World")
	app, _ := mgr.open(id)
	app.db.Exec(`INSERT INTO locations (id, name, kind, color) VALUES (1, 'Aurel', 'major', '#aa3344')`)
	app.db.Exec(`INSERT INTO locations (id, name, kind) VALUES (2, 'Town', 'minor')`)
	base := "/api/s/" + id

	body, ctype := form(t, map[string]string{"name": "Aureli", "description": "Proud people of [[Aurel]]", "kingdom_ids": "[1, 2, 99]"}, "", "", nil)
	rec := do(t, h, "POST", base+"/cultures", body, ctype)
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	var c cultureOut
	json.Unmarshal(rec.Body.Bytes(), &c)
	if len(c.Kingdoms) != 1 || c.Kingdoms[0].ID != 1 {
		t.Fatalf("kingdoms %+v (only a coloured major location is a kingdom)", c.Kingdoms)
	}
	if c.Description != "Proud people of [[location#1|Aurel]]" && c.Description != "Proud people of [[location#1]]" {
		t.Errorf("description stored as %q", c.Description)
	}

	ref, code, _ := saveReference(t, h, "POST", "/api/references", map[string]string{"title": "Marshal"}, tinyPNG(t))
	if code != 200 {
		t.Fatal(code)
	}
	body, ctype = form(t, map[string]string{
		"kind": "clothing", "name": "Marshal coat", "description": "Worn at court",
		"tags": `["formal", "military"]`, "reference_ids": fmt.Sprintf("[%d]", ref.ID),
	}, "pictures", "coat.png", tinyPNG(t))
	rec = do(t, h, "POST", fmt.Sprintf("%s/cultures/%d/elements", base, c.ID), body, ctype)
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	var e cultureElementOut
	json.Unmarshal(rec.Body.Bytes(), &e)
	if e.Kind != "clothing" || len(e.Pictures) != 1 || len(e.Tags) != 2 || len(e.ReferenceIDs) != 1 || e.CultureName != "Aureli" {
		t.Fatalf("element %+v", e)
	}
	if !strings.HasPrefix(e.Pictures[0].URL, "/uploads/s/"+id+"/cultures/") {
		t.Errorf("picture address %q", e.Pictures[0].URL)
	}

	body, ctype = form(t, map[string]string{"kind": "shoes", "name": "x"}, "", "", nil)
	if rec := do(t, h, "PUT", fmt.Sprintf("%s/culture-elements/%d", base, e.ID), body, ctype); rec.Code != 400 {
		t.Errorf("unknown kind: %d", rec.Code)
	}

	rec = do(t, h, "GET", "/api/references/used", nil, "")
	var used map[string][]referenceUse
	json.Unmarshal(rec.Body.Bytes(), &used)
	if u := used[fmt.Sprint(ref.ID)]; len(u) != 1 || u[0].Element != "Marshal coat" || u[0].StoryID != id {
		t.Fatalf("used %+v", used)
	}

	rec = do(t, h, "GET", fmt.Sprintf("%s/cultures/%d", base, c.ID), nil, "")
	json.Unmarshal(rec.Body.Bytes(), &c)
	if len(c.Elements) != 1 || c.ElementCount != 1 {
		t.Fatalf("culture %+v", c)
	}

	// The wiki: the culture's article lists its kingdom, the kingdom its culture.
	rec = do(t, h, "GET", base+"/wiki/location/1", nil, "")
	if !strings.Contains(rec.Body.String(), `"cultures_here"`) {
		t.Errorf("the kingdom's article doesn't list its cultures: %s", rec.Body.String())
	}

	if rec := do(t, h, "DELETE", fmt.Sprintf("%s/cultures/%d", base, c.ID), nil, ""); rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	left, _ := os.ReadDir(filepath.Join(mgr.uploadsFor(id), "cultures"))
	if len(left) != 0 {
		t.Errorf("%d pictures left after the culture went", len(left))
	}
}
