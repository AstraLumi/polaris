package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func spellingWords(t *testing.T, h http.Handler, method, url, body string) (int, []string) {
	t.Helper()
	rec := do(t, h, method, url, strings.NewReader(body), "application/json")
	var out struct {
		Words []string `json:"words"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out.Words
}

// The story's added words: empty at first, added as written (trimmed,
// sorted ignoring case, no duplicates), removed, kept per story.
func TestSpellingWords(t *testing.T) {
	_, h, base := planetSetup(t)
	url := base + "/spelling/words"
	if c, w := spellingWords(t, h, "GET", url, ""); c != 200 || w == nil || len(w) != 0 {
		t.Fatalf("empty at first: %d %v", c, w)
	}
	c, w := spellingWords(t, h, "POST", url, `{"add":[" Zarathul ","mira’s","Ælfwine","polish","Polish","Zarathul"]}`)
	if c != 200 || strings.Join(w, ",") != "mira’s,Polish,polish,Zarathul,Ælfwine" {
		t.Fatalf("added: %d %v", c, w)
	}
	if _, w2 := spellingWords(t, h, "GET", url, ""); strings.Join(w2, ",") != strings.Join(w, ",") {
		t.Errorf("read back: %v", w2)
	}
	if c, w := spellingWords(t, h, "POST", url, `{"remove":["polish","nothing"],"add":["Hollowmere"]}`); c != 200 || strings.Join(w, ",") != "Hollowmere,mira’s,Polish,Zarathul,Ælfwine" {
		t.Errorf("removed one, added one: %d %v", c, w)
	}
	for _, bad := range []string{`{"add":[""]}`, `{"add":["two words"]}`, `{"add":["1234"]}`, `{"add":["` + strings.Repeat("a", 65) + `"]}`, `{"add":["tab\there"]}`, `not json`} {
		if c, _ := spellingWords(t, h, "POST", url, bad); c != 400 {
			t.Errorf("%s: %d", bad, c)
		}
	}
	// Another story has its own list.
	s2 := createStory(t, h, map[string]string{"name": "Other"})
	if _, w := spellingWords(t, h, "GET", "/api/s/"+s2.ID+"/spelling/words", ""); len(w) != 0 {
		t.Errorf("other story: %v", w)
	}
}

func TestSpellingWordsLimit(t *testing.T) {
	_, h, base := planetSetup(t)
	url := base + "/spelling/words"
	words := make([]string, maxSpellingWords)
	for i := range words {
		words[i] = fmt.Sprintf(`"w%dx"`, i)
	}
	if c, w := spellingWords(t, h, "POST", url, `{"add":[`+strings.Join(words, ",")+`]}`); c != 200 || len(w) != maxSpellingWords {
		t.Fatalf("at the limit: %d %d", c, len(w))
	}
	if c, _ := spellingWords(t, h, "POST", url, `{"add":["onemore"]}`); c != 400 {
		t.Errorf("past the limit: %d", c)
	}
	if c, w := spellingWords(t, h, "POST", url, `{"add":["onemore"],"remove":["w0x"]}`); c != 200 || len(w) != maxSpellingWords {
		t.Errorf("one out, one in: %d %d", c, len(w))
	}
}
