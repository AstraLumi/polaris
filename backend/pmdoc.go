package main

// ProseMirror documents: each chapter's text in the writing editor. The
// contract (node and mark types, attrs, units) is docs/EDITOR.md, "The
// document"; the schema itself is frontend/src/editor/schema.js.
//
// API:
//
//	parsePMDoc(data []byte) (*pmNode, error)
//	    Decode a document sent by the client or read from the database and
//	    validate it: size limit (maxPMDocBytes), nesting limit, a "doc"
//	    root, known node and mark types, content rules, safe links and
//	    picture paths. Errors are meant for a 400 answer.
//	validatePMDoc(doc *pmNode) error
//	    The same checks on a document built in Go (the .docx import).
//	json.Marshal(doc) / json.Unmarshal(data, &node)
//	    Round trip: attrs (and any keys the Go side doesn't know, on nodes
//	    and marks) are kept as they came; numbers keep their exact text.
//	docWords(doc) int          EDITOR.md's word rule (the server's count).
//	docText(doc) string        Plain text: one line per block, hard breaks
//	                           as new lines, tabs as tabs, scene breaks as
//	                           their glyph.
//	docStoryLinks(doc) []pmStoryLink
//	    Every story_link mark's (type, id), in reading order, without
//	    repeats: what a chapter links to, for "appears in" lists.
//	    (chapterindex.go counts them, and the dialogues, per chapter.)
//	emptyPMDoc() *pmNode       A doc holding one empty paragraph.
//	docCommentIDs(doc)         Every comment mark's id (comments.go); a text
//	                           may carry several comment marks, each with
//	                           its own id.
//	docNotes(doc)              Every footnote and endnote (stage 5) in
//	                           reading order, with its number per kind
//	                           (exportnotes.go).
//
// Notes (stage 5): a `footnote` is an inline node (attr kind: footnote,
// the default, or endnote) holding one or more paragraphs, in any block
// that holds text, never inside another note. Its paragraphs count as
// words and their story links as mentions, like the text around them.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxPMDocBytes = 5 << 20 // a chapter's JSON
	maxPMDepth    = 64      // nesting of JSON values in a doc (a list level costs four: Word's nine levels fit)
	maxPMNodes    = 500000
)

// pmNode is one ProseMirror node. Attrs holds every attribute as decoded
// (json.Number for numbers read from JSON); Extra any other key.
type pmNode struct {
	Type    string
	Attrs   map[string]any
	Content []*pmNode
	Marks   []pmMark
	Text    string
	Extra   map[string]json.RawMessage
}

type pmMark struct {
	Type  string
	Attrs map[string]any
	Extra map[string]json.RawMessage
}

// pmStoryLink is a story_link mark's target (the wiki's type keys and ids).
type pmStoryLink struct {
	Type string `json:"type"`
	ID   int64  `json:"id"`
}

func emptyPMDoc() *pmNode {
	return &pmNode{Type: "doc", Content: []*pmNode{{Type: "paragraph"}}}
}

// --- JSON ---

func decodeAttrs(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

func (n *pmNode) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*n = pmNode{}
	for k, v := range raw {
		var err error
		switch k {
		case "type":
			err = json.Unmarshal(v, &n.Type)
		case "attrs":
			n.Attrs, err = decodeAttrs(v)
		case "content":
			err = json.Unmarshal(v, &n.Content)
		case "marks":
			err = json.Unmarshal(v, &n.Marks)
		case "text":
			err = json.Unmarshal(v, &n.Text)
		default:
			if n.Extra == nil {
				n.Extra = map[string]json.RawMessage{}
			}
			n.Extra[k] = v
		}
		if err != nil {
			return fmt.Errorf("%s: %w", k, err)
		}
	}
	return nil
}

func (n *pmNode) MarshalJSON() ([]byte, error) {
	out := struct {
		Type    string         `json:"type"`
		Attrs   map[string]any `json:"attrs,omitempty"`
		Content []*pmNode      `json:"content,omitempty"`
		Marks   []pmMark       `json:"marks,omitempty"`
		Text    *string        `json:"text,omitempty"`
	}{Type: n.Type, Attrs: n.Attrs, Content: n.Content, Marks: n.Marks}
	if n.Type == "text" {
		out.Text = &n.Text
	}
	b, err := json.Marshal(out)
	if err != nil || len(n.Extra) == 0 {
		return b, err
	}
	return mergeExtra(b, n.Extra)
}

func (m *pmMark) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*m = pmMark{}
	for k, v := range raw {
		var err error
		switch k {
		case "type":
			err = json.Unmarshal(v, &m.Type)
		case "attrs":
			m.Attrs, err = decodeAttrs(v)
		default:
			if m.Extra == nil {
				m.Extra = map[string]json.RawMessage{}
			}
			m.Extra[k] = v
		}
		if err != nil {
			return fmt.Errorf("mark %s: %w", k, err)
		}
	}
	return nil
}

func (m pmMark) MarshalJSON() ([]byte, error) {
	out := struct {
		Type  string         `json:"type"`
		Attrs map[string]any `json:"attrs,omitempty"`
	}{m.Type, m.Attrs}
	b, err := json.Marshal(out)
	if err != nil || len(m.Extra) == 0 {
		return b, err
	}
	return mergeExtra(b, m.Extra)
}

// mergeExtra adds keys the Go side doesn't know back to an encoded object.
func mergeExtra(obj []byte, extra map[string]json.RawMessage) ([]byte, error) {
	var all map[string]json.RawMessage
	if err := json.Unmarshal(obj, &all); err != nil {
		return nil, err
	}
	for k, v := range extra {
		if _, known := all[k]; !known {
			all[k] = v
		}
	}
	return json.Marshal(all)
}

// --- parsing and validation ---

// checkJSONDepth rejects deeply nested JSON before it is decoded.
func checkJSONDepth(data []byte, max int) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	depth := 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if d, ok := tok.(json.Delim); ok {
			switch d {
			case '{', '[':
				depth++
				if depth > max {
					return errors.New("the document is nested too deeply")
				}
			default:
				depth--
			}
		}
	}
}

func parsePMDoc(data []byte) (*pmNode, error) {
	if len(data) > maxPMDocBytes {
		return nil, fmt.Errorf("the document is too large (over %d MB)", maxPMDocBytes>>20)
	}
	if err := checkJSONDepth(data, maxPMDepth); err != nil {
		return nil, fmt.Errorf("invalid document: %w", err)
	}
	var doc pmNode
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("invalid document: %w", err)
	}
	if err := validatePMDoc(&doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

var (
	pmBlockTypes  = map[string]bool{"paragraph": true, "heading": true, "scene_break": true, "page_break": true, "image": true, "bullet_list": true, "ordered_list": true, "toc": true, "dialogue": true}
	pmInlineTypes = map[string]bool{"text": true, "hard_break": true, "tab": true}
	pmAtomTypes   = map[string]bool{"scene_break": true, "page_break": true, "image": true, "toc": true, "hard_break": true, "tab": true}
	pmMarkTypes   = map[string]bool{"bold": true, "italic": true, "underline": true, "strike": true, "superscript": true, "subscript": true, "smallcaps": true, "font": true, "size": true, "color": true, "highlight": true, "link": true, "story_link": true, "char_style": true}
	// Marks a text may carry more than once, with different ids (stage 5's
	// overlapping comments; comments.go).
	pmRepeatMarks = map[string]bool{"comment": true}
	pmAligns      = map[string]bool{"left": true, "center": true, "right": true, "justify": true}
	pmColorRe     = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	pmStoryTypeRe = regexp.MustCompile(`^[a-z_]{1,40}$`)
	pmSchemeRe    = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9+.-]*):`)
)

// pmNum reads a number attr however it was made (decoded or built in Go).
func pmNum(v any) (float64, bool) {
	switch x := v.(type) {
	case json.Number:
		f, err := x.Float64()
		return f, err == nil && !math.IsInf(f, 0) && !math.IsNaN(f)
	case float64:
		return x, !math.IsInf(x, 0) && !math.IsNaN(x)
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case int32:
		return float64(x), true
	}
	return 0, false
}

func pmInt(v any) (int64, bool) {
	switch x := v.(type) {
	case json.Number:
		i, err := strconv.ParseInt(x.String(), 10, 64)
		return i, err == nil
	case string:
		i, err := strconv.ParseInt(x, 10, 64)
		return i, err == nil
	}
	f, ok := pmNum(v)
	if !ok || f != math.Trunc(f) || math.Abs(f) > 1<<53 {
		return 0, false
	}
	return int64(f), true
}

// safeLinkHref allows web, mail and phone links, in-book anchors and
// relative paths; never script or data URLs.
func safeLinkHref(href string) bool {
	if href == "" || len(href) > 2048 || strings.ContainsAny(href, "\x00\r\n") {
		return false
	}
	m := pmSchemeRe.FindStringSubmatch(strings.TrimSpace(href))
	if m == nil {
		return true
	}
	switch strings.ToLower(m[1]) {
	case "http", "https", "mailto", "tel":
		return true
	}
	return false
}

// safeUploadSrc: a picture is always one of the story's uploads.
func safeUploadSrc(src string) bool {
	return strings.HasPrefix(src, "/uploads/") && !strings.Contains(src, "..") &&
		!strings.ContainsAny(src, "\\\"'<>\x00\r\n") && len(src) <= 512
}

type pmValidator struct {
	nodes  int
	inNote bool // inside a footnote's paragraphs: no notes there
}

func validatePMDoc(doc *pmNode) error {
	if doc == nil || doc.Type != "doc" {
		return errors.New("the document must have a doc root")
	}
	// The root's mode (stage 4): prose (the default) or script.
	if m, ok := doc.Attrs["mode"]; ok && m != nil && m != "prose" && m != "script" {
		return errors.New("doc: attr mode must be prose or script")
	}
	v := &pmValidator{}
	if len(doc.Content) == 0 {
		return errors.New("doc: needs at least one block")
	}
	for _, c := range doc.Content {
		if c != nil && c.Type == "table" { // tables only at the top level (pmtable.go)
			if err := v.table(c, 1); err != nil {
				return err
			}
			continue
		}
		if err := v.block(c, "doc", 1); err != nil {
			return err
		}
	}
	return nil
}

func (v *pmValidator) count() error {
	v.nodes++
	if v.nodes > maxPMNodes {
		return errors.New("the document has too many nodes")
	}
	return nil
}

func (v *pmValidator) block(n *pmNode, parent string, depth int) error {
	if n == nil {
		return fmt.Errorf("%s: empty node", parent)
	}
	if err := v.count(); err != nil {
		return err
	}
	if depth > maxPMDepth {
		return errors.New("the document is nested too deeply")
	}
	if !pmBlockTypes[n.Type] {
		return fmt.Errorf("%s: unknown or misplaced node %q", parent, n.Type)
	}
	if len(n.Marks) > 0 {
		return fmt.Errorf("%s: block nodes have no marks", n.Type)
	}
	if n.Text != "" {
		return fmt.Errorf("%s: only text nodes have text", n.Type)
	}
	if err := checkBlockAttrs(n); err != nil {
		return err
	}
	switch n.Type {
	case "paragraph", "heading", "dialogue":
		for _, c := range n.Content {
			var err error
			if c != nil && c.Type == "footnote" {
				err = v.footnote(c, depth+1)
			} else {
				err = v.inline(c, n.Type)
			}
			if err != nil {
				return err
			}
		}
	case "bullet_list", "ordered_list":
		if len(n.Content) == 0 {
			return fmt.Errorf("%s: needs at least one list_item", n.Type)
		}
		for _, item := range n.Content {
			if item == nil || item.Type != "list_item" {
				return fmt.Errorf("%s: may only hold list_item", n.Type)
			}
			if err := v.count(); err != nil {
				return err
			}
			// A paragraph, then any blocks: more paragraphs, nested lists.
			if len(item.Content) == 0 || item.Content[0] == nil || item.Content[0].Type != "paragraph" {
				return errors.New("list_item: must start with a paragraph")
			}
			for _, c := range item.Content {
				if err := v.block(c, "list_item", depth+2); err != nil {
					return err
				}
			}
		}
	default: // atoms
		if len(n.Content) > 0 {
			return fmt.Errorf("%s: has no content", n.Type)
		}
	}
	return nil
}

// footnote checks a note (stage 5): its kind, and one or more paragraphs
// holding no notes.
func (v *pmValidator) footnote(n *pmNode, depth int) error {
	if err := v.count(); err != nil {
		return err
	}
	if v.inNote {
		return errors.New("footnote: a note can't hold another note")
	}
	if len(n.Marks) > 0 || n.Text != "" {
		return errors.New("footnote: only text has marks and text")
	}
	if k, ok := n.Attrs["kind"]; ok && k != nil && k != "footnote" && k != "endnote" {
		return errors.New("footnote: attr kind must be footnote or endnote")
	}
	if len(n.Content) == 0 {
		return errors.New("footnote: needs at least one paragraph")
	}
	v.inNote = true
	defer func() { v.inNote = false }()
	for _, p := range n.Content {
		if p == nil || p.Type != "paragraph" {
			return errors.New("footnote: may only hold paragraphs")
		}
		if err := v.block(p, "footnote", depth+2); err != nil {
			return err
		}
	}
	return nil
}

func (v *pmValidator) inline(n *pmNode, parent string) error {
	if n == nil {
		return fmt.Errorf("%s: empty node", parent)
	}
	if err := v.count(); err != nil {
		return err
	}
	if !pmInlineTypes[n.Type] {
		return fmt.Errorf("%s: unknown or misplaced node %q", parent, n.Type)
	}
	if len(n.Content) > 0 {
		return fmt.Errorf("%s: has no content", n.Type)
	}
	if n.Type == "text" {
		if n.Text == "" {
			return errors.New("text: may not be empty")
		}
		if !utf8.ValidString(n.Text) {
			return errors.New("text: not valid UTF-8")
		}
	} else if n.Text != "" {
		return fmt.Errorf("%s: only text nodes have text", n.Type)
	}
	if len(n.Marks) > 0 && n.Type != "text" {
		return fmt.Errorf("%s: only text has marks", n.Type)
	}
	seen := map[string]bool{}
	for _, m := range n.Marks {
		if !pmMarkTypes[m.Type] && !pmRepeatMarks[m.Type] {
			return fmt.Errorf("unknown mark %q", m.Type)
		}
		key := m.Type
		if pmRepeatMarks[m.Type] {
			key += ":" + fmt.Sprint(m.Attrs["id"])
		}
		if seen[key] {
			return fmt.Errorf("mark %q twice on one text", m.Type)
		}
		seen[key] = true
		if err := checkMarkAttrs(m); err != nil {
			return err
		}
	}
	return nil
}

func optNum(attrs map[string]any, key string, lo, hi float64) error {
	v, ok := attrs[key]
	if !ok || v == nil {
		return nil
	}
	f, ok := pmNum(v)
	if !ok || f < lo || f > hi {
		return fmt.Errorf("attr %s: expected a number from %g to %g", key, lo, hi)
	}
	return nil
}

func optString(attrs map[string]any, key string, max int) (string, error) {
	v, ok := attrs[key]
	if !ok || v == nil {
		return "", nil
	}
	s, ok := v.(string)
	if !ok || utf8.RuneCountInString(s) > max {
		return "", fmt.Errorf("attr %s: expected text up to %d characters", key, max)
	}
	return s, nil
}

func checkBlockAttrs(n *pmNode) error {
	a := n.Attrs
	if a == nil {
		a = map[string]any{}
	}
	wrap := func(err error) error {
		if err != nil {
			return fmt.Errorf("%s: %w", n.Type, err)
		}
		return nil
	}
	switch n.Type {
	case "paragraph", "heading":
		if n.Type == "heading" {
			lvl, ok := pmInt(a["level"])
			if !ok || lvl < 1 || lvl > 6 {
				return errors.New("heading: level must be 1 to 6")
			}
		}
		if _, err := optString(a, "style", 120); err != nil {
			return wrap(err)
		}
		if al, err := optString(a, "align", 10); err != nil || (al != "" && !pmAligns[al]) {
			return wrap(fmt.Errorf("attr align: unknown value"))
		}
		for _, k := range []string{"indentFirst", "indentLeft", "indentRight"} {
			if err := optNum(a, k, -2000, 2000); err != nil {
				return wrap(err)
			}
		}
		for _, k := range []string{"spaceBefore", "spaceAfter", "lineExact"} {
			if err := optNum(a, k, 0, 2000); err != nil {
				return wrap(err)
			}
		}
		if err := optNum(a, "lineHeight", 0.1, 20); err != nil {
			return wrap(err)
		}
		if kn, ok := a["keepNext"]; ok && kn != nil {
			if _, isBool := kn.(bool); !isBool {
				return wrap(errors.New("attr keepNext: expected true or false"))
			}
		}
		if err := optNum(a, "boxWidth", 0, 20); err != nil {
			return wrap(err)
		}
		for _, k := range []string{"boxColor", "shade"} {
			if c, err := optString(a, k, 7); err != nil || (c != "" && !pmHexColor(c)) {
				return wrap(fmt.Errorf("attr %s: expected #rrggbb", k))
			}
		}
		if v, ok := a["dropCap"]; ok && v != nil {
			if n, ok := pmInt(v); !ok || n < 2 || n > 4 {
				return wrap(errors.New("attr dropCap: expected 2, 3 or 4"))
			}
		}
		if _, err := optString(a, "bookmark", 200); err != nil {
			return wrap(err)
		}
	case "scene_break":
		if _, err := optString(a, "glyph", 60); err != nil {
			return wrap(err)
		}
		if img, err := optString(a, "image", 512); err != nil || (img != "" && !safeUploadSrc(img)) {
			return wrap(errors.New("attr image: must be an /uploads/ path"))
		}
	case "image":
		src, err := optString(a, "src", 512)
		if err != nil || !safeUploadSrc(src) {
			return wrap(errors.New("attr src: must be an /uploads/ path"))
		}
		if _, err := optString(a, "alt", 2000); err != nil {
			return wrap(err)
		}
		if err := optNum(a, "width", 0, 5000); err != nil {
			return wrap(err)
		}
		if al, err := optString(a, "align", 10); err != nil || (al != "" && al != "left" && al != "center" && al != "right") {
			return wrap(errors.New("attr align: unknown value"))
		}
		if wr, err := optString(a, "wrap", 10); err != nil || (wr != "" && wr != "inline" && wr != "left" && wr != "right") {
			return wrap(errors.New("attr wrap: unknown value"))
		}
	case "ordered_list":
		if err := optNum(a, "start", 0, 1e9); err != nil {
			return wrap(err)
		}
	case "dialogue": // a line of Script Mode (stage 4)
		if c, ok := a["character"]; ok && c != nil {
			if id, ok := pmInt(c); !ok || id < 1 {
				return wrap(errors.New("attr character: expected a character id or null"))
			}
		}
		for _, k := range []string{"speaker", "manner"} {
			if _, err := optString(a, k, 200); err != nil {
				return wrap(err)
			}
		}
	case "toc": // a generated table of contents (stage 3)
		if lv, ok := a["levels"]; ok && lv != nil {
			if n, ok := pmInt(lv); !ok || n < 1 || n > 3 {
				return wrap(errors.New("attr levels: expected 1 to 3"))
			}
		}
		if _, err := optString(a, "title", 200); err != nil {
			return wrap(err)
		}
	}
	return nil
}

func checkMarkAttrs(m pmMark) error {
	a := m.Attrs
	if a == nil {
		a = map[string]any{}
	}
	wrap := func(err error) error {
		if err != nil {
			return fmt.Errorf("mark %s: %w", m.Type, err)
		}
		return nil
	}
	switch m.Type {
	case "font":
		f, err := optString(a, "family", 100)
		if err != nil || strings.TrimSpace(f) == "" {
			return wrap(errors.New("attr family: expected a font name"))
		}
	case "size":
		if v, ok := pmNum(a["pt"]); !ok || v < 1 || v > 1000 {
			return wrap(errors.New("attr pt: expected a size from 1 to 1000"))
		}
	case "color", "highlight":
		if c, ok := a["color"]; ok && c != nil {
			s, isStr := c.(string)
			if !isStr || !pmColorRe.MatchString(s) {
				return wrap(errors.New("attr color: expected #rrggbb"))
			}
		}
	case "link":
		h, ok := a["href"].(string)
		if !ok || !safeLinkHref(h) {
			return wrap(errors.New("attr href: not an allowed link"))
		}
	case "char_style":
		id, ok := a["id"].(string)
		if !ok || id == "" || utf8.RuneCountInString(id) > 120 || strings.ContainsAny(id, "\x00\r\n") {
			return wrap(errors.New("attr id: expected a style id"))
		}
	case "comment": // a comment on the text (stage 5, comments.go)
		if id, ok := pmInt(a["id"]); !ok || id < 1 {
			return wrap(errors.New("attr id: expected a comment id"))
		}
	case "story_link":
		t, ok := a["type"].(string)
		if !ok || !pmStoryTypeRe.MatchString(t) {
			return wrap(errors.New("attr type: expected a story item type"))
		}
		if id, ok := pmInt(a["id"]); !ok || id < 1 {
			return wrap(errors.New("attr id: expected a positive id"))
		}
	}
	return nil
}

// --- reading ---

// pmWalk calls fn for n and every node below it, depth first.
func pmWalk(n *pmNode, fn func(*pmNode)) {
	if n == nil {
		return
	}
	fn(n)
	for _, c := range n.Content {
		pmWalk(c, fn)
	}
}

// pmTextblockText is a paragraph's or heading's text, with hard breaks
// and tabs as given. A note in it adds nothing (its text is its own).
func pmTextblockText(n *pmNode, hardBreak, tab string) string {
	var b strings.Builder
	for _, c := range n.Content {
		switch c.Type {
		case "text":
			b.WriteString(c.Text)
		case "hard_break":
			b.WriteString(hardBreak)
		case "tab":
			b.WriteString(tab)
		}
	}
	return b.String()
}

// countWords: split on whitespace, count the pieces that hold a letter or
// digit.
func countWords(s string) int {
	n := 0
	inWord, hasAlnum := false, false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if inWord && hasAlnum {
				n++
			}
			inWord, hasAlnum = false, false
			continue
		}
		inWord = true
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			hasAlnum = true
		}
	}
	if inWord && hasAlnum {
		n++
	}
	return n
}

func docWords(doc *pmNode) int {
	n := 0
	pmWalk(doc, func(x *pmNode) {
		// A dialogue's words are its spoken line, not the speaker or manner.
		// A note's number separates words like a space; its paragraphs are
		// counted on their own (pmWalk goes into them).
		if x.Type == "paragraph" || x.Type == "heading" || x.Type == "dialogue" {
			var b strings.Builder
			for _, c := range x.Content {
				if c.Type == "text" {
					b.WriteString(c.Text)
				} else {
					b.WriteByte(' ')
				}
			}
			n += countWords(b.String())
		}
	})
	return n
}

func docText(doc *pmNode) string {
	var lines []string
	var walk func(n *pmNode)
	walk = func(n *pmNode) {
		switch n.Type {
		case "paragraph", "heading":
			lines = append(lines, pmTextblockText(n, "\n", "\t"))
		case "dialogue":
			lines = append(lines, dialogueLead(n)+pmTextblockText(n, "\n", "\t"))
		case "scene_break":
			g, _ := n.Attrs["glyph"].(string)
			if g == "" {
				g = "* * *"
			}
			lines = append(lines, g)
		default:
			for _, c := range n.Content {
				walk(c)
			}
		}
	}
	if doc != nil {
		walk(doc)
	}
	return strings.Join(lines, "\n")
}

func docStoryLinks(doc *pmNode) []pmStoryLink {
	var out []pmStoryLink
	seen := map[pmStoryLink]bool{}
	pmWalk(doc, func(n *pmNode) {
		for _, m := range n.Marks {
			if m.Type != "story_link" {
				continue
			}
			t, _ := m.Attrs["type"].(string)
			id, ok := pmInt(m.Attrs["id"])
			if t == "" || !ok || id < 1 {
				continue
			}
			l := pmStoryLink{Type: t, ID: id}
			if !seen[l] {
				seen[l] = true
				out = append(out, l)
			}
		}
	})
	return out
}

// pmHexColor reports whether s is "#rrggbb".
func pmHexColor(s string) bool {
	if len(s) != 7 || s[0] != 0x23 {
		return false
	}
	for _, c := range s[1:] {
		if !(c >= 0x30 && c <= 0x39 || c >= 0x61 && c <= 0x66 || c >= 0x41 && c <= 0x46) {
			return false
		}
	}
	return true
}
