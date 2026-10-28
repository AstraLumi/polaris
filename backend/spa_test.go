package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

// A page's path that is also a folder in the built frontend (/assets holds
// the scripts and is the Character Assets page) must load the page, not
// redirect to the folder.
func TestSPAHandlerServesPagesThatShareAFolderName(t *testing.T) {
	static := fstest.MapFS{
		"index.html":        {Data: []byte("<html>app</html>")},
		"assets/index-1.js": {Data: []byte("console.log(1)")},
	}
	h := spaHandler(static)

	for _, path := range []string{"/assets", "/characters/3", "/"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "app") {
			t.Errorf("%s: got %d %q, want the app page", path, rec.Code, rec.Body.String())
		}
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/index-1.js", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "console.log") {
		t.Errorf("built script: got %d %q", rec.Code, rec.Body.String())
	}
}

// Offline writing's service worker relies on these: the page and /sw.js are
// asked for again every time (a release shows at once), hashed built files
// are kept for good.
func TestSPAHandlerCacheHeaders(t *testing.T) {
	static := fstest.MapFS{
		"index.html":        {Data: []byte("<html>app</html>")},
		"sw.js":             {Data: []byte("self.x = 1")},
		"assets/index-1.js": {Data: []byte("console.log(1)")},
		"favicon.svg":       {Data: []byte("<svg/>")},
	}
	h := spaHandler(static)
	for path, want := range map[string]string{
		"/":                  "no-cache",
		"/write/3":           "no-cache",
		"/assets":            "no-cache",
		"/sw.js":             "no-cache",
		"/assets/index-1.js": "public, max-age=31536000, immutable",
		"/assets/gone-2.js":  "no-cache", // the page, not a script: never kept
		"/favicon.svg":       "",
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if got := rec.Header().Get("Cache-Control"); got != want {
			t.Errorf("%s: Cache-Control %q, want %q", path, got, want)
		}
	}
}
