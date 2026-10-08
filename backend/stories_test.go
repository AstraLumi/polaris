package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func newTestServer(t *testing.T) (*storyManager, http.Handler) {
	t.Helper()
	dir := t.TempDir()
	mgr, err := newStoryManager(filepath.Join(dir, "data"), filepath.Join(dir, "uploads"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mgr.Close)
	static := fstest.MapFS{"index.html": {Data: []byte("<html>spa</html>")}}
	return mgr, newRouter(mgr, static)
}

func do(t *testing.T, h http.Handler, method, url string, body io.Reader, ctype string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, url, body)
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func form(t *testing.T, fields map[string]string, fileField, fileName string, fileData []byte) (io.Reader, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		mw.WriteField(k, v)
	}
	if fileField != "" {
		fw, _ := mw.CreateFormFile(fileField, fileName)
		fw.Write(fileData)
	}
	mw.Close()
	return &buf, mw.FormDataContentType()
}

func createStory(t *testing.T, h http.Handler, fields map[string]string) storyInfo {
	t.Helper()
	body, ct := form(t, fields, "", "", nil)
	rec := do(t, h, "POST", "/api/stories", body, ct)
	if rec.Code != 200 {
		t.Fatalf("create story: %d %s", rec.Code, rec.Body.String())
	}
	var si storyInfo
	json.Unmarshal(rec.Body.Bytes(), &si)
	return si
}

func addClass(t *testing.T, h http.Handler, id, name string) int64 {
	t.Helper()
	rec := do(t, h, "POST", "/api/s/"+id+"/classes", strings.NewReader(`{"name":"`+name+`","modifiers":{"vit":2}}`), "application/json")
	if rec.Code != 200 {
		t.Fatalf("add class: %d %s", rec.Code, rec.Body.String())
	}
	var out struct{ ID int64 }
	json.Unmarshal(rec.Body.Bytes(), &out)
	return out.ID
}

func listNames(t *testing.T, h http.Handler, id, what string) []string {
	t.Helper()
	rec := do(t, h, "GET", "/api/s/"+id+"/"+what, nil, "")
	if rec.Code != 200 {
		t.Fatalf("list %s: %d %s", what, rec.Code, rec.Body.String())
	}
	var rows []struct{ Name string }
	json.Unmarshal(rec.Body.Bytes(), &rows)
	var names []string
	for _, r := range rows {
		names = append(names, r.Name)
	}
	return names
}

func TestEveryTableIsClassified(t *testing.T) {
	mgr, _ := newTestServer(t)
	si := createStoryDirect(t, mgr, "x")
	app, _ := mgr.open(si)
	known := map[string]bool{"schema_meta": true, "sqlite_sequence": true}
	assets := map[string]assetTable{}
	for _, a := range assetTables {
		known[a.name] = true
		assets[a.name] = a
	}
	for _, n := range storyOnlyTables {
		if known[n] {
			t.Errorf("%s is listed as both asset and story-only", n)
		}
		known[n] = true
	}
	rows, err := app.db.Query(`SELECT name FROM sqlite_master WHERE type = 'table'`)
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var n string
		rows.Scan(&n)
		tables = append(tables, n)
	}
	rows.Close()
	for _, n := range tables {
		if !known[n] {
			t.Errorf("table %q is not classified: add it to assetTables or storyOnlyTables in storyassets.go", n)
		}
	}
	// Asset tables may only reference other asset tables (or the column must
	// be nulled in the copy), and must come after what they reference.
	seen := map[string]bool{}
	for _, a := range assetTables {
		fk, err := app.db.Query(`PRAGMA foreign_key_list(` + quoteIdent(a.name) + `)`)
		if err != nil {
			t.Fatal(err)
		}
		for fk.Next() {
			var id, seq int
			var table, from, to, onUpdate, onDelete, match string
			fk.Scan(&id, &seq, &table, &from, &to, &onUpdate, &onDelete, &match)
			if isNullCol(a, from) {
				continue
			}
			if _, ok := assets[table]; !ok {
				t.Errorf("%s.%s references %s, which isn't copied: add %q to nullCols or copy %s", a.name, from, table, from, table)
			} else if !seen[table] && table != a.name {
				t.Errorf("%s must be listed after %s in assetTables", a.name, table)
			}
		}
		fk.Close()
		seen[a.name] = true
	}
}

func createStoryDirect(t *testing.T, mgr *storyManager, name string) string {
	t.Helper()
	si, err := mgr.create(name, nil, "", "")
	if err != nil {
		t.Fatal(err)
	}
	return si.ID
}

func TestStoriesAreIsolated(t *testing.T) {
	_, h := newTestServer(t)
	a := createStory(t, h, map[string]string{"name": "Alpha", "icon_choice": "builtin:castle"})
	b := createStory(t, h, map[string]string{"name": "Beta"})
	if a.Icon != "builtin:castle" || b.Icon != "" {
		t.Fatalf("icons: %q %q", a.Icon, b.Icon)
	}
	addClass(t, h, a.ID, "Mage")
	if got := listNames(t, h, a.ID, "classes"); len(got) != 1 || got[0] != "Mage" {
		t.Fatalf("alpha classes: %v", got)
	}
	if got := listNames(t, h, b.ID, "classes"); len(got) != 0 {
		t.Fatalf("beta should be empty, got %v", got)
	}
	if rec := do(t, h, "GET", "/api/s/0123456789abcdef/classes", nil, ""); rec.Code != 404 {
		t.Fatalf("unknown story: %d", rec.Code)
	}
	if rec := do(t, h, "GET", "/api/s/..%2f..%2fetc/classes", nil, ""); rec.Code != 404 {
		t.Fatalf("bad id: %d", rec.Code)
	}
	rec := do(t, h, "GET", "/api/stories", nil, "")
	var list []storyInfo
	json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 2 || list[0].Name != "Alpha" {
		t.Fatalf("list: %s", rec.Body.String())
	}
	if rec := do(t, h, "POST", "/api/stories", strings.NewReader(""), ""); rec.Code != 400 {
		t.Fatalf("missing name should be rejected: %d", rec.Code)
	}
}

func uploadClassIcon(t *testing.T, h http.Handler, id string, classID int64, data []byte) string {
	t.Helper()
	body, ct := form(t, nil, "icon", "i.png", data)
	rec := do(t, h, "PUT", "/api/s/"+id+"/classes/"+idStr(classID)+"/icon", body, ct)
	if rec.Code != 200 {
		t.Fatalf("icon: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		IconPath string `json:"icon_path"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	return out.IconPath
}

func idStr(n int64) string { b, _ := json.Marshal(n); return string(b) }

func TestUploadsAreStoryScoped(t *testing.T) {
	_, h := newTestServer(t)
	a := createStory(t, h, map[string]string{"name": "Alpha"})
	b := createStory(t, h, map[string]string{"name": "Beta"})
	cid := addClass(t, h, a.ID, "Mage")
	url := uploadClassIcon(t, h, a.ID, cid, []byte("PNGDATA"))
	if !strings.HasPrefix(url, "/uploads/s/"+a.ID+"/classes/") {
		t.Fatalf("url not story scoped: %s", url)
	}
	rec := do(t, h, "GET", url, nil, "")
	if rec.Code != 200 || rec.Body.String() != "PNGDATA" {
		t.Fatalf("fetch icon: %d %q", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Security-Policy") != "sandbox" {
		t.Fatal("uploads must be sandboxed")
	}
	other := strings.Replace(url, a.ID, b.ID, 1)
	if rec := do(t, h, "GET", other, nil, ""); rec.Code != 404 {
		t.Fatalf("other story must not serve it: %d", rec.Code)
	}
	if rec := do(t, h, "GET", "/uploads/s/"+a.ID+"/", nil, ""); rec.Code != 404 {
		t.Fatalf("no directory listing: %d", rec.Code)
	}
	// The listing endpoint rewrites too.
	rec = do(t, h, "GET", "/api/s/"+a.ID+"/classes", nil, "")
	if !strings.Contains(rec.Body.String(), `"/uploads/s/`+a.ID+`/classes/`) {
		t.Fatalf("list not rewritten: %s", rec.Body.String())
	}
}

func TestCopyAssets(t *testing.T) {
	mgr, h := newTestServer(t)
	a := createStory(t, h, map[string]string{"name": "Alpha"})
	cid := addClass(t, h, a.ID, "Mage")
	uploadClassIcon(t, h, a.ID, cid, []byte("ICON"))
	addClass(t, h, a.ID, "Knight")
	app, _ := mgr.open(a.ID)
	app.db.Exec(`INSERT INTO locations (name) VALUES ('Greyhold')`)
	app.db.Exec(`INSERT INTO characters DEFAULT VALUES`)
	app.db.Exec(`INSERT INTO spells (name, level, source_type, source_id, origin_location_id) VALUES ('Fireball', 3, 'class', ?, 1)`, cid)
	app.db.Exec(`INSERT INTO spells (name, icon_path) VALUES ('Spark', 'builtin:bolt')`)

	b := createStory(t, h, map[string]string{"name": "Beta", "copy_assets_from": a.ID})
	if got := listNames(t, h, b.ID, "classes"); len(got) != 2 {
		t.Fatalf("copied classes: %v", got)
	}
	bapp, _ := mgr.open(b.ID)
	var origin *int64
	var src int64
	if err := bapp.db.QueryRow(`SELECT origin_location_id, source_id FROM spells WHERE name = 'Fireball'`).Scan(&origin, &src); err != nil {
		t.Fatal(err)
	}
	if origin != nil || src != cid {
		t.Fatalf("origin should be cleared and source kept: origin=%v src=%d", origin, src)
	}
	var n int
	bapp.db.QueryRow(`SELECT COUNT(*) FROM characters`).Scan(&n)
	if n != 0 {
		t.Fatal("characters must not be copied")
	}
	bapp.db.QueryRow(`SELECT COUNT(*) FROM locations`).Scan(&n)
	if n != 0 {
		t.Fatal("locations must not be copied")
	}
	bapp.db.QueryRow(`SELECT COUNT(*) FROM stat_modifiers`).Scan(&n)
	if n == 0 {
		t.Fatal("modifiers should be copied")
	}
	var icon string
	bapp.db.QueryRow(`SELECT icon_path FROM classes WHERE name = 'Mage'`).Scan(&icon)
	if data, err := os.ReadFile(filepath.Join(bapp.uploadsDir, filepath.FromSlash(icon))); err != nil || string(data) != "ICON" {
		t.Fatalf("icon file not copied: %v %q", err, data)
	}
	// Independent afterwards: deleting in Beta leaves Alpha alone.
	do(t, h, "DELETE", "/api/s/"+b.ID+"/classes/"+idStr(cid), nil, "")
	if _, err := os.Stat(filepath.Join(app.uploadsDir, filepath.FromSlash(icon))); err != nil {
		t.Fatal("alpha's icon was removed by beta")
	}
	if got := listNames(t, h, a.ID, "classes"); len(got) != 2 {
		t.Fatalf("alpha changed: %v", got)
	}
	// New rows after a copy must not collide with copied ids.
	addClass(t, h, b.ID, "Rogue")
}

func TestDeleteNeedsTypedName(t *testing.T) {
	mgr, h := newTestServer(t)
	a := createStory(t, h, map[string]string{"name": "Alpha"})
	if rec := do(t, h, "DELETE", "/api/stories/"+a.ID+"?confirm=alpha", nil, ""); rec.Code != 400 {
		t.Fatalf("wrong confirmation must be refused: %d", rec.Code)
	}
	if rec := do(t, h, "DELETE", "/api/stories/"+a.ID, nil, ""); rec.Code != 400 {
		t.Fatalf("missing confirmation must be refused: %d", rec.Code)
	}
	if rec := do(t, h, "DELETE", "/api/stories/"+a.ID+"?confirm=Alpha", nil, ""); rec.Code != 200 {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(mgr.storyDir(a.ID)); !os.IsNotExist(err) {
		t.Fatal("data folder should be gone")
	}
	if _, err := os.Stat(mgr.uploadsFor(a.ID)); !os.IsNotExist(err) {
		t.Fatal("uploads folder should be gone")
	}
	if rec := do(t, h, "GET", "/api/s/"+a.ID+"/classes", nil, ""); rec.Code != 404 {
		t.Fatalf("deleted story must 404: %d", rec.Code)
	}
}

func TestUpdateStory(t *testing.T) {
	_, h := newTestServer(t)
	a := createStory(t, h, map[string]string{"name": "Alpha"})
	body, ct := form(t, map[string]string{"name": "Alpha 2"}, "icon", "pic.png", []byte("X"))
	rec := do(t, h, "PUT", "/api/stories/"+a.ID, body, ct)
	var si storyInfo
	json.Unmarshal(rec.Body.Bytes(), &si)
	if rec.Code != 200 || si.Name != "Alpha 2" || !strings.HasPrefix(si.Icon, "/uploads/s/"+a.ID+"/_story/") {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, h, "GET", si.Icon, nil, ""); rec.Code != 200 {
		t.Fatalf("icon not served: %d", rec.Code)
	}
	body, ct = form(t, map[string]string{"icon_choice": "builtin:moon"}, "", "", nil)
	rec = do(t, h, "PUT", "/api/stories/"+a.ID, body, ct)
	json.Unmarshal(rec.Body.Bytes(), &si)
	if si.Icon != "builtin:moon" || si.Name != "Alpha 2" {
		t.Fatalf("switch to builtin: %s", rec.Body.String())
	}
	body, ct = form(t, map[string]string{"icon_choice": "builtin:not-real"}, "", "", nil)
	rec = do(t, h, "PUT", "/api/stories/"+a.ID, body, ct)
	json.Unmarshal(rec.Body.Bytes(), &si)
	if si.Icon != "builtin:moon" {
		t.Fatalf("unknown builtin must be ignored: %s", si.Icon)
	}
	body, ct = form(t, nil, "icon", "evil.html", []byte("<script>"))
	if rec := do(t, h, "PUT", "/api/stories/"+a.ID, body, ct); rec.Code != 400 {
		t.Fatalf("non-image icon must be refused: %d", rec.Code)
	}
}

func exportZip(t *testing.T, h http.Handler, id string) []byte {
	t.Helper()
	rec := do(t, h, "GET", "/api/stories/"+id+"/export", nil, "")
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Disposition"), ".zip") {
		t.Fatalf("export: %d %s", rec.Code, rec.Header())
	}
	return rec.Body.Bytes()
}

func importZipBytes(t *testing.T, h http.Handler, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	body, ct := form(t, nil, "file", "story.zip", data)
	return do(t, h, "POST", "/api/stories/import", body, ct)
}

func TestExportImportRoundTrip(t *testing.T) {
	mgr, h := newTestServer(t)
	a := createStory(t, h, map[string]string{"name": "Alpha"})
	cid := addClass(t, h, a.ID, "Mage")
	uploadClassIcon(t, h, a.ID, cid, []byte("ICON"))
	app, _ := mgr.open(a.ID)
	app.db.Exec(`INSERT INTO characters DEFAULT VALUES`)
	body, ct := form(t, nil, "icon", "story.png", []byte("STORYICON"))
	do(t, h, "PUT", "/api/stories/"+a.ID, body, ct)

	data := exportZip(t, h, a.ID)
	rec := importZipBytes(t, h, data)
	if rec.Code != 200 {
		t.Fatalf("import: %d %s", rec.Code, rec.Body.String())
	}
	var si storyInfo
	json.Unmarshal(rec.Body.Bytes(), &si)
	if si.ID == a.ID || si.Name != "Alpha (imported)" || si.Characters != 1 {
		t.Fatalf("imported: %+v", si)
	}
	if got := listNames(t, h, si.ID, "classes"); len(got) != 1 || got[0] != "Mage" {
		t.Fatalf("imported classes: %v", got)
	}
	if r := do(t, h, "GET", si.Icon, nil, ""); r.Code != 200 || r.Body.String() != "STORYICON" {
		t.Fatalf("story icon: %d %q (%s)", r.Code, r.Body.String(), si.Icon)
	}
	rec = do(t, h, "GET", "/api/s/"+si.ID+"/classes", nil, "")
	var rows []struct {
		IconPath string `json:"icon_path"`
	}
	json.Unmarshal(rec.Body.Bytes(), &rows)
	if r := do(t, h, "GET", rows[0].IconPath, nil, ""); r.Body.String() != "ICON" {
		t.Fatalf("class icon after import: %q", r.Body.String())
	}
	if !strings.Contains(rows[0].IconPath, si.ID) {
		t.Fatal("imported story's files must live under its own id")
	}
}

func buildZip(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range files {
		w, _ := zw.Create(name)
		w.Write(data)
	}
	zw.Close()
	return buf.Bytes()
}

func TestImportRejectsBadFiles(t *testing.T) {
	mgr, h := newTestServer(t)
	a := createStory(t, h, map[string]string{"name": "Alpha"})
	good := exportZip(t, h, a.ID)

	if rec := importZipBytes(t, h, []byte("not a zip")); rec.Code != 400 {
		t.Fatalf("garbage: %d", rec.Code)
	}
	if rec := importZipBytes(t, h, buildZip(t, map[string][]byte{"hello.txt": []byte("hi")})); rec.Code != 400 {
		t.Fatalf("not an export: %d", rec.Code)
	}
	manifest := []byte(`{"format":"polaris-story","format_version":1,"name":"Evil"}`)
	if rec := importZipBytes(t, h, buildZip(t, map[string][]byte{"polaris-story.json": manifest, "story.db": []byte("this is not sqlite")})); rec.Code != 400 {
		t.Fatalf("bad db: %d %s", rec.Code, rec.Body.String())
	}
	if rec := importZipBytes(t, h, buildZip(t, map[string][]byte{"polaris-story.json": []byte(`{"format":"polaris-story","format_version":99,"name":"x"}`), "story.db": []byte("x")})); rec.Code != 400 {
		t.Fatalf("future format: %d", rec.Code)
	}
	list, _ := mgr.list()
	if len(list) != 1 {
		t.Fatalf("failed imports must leave no story behind: %d", len(list))
	}

	// Path tricks and non-images inside an otherwise good export are ignored.
	zr, _ := zip.NewReader(bytes.NewReader(good), int64(len(good)))
	files := map[string][]byte{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		files[f.Name] = b
	}
	files["uploads/../../escape.png"] = []byte("x")
	files["uploads/page.html"] = []byte("<script>")
	files["uploads/ok/pic.png"] = []byte("PIC")
	rec := importZipBytes(t, h, buildZip(t, files))
	if rec.Code != 200 {
		t.Fatalf("import with extras: %d %s", rec.Code, rec.Body.String())
	}
	var si storyInfo
	json.Unmarshal(rec.Body.Bytes(), &si)
	up := mgr.uploadsFor(si.ID)
	if _, err := os.Stat(filepath.Join(up, "page.html")); err == nil {
		t.Fatal("html must not be imported")
	}
	if _, err := os.Stat(filepath.Join(up, "ok", "pic.png")); err != nil {
		t.Fatal("plain image should be imported")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(up), "escape.png")); err == nil {
		t.Fatal("zip path escaped")
	}
}

func TestSafeRel(t *testing.T) {
	for _, bad := range []string{"", "/etc/passwd", "../x", "a/../../x", "..", "."} {
		if _, ok := safeRel(bad); ok {
			t.Errorf("%q should be refused", bad)
		}
	}
	if got, ok := safeRel("classes/1-2.png"); !ok || got != "classes/1-2.png" {
		t.Errorf("good path refused: %q %v", got, ok)
	}
}

func TestStoryNameValidation(t *testing.T) {
	if _, err := validStoryName("   "); err == nil {
		t.Error("blank name accepted")
	}
	if _, err := validStoryName(strings.Repeat("x", 61)); err == nil {
		t.Error("long name accepted")
	}
	if n, err := validStoryName("  The Long Night "); err != nil || n != "The Long Night" {
		t.Errorf("trim: %q %v", n, err)
	}
}
