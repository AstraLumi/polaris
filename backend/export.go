package main

// Book exports (docs/EDITOR.md, "Stage 3 and 4 additions"): a book (or one
// of its chapters) as a Word file (exportdocx.go), an EPUB (exportepub.go)
// or an Online Post copy in HTML, Markdown or plain text (exportpost.go).
//
// API:
//
//	GET /api/books/{volume}/export.docx | .epub | .html | .md | .txt
//	    ?chapter=ID for one chapter. A download named after the book
//	    (Content-Disposition with an ASCII name and an RFC 5987 UTF-8 one);
//	    404 for an unknown book or a chapter not in it.
//	loadExportBook(db, vol, chapterID, uploadsDir) (*exportBook, error)
//	    Everything an export needs, read once: the settings with every
//	    stage 3 default filled in (meta, layout, chapter_options), the
//	    styles (with heading4–6 made up when the book has none), and the
//	    chapters' docs in reading order. errNoBook / errNoChapter.
//	exportDocx(b) / exportEPUB(b) / exportHTML(b) / exportMarkdown(b) /
//	exportText(b) ([]byte, error)
//	    Pure apart from reading pictures from b's uploads folder (only
//	    /uploads/… paths, never outside it; a missing or unreadable picture
//	    is left out and counted in b.Missing).
//
// Formatting is resolved the way the editor shows it: a paragraph's own
// attrs > its style > that style's based_on chain > normal; a text's own
// marks > its character style (and that style's based_on chain) > its
// paragraph's style (resolvedPara, resolvedText).

import (
	"bytes"
	"crypto/sha1"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxExportPicture  = 32 << 20  // one picture read into an export
	maxExportPictures = 400 << 20 // every picture of one export
)

// --- the book as exports see it ---

type exportMeta struct {
	Title, Subtitle, Author, Language, Publisher, ISBN, Rights, Description string
}

// exportBand is a header or footer band (EDITOR.md "A band is …").
type exportBand struct {
	Left, Center, Right string
	Font                string
	Size                float64
	Italic, SmallCaps   bool
	Uppercase           bool
}

func (b exportBand) empty() bool {
	return strings.TrimSpace(b.Left+b.Center+b.Right) == ""
}

type exportLayout struct {
	DifferentFirst   bool
	DifferentOddEven bool
	Header, Footer   map[string]exportBand // "odd", "even", "first"
	ChapterStart     string                // "next" or "odd"
	NumFormat        string                // arabic roman upper_roman alpha upper_alpha
	NumStart         int
	TocDots          bool
}

type exportChapter struct {
	ID            int64
	Title         string
	Doc           *pmNode
	FrontMatter   bool
	NumberRestart *int
	NoHeader      bool
	Number        int // among the chapters that aren't front matter, from 1 (0 for front matter)
}

// exStyle is a book style as read from the settings.
type exStyle struct {
	ID, Name, Kind string
	BasedOn        string
	Builtin        bool
	Para           map[string]any
	Text           map[string]any
}

type exportBook struct {
	Volume      int64
	VolumeTitle string
	PageType    string
	Page        bookPage
	Styles      []*exStyle
	styleByID   map[string]*exStyle
	Meta        exportMeta
	Layout      exportLayout
	Chapters    []exportChapter
	Single      bool // one chapter (?chapter=ID)
	uploadsDir  string

	// Fonts from the library the export may embed (the EPUB; filled by
	// the handler from the fonts library, fontlib.go).
	Fonts []exportFont

	Missing  int // pictures that could not be read
	pics     map[string]*exportPicture
	picBytes int64

	// The chapters' comments by id (comments.go): the .docx export writes
	// them as Word comments; the other exports take their marks off.
	Comments map[int64]*exportComment
}

// withoutComments takes the comment marks off every chapter's doc (every
// export but .docx: comments never show in a book).
func (b *exportBook) withoutComments() {
	for _, c := range b.Chapters {
		stripCommentMarks(c.Doc)
	}
	b.Comments = nil
}

// exportPicture is one picture read from the uploads.
type exportPicture struct {
	Data        []byte
	ContentType string // image/png, image/jpeg, image/gif, image/webp
	Ext         string // ".png"…
	W, H        int    // pixels
}

// BookTitle is meta.title, else the volume's title, else "Book".
func (b *exportBook) BookTitle() string {
	if t := strings.TrimSpace(b.Meta.Title); t != "" {
		return t
	}
	if t := strings.TrimSpace(b.VolumeTitle); t != "" {
		return t
	}
	return "Book"
}

func (b *exportBook) online() bool { return b.PageType == "online" }

// --- settings ---

func decodeLoose(raw json.RawMessage, v any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	return dec.Decode(v)
}

func jsonString(raw json.RawMessage) (string, bool) {
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return "", false
	}
	return s, true
}

// parseExportSettings reads a book's settings JSON with EDITOR.md's
// defaults for anything missing.
func parseExportSettings(raw json.RawMessage, b *exportBook) {
	var m map[string]json.RawMessage
	json.Unmarshal(raw, &m)

	b.PageType, _ = jsonString(m["page_type"])
	if _, ok := pageTypeDefs[b.PageType]; !ok {
		b.PageType = "a5"
	}
	b.Page = defaultPage(b.PageType)
	if p, ok := m["page"]; ok {
		pg := b.Page
		if json.Unmarshal(p, &pg) == nil && pageProblem(pg) == "" {
			b.Page = pg
		}
	}
	if b.PageType != "custom" && b.PageType != "online" {
		d := pageTypeDefs[b.PageType]
		if pageProblem(bookPage{WidthMM: d.widthMM, HeightMM: d.heightMM, MarginTopMM: b.Page.MarginTopMM,
			MarginBottomMM: b.Page.MarginBottomMM, MarginInnerMM: b.Page.MarginInnerMM, MarginOuterMM: b.Page.MarginOuterMM,
			GutterMM: b.Page.GutterMM}) == "" {
			b.Page.WidthMM, b.Page.HeightMM = d.widthMM, d.heightMM
		}
	}
	if b.PageType == "online" {
		b.Page = defaultPage("a5")
	}

	// Styles: as stored, every built-in put back, heading4–6 made up.
	var styles []struct {
		ID      string         `json:"id"`
		Name    string         `json:"name"`
		Kind    string         `json:"kind"`
		BasedOn *string        `json:"based_on"`
		Builtin bool           `json:"builtin"`
		Para    map[string]any `json:"para"`
		Text    map[string]any `json:"text"`
	}
	if s, ok := m["styles"]; ok {
		if err := decodeLoose(s, &styles); err != nil {
			styles = nil
		}
	}
	b.styleByID = map[string]*exStyle{}
	add := func(s *exStyle) {
		if s.ID == "" || b.styleByID[s.ID] != nil {
			return
		}
		if s.Kind != "character" {
			s.Kind = "paragraph"
		}
		if s.Name == "" {
			s.Name = s.ID
		}
		b.Styles = append(b.Styles, s)
		b.styleByID[s.ID] = s
	}
	for _, s := range styles {
		es := &exStyle{ID: s.ID, Name: s.Name, Kind: s.Kind, Builtin: s.Builtin, Para: s.Para, Text: s.Text}
		if s.BasedOn != nil {
			es.BasedOn = *s.BasedOn
		}
		add(es)
	}
	fallback := b.PageType
	for _, d := range defaultStyles(fallback) {
		if b.styleByID[d.ID] == nil {
			raw, _ := json.Marshal(d)
			var es struct {
				Para map[string]any `json:"para"`
				Text map[string]any `json:"text"`
			}
			decodeLoose(raw, &es)
			base := ""
			if d.BasedOn != nil {
				base = *d.BasedOn
			}
			add(&exStyle{ID: d.ID, Name: d.Name, Kind: d.Kind, BasedOn: base, Builtin: true, Para: es.Para, Text: es.Text})
		}
	}
	body := pageTypeDefs[b.PageType].bodyPt
	for lvl := 4; lvl <= 6; lvl++ {
		id := fmt.Sprintf("heading%d", lvl)
		if b.styleByID[id] == nil {
			add(&exStyle{ID: id, Name: fmt.Sprintf("Heading %d", lvl), Kind: "paragraph", BasedOn: "normal",
				Para: map[string]any{"align": "left", "indentFirst": 0.0, "spaceBefore": roundPt(body), "spaceAfter": roundPt(body * 0.5), "keepNext": true},
				Text: map[string]any{"bold": true}})
		}
	}

	// meta, layout and chapter_options, with their defaults (booklayout.go).
	ex := bookLayoutOf(raw)
	bm := ex.Meta
	b.Meta = exportMeta{Title: strings.TrimSpace(bm.Title), Subtitle: strings.TrimSpace(bm.Subtitle), Author: strings.TrimSpace(bm.Author),
		Language: strings.TrimSpace(bm.Language), Publisher: strings.TrimSpace(bm.Publisher), ISBN: strings.TrimSpace(bm.ISBN),
		Rights: strings.TrimSpace(bm.Rights), Description: strings.TrimSpace(bm.Description)}
	if !validLangTag(b.Meta.Language) {
		b.Meta.Language = "en"
	}
	bl := ex.Layout
	L := exportLayout{DifferentFirst: bl.DifferentFirst, DifferentOddEven: bl.DifferentOddEven, ChapterStart: bl.ChapterStart,
		NumFormat: bl.Numbering.Format, NumStart: bl.Numbering.Start, TocDots: bl.TocDots}
	if L.ChapterStart != "odd" {
		L.ChapterStart = "next"
	}
	if pageNumFormats[L.NumFormat] == "" {
		L.NumFormat = "arabic"
	}
	if L.NumStart < 0 {
		L.NumStart = 1
	}
	band := func(x bookBand) exportBand {
		out := exportBand{Left: x.Left, Center: x.Center, Right: x.Right, Italic: x.Italic, SmallCaps: x.Smallcaps, Uppercase: x.Uppercase}
		if x.Font != nil {
			out.Font = strings.TrimSpace(*x.Font)
		}
		if x.Size != nil && *x.Size >= 1 && *x.Size <= 200 {
			out.Size = *x.Size
		}
		return out
	}
	L.Header = map[string]exportBand{"odd": band(bl.Header.Odd), "even": band(bl.Header.Even), "first": band(bl.Header.First)}
	L.Footer = map[string]exportBand{"odd": band(bl.Footer.Odd), "even": band(bl.Footer.Even), "first": band(bl.Footer.First)}
	b.Layout = L
	for i := range b.Chapters {
		c := &b.Chapters[i]
		if o, ok := ex.ChapterOptions[c.ID]; ok {
			c.FrontMatter, c.NoHeader = o.FrontMatter, o.NoHeader
			if o.NumberRestart != nil && *o.NumberRestart >= 0 {
				n := *o.NumberRestart
				c.NumberRestart = &n
			}
		}
	}
	n := 0
	for i := range b.Chapters {
		if !b.Chapters[i].FrontMatter {
			n++
			b.Chapters[i].Number = n
		}
	}
}

// pageNumFormats: numbering.format to Word's w:fmt.
var pageNumFormats = map[string]string{"arabic": "decimal", "roman": "lowerRoman", "upper_roman": "upperRoman",
	"alpha": "lowerLetter", "upper_alpha": "upperLetter"}

func validLangTag(s string) bool {
	if s == "" || len(s) > 35 {
		return false
	}
	for i, part := range strings.Split(s, "-") {
		if part == "" || len(part) > 8 || i == 0 && len(part) < 2 {
			return false
		}
		for _, r := range part {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
				return false
			}
		}
	}
	return true
}

// --- loading ---

func loadExportBook(db editorDB, vol, chapterID int64, uploadsDir string) (*exportBook, error) {
	title, err := bookTitle(db, vol)
	if err != nil {
		return nil, err
	}
	settings, _, err := loadBookSettings(db, vol)
	if err != nil {
		return nil, err
	}
	b := &exportBook{Volume: vol, VolumeTitle: title, uploadsDir: uploadsDir, Single: chapterID != 0}
	rows, err := db.Query(`
		SELECT c.id, c.title, COALESCE(t.doc, '')
		FROM chapters c LEFT JOIN chapter_texts t ON t.chapter_id = c.id
		WHERE c.volume_id IS ?
		ORDER BY c.position, c.id`, volumeNull(vol))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	found := chapterID == 0
	for rows.Next() {
		var c exportChapter
		var doc string
		if err := rows.Scan(&c.ID, &c.Title, &doc); err != nil {
			return nil, err
		}
		if chapterID != 0 && c.ID != chapterID {
			continue
		}
		found = true
		c.Doc = emptyPMDoc()
		if doc != "" {
			var d pmNode
			if err := json.Unmarshal([]byte(doc), &d); err != nil || d.Type != "doc" {
				log.Printf("export: chapter %d has an unreadable text: %v", c.ID, err)
			} else {
				c.Doc = &d
			}
		}
		b.Chapters = append(b.Chapters, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if !found {
		return nil, errNoChapter
	}
	ids := map[int64]bool{}
	for _, c := range b.Chapters {
		ids[c.ID] = true
	}
	if b.Comments, err = loadExportComments(db, ids); err != nil {
		return nil, err
	}
	parseExportSettings(settings, b)
	return b, nil
}

// --- styles ---

func (b *exportBook) style(id string) *exStyle { return b.styleByID[id] }

// paraStyleOf is the paragraph style a block uses: its style attr when the
// book has it, else heading<level> for a heading, else normal.
func (b *exportBook) paraStyleOf(n *pmNode) string {
	if id, _ := n.Attrs["style"].(string); id != "" {
		if s := b.styleByID[id]; s != nil && s.Kind == "paragraph" {
			return id
		}
	}
	if n.Type == "heading" {
		if id := fmt.Sprintf("heading%d", headingLevel(n)); b.styleByID[id] != nil {
			return id
		}
	}
	return "normal"
}

func headingLevel(n *pmNode) int {
	lv, ok := pmInt(n.Attrs["level"])
	if !ok || lv < 1 {
		return 1
	}
	if lv > 6 {
		return 6
	}
	return int(lv)
}

// chain is a style and its bases, base first. Paragraph styles end at
// normal; character styles follow character styles only.
func (b *exportBook) chain(id string) []*exStyle {
	var rev []*exStyle
	seen := map[string]bool{}
	s := b.styleByID[id]
	kind := ""
	if s != nil {
		kind = s.Kind
	}
	for s != nil && !seen[s.ID] && len(rev) < 40 && s.Kind == kind {
		seen[s.ID] = true
		rev = append(rev, s)
		next := s.BasedOn
		if next == "" && kind == "paragraph" && s.ID != "normal" {
			next = "normal"
		}
		s = b.styleByID[next]
	}
	if kind == "paragraph" && !seen["normal"] {
		if n := b.styleByID["normal"]; n != nil {
			rev = append(rev, n)
		}
	}
	out := make([]*exStyle, len(rev))
	for i, x := range rev {
		out[len(rev)-1-i] = x
	}
	return out
}

// exPara is a paragraph's formatting with every value known.
type exPara struct {
	Align                                string
	IndentFirst, IndentLeft, IndentRight float64
	SpaceBefore, SpaceAfter              float64
	LineHeight                           float64
	LineExact                            float64 // pt; 0 = not exact
	KeepNext                             bool
}

// exText is a text's formatting with every value known.
type exText struct {
	Font                            string
	Size                            float64
	Color, Highlight                string // "#rrggbb" or ""
	Bold, Italic, Underline, Strike bool
	SmallCaps, Sup, Sub             bool
}

// applyPara puts a style's or a block's paragraph values over p (null
// values leave p as it is).
func applyPara(p *exPara, m map[string]any) {
	if m == nil {
		return
	}
	if a, ok := m["align"].(string); ok && pmAligns[a] {
		p.Align = a
	}
	num := func(k string, dst *float64) bool {
		if v, ok := pmNum(m[k]); ok {
			*dst = v
			return true
		}
		return false
	}
	num("indentFirst", &p.IndentFirst)
	num("indentLeft", &p.IndentLeft)
	num("indentRight", &p.IndentRight)
	num("spaceBefore", &p.SpaceBefore)
	num("spaceAfter", &p.SpaceAfter)
	var exact float64
	hasExact := num("lineExact", &exact) && exact > 0
	var lh float64
	if num("lineHeight", &lh) && lh > 0 {
		p.LineHeight = lh
		if !hasExact {
			p.LineExact = 0
		}
	}
	if hasExact {
		p.LineExact = exact
	}
	if v, ok := m["keepNext"].(bool); ok {
		p.KeepNext = v
	}
}

// applyText puts a style's text values over t ("nil" values set none).
func applyText(t *exText, m map[string]any) {
	if m == nil {
		return
	}
	if v, ok := m["font"]; ok {
		if s, _ := v.(string); strings.TrimSpace(s) != "" {
			t.Font = strings.TrimSpace(s)
		}
	}
	if v, ok := pmNum(m["size"]); ok && v >= 1 && v <= 1000 {
		t.Size = v
	}
	for _, k := range []struct {
		key string
		dst *string
	}{{"color", &t.Color}, {"highlight", &t.Highlight}} {
		if v, ok := m[k.key]; ok {
			s, _ := v.(string)
			if pmColorRe.MatchString(s) {
				*k.dst = strings.ToLower(s)
			} else {
				*k.dst = ""
			}
		}
	}
	for _, k := range []struct {
		key string
		dst *bool
	}{{"bold", &t.Bold}, {"italic", &t.Italic}, {"underline", &t.Underline}, {"strike", &t.Strike}, {"smallcaps", &t.SmallCaps}} {
		if v, ok := m[k.key].(bool); ok {
			*k.dst = v
		}
	}
	if v, ok := m["superscript"].(bool); ok {
		t.Sup = v
		if v {
			t.Sub = false
		}
	}
	if v, ok := m["subscript"].(bool); ok {
		t.Sub = v
		if v {
			t.Sup = false
		}
	}
}

func (b *exportBook) bodySize() float64 {
	if v, ok := pmNum(b.styleByID["normal"].Text["size"]); ok && v > 0 {
		return v
	}
	return pageTypeDefs[b.PageType].bodyPt
}

// styleFormat resolves a paragraph style (base first).
func (b *exportBook) styleFormat(id string) (exPara, exText) {
	p := exPara{Align: "left", LineHeight: 1}
	t := exText{Size: pageTypeDefs[b.PageType].bodyPt}
	for _, s := range b.chain(id) {
		applyPara(&p, s.Para)
		applyText(&t, s.Text)
	}
	return p, t
}

// charStyleText is a character style's own settings along its chain, as
// a map ready for applyText.
func (b *exportBook) charStyleText(id string) map[string]any {
	s := b.styleByID[id]
	if s == nil || s.Kind != "character" {
		return nil
	}
	out := map[string]any{}
	for _, c := range b.chain(id) {
		for k, v := range c.Text {
			out[k] = v
		}
	}
	return out
}

// resolvedPara is what a block shows: its style, then its own attrs.
func (b *exportBook) resolvedPara(n *pmNode) exPara {
	p, _ := b.styleFormat(b.paraStyleOf(n))
	applyPara(&p, n.Attrs)
	return p
}

// markText applies a text node's marks (its character style first).
func (b *exportBook) markText(t *exText, marks []pmMark) {
	for _, m := range marks {
		if m.Type == "char_style" {
			id, _ := m.Attrs["id"].(string)
			applyText(t, b.charStyleText(id))
		}
	}
	for _, m := range marks {
		switch m.Type {
		case "bold":
			t.Bold = true
		case "italic":
			t.Italic = true
		case "underline":
			t.Underline = true
		case "strike":
			t.Strike = true
		case "smallcaps":
			t.SmallCaps = true
		case "superscript":
			t.Sup, t.Sub = true, false
		case "subscript":
			t.Sub, t.Sup = true, false
		case "font":
			if f, _ := m.Attrs["family"].(string); strings.TrimSpace(f) != "" {
				t.Font = strings.TrimSpace(f)
			}
		case "size":
			if v, ok := pmNum(m.Attrs["pt"]); ok && v >= 1 && v <= 1000 {
				t.Size = v
			}
		case "color":
			if c, _ := m.Attrs["color"].(string); pmColorRe.MatchString(c) {
				t.Color = strings.ToLower(c)
			}
		case "highlight":
			if c, _ := m.Attrs["highlight"].(string); pmColorRe.MatchString(c) {
				t.Highlight = strings.ToLower(c)
			} else if c, _ := m.Attrs["color"].(string); pmColorRe.MatchString(c) {
				t.Highlight = strings.ToLower(c)
			}
		}
	}
}

// resolvedText is what a text node shows inside a block of that style.
func (b *exportBook) resolvedText(styleID string, marks []pmMark) exText {
	_, t := b.styleFormat(styleID)
	b.markText(&t, marks)
	return t
}

func markOf(marks []pmMark, typ string) *pmMark {
	for i := range marks {
		if marks[i].Type == typ {
			return &marks[i]
		}
	}
	return nil
}

// linkHrefOf gives a text's link: a safe web/mail/phone link, or "#name".
func linkHrefOf(marks []pmMark) string {
	m := markOf(marks, "link")
	if m == nil {
		return ""
	}
	h, _ := m.Attrs["href"].(string)
	h = strings.TrimSpace(h)
	if !safeLinkHref(h) {
		return ""
	}
	return h
}

// isWebLink: a link that leaves the book (http, https, mailto, tel).
func isWebLink(h string) bool {
	l := strings.ToLower(h)
	return strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://") || strings.HasPrefix(l, "mailto:") || strings.HasPrefix(l, "tel:")
}

// --- shared block helpers ---

// sceneGlyphOf is a scene break's glyph ("* * *" when it has none).
func sceneGlyphOf(n *pmNode) string {
	g, _ := n.Attrs["glyph"].(string)
	if strings.TrimSpace(g) == "" {
		return "* * *"
	}
	return g
}

func attrString(n *pmNode, k string) string {
	s, _ := n.Attrs[k].(string)
	return s
}

func attrBool(n *pmNode, k string) bool {
	v, _ := n.Attrs[k].(bool)
	return v
}

// dialogueParts gives a dialogue's speaker and the text between it and the
// line: "Speaker, manner: “" … "”".
func dialogueParts(n *pmNode) (speaker, lead string) {
	speaker = strings.Join(strings.Fields(attrString(n, "speaker")), " ")
	manner := strings.Join(strings.Fields(attrString(n, "manner")), " ")
	switch {
	case speaker != "" && manner != "":
		lead = ", " + manner + ": “"
	case speaker != "":
		lead = ": “"
	case manner != "":
		lead = manner + ": “"
	default:
		lead = "“"
	}
	return speaker, lead
}

// tocEntry is a heading a table of contents lists.
type tocEntry struct {
	chapter int
	level   int
	text    string
	node    *pmNode
}

// headingsForTOC lists the book's headings up to a level, in order.
func (b *exportBook) headingsForTOC(levels int) []tocEntry {
	var out []tocEntry
	for ci, c := range b.Chapters {
		pmWalk(c.Doc, func(n *pmNode) {
			if n.Type != "heading" {
				return
			}
			lv := headingLevel(n)
			if lv > levels {
				return
			}
			t := strings.Join(strings.Fields(pmTextblockText(n, " ", " ")), " ")
			if t == "" {
				return
			}
			out = append(out, tocEntry{chapter: ci, level: lv, text: t, node: n})
		})
	}
	return out
}

func tocLevels(n *pmNode) int {
	lv, ok := pmInt(n.Attrs["levels"])
	if !ok || lv < 1 {
		return 1
	}
	if lv > 3 {
		return 3
	}
	return int(lv)
}

func tocTitle(n *pmNode) string {
	if v, ok := n.Attrs["title"]; ok {
		s, _ := v.(string)
		return s
	}
	return "Contents"
}

// trimPageBreaks drops page breaks at a chapter's start and end (every
// chapter starts on a new page anyway).
func trimPageBreaks(blocks []*pmNode) []*pmNode {
	for len(blocks) > 0 && blocks[0] != nil && blocks[0].Type == "page_break" {
		blocks = blocks[1:]
	}
	for len(blocks) > 0 && blocks[len(blocks)-1] != nil && blocks[len(blocks)-1].Type == "page_break" {
		blocks = blocks[:len(blocks)-1]
	}
	return blocks
}

// --- pictures ---

// picture reads an /uploads/… picture from the story's uploads folder
// (nil when it is missing, too large, or not a picture a reader shows).
func (b *exportBook) picture(src string) *exportPicture {
	if b.pics == nil {
		b.pics = map[string]*exportPicture{}
	}
	if p, ok := b.pics[src]; ok {
		return p
	}
	p := b.readPicture(src)
	b.pics[src] = p
	if p == nil {
		b.Missing++
	}
	return p
}

func (b *exportBook) readPicture(src string) *exportPicture {
	if !safeUploadSrc(src) || b.uploadsDir == "" {
		return nil
	}
	rel, ok := safeRel(strings.TrimPrefix(src, "/uploads/"))
	if !ok {
		return nil
	}
	full := filepath.Join(b.uploadsDir, filepath.FromSlash(rel))
	root, err := filepath.Abs(b.uploadsDir)
	if err != nil {
		return nil
	}
	if abs, err := filepath.Abs(full); err != nil || !strings.HasPrefix(abs, root+string(filepath.Separator)) {
		return nil
	}
	f, err := os.Open(full)
	if err != nil {
		return nil
	}
	defer f.Close()
	if st, err := f.Stat(); err != nil || !st.Mode().IsRegular() || st.Size() > maxExportPicture || b.picBytes+st.Size() > maxExportPictures {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(f, maxExportPicture+1))
	if err != nil || len(data) > maxExportPicture {
		return nil
	}
	ct, ext, ok := sniffPicture(data)
	if !ok {
		return nil
	}
	w, h := pictureSize(data, ct)
	if w <= 0 || h <= 0 {
		return nil
	}
	b.picBytes += int64(len(data))
	return &exportPicture{Data: data, ContentType: ct, Ext: ext, W: w, H: h}
}

// pictureSize reads a picture's size in pixels (WebP by hand: the standard
// library has no WebP decoder).
func pictureSize(data []byte, ct string) (int, int) {
	if ct == "image/webp" {
		return webpSize(data)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

func webpSize(d []byte) (int, int) {
	if len(d) < 30 || string(d[0:4]) != "RIFF" || string(d[8:12]) != "WEBP" {
		return 0, 0
	}
	switch string(d[12:16]) {
	case "VP8X":
		w := int(d[24]) | int(d[25])<<8 | int(d[26])<<16
		h := int(d[27]) | int(d[28])<<8 | int(d[29])<<16
		return w + 1, h + 1
	case "VP8L":
		if d[20] != 0x2f {
			return 0, 0
		}
		bits := binary.LittleEndian.Uint32(d[21:25])
		return int(bits&0x3fff) + 1, int(bits>>14&0x3fff) + 1
	case "VP8 ":
		if d[23] != 0x9d || d[24] != 0x01 || d[25] != 0x2a {
			return 0, 0
		}
		return int(binary.LittleEndian.Uint16(d[26:28]) & 0x3fff), int(binary.LittleEndian.Uint16(d[28:30]) & 0x3fff)
	}
	return 0, 0
}

// --- page geometry ---

const ptPerMM = 72 / 25.4

// textWidthPt / textHeightPt: the page's text block.
func (b *exportBook) textWidthPt() float64 {
	p := b.Page
	return math.Max(20, p.WidthMM-p.MarginInnerMM-p.MarginOuterMM-p.GutterMM) * ptPerMM
}

func (b *exportBook) textHeightPt() float64 {
	p := b.Page
	return math.Max(20, p.HeightMM-p.MarginTopMM-p.MarginBottomMM) * ptPerMM
}

// --- text helpers ---

// cleanXMLText drops what XML 1.0 can't hold (control characters, bad
// UTF-8).
func cleanXMLText(s string) string {
	ok := true
	for _, r := range s {
		if r == utf8.RuneError || r < 0x20 && r != '\t' && r != '\n' && r != '\r' || r == 0xfffe || r == 0xffff {
			ok = false
			break
		}
	}
	if ok {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if r == utf8.RuneError || r < 0x20 && r != '\t' && r != '\n' && r != '\r' || r == 0xfffe || r == 0xffff {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

var exportEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")

// xe escapes text for XML content or an attribute.
func xe(s string) string { return exportEscaper.Replace(cleanXMLText(s)) }

func fmtNum(v float64) string {
	return strconv.FormatFloat(math.Round(v*1000)/1000, 'f', -1, 64)
}

// --- downloads ---

var exportMIME = map[string]string{
	"docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	"epub": "application/epub+zip",
	"html": "text/html; charset=utf-8",
	"md":   "text/markdown; charset=utf-8",
	"txt":  "text/plain; charset=utf-8",
}

// exportFileName is the download's name: the book's title (and the
// chapter's, for one chapter), without characters file systems refuse.
func (b *exportBook) exportFileName(ext string) string {
	name := b.BookTitle()
	if b.Single && len(b.Chapters) == 1 && strings.TrimSpace(b.Chapters[0].Title) != "" {
		name += " - " + strings.TrimSpace(b.Chapters[0].Title)
	}
	var s strings.Builder
	for _, r := range name {
		switch {
		case r < 0x20 || r == 0x7f || strings.ContainsRune(`\/:*?"<>|`, r):
			s.WriteByte(' ')
		case unicode.IsSpace(r):
			s.WriteByte(' ')
		default:
			s.WriteRune(r)
		}
	}
	out := strings.Trim(strings.Join(strings.Fields(s.String()), " "), ". ")
	if r := []rune(out); len(r) > 120 {
		out = strings.TrimSpace(string(r[:120]))
	}
	if out == "" {
		out = "Book"
	}
	return out + "." + ext
}

// asciiFileName folds a name to ASCII for the plain filename parameter.
func asciiFileName(name string) string {
	var s strings.Builder
	for _, r := range name {
		switch {
		case r < 0x80 && r >= 0x20 && r != '"' && r != '\\' && r != ';' && r != '%':
			s.WriteRune(r)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if f := foldASCII(r); f != "" {
				s.WriteString(f)
			} else {
				s.WriteByte('_')
			}
		case unicode.IsSpace(r):
			s.WriteByte(' ')
		default:
			s.WriteByte('_')
		}
	}
	return s.String()
}

// foldASCII: common accented Latin letters to their base letter.
func foldASCII(r rune) string {
	const from = "ÀÁÂÃÄÅàáâãäåÇçÈÉÊËèéêëÌÍÎÏìíîïÑñÒÓÔÕÖØòóôõöøÙÚÛÜùúûüÝýÿ"
	const to = "AAAAAAaaaaaaCcEEEEeeeeIIIIiiiiNnOOOOOOooooooUUUUuuuuYyy"
	fr := []rune(from)
	for i, c := range fr {
		if c == r {
			return string(to[i])
		}
	}
	switch r {
	case 'ß':
		return "ss"
	case 'Æ':
		return "AE"
	case 'æ':
		return "ae"
	case 'Œ':
		return "OE"
	case 'œ':
		return "oe"
	}
	return ""
}

// rfc5987 percent-encodes a UTF-8 value (attr-char kept as is).
func rfc5987(s string) string {
	var b strings.Builder
	for _, c := range []byte(s) {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte("!#$&+-.^_`|~", c) >= 0 {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func contentDisposition(name string) string {
	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, asciiFileName(name), rfc5987(name))
}

var exportFormats = map[string]func(*exportBook) ([]byte, error){
	"docx": exportDocx,
	"epub": exportEPUB,
	"html": exportHTML,
	"md":   exportMarkdown,
	"txt":  exportText,
}

// exportBookHandler answers GET /api/books/{volume}/export.<format>.
func exportBookHandler(db *sql.DB, uploadsDir, format string) http.HandlerFunc {
	write := exportFormats[format]
	return func(w http.ResponseWriter, r *http.Request) {
		vol, ok := bookVolume(r)
		if !ok {
			http.Error(w, "book not found", http.StatusNotFound)
			return
		}
		var chapterID int64
		if c := r.URL.Query().Get("chapter"); c != "" {
			id, err := strconv.ParseInt(c, 10, 64)
			if err != nil || id <= 0 {
				http.Error(w, "chapter not found", http.StatusNotFound)
				return
			}
			chapterID = id
		}
		b, err := loadExportBook(db, vol, chapterID, uploadsDir)
		switch {
		case errors.Is(err, errNoBook):
			http.Error(w, "book not found", http.StatusNotFound)
			return
		case errors.Is(err, errNoChapter):
			http.Error(w, "chapter not found", http.StatusNotFound)
			return
		case err != nil:
			log.Printf("export book %d: %v", vol, err)
			http.Error(w, "failed to export the book", http.StatusInternalServerError)
			return
		}
		if format != "docx" {
			b.withoutComments()
		}
		if format == "epub" {
			if lib := fontLibraryFrom(r.Context()); lib != nil {
				b.Fonts = lib.embeddable(exportFamilies(b))
			}
		}
		data, err := write(b)
		if err != nil {
			log.Printf("export book %d as %s: %v", vol, format, err)
			http.Error(w, "failed to export the book", http.StatusInternalServerError)
			return
		}
		h := w.Header()
		h.Set("Content-Type", exportMIME[format])
		h.Set("Content-Disposition", contentDisposition(b.exportFileName(format)))
		h.Set("Content-Length", strconv.Itoa(len(data)))
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Content-Security-Policy", "sandbox")
		h.Set("Cache-Control", "no-store")
		w.Write(data)
	}
}

// stableUUID makes a UUID (version 5 style) from a text, so a book keeps
// its EPUB identifier from one export to the next.
func stableUUID(s string) string {
	sum := sha1.Sum([]byte("polaris-book:" + s))
	sum[6] = sum[6]&0x0f | 0x50
	sum[8] = sum[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", sum[0:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
}

// sortedKeys of a string set.
func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
