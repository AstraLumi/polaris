package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"
)

// The graph is kept between requests, stays right when the database changes
// by any road, and keeps its uploads rewriting and its "not modified".
func TestGraphCache(t *testing.T) {
	mgr, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Cache"})
	app, _ := mgr.open(s.ID)
	url := "/api/s/" + s.ID + "/wiki/graph"
	get := func(etag string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", url, nil)
		if etag != "" {
			req.Header.Set("If-None-Match", etag)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	nodes := func(rec *httptest.ResponseRecorder) []graphNode {
		var g graphPayload
		if err := json.Unmarshal(rec.Body.Bytes(), &g); err != nil {
			t.Fatalf("graph: %d %s", rec.Code, rec.Body.String())
		}
		return g.Nodes
	}

	app.db.Exec(`INSERT INTO lore_articles (name, picture_path) VALUES ('Old Gods', 'gods.png')`)
	first := get("")
	if n := nodes(first); len(n) != 1 || n[0].Picture != "/uploads/s/"+s.ID+"/gods.png" {
		t.Fatalf("nodes: %+v", n)
	}
	etag := first.Header().Get("ETag")
	if etag == "" || first.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("headers: %v", first.Header())
	}
	if again := get(""); again.Body.String() != first.Body.String() {
		t.Errorf("kept graph differs:\n%s\n%s", again.Body.String(), first.Body.String())
	}
	if rec := get(etag); rec.Code != 304 || rec.Body.Len() != 0 {
		t.Errorf("same graph: %d %q", rec.Code, rec.Body.String())
	}

	// A change made straight in the database, past every handler, is seen.
	app.db.Exec(`INSERT INTO lore_articles (name) VALUES ('New Sea')`)
	rec := get(etag)
	if rec.Code != 200 || len(nodes(rec)) != 2 || rec.Header().Get("ETag") == etag {
		t.Errorf("after a change: %d %s", rec.Code, rec.Body.String())
	}
}

// After a change, a graph someone looked at is rebuilt in the background;
// one nobody looks at is forgotten; a closed story keeps nothing.
func TestGraphCacheBackground(t *testing.T) {
	quiet, keep := graphQuiet, graphKeep
	graphQuiet = 20 * time.Millisecond
	t.Cleanup(func() { graphQuiet, graphKeep = quiet, keep })

	mgr, h := newTestServer(t)
	s := createStory(t, h, map[string]string{"name": "Warm"})
	app, _ := mgr.open(s.ID)
	c := app.graph
	kept := func() (bool, uint32) {
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.gz != nil, c.stamp
	}

	// Nobody has looked yet: a change builds nothing.
	createLore(t, h, s.ID, "Quiet")
	time.Sleep(100 * time.Millisecond)
	if ok, _ := kept(); ok {
		t.Fatal("built a graph nobody asked for")
	}

	do(t, h, "GET", "/api/s/"+s.ID+"/wiki/graph", nil, "")
	createLore(t, h, s.ID, "Loud")
	deadline := time.Now().Add(5 * time.Second)
	for {
		now, _ := c.changeCounter()
		if ok, stamp := kept(); ok && stamp == now {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the graph was not rebuilt after the change")
		}
		time.Sleep(10 * time.Millisecond)
	}
	var g graphPayload
	rec := do(t, h, "GET", "/api/s/"+s.ID+"/wiki/graph", nil, "")
	json.Unmarshal(rec.Body.Bytes(), &g)
	if len(g.Nodes) != 2 {
		t.Errorf("nodes: %+v", g.Nodes)
	}

	// The editor's saves don't wake it.
	c.mu.Lock()
	c.gz = nil
	c.mu.Unlock()
	c.changed("chapters/3/text")
	c.changed("books/0")
	time.Sleep(100 * time.Millisecond)
	if ok, _ := kept(); ok {
		t.Error("an editor save rebuilt the graph")
	}

	// Not looked at for graphKeep: forgotten.
	c.get(true)
	c.mu.Lock()
	c.seen = time.Now().Add(-2 * graphKeep)
	c.mu.Unlock()
	c.drop()
	if ok, _ := kept(); ok {
		t.Error("an unused graph was kept")
	}

	// A deleted story stops its cache.
	c.get(true)
	mgr.removeStory(s.ID)
	if ok, _ := kept(); ok {
		t.Error("a closed story kept its graph")
	}
}
