package main

// Word comments, read by the .docx import and written by the export
// (docs/EDITOR.md, "Comments (stage 5)"; comments.go keeps them).
//
// Import:
//   - word/comments.xml (the main part's "comments" relationship): each
//     w:comment's text (its paragraphs joined by new lines, tabs and line
//     breaks kept, formatting dropped) and w:date; commentsExtended.xml
//     (w15:commentEx by the paraId of the comment's last paragraph): done
//     is resolved, paraIdParent makes it a reply (a reply to a reply joins
//     the first comment's thread).
//   - In the body, commentRangeStart / commentRangeEnd (between paragraphs
//     or inside them) open and close a comment: the text between them gets
//     a `comment` mark, across paragraphs too. A reply's range is ignored
//     (a reply goes with its thread). A comment with no text in its range (a
//     point, or only a commentReference) marks the word before it in its
//     paragraph, else the next word.
//   - A range that crosses a Heading 1 belongs to the chapter where its text
//     starts; its marks in later chapters are dropped. A comment that marks
//     nothing (and its replies) is counted in left_out.comments, as is a
//     reference to a comment the file doesn't have.
//   - While parsing, marks carry the comment's index in docxImport.Comments
//     plus one; the import handler makes the rows and puts their ids in
//     (mapCommentMarks).
//
// Export:
//   - comments.xml with every comment of the book's chapters whose text is
//     in the book (a comment whose mark is gone is left out), its replies
//     after it; author the book's meta author (else "Polaris"), the date it
//     was made; commentsExtended.xml with done for a resolved thread and
//     paraIdParent for replies.
//   - commentRangeStart before the first text the comment marks and
//     commentRangeEnd after its last, then a run with its commentReference;
//     a reply's range and reference sit beside its comment's. Text between
//     two parts of one comment is inside its range (Word's ranges are one
//     stretch).

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// docxComment is one comment of an imported file.
type docxComment struct {
	Local    int64 // its index in docxImport.Comments plus one: the marks' id until stored
	Chapter  int   // the chapter (index in Chapters) it belongs to
	Parent   int64 // Local of the comment it answers, 0 for none
	Text     string
	Resolved bool
	Created  time.Time // zero when the file doesn't say
}

// docxCmt is a comment while the body is read.
type docxCmt struct {
	docxComment
	wordID, paraID string
	parent         *docxCmt
	started        bool // its range opened
	placed         bool // a point comment waiting for, or given, a word
	marked         bool // some text carries it
}

type docxComments struct {
	byWord map[string]*docxCmt
	list   []*docxCmt
}

// parseDocxComments reads comments.xml and commentsExtended.xml (nil when
// the file has no comments).
func parseDocxComments(pk *docxPackage, rels map[string]docxRel) (*docxComments, error) {
	part := relOfType(rels, "/comments")
	if part == "" {
		return nil, nil
	}
	root, err := pk.xml(part)
	if err != nil || root == nil {
		return nil, err
	}
	cs := &docxComments{byWord: map[string]*docxCmt{}}
	byPara := map[string]*docxCmt{}
	for _, el := range root.childrenNamed("w", "comment") {
		id := el.attr("id")
		if id == "" || cs.byWord[id] != nil {
			continue
		}
		c := &docxCmt{wordID: id}
		c.Local = int64(len(cs.list) + 1)
		c.Created = parseWordDate(el.attr("date"))
		var paras []string
		el.walk(func(x *xEl) bool {
			if !x.is("w", "p") {
				return true
			}
			paras = append(paras, docxCommentPara(x))
			if pid := x.attr("paraId"); pid != "" {
				c.paraID = pid
			}
			return false
		})
		c.Text, _ = cleanCommentText(strings.Join(paras, "\n"))
		if r := []rune(c.Text); len(r) > maxCommentText {
			c.Text = string(r[:maxCommentText])
		}
		cs.list = append(cs.list, c)
		cs.byWord[id] = c
		if c.paraID != "" {
			byPara[strings.ToUpper(c.paraID)] = c
		}
	}
	// Resolved and replies.
	if ext := relOfType(rels, "/commentsExtended"); ext != "" {
		exRoot, err := pk.xml(ext)
		if err != nil {
			return nil, err
		}
		if exRoot != nil {
			for _, x := range exRoot.kids {
				if x.name != "commentEx" {
					continue
				}
				c := byPara[strings.ToUpper(x.attr("paraId"))]
				if c == nil {
					continue
				}
				switch x.attr("done") {
				case "1", "true", "on":
					c.Resolved = true
				}
				if pp := byPara[strings.ToUpper(x.attr("paraIdParent"))]; pp != nil && pp != c {
					c.parent = pp
				}
			}
		}
	}
	// A reply's thread is its first comment's (and never a loop).
	for _, c := range cs.list {
		seen := map[*docxCmt]bool{c: true}
		for c.parent != nil && c.parent.parent != nil && !seen[c.parent.parent] {
			seen[c.parent] = true
			c.parent = c.parent.parent
		}
		if c.parent != nil && c.parent.parent != nil {
			c.parent = nil
		}
	}
	return cs, nil
}

// docxCommentPara is a comment paragraph's plain text.
func docxCommentPara(p *xEl) string {
	var b strings.Builder
	p.walk(func(x *xEl) bool {
		switch {
		case x.is("w", "t"):
			b.WriteString(x.text)
		case x.is("w", "tab"):
			b.WriteByte('\t')
		case x.is("w", "br"), x.is("w", "cr"):
			b.WriteByte('\n')
		case x.is("w", "del"), x.is("w", "moveFrom"):
			return false
		}
		return true
	})
	return b.String()
}

func parseWordDate(s string) time.Time {
	s = strings.TrimSpace(s)
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

// --- while reading the body (docxbody.go) ---

func (p *docxParser) comment(wordID string) *docxCmt {
	if p.cm == nil {
		return nil
	}
	return p.cm.byWord[wordID]
}

func (p *docxParser) commentStart(wordID string) {
	if p.inNote { // no comments in a note (docxnotes.go)
		return
	}
	c := p.comment(wordID)
	if c == nil || c.parent != nil || c.started {
		return
	}
	c.started = true
	p.openCm = append(p.openCm, c)
}

func (p *docxParser) commentEnd(wordID string, ps *paraState) {
	if p.inNote {
		return
	}
	c := p.comment(wordID)
	if c == nil || c.parent != nil {
		return
	}
	if i := slices.Index(p.openCm, c); i >= 0 {
		p.openCm = slices.Delete(p.openCm, i, i+1)
	}
	if !c.marked && !c.placed {
		p.pointComment(c, ps)
	}
}

// commentRef is a commentReference run: a comment with no range of its own
// is a point.
func (p *docxParser) commentRef(wordID string, ps *paraState) {
	if p.inNote {
		return
	}
	c := p.comment(wordID)
	if c == nil {
		p.count("comments") // nothing in comments.xml to import
		return
	}
	if c.parent != nil {
		return
	}
	if c.started && slices.Contains(p.openCm, c) {
		p.commentEnd(wordID, ps)
		return
	}
	if !c.marked && !c.placed {
		p.pointComment(c, ps)
	}
}

// pointComment gives a comment with no text the word before it in its
// paragraph, else the next word read.
func (p *docxParser) pointComment(c *docxCmt, ps *paraState) {
	c.placed = true
	if ps != nil {
		for i := len(ps.cur) - 1; i >= 0; i-- {
			in := ps.cur[i]
			if in.kind != 't' {
				continue
			}
			end := len(strings.TrimRightFunc(in.text, unicode.IsSpace))
			if end == 0 {
				continue
			}
			start := strings.LastIndexFunc(in.text[:end], unicode.IsSpace) + 1
			var parts []irInline
			if start > 0 {
				head := in
				head.text = in.text[:start]
				parts = append(parts, head)
			}
			mid := in
			mid.text = in.text[start:end]
			mid.comments = addCommentKey(in.comments, c.Local)
			parts = append(parts, mid)
			if end < len(in.text) {
				tail := in
				tail.text = in.text[end:]
				parts = append(parts, tail)
			}
			ps.cur = slices.Concat(ps.cur[:i], parts, ps.cur[i+1:])
			c.marked = true
			return
		}
	}
	p.pendingCm = append(p.pendingCm, c)
}

// commentKey is the comments open now, as an irInline's key.
func (p *docxParser) commentKey() string {
	key := ""
	for _, c := range p.openCm {
		key = addCommentKey(key, c.Local)
	}
	return key
}

// markCommented notes that text carries the comments of key.
func (p *docxParser) markCommented(key string) {
	if p.cm == nil {
		return
	}
	for _, id := range commentKeyIDs(key) {
		if id >= 1 && int(id) <= len(p.cm.list) {
			p.cm.list[id-1].marked = true
		}
	}
}

// addTextWithPoints adds text, the first word of it taking the point
// comments waiting for one.
func (p *docxParser) addTextWithPoints(ps *paraState, in irInline) {
	s := strings.IndexFunc(in.text, func(r rune) bool { return !unicode.IsSpace(r) })
	if s < 0 {
		p.addInline(ps, in)
		return
	}
	e := strings.IndexFunc(in.text[s:], unicode.IsSpace)
	if e < 0 {
		e = len(in.text)
	} else {
		e += s
	}
	if s > 0 {
		head := in
		head.text = in.text[:s]
		p.addInline(ps, head)
	}
	mid := in
	mid.text = in.text[s:e]
	for _, c := range p.pendingCm {
		mid.comments = addCommentKey(mid.comments, c.Local)
	}
	p.pendingCm = nil
	p.addInline(ps, mid)
	if e < len(in.text) {
		tail := in
		tail.text = in.text[e:]
		p.addInline(ps, tail)
	}
}

func addCommentKey(key string, id int64) string {
	ids := commentKeyIDs(key)
	if slices.Contains(ids, id) {
		return key
	}
	ids = append(ids, id)
	slices.Sort(ids)
	parts := make([]string, len(ids))
	for i, x := range ids {
		parts[i] = strconv.FormatInt(x, 10)
	}
	return strings.Join(parts, ",")
}

func unionCommentKeys(a, b string) string {
	for _, id := range commentKeyIDs(b) {
		a = addCommentKey(a, id)
	}
	return a
}

func commentKeyIDs(key string) []int64 {
	if key == "" {
		return nil
	}
	var out []int64
	for _, s := range strings.Split(key, ",") {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil {
			out = append(out, id)
		}
	}
	return out
}

// commentMarks are the marks for an irInline's comments.
func commentMarks(key string) []pmMark {
	var ms []pmMark
	for _, id := range commentKeyIDs(key) {
		ms = append(ms, pmMark{Type: "comment", Attrs: map[string]any{"id": id}})
	}
	return ms
}

// finishComments gives each comment the chapter its text starts in, drops
// its marks elsewhere and fills imp.Comments (docxemit.go's finish).
func (p *docxParser) finishComments(imp *docxImport) {
	if p.cm == nil {
		return
	}
	owner := map[int64]int{}
	for i, ch := range imp.Chapters {
		for id := range docCommentIDs(ch.Doc) {
			if _, ok := owner[id]; !ok {
				owner[id] = i
			}
		}
	}
	for i, ch := range imp.Chapters {
		keep := map[int64]int64{}
		for id, o := range owner {
			if o == i {
				keep[id] = id
			}
		}
		mapCommentMarks(ch.Doc, keep)
	}
	// Locals are renumbered to the kept comments, in the file's order.
	newID := map[*docxCmt]int64{}
	for _, c := range p.cm.list {
		root := c
		if c.parent != nil {
			root = c.parent
		}
		ch, ok := owner[root.Local]
		if !ok {
			p.count("comments")
			continue
		}
		out := c.docxComment
		out.Chapter = ch
		out.Parent = 0
		imp.Comments = append(imp.Comments, out)
		newID[c] = int64(len(imp.Comments))
	}
	renumber := map[int64]int64{}
	for i := range imp.Comments {
		c := p.cm.list[imp.Comments[i].Local-1]
		renumber[c.Local] = newID[c]
		if c.parent != nil {
			imp.Comments[i].Parent = newID[c.parent]
		}
		imp.Comments[i].Local = newID[c]
	}
	for _, ch := range imp.Chapters {
		mapCommentMarks(ch.Doc, renumber)
	}
	if n := p.leftOut["comments"]; n > 0 {
		imp.LeftOut["comments"] = n
	}
}

// --- export (exportdocx.go) ---

type docxCommentOut struct {
	c      *exportComment
	wordID int
	paraID string // of its last paragraph
}

// docxCommentsOut places a book's comments in the .docx body.
type docxCommentsOut struct {
	starts, ends map[*pmNode][]*docxCommentOut // the thread's root, at its first and last text
	replies      map[int64][]*docxCommentOut
	all          []*docxCommentOut // in Word id order
}

func newDocxCommentsOut(b *exportBook) *docxCommentsOut {
	out := &docxCommentsOut{starts: map[*pmNode][]*docxCommentOut{}, ends: map[*pmNode][]*docxCommentOut{}, replies: map[int64][]*docxCommentOut{}}
	if len(b.Comments) == 0 {
		return out
	}
	first, last := map[int64]*pmNode{}, map[int64]*pmNode{}
	var order []int64
	for _, ch := range b.Chapters {
		pmWalk(ch.Doc, func(n *pmNode) {
			if n.Type != "text" {
				return
			}
			for _, m := range n.Marks {
				id := commentMarkID(m)
				c := b.Comments[id]
				if c == nil || c.ParentID != 0 || c.ChapterID != ch.ID {
					continue
				}
				if first[id] == nil {
					first[id] = n
					order = append(order, id)
				}
				last[id] = n
			}
		})
	}
	var replies []*exportComment
	for _, c := range b.Comments {
		if c.ParentID != 0 {
			replies = append(replies, c)
		}
	}
	sort.Slice(replies, func(i, j int) bool { return replies[i].ID < replies[j].ID })
	for _, id := range order {
		root := &docxCommentOut{c: b.Comments[id], wordID: len(out.all)}
		out.all = append(out.all, root)
		for _, r := range replies {
			if r.ParentID == id {
				x := &docxCommentOut{c: r, wordID: len(out.all)}
				out.all = append(out.all, x)
				out.replies[id] = append(out.replies[id], x)
			}
		}
		out.starts[first[id]] = append(out.starts[first[id]], root)
		out.ends[last[id]] = append(out.ends[last[id]], root)
	}
	return out
}

// before writes the starts of the comments whose text begins at n.
func (cm *docxCommentsOut) before(p *docxPara, n *pmNode) {
	for _, c := range cm.starts[n] {
		for _, x := range append([]*docxCommentOut{c}, cm.replies[c.c.ID]...) {
			fmt.Fprintf(&p.body, `<w:commentRangeStart w:id="%d"/>`, x.wordID)
		}
	}
}

// after writes the ends and references of the comments whose text ends at n.
func (cm *docxCommentsOut) after(p *docxPara, n *pmNode) {
	for _, c := range cm.ends[n] {
		for _, x := range append([]*docxCommentOut{c}, cm.replies[c.c.ID]...) {
			fmt.Fprintf(&p.body, `<w:commentRangeEnd w:id="%d"/><w:r><w:commentReference w:id="%d"/></w:r>`, x.wordID, x.wordID)
		}
	}
}

const (
	commentsExtRelType = "http://schemas.microsoft.com/office/2011/relationships/commentsExtended"
	w14NS              = "http://schemas.microsoft.com/office/word/2010/wordml"
	w15NS              = "http://schemas.microsoft.com/office/word/2012/wordml"
)

func commentInitials(name string) string {
	var b strings.Builder
	for _, w := range strings.Fields(name) {
		r := []rune(w)
		b.WriteRune(unicode.ToUpper(r[0]))
		if b.Len() >= 3 {
			break
		}
	}
	return b.String()
}

// commentParts adds comments.xml and commentsExtended.xml to the package
// (nothing when the book has no comments in its text).
func (d *docxOut) commentParts() {
	cm := d.cm
	if cm == nil || len(cm.all) == 0 {
		return
	}
	author := strings.TrimSpace(d.b.Meta.Author)
	if author == "" {
		author = "Polaris"
	}
	var x strings.Builder
	x.WriteString(xmlHead + `<w:comments ` + docxNSDecl + ` xmlns:w14="` + w14NS + `" xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006" mc:Ignorable="w14">`)
	para := 0
	for _, c := range cm.all {
		at := c.c.CreatedAt
		if at.IsZero() {
			at = editorNow()
		}
		fmt.Fprintf(&x, `<w:comment w:id="%d" w:author="%s" w:date="%s" w:initials="%s">`, c.wordID, xe(author), at.UTC().Format("2006-01-02T15:04:05Z"), xe(commentInitials(author)))
		for i, line := range strings.Split(c.c.Text, "\n") {
			para++
			c.paraID = fmt.Sprintf("%08X", 0x10000000+para)
			fmt.Fprintf(&x, `<w:p w14:paraId="%s" w14:textId="77777777">`, c.paraID)
			if i == 0 {
				x.WriteString(`<w:r><w:annotationRef/></w:r>`)
			}
			if line = cleanXMLText(line); line != "" {
				x.WriteString(`<w:r>`)
				for j, part := range strings.Split(line, "\t") {
					if j > 0 {
						x.WriteString(`<w:tab/>`)
					}
					if part != "" {
						x.WriteString(`<w:t xml:space="preserve">` + xe(part) + `</w:t>`)
					}
				}
				x.WriteString(`</w:r>`)
			}
			x.WriteString(`</w:p>`)
		}
		x.WriteString(`</w:comment>`)
	}
	x.WriteString(`</w:comments>`)

	var ex strings.Builder
	ex.WriteString(xmlHead + `<w15:commentsEx xmlns:w15="` + w15NS + `" xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006" mc:Ignorable="w15">`)
	paraOf := map[int64]string{}
	for _, c := range cm.all {
		paraOf[c.c.ID] = c.paraID
	}
	for _, c := range cm.all {
		done := "0"
		if c.c.Resolved || (c.c.ParentID != 0 && cm.resolvedRoot(c.c.ParentID)) {
			done = "1"
		}
		fmt.Fprintf(&ex, `<w15:commentEx w15:paraId="%s"`, c.paraID)
		if c.c.ParentID != 0 {
			fmt.Fprintf(&ex, ` w15:paraIdParent="%s"`, paraOf[c.c.ParentID])
		}
		fmt.Fprintf(&ex, ` w15:done="%s"/>`, done)
	}
	ex.WriteString(`</w15:commentsEx>`)

	const wml = "application/vnd.openxmlformats-officedocument.wordprocessingml."
	d.rel("comments", "comments.xml", false)
	id := "rId" + strconv.Itoa(d.relN)
	d.relN++
	fmt.Fprintf(&d.rels, `<Relationship Id="%s" Type="%s" Target="commentsExtended.xml"/>`, id, commentsExtRelType)
	d.parts = append(d.parts,
		docxPart{name: "word/comments.xml", contentType: wml + "comments+xml", data: []byte(x.String())},
		docxPart{name: "word/commentsExtended.xml", contentType: wml + "commentsExtended+xml", data: []byte(ex.String())})
}

func (cm *docxCommentsOut) resolvedRoot(id int64) bool {
	for _, c := range cm.all {
		if c.c.ID == id {
			return c.c.Resolved
		}
	}
	return false
}
