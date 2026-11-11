package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func send(h http.Handler, method, url, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	req.Host = "polaris.test:8091"
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// Another site's page can't change anything: the browser says where a
// request comes from, and one from elsewhere is refused. The app itself, a
// script without a browser, and reading all pass.
func TestRequestsFromOtherSitesAreRefused(t *testing.T) {
	mgr, h := newTestServer(t)
	id := createStoryDirect(t, mgr, "Guarded")
	url := "/api/s/" + id + "/classes"
	const class = `{"name":"Keeper"}`
	for _, c := range []struct {
		why     string
		method  string
		headers map[string]string
		want    int
	}{
		{"another site, by Sec-Fetch-Site", "POST", map[string]string{"Sec-Fetch-Site": "cross-site"}, 403},
		{"another port or subdomain", "POST", map[string]string{"Sec-Fetch-Site": "same-site"}, 403},
		{"another site, by Origin", "POST", map[string]string{"Origin": "https://elsewhere.example"}, 403},
		{"a sandboxed page", "POST", map[string]string{"Origin": "null"}, 403},
		{"a form posted as text, no preflight", "POST", map[string]string{"Origin": "https://elsewhere.example", "Content-Type": "text/plain"}, 403},
		{"deleting", "DELETE", map[string]string{"Sec-Fetch-Site": "cross-site"}, 403},
		{"reading is not refused", "GET", map[string]string{"Sec-Fetch-Site": "cross-site"}, 200},
		{"the app itself", "POST", map[string]string{"Sec-Fetch-Site": "same-origin", "Origin": "http://polaris.test:8091"}, 200},
		{"the app, on a browser without Sec-Fetch-Site", "POST", map[string]string{"Origin": "http://polaris.test:8091"}, 200},
		{"the app behind a proxy that renames Host", "POST", map[string]string{"Origin": "https://stories.example", "X-Forwarded-Host": "stories.example"}, 200},
		{"a script, no browser", "POST", nil, 200},
	} {
		body := ""
		if c.method == "POST" {
			body = strings.Replace(class, "Keeper", c.why, 1)
		}
		if rec := send(h, c.method, url, body, c.headers); rec.Code != c.want {
			t.Errorf("%s: %d, want %d (%s)", c.why, rec.Code, c.want, strings.TrimSpace(rec.Body.String()))
		}
	}
	if rec := send(h, "POST", "/api/stories", "", map[string]string{"Sec-Fetch-Site": "cross-site"}); rec.Code != 403 {
		t.Errorf("the story list is guarded too: %d", rec.Code)
	}
	if rec := send(h, "GET", "/api/health", "", nil); rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("no answer is guessed to be a page")
	}
}

// With ALLOWED_HOSTS, a request addressed to any other name is refused,
// reading included.
func TestAllowedHosts(t *testing.T) {
	t.Setenv("ALLOWED_HOSTS", "polaris.test, Stories.Example:8091")
	_, h := newTestServer(t)
	for host, want := range map[string]int{
		"polaris.test:8091": 200, "polaris.test": 200, "stories.example": 200,
		// Names no other site can own always pass (the health check uses one).
		"localhost": 200, "127.0.0.1:8091": 200, "[::1]:8091": 200, "192.168.1.20:8091": 200,
		"nas": 200, "polaris:8091": 200, "nas.local": 200, "app.localhost": 200, "box.home.arpa": 200, "db.internal": 200,
		"rebound.example:8091": 403, "localhost.rebound.example": 403, "local": 200, "evil.local.example": 403, "": 403,
	} {
		if code := askAs(h, host, "").Code; code != want {
			t.Errorf("host %q: %d, want %d", host, code, want)
		}
	}

	// With nothing set, a domain is refused all the same: the check is on
	// from the start.
	t.Setenv("ALLOWED_HOSTS", "")
	_, bare := newTestServer(t)
	for host, want := range map[string]int{"example.com": 403, "polaris.test": 403, "localhost:8091": 200, "10.0.0.5": 200} {
		if code := askAs(bare, host, "").Code; code != want {
			t.Errorf("nothing set, host %q: %d, want %d", host, code, want)
		}
	}

	// "*" is the way back to answering any name.
	t.Setenv("ALLOWED_HOSTS", "*")
	_, open := newTestServer(t)
	if code := askAs(open, "anything.example", "").Code; code != 200 {
		t.Errorf("ALLOWED_HOSTS=*: %d", code)
	}
}

func askAs(h http.Handler, host, accept string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/api/health", nil)
	req.Host = host
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// Someone locked out by the name they use is told which line to write, with
// the name as text, never as markup.
func TestRefusedNameSaysWhatToDo(t *testing.T) {
	t.Setenv("ALLOWED_HOSTS", "")
	_, h := newTestServer(t)
	page := askAs(h, "stories.example:8091", "text/html,application/xhtml+xml")
	if page.Code != 403 || !strings.Contains(page.Body.String(), "ALLOWED_HOSTS=stories.example") || !strings.HasPrefix(page.Header().Get("Content-Type"), "text/html") {
		t.Errorf("the page: %d %s", page.Code, page.Body.String())
	}
	if csp := page.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'none'") {
		t.Errorf("the page runs nothing: %q", csp)
	}
	evil := askAs(h, `<script>alert(1)</script>.example`, "text/html")
	if evil.Code != 403 || strings.Contains(evil.Body.String(), "<script>") {
		t.Errorf("a name is shown as text: %s", evil.Body.String())
	}
	plain := askAs(h, "stories.example", "application/json")
	if plain.Code != 403 || !strings.Contains(plain.Body.String(), "ALLOWED_HOSTS") || strings.Contains(plain.Body.String(), "<") {
		t.Errorf("a script gets a line of text: %s", plain.Body.String())
	}
}

// The page says scripts may only come from Polaris; its own inline script
// is let through by its hash.
func TestPagePolicy(t *testing.T) {
	const inline = "try { document.documentElement.dataset.theme = 'x' } catch (e) {}"
	static := fstest.MapFS{"index.html": {Data: []byte(`<html><head><script>` + inline + `</script></head><script type="module" src="/assets/a.js"></script></html>`)}}
	h := spaHandler(static)
	sum := sha256.Sum256([]byte(inline))
	hash := "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
	for _, url := range []string{"/", "/characters/3"} {
		rec := send(h, "GET", url, "", nil)
		csp := rec.Header().Get("Content-Security-Policy")
		if !strings.Contains(csp, "script-src 'self' 'wasm-unsafe-eval' "+hash+";") || strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
			t.Errorf("%s: policy: %s", url, csp)
		}
		if !strings.Contains(csp, "frame-ancestors 'self'") || !strings.Contains(csp, "object-src 'none'") {
			t.Errorf("%s: policy: %s", url, csp)
		}
	}
}

// A picture form has a size limit; before, what didn't fit in memory was
// written to disk without end.
func TestPictureFormsAreCapped(t *testing.T) {
	_, h := newTestServer(t)
	big := bytes.Repeat([]byte{'x'}, maxPictureForm+1)
	body, ct := form(t, map[string]string{"name": "Heavy"}, "icon", "icon.png", big)
	if rec := do(t, h, "POST", "/api/stories", body, ct); rec.Code != 400 {
		t.Errorf("a form over the limit: %d", rec.Code)
	}
	body, ct = form(t, map[string]string{"name": "Light"}, "icon", "icon.png", []byte("png"))
	if rec := do(t, h, "POST", "/api/stories", body, ct); rec.Code != 200 {
		t.Errorf("an ordinary one: %d %s", rec.Code, rec.Body.String())
	}
}

// An export someone added a view to (to stand in for a table) is not
// imported.
func TestImportRefusesViews(t *testing.T) {
	mgr, h := newTestServer(t)
	id := createStoryDirect(t, mgr, "Tampered")
	app, _ := mgr.open(id)
	export := func() []byte {
		rec := do(t, h, "GET", "/api/stories/"+id+"/export", nil, "")
		if rec.Code != 200 {
			t.Fatalf("export: %d", rec.Code)
		}
		return rec.Body.Bytes()
	}
	clean := export()
	if _, err := app.db.Exec(`CREATE VIEW sneaky AS SELECT * FROM classes`); err != nil {
		t.Fatal(err)
	}
	tampered := export()
	for name, c := range map[string]struct {
		zip  []byte
		want int
	}{"clean": {clean, 200}, "tampered": {tampered, 400}} {
		body, ct := form(t, nil, "file", "story.zip", c.zip)
		if rec := do(t, h, "POST", "/api/stories/import", body, ct); rec.Code != c.want {
			t.Errorf("%s: %d, want %d (%s)", name, rec.Code, c.want, strings.TrimSpace(rec.Body.String()))
		}
	}
}
