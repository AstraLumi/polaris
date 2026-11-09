package main

// Answers are sent gzipped to browsers that take it: the app's scripts and
// styles shrink to about a third, JSON to less. Pictures, fonts, zips and
// anything already compressed go out as they are.

import (
	"compress/gzip"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// Below this an answer isn't worth the header.
const gzipMinSize = 1024

var gzipPool = sync.Pool{New: func() any {
	zw, _ := gzip.NewWriterLevel(io.Discard, 5)
	return zw
}}

func compressible(contentType string) bool {
	ct, _, _ := strings.Cut(contentType, ";")
	ct = strings.ToLower(strings.TrimSpace(ct))
	switch ct {
	case "application/json", "application/javascript", "application/xml", "application/manifest+json",
		"application/wasm", "image/svg+xml", "application/xhtml+xml":
		return true
	}
	return strings.HasPrefix(ct, "text/")
}

// compress wraps the server. A request for part of a file (Range) is left
// alone: the parts are counted in the file's own bytes.
func compress(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead || r.Header.Get("Range") != "" ||
			!strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		gw := &gzipResponse{ResponseWriter: w}
		defer gw.close()
		next.ServeHTTP(gw, r)
	})
}

type gzipResponse struct {
	http.ResponseWriter
	zw      *gzip.Writer
	decided bool
}

// Unwrap lets http.ResponseController reach the real writer.
func (g *gzipResponse) Unwrap() http.ResponseWriter { return g.ResponseWriter }

// decide runs once, when the status and headers are final.
func (g *gzipResponse) decide(status int, first []byte) {
	g.decided = true
	h := g.Header()
	if h.Get("Content-Type") == "" && len(first) > 0 {
		h.Set("Content-Type", http.DetectContentType(first))
	}
	if !compressible(h.Get("Content-Type")) {
		return
	}
	h.Add("Vary", "Accept-Encoding")
	if status == http.StatusNoContent || status == http.StatusNotModified || status < 200 ||
		h.Get("Content-Encoding") != "" || h.Get("Content-Range") != "" {
		return
	}
	if n, err := strconv.Atoi(h.Get("Content-Length")); err == nil && n < gzipMinSize {
		return
	}
	h.Set("Content-Encoding", "gzip")
	h.Del("Content-Length")
	g.zw = gzipPool.Get().(*gzip.Writer)
	g.zw.Reset(g.ResponseWriter)
}

func (g *gzipResponse) WriteHeader(status int) {
	if !g.decided {
		g.decide(status, nil)
	}
	g.ResponseWriter.WriteHeader(status)
}

func (g *gzipResponse) Write(b []byte) (int, error) {
	if !g.decided {
		g.decide(http.StatusOK, b)
	}
	if g.zw != nil {
		return g.zw.Write(b)
	}
	return g.ResponseWriter.Write(b)
}

func (g *gzipResponse) Flush() {
	if g.zw != nil {
		g.zw.Flush()
	}
	if f, ok := g.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (g *gzipResponse) close() {
	if g.zw == nil {
		return
	}
	g.zw.Close()
	g.zw.Reset(io.Discard)
	gzipPool.Put(g.zw)
	g.zw = nil
}
