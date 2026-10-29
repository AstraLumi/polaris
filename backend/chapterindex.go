package main

// What each chapter's text mentions and who speaks in it (stage 4 of the
// writing editor, docs/EDITOR.md): an index kept beside chapter_texts and
// rebuilt from the doc on every write of a chapter's text (a save, a
// restore, a .docx import), so the "Appears in" lists, a character's Lines
// and a chapter's "Who speaks" are a query away.
//
//	chapter_links: (chapter, type, item) and how many times the chapter
//	    names it: each story_link run (text next to text with the same link
//	    is one mention) and each dialogue spoken by a character (type
//	    "character") counts one.
//	chapter_lines: every dialogue of the chapter in reading order
//	    (position 0, 1, 2… among the chapter's dialogues): the character
//	    (null for a speaker who isn't one), the speaker and manner as
//	    written and the line's text.
//
// Both go with their chapter (ON DELETE CASCADE). Items are not foreign
// keys (a link names any kind of item); the readers only list items that
// exist, and the next save of the chapter drops what is gone.

import (
	"cmp"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
)

const chapterLinksTableSQL = `CREATE TABLE chapter_links (
	chapter_id INTEGER NOT NULL REFERENCES chapters(id) ON DELETE CASCADE,
	type       TEXT NOT NULL,
	item_id    INTEGER NOT NULL,
	count      INTEGER NOT NULL DEFAULT 1,
	PRIMARY KEY (chapter_id, type, item_id)
)`

const chapterLinesTableSQL = `CREATE TABLE chapter_lines (
	chapter_id   INTEGER NOT NULL REFERENCES chapters(id) ON DELETE CASCADE,
	position     INTEGER NOT NULL,
	character_id INTEGER,
	speaker      TEXT NOT NULL DEFAULT '',
	manner       TEXT NOT NULL DEFAULT '',
	text         TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (chapter_id, position)
)`

// schema45Statements create what schema 45 added. The fresh schema runs them
// too, after the editor's tables.
var schema45Statements = []string{
	chapterLinksTableSQL,
	`CREATE INDEX idx_chapter_links_item ON chapter_links(type, item_id)`,
	chapterLinesTableSQL,
	`CREATE INDEX idx_chapter_lines_character ON chapter_lines(character_id)`,
}

// migrateChapterIndex is schema 45's migration: the tables, then the index
// of every chapter text already written.
func migrateChapterIndex(tx *sql.Tx) error {
	for _, q := range schema45Statements {
		if _, err := tx.Exec(q); err != nil {
			return err
		}
	}
	rows, err := tx.Query(`SELECT t.chapter_id, t.doc FROM chapter_texts t JOIN chapters c ON c.id = t.chapter_id`)
	if err != nil {
		return err
	}
	type text struct {
		id  int64
		doc string
	}
	var all []text
	for rows.Next() {
		var x text
		if err := rows.Scan(&x.id, &x.doc); err != nil {
			rows.Close()
			return err
		}
		all = append(all, x)
	}
	rows.Close()
	for _, x := range all {
		// Schema 45's index only: the comments' table comes with schema 46.
		if err := indexChapterLinks(tx, x.id, x.doc); err != nil {
			return err
		}
	}
	return nil
}

// pmLinkCount is how many times a chapter names an item.
type pmLinkCount struct {
	pmStoryLink
	Count int
}

// pmDialogue is one dialogue of a doc, as chapter_lines holds it.
type pmDialogue struct {
	Character int64 // 0: not a character
	Speaker   string
	Manner    string
	Text      string
}

func dialogueOf(n *pmNode) pmDialogue {
	d := pmDialogue{Speaker: attrText(n, "speaker"), Manner: attrText(n, "manner"), Text: strings.TrimSpace(pmTextblockText(n, "\n", " "))}
	if id, ok := pmInt(n.Attrs["character"]); ok && id > 0 {
		d.Character = id
	}
	return d
}

func attrText(n *pmNode, k string) string {
	s, _ := n.Attrs[k].(string)
	return strings.Join(strings.Fields(s), " ")
}

// dialogueLead is what goes before a dialogue's line as plain text:
// "Speaker, manner: ", "Speaker: " or "".
func dialogueLead(n *pmNode) string {
	sp, m := attrText(n, "speaker"), attrText(n, "manner")
	switch {
	case sp != "" && m != "":
		return sp + ", " + m + ": "
	case sp != "":
		return sp + ": "
	case m != "":
		return m + ": "
	}
	return ""
}

// speakerMark is the speaker mark of a text node (the writing editor's
// "Tag character", docs/EDITOR.md), or nil.
func speakerMark(n *pmNode) *pmMark {
	for i := range n.Marks {
		if n.Marks[i].Type == "speaker" {
			return &n.Marks[i]
		}
	}
	return nil
}

// markedLines lists the runs of a paragraph that carry a speaker mark: text
// nodes in a row under the same character and name are one line.
func markedLines(n *pmNode) []pmDialogue {
	var out []pmDialogue
	var cur *pmDialogue
	var key string
	flush := func() {
		if cur != nil {
			cur.Text = strings.Join(strings.Fields(cur.Text), " ")
			if cur.Text != "" {
				out = append(out, *cur)
			}
			cur = nil
		}
	}
	for _, c := range n.Content {
		if c == nil {
			continue
		}
		if c.Type != "text" {
			if cur != nil && (c.Type == "hard_break" || c.Type == "tab") {
				cur.Text += " "
			}
			continue
		}
		m := speakerMark(c)
		if m == nil {
			flush()
			continue
		}
		d := pmDialogue{Speaker: attrText(&pmNode{Attrs: m.Attrs}, "speaker")}
		if id, ok := pmInt(m.Attrs["character"]); ok && id > 0 {
			d.Character = id
		}
		k := fmt.Sprint(d.Character, "|", d.Speaker)
		if cur != nil && k != key {
			flush()
		}
		if cur == nil {
			cur, key = &d, k
		}
		cur.Text += c.Text
	}
	flush()
	return out
}

// docDialogues lists a doc's dialogues in reading order: Script Mode's
// dialogue nodes and the speaker-tagged runs of prose paragraphs.
func docDialogues(doc *pmNode) []pmDialogue {
	var out []pmDialogue
	pmWalk(doc, func(n *pmNode) {
		switch n.Type {
		case "dialogue":
			out = append(out, dialogueOf(n))
		case "paragraph":
			out = append(out, markedLines(n)...)
		}
	})
	return out
}

// docLinkCounts counts each item a doc names, in order of first mention:
// story_link runs and the characters of its dialogues. A footnote's or
// endnote's text (stage 5) names items like any other text.
func docLinkCounts(doc *pmNode) []pmLinkCount {
	var out []pmLinkCount
	at := map[pmStoryLink]int{}
	add := func(l pmStoryLink) {
		if i, ok := at[l]; ok {
			out[i].Count++
			return
		}
		at[l] = len(out)
		out = append(out, pmLinkCount{l, 1})
	}
	pmWalk(doc, func(n *pmNode) {
		switch n.Type {
		case "paragraph", "heading", "dialogue":
		default:
			return
		}
		if n.Type == "dialogue" {
			if d := dialogueOf(n); d.Character > 0 {
				add(pmStoryLink{"character", d.Character})
			}
		}
		if n.Type == "paragraph" {
			for _, d := range markedLines(n) {
				if d.Character > 0 {
					add(pmStoryLink{"character", d.Character})
				}
			}
		}
		var prev *pmStoryLink
		for _, c := range n.Content {
			if c != nil && c.Type == "footnote" {
				continue // a note's number doesn't split a mention; its own text is walked on its own
			}
			l := storyLinkOf(c)
			if l != nil && (prev == nil || *prev != *l) {
				add(*l)
			}
			prev = l
		}
	})
	return out
}

func storyLinkOf(n *pmNode) *pmStoryLink {
	if n == nil || n.Type != "text" {
		return nil
	}
	for _, m := range n.Marks {
		if m.Type != "story_link" {
			continue
		}
		t, _ := m.Attrs["type"].(string)
		id, ok := pmInt(m.Attrs["id"])
		if t == "" || !ok || id < 1 {
			return nil
		}
		return &pmStoryLink{t, id}
	}
	return nil
}

// indexChapterText rebuilds a chapter's links and lines from its stored doc
// and keeps its comments in step with it (syncChapterComments, comments.go).
// A doc that can't be read leaves the chapter with no links or lines and its
// comments as they were.
func indexChapterText(tx editorDB, chapterID int64, doc string) error {
	d, err := indexChapterDoc(tx, chapterID, doc)
	if err != nil || d == nil {
		return err
	}
	return syncChapterComments(tx, chapterID, d)
}

// indexChapterLinks rebuilds a chapter's links and lines from its stored doc
// (schema 45's part of indexChapterText).
func indexChapterLinks(tx editorDB, chapterID int64, doc string) error {
	_, err := indexChapterDoc(tx, chapterID, doc)
	return err
}

// indexChapterDoc rebuilds a chapter's links and lines and gives the doc it
// read (nil when it can't be read: the chapter then has none).
func indexChapterDoc(tx editorDB, chapterID int64, doc string) (*pmNode, error) {
	if _, err := tx.Exec(`DELETE FROM chapter_links WHERE chapter_id = ?`, chapterID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`DELETE FROM chapter_lines WHERE chapter_id = ?`, chapterID); err != nil {
		return nil, err
	}
	var d pmNode
	if err := json.Unmarshal([]byte(doc), &d); err != nil {
		log.Printf("index chapter %d: %v", chapterID, err)
		return nil, nil
	}
	for _, l := range docLinkCounts(&d) {
		if _, err := tx.Exec(`INSERT INTO chapter_links (chapter_id, type, item_id, count) VALUES (?, ?, ?, ?)`,
			chapterID, l.Type, l.ID, l.Count); err != nil {
			return nil, err
		}
	}
	for i, x := range docDialogues(&d) {
		var who any
		if x.Character > 0 {
			who = x.Character
		}
		if _, err := tx.Exec(`INSERT INTO chapter_lines (chapter_id, position, character_id, speaker, manner, text) VALUES (?, ?, ?, ?, ?, ?)`,
			chapterID, i, who, x.Speaker, x.Manner, x.Text); err != nil {
			return nil, err
		}
	}
	return &d, nil
}

// ---- Reading ------------------------------------------------------------------

// chapterPlace is where a chapter sits in the story, for the lists.
type chapterPlace struct {
	ChapterID   int64  `json:"chapter_id"`
	Title       string `json:"title"`
	Number      int    `json:"number"`
	VolumeID    int64  `json:"volume_id"` // 0: no volume (the editor's book 0)
	VolumeTitle string `json:"volume_title"`
	order       int
}

// chapterPlaces gives every chapter's place, keyed by id, in reading order.
func chapterPlaces(db *sql.DB) (map[int64]chapterPlace, error) {
	idx, err := loadChapterIndex(db)
	if err != nil {
		return nil, err
	}
	vols := map[int64]string{}
	for _, v := range idx.Volumes {
		vols[v.ID] = v.Title
	}
	out := map[int64]chapterPlace{}
	for i, c := range idx.Chapters {
		p := chapterPlace{ChapterID: c.ID, Title: c.Title, Number: c.Number, order: i}
		if c.VolumeID != nil {
			p.VolumeID = *c.VolumeID
			p.VolumeTitle = vols[*c.VolumeID]
		}
		out[c.ID] = p
	}
	return out, nil
}

type appearsInRow struct {
	chapterPlace
	Count int `json:"count"`
}

// appearsInHandler: GET /api/appears/{type}/{id}, the chapters whose text
// names an item (links, and lines spoken for a character), in reading order.
func appearsInHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		typ := r.PathValue("type")
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || !pmStoryTypeRe.MatchString(typ) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		places, err := chapterPlaces(db)
		if err != nil {
			log.Printf("appears in: %v", err)
			http.Error(w, "failed to load the chapters", http.StatusInternalServerError)
			return
		}
		rows, err := db.Query(`SELECT chapter_id, count FROM chapter_links WHERE type = ? AND item_id = ?`, typ, id)
		if err != nil {
			http.Error(w, "failed to load the chapters", http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		out := []appearsInRow{}
		for rows.Next() {
			var ch int64
			var n int
			if rows.Scan(&ch, &n) != nil {
				continue
			}
			if p, ok := places[ch]; ok {
				out = append(out, appearsInRow{p, n})
			}
		}
		sortByOrder(out, func(x appearsInRow) int { return x.order })
		writeJSON(w, out)
	}
}

type lineRow struct {
	chapterPlace
	Position int    `json:"position"`
	Speaker  string `json:"speaker"`
	Manner   string `json:"manner"`
	Text     string `json:"text"`
}

// characterLinesHandler: GET /api/characters/{id}/lines, everything the
// character says, in reading order.
func characterLinesHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "character not found", http.StatusNotFound)
			return
		}
		places, err := chapterPlaces(db)
		if err != nil {
			log.Printf("character lines: %v", err)
			http.Error(w, "failed to load the lines", http.StatusInternalServerError)
			return
		}
		rows, err := db.Query(`SELECT chapter_id, position, speaker, manner, text FROM chapter_lines WHERE character_id = ?`, id)
		if err != nil {
			http.Error(w, "failed to load the lines", http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		out := []lineRow{}
		for rows.Next() {
			var ch int64
			var l lineRow
			if rows.Scan(&ch, &l.Position, &l.Speaker, &l.Manner, &l.Text) != nil {
				continue
			}
			if p, ok := places[ch]; ok {
				l.chapterPlace = p
				out = append(out, l)
			}
		}
		sortByOrder(out, func(x lineRow) int { return x.order*1_000_000 + x.Position })
		writeJSON(w, out)
	}
}

type speakerRow struct {
	CharacterID *int64 `json:"character_id"`
	Name        string `json:"name"` // the character's name now, or the speaker as written
	Lines       int    `json:"lines"`
	Words       int    `json:"words"`
	First       int    `json:"first"` // the position of their first line
}

// chapterSpeakersHandler: GET /api/chapters/{id}/speakers, who speaks in
// the chapter and how much, by number of lines.
func chapterSpeakersHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := chapterIDOf(r)
		if !ok {
			http.Error(w, "chapter not found", http.StatusNotFound)
			return
		}
		rows, err := db.Query(`SELECT l.position, l.character_id, l.speaker, l.text,
				(SELECT v.name FROM character_versions v WHERE v.character_id = l.character_id AND v.is_current = 1)
			FROM chapter_lines l WHERE l.chapter_id = ? ORDER BY l.position`, id)
		if err != nil {
			http.Error(w, "failed to load the speakers", http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		out := []speakerRow{}
		at := map[string]int{}
		for rows.Next() {
			var pos int
			var who sql.NullInt64
			var speaker, text string
			var name sql.NullString
			if rows.Scan(&pos, &who, &speaker, &text, &name) != nil {
				continue
			}
			key := "s:" + strings.ToLower(speaker)
			row := speakerRow{Name: speaker, First: pos}
			if who.Valid && name.Valid {
				key = "c:" + strconv.FormatInt(who.Int64, 10)
				c := who.Int64
				row.CharacterID, row.Name = &c, name.String
			}
			i, seen := at[key]
			if !seen {
				i = len(out)
				at[key] = i
				out = append(out, row)
			}
			out[i].Lines++
			out[i].Words += countWords(text)
		}
		sortByOrder(out, func(x speakerRow) int { return -x.Lines*1_000_000 + x.First })
		writeJSON(w, out)
	}
}

func sortByOrder[T any](list []T, key func(T) int) {
	slices.SortStableFunc(list, func(a, b T) int { return cmp.Compare(key(a), key(b)) })
}
