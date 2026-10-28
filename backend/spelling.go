package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// The spellchecker's per-story dictionary (docs/EDITOR.md, "Spellchecker"):
// the words the writer added with "Add to dictionary", one JSON list in
// app_settings. The dictionaries themselves and the checking live in the
// browser (frontend/src/editor/hunspell.js); the story's names count as
// words there without being stored here.
const settingSpellingWords = "spelling_words"

const (
	maxSpellingWords     = 5000
	maxSpellingWordRunes = 64
)

// One story's list is read, changed and written back in one go; two tabs
// adding words at once must not lose one.
var spellingMu sync.Mutex

// readSpellingWords gives the story's added words, sorted, never nil.
func readSpellingWords(db *sql.DB) []string {
	var raw string
	words := []string{}
	if db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, settingSpellingWords).Scan(&raw) == nil {
		json.Unmarshal([]byte(raw), &words)
	}
	if words == nil {
		words = []string{}
	}
	return words
}

// cleanSpellingWord trims a word and refuses one that is empty, too long,
// holds a space or a control character, or has no letter. The browser
// sends it in NFC.
func cleanSpellingWord(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" || !utf8.ValidString(s) || utf8.RuneCountInString(s) > maxSpellingWordRunes {
		return "", false
	}
	letter := false
	for _, r := range s {
		if unicode.IsSpace(r) || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return "", false
		}
		if unicode.IsLetter(r) {
			letter = true
		}
	}
	return s, letter
}

// sortWords sorts ignoring case, then by the exact text ("Polish" before
// "polish"), so the list is always in the same order.
func sortWords(words []string) {
	sort.Slice(words, func(i, j int) bool {
		a, b := strings.ToLower(words[i]), strings.ToLower(words[j])
		if a != b {
			return a < b
		}
		return words[i] < words[j]
	})
}

// GET /api/spelling/words: { "words": [...] }.
func getSpellingWordsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string][]string{"words": readSpellingWords(db)})
	}
}

// POST /api/spelling/words: { "add": [...], "remove": [...] } → { "words" }.
// Words are kept as written ("Polish" and "polish" are two words); adding
// one already there or removing one that isn't changes nothing.
func updateSpellingWordsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Add    []string `json:"add"`
			Remove []string `json:"remove"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		var add []string
		for _, s := range in.Add {
			c, ok := cleanSpellingWord(s)
			if !ok {
				http.Error(w, "a word is 1 to 64 characters, with a letter and no spaces", http.StatusBadRequest)
				return
			}
			add = append(add, c)
		}
		remove := map[string]bool{}
		for _, s := range in.Remove {
			remove[strings.TrimSpace(s)] = true
		}

		spellingMu.Lock()
		defer spellingMu.Unlock()
		have := map[string]bool{}
		words := []string{}
		for _, s := range readSpellingWords(db) {
			if !remove[s] && !have[s] {
				have[s] = true
				words = append(words, s)
			}
		}
		for _, s := range add {
			if !have[s] && !remove[s] {
				have[s] = true
				words = append(words, s)
			}
		}
		if len(words) > maxSpellingWords {
			http.Error(w, "the story's dictionary holds at most 5000 words", http.StatusBadRequest)
			return
		}
		sortWords(words)
		raw, _ := json.Marshal(words)
		if _, err := db.Exec(
			`INSERT INTO app_settings (key, value) VALUES (?, ?)
			 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
			settingSpellingWords, string(raw),
		); err != nil {
			log.Printf("save spelling words: %v", err)
			http.Error(w, "failed to save the dictionary", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string][]string{"words": words})
	}
}
