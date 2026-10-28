package main

// Comments on a chapter's text (stage 5 of the writing editor; docs/EDITOR.md,
// "Comments"): notes the writer leaves on their manuscript, like Word's. A
// comment is a row of chapter_comments (schema 46); the text it is about
// carries a `comment` mark with the row's id (pmdoc.go). Replies are rows
// with a parent_id (one level: a reply to a reply belongs to the same
// thread). Comment text is plain text: no wiki links, never Markdown.
//
//	GET    /api/chapters/{id}/comments   every comment of the chapter, by id
//	POST   /api/chapters/{id}/comments   { text, parent_id? } → the new row
//	PUT    /api/comments/{id}            { text?, resolved? } → the row
//	DELETE /api/comments/{id}            the comment and its replies
//
// Lifecycle. The client creates the row first and only then puts the mark
// in the text, so a mark never points at nothing it made. Every write of a
// chapter's text (indexChapterText → syncChapterComments) looks at which of
// the chapter's comments its text still holds:
//   - in the text: anchored (it was saved with its mark at least once);
//   - not in the text but in one of the chapter's saved versions: kept as
//     it is, for a restore to bring back;
//   - anchored once, now in neither: detached (detached_at), and deleted at
//     the first write of the text commentGrace later, so an undo or a
//     paste back in the meantime finds it still there;
//   - never anchored (its mark was never saved: the tab closed first):
//     deleted at the first write of the text commentGrace after it was made.
// A mark whose comment is gone (deleted, or pasted from another chapter) is
// drawn as plain text and dropped by the exports.

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxCommentText = 10000 // runes
	commentGrace   = 24 * time.Hour
)

// AUTOINCREMENT: an id is never used twice, so a mark left in an old version
// never points at a newer comment.
const chapterCommentsTableSQL = `CREATE TABLE chapter_comments (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	chapter_id  INTEGER NOT NULL REFERENCES chapters(id) ON DELETE CASCADE,
	parent_id   INTEGER REFERENCES chapter_comments(id) ON DELETE CASCADE,
	text        TEXT NOT NULL DEFAULT '',
	resolved    INTEGER NOT NULL DEFAULT 0,
	resolved_at TEXT,
	anchored    INTEGER NOT NULL DEFAULT 0,
	detached_at TEXT,
	created_at  TEXT NOT NULL DEFAULT (datetime('now')),
	updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
)`

// schema46Statements create what schema 46 added. The fresh schema runs them
// too, after the editor's tables.
var schema46Statements = []string{
	chapterCommentsTableSQL,
	`CREATE INDEX idx_chapter_comments_chapter ON chapter_comments(chapter_id)`,
	`CREATE INDEX idx_chapter_comments_parent ON chapter_comments(parent_id)`,
}

type commentOut struct {
	ID         int64   `json:"id"`
	ChapterID  int64   `json:"chapter_id"`
	ParentID   *int64  `json:"parent_id"`
	Text       string  `json:"text"`
	Resolved   bool    `json:"resolved"`
	ResolvedAt *string `json:"resolved_at"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}

const commentCols = `id, chapter_id, parent_id, text, resolved, resolved_at, created_at, updated_at`

func scanComment(s interface{ Scan(...any) error }) (commentOut, error) {
	var c commentOut
	var parent sql.NullInt64
	var resolvedAt sql.NullString
	err := s.Scan(&c.ID, &c.ChapterID, &parent, &c.Text, &c.Resolved, &resolvedAt, &c.CreatedAt, &c.UpdatedAt)
	if parent.Valid {
		c.ParentID = &parent.Int64
	}
	if resolvedAt.Valid {
		c.ResolvedAt = &resolvedAt.String
	}
	return c, err
}

func loadComment(db editorDB, id int64) (commentOut, error) {
	return scanComment(db.QueryRow(`SELECT `+commentCols+` FROM chapter_comments WHERE id = ?`, id))
}

// cleanCommentText checks a comment's text ("" when it is fine).
func cleanCommentText(s string) (string, string) {
	s = strings.TrimRight(strings.ReplaceAll(s, "\r\n", "\n"), " \t\r\n")
	if !utf8.ValidString(s) || strings.ContainsRune(s, 0) {
		return "", "the comment's text is not valid"
	}
	if utf8.RuneCountInString(s) > maxCommentText {
		return "", "that comment is too long"
	}
	return s, ""
}

func chapterExists(db editorDB, id int64) (bool, error) {
	var one int
	err := db.QueryRow(`SELECT 1 FROM chapters WHERE id = ?`, id).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func listCommentsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := chapterIDOf(r)
		if ok {
			ok, _ = chapterExists(db, id)
		}
		if !ok {
			http.Error(w, "chapter not found", http.StatusNotFound)
			return
		}
		rows, err := db.Query(`SELECT `+commentCols+` FROM chapter_comments WHERE chapter_id = ? ORDER BY id`, id)
		if err != nil {
			http.Error(w, "failed to load the comments", http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		out := []commentOut{}
		for rows.Next() {
			c, err := scanComment(rows)
			if err != nil {
				http.Error(w, "failed to load the comments", http.StatusInternalServerError)
				return
			}
			out = append(out, c)
		}
		writeJSON(w, out)
	}
}

func createCommentHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := chapterIDOf(r)
		if ok {
			ok, _ = chapterExists(db, id)
		}
		if !ok {
			http.Error(w, "chapter not found", http.StatusNotFound)
			return
		}
		var in struct {
			Text     string `json:"text"`
			ParentID *int64 `json:"parent_id"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&in); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		text, msg := cleanCommentText(in.Text)
		if msg != "" {
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		var parent any
		if in.ParentID != nil {
			// A reply joins the thread of the comment it answers (Word's
			// threads are one level deep).
			var chapter int64
			var grand sql.NullInt64
			err := db.QueryRow(`SELECT chapter_id, parent_id FROM chapter_comments WHERE id = ?`, *in.ParentID).Scan(&chapter, &grand)
			if err != nil || chapter != id {
				http.Error(w, "comment not found", http.StatusNotFound)
				return
			}
			if strings.TrimSpace(text) == "" {
				http.Error(w, "write the reply first", http.StatusBadRequest)
				return
			}
			parent = *in.ParentID
			if grand.Valid {
				parent = grand.Int64
			}
		}
		now := sqlTime(editorNow())
		res, err := db.Exec(`INSERT INTO chapter_comments (chapter_id, parent_id, text, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
			id, parent, text, now, now)
		if err != nil {
			log.Printf("add a comment to chapter %d: %v", id, err)
			http.Error(w, "failed to save the comment", http.StatusInternalServerError)
			return
		}
		cid, _ := res.LastInsertId()
		c, err := loadComment(db, cid)
		if err != nil {
			http.Error(w, "failed to save the comment", http.StatusInternalServerError)
			return
		}
		writeJSON(w, c)
	}
}

func commentIDOf(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil
}

func updateCommentHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := commentIDOf(r)
		if !ok {
			http.Error(w, "comment not found", http.StatusNotFound)
			return
		}
		var in struct {
			Text     *string `json:"text"`
			Resolved *bool   `json:"resolved"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&in); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		cur, err := loadComment(db, id)
		if err == sql.ErrNoRows {
			http.Error(w, "comment not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "failed to save the comment", http.StatusInternalServerError)
			return
		}
		now := sqlTime(editorNow())
		sets, args := []string{}, []any{}
		if in.Text != nil {
			text, msg := cleanCommentText(*in.Text)
			if msg != "" {
				http.Error(w, msg, http.StatusBadRequest)
				return
			}
			if text != cur.Text {
				sets = append(sets, "text = ?", "updated_at = ?")
				args = append(args, text, now)
			}
		}
		if in.Resolved != nil && *in.Resolved != cur.Resolved {
			if cur.ParentID != nil {
				http.Error(w, "a reply is resolved with its thread", http.StatusBadRequest)
				return
			}
			var at any
			if *in.Resolved {
				at = now
			}
			sets = append(sets, "resolved = ?", "resolved_at = ?")
			args = append(args, *in.Resolved, at)
		}
		if len(sets) > 0 {
			if _, err := db.Exec(`UPDATE chapter_comments SET `+strings.Join(sets, ", ")+` WHERE id = ?`, append(args, id)...); err != nil {
				log.Printf("update comment %d: %v", id, err)
				http.Error(w, "failed to save the comment", http.StatusInternalServerError)
				return
			}
		}
		c, err := loadComment(db, id)
		if err != nil {
			http.Error(w, "failed to save the comment", http.StatusInternalServerError)
			return
		}
		writeJSON(w, c)
	}
}

func deleteCommentHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := commentIDOf(r)
		if !ok {
			http.Error(w, "comment not found", http.StatusNotFound)
			return
		}
		res, err := db.Exec(`DELETE FROM chapter_comments WHERE id = ?`, id)
		if err != nil {
			log.Printf("delete comment %d: %v", id, err)
			http.Error(w, "failed to delete the comment", http.StatusInternalServerError)
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			http.Error(w, "comment not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// --- the marks ---

// commentMarkID is a comment mark's id (0 when it has none that is valid).
func commentMarkID(m pmMark) int64 {
	if m.Type != "comment" {
		return 0
	}
	id, ok := pmInt(m.Attrs["id"])
	if !ok || id < 1 {
		return 0
	}
	return id
}

// docCommentIDs is every comment id a doc's marks name.
func docCommentIDs(doc *pmNode) map[int64]bool {
	out := map[int64]bool{}
	pmWalk(doc, func(n *pmNode) {
		for _, m := range n.Marks {
			if id := commentMarkID(m); id > 0 {
				out[id] = true
			}
		}
	})
	return out
}

// mapCommentMarks gives each comment mark the id ids maps its id to; a mark
// whose id isn't in ids is dropped.
func mapCommentMarks(doc *pmNode, ids map[int64]int64) {
	pmWalk(doc, func(n *pmNode) {
		if len(n.Marks) == 0 {
			return
		}
		kept := n.Marks[:0]
		for _, m := range n.Marks {
			if m.Type == "comment" {
				to, ok := ids[commentMarkID(m)]
				if !ok {
					continue
				}
				attrs := map[string]any{}
				for k, v := range m.Attrs {
					attrs[k] = v
				}
				attrs["id"] = to
				m.Attrs = attrs
			}
			kept = append(kept, m)
		}
		n.Marks = kept
	})
}

// stripCommentMarks takes the comment marks off a doc (the exports other
// than .docx, which have no comments) and joins the text they split.
func stripCommentMarks(doc *pmNode) {
	pmWalk(doc, func(n *pmNode) {
		if len(n.Content) == 0 {
			return
		}
		out := n.Content[:0]
		for _, c := range n.Content {
			if c.Type == "text" && len(c.Marks) > 0 {
				kept := c.Marks[:0:0]
				for _, m := range c.Marks {
					if m.Type != "comment" {
						kept = append(kept, m)
					}
				}
				if len(kept) != len(c.Marks) {
					c.Marks = kept
				}
			}
			if k := len(out); k > 0 && c.Type == "text" && out[k-1].Type == "text" && len(c.Extra) == 0 && len(out[k-1].Extra) == 0 && sameMarks(out[k-1].Marks, c.Marks) {
				joined := *out[k-1]
				joined.Text += c.Text
				out[k-1] = &joined
				continue
			}
			out = append(out, c)
		}
		n.Content = out
	})
}

func sameMarks(a, b []pmMark) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 {
		return true
	}
	ja, err1 := json.Marshal(a)
	jb, err2 := json.Marshal(b)
	return err1 == nil && err2 == nil && string(ja) == string(jb)
}

// --- keeping the rows in step with the text ---

// syncChapterComments follows a write of a chapter's text (see the top of
// this file). Called by indexChapterText, in the same transaction.
func syncChapterComments(tx editorDB, chapterID int64, doc *pmNode) error {
	inText := docCommentIDs(doc)
	type row struct {
		id       int64
		anchored bool
		detached sql.NullString
		created  string
	}
	rows, err := tx.Query(`SELECT id, anchored, detached_at, created_at FROM chapter_comments WHERE chapter_id = ? AND parent_id IS NULL`, chapterID)
	if err != nil {
		return err
	}
	var all []row
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.id, &x.anchored, &x.detached, &x.created); err != nil {
			rows.Close()
			return err
		}
		all = append(all, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	now := editorNow()
	var missing []row
	for _, x := range all {
		if inText[x.id] {
			if !x.anchored || x.detached.Valid {
				if _, err := tx.Exec(`UPDATE chapter_comments SET anchored = 1, detached_at = NULL WHERE id = ?`, x.id); err != nil {
					return err
				}
			}
			continue
		}
		missing = append(missing, x)
	}
	if len(missing) == 0 {
		return nil
	}
	inVersions, err := versionCommentIDs(tx, chapterID)
	if err != nil {
		return err
	}
	older := func(at string) bool {
		t, err := time.Parse(sqlTimeLayout, at)
		return err == nil && now.Sub(t) >= commentGrace
	}
	for _, x := range missing {
		var err error
		switch {
		case inVersions[x.id]:
			if x.detached.Valid {
				_, err = tx.Exec(`UPDATE chapter_comments SET detached_at = NULL WHERE id = ?`, x.id)
			}
		case !x.anchored:
			if older(x.created) {
				_, err = tx.Exec(`DELETE FROM chapter_comments WHERE id = ?`, x.id)
			}
		case !x.detached.Valid:
			_, err = tx.Exec(`UPDATE chapter_comments SET detached_at = ? WHERE id = ?`, sqlTime(now), x.id)
		case older(x.detached.String):
			_, err = tx.Exec(`DELETE FROM chapter_comments WHERE id = ?`, x.id)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// versionCommentIDs: the comments the chapter's saved versions hold marks
// for (only versions mentioning a comment mark are read).
func versionCommentIDs(tx editorDB, chapterID int64) (map[int64]bool, error) {
	rows, err := tx.Query(`SELECT doc FROM chapter_text_versions WHERE chapter_id = ? AND instr(doc, '"comment"') > 0`, chapterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]bool{}
	for rows.Next() {
		var doc string
		if err := rows.Scan(&doc); err != nil {
			return nil, err
		}
		var d pmNode
		if json.Unmarshal([]byte(doc), &d) != nil {
			continue
		}
		for id := range docCommentIDs(&d) {
			out[id] = true
		}
	}
	return out, rows.Err()
}

// --- for the exports ---

// exportComment is a comment as the .docx export writes it.
type exportComment struct {
	ID        int64
	ChapterID int64
	ParentID  int64 // 0: a comment, else the one it answers
	Text      string
	Resolved  bool
	CreatedAt time.Time
}

// loadExportComments reads the comments of the given chapters, by id.
func loadExportComments(db editorDB, chapters map[int64]bool) (map[int64]*exportComment, error) {
	out := map[int64]*exportComment{}
	if len(chapters) == 0 {
		return out, nil
	}
	ids := make([]string, 0, len(chapters))
	for id := range chapters {
		ids = append(ids, strconv.FormatInt(id, 10))
	}
	rows, err := db.Query(`SELECT id, chapter_id, COALESCE(parent_id, 0), text, resolved, created_at FROM chapter_comments
		WHERE chapter_id IN (` + strings.Join(ids, ",") + `) ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		c := &exportComment{}
		var created string
		if err := rows.Scan(&c.ID, &c.ChapterID, &c.ParentID, &c.Text, &c.Resolved, &created); err != nil {
			return nil, err
		}
		c.CreatedAt, _ = time.Parse(sqlTimeLayout, created)
		out[c.ID] = c
	}
	return out, rows.Err()
}
