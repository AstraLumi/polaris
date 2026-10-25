package main

// What a Word file says about the book's layout and details, offered to
// the book with the page type after an import (docs/EDITOR.md, "Stage 3
// and 4 additions"): docxImport.Layout and docxImport.Meta.
//
// Mapping:
//   - The section holding the most words (the one the page comes from)
//     gives the layout. Its headers and footers are Word's: a section
//     without a header of a kind (default, even, first) uses the previous
//     section's.
//   - Bands: the default header or footer is the odd pages' band, the even
//     one the even pages', the first one a chapter's first page's. Without
//     even/odd headers (settings.xml's evenAndOddHeaders) the even band is
//     the odd one.
//   - A band's text: PAGE fields become {page}, STYLEREF fields {chapter};
//     NUMPAGES and other page counts are dropped; other fields keep their
//     shown text. Text that is a chapter's Heading 1 title (ignoring case,
//     spacing and a leading "1." number), or that changes from one chapter's
//     section to the next ("CHAPTER ONE", "CHAPTER TWO"…), becomes
//     {chapter}; text before and after it is kept. Text in capitals over
//     titles that aren't sets the band's uppercase.
//   - Where text goes: a positional tab's alignment, else the paragraph's
//     tab stops (the text after the first tab at the first stop, and so
//     on; without stops: centre then right), else the paragraph's
//     alignment (justified is left). Paragraphs of a band are joined.
//   - The band's font, size, italics, small caps and capitals: the first
//     run with text (the style chain under its direct formatting). The
//     font is left out when it is the body text's.
//   - different_first: the section's titlePg. numbering.format: its
//     pgNumType fmt (decimal, lowerRoman, upperRoman, lowerLetter,
//     upperLetter; others arabic); numbering.start: the pgNumType start of
//     the first section holding a Heading 1, else 1. chapter_start: "odd"
//     when most sections that open with a Heading 1 start on an odd page.
//   - Meta: the core properties' title, subject (as the subtitle), creator,
//     description and language, when not empty.
// Layout is nil when the file has no headers, footers or page numbering.

import (
	"sort"
	"strings"
	"unicode"
)

// docxBandText is a band read from a header or footer part, before
// {chapter} is worked out.
type docxBandText struct {
	pos       [3]string // left, center, right
	font      string
	size      float64
	italic    bool
	smallCaps bool
	caps      bool
	ok        bool // the part was read
}

// bandField is a complex field being read in a band.
type bandField struct {
	instr  strings.Builder
	result bool   // past the separator
	kind   string // PAGE, STYLEREF or a page count: its shown text is not kept
}

var wordNumFormats = map[string]string{"decimal": "arabic", "lowerRoman": "roman", "upperRoman": "upper_roman",
	"lowerLetter": "alpha", "upperLetter": "upper_alpha"}

// guessLayout fills imp.Layout. settings and stylesRoot may be nil.
func (p *docxParser) guessLayout(imp *docxImport, settings, stylesRoot *xEl) {
	if len(p.sects) == 0 {
		return
	}
	// Each section's Heading 1 titles and words.
	titles := map[int][]string{}
	words := map[int]int{}
	for _, b := range p.blocks {
		if b.kind != 'p' {
			continue
		}
		if b.level == 1 {
			if t := chapterTitle(b); t != "" {
				titles[b.section] = append(titles[b.section], t)
			}
		}
		words[b.section] += countWords(blockText(b))
	}
	best, bestWords := 0, -1
	for i := range p.sects {
		if words[i] > bestWords {
			best, bestWords = i, words[i]
		}
	}

	// The header and footer parts each section uses (inherited).
	type refKey struct{ kind, typ string }
	effective := make([]map[refKey]string, len(p.sects))
	cur := map[refKey]string{}
	anyBand := false
	for i, s := range p.sects {
		next := map[refKey]string{}
		for k, v := range cur {
			next[k] = v
		}
		for _, ref := range s.kids {
			kind := ""
			switch {
			case ref.is("w", "headerReference"):
				kind = "header"
			case ref.is("w", "footerReference"):
				kind = "footer"
			default:
				continue
			}
			typ := ref.attr("type")
			if typ == "" {
				typ = "default"
			}
			if rel, ok := p.rels[ref.rattr("id")]; ok && !rel.external {
				next[refKey{kind, typ}] = rel.target
				anyBand = true
			}
		}
		effective[i] = next
		cur = next
	}
	sect := p.sects[best]
	numType := sect.child("w", "pgNumType")
	if !anyBand && numType == nil {
		return
	}

	parts := map[string]docxBandText{}
	read := func(target string) docxBandText {
		if target == "" {
			return docxBandText{}
		}
		if b, ok := parts[target]; ok {
			return b
		}
		b := docxBandText{}
		if root, err := p.pk.xml(target); err == nil && root != nil {
			b = p.readBand(root, stylesRoot)
		}
		parts[target] = b
		return b
	}

	var allTitles []string
	for _, ts := range titles {
		allTitles = append(allTitles, ts...)
	}
	bodyFont := ""
	for _, s := range imp.Styles {
		if s.ID == "normal" {
			bodyFont, _ = s.Text["font"].(string)
		}
	}

	band := func(kind, typ string) bookBand {
		b := read(effective[best][refKey{kind, typ}])
		// Positions whose text changes from one chapter's section to another.
		varies := [3]bool{}
		var seen [3]map[string]bool
		for i := range seen {
			seen[i] = map[string]bool{}
		}
		for si := range p.sects {
			if len(titles[si]) == 0 {
				continue
			}
			o := read(effective[si][refKey{kind, typ}])
			for i := range o.pos {
				seen[i][normTitle(o.pos[i])] = true
			}
		}
		for i := range seen {
			varies[i] = len(seen[i]) > 1
		}
		out := bookBand{}
		texts := [3]*string{&out.Left, &out.Center, &out.Right}
		upper := false
		for i, t := range b.pos {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			if c, ok := withChapterField(t, titles[best], allTitles); ok {
				if hasLetters(t) && t == strings.ToUpper(t) && !allUpper(allTitles) {
					upper = true
				}
				t = c
			} else if varies[i] && !strings.Contains(t, "{page}") {
				if hasLetters(t) && t == strings.ToUpper(t) && !allUpper(allTitles) {
					upper = true
				}
				t = "{chapter}"
			}
			*texts[i] = t
		}
		if b.font != "" && !strings.EqualFold(b.font, bodyFont) {
			f := b.font
			out.Font = &f
		}
		if b.size >= 4 && b.size <= 72 {
			s := r1(b.size)
			out.Size = &s
		}
		out.Italic = b.italic
		out.Smallcaps = b.smallCaps
		out.Uppercase = b.caps || upper
		return out
	}

	l := defaultBookLayout()
	l.DifferentFirst = sect.child("w", "titlePg") != nil && wOn(sect.child("w", "titlePg"))
	l.DifferentOddEven = settings.child("w", "evenAndOddHeaders") != nil && wOn(settings.child("w", "evenAndOddHeaders"))
	for _, kind := range []string{"header", "footer"} {
		odd := band(kind, "default")
		even := odd
		if l.DifferentOddEven {
			even = band(kind, "even")
		}
		first := odd
		if l.DifferentFirst {
			first = band(kind, "first")
		}
		bands := bookBands{Odd: odd, Even: even, First: first}
		if kind == "header" {
			l.Header = bands
		} else {
			l.Footer = bands
		}
	}
	if f, ok := wordNumFormats[numType.attr("fmt")]; ok {
		l.Numbering.Format = f
	}
	l.Numbering.Start = 1
	odd, opening := 0, 0
	for i, s := range p.sects {
		if len(titles[i]) == 0 {
			continue
		}
		if opening == 0 {
			if n, ok := intAttr(s.child("w", "pgNumType"), "start"); ok && n >= 0 && n <= maxPageStart {
				l.Numbering.Start = n
			}
		}
		opening++
		if s.child("w", "type").attr("val") == "oddPage" {
			odd++
		}
	}
	if opening > 0 && odd*2 > opening {
		l.ChapterStart = "odd"
	}
	imp.Layout = &l
}

// readBand reads a header or footer part's paragraphs into a band.
func (p *docxParser) readBand(root *xEl, stylesRoot *xEl) docxBandText {
	b := docxBandText{ok: true}
	var paras []*xEl
	var collect func(e *xEl)
	collect = func(e *xEl) {
		for _, k := range e.kids {
			switch {
			case k.is("w", "p"):
				paras = append(paras, k)
			case k.is("w", "tbl"), k.is("w", "tr"), k.is("w", "tc"), k.is("w", "sdt"), k.is("w", "sdtContent"),
				k.is("w", "customXml"), k.is("w", "ins"):
				collect(k)
			}
		}
	}
	collect(root)
	gotFmt := false
	for _, para := range paras {
		pPr := para.child("w", "pPr")
		styleID := p.st.paraStyleID(pPr.child("w", "pStyle").attr("val"))
		eff := p.st.para(styleID)
		eff.over(parsePPr(pPr))
		align := "left"
		if eff.align != nil {
			align = *eff.align
		}
		stops := tabStops(pPr, styleID, p.st, stylesRoot)

		// The paragraph's pieces, split at tabs.
		type piece struct {
			text  strings.Builder
			align string // from a positional tab
		}
		pieces := []*piece{{}}
		var field *bandField
		add := func(s string) {
			if field != nil && field.result && field.kind != "" {
				return // the field's own text stands for its shown result
			}
			pieces[len(pieces)-1].text.WriteString(s)
		}
		fieldText := func(kind string) string {
			switch kind {
			case "PAGE":
				return "{page}"
			case "STYLEREF":
				return "{chapter}"
			}
			return ""
		}
		var runs func(kids []*xEl)
		runs = func(kids []*xEl) {
			for _, k := range kids {
				switch {
				case k.is("w", "r"):
					rPr := k.child("w", "rPr")
					for _, c := range k.kids {
						switch {
						case c.is("w", "t"):
							if c.text != "" && !gotFmt && strings.TrimSpace(c.text) != "" {
								wr := p.st.run(styleID, rPr.child("w", "rStyle").attr("val"))
								wr.over(parseRPr(rPr, p.th))
								f := concreteRun(wr, c.text)
								b.font, b.size, b.italic, b.smallCaps, b.caps = f.font, f.size, f.italic, f.smallCaps, f.caps
								gotFmt = true
							}
							add(c.text)
						case c.is("w", "tab"):
							if field == nil || !field.result || field.kind == "" {
								pieces = append(pieces, &piece{})
							}
						case c.is("w", "ptab"):
							pieces = append(pieces, &piece{align: c.attr("alignment")})
						case c.is("w", "br"), c.is("w", "cr"):
							add(" ")
						case c.is("w", "noBreakHyphen"):
							add("-")
						case c.is("w", "instrText"):
							if field != nil && !field.result {
								field.instr.WriteString(c.text)
							}
						case c.is("w", "fldChar"):
							switch c.attr("fldCharType") {
							case "begin":
								field = &bandField{}
							case "separate":
								if field != nil {
									field.result = true
									if w := splitFieldInstr(field.instr.String()); len(w) > 0 {
										switch k := strings.ToUpper(w[0]); k {
										case "PAGE", "STYLEREF":
											field.kind = k
											pieces[len(pieces)-1].text.WriteString(fieldText(k))
										case "NUMPAGES", "SECTIONPAGES", "SECTION", "PAGEREF":
											field.kind = k
										}
									}
								}
							case "end":
								field = nil
							}
						}
					}
				case k.is("w", "fldSimple"):
					w := splitFieldInstr(k.attr("instr"))
					kind := ""
					if len(w) > 0 {
						kind = strings.ToUpper(w[0])
					}
					switch kind {
					case "PAGE", "STYLEREF":
						add(fieldText(kind))
					case "NUMPAGES", "SECTIONPAGES", "SECTION", "PAGEREF":
					default:
						runs(k.kids)
					}
				case k.is("w", "hyperlink"), k.is("w", "smartTag"), k.is("w", "customXml"), k.is("w", "ins"),
					k.is("w", "sdt"), k.is("w", "sdtContent"):
					runs(k.kids)
				}
			}
		}
		runs(para.kids)

		// Where each piece goes.
		for i, pc := range pieces {
			text := strings.Join(strings.Fields(pc.text.String()), " ")
			if text == "" {
				continue
			}
			where := 0
			switch {
			case pc.align != "":
				where = alignPos(pc.align)
			case i == 0:
				where = alignPos(align)
			case i-1 < len(stops):
				where = alignPos(stops[i-1])
			case len(pieces) == 2:
				where = 2
			default:
				where = min(i, 2)
			}
			if b.pos[where] != "" {
				b.pos[where] += " "
			}
			b.pos[where] += text
		}
	}
	return b
}

// alignPos: 0 left, 1 center, 2 right.
func alignPos(a string) int {
	switch a {
	case "center":
		return 1
	case "right", "end":
		return 2
	}
	return 0
}

// tabStops are a paragraph's tab stops' alignments in order of position:
// its own, else its style chain's (as written in styles.xml).
func tabStops(pPr *xEl, styleID string, st *wStyles, stylesRoot *xEl) []string {
	read := func(tabs *xEl) []string {
		type stop struct {
			pos float64
			val string
		}
		var list []stop
		for _, t := range tabs.childrenNamed("w", "tab") {
			v := t.attr("val")
			if v == "clear" || v == "bar" {
				continue
			}
			pos, _ := twipsAttr(t, "pos")
			list = append(list, stop{pos, v})
		}
		sort.SliceStable(list, func(i, j int) bool { return list[i].pos < list[j].pos })
		out := make([]string, len(list))
		for i, s := range list {
			out[i] = s.val
		}
		return out
	}
	if t := pPr.child("w", "tabs"); t != nil {
		if s := read(t); len(s) > 0 {
			return s
		}
	}
	if stylesRoot == nil {
		return nil
	}
	byID := map[string]*xEl{}
	for _, s := range stylesRoot.childrenNamed("w", "style") {
		byID[s.attr("styleId")] = s
	}
	chain := st.chain(styleID)
	for i := len(chain) - 1; i >= 0; i-- {
		if el := byID[chain[i].id]; el != nil {
			if t := el.child("w", "pPr").child("w", "tabs"); t != nil {
				if s := read(t); len(s) > 0 {
					return s
				}
			}
		}
	}
	return nil
}

// normTitle compares titles: lower case, letters and digits only, a
// leading "1." or "12 -" number left out.
func normTitle(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}
	f := strings.Fields(b.String())
	if len(f) > 1 && strings.IndexFunc(f[0], func(r rune) bool { return !unicode.IsDigit(r) }) < 0 {
		f = f[1:]
	}
	return strings.Join(f, " ")
}

// withChapterField puts {chapter} in place of a chapter title in a band's
// text: the section's own titles first, then any chapter's.
func withChapterField(text string, own, all []string) (string, bool) {
	nt := normTitle(text)
	for _, list := range [][]string{own, all} {
		for _, title := range list {
			n := normTitle(title)
			if n == "" {
				continue
			}
			if nt == n {
				return "{chapter}", true
			}
			// Text around it: find the title as written, ignoring case.
			for _, cand := range []string{title, strings.TrimSpace(strings.TrimLeft(title, "0123456789.-–: "))} {
				if cand == "" {
					continue
				}
				lt, lc := strings.ToLower(text), strings.ToLower(cand)
				if len(lt) != len(text) || len(lc) != len(cand) {
					continue
				}
				if i := strings.Index(lt, lc); i >= 0 {
					return text[:i] + "{chapter}" + text[i+len(cand):], true
				}
			}
		}
	}
	return "", false
}

func hasLetters(s string) bool {
	return strings.IndexFunc(s, unicode.IsLetter) >= 0
}

func allUpper(titles []string) bool {
	if len(titles) == 0 {
		return false
	}
	for _, t := range titles {
		if t != strings.ToUpper(t) {
			return false
		}
	}
	return true
}

// docxMeta reads the core properties (docProps/core.xml): title, subject
// (the subtitle), creator, description and language when not empty.
func docxMeta(pk *docxPackage, pkgRels map[string]docxRel) map[string]string {
	target := relOfType(pkgRels, "/core-properties")
	if target == "" || pk.file(target) == nil {
		target = "docProps/core.xml"
	}
	root, err := pk.xml(target)
	if err != nil || root == nil {
		return nil
	}
	out := map[string]string{}
	for key, names := range map[string][]string{"title": {"title"}, "subtitle": {"subject"}, "author": {"creator"},
		"description": {"description"}, "language": {"language"}} {
		for _, n := range names {
			for _, k := range root.kids {
				if k.name == n {
					if v := strings.Join(strings.Fields(k.text), " "); v != "" {
						if r := []rune(v); len(r) > maxMetaField {
							v = string(r[:maxMetaField])
						}
						out[key] = v
					}
				}
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// hoistFullPagePictures puts each full-page picture (an anchored one the
// size of the page, docxbody.go) at the start of the page its anchor
// paragraph is on: after the page break before it, after the Heading 1
// that opens its chapter, or at the start of the file. Word draws such a
// picture over that page (a cover on the title page); after its paragraph
// it would come a page late. Pictures moved to the same place keep their
// order.
func (p *docxParser) hoistFullPagePictures() {
	blocks := p.blocks
	for i := 0; i < len(blocks); i++ {
		b := blocks[i]
		if b.kind != 'i' || !b.fullPage {
			continue
		}
		to := i
		for to > 0 {
			prev := blocks[to-1]
			if prev.kind == 'b' || (prev.kind == 'i' && prev.fullPage) || (prev.kind == 'p' && prev.level == 1 && chapterTitle(prev) != "") {
				break
			}
			to--
		}
		if to == i {
			continue
		}
		copy(blocks[to+1:i+1], blocks[to:i])
		blocks[to] = b
	}
}

// chapterRestarts gives, by index into imp.Chapters, the page number a
// chapter restarts at: a section opening with a Heading 1 whose pgNumType
// start is lower than the previous such section's. Starts that go up are
// Word carrying the count on (each section repeats where it got to), not
// restarts; the first chapter's start is numbering.start.
func (p *docxParser) chapterRestarts(imp *docxImport) map[int]int {
	out := map[int]int{}
	idx := 0
	if len(imp.Chapters) > 0 && imp.Chapters[0].FrontMatter {
		idx = 1
	}
	prevStart, haveStart := 0, false
	seenSect := map[int]bool{}
	for _, b := range p.blocks {
		if b.kind != 'p' || b.level != 1 || chapterTitle(b) == "" {
			continue
		}
		if !seenSect[b.section] && b.section < len(p.sects) {
			seenSect[b.section] = true
			if n, ok := intAttr(p.sects[b.section].child("w", "pgNumType"), "start"); ok && n >= 0 && n <= maxPageStart {
				if haveStart && n < prevStart && idx < len(imp.Chapters) {
					out[idx] = n
				}
				prevStart, haveStart = n, true
			}
		}
		idx++
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
