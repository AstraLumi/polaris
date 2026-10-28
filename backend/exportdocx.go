package main

// A book as a Word file (export.go; docs/EDITOR.md, "Exports"). The
// mirror of the .docx import (docx*.go): what the import reads, this
// writes, so a book exported and uploaded again comes back the same apart
// from the losses listed at the end of this comment.
//
// Package: [Content_Types].xml, _rels/.rels, docProps/core.xml (meta:
// title, subtitle as subject, author as creator, description, ISBN as
// identifier, language) and app.xml (publisher as company),
// word/document.xml, styles.xml, numbering.xml (when there are lists),
// settings.xml, header and footer parts, media/.
//
// Styles (styles.xml): every book style is a Word style. normal is
// Normal (and the document defaults), heading1–6 are Word's "heading 1"–6
// (outline levels 0–5, so Word's navigation pane and TOC see them; a
// renamed one keeps its name as an alias), title, subtitle and quote are
// Title, Subtitle and Quote; any other keeps its name, with its id (less a
// "word-" prefix) as the Word style id. based_on is basedOn (normal when
// none). Normal lists every value; other styles what differs from their
// base. Character styles are character styles, applied with rStyle. Also
// written: Scene Break, TOC Heading, toc 1–3 (right tab at the margin,
// dot leader when layout.toc_dots), header and footer (centre and right
// tabs).
//
// Body: one section per chapter (sectPr: page size, margins with the
// inner one as Word's left one and the gutter, mirrorMargins in
// settings.xml, titlePg for different_first, evenAndOddHeaders, type
// nextPage or oddPage per chapter_start, pgNumType with the numbering
// format and restarts: front matter in lower roman from i, the first
// other chapter from numbering.start, number_restart where set). Headers
// and footers come from the bands, per chapter ({chapter} and
// {chapter_number} are written out, {page} is a PAGE field); a slot equal
// to the previous chapter's is left to Word's "link to previous"; front
// matter and no_header chapters get an empty header.
//
//   - paragraph / heading: pStyle; align (jc), indents (ind; a negative
//     indentFirst is a hanging indent), spaceBefore/After, lineHeight (line
//     auto, 240 = 1) or lineExact (line exact), keepNext, bookmark
//     (bookmarkStart/End). A heading whose style isn't a heading style gets
//     its outline level directly; a paragraph whose style is one gets
//     outline level 9 (body text), so the import reads each as it was.
//   - runs: a text's formatting is worked out as the editor shows it and
//     as Word would from the styles alone (Word flips bold, italic, strike
//     and small caps when a character style sets them); what differs is
//     written directly. Highlight colours Word names are w:highlight,
//     others shading. link: a web link is a w:hyperlink with an external
//     relationship, "#name" a w:hyperlink anchor. story_link: plain text.
//     hard_break w:br, tab w:tab, soft and no-break hyphens their elements.
//   - page_break: the next paragraph starts a page (pageBreakBefore), so no
//     empty line is left at the top of the page; a chapter's first or last
//     page break is dropped (chapters start on a new page anyway).
//   - scene_break: a centred Scene Break paragraph with the glyph; an
//     image ornament is a centred picture there.
//   - image: an inline picture in its own paragraph, aligned, at its width
//     (natural size at 96 dpi when null), never wider than the text or
//     taller than the page's text; fullPage: a picture anchored to the
//     page at its full size (cropped to the page's shape, in front of the
//     text so it covers the header and footer) on a page of its own.
//   - lists: numbering.xml, one numbering per top-level list, nested lists
//     as its deeper levels (bullets • ◦ ▪; numbers 1. at every level, as the
//     editor shows them); a list item's other
//     paragraphs are indented to its text.
//   - toc: a "Table of Contents" content control holding the title (TOC
//     Heading) and a TOC field \o "1-N" \h \z \u whose shown result lists
//     the headings (linked, without page numbers); updateFields in
//     settings.xml has Word fill in the pages when the file is opened.
//   - dialogue: a paragraph "Speaker, manner: “line”", the speaker in small
//     caps.
//
// Round trip (export, then the .docx import): text, paragraph formatting,
// text formatting, styles in use, page setup and pictures come back as
// they were. Known losses: the toc (the import drops tables of contents),
// dialogue (a plain paragraph), image ornaments (pictures), page breaks
// around a full-page picture (come back as page_break nodes), a null
// picture width (comes back as the natural width), character styles (the
// import gives their formatting as marks), nested lists (the import makes
// them one list) and a list item's later paragraphs (plain indented
// paragraphs), story links (plain text), style ids (a style made in
// Polaris comes back as "word-<id>"), a renamed built-in style's name,
// styles nothing uses, sizes off the half point and lengths off the twip,
// bookmark names Word refuses (made safe), headers, footers and meta (not
// read by the import yet).

import (
	"archive/zip"
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

const (
	docxNSDecl = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
		`xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" ` +
		`xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
		`xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture"`
	xmlHead    = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n"
	relTypeDoc = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/"
	emuPerPt   = 12700
)

// --- zip ---

type zipEntry struct {
	name  string
	data  []byte
	store bool // no compression (an EPUB's mimetype)
}

func writeZip(entries []zipEntry) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	now := editorNow()
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate, Modified: now}
		if e.store {
			// Stored, with the old DOS time only: an EPUB's mimetype may
			// carry no extra field (Modified would add one).
			h = &zip.FileHeader{Name: e.name, Method: zip.Store}
			h.ModifiedDate = uint16(now.Day() + int(now.Month())<<5 + (max(now.Year(), 1980)-1980)<<9)
			h.ModifiedTime = uint16(now.Second()/2 + now.Minute()<<5 + now.Hour()<<11)
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(e.data); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// --- units ---

func twips(pt float64) int   { return int(math.Round(pt * 20)) }
func mmTwips(mm float64) int { return int(math.Round(mm * 1440 / 25.4)) }

// --- styles ---

var wordBuiltinNames = map[string]string{"normal": "Normal", "title": "Title", "subtitle": "Subtitle", "quote": "Quote",
	"heading1": "heading 1", "heading2": "heading 2", "heading3": "heading 3", "heading4": "heading 4",
	"heading5": "heading 5", "heading6": "heading 6"}

var wordBuiltinIDs = map[string]string{"normal": "Normal", "title": "Title", "subtitle": "Subtitle", "quote": "Quote",
	"heading1": "Heading1", "heading2": "Heading2", "heading3": "Heading3", "heading4": "Heading4",
	"heading5": "Heading5", "heading6": "Heading6"}

// Ids and names the export uses for its own styles.
var wordReservedIDs = []string{"Normal", "Title", "Subtitle", "Quote", "Heading1", "Heading2", "Heading3", "Heading4",
	"Heading5", "Heading6", "Heading7", "Heading8", "Heading9", "SceneBreak", "TOCHeading", "TOC1", "TOC2", "TOC3",
	"Header", "Footer", "DefaultParagraphFont"}
var wordReservedNames = []string{"normal", "title", "subtitle", "quote", "heading 1", "heading 2", "heading 3", "heading 4",
	"heading 5", "heading 6", "heading 7", "heading 8", "heading 9", "scene break", "toc heading", "toc 1", "toc 2", "toc 3",
	"header", "footer", "default paragraph font"}

type docxStyles struct {
	id      map[string]string // book style id → Word styleId
	name    map[string]string
	outline map[string]int  // a paragraph style's outline level (-1: body text)
	bookHas map[string]bool // the export's own styles the book already has
}

// exportOwnStyles: the styles the export adds, by their Word id, with
// their Word names. A book that has one (an imported "word-footer"…) keeps
// its own: the bands and the table of contents set what they need directly.
var exportOwnStyles = map[string]string{"SceneBreak": "Scene Break", "TOCHeading": "TOC Heading", "TOC1": "toc 1",
	"TOC2": "toc 2", "TOC3": "toc 3", "Header": "header", "Footer": "footer"}

func mapWordStyles(b *exportBook) docxStyles {
	sm := docxStyles{id: map[string]string{}, name: map[string]string{}, outline: map[string]int{}, bookHas: map[string]bool{}}
	usedIDs, usedNames := map[string]bool{}, map[string]bool{}
	for _, id := range wordReservedIDs {
		usedIDs[strings.ToLower(id)] = true
	}
	for _, n := range wordReservedNames {
		usedNames[n] = true
	}
	for _, s := range b.Styles {
		if wid, ok := wordBuiltinIDs[s.ID]; ok && s.Kind == "paragraph" {
			sm.id[s.ID], sm.name[s.ID] = wid, wordBuiltinNames[s.ID]
			continue
		}
		if rest, ok := strings.CutPrefix(s.ID, "word-"); ok && s.Kind == "paragraph" {
			matched := false
			for wid, name := range exportOwnStyles {
				if strings.EqualFold(strings.ReplaceAll(rest, "-", ""), wid) && !sm.bookHas[wid] {
					sm.id[s.ID], sm.name[s.ID], sm.bookHas[wid] = wid, name, true
					matched = true
				}
			}
			if matched {
				continue
			}
		}
		base := strings.TrimPrefix(s.ID, "word-")
		var w strings.Builder
		for _, r := range base {
			if r < 0x80 && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_') {
				w.WriteRune(r)
			}
		}
		wid := w.String()
		if wid == "" {
			wid = "Style"
		}
		if len(wid) > 60 {
			wid = wid[:60]
		}
		cand := wid
		for i := 2; usedIDs[strings.ToLower(cand)]; i++ {
			cand = fmt.Sprintf("%s%d", wid, i)
		}
		usedIDs[strings.ToLower(cand)] = true
		sm.id[s.ID] = cand
		name := strings.TrimSpace(s.Name)
		if name == "" {
			name = s.ID
		}
		cn := name
		for i := 2; usedNames[strings.ToLower(cn)]; i++ {
			cn = fmt.Sprintf("%s (%d)", name, i)
		}
		usedNames[strings.ToLower(cn)] = true
		sm.name[s.ID] = cn
	}
	for _, s := range b.Styles {
		if s.Kind != "paragraph" {
			continue
		}
		lv := -1
		for _, c := range b.chain(s.ID) {
			if n := headingStyleLevel(c.ID); n > 0 {
				lv = n - 1
			}
		}
		sm.outline[s.ID] = lv
	}
	return sm
}

func headingStyleLevel(id string) int {
	if len(id) == 8 && strings.HasPrefix(id, "heading") && id[7] >= '1' && id[7] <= '6' {
		return int(id[7] - '0')
	}
	return 0
}

var wordHighlightNames = func() map[string]string {
	m := map[string]string{}
	for name, c := range highlightColors {
		m[c] = name
	}
	return m
}()

func hexVal(c string) string { return strings.ToUpper(strings.TrimPrefix(c, "#")) }

// runPropsXML writes w:rPr's inner elements for what `want` has and
// `have` (what applies already) doesn't. full writes every value.
func runPropsXML(want, have exText, full bool, rStyle string) string {
	var b strings.Builder
	if rStyle != "" {
		fmt.Fprintf(&b, `<w:rStyle w:val="%s"/>`, xe(rStyle))
	}
	if want.Font != "" && (full || want.Font != have.Font) {
		f := xe(want.Font)
		fmt.Fprintf(&b, `<w:rFonts w:ascii="%s" w:hAnsi="%s" w:eastAsia="%s" w:cs="%s"/>`, f, f, f, f)
	}
	toggle := func(tag string, w, h bool, cs string) {
		if w == h && !(full && w) {
			return
		}
		if w {
			fmt.Fprintf(&b, `<w:%s/>`, tag)
			if cs != "" {
				fmt.Fprintf(&b, `<w:%s/>`, cs)
			}
		} else {
			fmt.Fprintf(&b, `<w:%s w:val="0"/>`, tag)
			if cs != "" {
				fmt.Fprintf(&b, `<w:%s w:val="0"/>`, cs)
			}
		}
	}
	toggle("b", want.Bold, have.Bold, "bCs")
	toggle("i", want.Italic, have.Italic, "iCs")
	toggle("smallCaps", want.SmallCaps, have.SmallCaps, "")
	toggle("strike", want.Strike, have.Strike, "")
	if want.Color != have.Color || full && want.Color != "" {
		if want.Color == "" {
			b.WriteString(`<w:color w:val="auto"/>`)
		} else {
			fmt.Fprintf(&b, `<w:color w:val="%s"/>`, hexVal(want.Color))
		}
	}
	if want.Size > 0 && (full || math.Round(want.Size*2) != math.Round(have.Size*2)) {
		hp := int(math.Round(want.Size * 2))
		fmt.Fprintf(&b, `<w:sz w:val="%d"/><w:szCs w:val="%d"/>`, hp, hp)
	}
	shd := ""
	if want.Highlight != have.Highlight || full && want.Highlight != "" {
		switch name := wordHighlightNames[want.Highlight]; {
		case want.Highlight == "":
			b.WriteString(`<w:highlight w:val="none"/>`)
			shd = `<w:shd w:val="clear" w:color="auto" w:fill="auto"/>`
		case name != "":
			fmt.Fprintf(&b, `<w:highlight w:val="%s"/>`, name)
		default:
			shd = fmt.Sprintf(`<w:shd w:val="clear" w:color="auto" w:fill="%s"/>`, hexVal(want.Highlight))
		}
	}
	if want.Underline != have.Underline || full && want.Underline {
		if want.Underline {
			b.WriteString(`<w:u w:val="single"/>`)
		} else {
			b.WriteString(`<w:u w:val="none"/>`)
		}
	}
	b.WriteString(shd)
	if want.Sup != have.Sup || want.Sub != have.Sub || full && (want.Sup || want.Sub) {
		v := "baseline"
		if want.Sup {
			v = "superscript"
		} else if want.Sub {
			v = "subscript"
		}
		fmt.Fprintf(&b, `<w:vertAlign w:val="%s"/>`, v)
	}
	return b.String()
}

// charRunXML: a character style's own settings (only the keys it sets).
func charRunXML(m map[string]any) string {
	var have, want exText
	set := map[string]bool{}
	for k := range m {
		set[k] = true
	}
	applyText(&want, m)
	// Start from the opposite of each set value so it is written.
	have = want
	if set["font"] {
		have.Font = "\x00"
	}
	if set["size"] {
		have.Size = -1
	}
	if set["color"] {
		have.Color = "\x00"
	}
	if set["highlight"] {
		have.Highlight = "\x00"
	}
	for _, t := range []struct {
		k    string
		w, h *bool
	}{{"bold", &want.Bold, &have.Bold}, {"italic", &want.Italic, &have.Italic}, {"underline", &want.Underline, &have.Underline},
		{"strike", &want.Strike, &have.Strike}, {"smallcaps", &want.SmallCaps, &have.SmallCaps}} {
		if set[t.k] {
			*t.h = !*t.w
		}
	}
	if set["superscript"] || set["subscript"] {
		have.Sup, have.Sub = !want.Sup, !want.Sub
		if !want.Sup && !want.Sub {
			have.Sup = true
		}
	}
	return runPropsXML(want, have, false, "")
}

// paraPropsXML writes the spacing, ind, jc and keepNext of `want` that
// differ from `have` (all of them when full). Ordered as Word wants:
// keepNext comes before, spacing, ind and jc after the elements the
// caller writes in between (returned apart).
func paraPropsXML(want, have exPara, full bool) (keep, rest string) {
	if want.KeepNext != have.KeepNext || full && want.KeepNext {
		if want.KeepNext {
			keep = `<w:keepNext/>`
		} else {
			keep = `<w:keepNext w:val="0"/>`
		}
	}
	var b strings.Builder
	var sp []string
	if full || twips(want.SpaceBefore) != twips(have.SpaceBefore) {
		sp = append(sp, fmt.Sprintf(`w:before="%d"`, twips(want.SpaceBefore)))
	}
	if full || twips(want.SpaceAfter) != twips(have.SpaceAfter) {
		sp = append(sp, fmt.Sprintf(`w:after="%d"`, twips(want.SpaceAfter)))
	}
	if want.LineExact > 0 {
		if full || want.LineExact != have.LineExact {
			sp = append(sp, fmt.Sprintf(`w:line="%d" w:lineRule="exact"`, twips(want.LineExact)))
		}
	} else if full || have.LineExact > 0 || math.Round(want.LineHeight*240) != math.Round(have.LineHeight*240) {
		sp = append(sp, fmt.Sprintf(`w:line="%d" w:lineRule="auto"`, int(math.Round(want.LineHeight*240))))
	}
	if len(sp) > 0 {
		b.WriteString(`<w:spacing ` + strings.Join(sp, " ") + `/>`)
	}
	var ind []string
	if full || twips(want.IndentLeft) != twips(have.IndentLeft) {
		ind = append(ind, fmt.Sprintf(`w:left="%d"`, twips(want.IndentLeft)))
	}
	if full || twips(want.IndentRight) != twips(have.IndentRight) {
		ind = append(ind, fmt.Sprintf(`w:right="%d"`, twips(want.IndentRight)))
	}
	if full || twips(want.IndentFirst) != twips(have.IndentFirst) {
		if want.IndentFirst < 0 {
			ind = append(ind, fmt.Sprintf(`w:hanging="%d"`, twips(-want.IndentFirst)))
		} else {
			ind = append(ind, fmt.Sprintf(`w:firstLine="%d"`, twips(want.IndentFirst)))
		}
	}
	if len(ind) > 0 {
		b.WriteString(`<w:ind ` + strings.Join(ind, " ") + `/>`)
	}
	if full || want.Align != have.Align {
		b.WriteString(`<w:jc w:val="` + wordJc(want.Align) + `"/>`)
	}
	return keep, b.String()
}

func wordJc(a string) string {
	switch a {
	case "center", "right":
		return a
	case "justify":
		return "both"
	}
	return "left"
}

func (d *docxOut) stylesXML() []byte {
	b := d.b
	var s strings.Builder
	s.WriteString(xmlHead + `<w:styles ` + docxNSDecl + `>`)
	_, nt := b.styleFormat("normal")
	lang := xe(b.Meta.Language)
	font := xe(nt.Font)
	s.WriteString(`<w:docDefaults><w:rPrDefault><w:rPr>`)
	if font != "" {
		fmt.Fprintf(&s, `<w:rFonts w:ascii="%s" w:hAnsi="%s" w:eastAsia="%s" w:cs="%s"/>`, font, font, font, font)
	}
	hp := int(math.Round(nt.Size * 2))
	fmt.Fprintf(&s, `<w:sz w:val="%d"/><w:szCs w:val="%d"/><w:lang w:val="%s"/></w:rPr></w:rPrDefault>`, hp, hp, lang)
	s.WriteString(`<w:pPrDefault><w:pPr><w:spacing w:after="0" w:line="240" w:lineRule="auto"/></w:pPr></w:pPrDefault></w:docDefaults>`)
	s.WriteString(`<w:style w:type="character" w:default="1" w:styleId="DefaultParagraphFont"><w:name w:val="Default Paragraph Font"/><w:uiPriority w:val="1"/><w:semiHidden/><w:unhideWhenUsed/></w:style>`)

	for _, st := range b.Styles {
		wid, name := d.sm.id[st.ID], d.sm.name[st.ID]
		if st.Kind == "character" {
			fmt.Fprintf(&s, `<w:style w:type="character" w:customStyle="1" w:styleId="%s"><w:name w:val="%s"/>`, xe(wid), xe(name))
			if base := b.style(st.BasedOn); base != nil && base.Kind == "character" && base.ID != st.ID {
				fmt.Fprintf(&s, `<w:basedOn w:val="%s"/>`, xe(d.sm.id[base.ID]))
			}
			s.WriteString(`<w:qFormat/><w:rPr>` + charRunXML(st.Text) + `</w:rPr></w:style>`)
			continue
		}
		_, builtin := wordBuiltinIDs[st.ID]
		def := ""
		if st.ID == "normal" {
			def = ` w:default="1"`
		}
		custom := ""
		if !builtin {
			custom = ` w:customStyle="1"`
		}
		fmt.Fprintf(&s, `<w:style w:type="paragraph"%s%s w:styleId="%s"><w:name w:val="%s"/>`, def, custom, xe(wid), xe(name))
		if builtin && st.Name != "" && !strings.EqualFold(st.Name, name) && !strings.EqualFold(st.Name, strings.Replace(name, "heading", "Heading", 1)) {
			fmt.Fprintf(&s, `<w:aliases w:val="%s"/>`, xe(strings.ReplaceAll(st.Name, ",", " ")))
		}
		p, t := b.styleFormat(st.ID)
		var bp exPara
		var bt exText
		full := st.ID == "normal"
		if !full {
			baseID := "normal"
			if base := b.style(st.BasedOn); base != nil && base.Kind == "paragraph" && base.ID != st.ID {
				baseID = base.ID
			}
			fmt.Fprintf(&s, `<w:basedOn w:val="%s"/>`, xe(d.sm.id[baseID]))
			bp, bt = b.styleFormat(baseID)
		}
		if d.sm.outline[st.ID] >= 0 || st.ID == "title" || st.ID == "subtitle" {
			s.WriteString(`<w:next w:val="Normal"/>`)
		}
		if lv := headingStyleLevel(st.ID); lv > 0 {
			s.WriteString(`<w:uiPriority w:val="9"/>`)
		}
		if builtin {
			s.WriteString(`<w:qFormat/>`)
		}
		keep, rest := paraPropsXML(p, bp, full)
		s.WriteString(`<w:pPr>` + keep + rest)
		if lv := headingStyleLevel(st.ID); lv > 0 {
			fmt.Fprintf(&s, `<w:outlineLvl w:val="%d"/>`, lv-1)
		}
		s.WriteString(`</w:pPr><w:rPr>` + runPropsXML(t, bt, full, "") + `</w:rPr></w:style>`)
	}

	// The export's own styles.
	body := b.bodySize()
	tw := twips(b.textWidthPt())
	own := func(id string) bool { return !d.sm.bookHas[id] }
	if own("SceneBreak") {
		fmt.Fprintf(&s, `<w:style w:type="paragraph" w:customStyle="1" w:styleId="SceneBreak"><w:name w:val="Scene Break"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/>`+
			`<w:pPr><w:spacing w:before="%d" w:after="%d"/><w:ind w:left="0" w:right="0" w:firstLine="0"/><w:jc w:val="center"/></w:pPr></w:style>`, twips(body), twips(body))
	}
	if own("TOCHeading") {
		s.WriteString(`<w:style w:type="paragraph" w:styleId="TOCHeading"><w:name w:val="TOC Heading"/><w:basedOn w:val="Heading1"/><w:next w:val="Normal"/><w:uiPriority w:val="39"/><w:unhideWhenUsed/><w:qFormat/>` +
			`<w:pPr><w:outlineLvl w:val="9"/></w:pPr></w:style>`)
	}
	leader := "dot"
	if !b.Layout.TocDots {
		leader = "none"
	}
	for lv := 1; lv <= 3; lv++ {
		if !own(fmt.Sprintf("TOC%d", lv)) {
			continue
		}
		fmt.Fprintf(&s, `<w:style w:type="paragraph" w:styleId="TOC%d"><w:name w:val="toc %d"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:uiPriority w:val="39"/><w:unhideWhenUsed/>`+
			`<w:pPr><w:tabs><w:tab w:val="right" w:leader="%s" w:pos="%d"/></w:tabs><w:spacing w:before="0" w:after="%d"/><w:ind w:left="%d" w:right="0" w:firstLine="0"/><w:jc w:val="left"/></w:pPr></w:style>`,
			lv, lv, leader, tw, twips(body*0.4), twips(body*1.5*float64(lv-1)))
	}
	for _, hf := range []struct{ id, name string }{{"Header", "header"}, {"Footer", "footer"}} {
		if !own(hf.id) {
			continue
		}
		fmt.Fprintf(&s, `<w:style w:type="paragraph" w:styleId="%s"><w:name w:val="%s"/><w:basedOn w:val="Normal"/><w:uiPriority w:val="99"/><w:unhideWhenUsed/>`+
			`<w:pPr><w:tabs><w:tab w:val="center" w:pos="%d"/><w:tab w:val="right" w:pos="%d"/></w:tabs><w:spacing w:before="0" w:after="0" w:line="240" w:lineRule="auto"/><w:ind w:left="0" w:right="0" w:firstLine="0"/><w:jc w:val="left"/></w:pPr></w:style>`,
			hf.id, hf.name, tw/2, tw)
	}
	s.WriteString(`</w:styles>`)
	return []byte(s.String())
}

// --- the writer ---

type docxPara struct {
	ppr  strings.Builder // pPr's inner elements, sectPr aside
	body strings.Builder
}

type docxPart struct {
	name, contentType string
	data              []byte
}

type docxOut struct {
	b  *exportBook
	sm docxStyles

	doc       strings.Builder
	pending   *docxPara
	rels      strings.Builder
	relN      int
	parts     []docxPart
	media     map[*exportPicture]string // picture → relationship id
	mediaN    int
	exts      map[string]string // extension → content type
	docPr     int
	bmID      int
	bmNames   map[string]string // editor bookmark → Word bookmark
	bmUsed    map[string]bool
	tocAnchor map[*pmNode]string
	hasTOC    bool

	absNums, nums strings.Builder
	numN          int

	breakNext bool      // the next paragraph starts a page
	anchored  *docxPara // the paragraph holding a full-page picture
	started   bool      // something was written in the current chapter

	slots   map[string]string // header/footer slot → the previous section's content
	hfN     int
	pageW   int // twips
	pageH   int
	textW   float64 // pt
	textH   float64
	section int
}

func exportDocx(b *exportBook) ([]byte, error) {
	d := &docxOut{b: b, sm: mapWordStyles(b), media: map[*exportPicture]string{}, exts: map[string]string{},
		bmNames: map[string]string{}, bmUsed: map[string]bool{}, tocAnchor: map[*pmNode]string{}, slots: map[string]string{}}
	d.pageW, d.pageH = mmTwips(b.Page.WidthMM), mmTwips(b.Page.HeightMM)
	d.textW, d.textH = b.textWidthPt(), b.textHeightPt()
	d.relN = 10 // rId1–rId9 for the fixed parts

	// Bookmarks the placeholder table of contents links to.
	hasTOC := false
	for _, c := range b.Chapters {
		pmWalk(c.Doc, func(n *pmNode) {
			if n.Type == "toc" {
				hasTOC = true
			}
		})
	}
	if hasTOC {
		for i, e := range b.headingsForTOC(3) {
			d.tocAnchor[e.node] = fmt.Sprintf("_TocPolaris%d", i+1)
		}
	}

	d.doc.WriteString(xmlHead + `<w:document ` + docxNSDecl + `><w:body>`)
	for i, c := range b.Chapters {
		d.section = i
		d.started = false
		d.breakNext = false
		d.blocks(trimPageBreaks(c.Doc.Content), nil)
		sect := d.sectPrFor(i)
		if i == len(b.Chapters)-1 {
			d.flush("")
			d.doc.WriteString(sect)
		} else {
			d.flush(sect)
		}
	}
	if len(b.Chapters) == 0 {
		d.doc.WriteString(`<w:p/>`)
		d.doc.WriteString(d.sectPrFor(-1))
	}
	d.doc.WriteString(`</w:body></w:document>`)

	// Package.
	var entries []zipEntry
	ct := strings.Builder{}
	ct.WriteString(xmlHead + `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>`)
	for _, ext := range sortedKeys(func() map[string]bool {
		m := map[string]bool{}
		for e := range d.exts {
			m[e] = true
		}
		return m
	}()) {
		fmt.Fprintf(&ct, `<Default Extension="%s" ContentType="%s"/>`, ext, d.exts[ext])
	}
	over := func(part, typ string) {
		fmt.Fprintf(&ct, `<Override PartName="/%s" ContentType="%s"/>`, part, typ)
	}
	const wml = "application/vnd.openxmlformats-officedocument.wordprocessingml."
	over("word/document.xml", wml+"document.main+xml")
	over("word/styles.xml", wml+"styles+xml")
	over("word/settings.xml", wml+"settings+xml")
	if d.numN > 0 {
		over("word/numbering.xml", wml+"numbering+xml")
	}
	for _, p := range d.parts {
		if p.contentType != "" {
			over(p.name, p.contentType)
		}
	}
	over("docProps/core.xml", "application/vnd.openxmlformats-package.core-properties+xml")
	over("docProps/app.xml", "application/vnd.openxmlformats-officedocument.extended-properties+xml")
	ct.WriteString(`</Types>`)
	entries = append(entries, zipEntry{name: "[Content_Types].xml", data: []byte(ct.String())})

	entries = append(entries, zipEntry{name: "_rels/.rels", data: []byte(xmlHead +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="` + relTypeDoc + `officeDocument" Target="word/document.xml"/>` +
		`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>` +
		`<Relationship Id="rId3" Type="` + relTypeDoc + `extended-properties" Target="docProps/app.xml"/>` +
		`</Relationships>`)})
	entries = append(entries, zipEntry{name: "docProps/core.xml", data: d.coreXML()})
	entries = append(entries, zipEntry{name: "docProps/app.xml", data: d.appXML()})
	entries = append(entries, zipEntry{name: "word/document.xml", data: []byte(d.doc.String())})
	entries = append(entries, zipEntry{name: "word/styles.xml", data: d.stylesXML()})
	entries = append(entries, zipEntry{name: "word/settings.xml", data: d.settingsXML()})

	var rels strings.Builder
	rels.WriteString(xmlHead + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="` + relTypeDoc + `styles" Target="styles.xml"/>` +
		`<Relationship Id="rId2" Type="` + relTypeDoc + `settings" Target="settings.xml"/>`)
	if d.numN > 0 {
		rels.WriteString(`<Relationship Id="rId3" Type="` + relTypeDoc + `numbering" Target="numbering.xml"/>`)
		entries = append(entries, zipEntry{name: "word/numbering.xml", data: []byte(xmlHead + `<w:numbering ` + docxNSDecl + `>` +
			d.absNums.String() + d.nums.String() + `</w:numbering>`)})
	}
	rels.WriteString(d.rels.String() + `</Relationships>`)
	entries = append(entries, zipEntry{name: "word/_rels/document.xml.rels", data: []byte(rels.String())})
	for _, p := range d.parts {
		entries = append(entries, zipEntry{name: p.name, data: p.data})
	}
	return writeZip(entries)
}

func (d *docxOut) rel(typ, target string, external bool) string {
	id := "rId" + strconv.Itoa(d.relN)
	d.relN++
	mode := ""
	if external {
		mode = ` TargetMode="External"`
	}
	fmt.Fprintf(&d.rels, `<Relationship Id="%s" Type="%s%s" Target="%s"%s/>`, id, relTypeDoc, typ, xe(target), mode)
	return id
}

// uriEscape makes a link safe as a relationship target: spaces, quotes and
// anything outside ASCII percent-encoded.
func uriEscape(h string) string {
	var b strings.Builder
	for _, c := range []byte(h) {
		if c <= 0x20 || c >= 0x7f || strings.IndexByte("\"<>\\^`{|}", c) >= 0 {
			fmt.Fprintf(&b, "%%%02X", c)
		} else {
			b.WriteByte(c)
		}
	}
	return b.String()
}

// flush writes the paragraph held back (sect goes in its pPr: a chapter's
// last paragraph carries its section). With sect and nothing held back,
// an empty paragraph carries it.
func (d *docxOut) flush(sect string) {
	p := d.pending
	d.pending = nil
	if p != nil && sect != "" && !strings.Contains(p.body.String(), "<w:r>") {
		// An empty paragraph carrying the section would be read as only
		// the section's break: keep it, and end the section after it.
		d.pending = p
		d.flush("")
		p = nil
	}
	if p == nil {
		if sect == "" {
			return
		}
		p = &docxPara{}
	}
	d.doc.WriteString(`<w:p>`)
	if p.ppr.Len() > 0 || sect != "" {
		d.doc.WriteString(`<w:pPr>` + p.ppr.String() + sect + `</w:pPr>`)
	}
	d.doc.WriteString(p.body.String() + `</w:p>`)
}

// para starts a paragraph (the previous one is written out).
func (d *docxOut) para() *docxPara {
	d.flush("")
	d.pending = &docxPara{}
	return d.pending
}

// breakBefore says whether this paragraph starts a page, and clears it.
func (d *docxOut) breakBefore() bool {
	br := d.breakNext && d.started
	d.breakNext = false
	d.started = true
	return br
}

type docxList struct {
	numID int
	ilvl  int
	first bool // the item's first paragraph: numbered
	left  int  // twips the item's text starts at
}

func (d *docxOut) blocks(nodes []*pmNode, lc *docxList) {
	for _, n := range nodes {
		if n != nil {
			d.block(n, lc)
		}
	}
}

func (d *docxOut) block(n *pmNode, lc *docxList) {
	switch n.Type {
	case "paragraph", "heading":
		d.textPara(n, lc)
	case "dialogue":
		d.dialogue(n, lc)
	case "scene_break":
		d.sceneBreak(n)
	case "page_break":
		if d.b.online() {
			break
		}
		if d.breakNext && d.started {
			// Two in a row: a blank page, held by an empty paragraph.
			d.para().ppr.WriteString(`<w:pageBreakBefore/>`)
		}
		d.breakNext = true
	case "image":
		d.image(n, lc)
	case "bullet_list", "ordered_list":
		d.list(n, lc)
	case "toc":
		d.toc(n)
	case "table": // docxtables.go
		d.table(n)
	default:
		d.blocks(n.Content, lc)
	}
}

// paraStart writes a block paragraph's pStyle … numPr (the parts before
// its spacing) and gives the rest.
func (d *docxOut) textParaPPr(p *docxPara, n *pmNode, lc *docxList) {
	styleID := d.b.paraStyleOf(n)
	if styleID != "normal" {
		fmt.Fprintf(&p.ppr, `<w:pStyle w:val="%s"/>`, xe(d.sm.id[styleID]))
	}
	if attrBool(n, "keepNext") {
		p.ppr.WriteString(`<w:keepNext/>`)
	}
	if d.breakBefore() {
		p.ppr.WriteString(`<w:pageBreakBefore/>`)
	}
	inList := lc != nil
	if inList && lc.first {
		fmt.Fprintf(&p.ppr, `<w:numPr><w:ilvl w:val="%d"/><w:numId w:val="%d"/></w:numPr>`, lc.ilvl, lc.numID)
	}
	// Spacing.
	a := n.Attrs
	var sp []string
	if v, ok := pmNum(a["spaceBefore"]); ok {
		sp = append(sp, fmt.Sprintf(`w:before="%d"`, twips(math.Max(0, v))))
	}
	if v, ok := pmNum(a["spaceAfter"]); ok {
		sp = append(sp, fmt.Sprintf(`w:after="%d"`, twips(math.Max(0, v))))
	}
	if v, ok := pmNum(a["lineExact"]); ok && v > 0 {
		sp = append(sp, fmt.Sprintf(`w:line="%d" w:lineRule="exact"`, twips(v)))
	} else if v, ok := pmNum(a["lineHeight"]); ok && v > 0 {
		sp = append(sp, fmt.Sprintf(`w:line="%d" w:lineRule="auto"`, int(math.Round(v*240))))
	}
	if len(sp) > 0 {
		p.ppr.WriteString(`<w:spacing ` + strings.Join(sp, " ") + `/>`)
	}
	var ind []string
	if inList && !lc.first {
		ind = append(ind, fmt.Sprintf(`w:left="%d"`, lc.left), `w:firstLine="0"`)
	} else if !inList {
		if v, ok := pmNum(a["indentLeft"]); ok {
			ind = append(ind, fmt.Sprintf(`w:left="%d"`, twips(v)))
		}
	}
	if v, ok := pmNum(a["indentRight"]); ok {
		ind = append(ind, fmt.Sprintf(`w:right="%d"`, twips(v)))
	}
	if !inList {
		if v, ok := pmNum(a["indentFirst"]); ok {
			if v < 0 {
				ind = append(ind, fmt.Sprintf(`w:hanging="%d"`, twips(-v)))
			} else {
				ind = append(ind, fmt.Sprintf(`w:firstLine="%d"`, twips(v)))
			}
		}
	}
	if len(ind) > 0 {
		p.ppr.WriteString(`<w:ind ` + strings.Join(ind, " ") + `/>`)
	}
	if al, _ := a["align"].(string); pmAligns[al] {
		p.ppr.WriteString(`<w:jc w:val="` + wordJc(al) + `"/>`)
	}
	// Outline level: a heading is one whatever its style; a paragraph never.
	styleLv := d.sm.outline[styleID]
	if n.Type == "heading" {
		if lv := headingLevel(n) - 1; lv != styleLv {
			fmt.Fprintf(&p.ppr, `<w:outlineLvl w:val="%d"/>`, lv)
		}
	} else if styleLv >= 0 {
		p.ppr.WriteString(`<w:outlineLvl w:val="9"/>`)
	}
}

func (d *docxOut) textPara(n *pmNode, lc *docxList) {
	if d.breakNext && d.started && d.pending != nil && len(n.Content) == 0 && attrString(n, "bookmark") == "" && d.tocAnchor[n] == "" {
		// An empty paragraph starting a page: the import would take it for
		// the break alone, so the break ends the paragraph before instead.
		if d.pending == d.anchored {
			// Not in a full-page picture's paragraph: the import puts the
			// picture after its paragraph, so the break goes in one of its own.
			d.flush("")
			d.doc.WriteString(`<w:p><w:r><w:br w:type="page"/></w:r></w:p>`)
		} else {
			d.pending.body.WriteString(`<w:r><w:br w:type="page"/></w:r>`)
		}
		d.breakNext = false
	}
	p := d.para()
	d.textParaPPr(p, n, lc)
	styleID := d.b.paraStyleOf(n)
	var marks []string
	if bm := attrString(n, "bookmark"); strings.TrimSpace(bm) != "" {
		marks = append(marks, d.bookmarkName(bm))
	}
	if a := d.tocAnchor[n]; a != "" {
		marks = append(marks, a)
	}
	ids := make([]int, len(marks))
	for i, m := range marks {
		ids[i] = d.bmID
		fmt.Fprintf(&p.body, `<w:bookmarkStart w:id="%d" w:name="%s"/>`, d.bmID, xe(m))
		d.bmID++
	}
	d.inlines(p, n.Content, styleID)
	for _, id := range ids {
		fmt.Fprintf(&p.body, `<w:bookmarkEnd w:id="%d"/>`, id)
	}
}

// bookmarkName makes a Word-safe bookmark name (letters, digits and
// underscores, at most 40, starting with a letter or an underscore for
// hidden ones), the same for a bookmark and the links to it.
func (d *docxOut) bookmarkName(name string) string {
	name = strings.TrimSpace(name)
	if w, ok := d.bmNames[name]; ok {
		return w
	}
	var b strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	w := b.String()
	if r := []rune(w); len(r) == 0 || !(unicode.IsLetter(r[0]) || r[0] == '_') {
		w = "B" + w
	}
	if r := []rune(w); len(r) > 40 {
		w = string(r[:40])
	}
	cand := w
	for i := 2; d.bmUsed[strings.ToLower(cand)]; i++ {
		suffix := strconv.Itoa(i)
		r := []rune(w)
		if len(r)+len(suffix) > 40 {
			r = r[:40-len(suffix)]
		}
		cand = string(r) + suffix
	}
	d.bmUsed[strings.ToLower(cand)] = true
	d.bmNames[name] = cand
	return cand
}

// wordStyleText is what Word gives a run from its paragraph style and
// character style alone: Word flips bold, italic, strike and small caps
// that a character style turns on.
func (d *docxOut) wordStyleText(styleID, charID string) exText {
	_, t := d.b.styleFormat(styleID)
	cm := d.b.charStyleText(charID)
	if cm == nil {
		return t
	}
	out := t
	applyText(&out, cm)
	for _, x := range []struct {
		k        string
		dst, was *bool
	}{{"bold", &out.Bold, &t.Bold}, {"italic", &out.Italic, &t.Italic}, {"strike", &out.Strike, &t.Strike}, {"smallcaps", &out.SmallCaps, &t.SmallCaps}} {
		if v, _ := cm[x.k].(bool); v {
			*x.dst = !*x.was
		} else {
			*x.dst = *x.was
		}
	}
	return out
}

// inlines writes a block's inline content, links as w:hyperlink.
func (d *docxOut) inlines(p *docxPara, content []*pmNode, styleID string) {
	open := ""
	closeLink := func() {
		if open != "" {
			p.body.WriteString(`</w:hyperlink>`)
			open = ""
		}
	}
	for _, c := range content {
		if c == nil {
			continue
		}
		href := ""
		if c.Type == "text" {
			href = linkHrefOf(c.Marks)
		}
		if href != open {
			closeLink()
			if href != "" {
				if strings.HasPrefix(href, "#") {
					fmt.Fprintf(&p.body, `<w:hyperlink w:anchor="%s" w:history="1">`, xe(d.bookmarkName(href[1:])))
				} else {
					fmt.Fprintf(&p.body, `<w:hyperlink r:id="%s" w:history="1">`, d.rel("hyperlink", uriEscape(href), true))
				}
				open = href
			}
		}
		switch c.Type {
		case "text":
			d.textRun(p, c.Text, styleID, c.Marks, nil)
		case "hard_break":
			p.body.WriteString(`<w:r><w:br/></w:r>`)
		case "tab":
			p.body.WriteString(`<w:r><w:tab/></w:r>`)
		}
	}
	closeLink()
}

// textRun writes text with its formatting; extra is applied over the
// marks (a dialogue's small caps).
func (d *docxOut) textRun(p *docxPara, text, styleID string, marks []pmMark, extra func(*exText)) {
	text = cleanXMLText(strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(text))
	if text == "" {
		return
	}
	charID := ""
	if m := markOf(marks, "char_style"); m != nil {
		id, _ := m.Attrs["id"].(string)
		if s := d.b.style(id); s != nil && s.Kind == "character" {
			charID = id
		}
	}
	want := d.b.resolvedText(styleID, marks)
	if extra != nil {
		extra(&want)
	}
	have := d.wordStyleText(styleID, charID)
	rStyle := ""
	if charID != "" {
		rStyle = d.sm.id[charID]
	}
	rpr := runPropsXML(want, have, false, rStyle)
	p.body.WriteString(`<w:r>`)
	if rpr != "" {
		p.body.WriteString(`<w:rPr>` + rpr + `</w:rPr>`)
	}
	// Tabs, soft and no-break hyphens are elements of their own.
	start := 0
	emit := func(s string) {
		if s != "" {
			p.body.WriteString(`<w:t xml:space="preserve">` + xe(s) + `</w:t>`)
		}
	}
	for i, r := range text {
		el := ""
		switch r {
		case '\t':
			el = `<w:tab/>`
		case 0x00ad:
			el = `<w:softHyphen/>`
		case 0x2011:
			el = `<w:noBreakHyphen/>`
		default:
			continue
		}
		emit(text[start:i])
		p.body.WriteString(el)
		start = i + len(string(r))
	}
	emit(text[start:])
	p.body.WriteString(`</w:r>`)
}

func (d *docxOut) dialogue(n *pmNode, lc *docxList) {
	p := d.para()
	d.textParaPPr(p, n, lc)
	styleID := d.b.paraStyleOf(n)
	speaker, lead := dialogueParts(n)
	if speaker != "" {
		d.textRun(p, speaker, styleID, nil, func(t *exText) { t.SmallCaps = true })
	}
	d.textRun(p, lead, styleID, nil, nil)
	d.inlines(p, n.Content, styleID)
	d.textRun(p, "”", styleID, nil, nil)
}

func (d *docxOut) sceneBreak(n *pmNode) {
	if src := attrString(n, "image"); src != "" {
		if pic := d.b.picture(src); pic != nil {
			p := d.para()
			p.ppr.WriteString(`<w:pStyle w:val="SceneBreak"/>`)
			if d.breakBefore() {
				p.ppr.WriteString(`<w:pageBreakBefore/>`)
			}
			p.ppr.WriteString(`<w:spacing w:line="240" w:lineRule="auto"/>`)
			w := float64(pic.W) * 0.75
			d.inlinePicture(p, pic, math.Min(w, d.textW/2), attrString(n, "glyph"))
			return
		}
	}
	p := d.para()
	p.ppr.WriteString(`<w:pStyle w:val="SceneBreak"/>`)
	if d.breakBefore() {
		p.ppr.WriteString(`<w:pageBreakBefore/>`)
	}
	d.textRun(p, sceneGlyphOf(n), "normal", nil, nil)
}

// pictureRel adds a picture to the package once.
func (d *docxOut) pictureRel(pic *exportPicture) string {
	if id, ok := d.media[pic]; ok {
		return id
	}
	d.mediaN++
	ext := strings.TrimPrefix(pic.Ext, ".")
	name := fmt.Sprintf("media/image%d.%s", d.mediaN, ext)
	d.exts[ext] = pic.ContentType
	d.parts = append(d.parts, docxPart{name: "word/" + name, data: pic.Data})
	id := d.rel("image", name, false)
	d.media[pic] = id
	return id
}

// graphicXML is a picture's DrawingML; fill paints behind it (a full
// page's transparent parts would show the header through).
func (d *docxOut) graphicXML(pic *exportPicture, rid string, cx, cy int64, crop, fill string) string {
	return fmt.Sprintf(`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic>`+
		`<pic:nvPicPr><pic:cNvPr id="0" name="image%d%s"/><pic:cNvPicPr/></pic:nvPicPr>`+
		`<pic:blipFill><a:blip r:embed="%s"/>%s<a:stretch><a:fillRect/></a:stretch></pic:blipFill>`+
		`<pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom>%s</pic:spPr>`+
		`</pic:pic></a:graphicData></a:graphic>`, d.docPr, pic.Ext, rid, crop, cx, cy, fill)
}

// inlinePicture writes a picture in the run flow, widthPt wide (kept in
// the text block).
func (d *docxOut) inlinePicture(p *docxPara, pic *exportPicture, widthPt float64, alt string) {
	if widthPt <= 0 {
		widthPt = float64(pic.W) * 0.75
	}
	widthPt = math.Min(widthPt, d.textW)
	h := widthPt * float64(pic.H) / float64(pic.W)
	if h > d.textH {
		widthPt *= d.textH / h
		h = d.textH
	}
	cx, cy := int64(math.Round(widthPt*emuPerPt)), int64(math.Round(h*emuPerPt))
	rid := d.pictureRel(pic)
	d.docPr++
	fmt.Fprintf(&p.body, `<w:r><w:drawing><wp:inline distT="0" distB="0" distL="0" distR="0"><wp:extent cx="%d" cy="%d"/><wp:effectExtent l="0" t="0" r="0" b="0"/>`+
		`<wp:docPr id="%d" name="Picture %d" descr="%s"/><wp:cNvGraphicFramePr><a:graphicFrameLocks noChangeAspect="1"/></wp:cNvGraphicFramePr>%s</wp:inline></w:drawing></w:r>`,
		cx, cy, d.docPr, d.docPr, xe(alt), d.graphicXML(pic, rid, cx, cy, "", ""))
}

func (d *docxOut) image(n *pmNode, lc *docxList) {
	pic := d.b.picture(attrString(n, "src"))
	if pic == nil {
		return
	}
	alt := attrString(n, "alt")
	if attrBool(n, "fullPage") {
		d.fullPage(pic, alt)
		return
	}
	p := d.para()
	if d.breakBefore() {
		p.ppr.WriteString(`<w:pageBreakBefore/>`)
	}
	left := 0
	if lc != nil {
		left = lc.left
	}
	align := attrString(n, "align")
	if align != "left" && align != "right" {
		align = "center"
	}
	fmt.Fprintf(&p.ppr, `<w:spacing w:before="0" w:after="0" w:line="240" w:lineRule="auto"/><w:ind w:left="%d" w:right="0" w:firstLine="0"/><w:jc w:val="%s"/>`, left, align)
	w, _ := pmNum(n.Attrs["width"])
	d.inlinePicture(p, pic, w, alt)
}

// fullPage: the picture anchored to the page at the page's size (cropped
// to its shape), in front of the text so it covers the header and footer,
// on a page of its own.
func (d *docxOut) fullPage(pic *exportPicture, alt string) {
	p := d.para()
	if d.started {
		p.ppr.WriteString(`<w:pageBreakBefore/>`)
	}
	d.breakNext = false
	d.started = true
	p.ppr.WriteString(`<w:spacing w:before="0" w:after="0" w:line="240" w:lineRule="auto"/><w:ind w:left="0" w:right="0" w:firstLine="0"/>`)
	cx, cy := int64(d.pageW)*635, int64(d.pageH)*635
	crop := ""
	pageAspect := float64(cx) / float64(cy)
	imgAspect := float64(pic.W) / float64(pic.H)
	if math.Abs(imgAspect-pageAspect) > 0.001 {
		if imgAspect > pageAspect { // wider: crop the sides
			c := int(math.Round((1 - pageAspect/imgAspect) / 2 * 100000))
			crop = fmt.Sprintf(`<a:srcRect l="%d" r="%d"/>`, c, c)
		} else {
			c := int(math.Round((1 - imgAspect/pageAspect) / 2 * 100000))
			crop = fmt.Sprintf(`<a:srcRect t="%d" b="%d"/>`, c, c)
		}
	}
	rid := d.pictureRel(pic)
	d.docPr++
	fmt.Fprintf(&p.body, `<w:r><w:drawing><wp:anchor distT="0" distB="0" distL="0" distR="0" simplePos="0" relativeHeight="%d" behindDoc="0" locked="1" layoutInCell="1" allowOverlap="1">`+
		`<wp:simplePos x="0" y="0"/><wp:positionH relativeFrom="page"><wp:posOffset>0</wp:posOffset></wp:positionH><wp:positionV relativeFrom="page"><wp:posOffset>0</wp:posOffset></wp:positionV>`+
		`<wp:extent cx="%d" cy="%d"/><wp:effectExtent l="0" t="0" r="0" b="0"/><wp:wrapNone/><wp:docPr id="%d" name="Picture %d" descr="%s"/>`+
		`<wp:cNvGraphicFramePr><a:graphicFrameLocks noChangeAspect="1"/></wp:cNvGraphicFramePr>%s</wp:anchor></w:drawing></w:r>`,
		251658240+d.docPr, cx, cy, d.docPr, d.docPr, xe(alt), d.graphicXML(pic, rid, cx, cy, crop, `<a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill>`))
	d.anchored = p
	d.breakNext = true
}

// --- lists ---

var bulletGlyphs = []string{"•", "◦", "▪"}

// listShape finds, for each depth of a list tree, whether its first list
// there is ordered and where it starts.
func listShape(n *pmNode, depth int, ordered []int, starts []int) {
	if depth >= 9 {
		return
	}
	if ordered[depth] == 0 {
		ordered[depth] = 1
		starts[depth] = 1
		if n.Type == "ordered_list" {
			ordered[depth] = 2
			if s, ok := pmInt(n.Attrs["start"]); ok && s >= 0 && s < 1e9 {
				starts[depth] = int(s)
			}
		}
	}
	for _, item := range n.Content {
		if item == nil {
			continue
		}
		for _, c := range item.Content {
			if c != nil && (c.Type == "bullet_list" || c.Type == "ordered_list") {
				listShape(c, depth+1, ordered, starts)
			}
		}
	}
}

func listLeft(ilvl int) int { return 720 * (ilvl + 1) }

func (d *docxOut) list(n *pmNode, parent *docxList) {
	var numID, ilvl int
	if parent == nil {
		ordered, starts := make([]int, 9), make([]int, 9)
		listShape(n, 0, ordered, starts)
		abs := d.numN
		d.numN++
		numID = d.numN
		fmt.Fprintf(&d.absNums, `<w:abstractNum w:abstractNumId="%d"><w:multiLevelType w:val="hybridMultilevel"/>`, abs)
		for lv := 0; lv < 9; lv++ {
			isOrdered := ordered[lv] == 2 || ordered[lv] == 0 && ordered[0] == 2
			start := starts[lv]
			if start == 0 && ordered[lv] == 0 {
				start = 1
			}
			fmt.Fprintf(&d.absNums, `<w:lvl w:ilvl="%d"><w:start w:val="%d"/>`, lv, start)
			if isOrdered {
				fmt.Fprintf(&d.absNums, `<w:numFmt w:val="decimal"/><w:lvlText w:val="%%%d."/>`, lv+1)
			} else {
				fmt.Fprintf(&d.absNums, `<w:numFmt w:val="bullet"/><w:lvlText w:val="%s"/>`, bulletGlyphs[lv%3])
			}
			fmt.Fprintf(&d.absNums, `<w:lvlJc w:val="left"/><w:pPr><w:ind w:left="%d" w:hanging="360"/></w:pPr></w:lvl>`, listLeft(lv))
		}
		d.absNums.WriteString(`</w:abstractNum>`)
		fmt.Fprintf(&d.nums, `<w:num w:numId="%d"><w:abstractNumId w:val="%d"/></w:num>`, numID, abs)
	} else {
		numID, ilvl = parent.numID, min(parent.ilvl+1, 8)
	}
	for _, item := range n.Content {
		if item == nil {
			continue
		}
		lc := &docxList{numID: numID, ilvl: ilvl, first: true, left: listLeft(ilvl)}
		for _, c := range item.Content {
			if c == nil {
				continue
			}
			switch c.Type {
			case "bullet_list", "ordered_list":
				d.list(c, lc)
			case "paragraph", "heading", "dialogue":
				d.block(c, lc)
				lc.first = false
			default:
				d.block(c, lc)
			}
		}
	}
}

// --- table of contents ---

func (d *docxOut) toc(n *pmNode) {
	d.flush("")
	d.hasTOC = true
	levels := tocLevels(n)
	title := tocTitle(n)
	d.doc.WriteString(`<w:sdt><w:sdtPr><w:docPartObj><w:docPartGallery w:val="Table of Contents"/><w:docPartUnique/></w:docPartObj></w:sdtPr><w:sdtContent>`)
	br := d.breakBefore()
	pb := ""
	if br {
		pb = `<w:pageBreakBefore/>`
	}
	if strings.TrimSpace(title) != "" {
		d.doc.WriteString(`<w:p><w:pPr><w:pStyle w:val="TOCHeading"/>` + pb + `</w:pPr><w:r><w:t xml:space="preserve">` + xe(title) + `</w:t></w:r></w:p>`)
		pb = ""
	}
	begin := fmt.Sprintf(`<w:r><w:fldChar w:fldCharType="begin" w:dirty="true"/></w:r><w:r><w:instrText xml:space="preserve"> TOC \o "1-%d" \h \z \u </w:instrText></w:r><w:r><w:fldChar w:fldCharType="separate"/></w:r>`, levels)
	end := `<w:r><w:fldChar w:fldCharType="end"/></w:r>`
	var entries []tocEntry
	for _, e := range d.b.headingsForTOC(levels) {
		if d.tocAnchor[e.node] != "" {
			entries = append(entries, e)
		}
	}
	if len(entries) == 0 {
		d.doc.WriteString(`<w:p><w:pPr><w:pStyle w:val="TOC1"/>` + pb + `</w:pPr>` + begin + end + `</w:p>`)
	}
	for i, e := range entries {
		leader := "dot"
		if !d.b.Layout.TocDots {
			leader = "none"
		}
		d.doc.WriteString(fmt.Sprintf(`<w:p><w:pPr><w:pStyle w:val="TOC%d"/>`, e.level) + pb +
			fmt.Sprintf(`<w:tabs><w:tab w:val="right" w:leader="%s" w:pos="%d"/></w:tabs><w:ind w:left="%d" w:firstLine="0"/></w:pPr>`,
				leader, twips(d.textW), twips(d.b.bodySize()*1.5*float64(e.level-1))))
		pb = ""
		if i == 0 {
			d.doc.WriteString(begin)
		}
		fmt.Fprintf(&d.doc, `<w:hyperlink w:anchor="%s" w:history="1"><w:r><w:t xml:space="preserve">%s</w:t></w:r></w:hyperlink>`, d.tocAnchor[e.node], xe(e.text))
		if i == len(entries)-1 {
			d.doc.WriteString(end)
		}
		d.doc.WriteString(`</w:p>`)
	}
	d.doc.WriteString(`</w:sdtContent></w:sdt>`)
}

// --- sections, headers and footers ---

// sectPrFor writes chapter ci's section (-1: an empty book).
func (d *docxOut) sectPrFor(ci int) string {
	b := d.b
	L := b.Layout
	var ch exportChapter
	if ci >= 0 {
		ch = b.Chapters[ci]
	}
	var s strings.Builder
	s.WriteString(`<w:sectPr>`)
	if !b.online() {
		types := []struct{ slot, band string }{{"default", "odd"}}
		if L.DifferentOddEven {
			types = append(types, struct{ slot, band string }{"even", "even"})
		}
		if L.DifferentFirst {
			types = append(types, struct{ slot, band string }{"first", "first"})
		}
		for _, kind := range []string{"header", "footer"} {
			for _, t := range types {
				band := L.Footer[t.band]
				if kind == "header" {
					band = L.Header[t.band]
					if ch.FrontMatter || ch.NoHeader {
						band = exportBand{}
					}
				}
				style := "Header"
				if kind == "footer" {
					style = "Footer"
				}
				content := d.bandXML(band, ch, style)
				key := kind + "/" + t.slot
				if prev, ok := d.slots[key]; ok && prev == content {
					continue // Word links it to the previous section's
				}
				d.slots[key] = content
				d.hfN++
				tag := "hdr"
				if kind == "footer" {
					tag = "ftr"
				}
				name := fmt.Sprintf("%s%d.xml", kind, d.hfN)
				d.parts = append(d.parts, docxPart{name: "word/" + name,
					contentType: "application/vnd.openxmlformats-officedocument.wordprocessingml." + kind + "+xml",
					data:        []byte(xmlHead + `<w:` + tag + ` ` + docxNSDecl + `>` + content + `</w:` + tag + `>`)})
				rid := d.rel(kind, name, false)
				fmt.Fprintf(&s, `<w:%sReference w:type="%s" r:id="%s"/>`, kind, t.slot, rid)
			}
		}
	}
	typ := "nextPage"
	if L.ChapterStart == "odd" && !b.online() {
		typ = "oddPage"
	}
	fmt.Fprintf(&s, `<w:type w:val="%s"/>`, typ)
	orient := ""
	if d.pageW > d.pageH {
		orient = ` w:orient="landscape"`
	}
	fmt.Fprintf(&s, `<w:pgSz w:w="%d" w:h="%d"%s/>`, d.pageW, d.pageH, orient)
	pg := b.Page
	top, bottom := mmTwips(pg.MarginTopMM), mmTwips(pg.MarginBottomMM)
	fmt.Fprintf(&s, `<w:pgMar w:top="%d" w:right="%d" w:bottom="%d" w:left="%d" w:header="%d" w:footer="%d" w:gutter="%d"/>`,
		top, mmTwips(pg.MarginOuterMM), bottom, mmTwips(pg.MarginInnerMM), max(120, top*9/20), max(120, bottom*9/20), mmTwips(pg.GutterMM))
	if !b.online() && ci >= 0 {
		var attrs string
		if ch.FrontMatter {
			attrs = ` w:fmt="lowerRoman"`
			if ci == 0 || !b.Chapters[ci-1].FrontMatter {
				attrs += ` w:start="1"`
			}
		} else {
			attrs = fmt.Sprintf(` w:fmt="%s"`, pageNumFormats[L.NumFormat])
			firstBody := true
			for j := 0; j < ci; j++ {
				if !b.Chapters[j].FrontMatter {
					firstBody = false
				}
			}
			if ch.NumberRestart != nil {
				attrs += fmt.Sprintf(` w:start="%d"`, *ch.NumberRestart)
			} else if firstBody {
				attrs += fmt.Sprintf(` w:start="%d"`, L.NumStart)
			}
		}
		s.WriteString(`<w:pgNumType` + attrs + `/>`)
	}
	if L.DifferentFirst && !b.online() {
		s.WriteString(`<w:titlePg/>`)
	}
	s.WriteString(`</w:sectPr>`)
	return s.String()
}

// bandXML is a header or footer's paragraph: left, tab, centre, tab, right.
func (d *docxOut) bandXML(band exportBand, ch exportChapter, style string) string {
	var rpr strings.Builder
	if f := strings.TrimSpace(band.Font); f != "" {
		f = xe(f)
		fmt.Fprintf(&rpr, `<w:rFonts w:ascii="%s" w:hAnsi="%s" w:eastAsia="%s" w:cs="%s"/>`, f, f, f, f)
	}
	if band.Italic {
		rpr.WriteString(`<w:i/><w:iCs/>`)
	}
	if band.Uppercase {
		rpr.WriteString(`<w:caps/>`)
	}
	if band.SmallCaps && !band.Uppercase {
		rpr.WriteString(`<w:smallCaps/>`)
	}
	if band.Size > 0 {
		hp := int(math.Round(band.Size * 2))
		fmt.Fprintf(&rpr, `<w:sz w:val="%d"/><w:szCs w:val="%d"/>`, hp, hp)
	}
	rp := ""
	if rpr.Len() > 0 {
		rp = `<w:rPr>` + rpr.String() + `</w:rPr>`
	}
	tw := twips(d.textW)
	head := fmt.Sprintf(`<w:p><w:pPr><w:pStyle w:val="%s"/><w:tabs><w:tab w:val="center" w:pos="%d"/><w:tab w:val="right" w:pos="%d"/></w:tabs>`+
		`<w:spacing w:before="0" w:after="0" w:line="240" w:lineRule="auto"/><w:ind w:left="0" w:right="0" w:firstLine="0"/><w:jc w:val="left"/></w:pPr>`, style, tw/2, tw)
	if band.empty() {
		return head + `</w:p>`
	}
	var s strings.Builder
	s.WriteString(head)
	parts := []string{band.Left, band.Center, band.Right}
	for i, text := range parts {
		if i > 0 {
			s.WriteString(`<w:r>` + rp + `<w:tab/></w:r>`)
		}
		for _, seg := range d.bandSegments(text, ch) {
			if seg.page {
				s.WriteString(`<w:fldSimple w:instr=" PAGE "><w:r>` + rp + `<w:t>1</w:t></w:r></w:fldSimple>`)
			} else if seg.text != "" {
				s.WriteString(`<w:r>` + rp + `<w:t xml:space="preserve">` + xe(seg.text) + `</w:t></w:r>`)
			}
		}
	}
	s.WriteString(`</w:p>`)
	return s.String()
}

type bandSeg struct {
	text string
	page bool
}

// bandSegments fills a band's fields; {page} stays a field.
func (d *docxOut) bandSegments(text string, ch exportChapter) []bandSeg {
	return fillBandFields(text, d.b, ch)
}

func fillBandFields(text string, b *exportBook, ch exportChapter) []bandSeg {
	var out []bandSeg
	var cur strings.Builder
	for len(text) > 0 {
		i := strings.IndexByte(text, '{')
		if i < 0 {
			cur.WriteString(text)
			break
		}
		cur.WriteString(text[:i])
		text = text[i:]
		j := strings.IndexByte(text, '}')
		if j < 0 {
			cur.WriteString(text)
			break
		}
		field := text[1:j]
		text = text[j+1:]
		switch field {
		case "page":
			out = append(out, bandSeg{text: cur.String()}, bandSeg{page: true})
			cur.Reset()
		case "chapter":
			cur.WriteString(ch.Title)
		case "chapter_number":
			if ch.Number > 0 {
				cur.WriteString(strconv.Itoa(ch.Number))
			}
		case "book":
			cur.WriteString(b.BookTitle())
		case "author":
			cur.WriteString(b.Meta.Author)
		default:
			cur.WriteString("{" + field + "}")
		}
	}
	out = append(out, bandSeg{text: cur.String()})
	return out
}

// --- settings and properties ---

func (d *docxOut) settingsXML() []byte {
	b := d.b
	var s strings.Builder
	s.WriteString(xmlHead + `<w:settings ` + docxNSDecl + `>`)
	if b.Page.Mirror && !b.online() {
		s.WriteString(`<w:mirrorMargins/>`)
	}
	s.WriteString(`<w:defaultTabStop w:val="720"/>`)
	if b.Layout.DifferentOddEven && !b.online() {
		s.WriteString(`<w:evenAndOddHeaders/>`)
	}
	s.WriteString(`<w:characterSpacingControl w:val="doNotCompress"/>`)
	if d.hasTOC {
		s.WriteString(`<w:updateFields w:val="true"/>`)
	}
	s.WriteString(`<w:compat><w:compatSetting w:name="compatibilityMode" w:uri="http://schemas.microsoft.com/office/word" w:val="15"/></w:compat>`)
	fmt.Fprintf(&s, `<w:themeFontLang w:val="%s"/>`, xe(b.Meta.Language))
	s.WriteString(`</w:settings>`)
	return []byte(s.String())
}

func (d *docxOut) coreXML() []byte {
	m := d.b.Meta
	var s strings.Builder
	s.WriteString(xmlHead + `<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" ` +
		`xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" ` +
		`xmlns:dcmitype="http://purl.org/dc/dcmitype/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">`)
	el := func(tag, v string) {
		if strings.TrimSpace(v) != "" {
			fmt.Fprintf(&s, `<%s>%s</%s>`, tag, xe(v), tag)
		}
	}
	el("dc:title", d.b.BookTitle())
	el("dc:subject", m.Subtitle)
	el("dc:creator", m.Author)
	el("dc:description", m.Description)
	el("dc:identifier", m.ISBN)
	el("dc:language", m.Language)
	now := editorNow().UTC().Format("2006-01-02T15:04:05Z")
	fmt.Fprintf(&s, `<dcterms:created xsi:type="dcterms:W3CDTF">%s</dcterms:created><dcterms:modified xsi:type="dcterms:W3CDTF">%s</dcterms:modified>`, now, now)
	s.WriteString(`</cp:coreProperties>`)
	return []byte(s.String())
}

func (d *docxOut) appXML() []byte {
	words := 0
	for _, c := range d.b.Chapters {
		words += docWords(c.Doc)
	}
	var s strings.Builder
	s.WriteString(xmlHead + `<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties" ` +
		`xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes"><Application>Polaris</Application>`)
	fmt.Fprintf(&s, `<Words>%d</Words>`, words)
	if p := strings.TrimSpace(d.b.Meta.Publisher); p != "" {
		fmt.Fprintf(&s, `<Company>%s</Company>`, xe(p))
	}
	s.WriteString(`</Properties>`)
	return []byte(s.String())
}
