package main

// Online Post exports (export.go; docs/EDITOR.md, "Exports"): a book or one
// chapter as HTML clean enough for AO3, Markdown, or plain text.
//
// HTML: p, h1–h3 (deeper headings are h3), em, strong, u, s, sup, sub, br,
// a (web links only), ul/ol/li, blockquote around paragraphs in the Quote
// style (or a style based on it), hr for a picture used as a scene break;
// no classes and no inline styles but text-align: center on centred
// paragraphs and headings. Text formatting is what the reader sees: a
// paragraph style's italics count, a heading style's own bold doesn't.
// Fonts, sizes, colours, highlights and small caps are left out;
// pictures too (an online post can't hold the story's uploads); page
// breaks mean nothing online. Scene breaks are a centred paragraph with
// the glyph; an empty paragraph is &nbsp;; the toc is a list of the
// headings; dialogue reads "Speaker, manner: “line”".
//
// Markdown: the same in CommonMark (with ~~strike~~, and <u>, <sup>,
// <sub> written as HTML); a scene break whose glyph is a thematic break
// (* * *, ---) stays one, any other glyph is its own escaped line.
// Text: one line per paragraph (hard breaks as new lines), a blank line
// between blocks, lists with • or numbers, no formatting. Tables in all
// three: exporttables.go.

import (
	"fmt"
	"regexp"
	"strings"
)

// postSeg is a run of text with the formatting an online post keeps.
type postSeg struct {
	text                        string
	brk, tab                    bool
	bold, italic, under, strike bool
	sup, sub                    bool
	link                        string   // a web link
	note                        *docNote // a footnote's or endnote's number (exportnotes.go)
	noteKey                     string   // its Markdown label
}

// postSegs reads a block's inline content. Formatting counts against
// `base` (a heading's style, or nothing for a paragraph).
func (b *exportBook) postSegs(content []*pmNode, styleID string, base exText) []postSeg {
	var out []postSeg
	for _, c := range content {
		if c == nil {
			continue
		}
		switch c.Type {
		case "hard_break":
			out = append(out, postSeg{brk: true})
		case "tab":
			out = append(out, postSeg{tab: true})
		case "footnote":
			x := b.noteOf(c)
			out = append(out, postSeg{note: &x, noteKey: b.postNoteKey(x)})
		case "text":
			t := b.resolvedText(styleID, c.Marks)
			s := postSeg{text: c.Text, bold: t.Bold && !base.Bold, italic: t.Italic && !base.Italic,
				under: t.Underline && !base.Underline, strike: t.Strike && !base.Strike,
				sup: t.Sup && !base.Sup, sub: t.Sub && !base.Sub}
			if h := linkHrefOf(c.Marks); h != "" && isWebLink(h) {
				s.link = h
			}
			out = append(out, s)
		}
	}
	return out
}

// postBase: what a block's own style already shows (headings only).
func (b *exportBook) postBase(n *pmNode, styleID string) exText {
	if n.Type == "heading" {
		_, t := b.styleFormat(styleID)
		return t
	}
	return exText{}
}

func (b *exportBook) isQuoteStyle(id string) bool {
	for _, s := range b.chain(id) {
		if s.ID == "quote" {
			return true
		}
	}
	return false
}

// dialogueSegs: "Speaker, manner: “" + the line + "”".
func (b *exportBook) blockSegs(n *pmNode) []postSeg {
	styleID := b.paraStyleOf(n)
	segs := b.postSegs(n.Content, styleID, b.postBase(n, styleID))
	if n.Type == "dialogue" {
		speaker, lead := dialogueParts(n)
		segs = append([]postSeg{{text: speaker + lead}}, segs...)
		segs = append(segs, postSeg{text: "”"})
	}
	return segs
}

// --- HTML ---

var htmlEsc = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")

func he(s string) string { return htmlEsc.Replace(cleanXMLText(s)) }

// noteMark holds a note's place while runs are joined (cleanXMLText keeps
// it out of any text), so a note after superscript text stays apart.
const noteMark = "\x00"

func htmlInline(segs []postSeg) string {
	var s strings.Builder
	var notes []string
	for _, g := range segs {
		switch {
		case g.brk:
			s.WriteString("<br>")
			continue
		case g.tab:
			s.WriteString(" ")
			continue
		case g.note != nil:
			s.WriteString(noteMark) // the note's number, put in after the runs are joined
			notes = append(notes, "<sup>"+he(g.note.Label)+"</sup>")
			continue
		}
		var open, close []string
		wrap := func(o, c string) {
			open = append(open, o)
			close = append([]string{c}, close...)
		}
		if g.link != "" {
			wrap(`<a href="`+he(g.link)+`">`, "</a>")
		}
		if g.bold {
			wrap("<strong>", "</strong>")
		}
		if g.italic {
			wrap("<em>", "</em>")
		}
		if g.under {
			wrap("<u>", "</u>")
		}
		if g.strike {
			wrap("<s>", "</s>")
		}
		if g.sup {
			wrap("<sup>", "</sup>")
		}
		if g.sub {
			wrap("<sub>", "</sub>")
		}
		s.WriteString(strings.Join(open, "") + he(g.text) + strings.Join(close, ""))
	}
	// Adjacent runs with the same formatting: </em><em> is noise.
	out := s.String()
	for _, t := range []string{"strong", "em", "u", "s", "sup", "sub"} {
		out = strings.ReplaceAll(out, "</"+t+"><"+t+">", "")
	}
	for _, n := range notes {
		out = strings.Replace(out, noteMark, n, 1)
	}
	return out
}

type htmlPost struct {
	b       *exportBook
	s       strings.Builder
	inQuote bool
}

func exportHTML(b *exportBook) ([]byte, error) {
	h := &htmlPost{b: b}
	title := b.BookTitle()
	if b.Single && len(b.Chapters) == 1 && strings.TrimSpace(b.Chapters[0].Title) != "" {
		title = b.Chapters[0].Title
	}
	h.s.WriteString("<!DOCTYPE html>\n<html lang=\"" + he(b.Meta.Language) + "\">\n<head>\n<meta charset=\"utf-8\">\n<title>" + he(title) + "</title>\n</head>\n<body>\n")
	for _, c := range b.Chapters {
		for _, n := range trimPageBreaks(c.Doc.Content) {
			h.block(n, 0)
		}
		h.endQuote()
		h.notes(c.Doc) // exportnotes.go
	}
	h.s.WriteString("</body>\n</html>\n")
	return []byte(h.s.String()), nil
}

func (h *htmlPost) endQuote() {
	if h.inQuote {
		h.s.WriteString("</blockquote>\n")
		h.inQuote = false
	}
}

func centred(p exPara) string {
	if p.Align == "center" {
		return ` style="text-align: center;"`
	}
	return ""
}

func (h *htmlPost) block(n *pmNode, depth int) {
	if n == nil {
		return
	}
	b := h.b
	isQuote := (n.Type == "paragraph" || n.Type == "dialogue") && b.isQuoteStyle(b.paraStyleOf(n))
	if depth == 0 && !isQuote {
		h.endQuote()
	}
	switch n.Type {
	case "paragraph", "dialogue":
		if depth == 0 && isQuote && !h.inQuote {
			h.s.WriteString("<blockquote>\n")
			h.inQuote = true
		}
		inner := htmlInline(b.blockSegs(n))
		if strings.TrimSpace(strings.ReplaceAll(inner, "<br>", "")) == "" {
			inner = "&nbsp;"
		}
		h.s.WriteString("<p" + centred(b.resolvedPara(n)) + ">" + inner + "</p>\n")
	case "heading":
		lv := min(3, headingLevel(n))
		inner := htmlInline(b.blockSegs(n))
		if strings.TrimSpace(inner) == "" {
			return
		}
		fmt.Fprintf(&h.s, "<h%d%s>%s</h%d>\n", lv, centred(b.resolvedPara(n)), inner, lv)
	case "scene_break":
		if attrString(n, "image") != "" && b.picture(attrString(n, "image")) != nil {
			h.s.WriteString("<hr>\n")
			return
		}
		h.s.WriteString(`<p style="text-align: center;">` + he(sceneGlyphOf(n)) + "</p>\n")
	case "bullet_list", "ordered_list":
		tag := "ul"
		attrs := ""
		if n.Type == "ordered_list" {
			tag = "ol"
			if st, ok := pmInt(n.Attrs["start"]); ok && st != 1 && st >= 0 {
				attrs = fmt.Sprintf(` start="%d"`, st)
			}
		}
		h.s.WriteString("<" + tag + attrs + ">\n")
		for _, item := range n.Content {
			if item == nil {
				continue
			}
			paras := 0
			for _, c := range item.Content {
				if c != nil && (c.Type == "paragraph" || c.Type == "dialogue") {
					paras++
				}
			}
			h.s.WriteString("<li>")
			for i, c := range item.Content {
				if c == nil {
					continue
				}
				if paras == 1 && i == 0 && (c.Type == "paragraph" || c.Type == "dialogue") {
					h.s.WriteString(htmlInline(b.blockSegs(c)))
					continue
				}
				if i == 0 {
					h.s.WriteString("\n")
				}
				h.block(c, depth+1)
			}
			h.s.WriteString("</li>\n")
		}
		h.s.WriteString("</" + tag + ">\n")
	case "toc":
		if t := strings.TrimSpace(tocTitle(n)); t != "" {
			h.s.WriteString("<p><strong>" + he(t) + "</strong></p>\n")
		}
		entries := b.headingsForTOC(tocLevels(n))
		if len(entries) == 0 {
			return
		}
		h.s.WriteString("<ul>\n")
		for _, e := range entries {
			h.s.WriteString("<li>" + strings.Repeat("&nbsp;&nbsp;&nbsp;", e.level-1) + he(e.text) + "</li>\n")
		}
		h.s.WriteString("</ul>\n")
	case "image", "page_break":
		// Not in an online post.
	case "table": // exporttables.go
		h.endQuote()
		h.table(n)
	default:
		for _, c := range n.Content {
			h.block(c, depth)
		}
	}
}

// --- Markdown ---

var mdLineStart = regexp.MustCompile(`^(\s*)([#>+=-]|\d+[.)])`)
var mdThematic = regexp.MustCompile(`^ *([*_-])( *([*_-]))( *([*_-]))[ *_-]*$`)

func mdEscape(s string) string {
	var b strings.Builder
	for i, r := range s {
		switch r {
		case '\\', '`', '*', '_', '[', ']', '<', '>', '~', '|':
			b.WriteByte('\\')
		case '&':
			if i+1 < len(s) && (s[i+1] == '#' || s[i+1] >= 'a' && s[i+1] <= 'z' || s[i+1] >= 'A' && s[i+1] <= 'Z') {
				b.WriteByte('\\')
			}
		}
		b.WriteRune(r)
	}
	return b.String()
}

type mdMark struct{ open, close string }

// mdInline writes segments with emphasis opened and closed around whole
// runs (whitespace kept outside the delimiters, as CommonMark wants).
func mdInline(segs []postSeg) string {
	var out strings.Builder
	var stack []string
	closers := map[string]string{}
	pending := ""
	closeTo := func(n int) {
		for len(stack) > n {
			top := stack[len(stack)-1]
			out.WriteString(closers[top])
			stack = stack[:len(stack)-1]
		}
	}
	for _, g := range segs {
		if g.brk {
			closeTo(0)
			out.WriteString(pending)
			pending = ""
			out.WriteString("\\\n")
			continue
		}
		if g.note != nil { // a footnote reference (exportnotes.go)
			closeTo(0)
			out.WriteString(pending)
			pending = ""
			out.WriteString("[^" + g.noteKey + "]")
			continue
		}
		text := g.text
		if g.tab {
			text = " "
		}
		core := strings.TrimLeft(text, " \t")
		lead := text[:len(text)-len(core)]
		trimmed := strings.TrimRight(core, " \t")
		trail := core[len(trimmed):]
		core = trimmed
		if core == "" {
			pending += strings.Repeat(" ", len(lead)+len(trail))
			continue
		}
		var want []string
		var marks []mdMark
		add := func(on bool, key, open, close string) {
			if on {
				want = append(want, key)
				marks = append(marks, mdMark{open, close})
			}
		}
		add(g.link != "", "link:"+g.link, "[", "]("+strings.NewReplacer(" ", "%20", "(", "%28", ")", "%29").Replace(g.link)+")")
		add(g.bold, "strong", "**", "**")
		add(g.italic, "em", "*", "*")
		add(g.strike, "strike", "~~", "~~")
		add(g.under, "u", "<u>", "</u>")
		add(g.sup, "sup", "<sup>", "</sup>")
		add(g.sub, "sub", "<sub>", "</sub>")
		keep := 0
		for keep < len(stack) && keep < len(want) && stack[keep] == want[keep] {
			keep++
		}
		if keep < len(stack) {
			closeTo(keep)
		}
		out.WriteString(pending + lead)
		pending = ""
		for i := keep; i < len(want); i++ {
			closers[want[i]] = marks[i].close
			out.WriteString(marks[i].open)
			stack = append(stack, want[i])
		}
		out.WriteString(mdEscape(core))
		pending = trail
	}
	closeTo(0)
	out.WriteString(pending)
	// Escape what would start a block at a line's start.
	lines := strings.Split(out.String(), "\n")
	for i, l := range lines {
		l = strings.TrimLeft(l, " \t")
		if m := mdLineStart.FindStringSubmatchIndex(l); m != nil {
			l = l[:m[4]] + "\\" + l[m[4]:]
		}
		lines[i] = l
	}
	return strings.Join(lines, "\n")
}

type mdPost struct {
	b      *exportBook
	blocks []string
}

func exportMarkdown(b *exportBook) ([]byte, error) {
	m := &mdPost{b: b}
	for ci, c := range b.Chapters {
		for _, n := range trimPageBreaks(c.Doc.Content) {
			if s := m.block(n); s != "" {
				m.blocks = append(m.blocks, s)
			}
		}
		if s := m.notes(c.Doc, ci); s != "" { // exportnotes.go
			m.blocks = append(m.blocks, s)
		}
	}
	return []byte(strings.Join(m.blocks, "\n\n") + "\n"), nil
}

func indentLines(s, first, rest string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		if i == 0 {
			lines[i] = first + lines[i]
		} else if lines[i] != "" {
			lines[i] = rest + lines[i]
		}
	}
	return strings.Join(lines, "\n")
}

func (m *mdPost) block(n *pmNode) string {
	if n == nil {
		return ""
	}
	b := m.b
	switch n.Type {
	case "paragraph", "dialogue":
		s := mdInline(b.blockSegs(n))
		if strings.TrimSpace(strings.ReplaceAll(s, "\\\n", "")) == "" {
			s = "&nbsp;"
		}
		if b.isQuoteStyle(b.paraStyleOf(n)) {
			s = indentLines(s, "> ", "> ")
		}
		return s
	case "heading":
		s := mdInline(b.blockSegs(n))
		s = strings.ReplaceAll(s, "\\\n", " ")
		if strings.TrimSpace(s) == "" {
			return ""
		}
		return strings.Repeat("#", headingLevel(n)) + " " + s
	case "scene_break":
		if attrString(n, "image") != "" && b.picture(attrString(n, "image")) != nil {
			return "---"
		}
		g := sceneGlyphOf(n)
		if mdThematic.MatchString(g) {
			return strings.TrimSpace(g)
		}
		return mdEscape(g)
	case "bullet_list", "ordered_list":
		var items []string
		num := 1
		if st, ok := pmInt(n.Attrs["start"]); ok && st >= 0 && st < 1e9 {
			num = int(st)
		}
		for _, item := range n.Content {
			if item == nil {
				continue
			}
			marker := "- "
			if n.Type == "ordered_list" {
				marker = fmt.Sprintf("%d. ", num)
				num++
			}
			pad := strings.Repeat(" ", len(marker))
			// A nested list follows its line directly; other blocks come
			// after a blank line.
			var body strings.Builder
			for _, c := range item.Content {
				s := m.block(c)
				if s == "" {
					continue
				}
				if body.Len() > 0 {
					if c.Type == "bullet_list" || c.Type == "ordered_list" {
						body.WriteString("\n")
					} else {
						body.WriteString("\n\n")
					}
				}
				body.WriteString(s)
			}
			items = append(items, indentLines(body.String(), marker, pad))
		}
		return strings.Join(items, "\n")
	case "toc":
		var lines []string
		if t := strings.TrimSpace(tocTitle(n)); t != "" {
			lines = append(lines, "**"+mdEscape(t)+"**", "")
		}
		for _, e := range b.headingsForTOC(tocLevels(n)) {
			lines = append(lines, strings.Repeat("  ", e.level-1)+"- "+mdEscape(e.text))
		}
		return strings.TrimSpace(strings.Join(lines, "\n"))
	case "image", "page_break":
		return ""
	case "table": // exporttables.go
		return m.table(n)
	default:
		var parts []string
		for _, c := range n.Content {
			if s := m.block(c); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "\n\n")
	}
}

// --- plain text ---

func textOf(segs []postSeg) string {
	var s strings.Builder
	for _, g := range segs {
		switch {
		case g.brk:
			s.WriteString("\n")
		case g.tab:
			s.WriteString("\t")
		case g.note != nil:
			s.WriteString("[" + g.note.Label + "]")
		default:
			s.WriteString(cleanXMLText(g.text))
		}
	}
	return s.String()
}

func exportText(b *exportBook) ([]byte, error) {
	var blocks []string
	var walk func(n *pmNode, indent string) string
	walk = func(n *pmNode, indent string) string {
		if n == nil {
			return ""
		}
		switch n.Type {
		case "paragraph", "dialogue", "heading":
			return indentLines(textOf(b.blockSegs(n)), indent, indent)
		case "scene_break":
			return indent + sceneGlyphOf(n)
		case "bullet_list", "ordered_list":
			var items []string
			num := 1
			if st, ok := pmInt(n.Attrs["start"]); ok && st >= 0 && st < 1e9 {
				num = int(st)
			}
			for _, item := range n.Content {
				if item == nil {
					continue
				}
				marker := "• "
				if n.Type == "ordered_list" {
					marker = fmt.Sprintf("%d. ", num)
					num++
				}
				pad := indent + strings.Repeat(" ", len([]rune(marker)))
				var parts []string
				for i, c := range item.Content {
					s := walk(c, pad)
					if i == 0 && strings.HasPrefix(s, pad) {
						s = indent + marker + s[len(pad):]
					}
					if s != "" {
						parts = append(parts, s)
					}
				}
				items = append(items, strings.Join(parts, "\n"))
			}
			return strings.Join(items, "\n")
		case "toc":
			var lines []string
			if t := strings.TrimSpace(tocTitle(n)); t != "" {
				lines = append(lines, indent+t)
			}
			for _, e := range b.headingsForTOC(tocLevels(n)) {
				lines = append(lines, indent+strings.Repeat("  ", e.level-1)+e.text)
			}
			return strings.Join(lines, "\n")
		case "image", "page_break":
			return ""
		case "table": // exporttables.go
			return textTable(n, indent, walk)
		default:
			var parts []string
			for _, c := range n.Content {
				if s := walk(c, indent); s != "" {
					parts = append(parts, s)
				}
			}
			return strings.Join(parts, "\n\n")
		}
	}
	for _, c := range b.Chapters {
		for _, n := range trimPageBreaks(c.Doc.Content) {
			if n == nil || n.Type == "image" || n.Type == "page_break" {
				continue
			}
			blocks = append(blocks, walk(n, ""))
		}
		if s := b.textNotes(c.Doc); s != "" { // exportnotes.go
			blocks = append(blocks, s)
		}
	}
	return []byte(strings.Join(blocks, "\n\n") + "\n"), nil
}
