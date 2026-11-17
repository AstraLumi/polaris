package main

// Chapter text as Markdown, for the API's tools (apimcp.go): an LLM reads
// and writes prose as Markdown, the editor keeps ProseMirror documents
// (pmdoc.go). Only what Markdown says plainly is read and written:
//
//	# … ######     headings (levels 1–6)
//	* * *, ***, ---  a scene break
//	- item, 1. item  bullet and numbered lists (one level)
//	**bold**, *italic*, _italic_, ~~struck~~
//	[[Name]], [[type:Name]], [[type#id]], [[…|text]]   a link to a story item
//	[text](https://…)  a web link
//	every other line   a paragraph of its own (novel prose: no joining of
//	                   lines into one paragraph, blank lines only separate)
//
// Reading also shows what Markdown has no word for: a dialogue as
// "**Speaker** (manner): text", a picture as "![picture]", a footnote as
// "[^note: …]", a table as "[table]". markdownCanReplace says whether a
// text holds any of it, so a "replace" doesn't drop it unasked.

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// ---- Markdown -> document ----

// markdownBlocks turns Markdown into the blocks of a chapter. Links are
// found in x (nil: links stay as plain text).
func markdownBlocks(md string, x *wikiLinkIndex) []*pmNode {
	var out []*pmNode
	var list *pmNode
	md = strings.ReplaceAll(strings.ReplaceAll(md, "\r\n", "\n"), "\r", "\n")
	for _, raw := range strings.Split(md, "\n") {
		line := strings.TrimRight(raw, " \t")
		trim := strings.TrimSpace(line)
		if trim == "" {
			list = nil
			continue
		}
		if isSceneBreakLine(trim) {
			list = nil
			out = append(out, &pmNode{Type: "scene_break", Attrs: map[string]any{"glyph": "* * *"}})
			continue
		}
		if lvl, text, ok := headingLine(trim); ok {
			list = nil
			out = append(out, &pmNode{Type: "heading", Attrs: map[string]any{"level": lvl}, Content: markdownInline(text, x)})
			continue
		}
		if kind, text, ok := listLine(trim); ok {
			if list == nil || list.Type != kind {
				list = &pmNode{Type: kind}
				out = append(out, list)
			}
			list.Content = append(list.Content, &pmNode{Type: "list_item", Content: []*pmNode{
				{Type: "paragraph", Content: markdownInline(text, x)},
			}})
			continue
		}
		list = nil
		trim = strings.TrimPrefix(strings.TrimPrefix(trim, ">"), " ")
		out = append(out, &pmNode{Type: "paragraph", Content: markdownInline(trim, x)})
	}
	return out
}

func isSceneBreakLine(s string) bool {
	c := strings.ReplaceAll(s, " ", "")
	if len(c) < 3 {
		return false
	}
	return strings.Trim(c, "*") == "" || strings.Trim(c, "-") == "" || strings.Trim(c, "_") == "" || c == "#"
}

func headingLine(s string) (int, string, bool) {
	n := 0
	for n < len(s) && s[n] == '#' {
		n++
	}
	if n == 0 || n > 6 || n >= len(s) || s[n] != ' ' {
		return 0, "", false
	}
	return n, strings.TrimSpace(s[n:]), true
}

func listLine(s string) (string, string, bool) {
	if len(s) > 2 && (s[0] == '-' || s[0] == '*' || s[0] == '+') && s[1] == ' ' {
		return "bullet_list", strings.TrimSpace(s[2:]), true
	}
	i := 0
	for i < len(s) && i < 4 && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i > 0 && i+1 < len(s) && (s[i] == '.' || s[i] == ')') && s[i+1] == ' ' {
		return "ordered_list", strings.TrimSpace(s[i+2:]), true
	}
	return "", "", false
}

// markdownInline reads one line's text and marks.
func markdownInline(s string, x *wikiLinkIndex) []*pmNode {
	p := &inlineParser{x: x}
	p.run(s, nil)
	return p.flush()
}

type inlineParser struct {
	x     *wikiLinkIndex
	out   []*pmNode
	text  strings.Builder
	marks []pmMark
}

func (p *inlineParser) emit() {
	if p.text.Len() == 0 {
		return
	}
	n := &pmNode{Type: "text", Text: p.text.String()}
	if len(p.marks) > 0 {
		n.Marks = append([]pmMark(nil), p.marks...)
	}
	p.out = append(p.out, n)
	p.text.Reset()
}

func (p *inlineParser) flush() []*pmNode {
	p.emit()
	return p.out
}

func (p *inlineParser) with(m pmMark, s string) {
	p.emit()
	saved := p.marks
	p.marks = append(append([]pmMark(nil), p.marks...), m)
	p.run(s, nil)
	p.emit()
	p.marks = saved
}

func (p *inlineParser) run(s string, _ []pmMark) {
	for i := 0; i < len(s); {
		rest := s[i:]
		switch {
		case rest[0] == '\\' && len(rest) > 1 && strings.ContainsRune(`\*_~[]()#`, rune(rest[1])):
			p.text.WriteByte(rest[1])
			i += 2
			continue
		case strings.HasPrefix(rest, "[["):
			if end := strings.Index(rest, "]]"); end > 2 {
				p.link(rest[2:end])
				i += end + 2
				continue
			}
		case rest[0] == '[':
			if close := strings.Index(rest, "]("); close > 0 {
				if end := strings.IndexByte(rest[close+2:], ')'); end >= 0 {
					href := rest[close+2 : close+2+end]
					if safeLinkHref(href) {
						p.with(pmMark{Type: "link", Attrs: map[string]any{"href": href}}, rest[1:close])
						i += close + 3 + end
						continue
					}
				}
			}
		case strings.HasPrefix(rest, "~~"):
			if end := strings.Index(rest[2:], "~~"); end > 0 {
				p.with(pmMark{Type: "strike"}, rest[2:2+end])
				i += end + 4
				continue
			}
		case strings.HasPrefix(rest, "***"):
			if end := strings.Index(rest[3:], "***"); end > 0 {
				p.emit()
				saved := p.marks
				p.marks = append(append([]pmMark(nil), p.marks...), pmMark{Type: "bold"}, pmMark{Type: "italic"})
				p.run(rest[3:3+end], nil)
				p.emit()
				p.marks = saved
				i += end + 6
				continue
			}
		case strings.HasPrefix(rest, "**") || strings.HasPrefix(rest, "__"):
			d := rest[:2]
			if end := strings.Index(rest[2:], d); end > 0 {
				p.with(pmMark{Type: "bold"}, rest[2:2+end])
				i += end + 4
				continue
			}
		case rest[0] == '*' || rest[0] == '_':
			d := rest[:1]
			// _ only at a word's edge, so snake_case stays as typed.
			edge := d == "*" || i == 0 || !isWordByte(s[i-1])
			if end := strings.Index(rest[1:], d); edge && end > 0 && rest[1] != ' ' {
				after := i + 1 + end + 1
				if d == "*" || after >= len(s) || !isWordByte(s[after]) {
					p.with(pmMark{Type: "italic"}, rest[1:1+end])
					i = after
					continue
				}
			}
		}
		r, size := utf8.DecodeRuneInString(rest)
		p.text.WriteRune(r)
		i += size
	}
}

func isWordByte(b byte) bool {
	return b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= 0x80
}

// link writes [[target|text]] as text with a story_link mark when the
// target is found, as its plain text otherwise.
func (p *inlineParser) link(inner string) {
	target, shown, hasText := strings.Cut(inner, "|")
	target = strings.TrimSpace(target)
	var ref *wikiRef
	if p.x != nil {
		ref = p.x.resolve(target)
	}
	if !hasText || strings.TrimSpace(shown) == "" {
		shown = target
		if ref != nil {
			shown = ref.Name
		} else if _, name := splitWikiTarget(target); name != "" {
			shown = name
		}
	}
	if ref == nil {
		p.text.WriteString(shown)
		return
	}
	p.emit()
	saved := p.marks
	p.marks = append(append([]pmMark(nil), p.marks...), pmMark{Type: "story_link", Attrs: map[string]any{"type": ref.Type, "id": ref.ID}})
	p.text.WriteString(shown)
	p.emit()
	p.marks = saved
}

// ---- document -> Markdown ----

// docMarkdown writes a chapter's document as Markdown (see the top).
func docMarkdown(doc *pmNode, x *wikiLinkIndex) string {
	var b strings.Builder
	for _, n := range doc.Content {
		blockMarkdown(&b, n, x, "")
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func blockMarkdown(b *strings.Builder, n *pmNode, x *wikiLinkIndex, indent string) {
	switch n.Type {
	case "paragraph":
		b.WriteString(indent + inlineMarkdown(n.Content, x) + "\n\n")
	case "heading":
		lvl, _ := pmInt(n.Attrs["level"])
		if lvl < 1 || lvl > 6 {
			lvl = 1
		}
		b.WriteString(strings.Repeat("#", int(lvl)) + " " + inlineMarkdown(n.Content, x) + "\n\n")
	case "dialogue":
		who, _ := n.Attrs["speaker"].(string)
		manner, _ := n.Attrs["manner"].(string)
		head := "**" + strings.TrimSpace(who) + "**"
		if strings.TrimSpace(manner) != "" {
			head += " (" + strings.TrimSpace(manner) + ")"
		}
		b.WriteString(head + ": " + inlineMarkdown(n.Content, x) + "\n\n")
	case "scene_break":
		b.WriteString("* * *\n\n")
	case "page_break":
		b.WriteString("[page break]\n\n")
	case "toc":
		b.WriteString("[table of contents]\n\n")
	case "image":
		b.WriteString("![picture]\n\n")
	case "table":
		b.WriteString("[table]\n\n")
	case "bullet_list", "ordered_list":
		for i, item := range n.Content {
			mark := "- "
			if n.Type == "ordered_list" {
				mark = strconv.Itoa(i+1) + ". "
			}
			for j, c := range item.Content {
				if j == 0 && c.Type == "paragraph" {
					b.WriteString(indent + mark + inlineMarkdown(c.Content, x) + "\n")
				} else {
					var sub strings.Builder
					blockMarkdown(&sub, c, x, indent+"   ")
					b.WriteString(strings.TrimRight(sub.String(), "\n") + "\n")
				}
			}
		}
		b.WriteString("\n")
	default:
		if t := pmTextblockText(n, " ", " "); t != "" {
			b.WriteString(t + "\n\n")
		}
	}
}

func inlineMarkdown(nodes []*pmNode, x *wikiLinkIndex) string {
	var b strings.Builder
	for _, n := range nodes {
		switch n.Type {
		case "text":
			b.WriteString(markedText(n, x))
		case "hard_break":
			b.WriteString(" / ")
		case "tab":
			b.WriteString("\t")
		case "footnote":
			var inner []string
			for _, c := range n.Content {
				inner = append(inner, inlineMarkdown(c.Content, x))
			}
			b.WriteString("[^note: " + strings.Join(inner, " ") + "]")
		default:
			b.WriteString(pmTextblockText(n, " ", " "))
		}
	}
	return b.String()
}

func markedText(n *pmNode, x *wikiLinkIndex) string {
	t := escapeMarkdown(n.Text)
	var bold, italic, strike bool
	var link *wikiRef
	href := ""
	for _, m := range n.Marks {
		switch m.Type {
		case "bold":
			bold = true
		case "italic":
			italic = true
		case "strike":
			strike = true
		case "link":
			href, _ = m.Attrs["href"].(string)
		case "story_link":
			typ, _ := m.Attrs["type"].(string)
			id, _ := pmInt(m.Attrs["id"])
			link = &wikiRef{Type: typ, ID: id}
		}
	}
	if link != nil {
		target := wikiIDLink(link.Type, link.ID)
		if x != nil {
			if r := x.resolve(target); r != nil && r.Name != "" {
				target = link.Type + ":" + r.Name
			}
		}
		t = "[[" + target + "|" + t + "]]"
	} else if href != "" {
		t = "[" + t + "](" + href + ")"
	}
	if strike {
		t = "~~" + t + "~~"
	}
	if italic {
		t = "*" + t + "*"
	}
	if bold {
		t = "**" + t + "**"
	}
	return t
}

var mdEscaper = strings.NewReplacer(`\`, `\\`, `*`, `\*`, `[[`, `\[\[`, `~~`, `\~\~`)

func escapeMarkdown(s string) string { return mdEscaper.Replace(s) }

// markdownCanReplace: the document holds only what Markdown writes back
// (markdownBlocks), with no formatting Markdown loses. When it doesn't, a
// replace would drop something and needs to be asked for plainly.
func markdownCanReplace(doc *pmNode) (bool, string) {
	plainMarks := map[string]bool{"bold": true, "italic": true, "strike": true, "link": true, "story_link": true}
	why := ""
	pmWalk(doc, func(n *pmNode) {
		if why != "" {
			return
		}
		switch n.Type {
		case "doc", "paragraph", "heading", "text", "scene_break", "bullet_list", "ordered_list", "list_item":
		default:
			why = "it holds a " + strings.ReplaceAll(n.Type, "_", " ")
			return
		}
		for k, v := range n.Attrs {
			if v == nil || (n.Type == "heading" && k == "level") || (n.Type == "scene_break" && k == "glyph") {
				continue
			}
			if k == "style" && v == "" {
				continue
			}
			why = "a paragraph has its own formatting (" + k + ")"
			return
		}
		for _, m := range n.Marks {
			if !plainMarks[m.Type] {
				why = "some text has " + strings.ReplaceAll(m.Type, "_", " ") + " formatting"
				return
			}
		}
	})
	return why == "", why
}
