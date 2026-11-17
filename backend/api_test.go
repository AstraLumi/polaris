package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// apiFixture: the app, the API beside it and one story.
type apiFixture struct {
	mgr   *storyManager
	app   http.Handler
	api   *apiServer
	story storyInfo
}

func newAPIFixture(t *testing.T) *apiFixture {
	t.Helper()
	apiWrongKeyWait = 0
	mgr, app := newTestServer(t)
	mgr.apiPort = "8092"
	return &apiFixture{mgr: mgr, app: app, api: newAPIServer(mgr), story: createStory(t, app, map[string]string{"name": "Saga"})}
}

// makeKey makes a key through the app's own port.
func (f *apiFixture) makeKey(t *testing.T, in newAPIKeyIn) (string, apiKey) {
	t.Helper()
	b, _ := json.Marshal(in)
	rec := do(t, f.app, "POST", "/api/api-access/keys", strings.NewReader(string(b)), "application/json")
	if rec.Code != 200 {
		t.Fatalf("make key: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Key  string `json:"key"`
		Info apiKey `json:"info"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	return out.Key, out.Info
}

func (f *apiFixture) call(t *testing.T, key, method, path, body, ctype string) *httptest.ResponseRecorder {
	t.Helper()
	var rd *strings.Reader
	if body != "" {
		rd = strings.NewReader(body)
	} else {
		rd = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, rd)
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	rec := httptest.NewRecorder()
	f.api.ServeHTTP(rec, req)
	return rec
}

func (f *apiFixture) tool(t *testing.T, key, name string, args map[string]any) (string, bool) {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": name, "arguments": args}})
	rec := f.call(t, key, "POST", "/mcp", string(b), "application/json")
	if rec.Code != 200 {
		t.Fatalf("%s: %d %s", name, rec.Code, rec.Body.String())
	}
	var out struct {
		Result struct {
			Content []struct{ Text string } `json:"content"`
			IsError bool                    `json:"isError"`
		} `json:"result"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out.Result.Content) == 0 {
		t.Fatalf("%s: no content: %s", name, rec.Body.String())
	}
	return out.Result.Content[0].Text, out.Result.IsError
}

func TestAPIKeysAreHashedAndShownOnce(t *testing.T) {
	f := newAPIFixture(t)
	key, info := f.makeKey(t, newAPIKeyIn{Name: "Claude", Write: true})
	if !strings.HasPrefix(key, "pol_") || len(key) != 4+43 {
		t.Fatalf("key %q", key)
	}
	if info.Hash != hashAPIKey(key) {
		t.Fatal("the answer's sha256 is not the key's")
	}
	rec := do(t, f.app, "GET", "/api/api-access", nil, "")
	if strings.Contains(rec.Body.String(), key) {
		t.Fatal("the list shows the key itself")
	}
	var row string
	f.mgr.reg.QueryRow(`SELECT name || hash FROM api_keys`).Scan(&row)
	if strings.Contains(row, key) {
		t.Fatal("the registry holds the key itself")
	}
	for _, bad := range []newAPIKeyIn{{Name: ""}, {Name: "x", Stories: []string{"0123456789abcdef"}}, {Name: "x", Expires: "2001-01-01"}} {
		b, _ := json.Marshal(bad)
		if rec := do(t, f.app, "POST", "/api/api-access/keys", strings.NewReader(string(b)), "application/json"); rec.Code != 400 {
			t.Errorf("%+v: %d", bad, rec.Code)
		}
	}
}

func TestAPIRefusesWithoutAKey(t *testing.T) {
	f := newAPIFixture(t)
	key, info := f.makeKey(t, newAPIKeyIn{Name: "Claude"})
	for _, k := range []string{"", "pol_wrong", "nonsense", key + "x"} {
		if rec := f.call(t, k, "GET", "/stories", "", ""); rec.Code != 401 {
			t.Errorf("key %q: %d", k, rec.Code)
		}
	}
	if rec := f.call(t, key, "GET", "/stories", "", ""); rec.Code != 200 {
		t.Fatalf("good key: %d %s", rec.Code, rec.Body.String())
	}
	// A browser can't use the port, even with the key.
	req := httptest.NewRequest("GET", "/stories", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()
	f.api.ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Fatalf("browser: %d", rec.Code)
	}
	// Revoked: refused at once.
	do(t, f.app, "DELETE", "/api/api-access/keys/"+keyIDText(info.ID), nil, "")
	if rec := f.call(t, key, "GET", "/stories", "", ""); rec.Code != 401 {
		t.Fatalf("revoked: %d", rec.Code)
	}
}

func keyIDText(n int64) string { b, _ := json.Marshal(n); return string(b) }

func TestAPIWrongKeysBlockTheAddressNotTheKeys(t *testing.T) {
	f := newAPIFixture(t)
	key, _ := f.makeKey(t, newAPIKeyIn{Name: "Claude"})
	last := 0
	for i := 0; i < apiFailsAllowed+1; i++ {
		last = f.call(t, "pol_guess", "GET", "/stories", "", "").Code
	}
	if last != 429 {
		t.Fatalf("after many wrong keys: %d", last)
	}
	if rec := f.call(t, key, "GET", "/stories", "", ""); rec.Code != 200 {
		t.Fatalf("a good key from the same address: %d", rec.Code)
	}
}

func TestAPIKeyLimits(t *testing.T) {
	f := newAPIFixture(t)
	other := createStory(t, f.app, map[string]string{"name": "Other"})
	ro, _ := f.makeKey(t, newAPIKeyIn{Name: "reader", Stories: []string{f.story.ID}})
	base := "/stories/" + f.story.ID + "/"
	if rec := f.call(t, ro, "GET", base+"characters", "", ""); rec.Code != 200 {
		t.Fatalf("read: %d %s", rec.Code, rec.Body.String())
	}
	if rec := f.call(t, ro, "POST", base+"chapters", `{"title":"One"}`, "application/json"); rec.Code != 403 {
		t.Fatalf("read-only write: %d", rec.Code)
	}
	if rec := f.call(t, ro, "GET", "/stories/"+other.ID+"/characters", "", ""); rec.Code != 404 {
		t.Fatalf("another story: %d", rec.Code)
	}
	rec := f.call(t, ro, "GET", "/stories", "", "")
	if strings.Contains(rec.Body.String(), other.ID) || !strings.Contains(rec.Body.String(), f.story.ID) {
		t.Fatalf("stories: %s", rec.Body.String())
	}
	// The keys and the story-wide routes aren't on this port.
	for _, p := range []string{"/api/api-access", "/api/stories", "/api/s/" + f.story.ID + "/characters"} {
		if rec := f.call(t, ro, "GET", p, "", ""); rec.Code != 404 {
			t.Errorf("%s: %d", p, rec.Code)
		}
	}
	// A read-only key isn't shown the tools that write.
	b := f.call(t, ro, "POST", "/mcp", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, "application/json").Body.String()
	if strings.Contains(b, `"send"`) || strings.Contains(b, "write_chapter") || !strings.Contains(b, "read_chapter") {
		t.Fatalf("tools for a reader: %s", b)
	}
	// The refused write is in the log.
	var n int
	f.mgr.reg.QueryRow(`SELECT COUNT(*) FROM api_log WHERE status = 403`).Scan(&n)
	if n != 1 {
		t.Fatalf("logged refusals: %d", n)
	}
}

func TestAPIWritesAreLoggedAndCopied(t *testing.T) {
	f := newAPIFixture(t)
	key, _ := f.makeKey(t, newAPIKeyIn{Name: "Claude", Write: true})
	base := "/stories/" + f.story.ID + "/"
	// The story must have changed since it was made for a copy to be taken.
	do(t, f.app, "POST", "/api/s/"+f.story.ID+"/chapters", strings.NewReader(`{"title":"Zero"}`), "application/json")
	if rec := f.call(t, key, "POST", base+"chapters", `{"title":"One"}`, "application/json"); rec.Code != 200 {
		t.Fatalf("write: %d %s", rec.Code, rec.Body.String())
	}
	var path string
	var status int
	f.mgr.reg.QueryRow(`SELECT path, status FROM api_log`).Scan(&path, &status)
	if path != "/chapters" || status != 200 {
		t.Fatalf("log: %q %d", path, status)
	}
	if len(f.mgr.snapshots(f.story.ID)) != 1 {
		t.Fatalf("copies before the first change: %d", len(f.mgr.snapshots(f.story.ID)))
	}
}

func TestMCPInitializeAndChapterTools(t *testing.T) {
	f := newAPIFixture(t)
	key, _ := f.makeKey(t, newAPIKeyIn{Name: "Claude", Write: true})
	rec := f.call(t, key, "POST", "/mcp", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`, "application/json")
	if !strings.Contains(rec.Body.String(), `"protocolVersion":"2025-03-26"`) || !strings.Contains(rec.Body.String(), "instructions") {
		t.Fatalf("initialize: %s", rec.Body.String())
	}
	if rec := f.call(t, key, "POST", "/mcp", `{"jsonrpc":"2.0","method":"notifications/initialized"}`, "application/json"); rec.Code != 202 {
		t.Fatalf("notification: %d", rec.Code)
	}

	// A character to link to, and a chapter.
	body, ct := form(t, map[string]string{"name": "Mira"}, "", "", nil)
	if rec := do(t, f.app, "POST", "/api/s/"+f.story.ID+"/characters", body, ct); rec.Code != 200 {
		t.Fatalf("character: %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, f.app, "POST", "/api/s/"+f.story.ID+"/chapters", strings.NewReader(`{"title":"One"}`), "application/json")
	var ch struct{ ID int64 }
	json.Unmarshal(rec.Body.Bytes(), &ch)

	args := map[string]any{"story": f.story.ID, "chapter_id": ch.ID, "markdown": "# The gate\n\n[[Mira]] walked *slowly*.\n\n* * *\n\nNight fell."}
	if text, isErr := f.tool(t, key, "write_chapter", args); isErr {
		t.Fatalf("write_chapter: %s", text)
	}
	args["markdown"] = "**Morning.**"
	if text, isErr := f.tool(t, key, "write_chapter", args); isErr {
		t.Fatalf("append again: %s", text)
	}
	text, isErr := f.tool(t, key, "read_chapter", map[string]any{"story": f.story.ID, "chapter_id": ch.ID})
	if isErr {
		t.Fatal(text)
	}
	var got struct {
		Markdown string
		Version  int64
		Words    int
	}
	json.Unmarshal([]byte(text), &got)
	want := "# The gate\n\n[[character:Mira|Mira]] walked *slowly*.\n\n* * *\n\nNight fell.\n\n**Morning.**\n"
	if got.Markdown != want {
		t.Fatalf("markdown:\n%q\nwant\n%q", got.Markdown, want)
	}
	// "Appears in" sees the link: the text went through the chapter index.
	var n int
	app, done, _ := f.mgr.use(f.story.ID)
	app.db.QueryRow(`SELECT COUNT(*) FROM chapter_links`).Scan(&n)
	done()
	if n == 0 {
		t.Fatal("the link wasn't indexed")
	}
	// Replace needs the version it was read at.
	if text, isErr := f.tool(t, key, "write_chapter", map[string]any{"story": f.story.ID, "chapter_id": ch.ID, "markdown": "New.", "mode": "replace", "base_version": got.Version - 1}); !isErr {
		t.Fatalf("stale replace went through: %s", text)
	}
	if text, isErr := f.tool(t, key, "write_chapter", map[string]any{"story": f.story.ID, "chapter_id": ch.ID, "markdown": "New.", "mode": "replace", "base_version": got.Version}); isErr {
		t.Fatalf("replace: %s", text)
	}
}

func TestMCPUpdateArticleMerges(t *testing.T) {
	f := newAPIFixture(t)
	key, _ := f.makeKey(t, newAPIKeyIn{Name: "Claude", Write: true})
	body, ct := form(t, map[string]string{"name": "Mira"}, "", "", nil)
	rec := do(t, f.app, "POST", "/api/s/"+f.story.ID+"/characters", body, ct)
	var c struct{ ID int64 }
	json.Unmarshal(rec.Body.Bytes(), &c)
	if c.ID == 0 {
		t.Fatalf("character: %s", rec.Body.String())
	}
	a := map[string]any{"story": f.story.ID, "type": "character", "id": c.ID}
	for _, extra := range []map[string]any{{"summary": "A guard."}, {"fields": map[string]any{"appearance": "Tall, scarred."}}} {
		args := map[string]any{}
		for k, v := range a {
			args[k] = v
		}
		for k, v := range extra {
			args[k] = v
		}
		if text, isErr := f.tool(t, key, "update_article", args); isErr {
			t.Fatalf("update: %s", text)
		}
	}
	text, _ := f.tool(t, key, "get_article", a)
	if !strings.Contains(text, "A guard.") || !strings.Contains(text, "Tall, scarred.") {
		t.Fatalf("the second change undid the first: %s", text)
	}
	found, _ := f.tool(t, key, "find", map[string]any{"story": f.story.ID, "query": "mir"})
	if !strings.Contains(found, `"Mira"`) {
		t.Fatalf("find: %s", found)
	}
}

func TestAPIGuideListsEveryStoryRoute(t *testing.T) {
	routes := apiRouteList(true)
	has := func(s string) bool {
		for _, r := range routes {
			if strings.HasPrefix(r, s) {
				return true
			}
		}
		return false
	}
	if !has("GET characters") || !has("PUT wiki/{type}/{id}") || !has("GET cultures") {
		t.Fatalf("routes: %v", routes)
	}
	for _, r := range apiRouteList(false) {
		if !strings.HasPrefix(r, "GET ") {
			t.Fatalf("a reader's list has %q", r)
		}
	}
}

func TestMarkdownBlocksAndBack(t *testing.T) {
	blocks := markdownBlocks("## Part\nShe said *no* and **meant it**.\n- one\n- two\n1. first\n---\nsnake_case_name stays", nil)
	doc := &pmNode{Type: "doc", Content: blocks}
	if err := validatePMDoc(doc); err != nil {
		t.Fatal(err)
	}
	got := docMarkdown(doc, nil)
	want := "## Part\n\nShe said *no* and **meant it**.\n\n- one\n- two\n\n1. first\n\n* * *\n\nsnake_case_name stays\n"
	if got != want {
		t.Fatalf("got\n%q\nwant\n%q", got, want)
	}
	if ok, _ := markdownCanReplace(doc); !ok {
		t.Fatal("plain Markdown can't be replaced")
	}
	doc.Content = append(doc.Content, &pmNode{Type: "dialogue", Attrs: map[string]any{"speaker": "Mira"}})
	if ok, _ := markdownCanReplace(doc); ok {
		t.Fatal("a dialogue would be lost silently")
	}
}
