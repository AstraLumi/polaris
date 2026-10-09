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
