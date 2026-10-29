package main

// A book as an EPUB 3 (export.go; docs/EDITOR.md, "Exports").
//
// Package: mimetype (first, stored), META-INF/container.xml,
// OEBPS/content.opf (metadata from meta: title and subtitle, author as
// creator, language, publisher, rights, description, the ISBN as the
// unique identifier, else a urn:uuid made from the book; dcterms:modified),
// OEBPS/nav.xhtml (the chapters, each with its headings below Heading 1
// nested), OEBPS/toc.ncx (for EPUB 2 readers), OEBPS/style.css (from the
// book's styles), one OEBPS/chNNN.xhtml per chapter, and OEBPS/images/. The
// first chapter's picture is the cover when that chapter starts with a
// fullPage picture. Fonts: the library's files in b.Fonts (only the
// families the book uses whose license allows e-books and whose fsType
// allows embedding; fontlib.go) go into OEBPS/fonts/ with an @font-face
// each at the top of the CSS, obfuscated the IDPF way (the first 1040
// bytes XORed with the SHA-1 of the unique identifier, listed in
// META-INF/encryption.xml), as font licenses that allow e-books ask.
//
// XHTML: paragraphs p, headings h1–h6, each with its style's class
// (s-<id>; normal is plain p) and its own attrs inline (lengths in em of
// its text, so a reader's font size scales everything); text formatting
// that differs from the paragraph's style as em, strong, u, s, sup, sub
// and spans; links (web ones as they are, "#name" to the chapter holding
// that bookmark); story links as plain text; scene breaks as a centred
// glyph (or the ornament picture); page breaks as a break-after div;
// pictures as figures (fullPage: a page of its own); lists ul/ol; the toc
// as a linked list of the headings; dialogue as "Speaker, manner: “line”"
// with the speaker in small caps; tables: exporttables.go.

import (
	"crypto/sha1"
	"fmt"
	"math"
	"strings"
	"unicode"
)

type epubOut struct {
	b        *exportBook
	files    []string // chapter file names
	classes  map[string]string
	images   map[*exportPicture]string // picture → href
	imgIDs   map[*exportPicture]string
	imgOrder []*exportPicture
	ids      []map[string]bool  // per chapter: ids used
	anchors  map[string]string  // bookmark → "chNNN.xhtml#id"
	headIDs  map[*pmNode]string // heading → id
	headFile map[*pmNode]string // heading → file
	bmIDs    map[*pmNode]string // a block's bookmark → its id
	cover    *exportPicture
	body     float64               // normal's size, pt
	fonts    []string              // b.Fonts' hrefs, in order
	noteIDs  map[*pmNode][2]string // a note → its reference's id and its own (exportnotes.go)
}

func exportEPUB(b *exportBook) ([]byte, error) {
	e := &epubOut{b: b, classes: map[string]string{}, images: map[*exportPicture]string{}, imgIDs: map[*exportPicture]string{},
		anchors: map[string]string{}, headIDs: map[*pmNode]string{}, headFile: map[*pmNode]string{}, bmIDs: map[*pmNode]string{}, body: b.bodySize()}
	used := map[string]bool{"s-normal": true}
	for _, s := range b.Styles {
		if s.ID == "normal" {
			e.classes[s.ID] = "s-normal"
			continue
		}
		c := "s-" + cssIdent(s.ID)
		cand := c
		for i := 2; used[cand]; i++ {
			cand = fmt.Sprintf("%s-%d", c, i)
		}
		used[cand] = true
		e.classes[s.ID] = cand
	}

	chapters := b.Chapters
	if len(chapters) == 0 {
		// An empty book still needs a page: its title.
		chapters = []exportChapter{{Title: b.BookTitle(), Doc: &pmNode{Type: "doc", Content: []*pmNode{
			{Type: "heading", Attrs: map[string]any{"level": 1}, Content: []*pmNode{{Type: "text", Text: b.BookTitle()}}}}}}}
	}
	// Ids: bookmarks and headings, per file.
	for i, c := range chapters {
		file := fmt.Sprintf("ch%03d.xhtml", i+1)
		e.files = append(e.files, file)
		ids := map[string]bool{"chapter": true}
		e.ids = append(e.ids, ids)
		h := 0
		pmWalk(c.Doc, func(n *pmNode) {
			if bm := strings.TrimSpace(attrString(n, "bookmark")); bm != "" && (n.Type == "paragraph" || n.Type == "heading") {
				id := uniqueID(ids, xmlID(bm))
				if _, ok := e.anchors[bm]; !ok {
					e.anchors[bm] = file + "#" + id
				}
				e.bmIDs[n] = id
			}
			if n.Type == "heading" {
				h++
				id := uniqueID(ids, fmt.Sprintf("h%d", h))
				e.headIDs[n], e.headFile[n] = id, file
			}
		})
	}
	e.noteIDsFor(chapters) // exportnotes.go
	// The cover: the first chapter starting with a full-page picture.
	if len(chapters) > 0 && len(chapters[0].Doc.Content) > 0 {
		if first := chapters[0].Doc.Content[0]; first != nil && first.Type == "image" && attrBool(first, "fullPage") {
			e.cover = b.picture(attrString(first, "src"))
		}
	}

	var pages [][]byte
	for i, c := range chapters {
		pages = append(pages, e.chapterXHTML(i, c))
	}

	ident := "urn:uuid:" + stableUUID(fmt.Sprintf("%d\x00%s\x00%s", b.Volume, b.BookTitle(), b.Meta.Author))
	if isbn := cleanISBN(b.Meta.ISBN); isbn != "" {
		ident = "urn:isbn:" + isbn
	}
	for i, f := range b.Fonts {
		e.fonts = append(e.fonts, fmt.Sprintf("fonts/f%02d.%s", i+1, f.Format))
	}

	entries := []zipEntry{
		{name: "mimetype", data: []byte("application/epub+zip"), store: true},
		{name: "META-INF/container.xml", data: []byte(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" +
			`<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles>` +
			`<rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`)},
		{name: "OEBPS/content.opf", data: e.opf(ident, chapters)},
		{name: "OEBPS/nav.xhtml", data: e.nav(chapters)},
		{name: "OEBPS/toc.ncx", data: e.ncx(ident, chapters)},
		{name: "OEBPS/style.css", data: e.css()},
	}
	for i, p := range pages {
		entries = append(entries, zipEntry{name: "OEBPS/" + e.files[i], data: p})
	}
	for _, pic := range e.imgOrder {
		entries = append(entries, zipEntry{name: "OEBPS/" + e.images[pic], data: pic.Data, store: true})
	}
	if len(b.Fonts) > 0 {
		var enc strings.Builder
		enc.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" +
			`<encryption xmlns="urn:oasis:names:tc:opendocument:xmlns:container" xmlns:enc="http://www.w3.org/2001/04/xmlenc#">` + "\n")
		for i, f := range b.Fonts {
			fmt.Fprintf(&enc, `<enc:EncryptedData><enc:EncryptionMethod Algorithm="http://www.idpf.org/2008/embedding"/>`+
				`<enc:CipherData><enc:CipherReference URI="OEBPS/%s"/></enc:CipherData></enc:EncryptedData>`+"\n", e.fonts[i])
			entries = append(entries, zipEntry{name: "OEBPS/" + e.fonts[i], data: obfuscateFont(f.Data, ident), store: f.Format == "woff" || f.Format == "woff2"})
		}
		enc.WriteString("</encryption>\n")
		// Right after the container, as readers look for it there.
		entries = append(entries[:2], append([]zipEntry{{name: "META-INF/encryption.xml", data: []byte(enc.String())}}, entries[2:]...)...)
	}
	return writeZip(entries)
}

// obfuscateFont is the IDPF font obfuscation (EPUB 3 "Font Obfuscation"):
// the first 1040 bytes XORed with the SHA-1 of the unique identifier,
// whitespace removed. Doing it again undoes it.
func obfuscateFont(data []byte, ident string) []byte {
	key := sha1.Sum([]byte(strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '\r' || r == '\n' {
			return -1
		}
		return r
	}, ident)))
	out := append([]byte(nil), data...)
	for i := 0; i < len(out) && i < 1040; i++ {
		out[i] ^= key[i%len(key)]
	}
	return out
}

// cssIdent keeps letters, digits, - and _ (ASCII), for class names.
func cssIdent(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 0x80 && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "x"
	}
	return b.String()
}

// xmlID makes an XML id (an NCName) from a bookmark name.
func xmlID(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 0x80 && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	id := b.String()
	if id == "" || !(id[0] >= 'a' && id[0] <= 'z' || id[0] >= 'A' && id[0] <= 'Z' || id[0] == '_') {
		id = "b-" + id
	}
	return id
}

func uniqueID(used map[string]bool, id string) string {
	cand := id
	for i := 2; used[cand]; i++ {
		cand = fmt.Sprintf("%s-%d", id, i)
	}
	used[cand] = true
	return cand
}

func cleanISBN(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if r >= '0' && r <= '9' || r == 'X' {
			b.WriteRune(r)
		}
	}
	if n := b.Len(); n == 10 || n == 13 {
		return b.String()
	}
	return ""
}

func (e *epubOut) image(pic *exportPicture) string {
	if h, ok := e.images[pic]; ok {
		return h
	}
	n := len(e.imgOrder) + 1
	h := fmt.Sprintf("images/pic%d%s", n, pic.Ext)
	e.images[pic] = h
	e.imgIDs[pic] = fmt.Sprintf("img%d", n)
	e.imgOrder = append(e.imgOrder, pic)
	return h
}

// --- CSS ---

func emOf(pt, size float64) string {
	if size <= 0 {
		size = 12
	}
	return fmtNum(pt/size) + "em"
}

func familyCSS(f string) string {
	f = strings.Map(func(r rune) rune {
		if r == '"' || r == '\\' || r == ';' || r == '{' || r == '}' || r == '<' || r < 0x20 {
			return -1
		}
		return r
	}, f)
	return `"` + f + `", serif`
}

// paraCSS: declarations for a paragraph's formatting, lengths in em of a
// text of size.
func paraCSS(p exPara, size float64) []string {
	out := []string{"text-align: " + p.Align, "text-indent: " + emOf(p.IndentFirst, size),
		"margin: " + emOf(p.SpaceBefore, size) + " " + emOf(p.IndentRight, size) + " " + emOf(p.SpaceAfter, size) + " " + emOf(p.IndentLeft, size)}
	if p.LineExact > 0 {
		out = append(out, "line-height: "+fmtNum(p.LineExact/size))
	} else {
		out = append(out, "line-height: "+fmtNum(p.LineHeight*1.2))
	}
	if p.KeepNext {
		out = append(out, "page-break-after: avoid", "break-after: avoid")
	}
	return out
}

func textCSS(t exText, body float64, withSize bool) []string {
	var out []string
	if t.Font != "" {
		out = append(out, "font-family: "+familyCSS(t.Font))
	}
	if withSize && t.Size > 0 {
		out = append(out, "font-size: "+fmtNum(t.Size/body)+"em")
	}
	if t.Color != "" {
		out = append(out, "color: "+t.Color)
	}
	if t.Highlight != "" {
		out = append(out, "background-color: "+t.Highlight)
	}
	out = append(out, "font-weight: "+map[bool]string{true: "bold", false: "normal"}[t.Bold],
		"font-style: "+map[bool]string{true: "italic", false: "normal"}[t.Italic],
		"font-variant: "+map[bool]string{true: "small-caps", false: "normal"}[t.SmallCaps])
	var deco []string
	if t.Underline {
		deco = append(deco, "underline")
	}
	if t.Strike {
		deco = append(deco, "line-through")
	}
	if len(deco) == 0 {
		deco = []string{"none"}
	}
	out = append(out, "text-decoration: "+strings.Join(deco, " "))
	return out
}

func (e *epubOut) css() []byte {
	b := e.b
	var s strings.Builder
	s.WriteString("@charset \"UTF-8\";\n/* Made from the book's styles in Polaris. */\n")
	for i, f := range b.Fonts {
		style := "normal"
		if f.Italic {
			style = "italic"
		}
		fmt.Fprintf(&s, "@font-face { font-family: %s; src: url(%s); font-weight: %d; font-style: %s; }\n",
			strings.TrimSuffix(familyCSS(f.Family), ", serif"), e.fonts[i], f.Weight, style)
	}
	s.WriteString("body { margin: 0; padding: 0; }\n")
	s.WriteString("p, h1, h2, h3, h4, h5, h6 { margin: 0; padding: 0; }\n")
	s.WriteString("h1, h2, h3, h4, h5, h6 { font-size: 1em; font-weight: normal; }\n")
	for _, st := range b.Styles {
		if st.Kind != "paragraph" {
			continue
		}
		p, t := b.styleFormat(st.ID)
		sel := "." + e.classes[st.ID]
		if st.ID == "normal" {
			sel = "p, .s-normal"
		}
		decl := append(paraCSS(p, t.Size), textCSS(t, e.body, st.ID != "normal")...)
		fmt.Fprintf(&s, "%s { %s; }\n", sel, strings.Join(decl, "; "))
	}
	s.WriteString(`.boxed + .boxed { border-top-width: 0 !important; margin-top: 0 !important; padding-top: 0 !important; }
.dropcap2, .dropcap3, .dropcap4 { text-indent: 0 !important; }
.dropcap2::first-letter { initial-letter: 2; }
.dropcap3::first-letter { initial-letter: 3; }
.dropcap4::first-letter { initial-letter: 4; }
@supports not (initial-letter: 2) {
.dropcap2::first-letter, .dropcap3::first-letter, .dropcap4::first-letter { float: left; line-height: 0.8; padding: 0.05em 0.08em 0 0; }
.dropcap2::first-letter { font-size: 2.3em; }
.dropcap3::first-letter { font-size: 3.4em; }
.dropcap4::first-letter { font-size: 4.5em; }
}
li > p, li > h1, li > h2, li > h3 { text-indent: 0; }
ul, ol { margin: 0.5em 0; padding-left: 2em; }
.empty { min-height: 1em; }
.tab { display: inline-block; width: 2em; }
.scene-break { text-align: center; text-indent: 0; margin: 1em 0; }
.scene-break img { max-width: 40%; }
.page-break { page-break-after: always; break-after: page; height: 0; margin: 0; }
figure.image { margin: 0.5em 0; text-indent: 0; }
figure.image img { max-width: 100%; height: auto; }
h1, h2, h3, .page-break, .scene-break { clear: both; }
.full-page { page-break-before: always; break-before: page; page-break-after: always; break-after: page; margin: 0; padding: 0; text-align: center; text-indent: 0; height: 100%; }
.full-page img { max-width: 100%; max-height: 100%; height: auto; }
.speaker { font-variant: small-caps; }
.toc { margin: 1em 0; }
.toc-title { text-align: center; text-indent: 0; font-weight: bold; margin-bottom: 1em; }
.toc ol { list-style: none; padding-left: 0; margin: 0; }
.toc ol ol { padding-left: 1.5em; }
.toc a { text-decoration: none; color: inherit; }
`)
	s.WriteString(epubTableCSS(e.body))
	s.WriteString(epubNotesCSS())
	return []byte(s.String())
}

// --- chapters ---

const xhtmlHead = `<?xml version="1.0" encoding="UTF-8"?>` + "\n" + `<!DOCTYPE html>` + "\n"

func (e *epubOut) pageHead(title string) string {
	lang := xe(e.b.Meta.Language)
	return xhtmlHead + `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="` + lang + `" xml:lang="` + lang + `">` +
		"\n<head>\n<meta charset=\"utf-8\"/>\n<title>" + xe(title) + "</title>\n" +
		`<link rel="stylesheet" type="text/css" href="style.css"/>` + "\n</head>\n"
}

type epubCtx struct {
	ci int
	s  *strings.Builder
}

func (e *epubOut) chapterXHTML(ci int, c exportChapter) []byte {
	var s strings.Builder
	title := strings.TrimSpace(c.Title)
	if title == "" {
		title = e.b.BookTitle()
	}
	s.WriteString(e.pageHead(title))
	kind := "chapter"
	if c.FrontMatter {
		kind = "frontmatter"
	}
	s.WriteString(`<body>` + "\n" + `<section epub:type="` + kind + `" id="chapter">` + "\n")
	ctx := &epubCtx{ci: ci, s: &s}
	blocks := trimPageBreaks(c.Doc.Content)
	if len(blocks) == 0 {
		s.WriteString(`<p class="empty">&#160;</p>` + "\n")
	}
	for _, n := range blocks {
		e.block(ctx, n)
	}
	e.chapterNotesXHTML(&s, c.Doc) // exportnotes.go
	s.WriteString("</section>\n</body>\n</html>\n")
	return []byte(s.String())
}

func (e *epubOut) block(ctx *epubCtx, n *pmNode) {
	if n == nil {
		return
	}
	s := ctx.s
	switch n.Type {
	case "paragraph", "heading", "dialogue":
		e.textBlock(ctx, n)
	case "scene_break":
		if src := attrString(n, "image"); src != "" {
			if pic := e.b.picture(src); pic != nil {
				fmt.Fprintf(s, `<p class="scene-break" role="separator"><img src="%s" alt="%s"/></p>`+"\n", e.image(pic), xe(attrString(n, "glyph")))
				return
			}
		}
		fmt.Fprintf(s, `<p class="scene-break" role="separator">%s</p>`+"\n", xe(sceneGlyphOf(n)))
	case "page_break":
		if !e.b.online() {
			s.WriteString(`<div class="page-break"></div>` + "\n")
		}
	case "image":
		pic := e.b.picture(attrString(n, "src"))
		if pic == nil {
			return
		}
		alt := xe(attrString(n, "alt"))
		if attrBool(n, "fullPage") {
			fmt.Fprintf(s, `<div class="full-page"><img src="%s" alt="%s"/></div>`+"\n", e.image(pic), alt)
			return
		}
		align := attrString(n, "align")
		if align != "left" && align != "right" {
			align = "center"
		}
		width := float64(pic.W) * 0.75
		if w, ok := pmNum(n.Attrs["width"]); ok && w > 0 {
			width = w
		}
		pct := math.Min(100, width/e.b.textWidthPt()*100)
		if wr := attrString(n, "wrap"); wr == "left" || wr == "right" {
			// Text flows around it: a float, 60% of the text at most.
			pct = math.Min(pct, 60)
			gap := "margin: 0.3em 0.6em 0.3em 0"
			if wr == "right" {
				gap = "margin: 0.3em 0 0.3em 0.6em"
			}
			fmt.Fprintf(s, `<figure class="image wrap-%s" style="float: %s; width: %s%%; %s"><img src="%s" alt="%s" style="width: 100%%"/></figure>`+"\n",
				wr, wr, fmtNum(math.Round(pct*10)/10), gap, e.image(pic), alt)
			return
		}
		fmt.Fprintf(s, `<figure class="image" style="text-align: %s"><img src="%s" alt="%s" style="width: %s%%"/></figure>`+"\n",
			align, e.image(pic), alt, fmtNum(math.Round(pct*10)/10))
	case "bullet_list", "ordered_list":
		tag := "ul"
		attrs := ""
		if n.Type == "ordered_list" {
			tag = "ol"
			if st, ok := pmInt(n.Attrs["start"]); ok && st != 1 && st >= 0 {
				attrs = fmt.Sprintf(` start="%d"`, st)
			}
		}
		s.WriteString("<" + tag + attrs + ">\n")
		for _, item := range n.Content {
			if item == nil {
				continue
			}
			s.WriteString("<li>")
			for _, c := range item.Content {
				e.block(ctx, c)
			}
			s.WriteString("</li>\n")
		}
		s.WriteString("</" + tag + ">\n")
	case "toc":
		e.tocBlock(ctx, n)
	case "table": // exporttables.go
		e.table(ctx, n)
	default:
		for _, c := range n.Content {
			e.block(ctx, c)
		}
	}
}

func (e *epubOut) textBlock(ctx *epubCtx, n *pmNode) {
	s := ctx.s
	b := e.b
	styleID := b.paraStyleOf(n)
	tag := "p"
	if n.Type == "heading" {
		tag = fmt.Sprintf("h%d", headingLevel(n))
	}
	attrs := ""
	id := ""
	if n.Type == "heading" {
		id = e.headIDs[n]
	}
	bid := e.bmIDs[n]
	if id == "" {
		id = bid
	}
	if id != "" {
		attrs += ` id="` + id + `"`
	}
	classes := []string{}
	if styleID != "normal" || tag != "p" {
		classes = append(classes, e.classes[styleID])
	}
	if n.Type == "dialogue" {
		classes = append(classes, "dialogue")
	}
	if len(n.Content) == 0 && n.Type != "dialogue" {
		classes = append(classes, "empty")
	}
	if boxCSS(n) != "" {
		classes = append(classes, "boxed")
	}
	if dc, ok := pmInt(n.Attrs["dropCap"]); ok && dc >= 2 && dc <= 4 {
		classes = append(classes, fmt.Sprintf("dropcap%d", dc))
	}
	if len(classes) > 0 {
		attrs += ` class="` + strings.Join(classes, " ") + `"`
	}
	// The block's own attrs, in em of its text.
	_, st := b.styleFormat(styleID)
	var own exPara
	ownSet := map[string]bool{}
	for k := range n.Attrs {
		ownSet[k] = true
	}
	if len(ownSet) > 0 {
		own = b.resolvedPara(n)
		var decl []string
		if ownSet["align"] {
			if a, _ := n.Attrs["align"].(string); pmAligns[a] {
				decl = append(decl, "text-align: "+a)
			}
		}
		for _, x := range []struct {
			k, css string
			v      float64
		}{{"indentFirst", "text-indent", own.IndentFirst}, {"indentLeft", "margin-left", own.IndentLeft},
			{"indentRight", "margin-right", own.IndentRight}, {"spaceBefore", "margin-top", own.SpaceBefore},
			{"spaceAfter", "margin-bottom", own.SpaceAfter}} {
			if _, ok := pmNum(n.Attrs[x.k]); ok {
				decl = append(decl, x.css+": "+emOf(x.v, st.Size))
			}
		}
		if _, ok := pmNum(n.Attrs["lineExact"]); ok && own.LineExact > 0 {
			decl = append(decl, "line-height: "+fmtNum(own.LineExact/st.Size))
		} else if _, ok := pmNum(n.Attrs["lineHeight"]); ok {
			decl = append(decl, "line-height: "+fmtNum(own.LineHeight*1.2))
		}
		if attrBool(n, "keepNext") {
			decl = append(decl, "page-break-after: avoid")
		}
		if b := boxCSS(n); b != "" {
			decl = append(decl, b)
		}
		if len(decl) > 0 {
			attrs += ` style="` + xe(strings.Join(decl, "; ")) + `"`
		}
	}
	s.WriteString("<" + tag + attrs + ">")
	if bid != "" && bid != id {
		// A heading with a bookmark: the bookmark is an anchor inside.
		s.WriteString(`<span id="` + bid + `"></span>`)
	}
	if n.Type == "dialogue" {
		speaker, lead := dialogueParts(n)
		if speaker != "" {
			s.WriteString(`<span class="speaker">` + xe(speaker) + `</span>`)
		}
		s.WriteString(xe(lead))
		e.inlines(s, n.Content, styleID)
		s.WriteString("”")
	} else if len(n.Content) == 0 {
		s.WriteString("&#160;")
	} else {
		e.inlines(s, n.Content, styleID)
	}
	s.WriteString("</" + tag + ">\n")
}

// inlines writes text with the formatting that differs from its
// paragraph's style.
func (e *epubOut) inlines(s *strings.Builder, content []*pmNode, styleID string) {
	_, base := e.b.styleFormat(styleID)
	for _, c := range content {
		if c == nil {
			continue
		}
		switch c.Type {
		case "hard_break":
			s.WriteString("<br/>")
		case "tab":
			s.WriteString(`<span class="tab"> </span>`)
		case "footnote": // exportnotes.go
			e.noteRef(s, c)
		case "text":
			t := e.b.resolvedText(styleID, c.Marks)
			var open, close []string
			wrap := func(o, cl string) {
				open = append(open, o)
				close = append([]string{cl}, close...)
			}
			if href := linkHrefOf(c.Marks); href != "" {
				if strings.HasPrefix(href, "#") {
					if target, ok := e.anchors[strings.TrimSpace(href[1:])]; ok {
						wrap(`<a href="`+xe(target)+`">`, "</a>")
					}
				} else {
					wrap(`<a href="`+xe(uriEscape(href))+`">`, "</a>")
				}
			}
			if t.Bold && !base.Bold {
				wrap("<strong>", "</strong>")
			}
			if t.Italic && !base.Italic {
				wrap("<em>", "</em>")
			}
			if t.Underline && !base.Underline {
				wrap("<u>", "</u>")
			}
			if t.Strike && !base.Strike {
				wrap("<s>", "</s>")
			}
			if t.Sup && !base.Sup {
				wrap("<sup>", "</sup>")
			}
			if t.Sub && !base.Sub {
				wrap("<sub>", "</sub>")
			}
			var decl []string
			if t.Font != base.Font && t.Font != "" {
				decl = append(decl, "font-family: "+familyCSS(t.Font))
			}
			if math.Abs(t.Size-base.Size) > 0.01 && base.Size > 0 {
				decl = append(decl, "font-size: "+fmtNum(t.Size/base.Size)+"em")
			}
			if t.Color != base.Color {
				c := t.Color
				if c == "" {
					c = "inherit"
				}
				decl = append(decl, "color: "+c)
			}
			if t.Highlight != base.Highlight {
				h := t.Highlight
				if h == "" {
					h = "transparent"
				}
				decl = append(decl, "background-color: "+h)
			}
			if t.SmallCaps != base.SmallCaps {
				decl = append(decl, "font-variant: "+map[bool]string{true: "small-caps", false: "normal"}[t.SmallCaps])
			}
			if !t.Bold && base.Bold {
				decl = append(decl, "font-weight: normal")
			}
			if !t.Italic && base.Italic {
				decl = append(decl, "font-style: normal")
			}
			if !t.Underline && base.Underline || !t.Strike && base.Strike {
				decl = append(decl, "text-decoration: none")
			}
			if len(decl) > 0 {
				wrap(`<span style="`+xe(strings.Join(decl, "; "))+`">`, "</span>")
			}
			s.WriteString(strings.Join(open, "") + xe(c.Text) + strings.Join(close, ""))
		}
	}
}

func (e *epubOut) tocBlock(ctx *epubCtx, n *pmNode) {
	s := ctx.s
	s.WriteString(`<section class="toc">` + "\n")
	if t := strings.TrimSpace(tocTitle(n)); t != "" {
		s.WriteString(`<p class="toc-title">` + xe(t) + "</p>\n")
	}
	var items []navItem
	for _, h := range e.b.headingsForTOC(tocLevels(n)) {
		items = append(items, navItem{level: h.level, label: h.text, href: e.headFile[h.node] + "#" + e.headIDs[h.node]})
	}
	if len(items) == 0 {
		s.WriteString("</section>\n")
		return
	}
	s.WriteString(navList(items, false))
	s.WriteString("</section>\n")
}

// --- navigation ---

type navItem struct {
	level int
	label string
	href  string
}

// navList writes items as nested ol by level.
func navList(items []navItem, withIDs bool) string {
	var s strings.Builder
	depth := 0
	base := items[0].level
	for _, it := range items {
		if it.level < base {
			base = it.level
		}
	}
	open := []bool{} // an li is open at each depth
	for _, it := range items {
		lv := it.level - base + 1
		if lv > depth+1 {
			lv = depth + 1
		}
		for depth < lv {
			s.WriteString("<ol>\n")
			depth++
			open = append(open, false)
		}
		for depth > lv {
			if open[depth-1] {
				s.WriteString("</li>\n")
			}
			s.WriteString("</ol>\n")
			open = open[:depth-1]
			depth--
		}
		if open[depth-1] {
			s.WriteString("</li>\n")
		}
		s.WriteString(`<li><a href="` + xe(it.href) + `">` + xe(it.label) + `</a>`)
		open[depth-1] = true
	}
	for depth > 0 {
		if open[depth-1] {
			s.WriteString("</li>\n")
		}
		s.WriteString("</ol>\n")
		open = open[:depth-1]
		depth--
	}
	return s.String()
}

// navItems: each chapter (its title), with its headings below Heading 1
// (and any later Heading 1) nested under it.
func (e *epubOut) navItems(chapters []exportChapter) []navItem {
	var out []navItem
	for i, c := range chapters {
		label := strings.TrimSpace(c.Title)
		first := true
		var sub []navItem
		pmWalk(c.Doc, func(n *pmNode) {
			if n.Type != "heading" {
				return
			}
			text := strings.Join(strings.Fields(pmTextblockText(n, " ", " ")), " ")
			lv := headingLevel(n)
			if first && lv == 1 {
				first = false
				if label == "" {
					label = text
				}
				return
			}
			first = false
			if text == "" || lv > 3 {
				return
			}
			sub = append(sub, navItem{level: max(2, lv), label: text, href: e.files[i] + "#" + e.headIDs[n]})
		})
		if label == "" {
			label = fmt.Sprintf("%d", i+1)
		}
		out = append(out, navItem{level: 1, label: label, href: e.files[i]})
		out = append(out, sub...)
	}
	return out
}

func (e *epubOut) nav(chapters []exportChapter) []byte {
	var s strings.Builder
	s.WriteString(e.pageHead("Contents"))
	s.WriteString("<body>\n<nav epub:type=\"toc\" id=\"toc\">\n<h1>Contents</h1>\n")
	s.WriteString(navList(e.navItems(chapters), false))
	s.WriteString("</nav>\n</body>\n</html>\n")
	return []byte(s.String())
}

func (e *epubOut) ncx(ident string, chapters []exportChapter) []byte {
	var s strings.Builder
	s.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" + `<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">` +
		`<head><meta name="dtb:uid" content="` + xe(ident) + `"/><meta name="dtb:depth" content="1"/>` +
		`<meta name="dtb:totalPageCount" content="0"/><meta name="dtb:maxPageNumber" content="0"/></head>` +
		`<docTitle><text>` + xe(e.b.BookTitle()) + `</text></docTitle><navMap>`)
	n := 0
	for _, it := range e.navItems(chapters) {
		if it.level != 1 {
			continue
		}
		n++
		fmt.Fprintf(&s, `<navPoint id="np%d" playOrder="%d"><navLabel><text>%s</text></navLabel><content src="%s"/></navPoint>`, n, n, xe(it.label), xe(it.href))
	}
	s.WriteString("</navMap></ncx>\n")
	return []byte(s.String())
}

func (e *epubOut) opf(ident string, chapters []exportChapter) []byte {
	b := e.b
	m := b.Meta
	var s strings.Builder
	s.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" +
		`<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="bookid" xml:lang="` + xe(m.Language) + `">` + "\n" +
		`<metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` + "\n")
	fmt.Fprintf(&s, `<dc:identifier id="bookid">%s</dc:identifier>`+"\n", xe(ident))
	fmt.Fprintf(&s, `<dc:title id="title">%s</dc:title>`+"\n"+`<meta refines="#title" property="title-type">main</meta>`+"\n", xe(b.BookTitle()))
	if m.Subtitle != "" {
		fmt.Fprintf(&s, `<dc:title id="subtitle">%s</dc:title>`+"\n"+`<meta refines="#subtitle" property="title-type">subtitle</meta>`+"\n", xe(m.Subtitle))
	}
	if m.Author != "" {
		fmt.Fprintf(&s, `<dc:creator id="creator">%s</dc:creator>`+"\n"+`<meta refines="#creator" property="role" scheme="marc:relators">aut</meta>`+"\n", xe(m.Author))
	}
	fmt.Fprintf(&s, `<dc:language>%s</dc:language>`+"\n", xe(m.Language))
	for _, x := range []struct{ tag, v string }{{"publisher", m.Publisher}, {"rights", m.Rights}, {"description", m.Description}} {
		if x.v != "" {
			fmt.Fprintf(&s, `<dc:%s>%s</dc:%s>`+"\n", x.tag, xe(x.v), x.tag)
		}
	}
	fmt.Fprintf(&s, `<meta property="dcterms:modified">%s</meta>`+"\n", editorNow().UTC().Format("2006-01-02T15:04:05Z"))
	if e.cover != nil {
		e.image(e.cover)
		fmt.Fprintf(&s, `<meta name="cover" content="%s"/>`+"\n", e.imgIDs[e.cover])
	}
	s.WriteString("</metadata>\n<manifest>\n")
	s.WriteString(`<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>` + "\n")
	s.WriteString(`<item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>` + "\n")
	s.WriteString(`<item id="css" href="style.css" media-type="text/css"/>` + "\n")
	for i := range chapters {
		fmt.Fprintf(&s, `<item id="ch%03d" href="%s" media-type="application/xhtml+xml"/>`+"\n", i+1, e.files[i])
	}
	for i, f := range b.Fonts {
		fmt.Fprintf(&s, `<item id="font%02d" href="%s" media-type="%s"/>`+"\n", i+1, e.fonts[i], fontMIME(f.Format))
	}
	for _, pic := range e.imgOrder {
		props := ""
		if pic == e.cover {
			props = ` properties="cover-image"`
		}
		fmt.Fprintf(&s, `<item id="%s" href="%s" media-type="%s"%s/>`+"\n", e.imgIDs[pic], e.images[pic], pic.ContentType, props)
	}
	s.WriteString("</manifest>\n<spine toc=\"ncx\">\n")
	for i := range chapters {
		fmt.Fprintf(&s, `<itemref idref="ch%03d"/>`+"\n", i+1)
	}
	s.WriteString("</spine>\n</package>\n")
	return []byte(s.String())
}

// boxCSS: the declarations of a paragraph's box (border, shading), "" when
// it has neither. The editor draws the same: 4pt 6pt of padding inside.
func boxCSS(n *pmNode) string {
	var d []string
	w, ok := pmNum(n.Attrs["boxWidth"])
	if ok && w > 0 {
		col := "#000000"
		if c, _ := n.Attrs["boxColor"].(string); pmHexColor(c) {
			col = c
		}
		d = append(d, fmt.Sprintf("border: %spt solid %s", fmtNum(w), col))
	}
	if c, _ := n.Attrs["shade"].(string); pmHexColor(c) {
		d = append(d, "background-color: "+c)
	}
	if len(d) > 0 {
		d = append(d, "padding: 4pt 6pt")
	}
	return strings.Join(d, "; ")
}
