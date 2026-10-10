package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// The wiki has no pages of its own. Every character, location, event, class,
// subclass, specialization, race, body type, spell and piece of gear is
// already an article; this file reads the facts straight from those tables and
// stores only what the user adds on top (wiki_entries and its two child
// tables). Deleting the source deletes the extras too, through the triggers
// that wikiSchemaStatements creates (the source tables can't have a real
// foreign key to it, because the entry points at one of ten tables).
//
// Adding a new kind of article: add a wikiTypeDef below, its facts in
// buildWikiArticle, its labels in frontend/src/wiki.js, and a migration that
// creates its delete trigger.

const (
	maxWikiText     = 60000
	maxWikiSections = 60
	maxWikiRows     = 40
	maxWikiTitle    = 120
)

type wikiTypeDef struct {
	Key    string
	Table  string
	Select string // id, name, picture of every row
	Filter string // restricts which rows are articles ("" = all)
	IDCol  string
	Fields []string // the fixed markdown boxes, in order
}

// Order matters: it is the order of the index and the tie-break for a
// [[link]] whose name exists in more than one kind of article.
var wikiTypes = []wikiTypeDef{
	{Key: "character", Table: "characters", IDCol: "c.id",
		Select: `SELECT c.id, v.name, COALESCE(v.picture_path, '') FROM characters c
			JOIN character_versions v ON v.character_id = c.id AND v.is_current = 1`,
		Fields: []string{"appearance", "personality", "background", "relationships", "abilities", "trivia"}},
	{Key: "location", Table: "locations", IDCol: "id",
		Select: `SELECT id, name, COALESCE(picture_path, '') FROM locations`,
		Fields: []string{"geography", "history", "culture", "inhabitants", "points_of_interest", "trivia"}},
	{Key: "faction", Table: "factions", IDCol: "id",
		Select: `SELECT id, name, COALESCE(picture_path, '') FROM factions`,
		Fields: []string{"history", "goals", "structure", "notable_members", "trivia"}},
	// Events that merely carry a character's birth or a location's founding
	// have no name of their own; that information lives on the character or
	// location article.
	{Key: "event", Table: "events", IDCol: "id", Filter: "source_type IS NULL",
		Select: `SELECT id, name, COALESCE(picture_path, '') FROM events`,
		Fields: []string{"background", "course", "aftermath", "trivia"}},
	{Key: "class", Table: "classes", IDCol: "id",
		Select: `SELECT id, name, COALESCE(icon_path, '') FROM classes`,
		Fields: []string{"lore", "history", "training", "notable_members", "trivia"}},
	{Key: "subclass", Table: "subclasses", IDCol: "id",
		Select: `SELECT id, name, '' FROM subclasses`,
		Fields: []string{"lore", "history", "notable_members", "trivia"}},
	{Key: "specialization", Table: "specializations", IDCol: "id",
		Select: `SELECT id, name, '' FROM specializations`,
		Fields: []string{"lore", "history", "notable_members", "trivia"}},
	{Key: "race", Table: "races", IDCol: "id",
		Select: `SELECT id, name, '' FROM races`,
		Fields: []string{"appearance", "culture", "history", "trivia"}},
	{Key: "body_type", Table: "body_types", IDCol: "id",
		Select: `SELECT id, name, '' FROM body_types`,
		Fields: []string{"appearance", "notes"}},
	{Key: "spell", Table: "spells", IDCol: "id",
		Select: `SELECT id, name, COALESCE(icon_path, '') FROM spells`,
		Fields: []string{"lore", "usage", "trivia"}},
	{Key: "gear", Table: "gear", IDCol: "id",
		Select: `SELECT id, name, COALESCE(icon_path, '') FROM gear`,
		Fields: []string{"appearance", "lore", "origin", "trivia"}},
}

func wikiTypeFor(key string) (wikiTypeDef, bool) {
	for _, t := range wikiTypes {
		if t.Key == key {
			return t, true
		}
	}
	return wikiTypeDef{}, false
}

func (t wikiTypeDef) hasField(k string) bool {
	for _, f := range t.Fields {
		if f == k {
			return true
		}
	}
	return false
}

func (t wikiTypeDef) listSQL() string {
	q := t.Select
	if t.Filter != "" {
		q += " WHERE " + t.Filter
	}
	return q
}

func (t wikiTypeDef) oneSQL() string {
	q := t.Select + " WHERE " + t.IDCol + " = ?"
	if t.Filter != "" {
		q += " AND " + t.Filter
	}
	return q
}

const wikiEntriesTableSQL = `CREATE TABLE IF NOT EXISTS wiki_entries (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	entity_type TEXT NOT NULL,
	entity_id   INTEGER NOT NULL,
	summary     TEXT NOT NULL DEFAULT '',
	fields      TEXT NOT NULL DEFAULT '{}',
	updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
	UNIQUE (entity_type, entity_id)
)`

const wikiSectionsTableSQL = `CREATE TABLE IF NOT EXISTS wiki_sections (
	id       INTEGER PRIMARY KEY AUTOINCREMENT,
	entry_id INTEGER NOT NULL REFERENCES wiki_entries(id) ON DELETE CASCADE,
	position INTEGER NOT NULL,
	title    TEXT NOT NULL,
	body     TEXT NOT NULL DEFAULT ''
)`

const wikiInfoboxTableSQL = `CREATE TABLE IF NOT EXISTS wiki_infobox (
	id       INTEGER PRIMARY KEY AUTOINCREMENT,
	entry_id INTEGER NOT NULL REFERENCES wiki_entries(id) ON DELETE CASCADE,
	position INTEGER NOT NULL,
	label    TEXT NOT NULL,
	value    TEXT NOT NULL DEFAULT ''
)`

// Everything the wiki adds to a database: its tables, plus one trigger per
// source table so a deleted source takes its wiki entry with it. Used by both
// the fresh schema and the migration that introduced the wiki.
var wikiSchemaStatements = func() []string {
	var keys []string
	for _, t := range wikiTypes {
		keys = append(keys, t.Key)
	}
	return wikiSchemaFor(keys...)
}()

// wikiTypesAtSchema13 are the article kinds the wiki started with. The
// migration that created the wiki makes triggers for exactly these, since
// later kinds' tables don't exist yet at that point.
var wikiTypesAtSchema13 = []string{
	"character", "location", "event", "class", "subclass", "specialization", "race", "body_type", "spell", "gear",
}

func wikiSchemaFor(keys ...string) []string {
	stmts := []string{wikiEntriesTableSQL, wikiSectionsTableSQL, wikiInfoboxTableSQL}
	for _, k := range keys {
		stmts = append(stmts, wikiTriggerSQL(k))
	}
	return stmts
}

// wikiTriggerSQL makes a deleted source take its wiki entry with it.
func wikiTriggerSQL(key string) string {
	t, _ := wikiTypeFor(key)
	return fmt.Sprintf(
		`CREATE TRIGGER IF NOT EXISTS wiki_cleanup_%s AFTER DELETE ON %s BEGIN
			DELETE FROM wiki_entries WHERE entity_type = '%s' AND entity_id = OLD.id;
		END`, t.Table, t.Table, t.Key)
}

// ---- Payloads ----------------------------------------------------------------

type wikiRef struct {
	Type string `json:"type"`
	ID   int64  `json:"id"`
	Name string `json:"name"`
	// Note is shown after the name in a related list: a member's role, or
	// how a relation reads ("Child of"). NoteWord marks an English phrase
	// the frontend translates rather than the user's own text.
	Note     string `json:"note,omitempty"`
	NoteWord bool   `json:"note_word,omitempty"`
}

// wikiFact is one row of the infobox that comes from the source itself. The
// frontend turns Key into a translated label. Kind is "" (plain text), "date"
// (a story date to format with the calendar) or "word" (an English word the
// frontend translates).
type wikiFact struct {
	Key   string   `json:"key"`
	Value string   `json:"value"`
	Kind  string   `json:"kind,omitempty"`
	Link  *wikiRef `json:"link,omitempty"`
}

type wikiGroup struct {
	Key   string    `json:"key"`
	Items []wikiRef `json:"items"`
}

// wikiSheet is text the user already wrote elsewhere (a character's bio, an
// event's description), shown read-only; it is edited where it came from.
type wikiSheet struct {
	Key  string `json:"key"`
	Body string `json:"body"`
}

type wikiSection struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type wikiInfoRow struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type wikiArticle struct {
	Type      string            `json:"type"`
	ID        int64             `json:"id"`
	Name      string            `json:"name"`
	Picture   string            `json:"picture"`
	FieldKeys []string          `json:"field_keys"`
	Facts     []wikiFact        `json:"facts"`
	Groups    []wikiGroup       `json:"groups"`
	Sheet     []wikiSheet       `json:"sheet"`
	Summary   string            `json:"summary"`
	Fields    map[string]string `json:"fields"`
	Sections  []wikiSection     `json:"sections"`
	Infobox   []wikiInfoRow     `json:"infobox"`
	Backlinks []wikiRef         `json:"backlinks"`
	UpdatedAt string            `json:"updated_at"`
}

type wikiListItem struct {
	Type    string `json:"type"`
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
	Written bool   `json:"written"` // someone has added wiki content
	// EditedAt is when its written text last changed (UTC, SQLite format);
	// empty for an article nobody has written in.
	EditedAt string `json:"edited_at,omitempty"`
}

// pictureURL turns a stored picture or icon into what the frontend expects:
// built-in icons stay as "builtin:<name>", uploads become /uploads/<file>.
func pictureURL(stored string) string {
	if isBuiltinIcon(stored) {
		return stored
	}
	return uploadURL(stored)
}

// ---- Reading -----------------------------------------------------------------

func listWikiItems(db *sql.DB) ([]wikiListItem, error) {
	written := map[string]string{} // "type:id" -> edited at
	rows, err := db.Query(`SELECT entity_type, entity_id, updated_at FROM wiki_entries`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var typ, edited string
		var id int64
		if err := rows.Scan(&typ, &id, &edited); err != nil {
			rows.Close()
			return nil, err
		}
		written[typ+":"+strconv.FormatInt(id, 10)] = edited
	}
	rows.Close()

	items := []wikiListItem{}
	for _, t := range wikiTypes {
		rows, err := db.Query(t.listSQL() + ` ORDER BY 2 COLLATE NOCASE`)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			it := wikiListItem{Type: t.Key}
			if err := rows.Scan(&it.ID, &it.Name, &it.Picture); err != nil {
				rows.Close()
				return nil, err
			}
			it.Picture = pictureURL(it.Picture)
			it.EditedAt, it.Written = written[t.Key+":"+strconv.FormatInt(it.ID, 10)]
			items = append(items, it)
		}
		rows.Close()
	}
	return items, nil
}

func listWikiHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := listWikiItems(db)
		if err != nil {
			http.Error(w, "failed to load the wiki", http.StatusInternalServerError)
			log.Printf("list wiki: %v", err)
			return
		}
		writeJSON(w, items)
	}
}

func queryRefs(db *sql.DB, typ, q string, args ...any) []wikiRef {
	out := []wikiRef{}
	rows, err := db.Query(q, args...)
	if err != nil {
		log.Printf("wiki refs (%s): %v", typ, err)
		return out
	}
	defer rows.Close()
	for rows.Next() {
		ref := wikiRef{Type: typ}
		if rows.Scan(&ref.ID, &ref.Name) == nil {
			out = append(out, ref)
		}
	}
	return out
}

const wikiMembersSQL = `SELECT c.id, v.name FROM characters c
	JOIN character_versions v ON v.character_id = c.id AND v.is_current = 1
	JOIN character_story st ON st.version_id = v.id
	WHERE %s = ? ORDER BY v.name COLLATE NOCASE`

// Same, for the columns that live on the version itself.
const wikiVersionMembersSQL = `SELECT c.id, v.name FROM characters c
	JOIN character_versions v ON v.character_id = c.id AND v.is_current = 1
	WHERE %s = ? ORDER BY v.name COLLATE NOCASE`

func refOf(typ string, id sql.NullInt64, name sql.NullString) *wikiRef {
	if !id.Valid || !name.Valid {
		return nil
	}
	return &wikiRef{Type: typ, ID: id.Int64, Name: name.String}
}

// addFact appends a row unless it is empty.
// statusWord is how a character's status reads in the infobox.
var statusWord = map[string]string{"alive": "Alive", "missing": "Missing", "dead": "Dead"}

func (a *wikiArticle) addFact(key, value, kind string, link *wikiRef) {
	if link != nil {
		value = link.Name
	}
	if strings.TrimSpace(value) == "" {
		return
	}
	a.Facts = append(a.Facts, wikiFact{Key: key, Value: value, Kind: kind, Link: link})
}

func (a *wikiArticle) addGroup(key string, items []wikiRef) {
	if len(items) > 0 {
		a.Groups = append(a.Groups, wikiGroup{Key: key, Items: items})
	}
}

func (a *wikiArticle) addSheet(key, body string) {
	if strings.TrimSpace(body) != "" {
		a.Sheet = append(a.Sheet, wikiSheet{Key: key, Body: body})
	}
}

// loadSourceFacts fills the infobox rows, the related lists and the text that
// already exists elsewhere, from the live source tables.
func loadSourceFacts(db *sql.DB, a *wikiArticle) {
	id := a.ID
	switch a.Type {
	case "character":
		var nick, gender, birth, bornText, nationText, desc, bio, speech, status string
		var raceID, bodyID, classID, subID, specID, bornLoc, nationLoc sql.NullInt64
		var raceName, bodyName, className, subName, specName, bornName, nationName sql.NullString
		err := db.QueryRow(`
			SELECT COALESCE(v.nickname, ''), COALESCE(st.gender, ''), COALESCE(st.birth_date, ''),
			       COALESCE(st.born_in, ''), COALESCE(st.nation, ''),
			       COALESCE(st.description, ''), COALESCE(st.bio, ''), COALESCE(st.speech_mannerisms, ''),
			       COALESCE(st.status, 'alive'), st.race_id, r.name, st.body_type_id, b.name,
			       v.class_id, cl.name, v.subclass_id, sc.name, v.specialization_id, sp.name,
			       st.born_in_location_id, bl.name, st.nation_location_id, nl.name
			FROM character_versions v
			LEFT JOIN character_story st ON st.version_id = v.id
			LEFT JOIN races r ON r.id = st.race_id
			LEFT JOIN body_types b ON b.id = st.body_type_id
			LEFT JOIN classes cl ON cl.id = v.class_id
			LEFT JOIN subclasses sc ON sc.id = v.subclass_id
			LEFT JOIN specializations sp ON sp.id = v.specialization_id
			LEFT JOIN locations bl ON bl.id = st.born_in_location_id
			LEFT JOIN locations nl ON nl.id = st.nation_location_id
			WHERE v.character_id = ? AND v.is_current = 1`, id).Scan(
			&nick, &gender, &birth, &bornText, &nationText, &desc, &bio, &speech,
			&status, &raceID, &raceName, &bodyID, &bodyName,
			&classID, &className, &subID, &subName, &specID, &specName,
			&bornLoc, &bornName, &nationLoc, &nationName)
		if err != nil {
			log.Printf("wiki character facts: %v", err)
			return
		}
		a.addFact("nickname", nick, "", nil)
		a.addFact("status", statusWord[status], "word", nil)
		a.addFact("race", "", "", refOf("race", raceID, raceName))
		a.addFact("gender", gender, "", nil)
		a.addFact("body_type", "", "", refOf("body_type", bodyID, bodyName))
		a.addFact("class", "", "", refOf("class", classID, className))
		a.addFact("subclass", "", "", refOf("subclass", subID, subName))
		a.addFact("specialization", "", "", refOf("specialization", specID, specName))
		a.addFact("born", normalizeStoryDate(birth), "date", nil)
		if l := refOf("location", bornLoc, bornName); l != nil {
			a.addFact("born_in", "", "", l)
		} else {
			a.addFact("born_in", bornText, "", nil)
		}
		if l := refOf("location", nationLoc, nationName); l != nil {
			a.addFact("nation", "", "", l)
		} else {
			a.addFact("nation", nationText, "", nil)
		}
		a.addSheet("description", desc)
		a.addSheet("bio", bio)
		a.addSheet("speech", speech)
		a.addGroup("events", queryRefs(db, "event", `
			SELECT e.id, e.name FROM events e JOIN event_characters ec ON ec.event_id = e.id
			WHERE ec.character_id = ? AND e.source_type IS NULL ORDER BY e.name COLLATE NOCASE`, id))
		var rels []wikiRef
		for _, r := range relationsOf(db, id) {
			text, builtin := r.wording(id)
			other := wikiRef{Type: "character", ID: r.ToID, Name: r.ToName, Note: text, NoteWord: builtin}
			if r.ToID == id {
				other.ID, other.Name = r.FromID, r.FromName
			}
			rels = append(rels, other)
		}
		a.addGroup("relations", rels)
		var facs []wikiRef
		for _, m := range membershipsOf(db, id) {
			facs = append(facs, wikiRef{Type: "faction", ID: m.FactionID, Name: m.Name, Note: m.Role})
		}
		a.addGroup("factions", facs)

	case "location":
		var kind, color, desc, founded string
		var parentID sql.NullInt64
		var parentName sql.NullString
		var isCity, isCapital bool
		err := db.QueryRow(`
			SELECT l.kind, COALESCE(l.color, ''), COALESCE(l.description, ''), COALESCE(l.founding_date, ''),
			       l.belongs_to_id, p.name, l.is_city, l.is_capital
			FROM locations l LEFT JOIN locations p ON p.id = l.belongs_to_id WHERE l.id = ?`, id).Scan(
			&kind, &color, &desc, &founded, &parentID, &parentName, &isCity, &isCapital)
		if err != nil {
			log.Printf("wiki location facts: %v", err)
			return
		}
		word := "Minor location"
		if kind == "major" {
			word = "Major location"
			if color != "" {
				word = "Kingdom"
			}
		}
		a.addFact("type", word, "word", nil)
		if isCapital {
			a.addFact("settlement", "Capital", "word", nil)
		} else if isCity {
			a.addFact("settlement", "City", "word", nil)
		}
		a.addFact("part_of", "", "", refOf("location", parentID, parentName))
		a.addFact("founded", normalizeStoryDate(founded), "date", nil)
		a.addSheet("description", desc)
		a.addGroup("places", queryRefs(db, "location",
			`SELECT id, name FROM locations WHERE belongs_to_id = ? ORDER BY name COLLATE NOCASE`, id))
		a.addGroup("events_here", queryRefs(db, "event",
			`SELECT id, name FROM events WHERE location_id = ? AND source_type IS NULL ORDER BY name COLLATE NOCASE`, id))
		a.addGroup("born_here", queryRefs(db, "character", fmt.Sprintf(wikiMembersSQL, "st.born_in_location_id"), id))
		a.addGroup("nation_members", queryRefs(db, "character", fmt.Sprintf(wikiMembersSQL, "st.nation_location_id"), id))
		a.addGroup("spells_from", queryRefs(db, "spell",
			`SELECT id, name FROM spells WHERE origin_location_id = ? ORDER BY name COLLATE NOCASE`, id))
		a.addGroup("factions_here", queryRefs(db, "faction",
			`SELECT id, name FROM factions WHERE hq_location_id = ? ORDER BY name COLLATE NOCASE`, id))

	case "faction":
		f, err := loadFaction(db, id)
		if err != nil {
			log.Printf("wiki faction facts: %v", err)
			return
		}
		if f.ParentID != nil {
			a.addFact("part_of", "", "", &wikiRef{Type: "faction", ID: *f.ParentID, Name: f.ParentName})
		}
		if f.HQLocationID != nil {
			a.addFact("headquarters", "", "", &wikiRef{Type: "location", ID: *f.HQLocationID, Name: f.HQName})
		}
		a.addFact("founded", f.FoundingDate, "date", nil)
		a.addSheet("description", f.Description)
		var current, former []wikiRef
		for _, m := range f.Members {
			ref := wikiRef{Type: "character", ID: m.CharacterID, Name: m.Name, Note: m.Role}
			if m.Until != "" {
				former = append(former, ref)
			} else {
				current = append(current, ref)
			}
		}
		a.addGroup("members", current)
		a.addGroup("former_members", former)
		a.addGroup("subfactions", queryRefs(db, "faction",
			`SELECT id, name FROM factions WHERE parent_id = ? ORDER BY name COLLATE NOCASE`, id))

	case "event":
		var desc, date string
		var locID sql.NullInt64
		var locName sql.NullString
		err := db.QueryRow(`
			SELECT COALESCE(e.description, ''), e.event_date, e.location_id, l.name
			FROM events e LEFT JOIN locations l ON l.id = e.location_id WHERE e.id = ?`, id).Scan(
			&desc, &date, &locID, &locName)
		if err != nil {
			log.Printf("wiki event facts: %v", err)
			return
		}
		a.addFact("date", normalizeStoryDate(date), "date", nil)
		a.addFact("location", "", "", refOf("location", locID, locName))
		var tags []string
		for _, ref := range queryRefs(db, "", `SELECT 0, tag FROM event_tags WHERE event_id = ? ORDER BY tag`, id) {
			tags = append(tags, "#"+ref.Name)
		}
		a.addFact("tags", strings.Join(tags, ", "), "", nil)
		a.addSheet("description", desc)
		a.addGroup("people", queryRefs(db, "character", `
			SELECT c.id, v.name FROM event_characters ec
			JOIN characters c ON c.id = ec.character_id
			JOIN character_versions v ON v.character_id = c.id AND v.is_current = 1
			WHERE ec.event_id = ? ORDER BY v.name COLLATE NOCASE`, id))

	case "class":
		a.addGroup("subclasses", queryRefs(db, "subclass", `
			SELECT s.id, s.name FROM subclasses s JOIN subclass_classes sc ON sc.subclass_id = s.id
			WHERE sc.class_id = ? ORDER BY s.name COLLATE NOCASE`, id))
		a.addGroup("specializations", queryRefs(db, "specialization",
			`SELECT id, name FROM specializations WHERE class_id = ? ORDER BY name COLLATE NOCASE`, id))
		a.addGroup("spells", queryRefs(db, "spell",
			`SELECT id, name FROM spells WHERE source_type = 'class' AND source_id = ? ORDER BY name COLLATE NOCASE`, id))
		a.addGroup("members", queryRefs(db, "character", fmt.Sprintf(wikiVersionMembersSQL, "v.class_id"), id))

	case "subclass":
		a.addGroup("classes", queryRefs(db, "class", `
			SELECT c.id, c.name FROM classes c JOIN subclass_classes sc ON sc.class_id = c.id
			WHERE sc.subclass_id = ? ORDER BY c.name COLLATE NOCASE`, id))
		a.addGroup("spells", queryRefs(db, "spell",
			`SELECT id, name FROM spells WHERE source_type = 'subclass' AND source_id = ? ORDER BY name COLLATE NOCASE`, id))
		a.addGroup("members", queryRefs(db, "character", fmt.Sprintf(wikiVersionMembersSQL, "v.subclass_id"), id))

	case "specialization":
		var classID sql.NullInt64
		var className sql.NullString
		db.QueryRow(`SELECT c.id, c.name FROM specializations s JOIN classes c ON c.id = s.class_id WHERE s.id = ?`, id).
			Scan(&classID, &className)
		a.addFact("class", "", "", refOf("class", classID, className))
		a.addGroup("spells", queryRefs(db, "spell",
			`SELECT id, name FROM spells WHERE source_type = 'specialization' AND source_id = ? ORDER BY name COLLATE NOCASE`, id))
		a.addGroup("members", queryRefs(db, "character", fmt.Sprintf(wikiVersionMembersSQL, "v.specialization_id"), id))

	case "race":
		a.addGroup("members", queryRefs(db, "character", fmt.Sprintf(wikiMembersSQL, "st.race_id"), id))

	case "body_type":
		a.addGroup("members", queryRefs(db, "character", fmt.Sprintf(wikiMembersSQL, "st.body_type_id"), id))

	case "spell":
		var desc string
		var srcType sql.NullString
		var srcID, locID sql.NullInt64
		var locName sql.NullString
		err := db.QueryRow(`
			SELECT COALESCE(s.description, ''), s.source_type, s.source_id, s.origin_location_id, l.name
			FROM spells s LEFT JOIN locations l ON l.id = s.origin_location_id WHERE s.id = ?`, id).Scan(
			&desc, &srcType, &srcID, &locID, &locName)
		if err != nil {
			log.Printf("wiki spell facts: %v", err)
			return
		}
		if srcType.Valid && srcID.Valid {
			if t, ok := wikiTypeFor(srcType.String); ok {
				var name string
				if db.QueryRow(t.oneSQL(), srcID.Int64).Scan(new(int64), &name, new(string)) == nil {
					a.addFact("source", "", "", &wikiRef{Type: t.Key, ID: srcID.Int64, Name: name})
				}
			}
		}
		a.addFact("origin", "", "", refOf("location", locID, locName))
		a.addSheet("description", desc)

	case "gear":
		var slot string
		if db.QueryRow(`SELECT slot FROM gear WHERE id = ?`, id).Scan(&slot) == nil {
			a.addFact("slot", slot, "", nil)
		}
	}
}

func loadWikiArticle(db *sql.DB, t wikiTypeDef, id int64) (*wikiArticle, error) {
	a := &wikiArticle{
		Type: t.Key, ID: id, FieldKeys: t.Fields,
		Facts: []wikiFact{}, Groups: []wikiGroup{}, Sheet: []wikiSheet{},
		Fields: map[string]string{}, Sections: []wikiSection{}, Infobox: []wikiInfoRow{},
		Backlinks: []wikiRef{},
	}
	if err := db.QueryRow(t.oneSQL(), id).Scan(new(int64), &a.Name, &a.Picture); err != nil {
		return nil, err
	}
	a.Picture = pictureURL(a.Picture)
	loadSourceFacts(db, a)

	var entryID int64
	var fieldsJSON string
	err := db.QueryRow(`SELECT id, summary, fields, updated_at FROM wiki_entries WHERE entity_type = ? AND entity_id = ?`,
		t.Key, id).Scan(&entryID, &a.Summary, &fieldsJSON, &a.UpdatedAt)
	if err == nil {
		var stored map[string]string
		if json.Unmarshal([]byte(fieldsJSON), &stored) == nil {
			for _, k := range t.Fields {
				if v := stored[k]; v != "" {
					a.Fields[k] = v
				}
			}
		}
		if rows, err := db.Query(`SELECT title, body FROM wiki_sections WHERE entry_id = ? ORDER BY position`, entryID); err == nil {
			for rows.Next() {
				var s wikiSection
				if rows.Scan(&s.Title, &s.Body) == nil {
					a.Sections = append(a.Sections, s)
				}
			}
			rows.Close()
		}
		if rows, err := db.Query(`SELECT label, value FROM wiki_infobox WHERE entry_id = ? ORDER BY position`, entryID); err == nil {
			for rows.Next() {
				var r wikiInfoRow
				if rows.Scan(&r.Label, &r.Value) == nil {
					a.Infobox = append(a.Infobox, r)
				}
			}
			rows.Close()
		}
	} else if err != sql.ErrNoRows {
		return nil, err
	}

	a.Backlinks = wikiBacklinks(db, t.Key, id)
	return a, nil
}

func getWikiHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t, ok := wikiTypeFor(r.PathValue("type"))
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if !ok || err != nil {
			http.Error(w, "article not found", http.StatusNotFound)
			return
		}
		a, err := loadWikiArticle(db, t, id)
		if err == sql.ErrNoRows {
			http.Error(w, "article not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "failed to load the article", http.StatusInternalServerError)
			log.Printf("get wiki article: %v", err)
			return
		}
		writeJSON(w, a)
	}
}

// ---- Links -------------------------------------------------------------------

// [[Name]], [[Name|shown text]] and [[type:Name]] (to pick between a class and
// a race that share a name). The frontend resolves them with the same rules.
var wikiLinkRe = regexp.MustCompile(`\[\[([^\[\]|\n]+?)(?:\|[^\[\]\n]*)?\]\]`)

func normalizeWikiKey(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.ReplaceAll(s, "_", " "))), " ")
}

// resolveWikiLink finds the article a [[...]] points at, or nil.
func resolveWikiLink(inner string, index map[string][]wikiRef) *wikiRef {
	inner = strings.TrimSpace(inner)
	if i := strings.Index(inner, ":"); i > 0 {
		prefix := normalizeWikiKey(inner[:i])
		for _, t := range wikiTypes {
			if normalizeWikiKey(t.Key) == prefix {
				name := normalizeWikiKey(inner[i+1:])
				for _, ref := range index[name] {
					if ref.Type == t.Key {
						r := ref
						return &r
					}
				}
				return nil
			}
		}
	}
	if refs := index[normalizeWikiKey(inner)]; len(refs) > 0 {
		r := refs[0] // index is built in wikiTypes order
		return &r
	}
	return nil
}

func wikiNameIndex(items []wikiListItem) map[string][]wikiRef {
	idx := map[string][]wikiRef{}
	for _, it := range items {
		k := normalizeWikiKey(it.Name)
		idx[k] = append(idx[k], wikiRef{Type: it.Type, ID: it.ID, Name: it.Name})
	}
	return idx
}

// wikiBacklinks lists the articles whose wiki text links to the given one.
func wikiBacklinks(db *sql.DB, typ string, id int64) []wikiRef {
	out := []wikiRef{}
	items, err := listWikiItems(db)
	if err != nil {
		return out
	}
	index := wikiNameIndex(items)
	names := map[string]wikiRef{}
	for _, it := range items {
		names[it.Type+":"+strconv.FormatInt(it.ID, 10)] = wikiRef{Type: it.Type, ID: it.ID, Name: it.Name}
	}

	texts := map[int64][]string{} // entry id -> everything written in it
	owner := map[int64]string{}
	rows, err := db.Query(`SELECT id, entity_type, entity_id, summary, fields FROM wiki_entries`)
	if err != nil {
		return out
	}
	for rows.Next() {
		var eid, ent int64
		var et, summary, fields string
		if rows.Scan(&eid, &et, &ent, &summary, &fields) != nil {
			continue
		}
		owner[eid] = et + ":" + strconv.FormatInt(ent, 10)
		texts[eid] = append(texts[eid], summary)
		var m map[string]string
		if json.Unmarshal([]byte(fields), &m) == nil {
			for _, v := range m {
				texts[eid] = append(texts[eid], v)
			}
		}
	}
	rows.Close()
	for _, q := range []string{
		`SELECT entry_id, title || char(10) || body FROM wiki_sections`,
		`SELECT entry_id, value FROM wiki_infobox`,
	} {
		rows, err := db.Query(q)
		if err != nil {
			continue
		}
		for rows.Next() {
			var eid int64
			var s string
			if rows.Scan(&eid, &s) == nil {
				texts[eid] = append(texts[eid], s)
			}
		}
		rows.Close()
	}

	self := typ + ":" + strconv.FormatInt(id, 10)
	seen := map[string]bool{}
	for eid, parts := range texts {
		from := owner[eid]
		if from == self || seen[from] {
			continue
		}
	scan:
		for _, p := range parts {
			for _, m := range wikiLinkRe.FindAllStringSubmatch(p, -1) {
				if ref := resolveWikiLink(m[1], index); ref != nil && ref.Type == typ && ref.ID == id {
					seen[from] = true
					if src, ok := names[from]; ok {
						out = append(out, src)
					}
					break scan
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

// ---- Writing -----------------------------------------------------------------

type wikiSavePayload struct {
	Summary  string            `json:"summary"`
	Fields   map[string]string `json:"fields"`
	Sections []wikiSection     `json:"sections"`
	Infobox  []wikiInfoRow     `json:"infobox"`
}

func tooLong(s string, max int) bool { return utf8.RuneCountInString(s) > max }

// clean validates a save and drops blank rows. It returns a message for the
// user when something is out of bounds.
func (p *wikiSavePayload) clean(t wikiTypeDef) string {
	if tooLong(p.Summary, maxWikiText) {
		return "that text is too long"
	}
	fields := map[string]string{}
	for k, v := range p.Fields {
		if !t.hasField(k) {
			continue
		}
		if tooLong(v, maxWikiText) {
			return "that text is too long"
		}
		if strings.TrimSpace(v) != "" {
			fields[k] = v
		}
	}
	p.Fields = fields

	var sections []wikiSection
	for _, s := range p.Sections {
		s.Title = strings.TrimSpace(s.Title)
		if s.Title == "" && strings.TrimSpace(s.Body) == "" {
			continue
		}
		if s.Title == "" {
			return "every section needs a title"
		}
		if tooLong(s.Title, maxWikiTitle) || tooLong(s.Body, maxWikiText) {
			return "that text is too long"
		}
		sections = append(sections, s)
	}
	if len(sections) > maxWikiSections {
		return "too many sections"
	}
	p.Sections = sections

	var rows []wikiInfoRow
	for _, r := range p.Infobox {
		r.Label = strings.TrimSpace(r.Label)
		if r.Label == "" && strings.TrimSpace(r.Value) == "" {
			continue
		}
		if r.Label == "" {
			return "every info row needs a label"
		}
		if tooLong(r.Label, maxWikiTitle) || tooLong(r.Value, 2000) {
			return "that text is too long"
		}
		rows = append(rows, r)
	}
	if len(rows) > maxWikiRows {
		return "too many info rows"
	}
	p.Infobox = rows
	return ""
}

func (p *wikiSavePayload) isEmpty() bool {
	return strings.TrimSpace(p.Summary) == "" && len(p.Fields) == 0 && len(p.Sections) == 0 && len(p.Infobox) == 0
}

func saveWikiHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t, ok := wikiTypeFor(r.PathValue("type"))
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if !ok || err != nil {
			http.Error(w, "article not found", http.StatusNotFound)
			return
		}
		if db.QueryRow(t.oneSQL(), id).Scan(new(int64), new(string), new(string)) != nil {
			http.Error(w, "article not found", http.StatusNotFound)
			return
		}
		var p wikiSavePayload
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20)).Decode(&p); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		if msg := p.clean(t); msg != "" {
			http.Error(w, msg, http.StatusBadRequest)
			return
		}

		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "failed to save the article", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		if p.isEmpty() {
			// Nothing left to keep: the article is back to just its facts.
			if _, err := tx.Exec(`DELETE FROM wiki_entries WHERE entity_type = ? AND entity_id = ?`, t.Key, id); err != nil {
				http.Error(w, "failed to save the article", http.StatusInternalServerError)
				log.Printf("clear wiki entry: %v", err)
				return
			}
		} else {
			fields, _ := json.Marshal(p.Fields)
			if _, err := tx.Exec(`
				INSERT INTO wiki_entries (entity_type, entity_id, summary, fields) VALUES (?, ?, ?, ?)
				ON CONFLICT(entity_type, entity_id) DO UPDATE SET
					summary = excluded.summary, fields = excluded.fields, updated_at = datetime('now')`,
				t.Key, id, p.Summary, string(fields)); err != nil {
				http.Error(w, "failed to save the article", http.StatusInternalServerError)
				log.Printf("save wiki entry: %v", err)
				return
			}
			var entryID int64
			if err := tx.QueryRow(`SELECT id FROM wiki_entries WHERE entity_type = ? AND entity_id = ?`, t.Key, id).Scan(&entryID); err != nil {
				http.Error(w, "failed to save the article", http.StatusInternalServerError)
				return
			}
			tx.Exec(`DELETE FROM wiki_sections WHERE entry_id = ?`, entryID)
			tx.Exec(`DELETE FROM wiki_infobox WHERE entry_id = ?`, entryID)
			for i, s := range p.Sections {
				if _, err := tx.Exec(`INSERT INTO wiki_sections (entry_id, position, title, body) VALUES (?, ?, ?, ?)`,
					entryID, i, s.Title, s.Body); err != nil {
					http.Error(w, "failed to save the article", http.StatusInternalServerError)
					log.Printf("save wiki section: %v", err)
					return
				}
			}
			for i, row := range p.Infobox {
				if _, err := tx.Exec(`INSERT INTO wiki_infobox (entry_id, position, label, value) VALUES (?, ?, ?, ?)`,
					entryID, i, row.Label, row.Value); err != nil {
					http.Error(w, "failed to save the article", http.StatusInternalServerError)
					log.Printf("save wiki infobox: %v", err)
					return
				}
			}
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to save the article", http.StatusInternalServerError)
			return
		}

		a, err := loadWikiArticle(db, t, id)
		if err != nil {
			http.Error(w, "failed to load the article", http.StatusInternalServerError)
			return
		}
		writeJSON(w, a)
	}
}
