package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"
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

// A story nobody used for a while is closed and opens again on the next
// request; one that is in use is left alone.
func TestIdleStoriesAreClosed(t *testing.T) {
	mgr, h := newTestServer(t)
	idle := createStoryDirect(t, mgr, "Idle")
	busy := createStoryDirect(t, mgr, "Busy")
	addClass(t, h, idle, "Keeper")
	_, done, err := mgr.use(busy)
	if err != nil {
		t.Fatal(err)
	}

	later := time.Now().Add(storyIdleAfter + time.Minute)
	if n := mgr.closeIdle(time.Now()); n != 0 {
		t.Fatalf("nothing is idle yet: %d closed", n)
	}
	if n := mgr.closeIdle(later); n != 1 {
		t.Fatalf("the idle story is closed, the one in use is not: %d closed", n)
	}
	mgr.mu.Lock()
	_, idleOpen := mgr.apps[idle]
	_, busyOpen := mgr.apps[busy]
	mgr.mu.Unlock()
	if idleOpen || !busyOpen {
		t.Fatalf("idle open %v, busy open %v", idleOpen, busyOpen)
	}
	if rec := do(t, h, "GET", "/api/s/"+idle+"/classes", nil, ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), "Keeper") {
		t.Errorf("it opens again when asked for: %d %s", rec.Code, rec.Body.String())
	}
	// The list still counts a closed story's characters.
	mgr.closeIdle(time.Now().Add(2 * storyIdleAfter))
	if rec := do(t, h, "GET", "/api/stories", nil, ""); rec.Code != 200 || strings.Count(rec.Body.String(), `"characters":0`) != 2 {
		t.Errorf("list: %d %s", rec.Code, rec.Body.String())
	}

	done()
	if n := mgr.closeIdle(time.Now().Add(2 * storyIdleAfter)); n != 1 {
		t.Errorf("once released, it closes too: %d", n)
	}
}

// Many requests for a story that isn't open yet open it once, and all get
// the same one.
func TestAStoryIsOpenedOnce(t *testing.T) {
	mgr, _ := newTestServer(t)
	id := createStoryDirect(t, mgr, "Crowd")
	closeStory(mgr, id)
	const n = 12
	got := make(chan *storyApp, n)
	for range n {
		go func() {
			a, err := mgr.open(id)
			if err != nil {
				t.Error(err)
			}
			got <- a
		}()
	}
	first := <-got
	for range n - 1 {
		if a := <-got; a != first {
			t.Fatal("two copies of one story were opened")
		}
	}
	if _, err := mgr.open("00000000000000ff"); err != errNoStory {
		t.Errorf("no such story: %v", err)
	}
}

// The character list works its cards' stats out for everyone at once
// (loadCardBuild); a sheet works one character's out on its own
// (buildVersionDetail). Both must give the same numbers, with bonuses from
// a class, a race and gear, a piece worn twice, and typed numbers.
func TestCharacterListStatsMatchTheSheet(t *testing.T) {
	mgr, h := newTestServer(t)
	id := createStoryDirect(t, mgr, "Cards")
	app, _ := mgr.open(id)
	sys, err := loadStatSystem(app.db)
	if err != nil {
		t.Fatal(err)
	}
	var base, primary []string
	for _, d := range sys.Stats {
		switch d.Category {
		case "base":
			base = append(base, d.Key)
		case "primary":
			primary = append(primary, d.Key)
		}
	}
	if len(base) < 2 || len(primary) == 0 {
		t.Fatalf("the default stat system has base stats and points: %v %v", base, primary)
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := app.db.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	exec(`INSERT INTO classes (id, name) VALUES (1, 'Keeper')`)
	exec(`INSERT INTO races (id, name) VALUES (1, 'Elf')`)
	exec(`INSERT INTO gear (id, name, slot) VALUES (1, 'Ring', 'ring'), (2, 'Helm', 'head')`)
	for _, m := range [][]any{
		{"class", 1, base[0], "per_level", 3.0}, {"class", 1, base[1], "flat", 7.0},
		{"race", 1, base[0], "multiplier", 0.25}, {"race", 1, primary[0], "flat", 2.0},
		{"item", 1, base[0], "flat", 11.0}, {"item", 2, base[1], "flat", 5.0}, {"item", 2, primary[0], "flat", 1.0},
	} {
		exec(`INSERT INTO stat_modifiers (source_type, source_id, target_stat, scaling, value) VALUES (?, ?, ?, ?, ?)`, m...)
	}
	// 1: everything; 2: nothing at all; 3: an older version that must not count.
	exec(`INSERT INTO characters (id) VALUES (1), (2)`)
	exec(`INSERT INTO character_versions (id, character_id, is_current, name, level, class_id) VALUES (1, 1, 0, 'Aria', 2, NULL), (2, 1, 1, 'Aria', 9, 1), (3, 2, 1, 'Bram', 1, NULL)`)
	exec(`INSERT INTO character_story (version_id, race_id, age, height, weight) VALUES (2, 1, 40, 181, 77)`)
	exec(`INSERT INTO character_gear (version_id, slot, gear_id) VALUES (2, 'ring1', 1), (2, 'ring2', 1), (2, 'head', 2), (1, 'head', 2)`)
	exec(`INSERT INTO character_special_bases (version_id, stat_key, value) VALUES (2, ?, 6), (2, ?, 13), (1, ?, 99)`, primary[0], base[0], base[0])

	var list []struct {
		VersionID int64 `json:"version_id"`
		Stats     []struct {
			Name  string  `json:"name"`
			Value float64 `json:"value"`
		} `json:"stats"`
	}
	rec := do(t, h, "GET", "/api/s/"+id+"/characters", nil, "")
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != 2 {
		t.Fatalf("list: %v %s", err, rec.Body.String())
	}
	for _, c := range list {
		detail, err := buildVersionDetail(app.db, strconv.FormatInt(c.VersionID, 10))
		if err != nil {
			t.Fatal(err)
		}
		if len(c.Stats) != 2 {
			t.Fatalf("version %d: two stats on a card: %+v", c.VersionID, c.Stats)
		}
		for i, s := range c.Stats {
			if want := detail.Computed[base[i]]; s.Value != want {
				t.Errorf("version %d, %s: the list says %v, the sheet %v", c.VersionID, s.Name, s.Value, want)
			}
		}
	}
	// And the bonuses did reach the numbers: the two characters differ.
	if list[0].Stats[0].Value == list[1].Stats[0].Value {
		t.Errorf("bonuses made no difference: %+v", list)
	}
}
