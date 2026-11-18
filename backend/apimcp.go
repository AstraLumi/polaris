package main

// The API's tools, for LLMs: one list of tools served two ways.
//
//	POST /mcp            Model Context Protocol over streamable HTTP. Every
//	                     request gets its answer as JSON (no event stream, no
//	                     session): initialize, ping, tools/list, tools/call.
//	GET  /openapi.json   the same tools as OpenAPI 3.1 operations,
//	POST /tools/{name}   each called with its arguments as the JSON body.
//
// A key that may only read is never shown the tools that write. Every tool
// goes through apiServer.forward, so a tool's request has the same limits,
// copy and log as a request made by hand.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"slices"
	"sort"
	"strings"
	"sync"
)

const mcpProtocolLatest = "2025-06-18"

var mcpProtocols = map[string]bool{"2025-06-18": true, "2025-03-26": true, "2024-11-05": true}

// The guide every MCP client hands its model (initialize's instructions),
// and the api_guide tool's text.
const apiGuide = `Polaris is a writer's organizer for a novel: characters (each with versions over the story's time), locations, factions, events on a timeline, cultures, spells, gear, creatures, lore articles, a wiki made of all of them, and the book's chapters in volumes with their text. Apart from the story, the writer keeps drafts: loose notes and idea boards.

How to work:
- Start with list_stories, then story_overview for the story you are asked about: it gives its calendar, stats, article counts and chapters.
- Find things by name with find. Read before you change: get_article, read_chapter or get.
- Dates are DD-MM-YYYY (a year alone is allowed); there is no year 0, a year before 1 is negative.
- In any text, link to a story item as [[Name]], [[type:Name]] or [[Name|shown text]]; Polaris stores the link by id. Types: character, location, faction, event, class, subclass, specialization, race, body_type, spell, gear, creature, lore, culture, kingdom.
- Descriptions belong in the item's wiki article (update_article): a summary and fixed fields (for a character: appearance, personality, background, relationships, abilities, trivia), plus free sections.
- Prose goes in a chapter with write_chapter, as Markdown: every line is a paragraph, # headings, * * * for a scene break, **bold**, *italic*. Prefer mode "append". "replace" needs the base_version read_chapter gave and replaces the whole chapter.
- Drafts are the writer's scratch space, kept apart from the story: notes (free text) and boards of cards joined by lines. They are ideas, not canon: never take a fact from a draft into the story's articles or chapters unless the writer asks. list_drafts, read_draft, write_note and add_card work on them; a card's other changes go through send (PUT draft-cards/{id} changes only the fields sent). Card bodies are Markdown in Discord's syntax (__underline__, ||spoiler||, -# subtext, # headings, nested - lists) and may link to story items with [[Name]]; the wiki never shows links from drafts.
- Every change is logged, and the story is copied before the first change of each day, so mistakes can be undone by the writer. Still: never delete anything unless the writer asked for exactly that.

Everything else: get and send reach every route of the story's API (call api_guide for the list). Paths are relative, e.g. "characters", "events/12", "wiki/character/3". Many writes are forms (send's form argument), the rest JSON (send's json argument), as listed. A PUT replaces the whole item: get it first and send back every field, changing only what you mean to.`

// Notes on the routes an LLM uses most; the rest are listed bare.
var apiRouteNotes = map[string]string{
	"GET /api/characters":                "every character (current version's name, level, picture)",
	"POST /api/characters":               "form: name (required), nickname, level, class, subclass, specialization, version_date, version_reference",
	"GET /api/characters/{id}/current":   "the character's current version, full",
	"GET /api/characters/{id}/versions":  "every version of a character",
	"POST /api/characters/{id}/versions": "json: {version_date, version_reference, chapter_id, clone_from_version_id}",
	"GET /api/versions/{id}":             "one version, full",
	"PUT /api/versions/{id}":             "form, replaces the version: name (required), nickname, level, class, subclass, specialization, race, body_type, age, height, weight, build, bio, description, speech_mannerisms, version_date, version_reference, chapter_id, born_in_location_id, nation_location_id, human_birth_date, special_bases. Read it first, send every field back",
	"GET /api/wiki":                      "every article: type, id, name",
	"GET /api/wiki/{type}/{id}":          "an article: facts, summary, fields, sections, infobox",
	"PUT /api/wiki/{type}/{id}":          "json, replaces the written text: {summary, fields: {key: markdown}, sections: [{title, body}], infobox: [{label, value}]}; update_article merges for you",
	"POST /api/lore":                     "form: name. Makes a lore article; write its text with update_article (type lore)",
	"GET /api/events":                    "every event",
	"POST /api/events":                   "form: name (required), event_date, description, location_id, chapter_id",
	"GET /api/timeline":                  "the timeline: events and character versions by date",
	"GET /api/chapters":                  "volumes and chapters, in order",
	"POST /api/chapters":                 "json: {title, volume_id}",
	"PUT /api/chapters/{id}":             "json: the chapter's title, volume, notes (get it first)",
	"POST /api/volumes":                  "json: {title}",
	"GET /api/chapters/{id}/text":        "the chapter's document (ProseMirror JSON); read_chapter gives Markdown",
	"GET /api/locations":                 "every location",
	"GET /api/factions":                  "every faction",
	"GET /api/settings":                  "the story's settings: calendar, map",
	"GET /api/stat-system":               "the story's stats and formulas",
	"GET /api/drafts":                    "the writer's drafts, kept apart from the story (list_drafts): pages [{id, kind note|board, title, words, cards}] and the card categories [{id, name, color}]",
	"POST /api/drafts":                   "json: {kind: \"note\" or \"board\", title}: a new draft",
	"PUT /api/drafts/order":              "json: {ids}: every draft id in the order the list shows them",
	"PUT /api/drafts/{id}":               "json: {title}: rename a draft",
	"DELETE /api/drafts/{id}":            "a draft, with a board's cards and lines (only when the writer asks)",
	"GET /api/drafts/{id}/note":          "a note's document (ProseMirror JSON); read_draft gives Markdown, write_note writes it",
	"PUT /api/drafts/{id}/note":          "json: {doc, base_version}; prefer write_note",
	"GET /api/drafts/{id}/board":         "a board's cards (id, title, category_id, Markdown body, x, y, w, h) and lines (id, a, b, label); read_draft shows links by name",
	"POST /api/drafts/{id}/cards":        "json: {title, category_id, body, x, y, w, h} (board units; w 160-1600, h 80-1600); add_card places it for you",
	"PUT /api/draft-cards/{id}":          "json: only the fields to change (title, category_id, body, x, y, w, h); {no_category: true} clears the category",
	"DELETE /api/draft-cards/{id}":       "a card and its lines",
	"POST /api/drafts/{id}/links":        "json: {a, b, label}: join two cards of this board with a line (the same pair again gives the line there is)",
	"PUT /api/draft-links/{id}":          "json: {label}",
	"DELETE /api/draft-links/{id}":       "a line between two cards",
	"POST /api/draft-categories":         "json: {name, color \"#rrggbb\"}: a category for cards, shared by every board",
	"PUT /api/draft-categories/{id}":     "json: {name, color}",
	"DELETE /api/draft-categories/{id}":  "a category; its cards keep no category",
}

type apiTool struct {
	Name        string
	Description string
	Schema      map[string]any
	Write       bool
	Run         func(a *apiServer, r *http.Request, k *apiKey, args map[string]any) (any, error)
}

func obj(props map[string]any, required ...string) map[string]any {
	s := map[string]any{"type": "object", "properties": props, "additionalProperties": false}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
func num(desc string) map[string]any { return map[string]any{"type": "integer", "description": desc} }

var storyArg = str("The story's id, from list_stories.")

var apiTools []apiTool

func init() {
	apiTools = []apiTool{
		{Name: "list_stories", Description: "The stories this key reaches: id, name, how many characters.",
			Schema: obj(map[string]any{}),
			Run: func(a *apiServer, r *http.Request, k *apiKey, _ map[string]any) (any, error) {
				return a.storiesFor(k)
			}},
		{Name: "api_guide", Description: "How Polaris is organised and every route of a story's API, with the fields of the common ones. Call it before using get or send on a route you haven't used.",
			Schema: obj(map[string]any{}),
			Run: func(a *apiServer, r *http.Request, k *apiKey, _ map[string]any) (any, error) {
				return map[string]any{"guide": apiGuide, "routes": apiRouteList(k.CanWrite)}, nil
			}},
		{Name: "story_overview", Description: "One story at a glance: its settings (calendar, months, eras), its stats, how many articles of each kind, and its volumes and chapters.",
			Schema: obj(map[string]any{"story": storyArg}, "story"),
			Run:    toolOverview},
		{Name: "find", Description: "Find story items by name (characters, locations, events, lore… every wiki article). Case-insensitive; also matches a character's other names.",
			Schema: obj(map[string]any{
				"story": storyArg,
				"query": str("Part of the name; empty lists everything."),
				"type":  str("Only this kind, e.g. character, location, event, lore."),
				"limit": num("At most this many (default 50, at most 500)."),
			}, "story"),
			Run: toolFind},
		{Name: "get_article", Description: "An item's wiki article: its facts, summary, fields, sections and infobox, with links shown by name.",
			Schema: obj(map[string]any{"story": storyArg, "type": str("The item's kind, e.g. character."), "id": num("The item's id.")}, "story", "type", "id"),
			Run:    toolGetArticle},
		{Name: "update_article", Write: true, Description: "Change an item's wiki article. Only what you send changes; the rest stays. fields are the article's fixed boxes (get_article lists their keys), sections replace the free sections when sent. Text is Markdown with [[links]].",
			Schema: obj(map[string]any{
				"story":   storyArg,
				"type":    str("The item's kind, e.g. character."),
				"id":      num("The item's id."),
				"summary": str("The article's opening summary."),
				"fields":  map[string]any{"type": "object", "description": "Field key to Markdown text; an empty text clears it.", "additionalProperties": map[string]any{"type": "string"}},
				"sections": map[string]any{"type": "array", "description": "The free sections, in order (replaces all of them).",
					"items": obj(map[string]any{"title": str("Heading."), "body": str("Markdown.")}, "title", "body")},
				"infobox": map[string]any{"type": "array", "description": "Extra rows of the infobox (replaces all of them).",
					"items": obj(map[string]any{"label": str("Label."), "value": str("Value.")}, "label", "value")},
			}, "story", "type", "id"),
			Run: toolUpdateArticle},
		{Name: "read_chapter", Description: "A chapter's text as Markdown, with its version (needed to replace it) and word count.",
			Schema: obj(map[string]any{"story": storyArg, "chapter_id": num("The chapter's id, from story_overview.")}, "story", "chapter_id"),
			Run:    toolReadChapter},
		{Name: "write_chapter", Write: true, Description: "Write prose into a chapter, from Markdown: every line is a paragraph, # headings, * * * a scene break, **bold**, *italic*, - lists, [[Name]] links. mode append (default) adds after the text; replace swaps the whole text and needs base_version from read_chapter (it refuses when the chapter holds what Markdown can't keep, such as dialogue blocks, pictures or notes, unless allow_loss is true).",
			Schema: obj(map[string]any{
				"story":        storyArg,
				"chapter_id":   num("The chapter's id."),
				"markdown":     str("The text."),
				"mode":         map[string]any{"type": "string", "enum": []string{"append", "replace"}, "description": "append (default) or replace."},
				"base_version": num("For replace: the version read_chapter gave."),
				"allow_loss":   map[string]any{"type": "boolean", "description": "For replace: go ahead even if formatting Markdown can't hold is lost."},
			}, "story", "chapter_id", "markdown"),
			Run: toolWriteChapter},
		{Name: "get", Description: "Read any route of a story's API (api_guide lists them), e.g. \"characters\", \"events/12\", \"timeline\". Answers JSON.",
			Schema: obj(map[string]any{"story": storyArg, "path": str("The path after /api/, with a ?query if needed.")}, "story", "path"),
			Run: func(a *apiServer, r *http.Request, k *apiKey, args map[string]any) (any, error) {
				var out json.RawMessage
				_, err := a.callJSON(r, k, argStr(args, "story"), http.MethodGet, argStr(args, "path"), nil, &out, "get")
				return out, err
			}},
		{Name: "send", Write: true, Description: "Change something through any route of a story's API: POST makes, PUT replaces, DELETE removes. Give json or form, as api_guide says for the route. A PUT replaces the whole item, so get it first and send back every field. Delete only what the writer asked for.",
			Schema: obj(map[string]any{
				"story":  storyArg,
				"method": map[string]any{"type": "string", "enum": []string{"POST", "PUT", "DELETE"}},
				"path":   str("The path after /api/, e.g. \"events\" or \"events/12\"."),
				"json":   map[string]any{"description": "A JSON body, for routes that take JSON."},
				"form":   map[string]any{"type": "object", "description": "Form fields (text), for routes that take a form.", "additionalProperties": map[string]any{"type": []string{"string", "number", "boolean"}}},
			}, "story", "method", "path"),
			Run: toolSend},
	}
	// The drafts' tools (apidrafts.go) go before get and send.
	apiTools = slices.Insert(apiTools, len(apiTools)-2, draftTools()...)
}

func argStr(args map[string]any, k string) string {
	switch v := args[k].(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return fmt.Sprintf("%.0f", v)
	case json.Number:
		return v.String()
	}
	return ""
}

func argInt(args map[string]any, k string) (int64, bool) {
	switch v := args[k].(type) {
	case float64:
		return int64(v), v == float64(int64(v)) && v > 0
	case string:
		var n int64
		_, err := fmt.Sscan(v, &n)
		return n, err == nil && n > 0
	}
	return 0, false
}

// apiRouteList is every route of a story's API, from the route table
// itself, with its note.
func apiRouteList(write bool) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range storyRoutePatterns() {
		if seen[p] || p == "GET /api/health" {
			continue
		}
		seen[p] = true
		method, _, _ := strings.Cut(p, " ")
		if !write && method != "GET" {
			continue
		}
		line := strings.Replace(p, "/api/", "", 1)
		if n := apiRouteNotes[p]; n != "" {
			line += "  — " + n
		}
		out = append(out, line)
	}
	return out
}

// storyRoutePatterns reads the patterns a story's mux was given.
var (
	storyPatternList []string
	storyPatternOnce sync.Once
)

func storyRoutePatterns() []string {
	storyPatternOnce.Do(func() {
		var list []string
		rec := &patternRecorder{list: &list}
		registerStoryRoutes(rec, nil, "", nil)
		sort.SliceStable(list, func(i, j int) bool {
			pi, pj := strings.SplitN(list[i], " ", 2), strings.SplitN(list[j], " ", 2)
			if len(pi) < 2 || len(pj) < 2 || pi[1] != pj[1] {
				return list[i] < list[j]
			}
			return methodRank(pi[0]) < methodRank(pj[0])
		})
		storyPatternList = list
	})
	return storyPatternList
}

// patternRecorder keeps the patterns instead of serving them.
type patternRecorder struct{ list *[]string }

func (p *patternRecorder) HandleFunc(pattern string, _ func(http.ResponseWriter, *http.Request)) {
	*p.list = append(*p.list, pattern)
}
func (p *patternRecorder) Handle(pattern string, _ http.Handler) { *p.list = append(*p.list, pattern) }

func methodRank(m string) int {
	return map[string]int{"GET": 0, "POST": 1, "PUT": 2, "DELETE": 3}[m]
}

// ---- the tools ----

func toolOverview(a *apiServer, r *http.Request, k *apiKey, args map[string]any) (any, error) {
	story := argStr(args, "story")
	var settings, stats, chapters json.RawMessage
	if _, err := a.callJSON(r, k, story, http.MethodGet, "settings", nil, &settings, "story_overview"); err != nil {
		return nil, err
	}
	a.callJSON(r, k, story, http.MethodGet, "stat-system", nil, &stats, "story_overview")
	a.callJSON(r, k, story, http.MethodGet, "chapters", nil, &chapters, "story_overview")
	var items []wikiListItem
	a.callJSON(r, k, story, http.MethodGet, "wiki", nil, &items, "story_overview")
	counts := map[string]int{}
	for _, it := range items {
		counts[it.Type]++
	}
	return map[string]any{"settings": settings, "stat_system": stats, "articles": counts, "chapters": chapters}, nil
}

func toolFind(a *apiServer, r *http.Request, k *apiKey, args map[string]any) (any, error) {
	var items []wikiListItem
	if _, err := a.callJSON(r, k, argStr(args, "story"), http.MethodGet, "wiki", nil, &items, "find"); err != nil {
		return nil, err
	}
	q := strings.ToLower(argStr(args, "query"))
	typ := argStr(args, "type")
	limit, ok := argInt(args, "limit")
	if !ok || limit > 500 {
		limit = 50
	}
	type hit struct {
		Type    string   `json:"type"`
		ID      int64    `json:"id"`
		Name    string   `json:"name"`
		Aliases []string `json:"aliases,omitempty"`
	}
	out := []hit{}
	total := 0
	for _, it := range items {
		if typ != "" && it.Type != typ {
			continue
		}
		match := q == "" || strings.Contains(strings.ToLower(it.Name), q)
		for _, al := range it.Aliases {
			match = match || strings.Contains(strings.ToLower(al), q)
		}
		if !match {
			continue
		}
		total++
		if int64(len(out)) < limit {
			out = append(out, hit{it.Type, it.ID, it.Name, it.Aliases})
		}
	}
	return map[string]any{"found": total, "items": out}, nil
}

// articleText is the written part of an article (what PUT /wiki takes).
type articleText struct {
	Summary  string            `json:"summary"`
	Fields   map[string]string `json:"fields"`
	Sections []wikiSection     `json:"sections"`
	Infobox  []wikiInfoRow     `json:"infobox"`
}

func articlePath(args map[string]any) (string, error) {
	typ := argStr(args, "type")
	id, ok := argInt(args, "id")
	if _, known := wikiTypeFor(typ); !known || !ok {
		return "", errors.New("unknown article: give a type (such as character) and an id, from find")
	}
	return fmt.Sprintf("wiki/%s/%d", typ, id), nil
}

// shownByName turns stored [[type#id]] links back into names, as the app's
// editors do, so the model reads names and its edits keep them.
func shownByName(x *wikiLinkIndex, s string) string {
	if x == nil {
		return s
	}
	return mapWikiLinks(s, func(m wikiLinkMatch) string {
		typ, id, ok := parseWikiIDLink(m.Target)
		if !ok {
			return m.Full
		}
		ref := x.resolve(wikiIDLink(typ, id))
		if ref == nil {
			return m.Full
		}
		if m.HasText {
			return "[[" + typ + ":" + ref.Name + "|" + m.Text + "]]"
		}
		return "[[" + typ + ":" + ref.Name + "]]"
	})
}

func (a *apiServer) linkIndex(r *http.Request, k *apiKey, story string) *wikiLinkIndex {
	var items []wikiListItem
	if _, err := a.callJSON(r, k, story, http.MethodGet, "wiki", nil, &items, "links"); err != nil {
		return nil
	}
	return newWikiLinkIndex(items)
}

func toolGetArticle(a *apiServer, r *http.Request, k *apiKey, args map[string]any) (any, error) {
	path, err := articlePath(args)
	if err != nil {
		return nil, err
	}
	story := argStr(args, "story")
	var art map[string]any
	if _, err := a.callJSON(r, k, story, http.MethodGet, path, nil, &art, "get_article"); err != nil {
		return nil, err
	}
	x := a.linkIndex(r, k, story)
	var text articleText
	raw, _ := json.Marshal(art)
	json.Unmarshal(raw, &text)
	for key, v := range text.Fields {
		text.Fields[key] = shownByName(x, v)
	}
	for i := range text.Sections {
		text.Sections[i].Body = shownByName(x, text.Sections[i].Body)
	}
	out := map[string]any{
		"type": art["type"], "id": art["id"], "name": art["name"],
		"facts": art["facts"], "groups": art["groups"], "field_keys": art["field_keys"],
		"summary": shownByName(x, text.Summary), "fields": text.Fields,
		"sections": text.Sections, "infobox": text.Infobox, "backlinks": art["backlinks"],
	}
	return out, nil
}

func toolUpdateArticle(a *apiServer, r *http.Request, k *apiKey, args map[string]any) (any, error) {
	path, err := articlePath(args)
	if err != nil {
		return nil, err
	}
	story := argStr(args, "story")
	var cur articleText
	if _, err := a.callJSON(r, k, story, http.MethodGet, path, nil, &cur, "update_article"); err != nil {
		return nil, err
	}
	if cur.Fields == nil {
		cur.Fields = map[string]string{}
	}
	if v, ok := args["summary"].(string); ok {
		cur.Summary = v
	}
	if f, ok := args["fields"].(map[string]any); ok {
		for key, v := range f {
			s, _ := v.(string)
			cur.Fields[key] = s
		}
	}
	if raw, ok := args["sections"]; ok {
		b, _ := json.Marshal(raw)
		var secs []wikiSection
		if json.Unmarshal(b, &secs) != nil {
			return nil, errors.New("sections: a list of {title, body}")
		}
		cur.Sections = secs
	}
	if raw, ok := args["infobox"]; ok {
		b, _ := json.Marshal(raw)
		var rows []wikiInfoRow
		if json.Unmarshal(b, &rows) != nil {
			return nil, errors.New("infobox: a list of {label, value}")
		}
		cur.Infobox = rows
	}
	if cur.Sections == nil {
		cur.Sections = []wikiSection{}
	}
	if cur.Infobox == nil {
		cur.Infobox = []wikiInfoRow{}
	}
	if _, err := a.callJSON(r, k, story, http.MethodPut, path, cur, nil, "update_article"); err != nil {
		return nil, err
	}
	return map[string]any{"saved": true}, nil
}

func chapterTextPath(args map[string]any) (string, error) {
	id, ok := argInt(args, "chapter_id")
	if !ok {
		return "", errors.New("chapter_id: the chapter's id, from story_overview")
	}
	return fmt.Sprintf("chapters/%d/text", id), nil
}

func toolReadChapter(a *apiServer, r *http.Request, k *apiKey, args map[string]any) (any, error) {
	path, err := chapterTextPath(args)
	if err != nil {
		return nil, err
	}
	story := argStr(args, "story")
	var t chapterText
	if _, err := a.callJSON(r, k, story, http.MethodGet, path, nil, &t, "read_chapter"); err != nil {
		return nil, err
	}
	doc, err := parsePMDoc(t.Doc)
	if err != nil {
		return nil, errors.New("the chapter's text could not be read")
	}
	return map[string]any{"version": t.Version, "words": t.Words, "updated_at": t.UpdatedAt,
		"markdown": docMarkdown(doc, a.linkIndex(r, k, story))}, nil
}

func toolWriteChapter(a *apiServer, r *http.Request, k *apiKey, args map[string]any) (any, error) {
	path, err := chapterTextPath(args)
	if err != nil {
		return nil, err
	}
	return writeMarkdownText(a, r, k, args, path, "chapter", "read_chapter", "write_chapter")
}

// writeMarkdownText adds Markdown after a document (a chapter's text, a
// draft note) or replaces it, through its GET/PUT route at path; noun and
// readTool word the errors.
func writeMarkdownText(a *apiServer, r *http.Request, k *apiKey, args map[string]any, path, noun, readTool, tool string) (any, error) {
	story := argStr(args, "story")
	md, _ := args["markdown"].(string)
	mode := argStr(args, "mode")
	if mode == "" {
		mode = "append"
	}
	if mode != "append" && mode != "replace" {
		return nil, errors.New(`mode is "append" or "replace"`)
	}
	blocks := markdownBlocks(md, a.linkIndex(r, k, story))
	if len(blocks) == 0 && mode == "append" {
		return nil, errors.New("there is no text to add")
	}
	for attempt := 0; ; attempt++ {
		var t chapterText
		if _, err := a.callJSON(r, k, story, http.MethodGet, path, nil, &t, tool); err != nil {
			return nil, err
		}
		doc, err := parsePMDoc(t.Doc)
		if err != nil {
			return nil, fmt.Errorf("the %s's text could not be read", noun)
		}
		if mode == "replace" {
			base, ok := args["base_version"].(float64)
			if !ok || int64(base) != t.Version {
				return nil, fmt.Errorf("the %s is at version %d: %s it again and pass that base_version (it may have changed since you read it)", noun, t.Version, readTool)
			}
			if fine, why := markdownCanReplace(doc); !fine && args["allow_loss"] != true {
				return nil, fmt.Errorf("replacing would lose part of the %s: %s. Use append, or replace with allow_loss true if the writer agreed", noun, why)
			}
			doc = &pmNode{Type: "doc", Attrs: doc.Attrs, Content: blocks}
			if len(doc.Content) == 0 {
				doc.Content = emptyPMDoc().Content
			}
		} else {
			// An empty text's one blank paragraph gives way to the new text.
			if len(doc.Content) == 1 && doc.Content[0].Type == "paragraph" && len(doc.Content[0].Content) == 0 {
				doc.Content = nil
			}
			doc.Content = append(doc.Content, blocks...)
		}
		var saved chapterText
		status, err := a.callJSON(r, k, story, http.MethodPut, path, map[string]any{"doc": doc, "base_version": t.Version}, &saved, tool)
		if status == http.StatusConflict && mode == "append" && attempt < 2 {
			continue // written meanwhile: add to the newer text
		}
		if status == http.StatusConflict {
			return nil, fmt.Errorf("the %s changed while writing: read it again", noun)
		}
		if err != nil {
			return nil, err
		}
		return map[string]any{"saved": true, "version": saved.Version, "words": saved.Words}, nil
	}
}

func toolSend(a *apiServer, r *http.Request, k *apiKey, args map[string]any) (any, error) {
	method := strings.ToUpper(argStr(args, "method"))
	if method != http.MethodPost && method != http.MethodPut && method != http.MethodDelete {
		return nil, errors.New("method is POST, PUT or DELETE")
	}
	var body []byte
	ct := ""
	if form, ok := args["form"].(map[string]any); ok {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		keys := make([]string, 0, len(form))
		for key := range form {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			v := form[key]
			s := ""
			switch t := v.(type) {
			case string:
				s = t
			case float64:
				s = fmt.Sprintf("%v", t)
			case bool:
				s = fmt.Sprint(t)
			case nil:
			default:
				b, _ := json.Marshal(t)
				s = string(b)
			}
			mw.WriteField(key, s)
		}
		mw.Close()
		body, ct = buf.Bytes(), mw.FormDataContentType()
	} else if j, ok := args["json"]; ok {
		body, _ = json.Marshal(j)
		ct = "application/json"
	}
	status, resp := a.call(r, k, argStr(args, "story"), method, argStr(args, "path"), body, ct, "send")
	if status >= 300 {
		return nil, apiCallError(status, resp)
	}
	if json.Valid(resp) {
		return json.RawMessage(resp), nil
	}
	return map[string]any{"status": status, "answer": string(resp)}, nil
}

// ---- running a tool ----

func toolsFor(k *apiKey) []apiTool {
	var out []apiTool
	for _, t := range apiTools {
		if !t.Write || k.CanWrite {
			out = append(out, t)
		}
	}
	return out
}

func (a *apiServer) runTool(r *http.Request, k *apiKey, name string, args map[string]any) (any, error) {
	for _, t := range toolsFor(k) {
		if t.Name == name {
			if args == nil {
				args = map[string]any{}
			}
			return t.Run(a, r, k, args)
		}
	}
	return nil, fmt.Errorf("no tool %q for this key", name)
}

// ---- MCP ----

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

func (a *apiServer) mcp(w http.ResponseWriter, r *http.Request) {
	var raw json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, rpcResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{-32700, "parse error"}})
		return
	}
	k := keyOf(r)
	if t := bytes.TrimSpace(raw); len(t) > 0 && t[0] == '[' {
		var reqs []rpcRequest
		if json.Unmarshal(raw, &reqs) != nil || len(reqs) == 0 || len(reqs) > 20 {
			writeJSONStatus(w, http.StatusBadRequest, rpcResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{-32600, "invalid request"}})
			return
		}
		var out []rpcResponse
		for _, q := range reqs {
			if resp, ok := a.rpc(r, k, q); ok {
				out = append(out, resp)
			}
		}
		if len(out) == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		writeJSON(w, out)
		return
	}
	var q rpcRequest
	if json.Unmarshal(raw, &q) != nil {
		writeJSONStatus(w, http.StatusBadRequest, rpcResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{-32600, "invalid request"}})
		return
	}
	resp, ok := a.rpc(r, k, q)
	if !ok {
		w.WriteHeader(http.StatusAccepted) // a notification
		return
	}
	writeJSON(w, resp)
}

// rpc answers one request; ok is false for a notification (no answer).
func (a *apiServer) rpc(r *http.Request, k *apiKey, q rpcRequest) (rpcResponse, bool) {
	if len(q.ID) == 0 || string(q.ID) == "null" {
		return rpcResponse{}, false
	}
	resp := rpcResponse{JSONRPC: "2.0", ID: q.ID}
	switch q.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		json.Unmarshal(q.Params, &p)
		ver := mcpProtocolLatest
		if mcpProtocols[p.ProtocolVersion] {
			ver = p.ProtocolVersion
		}
		resp.Result = map[string]any{
			"protocolVersion": ver,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": "polaris", "title": "Polaris", "version": fmt.Sprint(currentSchemaVersion)},
			"instructions":    apiGuide,
		}
	case "ping":
		resp.Result = map[string]any{}
	case "tools/list":
		var tools []map[string]any
		for _, t := range toolsFor(k) {
			tools = append(tools, map[string]any{
				"name": t.Name, "description": t.Description, "inputSchema": t.Schema,
				"annotations": map[string]any{"readOnlyHint": !t.Write, "destructiveHint": t.Name == "send" || t.Name == "write_chapter"},
			})
		}
		resp.Result = map[string]any{"tools": tools}
	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if json.Unmarshal(q.Params, &p) != nil {
			resp.Error = &rpcError{-32602, "invalid params"}
			break
		}
		out, err := a.runTool(r, k, p.Name, p.Arguments)
		if err != nil {
			resp.Result = map[string]any{"content": []map[string]any{{"type": "text", "text": err.Error()}}, "isError": true}
			break
		}
		text, _ := json.MarshalIndent(out, "", " ")
		resp.Result = map[string]any{"content": []map[string]any{{"type": "text", "text": string(text)}}}
	default:
		resp.Error = &rpcError{-32601, "method not found: " + q.Method}
	}
	return resp, true
}

// ---- OpenAPI ----

func (a *apiServer) openAPI(w http.ResponseWriter, r *http.Request) {
	paths := map[string]any{}
	for _, t := range toolsFor(keyOf(r)) {
		paths["/tools/"+t.Name] = map[string]any{"post": map[string]any{
			"operationId": t.Name,
			"summary":     t.Name,
			"description": t.Description,
			"requestBody": map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": t.Schema}}},
			"responses": map[string]any{
				"200":     map[string]any{"description": "The tool's answer", "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{}}}},
				"default": map[string]any{"description": "An error", "content": map[string]any{"application/json": map[string]any{"schema": obj(map[string]any{"error": str("What went wrong.")})}}},
			},
		}}
	}
	writeJSON(w, map[string]any{
		"openapi": "3.1.0",
		"info":    map[string]any{"title": "Polaris", "version": fmt.Sprint(currentSchemaVersion), "description": apiGuide},
		"paths":   paths,
		"components": map[string]any{"securitySchemes": map[string]any{
			"bearer": map[string]any{"type": "http", "scheme": "bearer", "description": "A Polaris API key (pol_…), made in Settings → API access."},
		}},
		"security": []map[string]any{{"bearer": []string{}}},
	})
}

func (a *apiServer) toolREST(w http.ResponseWriter, r *http.Request) {
	args := map[string]any{}
	if err := json.NewDecoder(r.Body).Decode(&args); err != nil && err.Error() != "EOF" {
		apiError(w, http.StatusBadRequest, "the body must be a JSON object of the tool's arguments")
		return
	}
	out, err := a.runTool(r, keyOf(r), r.PathValue("name"), args)
	if err != nil {
		status := http.StatusBadRequest
		var ce *apiCallErr
		if errors.As(err, &ce) {
			status = ce.status
		}
		apiError(w, status, err.Error())
		return
	}
	writeJSON(w, out)
}
