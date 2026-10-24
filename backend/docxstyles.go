package main

// Word formatting for the .docx import (docx.go): paragraph and run
// properties as set at one level (wPara, wRun: nil = not set there), the
// style sheet with its basedOn chains, theme fonts and numbering, resolved
// the way Word does:
//
//	paragraph: document defaults < paragraph style chain (base first) < direct
//	run:       document defaults < paragraph style chain's run props
//	           < character style chain < direct
//
// Each attribute of w:ind and w:spacing is inherited on its own, as in
// Word. Toggle properties (bold, italic…) simply override along the chain;
// Word's rule that a character style's bold toggles a bold paragraph style
// off is not followed (it is rare and surprises writers too).

import (
	"strconv"
	"strings"
)

type wLine struct {
	rule string  // "auto" (val is a multiple), "exact" or "atLeast" (val in pt)
	val  float64 // see rule
}

type wPara struct {
	align                        *string // left center right justify
	indFirst, indLeft, indRight  *float64
	before, after                *float64
	line                         *wLine
	keepNext, pageBreakBefore    *bool
	numID, ilvl, outline         *int
	hasBorders, hasShade, isDrop bool // direct only, for the left-out report
}

type wRun struct {
	bold, italic, underline, strike, smallCaps, caps, vanish *bool
	vert                                                     *string // superscript subscript baseline
	asciiFont, hAnsiFont                                     *string
	size                                                     *float64
	color, highlight, shade                                  *string // "" = none
}

func ptrF(f float64) *float64 { return &f }
func ptrS(s string) *string   { return &s }
func ptrB(b bool) *bool       { return &b }
func ptrI(i int) *int         { return &i }

func (p *wPara) over(t wPara) {
	if t.align != nil {
		p.align = t.align
	}
	if t.indFirst != nil {
		p.indFirst = t.indFirst
	}
	if t.indLeft != nil {
		p.indLeft = t.indLeft
	}
	if t.indRight != nil {
		p.indRight = t.indRight
	}
	if t.before != nil {
		p.before = t.before
	}
	if t.after != nil {
		p.after = t.after
	}
	if t.line != nil {
		p.line = t.line
	}
	if t.keepNext != nil {
		p.keepNext = t.keepNext
	}
	if t.pageBreakBefore != nil {
		p.pageBreakBefore = t.pageBreakBefore
	}
	if t.numID != nil {
		p.numID = t.numID
		p.ilvl = nil
	}
	if t.ilvl != nil {
		p.ilvl = t.ilvl
	}
	if t.outline != nil {
		p.outline = t.outline
	}
}

func (r *wRun) over(t wRun) {
	for _, f := range [][2]**bool{{&r.bold, &t.bold}, {&r.italic, &t.italic}, {&r.underline, &t.underline},
		{&r.strike, &t.strike}, {&r.smallCaps, &t.smallCaps}, {&r.caps, &t.caps}, {&r.vanish, &t.vanish}} {
		if *f[1] != nil {
			*f[0] = *f[1]
		}
	}
	for _, f := range [][2]**string{{&r.vert, &t.vert}, {&r.asciiFont, &t.asciiFont}, {&r.hAnsiFont, &t.hAnsiFont},
		{&r.color, &t.color}, {&r.highlight, &t.highlight}, {&r.shade, &t.shade}} {
		if *f[1] != nil {
			*f[0] = *f[1]
		}
	}
	if t.size != nil {
		r.size = t.size
	}
}

// twips (1/20 pt) attribute to pt; ok false when missing or not a number.
func twipsAttr(e *xEl, name string) (float64, bool) {
	s := e.attr(name)
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v / 20, true
}

func intAttr(e *xEl, name string) (int, bool) {
	s := e.attr(name)
	if s == "" {
		return 0, false
	}
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, false
	}
	return v, true
}

func parsePPr(e *xEl) wPara {
	var p wPara
	if e == nil {
		return p
	}
	for _, k := range e.kids {
		if k.ns != "w" {
			continue
		}
		switch k.name {
		case "jc":
			switch k.attr("val") {
			case "both", "distribute", "thaiDistribute", "lowKashida", "mediumKashida", "highKashida":
				p.align = ptrS("justify")
			case "center":
				p.align = ptrS("center")
			case "right", "end":
				p.align = ptrS("right")
			case "left", "start":
				p.align = ptrS("left")
			}
		case "ind":
			if v, ok := twipsAttr(k, "left"); ok {
				p.indLeft = ptrF(v)
			} else if v, ok := twipsAttr(k, "start"); ok {
				p.indLeft = ptrF(v)
			}
			if v, ok := twipsAttr(k, "right"); ok {
				p.indRight = ptrF(v)
			} else if v, ok := twipsAttr(k, "end"); ok {
				p.indRight = ptrF(v)
			}
			if v, ok := twipsAttr(k, "hanging"); ok {
				p.indFirst = ptrF(-v)
			} else if v, ok := twipsAttr(k, "firstLine"); ok {
				p.indFirst = ptrF(v)
			}
		case "spacing":
			if v, ok := twipsAttr(k, "before"); ok {
				p.before = ptrF(v)
			}
			if v, ok := twipsAttr(k, "after"); ok {
				p.after = ptrF(v)
			}
			// Automatic spacing ("as in HTML") is 14 pt in Word.
			if k.hasAttr("beforeAutospacing") && wOnAttr(k, "beforeAutospacing") {
				p.before = ptrF(14)
			}
			if k.hasAttr("afterAutospacing") && wOnAttr(k, "afterAutospacing") {
				p.after = ptrF(14)
			}
			if s := k.attr("line"); s != "" {
				if v, err := strconv.ParseFloat(s, 64); err == nil && v > 0 {
					switch k.attr("lineRule") {
					case "exact":
						p.line = &wLine{"exact", v / 20}
					case "atLeast":
						p.line = &wLine{"atLeast", v / 20}
					default:
						p.line = &wLine{"auto", v / 240}
					}
				}
			}
		case "keepNext":
			p.keepNext = ptrB(wOn(k))
		case "pageBreakBefore":
			p.pageBreakBefore = ptrB(wOn(k))
		case "numPr":
			if id, ok := intAttr(k.child("w", "numId"), "val"); ok {
				p.numID = ptrI(id)
			}
			if lv, ok := intAttr(k.child("w", "ilvl"), "val"); ok {
				p.ilvl = ptrI(lv)
			}
		case "outlineLvl":
			if lv, ok := intAttr(k, "val"); ok {
				p.outline = ptrI(lv)
			}
		case "pBdr":
			p.hasBorders = len(k.kids) > 0
		case "shd":
			if f := strings.ToLower(k.attr("fill")); f != "" && f != "auto" && f != "ffffff" {
				p.hasShade = true
			}
		case "framePr":
			if dc := k.attr("dropCap"); dc == "drop" || dc == "margin" {
				p.isDrop = true
			}
		}
	}
	return p
}

func wOnAttr(e *xEl, name string) bool {
	switch strings.ToLower(e.attr(name)) {
	case "0", "false", "off":
		return false
	}
	return true
}

var highlightColors = map[string]string{
	"black": "#000000", "blue": "#0000ff", "cyan": "#00ffff", "green": "#00ff00",
	"magenta": "#ff00ff", "red": "#ff0000", "yellow": "#ffff00", "white": "#ffffff",
	"darkBlue": "#000080", "darkCyan": "#008080", "darkGreen": "#008000",
	"darkMagenta": "#800080", "darkRed": "#800000", "darkYellow": "#808000",
	"darkGray": "#808080", "lightGray": "#c0c0c0",
}

// hexColor turns Word's RRGGBB into #rrggbb ("" for auto or junk).
func hexColor(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if len(s) != 6 {
		return ""
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return ""
		}
	}
	return "#" + s
}

type docxTheme struct{ major, minor string }

func (th docxTheme) font(name string) string {
	switch {
	case strings.HasPrefix(name, "major"):
		return th.major
	case strings.HasPrefix(name, "minor"):
		return th.minor
	}
	return ""
}

func parseTheme(root *xEl) docxTheme {
	var th docxTheme
	fs := root.find("a", "fontScheme")
	if lat := fs.child("a", "majorFont").child("a", "latin"); lat != nil {
		th.major = lat.attr("typeface")
	}
	if lat := fs.child("a", "minorFont").child("a", "latin"); lat != nil {
		th.minor = lat.attr("typeface")
	}
	return th
}

func parseRPr(e *xEl, th docxTheme) wRun {
	var r wRun
	if e == nil {
		return r
	}
	for _, k := range e.kids {
		if k.ns != "w" {
			continue
		}
		switch k.name {
		case "b":
			r.bold = ptrB(wOn(k))
		case "i":
			r.italic = ptrB(wOn(k))
		case "u":
			v := k.attr("val")
			r.underline = ptrB(v != "none" && v != "0" && v != "false")
		case "strike", "dstrike":
			on := wOn(k)
			if on || r.strike == nil {
				r.strike = ptrB(on)
			}
		case "smallCaps":
			r.smallCaps = ptrB(wOn(k))
		case "caps":
			r.caps = ptrB(wOn(k))
		case "vanish":
			r.vanish = ptrB(wOn(k))
		case "vertAlign":
			r.vert = ptrS(k.attr("val"))
		case "rFonts":
			// A theme font wins over a named one on the same element.
			if f := th.font(k.attr("asciiTheme")); f != "" {
				r.asciiFont = ptrS(f)
			} else if f := k.attr("ascii"); f != "" {
				r.asciiFont = ptrS(f)
			}
			if f := th.font(k.attr("hAnsiTheme")); f != "" {
				r.hAnsiFont = ptrS(f)
			} else if f := k.attr("hAnsi"); f != "" {
				r.hAnsiFont = ptrS(f)
			}
		case "sz":
			if v, err := strconv.ParseFloat(k.attr("val"), 64); err == nil && v > 0 {
				r.size = ptrF(v / 2)
			}
		case "color":
			// The theme's text colour (text1/dark1) is the default text
			// colour, not a chosen black: it follows the editor's colour.
			switch strings.ToLower(k.attr("themeColor")) {
			case "text1", "dark1":
				r.color = ptrS("")
			default:
				r.color = ptrS(hexColor(k.attr("val")))
			}
		case "highlight":
			r.highlight = ptrS(highlightColors[k.attr("val")])
		case "shd":
			// White shading reads as no shading (it would show as a white
			// box on a dark page).
			if c := hexColor(k.attr("fill")); c != "#ffffff" {
				r.shade = ptrS(c)
			} else {
				r.shade = ptrS("")
			}
		}
	}
	return r
}

// --- the style sheet ---

type wStyle struct {
	id, name, kind, basedOn string
	isDefault               bool
	p                       wPara
	r                       wRun
}

type wStyles struct {
	byID         map[string]*wStyle
	defP         wPara
	defR         wRun
	defaultPara  string // the default paragraph style's id ("Normal")
	defaultChar  string
	paraCache    map[string]wPara
	runCache     map[[2]string]wRun
	chainCache   map[string][]*wStyle
	headingCache map[string]int
}

func parseStyles(root *xEl, th docxTheme) *wStyles {
	s := &wStyles{byID: map[string]*wStyle{}, paraCache: map[string]wPara{},
		runCache: map[[2]string]wRun{}, chainCache: map[string][]*wStyle{}, headingCache: map[string]int{}}
	if root == nil {
		return s
	}
	if dd := root.child("w", "docDefaults"); dd != nil {
		s.defP = parsePPr(dd.child("w", "pPrDefault").child("w", "pPr"))
		s.defR = parseRPr(dd.child("w", "rPrDefault").child("w", "rPr"), th)
	}
	for _, st := range root.childrenNamed("w", "style") {
		ws := &wStyle{
			id:        st.attr("styleId"),
			kind:      st.attr("type"),
			name:      st.child("w", "name").attr("val"),
			basedOn:   st.child("w", "basedOn").attr("val"),
			isDefault: wOnAttr(st, "default") && st.hasAttr("default"),
			p:         parsePPr(st.child("w", "pPr")),
			r:         parseRPr(st.child("w", "rPr"), th),
		}
		if ws.id == "" {
			continue
		}
		if ws.kind == "" {
			ws.kind = "paragraph"
		}
		if ws.name == "" {
			ws.name = ws.id
		}
		s.byID[ws.id] = ws
		if ws.isDefault && ws.kind == "paragraph" && s.defaultPara == "" {
			s.defaultPara = ws.id
		}
		if ws.isDefault && ws.kind == "character" && s.defaultChar == "" {
			s.defaultChar = ws.id
		}
	}
	return s
}

// chain is the style and its bases, base first; unknown ids give nil.
func (s *wStyles) chain(id string) []*wStyle {
	if c, ok := s.chainCache[id]; ok {
		return c
	}
	var rev []*wStyle
	seen := map[string]bool{}
	for cur := id; cur != "" && !seen[cur] && len(rev) < 30; {
		st := s.byID[cur]
		if st == nil {
			break
		}
		seen[cur] = true
		rev = append(rev, st)
		cur = st.basedOn
	}
	out := make([]*wStyle, len(rev))
	for i, st := range rev {
		out[len(rev)-1-i] = st
	}
	s.chainCache[id] = out
	return out
}

// paraStyleID gives the paragraph style a paragraph uses (the default one
// when it names none or an unknown one).
func (s *wStyles) paraStyleID(id string) string {
	if st := s.byID[id]; st != nil && st.kind == "paragraph" {
		return id
	}
	return s.defaultPara
}

// para: document defaults < the style chain.
func (s *wStyles) para(styleID string) wPara {
	if p, ok := s.paraCache[styleID]; ok {
		return p
	}
	p := s.defP
	for _, st := range s.chain(styleID) {
		p.over(st.p)
	}
	s.paraCache[styleID] = p
	return p
}

// run: document defaults < paragraph style chain < character style chain.
func (s *wStyles) run(paraStyle, charStyle string) wRun {
	key := [2]string{paraStyle, charStyle}
	if r, ok := s.runCache[key]; ok {
		return r
	}
	r := s.defR
	for _, st := range s.chain(paraStyle) {
		r.over(st.r)
	}
	if st := s.byID[charStyle]; st != nil && st.kind == "character" {
		for _, c := range s.chain(charStyle) {
			r.over(c.r)
		}
	}
	s.runCache[key] = r
	return r
}

// headingLevel of a paragraph style: from its name ("heading N", how Word
// names its built-ins whatever the language of the styleId).
func (s *wStyles) headingLevel(styleID string) int {
	if lv, ok := s.headingCache[styleID]; ok {
		return lv
	}
	lv := 0
	ch := s.chain(styleID)
	for i := len(ch) - 1; i >= 0 && lv == 0; i-- {
		lv = headingNameLevel(ch[i].name)
	}
	s.headingCache[styleID] = lv
	return lv
}

func headingNameLevel(name string) int {
	n := strings.ToLower(strings.TrimSpace(name))
	if strings.HasPrefix(n, "heading ") {
		if v, err := strconv.Atoi(strings.TrimSpace(n[8:])); err == nil && v >= 1 && v <= 9 {
			return v
		}
	}
	return 0
}

// --- concrete formatting ---

// pFmt is a paragraph's formatting with every value known.
type pFmt struct {
	align                       string
	indFirst, indLeft, indRight float64
	before, after               float64
	lineRule                    string
	line                        float64
	keepNext                    bool
}

// rFmt is a run's formatting with every value known.
type rFmt struct {
	bold, italic, underline, strike, smallCaps, caps bool
	vert                                             string // "", "superscript", "subscript"
	font                                             string
	size                                             float64
	color, highlight                                 string // "#rrggbb" or ""
}

func concretePara(p wPara) pFmt {
	f := pFmt{align: "left", lineRule: "auto", line: 1}
	deref := func(dst *float64, v *float64) {
		if v != nil {
			*dst = *v
		}
	}
	if p.align != nil {
		f.align = *p.align
	}
	deref(&f.indFirst, p.indFirst)
	deref(&f.indLeft, p.indLeft)
	deref(&f.indRight, p.indRight)
	deref(&f.before, p.before)
	deref(&f.after, p.after)
	if p.line != nil {
		f.lineRule, f.line = p.line.rule, p.line.val
	}
	if p.keepNext != nil {
		f.keepNext = *p.keepNext
	}
	return f
}

// concreteRun resolves a run. Word picks the ascii font for ASCII text and
// the hAnsi one for the rest; a run of only non-ASCII characters (symbols,
// accented words set in another font) takes hAnsi.
func concreteRun(r wRun, text string) rFmt {
	b := func(v *bool) bool { return v != nil && *v }
	f := rFmt{bold: b(r.bold), italic: b(r.italic), underline: b(r.underline), strike: b(r.strike),
		smallCaps: b(r.smallCaps), caps: b(r.caps), size: 10, font: "Times New Roman"}
	if r.vert != nil && (*r.vert == "superscript" || *r.vert == "subscript") {
		f.vert = *r.vert
	}
	ascii, hansi := "", ""
	if r.asciiFont != nil {
		ascii = *r.asciiFont
	}
	if r.hAnsiFont != nil {
		hansi = *r.hAnsiFont
	}
	useH := hansi != "" && text != ""
	if useH {
		for _, c := range text {
			if c < 0x80 && c != ' ' {
				useH = false
				break
			}
		}
	}
	switch {
	case useH:
		f.font = hansi
	case ascii != "":
		f.font = ascii
	case hansi != "":
		f.font = hansi
	}
	if r.size != nil {
		f.size = *r.size
	}
	if r.color != nil {
		f.color = *r.color
	}
	if r.highlight != nil && *r.highlight != "" {
		f.highlight = *r.highlight
	} else if r.shade != nil {
		f.highlight = *r.shade
	}
	if f.caps {
		f.smallCaps = false
	}
	return f
}

// --- numbering ---

type wNumLevel struct {
	format string // decimal, bullet, lowerRoman…, none
	text   string // "%1."
	start  int
	suff   string // tab, space, nothing
}

type wNumbering struct {
	levels map[int]map[int]*wNumLevel // numId -> ilvl -> level
}

func parseNumLevel(l *xEl, into *wNumLevel) {
	if v := l.child("w", "numFmt"); v != nil {
		into.format = v.attr("val")
	}
	if v := l.child("w", "lvlText"); v != nil {
		into.text = v.attr("val")
	}
	if v, ok := intAttr(l.child("w", "start"), "val"); ok {
		into.start = v
	}
	if v := l.child("w", "suff"); v != nil {
		into.suff = v.attr("val")
	}
}

func parseNumbering(root *xEl) *wNumbering {
	n := &wNumbering{levels: map[int]map[int]*wNumLevel{}}
	if root == nil {
		return n
	}
	abstract := map[string]map[int]wNumLevel{}
	for _, a := range root.childrenNamed("w", "abstractNum") {
		lv := map[int]wNumLevel{}
		for _, l := range a.childrenNamed("w", "lvl") {
			il, ok := intAttr(l, "ilvl")
			if !ok {
				continue
			}
			x := wNumLevel{format: "decimal", start: 1, suff: "tab"}
			parseNumLevel(l, &x)
			lv[il] = x
		}
		abstract[a.attr("abstractNumId")] = lv
	}
	for _, num := range root.childrenNamed("w", "num") {
		id, ok := intAttr(num, "numId")
		if !ok {
			continue
		}
		levels := map[int]*wNumLevel{}
		for il, l := range abstract[num.child("w", "abstractNumId").attr("val")] {
			c := l
			levels[il] = &c
		}
		for _, ov := range num.childrenNamed("w", "lvlOverride") {
			il, ok := intAttr(ov, "ilvl")
			if !ok {
				continue
			}
			l := levels[il]
			if l == nil {
				l = &wNumLevel{format: "decimal", start: 1, suff: "tab"}
				levels[il] = l
			}
			if lvl := ov.child("w", "lvl"); lvl != nil {
				parseNumLevel(lvl, l)
			}
			if v, ok := intAttr(ov.child("w", "startOverride"), "val"); ok {
				l.start = v
			}
		}
		n.levels[id] = levels
	}
	return n
}

func (n *wNumbering) level(numID, ilvl int) *wNumLevel {
	if n == nil || n.levels[numID] == nil {
		return nil
	}
	return n.levels[numID][ilvl]
}

// numberLabel formats a level's number as Word shows it.
func numberLabel(format string, v int) string {
	switch format {
	case "upperRoman":
		return romanNumeral(v)
	case "lowerRoman":
		return strings.ToLower(romanNumeral(v))
	case "upperLetter":
		return letterNumeral(v)
	case "lowerLetter":
		return strings.ToLower(letterNumeral(v))
	case "decimalZero":
		if v < 10 {
			return "0" + strconv.Itoa(v)
		}
	}
	return strconv.Itoa(v)
}

func romanNumeral(v int) string {
	if v <= 0 || v >= 4000 {
		return strconv.Itoa(v)
	}
	vals := []int{1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1}
	syms := []string{"M", "CM", "D", "CD", "C", "XC", "L", "XL", "X", "IX", "V", "IV", "I"}
	var b strings.Builder
	for i, n := range vals {
		for v >= n {
			b.WriteString(syms[i])
			v -= n
		}
	}
	return b.String()
}

// letterNumeral: A…Z, then AA…ZZ (Word repeats the letter).
func letterNumeral(v int) string {
	if v <= 0 {
		return strconv.Itoa(v)
	}
	ch := string(rune('A' + (v-1)%26))
	return strings.Repeat(ch, (v-1)/26+1)
}
