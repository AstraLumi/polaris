package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// The wiki graph, kept ready (wikigraph.go builds it). Building reads every
// article's facts, about a second on a small server, so each story keeps its
// last graph:
//
//   - only as gzipped JSON, a small fraction of the graph in memory, unzipped
//     straight into each response;
//   - stamped with the database file's change counter, which SQLite raises on
//     every commit, whoever makes it: a graph whose stamp is behind is stale,
//     so nothing that saves has to remember the graph;
//   - rebuilt in the background a moment after the story was changed, when
//     someone has looked at the graph lately, so opening it is instant;
//   - dropped when nobody has looked at it for a while, giving the memory
//     back.
//
// One build runs at a time per story; a request that comes during one waits
// for it rather than starting another.

// Variables only so the tests can shorten them.
var (
	graphQuiet = 5 * time.Second  // rebuild this long after the last change
	graphKeep  = 30 * time.Minute // forget a graph nobody looked at for this long
)

type graphCache struct {
	db   *sql.DB
	path string // the database file; "" never keeps a graph

	mu       sync.Mutex
	gz       []byte // the graph's JSON, gzipped; nil when none is kept
	etag     string
	stamp    uint32        // the file's change counter when the graph was built
	seen     time.Time     // when the graph was last asked for
	building chan struct{} // closed when the build under way ends
	quiet    *time.Timer   // the background rebuild, after changes
	forget   *time.Timer   // drops the graph when it isn't looked at
	stopped  bool
}

func newGraphCache(db *sql.DB, path string) *graphCache {
	return &graphCache{db: db, path: path}
}

// changeCounter is the database file's change counter (bytes 24-27 of its
// header). It can't be trusted in WAL mode (header bytes 18-19 are 2), where
// commits don't touch the main file: then ok is false and nothing is kept.
func (c *graphCache) changeCounter() (n uint32, ok bool) {
	if c.path == "" {
		return 0, false
	}
	f, err := os.Open(c.path)
	if err != nil {
		return 0, false
	}
	defer f.Close()
	var h [28]byte
	if _, err := f.ReadAt(h[:], 0); err != nil || h[18] == 2 || h[19] == 2 {
		return 0, false
	}
	return binary.BigEndian.Uint32(h[24:28]), true
}

// get gives the graph, building it if the kept one is stale. A view (not
// the background rebuild) keeps the graph from being forgotten.
func (c *graphCache) get(view bool) (gz []byte, etag string, err error) {
	for {
		stamp, ok := c.changeCounter()
		c.mu.Lock()
		if view {
			c.seen = time.Now()
			if c.forget == nil {
				c.forget = time.AfterFunc(graphKeep, c.drop)
			} else {
				c.forget.Reset(graphKeep)
			}
		}
		if ok && c.gz != nil && c.stamp == stamp {
			gz, etag = c.gz, c.etag
			c.mu.Unlock()
			return gz, etag, nil
		}
		if ch := c.building; ch != nil {
			// Someone is building it: wait, then look again (a change may
			// have come in during that build).
			c.mu.Unlock()
			<-ch
			continue
		}
		ch := make(chan struct{})
		c.building = ch
		c.mu.Unlock()

		gz, etag, err = buildGraphJSON(c.db)

		c.mu.Lock()
		c.building = nil
		close(ch)
		if err == nil && ok && !c.stopped {
			c.gz, c.etag, c.stamp = gz, etag, stamp
		}
		c.mu.Unlock()
		return gz, etag, err
	}
}

// buildGraphJSON builds the graph and gzips its JSON; the ETag is a hash of
// that JSON, so an unchanged graph keeps it even after a rebuild.
func buildGraphJSON(db *sql.DB) ([]byte, string, error) {
	g, err := buildWikiGraph(db)
	if err != nil {
		return nil, "", err
	}
	raw, err := json.Marshal(g)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(raw)
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	zw.Write(raw)
	zw.Close()
	// A slice of its own size: the buffer grew in steps and may be larger.
	return bytes.Clone(buf.Bytes()), `"` + hex.EncodeToString(sum[:8]) + `"`, nil
}

// changed is told about every request that may have changed the story
// (anything but GET), with its path under /api/. When the graph was looked
// at lately, it is rebuilt once the story has been quiet for graphQuiet.
// The writing editor's saves are left out: it saves every few seconds and
// nothing it saves is in the graph (the graph is still checked when asked
// for).
func (c *graphCache) changed(path string) {
	if strings.HasPrefix(path, "books/") || strings.HasPrefix(path, "comments/") ||
		(strings.HasPrefix(path, "chapters/") && (strings.Contains(path, "/text") || strings.Contains(path, "/comments"))) {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped || c.seen.IsZero() || time.Since(c.seen) > graphKeep {
		return
	}
	if c.quiet == nil {
		c.quiet = time.AfterFunc(graphQuiet, c.rebuild)
	} else {
		c.quiet.Reset(graphQuiet)
	}
}

func (c *graphCache) rebuild() {
	c.mu.Lock()
	stopped := c.stopped
	c.mu.Unlock()
	if stopped {
		return
	}
	if _, _, err := c.get(false); err != nil {
		log.Printf("wiki graph (background): %v", err)
	}
}

// drop forgets the graph (nobody has looked at it for graphKeep).
func (c *graphCache) drop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Since(c.seen) < graphKeep {
		return // looked at again just as the timer went off
	}
	c.gz, c.etag = nil, ""
}

// stop is for a story being closed: no more background work, nothing kept.
func (c *graphCache) stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopped = true
	c.gz, c.etag = nil, ""
	for _, t := range []*time.Timer{c.quiet, c.forget} {
		if t != nil {
			t.Stop()
		}
	}
}

func wikiGraphHandler(c *graphCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gz, etag, err := c.get(true)
		if err != nil {
			http.Error(w, "failed to load the graph", http.StatusInternalServerError)
			log.Printf("wiki graph: %v", err)
			return
		}
		// The browser asks again every time and gets "not modified" while
		// the graph is the same.
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("ETag", etag)
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		zr, err := gzip.NewReader(bytes.NewReader(gz))
		if err != nil {
			http.Error(w, "failed to load the graph", http.StatusInternalServerError)
			return
		}
		io.Copy(w, zr)
	}
}
