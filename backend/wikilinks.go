package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Links between wiki articles, written in Markdown text as [[…]].
//
// Stored form: [[type#id]] shows the article's current name, so a rename
// carries through; [[type#id|shown text]] shows the given text. `type` is a
// wikiTypes key, `id` the article's row id.
//
// Typed form, read everywhere and forever: [[Name]], [[Name|text]],
// [[type:Name]] and [[type:Name|text]], found by name (and a character's
// former names, after every real name).
//
// When text is saved the server turns every typed link that finds an article
// into the stored form (normalizeWikiLinks); one that finds nothing stays as
// typed and shows as missing. The editors show stored links by name again
// (linksForEditing in frontend/src/wikiLinks.js), so loading and saving
// unchanged text gives back the same bytes. Which texts: wikiLinkColumns.
// Where one is shown as plain text, its links read as their text or name
// (linksAsText in JS, wikiLinksAsText here), never raw. Go and JS must read links the
// same way: frontend/tests/wikiLinks.fixtures.json holds the cases both test
// suites check.
//
// Links inside code (a `span` or a ``` fenced block) are text, not links,
// and are never rewritten.

var (
	wikiLinkRe   = regexp.MustCompile(`\[\[([^\[\]|\n]+?)(?:\|([^\[\]\n]*))?\]\]`)
	wikiIDLinkRe = regexp.MustCompile(`^\s*([a-z_]+)#([1-9][0-9]{0,17})\s*$`)
	codeSpanRe   = regexp.MustCompile("`[^`\n]+`")
	codeFenceRe  = regexp.MustCompile("^[\t\f\r ]*(?:>[\t\f\r ]?)*[\t\f\r ]*```")
)

// wikiLinkMatch is one [[…]] in a text.
type wikiLinkMatch struct {
	Full    string // the whole [[…]]
	Target  string // before the |
	Text    string // after the | (may be empty)
	HasText bool   // there was a |
}

// mapWikiLinks calls fn for every link outside code and puts what it returns
// in the link's place.
func mapWikiLinks(text string, fn func(m wikiLinkMatch) string) string {
	if !strings.Contains(text, "[[") {
		return text
	}
	lines := strings.Split(text, "\n")
	inFence := false
	for i, line := range lines {
		if codeFenceRe.MatchString(line) {
			inFence = !inFence
			continue
		}
		if inFence || !strings.Contains(line, "[[") {
			continue
		}
		lines[i] = mapLinksOutsideCode(line, fn)
	}
	return strings.Join(lines, "\n")
}

func mapLinksOutsideCode(line string, fn func(m wikiLinkMatch) string) string {
	var b strings.Builder
	at := 0
	gap := func(s string) {
		last := 0
		for _, sm := range wikiLinkRe.FindAllStringSubmatchIndex(s, -1) {
			m := wikiLinkMatch{Full: s[sm[0]:sm[1]], Target: s[sm[2]:sm[3]]}
			if sm[4] >= 0 {
				m.Text, m.HasText = s[sm[4]:sm[5]], true
			}
			b.WriteString(s[last:sm[0]])
			b.WriteString(fn(m))
			last = sm[1]
		}
		b.WriteString(s[last:])
	}
	for _, span := range codeSpanRe.FindAllStringIndex(line, -1) {
		gap(line[at:span[0]])
		b.WriteString(line[span[0]:span[1]])
		at = span[1]
	}
	gap(line[at:])
	return b.String()
}

// findWikiLinks lists the links in a text, outside code.
func findWikiLinks(text string) []wikiLinkMatch {
	var out []wikiLinkMatch
	mapWikiLinks(text, func(m wikiLinkMatch) string {
		out = append(out, m)
		return m.Full
	})
	return out
}

// parseWikiIDLink reads a stored link's target ("character#12").
func parseWikiIDLink(target string) (string, int64, bool) {
	m := wikiIDLinkRe.FindStringSubmatch(target)
	if m == nil {
		return "", 0, false
	}
	if _, ok := wikiTypeFor(m[1]); !ok {
		return "", 0, false
	}
	id, err := strconv.ParseInt(m[2], 10, 64)
	if err != nil {
		return "", 0, false
	}
	return m[1], id, true
}

func wikiIDLink(typ string, id int64) string { return typ + "#" + strconv.FormatInt(id, 10) }

func normalizeWikiKey(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.ReplaceAll(s, "_", " "))), " ")
}

// splitWikiTarget takes a typed target apart: the article kind it names
// ("" when none) and the name after it.
func splitWikiTarget(target string) (string, string) {
	target = strings.TrimSpace(target)
	if i := strings.Index(target, ":"); i > 0 {
		prefix := normalizeWikiKey(target[:i])
		for _, t := range wikiTypes {
			if normalizeWikiKey(t.Key) == prefix {
				return t.Key, target[i+1:]
			}
		}
	}
	return "", target
}

// wikiLinkIndex finds articles by name and by id.
type wikiLinkIndex struct {
	byName map[string][]wikiRef // normalised name -> articles, wikiTypes order, former names last
	byID   map[string]wikiRef   // refKey -> article
}

func newWikiLinkIndex(items []wikiListItem) *wikiLinkIndex {
	x := &wikiLinkIndex{byName: map[string][]wikiRef{}, byID: map[string]wikiRef{}}
	for _, it := range items {
		ref := wikiRef{Type: it.Type, ID: it.ID, Name: it.Name}
		k := normalizeWikiKey(it.Name)
		x.byName[k] = append(x.byName[k], ref)
		x.byID[refKey(it.Type, it.ID)] = ref
	}
	// Former names come after every real name, so they never take a link
	// that already points at something else.
	for _, it := range items {
		for _, a := range it.Aliases {
			k := normalizeWikiKey(a)
			x.byName[k] = append(x.byName[k], wikiRef{Type: it.Type, ID: it.ID, Name: it.Name})
		}
	}
	return x
}

func loadWikiLinkIndex(q wikiQuerier) (*wikiLinkIndex, error) {
	items, err := listWikiItems(q)
	if err != nil {
		return nil, err
	}
	return newWikiLinkIndex(items), nil
}

// resolve finds the article a link's target points at, or nil.
func (x *wikiLinkIndex) resolve(target string) *wikiRef {
	if typ, id, ok := parseWikiIDLink(target); ok {
		if r, found := x.byID[refKey(typ, id)]; found {
			return &r
		}
		return nil
	}
	typ, name := splitWikiTarget(target)
	for _, ref := range x.byName[normalizeWikiKey(name)] {
		if typ == "" || ref.Type == typ {
			r := ref
			return &r
		}
	}
	return nil
}

// normalizeWikiLinks turns every typed link that finds an article into the
// stored form. The shown text: an explicit |text is kept as it is; else the
// typed name is kept as the text unless it is the article's current name.
// Links that find nothing, and links already stored, stay as they are.
func normalizeWikiLinks(text string, x *wikiLinkIndex) string {
	return mapWikiLinks(text, func(m wikiLinkMatch) string {
		if _, _, ok := parseWikiIDLink(m.Target); ok {
			return m.Full
		}
		ref := x.resolve(m.Target)
		if ref == nil {
			return m.Full
		}
		head := "[[" + wikiIDLink(ref.Type, ref.ID)
		if m.HasText {
			return head + "|" + m.Text + "]]"
		}
		_, name := splitWikiTarget(m.Target)
		name = strings.TrimSpace(name)
		if normalizeWikiKey(name) == normalizeWikiKey(ref.Name) {
			return head + "]]"
		}
		return head + "|" + name + "]]"
	})
}

// wikiLinkNames is the current name of every article the stored links in
// texts point at, by "type#id", for readers that show text without the
// whole index (article previews).
func wikiLinkNames(x *wikiLinkIndex, texts ...string) map[string]string {
	out := map[string]string{}
	for _, s := range texts {
		for _, m := range findWikiLinks(s) {
			if typ, id, ok := parseWikiIDLink(m.Target); ok {
				if r, found := x.byID[refKey(typ, id)]; found {
					out[wikiIDLink(typ, id)] = r.Name
				}
			}
		}
	}
	return out
}

// wikiLinksAsText is text with each link outside code as the words a reader
// sees: its |text when it has one, else the article's current name for a
// stored link (`deleted` for one whose article is gone), else the typed name
// without its kind. The same rules as linksAsText in
// frontend/src/wikiLinks.js (the fixtures' "plain" cases). For matching a
// search against what is shown.
func wikiLinksAsText(text string, x *wikiLinkIndex, deleted string) string {
	return mapWikiLinks(text, func(m wikiLinkMatch) string {
		if m.HasText && strings.TrimSpace(m.Text) != "" {
			return m.Text
		}
		if typ, id, ok := parseWikiIDLink(m.Target); ok {
			if r, found := x.byID[refKey(typ, id)]; found {
				return r.Name
			}
			return deleted
		}
		_, name := splitWikiTarget(m.Target)
		return strings.TrimSpace(name)
	})
}

// storeLinksByID normalises each text in place (normalizeWikiLinks), reading
// the wiki's index only when one of them holds a link. Every handler that
// saves a text in wikiLinkColumns calls it (or normalizeWikiLinks).
func storeLinksByID(q wikiQuerier, texts ...*string) error {
	var x *wikiLinkIndex
	for _, s := range texts {
		if s == nil || !strings.Contains(*s, "[[") {
			continue
		}
		if x == nil {
			var err error
			if x, err = loadWikiLinkIndex(q); err != nil {
				return fmt.Errorf("index the wiki: %w", err)
			}
		}
		*s = normalizeWikiLinks(*s, x)
	}
	return nil
}

// ---- Every stored text that can hold links -------------------------------

// wikiQuerier is a *sql.DB or a *sql.Tx.
type wikiQuerier interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

type wikiExecQuerier interface {
	wikiQuerier
	Exec(query string, args ...any) (sql.Result, error)
}

// linkColumn is a column whose text can hold stored links. where limits the
// rows ("" = all); key is the table's integer key column ("" = id).
type linkColumn struct{ table, column, where, key string }

func (c linkColumn) keyCol() string {
	if c.key == "" {
		return "id"
	}
	return c.key
}

// schema42LinkColumns are the texts schema 42 rewrote: what is written in
// the wiki (wiki_entries.fields is a JSON object of such texts) and chapter
// notes. Section titles are plain text and keep typed links. Frozen: schema
// 42's migration reads it.
var schema42LinkColumns = []linkColumn{
	{"wiki_entries", "summary", "", ""},
	{"wiki_entries", "fields", "", ""},
	{"wiki_sections", "body", "", ""},
	{"wiki_infobox", "value", "kind = ''", ""},
	{"chapters", "notes", "", ""},
}

// itemLinkColumns are the texts the story's own pages write and the wiki
// shows as Markdown on the item's article ("Written on the original page"):
// a character version's description, bio and speech mannerisms, and the
// description of a location, faction, event, spell, gear or creature (schema
// 43). Their forms show links by name (linksForEditing); every other page
// shows them as plain text through linksAsText (frontend/src/wikiLinks.js),
// never raw. Spells and gear are copied with "copy Character Assets", which
// turns their links to anything not copied back into names (linksForCopy).
var itemLinkColumns = []linkColumn{
	{"character_story", "description", "", "version_id"},
	{"character_story", "bio", "", "version_id"},
	{"character_story", "speech_mannerisms", "", "version_id"},
	{"locations", "description", "", ""},
	{"factions", "description", "", ""},
	{"events", "description", "", ""},
	{"spells", "description", "", ""},
	{"gear", "description", "", ""},
	{"creatures", "description", "", ""},
}

// wikiLinkColumns are all the columns whose text is Markdown that can hold
// stored links. A new one goes in a new list here with a migration that
// rewrites what is already written; its handler normalises it on save
// (storeLinksByID) and its form shows the links by name.
var wikiLinkColumns = slices.Concat(schema42LinkColumns, itemLinkColumns)

// rewriteStoredWikiLinks runs fn over every text in cols and saves the ones
// it changes.
func rewriteStoredWikiLinks(q wikiExecQuerier, cols []linkColumn, fn func(string) string) error {
	for _, c := range cols {
		sel := fmt.Sprintf(`SELECT %s, %s FROM %s WHERE %s LIKE '%%[[%%'`, c.keyCol(), c.column, c.table, c.column)
		if c.where != "" {
			sel += " AND " + c.where
		}
		rows, err := q.Query(sel)
		if err != nil {
			return fmt.Errorf("read %s.%s: %w", c.table, c.column, err)
		}
		changed := map[int64]string{}
		for rows.Next() {
			var id int64
			var s string
			if err := rows.Scan(&id, &s); err != nil {
				rows.Close()
				return err
			}
			var out string
			if c.table == "wiki_entries" && c.column == "fields" {
				out = rewriteFieldsJSON(s, fn)
			} else {
				out = fn(s)
			}
			if out != s {
				changed[id] = out
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for id, s := range changed {
			if _, err := q.Exec(fmt.Sprintf(`UPDATE %s SET %s = ? WHERE %s = ?`, c.table, c.column, c.keyCol()), s, id); err != nil {
				return fmt.Errorf("write %s.%s: %w", c.table, c.column, err)
			}
		}
	}
	return nil
}

// rewriteFieldsJSON runs fn over each value of a wiki entry's fields; text
// that isn't the JSON object it should be is left alone.
func rewriteFieldsJSON(raw string, fn func(string) string) string {
	var m map[string]string
	if json.Unmarshal([]byte(raw), &m) != nil {
		return raw
	}
	changed := false
	for k, v := range m {
		if nv := fn(v); nv != v {
			m[k], changed = nv, true
		}
	}
	if !changed {
		return raw
	}
	out, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return string(out)
}

// normalizeStoredWikiLinks is schema 42's migration: every link written
// before links were stored by id is turned into the stored form. It reads
// the articles through listWikiItems, so the tables that lists must exist
// in the shape it expects at schema 42.
func normalizeStoredWikiLinks(tx *sql.Tx) error {
	return normalizeLinksIn(tx, schema42LinkColumns)
}

// normalizeItemTextLinks is schema 43's: the same for the items' own texts
// (itemLinkColumns), which kept typed links until then. Same caveat, at
// schema 43.
func normalizeItemTextLinks(tx *sql.Tx) error {
	return normalizeLinksIn(tx, itemLinkColumns)
}

func normalizeLinksIn(tx *sql.Tx, cols []linkColumn) error {
	x, err := loadWikiLinkIndex(tx)
	if err != nil {
		return fmt.Errorf("index the wiki: %w", err)
	}
	return rewriteStoredWikiLinks(tx, cols, func(s string) string { return normalizeWikiLinks(s, x) })
}

// ---- Copying a text into another story -------------------------------------

// linksForCopy prepares a text that "copy Character Assets" copies into a
// new story, which keeps the ids of the rows it copies. A stored link to a
// kind of article in copied (wikiTypes keys) stays as it is. A link to
// anything else would find an unrelated row of the new story by its id, so
// it turns back into a typed name with the source story's name (x),
// [[type:Name]] or [[type:Name|text]]: the new story finds it by name or
// shows it as missing. A link to an article that is gone, or whose name
// can't be written in a link, becomes its text (or the name).
func linksForCopy(text string, x *wikiLinkIndex, copied map[string]bool) string {
	return mapWikiLinks(text, func(m wikiLinkMatch) string {
		typ, id, ok := parseWikiIDLink(m.Target)
		if !ok {
			return m.Full
		}
		r, found := x.byID[refKey(typ, id)]
		if found && copied[typ] {
			return m.Full
		}
		shown := ""
		if m.HasText && strings.TrimSpace(m.Text) != "" {
			shown = m.Text
		}
		if !found {
			return shown
		}
		name := strings.TrimSpace(r.Name)
		if name == "" || strings.ContainsAny(name, "[]|\n") {
			if shown != "" {
				return shown
			}
			return r.Name
		}
		if m.HasText {
			return "[[" + typ + ":" + name + "|" + m.Text + "]]"
		}
		return "[[" + typ + ":" + name + "]]"
	})
}

// storeLinksOrFail is storeLinksByID for a handler: false (with the error
// answered) when the wiki's index can't be read.
func storeLinksOrFail(w http.ResponseWriter, q wikiQuerier, texts ...*string) bool {
	if err := storeLinksByID(q, texts...); err != nil {
		log.Printf("store links: %v", err)
		http.Error(w, "failed to save the text", http.StatusInternalServerError)
		return false
	}
	return true
}
