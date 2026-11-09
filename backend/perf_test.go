package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func get(h http.Handler, url string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", url, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func gunzip(t *testing.T, b []byte) string {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("not gzip: %v", err)
	}
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// Text goes out gzipped to a browser that takes it, and comes back the
// same; pictures, small answers, parts of files and browsers that don't
// take it get the plain bytes.
func TestAnswersAreCompressed(t *testing.T) {
	script := strings.Repeat("console.log('polaris');\n", 400)
	picture := bytes.Repeat([]byte{0x89, 'P', 'N', 'G', 1, 2, 3, 4}, 400)
	static := fstest.MapFS{
		"index.html":       {Data: []byte("<html>app</html>")},
		"assets/app-1.js":  {Data: []byte(script)},
		"assets/pic-1.png": {Data: picture},
	}
	h := compress(spaHandler(static))
	gz := map[string]string{"Accept-Encoding": "gzip, deflate, br"}

	rec := get(h, "/assets/app-1.js", gz)
	if rec.Header().Get("Content-Encoding") != "gzip" || rec.Header().Get("Content-Length") != "" ||
		!strings.Contains(rec.Header().Get("Vary"), "Accept-Encoding") {
		t.Fatalf("script headers: %v", rec.Header())
	}
	if rec.Body.Len() >= len(script)/4 || gunzip(t, rec.Body.Bytes()) != script {
		t.Errorf("script: %d bytes of %d", rec.Body.Len(), len(script))
	}
	for why, c := range map[string]struct {
		url     string
		headers map[string]string
		want    string
	}{
		"a browser that doesn't take gzip": {"/assets/app-1.js", nil, script},
		"a picture":                        {"/assets/pic-1.png", gz, string(picture)},
		"a small answer":                   {"/", gz, "<html>app</html>"},
		"part of a file":                   {"/assets/app-1.js", map[string]string{"Accept-Encoding": "gzip", "Range": "bytes=0-9"}, script[:10]},
	} {
		rec := get(h, c.url, c.headers)
		if rec.Header().Get("Content-Encoding") != "" || rec.Body.String() != c.want {
			t.Errorf("%s: encoding %q, %d bytes", why, rec.Header().Get("Content-Encoding"), rec.Body.Len())
		}
	}
}

// Through the whole server: a story's JSON is gzipped after its picture
// addresses were rewritten, and what isn't JSON isn't held back.
func TestStoryAnswersThroughTheServer(t *testing.T) {
	mgr, h := newTestServer(t)
	id := createStoryDirect(t, mgr, "Zipped")
	for i := range 60 {
		addClass(t, h, id, "Class number "+strings.Repeat("x", i+1))
	}
	rec := get(h, "/api/s/"+id+"/classes", map[string]string{"Accept-Encoding": "gzip"})
	if rec.Code != 200 || rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("classes: %d %v", rec.Code, rec.Header())
	}
	if body := gunzip(t, rec.Body.Bytes()); !strings.Contains(body, "Class number x") {
		t.Errorf("classes: %.80s", body)
	}

	// An error's text passes the rewriter untouched, with its status.
	rw := &uploadsRewriter{w: httptest.NewRecorder(), replacement: []byte(`"/uploads/s/x/`)}
	rw.Header().Set("Content-Type", "text/plain; charset=utf-8")
	rw.WriteHeader(http.StatusTeapot)
	rw.Write([]byte(`"/uploads/a.png"`))
	if out := rw.w.(*httptest.ResponseRecorder); out.Code != http.StatusTeapot || out.Body.String() != `"/uploads/a.png"` || !rw.direct {
		t.Errorf("not JSON: %d %s", out.Code, out.Body.String())
	}
	rw.finish()
	rw = &uploadsRewriter{w: httptest.NewRecorder(), replacement: []byte(`"/uploads/s/x/`)}
	rw.Header().Set("Content-Type", "application/json")
	rw.Write([]byte(`{"p":"/uploads/a.png"}`))
	if out := rw.w.(*httptest.ResponseRecorder); out.Body.Len() != 0 {
		t.Error("JSON waits until it is whole")
	}
	rw.finish()
	if out := rw.w.(*httptest.ResponseRecorder); out.Body.String() != `{"p":"/uploads/s/x/a.png"}` {
		t.Errorf("JSON: %s", out.Body.String())
	}
}

// Listing the stories doesn't open them: one that needs upgrading is only
// upgraded (and backed up) when it is really opened.
func TestListingDoesNotOpenStories(t *testing.T) {
	mgr, h := newTestServer(t)
	id := createStoryDirect(t, mgr, "Shelved")
	app, _ := mgr.open(id)
	app.db.Exec(`INSERT INTO characters DEFAULT VALUES`)
	rollBackSchema28(t, app.db)
	closeStory(mgr, id)

	rec := do(t, h, "GET", "/api/stories", nil, "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"characters":1`) {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	mgr.mu.Lock()
	_, open := mgr.apps[id]
	mgr.mu.Unlock()
	db := openRaw(t, mgr.dbPathFor(id))
	defer db.Close()
	if v := storedSchema(db); open || v != "27" {
		t.Errorf("the story was opened by the list (open %v, schema %s)", open, v)
	}
}
