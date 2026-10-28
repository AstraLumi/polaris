package main

// Footnotes and endnotes (stage 5; docs/EDITOR.md, "Footnotes and
// endnotes") in the exports. A note is a `footnote` node (pmdoc.go): its
// kind (footnote or endnote) and its paragraphs. Numbers are never
// stored: in each chapter, footnotes count 1, 2, 3… and endnotes i, ii,
// iii… (Word's default formats), in reading order (docNotes; the editor's
// footnotes.js counts the same way).
//
//   - .docx: real Word notes. The reference is a FootnoteReference /
//     EndnoteReference run (superscript) holding w:footnoteReference /
//     w:endnoteReference; the note's paragraphs go in word/footnotes.xml or
//     word/endnotes.xml (after Word's separator and continuation separator,
//     ids -1 and 0), in the Footnote Text / Endnote Text style (Normal at
//     5/6 of its size on the half point, no first-line indent, no
//     spacing), the first starting with the footnoteRef / endnoteRef mark
//     and a space. A note's web links get relationships of the note part.
//     settings.xml restarts the numbers in each section (each chapter is
//     one) and puts endnotes at the end of each section, so Word lays the
//     book out as the page view does. The import (docxnotes.go) reads them
//     back the same.
//   - EPUB: a noteref link (epub:type="noteref", role doc-noteref) in the
//     text, and at the chapter's end its footnotes as
//     <aside epub:type="footnote"> (readers that can show them as pop-ups
//     do) then its endnotes in a <section epub:type="endnotes">, each with
//     a link back to its reference.
//   - Online Post (HTML, Markdown, text): the number in the text (<sup>1</sup>,
//     [^1], [1]) and every note at the chapter's end, footnotes first then
//     endnotes: HTML a rule then one paragraph per note paragraph starting
//     with its number; Markdown footnote definitions ([^1]: …; when the
//     export holds several chapters a label is "<chapter>-<number>", as
//     Markdown labels must be unique); text "[1] …" lines.

import (
	"fmt"
	"math"
	"strings"
)

// docNote is one note of a chapter, numbered.
type docNote struct {
	Node    *pmNode
	Kind    string // "footnote" or "endnote"
	N       int
	Label   string // "3" or "iii"
	Chapter int    // the chapter's index in the export
}

func noteKindOf(n *pmNode) string {
	if k, _ := n.Attrs["kind"].(string); k == "endnote" {
		return "endnote"
	}
	return "footnote"
}

func noteLabelOf(kind string, n int) string {
	if kind == "endnote" {
		return numberLabel("lowerRoman", n)
	}
	return fmt.Sprint(n)
}

// docNotes lists a doc's notes in reading order, numbered per kind.
func docNotes(doc *pmNode) []docNote {
	var out []docNote
	count := map[string]int{}
	var walk func(n *pmNode)
	walk = func(n *pmNode) {
		if n == nil {
			return
		}
		if n.Type == "footnote" {
			k := noteKindOf(n)
			count[k]++
			out = append(out, docNote{Node: n, Kind: k, N: count[k], Label: noteLabelOf(k, count[k])})
			return
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(doc)
	return out
}

// noteSizeOf is a note's text size: 5/6 of the body's, on the half point
// (css.js's noteSize).
func noteSizeOf(body float64) float64 {
	if body <= 0 {
		body = 10.5
	}
	return math.Max(4, math.Round(body*5/6*2)/2)
}

// noteOf gives a note's number in its chapter (worked out once for the
// whole export).
func (b *exportBook) noteOf(n *pmNode) docNote {
	if b.notes == nil {
		b.notes = map[*pmNode]docNote{}
		for ci, c := range b.Chapters {
			for _, x := range docNotes(c.Doc) {
				x.Chapter = ci
				b.notes[x.Node] = x
			}
		}
	}
	return b.notes[n]
}

// notesInReadingOrder: a chapter's footnotes, then its endnotes (where an
// online post and the EPUB put them).
func notesInReadingOrder(doc *pmNode) []docNote {
	all := docNotes(doc)
	var out []docNote
	for _, kind := range []string{"footnote", "endnote"} {
		for _, x := range all {
			if x.Kind == kind {
				out = append(out, x)
			}
		}
	}
	return out
}

// --- .docx ---

// docxNotesOut holds the notes written so far.
type docxNotesOut struct {
	parts map[string]*strings.Builder // "footnote" / "endnote" → the part's notes
	rels  map[string]*strings.Builder // their relationships (web links)
	next  map[string]int              // the next id of each kind
}

func (d *docxOut) notesOut() *docxNotesOut {
	if d.notes == nil {
		d.notes = &docxNotesOut{parts: map[string]*strings.Builder{}, rels: map[string]*strings.Builder{}, next: map[string]int{"footnote": 1, "endnote": 1}}
	}
	return d.notes
}

var noteWordNames = map[string]struct{ ref, text, refName, textName, el string }{
	"footnote": {"FootnoteReference", "FootnoteText", "footnote reference", "footnote text", "footnote"},
	"endnote":  {"EndnoteReference", "EndnoteText", "endnote reference", "endnote text", "endnote"},
}

// noteRef writes a note's reference run in the text and its paragraphs in
// the notes part.
func (d *docxOut) noteRef(p *docxPara, n *pmNode) {
	no := d.notesOut()
	kind := noteKindOf(n)
	w := noteWordNames[kind]
	id := no.next[kind]
	no.next[kind]++
	fmt.Fprintf(&p.body, `<w:r><w:rPr><w:rStyle w:val="%s"/></w:rPr><w:%sReference w:id="%d"/></w:r>`, w.ref, w.el, id)

	part := no.parts[kind]
	if part == nil {
		part = &strings.Builder{}
		no.parts[kind] = part
		no.rels[kind] = &strings.Builder{}
	}
	// Web links in the note are relationships of the note part (d.rel).
	d.noteRels = no.rels[kind]
	fmt.Fprintf(part, `<w:%s w:id="%d">`, w.el, id)
	for i, para := range n.Content {
		if para == nil {
			continue
		}
		np := &docxPara{}
		fmt.Fprintf(&np.ppr, `<w:pStyle w:val="%s"/>`, w.text)
		writeParaBox(&np.ppr, para.Attrs, nil)
		if i == 0 {
			fmt.Fprintf(&np.body, `<w:r><w:rPr><w:rStyle w:val="%s"/></w:rPr><w:%sRef/></w:r><w:r><w:t xml:space="preserve"> </w:t></w:r>`, w.ref, w.el)
		}
		d.inlines(np, para.Content, "normal")
		part.WriteString(`<w:p><w:pPr>` + np.ppr.String() + `</w:pPr>` + np.body.String() + `</w:p>`)
	}
	fmt.Fprintf(part, `</w:%s>`, w.el)
	d.noteRels = nil
}

// noteParts adds footnotes.xml and endnotes.xml (and their relationships)
// to the package when the book has notes.
func (d *docxOut) noteParts() {
	if d.notes == nil {
		return
	}
	const wml = "application/vnd.openxmlformats-officedocument.wordprocessingml."
	for _, kind := range []string{"footnote", "endnote"} {
		part := d.notes.parts[kind]
		if part == nil {
			continue
		}
		w := noteWordNames[kind]
		sep := `<w:p><w:pPr><w:spacing w:after="0" w:line="240" w:lineRule="auto"/></w:pPr><w:r><w:%s/></w:r></w:p>`
		var x strings.Builder
		x.WriteString(xmlHead + `<w:` + w.el + `s ` + docxNSDecl + `>`)
		fmt.Fprintf(&x, `<w:%s w:type="separator" w:id="-1">`+sep+`</w:%s>`, w.el, "separator", w.el)
		fmt.Fprintf(&x, `<w:%s w:type="continuationSeparator" w:id="0">`+sep+`</w:%s>`, w.el, "continuationSeparator", w.el)
		x.WriteString(part.String())
		x.WriteString(`</w:` + w.el + `s>`)
		name := w.el + "s.xml"
		d.rel(w.el+"s", name, false)
		d.parts = append(d.parts, docxPart{name: "word/" + name, contentType: wml + w.el + "s+xml", data: []byte(x.String())})
		if rels := d.notes.rels[kind].String(); rels != "" {
			d.parts = append(d.parts, docxPart{name: "word/_rels/" + name + ".rels", data: []byte(xmlHead +
				`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` + rels + `</Relationships>`)})
		}
	}
}

// noteSettingsXML: numbers restart in each section (chapter), endnotes at
// each section's end; Word's separators listed. Empty without notes.
func (d *docxOut) noteSettingsXML() string {
	if d.notes == nil {
		return ""
	}
	var s strings.Builder
	if d.notes.parts["footnote"] != nil {
		s.WriteString(`<w:footnotePr><w:numRestart w:val="eachSect"/><w:footnote w:id="-1"/><w:footnote w:id="0"/></w:footnotePr>`)
	}
	if d.notes.parts["endnote"] != nil {
		s.WriteString(`<w:endnotePr><w:pos w:val="sectEnd"/><w:numFmt w:val="lowerRoman"/><w:numRestart w:val="eachSect"/><w:endnote w:id="-1"/><w:endnote w:id="0"/></w:endnotePr>`)
	}
	return s.String()
}

// noteStylesXML: the styles of the kinds of notes the book has (their ids
// are reserved: no book style takes them, mapWordStyles).
func (d *docxOut) noteStylesXML() string {
	if d.notes == nil {
		return ""
	}
	var s strings.Builder
	size := int(math.Round(noteSizeOf(d.b.bodySize()) * 2))
	for _, kind := range []string{"footnote", "endnote"} {
		if d.notes.parts[kind] == nil {
			continue
		}
		w := noteWordNames[kind]
		fmt.Fprintf(&s, `<w:style w:type="paragraph" w:styleId="%s"><w:name w:val="%s"/><w:basedOn w:val="Normal"/><w:uiPriority w:val="99"/><w:unhideWhenUsed/>`+
			`<w:pPr><w:spacing w:before="0" w:after="0"/><w:ind w:left="0" w:right="0" w:firstLine="0"/></w:pPr><w:rPr><w:sz w:val="%d"/><w:szCs w:val="%d"/></w:rPr></w:style>`,
			w.text, w.textName, size, size)
		fmt.Fprintf(&s, `<w:style w:type="character" w:styleId="%s"><w:name w:val="%s"/><w:uiPriority w:val="99"/><w:unhideWhenUsed/><w:rPr><w:vertAlign w:val="superscript"/></w:rPr></w:style>`,
			w.ref, w.refName)
	}
	return s.String()
}

// --- EPUB ---

// noteIDs gives each note its ids in its chapter's file: the reference's
// and the note's.
func (e *epubOut) noteIDsFor(chapters []exportChapter) {
	e.noteIDs = map[*pmNode][2]string{}
	for i, c := range chapters {
		for _, x := range docNotes(c.Doc) {
			p := "fn"
			if x.Kind == "endnote" {
				p = "en"
			}
			ref := uniqueID(e.ids[i], fmt.Sprintf("%sref-%s", p, x.Label))
			e.noteIDs[x.Node] = [2]string{ref, uniqueID(e.ids[i], fmt.Sprintf("%s-%s", p, x.Label))}
		}
	}
}

func (e *epubOut) noteRef(s *strings.Builder, n *pmNode) {
	ids := e.noteIDs[n]
	label := e.b.noteOf(n).Label
	fmt.Fprintf(s, `<a epub:type="noteref" role="doc-noteref" class="noteref" id="%s" href="#%s"><sup>%s</sup></a>`, ids[0], ids[1], xe(label))
}

// chapterNotesXHTML writes a chapter's notes at its end.
func (e *epubOut) chapterNotesXHTML(s *strings.Builder, doc *pmNode) {
	notes := notesInReadingOrder(doc)
	if len(notes) == 0 {
		return
	}
	writeNote := func(x docNote, typ string) {
		ids := e.noteIDs[x.Node]
		fmt.Fprintf(s, `<aside epub:type="%s" role="doc-%s" class="note" id="%s">`, typ, typ, ids[1])
		for i, p := range x.Node.Content {
			if p == nil {
				continue
			}
			s.WriteString(`<p class="note-p">`)
			if i == 0 {
				fmt.Fprintf(s, `<a role="doc-backlink" class="note-num" href="#%s">%s</a> `, ids[0], xe(x.Label))
			}
			e.inlines(s, p.Content, "normal")
			s.WriteString("</p>")
		}
		s.WriteString("</aside>\n")
	}
	var foot, end []docNote
	for _, x := range notes {
		if x.Kind == "endnote" {
			end = append(end, x)
		} else {
			foot = append(foot, x)
		}
	}
	if len(foot) > 0 {
		s.WriteString(`<section class="notes footnotes">` + "\n")
		for _, x := range foot {
			writeNote(x, "footnote")
		}
		s.WriteString("</section>\n")
	}
	if len(end) > 0 {
		s.WriteString(`<section class="notes endnotes" epub:type="endnotes" role="doc-endnotes">` + "\n")
		for _, x := range end {
			writeNote(x, "endnote")
		}
		s.WriteString("</section>\n")
	}
}

func epubNotesCSS() string {
	return `.noteref { text-decoration: none; }
.notes { margin-top: 2em; padding-top: 0.5em; border-top: 1px solid currentColor; }
.endnotes { margin-top: 1em; }
.note { margin: 0 0 0.4em; }
.note-p { text-indent: 0; font-size: 0.83em; }
.note-num { text-decoration: none; }
`
}

// --- Online Post ---

// postNoteKey is a note's Markdown label: its number, or
// "<chapter>-<number>" when the export holds several chapters.
func (b *exportBook) postNoteKey(x docNote) string {
	if len(b.Chapters) > 1 {
		return fmt.Sprintf("%d-%s", x.Chapter+1, x.Label)
	}
	return x.Label
}

// postNoteSegs: a note's paragraphs as segments.
func (b *exportBook) postNoteParas(x docNote) [][]postSeg {
	var out [][]postSeg
	for _, p := range x.Node.Content {
		if p != nil {
			out = append(out, b.postSegs(p.Content, "normal", exText{}))
		}
	}
	return out
}

// htmlNotes: a rule, then each note's paragraphs, the first starting with
// its number.
func (h *htmlPost) notes(doc *pmNode) {
	notes := notesInReadingOrder(doc)
	if len(notes) == 0 {
		return
	}
	h.s.WriteString("<hr>\n")
	for _, x := range notes {
		for i, segs := range h.b.postNoteParas(x) {
			lead := ""
			if i == 0 {
				lead = "<sup>" + he(x.Label) + "</sup> "
			}
			h.s.WriteString("<p>" + lead + htmlInline(segs) + "</p>\n")
		}
	}
}

// mdNotes: Markdown footnote definitions; a note's later paragraphs
// indented four spaces under it.
func (m *mdPost) notes(doc *pmNode, ci int) string {
	var defs []string
	for _, x := range notesInReadingOrder(doc) {
		x.Chapter = ci
		var paras []string
		for _, segs := range m.b.postNoteParas(x) {
			paras = append(paras, mdInline(segs))
		}
		body := strings.Join(paras, "\n\n")
		defs = append(defs, indentLines("[^"+m.b.postNoteKey(x)+"]: "+body, "", "    "))
	}
	return strings.Join(defs, "\n\n")
}

// textNotes: "[1] …" per note, its paragraphs one under another.
func (b *exportBook) textNotes(doc *pmNode) string {
	var lines []string
	for _, x := range notesInReadingOrder(doc) {
		var paras []string
		for _, segs := range b.postNoteParas(x) {
			paras = append(paras, textOf(segs))
		}
		lines = append(lines, indentLines("["+x.Label+"] "+strings.Join(paras, "\n"), "", "    "))
	}
	return strings.Join(lines, "\n")
}
