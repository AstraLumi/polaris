package main

// Footnotes and endnotes in a .docx import (docx.go; stage 5, docs/EDITOR.md
// "Footnotes and endnotes"). word/footnotes.xml and word/endnotes.xml (by
// the document's relationships) hold the notes by id; a run's
// w:footnoteReference / w:endnoteReference becomes a `footnote` node there
// (kind footnote or endnote) holding the note's paragraphs, read like the
// body's (formatting, links through the note part's own relationships,
// character styles, fields, text boxes) but:
//   - the note's own number (w:footnoteRef / w:endnoteRef) and the space
//     after it are dropped: numbers are never stored;
//   - every paragraph is a plain paragraph with no style (headings, list
//     numbers, scene breaks and bookmarks don't exist in a note); its
//     formatting and its text's are kept as far as they differ from its
//     Word style (Footnote Text, usually), which the editor draws as a
//     note's look (Normal's font at 5/6 of its size, no indent);
//   - tables in a note keep their cells' paragraphs, in reading order;
//     pictures, page breaks and comments in a note are left out (pictures
//     counted in left_out.pictures, comments in left_out.comments as
//     marking no text); a note inside a note is left out.
// left_out.footnotes / endnotes now count only references to notes the
// file doesn't have, and notes inside notes. Word's separator and
// continuation notes (w:type) are never referenced from the text.

import "strings"

// irNote is a note read from the file: its kind and its paragraphs, each
// with its Word style's formatting (what its own is measured against).
type irNote struct {
	kind  string // "footnote" or "endnote"
	paras []*irBlock
	bases []outStyle
}

// docxNotes is the file's notes by kind and id, with each part's
// relationships.
type docxNotes struct {
	byID map[string]map[string]*xEl
	rels map[string]map[string]docxRel
}

func parseDocxNotes(pk *docxPackage, rels map[string]docxRel) (*docxNotes, error) {
	out := &docxNotes{byID: map[string]map[string]*xEl{}, rels: map[string]map[string]docxRel{}}
	for _, kind := range []string{"footnote", "endnote"} {
		part := relOfType(rels, "/"+kind+"s")
		if part == "" {
			continue
		}
		root, err := pk.xml(part)
		if err != nil {
			return nil, err
		}
		if root == nil {
			continue
		}
		r, err := pk.rels(part)
		if err != nil {
			return nil, err
		}
		notes := map[string]*xEl{}
		for _, el := range root.childrenNamed("w", kind) {
			if t := el.attr("type"); t != "" && t != "normal" {
				continue // Word's separators
			}
			if id := el.attr("id"); id != "" && notes[id] == nil {
				notes[id] = el
			}
		}
		out.byID[kind], out.rels[kind] = notes, r
	}
	return out, nil
}

// noteRef reads the note a reference points at into the paragraph, as a
// note inline (kind 'f').
func (p *docxParser) noteRef(kind, id string, ps *paraState) {
	var el *xEl
	if p.notes != nil {
		el = p.notes.byID[kind][id]
	}
	if el == nil || p.inNote {
		p.count(kind + "s")
		return
	}
	var blocks []*irBlock
	saved := struct {
		out       *[]*irBlock
		fields    []*docxField
		marks     []string
		rels      map[string]docxRel
		openCm    []*docxCmt
		pendingCm []*docxCmt
		tocGap    bool
	}{p.out, p.fields, p.pendingMark, p.rels, p.openCm, p.pendingCm, p.tocGap}
	p.out, p.fields, p.pendingMark, p.rels, p.openCm, p.pendingCm = &blocks, nil, nil, p.notes.rels[kind], nil, nil
	p.inNote = true
	p.walkBlocks(el.kids)
	p.inNote = false
	p.out, p.fields, p.pendingMark, p.rels, p.openCm, p.pendingCm, p.tocGap = saved.out, saved.fields, saved.marks, saved.rels, saved.openCm, saved.pendingCm, saved.tocGap

	note := &irNote{kind: kind}
	for _, b := range irFlat(blocks) {
		if b.kind != 'p' {
			continue // pictures (counted), page breaks; a table's cells follow it
		}
		b.level, b.list, b.bookmarks = 0, nil, nil
		note.paras = append(note.paras, b)
		note.bases = append(note.bases, outStyle{id: "normal", para: concretePara(p.st.para(b.style)), text: concreteRun(p.st.run(b.style, ""), "")})
	}
	// The space Word puts after the note's number.
	for len(note.paras) > 0 {
		first := note.paras[0]
		for len(first.inl) > 0 && first.inl[0].kind == 't' {
			first.inl[0].text = strings.TrimLeft(first.inl[0].text, " \t ")
			if first.inl[0].text != "" {
				break
			}
			first.inl = first.inl[1:]
		}
		if len(first.inl) > 0 && first.inl[0].kind == '\t' {
			first.inl = first.inl[1:]
			continue
		}
		break
	}
	if len(note.paras) == 0 {
		note.paras = []*irBlock{{kind: 'p'}}
		note.bases = []outStyle{{id: "normal"}}
	}
	ps.cur = append(ps.cur, irInline{kind: 'f', note: note})
}

// noteNode makes a note's node: its paragraphs, each measured against its
// Word style (so they carry no style of their own).
func noteNode(note *irNote, set *styleSet) *pmNode {
	n := &pmNode{Type: "footnote", Attrs: map[string]any{"kind": note.kind}}
	for i, b := range note.paras {
		n.Content = append(n.Content, paraNodeIn(b, &note.bases[i], set, nil))
	}
	return n
}
