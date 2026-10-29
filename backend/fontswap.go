package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"unicode"
)

// Swapping a font in a book's text, from the Book and page panel's "Fonts and
// licenses" (docs/EDITOR.md):
//
//	POST /api/books/{volume}/fonts/swap
//	{family, what, chars, to, label}
//
// what: "spaces" (the family's whitespace), "chars" (the characters in
// chars, such as symbols), or "all". to: "" for the body font (the font mark
// goes, so the paragraph's style decides), else another family. Only text
// in a font mark naming family (any case) changes. Every chapter it changes
// keeps its text first as a version named label, like a page type switch.

// isFontSpace: what counts as a space for "spaces" (any Unicode space, the
// no-break and zero-width ones too).
func isFontSpace(r rune) bool {
	return unicode.IsSpace(r) || unicode.Is(unicode.Zs, r) || r == 0x200b || r == 0xfeff
}

// isFontSymbol: a character the panel lists as a symbol of a font (math,
// currency, arrows, shapes, dingbats, emoji…, and private-use characters,
// which old Symbol and Wingdings text in Word files uses).
func isFontSymbol(r rune) bool {
	return unicode.IsSymbol(r) || unicode.Is(unicode.Co, r)
}

// maxFontSymbols caps the symbols listed per font.
const maxFontSymbols = 40

type fontSymbol struct {
	Char  string `json:"char"`
	Count int    `json:"count"`
}

type fontChars struct {
	spaces  int
	symbols map[string]int
}

// fontMarkFamily is a node's font mark family (trimmed), or "".
func fontMarkFamily(n *pmNode) string {
	for _, m := range n.Marks {
		if f, ok := m.Attrs["family"].(string); ok && m.Type == "font" {
			return strings.TrimSpace(f)
		}
	}
	return ""
}

// docFontChars counts each font's spaces and symbols in a doc (keyed as
// first written, like docFontRuns).
func docFontChars(d *pmNode) map[string]*fontChars {
	names := map[string]string{}
	out := map[string]*fontChars{}
	pmWalk(d, func(n *pmNode) {
		if n.Type != "text" {
			return
		}
		fam := fontMarkFamily(n)
		if fam == "" {
			return
		}
		key := strings.ToLower(fam)
		if names[key] == "" {
			names[key] = fam
			out[fam] = &fontChars{symbols: map[string]int{}}
		}
		c := out[names[key]]
		for _, r := range n.Text {
			if isFontSpace(r) {
				c.spaces++
			} else if isFontSymbol(r) {
				c.symbols[string(r)]++
			}
		}
	})
	return out
}

// addFontChars adds a doc's counts to a use.
func addFontChars(u *bookFontUse, c *fontChars) {
	u.Spaces += c.spaces
	for ch, n := range c.symbols {
		found := false
		for i := range u.Symbols {
			if u.Symbols[i].Char == ch {
				u.Symbols[i].Count += n
				found = true
				break
			}
		}
		if !found {
			u.Symbols = append(u.Symbols, fontSymbol{ch, n})
		}
	}
}

// sortFontSymbols puts the most used first and keeps maxFontSymbols.
func sortFontSymbols(list []fontSymbol) []fontSymbol {
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].Count != list[j].Count {
			return list[i].Count > list[j].Count
		}
		return list[i].Char < list[j].Char
	})
	if len(list) > maxFontSymbols {
		list = list[:maxFontSymbols]
	}
	return list
}

// swapDocFont changes, in a doc, the text in family that pick chooses to
// the family to ("" removes the font mark), and says how many pieces of
// text changed. Neighbouring text that ends up with the same marks is
// joined again; untouched paragraphs keep their nodes as they were.
func swapDocFont(d *pmNode, family string, pick func(rune) bool, to string) int {
	want := strings.ToLower(strings.TrimSpace(family))
	changed := 0
	pmWalk(d, func(n *pmNode) {
		var out []*pmNode
		touched := false
		for _, c := range n.Content {
			if c.Type != "text" || strings.ToLower(fontMarkFamily(c)) != want {
				out = append(out, c)
				continue
			}
			// Split the text into pieces that pick chooses and the rest.
			var piece []rune
			var picked bool
			flush := func() {
				if len(piece) == 0 {
					return
				}
				p := *c
				p.Text = string(piece)
				if picked {
					p.Marks = swapFontMark(c.Marks, to)
					changed++
					touched = true
				}
				out = append(out, &p)
				piece = nil
			}
			for _, r := range c.Text {
				if p := pick(r); p != picked && len(piece) > 0 {
					flush()
					picked = p
				} else {
					picked = p
				}
				piece = append(piece, r)
			}
			flush()
		}
		if touched {
			n.Content = joinTextNodes(out)
		}
	})
	return changed
}

// swapFontMark: marks with the font mark's family set to to, or without it.
func swapFontMark(marks []pmMark, to string) []pmMark {
	var out []pmMark
	for _, m := range marks {
		if m.Type != "font" {
			out = append(out, m)
			continue
		}
		if to == "" {
			continue
		}
		attrs := map[string]any{}
		for k, v := range m.Attrs {
			attrs[k] = v
		}
		attrs["family"] = to
		m.Attrs = attrs
		out = append(out, m)
	}
	return out
}

// joinTextNodes joins neighbouring text nodes with the same marks.
func joinTextNodes(list []*pmNode) []*pmNode {
	var out []*pmNode
	for _, n := range list {
		if k := len(out) - 1; k >= 0 && n.Type == "text" && out[k].Type == "text" &&
			len(n.Marks) == len(out[k].Marks) && (len(n.Marks) == 0 || reflect.DeepEqual(n.Marks, out[k].Marks)) &&
			len(n.Attrs) == 0 && len(out[k].Attrs) == 0 && len(n.Extra) == 0 && len(out[k].Extra) == 0 {
			j := *out[k]
			j.Text += n.Text
			out[k] = &j
			continue
		}
		out = append(out, n)
	}
	return out
}

type fontSwapRequest struct {
	Family string `json:"family"`
	What   string `json:"what"`
	Chars  string `json:"chars"`
	To     string `json:"to"`
	Label  string `json:"label"`
}

func swapBookFontHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vol, ok := bookVolume(r)
		if !ok {
			http.Error(w, "book not found", http.StatusNotFound)
			return
		}
		if _, err := bookTitle(db, vol); err != nil {
			if errors.Is(err, errNoBook) {
				http.Error(w, "book not found", http.StatusNotFound)
			} else {
				http.Error(w, "failed to change the font", http.StatusInternalServerError)
			}
			return
		}
		var req fontSwapRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		req.Family, req.To, req.Label = strings.TrimSpace(req.Family), strings.TrimSpace(req.To), strings.TrimSpace(req.Label)
		if req.Family == "" || len(req.Family) > 200 || len(req.To) > 200 || len(req.Label) > 200 || len([]rune(req.Chars)) > 200 {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		if strings.EqualFold(req.Family, req.To) {
			http.Error(w, "that is the same font", http.StatusBadRequest)
			return
		}
		var pick func(rune) bool
		switch req.What {
		case "spaces":
			pick = isFontSpace
		case "chars":
			set := map[rune]bool{}
			for _, c := range req.Chars {
				set[c] = true
			}
			if len(set) == 0 {
				http.Error(w, "invalid request", http.StatusBadRequest)
				return
			}
			pick = func(c rune) bool { return set[c] }
		case "all":
			pick = func(rune) bool { return true }
		default:
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		if req.Label == "" {
			req.Label = "Before changing the font " + req.Family
		}

		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to change the font", http.StatusInternalServerError)
			return
		}
		fail := func(err error) {
			tx.Rollback()
			log.Printf("swap font %q in book %d: %v", req.Family, vol, err)
			http.Error(w, "failed to change the font", http.StatusInternalServerError)
		}
		type text struct {
			id    int64
			doc   string
			words int
		}
		var texts []text
		rows, err := tx.Query(`SELECT c.id, t.doc, t.words FROM chapter_texts t JOIN chapters c ON c.id = t.chapter_id
			WHERE c.volume_id IS ? AND t.doc LIKE '%"font"%' ORDER BY c.position, c.id`, volumeNull(vol))
		if err != nil {
			fail(err)
			return
		}
		for rows.Next() {
			var t text
			if err := rows.Scan(&t.id, &t.doc, &t.words); err != nil {
				rows.Close()
				fail(err)
				return
			}
			texts = append(texts, t)
		}
		rows.Close()
		now := editorNow()
		out := struct {
			Chapters int `json:"chapters"`
			Pieces   int `json:"pieces"`
		}{}
		for _, t := range texts {
			var d pmNode
			if json.Unmarshal([]byte(t.doc), &d) != nil {
				continue
			}
			n := swapDocFont(&d, req.Family, pick, req.To)
			if n == 0 {
				continue
			}
			if err := validatePMDoc(&d); err != nil {
				fail(err)
				return
			}
			b, err := json.Marshal(&d)
			if err != nil {
				fail(err)
				return
			}
			if _, err := keepVersion(tx, t.id, t.doc, t.words, req.Label, false, now); err != nil {
				fail(err)
				return
			}
			if _, err := tx.Exec(`UPDATE chapter_texts SET doc = ?, version = version + 1, updated_at = ?, auto_since = ? WHERE chapter_id = ?`,
				string(b), sqlTime(now), sqlTime(now), t.id); err != nil {
				fail(err)
				return
			}
			if err := indexChapterText(tx, t.id, string(b)); err != nil { // chapterindex.go
				fail(err)
				return
			}
			out.Chapters++
			out.Pieces += n
		}
		if err := tx.Commit(); err != nil {
			fail(err)
			return
		}
		writeJSON(w, out)
	}
}
