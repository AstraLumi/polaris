package main

// The API listener: a second port (API_PORT, off when unset) for programs
// such as an LLM's tools. It is the one part of Polaris meant to face the
// internet, so it serves nothing without a key and no web pages at all;
// the app itself stays on PORT, where the keys are made (apikeys.go).
//
//	GET  /                          what this is (with a key)
//	GET  /stories                   the stories the key reaches
//	ANY  /stories/{id}/{rest...}    that story's API, as the app's own
//	                                /api/s/{id}/api/{rest} (main.go, storyRoutes)
//	GET  /uploads/s/{id}/{path...}  a story's picture, as the answers name them
//	POST /mcp                       Model Context Protocol (apimcp.go)
//	GET  /openapi.json              the tools, for OpenAPI clients (apimcp.go)
//	POST /tools/{name}              one tool, for OpenAPI clients
//
// Every request:
//
//   - is refused when a browser sent it (Origin, Sec-Fetch-*): only
//     programs use this port, so no web page can be made to use a key;
//   - needs "Authorization: Bearer pol_…", a known key that hasn't expired.
//     A wrong key is answered slowly, and an address that keeps trying is
//     refused for a while; a known key is never held back by that;
//   - is counted against its key (apiPerMinute);
//   - can't be larger than maxAPIBody;
//   - with a read-only key may only read; a key reaches only its stories.
//
// Every change is written to the activity log (Settings → API access), and
// before a story's first change through the API each day the server makes
// one of its copies (snapshots.go), so an LLM's mistake can be undone.
//
// The story-wide management (making, deleting, importing, restoring
// stories), the fonts and reference libraries and the keys themselves are
// not reachable here.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	maxAPIBody      = 32 << 20
	apiPerMinute    = 600 // requests per key per minute
	apiFailWindow   = 10 * time.Minute
	apiFailsAllowed = 20 // wrong keys per address per window before it is refused
)

// How long a wrong key waits for its answer (a variable for the tests).
var apiWrongKeyWait = 300 * time.Millisecond

type apiServer struct {
	m   *storyManager
	mux *http.ServeMux
	now func() time.Time

	mu       sync.Mutex
	fails    map[string][]time.Time // wrong keys per remote address
	counts   map[int64]int          // requests per key this minute
	minute   int64
	snapDays map[string]string // story id -> the UTC day its API copy was made
	lastWarn time.Time
}

func newAPIServer(m *storyManager) *apiServer {
	a := &apiServer{
		m: m, now: time.Now,
		fails: map[string][]time.Time{}, counts: map[int64]int{}, snapDays: map[string]string{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", a.info)
	mux.HandleFunc("GET /stories", a.stories)
	mux.HandleFunc("/stories/{id}/{rest...}", a.story)
	mux.HandleFunc("GET /uploads/s/{id}/{path...}", a.uploads)
	mux.HandleFunc("POST /mcp", a.mcp)
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		// No server-sent stream: every answer comes with its request.
		w.Header().Set("Allow", "POST")
		apiError(w, http.StatusMethodNotAllowed, "use POST")
	})
	mux.HandleFunc("GET /openapi.json", a.openAPI)
	mux.HandleFunc("POST /tools/{name}", a.toolREST)
	a.mux = mux
	return a
}

type apiKeyCtx struct{}

func apiError(w http.ResponseWriter, status int, msg string) {
	writeJSONStatus(w, status, map[string]string{"error": msg})
}

func (a *apiServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if fromBrowser(r) {
		apiError(w, http.StatusForbidden, "this port is for programs, not browsers")
		return
	}
	now := a.now()
	key := a.m.keys.check(bearerKey(r), now)
	if key == nil {
		a.wrongKey(w, r, now)
		return
	}
	if !a.count(key, now) {
		w.Header().Set("Retry-After", "60")
		apiError(w, http.StatusTooManyRequests, "too many requests: at most 600 a minute per key")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAPIBody)
	a.mux.ServeHTTP(w, r.WithContext(contextWithKey(r, key)))
}

// fromBrowser: the request carries what only a browser sends.
func fromBrowser(r *http.Request) bool {
	for _, h := range []string{"Origin", "Sec-Fetch-Site", "Sec-Fetch-Mode", "Sec-Fetch-Dest"} {
		if r.Header.Get(h) != "" {
			return true
		}
	}
	return false
}

func bearerKey(r *http.Request) string {
	v := r.Header.Get("Authorization")
	if len(v) > 7 && strings.EqualFold(v[:7], "bearer ") {
		return strings.TrimSpace(v[7:])
	}
	return ""
}

// wrongKey answers a request without a usable key: slowly, and not at all
// for an address that has kept trying. The address is the connection's own,
// never a header a client could make up.
func (a *apiServer) wrongKey(w http.ResponseWriter, r *http.Request, now time.Time) {
	addr, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		addr = r.RemoteAddr
	}
	a.mu.Lock()
	recent := a.fails[addr][:0]
	for _, t := range a.fails[addr] {
		if now.Sub(t) < apiFailWindow {
			recent = append(recent, t)
		}
	}
	blocked := len(recent) >= apiFailsAllowed
	if !blocked {
		recent = append(recent, now)
	}
	a.fails[addr] = recent
	if len(a.fails) > 10000 { // a flood from many addresses: start over rather than grow
		a.fails = map[string][]time.Time{addr: recent}
	}
	warn := now.Sub(a.lastWarn) > time.Minute
	if warn {
		a.lastWarn = now
	}
	a.mu.Unlock()
	if warn {
		log.Printf("api: refused a request without a valid key from %s", addr)
	}
	if blocked {
		w.Header().Set("Retry-After", "600")
		apiError(w, http.StatusTooManyRequests, "too many wrong keys from this address; wait 10 minutes")
		return
	}
	time.Sleep(apiWrongKeyWait)
	w.Header().Set("WWW-Authenticate", `Bearer realm="polaris"`)
	apiError(w, http.StatusUnauthorized, "a valid API key is needed: Authorization: Bearer pol_…")
}

func (a *apiServer) count(k *apiKey, now time.Time) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if m := now.Unix() / 60; m != a.minute {
		a.minute, a.counts = m, map[int64]int{}
	}
	a.counts[k.ID]++
	return a.counts[k.ID] <= apiPerMinute
}

// ---- routes ----

func (a *apiServer) info(w http.ResponseWriter, r *http.Request) {
	k := keyOf(r)
	writeJSON(w, map[string]any{
		"name":           "Polaris API",
		"schema_version": currentSchemaVersion,
		"key":            map[string]any{"name": k.Name, "write": k.CanWrite, "stories": k.Stories},
		"routes": map[string]string{
			"stories": "GET /stories",
			"story":   "/stories/{id}/{path}, the same paths as the app's /api/{path}",
			"mcp":     "POST /mcp (Model Context Protocol, streamable HTTP, JSON answers)",
			"openapi": "GET /openapi.json",
		},
	})
}

type apiStoryOut struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Characters int    `json:"characters"`
	CreatedAt  string `json:"created_at"`
}

func (a *apiServer) storiesFor(k *apiKey) ([]apiStoryOut, error) {
	list, err := a.m.list()
	if err != nil {
		return nil, err
	}
	out := []apiStoryOut{}
	for _, s := range list {
		if k.reaches(s.ID) {
			out = append(out, apiStoryOut{ID: s.ID, Name: s.Name, Characters: s.Characters, CreatedAt: s.CreatedAt})
		}
	}
	return out, nil
}

func (a *apiServer) stories(w http.ResponseWriter, r *http.Request) {
	out, err := a.storiesFor(keyOf(r))
	if err != nil {
		apiError(w, http.StatusInternalServerError, "failed to load stories")
		return
	}
	writeJSON(w, out)
}

func (a *apiServer) story(w http.ResponseWriter, r *http.Request) {
	a.forward(w, r, keyOf(r), r.PathValue("id"), r.PathValue("rest"), "")
}

// forward hands a request to a story's own routes, with the key's limits.
// via names the tool that asked, for the log ("" for a plain request).
func (a *apiServer) forward(w http.ResponseWriter, r *http.Request, k *apiKey, id, rest, via string) {
	if !storyIDPattern.MatchString(id) || !k.reaches(id) {
		apiError(w, http.StatusNotFound, "story not found")
		return
	}
	if _, _, err := a.m.get(id); err != nil {
		apiError(w, http.StatusNotFound, "story not found")
		return
	}
	read := r.Method == http.MethodGet || r.Method == http.MethodHead
	logPath := "/" + rest
	if via != "" {
		logPath = via + ": " + logPath
	}
	if !read && !k.CanWrite {
		a.m.keys.note(k, id, r.Method, logPath, http.StatusForbidden)
		apiError(w, http.StatusForbidden, "this key may only read")
		return
	}
	if !read {
		a.copyBeforeChange(id)
	}
	r2 := r.Clone(r.Context())
	r2.SetPathValue("id", id)
	r2.SetPathValue("rest", rest)
	sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
	a.m.storyHandler(sw, r2)
	if !read {
		a.m.keys.note(k, id, r.Method, logPath, sw.status)
	}
}

// copyBeforeChange makes the story's copy before its first change through
// the API each day (snapshots.go; the copy is skipped when nothing changed
// since the last one).
func (a *apiServer) copyBeforeChange(id string) {
	day := a.now().UTC().Format("2006-01-02")
	a.mu.Lock()
	done := a.snapDays[id] == day
	a.snapDays[id] = day
	a.mu.Unlock()
	if done || !autoBackupsOn() {
		return
	}
	if _, err := a.m.snapshotStoryNow(id); err != nil {
		log.Printf("api: copy story %s before a change: %v", id, err)
	}
}

func (a *apiServer) uploads(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !keyOf(r).reaches(id) {
		apiError(w, http.StatusNotFound, "not found")
		return
	}
	a.m.uploadsHandler(w, r)
}

// statusWriter remembers the status an answer was given.
type statusWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (s *statusWriter) WriteHeader(code int) {
	if !s.wrote {
		s.status, s.wrote = code, true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusWriter) Write(b []byte) (int, error) {
	s.wrote = true
	return s.ResponseWriter.Write(b)
}

func (s *statusWriter) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// ---- calling a story from inside (the tools) ----

// bufferWriter collects an answer in memory.
type bufferWriter struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (b *bufferWriter) Header() http.Header { return b.header }
func (b *bufferWriter) WriteHeader(code int) {
	if b.status == 0 {
		b.status = code
	}
}
func (b *bufferWriter) Write(p []byte) (int, error) {
	if b.status == 0 {
		b.status = http.StatusOK
	}
	if b.body.Len() > 16<<20 {
		return 0, io.ErrShortWrite
	}
	return b.body.Write(p)
}

// call runs one request on a story for a tool, with the same checks and
// log as a request from outside. body is JSON (a value) or a form
// (url.Values-like map), or nil.
func (a *apiServer) call(r *http.Request, k *apiKey, story, method, path string, body []byte, contentType, via string) (int, []byte) {
	path = strings.TrimPrefix(strings.TrimPrefix(path, "/"), "api/")
	rest, query, _ := strings.Cut(path, "?")
	var rd io.Reader = http.NoBody
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(r.Context(), method, "/stories/"+story+"/"+rest, rd)
	if err != nil {
		return http.StatusBadRequest, []byte(`{"error":"invalid path"}`)
	}
	req.URL.RawQuery = query
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.RemoteAddr = r.RemoteAddr
	bw := &bufferWriter{header: http.Header{}}
	a.forward(bw, req, k, story, rest, via)
	if bw.status == 0 {
		bw.status = http.StatusOK
	}
	return bw.status, bw.body.Bytes()
}

// callJSON is call with a JSON body, its answer decoded into out (when the
// call succeeded and out isn't nil). The error is the answer's text.
func (a *apiServer) callJSON(r *http.Request, k *apiKey, story, method, path string, in, out any, via string) (int, error) {
	var body []byte
	ct := ""
	if in != nil {
		body, _ = json.Marshal(in)
		ct = "application/json"
	}
	status, resp := a.call(r, k, story, method, path, body, ct, via)
	if status >= 300 {
		return status, apiCallError(status, resp)
	}
	if out != nil {
		if err := json.Unmarshal(resp, out); err != nil {
			return status, err
		}
	}
	return status, nil
}

type apiCallErr struct {
	status int
	msg    string
}

func (e *apiCallErr) Error() string { return e.msg }

func apiCallError(status int, body []byte) error {
	msg := strings.TrimSpace(string(body))
	var j struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &j) == nil && j.Error != "" {
		msg = j.Error
	}
	if len(msg) > 2000 {
		msg = msg[:2000]
	}
	if msg == "" {
		msg = http.StatusText(status)
	}
	return &apiCallErr{status: status, msg: msg}
}

func contextWithKey(r *http.Request, k *apiKey) context.Context {
	return context.WithValue(r.Context(), apiKeyCtx{}, k)
}

// keyOf is the request's key; ServeHTTP lets no request through without one.
func keyOf(r *http.Request) *apiKey {
	k, _ := r.Context().Value(apiKeyCtx{}).(*apiKey)
	if k == nil {
		return &apiKey{Stories: []string{"-"}} // reaches nothing, writes nothing
	}
	return k
}
