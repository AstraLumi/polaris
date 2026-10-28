package main

// From the .docx body's blocks (docxbody.go) to the import's styles,
// chapters and report (docx.go).
//
// Styles. Every Word paragraph style the text uses (and the styles it is
// based on) and every character style it uses becomes an EDITOR.md style.
// Word's default paragraph style is `normal`; "heading 1"–"heading 3",
// "Title", "Subtitle" and "Quote" (by their Word names, which stay English
// whatever the language of Word) are the built-ins `heading1`–`heading3`,
// `title`, `subtitle`, `quote`; "heading 4"–"heading 6" are `heading4`…;
// any other is "word-<styleId>" and keeps its Word name. Document defaults
// are folded into normal, which lists every value; other styles list what
// differs from the style they are based on (normal when Word's base isn't
// imported).
//
// Writers often format by hand ("select all, Merriweather 8 pt,
// justified") and leave Normal as Word made it. So a style takes the
// formatting most of its paragraphs actually have: a paragraph value
// (alignment, each indent, each spacing, line height) that more than half
// of the style's text has, and a font, size or colour more than half of its
// characters have, become the style's; bold, italic, underline, strike and
// small caps only when all of its text has them. Paragraphs and text then
// carry only what differs from their style, so the book's styles do the
// work. List items and scene breaks don't count.
//
// Character styles are listed (for the book's style list) with the settings
// they have, and text in one carries a `char_style` mark naming it (id
// "word-<styleId>"). A run then carries as marks only what it has beyond
// its paragraph style and its character style (and the styles that one is
// based on): direct formatting. A character style Word sets bold, italic or
// the like on can't be turned off by a run in the editor, so a run that
// switches off what its style turns on keeps the style's look.
//
// Paragraph attrs: align, indentFirst (hanging = negative), indentLeft,
// indentRight, spaceBefore, spaceAfter in pt; lineHeight a multiple of
// the font size. Word's "exact" and "at least" line spacing are given in
// pt: lineHeight becomes pt / (1.2 × font size) (1.2 being a single
// line), at least 1 for "at least", and an exact one also keeps the pt
// value as `lineExact` so a later export can write it back. keepNext,
// and `bookmark` (see docxbody.go). A heading's style attr is its style
// (heading1…), not null.
//
// Scene breaks: a paragraph (not a heading or list item) whose only text
// is separator characters — *, #, ~, ⁂, ◇, ◆, •, ·, ○, ●, ☆, ✦, ❖, §, ※,
// =, dashes and the like, with spaces, at most 40 characters — becomes a
// scene_break with that text (spaces tidied) as its glyph: "* * *",
// "***", "◇◇◇", "☆ ☆ ☆". A single mark counts only when it is one of
// *, #, ~, ⁂, §, ※, ❧, ☙, ⸙, ❦, ✠, ⁑ (a lone ● or ☾ is more likely the
// story's own, a moon or a bullet). Pictures are never taken for scene
// breaks.
//
// Chapters: see docx.go. Page breaks that end a chapter (and any before
// the front matter's first block) are dropped: every chapter starts a page.

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
)

var builtinStyleNames = map[string]string{"normal": "Normal", "heading1": "Heading 1", "heading2": "Heading 2",
	"heading3": "Heading 3", "title": "Title", "subtitle": "Subtitle", "quote": "Quote"}

func r2(x float64) float64 { return math.Round(x*100) / 100 }
func r1(x float64) float64 { return math.Round(x*10) / 10 }

// --- scene breaks ---

// sceneBreakSingles: the marks that are a scene break on their own.
const sceneBreakSingles = "*#~⁂§※❧☙⸙❦✠⁑"

const sceneBreakChars = "*#~⁂◇◆♦❖•·◦○●✦✧✶✷✸✹✺✻✼❂❈❉❊❋⁕⁑※§=+_-–—―‒⸺⸻⁘⁙⁛⁜⸙☙❧❦✠✣✤✥⊹⋆★☆♢♤♧♡♠♣♥◈▪▫■□◊⟡⟐〜"

func sceneGlyph(b *irBlock) string {
	if b.kind != 'p' || b.level > 0 || b.list != nil {
		return ""
	}
	var s strings.Builder
	for _, in := range b.inl {
		if in.kind != 't' {
			s.WriteByte(' ')
			continue
		}
		s.WriteString(in.text)
	}
	t := strings.Join(strings.FieldsFunc(s.String(), unicode.IsSpace), " ")
	if t == "" || len([]rune(t)) > 40 {
		return ""
	}
	glyphs := 0
	for _, r := range t {
		if r == ' ' {
			continue
		}
		if !strings.ContainsRune(sceneBreakChars, r) {
			return ""
		}
		glyphs++
	}
	// One mark alone is a scene break only when it is a usual one: a lone
	// dot, star or moon-like glyph is more often part of the story.
	if glyphs == 1 && !strings.Contains(sceneBreakSingles, t) {
		return ""
	}
	return t
}

// --- styles ---

type outStyle struct {
	wid, id, name, kind string
	builtin             bool
	base                string // out id; "" for normal and base-less character styles
	para                pFmt
	text                rFmt
	charRun             wRun // a character style's own run props
	used                bool
}

func styleSlug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else if b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// prettyStyleName: Word stores its built-in names in lower case ("toc 1",
// "footer"); show them as Word does.
func prettyStyleName(name string) string {
	if strings.HasPrefix(name, "toc ") {
		return "TOC " + name[4:]
	}
	r := []rune(name)
	if len(r) > 0 && unicode.IsLower(r[0]) {
		r[0] = unicode.ToUpper(r[0])
	}
	return string(r)
}

type styleSet struct {
	byWid   map[string]*outStyle // paragraph styles by Word id
	charWid map[string]*outStyle
	ids     map[string]bool
	order   []*outStyle
}

func (p *docxParser) newOutStyle(set *styleSet, wid, kind string) *outStyle {
	ws := p.st.byID[wid]
	name := wid
	if ws != nil {
		name = ws.name
	}
	o := &outStyle{wid: wid, kind: kind}
	lower := strings.ToLower(strings.TrimSpace(name))
	id := ""
	if kind == "paragraph" {
		switch {
		case wid == p.st.defaultPara:
			id = "normal"
		case headingNameLevel(lower) >= 1 && headingNameLevel(lower) <= 6:
			id = fmt.Sprintf("heading%d", headingNameLevel(lower))
		case lower == "title" || lower == "subtitle" || lower == "quote":
			id = lower
		}
	}
	if id != "" && !set.ids[id] {
		o.id = id
		if n, ok := builtinStyleNames[id]; ok {
			o.builtin, o.name = true, n
		}
	}
	if o.id == "" {
		base := "word-" + styleSlug(wid)
		if base == "word-" {
			base = "word-style"
		}
		o.id = base
		for i := 2; set.ids[o.id]; i++ {
			o.id = fmt.Sprintf("%s-%d", base, i)
		}
	}
	if o.name == "" {
		o.name = prettyStyleName(name)
	}
	set.ids[o.id] = true
	set.order = append(set.order, o)
	return o
}

// buildStyles makes the paragraph styles the blocks use (and their bases)
// and the character styles the text used, folding each paragraph style's
// common formatting into it.
func (p *docxParser) buildStyles(blocks []*irBlock) *styleSet {
	set := &styleSet{byWid: map[string]*outStyle{}, charWid: map[string]*outStyle{}, ids: map[string]bool{}}
	var addPara func(wid string) *outStyle
	addPara = func(wid string) *outStyle {
		if o := set.byWid[wid]; o != nil {
			return o
		}
		o := p.newOutStyle(set, wid, "paragraph")
		set.byWid[wid] = o
		o.para = concretePara(p.st.para(wid))
		o.text = concreteRun(p.st.run(wid, ""), "")
		if wid != p.st.defaultPara {
			ch := p.st.chain(wid)
			for i := len(ch) - 2; i >= 0; i-- {
				if ch[i].kind == "paragraph" {
					o.base = addPara(ch[i].id).id
					break
				}
			}
			if o.base == "" {
				o.base = addPara(p.st.defaultPara).id
			}
		}
		return o
	}
	addPara(p.st.defaultPara)

	byStyle := map[string][]*irBlock{}
	for _, b := range blocks {
		if b.kind != 'p' || b.scene != "" {
			continue
		}
		o := addPara(b.style)
		o.used = true
		if b.list == nil {
			byStyle[b.style] = append(byStyle[b.style], b)
		}
	}
	for wid, bs := range byStyle {
		foldStyle(set.byWid[wid], bs)
	}

	var addChar func(wid string) *outStyle
	addChar = func(wid string) *outStyle {
		if o := set.charWid[wid]; o != nil {
			return o
		}
		o := p.newOutStyle(set, wid, "character")
		set.charWid[wid] = o
		if ws := p.st.byID[wid]; ws != nil {
			o.charRun = ws.r
			if b := p.st.byID[ws.basedOn]; b != nil && b.kind == "character" && b.id != p.st.defaultChar {
				o.base = addChar(b.id).id
			}
		}
		return o
	}
	chars := make([]string, 0, len(p.usedChar))
	for wid := range p.usedChar {
		if ws := p.st.byID[wid]; ws != nil && ws.kind == "character" {
			chars = append(chars, wid)
		}
	}
	sort.Strings(chars)
	for _, wid := range chars {
		addChar(wid).used = true
	}
	return set
}

// foldStyle gives a style the formatting most of its paragraphs have.
func foldStyle(o *outStyle, blocks []*irBlock) {
	type prop struct {
		key func(f pFmt) string
		set func(dst *pFmt, src pFmt)
	}
	props := []prop{
		{func(f pFmt) string { return f.align }, func(d *pFmt, s pFmt) { d.align = s.align }},
		{func(f pFmt) string { return fmt.Sprint(f.indFirst) }, func(d *pFmt, s pFmt) { d.indFirst = s.indFirst }},
		{func(f pFmt) string { return fmt.Sprint(f.indLeft) }, func(d *pFmt, s pFmt) { d.indLeft = s.indLeft }},
		{func(f pFmt) string { return fmt.Sprint(f.indRight) }, func(d *pFmt, s pFmt) { d.indRight = s.indRight }},
		{func(f pFmt) string { return fmt.Sprint(f.before) }, func(d *pFmt, s pFmt) { d.before = s.before }},
		{func(f pFmt) string { return fmt.Sprint(f.after) }, func(d *pFmt, s pFmt) { d.after = s.after }},
		{func(f pFmt) string { return f.lineRule + fmt.Sprint(f.line) }, func(d *pFmt, s pFmt) { d.lineRule, d.line = s.lineRule, s.line }},
	}
	weightOf := func(b *irBlock) int {
		n := 1
		for _, in := range b.inl {
			n += len([]rune(in.text))
		}
		return n
	}
	total := 0
	for _, b := range blocks {
		total += weightOf(b)
	}
	for _, pr := range props {
		w := map[string]int{}
		from := map[string]pFmt{}
		for _, b := range blocks {
			k := pr.key(b.p)
			w[k] += weightOf(b)
			from[k] = b.p
		}
		if k, n := topKey(w); n*2 > total {
			pr.set(&o.para, from[k])
		}
	}

	fonts, sizes, colors := map[string]int{}, map[string]int{}, map[string]int{}
	sizeOf := map[string]float64{}
	chars := 0
	all := [5]bool{true, true, true, true, true}
	for _, b := range blocks {
		for _, in := range b.inl {
			if in.kind != 't' {
				continue
			}
			n := len([]rune(in.text))
			if strings.TrimSpace(in.text) == "" {
				continue // spaces don't show a font
			}
			chars += n
			fonts[in.r.font] += n
			k := fmt.Sprint(in.r.size)
			sizes[k] += n
			sizeOf[k] = in.r.size
			colors[in.r.color] += n
			for i, on := range []bool{in.r.bold, in.r.italic, in.r.underline, in.r.strike, in.r.smallCaps} {
				all[i] = all[i] && on
			}
		}
	}
	if chars == 0 {
		return
	}
	if k, n := topKey(fonts); n*2 > chars {
		o.text.font = k
	}
	if k, n := topKey(sizes); n*2 > chars {
		o.text.size = sizeOf[k]
	}
	if k, n := topKey(colors); n*2 > chars {
		o.text.color = k
	}
	o.text.bold, o.text.italic, o.text.underline, o.text.strike, o.text.smallCaps = all[0], all[1], all[2], all[3], all[4]
}

func topKey(w map[string]int) (string, int) {
	best, bestN := "", -1
	for k, n := range w {
		if n > bestN || n == bestN && k < best {
			best, bestN = k, n
		}
	}
	return best, bestN
}

// lineAttrs turns Word's line spacing into lineHeight (and lineExact).
func lineAttrs(rule string, val, size float64) (float64, *float64) {
	if size <= 0 {
		size = 10
	}
	switch rule {
	case "exact":
		pt := r2(val)
		return r2(val / (1.2 * size)), &pt
	case "atLeast":
		return r2(math.Max(1, val/(1.2*size))), nil
	}
	return r2(val), nil
}

func colorOrNil(c string) any {
	if c == "" {
		return nil
	}
	return c
}

// paraJSON: a style's paragraph values (all of them for normal, else what
// differs from base).
func paraJSON(f pFmt, size float64, base *pFmt, baseSize float64) map[string]any {
	m := map[string]any{}
	if base == nil || f.align != base.align {
		m["align"] = f.align
	}
	for _, x := range []struct {
		k    string
		v, b float64
	}{{"indentFirst", f.indFirst, 0}, {"indentLeft", f.indLeft, 0}, {"indentRight", f.indRight, 0},
		{"spaceBefore", f.before, 0}, {"spaceAfter", f.after, 0}} {
		if base != nil {
			switch x.k {
			case "indentFirst":
				x.b = base.indFirst
			case "indentLeft":
				x.b = base.indLeft
			case "indentRight":
				x.b = base.indRight
			case "spaceBefore":
				x.b = base.before
			case "spaceAfter":
				x.b = base.after
			}
		}
		if base == nil || r2(x.v) != r2(x.b) {
			m[x.k] = r2(x.v)
		}
	}
	lh, exact := lineAttrs(f.lineRule, f.line, size)
	if base == nil {
		m["lineHeight"] = lh
		if exact != nil {
			m["lineExact"] = *exact
		}
	} else {
		blh, bexact := lineAttrs(base.lineRule, base.line, baseSize)
		if lh != blh || (exact == nil) != (bexact == nil) || exact != nil && *exact != *bexact {
			m["lineHeight"] = lh
			if exact != nil {
				m["lineExact"] = *exact
			}
		}
	}
	if base == nil || f.keepNext != base.keepNext {
		m["keepNext"] = f.keepNext
	}
	return m
}

func textJSON(f rFmt, base *rFmt) map[string]any {
	m := map[string]any{}
	if base == nil || f.font != base.font {
		m["font"] = f.font
	}
	if base == nil || f.size != base.size {
		m["size"] = r2(f.size)
	}
	if base == nil || f.color != base.color {
		m["color"] = colorOrNil(f.color)
	}
	if base != nil && f.highlight != base.highlight || base == nil && f.highlight != "" {
		m["highlight"] = colorOrNil(f.highlight)
	}
	toggles := []struct {
		k    string
		v, b bool
	}{{"bold", f.bold, false}, {"italic", f.italic, false}, {"underline", f.underline, false},
		{"strike", f.strike, false}, {"smallcaps", f.smallCaps, false},
		{"superscript", f.vert == "superscript", false}, {"subscript", f.vert == "subscript", false}}
	if base != nil {
		bt := []bool{base.bold, base.italic, base.underline, base.strike, base.smallCaps, base.vert == "superscript", base.vert == "subscript"}
		for i := range toggles {
			toggles[i].b = bt[i]
		}
	}
	for _, t := range toggles {
		if t.v != t.b {
			m[t.k] = t.v
		}
	}
	return m
}

// charStyleJSON: a character style's own settings.
func charStyleJSON(r wRun) map[string]any {
	m := map[string]any{}
	if r.asciiFont != nil {
		m["font"] = *r.asciiFont
	} else if r.hAnsiFont != nil {
		m["font"] = *r.hAnsiFont
	}
	if r.size != nil {
		m["size"] = r2(*r.size)
	}
	if r.color != nil {
		m["color"] = colorOrNil(*r.color)
	}
	if r.highlight != nil {
		m["highlight"] = colorOrNil(*r.highlight)
	} else if r.shade != nil {
		m["highlight"] = colorOrNil(*r.shade)
	}
	for _, t := range []struct {
		k string
		v *bool
	}{{"bold", r.bold}, {"italic", r.italic}, {"underline", r.underline}, {"strike", r.strike}, {"smallcaps", r.smallCaps}} {
		if t.v != nil {
			m[t.k] = *t.v
		}
	}
	if r.vert != nil {
		m["superscript"] = *r.vert == "superscript"
		m["subscript"] = *r.vert == "subscript"
	}
	return m
}

func (set *styleSet) export() []bookStyle {
	byID := map[string]*outStyle{}
	for _, o := range set.order {
		byID[o.id] = o
	}
	rank := func(o *outStyle) int {
		order := []string{"normal", "heading1", "heading2", "heading3", "title", "subtitle", "quote"}
		for i, id := range order {
			if o.id == id {
				return i
			}
		}
		if o.kind == "character" {
			return 200
		}
		return 100
	}
	list := append([]*outStyle(nil), set.order...)
	sort.SliceStable(list, func(i, j int) bool {
		ri, rj := rank(list[i]), rank(list[j])
		if ri != rj {
			return ri < rj
		}
		return strings.ToLower(list[i].name) < strings.ToLower(list[j].name)
	})
	var out []bookStyle
	for _, o := range list {
		bs := bookStyle{ID: o.id, Name: o.name, Kind: o.kind, Builtin: o.builtin}
		if o.base != "" {
			b := o.base
			bs.BasedOn = &b
		}
		if o.kind == "character" {
			bs.Text = charStyleJSON(o.charRun)
		} else if base := byID[o.base]; base != nil {
			bs.Para = paraJSON(o.para, o.text.size, &base.para, base.text.size)
			bs.Text = textJSON(o.text, &base.text)
		} else {
			bs.Para = paraJSON(o.para, o.text.size, nil, 0)
			bs.Text = textJSON(o.text, nil)
		}
		out = append(out, bs)
	}
	return out
}

// --- nodes ---

// charRun is a character style's settings with those of the character
// styles it is based on under them (the settings charStyleJSON lists).
func (set *styleSet) charRun(o *outStyle) wRun {
	var chain []*outStyle
	seen := map[string]bool{}
	for cur := o; cur != nil && !seen[cur.id]; {
		seen[cur.id] = true
		chain = append([]*outStyle{cur}, chain...)
		var next *outStyle
		for _, c := range set.order {
			if c.kind == "character" && c.id == cur.base && cur.base != "" {
				next = c
			}
		}
		cur = next
	}
	var r wRun
	for _, c := range chain {
		r.over(c.charRun)
	}
	return r
}

// marksFor gives a run's marks: its link, its character style, then
// whatever its formatting has beyond what its paragraph style and character
// style already give (direct formatting). A run in a character style looks
// like paragraph style + character style, as the editor draws it.
func marksFor(in irInline, s *outStyle, set *styleSet) []pmMark {
	var ms []pmMark
	if in.link != "" {
		ms = append(ms, pmMark{Type: "link", Attrs: map[string]any{"href": in.link}})
	}
	r, t := in.r, s.text
	var cr wRun
	if cs := set.charWid[in.cstyle]; cs != nil && in.cstyle != "" {
		ms = append(ms, pmMark{Type: "char_style", Attrs: map[string]any{"id": cs.id}})
		cr = set.charRun(cs)
	}
	by := func(own *bool, base bool) bool {
		if own != nil {
			return *own
		}
		return base
	}
	vertBy := func(want string) bool {
		if cr.vert != nil {
			return *cr.vert == want
		}
		return t.vert == want
	}
	for _, x := range []struct {
		name   string
		on, by bool
	}{{"bold", r.bold, by(cr.bold, t.bold)}, {"italic", r.italic, by(cr.italic, t.italic)}, {"underline", r.underline, by(cr.underline, t.underline)},
		{"strike", r.strike, by(cr.strike, t.strike)}, {"superscript", r.vert == "superscript", vertBy("superscript")},
		{"subscript", r.vert == "subscript", vertBy("subscript")}, {"smallcaps", r.smallCaps, by(cr.smallCaps, t.smallCaps)}} {
		if x.on && !x.by {
			ms = append(ms, pmMark{Type: x.name})
		}
	}
	font, size, color, highlight := t.font, t.size, t.color, t.highlight
	if cr.asciiFont != nil {
		font = *cr.asciiFont
	} else if cr.hAnsiFont != nil {
		font = *cr.hAnsiFont
	}
	if cr.size != nil {
		size = *cr.size
	}
	if cr.color != nil {
		color = *cr.color
	}
	if cr.highlight != nil {
		highlight = *cr.highlight
	} else if cr.shade != nil {
		highlight = *cr.shade
	}
	if r.font != font && r.font != "" {
		ms = append(ms, pmMark{Type: "font", Attrs: map[string]any{"family": r.font}})
	}
	if r.size != size && r.size > 0 {
		ms = append(ms, pmMark{Type: "size", Attrs: map[string]any{"pt": r2(r.size)}})
	}
	if r.color != color && r.color != "" {
		ms = append(ms, pmMark{Type: "color", Attrs: map[string]any{"color": r.color}})
	}
	if r.highlight != "" && r.highlight != highlight {
		ms = append(ms, pmMark{Type: "highlight", Attrs: map[string]any{"color": r.highlight}})
	}
	return append(ms, commentMarks(in.comments)...) // docxcomments.go
}

func marksKey(ms []pmMark) string {
	var b strings.Builder
	for _, m := range ms {
		b.WriteString(m.Type)
		for _, k := range []string{"href", "family", "pt", "color", "id"} {
			if v, ok := m.Attrs[k]; ok {
				fmt.Fprintf(&b, ":%v", v)
			}
		}
		b.WriteByte('|')
	}
	return b.String()
}

func inlineNodes(b *irBlock, s *outStyle, set *styleSet) []*pmNode {
	var out []*pmNode
	lastKey := ""
	for _, in := range b.inl {
		switch in.kind {
		case '\t':
			out = append(out, &pmNode{Type: "tab"})
			lastKey = "\x00"
		case 'n':
			out = append(out, &pmNode{Type: "hard_break"})
			lastKey = "\x00"
		default:
			if in.text == "" {
				continue
			}
			ms := marksFor(in, s, set)
			k := marksKey(ms)
			if n := len(out); n > 0 && out[n-1].Type == "text" && k == lastKey {
				out[n-1].Text += in.text
				continue
			}
			out = append(out, &pmNode{Type: "text", Text: in.text, Marks: ms})
			lastKey = k
		}
	}
	return out
}

func firstSize(b *irBlock, fallback float64) float64 {
	for _, in := range b.inl {
		if in.kind == 't' && strings.TrimSpace(in.text) != "" {
			return in.r.size
		}
	}
	return fallback
}

func (p *docxParser) paraNode(b *irBlock, set *styleSet, keep map[string]bool) *pmNode {
	s := set.byWid[b.style]
	attrs := map[string]any{}
	n := &pmNode{Type: "paragraph", Attrs: attrs}
	if b.level > 0 {
		n.Type = "heading"
		attrs["level"] = b.level
	}
	if s.id != "normal" || b.level > 0 {
		attrs["style"] = s.id
	}
	if b.p.align != s.para.align {
		attrs["align"] = b.p.align
	}
	if b.list == nil {
		if r2(b.p.indFirst) != r2(s.para.indFirst) {
			attrs["indentFirst"] = r2(b.p.indFirst)
		}
		if r2(b.p.indLeft) != r2(s.para.indLeft) {
			attrs["indentLeft"] = r2(b.p.indLeft)
		}
	}
	if r2(b.p.indRight) != r2(s.para.indRight) {
		attrs["indentRight"] = r2(b.p.indRight)
	}
	if r2(b.p.before) != r2(s.para.before) {
		attrs["spaceBefore"] = r2(b.p.before)
	}
	if r2(b.p.after) != r2(s.para.after) {
		attrs["spaceAfter"] = r2(b.p.after)
	}
	if b.p.lineRule != s.para.lineRule || r2(b.p.line) != r2(s.para.line) {
		lh, exact := lineAttrs(b.p.lineRule, b.p.line, firstSize(b, s.text.size))
		attrs["lineHeight"] = lh
		if exact != nil {
			attrs["lineExact"] = *exact
		}
	}
	if b.p.keepNext && !s.para.keepNext {
		attrs["keepNext"] = true
	}
	for _, name := range b.bookmarks {
		if keep[name] {
			attrs["bookmark"] = name
			break
		}
	}
	n.Content = inlineNodes(b, s, set)
	if len(attrs) == 0 {
		n.Attrs = nil
	}
	return n
}

// listLevel is a list being filled: the one an item goes in, at Word's
// level ilvl, of Word's list numID.
type listLevel struct {
	node        *pmNode
	ilvl, numID int
	ordered     bool
}

const maxListLevels = 9 // Word's, and what the editor lets a writer nest

func newListNode(ref *irListRef) *pmNode {
	l := &pmNode{Type: "bullet_list"}
	if ref.ordered {
		l.Type = "ordered_list"
		l.Attrs = map[string]any{"start": ref.value}
	}
	return l
}

// addListItem puts an item in the lists open so far (stack, outermost
// first) and returns the stack to continue with. A deeper level nests a
// list in the item before it; a shallower one closes the deeper lists; a
// different list or kind of list at the top starts a new one.
func addListItem(out *[]*pmNode, stack []listLevel, ref *irListRef, item *pmNode) []listLevel {
	for len(stack) > 1 && stack[len(stack)-1].ilvl > ref.ilvl {
		stack = stack[:len(stack)-1]
	}
	level := listLevel{ilvl: ref.ilvl, numID: ref.numID, ordered: ref.ordered}
	// nest opens a list inside the last item of the list below it.
	nest := func(below []listLevel) []listLevel {
		level.node = newListNode(ref)
		parent := below[len(below)-1].node
		last := parent.Content[len(parent.Content)-1]
		last.Content = append(last.Content, level.node)
		return append(below, level)
	}
	top := len(stack) - 1
	switch {
	case top < 0:
		level.node = newListNode(ref)
		*out = append(*out, level.node)
		stack = []listLevel{level}
	case stack[top].ilvl < ref.ilvl && len(stack) < maxListLevels:
		stack = nest(stack)
	case top == 0 && (stack[0].numID != ref.numID || stack[0].ordered != ref.ordered):
		level.node = newListNode(ref)
		*out = append(*out, level.node)
		stack = []listLevel{level}
	case top > 0 && stack[top].ordered != ref.ordered && stack[top].ilvl == ref.ilvl:
		stack = nest(stack[:top])
	default:
		if top == 0 && ref.ilvl < stack[0].ilvl {
			stack[0].ilvl = ref.ilvl
		}
	}
	list := stack[len(stack)-1].node
	list.Content = append(list.Content, item)
	return stack
}

func (p *docxParser) blockNodes(blocks []*irBlock, set *styleSet, keep map[string]bool) []*pmNode {
	var out []*pmNode
	var stack []listLevel
	for _, b := range blocks {
		if b.kind == 'p' && b.list != nil && b.scene == "" {
			item := &pmNode{Type: "list_item", Content: []*pmNode{p.paraNode(b, set, keep)}}
			stack = addListItem(&out, stack, b.list, item)
			continue
		}
		stack = nil
		switch {
		case b.kind == 'b':
			out = append(out, &pmNode{Type: "page_break"})
		case b.kind == 'i':
			attrs := map[string]any{"src": docxPictureSrc(b.pic), "alt": b.alt, "width": nil, "align": b.align, "fullPage": b.fullPage}
			if b.width > 0 {
				attrs["width"] = r2(b.width)
			}
			out = append(out, &pmNode{Type: "image", Attrs: attrs})
		case b.scene != "":
			out = append(out, &pmNode{Type: "scene_break", Attrs: map[string]any{"glyph": b.scene}})
		default:
			out = append(out, p.paraNode(b, set, keep))
		}
	}
	return out
}

func blockText(b *irBlock) string {
	var s strings.Builder
	for _, in := range b.inl {
		if in.kind == 't' {
			s.WriteString(in.text)
		} else {
			s.WriteByte(' ')
		}
	}
	return s.String()
}

func hasContent(blocks []*irBlock) bool {
	for _, b := range blocks {
		if b.kind == 'i' || b.scene != "" || b.kind == 'p' && strings.TrimSpace(blockText(b)) != "" {
			return true
		}
	}
	return false
}

func chapterTitle(b *irBlock) string {
	t := strings.Join(strings.Fields(blockText(b)), " ")
	if r := []rune(t); len(r) > 200 {
		t = string(r[:200])
	}
	return t
}

func (p *docxParser) finish() *docxImport {
	blocks := p.blocks
	for _, b := range blocks {
		b.scene = sceneGlyph(b)
	}
	// Bookmarks: the visible ones, and hidden ones a kept link points at.
	keep := map[string]bool{}
	for _, b := range blocks {
		for _, name := range b.bookmarks {
			if name != "" && (!strings.HasPrefix(name, "_") || p.linkAnchors[name]) {
				keep[name] = true
			}
		}
	}
	set := p.buildStyles(blocks)

	// Split at Heading 1.
	type part struct {
		title  string
		front  bool
		blocks []*irBlock
	}
	parts := []*part{{front: true}}
	for _, b := range blocks {
		if b.kind == 'p' && b.level == 1 {
			if t := chapterTitle(b); t != "" {
				parts = append(parts, &part{title: t})
			}
		}
		cur := parts[len(parts)-1]
		cur.blocks = append(cur.blocks, b)
	}
	imp := &docxImport{Pictures: p.pics}
	for _, pt := range parts {
		bs := pt.blocks
		for len(bs) > 0 && bs[len(bs)-1].kind == 'b' {
			bs = bs[:len(bs)-1]
		}
		for len(bs) > 0 && bs[0].kind == 'b' {
			bs = bs[1:]
		}
		if pt.front && !hasContent(bs) {
			continue
		}
		doc := &pmNode{Type: "doc", Content: p.blockNodes(bs, set, keep)}
		if len(doc.Content) == 0 {
			doc.Content = []*pmNode{{Type: "paragraph"}}
		}
		imp.Chapters = append(imp.Chapters, docxChapter{Title: pt.title, FrontMatter: pt.front, Doc: doc, Words: docWords(doc)})
	}

	imp.Styles = set.export()

	// Fonts, by how much text is set in them.
	fontUse := map[string]int{}
	sectWords := map[int]int{}
	for _, b := range blocks {
		if b.kind != 'p' {
			continue
		}
		for _, in := range b.inl {
			if in.kind == 't' && strings.TrimSpace(in.text) != "" && in.r.font != "" {
				fontUse[in.r.font] += len([]rune(in.text))
			}
		}
		sectWords[b.section] += countWords(blockText(b))
	}
	for f := range fontUse {
		imp.Fonts = append(imp.Fonts, f)
	}
	sort.Slice(imp.Fonts, func(i, j int) bool {
		a, b := imp.Fonts[i], imp.Fonts[j]
		if fontUse[a] != fontUse[b] {
			return fontUse[a] > fontUse[b]
		}
		return a < b
	})
	if imp.Fonts == nil {
		imp.Fonts = []string{}
	}

	// The page: the section holding the most words.
	best, bestWords := 0, -1
	for i := 0; i < max(1, len(p.sects)); i++ {
		if sectWords[i] > bestWords {
			best, bestWords = i, sectWords[i]
		}
	}
	var sect *xEl
	if best < len(p.sects) {
		sect = p.sects[best]
	}
	imp.Page = pageOf(sect, p.mirror)
	imp.PageTypeGuess = guessPageType(imp.Page)

	imp.LeftOut = map[string]int{}
	for k, v := range p.leftOut {
		if v > 0 {
			imp.LeftOut[k] = v
		}
	}
	if len(p.hfParts) > 0 {
		imp.LeftOut["headers_footers"] = len(p.hfParts)
	}
	p.finishComments(imp) // docxcomments.go
	return imp
}

// --- page setup ---

func twipsMM(tw float64) float64 { return r1(tw / 1440 * 25.4) }

// pageOf reads a section's page size and margins (Word's Letter page with
// 1-inch margins when there is none). With mirrored margins Word's left
// margin is the inner one; without, left is taken as inner.
func pageOf(s *xEl, mirror bool) bookPage {
	w, h := 12240.0, 15840.0
	top, bottom, left, right, gutter := 1440.0, 1440.0, 1440.0, 1440.0, 0.0
	if s != nil {
		get := func(e *xEl, name string, dst *float64) {
			if v, ok := twipsAttr(e, name); ok {
				*dst = math.Abs(v * 20)
			}
		}
		sz, mar := s.child("w", "pgSz"), s.child("w", "pgMar")
		get(sz, "w", &w)
		get(sz, "h", &h)
		get(mar, "top", &top)
		get(mar, "bottom", &bottom)
		get(mar, "left", &left)
		get(mar, "right", &right)
		get(mar, "gutter", &gutter)
	}
	return bookPage{WidthMM: twipsMM(w), HeightMM: twipsMM(h), MarginTopMM: twipsMM(top), MarginBottomMM: twipsMM(bottom),
		MarginInnerMM: twipsMM(left), MarginOuterMM: twipsMM(right), GutterMM: twipsMM(gutter), Mirror: mirror}
}

// guessPageType: a4, a5 or a6 when the page is within 3 mm of one
// (portrait), else custom.
func guessPageType(pg bookPage) string {
	for _, t := range []struct {
		key  string
		w, h float64
	}{{"a4", 210, 297}, {"a5", 148, 210}, {"a6", 105, 148}} {
		if math.Abs(pg.WidthMM-t.w) <= 3 && math.Abs(pg.HeightMM-t.h) <= 3 {
			return t.key
		}
	}
	return "custom"
}
