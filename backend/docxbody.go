package main

// The .docx body walk (docx.go): w:body into a flat list of blocks
// (irBlock) with fully resolved formatting, before styles are worked out
// and chapters split (docxemit.go).
//
// Mapping:
//   - w:p: a paragraph (a heading when its outline level is 0–5, from its
//     style chain or set directly, or when its style is a "heading N").
//     pageBreakBefore puts a page break before it. A w:br type=page splits
//     it: page break between the parts, empty parts dropped (a paragraph
//     holding only a page break is just the page break). A paragraph that
//     is empty and only ends a section is just the section's page break.
//     Other empty paragraphs are kept. A paragraph whose content was all
//     dropped (a table of contents, a page number field, hidden text) is
//     dropped.
//   - Numbered paragraphs (numPr, directly or from the style; numbering.xml)
//     become list items: "bullet" levels a bullet_list, others an
//     ordered_list whose start continues Word's count. A deeper level (ilvl)
//     is a list nested in the item before it (EDITOR.md's list_item holds a
//     paragraph, then blocks), up to Word's nine levels; a deeper level with
//     no item before it opens the list there. The list's own indents are
//     dropped. A numbered heading keeps its
//     heading and gets its number written in front ("Chapter 3"), as Word
//     shows it.
//   - A run's character style (w:rStyle) is kept by name (irInline.cstyle)
//     beside the formatting the run ends up with; emitting turns it into a
//     `char_style` mark and leaves only the direct formatting as marks
//     (docxemit.go).
//   - Runs: w:t text, w:tab and w:ptab a tab, w:br (not page) and w:cr a
//     hard break, w:sym its character in its font, no-break and soft
//     hyphens as U+2011 and U+00AD. Hidden text (vanish) is dropped. All
//     caps (w:caps) is written as capitals: the text is what readers see.
//   - w:hyperlink: an external target becomes a link mark (web, mail and
//     phone links only), w:anchor a link to "#anchor".
//   - Bookmarks: a paragraph keeps the first bookmark that starts in it (or
//     just before it) as its `bookmark` attr. Word's hidden ones (names
//     starting with "_", like _Toc… and _GoBack) are kept only when a kept
//     link points at them.
//   - Fields (complex and w:fldSimple): their shown result is kept as text,
//     except TOC, TOA, INDEX, PAGE, NUMPAGES, SECTIONPAGES, SECTION,
//     PAGEREF, NOTEREF and the entry fields (XE, TC, TA, RD), which are
//     dropped. HYPERLINK fields become links.
//   - w:sdt content controls are read through, except a table of contents
//     (docPartGallery "Table of Contents"), dropped whole.
//   - Tracked changes: w:ins and w:moveTo kept, w:del and w:moveFrom
//     dropped. proofErr, rsids, permissions and other noise are ignored.
//   - mc:AlternateContent: the Choice when it holds a picture or a text box
//     this reader understands, else the Fallback.
//   - Pictures (DrawingML pic:pic, inline or anchored, and VML imagedata)
//     become image blocks: an inline one splits its paragraph where it
//     sits, an anchored one goes right after its paragraph. width is the
//     picture's size in pt; align from wp:align or where it sits, an inline
//     one the paragraph's alignment. An anchored picture placed relative to
//     the page and at least 85% of the page's width and height is fullPage
//     (a cover); nothing else is guessed (an ornament used between scenes
//     stays a picture).
//   - Tables: their cells' paragraphs in reading order. Text boxes: their
//     paragraphs after the paragraph they are anchored in.

import (
	"path"
	"strconv"
	"strings"
)

type irInline struct {
	kind byte // 't' text, '\t' tab, 'n' hard break
	text string
	r    rFmt // as the text looks, character style included
	link string
	// The Word character style id the run uses ("" = none): emitting turns
	// it into a char_style mark and keeps only the direct formatting as marks.
	cstyle string
	// The comments on the text: their locals, "1,3" (docxcomments.go).
	comments string
}

type irListRef struct {
	numID, ilvl int
	ordered     bool
	value       int
}

type irBlock struct {
	kind      byte   // 'p' paragraph or heading, 'i' image, 'b' page break
	style     string // Word paragraph style id
	level     int    // heading level, 0 = not a heading
	p         pFmt
	inl       []irInline
	list      *irListRef
	bookmarks []string
	section   int
	scene     string // set when a paragraph is a scene break (its glyph)

	pic      int
	width    float64 // pt, 0 = natural size
	align    string
	fullPage bool
	alt      string
}

type docxField struct {
	instr   strings.Builder
	result  bool // past the separator: showing the result
	decided bool
	drop    bool
	inert   bool // inside something dropped: not counted
	href    string
}

type docxParser struct {
	pk     *docxPackage
	rels   map[string]docxRel
	st     *wStyles
	num    *wNumbering
	th     docxTheme
	mirror bool
	err    error

	blocks      []*irBlock
	out         *[]*irBlock
	pics        []docxPicture
	picByTarget map[string]int
	leftOut     map[string]int
	fields      []*docxField
	sects       []*xEl
	sect        int
	pendingMark []string
	linkAnchors map[string]bool
	hfParts     map[string]bool
	counters    map[int][]int
	usedChar    map[string]bool
	tocGap      bool // a table of contents was just dropped

	// Comments (docxcomments.go): the file's, the ones whose range is open,
	// and point comments waiting for the next word.
	cm        *docxComments
	openCm    []*docxCmt
	pendingCm []*docxCmt
}

func newDocxParser(pk *docxPackage, rels map[string]docxRel, st *wStyles, num *wNumbering, th docxTheme) *docxParser {
	p := &docxParser{pk: pk, rels: rels, st: st, num: num, th: th,
		picByTarget: map[string]int{}, leftOut: map[string]int{}, linkAnchors: map[string]bool{},
		hfParts: map[string]bool{}, counters: map[int][]int{}, usedChar: map[string]bool{}}
	p.out = &p.blocks
	return p
}

func (p *docxParser) count(key string) { p.leftOut[key]++ }

func (p *docxParser) parseBody(body *xEl) error {
	// Every section's properties, in order: the ones ending sections inside
	// the text, then the body's own (the last section).
	body.walk(func(e *xEl) bool {
		if e.is("w", "pPr") {
			if s := e.child("w", "sectPr"); s != nil {
				p.sects = append(p.sects, s)
			}
			return false
		}
		return !e.is("w", "txbxContent")
	})
	if s := body.child("w", "sectPr"); s != nil {
		p.sects = append(p.sects, s)
	}
	for _, s := range p.sects {
		for _, ref := range s.kids {
			if ref.is("w", "headerReference") || ref.is("w", "footerReference") {
				if rel, ok := p.rels[ref.rattr("id")]; ok {
					p.hfParts[rel.target] = true
				}
			}
		}
		if n, ok := intAttr(s.child("w", "cols"), "num"); ok && n > 1 {
			p.count("columns")
		}
	}
	if len(p.sects) > 1 {
		p.leftOut["sections"] = len(p.sects) - 1
	}
	p.walkBlocks(body.kids)
	return p.err
}

// emit adds a block. A dropped table of contents leaves the page breaks
// around its page; the first one after it is dropped when a page break
// already comes before it, so no blank page is left in its place.
func (p *docxParser) emit(b *irBlock) {
	if p.tocGap {
		if b.kind == 'b' {
			p.tocGap = false
			if n := len(*p.out); n > 0 && (*p.out)[n-1].kind == 'b' {
				return
			}
		} else {
			p.tocGap = false
		}
	}
	*p.out = append(*p.out, b)
}

// chooseAlternate picks what to read of an mc:AlternateContent.
func chooseAlternate(ac *xEl) []*xEl {
	var choice, fallback *xEl
	for _, k := range ac.kids {
		if k.is("mc", "Choice") && choice == nil {
			choice = k
		} else if k.is("mc", "Fallback") {
			fallback = k
		}
	}
	if choice != nil {
		if gd := choice.find("a", "graphicData"); gd != nil && gd.child("pic", "pic") != nil {
			return choice.kids
		}
		if choice.find("w", "txbxContent") != nil {
			return choice.kids
		}
	}
	if fallback != nil {
		return fallback.kids
	}
	if choice != nil {
		return choice.kids
	}
	return nil
}

func isTOCSdt(sdt *xEl) bool {
	g := sdt.child("w", "sdtPr").child("w", "docPartObj").child("w", "docPartGallery")
	return g != nil && strings.HasPrefix(strings.ToLower(g.attr("val")), "table of contents")
}

func (p *docxParser) walkBlocks(kids []*xEl) {
	for _, k := range kids {
		if p.err != nil {
			return
		}
		switch {
		case k.is("w", "p"):
			p.paragraph(k)
		case k.is("w", "tbl"):
			p.count("tables")
			p.table(k)
		case k.is("w", "sdt"):
			if isTOCSdt(k) {
				p.count("toc")
				p.skipped(k)
				p.tocGap = true
				continue
			}
			p.walkBlocks(k.child("w", "sdtContent").kids)
		case k.is("w", "customXml"), k.is("w", "smartTag"):
			p.walkBlocks(k.kids)
		case k.is("w", "ins"), k.is("w", "moveTo"):
			p.count("tracked_changes")
			p.walkBlocks(k.kids)
		case k.is("w", "del"), k.is("w", "moveFrom"):
			p.count("tracked_changes")
			p.skipped(k)
		case k.is("w", "bookmarkStart"):
			p.pendingMark = append(p.pendingMark, k.attr("name"))
		case k.is("w", "commentRangeStart"):
			p.commentStart(k.attr("id"))
		case k.is("w", "commentRangeEnd"):
			p.commentEnd(k.attr("id"), nil)
		case k.is("mc", "AlternateContent"):
			p.walkBlocks(chooseAlternate(k))
		case k.is("w", "altChunk"):
			p.count("objects")
		case k.is("m", "oMathPara"), k.is("m", "oMath"):
			p.count("equations")
		}
	}
}

// skipped keeps the section count right across dropped content.
func (p *docxParser) skipped(e *xEl) {
	e.walk(func(x *xEl) bool {
		if x.is("w", "pPr") {
			if x.child("w", "sectPr") != nil {
				p.endSection()
			}
			return false
		}
		return true
	})
}

// endSection ends the current section; a break to a new page becomes a
// page break.
func (p *docxParser) endSection() {
	p.sect++
	if p.sect < len(p.sects) {
		switch p.sects[p.sect].child("w", "type").attr("val") {
		case "continuous", "nextColumn":
			return
		}
	}
	p.emit(&irBlock{kind: 'b', section: p.sect - 1})
}

func (p *docxParser) table(e *xEl) {
	for _, k := range e.kids {
		switch {
		case k.is("w", "tr"), k.is("w", "customXml"), k.is("w", "ins"):
			p.table(k)
		case k.is("w", "sdt"):
			p.table(k.child("w", "sdtContent"))
		case k.is("w", "tc"):
			p.walkBlocks(k.kids)
		case k.is("w", "del"):
			p.count("tracked_changes")
		}
	}
}

// --- paragraphs ---

type paraState struct {
	tmpl       irBlock // what every part of the paragraph shares
	pStyle     string
	cur        []irInline
	items      []*irBlock
	after      []*irBlock
	link       string
	suppressed bool // some content was dropped
	bookmarks  []string
	breakOnly  bool // ends a section
}

func (p *docxParser) paragraph(el *xEl) {
	pPr := el.child("w", "pPr")
	styleID := p.st.paraStyleID(pPr.child("w", "pStyle").attr("val"))
	direct := parsePPr(pPr)
	eff := p.st.para(styleID)
	eff.over(direct)
	if direct.hasBorders || direct.hasShade {
		p.count("borders_shading")
	}
	if direct.isDrop {
		p.count("drop_caps")
	}

	ps := &paraState{pStyle: styleID, bookmarks: p.pendingMark}
	p.pendingMark = nil
	ps.tmpl = irBlock{kind: 'p', style: styleID, p: concretePara(eff), section: p.sect}

	// Heading level: the outline level, else a "heading N" style.
	if eff.outline != nil {
		if *eff.outline >= 0 && *eff.outline <= 5 {
			ps.tmpl.level = *eff.outline + 1
		}
	} else if lv := p.st.headingLevel(styleID); lv >= 1 && lv <= 6 {
		ps.tmpl.level = lv
	}

	label := ""
	if eff.numID != nil && *eff.numID > 0 {
		ilvl := 0
		if eff.ilvl != nil {
			ilvl = max(0, min(*eff.ilvl, 8))
		}
		if lvl := p.num.level(*eff.numID, ilvl); lvl != nil && lvl.format != "none" {
			value := p.countItem(*eff.numID, ilvl, lvl.start)
			if ps.tmpl.level > 0 {
				label = p.numberText(*eff.numID, ilvl)
				switch lvl.suff {
				case "space":
					label += " "
				case "nothing":
				default:
					label += "\t"
				}
			} else {
				ps.tmpl.list = &irListRef{numID: *eff.numID, ilvl: ilvl, ordered: lvl.format != "bullet", value: value}
			}
		}
	}

	if eff.pageBreakBefore != nil && *eff.pageBreakBefore {
		ps.items = append(ps.items, &irBlock{kind: 'b', section: p.sect})
	}

	p.inlines(el.kids, ps)
	p.flush(ps)

	if label != "" {
		for _, it := range ps.items {
			if it.kind == 'p' {
				r := concreteRun(p.st.run(styleID, ""), label)
				for _, in := range it.inl {
					if in.kind == 't' {
						r = in.r
						break
					}
				}
				head := []irInline{{kind: 't', text: strings.TrimSuffix(label, "\t"), r: r}}
				if strings.HasSuffix(label, "\t") {
					head = append(head, irInline{kind: '\t', r: r})
				}
				it.inl = append(head, it.inl...)
				break
			}
		}
	}

	ps.breakOnly = pPr.child("w", "sectPr") != nil
	p.finishParagraph(ps)
	if ps.breakOnly {
		p.endSection()
	}
}

// countItem advances Word's list counter for a numbered paragraph.
func (p *docxParser) countItem(numID, ilvl, start int) int {
	c := p.counters[numID]
	if c == nil {
		c = make([]int, 9)
		for i := range c {
			c[i] = -1 << 30
		}
		p.counters[numID] = c
	}
	if c[ilvl] == -1<<30 {
		c[ilvl] = start - 1
	}
	c[ilvl]++
	for i := ilvl + 1; i < 9; i++ {
		c[i] = -1 << 30
	}
	return c[ilvl]
}

// numberText is a level's lvlText with its %1…%9 filled in.
func (p *docxParser) numberText(numID, ilvl int) string {
	lvl := p.num.level(numID, ilvl)
	c := p.counters[numID]
	text := lvl.text
	for i := 9; i >= 1; i-- {
		ph := "%" + strconv.Itoa(i)
		if !strings.Contains(text, ph) {
			continue
		}
		v := 1
		f := "decimal"
		if c != nil && c[i-1] != -1<<30 {
			v = c[i-1]
		}
		if l := p.num.level(numID, i-1); l != nil {
			f = l.format
			if c == nil || c[i-1] == -1<<30 {
				v = l.start
			}
		}
		text = strings.ReplaceAll(text, ph, numberLabel(f, v))
	}
	return text
}

// flush closes the current part of a paragraph.
func (p *docxParser) flush(ps *paraState) {
	b := ps.tmpl
	b.inl = ps.cur
	ps.cur = nil
	ps.items = append(ps.items, &b)
}

func (p *docxParser) finishParagraph(ps *paraState) {
	something := len(ps.after) > 0
	for _, it := range ps.items {
		if it.kind != 'p' || len(it.inl) > 0 {
			something = true
		}
	}
	var items []*irBlock
	for _, it := range ps.items {
		if it.kind == 'p' && len(it.inl) == 0 && (something || ps.suppressed || ps.breakOnly) {
			continue
		}
		items = append(items, it)
	}
	// Bookmarks go on the first paragraph part; with none, on to the next
	// paragraph.
	placed := false
	for _, it := range items {
		if it.kind == 'p' {
			it.bookmarks = ps.bookmarks
			placed = true
			break
		}
	}
	if !placed {
		p.pendingMark = append(ps.bookmarks, p.pendingMark...)
	}
	for _, it := range items {
		p.emit(it)
	}
	for _, it := range ps.after {
		p.emit(it)
	}
}

// --- fields ---

var droppedFields = map[string]bool{"TOC": true, "TOA": true, "INDEX": true, "PAGE": true, "NUMPAGES": true,
	"SECTIONPAGES": true, "SECTION": true, "PAGEREF": true, "NOTEREF": true, "XE": true, "TC": true, "TA": true, "RD": true}

func (p *docxParser) hidingField() bool {
	for _, f := range p.fields {
		if !f.result || f.drop {
			return true
		}
	}
	return false
}

func (p *docxParser) decideField(f *docxField) {
	if f.decided {
		return
	}
	f.decided = true
	words := splitFieldInstr(f.instr.String())
	if len(words) == 0 {
		return
	}
	kind := strings.ToUpper(words[0])
	switch {
	case kind == "TOC":
		f.drop = true
		if !f.inert {
			p.count("toc")
			p.tocGap = true
		}
	case droppedFields[kind]:
		f.drop = true
		if !f.inert {
			p.count("fields")
		}
	case kind == "HYPERLINK":
		target, anchor := "", ""
		for i := 1; i < len(words); i++ {
			switch w := words[i]; {
			case strings.EqualFold(w, `\l`) && i+1 < len(words):
				anchor = words[i+1]
				i++
			case strings.HasPrefix(w, `\`):
				if i+1 < len(words) && !strings.HasPrefix(words[i+1], `\`) && (strings.EqualFold(w, `\o`) || strings.EqualFold(w, `\t`)) {
					i++
				}
			case target == "":
				target = w
			}
		}
		f.href = linkHref(target, anchor)
	default:
		if !f.inert {
			p.count("fields_as_text")
		}
	}
}

// splitFieldInstr splits a field code into words, "quoted text" as one.
func splitFieldInstr(s string) []string {
	var out []string
	var b strings.Builder
	inQ := false
	for _, r := range s {
		switch {
		case r == '"':
			if inQ {
				out = append(out, b.String())
				b.Reset()
			}
			inQ = !inQ
		case !inQ && (r == ' ' || r == '\t' || r == '\n' || r == '\r'):
			if b.Len() > 0 {
				out = append(out, b.String())
				b.Reset()
			}
		default:
			b.WriteRune(r)
		}
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out
}

// linkHref makes a link mark's href from a target and/or an anchor; ""
// when it isn't one EDITOR.md allows.
func linkHref(target, anchor string) string {
	target = strings.TrimSpace(target)
	anchor = strings.TrimSpace(anchor)
	if target == "" {
		if anchor == "" {
			return ""
		}
		return "#" + anchor
	}
	if anchor != "" {
		target += "#" + anchor
	}
	m := pmSchemeRe.FindStringSubmatch(target)
	if m == nil {
		return "" // a file on the writer's computer
	}
	if !safeLinkHref(target) {
		return ""
	}
	return target
}

func (p *docxParser) fieldLink() string {
	for i := len(p.fields) - 1; i >= 0; i-- {
		if p.fields[i].href != "" && p.fields[i].result {
			return p.fields[i].href
		}
	}
	return ""
}

// --- inline content ---

func (p *docxParser) inlines(kids []*xEl, ps *paraState) {
	for _, k := range kids {
		if p.err != nil {
			return
		}
		switch {
		case k.is("w", "r"):
			p.run(k, ps)
		case k.is("w", "hyperlink"):
			href := ""
			if id := k.rattr("id"); id != "" {
				if rel, ok := p.rels[id]; ok && rel.external {
					href = linkHref(rel.target, k.attr("anchor"))
				}
			} else if a := k.attr("anchor"); a != "" {
				href = linkHref("", a)
			}
			saved := ps.link
			if href != "" {
				ps.link = href
			}
			p.inlines(k.kids, ps)
			ps.link = saved
		case k.is("w", "fldSimple"):
			f := &docxField{inert: p.hidingField()}
			f.instr.WriteString(k.attr("instr"))
			f.result = true
			p.decideField(f)
			ps.suppressed = true
			p.fields = append(p.fields, f)
			p.inlines(k.kids, ps)
			p.fields = p.fields[:len(p.fields)-1]
		case k.is("w", "sdt"):
			p.inlines(k.child("w", "sdtContent").kids, ps)
		case k.is("w", "smartTag"), k.is("w", "customXml"), k.is("w", "dir"), k.is("w", "bdo"):
			p.inlines(k.kids, ps)
		case k.is("w", "ins"), k.is("w", "moveTo"):
			p.count("tracked_changes")
			p.inlines(k.kids, ps)
		case k.is("w", "del"), k.is("w", "moveFrom"):
			p.count("tracked_changes")
			p.deletedFields(k)
		case k.is("w", "bookmarkStart"):
			ps.bookmarks = append(ps.bookmarks, k.attr("name"))
		case k.is("w", "commentRangeStart"):
			p.commentStart(k.attr("id"))
		case k.is("w", "commentRangeEnd"):
			p.commentEnd(k.attr("id"), ps)
		case k.is("m", "oMath"), k.is("m", "oMathPara"):
			p.count("equations")
			ps.suppressed = true
		case k.is("mc", "AlternateContent"):
			p.inlines(chooseAlternate(k), ps)
		case k.is("w", "subDoc"):
			p.count("objects")
		}
	}
}

// deletedFields: a deleted run can still hold a field's begin or end; keep
// the field stack balanced.
func (p *docxParser) deletedFields(e *xEl) {
	e.walk(func(x *xEl) bool {
		if x.is("w", "fldChar") {
			p.fieldChar(x.attr("fldCharType"))
		}
		return true
	})
}

func (p *docxParser) fieldChar(kind string) {
	switch kind {
	case "begin":
		p.fields = append(p.fields, &docxField{inert: p.hidingField()})
	case "separate":
		if n := len(p.fields); n > 0 {
			f := p.fields[n-1]
			p.decideField(f)
			f.result = true
		}
	case "end":
		if n := len(p.fields); n > 0 {
			p.decideField(p.fields[n-1])
			p.fields = p.fields[:n-1]
		}
	}
}

func (p *docxParser) run(el *xEl, ps *paraState) {
	rPr := el.child("w", "rPr")
	charStyle := rPr.child("w", "rStyle").attr("val")
	wr := p.st.run(ps.pStyle, charStyle)
	wr.over(parseRPr(rPr, p.th))
	hidden := wr.vanish != nil && *wr.vanish
	p.runContent(el.kids, ps, wr, charStyle, hidden)
}

func (p *docxParser) runContent(kids []*xEl, ps *paraState, wr wRun, charStyle string, hidden bool) {
	countedHidden := false
	for _, k := range kids {
		if k.ns == "w" {
			switch k.name {
			case "fldChar":
				p.fieldChar(k.attr("fldCharType"))
				ps.suppressed = true
				continue
			case "instrText":
				if n := len(p.fields); n > 0 && !p.fields[n-1].result {
					p.fields[n-1].instr.WriteString(k.text)
				}
				continue
			case "rPr", "delText", "delInstrText", "lastRenderedPageBreak", "annotationRef", "proofErr":
				continue
			case "footnoteReference":
				p.count("footnotes")
				continue
			case "endnoteReference":
				p.count("endnotes")
				continue
			case "commentReference":
				p.commentRef(k.attr("id"), ps) // docxcomments.go
				continue
			}
		}
		if p.hidingField() {
			ps.suppressed = true
			continue
		}
		if hidden {
			if !countedHidden && (k.is("w", "t") || k.is("w", "drawing")) {
				p.count("hidden_text")
				countedHidden = true
			}
			ps.suppressed = true
			continue
		}
		switch {
		case k.is("w", "t"):
			p.addText(ps, k.text, wr, charStyle)
		case k.is("w", "tab"), k.is("w", "ptab"):
			p.addInline(ps, irInline{kind: '\t', r: concreteRun(wr, " ")})
		case k.is("w", "br"):
			if k.attr("type") == "page" {
				p.flush(ps)
				ps.items = append(ps.items, &irBlock{kind: 'b', section: p.sect})
			} else {
				p.addInline(ps, irInline{kind: 'n', r: concreteRun(wr, " ")})
			}
		case k.is("w", "cr"):
			p.addInline(ps, irInline{kind: 'n', r: concreteRun(wr, " ")})
		case k.is("w", "sym"):
			code, err := strconv.ParseUint(k.attr("char"), 16, 32)
			if err == nil && code > 0 {
				sw := wr
				if f := k.attr("font"); f != "" {
					sw.asciiFont, sw.hAnsiFont = ptrS(f), ptrS(f)
				}
				p.addText(ps, string(rune(code)), sw, charStyle)
			}
		case k.is("w", "noBreakHyphen"):
			p.addText(ps, string(rune(0x2011)), wr, charStyle) // non-breaking hyphen
		case k.is("w", "softHyphen"):
			p.addText(ps, string(rune(0x00ad)), wr, charStyle) // soft hyphen
		case k.is("w", "drawing"):
			p.drawing(k, ps)
		case k.is("w", "pict"):
			p.vml(k, ps)
		case k.is("w", "object"), k.is("w", "embedded"):
			p.count("objects")
			ps.suppressed = true
		case k.is("w", "ruby"):
			if base := k.child("w", "rubyBase"); base != nil {
				p.inlines(base.kids, ps)
			}
		case k.is("mc", "AlternateContent"):
			p.runContent(chooseAlternate(k), ps, wr, charStyle, hidden)
		}
	}
}

func (p *docxParser) addText(ps *paraState, text string, wr wRun, charStyle string) {
	if text == "" {
		return
	}
	text = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(text)
	r := concreteRun(wr, text)
	if r.caps {
		text = strings.ToUpper(text)
	}
	cs := ""
	if charStyle != "" && charStyle != p.st.defaultChar {
		p.usedChar[charStyle] = true
		if st := p.st.byID[charStyle]; st != nil && st.kind == "character" {
			cs = charStyle
		}
	}
	in := irInline{kind: 't', text: text, r: r, cstyle: cs}
	if len(p.pendingCm) > 0 {
		p.addTextWithPoints(ps, in) // docxcomments.go
		return
	}
	p.addInline(ps, in)
}

func (p *docxParser) addInline(ps *paraState, in irInline) {
	in.link = ps.link
	if fl := p.fieldLink(); fl != "" {
		in.link = fl
	}
	if strings.HasPrefix(in.link, "#") {
		p.linkAnchors[in.link[1:]] = true
	}
	if in.kind == 't' {
		in.comments = unionCommentKeys(in.comments, p.commentKey())
		p.markCommented(in.comments)
	}
	if n := len(ps.cur); n > 0 && in.kind == 't' {
		last := &ps.cur[n-1]
		if last.kind == 't' && last.r == in.r && last.link == in.link && last.cstyle == in.cstyle && last.comments == in.comments {
			last.text += in.text
			return
		}
	}
	ps.cur = append(ps.cur, in)
}

// --- pictures and drawings ---

// picture gives the index of a related picture in p.pics (-1 when it
// can't be imported, counted).
func (p *docxParser) picture(relID string) int {
	rel, ok := p.rels[relID]
	if !ok || rel.external {
		p.count("pictures")
		return -1
	}
	if i, ok := p.picByTarget[rel.target]; ok {
		if i < 0 {
			p.count("pictures")
		}
		return i
	}
	p.picByTarget[rel.target] = -1
	f := p.pk.file(rel.target)
	if f == nil || f.UncompressedSize64 > uint64(maxDocxPicture) {
		p.count("pictures")
		return -1
	}
	data, err := p.pk.readPart(rel.target, maxDocxPicture)
	if err == errDocxTooBig && p.pk.read <= maxDocxTotal {
		p.count("pictures")
		return -1
	}
	if err != nil {
		p.err = err
		return -1
	}
	ct, ext, ok := sniffPicture(data)
	if !ok {
		p.count("pictures")
		return -1
	}
	base := path.Base(rel.target)
	base = strings.TrimSuffix(base, path.Ext(base))
	if base == "" || base == "." {
		base = "picture"
	}
	p.pics = append(p.pics, docxPicture{Name: base + ext, ContentType: ct, Data: data})
	i := len(p.pics) - 1
	p.picByTarget[rel.target] = i
	return i
}

// sectionSize is the current section's page size, text width and where
// the text starts across the page (left margin and gutter), in EMU.
func (p *docxParser) sectionSize() (pageW, pageH, textW, textLeft float64) {
	pageW, pageH = 12240*635, 15840*635 // Letter
	textW = (12240 - 2*1440) * 635
	textLeft = 1440 * 635
	if p.sect < len(p.sects) {
		s := p.sects[p.sect]
		if w, ok := twipsAttr(s.child("w", "pgSz"), "w"); ok {
			pageW = w * 20 * 635
		}
		if h, ok := twipsAttr(s.child("w", "pgSz"), "h"); ok {
			pageH = h * 20 * 635
		}
		m := s.child("w", "pgMar")
		l, _ := twipsAttr(m, "left")
		r, _ := twipsAttr(m, "right")
		g, _ := twipsAttr(m, "gutter")
		textW = pageW - (l+r+g)*20*635
		textLeft = (l + g) * 20 * 635
	}
	return
}

func paraImageAlign(align string) string {
	switch align {
	case "center", "right":
		return align
	}
	return "left"
}

func (p *docxParser) drawing(el *xEl, ps *paraState) {
	// A paragraph holding only a left-out drawing goes with it.
	ps.suppressed = true
	for _, frame := range el.kids {
		if !frame.is("wp", "inline") && !frame.is("wp", "anchor") {
			continue
		}
		gd := frame.find("a", "graphicData")
		pic := gd.child("pic", "pic")
		if pic == nil {
			boxes := 0
			frame.walk(func(x *xEl) bool {
				if x.is("w", "txbxContent") {
					boxes++
					p.textBox(x, ps)
					return false
				}
				return true
			})
			if boxes == 0 {
				p.count("drawings")
			}
			continue
		}
		blip := pic.find("a", "blip")
		embed := blip.rattr("embed")
		if embed == "" {
			p.count("pictures")
			continue
		}
		ext := frame.child("wp", "extent")
		cx, _ := strconv.ParseFloat(ext.attr("cx"), 64)
		cy, _ := strconv.ParseFloat(ext.attr("cy"), 64)
		pageW, pageH, textW, textLeft := p.sectionSize()
		posH, posV := frame.child("wp", "positionH"), frame.child("wp", "positionV")
		relH, relV := posH.attr("relativeFrom"), posV.attr("relativeFrom")
		if frame.is("wp", "anchor") && offPage(posH, cx, pageW, textLeft) {
			p.count("pictures") // Word doesn't show it either
			continue
		}
		idx := p.picture(embed)
		if idx < 0 {
			continue
		}
		img := &irBlock{kind: 'i', pic: idx, width: cx / 12700, section: p.sect, alt: frame.child("wp", "docPr").attr("descr")}
		if frame.is("wp", "inline") {
			img.align = paraImageAlign(ps.tmpl.p.align)
			p.flush(ps)
			ps.items = append(ps.items, img)
			continue
		}
		if (relH == "page" || relV == "page") && cx >= 0.85*pageW && cy >= 0.85*pageH {
			img.fullPage = true
			img.width = 0
			img.align = "center"
		} else {
			img.align = anchorAlign(posH, cx, pageW, textW)
		}
		ps.after = append(ps.after, img)
	}
}

// offPage: an anchored picture placed wholly beside the page (Word keeps
// it but never shows or prints it).
func offPage(posH *xEl, cx, pageW, textLeft float64) bool {
	off, err := strconv.ParseFloat(strings.TrimSpace(posH.child("wp", "posOffset").txt()), 64)
	if err != nil || posH.child("wp", "align") != nil {
		return false
	}
	left := 0.0
	switch posH.attr("relativeFrom") {
	case "page", "leftMargin":
		left = off
	case "column", "margin", "":
		left = textLeft + off
	default:
		return false
	}
	return left >= pageW || left+cx <= 0
}

// anchorAlign: Word's own alignment when given, else where the picture's
// middle falls across the page or the text column.
func anchorAlign(posH *xEl, cx, pageW, textW float64) string {
	switch posH.child("wp", "align").txt() {
	case "left", "inside":
		return "left"
	case "right", "outside":
		return "right"
	case "center":
		return "center"
	}
	off, err := strconv.ParseFloat(strings.TrimSpace(posH.child("wp", "posOffset").txt()), 64)
	if err != nil {
		return "left"
	}
	ref := textW
	if posH.attr("relativeFrom") == "page" {
		ref = pageW
	}
	if ref <= 0 {
		return "left"
	}
	switch f := (off + cx/2) / ref; {
	case f < 0.4:
		return "left"
	case f > 0.6:
		return "right"
	}
	return "center"
}

// vml reads an old-style w:pict: a picture (v:imagedata) or a text box.
func (p *docxParser) vml(el *xEl, ps *paraState) {
	ps.suppressed = true
	if im := el.find("v", "imagedata"); im != nil {
		id := im.rattr("id")
		if id == "" {
			p.count("pictures")
			return
		}
		idx := p.picture(id)
		if idx < 0 {
			return
		}
		width := 0.0
		if shape := el.find("v", "shape"); shape != nil {
			width = cssLengthPt(cssProp(shape.attr("style"), "width"))
		}
		p.flush(ps)
		ps.items = append(ps.items, &irBlock{kind: 'i', pic: idx, width: width, align: paraImageAlign(ps.tmpl.p.align), section: p.sect})
		return
	}
	boxes := 0
	el.walk(func(x *xEl) bool {
		if x.is("w", "txbxContent") {
			boxes++
			p.textBox(x, ps)
			return false
		}
		return true
	})
	if boxes == 0 && len(el.kids) > 0 {
		p.count("drawings")
	}
}

func cssProp(style, name string) string {
	for _, part := range strings.Split(style, ";") {
		k, v, ok := strings.Cut(part, ":")
		if ok && strings.EqualFold(strings.TrimSpace(k), name) {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func cssLengthPt(s string) float64 {
	units := map[string]float64{"pt": 1, "in": 72, "cm": 72 / 2.54, "mm": 72 / 25.4, "px": 0.75, "pc": 12}
	for u, f := range units {
		if strings.HasSuffix(s, u) {
			v, err := strconv.ParseFloat(strings.TrimSuffix(s, u), 64)
			if err == nil && v > 0 {
				return v * f
			}
			return 0
		}
	}
	return 0
}

// textBox reads a text box's paragraphs into the blocks that follow the
// paragraph it sits in.
func (p *docxParser) textBox(content *xEl, ps *paraState) {
	p.count("text_boxes")
	var blocks []*irBlock
	savedOut, savedFields, savedMarks := p.out, p.fields, p.pendingMark
	p.out, p.fields, p.pendingMark = &blocks, nil, nil
	p.walkBlocks(content.kids)
	p.out, p.fields, p.pendingMark = savedOut, savedFields, savedMarks
	for _, b := range blocks {
		b.section = p.sect
		if b.kind == 'p' {
			b.level = 0 // a heading in a box doesn't start a chapter
		}
	}
	ps.after = append(ps.after, blocks...)
}
