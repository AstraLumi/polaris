package main

// API keys: what lets a program (an LLM's tools, a script) use Polaris
// through the API listener (apiserver.go). Polaris has no login, so the keys
// are made and revoked only on the app's own port, never on the API's: a
// leaked key can't make more keys or keep its owner out.
//
// A key is "pol_" and 43 characters of random base64url (256 bits). It is
// shown once, when it is made; the registry keeps only its SHA-256, in hex,
// which is also the form a gateway in front may ask for (update-server's
// apps.json keySha256). Each key says whether it may change things, which
// stories it reaches (none listed: all of them, new ones too) and,
// optionally, until when it works.
//
// Storage: two tables in the registry (<DATA_DIR>/stories.db), made in place
// like the fonts library's.
//
// API (the app's port only):
//
//	GET    /api/api-access            whether the API is on, its port, the keys, recent activity
//	POST   /api/api-access/keys       make a key {name, write, stories, expires}; the answer holds it once
//	DELETE /api/api-access/keys/{id}  revoke it, at once

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	apiKeyPrefix   = "pol_"
	maxAPIKeyName  = 60
	maxAPIKeys     = 50
	apiLogKept     = 1000 // rows of activity kept
	apiLogShown    = 100
	apiUsedEvery   = time.Minute // last_used_at is written at most this often per key
	apiKeysSQLText = `
CREATE TABLE IF NOT EXISTS api_keys (
	id           INTEGER PRIMARY KEY,
	name         TEXT NOT NULL,
	hint         TEXT NOT NULL,            -- the key's first characters, to tell keys apart
	hash         TEXT NOT NULL UNIQUE,     -- hex SHA-256 of the whole key
	can_write    INTEGER NOT NULL DEFAULT 0,
	stories      TEXT NOT NULL DEFAULT '', -- JSON list of story ids; '' = every story
	created_at   TEXT NOT NULL DEFAULT (datetime('now')),
	expires_at   TEXT NOT NULL DEFAULT '', -- '' = never
	last_used_at TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS api_log (
	id       INTEGER PRIMARY KEY,
	at       TEXT NOT NULL DEFAULT (datetime('now')),
	key_id   INTEGER NOT NULL,
	key_name TEXT NOT NULL,
	story_id TEXT NOT NULL DEFAULT '',
	method   TEXT NOT NULL,
	path     TEXT NOT NULL,
	status   INTEGER NOT NULL
);`
)

// apiKey is one key as the server holds it (never the key itself).
type apiKey struct {
	ID       int64    `json:"id"`
	Name     string   `json:"name"`
	Hint     string   `json:"hint"`
	Hash     string   `json:"sha256"`
	CanWrite bool     `json:"write"`
	Stories  []string `json:"stories"` // empty: every story
	Created  string   `json:"created_at"`
	Expires  string   `json:"expires_at"` // "" or YYYY-MM-DD (the key works through that day, UTC)
	LastUsed string   `json:"last_used_at"`
}

func (k *apiKey) expired(now time.Time) bool {
	if k.Expires == "" {
		return false
	}
	end, err := time.Parse("2006-01-02", k.Expires)
	return err != nil || !now.UTC().Before(end.Add(24*time.Hour))
}

// reaches: may this key use the story?
func (k *apiKey) reaches(storyID string) bool {
	if len(k.Stories) == 0 {
		return true
	}
	for _, s := range k.Stories {
		if s == storyID {
			return true
		}
	}
	return false
}

type apiLogRow struct {
	At      string `json:"at"`
	KeyID   int64  `json:"key_id"`
	KeyName string `json:"key_name"`
	Story   string `json:"story_id"`
	Method  string `json:"method"`
	Path    string `json:"path"`
	Status  int    `json:"status"`
}

// apiKeyStore holds every key in memory (looked up by hash on each request)
// and writes through to the registry.
type apiKeyStore struct {
	db *sql.DB

	mu     sync.Mutex
	byHash map[string]*apiKey
	used   map[int64]time.Time // when last_used_at was last written
}

func newAPIKeyStore(reg *sql.DB) (*apiKeyStore, error) {
	if _, err := reg.Exec(apiKeysSQLText); err != nil {
		return nil, err
	}
	s := &apiKeyStore{db: reg, used: map[int64]time.Time{}}
	return s, s.reload()
}

func (s *apiKeyStore) reload() error {
	rows, err := s.db.Query(`SELECT id, name, hint, hash, can_write, stories, created_at, expires_at, last_used_at
		FROM api_keys ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	m := map[string]*apiKey{}
	for rows.Next() {
		k := &apiKey{}
		var stories string
		if err := rows.Scan(&k.ID, &k.Name, &k.Hint, &k.Hash, &k.CanWrite, &stories, &k.Created, &k.Expires, &k.LastUsed); err != nil {
			return err
		}
		k.Stories = []string{}
		if stories != "" {
			json.Unmarshal([]byte(stories), &k.Stories)
		}
		m[k.Hash] = k
	}
	if err := rows.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	s.byHash = m
	s.mu.Unlock()
	return nil
}

func hashAPIKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// check finds the key a request carries. It gives a copy, or nil for a key
// that is unknown, revoked or expired.
func (s *apiKeyStore) check(key string, now time.Time) *apiKey {
	if !strings.HasPrefix(key, apiKeyPrefix) || len(key) > 200 {
		return nil
	}
	h := hashAPIKey(key)
	s.mu.Lock()
	k := s.byHash[h]
	var stamp bool
	if k != nil && !k.expired(now) && now.Sub(s.used[k.ID]) >= apiUsedEvery {
		s.used[k.ID] = now
		k.LastUsed = sqlTime(now)
		stamp = true
	}
	var out *apiKey
	if k != nil && !k.expired(now) {
		c := *k
		out = &c
	}
	s.mu.Unlock()
	if stamp {
		if _, err := s.db.Exec(`UPDATE api_keys SET last_used_at = ? WHERE id = ?`, sqlTime(now), k.ID); err != nil {
			log.Printf("api key used: %v", err)
		}
	}
	return out
}

func (s *apiKeyStore) list() []apiKey {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]apiKey, 0, len(s.byHash))
	for _, k := range s.byHash {
		out = append(out, *k)
	}
	// Newest first.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].ID > out[j-1].ID; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

type newAPIKeyIn struct {
	Name    string   `json:"name"`
	Write   bool     `json:"write"`
	Stories []string `json:"stories"`
	Expires string   `json:"expires"` // "" or YYYY-MM-DD
}

// create makes a key. The key itself is in the answer and nowhere else.
func (s *apiKeyStore) create(in newAPIKeyIn, known func(string) bool, now time.Time) (string, apiKey, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" || utf8.RuneCountInString(name) > maxAPIKeyName {
		return "", apiKey{}, errors.New("a key needs a name (60 characters at most)")
	}
	stories := []string{}
	seen := map[string]bool{}
	for _, id := range in.Stories {
		if !storyIDPattern.MatchString(id) || !known(id) {
			return "", apiKey{}, errors.New("story not found")
		}
		if !seen[id] {
			seen[id] = true
			stories = append(stories, id)
		}
	}
	if in.Expires != "" {
		d, err := time.Parse("2006-01-02", in.Expires)
		if err != nil || !now.UTC().Before(d.Add(24*time.Hour)) {
			return "", apiKey{}, errors.New("the expiry date must be today or later")
		}
	}
	if len(s.list()) >= maxAPIKeys {
		return "", apiKey{}, errors.New("there are already 50 keys; revoke one first")
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", apiKey{}, err
	}
	key := apiKeyPrefix + base64.RawURLEncoding.EncodeToString(b)
	storiesJSON := ""
	if len(stories) > 0 {
		j, _ := json.Marshal(stories)
		storiesJSON = string(j)
	}
	if _, err := s.db.Exec(`INSERT INTO api_keys (name, hint, hash, can_write, stories, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, name, key[:len(apiKeyPrefix)+6], hashAPIKey(key), in.Write, storiesJSON,
		sqlTime(now), in.Expires); err != nil {
		return "", apiKey{}, err
	}
	if err := s.reload(); err != nil {
		return "", apiKey{}, err
	}
	s.mu.Lock()
	k := *s.byHash[hashAPIKey(key)]
	s.mu.Unlock()
	return key, k, nil
}

func (s *apiKeyStore) revoke(id int64) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM api_keys WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, s.reload()
}

// note records something a key did (apiserver.go: every change, and every
// refusal of a known key).
func (s *apiKeyStore) note(k *apiKey, story, method, path string, status int) {
	if len(path) > 300 {
		path = path[:300]
	}
	if _, err := s.db.Exec(`INSERT INTO api_log (key_id, key_name, story_id, method, path, status) VALUES (?, ?, ?, ?, ?, ?)`,
		k.ID, k.Name, story, method, path, status); err != nil {
		log.Printf("api log: %v", err)
		return
	}
	s.db.Exec(`DELETE FROM api_log WHERE id <= (SELECT MAX(id) FROM api_log) - ?`, apiLogKept)
}

func (s *apiKeyStore) recent(n int) []apiLogRow {
	out := []apiLogRow{}
	rows, err := s.db.Query(`SELECT at, key_id, key_name, story_id, method, path, status FROM api_log ORDER BY id DESC LIMIT ?`, n)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var r apiLogRow
		if rows.Scan(&r.At, &r.KeyID, &r.KeyName, &r.Story, &r.Method, &r.Path, &r.Status) == nil {
			out = append(out, r)
		}
	}
	return out
}

// ---- the app's own routes ----

func (m *storyManager) registerAPIKeyRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/api-access", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"enabled": m.apiPort != "",
			"port":    m.apiPort,
			"keys":    m.keys.list(),
			"log":     m.keys.recent(apiLogShown),
		})
	})
	mux.HandleFunc("POST /api/api-access/keys", func(w http.ResponseWriter, r *http.Request) {
		var in newAPIKeyIn
		if !decodeJSON(w, r, &in) {
			return
		}
		known := func(id string) bool { _, _, err := m.get(id); return err == nil }
		key, k, err := m.keys.create(in, known, time.Now())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, map[string]any{"key": key, "info": k})
	})
	mux.HandleFunc("DELETE /api/api-access/keys/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "key not found", http.StatusNotFound)
			return
		}
		ok, err := m.keys.revoke(id)
		if err != nil {
			http.Error(w, "failed to revoke the key", http.StatusInternalServerError)
			return
		}
		if !ok {
			http.Error(w, "key not found", http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]bool{"revoked": true})
	})
}
